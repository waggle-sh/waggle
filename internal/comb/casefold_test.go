// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// T-073 test A1: only A to Z fold.
func TestFoldASCII(t *testing.T) {
	cases := map[string]string{
		"ABCxyz":              "abcxyz",
		"internal/Comb-2_x.y": "internal/comb-2_x.y",
		"K":                   "K",    // KELVIN SIGN
		"ſ":                   "ſ",    // LONG S
		"Ａ":                   "Ａ",    // FULLWIDTH A
		"@[`{":                "@[`{", // bytes next to the letter ranges
	}
	for in, want := range cases {
		if got := foldASCII(in); got != want {
			t.Errorf("foldASCII(%q) = %q; want %q", in, got, want)
		}
	}
}

// T-073 tests A2 and A3: the folded overlap keeps every exact answer, adds case
// variants, and DirsOverlap stays exact.
func TestDirsOverlapFold(t *testing.T) {
	cases := []struct {
		a, b        string
		fold, exact bool
	}{
		{"internal/comb", "internal/combx", false, false},
		{"internal/combx", "internal/comb", false, false},
		{"internal", "internal/comb", true, true},
		{"internal/comb", "internal", true, true},
		{"internal/comb", "internal/comb", true, true},
		{"internal/comb/x", "internal/comb", true, true},
		{"cmd", "internal", false, false},
		{"a/b", "a/c", false, false},
		{"internal/comb", "Internal/Comb", true, false},
		{"INTERNAL", "internal/comb", true, false},
		{"internal/Comb", "internal/combx", false, false},
		{"a/B", "a/c", false, false},
	}
	for _, tc := range cases {
		if got := dirsOverlapFold(tc.a, tc.b); got != tc.fold {
			t.Errorf("dirsOverlapFold(%q, %q) = %v; want %v", tc.a, tc.b, got, tc.fold)
		}
		if got := DirsOverlap(tc.a, tc.b); got != tc.exact {
			t.Errorf("DirsOverlap(%q, %q) = %v; want %v", tc.a, tc.b, got, tc.exact)
		}
		if got := caseOnlyOverlap(tc.a, tc.b); got != (tc.fold && !tc.exact) {
			t.Errorf("caseOnlyOverlap(%q, %q) = %v", tc.a, tc.b, got)
		}
	}
}

// caseRig is a claim test setup: claimer writes claims, other (a second clone in
// remote mode, the same backend in local mode) sees them, and store is the repo
// that forged claims go to.
type caseRig struct {
	clock           *fakeClock
	claimer, other  *GitBackend
	store, otherDir string
}

func newCaseRig(t *testing.T, remote bool, ignoreCase string) caseRig {
	t.Helper()
	requireGit(t)
	clock := newClock()
	if !remote {
		dir := newRepo(t, false)
		runGit(t, dir, "", nil, "config", "core.ignorecase", ignoreCase)
		b := newBackend(t, dir, "", clock)
		return caseRig{clock, b, b, dir, dir}
	}
	origin, a, b := remoteSetup(t)
	runGit(t, a, "", nil, "config", "core.ignorecase", ignoreCase)
	runGit(t, b, "", nil, "config", "core.ignorecase", ignoreCase)
	return caseRig{clock, newBackend(t, a, "origin", clock), newBackend(t, b, "origin", clock), origin, b}
}

func caseModes() []struct {
	name       string
	remote     bool
	ignoreCase string
} {
	return []struct {
		name       string
		remote     bool
		ignoreCase string
	}{
		{"local/sensitive", false, "false"},
		{"local/insensitive", false, "true"},
		{"remote/sensitive", true, "false"},
		{"remote/insensitive", true, "true"},
	}
}

const foldedText = "when letter case is ignored (macOS and Windows checkouts treat them as one directory)"

// T-073 tests A4, A8, A9 and A10: case variants of a held directory are refused
// the same way on case-sensitive and case-insensitive settings, locally and
// across clones.
func TestAcquireFoldedOverlap(t *testing.T) {
	for _, m := range caseModes() {
		t.Run(m.name, func(t *testing.T) {
			r := newCaseRig(t, m.remote, m.ignoreCase)
			ctx := context.Background()
			if err := r.claimer.AcquireLease(ctx, mkLease("T-001", "ben", "internal/comb")); err != nil {
				t.Fatal(err)
			}
			for _, d := range []string{"Internal/Comb", "INTERNAL", "internal/COMB/sub", "Internal"} {
				err := r.other.AcquireLease(ctx, mkLease("T-002", "ana", d))
				if !errors.Is(err, ErrLeaseOverlap) || !strings.Contains(err.Error(), foldedText) {
					t.Errorf("T-002 on %s: %v; want folded ErrLeaseOverlap", d, err)
				}
			}
			err := r.other.AcquireLease(ctx, mkLease("T-002", "ana", "Internal/Comb"))
			want := `T-002 directory "Internal/Comb" overlaps T-001 directory "internal/comb" held by ben ` +
				foldedText + ": comb: directories overlap an active lease"
			if err == nil || err.Error() != want {
				t.Errorf("message = %v; want %s", err, want)
			}
			for i := 0; err != nil && i < len(err.Error()); i++ {
				if c := err.Error()[i]; c < 0x20 || c > 0x7e {
					t.Errorf("message byte %#x outside printable ASCII", c)
				}
			}
			// Exact overlaps keep today's message.
			err = r.other.AcquireLease(ctx, mkLease("T-002", "ana", "internal"))
			if !errors.Is(err, ErrLeaseOverlap) || strings.Contains(err.Error(), "letter case") {
				t.Errorf("T-002 on internal: %v; want exact ErrLeaseOverlap", err)
			}
			if err := r.other.AcquireLease(ctx, mkLease("T-002", "ana", "internal/Combx")); err != nil {
				t.Errorf("T-002 on internal/Combx: %v", err)
			}
			if err := r.claimer.ReleaseLease(ctx, "T-001", "ben"); err != nil {
				t.Fatal(err)
			}
			if err := r.other.AcquireLease(ctx, mkLease("T-003", "ana", "Internal/Comb")); err != nil {
				t.Errorf("after release, T-003 on Internal/Comb: %v", err)
			}
		})
	}
}

