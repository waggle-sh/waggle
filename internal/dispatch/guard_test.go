package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// A planted plain repo at the worktree path must not be reused and its
// config must never run.
func TestPlantedRepoNotReused(t *testing.T) {
	dir, g := newRepo(t)
	base, _ := ResolveBase(context.Background(), g, "", "")
	sh(t, dir, "branch", "t-023", base)
	planted := filepath.Join(dir, ".waggle/worktrees/T-023-builder")
	if err := os.MkdirAll(planted, 0o755); err != nil {
		t.Fatal(err)
	}
	sh(t, planted, "init", "-q", "-b", "t-023")
	marker := filepath.Join(t.TempDir(), "ran")
	sh(t, planted, "config", "core.fsmonitor", "touch "+marker)
	_, err := Provision(context.Background(), g, mustValid(t, okReq(dir)), base, t0)
	if code(err) != ExitEnv {
		t.Fatalf("planted repo reused: %v", err)
	}
	if _, serr := os.Stat(marker); serr == nil {
		t.Fatal("planted repo config ran")
	}
}

// Symlinks at the target or at .waggle/worktrees are refused.
func TestSymlinkedWorktreeRefused(t *testing.T) {
	dir, g := newRepo(t)
	base, _ := ResolveBase(context.Background(), g, "", "")
	sh(t, dir, "branch", "t-023", base)
	req := mustValid(t, okReq(dir))
	ctx := context.Background()

	real := filepath.Join(t.TempDir(), "real")
	sh(t, dir, "worktree", "add", real, "t-023")
	wdir := filepath.Join(dir, ".waggle/worktrees")
	if err := os.MkdirAll(wdir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(wdir, "T-023-builder")); err != nil {
		t.Fatal(err)
	}
	if _, err := Provision(ctx, g, req, base, t0); code(err) != ExitEnv {
		t.Fatalf("symlinked target: %v", err)
	}
	if _, serr := os.Stat(filepath.Join(wdir, "T-023-builder.lock")); serr == nil {
		t.Fatal("lock left behind")
	}

	if err := os.RemoveAll(wdir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), wdir); err != nil {
		t.Fatal(err)
	}
	if _, err := Provision(ctx, g, req, base, t0); code(err) != ExitEnv {
		t.Fatalf("symlinked worktrees dir: %v", err)
	}
}

// A clone elsewhere on a same-named branch is not the registered worktree.
func TestForeignSameBranchRefused(t *testing.T) {
	dir, g := newRepo(t)
	base, _ := ResolveBase(context.Background(), g, "", "")
	sh(t, dir, "branch", "t-023", base)
	other := t.TempDir()
	sh(t, other, "clone", "-q", dir, "c")
	quietGit(t, filepath.Join(other, "c"))
	target := filepath.Join(dir, ".waggle/worktrees/T-023-builder")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(other, "c"), target); err != nil {
		t.Fatal(err)
	}
	sh(t, target, "checkout", "-q", "-B", "t-023")
	if _, err := Provision(context.Background(), g, mustValid(t, okReq(dir)), base, t0); code(err) != ExitEnv {
		t.Fatalf("foreign repo reused: %v", err)
	}
}

// Of many dispatchers that see the same stale lock, exactly one wins.
func TestStaleLockTakeoverIsAtomic(t *testing.T) {
	old := pidAlive
	t.Cleanup(func() { pidAlive = old })
	self := os.Getpid()
	pidAlive = func(pid int) bool { return pid == self }
	host, _ := os.Hostname()
	for round := 0; round < 20; round++ {
		lock := filepath.Join(t.TempDir(), "T-023-builder.lock")
		stale := `{"pid":4242,"host":"` + host + `","started":"2026-10-18T10:00:00Z"}`
		if err := os.WriteFile(lock, []byte(stale), 0o600); err != nil {
			t.Fatal(err)
		}
		const n = 8
		var wg sync.WaitGroup
		errs := make([]error, n)
		start := make(chan struct{})
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[i] = takeLock(lock, t0)
			}()
		}
		close(start)
		wg.Wait()
		wins := 0
		for _, e := range errs {
			if e == nil {
				wins++
			}
		}
		if wins != 1 {
			t.Fatalf("round %d: %d dispatchers won the stale lock: %v", round, wins, errs)
		}
		if left, _ := filepath.Glob(lock + ".stale.*"); len(left) != 0 {
			t.Fatalf("stale files left: %v", left)
		}
	}
}

