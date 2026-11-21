// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

const headKeeper = "paulternate"

// hkBackend returns a backend whose Head Keeper allow-list is exactly keepers.
func hkBackend(t *testing.T, dir, remote string, clock *fakeClock, keepers ...string) *GitBackend {
	t.Helper()
	var auth HeadKeeperAuthorizer
	if keepers != nil {
		auth = StaticHeadKeepers(keepers)
	}
	b, err := NewGitBackend(Options{RepoDir: dir, Remote: remote, Now: clock.Now, ExtraEnv: isolatedEnv, HeadKeeper: auth})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func forceReq(ref, oid string) ForceRelease {
	return ForceRelease{Ref: ref, ExpectedOID: oid, HeadKeeperID: headKeeper, Reason: "keeper unreachable since 2026-10-12"}
}

// auditChain returns the first-parent OIDs of the audit ref, newest first,
// without the root commit, or nil when the ref is absent. It checks that the
// walk ends at a root with an empty tree and no parents and that every
// entry carries force-release.json.
func auditChain(t *testing.T, dir string) []string {
	t.Helper()
	if runGit(t, dir, "", nil, "for-each-ref", "--format=%(refname)", ForceReleaseAuditRef) == "" {
		return nil
	}
	chain := strings.Fields(runGit(t, dir, "", nil, "rev-list", "--first-parent", ForceReleaseAuditRef))
	root := chain[len(chain)-1]
	if ps := parents(t, dir, root); len(ps) != 0 || runGit(t, dir, "", nil, "ls-tree", root) != "" {
		t.Fatalf("audit root %s: parents %v, tree %q", root, ps, runGit(t, dir, "", nil, "ls-tree", root))
	}
	if msg := runGit(t, dir, "", nil, "log", "-1", "--format=%s", root); msg != "waggle: audit root" {
		t.Fatalf("audit root subject = %q", msg)
	}
	for _, c := range chain[:len(chain)-1] {
		if names := runGit(t, dir, "", nil, "ls-tree", "--name-only", c); !strings.Contains(names, auditRecordFile) {
			t.Fatalf("first-parent commit %s is not an audit entry: %q", c, names)
		}
	}
	return chain[:len(chain)-1]
}

func readAudit(t *testing.T, dir, rev string) auditRecord {
	t.Helper()
	var rec auditRecord
	raw := runGit(t, dir, "", nil, "cat-file", "blob", rev+":"+auditRecordFile)
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatalf("decode %s: %v\n%s", auditRecordFile, err, raw)
	}
	return rec
}

func refOID(t *testing.T, dir, ref string) string {
	t.Helper()
	return runGit(t, dir, "", nil, "rev-parse", "--verify", ref)
}

func hasRef(t *testing.T, dir, ref string) bool {
	t.Helper()
	return runGit(t, dir, "", nil, "for-each-ref", "--format=%(refname)", ref) == ref
}

// StaticHeadKeepers matches exactly and never authorizes an empty ID.
func TestStaticHeadKeepers(t *testing.T) {
	s := StaticHeadKeepers{"alice", "carol"}
	ctx := context.Background()
	if err := s.AuthorizeHeadKeeper(ctx, "alice"); err != nil {
		t.Errorf("alice: %v", err)
	}
	for _, id := range []string{"bob", "", "Alice", "alice "} {
		if err := s.AuthorizeHeadKeeper(ctx, id); !errors.Is(err, ErrNotHeadKeeper) {
			t.Errorf("%q: got %v; want ErrNotHeadKeeper", id, err)
		}
	}
	if err := (StaticHeadKeepers{""}).AuthorizeHeadKeeper(ctx, ""); !errors.Is(err, ErrNotHeadKeeper) {
		t.Errorf("empty list entry authorized empty id: %v", err)
	}
	if err := StaticHeadKeepers(nil).AuthorizeHeadKeeper(ctx, "alice"); !errors.Is(err, ErrNotHeadKeeper) {
		t.Errorf("nil list: %v", err)
	}
}

