// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/layout"
)

// gitIn runs git with stdin and the harness environment.
func (r gateRepo) gitIn(stdin string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_AUTHOR_DATE=2026-10-17T00:00:00Z",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com", "GIT_COMMITTER_DATE=2026-10-17T00:00:00Z")
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// claim forges refs/waggle/claims/ticket-<n> with plumbing, as T-009 test 8
// does. expires is an RFC 3339 time; the far future means active.
func (r gateRepo) claim(n int, gate, expires string, dirs ...string) {
	r.t.Helper()
	lease := fmt.Sprintf(`{"version":2,"ticket_id":"T-%03d","keeper_id":"keeper","role":"forager",`+
		`"gate":%q,"directories":["%s"],"claimed_at":"2020-01-01T00:00:00Z","expires_at":%q}`,
		n, gate, strings.Join(dirs, `","`), expires)
	blob := r.gitIn(lease, "hash-object", "-w", "--stdin")
	tree := r.gitIn("100644 blob "+blob+"\tlease.json\n", "mktree")
	oid := r.git("commit-tree", tree, "-m", "claim")
	r.git("update-ref", fmt.Sprintf("refs/waggle/claims/ticket-%d", n), oid)
}

// claimOn is claim with a branch list, as waggle claim --branch writes it.
func (r gateRepo) claimOn(n int, branches []string, gate, expires string, dirs ...string) {
	r.t.Helper()
	lease := fmt.Sprintf(`{"version":2,"ticket_id":"T-%03d","keeper_id":"keeper","role":"forager",`+
		`"gate":%q,"directories":["%s"],"claimed_at":"2020-01-01T00:00:00Z","expires_at":%q,"branches":["%s"]}`,
		n, gate, strings.Join(dirs, `","`), expires, strings.Join(branches, `","`))
	blob := r.gitIn(lease, "hash-object", "-w", "--stdin")
	tree := r.gitIn("100644 blob "+blob+"\tlease.json\n", "mktree")
	oid := r.git("commit-tree", tree, "-m", "claim")
	r.git("update-ref", fmt.Sprintf("refs/waggle/claims/ticket-%d", n), oid)
}

const active = "2999-01-01T00:00:00Z"

// index stages path in the index only, so case variants and hostile names
// work on any filesystem.
func (r gateRepo) index(path string) {
	r.t.Helper()
	blob := r.gitIn("x\n", "hash-object", "-w", "--stdin")
	r.git("update-index", "--add", "--cacheinfo", "100644,"+blob+","+path)
}

// leaseRepo is main with a README and a decision log, then a branch.
func leaseRepo(t *testing.T) gateRepo {
	t.Helper()
	r := newGateRepo(t)
	r.write("README.md", []byte("hive\n"))
	r.write("other/x.md", []byte("x\n"))
	r.write(layout.Decisions, gateLog(1))
	r.commit("init")
	r.git("checkout", "-q", "-b", "branch")
	return r
}

func ghaEnv(k string) string {
	if k == "GITHUB_ACTIONS" {
		return "true"
	}
	return ""
}

// run runs a gate check on main..branch and returns the exit code and the
// combined output.
func (r gateRepo) run(check string, gha bool) (int, string) {
	env := func(string) string { return "" }
	if gha {
		env = ghaEnv
	}
	code, out, errs := gate(env, check, "--base", "main", "--head", "branch", "--repo", r.dir)
	return code, out + errs
}

func (r gateRepo) expect(t *testing.T, wantCode int, wants ...string) string {
	t.Helper()
	code, out := r.run("verify", false)
	if code != wantCode {
		t.Errorf("verify exit %d; want %d\n%s", code, wantCode, out)
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q\n%s", w, out)
		}
	}
	return out
}

// Test 3.
func TestVerifyInsideClaim(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "full", active, "docs")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-001: add docs")
	r.expect(t, 0, "leases: ok\n", "tiers: ok\n", "decisions: unchanged\n")
}

