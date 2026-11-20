// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
)

// releaseClaim writes ticket 23's claim for keeper through the Comb and returns
// the backend.
func releaseClaim(t *testing.T, f *fixture, keeper string) *comb.GitBackend {
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
	if err = b.AcquireLease(context.Background(), comb.Lease{TicketID: "T-023", KeeperID: keeper, Role: "builder", Gate: "full", Directories: []string{"internal/x"}}); err != nil {
		t.Fatal(err)
	}
	return b
}

// branchWith creates branch from main with one commit of its own and returns to main.
func branchWith(f *fixture, branch, file string) {
	f.git("checkout", "-q", "-b", branch, "main")
	write(f.t, f.real, file, branch+"\n")
	f.git("add", file)
	f.git("commit", "-q", "-m", "T-023: "+branch)
	f.git("checkout", "-q", "main")
}

func doRelease(f *fixture, mut ...func(*ReleaseRequest)) (string, error) {
	r := ReleaseRequest{RepoDir: f.repo, Ticket: "23"}
	for _, m := range mut {
		m(&r)
	}
	var out bytes.Buffer
	res, err := ReleaseTask(context.Background(), r)
	for _, w := range res.Warnings {
		out.WriteString("warning: " + w + "\n")
	}
	if err == nil {
		out.WriteString(ReleaseLine(res) + "\n")
	}
	return out.String(), err
}

func unmerged(r *ReleaseRequest) { r.Unmerged = true }

func claimKept(t *testing.T, f *fixture) bool {
	t.Helper()
	return f.git("for-each-ref", "refs/waggle/claims/ticket-23") != ""
}

// AC3: an owned claim whose branch merged is deleted, and an overlapping ticket can claim at once.
func TestReleaseClaimMerged(t *testing.T) {
	f := newFixture(t, false, false)
	b := releaseClaim(t, f, "alice")
	branchWith(f, "t-023", "internal/x/b.go")
	f.git("merge", "-q", "--no-ff", "-m", "merge t-023", "t-023")
	out, err := doRelease(f)
	if err != nil || !strings.HasPrefix(out, "released T-023 (claim ") || !strings.HasSuffix(out, ", merged into main)\n") {
		t.Fatalf("out %q err %v", out, err)
	}
	if claimKept(t, f) {
		t.Fatal("the claim ref is still there")
	}
	err = b.AcquireLease(context.Background(), comb.Lease{TicketID: "T-024", KeeperID: "alice", Role: "builder", Gate: "full", Directories: []string{"internal/x"}})
	if err != nil {
		t.Fatalf("an overlapping ticket cannot claim: %v", err)
	}
}

// AC4: every refusal is exit 4 (3 for a held lock) and leaves the claim.
func TestReleaseClaimRefusals(t *testing.T) {
	wtDir := ".waggle/worktrees/"
	cases := []struct {
		name  string
		setup func(f *fixture)
		mut   func(*ReleaseRequest)
		exit  int
	}{
		{"branch not merged", func(f *fixture) { branchWith(f, "t-023", "internal/x/b.go") }, nil, ExitClaim},
		{"branch absent", func(f *fixture) {}, nil, ExitClaim},
		{"branch for another ticket", func(f *fixture) {}, func(r *ReleaseRequest) { r.Branch = "t-024" }, ExitUsage},
		{"no-commit branch at a tip that a merge has as first parent", func(f *fixture) {
			f.git("branch", "t-023", "main")
			branchWith(f, "t-023b", "internal/x/b.go")
			f.git("merge", "-q", "--no-ff", "-m", "merge t-023b", "t-023b")
		}, nil, ExitClaim},
		{"no-commit branch at a tip that a branch updated from main has as second parent", func(f *fixture) {
			branchWith(f, "t-023b", "internal/x/b.go")
			write(f.t, f.real, "internal/x/c.go", "main\n")
			f.git("add", "internal/x/c.go")
			f.git("commit", "-q", "-m", "advance main")
			f.git("branch", "t-023", "main")
			f.git("checkout", "-q", "t-023b")
			f.git("merge", "-q", "--no-ff", "-m", "update t-023b from main", "main")
			f.git("checkout", "-q", "main")
			f.git("merge", "-q", "--no-ff", "-m", "merge t-023b", "t-023b")
		}, nil, ExitClaim},
		{"worktree exists", func(f *fixture) {
			f.git("worktree", "add", "-q", "-b", "t-023", filepath.Join(f.real, wtDir, "T-023-builder"), "main")
		}, unmerged, ExitClaim},
		{"lone run lock", func(f *fixture) { write(t, f.real, wtDir+"T-023-guard.lock", "") }, unmerged, ExitClaim},
		{"tainted marker", func(f *fixture) { write(t, f.real, wtDir+"T-023-builder.tainted", "run-1") }, unmerged, ExitClaim},
		{"takeover guard", func(f *fixture) { write(t, f.real, wtDir+"T-023-forager.lock.takeover", "") }, unmerged, ExitClaim},
		{"held dispatch.lock", func(f *fixture) {
			write(t, f.real, wtDir+"keep", "")
			writeLock(t, filepath.Join(f.real, wtDir, "dispatch.lock"), os.Getpid())
		}, unmerged, ExitEnv},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, false, false)
			releaseClaim(t, f, "alice")
			c.setup(f)
			var mut []func(*ReleaseRequest)
			if c.mut != nil {
				mut = append(mut, c.mut)
			}
			if out, err := doRelease(f, mut...); code(err) != c.exit {
				t.Fatalf("out %q err %v, want exit %d", out, err, c.exit)
			}
			if !claimKept(t, f) {
				t.Fatal("the claim ref was deleted")
			}
			if hasReceipts(f) {
				t.Fatal("a refused release wrote a receipt")
			}
		})
	}
}

