package main

import (
	"bytes"
	"os/exec"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/scaffold"
)

func runUp(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = runUpgrade(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRunUpgradeUsage(t *testing.T) {
	if code, _, errb := runUp("--bogus"); code != 2 || !strings.Contains(errb, "Usage: waggle upgrade") {
		t.Errorf("bad flag: code %d, stderr %q", code, errb)
	}
	if code, _, errb := runUp("extra"); code != 2 || !strings.Contains(errb, "unexpected argument") {
		t.Errorf("extra arg: code %d, stderr %q", code, errb)
	}
	if code, _, errb := runUp("--runtime=aider"); code != 2 || !strings.Contains(errb, "unsupported --runtime") {
		t.Errorf("bad runtime: code %d, stderr %q", code, errb)
	}
	if code, _, _ := runUp("-h"); code != 0 {
		t.Errorf("help: code %d", code)
	}
}

func TestRunUpgradeRefused(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git not found on PATH; upgrade tests need it")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "empty")
	chdir(t, dir)
	code, _, errb := runUp()
	if code != 1 || !strings.Contains(errb, "waggle upgrade refused") || !strings.Contains(errb, "run waggle init first") {
		t.Errorf("code %d, stderr %q", code, errb)
	}
}

func TestRunUpgradeDryRunAndApply(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git not found on PATH; upgrade tests need it")
	}
	pinned := func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v1.0.0"}}, true
	}
	old := upgradeBuildInfo
	upgradeBuildInfo = pinned
	t.Cleanup(func() { upgradeBuildInfo = old })

	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "gc.auto", "0")
	git(t, dir, "config", "maintenance.auto", "0")
	err := scaffold.InitRepo(scaffold.InitOptions{
		TargetDir: dir, Profile: "cli", Delivery: "installed", Forge: "github", CombMode: "direct",
		Runtime: "none", Out: &bytes.Buffer{}, BuildInfo: pinned,
	})
	if err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "init")
	chdir(t, dir)

	last := "Review with git status, then open a hive PR. This changes .waggle/control/ (Tier 3) and needs Head Keeper approval."
	code, out, errb := runUp("--runtime=none")
	if code != 0 || !strings.Contains(out, "Dry run: nothing changed") || !strings.Contains(out, last) || errb != "" {
		t.Errorf("dry run: code %d, stdout %q, stderr %q", code, out, errb)
	}
	code, out, errb = runUp("--apply", "--runtime=none")
	if code != 0 || strings.Contains(out, "Dry run") || !strings.HasSuffix(out, last+"\n") || errb != "" {
		t.Errorf("apply: code %d, stdout %q, stderr %q", code, out, errb)
	}
	if st := gitOut(t, dir, "", "status", "--porcelain=v1"); strings.TrimSpace(st) != "" {
		t.Errorf("apply with an unchanged pin left changes: %q", st)
	}
}
