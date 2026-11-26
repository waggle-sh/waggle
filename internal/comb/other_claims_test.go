// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// otherFixture writes ticket-5 through AcquireLease and raw refs for the rest.
type otherFixture struct {
	t     *testing.T
	dir   string
	b     *GitBackend
	clock *fakeClock
	idEnv []string
}

func newOtherFixture(t *testing.T) *otherFixture {
	t.Helper()
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	return &otherFixture{t: t, dir: dir, b: newBackend(t, dir, "", clock), clock: clock,
		idEnv: []string{"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}}
}

// commitWith returns a commit whose tree holds one file.
func (f *otherFixture) commitWith(name, content string) string {
	f.t.Helper()
	blob := runGit(f.t, f.dir, content, nil, "hash-object", "-w", "--stdin")
	tree := runGit(f.t, f.dir, fmt.Sprintf("100644 blob %s\t%s\n", blob, name), nil, "mktree")
	return runGit(f.t, f.dir, "", f.idEnv, "commit-tree", tree, "-m", "test")
}

func (f *otherFixture) set(ref, oid string) {
	f.t.Helper()
	runGit(f.t, f.dir, "", nil, "update-ref", ClaimRefPrefix+ref, oid)
}

func record(holder, lead, expires string) string {
	return fmt.Sprintf(`{"v":1,"holder":%q,"lead":%q,"expires_at":%q,"paths":["shared"]}`, holder, lead, expires)
}

func TestOtherClaimsSkippedByScan(t *testing.T) {
	f := newOtherFixture(t)
	ctx := context.Background()
	exp := f.clock.Now().Add(time.Hour).Format(time.RFC3339)
	if err := f.b.AcquireLease(ctx, mkLease("T-005", "alice", "shared")); err != nil {
		t.Fatal(err)
	}
	f.set("bl-back-260", f.commitWith("claim.json", record("bob", "expo-queen", exp)))
	f.set("gh-123", f.commitWith("claim.json", record("carol", "lead", exp)))
	f.set("ticket-007", f.commitWith("claim.json", record("x", "y", exp)))
	f.set("Ticket-5", f.commitWith("claim.json", record("x", "y", exp)))
	f.set("a/b", f.commitWith("claim.json", record("x", "y", exp)))

	wantBad := map[string]string{
		ClaimRefPrefix + "ticket-007": "ref name is not canonical ticket-<n>",
		ClaimRefPrefix + "Ticket-5":   "ref name is not canonical ticket-<n>",
		ClaimRefPrefix + "a/b":        "ref name is not canonical ticket-<n>",
	}
	check := func(name string, leases []Lease, err error) {
		t.Helper()
		var mce *MalformedClaimsError
		if !errors.As(err, &mce) {
			t.Fatalf("%s: err = %v; want *MalformedClaimsError", name, err)
		}
		got := map[string]string{}
		for _, c := range mce.Claims {
			got[c.Ref] = c.Reason
		}
		if fmt.Sprint(got) != fmt.Sprint(wantBad) {
			t.Errorf("%s: malformed = %v; want %v", name, got, wantBad)
		}
		if len(leases) != 1 || leases[0].TicketID != "T-005" {
			t.Errorf("%s: leases = %+v; want only T-005", name, leases)
		}
	}
	leases, err := f.b.ListActiveLeases(ctx)
	check("ListActiveLeases", leases, err)
	leases, leasesErr, collisions, err := f.b.ClaimsView(ctx)
	if err != nil || len(collisions) != 0 {
		t.Fatalf("ClaimsView: collisions %v, err %v", collisions, err)
	}
	check("ClaimsView", leases, leasesErr)

	// A new-kind claim on the same paths does not block a ticket claim.
	if err := f.b.AcquireLease(ctx, mkLease("T-006", "dave", "shared/sub")); err == nil || !errors.Is(err, ErrLeaseOverlap) {
		t.Fatalf("overlap with ticket-5 must still be refused, got %v", err)
	}
	if err := f.b.AcquireLease(ctx, mkLease("T-008", "dave", "elsewhere")); err != nil {
		t.Fatal(err)
	}
	f.set("gh-9", f.commitWith("claim.json", record("bob", "l", exp)))
	if err := f.b.AcquireLease(ctx, mkLease("T-009", "dave", "elsewhere/x")); !errors.Is(err, ErrLeaseOverlap) {
		t.Fatalf("got %v", err)
	}
	f.set("gh-10", f.commitWith("claim.json", `{"paths":["only-here"]}`))
	if err := f.b.AcquireLease(ctx, mkLease("T-010", "dave", "only-here")); err != nil {
		t.Fatalf("AcquireLease over a new-kind claim with overlapping paths: %v", err)
	}
}

func TestOtherClaimKeyRules(t *testing.T) {
	long := strings.Repeat("a", 100)
	for key, want := range map[string]bool{
		"bl-back-260": true, "gh-123": true, "bl-task-4.1": true, "h-0123456789abcdef0123": true, "a_b": true,
		long: true, long + "a": false,
		"": false, "Back-1": false, "a..b": false, "a.": false, "a.lock": false, "a/b": false, "a b": false,
		"ticket-1": false, "ticket-007": false, "Ticket-5": false, "TICKET-5": false, "ticket-": false,
		"ticketing": true, "t": true,
	} {
		_, got := otherClaimKey(ClaimRefPrefix + key)
		if got != want {
			t.Errorf("otherClaimKey(%q) = %v; want %v", key, got, want)
		}
	}
	if _, ok := otherClaimKey("refs/waggle/other/gh-1"); ok {
		t.Error("a ref outside the claim namespace is not an other claim")
	}
}

func TestOtherClaimsRecordRead(t *testing.T) {
	f := newOtherFixture(t)
	ctx := context.Background()
	exp := t0.Add(time.Hour)
	f.set("gh-2", f.commitWith("claim.json", record("bob", "expo-queen", exp.Format(time.RFC3339))))
	f.set("bl-back-1", f.commitWith("claim.json", record("carol", "q", exp.Add(-3*time.Hour).Format(time.RFC3339))))
	f.set("gh-3", f.commitWith("lease.json", "{}")) // no claim.json
	// A valid record padded past the cap: only the size check makes it unreadable.
	f.set("gh-4", f.commitWith("claim.json", record("bob", "l", exp.Format(time.RFC3339))+strings.Repeat(" ", MaxLeaseBytes)))
	f.set("gh-5", f.commitWith("claim.json", "{not json"))
	f.set("gh-6", f.commitWith("claim.json", `{"holder":5,"lead":"l","expires_at":"2026-10-17T15:00:00Z"}`))
	f.set("gh-7", f.commitWith("claim.json", `{"holder":"h","lead":"l","expires_at":"soon"}`))
	f.set("gh-8", runGit(t, f.dir, "x", nil, "hash-object", "-w", "--stdin")) // a blob, not a commit
	f.set("ticket-7", f.commitWith("claim.json", record("x", "y", "2026-10-17T15:00:00Z")))

	got, err := f.b.OtherClaims(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, c := range got {
		keys = append(keys, c.Key)
	}
	if want := "bl-back-1 gh-2 gh-3 gh-4 gh-5 gh-6 gh-7 gh-8"; strings.Join(keys, " ") != want {
		t.Fatalf("keys = %v; want %s", keys, want)
	}
	want := map[string]OtherClaim{
		"gh-2":      {Key: "gh-2", Holder: "bob", Lead: "expo-queen", ExpiresAt: exp, Readable: true},
		"bl-back-1": {Key: "bl-back-1", Holder: "carol", Lead: "q", ExpiresAt: exp.Add(-3 * time.Hour), Readable: true},
	}
	for _, c := range got {
		if w, ok := want[c.Key]; ok {
			if c != w {
				t.Errorf("%s = %+v; want %+v", c.Key, c, w)
			}
		} else if c.Readable || c.Holder != "" || c.Lead != "" || !c.ExpiresAt.IsZero() {
			t.Errorf("%s = %+v; want unreadable and blank", c.Key, c)
		}
	}
}

func TestOtherClaimsEmpty(t *testing.T) {
	f := newOtherFixture(t)
	if err := f.b.AcquireLease(context.Background(), mkLease("T-005", "alice", "x")); err != nil {
		t.Fatal(err)
	}
	got, err := f.b.OtherClaims(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("OtherClaims = %v, %v; want none", got, err)
	}
}
