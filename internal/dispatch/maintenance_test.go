//go:build unix

package dispatch

// T-023 item 23 (D-181): a worker commit must not start a detached git
// maintenance daemon that changes host state around the post-run check.

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedLoose17 writes 40 blob bodies whose git blob id starts with byte 0x17 into
// a fresh directory and returns it. Loaded as loose objects they put 40 files
// in objects/17/, above the auto gc threshold of ceil(6700/256) = 27.
func seedLoose17(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for n, found := 0, 0; found < 40; n++ {
		body := fmt.Sprintf("seed-%d\n", n)
		sum := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(body), body)))
		if sum[0] != 0x17 {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("s%02d", found)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		found++
	}
	return dir
}

// TestRunPlantDuringAutoMaintenance is the proving test: the fixture's config
// forces auto gc and the worker seeds loose objects, so the second worker
// commit starts git's detached auto gc while the plant has no gitdir file yet.
// Only the worker's env (maintenance.auto and gc.auto in tripwireEnv) stops it;
// without those keys the daemon, which left the worker's process group, prunes
// the entry before the post-run check.
func TestRunPlantDuringAutoMaintenance(t *testing.T) {
	eachRepoSpelling(t, func(t *testing.T, link bool) {
		f := newFixture(t, true, link)
		// the fixture's own quiet config would hide a missing worker-side key
		for _, kv := range [][2]string{{"maintenance.auto", "true"}, {"gc.auto", "6700"}, {"gc.autoDetach", "true"},
			{"maintenance.autoDetach", "true"}, {"maintenance.gc.enabled", "true"}} {
			sh(t, f.real, "config", "--local", kv[0], kv[1])
		}
		seeds := seedLoose17(t)
		if id := sh(t, f.real, "hash-object", filepath.Join(seeds, "s00")); !strings.HasPrefix(id, "17") {
			t.Fatalf("seed blob id %s does not start with 17 (SHA-256 repo?)", id)
		}
		planted := f.common() + "/worktrees/planted"
		scen := okScenario + "mkdir '" + planted + "'\n" + "git hash-object -w '" + seeds + "'/*\n" +
			commitFile("internal/x/c.go", "two") +
			"sleep 3\nprintf '%s\\n' \"$PWD/.git\" > '" + planted + "/gitdir'\n"
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
