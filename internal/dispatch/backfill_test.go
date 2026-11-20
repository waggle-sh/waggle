// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/decisions"
	"github.com/waggle-sh/waggle/internal/layout"
)

func backfill(t *testing.T, f *fixture, mut ...func(*BackfillRequest)) BackfillResult {
	t.Helper()
	req := BackfillRequest{RepoDir: f.repo}
	for _, m := range mut {
		m(&req)
	}
	res, err := Backfill(context.Background(), req)
	if err != nil {
		t.Fatalf("backfill: %v (%+v)", err, res)
	}
	return res
}

func apply(r *BackfillRequest) { r.Apply = true }

func item(t *testing.T, res BackfillResult, task string) BackfillItem {
	t.Helper()
	for _, it := range res.Items {
		if it.Task == task {
			return it
		}
	}
	t.Fatalf("no item for %s in %+v", task, res.Items)
	return BackfillItem{}
}

// writeDecision commits a DECISIONS.md that names tickets in one decision.
func writeDecision(f *fixture, tickets ...int) {
	e := decisions.Entry{ID: 1, Date: "2026-01-02", Kind: decisions.KindStrategy, Title: "Pick the plan", Question: "Which plan?",
		Options: []string{"a", "b"}, Recommendation: "a", Choice: "a", Rationale: "simple", Tickets: tickets}
	write(f.t, f.real, layout.Decisions, string(decisions.Render([]decisions.Entry{e})))
	f.git("add", layout.Decisions)
	f.git("commit", "-q", "-m", "Head Keeper: record a decision")
}

// A8: a live claim of a task that squash-merged. The dry run writes nothing,
// --apply appends one backfill receipt that holds the claim as second parent
// and leaves the claim ref, and a second --apply appends nothing.
func TestBackfillLiveClaim(t *testing.T) {
	f := newFixture(t, false, false)
	claimFor(t, f, "bob", "t-023")
	claimOID := f.git("rev-parse", "refs/waggle/claims/ticket-23")
	squash := squashOnMain(f, "internal/x/s.go", "T-023: squash")

	res := backfill(t, f)
	it := item(t, res, "T-023")
	if it.State != BackfillPlanned || it.Owner != "bob" || it.Origin != BackfillLiveClaim || it.MergedSHA != squash || it.Proof != comb.ProofBinding || res.Applied {
		t.Fatalf("plan item %+v", it)
	}
	if hasReceipts(f) {
		t.Fatal("the dry run wrote a receipt")
	}

	res = backfill(t, f, apply)
	if it = item(t, res, "T-023"); it.State != BackfillWritten || res.Written != 1 {
		t.Fatalf("applied item %+v written %d", it, res.Written)
	}
	r, oid := lastReceipt(t, f)
	if ps := strings.Fields(f.git("rev-list", "--parents", "-n", "1", oid)); len(ps) != 3 || ps[2] != claimOID {
		t.Errorf("receipt parents %v; second must be the claim %s", ps, claimOID)
	}
	if r.Source != "backfill" || r.KeeperID != "bob" || r.ReleasedBy != "alice" || r.TicketID != "T-023" || r.Role != "builder" || r.Gate != "full" ||
		str(r.MergedSHA) != squash || r.MergeProof != "binding" || r.Status != "merged" || !r.Incomplete || r.ClaimedAt == nil ||
		len(r.Branches) != 1 || r.Branches[0] != "t-023" || len(r.CoKeepers) != 0 || r.PRNumber != nil {
		t.Errorf("receipt = %+v", r)
	}
	if f.git("for-each-ref", "refs/waggle/claims/ticket-23") == "" {
		t.Error("backfill released the live claim")
	}
	before := f.git("rev-parse", receiptRef)
	res = backfill(t, f, apply)
	if len(res.Items) != 0 || res.Written != 0 || f.git("rev-parse", receiptRef) != before {
		t.Errorf("second apply: %+v", res)
	}
}

