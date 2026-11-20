// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/forge"
)

// fakeForge answers MergedPRByHead from a fixed answer, as the adapter would
// after one GitHub read.
type fakeForge struct {
	pr    MergedPR
	found bool
	err   error
	calls int
}

func (f *fakeForge) MergedPRByHead(context.Context, []string) (MergedPR, bool, error) {
	f.calls++
	return f.pr, f.found, f.err
}

// claimFor writes ticket 23's claim for keeper through the Comb, naming branches.
func claimFor(t *testing.T, f *fixture, keeper string, branches ...string) *comb.GitBackend {
	t.Helper()
	cfg, err := config.Load(filepath.Join(f.real, ".waggle/control/config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	opts, err := cfg.CombOptions(f.real)
	if err != nil {
		t.Fatal(err)
	}
	opts.ExtraEnv = combEnv
	b, err := comb.NewGitBackend(opts)
	if err != nil {
		t.Fatal(err)
	}
	l := comb.Lease{TicketID: "T-023", KeeperID: keeper, Role: "builder", Gate: "full", Directories: []string{"internal/x"}, Branches: branches}
	if err = b.AcquireLease(context.Background(), l); err != nil {
		t.Fatal(err)
	}
	return b
}

const receiptRef = "refs/waggle/receipts/main"

func hasReceipts(f *fixture) bool { return f.git("for-each-ref", receiptRef) != "" }

// lastReceipt reads the newest receipt and its commit.
func lastReceipt(t *testing.T, f *fixture) (comb.Receipt, string) {
	t.Helper()
	oid := f.git("rev-parse", receiptRef)
	var r comb.Receipt
	if err := json.Unmarshal([]byte(f.git("cat-file", "blob", oid+":receipt.json")), &r); err != nil {
		t.Fatal(err)
	}
	return r, oid
}

// squashOnMain lands one commit with subject on main, as a squash merge would.
func squashOnMain(f *fixture, file, subject string) string {
	write(f.t, f.real, file, subject+"\n")
	f.git("add", file)
	f.git("commit", "-q", "-m", subject)
	return f.git("rev-parse", "HEAD")
}

func str(p *string) string {
	if p == nil {
		return "<null>"
	}
	return *p
}

// A1: a merge commit and a forge that reports the PR.
func TestReceiptForgeMergeCommit(t *testing.T) {
	f := newFixture(t, false, false)
	claimFor(t, f, "alice", "t-023")
	claimOID := f.git("rev-parse", "refs/waggle/claims/ticket-23")
	branchWith(f, "t-023", "internal/x/b.go")
	f.git("merge", "-q", "--no-ff", "-m", "merge t-023", "t-023")
	m := f.git("rev-parse", "HEAD")
	fg := &fakeForge{found: true, pr: MergedPR{Number: 234, Author: "alice", HeadBranch: "t-023", MergeSHA: m, Approvers: []string{"bob"}}}
	res, err := ReleaseTask(context.Background(), ReleaseRequest{RepoDir: f.repo, Ticket: "23", Forge: fg})
	if err != nil || len(res.Warnings) != 0 || res.Receipt == "" {
		t.Fatalf("res %+v err %v", res, err)
	}
	r, oid := lastReceipt(t, f)
	if oid != res.Receipt || claimKept(t, f) {
		t.Errorf("receipt %s vs result %s; claim kept %v", oid, res.Receipt, claimKept(t, f))
	}
	if ps := strings.Fields(f.git("rev-list", "--parents", "-n", "1", oid)); len(ps) != 3 || ps[2] != claimOID {
		t.Errorf("receipt parents %v; second must be the claim %s", ps, claimOID)
	}
	if r.MergeProof != "forge" || str(r.MergedSHA) != m || str(r.BaseSHA) != f.git("rev-parse", m+"^1") || r.Status != "merged" || r.Incomplete ||
		r.MergedAt == nil || strconv.FormatInt(r.MergedAt.Unix(), 10) != f.git("log", "-1", "--format=%ct", m) ||
		r.PRNumber == nil || *r.PRNumber != 234 || str(r.PRAuthor) != "alice" || str(r.HeadBranch) != "t-023" ||
		len(r.Approvers) != 1 || r.Approvers[0] != "bob" || r.ReleasedBy != "alice" || r.KeeperID != "alice" || r.Source != "release" {
		t.Errorf("receipt = %+v", r)
	}
}

// A2: --unmerged and the resolver order.
func TestReceiptResolverSteps(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(f *fixture) (squash string)
		forge     func(squash string) PRLookup
		mut       func(*ReleaseRequest)
		proof     string
		incompl   bool
		mergedSet bool
	}{
		{"squash with the forge", func(f *fixture) string { return squashOnMain(f, "internal/x/s.go", "T-023: squash") },
			func(s string) PRLookup {
				return &fakeForge{found: true, pr: MergedPR{Number: 5, Author: "alice", HeadBranch: "t-023", MergeSHA: s}}
			}, unmerged, "forge", false, true},
		{"squash with provider none", func(f *fixture) string { return squashOnMain(f, "internal/x/s.go", "T-023: squash") },
			func(string) PRLookup { return nil }, unmerged, "binding", true, true},
		{"squash, forge has no such PR", func(f *fixture) string { return squashOnMain(f, "internal/x/s.go", "T-023: squash") },
			func(string) PRLookup { return &fakeForge{} }, unmerged, "binding", false, true},
		{"forge merge commit not on main", func(f *fixture) string { return squashOnMain(f, "internal/x/s.go", "T-023: squash") },
			func(string) PRLookup {
				return &fakeForge{found: true, pr: MergedPR{Number: 5, Author: "alice", HeadBranch: "t-023", MergeSHA: strings.Repeat("c", 40)}}
			}, unmerged, "binding", false, true},
		{"forge head branch not on the claim", func(f *fixture) string { return squashOnMain(f, "internal/x/s.go", "T-023: squash") },
			func(s string) PRLookup {
				return &fakeForge{found: true, pr: MergedPR{Number: 5, Author: "alice", HeadBranch: "other", MergeSHA: s}}
			}, unmerged, "binding", false, true},
		{"trailer binds", func(f *fixture) string {
			write(f.t, f.real, "internal/x/s.go", "x\n")
			f.git("add", "internal/x/s.go")
			f.git("commit", "-q", "-m", "squash\n\nWaggle-Ticket: T-023")
			return f.git("rev-parse", "HEAD")
		}, func(string) PRLookup { return nil }, unmerged, "binding", true, true},
		{"a rebase run binds at its last commit", func(f *fixture) string {
			squashOnMain(f, "internal/x/r1.go", "T-023: first")
			return squashOnMain(f, "internal/x/r2.go", "T-023: second")
		}, func(string) PRLookup { return nil }, unmerged, "binding", true, true},
		{"two separate runs give none", func(f *fixture) string {
			squashOnMain(f, "internal/x/r1.go", "T-023: first")
			squashOnMain(f, "internal/x/o.go", "T-024: other")
			return squashOnMain(f, "internal/x/r2.go", "T-023: second")
		}, func(string) PRLookup { return nil }, unmerged, "none", true, false},
		{"a conflicting subject and trailer bind none", func(f *fixture) string {
			write(f.t, f.real, "internal/x/s.go", "x\n")
			f.git("add", "internal/x/s.go")
			f.git("commit", "-q", "-m", "T-023: squash\n\nWaggle-Ticket: T-024")
			return ""
		}, func(string) PRLookup { return nil }, unmerged, "none", true, false},
		{"a maintainer commit binds none", func(f *fixture) string { return squashOnMain(f, "internal/x/s.go", "Head Keeper: T-023: records") },
			func(string) PRLookup { return nil }, unmerged, "none", true, false},
		{"no bound commit", func(f *fixture) string { return "" },
			func(string) PRLookup { return nil }, unmerged, "none", true, false},
		{"merged tip when nothing binds", func(f *fixture) string {
			f.git("checkout", "-q", "-b", "t-023", "main")
			write(f.t, f.real, "internal/x/b.go", "b\n")
			f.git("add", "internal/x/b.go")
			f.git("commit", "-q", "-m", "wip")
			f.git("checkout", "-q", "main")
			f.git("merge", "-q", "--no-ff", "-m", "merge t-023", "t-023")
			return f.git("rev-parse", "HEAD")
		}, func(string) PRLookup { return nil }, nil, "merged_tip", true, true},
		{"a merge commit binds through its commits", func(f *fixture) string {
			branchWith(f, "t-023", "internal/x/b.go")
			f.git("merge", "-q", "--no-ff", "-m", "merge t-023", "t-023")
			return f.git("rev-parse", "HEAD")
		}, func(string) PRLookup { return nil }, nil, "binding", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, false, false)
			claimFor(t, f, "alice", "t-023")
			want := c.setup(f)
			req := ReleaseRequest{RepoDir: f.repo, Ticket: "23", Forge: c.forge(want)}
			if c.mut != nil {
				c.mut(&req)
			}
			res, err := ReleaseTask(context.Background(), req)
			if err != nil || len(res.Warnings) != 0 {
				t.Fatalf("res %+v err %v", res, err)
			}
			r, _ := lastReceipt(t, f)
			if r.MergeProof != c.proof || r.Incomplete != c.incompl {
				t.Fatalf("proof %s incomplete %v; want %s %v", r.MergeProof, r.Incomplete, c.proof, c.incompl)
			}
			if c.mergedSet {
				if r.Status != "merged" || str(r.MergedSHA) != want || r.BaseSHA == nil || r.MergedAt == nil {
					t.Errorf("receipt = %+v; merged_sha want %s", r, want)
				}
			} else if r.Status != "unmerged" || r.MergedSHA != nil || r.BaseSHA != nil || r.MergedAt != nil {
				t.Errorf("receipt = %+v; want unmerged with null SHAs", r)
			}
		})
	}
}