// In remote mode the notes come from the remote claim, not a stale mirror.
func TestAcquireClaimRemoteNotes(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	origin := t.TempDir()
	sh(t, origin, "init", "-q", "--bare", "-b", "main")
	quietGit(t, origin)
	mk := func() string {
		d := t.TempDir()
		sh(t, d, "init", "-q", "-b", "main")
		quietGit(t, d)
		sh(t, d, "remote", "add", "origin", origin)
		return d
	}
	now := t0
	backend := func(d string) *comb.GitBackend {
		b, err := comb.NewGitBackend(comb.Options{RepoDir: d, Remote: "origin", DefaultTTL: time.Hour,
			Now: func() time.Time { return now }, ExtraEnv: []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	da, db := mk(), mk()
	ba, bb := backend(da), backend(db)
	ctx := context.Background()
	req := mustValid(t, okReq(da))
	if _, err := AcquireClaim(ctx, ba, &Git{Dir: da}, req, "alice", now); err != nil {
		t.Fatal(err)
	}
	// Bob's mirror has never fetched; after expiry he takes over and is told.
	now = t0.Add(3 * time.Hour)
	c, err := AcquireClaim(ctx, bb, &Git{Dir: db}, req, "bob", now)
	if err != nil || c.Verb != "takeover" || len(c.Notes) != 1 || !strings.Contains(c.Notes[0], "alice") {
		t.Fatalf("takeover: %+v %v", c, err)
	}
	c, err = AcquireClaim(ctx, bb, &Git{Dir: db}, req, "bob", now)
	if err != nil || c.Verb != "renew" || len(c.Notes) != 0 {
		t.Fatalf("renew: %+v %v", c, err)
	}
}

// A read path that git echoes back differently is never mislabeled.
func TestBlobAtChecksReturnedPath(t *testing.T) {
	dir, g := newRepo(t)
	oid := sh(t, dir, "rev-parse", "HEAD")
	ctx := context.Background()
	for _, p := range []string{".", ":(top)internal/x/a.go", ":/internal/x/a.go", "internal/x"} {
		if data, found, err := blobAt(ctx, g, oid, p, 1<<10); err == nil {
			t.Fatalf("%q accepted: %q found=%v", p, data, found)
		}
	}
	if data, found, err := blobAt(ctx, g, oid, "internal/x/a.go", 1<<10); err != nil || !found || string(data) != "package x\n" {
		t.Fatalf("plain path: %q %v %v", data, found, err)
	}
	if _, found, err := blobAt(ctx, g, oid, "nope", 1<<10); err != nil || found {
		t.Fatalf("absent: %v %v", found, err)
	}
}

// Dispatch's own git calls run with hooks and fsmonitor off.
func TestGitCallsDisableHooksAndFsmonitor(t *testing.T) {
	dir, g := newRepo(t)
	marker := filepath.Join(t.TempDir(), "ran")
	sh(t, dir, "config", "core.fsmonitor", "touch "+marker)
	if _, err := g.Run(context.Background(), "status", "--porcelain"); err != nil {
		t.Fatal(err)
	}
	if _, serr := os.Stat(marker); serr == nil {
		t.Fatal("fsmonitor ran")
	}
	hook := filepath.Join(dir, ".git/hooks/reference-transaction")
	write(t, dir, ".git/hooks/reference-transaction", "#!/bin/sh\ntouch "+marker+"\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Run(context.Background(), "update-ref", "refs/heads/x", "HEAD"); err != nil {
		t.Fatal(err)
	}
	if _, serr := os.Stat(marker); serr == nil {
		t.Fatal("hook ran")
	}
}
