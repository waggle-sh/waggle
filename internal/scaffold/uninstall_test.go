package scaffold

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// gitEnv keeps tests independent of the developer's git config and gives
// commits an identity.
var gitEnv = []string{
	"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
	"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
	"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
}

// git runs git in dir for test setup and inspection.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0"), gitEnv...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, errb.String())
	}
	return strings.TrimSpace(out.String())
}

func newRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git not found on PATH; uninstall tests need it")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	return dir
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", msg)
}

// project is snapshot without the .git directory.
func project(t *testing.T, dir string) map[string]string {
	t.Helper()
	m := snapshot(t, dir)
	for rel := range m {
		if rel == ".git/" || strings.HasPrefix(rel, ".git/") {
			delete(m, rel)
		}
	}
	return m
}

func uninstall(t *testing.T, dir string, apply bool) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := Uninstall(UninstallOptions{Dir: dir, Apply: apply, Out: &out, GitEnv: gitEnv})
	return out.String(), err
}

// installed runs init in a repo and commits the result.
func installed(t *testing.T, dir, rt string) {
	t.Helper()
	o, _ := opts(dir, rt)
	mustInit(t, o)
	commitAll(t, dir, "waggle init")
}

func TestUninstallDryRunChangesNothing(t *testing.T) {
	dir := newRepo(t)
	installed(t, dir, "claude-code")
	before := snapshot(t, dir)

	out, err := uninstall(t, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if after := snapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Error("dry run changed files")
	}
	for _, want := range []string{
		"[remove] .waggle/ (",
		"[remove] .github/workflows/waggle-gate.yml\n",
		"[remove] .github/workflows/ (empty after uninstall)",
		"[remove] .github/ (empty after uninstall)",
		"[remove] .gitignore (holds only the waggle block)",
		"[remove] CLAUDE.md (holds only the waggle block)",
		"[remove] .claude/agents/waggle-forager.md\n",
		"[remove] .claude/agents/ (empty after uninstall)",
		"[remove] .claude/ (empty after uninstall)",
		"[keep]   refs/waggle/* (list with: git for-each-ref --format='%(refname)' refs/waggle/)",
		"Dry run: nothing changed. Rerun with --apply to execute the plan above.\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plan lacks %q:\n%s", want, out)
		}
	}
	if !strings.HasSuffix(out, uninstallLastLine+"\n") {
		t.Errorf("last line wrong:\n%s", out)
	}
}

