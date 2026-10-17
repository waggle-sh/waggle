package comb

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestParseTicketID(t *testing.T) {
	valid := []struct{ in, canon, ref string }{
		{"1", "T-001", "refs/waggle/claims/ticket-1"},
		{"001", "T-001", "refs/waggle/claims/ticket-1"},
		{"T-1", "T-001", "refs/waggle/claims/ticket-1"},
		{"T-001", "T-001", "refs/waggle/claims/ticket-1"},
		{"T-002", "T-002", "refs/waggle/claims/ticket-2"},
		{"T-1234", "T-1234", "refs/waggle/claims/ticket-1234"},
		{"999999", "T-999999", "refs/waggle/claims/ticket-999999"},
		{"T-000010", "T-010", "refs/waggle/claims/ticket-10"},
	}
	for _, tc := range valid {
		got, err := ParseTicketID(tc.in)
		if err != nil || got != tc.canon {
			t.Errorf("ParseTicketID(%q) = %q, %v; want %q", tc.in, got, err, tc.canon)
		}
		ref, err := RefName(tc.in)
		if err != nil || ref != tc.ref {
			t.Errorf("RefName(%q) = %q, %v; want %q", tc.in, ref, err, tc.ref)
		}
	}

	invalid := []string{
		"", " ", " 1", "1 ", "t-1", "T-", "T-0", "0", "000", "-1", "1a", "1/2", "../x", "1..2",
		"ticket-1", "T-1234567", "1234567", "1\x00", "\x00", "1\n", "\n1", "@{", "T-1@{0}",
		"\u0661", "T-\u0661", "\uff11", "T--1", "TT-1", "T-+1", "+1",
	}
	for _, in := range invalid {
		if got, err := ParseTicketID(in); !errors.Is(err, ErrInvalidTicketID) {
			t.Errorf("ParseTicketID(%q) = %q, %v; want ErrInvalidTicketID", in, got, err)
		}
		if got, err := RefName(in); !errors.Is(err, ErrInvalidTicketID) {
			t.Errorf("RefName(%q) = %q, %v; want ErrInvalidTicketID", in, got, err)
		}
	}
}

func TestParseClaimRef(t *testing.T) {
	cases := map[string]int{
		"refs/waggle/claims/ticket-1":      1,
		"refs/waggle/claims/ticket-999999": 999999,
		"refs/waggle/claims/ticket-007":    0,
		"refs/waggle/claims/ticket-0":      0,
		"refs/waggle/claims/T-001":         0,
		"refs/waggle/claims/ticket-1/x":    0,
		"refs/heads/ticket-1":              0,
	}
	for ref, want := range cases {
		n, ok := parseClaimRef(ref)
		if want == 0 && ok {
			t.Errorf("parseClaimRef(%q) = %d, true; want not ok", ref, n)
		}
		if want != 0 && (!ok || n != want) {
			t.Errorf("parseClaimRef(%q) = %d, %v; want %d", ref, n, ok, want)
		}
	}
}

