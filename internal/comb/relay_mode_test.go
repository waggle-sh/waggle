// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func newRelayBackend(t *testing.T, dir, remote string, clock *fakeClock) *GitBackend {
	t.Helper()
	b, err := NewGitBackend(Options{
		RepoDir: dir, Remote: remote, Now: clock.Now, ExtraEnv: isolatedEnv,
		CombMode: CombModeRelay, HeadKeeper: StaticHeadKeepers{headKeeper},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// A8: relay mode claims, renews, reads, lists and releases like direct mode.
func TestRelayModeLocal(t *testing.T) {
	requireGit(t)
	clock := newClock()
	b := newRelayBackend(t, newRepo(t, false), "", clock)
	ctx := context.Background()

	if err := b.AcquireLease(ctx, mkLease("T-001", "k", "x")); err != nil {
		t.Fatalf("AcquireLease: %v", err)
	}
	if err := b.AcquireLease(ctx, mkLease("T-001", "k", "x")); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if err := b.AcquireLease(ctx, mkLease("T-001", "other", "y")); !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("other owner: %v; want ErrAlreadyClaimed", err)
	}
	l, err := b.ReadClaim(ctx, "T-001")
	if err != nil || l.KeeperID != "k" {
		t.Fatalf("ReadClaim = %+v, %v", l, err)
	}
	leases, err := b.ListActiveLeases(ctx)
	if err != nil || len(leases) != 1 {
		t.Fatalf("ListActiveLeases = %+v, %v", leases, err)
	}
	if _, err := b.ReadClaimRecords(ctx); err != nil {
		t.Fatalf("ReadClaimRecords: %v", err)
	}
	if _, err := b.ReadReceipts(ctx); err != nil {
		t.Fatalf("ReadReceipts: %v", err)
	}
	oid, err := b.ClaimOID(ctx, "T-001")
	if err != nil {
		t.Fatalf("ClaimOID: %v", err)
	}
	if got, err := b.RefOID(ctx, "refs/waggle/claims/ticket-1"); err != nil || got != oid {
		t.Fatalf("RefOID = %q, %v; want %q", got, err, oid)
	}
	if err := b.ReleaseLease(ctx, "T-001", "other"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("release by other: %v; want ErrNotOwner", err)
	}
	if err := b.ReleaseLease(ctx, "T-001", "k"); err != nil {
		t.Fatalf("ReleaseLease: %v", err)
	}
	if _, err := b.ReadClaim(ctx, "T-001"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadClaim after release: %v; want ErrNotFound", err)
	}
}

func TestRelayModeRemote(t *testing.T) {
	requireGit(t)
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ba := newRelayBackend(t, dirA, "origin", clock)
	bb := newRelayBackend(t, dirB, "origin", clock)
	ctx := context.Background()

	if err := ba.AcquireLease(ctx, mkLease("T-010", "alice", "internal/comb")); err != nil {
		t.Fatal(err)
	}
	if claimRefs(t, origin) != "refs/waggle/claims/ticket-10" {
		t.Fatal("claim not on origin")
	}
	leases, err := bb.ListActiveLeases(ctx)
	if err != nil || len(leases) != 1 || leases[0].KeeperID != "alice" {
		t.Fatalf("B sees %+v, %v", leases, err)
	}
	if err := bb.AcquireLease(ctx, mkLease("T-011", "bob", "internal")); !errors.Is(err, ErrLeaseOverlap) {
		t.Fatalf("B overlap: %v; want ErrLeaseOverlap", err)
	}
	oid := refOID(t, origin, "refs/waggle/claims/ticket-10")
	if err := bb.ForceRelease(ctx, forceReq("refs/waggle/claims/ticket-10", oid)); err != nil {
		t.Fatalf("ForceRelease: %v", err)
	}
	if refs := claimRefs(t, origin); refs != "" {
		t.Fatalf("origin still has %s", refs)
	}
}

// A8: AcquireLease output is unchanged byte for byte.
func TestAcquireLeaseMessageGolden(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	b := newBackend(t, dir, "", newClock())
	ctx := context.Background()
	if err := b.AcquireLease(ctx, mkLease("T-001", "k", "x")); err != nil {
		t.Fatal(err)
	}
	oid, _ := b.ClaimOID(ctx, "T-001")
	const want = "waggle: claim T-001 by k (builder, full)"
	if got := runGit(t, dir, "", nil, "log", "-1", "--format=%B", oid); got != want {
		t.Errorf("message = %q; want %q", got, want)
	}
	// The raw message is exactly the subject and one newline.
	if got := runGit(t, dir, "", nil, "cat-file", "commit", oid); !strings.HasSuffix(got, "\n\n"+want) {
		t.Errorf("raw commit = %q", got)
	}
	// An empty trailer list is the same method call as AcquireLease.
	if err := b.AcquireLeaseWithTrailers(ctx, mkLease("T-001", "k", "x"), nil); err != nil {
		t.Fatal(err)
	}
	oid2, _ := b.ClaimOID(ctx, "T-001")
	if got := runGit(t, dir, "", nil, "log", "-1", "--format=%B", oid2); got != "waggle: renew T-001 by k (builder, full)" {
		t.Errorf("renew message = %q", got)
	}
}

func TestAcquireLeaseWithTrailers(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	b := newRelayBackend(t, dir, "", clock)
	ctx := context.Background()
	trailers := []Trailer{
		{TrailerRelayRequest, "https://github.com/o/r/issues/40#issuecomment-1"},
		{TrailerRelayRequester, "alice"},
		{TrailerRelayApprovedBy, "maintainer1"},
	}
	if err := b.AcquireLeaseWithTrailers(ctx, mkLease("T-001", "alice", "x"), trailers); err != nil {
		t.Fatal(err)
	}
	oid, _ := b.ClaimOID(ctx, "T-001")
	want := "waggle: claim T-001 by alice (builder, full)\n\n" +
		"Waggle-Relay-Request: https://github.com/o/r/issues/40#issuecomment-1\n" +
		"Waggle-Relay-Requester: alice\n" +
		"Waggle-Relay-Approved-By: maintainer1"
	if got := runGit(t, dir, "", nil, "log", "-1", "--format=%B", oid); got != want {
		t.Errorf("message = %q; want %q", got, want)
	}
	if got, err := b.RelayRequester(ctx, "T-001"); err != nil || got != "alice" {
		t.Errorf("RelayRequester = %q, %v", got, err)
	}
	// A later renewal by an older CLI (no trailers) does not hide the requester.
	clock.Advance(time.Minute)
	if err := b.AcquireLease(ctx, mkLease("T-001", "alice", "x")); err != nil {
		t.Fatal(err)
	}
	if got, err := b.RelayRequester(ctx, "T-001"); err != nil || got != "alice" {
		t.Errorf("after plain renewal RelayRequester = %q, %v", got, err)
	}
	// A maintainer claim has none.
	if err := b.AcquireLease(ctx, mkLease("T-002", "bob", "y")); err != nil {
		t.Fatal(err)
	}
	if got, err := b.RelayRequester(ctx, "T-002"); err != nil || got != "" {
		t.Errorf("maintainer claim RelayRequester = %q, %v", got, err)
	}
	if _, err := b.RelayRequester(ctx, "T-009"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing claim: %v; want ErrNotFound", err)
	}
}

func TestTrailerValidation(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	b := newRelayBackend(t, dir, "", newClock())
	ctx := context.Background()
	cases := map[string][]Trailer{
		"newline in value":     {{TrailerRelayRequester, "alice\nWaggle-Relay-Approved-By: root"}},
		"carriage return":      {{TrailerRelayRequest, "x\ry"}},
		"over 256 bytes":       {{TrailerRelayRequest, strings.Repeat("a", 257)}},
		"non ASCII":            {{TrailerRelayRequest, "café"}},
		"NUL":                  {{TrailerRelayRequest, "a\x00b"}},
		"empty value":          {{TrailerRelayRequest, ""}},
		"unknown key":          {{"Signed-off-by", "alice"}},
		"duplicate key":        {{TrailerRelayRequester, "a"}, {TrailerRelayRequester, "b"}},
		"requester not login":  {{TrailerRelayRequester, "a b"}},
		"approver not login":   {{TrailerRelayApprovedBy, "auto:"}},
		"approver bad pattern": {{TrailerRelayApprovedBy, "-x"}},
	}
	for name, tr := range cases {
		t.Run(name, func(t *testing.T) {
			err := b.AcquireLeaseWithTrailers(ctx, mkLease("T-001", "k", "x"), tr)
			if !errors.Is(err, ErrInvalidLease) {
				t.Fatalf("err = %v; want ErrInvalidLease", err)
			}
			if _, err := b.ReadClaim(ctx, "T-001"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("a claim was written: %v", err)
			}
		})
	}
	ok := []Trailer{
		{TrailerRelayRequest, strings.Repeat("a", 256)},
		{TrailerRelayApprovedBy, "auto:MEMBER"},
		{TrailerRelayApprovedBy, "auto:nectar"},
	}
	for i, tr := range ok {
		if _, err := formatTrailers([]Trailer{tr}); err != nil {
			t.Errorf("ok[%d]: %v", i, err)
		}
	}
}
