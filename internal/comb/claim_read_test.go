package comb

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// T-014 phase A test 2: ReadClaim returns an expired claim with its
// normalized directories, which ListActiveLeases no longer lists.
func TestReadClaimExpired(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	b := newBackend(t, dir, "", clock)
	ctx := context.Background()
	if err := b.AcquireLease(ctx, mkLease("T-014", "alice", "pkg/b/", "pkg/a")); err != nil {
		t.Fatal(err)
	}
	clock.Advance(DefaultLeaseTTL + time.Minute)
	if leases, err := b.ListActiveLeases(ctx); err != nil || len(leases) != 0 {
		t.Fatalf("ListActiveLeases = %+v, %v; want none active", leases, err)
	}
	l, err := b.ReadClaim(ctx, "14")
	if err != nil {
		t.Fatalf("ReadClaim: %v", err)
	}
	if l.TicketID != "T-014" || l.KeeperID != "alice" || !reflect.DeepEqual(l.Directories, []string{"pkg/a", "pkg/b"}) {
		t.Errorf("ReadClaim = %+v; want T-014 by alice on [pkg/a pkg/b]", l)
	}
	if !l.ExpiresAt.Before(clock.Now()) {
		t.Errorf("expires_at %v is not in the past", l.ExpiresAt)
	}
}

// T-014 phase A test 2: a missing ticket is ErrNotFound, and the ref match is
// exact (ticket-10 does not answer for ticket-1).
func TestReadClaimNotFound(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	b := newBackend(t, dir, "", newClock())
	ctx := context.Background()
	if _, err := b.ReadClaim(ctx, "T-001"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty repo: %v; want ErrNotFound", err)
	}
	if err := b.AcquireLease(ctx, mkLease("T-010", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReadClaim(ctx, "T-001"); !errors.Is(err, ErrNotFound) {
		t.Errorf("T-001 beside ticket-10: %v; want ErrNotFound", err)
	}
	if _, err := b.ReadClaim(ctx, "T-0"); !errors.Is(err, ErrInvalidTicketID) {
		t.Errorf("T-0: %v; want ErrInvalidTicketID", err)
	}
}

// T-014 phase A test 2: forged claims (T-009 test 8 plumbing) are
// ErrMalformedClaim with the reason, and no lease is returned.
func TestReadClaimMalformed(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	b := newBackend(t, dir, "", newClock())
	ctx := context.Background()
	data, err := json.Marshal(Lease{Version: LeaseVersion, TicketID: "T-004", KeeperID: "mallory", Role: "builder",
		Gate: "full", Directories: []string{".waggle"}, ClaimedAt: t0, ExpiresAt: t0.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	forgeClaim(t, dir, ticketRef(4), string(data))
	forgeClaim(t, dir, ticketRef(5), `{"version": 2, "ticket_id": "T-006", "keeper_id": "m", "role": "builder",`+
		` "gate": "full", "directories": ["ok"], "claimed_at": "2026-10-17T14:00:00Z", "expires_at": "2026-10-17T16:00:00Z"}`)
	blob := runGit(t, dir, "x", nil, "hash-object", "-w", "--stdin")
	runGit(t, dir, "", nil, "update-ref", ticketRef(6), blob)

	want := map[string]string{"T-004": ".waggle", "T-005": "does not match ref", "T-006": "not a commit"}
	for id, reason := range want {
		l, err := b.ReadClaim(ctx, id)
		if !errors.Is(err, ErrMalformedClaim) || !strings.Contains(err.Error(), reason) {
			t.Errorf("ReadClaim(%s) = %v; want ErrMalformedClaim naming %q", id, err, reason)
		}
		if !reflect.DeepEqual(l, Lease{}) {
			t.Errorf("ReadClaim(%s) returned lease %+v; want zero", id, l)
		}
	}
}

// ReadClaim is unavailable in relay comb mode, like every Backend method.
func TestReadClaimRelay(t *testing.T) {
	b, err := NewGitBackend(Options{RepoDir: t.TempDir(), CombMode: CombModeRelay})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReadClaim(context.Background(), "T-001"); !errors.Is(err, ErrRelayNotImplemented) {
		t.Errorf("ReadClaim: %v; want ErrRelayNotImplemented", err)
	}
}