// A8: claims kept in the audit chain and in a receipt the reader cannot decode.
func TestBackfillClaimHistory(t *testing.T) {
	f := newFixture(t, false, false)
	b := claimFor(t, f, "bob", "t-023")
	oid23 := f.git("rev-parse", "refs/waggle/claims/ticket-23")
	squashOnMain(f, "internal/x/s23.go", "T-023: first")
	l := comb.Lease{TicketID: "T-024", KeeperID: "carol", Role: "builder", Gate: "full", Directories: []string{"internal/y"}, Branches: []string{"t-024"}}
	if err := b.AcquireLease(context.Background(), l); err != nil {
		t.Fatal(err)
	}
	oid24 := f.git("rev-parse", "refs/waggle/claims/ticket-24")
	squashOnMain(f, "internal/x/s24.go", "T-024: second")
	empty := f.git("mktree")
	// T-023 sits in the force-release audit chain, T-024 in a receipt that does not decode.
	root := f.git("commit-tree", empty, "-m", "root")
	audit := f.git("commit-tree", empty, "-p", root, "-p", oid23, "-m", "force release")
	f.git("update-ref", "refs/waggle/audit/force-releases", audit)
	blob := shIn(t, f.real, "{}\n", "hash-object", "-w", "--stdin")
	tree := shIn(t, f.real, "100644 blob "+blob+"\treceipt.json\n", "mktree")
	rroot := f.git("commit-tree", empty, "-m", "root")
	bad := f.git("commit-tree", tree, "-p", rroot, "-p", oid24, "-m", "bad receipt")
	f.git("update-ref", receiptRef, bad)
	f.git("update-ref", "-d", "refs/waggle/claims/ticket-23")
	f.git("update-ref", "-d", "refs/waggle/claims/ticket-24")

	res := backfill(t, f)
	a, c := item(t, res, "T-023"), item(t, res, "T-024")
	if a.State != BackfillPlanned || a.Owner != "bob" || a.Origin != BackfillClaimHistory ||
		c.State != BackfillPlanned || c.Owner != "carol" || c.Origin != BackfillClaimHistory {
		t.Fatalf("items %+v %+v", a, c)
	}
	res = backfill(t, f, apply)
	if res.Written != 2 {
		t.Fatalf("written %d", res.Written)
	}
	if n := f.git("rev-list", "--first-parent", "--count", receiptRef); n != "4" { // root, bad, two new
		t.Errorf("chain length %s; the old commits stay and two are appended", n)
	}
	if f.git("rev-parse", receiptRef+"~2") != bad {
		t.Error("the existing receipt was moved")
	}
}

// A8: a task only DECISIONS.md names is credited to --owner, else unattributed.
func TestBackfillDecisions(t *testing.T) {
	f := newFixture(t, false, false)
	squash := squashOnMain(f, "internal/x/s.go", "T-024: squash")
	writeDecision(f, 24)

	res := backfill(t, f)
	it := item(t, res, "T-024")
	if it.State != BackfillUnattributed || !strings.Contains(it.Evidence, "D-001") || !strings.Contains(it.Evidence, "--owner") {
		t.Fatalf("without --owner: %+v", it)
	}
	if _, err := Backfill(context.Background(), BackfillRequest{RepoDir: f.repo, Owner: "bad owner!"}); err == nil || ExitCode(err) != ExitUsage {
		t.Errorf("bad --owner: %v", err)
	}
	owner := func(r *BackfillRequest) { r.Owner = "carol" }
	res = backfill(t, f, owner)
	it = item(t, res, "T-024")
	if it.State != BackfillPlanned || it.Owner != "carol" || it.Origin != BackfillDecisions || it.MergedSHA != squash {
		t.Fatalf("with --owner: %+v", it)
	}
	if hasReceipts(f) {
		t.Fatal("the dry run wrote a receipt")
	}
	backfill(t, f, owner, apply)
	r, oid := lastReceipt(t, f)
	if ps := strings.Fields(f.git("rev-list", "--parents", "-n", "1", oid)); len(ps) != 2 {
		t.Errorf("a decisions receipt has no claim parent: %v", ps)
	}
	if r.Source != "backfill" || r.KeeperID != "carol" || r.ReleasedBy != "alice" || r.Role != "" || r.ClaimedAt != nil || len(r.Branches) != 0 ||
		str(r.MergedSHA) != squash || !r.Incomplete {
		t.Errorf("receipt = %+v", r)
	}
	if res = backfill(t, f, owner, apply); len(res.Items) != 0 {
		t.Errorf("second apply: %+v", res)
	}
}

