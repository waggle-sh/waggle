// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

const claimRef23 = "refs/waggle/claims/ticket-23"

// forceFixture is a fixture whose committed config lists approvers (none when
// empty), pushed to the origin in remote mode.
func forceFixture(t *testing.T, remote bool, approvers string) *fixture {
	t.Helper()
	f := newFixture(t, remote, false)
	if approvers != "" {
		setApprovers(f, approvers)
		f.git("commit", "-q", "-am", "approvers")
		if remote {
			f.git("push", "-q", "origin", "main")
		}
	}
	return f
}

// setApprovers writes gate.approvers into the working tree config.
func setApprovers(f *fixture, approvers string) {
	p := filepath.Join(f.real, ".waggle/control/config.yaml")
	data, err := os.ReadFile(p)
	if err != nil {
		f.t.Fatal(err)
	}
	s := string(data)
	if i := strings.Index(s, "\ngate:\n"); i >= 0 {
		s = s[:i+1]
	}
	write(f.t, f.real, ".waggle/control/config.yaml", s+"gate:\n  approvers: "+approvers+"\n")
}

func doForce(f *fixture, mut ...func(*ForceReleaseRequest)) (ForceReleaseResult, error) {
	r := ForceReleaseRequest{RepoDir: f.repo, Ticket: "23", Owner: "alice"}
	for _, m := range mut {
		m(&r)
	}
	return ForceReleaseTask(context.Background(), r)
}

func execute(oid string) func(*ForceReleaseRequest) {
	return func(r *ForceReleaseRequest) { r.OID, r.Reason = oid, "owner unreachable" }
}

func refOID(f *fixture, dir, ref string) string {
	return sh(f.t, dir, "for-each-ref", "--format=%(objectname)", ref)
}

// snapshot is the claim and audit OIDs, to show nothing was written.
func snapshot(f *fixture) string {
	return refOID(f, f.real, comb.ClaimRefPrefix) + "|" + refOID(f, f.real, comb.ForceReleaseAuditRef)
}

// A1: inspect on a valid claim reports it and writes nothing.
func TestForceInspectValid(t *testing.T) {
	f := forceFixture(t, false, "alice")
	releaseClaim(t, f, "carol")
	before := snapshot(f)
	res, err := doForce(f)
	if err != nil || res.Outcome != "inspected" || res.Lease == nil || res.Lease.KeeperID != "carol" || res.Lease.Directories[0] != "internal/x" ||
		res.OID != refOID(f, f.real, claimRef23) || !res.Authorized || res.FirstApprover != "alice" || res.Expired || res.Task != "T-023" {
		t.Fatalf("res %+v err %v", res, err)
	}
	if snapshot(f) != before {
		t.Fatal("inspect wrote a ref")
	}
	// Not authorized: the facts are still there, exit 0.
	if res, err = doForce(f, func(r *ForceReleaseRequest) { r.Owner = "bob" }); err != nil || res.Authorized || res.Lease == nil || res.FirstApprover != "alice" {
		t.Fatalf("other caller: res %+v err %v", res, err)
	}
	// An expired claim is still inspected, and flagged (Q-T072-7).
	later := func() time.Time { return time.Now().Add(30 * 24 * time.Hour) }
	if res, err = doForce(f, func(r *ForceReleaseRequest) { r.Now = later }); err != nil || !res.Expired {
		t.Fatalf("expired: res %+v err %v", res, err)
	}
}

