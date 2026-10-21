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
		if out, err := doClean(f, "", false); err != nil || out != "nothing to clean for 23\n" {
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

// nameTwice plants admin entry 0 whose gitdir names the worktree wt.
func nameTwice(t *testing.T, f *fixture, wt string) {
	t.Helper()
	real, _ := filepath.EvalSymlinks(wt)
	write(t, f.common(), "worktrees/0/gitdir", real+"/.git\n")
}

// Two admin entries naming one worktree: exit 3, nothing removed, whether the
// worktree directory exists or is gone.
func TestCleanRefusesAmbiguousEntries(t *testing.T) {
	for _, gone := range []bool{false, true} {
		f, wt := cleanRepo(t, false)
		nameTwice(t, f, wt)
		if gone {
			if err := os.RemoveAll(wt); err != nil {
				t.Fatal(err)
			}
		}
		out, err := doClean(f, "", false)
		if code(err) != ExitEnv || err == nil || strings.Contains(out, "removed") || !exists(filepath.Join(f.common(), "worktrees/0")) || !exists(filepath.Join(f.common(), "worktrees/T-023-builder")) || gone == exists(wt) {
			t.Fatalf("gone %v: out %q err %v", gone, out, err)
		}
	}
}

// Planted entries that copy or alias another ticket's (D-202): clean never
// prunes, so they stay as planted and T-024's entry and worktree stay usable.
func TestCleanLeavesPlantedEntriesAlone(t *testing.T) {
	for _, mode := range []string{"copy", "case", "GIT", "upper", "parent", "relative"} {
		f, wt := cleanRepo(t, false)
		other := filepath.Join(f.real, ".waggle/worktrees/T-024-builder")
		f.git("worktree", "add", "-q", "-b", "t-024", other, "main")
		b, _ := os.ReadFile(filepath.Join(f.common(), "worktrees/T-024-builder/gitdir"))
		g := strings.TrimSpace(string(b))
		switch mode {
		case "case":
			g = strings.Replace(g, "T-024-builder", "t-024-builder", 1)
		case "GIT":
			g = strings.TrimSuffix(g, ".git") + ".GIT"
		case "upper":
			g = strings.ToUpper(g)
		case "parent":
			ln := filepath.Join(t.TempDir(), "ln")
			if os.Symlink(filepath.Dir(filepath.Dir(g)), ln) != nil {
				t.Fatal("cannot plant the parent link")
			}
			g = filepath.Join(ln, "T-024-builder", ".git")
		case "relative":
			if os.MkdirAll(filepath.Join(f.real, "Z"), 0o755) != nil || os.Symlink(other, filepath.Join(f.real, "Z/X")) != nil {
				t.Fatal("cannot plant the relative link")
			}
			g = "../../../Z/X/../T-024-builder/.git"
		}
		write(t, f.common(), "worktrees/0/gitdir", g+"\n")
		out, err := doClean(f, "", false)
		if err != nil || exists(wt) || !exists(other) || !exists(filepath.Join(f.common(), "worktrees/0/gitdir")) || !exists(filepath.Join(f.common(), "worktrees/T-024-builder/gitdir")) {
			t.Fatalf("%s: out %q err %v", mode, out, err)
		}
		if _, err := (&Git{Dir: other}).Run(context.Background(), "status"); err != nil {
			t.Fatalf("%s: T-024 is not usable: %v", mode, err)
		}
	}
}

// A missing T-023 worktree: its own admin entry goes, other tickets' entries
// (usable and prunable alike) stay.
func TestCleanMissingWorktreeRemovesOwnEntry(t *testing.T) {
	eachRepoSpelling(t, func(t *testing.T, link bool) {
		f, wt := cleanRepo(t, link)
		other, gone := filepath.Join(f.real, ".waggle/worktrees/T-024-builder"), filepath.Join(f.real, ".waggle/worktrees/T-025-builder")
		f.git("worktree", "add", "-q", "-b", "t-024", other, "main")
		f.git("worktree", "add", "-q", "-b", "t-025", gone, "main")
		if os.RemoveAll(gone) != nil || os.RemoveAll(wt) != nil {
			t.Fatal("cannot remove the worktrees")
		}
		out, err := doClean(f, "", false)
		w := filepath.Join(f.common(), "worktrees")
		if err != nil || !strings.Contains(out, "removed worktree") || exists(filepath.Join(w, "T-023-builder")) || !exists(filepath.Join(w, "T-024-builder")) || !exists(filepath.Join(w, "T-025-builder")) || !exists(other) {
			t.Fatalf("out %q err %v", out, err)
		}
	})
}

// A healthy repo with relative worktree paths and a prunable entry of another
// ticket cleans with exit 0 and leaves that entry alone.
func TestCleanKeepsOtherTicketsPrunableEntry(t *testing.T) {
	f, wt := cleanRepo(t, false)
	f.git("config", "worktree.useRelativePaths", "true")
	other, gone := filepath.Join(f.real, ".waggle/worktrees/T-024-builder"), filepath.Join(f.real, ".waggle/worktrees/T-025-builder")
	f.git("worktree", "add", "-q", "-b", "t-024", other, "main")
	f.git("worktree", "add", "-q", "-b", "t-025", gone, "main")
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	if out, err := doClean(f, "", false); err != nil || exists(wt) || !exists(other) || !exists(filepath.Join(f.common(), "worktrees/T-024-builder")) || !exists(filepath.Join(f.common(), "worktrees/T-025-builder")) {
		t.Fatalf("out %q err %v", out, err)
	}
}

// Under worktree.useRelativePaths a missing worktree's own entry (relative
// gitdir) is still found and dropped, and other entries stay.
func TestCleanDropsMissingWorktreeEntryWithRelativeGitdir(t *testing.T) {
	f := newFixture(t, false, false)
	f.git("config", "worktree.useRelativePaths", "true")
	wt, other := filepath.Join(f.real, ".waggle/worktrees/T-023-builder"), filepath.Join(f.real, ".waggle/worktrees/T-024-builder")
	f.git("worktree", "add", "-q", "-b", "t-023", wt, "main")
	f.git("worktree", "add", "-q", "-b", "t-024", other, "main")
	f.git("update-ref", "refs/waggle/claims/ticket-23", "HEAD")
	if err := os.RemoveAll(wt); err != nil {
		t.Fatal(err)
	}
	if out, err := doClean(f, "", false); err != nil || exists(filepath.Join(f.common(), "worktrees/T-023-builder")) || !exists(filepath.Join(f.common(), "worktrees/T-024-builder")) || !exists(other) {
		t.Fatalf("out %q err %v", out, err)
	}
}

// A gitdir that is not a readable regular file (here a link) refuses with exit 3.
func TestCleanRefusesLinkedGitdir(t *testing.T) {
	f, wt := cleanRepo(t, false)
	if err := os.MkdirAll(filepath.Join(f.common(), "worktrees/0"), 0o755); err != nil || os.Symlink(filepath.Join(f.common(), "worktrees/T-023-builder/gitdir"), filepath.Join(f.common(), "worktrees/0/gitdir")) != nil {
		t.Fatal("cannot plant the link")
	}
	if _, err := doClean(f, "", false); code(err) != ExitEnv || !exists(wt) {
		t.Fatalf("a linked gitdir: err %v", err)
	}
}

// A stray file and a gitdir-less entry directory are skipped (git prunes both);
// a symlinked entry or gitdir still refuses. The real worktrees stay intact.
func TestCleanSkipsStrayAdminEntries(t *testing.T) {
	f, wt := cleanRepo(t, false)
	write(t, f.common(), "worktrees/.DS_Store", "x")
	if err := os.MkdirAll(filepath.Join(f.common(), "worktrees/half"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := doClean(f, "", false); err != nil || exists(wt) {
		t.Fatalf("out %q err %v", out, err)
	}
	f, wt = cleanRepo(t, false)
	if err := os.Symlink(filepath.Join(f.common(), "worktrees/T-023-builder"), filepath.Join(f.common(), "worktrees/ln")); err != nil {
		t.Fatal(err)
	}
	if _, err := doClean(f, "", false); code(err) != ExitEnv || !exists(wt) {
		t.Fatalf("a symlinked entry: err %v", err)
	}
}

// Outside a repository clean exits 3 and leaves no .waggle/worktrees behind.
func TestCleanOutsideRepoCreatesNothing(t *testing.T) {
	dir := t.TempDir()
	err := Clean(context.Background(), CleanRequest{RepoDir: dir, Ticket: "23"}, &bytes.Buffer{})
	if code(err) != ExitEnv || exists(filepath.Join(dir, ".waggle")) {
		t.Fatalf("err %v", err)
	}
}

// D2b-T6: a plain directory at the worktree path is not a registered worktree:
// exit 3, directory and tainted marker kept.
func TestCleanRefusesUnregisteredDirectory(t *testing.T) {
	f := newFixture(t, false, false)
	wt := filepath.Join(f.real, ".waggle/worktrees/T-023-builder")
	write(t, wt, "keep.txt", "x")
	write(t, f.real, ".waggle/worktrees/T-023-builder.tainted", "run-1")
	out, err := doClean(f, "", false)
	if code(err) != ExitEnv || err == nil || !strings.Contains(err.Error(), "not a registered worktree") || !exists(wt+"/keep.txt") || !exists(wt+".tainted") || strings.Contains(out, "removed") {
		t.Fatalf("out %q err %v", out, err)
	}
}

// D2b-T6: a gone directory with no entry says so before the markers go.
func TestCleanSaysWhenNoEntryNamesTheDirectory(t *testing.T) {
	f := newFixture(t, false, false)
	write(t, f.real, ".waggle/worktrees/T-023-builder.tainted", "run-1")
	out, err := doClean(f, "", false)
	if err != nil || !strings.Contains(out, "no worktree entry for .waggle/worktrees/T-023-builder") || exists(filepath.Join(f.real, ".waggle/worktrees/T-023-builder.tainted")) {
		t.Fatalf("out %q err %v", out, err)
	}
}

// D2b-T6: nothing to clean prints its line.
func TestCleanNothingPrintsALine(t *testing.T) {
	f := newFixture(t, false, false)
	if out, err := doClean(f, "", false); err != nil || out != "nothing to clean for 23\n" {
		t.Fatalf("out %q err %v", out, err)
	}
}
