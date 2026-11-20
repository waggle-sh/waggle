// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

var (
	sha1a = strings.Repeat("a", 40)
	sha1b = strings.Repeat("b", 40)
)

// mergedRec is a valid release receipt; the claim fields are filled by the backend.
func mergedRec(releasedBy string) Receipt {
	at := t0.Add(time.Hour)
	n, author, head := 234, "alice", "t-050-nectar"
	return Receipt{
		Version: ReceiptVersion, Source: ReceiptSourceRelease, CoKeepers: []string{}, Approvers: []string{"bob"},
		BaseSHA: &sha1a, MergedSHA: &sha1b, MergedAt: &at, PRNumber: &n, PRAuthor: &author, HeadBranch: &head,
		ReleasedBy: releasedBy, Status: ReceiptMerged, MergeProof: ProofForge,
	}
}

func unmergedRec(releasedBy string) Receipt {
	return Receipt{
		Version: ReceiptVersion, Source: ReceiptSourceRelease, CoKeepers: []string{}, Approvers: []string{},
		ReleasedBy: releasedBy, Status: ReceiptUnmerged, MergeProof: ProofNone, Incomplete: true,
	}
}

func readReceipt(t *testing.T, dir, rev string) Receipt {
	t.Helper()
	var r Receipt
	if err := json.Unmarshal([]byte(runGit(t, dir, "", nil, "cat-file", "blob", rev+":receipt.json")), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// receiptChain returns the first-parent receipt commits, oldest first, after
// checking the chain starts at an empty root commit.
func receiptChain(t *testing.T, dir string) []string {
	t.Helper()
	if !hasRef(t, dir, ReceiptRef) {
		return nil
	}
	all := strings.Fields(runGit(t, dir, "", nil, "rev-list", "--first-parent", "--reverse", ReceiptRef))
	if ps := parents(t, dir, all[0]); len(ps) != 0 || runGit(t, dir, "", nil, "ls-tree", all[0]) != "" {
		t.Fatalf("receipt root %s: parents %v", all[0], ps)
	}
	return all[1:]
}

// A9: validation.
func TestReceiptValidate(t *testing.T) {
	str := func(s string) *string { return &s }
	cases := []struct {
		name string
		edit func(*Receipt)
	}{
		{"keeper_id", func(r *Receipt) { r.KeeperID = "-bad" }},
		{"released_by", func(r *Receipt) { r.ReleasedBy = "has space" }},
		{"approver login", func(r *Receipt) { r.Approvers = []string{"bad_login"} }},
		{"over 32 approvers", func(r *Receipt) {
			r.Approvers = make([]string, 33)
			for i := range r.Approvers {
				r.Approvers[i] = "bob"
			}
		}},
		{"short sha", func(r *Receipt) { r.MergedSHA = str("abc123") }},
		{"upper-case sha", func(r *Receipt) { r.BaseSHA = str(strings.ToUpper(sha1a)) }},
		{"five co-keepers", func(r *Receipt) { r.CoKeepers = []string{"a", "b", "c", "d", "e"} }},
		{"duplicate co-keeper", func(r *Receipt) { r.CoKeepers = []string{"a", "a"} }},
		{"owner as co-keeper", func(r *Receipt) { r.CoKeepers = []string{"alice"} }},
		{"bad head branch", func(r *Receipt) { r.HeadBranch = str("a..b") }},
		{"pr fields not together", func(r *Receipt) { r.PRAuthor = nil }},
		{"approvers without a pr", func(r *Receipt) { r.PRNumber, r.PRAuthor, r.HeadBranch = nil, nil, nil }},
		{"unmerged with a sha", func(r *Receipt) { r.Status, r.MergeProof = ReceiptUnmerged, ProofNone }},
		{"merged without proof", func(r *Receipt) { r.MergeProof = ProofNone }},
		{"nil approvers", func(r *Receipt) { r.Approvers = nil }},
		{"version", func(r *Receipt) { r.Version = 2 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := mergedRec("alice")
			r.TicketID, r.KeeperID, r.Role, r.Gate, r.Branches = "T-050", "alice", "builder", "full", []string{"t-050-nectar"}
			if err := r.Validate(40); err != nil {
				t.Fatalf("base receipt: %v", err)
			}
			c.edit(&r)
			if err := r.Validate(40); !errors.Is(err, ErrInvalidReceipt) {
				t.Errorf("got %v; want ErrInvalidReceipt", err)
			}
		})
	}
}

func TestReleaseWithReceiptLocal(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	b := newBackend(t, dir, "", clock)
	ctx := context.Background()
	l := mkLease("T-050", "alice", "pkg")
	l.Branches = []string{"t-050-nectar"}
	if err := b.AcquireLease(ctx, l); err != nil {
		t.Fatal(err)
	}
	claimOID := refOID(t, dir, "refs/waggle/claims/ticket-50")
	clock.Advance(time.Hour)
	oid, err := b.ReleaseWithReceipt(ctx, "T-050", "alice", mergedRec("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if claimRefs(t, dir) != "" {
		t.Errorf("claim ref left: %s", claimRefs(t, dir))
	}
	chain := receiptChain(t, dir)
	if len(chain) != 1 || chain[0] != oid {
		t.Fatalf("chain = %v; want [%s]", chain, oid)
	}
	if ps := parents(t, dir, oid); len(ps) != 2 || ps[1] != claimOID {
		t.Errorf("parents = %v; second must be the claim %s", ps, claimOID)
	}
	r := readReceipt(t, dir, oid)
	if r.TicketID != "T-050" || r.KeeperID != "alice" || r.Role != "builder" || r.Gate != "full" || r.ClaimedAt == nil || !r.ClaimedAt.Equal(t0) ||
		len(r.Branches) != 1 || r.Branches[0] != "t-050-nectar" || *r.MergedSHA != sha1b || r.Source != "release" {
		t.Errorf("receipt = %+v", r)
	}
	// An unmerged receipt writes JSON null for the unknown fields and [] for the lists.
	if err := b.AcquireLease(ctx, mkLease("T-051", "alice", "pkg2")); err != nil {
		t.Fatal(err)
	}
	oid2, err := b.ReleaseWithReceipt(ctx, "T-051", "alice", unmergedRec("alice"))
	if err != nil {
		t.Fatal(err)
	}
	raw := runGit(t, dir, "", nil, "cat-file", "blob", oid2+":receipt.json")
	for _, want := range []string{`"merged_sha": null`, `"base_sha": null`, `"merged_at": null`, `"pr_number": null`, `"approvers": []`, `"co_keepers": []`, `"branches": []`} {
		if !strings.Contains(raw, want) {
			t.Errorf("receipt json lacks %s:\n%s", want, raw)
		}
	}
	if ps := parents(t, dir, oid2); len(ps) != 2 || ps[0] != oid {
		t.Errorf("second receipt parents = %v; first must be %s", ps, oid)
	}
	got, err := b.ReadReceipts(ctx)
	if err != nil || len(got) != 2 || got[0].OID != oid || got[1].OID != oid2 || got[0].ClaimOID != claimOID || got[1].Receipt.Status != ReceiptUnmerged {
		t.Errorf("ReadReceipts = %+v, %v", got, err)
	}
}

// A5: refusals leave both refs alone.
func TestReleaseWithReceiptRefusals(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	b := newBackend(t, dir, "", newClock())
	ctx := context.Background()
	if _, err := b.ReleaseWithReceipt(ctx, "T-060", "alice", mergedRec("alice")); !errors.Is(err, ErrNotFound) {
		t.Errorf("no claim: %v", err)
	}
	if err := b.AcquireLease(ctx, mkLease("T-060", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReleaseWithReceipt(ctx, "T-060", "mallory", mergedRec("mallory")); !errors.Is(err, ErrNotOwner) {
		t.Errorf("not owner: %v", err)
	}
	bad := mergedRec("alice")
	bad.Approvers = []string{"bad_login"}
	if _, err := b.ReleaseWithReceipt(ctx, "T-060", "alice", bad); !errors.Is(err, ErrInvalidReceipt) {
		t.Errorf("invalid receipt: %v", err)
	}
	if claimRefs(t, dir) == "" || hasRef(t, dir, ReceiptRef) {
		t.Errorf("a refusal moved a ref: claims=%q receipts=%v", claimRefs(t, dir), hasRef(t, dir, ReceiptRef))
	}
}

// A4, local: a claim renewed between read and write loses the CAS; neither ref moves.
func TestReleaseWithReceiptLocalAtomic(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	a := newBackend(t, dir, "", clock)
	ctx := context.Background()
	if err := a.AcquireLease(ctx, mkLease("T-008", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	renewer := newBackend(t, dir, "", clock)
	a.hookBeforeWrite = func() {
		if err := renewer.AcquireLease(ctx, mkLease("T-008", "alice", "pkg")); err != nil {
			panic(err)
		}
	}
	before := refOID(t, dir, "refs/waggle/claims/ticket-8")
	if _, err := a.ReleaseWithReceipt(ctx, "T-008", "alice", mergedRec("alice")); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("got %v; want ErrConcurrentUpdate", err)
	}
	if claimRefs(t, dir) == "" || hasRef(t, dir, ReceiptRef) {
		t.Errorf("a lost CAS moved a ref: claims=%q receipts=%v", claimRefs(t, dir), hasRef(t, dir, ReceiptRef))
	}
	if refOID(t, dir, "refs/waggle/claims/ticket-8") == before {
		t.Error("the renewal did not land; the test raced nothing")
	}
}

func TestReleaseWithReceiptRemote(t *testing.T) {
	requireGit(t)
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ctx := context.Background()
	ba, bb := newBackend(t, dirA, "origin", clock), newBackend(t, dirB, "origin", clock)
	for _, id := range []string{"T-070", "T-071"} {
		if err := ba.AcquireLease(ctx, mkLease(id, "alice", "d/"+id)); err != nil {
			t.Fatal(err)
		}
	}
	claim70 := refOID(t, origin, "refs/waggle/claims/ticket-70")
	oid, err := ba.ReleaseWithReceipt(ctx, "T-070", "alice", mergedRec("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if hasRef(t, origin, "refs/waggle/claims/ticket-70") || !hasRef(t, origin, "refs/waggle/claims/ticket-71") {
		t.Error("origin claims wrong after release")
	}
	if c := receiptChain(t, origin); len(c) != 1 || c[0] != oid {
		t.Fatalf("origin chain = %v", c)
	}
	if ps := parents(t, origin, oid); len(ps) != 2 || ps[1] != claim70 {
		t.Errorf("parents = %v", ps)
	}
	if !hasRef(t, dirA, ReceiptRef) || hasRef(t, dirA, "refs/waggle/claims/ticket-70") {
		t.Error("local mirror not updated")
	}
	// B has never seen the receipts: its release fetches them and chains on the head.
	oid2, err := bb.ReleaseWithReceipt(ctx, "T-071", "alice", unmergedRec("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if c := receiptChain(t, origin); len(c) != 2 || c[0] != oid || c[1] != oid2 {
		t.Errorf("origin chain = %v; want [%s %s]", c, oid, oid2)
	}
}

// A4, remote: a receipt appended by another clone between read and push is
// retried; both receipts land in order and the claim is released once.
func TestReleaseWithReceiptRemoteRetry(t *testing.T) {
	requireGit(t)
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ctx := context.Background()
	ba, bb := newBackend(t, dirA, "origin", clock), newBackend(t, dirB, "origin", clock)
	if err := ba.AcquireLease(ctx, mkLease("T-080", "alice", "d/80")); err != nil {
		t.Fatal(err)
	}
	if err := bb.AcquireLease(ctx, mkLease("T-081", "bob", "d/81")); err != nil {
		t.Fatal(err)
	}
	fired := false
	ba.hookBeforeWrite = func() {
		if fired {
			return
		}
		fired = true
		if _, err := bb.ReleaseWithReceipt(ctx, "T-081", "bob", unmergedRec("bob")); err != nil {
			panic(err)
		}
	}
	oid, err := ba.ReleaseWithReceipt(ctx, "T-080", "alice", mergedRec("alice"))
	if err != nil {
		t.Fatal(err)
	}
	c := receiptChain(t, origin)
	if len(c) != 2 || c[1] != oid || readReceipt(t, origin, c[0]).TicketID != "T-081" {
		t.Errorf("origin chain = %v (second %s)", c, oid)
	}
	if claimRefs(t, origin) != "" {
		t.Errorf("origin claims left: %s", claimRefs(t, origin))
	}
}

// A4, remote: a lost claim race writes nothing.
func TestReleaseWithReceiptRemoteClaimLost(t *testing.T) {
	requireGit(t)
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ctx := context.Background()
	ba, bb := newBackend(t, dirA, "origin", clock), newBackend(t, dirB, "origin", clock)
	if err := ba.AcquireLease(ctx, mkLease("T-090", "alice", "d/90")); err != nil {
		t.Fatal(err)
	}
	ba.hookBeforeWrite = func() {
		if err := bb.AcquireLease(ctx, mkLease("T-090", "alice", "d/90")); err != nil {
			panic(err)
		}
	}
	if _, err := ba.ReleaseWithReceipt(ctx, "T-090", "alice", mergedRec("alice")); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("got %v; want ErrConcurrentUpdate", err)
	}
	if !hasRef(t, origin, "refs/waggle/claims/ticket-90") || hasRef(t, origin, ReceiptRef) {
		t.Error("a lost claim race moved a ref")
	}
}

// A7: a rewound remote receipt head fails the fetch and changes no local ref.
func TestReceiptsRewound(t *testing.T) {
	requireGit(t)
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ctx := context.Background()
	ba, bb := newBackend(t, dirA, "origin", clock), newBackend(t, dirB, "origin", clock)
	for _, id := range []string{"T-100", "T-101"} {
		if err := ba.AcquireLease(ctx, mkLease(id, "alice", "d/"+id)); err != nil {
			t.Fatal(err)
		}
		if _, err := ba.ReleaseWithReceipt(ctx, id, "alice", unmergedRec("alice")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := bb.ReadReceipts(ctx); err != nil {
		t.Fatal(err)
	}
	local := refOID(t, dirB, ReceiptRef)
	// Rewind the remote chain to its first receipt, then add an unrelated one.
	first := receiptChain(t, origin)[0]
	runGit(t, origin, "", nil, "update-ref", ReceiptRef, first)
	runGit(t, origin, "", nil, "update-ref", ReceiptRef, runGit(t, origin, "", []string{"GIT_AUTHOR_NAME=x", "GIT_AUTHOR_EMAIL=x@x", "GIT_COMMITTER_NAME=x", "GIT_COMMITTER_EMAIL=x@x"}, "commit-tree", "-m", "other", first+"^{tree}"))
	if _, err := bb.ReadReceipts(ctx); !errors.Is(err, ErrReceiptsRewound) {
		t.Fatalf("got %v; want ErrReceiptsRewound", err)
	}
	if got := refOID(t, dirB, ReceiptRef); got != local {
		t.Errorf("local receipt head moved to %s from %s", got, local)
	}
	if err := bb.AcquireLease(ctx, mkLease("T-102", "bob", "d/102")); err != nil {
		t.Fatal(err)
	}
	if _, err := bb.ReleaseWithReceipt(ctx, "T-102", "bob", unmergedRec("bob")); !errors.Is(err, ErrReceiptsRewound) {
		t.Errorf("release over a rewound chain: %v", err)
	}
	if !hasRef(t, dirB, "refs/waggle/claims/ticket-102") {
		t.Error("release over a rewound chain deleted the claim")
	}
}