// A8: tasks no source names, and a claim whose merge cannot be found, stay
// unattributed and get no receipt; a task that already has a receipt is left alone.
func TestBackfillUnattributed(t *testing.T) {
	f := newFixture(t, false, false)
	squashOnMain(f, "internal/x/s25.go", "T-025: nobody claimed this")
	t.Setenv("GIT_COMMITTER_DATE", "2020-01-01T00:00:00Z")
	squashOnMain(f, "internal/x/s23.go", "T-023: merged long before the claim")
	if err := os.Unsetenv("GIT_COMMITTER_DATE"); err != nil {
		t.Fatal(err)
	}
	b := claimFor(t, f, "bob", "t-023")
	l := comb.Lease{TicketID: "T-024", KeeperID: "alice", Role: "builder", Gate: "full", Directories: []string{"internal/y"}, Branches: []string{"t-024"}}
	if err := b.AcquireLease(context.Background(), l); err != nil {
		t.Fatal(err)
	}
	squashOnMain(f, "internal/x/s24.go", "T-024: released")
	if _, err := ReleaseTask(context.Background(), ReleaseRequest{RepoDir: f.repo, Ticket: "24", Unmerged: true}); err != nil {
		t.Fatal(err)
	}
	before := f.git("rev-parse", receiptRef)

	res := backfill(t, f, apply)
	if len(res.Items) != 2 || res.Written != 0 {
		t.Fatalf("items %+v", res.Items)
	}
	for _, task := range []string{"T-023", "T-025"} {
		if it := item(t, res, task); it.State != BackfillUnattributed || it.Owner != "" || it.MergedSHA != "" {
			t.Errorf("%s: %+v", task, it)
		}
	}
	if !strings.Contains(item(t, res, "T-025").Evidence, "no claim") || !strings.Contains(item(t, res, "T-023").Evidence, "bob") {
		t.Errorf("evidence %q / %q", item(t, res, "T-025").Evidence, item(t, res, "T-023").Evidence)
	}
	if f.git("rev-parse", receiptRef) != before {
		t.Error("the receipt chain moved with nothing to write")
	}
}

// A task with two claim records takes the live claim, else the newest.
func TestBackfillPickClaim(t *testing.T) {
	old := comb.ClaimRecord{Lease: comb.Lease{TicketID: "T-023", KeeperID: "a"}, OID: "1", Source: comb.ClaimFromAudit}
	newer := old
	newer.Lease.KeeperID, newer.Lease.ClaimedAt, newer.OID = "b", old.Lease.ClaimedAt.Add(1), "2"
	live := comb.ClaimRecord{Lease: comb.Lease{TicketID: "T-023", KeeperID: "c"}, OID: "3", Source: comb.ClaimFromLive}
	other := comb.ClaimRecord{Lease: comb.Lease{TicketID: "T-024", KeeperID: "d"}, OID: "4", Source: comb.ClaimFromLive}
	for _, c := range []struct {
		name string
		recs []comb.ClaimRecord
		want string
	}{
		{"newest history", []comb.ClaimRecord{old, newer, other}, "b"},
		{"newest history in any order", []comb.ClaimRecord{newer, old}, "b"},
		{"live wins", []comb.ClaimRecord{old, live, newer}, "c"},
		{"live first", []comb.ClaimRecord{live, newer}, "c"},
	} {
		got, ok := pickClaim(c.recs, "T-023")
		if !ok || got.Lease.KeeperID != c.want {
			t.Errorf("%s: %+v %v", c.name, got, ok)
		}
	}
	if _, ok := pickClaim([]comb.ClaimRecord{other}, "T-023"); ok {
		t.Error("another task's claim picked")
	}
}

