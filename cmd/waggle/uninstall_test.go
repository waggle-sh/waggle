// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/scaffold"
)

var gitEnv = []string{
	"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
	"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
	"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(append(os.Environ(), "LC_ALL=C"), gitEnv...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// chdir moves the process into dir for the rest of the test. Tests in this
// package must not run in parallel.
func chdir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func run(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = runUninstall(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRunUninstallUsage(t *testing.T) {
	if code, _, errb := run("--bogus"); code != 2 || !strings.Contains(errb, "Usage: waggle uninstall") {
		t.Errorf("bad flag: code %d, stderr %q", code, errb)
	}
	if code, _, errb := run("extra"); code != 2 || !strings.Contains(errb, "unexpected argument") {
		t.Errorf("extra arg: code %d, stderr %q", code, errb)
	}
	if code, _, _ := run("-h"); code != 0 {
		t.Errorf("help: code %d", code)
	}
}

func TestRunUninstallRefused(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git not found on PATH; uninstall tests need it")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	chdir(t, dir)
	code, _, errb := run()
	if code != 1 || !strings.Contains(errb, "waggle uninstall refused") {
		t.Errorf("code %d, stderr %q", code, errb)
	}
}

func TestRunUninstallDryRunAndApply(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git not found on PATH; uninstall tests need it")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	err := scaffold.InitRepo(scaffold.InitOptions{
		TargetDir: dir, Profile: "cli", Delivery: "installed", Forge: "github", CombMode: "direct",
		Runtime: "none", Out: &bytes.Buffer{},
		BuildInfo: func() (*debug.BuildInfo, bool) { return &debug.BuildInfo{Main: debug.Module{Version: "v1.0.0"}}, true },
	})
	if err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "init")
	chdir(t, dir)

	code, out, errb := run()
	if code != 0 || !strings.Contains(out, "Dry run: nothing changed") || errb != "" {
		t.Errorf("dry run: code %d, stdout %q, stderr %q", code, out, errb)
	}
	if _, err := os.Stat(".waggle"); err != nil {
		t.Error("dry run removed .waggle")
	}
	code, out, errb = run("--apply")
	if code != 0 || strings.Contains(out, "Dry run") || errb != "" {
		t.Errorf("apply: code %d, stdout %q, stderr %q", code, out, errb)
	}
	if _, err := os.Stat(".waggle"); !os.IsNotExist(err) {
		t.Error("apply left .waggle")
	}
}