// A2 and RFC-0005 6.2: a claim renewed after the merge still finds it, since
// the binding step looks back to the first claim commit.
func TestReceiptBindingAfterRenewal(t *testing.T) {
	f := newFixture(t, false, false)
	b := claimFor(t, f, "alice", "t-023")
	want := squashOnMain(f, "internal/x/s.go", "T-023: squash")
	// Renewal with a claimed_at after the squash: the lease's own time no longer precedes it.
	lease, err := b.ReadClaim(context.Background(), "T-023")
	if err != nil {
		t.Fatal(err)
	}
	lease.ClaimedAt = lease.ClaimedAt.Add(2 * 3600e9)
	lease.ExpiresAt = lease.ClaimedAt.Add(2 * 3600e9)
	if err := b.AcquireLease(context.Background(), lease); err != nil {
		t.Fatal(err)
	}
	if _, err := doRelease(f, unmerged); err != nil {
		t.Fatal(err)
	}
	if r, _ := lastReceipt(t, f); r.MergeProof != "binding" || str(r.MergedSHA) != want {
		t.Errorf("receipt = %+v; want binding at %s", r, want)
	}
}

// A3: a forge failure releases, writes an incomplete receipt and warns once.
func TestReceiptForgeFailure(t *testing.T) {
	// The failures come from the real GitHub adapter reading an httptest server.
	adapter := func(h http.HandlerFunc, timeout time.Duration) func(*testing.T) PRLookup {
		return func(t *testing.T) PRLookup {
			srv := httptest.NewServer(h)
			t.Cleanup(srv.Close)
			pf, err := forge.NewTokenPRFinder("o/r", "tok", srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			return forgeLookup{finder: pf, timeout: timeout}
		}
	}
	for _, c := range []struct {
		name   string
		lookup func(*testing.T) PRLookup
		warns  int
	}{
		{"HTTP 500", adapter(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }, time.Minute), 1},
		{"timeout", adapter(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }, 100*time.Millisecond), 1},
		{"provider none", func(*testing.T) PRLookup { return forgeLookup{provider: "none", opt: forge.Options{RepoDir: "."}} }, 0},
		{"no forge", func(*testing.T) PRLookup { return &fakeForge{err: ErrNoForge} }, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, false, false)
			claimFor(t, f, "alice", "t-023")
			want := squashOnMain(f, "internal/x/s.go", "T-023: squash")
			res, err := ReleaseTask(context.Background(), ReleaseRequest{RepoDir: f.repo, Ticket: "23", Unmerged: true, Forge: c.lookup(t)})
			if err != nil || len(res.Warnings) != c.warns || res.Outcome != "released" || claimKept(t, f) {
				t.Fatalf("res %+v err %v", res, err)
			}
			if r, _ := lastReceipt(t, f); !r.Incomplete || r.MergeProof != "binding" || str(r.MergedSHA) != want || r.PRNumber != nil {
				t.Errorf("receipt = %+v", r)
			}
		})
	}
}