// Test 4.
func TestVerifyOutsideClaim(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "full", active, "docs/sub")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-001: add docs")
	code, out := r.run("verify", true)
	if code != 1 || !strings.Contains(out, "::error file=docs/a.md::leases: T-001: path is outside") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

// Test 5.
func TestVerifyDeleteAndRenameIntoClaim(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "full", active, "docs")
	r.git("rm", "-q", "other/x.md")
	r.commit("T-001: delete")
	r.expect(t, 1, "leases: FAIL: other/x.md:")

	r = leaseRepo(t)
	r.claim(1, "full", active, "docs")
	if err := os.MkdirAll(filepath.Join(r.dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	r.git("mv", "other/x.md", "docs/x.md")
	r.commit("T-001: move")
	out := r.expect(t, 1, "leases: FAIL: other/x.md:")
	if strings.Contains(out, "docs/x.md") {
		t.Errorf("rename target inside the claim was reported\n%s", out)
	}
}

// Test 6.
func TestVerifyCaseDiffers(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "full", active, "internal/comb")
	r.write("internal/Comb/x.go", []byte("package comb\n"))
	r.commit("T-001: case")
	r.expect(t, 1, "internal/Comb/x.go: T-001: case differs from lease")
}

// Test 7. No claim may lease any part of .waggle/, so the hive path also
// fails leases under either gate; the tier rule must fire only under light.
// comb.TestCheckChanges covers a full claim passing the tier rule alone.
func TestVerifyTier2UnderLight(t *testing.T) {
	const msg = "Tier 2 path under a light claim"
	for _, g := range []string{"light", "full"} {
		r := leaseRepo(t)
		r.claim(1, g, active, "docs")
		r.write(".waggle/hive/tasks/T-099.md", []byte("brief\n"))
		r.commit("T-001: brief")
		out := r.expect(t, 1)
		if strings.Contains(out, msg) != (g == "light") || strings.Contains(out, "tiers: ok") == (g == "light") {
			t.Errorf("%s claim\n%s", g, out)
		}
	}
}

// Test 8. Each path fails leases; the tier check fires exactly for the paths
// RequiredMinTier puts in Tier 3 (GO.MOD joins them when T-008 lands).
func TestVerifyTier3Paths(t *testing.T) {
	for _, p := range []string{"GO.MOD", ".WAGGLE/control/waggle.md", ".github/workflows/x.yml"} {
		r := leaseRepo(t)
		r.claim(1, "full", active, "docs")
		r.index(p)
		r.git("commit", "-q", "-m", "T-001: edit")
		out := r.expect(t, 1, "leases: FAIL: "+p)
		if strings.Contains(out, "Tier 3 path") != (comb.RequiredMinTier(p) == comb.Tier3ControlPlane) {
			t.Errorf("%s\n%s", p, out)
		}
	}
}

// Test 9.
func TestVerifyNonTicketCommits(t *testing.T) {
	r := leaseRepo(t)
	r.write("docs/a.md", []byte("a\n"))
	r.commit("Head Keeper: tweak")
	r.expect(t, 1, "merge with an admin bypass")

	r = leaseRepo(t)
	r.write("docs/a.md", []byte("a\n"))
	r.commit("fix stuff")
	r.expect(t, 1, `"fix stuff": commit has no T-<id>: prefix`)

	r = leaseRepo(t)
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-000: zero")
	r.expect(t, 1, "commit has no T-<id>: prefix")
}

// Test 10.
func TestVerifyMergeCommit(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "full", active, "docs")
	r.git("checkout", "-q", "-b", "side")
	r.write("docs/b.md", []byte("b\n"))
	r.commit("T-001: side")
	r.git("checkout", "-q", "branch")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-001: main line")
	r.git("merge", "-q", "--no-ff", "-m", "T-001: merge", "side")
	r.expect(t, 1, "in PR; rebase onto main")
}