// A forge that reports the PR fills the PR fields; one that fails leaves the
// receipt incomplete with one warning, and the run still works.
func TestBackfillForge(t *testing.T) {
	f := newFixture(t, false, false)
	claimFor(t, f, "bob", "t-023")
	squash := squashOnMain(f, "internal/x/s.go", "T-023: squash")
	fg := &fakeForge{found: true, pr: MergedPR{Number: 9, Author: "bob", HeadBranch: "t-023", MergeSHA: squash, Approvers: []string{"alice"}}}
	backfill(t, f, apply, func(r *BackfillRequest) { r.Forge = fg })
	r, _ := lastReceipt(t, f)
	if r.MergeProof != "forge" || r.Incomplete || r.PRNumber == nil || *r.PRNumber != 9 || len(r.Approvers) != 1 || r.Source != "backfill" {
		t.Errorf("receipt = %+v", r)
	}

	g := newFixture(t, false, false)
	claimFor(t, g, "bob", "t-023")
	squashOnMain(g, "internal/x/s.go", "T-023: squash")
	res := backfill(t, g, apply, func(r *BackfillRequest) { r.Forge = &fakeForge{err: errors.New("HTTP 500")} })
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "incomplete") {
		t.Errorf("warnings %v", res.Warnings)
	}
	if r, _ = lastReceipt(t, g); r.MergeProof != "binding" || !r.Incomplete {
		t.Errorf("receipt = %+v", r)
	}
}

// A forge answer the receipt rules refuse is dropped, not fatal.
func TestBackfillBadForgeAnswerDropped(t *testing.T) {
	f := newFixture(t, false, false)
	claimFor(t, f, "bob", "t-023")
	squash := squashOnMain(f, "internal/x/s.go", "T-023: squash")
	fg := &fakeForge{found: true, pr: MergedPR{Number: 9, Author: "bad author!", HeadBranch: "t-023", MergeSHA: squash}}
	backfill(t, f, apply, func(r *BackfillRequest) { r.Forge = fg })
	r, _ := lastReceipt(t, f)
	if r.PRNumber != nil || !r.Incomplete || str(r.MergedSHA) != squash {
		t.Errorf("receipt = %+v", r)
	}
}

// Remote mode: the receipts are pushed to the remote in the apply.
func TestBackfillRemote(t *testing.T) {
	f := newFixture(t, true, false)
	claimFor(t, f, "bob", "t-023")
	squashOnMain(f, "internal/x/s.go", "T-023: squash")
	f.git("push", "-q", "origin", "main")
	res := backfill(t, f)
	if item(t, res, "T-023").State != BackfillPlanned || f.git("ls-remote", "origin", receiptRef) != "" {
		t.Fatalf("plan %+v", res)
	}
	backfill(t, f, apply)
	if got := f.git("ls-remote", "origin", receiptRef); !strings.HasPrefix(got, f.git("rev-parse", receiptRef)) {
		t.Errorf("remote receipt head %q vs local %s", got, f.git("rev-parse", receiptRef))
	}
	if res = backfill(t, f, apply); len(res.Items) != 0 {
		t.Errorf("second apply: %+v", res)
	}
}

// Apply needs a keeper id to record as released_by, from .waggle/local.yaml.
func TestBackfillApplyNeedsKeeper(t *testing.T) {
	f := newFixture(t, false, false)
	claimFor(t, f, "bob", "t-023")
	squashOnMain(f, "internal/x/s.go", "T-023: squash")
	if err := os.Remove(filepath.Join(f.real, ".waggle/control/local.yaml")); err != nil {
		t.Fatal(err)
	}
	backfill(t, f) // a dry run does not need it
	if _, err := Backfill(context.Background(), BackfillRequest{RepoDir: f.repo, Apply: true}); err == nil || ExitCode(err) != ExitUsage {
		t.Errorf("apply without a keeper id: %v", err)
	}
	if hasReceipts(f) {
		t.Error("a receipt was written")
	}
	backfill(t, f, apply, func(r *BackfillRequest) { r.ReleasedBy = "dave" })
	if r, _ := lastReceipt(t, f); r.ReleasedBy != "dave" {
		t.Errorf("released_by %q", r.ReleasedBy)
	}
}