func TestUninstallApplyFreshRepo(t *testing.T) {
	dir := newRepo(t)
	installed(t, dir, "claude-code")
	put(t, dir, ".waggle/hive/tasks/T-1.md", "brief\n")
	put(t, dir, ".waggle/hive/DECISIONS.md", "log\n")
	commitAll(t, dir, "hive record")

	out, err := uninstall(t, dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := project(t, dir); len(got) != 0 {
		t.Errorf("files left after uninstall: %v", got)
	}
	if !strings.Contains(out, ", 2 of them under .waggle/hive/)") {
		t.Errorf("hive file count missing:\n%s", out)
	}
	if !strings.HasSuffix(out, uninstallLastLine+"\n") {
		t.Errorf("last line wrong:\n%s", out)
	}
	if status := git(t, dir, "status", "--porcelain=v1"); !strings.Contains(status, " D CLAUDE.md") || strings.Contains(status, "??") {
		t.Errorf("uninstall committed or left strays:\n%s", status)
	}
	if git(t, dir, "log", "--format=%s", "-1") != "hive record" {
		t.Error("uninstall committed")
	}
}

func TestUninstallApplyRestoresProjectFiles(t *testing.T) {
	dir := newRepo(t)
	put(t, dir, "CLAUDE.md", "# Project rules\n\nBe nice.\n")
	put(t, dir, ".gitignore", "node_modules/"+legacyGitignore+"dist/\n")
	put(t, dir, ".claude/agents/foo.md", "mine\n")
	put(t, dir, ".github/workflows/ci.yml", "name: ci\n")
	put(t, dir, "src/main.go", "package main\n")
	commitAll(t, dir, "project")
	before := project(t, dir)

	installed(t, dir, "claude-code")
	put(t, dir, ".waggle/hive/tasks/T-1.md", "brief\n")
	commitAll(t, dir, "hive record")
	git(t, dir, "update-ref", "refs/waggle/claims/T-1", "HEAD")

	out, err := uninstall(t, dir, true)
	if err != nil {
		t.Fatal(err)
	}
	// Waggle only ever appends to a terminated .gitignore, so the one rerun of
	// init replaced the legacy block and uninstall drops it entirely.
	before[".gitignore"] = "node_modules/\ndist/\n"
	if after := project(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("project files:\n got %v\nwant %v", after, before)
	}
	for _, want := range []string{
		"[edit]   CLAUDE.md (waggle block removed)",
		"[edit]   .gitignore (waggle block removed)",
		"[remove] .github/workflows/waggle-gate.yml\n",
		"[remove] .claude/agents/waggle-queen.md\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plan lacks %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"[remove] .github/ ", "[remove] .claude/ ", "[remove] .github/workflows/ ", "[remove] .claude/agents/ "} {
		if strings.Contains(out, unwanted) {
			t.Errorf("plan removes a project directory %q:\n%s", unwanted, out)
		}
	}
	if refs := git(t, dir, "for-each-ref", "--format=%(refname)", "refs/waggle/"); refs != "refs/waggle/claims/T-1" {
		t.Errorf("refs/waggle/* touched: %q", refs)
	}
}

func TestUninstallAgentsImport(t *testing.T) {
	dir := newRepo(t)
	put(t, dir, "AGENTS.md", "# Agents\n")
	commitAll(t, dir, "project")
	before := project(t, dir)
	installed(t, dir, "none")

	if _, err := uninstall(t, dir, true); err != nil {
		t.Fatal(err)
	}
	if after := project(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("project files:\n got %v\nwant %v", after, before)
	}
}

func TestUninstallKeepsUnmarkedWorkflow(t *testing.T) {
	dir := newRepo(t)
	installed(t, dir, "none")
	custom := "name: gate\non: [pull_request]\njobs: {}\n"
	put(t, dir, ".github/workflows/waggle-gate.yml", custom)
	commitAll(t, dir, "hand-written gate")

	out, err := uninstall(t, dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := get(t, dir, ".github/workflows/waggle-gate.yml"); got != custom {
		t.Errorf("unmarked workflow changed: %q", got)
	}
	if !strings.Contains(out, "[skip]   .github/workflows/waggle-gate.yml (warning: no \"# waggle:managed\" line") {
		t.Errorf("missing skip warning:\n%s", out)
	}
	if strings.Contains(out, "[remove] .github") {
		t.Errorf("plan removes .github:\n%s", out)
	}
}

func TestUninstallRefusesWithoutWriting(t *testing.T) {
	type tc struct {
		setup   func(t *testing.T, dir string) (runDir string)
		wantErr error  // beyond ErrRefused
		wantMsg string // substring of the error
	}
	fresh := func(t *testing.T, dir string) string {
		installed(t, dir, "claude-code")
		return dir
	}
	cases := map[string]tc{
		"subdirectory": {setup: func(t *testing.T, dir string) string {
			put(t, dir, "src/x.go", "package x\n")
			commitAll(t, dir, "src")
			return fresh(t, dir)
		}, wantMsg: "work tree root"},
		"untracked file": {setup: func(t *testing.T, dir string) string {
			fresh(t, dir)
			put(t, dir, "stray.txt", "x\n")
			return dir
		}, wantMsg: "first dirty path: stray.txt"},
		"modified file": {setup: func(t *testing.T, dir string) string {
			fresh(t, dir)
			put(t, dir, "CLAUDE.md", "edited\n")
			return dir
		}, wantMsg: "first dirty path: CLAUDE.md"},
		"legacy layout": {setup: func(t *testing.T, dir string) string {
			put(t, dir, ".waggle/config.yaml", "x\n")
			commitAll(t, dir, "legacy")
			return dir
		}, wantErr: ErrLegacyLayout},
		"not installed": {setup: func(t *testing.T, dir string) string {
			commitAll(t, dir, "empty")
			return dir
		}, wantMsg: "nothing to uninstall"},
		"socket present": {setup: func(t *testing.T, dir string) string {
			fresh(t, dir)
			put(t, dir, ".waggle/control/run.sock", "")
			return dir
		}, wantMsg: "stop waggled first"},
		"bad markers": {setup: func(t *testing.T, dir string) string {
			fresh(t, dir)
			put(t, dir, "CLAUDE.md", "<!-- waggle:start -->\nx\n")
			commitAll(t, dir, "broken markers")
			return dir
		}, wantErr: ErrMarkers},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir := newRepo(t)
			runDir := c.setup(t, dir)
			if name == "subdirectory" {
				runDir = filepath.Join(dir, "src")
			}
			before := snapshot(t, dir)
			var out bytes.Buffer
			err := Uninstall(UninstallOptions{Dir: runDir, Apply: true, Out: &out, GitEnv: gitEnv})
			if !errors.Is(err, ErrRefused) {
				t.Fatalf("err = %v; want ErrRefused", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Errorf("err = %v; want %v", err, c.wantErr)
			}
			if c.wantMsg != "" && !strings.Contains(err.Error(), c.wantMsg) {
				t.Errorf("err = %v; want substring %q", err, c.wantMsg)
			}
			if after := snapshot(t, dir); !reflect.DeepEqual(before, after) {
				t.Error("refused uninstall changed files")
			}
		})
	}
}

func TestUninstallOutsideRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := uninstall(t, dir, false)
	if !errors.Is(err, ErrRefused) {
		t.Errorf("err = %v; want ErrRefused", err)
	}
}

func TestUninstallMissingGit(t *testing.T) {
	dir := newRepo(t)
	installed(t, dir, "none")
	err := Uninstall(UninstallOptions{Dir: dir, Out: &bytes.Buffer{}, GitBinary: filepath.Join(dir, "no-such-git")})
	if err == nil || errors.Is(err, ErrRefused) {
		t.Errorf("err = %v; want an I/O error that is not ErrRefused", err)
	}
}