// Test 11.
func TestVerifyMissingAndExpiredClaim(t *testing.T) {
	r := leaseRepo(t)
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-001: add")
	r.expect(t, 1, "no claim for T-001; claims must stay until merge")

	r.claim(1, "full", "2020-01-01T02:00:00Z", "docs")
	r.expect(t, 0, "leases: warning: claim for T-001 expired at 2020-01-01T02:00:00Z")
	if _, out := r.run("leases", true); !strings.Contains(out, "::warning::leases: claim for T-001 expired") {
		t.Errorf("annotation\n%s", out)
	}

	r.claim(1, "full", active, ".waggle")
	r.expect(t, 1, "T-001: refs/waggle/claims/ticket-1: comb: malformed claim ref")
}

// Test 12.
func TestVerifyTwoTickets(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "light", active, "docs")
	r.claim(2, "light", active, "src")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-001: docs")
	r.write("src/a.go", []byte("package src\n"))
	r.commit("T-002: src")
	r.expect(t, 0, "leases: ok\n")

	r.write("src/b.go", []byte("package src\n"))
	r.commit("T-001: swap")
	r.expect(t, 1, "src/b.go: T-001: path is outside")
}

// Test 13.
func TestVerifySymlink(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "full", active, "docs")
	if err := os.MkdirAll(filepath.Join(r.dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../other", filepath.Join(r.dir, "docs", "l")); err != nil {
		t.Skip(err)
	}
	r.commit("T-001: link")
	r.expect(t, 1, "docs/l: T-001: symlink or submodule mode 120000")
}

// Test 14. Built in the index so a case-insensitive filesystem can hold it.
func TestVerifyCaseCollision(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "full", active, "docs")
	r.index("docs/a.md")
	r.index("docs/A.md")
	r.git("commit", "-q", "-m", "T-001: collide")
	r.expect(t, 1, "path collides with")
	if code, out := r.run("tiers", false); code != 1 || strings.Contains(out, "leases") {
		t.Errorf("tiers alone: %d\n%s", code, out)
	}
}

// Test 15.
func TestVerifyEncodesHostileText(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "full", active, "src")
	blob := r.gitIn("a\n", "hash-object", "-w", "--stdin")
	r.gitIn("100644 blob "+blob+"\tdocs/x\n::warning::x,y:%0A\x00", "update-index", "-z", "--index-info")
	r.git("commit", "-q", "-m", "T-001: evil path")
	r.git("commit", "-q", "--allow-empty", "-m", "evil\r::warning::y %0A")
	for _, gha := range []bool{true, false} {
		code, out := r.run("verify", gha)
		if code != 1 || strings.Contains(out, "\r") {
			t.Errorf("gha %v: exit %d %q", gha, code, out)
		}
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "::warning::") || strings.HasPrefix(line, "x,y") {
				t.Errorf("gha %v: injected line %q", gha, line)
			}
		}
		if gha && !strings.Contains(out, "file=docs/x%0A%3A%3Awarning%3A%3Ax%2Cy%3A%250A::") {
			t.Errorf("file property not encoded\n%s", out)
		}
	}
}

// Test 16.
func TestVerifyReportsEveryCheck(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "full", active, "docs")
	r.write(layout.Decisions, []byte("rewritten\n"))
	r.write("src/a.go", []byte("package src\n"))
	r.commit("T-001: both")
	r.expect(t, 1, "leases: FAIL: src/a.go", "decisions: FAIL")
}

// Usage errors exit 2 before git runs; git failures exit 3.
func TestVerifyUsageAndErrors(t *testing.T) {
	for _, args := range [][]string{
		{"verify"}, {"leases", "--base", "main"}, {"tiers", "--base", "-x", "--head", "h"},
		{"verify", "--base", "a", "--head", "b", "extra"}, {"bogus"}, {},
	} {
		if code, _, _ := gate(nil, args...); code != 2 {
			t.Errorf("%q: exit %d; want 2", args, code)
		}
	}
	r := leaseRepo(t)
	code, _, errs := gate(nil, "leases", "--base", "main", "--head", "nope", "--repo", r.dir)
	if code != 3 || !strings.HasPrefix(errs, "leases: error: ") {
		t.Errorf("exit %d %q", code, errs)
	}
}
