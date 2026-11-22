// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/scaffold"
	"github.com/waggle-sh/waggle/internal/usage"
)

// fixedNow is the clock every status test runs at (T-022 section 5).
// dashes holds U+2013 and U+2014, which must never appear in output.
var fixedNow = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

const dashes = "\u2013\u2014"

// requireGit skips a git-backed test when git is absent (T-002 section 9.1).
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
}

// gitOut runs git in dir with stdin and returns its trimmed stdout.
func gitOut(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(append(os.Environ(), "LC_ALL=C"), gitEnv...)
	cmd.Stdin = strings.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, stderr.String())
	}
	return strings.TrimSpace(string(out))
}

// fixClock pins statusClock to fixedNow for the test.
func fixClock(t *testing.T) {
	t.Helper()
	statusClock = func() time.Time { return fixedNow }
	t.Cleanup(func() { statusClock = time.Now })
}

// runStatusT runs the command and fails on any U+2013 or U+2014 in its output
// (test 7), so every status test checks it.
func runStatusT(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = runStatus(args, &out, &errb)
	for _, s := range []string{out.String(), errb.String()} {
		if strings.ContainsAny(s, dashes) {
			t.Errorf("output contains an en or em dash: %q", s)
		}
	}
	return code, out.String(), errb.String()
}