// A2, A3: malformed targets inspect with a reason and clear with an audit entry.
func TestForceMalformedAndAudit(t *testing.T) {
	for _, c := range []struct {
		name, ref, reason string
		setup             func(f *fixture) string // returns the target oid
	}{
		{"commit", claimRef23, "tree has no lease.json", func(f *fixture) string { return f.git("rev-parse", "HEAD") }},
		{"blob", claimRef23, "points to a blob", func(f *fixture) string { return f.git("hash-object", "-w", "README") }},
		{"noncanonical", "refs/waggle/claims/ticket-023", "not canonical", func(f *fixture) string { return f.git("rev-parse", "HEAD") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := forceFixture(t, false, "alice")
			target := c.setup(f)
			f.git("update-ref", c.ref, target)
			byRef := func(r *ForceReleaseRequest) { r.Ticket, r.Ref = "", c.ref }
			res, err := doForce(f, byRef)
			if err != nil || res.Outcome != "inspected" || res.Lease != nil || !strings.Contains(res.Malformed, c.reason) || res.OID != target {
				t.Fatalf("inspect: res %+v err %v", res, err)
			}
			res, err = doForce(f, byRef, execute(target))
			if err != nil || res.Outcome != "released" || refOID(f, f.real, c.ref) != "" || res.AuditOID != refOID(f, f.real, comb.ForceReleaseAuditRef) {
				t.Fatalf("execute: res %+v err %v", res, err)
			}
			// The audit root plus one entry; parent 2 is a released commit, a blob is kept in the tree.
			if n := f.git("rev-list", "--count", "--first-parent", comb.ForceReleaseAuditRef); n != "2" {
				t.Fatalf("first-parent walk has %s commits", n)
			}
			parents := strings.Fields(f.git("rev-list", "--parents", "-n", "1", comb.ForceReleaseAuditRef))
			tree := f.git("ls-tree", "--name-only", comb.ForceReleaseAuditRef)
			if c.name == "blob" {
				if len(parents) != 2 || !strings.Contains(tree, "released-claim.raw") {
					t.Fatalf("parents %v tree %q", parents, tree)
				}
			} else if len(parents) != 3 || parents[2] != target {
				t.Fatalf("parents %v, want parent 2 %s", parents, target)
			}
			if rec := f.git("show", comb.ForceReleaseAuditRef+":force-release.json"); !strings.Contains(rec, `"head_keeper_id": "alice"`) || !strings.Contains(rec, "owner unreachable") {
				t.Fatalf("record %s", rec)
			}
		})
	}
}

// A4, A5, A7: only the first approver, case folded; no approvers denies all.
func TestForceAuthorizer(t *testing.T) {
	for _, c := range []struct {
		approvers, caller, msg string
		ok                     bool
	}{
		{"bob,alice", "alice", `"alice" is not bob, the first login in gate.approvers`, false},
		{"Alice,bob", "alice", "", true},
		{"alice", "ALICE", "", true},
		{"", "alice", "no first approver in gate.approvers at", false},
	} {
		f := forceFixture(t, false, c.approvers)
		releaseClaim(t, f, "carol")
		oid, before := refOID(f, f.real, claimRef23), snapshot(f)
		res, err := doForce(f, execute(oid), func(r *ForceReleaseRequest) { r.Owner = c.caller })
		switch {
		case c.ok && (err != nil || res.Outcome != "released"):
			t.Fatalf("%s/%s: res %+v err %v", c.approvers, c.caller, res, err)
		case !c.ok && (code(err) != ExitClaim || !strings.Contains(err.Error(), c.msg) || snapshot(f) != before):
			t.Fatalf("%s/%s: err %v, written %v", c.approvers, c.caller, err, snapshot(f) != before)
		}
	}
}

// A6: the approvers come from the base commit, never the working tree.
func TestForceReadsBase(t *testing.T) {
	f := forceFixture(t, false, "bob")
	releaseClaim(t, f, "carol")
	setApprovers(f, "alice") // uncommitted edit naming the caller
	oid, before := refOID(f, f.real, claimRef23), snapshot(f)
	if _, err := doForce(f, execute(oid)); code(err) != ExitClaim || snapshot(f) != before {
		t.Fatalf("worktree edit: err %v", err)
	}

	f = forceFixture(t, false, "alice")
	releaseClaim(t, f, "carol")
	setApprovers(f, "bob")
	if res, err := doForce(f, execute(refOID(f, f.real, claimRef23))); err != nil || res.Outcome != "released" {
		t.Fatalf("base names the caller: res %+v err %v", res, err)
	}
}

// A8: a claim renewed after inspect is refused, nothing written.
func TestForceStaleOID(t *testing.T) {
	f := forceFixture(t, false, "alice")
	b := releaseClaim(t, f, "carol")
	res, err := doForce(f)
	if err != nil {
		t.Fatal(err)
	}
	if err = b.AcquireLease(context.Background(), comb.Lease{TicketID: "T-023", KeeperID: "carol", Role: "builder", Gate: "full", Directories: []string{"internal/x"}}); err != nil {
		t.Fatal(err)
	}
	before := snapshot(f)
	if _, err = doForce(f, execute(res.OID)); code(err) != ExitClaim || !strings.Contains(err.Error(), "re-inspect: the claim changed since you looked") || snapshot(f) != before {
		t.Fatalf("err %v", err)
	}
}

