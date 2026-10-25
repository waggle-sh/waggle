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

// T-039 2.1: the branch list table.
func TestValidateBranches(t *testing.T) {
	ok := [][]string{
		nil, {}, {"t-039"}, {"t-036a"}, {"copilot/add-x"}, {"o/r:t-039"}, {"a", "b", "c", "d"},
	}
	for _, l := range ok {
		if err := ValidateBranches(l); err != nil {
			t.Errorf("%q: %v; want valid", l, err)
		}
	}
	bad := map[string][]string{
		"five":           {"a", "b", "c", "d", "e"},
		"duplicate":      {"a", "a"},
		"empty":          {""},
		"256 bytes":      {strings.Repeat("a", 256)},
		"dash first":     {"-x"},
		"slash first":    {"/x"},
		"slash last":     {"x/"},
		"dot last":       {"x."},
		"lock":           {"x.lock"},
		"dot component":  {"a/.b"},
		"lock component": {"a/b.lock/c"},
		"double dot":     {"a..b"},
		"at brace":       {"a@{b"},
		"double slash":   {"a//b"},
		"at":             {"@"},
		"refs":           {"refs/heads/x"},
		"space":          {"a b"},
		"control":        {"a\x01b"},
		"tilde":          {"a~b"},
		"non-ascii":      {"café"},
		"bad owner":      {"-o/r:x"},
		"bad repo":       {"o/r r:x"},
		"no repo":        {"o:x"},
		"dot repo":       {"o/..:x"},
		"bad fork name":  {"o/r:a..b"},
	}
	for name, l := range bad {
		err := ValidateBranches(l)
		if !errors.Is(err, ErrInvalidLease) {
			t.Errorf("%s: %v; want ErrInvalidLease", name, err)
			continue
		}
		for _, e := range l {
			if len(e) > 1 && strings.Contains(err.Error(), e) {
				t.Errorf("%s: error echoes the entry: %v", name, err)
			}
		}
	}
	if err := ValidateBranches([]string{"a", "b..c"}); err == nil || !strings.Contains(err.Error(), "entry 2") {
		t.Errorf("error should name the position: %v", err)
	}
	l := mkLease("T-039", "alice", "pkg")
	l.Branches = []string{"a..b"}
	if _, err := validateLeaseFields(l, 8); !errors.Is(err, ErrInvalidLease) {
		t.Errorf("validateLeaseFields with a bad list: %v", err)
	}
}

// T-039 2.1, Q-T039-4 (a): a claim round-trips its list at version 2, an older
// claim reads with none, and a bad stored list is malformed.
func TestLeaseBranchesRoundTrip(t *testing.T) {
	requireGit(t)
	if LeaseVersion != 2 {
		t.Fatalf("LeaseVersion = %d; want 2", LeaseVersion)
	}
	dir := newRepo(t, false)
	b := newBackend(t, dir, "", newClock())
	ctx := context.Background()
	l := mkLease("T-039", "alice", "pkg")
	l.Branches = []string{"copilot/x", "t-039"}
	if err := b.AcquireLease(ctx, l); err != nil {
		t.Fatal(err)
	}
	got, err := b.ReadClaim(ctx, "T-039")
	if err != nil || !reflect.DeepEqual(got.Branches, l.Branches) {
		t.Fatalf("ReadClaim = %+v, %v; want branches %v", got, err, l.Branches)
	}
	old, err := json.Marshal(Lease{Version: LeaseVersion, TicketID: "T-040", KeeperID: "alice", Role: "builder",
		Gate: "full", Directories: []string{"pkg"}, ClaimedAt: t0, ExpiresAt: t0.Add(100 * 365 * 24 * time.Hour), Branches: []string{}})
	if err != nil || strings.Contains(string(old), "branches") {
		t.Fatalf("empty list written: %s, %v", old, err)
	}
	forgeClaim(t, dir, ticketRef(40), string(old))
	if got, err := b.ReadClaim(ctx, "T-040"); err != nil || got.Branches != nil {
		t.Errorf("claim with no key = %+v, %v; want nil branches", got, err)
	}
	data, err := json.Marshal(Lease{Version: LeaseVersion, TicketID: "T-041", KeeperID: "m", Role: "builder",
		Gate: "full", Directories: []string{"pkg"}, ClaimedAt: t0, ExpiresAt: t0.Add(time.Hour), Branches: []string{"a..b"}})
	if err != nil {
		t.Fatal(err)
	}
	forgeClaim(t, dir, ticketRef(41), string(data))
	if _, err := b.ReadClaim(ctx, "T-041"); !errors.Is(err, ErrMalformedClaim) {
		t.Errorf("bad stored list: %v; want ErrMalformedClaim", err)
	}
}