// A3: forge data the receipt rules refuse is dropped, never a reason to keep the claim.
func TestReceiptForgeUnusableAnswer(t *testing.T) {
	f := newFixture(t, false, false)
	claimFor(t, f, "alice", "t-023")
	m := squashOnMain(f, "internal/x/s.go", "T-023: squash")
	fg := &fakeForge{found: true, pr: MergedPR{Number: 5, Author: "alice", HeadBranch: "t-023", MergeSHA: m, Approvers: []string{"not a login"}}}
	res, err := ReleaseTask(context.Background(), ReleaseRequest{RepoDir: f.repo, Ticket: "23", Unmerged: true, Forge: fg})
	if err != nil || len(res.Warnings) != 1 || claimKept(t, f) {
		t.Fatalf("res %+v err %v", res, err)
	}
	if r, _ := lastReceipt(t, f); !r.Incomplete || r.PRNumber != nil || len(r.Approvers) != 0 || r.MergeProof != "forge" {
		t.Errorf("receipt = %+v", r)
	}
}

// A5: refusals write no receipt and keep the claim.
func TestReceiptRefusalsWriteNothing(t *testing.T) {
	f := newFixture(t, false, false)
	if _, err := doRelease(f, unmerged); err != nil || hasReceipts(f) {
		t.Fatalf("no claim: err %v, receipts %v", err, hasReceipts(f))
	}
	claimFor(t, f, "bob", "t-023")
	if _, err := doRelease(f, unmerged); !errors.Is(err, comb.ErrNotOwner) || hasReceipts(f) || !claimKept(t, f) {
		t.Fatalf("not owner: err %v", err)
	}
	f.git("update-ref", "-d", "refs/waggle/claims/ticket-23")
	claimFor(t, f, "alice", "t-023")
	if _, err := doRelease(f); code(err) != ExitClaim || hasReceipts(f) || !claimKept(t, f) {
		t.Fatalf("not merged: err %v", err)
	}
}