// Test 1: fail closed. A nil or refusing authorizer returns ErrNotHeadKeeper
// and touches nothing.
func TestForceReleaseFailClosed(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	ctx := context.Background()
	if err := newBackend(t, dir, "", clock).AcquireLease(ctx, mkLease("T-001", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	ref := "refs/waggle/claims/ticket-1"
	oid := refOID(t, dir, ref)

	cases := []struct {
		name string
		b    *GitBackend
		id   string
	}{
		{"nil authorizer", hkBackend(t, dir, "", clock), headKeeper},
		{"bob", hkBackend(t, dir, "", clock, "alice"), "bob"},
		{"Alice", hkBackend(t, dir, "", clock, "alice"), "Alice"},
		{"empty list", hkBackend(t, dir, "", clock, []string{}...), "alice"},
	}
	for _, c := range cases {
		req := forceReq(ref, oid)
		req.HeadKeeperID = c.id
		if err := c.b.ForceRelease(ctx, req); !errors.Is(err, ErrNotHeadKeeper) {
			t.Errorf("%s: got %v; want ErrNotHeadKeeper", c.name, err)
		}
		if got := refOID(t, dir, ref); got != oid {
			t.Errorf("%s: ref moved to %s", c.name, got)
		}
		if hasRef(t, dir, ForceReleaseAuditRef) {
			t.Errorf("%s: audit ref created", c.name)
		}
	}
	// An authorizer error that is not ErrNotHeadKeeper is still reported as one.
	odd := hkBackend(t, dir, "", clock)
	odd.headKeeper = authorizerFunc(func(context.Context, string) error { return errors.New("boom") })
	if err := odd.ForceRelease(ctx, forceReq(ref, oid)); !errors.Is(err, ErrNotHeadKeeper) || !strings.Contains(err.Error(), "boom") {
		t.Errorf("custom error: got %v", err)
	}
}

type authorizerFunc func(context.Context, string) error

func (f authorizerFunc) AuthorizeHeadKeeper(ctx context.Context, id string) error { return f(ctx, id) }

// Test 2: request validation, pure part.
func TestValidateForceRelease(t *testing.T) {
	oid := strings.Repeat("a", 40)
	good := ForceRelease{Ref: "refs/waggle/claims/ticket-7", ExpectedOID: oid, HeadKeeperID: headKeeper, Reason: "why"}
	if err := validateForceRelease(good, 40); err != nil {
		t.Fatalf("good request rejected: %v", err)
	}
	with := func(f func(r *ForceRelease)) ForceRelease { r := good; f(&r); return r }
	cases := map[string]ForceRelease{
		"empty ref":          with(func(r *ForceRelease) { r.Ref = "" }),
		"heads ref":          with(func(r *ForceRelease) { r.Ref = "refs/heads/main" }),
		"prefix only":        with(func(r *ForceRelease) { r.Ref = "refs/waggle/claims/" }),
		"traversal":          with(func(r *ForceRelease) { r.Ref = "refs/waggle/claims/../heads/x" }),
		"dotdot":             with(func(r *ForceRelease) { r.Ref = "refs/waggle/claims/a..b" }),
		"dash":               with(func(r *ForceRelease) { r.Ref = "refs/waggle/claims/-x" }),
		"lock":               with(func(r *ForceRelease) { r.Ref = "refs/waggle/claims/ticket-1.lock" }),
		"trailing dot":       with(func(r *ForceRelease) { r.Ref = "refs/waggle/claims/ticket-1." }),
		"nested":             with(func(r *ForceRelease) { r.Ref = "refs/waggle/claims/a/b" }),
		"space":              with(func(r *ForceRelease) { r.Ref = "refs/waggle/claims/a b" }),
		"long suffix":        with(func(r *ForceRelease) { r.Ref = ClaimRefPrefix + strings.Repeat("x", 65) }),
		"audit ref":          with(func(r *ForceRelease) { r.Ref = ForceReleaseAuditRef }),
		"empty oid":          with(func(r *ForceRelease) { r.ExpectedOID = "" }),
		"short oid":          with(func(r *ForceRelease) { r.ExpectedOID = oid[:39] }),
		"sha256 oid in sha1": with(func(r *ForceRelease) { r.ExpectedOID = strings.Repeat("a", 64) }),
		"upper oid":          with(func(r *ForceRelease) { r.ExpectedOID = strings.ToUpper(oid) }),
		"flag oid":           with(func(r *ForceRelease) { r.ExpectedOID = "-" + oid[1:] }),
		"empty reason":       with(func(r *ForceRelease) { r.Reason = "" }),
		"blank reason":       with(func(r *ForceRelease) { r.Reason = " \n " }),
		"513 bytes":          with(func(r *ForceRelease) { r.Reason = strings.Repeat("r", 513) }),
		"nul":                with(func(r *ForceRelease) { r.Reason = "a\x00b" }),
		"cr":                 with(func(r *ForceRelease) { r.Reason = "a\r\nb" }),
		"escape":             with(func(r *ForceRelease) { r.Reason = "a\x1bb" }),
		"del":                with(func(r *ForceRelease) { r.Reason = "a\x7fb" }),
		"bad utf8":           with(func(r *ForceRelease) { r.Reason = "a\xffb" }),
		"empty keeper":       with(func(r *ForceRelease) { r.HeadKeeperID = "" }),
		"flag keeper":        with(func(r *ForceRelease) { r.HeadKeeperID = "-x" }),
		"space keeper":       with(func(r *ForceRelease) { r.HeadKeeperID = "a b" }),
	}
	for name, req := range cases {
		if err := validateForceRelease(req, 40); !errors.Is(err, ErrInvalidForceRelease) {
			t.Errorf("%s: got %v; want ErrInvalidForceRelease", name, err)
		}
	}
	ok := with(func(r *ForceRelease) { r.Reason = strings.Repeat("r", 511) + "\n" })
	if err := validateForceRelease(ok, 40); err != nil {
		t.Errorf("512-byte reason with newline rejected: %v", err)
	}
	if err := validateForceRelease(with(func(r *ForceRelease) { r.ExpectedOID = strings.Repeat("a", 64) }), 64); err != nil {
		t.Errorf("sha256 oid in sha256 repo rejected: %v", err)
	}
}

func TestSanitizeReason(t *testing.T) {
	if got := sanitizeReason("  a\x00b\x1b\r\nc\n "); got != "ab\nc" {
		t.Errorf("sanitizeReason = %q", got)
	}
}

// Test 2, git-backed: validation runs before authorization and writes
// nothing, and git check-ref-format rejects what the regex alone allows.
func TestForceReleaseValidationGit(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	ctx := context.Background()
	if err := newBackend(t, dir, "", clock).AcquireLease(ctx, mkLease("T-001", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	oid := refOID(t, dir, "refs/waggle/claims/ticket-1")
	b := hkBackend(t, dir, "", clock) // nil authorizer: a validation failure must win
	for _, ref := range []string{"refs/waggle/claims/.hidden", "refs/waggle/claims/ticket-1.lock", "refs/heads/main", ""} {
		if err := b.ForceRelease(ctx, forceReq(ref, oid)); !errors.Is(err, ErrInvalidForceRelease) {
			t.Errorf("ref %q: got %v; want ErrInvalidForceRelease", ref, err)
		}
		if _, err := b.RefOID(ctx, ref); !errors.Is(err, ErrInvalidForceRelease) {
			t.Errorf("RefOID(%q): got %v; want ErrInvalidForceRelease", ref, err)
		}
	}
	bad := forceReq("refs/waggle/claims/ticket-1", "")
	if err := b.ForceRelease(ctx, bad); !errors.Is(err, ErrInvalidForceRelease) {
		t.Errorf("empty oid: got %v", err)
	}
	if got := refOID(t, dir, "refs/waggle/claims/ticket-1"); got != oid || hasRef(t, dir, ForceReleaseAuditRef) {
		t.Errorf("refs touched: claim=%s audit=%v", got, hasRef(t, dir, ForceReleaseAuditRef))
	}
}

// Test 3: active claim, local mode.
func TestForceReleaseActiveLocal(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statusBefore := runGit(t, dir, "", nil, "status", "--porcelain")
	_, idxErr := os.Stat(filepath.Join(dir, ".git", "index"))
	indexBefore := idxErr == nil

	clock := newClock()
	ctx := context.Background()
	alice := newBackend(t, dir, "", clock)
	if err := alice.AcquireLease(ctx, mkLease("T-001", "alice", "internal/comb/")); err != nil {
		t.Fatal(err)
	}
	b := hkBackend(t, dir, "", clock, headKeeper)
	ref, _ := RefName("T-001")
	oid, err := b.RefOID(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := b.ClaimOID(ctx, "T-001"); got != oid {
		t.Fatalf("RefOID %s != ClaimOID %s", oid, got)
	}
	clock.Advance(time.Minute)
	if err := b.ForceRelease(ctx, forceReq(ref, oid)); err != nil {
		t.Fatalf("ForceRelease: %v", err)
	}
	if claimRefs(t, dir) != "" {
		t.Errorf("claim ref remains: %s", claimRefs(t, dir))
	}
	chain := auditChain(t, dir)
	if len(chain) != 1 {
		t.Fatalf("audit chain = %v; want one entry", chain)
	}
	if ps := parents(t, dir, chain[0]); len(ps) != 2 || ps[1] != oid {
		t.Errorf("audit parents = %v; want [root %s]", ps, oid)
	}
	rec := readAudit(t, dir, chain[0])
	want := auditRecord{Version: 1, Ref: ref, ReleasedOID: oid, ReleasedType: "commit", HeadKeeperID: headKeeper,
		Reason: "keeper unreachable since 2026-10-12", ReleasedAt: t0.Add(time.Minute)}
	want.Lease = &Lease{Version: LeaseVersion, TicketID: "T-001", KeeperID: "alice", Role: "builder", Gate: "full",
		Directories: []string{"internal/comb"}, ClaimedAt: t0, ExpiresAt: t0.Add(DefaultLeaseTTL)}
	if !reflect.DeepEqual(rec, want) {
		t.Errorf("record = %+v (lease %+v)\nwant %+v (lease %+v)", rec, rec.Lease, want, want.Lease)
	}
	if tree := runGit(t, dir, "", nil, "ls-tree", "--name-only", chain[0]); tree != auditRecordFile {
		t.Errorf("audit tree = %q", tree)
	}
	commit := runGit(t, dir, "", nil, "cat-file", "-p", chain[0])
	if !strings.Contains(commit, "author waggle <waggle@localhost> 1792245660 +0000") ||
		!strings.HasSuffix(commit, "waggle: force-release "+ref+" by "+headKeeper+"\n\nkeeper unreachable since 2026-10-12") {
		t.Errorf("unexpected commit:\n%s", commit)
	}
	if after := runGit(t, dir, "", nil, "status", "--porcelain"); after != statusBefore {
		t.Errorf("git status changed: %q -> %q", statusBefore, after)
	}
	_, idxErr = os.Stat(filepath.Join(dir, ".git", "index"))
	if (idxErr == nil) != indexBefore {
		t.Errorf("index existence changed")
	}
	if _, err := b.RefOID(ctx, ref); !errors.Is(err, ErrNotFound) {
		t.Errorf("RefOID after release: %v", err)
	}
	if err := alice.AcquireLease(ctx, mkLease("T-001", "bob", "internal/comb/")); err != nil {
		t.Errorf("re-claim after force-release: %v", err)
	}
}

// Test 4: an expired claim with a renewal chain stays reachable from the audit ref.
func TestForceReleaseExpiredChain(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	ctx := context.Background()
	alice := newBackend(t, dir, "", clock)
	if err := alice.AcquireLease(ctx, mkLease("T-002", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	first := refOID(t, dir, "refs/waggle/claims/ticket-2")
	clock.Advance(time.Hour)
	if err := alice.AcquireLease(ctx, mkLease("T-002", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	second := refOID(t, dir, "refs/waggle/claims/ticket-2")
	clock.Advance(3 * time.Hour)
	if leases, err := alice.ListActiveLeases(ctx); err != nil || len(leases) != 0 {
		t.Fatalf("claim should be expired: %v, %v", leases, err)
	}
	b := hkBackend(t, dir, "", clock, headKeeper)
	if err := b.ForceRelease(ctx, forceReq("refs/waggle/claims/ticket-2", second)); err != nil {
		t.Fatal(err)
	}
	reach := strings.Fields(runGit(t, dir, "", nil, "rev-list", ForceReleaseAuditRef))
	for _, oid := range []string{first, second} {
		found := false
		for _, r := range reach {
			found = found || r == oid
		}
		if !found {
			t.Errorf("claim commit %s not reachable from audit ref: %v", oid, reach)
		}
	}
	if len(reach) != 4 {
		t.Errorf("rev-list = %v; want root, audit entry and two claim commits", reach)
	}
}

// Test 5: every malformed shape from T-002 test 11 can be cleared.
func TestForceReleaseMalformed(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
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
		return Lease{Version: LeaseVersion, TicketID: ticket, KeeperID: "mallory", Role: "builder", Gate: "full",
			Directories: []string{"shared"}, ClaimedAt: t0, ExpiresAt: t0.Add(time.Hour)}
	}
	set := func(ref, oid string) { runGit(t, dir, "", nil, "update-ref", ref, oid) }
	rawBlob := blobOf("raw claim content")
	set("refs/waggle/claims/ticket-1", rawBlob)
	set("refs/waggle/claims/ticket-2", commitWith("lease.json", "{not json"))
	set("refs/waggle/claims/ticket-3", commitWith("other.txt", "hi"))
	set("refs/waggle/claims/ticket-4", commitWith("lease.json", leaseJSON(good("T-005"))))
	set("refs/waggle/claims/ticket-007", commitWith("lease.json", leaseJSON(good("T-007"))))

	b := hkBackend(t, dir, "", clock, headKeeper)
	if _, err := b.ListActiveLeases(ctx); !errors.Is(err, ErrMalformedClaim) {
		t.Fatalf("setup: %v", err)
	}
	cases := []struct {
		ref, typ, reason string
	}{
		{"refs/waggle/claims/ticket-1", "blob", "points to a blob, not a commit"},
		{"refs/waggle/claims/ticket-2", "commit", "lease.json is not valid JSON"},
		{"refs/waggle/claims/ticket-3", "commit", "tree has no lease.json"},
		{"refs/waggle/claims/ticket-4", "commit", `lease ticket_id "T-005" does not match ref`},
		{"refs/waggle/claims/ticket-007", "commit", "ref name is not canonical ticket-<n>"},
	}
	for i, c := range cases {
		oid, err := b.RefOID(ctx, c.ref)
		if err != nil {
			t.Fatalf("RefOID(%s): %v", c.ref, err)
		}
		if err := b.ForceRelease(ctx, forceReq(c.ref, oid)); err != nil {
			t.Fatalf("ForceRelease(%s): %v", c.ref, err)
		}
		if hasRef(t, dir, c.ref) {
			t.Errorf("%s still exists", c.ref)
		}
		chain := auditChain(t, dir)
		if len(chain) != i+1 {
			t.Fatalf("%s: audit chain %v; want %d entries", c.ref, chain, i+1)
		}
		rec := readAudit(t, dir, chain[0])
		if rec.Ref != c.ref || rec.ReleasedOID != oid || rec.ReleasedType != c.typ || rec.Lease != nil ||
			!strings.HasPrefix(rec.MalformedReason, c.reason) {
			t.Errorf("%s: record = %+v", c.ref, rec)
		}
		ps := parents(t, dir, chain[0])
		switch {
		case c.typ == "commit" && (len(ps) != 2 || ps[1] != oid):
			t.Errorf("%s: parents = %v; want [head %s]", c.ref, ps, oid)
		case c.typ != "commit" && len(ps) != 1:
			t.Errorf("%s: parents = %v; want only the previous head", c.ref, ps)
		}
		if i > 0 && ps[0] != chain[1] {
			t.Errorf("%s: first parent %s; want previous entry %s", c.ref, ps[0], chain[1])
		}
		names := runGit(t, dir, "", nil, "ls-tree", "--name-only", chain[0])
		if c.typ == "blob" {
			if names != auditRecordFile+"\n"+auditRawFile {
				t.Errorf("%s: tree = %q", c.ref, names)
			}
			if got := runGit(t, dir, "", nil, "rev-parse", chain[0]+":"+auditRawFile); got != rawBlob {
				t.Errorf("%s: %s = %s; want %s", c.ref, auditRawFile, got, rawBlob)
			}
		} else if names != auditRecordFile {
			t.Errorf("%s: tree = %q", c.ref, names)
		}
	}
	if leases, err := b.ListActiveLeases(ctx); err != nil || len(leases) != 0 {
		t.Errorf("ListActiveLeases after clearing: %v, %v", leases, err)
	}
	for _, ticket := range []string{"T-001", "T-002", "T-003", "T-004", "T-007"} {
		if err := b.AcquireLease(ctx, mkLease(ticket, "alice", "elsewhere/"+ticket)); err != nil {
			t.Errorf("AcquireLease(%s) after force-release: %v", ticket, err)
		}
	}
}

// Test 6: compare-and-swap on the inspected OID.
func TestForceReleaseCAS(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	ctx := context.Background()
	alice := newBackend(t, dir, "", clock)
	if err := alice.AcquireLease(ctx, mkLease("T-003", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	ref := "refs/waggle/claims/ticket-3"
	oid := refOID(t, dir, ref)
	b := hkBackend(t, dir, "", clock, headKeeper)
	wrong := strings.Repeat("1", len(oid))
	if err := b.ForceRelease(ctx, forceReq(ref, wrong)); !errors.Is(err, ErrConcurrentUpdate) {
		t.Errorf("wrong oid: got %v; want ErrConcurrentUpdate", err)
	}
	if err := b.ForceRelease(ctx, forceReq("refs/waggle/claims/ticket-4", oid)); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing ref: got %v; want ErrNotFound", err)
	}
	if got := refOID(t, dir, ref); got != oid || hasRef(t, dir, ForceReleaseAuditRef) {
		t.Fatalf("refs touched: claim=%s audit=%v", got, hasRef(t, dir, ForceReleaseAuditRef))
	}
	// The owner renews between inspect and write.
	b.hookBeforeWrite = func() {
		if err := alice.AcquireLease(ctx, mkLease("T-003", "alice", "pkg")); err != nil {
			panic(err)
		}
	}
	if err := b.ForceRelease(ctx, forceReq(ref, oid)); !errors.Is(err, ErrConcurrentUpdate) {
		t.Errorf("renewed under us: got %v; want ErrConcurrentUpdate", err)
	}
	if !hasRef(t, dir, ref) {
		t.Error("claim deleted despite concurrent renewal")
	}
	if hasRef(t, dir, ForceReleaseAuditRef) {
		t.Error("audit entry written without a delete")
	}
}

// Test 7: three force-releases produce three first-parent commits in order.
func TestForceReleaseAuditChain(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	ctx := context.Background()
	alice := newBackend(t, dir, "", clock)
	b := hkBackend(t, dir, "", clock, headKeeper)
	var released []string
	for i := 1; i <= 3; i++ {
		id := ticketDisplay(i)
		if err := alice.AcquireLease(ctx, mkLease(id, "alice", "pkg/"+id)); err != nil {
			t.Fatal(err)
		}
		ref, _ := RefName(id)
		oid := refOID(t, dir, ref)
		released = append(released, oid)
		clock.Advance(time.Minute)
		req := forceReq(ref, oid)
		req.Reason = "entry " + id
		if err := b.ForceRelease(ctx, req); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	chain := auditChain(t, dir)
	if len(chain) != 3 {
		t.Fatalf("chain = %v", chain)
	}
	for i, c := range chain {
		n := 3 - i
		rec := readAudit(t, dir, c)
		want, _ := RefName(ticketDisplay(n))
		if rec.Ref != want || rec.Reason != "entry "+ticketDisplay(n) || rec.ReleasedOID != released[n-1] ||
			!rec.ReleasedAt.Equal(t0.Add(time.Duration(n)*time.Minute)) {
			t.Errorf("entry %d: %+v", n, rec)
		}
		ps := parents(t, dir, c)
		if len(ps) != 2 || ps[1] != released[n-1] || (n > 1 && ps[0] != chain[i+1]) {
			t.Errorf("entry %d parents = %v; want [previous head %s]", n, ps, released[n-1])
		}
	}
	// Audit entries are not claims: the claim namespace is empty and clean.
	if leases, err := b.ListActiveLeases(ctx); err != nil || len(leases) != 0 {
		t.Errorf("ListActiveLeases = %v, %v", leases, err)
	}
}

// auditBehindBack appends a foreign commit to the audit ref with raw git.
func auditBehindBack(t *testing.T, dir string) string {
	t.Helper()
	idEnv := []string{"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	// The foreign entry carries a force-release.json so auditChain accepts it,
	// and chains on the current head or on a root of its own.
	blob := runGit(t, dir, "{}\n", nil, "hash-object", "-w", "--stdin")
	tree := runGit(t, dir, "100644 blob "+blob+"\t"+auditRecordFile+"\n", nil, "mktree")
	parent := ""
	if hasRef(t, dir, ForceReleaseAuditRef) {
		parent = refOID(t, dir, ForceReleaseAuditRef)
	} else {
		empty := runGit(t, dir, "", nil, "mktree")
		parent = runGit(t, dir, "", idEnv, "commit-tree", empty, "-m", "waggle: audit root")
	}
	oid := runGit(t, dir, "", idEnv, "commit-tree", tree, "-p", parent, "-m", "foreign")
	runGit(t, dir, "", nil, "update-ref", ForceReleaseAuditRef, oid)
	return oid
}

// Test 8: local atomicity against a stale audit head.
func TestForceReleaseLocalAtomicity(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	clock := newClock()
	ctx := context.Background()
	alice := newBackend(t, dir, "", clock)
	for _, id := range []string{"T-001", "T-002"} {
		if err := alice.AcquireLease(ctx, mkLease(id, "alice", "pkg/"+id)); err != nil {
			t.Fatal(err)
		}
	}

	// One stale read: the first transaction fails, the retry rebuilds on the
	// new head and succeeds with exactly one new entry.
	b := hkBackend(t, dir, "", clock, headKeeper)
	var foreign string
	writes, retries := 0, 0
	b.hookBeforeWrite = func() {
		writes++
		if writes == 1 {
			foreign = auditBehindBack(t, dir)
		}
	}
	b.hookBeforeRetry = func(int) { retries++ }
	oid1 := refOID(t, dir, "refs/waggle/claims/ticket-1")
	if err := b.ForceRelease(ctx, forceReq("refs/waggle/claims/ticket-1", oid1)); err != nil {
		t.Fatalf("ForceRelease: %v", err)
	}
	if writes != 2 || retries != 1 {
		t.Errorf("writes=%d retries=%d; want 2 and 1", writes, retries)
	}
	if hasRef(t, dir, "refs/waggle/claims/ticket-1") {
		t.Error("claim remains after successful retry")
	}
	chain := auditChain(t, dir)
	if len(chain) != 2 || chain[1] != foreign {
		t.Fatalf("chain = %v; want [new %s]", chain, foreign)
	}
	if ps := parents(t, dir, chain[0]); len(ps) != 2 || ps[0] != foreign || ps[1] != oid1 {
		t.Errorf("parents = %v; want [%s %s]", ps, foreign, oid1)
	}

	// The audit head moves on every attempt: the transaction never applies,
	// the claim survives and no entry of ours is written.
	c := hkBackend(t, dir, "", clock, headKeeper)
	writes, retries = 0, 0
	c.hookBeforeWrite = func() { writes++; auditBehindBack(t, dir) }
	c.hookBeforeRetry = func(int) { retries++ }
	oid2 := refOID(t, dir, "refs/waggle/claims/ticket-2")
	err := c.ForceRelease(ctx, forceReq("refs/waggle/claims/ticket-2", oid2))
	if !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("got %v; want ErrConcurrentUpdate", err)
	}
	if writes != maxCASRetries+1 || retries != maxCASRetries {
		t.Errorf("writes=%d retries=%d", writes, retries)
	}
	if got := refOID(t, dir, "refs/waggle/claims/ticket-2"); got != oid2 {
		t.Errorf("claim moved to %s", got)
	}
	for _, c := range auditChain(t, dir) {
		if msg := runGit(t, dir, "", nil, "log", "-1", "--format=%s", c); strings.Contains(msg, "ticket-2") {
			t.Errorf("audit entry for ticket-2 written without its delete: %s", c)
		}
	}
}

// Test 8b: a stale lock on the audit ref is retried like T-002 lock contention.
func TestForceReleaseLocalLockContention(t *testing.T) {
	requireGit(t)
	dir := newFilesRepo(t)
	clock := newClock()
	ctx := context.Background()
	if err := newBackend(t, dir, "", clock).AcquireLease(ctx, mkLease("T-001", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	b := hkBackend(t, dir, "", clock, headKeeper)
	gitDir := runGit(t, dir, "", nil, "rev-parse", "--absolute-git-dir")
	lock := filepath.Join(gitDir, filepath.FromSlash(ForceReleaseAuditRef)+".lock")
	t.Cleanup(func() { _ = os.Remove(lock) })
	retries, writes := 0, 0
	b.hookBeforeWrite = func() {
		writes++
		if writes > 1 {
			return
		}
		if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
			panic(err)
		}
		if err := os.WriteFile(lock, nil, 0o644); err != nil {
			panic(err)
		}
	}
	b.hookBeforeRetry = func(int) {
		retries++
		if err := os.Remove(lock); err != nil {
			panic(err)
		}
	}
	oid := refOID(t, dir, "refs/waggle/claims/ticket-1")
	if err := b.ForceRelease(ctx, forceReq("refs/waggle/claims/ticket-1", oid)); err != nil {
		t.Fatalf("ForceRelease: %v", err)
	}
	if retries != 1 || hasRef(t, dir, "refs/waggle/claims/ticket-1") || len(auditChain(t, dir)) != 1 {
		t.Errorf("retries=%d claim=%v chain=%v", retries, hasRef(t, dir, "refs/waggle/claims/ticket-1"), auditChain(t, dir))
	}
}

// Test 9: remote force-release from another clone.
func TestForceReleaseRemote(t *testing.T) {
	requireGit(t)
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ctx := context.Background()
	ba := newBackend(t, dirA, "origin", clock)
	bb := hkBackend(t, dirB, "origin", clock, headKeeper)
	if err := ba.AcquireLease(ctx, mkLease("T-020", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	ref := "refs/waggle/claims/ticket-20"
	claimOID := refOID(t, origin, ref)
	oid, err := bb.RefOID(ctx, ref)
	if err != nil || oid != claimOID {
		t.Fatalf("RefOID = %s, %v; origin has %s", oid, err, claimOID)
	}
	if err := bb.ForceRelease(ctx, forceReq(ref, oid)); err != nil {
		t.Fatalf("ForceRelease: %v", err)
	}
	if claimRefs(t, origin) != "" {
		t.Errorf("origin claim refs: %s", claimRefs(t, origin))
	}
	chain := auditChain(t, origin)
	if len(chain) != 1 {
		t.Fatalf("origin audit chain = %v", chain)
	}
	if ps := parents(t, origin, chain[0]); len(ps) != 2 || ps[1] != claimOID {
		t.Errorf("origin audit parents = %v; want [root %s]", ps, claimOID)
	}
	if rec := readAudit(t, origin, chain[0]); rec.Lease == nil || rec.Lease.KeeperID != "alice" {
		t.Errorf("origin record = %+v", rec)
	}
	// Both local mirrors in B match the remote.
	if hasRef(t, dirB, ref) || auditChain(t, dirB)[0] != chain[0] {
		t.Errorf("B mirror: claim=%v audit=%v", hasRef(t, dirB, ref), auditChain(t, dirB))
	}
	// A's sync neither prunes nor fails on the audit ref.
	runGit(t, dirA, "", nil, "fetch", "--quiet", "origin", "+"+ForceReleaseAuditRef+":"+ForceReleaseAuditRef)
	leases, err := ba.ListActiveLeases(ctx)
	if err != nil || len(leases) != 0 {
		t.Errorf("A ListActiveLeases = %v, %v", leases, err)
	}
	if hasRef(t, dirA, ref) || !hasRef(t, dirA, ForceReleaseAuditRef) {
		t.Errorf("A after sync: claim=%v audit=%v", hasRef(t, dirA, ref), hasRef(t, dirA, ForceReleaseAuditRef))
	}
	// Second release of the same ref: gone.
	if err := bb.ForceRelease(ctx, forceReq(ref, oid)); !errors.Is(err, ErrNotFound) {
		t.Errorf("second release: %v", err)
	}
}

// Test 10: remote race between a renewal and a force-release.
func TestForceReleaseRemoteRace(t *testing.T) {
	requireGit(t)
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ctx := context.Background()
	ba := newBackend(t, dirA, "origin", clock)
	bb := hkBackend(t, dirB, "origin", clock, headKeeper)
	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("T-%03d", 20+i)
		ref, _ := RefName(id)
		if err := ba.AcquireLease(ctx, mkLease(id, "alice", "race/"+id)); err != nil {
			t.Fatal(err)
		}
		oid, err := bb.RefOID(ctx, ref)
		if err != nil {
			t.Fatal(err)
		}
		clock.Advance(time.Second)
		start := make(chan struct{})
		var renewErr, forceErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; renewErr = ba.AcquireLease(ctx, mkLease(id, "alice", "race/"+id)) }()
		go func() { defer wg.Done(); <-start; forceErr = bb.ForceRelease(ctx, forceReq(ref, oid)) }()
		close(start)
		wg.Wait()
		switch {
		case renewErr == nil && errors.Is(forceErr, ErrConcurrentUpdate):
			if !hasRef(t, origin, ref) || refOID(t, origin, ref) == oid {
				t.Errorf("%s: renewal won but origin ref is not renewed", id)
			}
		case forceErr == nil && errors.Is(renewErr, ErrAlreadyClaimed):
			if hasRef(t, origin, ref) {
				t.Errorf("%s: force-release won but origin ref exists", id)
			}
			if rec := readAudit(t, origin, auditChain(t, origin)[0]); rec.ReleasedOID != oid {
				t.Errorf("%s: audit records %s; want %s", id, rec.ReleasedOID, oid)
			}
		default:
			t.Fatalf("%s: renew=%v force=%v; want exactly one winner", id, renewErr, forceErr)
		}
		// Leave each round clean for the next.
		if hasRef(t, origin, ref) {
			cur, _ := bb.RefOID(ctx, ref)
			if err := bb.ForceRelease(ctx, forceReq(ref, cur)); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// Test 11: concurrent force-releases on different tickets both succeed
// through the audit retry path.
func TestForceReleaseRemoteConcurrentAudit(t *testing.T) {
	requireGit(t)
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ctx := context.Background()
	owner := newBackend(t, dirA, "origin", clock)
	ba := hkBackend(t, dirA, "origin", clock, headKeeper)
	bb := hkBackend(t, dirB, "origin", clock, headKeeper)
	for _, id := range []string{"T-050", "T-051"} {
		if err := owner.AcquireLease(ctx, mkLease(id, "alice", "pkg/"+id)); err != nil {
			t.Fatal(err)
		}
	}
	refA, refB := "refs/waggle/claims/ticket-50", "refs/waggle/claims/ticket-51"
	oidA, oidB := refOID(t, origin, refA), refOID(t, origin, refB)
	if _, err := bb.RefOID(ctx, refB); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var errA, errB error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); <-start; errA = ba.ForceRelease(ctx, forceReq(refA, oidA)) }()
	go func() { defer wg.Done(); <-start; errB = bb.ForceRelease(ctx, forceReq(refB, oidB)) }()
	close(start)
	wg.Wait()
	if errA != nil || errB != nil {
		t.Fatalf("A=%v B=%v", errA, errB)
	}
	if claimRefs(t, origin) != "" {
		t.Errorf("origin claim refs: %s", claimRefs(t, origin))
	}
	chain := auditChain(t, origin)
	if len(chain) != 2 {
		t.Fatalf("origin chain = %v", chain)
	}
	got := map[string]bool{}
	for _, c := range chain {
		got[readAudit(t, origin, c).Ref] = true
	}
	if !got[refA] || !got[refB] {
		t.Errorf("audit refs = %v", got)
	}
	if ps := parents(t, origin, chain[0]); len(ps) != 2 || ps[0] != chain[1] {
		t.Errorf("newest entry parents = %v; want first parent %s", ps, chain[1])
	}
}

// A rejected push with both remote refs unchanged (a hook) is retried, then
// reported as a git error, never as a concurrency sentinel.
func TestForceReleaseRemoteRejectedUnchanged(t *testing.T) {
	requireGit(t)
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ctx := context.Background()
	if err := newBackend(t, dirA, "origin", clock).AcquireLease(ctx, mkLease("T-001", "alice", "pkg")); err != nil {
		t.Fatal(err)
	}
	ref := "refs/waggle/claims/ticket-1"
	oid := refOID(t, origin, ref)
	bb := hkBackend(t, dirB, "origin", clock, headKeeper)
	retries := rejectHooks(t, bb, origin, false)
	err := bb.ForceRelease(ctx, forceReq(ref, oid))
	if err == nil || errors.Is(err, ErrConcurrentUpdate) || !strings.Contains(err.Error(), "refs unchanged") {
		t.Fatalf("got %v; want unchanged-refs push error", err)
	}
	if *retries != maxCASRetries {
		t.Errorf("retries = %d; want %d", *retries, maxCASRetries)
	}
	if refOID(t, origin, ref) != oid || hasRef(t, origin, ForceReleaseAuditRef) {
		t.Error("origin changed")
	}
	// Transport failure: no porcelain line, a wrapped git error.
	runGit(t, dirB, "", nil, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing"))
	err = bb.ForceRelease(ctx, forceReq(ref, oid))
	var ge *gitError
	if !errors.As(err, &ge) || errors.Is(err, ErrConcurrentUpdate) || errors.Is(err, ErrNotFound) {
		t.Errorf("transport failure: %v", err)
	}
}

// Test 13: Backend is unchanged. A type with exactly the three T-002 methods
// still satisfies it, and force-release lives on *GitBackend only.
type threeMethodBackend struct{}

func (threeMethodBackend) AcquireLease(context.Context, Lease) error          { return nil }
func (threeMethodBackend) ReleaseLease(context.Context, string, string) error { return nil }
func (threeMethodBackend) ListActiveLeases(context.Context) ([]Lease, error)  { return nil, nil }

var _ Backend = threeMethodBackend{}

func TestBackendSurfaceUnchanged(t *testing.T) {
	iface := reflect.TypeOf((*Backend)(nil)).Elem()
	want := []string{"AcquireLease", "ListActiveLeases", "ReleaseLease"}
	if iface.NumMethod() != len(want) {
		t.Fatalf("Backend has %d methods; want %d", iface.NumMethod(), len(want))
	}
	for i, name := range want {
		if iface.Method(i).Name != name {
			t.Errorf("method %d = %s; want %s", i, iface.Method(i).Name, name)
		}
	}
	if _, ok := iface.MethodByName("ForceRelease"); ok {
		t.Error("ForceRelease leaked into Backend")
	}
	if _, ok := reflect.TypeOf((*GitBackend)(nil)).MethodByName("ForceRelease"); !ok {
		t.Error("*GitBackend lacks ForceRelease")
	}
}