// newHive creates a git repo named "hive" and runs waggle init in it with a
// fixed release pin. Runtime "none" keeps shims out of the tree.
func newHive(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "hive")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-q")
	err := scaffold.InitRepo(scaffold.InitOptions{
		TargetDir: dir, Profile: "cli", Delivery: "installed", Forge: "github", CombMode: "direct",
		Runtime: "none", Out: &bytes.Buffer{},
		BuildInfo: func() (*debug.BuildInfo, bool) { return &debug.BuildInfo{Main: debug.Module{Version: "v1.0.0"}}, true },
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// acquire writes a claim through the comb backend with the backend clock at now.
func acquire(t *testing.T, dir string, now time.Time, l comb.Lease) {
	t.Helper()
	b, err := comb.NewGitBackend(comb.Options{RepoDir: dir, Now: func() time.Time { return now }, ExtraEnv: gitEnv})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.AcquireLease(context.Background(), l); err != nil {
		t.Fatalf("acquire %s: %v", l.TicketID, err)
	}
}

// forgeClaim stores raw bytes as lease.json under a claim ref, bypassing validation.
func forgeClaim(t *testing.T, dir, ref, leaseJSON string) {
	t.Helper()
	blob := gitOut(t, dir, leaseJSON, "hash-object", "-w", "--stdin")
	tree := gitOut(t, dir, "100644 blob "+blob+"\tlease.json\n", "mktree")
	oid := gitOut(t, dir, "", "commit-tree", tree, "-m", "forged")
	gitOut(t, dir, "", "update-ref", ref, oid)
}

// twoLeases plants T-012 (1h42m left), T-016 (12m left) and an expired T-003.
func twoLeases(t *testing.T, dir string) {
	t.Helper()
	acquire(t, dir, fixedNow, comb.Lease{TicketID: "T-012", KeeperID: "paul", Role: "builder", Gate: "full",
		Directories: []string{"internal/runtime", "internal/config"}, ExpiresAt: fixedNow.Add(102*time.Minute + 30*time.Second)})
	acquire(t, dir, fixedNow, comb.Lease{TicketID: "T-016", KeeperID: "paul", Role: "builder", Gate: "full",
		Directories: []string{"internal/scaffold", "cmd/waggle"}, ExpiresAt: fixedNow.Add(12 * time.Minute)})
	acquire(t, dir, fixedNow.Add(-3*time.Hour), comb.Lease{TicketID: "T-003", KeeperID: "paul", Role: "guard", Gate: "light",
		Directories: []string{"docs"}, ExpiresAt: fixedNow.Add(-time.Hour)})
}

const freshGolden = `Repo: hive  pin v1.0.0  claim refs direct (local)
daemon: stopped (no .waggle/control/run.sock)

Claims (0 active):

Usage: none recorded (.waggle/control/usage.jsonl absent)
`

const claimsGolden = `Claims (2 active):
  T-012  builder  paul  full  internal/config/ internal/runtime/  expires in 1h42m
  T-016  builder  paul  full  cmd/waggle/ internal/scaffold/      expires in 12m
`

// Test 1: a fresh hive renders the golden report, run from inside the tree.
func TestStatusFreshHive(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := newHive(t)
	chdir(t, dir)
	code, out, errb := runStatusT(t)
	if code != 0 || errb != "" {
		t.Fatalf("code %d, stderr %q", code, errb)
	}
	if out != freshGolden {
		t.Errorf("stdout:\n%s\nwant:\n%s", out, freshGolden)
	}
}

// Test 2: active leases print sorted with expiry text; the expired one is absent.
func TestStatusClaims(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := newHive(t)
	twoLeases(t, dir)
	code, out, errb := runStatusT(t, "--repo", dir)
	if code != 0 || errb != "" {
		t.Fatalf("code %d, stderr %q", code, errb)
	}
	if !strings.Contains(out, claimsGolden) {
		t.Errorf("stdout:\n%s\nwant block:\n%s", out, claimsGolden)
	}
	if strings.Contains(out, "T-003") {
		t.Error("expired lease listed")
	}
}

// Test 3 and test 6: a malformed claim ref adds one line, the valid lease still
// prints, exit 0, and the run leaves the work tree, index and refs untouched.
func TestStatusMalformedAndReadOnly(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := newHive(t)
	twoLeases(t, dir)
	forgeClaim(t, dir, "refs/waggle/claims/ticket-7", "not json")
	snapshot := func() string {
		return gitOut(t, dir, "", "status", "--porcelain") + "\n" + gitOut(t, dir, "", "for-each-ref", "refs/waggle/")
	}
	before := snapshot()

	code, out, errb := runStatusT(t, "--repo", dir)
	if code != 0 || errb != "" {
		t.Fatalf("code %d, stderr %q", code, errb)
	}
	want := "  malformed: 1 (refs/waggle/claims/ticket-7: lease.json is not valid JSON"
	if !strings.Contains(out, claimsGolden+want) {
		t.Errorf("stdout:\n%s\nwant claims block followed by %q", out, want)
	}
	if after := snapshot(); after != before {
		t.Errorf("status changed the repo:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// Test 4: a socket file present reports waggled as running.
func TestStatusSocketPresent(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := newHive(t)
	if err := os.WriteFile(filepath.Join(dir, layout.Socket), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runStatusT(t, "--repo", dir)
	if code != 0 || !strings.Contains(out, "daemon: running (socket present)\n") {
		t.Errorf("code %d, stdout %q", code, out)
	}
}

// Test 5: refusals and usage errors.
func TestStatusRefused(t *testing.T) {
	requireGit(t)
	fixClock(t)
	if code, _, errb := runStatusT(t, "--bogus"); code != 2 || !strings.Contains(errb, "Usage: waggle status") {
		t.Errorf("bad flag: code %d, stderr %q", code, errb)
	}
	if code, _, errb := runStatusT(t, "extra"); code != 2 || !strings.Contains(errb, "unexpected argument") {
		t.Errorf("extra arg: code %d, stderr %q", code, errb)
	}
	if code, _, _ := runStatusT(t, "-h"); code != 0 {
		t.Errorf("help: code %d", code)
	}

	plain := t.TempDir()
	if code, _, errb := runStatusT(t, "--repo", plain); code != 3 || !strings.Contains(errb, "rev-parse") {
		t.Errorf("not a git repo: code %d, stderr %q", code, errb)
	}

	dir := t.TempDir()
	git(t, dir, "init", "-q")
	chdir(t, dir)
	wantHive := "not a Waggle repo: .waggle/control/config.yaml not found; run waggle init"
	if code, _, errb := runStatusT(t); code != 1 || !strings.Contains(errb, wantHive) {
		t.Errorf("no .waggle: code %d, stderr %q", code, errb)
	}

	if err := os.MkdirAll(filepath.Join(dir, layout.Root, "roles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code, _, errb := runStatusT(t, "--repo", dir); code != 1 || !strings.Contains(errb, "found .waggle/roles: scaffold: legacy waggle layout") {
		t.Errorf("legacy layout: code %d, stderr %q", code, errb)
	}

	if err := os.MkdirAll(filepath.Join(dir, layout.ControlDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, layout.Config), []byte("version: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errb := runStatusT(t, "--repo", dir); code != 1 || !strings.Contains(errb, "invalid .waggle/control/config.yaml") {
		t.Errorf("invalid config: code %d, stderr %q", code, errb)
	}
}

// Pure tests: the renderer with a remote, a 40-hex pin, an unsorted lease
// list and a socket; the pin scanner; and the expiry formatter.
func TestRenderStatusPure(t *testing.T) {
	h := header{Name: "waggle", Pin: "3f9c1e2a3f9c1e2a3f9c1e2a3f9c1e2a3f9c1e2a", Mode: "direct", Remote: "origin", Daemon: true}
	leases := []comb.Lease{
		{TicketID: "T-016", KeeperID: "paul", Role: "r2", Gate: "full", Directories: []string{"cmd/waggle"}, ExpiresAt: fixedNow.Add(59 * time.Second)},
		{TicketID: "T-002", KeeperID: "ann", Role: "r1", Gate: "light", Directories: []string{"a", "b/c"}, ExpiresAt: fixedNow.Add(2 * time.Hour)},
	}
	malformed := &comb.MalformedClaimsError{Claims: []comb.MalformedClaim{
		{Ref: "refs/waggle/claims/ticket-7", Reason: "tree has no lease.json"},
		{Ref: "refs/waggle/claims/ticket-9", Reason: "invalid object id"},
	}}
	want := `Repo: waggle  pin 3f9c1e2...  claim refs direct (origin)
daemon: running (socket present)

Claims (2 active):
  T-002  r1  ann   light  a/ b/c/      expires in 2h0m
  T-016  r2  paul  full   cmd/waggle/  expires in 0m
  malformed: 2 (refs/waggle/claims/ticket-7: tree has no lease.json); the first login in gate.approvers can clear it: waggle force-release --ref refs/waggle/claims/ticket-7

Usage: none recorded (.waggle/control/usage.jsonl absent)
`
	block := usage.Render(nil, layout.Usage)
	if got := string(renderStatus(h, leases, malformed, nil, block, fixedNow)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if strings.ContainsAny(want, dashes) {
		t.Error("golden contains an en or em dash")
	}
}

// Test 14: with a ledger present status prints the usage block and the footer;
// the stderr warning appears only when bad lines exist; the ledger is untouched.
func TestStatusUsageSection(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := newHive(t)
	path := filepath.Join(dir, layout.Usage)
	rec := `{"ts":"2026-10-17T08:00:00Z","ticket":"T-005","role":"r1","session":"s1","input_tokens":1000,"cache_read_tokens":500,"output_tokens":52381,"cost_usd":1.5,"end":"done"}` + "\n"
	if err := os.WriteFile(path, []byte(rec), 0o600); err != nil {
		t.Fatal(err)
	}

	code, out, errb := runStatusT(t, "--repo", dir)
	if code != 0 || errb != "" {
		t.Fatalf("code %d, stderr %q", code, errb)
	}
	want := "Usage (this machine, .waggle/control/usage.jsonl):\n" +
		"  T-005  r1 1 sess  in 2k (33% cached)  out 52k  est. $1.50\n" +
		"  total             in 2k (33% cached)  out 52k  est. $1.50\n" +
		"  Not counted: sessions a Keeper starts directly (today the queen and the planning role) and other Keepers' machines.\n"
	if !strings.HasSuffix(out, "\n"+want) {
		t.Errorf("stdout:\n%s\nwant suffix:\n%s", out, want)
	}

	bad := rec + "{oops\n"
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errb = runStatusT(t, "--repo", dir)
	if code != 0 || !strings.HasPrefix(errb, "warning: 1 unreadable ledger lines (first: line 2: ") || strings.Count(errb, "\n") != 1 {
		t.Errorf("code %d, stderr %q", code, errb)
	}
	if !strings.Contains(out, "T-005") {
		t.Errorf("good line not reported:\n%s", out)
	}
	if got, _ := os.ReadFile(path); string(got) != bad {
		t.Errorf("status changed the ledger: %q", got)
	}
}

func TestReadPin(t *testing.T) {
	cases := map[string]string{
		"version: 1\n\nwaggle:\n  pin: v1.2.3 # a comment\n\nhive:\n  profile: cli\n": "v1.2.3",
		"waggle:\n  pin: \"unpinned\"\n":                                              "unpinned",
		"waggle:\n  pin: 'abc'\n":                                                     "abc",
		"waggle:\n  pin: # nothing\n":                                                 "unpinned",
		"version: 1\nhive:\n  pin: nope\n":                                            "unpinned",
		"version: 1\n":                                                                "unpinned",
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	for in, want := range cases {
		if err := os.WriteFile(path, []byte(in), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := readPin(path); got != want {
			t.Errorf("readPin(%q) = %q, want %q", in, got, want)
		}
	}
	if got := readPin(filepath.Join(t.TempDir(), "missing.yaml")); got != "unpinned" {
		t.Errorf("readPin(missing) = %q, want unpinned", got)
	}
}

func TestExpiresIn(t *testing.T) {
	cases := map[time.Duration]string{
		102*time.Minute + 30*time.Second: "1h42m",
		12 * time.Minute:                 "12m",
		59 * time.Second:                 "0m",
		0:                                "0m",
		-time.Minute:                     "0m",
		2 * time.Hour:                    "2h0m",
		25 * time.Hour:                   "25h0m",
	}
	for d, want := range cases {
		if got := expiresIn(d); got != want {
			t.Errorf("expiresIn(%s) = %q, want %q", d, got, want)
		}
	}
}