// objectState is the claim refs and loose object count of a repo.
func objectState(t *testing.T, dir string) string {
	t.Helper()
	return claimRefs(t, dir) + "\n" + runGit(t, dir, "", nil, "count-objects")
}

// T-073 test A5: non-ASCII directories never reach the overlap check.
func TestAcquireUnicodeRejected(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	b := newBackend(t, dir, "", newClock())
	ctx := context.Background()
	before := objectState(t, dir)
	for _, d := range []string{"café", "café", "Kernel", "internal/ｃomb"} {
		if err := b.AcquireLease(ctx, mkLease("T-002", "ana", d)); !errors.Is(err, ErrInvalidLease) {
			t.Errorf("claim on %q: %v; want ErrInvalidLease", d, err)
		}
	}
	if after := objectState(t, dir); after != before {
		t.Errorf("git state changed: %q to %q", before, after)
	}
	forgeClaim(t, dir, "refs/waggle/claims/ticket-1", forgedJSON("T-001", "ben", `"docs", "café"`))
	var mce *MalformedClaimsError
	if leases, err := b.ListActiveLeases(ctx); len(leases) != 0 || !errors.As(err, &mce) {
		t.Fatalf("ListActiveLeases = %+v, %v; want one malformed claim", leases, err)
	}
	if err := b.AcquireLease(ctx, mkLease("T-002", "ana", "DOCS")); err != nil {
		t.Errorf("malformed claim was not skipped: %v", err)
	}
}

// forgedJSON is a stored lease.json from t0 to t0 plus two hours; dirs is the
// inside of the JSON array.
func forgedJSON(ticket, owner, dirs string) string {
	return fmt.Sprintf(`{"version": 2, "ticket_id": %q, "keeper_id": %q, "role": "builder", "gate": "full",`+
		` "directories": [%s], "claimed_at": "2026-10-17T14:00:00Z", "expires_at": "2026-10-17T16:00:00Z"}`,
		ticket, owner, dirs)
}

