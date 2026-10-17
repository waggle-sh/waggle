package comb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// isolatedEnv keeps tests independent of the developer's git config.
var isolatedEnv = []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}

var t0 = time.Date(2026, 10, 17, 14, 0, 0, 0, time.UTC)

// requireGit skips the test when git is not on PATH.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *fakeClock { return &fakeClock{t: t0} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// runGit runs git in dir for test setup and inspection. It must only be called
// from the test goroutine.
func runGit(t *testing.T, dir, stdin string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(append(append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0"), isolatedEnv...), env...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, errb.String())
	}
	return strings.TrimSpace(out.String())
}

func newRepo(t *testing.T, bare bool) string {
	t.Helper()
	dir := t.TempDir()
	if bare {
		runGit(t, dir, "", nil, "init", "-q", "--bare")
	} else {
		runGit(t, dir, "", nil, "init", "-q")
	}
	return dir
}

func newBackend(t *testing.T, dir, remote string, clock *fakeClock) *GitBackend {
	t.Helper()
	b, err := NewGitBackend(Options{RepoDir: dir, Remote: remote, Now: clock.Now, ExtraEnv: isolatedEnv})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mkLease(ticket, keeper string, dirs ...string) Lease {
	return Lease{TicketID: ticket, KeeperID: keeper, Role: "builder", Gate: "full", Directories: dirs}
}

func claimRefs(t *testing.T, dir string) string {
	t.Helper()
	return runGit(t, dir, "", nil, "for-each-ref", "--format=%(refname)", ClaimRefPrefix)
}

func parents(t *testing.T, dir, oid string) []string {
	t.Helper()
	var ps []string
	for _, line := range strings.Split(runGit(t, dir, "", nil, "cat-file", "-p", oid), "\n") {
		if p, ok := strings.CutPrefix(line, "parent "); ok {
			ps = append(ps, p)
		}
	}
	return ps
}

func readLease(t *testing.T, dir, rev string) Lease {
	t.Helper()
	var l Lease
	if err := json.Unmarshal([]byte(runGit(t, dir, "", nil, "cat-file", "blob", rev+":lease.json")), &l); err != nil {
		t.Fatal(err)
	}
	return l
}

// Test 3: Tier 3 rejection writes nothing.
func TestAcquireTier3Rejected(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	b := newBackend(t, dir, "", newClock())
	ctx := context.Background()
	for _, d := range []string{
		".waggle", ".waggle/roles", ".devcontainer", ".github", ".github/workflows",
		".github/workflows/x", ".gitlab-ci.yml", ".WAGGLE", ".", "./", "a/..",
	} {
		err := b.AcquireLease(ctx, mkLease("T-001", "k1", "internal/ok", d))
		if !errors.Is(err, ErrTier3Path) {
			t.Errorf("dir %q: got %v; want ErrTier3Path", d, err)
		}
	}
	if refs := claimRefs(t, dir); refs != "" {
		t.Fatalf("refs created after rejection: %s", refs)
	}
	if err := b.AcquireLease(ctx, mkLease("T-001", "k1", ".github/ISSUE_TEMPLATE")); err != nil {
		t.Fatalf(".github/ISSUE_TEMPLATE: %v", err)
	}
}

// Test 4: Tier 2 floor.
func TestAcquireTier2Floor(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	b := newBackend(t, dir, "", newClock())
	ctx := context.Background()
	light := mkLease("T-001", "k1", "CLAUDE.md")
	light.Gate = "light"
	if err := b.AcquireLease(ctx, light); !errors.Is(err, ErrGateTooLow) {
		t.Errorf("CLAUDE.md light: %v", err)
	}
	agents := mkLease("T-001", "k1", "agents.md")
	agents.Gate = "light"
	if err := b.AcquireLease(ctx, agents); !errors.Is(err, ErrGateTooLow) {
		t.Errorf("agents.md light: %v", err)
	}
	if refs := claimRefs(t, dir); refs != "" {
		t.Fatalf("refs created after rejection: %s", refs)
	}
	if err := b.AcquireLease(ctx, mkLease("T-001", "k1", "CLAUDE.md")); err != nil {
		t.Errorf("CLAUDE.md full: %v", err)
	}
}

func TestAcquireInvalidLease(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	b := newBackend(t, dir, "", clock)
	ctx := context.Background()
	v2 := mkLease("T-001", "k1", "x")
	v2.Version = 2
	past := mkLease("T-001", "k1", "x")
	past.ExpiresAt = t0
	backwards := mkLease("T-001", "k1", "x")
	backwards.ClaimedAt = t0.Add(3 * time.Hour)
	backwards.ExpiresAt = t0.Add(2 * time.Hour)
	tooMany := mkLease("T-001", "k1", "a", "b", "c", "d", "e", "f")
	cases := []struct {
		name string
		l    Lease
		want error
	}{
		{"ticket", mkLease("T-0", "k1", "x"), ErrInvalidTicketID},
		{"version", v2, ErrInvalidLease},
		{"expired", past, ErrInvalidLease},
		{"backwards", backwards, ErrInvalidLease},
		{"too many dirs", tooMany, ErrInvalidLease},
		{"keeper", mkLease("T-001", "--upload-pack=x", "x"), ErrInvalidLease},
	}
	for _, tc := range cases {
		if err := b.AcquireLease(ctx, tc.l); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v; want %v", tc.name, err, tc.want)
		}
	}
	if refs := claimRefs(t, dir); refs != "" {
		t.Fatalf("refs created after rejection: %s", refs)
	}
}

// Test 5: backend-level overlap.
func TestAcquireOverlap(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	b := newBackend(t, dir, "", clock)
	ctx := context.Background()
	if err := b.AcquireLease(ctx, mkLease("T-001", "k1", "internal/comb")); err != nil {
		t.Fatal(err)
	}
	if err := b.AcquireLease(ctx, mkLease("T-002", "k2", "internal")); !errors.Is(err, ErrLeaseOverlap) {
		t.Errorf("T-002 on internal: %v; want ErrLeaseOverlap", err)
	}
	if err := b.AcquireLease(ctx, mkLease("T-003", "k2", "internal/combx")); err != nil {
		t.Errorf("T-003 on internal/combx: %v", err)
	}
	// Same keeper does not bypass overlap on a different ticket.
	if err := b.AcquireLease(ctx, mkLease("T-004", "k1", "internal/comb/sub")); !errors.Is(err, ErrLeaseOverlap) {
		t.Errorf("T-004 on internal/comb/sub: %v; want ErrLeaseOverlap", err)
	}
	clock.Advance(3 * time.Hour)
	if err := b.AcquireLease(ctx, mkLease("T-002", "k2", "internal")); err != nil {
		t.Errorf("after expiry, T-002 on internal: %v", err)
	}
}

// Test 6: commit shape; working tree and index untouched; no user config needed.
func TestAcquireCommitShape(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statusBefore := runGit(t, dir, "", nil, "status", "--porcelain")
	_, idxErr := os.Stat(filepath.Join(dir, ".git", "index"))
	indexBefore := idxErr == nil

	clock := newClock()
	b := newBackend(t, dir, "", clock)
	ctx := context.Background()
	if err := b.AcquireLease(ctx, mkLease("1", "paulternate", "internal/comb/")); err != nil {
		t.Fatal(err)
	}
	oid, err := b.ClaimOID(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if got := runGit(t, dir, "", nil, "rev-parse", "refs/waggle/claims/ticket-1"); got != oid {
		t.Fatalf("ref = %s; ClaimOID = %s", got, oid)
	}
	if typ := runGit(t, dir, "", nil, "cat-file", "-t", oid); typ != "commit" {
		t.Fatalf("ref points to %s", typ)
	}
	if ps := parents(t, dir, oid); len(ps) != 0 {
		t.Fatalf("first claim has parents %v", ps)
	}
	tree := runGit(t, dir, "", nil, "ls-tree", oid)
	if lines := strings.Split(tree, "\n"); len(lines) != 1 || !strings.HasPrefix(lines[0], "100644 blob ") || !strings.HasSuffix(lines[0], "\tlease.json") {
		t.Fatalf("tree = %q", tree)
	}
	commit := runGit(t, dir, "", nil, "cat-file", "-p", oid)
	if !strings.Contains(commit, "author waggle <waggle@localhost> 1792245600 +0000") ||
		!strings.Contains(commit, "waggle: claim T-001 by paulternate (builder, full)") {
		t.Errorf("unexpected commit:\n%s", commit)
	}

	stored := readLease(t, dir, oid)
	want := Lease{Version: 1, TicketID: "T-001", KeeperID: "paulternate", Role: "builder", Gate: "full",
		Directories: []string{"internal/comb"}, ClaimedAt: t0, ExpiresAt: t0.Add(DefaultLeaseTTL)}
	if !reflect.DeepEqual(stored, want) {
		t.Errorf("stored lease = %+v\nwant %+v", stored, want)
	}
	listed, err := b.ListActiveLeases(ctx)
	if err != nil || len(listed) != 1 || !reflect.DeepEqual(listed[0], want) {
		t.Errorf("ListActiveLeases = %+v, %v", listed, err)
	}

	if after := runGit(t, dir, "", nil, "status", "--porcelain"); after != statusBefore {
		t.Errorf("git status changed: %q -> %q", statusBefore, after)
	}
	_, idxErr = os.Stat(filepath.Join(dir, ".git", "index"))
	if (idxErr == nil) != indexBefore {
		t.Errorf("index existence changed: %v -> %v", indexBefore, idxErr == nil)
	}
}

// Tests 7, 8, 9: already claimed, renewal, expiry takeover.
func TestAcquireClaimRenewTakeover(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	a := newBackend(t, dir, "", clock)
	b := newBackend(t, dir, "", clock)
	ctx := context.Background()

	if err := a.AcquireLease(ctx, mkLease("T-005", "alice", "pkg/a")); err != nil {
		t.Fatal(err)
	}
	first, _ := a.ClaimOID(ctx, "T-005")

	if err := b.AcquireLease(ctx, mkLease("T-005", "bob", "pkg/b")); !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("second keeper: %v; want ErrAlreadyClaimed", err)
	}

	clock.Advance(time.Hour)
	if err := a.AcquireLease(ctx, mkLease("T-005", "alice", "pkg/a")); err != nil {
		t.Fatalf("renewal: %v", err)
	}
	renewed, _ := a.ClaimOID(ctx, "T-005")
	if ps := parents(t, dir, renewed); !reflect.DeepEqual(ps, []string{first}) {
		t.Fatalf("renewal parents = %v; want [%s]", ps, first)
	}
	if l := readLease(t, dir, renewed); !l.ExpiresAt.Equal(t0.Add(time.Hour + DefaultLeaseTTL)) {
		t.Errorf("renewal expires_at = %v", l.ExpiresAt)
	}

	clock.Advance(DefaultLeaseTTL + time.Minute)
	if err := b.AcquireLease(ctx, mkLease("T-005", "bob", "pkg/b")); err != nil {
		t.Fatalf("takeover: %v", err)
	}
	taken, _ := b.ClaimOID(ctx, "T-005")
	if ps := parents(t, dir, taken); !reflect.DeepEqual(ps, []string{renewed}) {
		t.Fatalf("takeover parents = %v; want [%s]", ps, renewed)
	}
	if l := readLease(t, dir, taken); l.KeeperID != "bob" {
		t.Errorf("takeover keeper = %s", l.KeeperID)
	}
	if n := runGit(t, dir, "", nil, "rev-list", "--count", "refs/waggle/claims/ticket-5"); n != "3" {
		t.Errorf("chain length = %s; want 3", n)
	}
}

// Test 9 exactly as specified: one claim, expiry, takeover, 2 commits in git log.
func TestExpiryTakeoverLog(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	a := newBackend(t, dir, "", clock)
	b := newBackend(t, dir, "", clock)
	ctx := context.Background()
	l := mkLease("T-006", "alice", "pkg/x")
	l.ExpiresAt = t0.Add(10 * time.Minute)
	if err := a.AcquireLease(ctx, l); err != nil {
		t.Fatal(err)
	}
	prev, _ := a.ClaimOID(ctx, "T-006")
	clock.Advance(11 * time.Minute)
	if leases, err := b.ListActiveLeases(ctx); err != nil || len(leases) != 0 {
		t.Fatalf("expired lease listed: %+v, %v", leases, err)
	}
	if err := b.AcquireLease(ctx, mkLease("T-006", "bob", "pkg/x")); err != nil {
		t.Fatal(err)
	}
	cur, _ := b.ClaimOID(ctx, "T-006")
	if ps := parents(t, dir, cur); !reflect.DeepEqual(ps, []string{prev}) {
		t.Fatalf("parents = %v; want [%s]", ps, prev)
	}
	logOut := runGit(t, dir, "", nil, "log", "--format=%H", "refs/waggle/claims/ticket-6")
	if n := len(strings.Split(logOut, "\n")); n != 2 {
		t.Errorf("git log shows %d commits; want 2", n)
	}
	if !strings.Contains(runGit(t, dir, "", nil, "log", "-1", "--format=%s", cur), "takeover T-006 by bob") {
		t.Error("takeover message missing")
	}
	// An expired claim is still owned: the previous keeper cannot release it now.
	if err := a.ReleaseLease(ctx, "T-006", "alice"); !errors.Is(err, ErrNotOwner) {
		t.Errorf("old owner release: %v; want ErrNotOwner", err)
	}
}

// Test 10: release.
func TestRelease(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	b := newBackend(t, dir, "", clock)
	ctx := context.Background()
	if err := b.AcquireLease(ctx, mkLease("T-007", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	if err := b.ReleaseLease(ctx, "T-007", "bob"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("non-owner: %v; want ErrNotOwner", err)
	}
	if claimRefs(t, dir) != "refs/waggle/claims/ticket-7" {
		t.Fatal("ref missing after non-owner release")
	}
	// Expired claims still require the owner.
	clock.Advance(DefaultLeaseTTL + time.Second)
	if err := b.ReleaseLease(ctx, "T-007", "bob"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("non-owner on expired: %v; want ErrNotOwner", err)
	}
	if err := b.ReleaseLease(ctx, "7", "alice"); err != nil {
		t.Fatalf("owner release: %v", err)
	}
	if refs := claimRefs(t, dir); refs != "" {
		t.Fatalf("ref remains: %s", refs)
	}
	if err := b.ReleaseLease(ctx, "T-007", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second release: %v; want ErrNotFound", err)
	}
	if _, err := b.ClaimOID(ctx, "T-007"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ClaimOID after release: %v", err)
	}
	for _, bad := range []string{"", "t-7", "../x", "T-0"} {
		if err := b.ReleaseLease(ctx, bad, "alice"); !errors.Is(err, ErrInvalidTicketID) {
			t.Errorf("release %q: %v; want ErrInvalidTicketID", bad, err)
		}
	}
}

// ReleaseLease returns ErrConcurrentUpdate when the ref moves between read and delete.
func TestReleaseConcurrentUpdateLocal(t *testing.T) {
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
	if err := a.ReleaseLease(ctx, "T-008", "alice"); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("got %v; want ErrConcurrentUpdate", err)
	}
	if claimRefs(t, dir) == "" {
		t.Fatal("ref deleted despite concurrent update")
	}
}

// AcquireLease returns ErrAlreadyClaimed when another keeper writes between read and CAS.
func TestAcquireLostRaceLocalDeterministic(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	a := newBackend(t, dir, "", clock)
	rival := newBackend(t, dir, "", clock)
	ctx := context.Background()
	a.hookBeforeWrite = func() {
		if err := rival.AcquireLease(ctx, mkLease("T-009", "rival", "pkg")); err != nil {
			panic(err)
		}
	}
	if err := a.AcquireLease(ctx, mkLease("T-009", "alice", "pkg")); !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("got %v; want ErrAlreadyClaimed", err)
	}
	if l := readLease(t, dir, "refs/waggle/claims/ticket-9"); l.KeeperID != "rival" {
		t.Fatalf("winner = %s", l.KeeperID)
	}
}

// Test 11: malformed claims.
func TestMalformedClaims(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	b := newBackend(t, dir, "", clock)
	ctx := context.Background()
	idEnv := []string{"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}

	blobOf := func(s string) string { return runGit(t, dir, s, nil, "hash-object", "-w", "--stdin") }
	commitWith := func(name, content string) string {
		tree := runGit(t, dir, fmt.Sprintf("100644 blob %s\t%s\n", blobOf(content), name), nil, "mktree")
		return runGit(t, dir, "", idEnv, "commit-tree", tree, "-m", "test")
	}
	leaseJSON := func(l Lease) string {
		data, err := json.Marshal(l)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	good := func(ticket string) Lease {
		return Lease{Version: 1, TicketID: ticket, KeeperID: "mallory", Role: "builder", Gate: "full",
			Directories: []string{"shared"}, ClaimedAt: t0, ExpiresAt: t0.Add(time.Hour)}
	}
	set := func(ref, oid string) { runGit(t, dir, "", nil, "update-ref", ref, oid) }

	set("refs/waggle/claims/ticket-1", blobOf("x"))
	set("refs/waggle/claims/ticket-2", commitWith("lease.json", "{not json"))
	set("refs/waggle/claims/ticket-3", commitWith("other.txt", "hi"))
	set("refs/waggle/claims/ticket-007", commitWith("lease.json", leaseJSON(good("T-007"))))
	set("refs/waggle/claims/ticket-4", commitWith("lease.json", leaseJSON(good("T-005"))))
	badDirs := good("T-011")
	badDirs.Directories = []string{"../escape"}
	set("refs/waggle/claims/ticket-11", commitWith("lease.json", leaseJSON(badDirs)))

	if err := b.AcquireLease(ctx, mkLease("T-008", "alice", "pkg")); err != nil {
		t.Fatalf("valid acquire alongside malformed refs: %v", err)
	}
	// Malformed refs for other tickets are ignored by the overlap check.
	if err := b.AcquireLease(ctx, mkLease("T-009", "alice", "shared")); err != nil {
		t.Fatalf("overlap with malformed claim should be ignored: %v", err)
	}

	leases, err := b.ListActiveLeases(ctx)
	if !errors.Is(err, ErrMalformedClaim) {
		t.Fatalf("ListActiveLeases err = %v; want ErrMalformedClaim", err)
	}
	var mce *MalformedClaimsError
	if !errors.As(err, &mce) {
		t.Fatalf("err is not *MalformedClaimsError: %T", err)
	}
	if len(leases) != 2 || leases[0].TicketID != "T-008" || leases[1].TicketID != "T-009" {
		t.Fatalf("valid leases = %+v", leases)
	}
	gotRefs := map[string]string{}
	for _, c := range mce.Claims {
		gotRefs[c.Ref] = c.Reason
	}
	for _, ref := range []string{
		"refs/waggle/claims/ticket-1", "refs/waggle/claims/ticket-2", "refs/waggle/claims/ticket-3",
		"refs/waggle/claims/ticket-007", "refs/waggle/claims/ticket-4", "refs/waggle/claims/ticket-11",
	} {
		if gotRefs[ref] == "" {
			t.Errorf("malformed ref %s not reported (got %v)", ref, gotRefs)
		}
	}
	if len(mce.Claims) != 6 {
		t.Errorf("reported %d malformed refs; want 6: %v", len(mce.Claims), gotRefs)
	}

	for _, ticket := range []string{"T-001", "T-002", "T-003", "T-004", "T-011"} {
		if err := b.AcquireLease(ctx, mkLease(ticket, "alice", "elsewhere/"+ticket)); !errors.Is(err, ErrMalformedClaim) {
			t.Errorf("AcquireLease(%s) = %v; want ErrMalformedClaim", ticket, err)
		}
		if err := b.ReleaseLease(ctx, ticket, "mallory"); !errors.Is(err, ErrMalformedClaim) {
			t.Errorf("ReleaseLease(%s) = %v; want ErrMalformedClaim", ticket, err)
		}
	}
}

// Test 12: local concurrency.
func TestLocalConcurrency(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	const racers = 8
	backends := make([]*GitBackend, racers)
	for i := range backends {
		backends[i] = newBackend(t, dir, "", clock)
	}
	ctx := context.Background()
	for ticket := 1; ticket <= 5; ticket++ {
		id := fmt.Sprintf("T-%03d", 20+ticket)
		start := make(chan struct{})
		errs := make([]error, racers)
		var wg sync.WaitGroup
		for i := range backends {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				errs[i] = backends[i].AcquireLease(ctx, mkLease(id, fmt.Sprintf("keeper-%d", i), "race/"+id))
			}(i)
		}
		close(start)
		wg.Wait()
		winners := 0
		winner := -1
		for i, err := range errs {
			switch {
			case err == nil:
				winners++
				winner = i
			case !errors.Is(err, ErrAlreadyClaimed):
				t.Errorf("%s racer %d: %v; want ErrAlreadyClaimed", id, i, err)
			}
		}
		if winners != 1 {
			t.Fatalf("%s: %d winners; want 1", id, winners)
		}
		ref, _ := RefName(id)
		if l := readLease(t, dir, ref); l.KeeperID != fmt.Sprintf("keeper-%d", winner) {
			t.Errorf("%s: ref names %s; winner was keeper-%d", id, l.KeeperID, winner)
		}
	}
}

// remoteSetup creates a bare origin and two clones wired to it.
func remoteSetup(t *testing.T) (origin, a, b string) {
	t.Helper()
	origin = newRepo(t, true)
	a = newRepo(t, false)
	b = newRepo(t, false)
	runGit(t, a, "", nil, "remote", "add", "origin", origin)
	runGit(t, b, "", nil, "remote", "add", "origin", origin)
	return origin, a, b
}

// Test 13: remote concurrency.
func TestRemoteConcurrency(t *testing.T) {
	requireGit(t)
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ba := newBackend(t, dirA, "origin", clock)
	bb := newBackend(t, dirB, "origin", clock)
	ctx := context.Background()
	for i := 1; i <= 6; i++ {
		id := fmt.Sprintf("T-%03d", 30+i)
		start := make(chan struct{})
		var errA, errB error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; errA = ba.AcquireLease(ctx, mkLease(id, "alice", "race/"+id)) }()
		go func() { defer wg.Done(); <-start; errB = bb.AcquireLease(ctx, mkLease(id, "bob", "race/"+id)) }()
		close(start)
		wg.Wait()
		var winner string
		switch {
		case errA == nil && errors.Is(errB, ErrAlreadyClaimed):
			winner = "alice"
		case errB == nil && errors.Is(errA, ErrAlreadyClaimed):
			winner = "bob"
		default:
			t.Fatalf("%s: alice=%v bob=%v; want exactly one winner and ErrAlreadyClaimed", id, errA, errB)
		}
		ref, _ := RefName(id)
		if l := readLease(t, origin, ref); l.KeeperID != winner {
			t.Errorf("%s: origin names %s; winner %s", id, l.KeeperID, winner)
		}
	}
}

// Remote acquire loses deterministically when the rival pushes between read and push.
func TestRemoteLostRaceDeterministic(t *testing.T) {
	requireGit(t)
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ba := newBackend(t, dirA, "origin", clock)
	bb := newBackend(t, dirB, "origin", clock)
	ctx := context.Background()
	ba.hookBeforeWrite = func() {
		if err := bb.AcquireLease(ctx, mkLease("T-040", "bob", "pkg")); err != nil {
			panic(err)
		}
	}
	if err := ba.AcquireLease(ctx, mkLease("T-040", "alice", "pkg")); !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("got %v; want ErrAlreadyClaimed", err)
	}
	if l := readLease(t, origin, "refs/waggle/claims/ticket-40"); l.KeeperID != "bob" {
		t.Fatalf("origin names %s", l.KeeperID)
	}
	// Remote release CAS: bob renews from another clone between B's read and push.
	renewer := newBackend(t, dirA, "origin", clock)
	bb.hookBeforeWrite = func() {
		if err := renewer.AcquireLease(ctx, mkLease("T-040", "bob", "pkg")); err != nil {
			panic(err)
		}
	}
	if err := bb.ReleaseLease(ctx, "T-040", "bob"); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("release got %v; want ErrConcurrentUpdate", err)
	}
	if claimRefs(t, origin) != "refs/waggle/claims/ticket-40" {
		t.Fatal("origin ref deleted despite concurrent update")
	}
}