func TestNormalizeDirectories(t *testing.T) {
	ok := []struct {
		in   []string
		want []string
	}{
		{[]string{"internal/comb/"}, []string{"internal/comb"}},
		{[]string{"./internal//comb"}, []string{"internal/comb"}},
		{[]string{"internal/comb", "internal/comb/", "./internal/comb"}, []string{"internal/comb"}},
		{[]string{"z", "cmd/waggle", "a/b/../c"}, []string{"a/c", "cmd/waggle", "z"}},
		{[]string{".github/ISSUE_TEMPLATE"}, []string{".github/ISSUE_TEMPLATE"}},
		{[]string{".gitignore-dir"}, []string{".gitignore-dir"}},
		// Duplicates do not count against the limit.
		{[]string{"a", "b", "c", "d", "e", "a", "b"}, []string{"a", "b", "c", "d", "e"}},
	}
	for _, tc := range ok {
		got, err := NormalizeDirectories(tc.in, 5)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("NormalizeDirectories(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}

	bad := []struct {
		in   []string
		want error
	}{
		{nil, ErrInvalidLease},
		{[]string{}, ErrInvalidLease},
		{[]string{""}, ErrInvalidLease},
		{[]string{"   "}, ErrInvalidLease},
		{[]string{" internal"}, ErrInvalidLease},
		{[]string{"/"}, ErrInvalidLease},
		{[]string{"/etc"}, ErrInvalidLease},
		{[]string{"C:/x"}, ErrInvalidLease},
		{[]string{"../x"}, ErrInvalidLease},
		{[]string{".."}, ErrInvalidLease},
		{[]string{"a/../../x"}, ErrInvalidLease},
		{[]string{"."}, ErrTier3Path},
		{[]string{"./"}, ErrTier3Path},
		{[]string{"a/.."}, ErrTier3Path},
		{[]string{".git"}, ErrInvalidLease},
		{[]string{".git/hooks"}, ErrInvalidLease},
		{[]string{".GIT/hooks"}, ErrInvalidLease},
		{[]string{"./.git/"}, ErrInvalidLease},
		{[]string{"internal/*"}, ErrInvalidLease},
		{[]string{"internal/?"}, ErrInvalidLease},
		{[]string{"internal/[ab]"}, ErrInvalidLease},
		{[]string{"a\\b"}, ErrInvalidLease},
		{[]string{"a\x00b"}, ErrInvalidLease},
		{[]string{"a\nb"}, ErrInvalidLease},
		{[]string{"good", ""}, ErrInvalidLease},
		{[]string{"a", "b", "c", "d", "e", "f"}, ErrInvalidLease},
	}
	for _, tc := range bad {
		got, err := NormalizeDirectories(tc.in, 5)
		if !errors.Is(err, tc.want) {
			t.Errorf("NormalizeDirectories(%q) = %q, %v; want %v", tc.in, got, err, tc.want)
		}
	}

	// maxDirs <= 0 falls back to the default of 5.
	if _, err := NormalizeDirectories([]string{"a", "b", "c", "d", "e", "f"}, 0); !errors.Is(err, ErrInvalidLease) {
		t.Errorf("default limit not applied: %v", err)
	}
}

func TestDirsOverlap(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"internal/comb", "internal/combx", false},
		{"internal/combx", "internal/comb", false},
		{"internal", "internal/comb", true},
		{"internal/comb", "internal", true},
		{"internal/comb", "internal/comb", true},
		{"internal/comb/x", "internal/comb", true},
		{"cmd", "internal", false},
		{"a/b", "a/c", false},
	}
	for _, tc := range cases {
		if got := DirsOverlap(tc.a, tc.b); got != tc.want {
			t.Errorf("DirsOverlap(%q, %q) = %v; want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestValidateLeaseFields(t *testing.T) {
	base := Lease{TicketID: "2", KeeperID: "paul", Role: "forager", Gate: "full", Directories: []string{"internal/comb/"}}
	got, err := validateLeaseFields(base, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.TicketID != "T-002" || !reflect.DeepEqual(got.Directories, []string{"internal/comb"}) {
		t.Errorf("unexpected normalized lease %+v", got)
	}

	mut := func(f func(*Lease)) Lease {
		l := base
		l.Directories = append([]string(nil), base.Directories...)
		f(&l)
		return l
	}
	cases := []struct {
		name string
		l    Lease
		want error
	}{
		{"bad ticket", mut(func(l *Lease) { l.TicketID = "t-1" }), ErrInvalidTicketID},
		{"empty keeper", mut(func(l *Lease) { l.KeeperID = "" }), ErrInvalidLease},
		{"dash keeper", mut(func(l *Lease) { l.KeeperID = "-x" }), ErrInvalidLease},
		{"space keeper", mut(func(l *Lease) { l.KeeperID = "a b" }), ErrInvalidLease},
		{"bad role", mut(func(l *Lease) { l.Role = "queen" }), ErrInvalidLease},
		{"bad gate", mut(func(l *Lease) { l.Gate = "none" }), ErrInvalidLease},
		{"tier3", mut(func(l *Lease) { l.Directories = []string{".github"} }), ErrTier3Path},
		{"tier2 light", mut(func(l *Lease) { l.Gate = "light"; l.Directories = []string{"CLAUDE.md"} }), ErrGateTooLow},
		{"tier2 file light", mut(func(l *Lease) { l.Gate = "light"; l.Directories = []string{"agents.md"} }), ErrGateTooLow},
	}
	for _, tc := range cases {
		if _, err := validateLeaseFields(tc.l, 5); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v; want %v", tc.name, err, tc.want)
		}
	}
}

func TestTier3Detection(t *testing.T) {
	tier3 := []string{
		".waggle", ".waggle/roles", ".devcontainer", ".github", ".github/workflows",
		".github/workflows/x", ".gitlab-ci.yml", ".WAGGLE", ".GitHub/Workflows", ".DevContainer/x",
	}
	for _, d := range tier3 {
		if !isTier3Dir(d) {
			t.Errorf("isTier3Dir(%q) = false; want true", d)
		}
	}
	allowed := []string{".github/ISSUE_TEMPLATE", "internal/comb", ".githubx", ".devcontainerx", "docs", "waggle"}
	for _, d := range allowed {
		if isTier3Dir(d) {
			t.Errorf("isTier3Dir(%q) = true; want false", d)
		}
	}
}

func TestMalformedClaimsError(t *testing.T) {
	var err error = &MalformedClaimsError{Claims: []MalformedClaim{{Ref: "refs/waggle/claims/x", Reason: "bad"}}}
	if !errors.Is(err, ErrMalformedClaim) {
		t.Error("errors.Is(MalformedClaimsError, ErrMalformedClaim) = false")
	}
	if err.Error() == "" {
		t.Error("empty error message")
	}
}

// Test 16: relay mode.
func TestRelayMode(t *testing.T) {
	b, err := NewGitBackend(Options{RepoDir: t.TempDir(), CombMode: CombModeRelay})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	l := Lease{TicketID: "T-001", KeeperID: "k", Role: "forager", Gate: "full", Directories: []string{"x"}}
	if err := b.AcquireLease(ctx, l); !errors.Is(err, ErrRelayNotImplemented) {
		t.Errorf("AcquireLease: %v", err)
	}
	if err := b.ReleaseLease(ctx, "T-001", "k"); !errors.Is(err, ErrRelayNotImplemented) {
		t.Errorf("ReleaseLease: %v", err)
	}
	if _, err := b.ListActiveLeases(ctx); !errors.Is(err, ErrRelayNotImplemented) {
		t.Errorf("ListActiveLeases: %v", err)
	}
}

// Test 17: options validation.
func TestNewGitBackendOptions(t *testing.T) {
	bad := []Options{
		{},
		{RepoDir: "  "},
		{RepoDir: "r", Remote: "-oops"},
		{RepoDir: "r", Remote: "origin;rm"},
		{RepoDir: "r", Remote: "a b"},
		{RepoDir: "r", Remote: ".hidden"},
		{RepoDir: "r", CombMode: "carrier-pigeon"},
		{RepoDir: "r", CombMode: "Direct"},
	}
	for _, o := range bad {
		if _, err := NewGitBackend(o); err == nil {
			t.Errorf("NewGitBackend(%+v) succeeded; want error", o)
		}
	}
	good := []Options{
		{RepoDir: "r"},
		{RepoDir: "r", Remote: "origin", CombMode: "direct"},
		{RepoDir: "r", Remote: "my-fork_2.x", CombMode: "relay"},
	}
	for _, o := range good {
		b, err := NewGitBackend(o)
		if err != nil {
			t.Errorf("NewGitBackend(%+v): %v", o, err)
			continue
		}
		if b.maxDirs != DefaultMaxDirectoriesPerClaim || b.ttl != DefaultLeaseTTL || b.gitBin != "git" || b.now == nil {
			t.Errorf("defaults not applied: %+v", b)
		}
	}
}
