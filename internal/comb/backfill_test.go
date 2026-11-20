// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"context"
	"errors"
	"testing"
)

var ident = []string{"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com"}

func backfillRec(ticket, keeper string) Receipt {
	r := unmergedRec("alice")
	r.Source, r.TicketID, r.KeeperID, r.Role, r.Gate = ReceiptSourceBackfill, ticket, keeper, "", ""
	r.Branches = []string{}
	return r
}

// AppendReceipts writes the receipts oldest first in one update, each with its
// claim as second parent, and leaves earlier receipts in place.
func TestAppendReceiptsLocal(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	b := newBackend(t, dir, "", newClock())
	ctx := context.Background()
	if err := b.AcquireLease(ctx, mkLease("T-008", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	claim := refOID(t, dir, "refs/waggle/claims/ticket-8")
	got, err := b.AppendReceipts(ctx, []ReceiptAppend{
		{Receipt: backfillRec("T-008", "alice"), ClaimOID: claim},
		{Receipt: backfillRec("T-009", "bob")},
		{Receipt: backfillRec("T-008", "carol")}, // the same ticket twice: the first stays
	})
	if err != nil || len(got) != 2 || got[0] != "T-008" || got[1] != "T-009" {
		t.Fatalf("got %v, %v", got, err)
	}
	chain := receiptChain(t, dir)
	if len(chain) != 2 {
		t.Fatalf("chain %v", chain)
	}
	if ps := parents(t, dir, chain[0]); len(ps) != 2 || ps[1] != claim {
		t.Errorf("first receipt parents %v; second must be the claim", ps)
	}
	if ps := parents(t, dir, chain[1]); len(ps) != 1 || ps[0] != chain[0] {
		t.Errorf("second receipt parents %v", ps)
	}
	if r := readReceipt(t, dir, chain[0]); r.KeeperID != "alice" || r.Source != ReceiptSourceBackfill {
		t.Errorf("receipt %+v", r)
	}
	// Nothing new to write: the ref does not move.
	head := refOID(t, dir, ReceiptRef)
	if got, err = b.AppendReceipts(ctx, []ReceiptAppend{{Receipt: backfillRec("T-009", "dave")}}); err != nil || got != nil || refOID(t, dir, ReceiptRef) != head {
		t.Errorf("repeat: %v, %v", got, err)
	}
	// An invalid receipt writes nothing.
	bad := backfillRec("T-010", "-bad")
	if _, err = b.AppendReceipts(ctx, []ReceiptAppend{{Receipt: backfillRec("T-011", "bob")}, {Receipt: bad}}); !errors.Is(err, ErrInvalidReceipt) || refOID(t, dir, ReceiptRef) != head {
		t.Errorf("invalid: %v", err)
	}
}

// A release that lands between the plan and the write wins: the retry sees its
// ticket has a receipt and appends only the others, after it.
func TestAppendReceiptsRetryKeepsConcurrentRelease(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	a := newBackend(t, dir, "", clock)
	ctx := context.Background()
	if err := a.AcquireLease(ctx, mkLease("T-008", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	racer := newBackend(t, dir, "", clock)
	fired := false
	a.hookBeforeWrite = func() {
		if fired {
			return
		}
		fired = true
		if _, err := racer.ReleaseWithReceipt(ctx, "T-008", "alice", unmergedRec("alice")); err != nil {
			panic(err)
		}
	}
	got, err := a.AppendReceipts(ctx, []ReceiptAppend{{Receipt: backfillRec("T-008", "bob")}, {Receipt: backfillRec("T-009", "bob")}})
	if err != nil || len(got) != 1 || got[0] != "T-009" {
		t.Fatalf("got %v, %v", got, err)
	}
	chain := receiptChain(t, dir)
	if len(chain) != 2 || readReceipt(t, dir, chain[0]).Source != ReceiptSourceRelease || readReceipt(t, dir, chain[1]).TicketID != "T-009" {
		t.Errorf("chain %v", chain)
	}
}

// ReadClaimRecords lists live claims, then claims held as second parents of the
// audit chain and the receipt chain, each commit once.
func TestReadClaimRecords(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	b := newBackend(t, dir, "", newClock())
	ctx := context.Background()
	for _, id := range []string{"T-008", "T-009", "T-010"} {
		if err := b.AcquireLease(ctx, mkLease(id, "alice", "pkg"+id)); err != nil {
			t.Fatal(err)
		}
	}
	c9 := refOID(t, dir, "refs/waggle/claims/ticket-9")
	if _, err := b.ReleaseWithReceipt(ctx, "T-008", "alice", unmergedRec("alice")); err != nil {
		t.Fatal(err)
	}
	empty := runGit(t, dir, "", nil, "mktree")
	root := runGit(t, dir, "", ident, "commit-tree", empty, "-m", "root")
	audit := runGit(t, dir, "", ident, "commit-tree", empty, "-p", root, "-p", c9, "-m", "force release")
	runGit(t, dir, "", nil, "update-ref", ForceReleaseAuditRef, audit)
	// A garbage second parent is skipped.
	junk := runGit(t, dir, "", ident, "commit-tree", empty, "-m", "no lease")
	runGit(t, dir, "", nil, "update-ref", ReceiptRef, runGit(t, dir, "", ident, "commit-tree", empty, "-p", refOID(t, dir, ReceiptRef), "-p", junk, "-m", "x"))

	recs, err := b.ReadClaimRecords(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var live, hist []string
	for _, r := range recs {
		if r.Source == ClaimFromLive {
			live = append(live, r.Lease.TicketID)
		} else {
			hist = append(hist, r.Lease.TicketID+"/"+r.Source)
		}
	}
	// T-009 is live and in the audit chain: listed once, as live. T-008 is in the receipt chain.
	if len(live) != 2 || !(live[0] == "T-009" && live[1] == "T-010" || live[0] == "T-010" && live[1] == "T-009") || len(hist) != 1 || hist[0] != "T-008/"+ClaimFromReceipt {
		t.Errorf("live %v history %v", live, hist)
	}
}