// Test 14: remote visibility and prune.
func TestRemoteVisibility(t *testing.T) {
	requireGit(t)
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ba := newBackend(t, dirA, "origin", clock)
	bb := newBackend(t, dirB, "origin", clock)
	ctx := context.Background()

	if leases, err := bb.ListActiveLeases(ctx); err != nil || len(leases) != 0 {
		t.Fatalf("empty remote: %+v, %v", leases, err)
	}
	if err := ba.AcquireLease(ctx, mkLease("T-010", "alice", "internal/comb")); err != nil {
		t.Fatal(err)
	}
	if claimRefs(t, origin) != "refs/waggle/claims/ticket-10" {
		t.Fatal("claim not on origin")
	}
	leases, err := bb.ListActiveLeases(ctx)
	if err != nil || len(leases) != 1 || leases[0].TicketID != "T-010" || leases[0].KeeperID != "alice" {
		t.Fatalf("B sees %+v, %v", leases, err)
	}
	if err := bb.AcquireLease(ctx, mkLease("T-011", "bob", "internal")); !errors.Is(err, ErrLeaseOverlap) {
		t.Fatalf("B overlap: %v; want ErrLeaseOverlap", err)
	}
	if err := bb.AcquireLease(ctx, mkLease("T-010", "bob", "elsewhere")); !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("B same ticket: %v; want ErrAlreadyClaimed", err)
	}
	if err := bb.ReleaseLease(ctx, "T-010", "bob"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("B release: %v; want ErrNotOwner", err)
	}
	if err := ba.ReleaseLease(ctx, "T-010", "alice"); err != nil {
		t.Fatal(err)
	}
	if refs := claimRefs(t, origin); refs != "" {
		t.Fatalf("origin still has %s", refs)
	}
	leases, err = bb.ListActiveLeases(ctx)
	if err != nil || len(leases) != 0 {
		t.Fatalf("after release B sees %+v, %v", leases, err)
	}
	if refs := claimRefs(t, dirB); refs != "" {
		t.Fatalf("B mirror not pruned: %s", refs)
	}
	if err := bb.ReleaseLease(ctx, "T-010", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("release missing: %v; want ErrNotFound", err)
	}
}

