//go:build unix

package dispatch

// T-023 item 23 (D-181): a worker commit must not start a detached git
// maintenance daemon that changes host state around the post-run check.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunPlantDuringAutoMaintenance is the proving test: the plant follows the
// result line, and a second worker commit starts git's detached auto
// maintenance while the entry has no gitdir file yet. Without maintenance.auto
// in the worker's git config, that daemon (which left the worker's process
// group) prunes the entry before the post-run check.
func TestRunPlantDuringAutoMaintenance(t *testing.T) {
	eachRepoSpelling(t, func(t *testing.T, link bool) {
		f := newFixture(t, true, link)
		planted := f.common() + "/worktrees/planted"
		scen := okScenario + "mkdir '" + planted + "'\n" + commitFile("internal/x/c.go", "two") +
			"sleep 1\nprintf '%s\\n' \"$PWD/.git\" > '" + planted + "/gitdir'\n"
		res, err := f.run(scen)
		if err != nil || res.ExitCode != 1 || !hasViolation(res.Violations, ruleHost, "worktrees/planted") {
			_, serr := os.Stat(planted)
			t.Fatalf("exit %d err %v violations %v (planted stat after run: %v)", res.ExitCode, err, res.Violations, serr)
		}
	})
}

// TestFixtureReposQuietGit: every fixture repo, the bare origin included, keeps
// auto maintenance off in its own config (a local push drops the client's -c).
func TestFixtureReposQuietGit(t *testing.T) {
	f := newFixture(t, true, false)
	rdir, _ := newRepo(t)
	for _, dir := range []string{f.real, f.origin, rdir} {
		for key, want := range map[string]string{"maintenance.auto": "false", "gc.auto": "0", "receive.autogc": "false"} {
			if got := sh(t, dir, "config", "--local", "--get", key); got != want {
				t.Errorf("%s: %s = %q, want %q", dir, key, got, want)
			}
		}
	}
}

// TestRunInDisablesAutoMaintenance: host-side git never starts auto maintenance or gc.
func TestRunInDisablesAutoMaintenance(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "git")
	args := filepath.Join(dir, "args")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > '"+args+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Git{Bin: bin}).RunIn(context.Background(), dir, "status"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(args)
	got := "\n" + string(b)
	for _, want := range []string{"\nmaintenance.auto=false\n", "\ngc.auto=0\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("RunIn args lack %q:\n%s", want, got)
		}
	}
}