func TestReleaseClaimOtherKeepersClaim(t *testing.T) {
	f := newFixture(t, false, false)
	releaseClaim(t, f, "bob")
	if _, err := doRelease(f, unmerged); code(err) != ExitClaim || !errors.Is(err, comb.ErrNotOwner) || !claimKept(t, f) {
		t.Fatalf("err %v, claim kept %v", err, claimKept(t, f))
	}
}

func TestReleaseClaimUnmerged(t *testing.T) {
	f := newFixture(t, false, false)
	releaseClaim(t, f, "alice")
	out, err := doRelease(f, unmerged)
	if err != nil || !strings.HasSuffix(out, ", unmerged)\n") || claimKept(t, f) {
		t.Fatalf("out %q err %v, claim kept %v", out, err, claimKept(t, f))
	}
}

// AC5: no claim is exit 0; a malformed claim is exit 4 with the hint.
func TestReleaseClaimNoneAndMalformed(t *testing.T) {
	f := newFixture(t, false, false)
	if out, err := doRelease(f); err != nil || out != "no claim for T-023\n" {
		t.Fatalf("out %q err %v", out, err)
	}
	f.git("update-ref", "refs/waggle/claims/ticket-23", "HEAD") // a commit that is no lease
	if _, err := doRelease(f, unmerged); code(err) != ExitClaim || !strings.Contains(err.Error(), "delete the malformed claim ref by hand") || !claimKept(t, f) {
		t.Fatalf("err %v, claim kept %v", err, claimKept(t, f))
	}
}

// fakeRelease is a ClaimBackend whose release fails with err.
type fakeRelease struct {
	ClaimBackend
	err error
}

func (f fakeRelease) ClaimOID(context.Context, string) (string, error) {
	return "0123456789abcdef", nil
}

func (f fakeRelease) ReleaseLease(context.Context, string, string) error { return f.err }

// AC6: a lost compare-and-swap is exit 4, relay mode is exit 3.
func TestReleaseClaimBackendErrors(t *testing.T) {
	for _, c := range []struct {
		err  error
		exit int
	}{{comb.ErrConcurrentUpdate, ExitClaim}, {comb.ErrRelayNotImplemented, ExitEnv}, {errors.New("transport"), ExitEnv}} {
		f := newFixture(t, false, false)
		releaseClaim(t, f, "alice")
		out, err := doRelease(f, unmerged, func(r *ReleaseRequest) { r.Backend = fakeRelease{err: c.err} })
		if code(err) != c.exit || out != "" || !claimKept(t, f) {
			t.Fatalf("%v: out %q err %v, claim kept %v", c.err, out, err, claimKept(t, f))
		}
	}
}

// Remote mode: the head is the pushed branch (the local one is gone) and the
// claim is deleted on the origin. These push to a local origin.
func TestReleaseClaimRemote(t *testing.T) {
	f := newFixture(t, true, false)
	releaseClaim(t, f, "alice")
	remoteClaim := func() string { return sh(t, f.origin, "for-each-ref", "refs/waggle/claims/ticket-23") }
	branchWith(f, "t-023", "internal/x/b.go")
	f.git("push", "-q", "origin", "t-023")
	if _, err := doRelease(f); code(err) != ExitClaim || remoteClaim() == "" {
		t.Fatalf("unmerged: err %v, claim on origin %q", err, remoteClaim())
	}
	f.git("merge", "-q", "--no-ff", "-m", "merge t-023", "t-023")
	f.git("push", "-q", "origin", "main")
	f.git("branch", "-q", "-D", "t-023")
	out, err := doRelease(f)
	if err != nil || !strings.Contains(out, "merged into main") || remoteClaim() != "" || claimKept(t, f) {
		t.Fatalf("out %q err %v, claim on origin %q", out, err, remoteClaim())
	}
}