// Test 15: remote expiry takeover.
func TestRemoteExpiryTakeover(t *testing.T) {
	requireGit(t)
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ba := newBackend(t, dirA, "origin", clock)
	bb := newBackend(t, dirB, "origin", clock)
	ctx := context.Background()
	if err := ba.AcquireLease(ctx, mkLease("T-012", "alice", "pkg/x")); err != nil {
		t.Fatal(err)
	}
	aCommit := runGit(t, origin, "", nil, "rev-parse", "refs/waggle/claims/ticket-12")
	if err := bb.AcquireLease(ctx, mkLease("T-012", "bob", "pkg/x")); !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("before expiry: %v; want ErrAlreadyClaimed", err)
	}
	clock.Advance(DefaultLeaseTTL)
	if err := bb.AcquireLease(ctx, mkLease("T-012", "bob", "pkg/x")); err != nil {
		t.Fatalf("takeover: %v", err)
	}
	bCommit := runGit(t, origin, "", nil, "rev-parse", "refs/waggle/claims/ticket-12")
	if ps := parents(t, origin, bCommit); !reflect.DeepEqual(ps, []string{aCommit}) {
		t.Fatalf("origin parents = %v; want [%s]", ps, aCommit)
	}
	if l := readLease(t, origin, bCommit); l.KeeperID != "bob" {
		t.Errorf("origin keeper = %s", l.KeeperID)
	}
	if got := runGit(t, dirB, "", nil, "rev-parse", "refs/waggle/claims/ticket-12"); got != bCommit {
		t.Errorf("B mirror = %s; want %s", got, bCommit)
	}
	if err := ba.ReleaseLease(ctx, "T-012", "alice"); !errors.Is(err, ErrNotOwner) {
		t.Errorf("A release after takeover: %v; want ErrNotOwner", err)
	}
}

// A transport failure surfaces as a git error, not a race sentinel.
func TestRemoteTransportFailure(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	runGit(t, dir, "", nil, "remote", "add", "origin", filepath.Join(t.TempDir(), "missing.git"))
	b := newBackend(t, dir, "origin", newClock())
	ctx := context.Background()
	err := b.AcquireLease(ctx, mkLease("T-001", "alice", "pkg"))
	if err == nil || errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("got %v; want a git error", err)
	}
	if _, err := b.ListActiveLeases(ctx); err == nil {
		t.Fatal("ListActiveLeases on missing remote succeeded")
	}
}

func TestContextCanceled(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	b := newBackend(t, dir, "", newClock())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.AcquireLease(ctx, mkLease("T-001", "alice", "pkg")); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v; want context.Canceled", err)
	}
}