// T-073 tests A6 and A8: claims that already collide keep renewing as they are,
// never grow, never transfer, and are reported by CaseCollisions.
func TestCaseCollisionGrandfather(t *testing.T) {
	for _, m := range caseModes() {
		t.Run(m.name, func(t *testing.T) {
			r := newCaseRig(t, m.remote, m.ignoreCase)
			ctx := context.Background()
			forgeClaim(t, r.store, "refs/waggle/claims/ticket-1", forgedJSON("T-001", "ben", `"docs"`))
			forgeClaim(t, r.store, "refs/waggle/claims/ticket-2", forgedJSON("T-002", "carl", `"Docs/api"`))

			leases, err := r.other.ListActiveLeases(ctx)
			if err != nil || len(leases) != 2 || leases[0].Directories[0] != "docs" || leases[1].Directories[0] != "Docs/api" {
				t.Fatalf("ListActiveLeases = %+v, %v", leases, err)
			}
			pair := []CaseCollision{{"T-001", "docs", "ben", "T-002", "Docs/api", "carl"}}
			if got, err := r.other.CaseCollisions(ctx); err != nil || !reflect.DeepEqual(got, pair) {
				t.Fatalf("CaseCollisions = %+v, %v; want %+v", got, err, pair)
			}

			renew := func(b *GitBackend, l Lease) {
				t.Helper()
				ref, _ := RefName(l.TicketID)
				old := runGit(t, r.store, "", nil, "rev-parse", ref)
				if err := b.AcquireLease(ctx, l); err != nil {
					t.Fatalf("renew %s %v: %v", l.TicketID, l.Directories, err)
				}
				if ps := parents(t, r.store, runGit(t, r.store, "", nil, "rev-parse", ref)); !reflect.DeepEqual(ps, []string{old}) {
					t.Fatalf("%s parents = %v; want [%s]", l.TicketID, ps, old)
				}
			}
			renew(r.claimer, mkLease("T-001", "ben", "docs"))
			renew(r.other, mkLease("T-002", "carl", "Docs/api"))

			refuse := func(l Lease, folded bool) {
				t.Helper()
				err := r.other.AcquireLease(ctx, l)
				if !errors.Is(err, ErrLeaseOverlap) || strings.Contains(err.Error(), "letter case") != folded {
					t.Errorf("%s %s on %v: %v; want ErrLeaseOverlap, folded %v", l.TicketID, l.KeeperID, l.Directories, err, folded)
				}
			}
			refuse(mkLease("T-002", "carl", "Docs/api", "DOCS/guide"), true)
			refuse(mkLease("T-002", "carl", "docs/api"), false)
			refuse(mkLease("T-003", "ana", "docs/x"), false)
			refuse(mkLease("T-003", "ana", "DOCS"), true)

			// T-001 renews an hour later, so T-002 expires first.
			r.clock.Advance(time.Hour)
			renew(r.claimer, mkLease("T-001", "ben", "docs"))
			r.clock.Advance(time.Hour)
			if got, err := r.other.CaseCollisions(ctx); err != nil || len(got) != 0 {
				t.Errorf("after T-002 expired, CaseCollisions = %+v, %v", got, err)
			}
			refuse(mkLease("T-002", "dana", "Docs/api"), true)
			refuse(mkLease("T-002", "carl", "Docs/api"), true) // a takeover by the old owner is not a renewal
			if err := r.claimer.ReleaseLease(ctx, "T-001", "ben"); err != nil {
				t.Fatal(err)
			}
			if err := r.other.AcquireLease(ctx, mkLease("T-002", "dana", "Docs/api")); err != nil {
				t.Errorf("takeover after release: %v", err)
			}
		})
	}
}

// T-073 test A6: a released claim drops out of CaseCollisions, and pairs come
// back sorted and once each.
func TestCaseCollisionsListing(t *testing.T) {
	r := newCaseRig(t, false, "false")
	ctx := context.Background()
	forgeClaim(t, r.store, "refs/waggle/claims/ticket-10", forgedJSON("T-010", "ben", `"Docs", "pkg"`))
	forgeClaim(t, r.store, "refs/waggle/claims/ticket-2", forgedJSON("T-002", "carl", `"docs/api", "docs/b"`))
	forgeClaim(t, r.store, "refs/waggle/claims/ticket-3", forgedJSON("T-003", "ana", `"PKG/x"`))
	want := []CaseCollision{
		{"T-002", "docs/api", "carl", "T-010", "Docs", "ben"},
		{"T-002", "docs/b", "carl", "T-010", "Docs", "ben"},
		{"T-003", "PKG/x", "ana", "T-010", "pkg", "ben"},
	}
	if got, err := r.claimer.CaseCollisions(ctx); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("CaseCollisions = %+v, %v; want %+v", got, err, want)
	}
	if err := r.claimer.ReleaseLease(ctx, "T-010", "ben"); err != nil {
		t.Fatal(err)
	}
	if got, err := r.claimer.CaseCollisions(ctx); err != nil || len(got) != 0 {
		t.Errorf("after release, CaseCollisions = %+v, %v", got, err)
	}
}

// T-073 test A7: a lease whose own directories differ only in case is refused
// when written, but a stored one is still read and enforced.
func TestLeaseSelfCaseCheck(t *testing.T) {
	r := newCaseRig(t, false, "false")
	ctx := context.Background()
	before := objectState(t, r.store)
	for _, dirs := range [][]string{{"internal/Comb", "internal/comb"}, {"internal", "Internal/comb"}} {
		err := r.claimer.AcquireLease(ctx, mkLease("T-001", "ben", dirs...))
		if !errors.Is(err, ErrInvalidLease) || !strings.Contains(err.Error(), "differ only in letter case") {
			t.Errorf("lease %v: %v; want ErrInvalidLease", dirs, err)
		}
	}
	if after := objectState(t, r.store); after != before {
		t.Errorf("git state changed: %q to %q", before, after)
	}
	if err := r.claimer.AcquireLease(ctx, mkLease("T-001", "ben", "internal", "internal/comb")); err != nil {
		t.Errorf("lease [internal internal/comb]: %v", err)
	}
	forgeClaim(t, r.store, "refs/waggle/claims/ticket-5", forgedJSON("T-005", "carl", `"a", "A"`))
	leases, err := r.claimer.ListActiveLeases(ctx)
	if err != nil || len(leases) != 2 || !reflect.DeepEqual(leases[1].Directories, []string{"A", "a"}) {
		t.Fatalf("ListActiveLeases = %+v, %v; want the stored pair listed", leases, err)
	}
	if err := r.claimer.AcquireLease(ctx, mkLease("T-009", "ana", "a/x")); !errors.Is(err, ErrLeaseOverlap) {
		t.Errorf("T-009 on a/x: %v; want ErrLeaseOverlap", err)
	}
}
