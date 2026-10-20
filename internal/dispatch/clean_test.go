//go:build unix

package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cleanRepo is a fixture with ticket 23's builder worktree on branch t-023 and a
// claim ref, as a run leaves them.
func cleanRepo(t *testing.T, link bool) (f *fixture, wt string) {
	t.Helper()
	f = newFixture(t, false, link)
	wt = filepath.Join(f.real, ".waggle/worktrees/T-023-builder")
	f.git("worktree", "add", "-q", "-b", "t-023", wt, "main")
	f.git("update-ref", "refs/waggle/claims/ticket-23", "HEAD")
	return f, wt
}

func doClean(f *fixture, role string, runs bool) (string, error) {
	var out bytes.Buffer
	err := Clean(context.Background(), CleanRequest{RepoDir: f.repo, Ticket: "23", Role: role, Runs: runs}, &out)
	return out.String(), err
}

func writeLock(t *testing.T, path string, pid int) {
	t.Helper()
	host, _ := hostName()
	b, _ := json.Marshal(lockInfo{PID: pid, Host: host, Started: time.Now()})
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

// D2-T7: a live run lock and a held dispatch.lock are exit 3 and remove nothing.
func TestCleanRefusesLiveLocks(t *testing.T) {
	f, wt := cleanRepo(t, false)
	writeLock(t, wt+".lock", os.Getpid())
	if _, err := doClean(f, "", false); code(err) != ExitEnv || !exists(wt) {
		t.Fatalf("run lock: err %v, worktree kept %v", err, exists(wt))
	}
	os.Remove(wt + ".lock")
	lock := filepath.Join(f.real, ".waggle/worktrees/dispatch.lock")
	writeLock(t, lock, os.Getpid())
	if _, err := doClean(f, "", false); code(err) != ExitEnv || !strings.Contains(err.Error(), "dispatch run is in progress") || !exists(wt) {
		t.Fatalf("dispatch.lock: err %v, worktree kept %v", err, exists(wt))
	}
}

func TestCleanTakesOverDeadLock(t *testing.T) {
	old := pidAlive
	pidAlive = func(int) bool { return false }
	t.Cleanup(func() { pidAlive = old })
	f, wt := cleanRepo(t, false)
	writeLock(t, wt+".lock", 424242)
	out, err := doClean(f, "", false)
	if err != nil || !strings.Contains(out, "took over stale run lock of dead pid 424242") || exists(wt) || exists(wt+".lock") {
		t.Fatalf("out %q err %v", out, err)
	}
}

func TestCleanRemovesWorktreeAndMarkers(t *testing.T) {
	eachRepoSpelling(t, func(t *testing.T, link bool) {
		f, wt := cleanRepo(t, link)
		branch, claim := f.git("rev-parse", "t-023"), f.git("rev-parse", "refs/waggle/claims/ticket-23")
		record := filepath.Join(f.real, ".waggle/worktrees", hostRecordName)
		write(t, f.real, ".waggle/worktrees/"+hostRecordName, `{"run":"r","files":{}}`)
		write(t, f.real, ".waggle/worktrees/T-023-builder.tainted", "run-1\x1b[31m\n\x00"+strings.Repeat("x", 300))
		write(t, f.real, ".waggle/worktrees/T-023-builder.lock.takeover", "")
		out, err := doClean(f, "", false)
		if err != nil || exists(wt) || exists(wt+".tainted") || exists(wt+".lock.takeover") || exists(wt+".lock") {
			t.Fatalf("out %q err %v", out, err)
		}
		if i := strings.Index(out, "tainted by run "); i < 0 || strings.ContainsAny(out, "\x1b\x00") || len(strings.TrimSpace(out[i:])) > 160 {
			t.Fatalf("run dir not printed cleanly: %q", out)
		}
		if got := f.git("worktree", "list", "--porcelain"); strings.Contains(got, "T-023-builder") {
			t.Fatalf("still registered: %s", got)
		}
		if f.git("rev-parse", "t-023") != branch || f.git("rev-parse", "refs/waggle/claims/ticket-23") != claim {
			t.Fatal("the branch or the claim ref moved")
		}
		if b, _ := os.ReadFile(record); string(b) != `{"run":"r","files":{}}` {
			t.Fatalf("host-files.json changed: %s", b)
		}
		if out, err := doClean(f, "", false); err != nil || out != "" {
			t.Fatalf("nothing left to clean: out %q err %v", out, err)
		}
	})
}

func TestCleanRunsOnlyMatching(t *testing.T) {
	f, _ := cleanRepo(t, false)
	runs := filepath.Join(f.real, ".waggle/worktrees/runs")
	names := []string{"T-023-builder-1", "T-023-builder-12", "T-023-guard-2", "T-024-builder-1", "T-023-builder-x", "T-0230-builder-1"}
	for _, n := range names {
		write(t, runs, n+"/result.json", "{}")
	}
	if _, err := doClean(f, "builder", true); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if want := n != "T-023-builder-1" && n != "T-023-builder-12"; exists(filepath.Join(runs, n)) != want {
			t.Errorf("%s: kept %v, want %v", n, !want, want)
		}
	}
	if _, err := doClean(f, "", true); err != nil || exists(filepath.Join(runs, "T-023-guard-2")) || !exists(filepath.Join(runs, "T-024-builder-1")) {
		t.Fatalf("all roles: err %v", err)
	}
	elsewhere := t.TempDir()
	write(t, elsewhere, "T-023-builder-1/keep", "x")
	if err := os.RemoveAll(runs); err != nil || os.Symlink(elsewhere, runs) != nil {
		t.Fatal("cannot plant the runs symlink")
	}
	if _, err := doClean(f, "", true); code(err) != ExitEnv || !exists(filepath.Join(elsewhere, "T-023-builder-1/keep")) {
		t.Fatalf("a symlinked runs directory: err %v", err)
	}
}

// Two admin entries naming one worktree: exit 3, naming both, nothing removed.
func TestCleanRefusesDuplicateEntries(t *testing.T) {
	f, wt := cleanRepo(t, false)
	real, _ := filepath.EvalSymlinks(wt)
	write(t, f.common(), "worktrees/0/gitdir", real+"/.git\n")
	out, err := doClean(f, "", false)
	if code(err) != ExitEnv || err == nil || !strings.Contains(err.Error(), "worktrees/0 and worktrees/T-023-builder") || !exists(wt) || !exists(filepath.Join(f.common(), "worktrees/0")) || !exists(filepath.Join(f.common(), "worktrees/T-023-builder")) {
		t.Fatalf("out %q err %v", out, err)
	}
}

// Every git call runs pinned: the Keeper's global config must not reach it.
func TestCleanGitCallsArePinned(t *testing.T) {
	f, wt := cleanRepo(t, false)
	log, cfg := filepath.Join(t.TempDir(), "trace"), filepath.Join(t.TempDir(), "gitconfig")
	write(t, filepath.Dir(cfg), filepath.Base(cfg), "[trace2]\n\teventTarget = "+log+"\n")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	if _, err := (&Git{Dir: f.real}).Run(context.Background(), "status"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(log); len(b) == 0 {
		t.Fatal("the probe config does not reach an unpinned git call")
	}
	if err := os.Truncate(log, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := doClean(f, "", false); err != nil || exists(wt) {
		t.Fatalf("err %v", err)
	}
	if b, _ := os.ReadFile(log); len(b) != 0 {
		t.Fatalf("an unpinned git call ran: %s", b)
	}
}