// A9: usage and invalid requests fail before any write.
func TestForceRefusals(t *testing.T) {
	f := forceFixture(t, false, "alice")
	releaseClaim(t, f, "carol")
	oid, before := refOID(f, f.real, claimRef23), snapshot(f)
	for _, c := range []struct {
		name string
		mut  func(*ForceReleaseRequest)
		exit int
	}{
		{"neither", func(r *ForceReleaseRequest) { r.Ticket = "" }, ExitUsage},
		{"both", func(r *ForceReleaseRequest) { r.Ref = claimRef23 }, ExitUsage},
		{"bad task", func(r *ForceReleaseRequest) { r.Ticket = "x" }, ExitUsage},
		{"short oid", execute(oid[:12]), ExitUsage},
		{"nonhex oid", execute(strings.Repeat("g", 40)), ExitUsage},
		{"HEAD", execute("HEAD"), ExitUsage},
		{"oid no reason", func(r *ForceReleaseRequest) { r.OID = oid }, ExitUsage},
		{"reason no oid", func(r *ForceReleaseRequest) { r.Reason = "why" }, ExitUsage},
		{"control char", func(r *ForceReleaseRequest) { r.OID, r.Reason = oid, "a\x01b" }, ExitClaim},
		{"too long", func(r *ForceReleaseRequest) { r.OID, r.Reason = oid, strings.Repeat("a", 513) }, ExitClaim},
		{"blank", func(r *ForceReleaseRequest) { r.OID, r.Reason = oid, "  " }, ExitClaim},
		{"bad ref", func(r *ForceReleaseRequest) { r.Ticket, r.Ref = "", "refs/heads/main" }, ExitClaim},
	} {
		if _, err := doForce(f, c.mut); code(err) != c.exit || snapshot(f) != before {
			t.Errorf("%s: err %v (exit %d), written %v", c.name, err, code(err), snapshot(f) != before)
		}
	}
}

// A11: the command refuses inside a dispatch worktree.
func TestForceRefusesDispatchWorktree(t *testing.T) {
	f := forceFixture(t, false, "alice")
	releaseClaim(t, f, "carol")
	wt := filepath.Join(f.real, ".waggle/worktrees/T-023-builder")
	f.git("worktree", "add", "-q", "-b", "t-023", wt)
	before := snapshot(f)
	for _, m := range []func(*ForceReleaseRequest){nil, execute(refOID(f, f.real, claimRef23))} {
		r := func(r *ForceReleaseRequest) {
			r.RepoDir = filepath.Join(wt, "internal")
			if m != nil {
				m(r)
			}
		}
		if _, err := doForce(f, r); code(err) != ExitClaim || !strings.Contains(err.Error(), "not a dispatch worktree") || snapshot(f) != before {
			t.Fatalf("err %v", err)
		}
	}
}

// A12: an absent ref is no_claim with no audit entry.
func TestForceNoClaim(t *testing.T) {
	f := forceFixture(t, false, "alice")
	res, err := doForce(f)
	if err != nil || res.Outcome != "no_claim" {
		t.Fatalf("inspect: res %+v err %v", res, err)
	}
	if res, err = doForce(f, execute(strings.Repeat("a", 40))); err != nil || res.Outcome != "no_claim" || snapshot(f) != "|" {
		t.Fatalf("execute: res %+v err %v, refs %q", res, err, snapshot(f))
	}
}

// A15: in remote mode the delete and the audit entry land on the origin together and are mirrored.
func TestForceRemote(t *testing.T) {
	f := forceFixture(t, true, "alice")
	releaseClaim(t, f, "carol")
	oid := refOID(f, f.origin, claimRef23)
	f.git("update-ref", "-d", claimRef23) // the local mirror is stale; inspect syncs
	res, err := doForce(f)
	if err != nil || res.OID != oid || res.Lease == nil {
		t.Fatalf("inspect: res %+v err %v", res, err)
	}
	if res, err = doForce(f, execute(oid)); err != nil || res.Outcome != "released" {
		t.Fatalf("execute: res %+v err %v", res, err)
	}
	audit := refOID(f, f.origin, comb.ForceReleaseAuditRef)
	if refOID(f, f.origin, claimRef23) != "" || audit == "" || audit != refOID(f, f.real, comb.ForceReleaseAuditRef) || audit != res.AuditOID || refOID(f, f.real, claimRef23) != "" {
		t.Fatalf("origin claim %q audit %q local audit %q", refOID(f, f.origin, claimRef23), audit, refOID(f, f.real, comb.ForceReleaseAuditRef))
	}
}