// A6: --co.
func TestReceiptCoKeepers(t *testing.T) {
	ok := []struct {
		name string
		co   []string
	}{{"one", []string{"bob"}}, {"four", []string{"b", "c", "d", "e"}}}
	for _, c := range ok {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, false, false)
			claimFor(t, f, "alice", "t-023")
			if _, err := doRelease(f, unmerged, func(r *ReleaseRequest) { r.CoKeepers = c.co }); err != nil {
				t.Fatal(err)
			}
			if r, _ := lastReceipt(t, f); strings.Join(r.CoKeepers, ",") != strings.Join(c.co, ",") {
				t.Errorf("co_keepers = %v", r.CoKeepers)
			}
		})
	}
	for _, c := range []struct {
		name string
		co   []string
	}{{"five", []string{"b", "c", "d", "e", "f"}}, {"duplicate", []string{"bob", "bob"}}, {"owner", []string{"alice"}}, {"bad id", []string{"-x"}}} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, false, false)
			claimFor(t, f, "alice", "t-023")
			if _, err := doRelease(f, unmerged, func(r *ReleaseRequest) { r.CoKeepers = c.co }); code(err) != ExitUsage || hasReceipts(f) || !claimKept(t, f) {
				t.Fatalf("err %v (code %d), receipts %v", err, code(err), hasReceipts(f))
			}
		})
	}
}

// A4 at the release level, remote mode: the claim leaves and the receipt lands on origin together.
func TestReceiptRemoteRelease(t *testing.T) {
	f := newFixture(t, true, false)
	claimFor(t, f, "alice", "t-023")
	squashOnMain(f, "internal/x/s.go", "T-023: squash")
	f.git("push", "-q", "origin", "main")
	if _, err := doRelease(f, unmerged); err != nil {
		t.Fatal(err)
	}
	if got := sh(t, f.origin, "for-each-ref", "--format=%(refname)", "refs/waggle/"); got != receiptRef {
		t.Errorf("origin refs = %q; want only the receipt ref", got)
	}
	if r, _ := lastReceipt(t, f); r.MergeProof != "binding" {
		t.Errorf("receipt = %+v", r)
	}
}

// bindTask follows the gate's subject and trailer rules (cmd/waggle/gate_commits.go).
func TestBindTask(t *testing.T) {
	for _, c := range []struct{ subject, trailer, want string }{
		{"T-023: work", "", "T-023"},
		{"work", "T-023", "T-023"},
		{"work", "T-023, T-023", "T-023"},
		{"T-023: work", "T-023", "T-023"},
		{"T-023: work", "T-024", ""},
		{"work", "T-023,T-024", ""},
		{"work", "bogus", ""},
		{"Head Keeper: T-023: records", "", ""},
		{"Head Keeper: work", "T-023", ""},
		{"t-023: work", "", ""},
		{"T-23: work", "", ""},
		{"work", "", ""},
		{"work", strings.Repeat("T-023,", 9), ""},
	} {
		if got := bindTask(c.subject, c.trailer); got != c.want {
			t.Errorf("bindTask(%q, %q) = %q; want %q", c.subject, c.trailer, got, c.want)
		}
	}
}
