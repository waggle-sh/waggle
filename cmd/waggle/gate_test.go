// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/decisions"
	"github.com/waggle-sh/waggle/internal/layout"
)

// gateRepo is a throwaway git repository with deterministic commits.
type gateRepo struct {
	t   *testing.T
	dir string
}

func newGateRepo(t *testing.T) gateRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	r := gateRepo{t, t.TempDir()}
	r.git("init", "-q")
	r.git("symbolic-ref", "HEAD", "refs/heads/main")
	return r
}

func (r gateRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_AUTHOR_DATE=2026-10-17T00:00:00Z",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com", "GIT_COMMITTER_DATE=2026-10-17T00:00:00Z")
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r gateRepo) write(rel string, data []byte) {
	r.t.Helper()
	p := filepath.Join(r.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// commit stages everything and commits it.
func (r gateRepo) commit(msg string) {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-q", "--allow-empty", "-m", msg)
}

// gateLog renders a log of n well-formed entries.
func gateLog(n int) []byte {
	var es []decisions.Entry
	for i := 1; i <= n; i++ {
		es = append(es, decisions.Entry{
			ID: i, Date: "2026-10-17", Kind: decisions.KindTechnical, Title: "Decision",
			Question: "Which?", Options: []string{"A", "B"}, Recommendation: "B", Choice: "B",
			Rationale: "Config is machine-written and tiny.",
		})
	}
	return decisions.Render(es)
}

// mainWithLog creates main with a README and a one-entry log, then a branch.
func mainWithLog(t *testing.T) gateRepo {
	t.Helper()
	r := newGateRepo(t)
	r.write("README.md", []byte("hive\n"))
	r.write(layout.Decisions, gateLog(1))
	r.commit("D-001")
	r.git("checkout", "-q", "-b", "branch")
	return r
}

func gate(getenv func(string) string, args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	code = runGate(args, &out, &errb, getenv)
	return code, out.String(), errb.String()
}

func (r gateRepo) check(t *testing.T, wantCode int, wantOut, wantErr string, getenv func(string) string) {
	t.Helper()
	code, out, errs := gate(getenv, "decisions", "--base", "main", "--head", "branch", "--repo", r.dir)
	if code != wantCode || !strings.HasPrefix(out, wantOut) || !strings.HasPrefix(errs, wantErr) {
		t.Errorf("code %d stdout %q stderr %q; want %d %q %q", code, out, errs, wantCode, wantOut, wantErr)
	}
}

func TestGateUntouched(t *testing.T) {
	r := mainWithLog(t)
	r.write("other.txt", []byte("x\n"))
	r.commit("other")
	r.check(t, 0, "decisions: unchanged\n", "", nil)
}

func TestGateAppend(t *testing.T) {
	r := mainWithLog(t)
	r.write(layout.Decisions, gateLog(2))
	r.commit("D-002")
	r.check(t, 0, "decisions: ok, appended D-002\n", "", nil)
	// --repo may point inside the repository; paths stay root-relative.
	code, out, _ := gate(nil, "decisions", "--base", "main", "--head", "branch", "--repo", filepath.Join(r.dir, ".waggle"))
	if code != 0 || out != "decisions: ok, appended D-002\n" {
		t.Errorf("from subdir: code %d stdout %q", code, out)
	}
}

func TestGateEditedEntry(t *testing.T) {
	r := mainWithLog(t)
	r.write(layout.Decisions, bytes.Replace(gateLog(2), []byte("tiny."), []byte("tidy."), 1))
	r.commit("edit D-001")
	r.check(t, 1, "", "decisions: FAIL: ", nil)
	actions := func(k string) string {
		if k == "GITHUB_ACTIONS" {
			return "true"
		}
		return ""
	}
	_, _, errs := gate(actions, "decisions", "--base", "main", "--head", "branch", "--repo", r.dir)
	if !strings.HasPrefix(errs, "::error file="+layout.Decisions+"::") || strings.Count(errs, "\n") != 1 || !strings.HasSuffix(errs, "\n") {
		t.Errorf("annotation %q", errs)
	}
}

// Moved base: main gains D-002 after the branch is cut. Against the merge-base
// the branch shows no change, so a base-to-head diff must not be used.
func TestGateMovedBase(t *testing.T) {
	r := mainWithLog(t)
	r.write("other.txt", []byte("x\n"))
	r.commit("other")
	r.git("checkout", "-q", "main")
	r.write(layout.Decisions, gateLog(2))
	r.commit("D-002 on main")
	r.check(t, 0, "decisions: unchanged\n", "", nil)
}

// Moved base with append: both sides append their own D-002. Each passes
// against its own merge-base; the real merge is blocked by the git conflict
// both appends create after the same final line, and once main is merged in
// the merge-base moves and the check reruns.
func TestGateMovedBaseAppend(t *testing.T) {
	r := mainWithLog(t)
	r.write(layout.Decisions, gateLog(2))
	r.commit("D-002 on branch")
	r.git("checkout", "-q", "main")
	r.write(layout.Decisions, bytes.Replace(gateLog(2), []byte("D-002: Decision"), []byte("D-002: Other"), 1))
	r.commit("D-002 on main")
	r.check(t, 0, "decisions: ok, appended D-002\n", "", nil)
}

func TestGateDeleted(t *testing.T) {
	r := mainWithLog(t)
	r.git("rm", "-q", layout.Decisions)
	r.commit("delete")
	r.check(t, 1, "", "decisions: FAIL: ", nil)
}

func TestGateSymlink(t *testing.T) {
	r := mainWithLog(t)
	r.write(".waggle/hive/copy.md", gateLog(1))
	p := filepath.Join(r.dir, filepath.FromSlash(layout.Decisions))
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("copy.md", p); err != nil {
		t.Skipf("symlink: %v", err)
	}
	r.commit("symlink")
	r.check(t, 1, "", "decisions: FAIL: head: "+layout.Decisions+" is mode 120000", nil)
}

// A case variant is added through the index so the test works on
// case-insensitive filesystems too.
func TestGateCaseVariant(t *testing.T) {
	r := mainWithLog(t)
	r.write("blob.txt", gateLog(1))
	oid := r.git("hash-object", "-w", "blob.txt")
	r.git("update-index", "--add", "--cacheinfo", "100644,"+oid+",.waggle/hive/decisions.md")
	r.git("commit", "-q", "-m", "case variant")
	r.check(t, 1, "", "decisions: FAIL: .waggle/hive/decisions.md is a case variant", nil)
}

func TestGateOversize(t *testing.T) {
	r := mainWithLog(t)
	r.write(layout.Decisions, append(gateLog(1), bytes.Repeat([]byte("x"), 5<<20)...))
	r.commit("huge")
	r.check(t, 1, "", "decisions: FAIL: head: "+layout.Decisions+" is ", nil)
}

func TestGateCreatedOnBranch(t *testing.T) {
	r := newGateRepo(t)
	r.write("README.md", []byte("hive\n"))
	r.commit("init")
	r.git("checkout", "-q", "-b", "branch")
	r.write(layout.Decisions, gateLog(2))
	r.commit("D-001, D-002")
	r.check(t, 0, "decisions: ok, appended D-001, D-002\n", "", nil)
}

func TestGateUsageAndErrors(t *testing.T) {
	r := mainWithLog(t)
	cases := []struct {
		args []string
		code int
		err  string
	}{
		{nil, 2, "Usage: waggle gate"},
		{[]string{"frobnicate"}, 2, "waggle gate: unknown check \"frobnicate\""},
		{[]string{"decisions", "--head", "branch", "--repo", r.dir}, 2, "waggle gate decisions: --base is required"},
		{[]string{"decisions", "--base", "-x", "--head", "branch", "--repo", r.dir}, 2, "waggle gate decisions: --base must not start with -"},
		{[]string{"decisions", "--base", "main", "--head", "a b", "--repo", r.dir}, 2, "waggle gate decisions: --head must not contain"},
		{[]string{"decisions", "--base", "nosuchref", "--head", "branch", "--repo", r.dir}, 3, "decisions: error: git: cannot resolve \"nosuchref\""},
		{[]string{"decisions", "--base", "main", "--head", "branch", "extra"}, 2, "waggle gate decisions: unexpected argument"},
		{[]string{"decisions", "--base", "main", "--head", "branch", "--repo", t.TempDir()}, 3, "decisions: error: "},
	}
	for _, c := range cases {
		code, out, errs := gate(nil, c.args...)
		if code != c.code || out != "" || !strings.HasPrefix(errs, c.err) {
			t.Errorf("%q: code %d stdout %q stderr %q; want %d %q", c.args, code, out, errs, c.code, c.err)
		}
	}
	if code, _, _ := gate(nil, "decisions", "-h"); code != 0 {
		t.Errorf("-h: code %d", code)
	}
}

func TestGateNoCommonHistory(t *testing.T) {
	r := mainWithLog(t)
	r.git("checkout", "-q", "--orphan", "other")
	r.write(layout.Decisions, gateLog(2))
	r.commit("unrelated root")
	code, _, errs := gate(nil, "decisions", "--base", "main", "--head", "other", "--repo", r.dir)
	if code != 3 || !strings.Contains(errs, "no merge-base; is the clone shallow? use fetch-depth: 0") {
		t.Errorf("code %d stderr %q", code, errs)
	}
}

// Only blobs are read: an uncommitted edit in the working tree changes nothing.
func TestGateIgnoresWorkingTree(t *testing.T) {
	r := mainWithLog(t)
	r.write(layout.Decisions, gateLog(2))
	r.commit("D-002")
	r.write(layout.Decisions, []byte("garbage\n"))
	r.check(t, 0, "decisions: ok, appended D-002\n", "", nil)
}
