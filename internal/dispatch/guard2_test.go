package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// reuseAfter provisions, unlocks, runs tamper on the worktree path and
// provisions again. It returns the second error and the filter canary path.
func reuseAfter(t *testing.T, tamper func(wt, canary string)) (error, string) {
	t.Helper()
	dir, g := newRepo(t)
	ctx := context.Background()
	base, _ := ResolveBase(ctx, g, "", "")
	w, err := Provision(ctx, g, mustValid(t, okReq(dir)), base, t0)
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Unlock()
	canary := filepath.Join(t.TempDir(), "filter-ran")
	tamper(w.Path, canary)
	w2, err := Provision(ctx, g, mustValid(t, okReq(dir)), base, t0)
	if w2 != nil {
		_ = w2.Unlock()
	}
	return err, canary
}

// plant makes a foreign repo at path with a clean filter that touches canary
// and one tracked file whose mtime is newer than any index entry.
func plant(t *testing.T, path, canary string) {
	t.Helper()
	write(t, path, ".gitattributes", "* filter=x\n")
	write(t, path, "f", "x")
	sh(t, path, "add", "-A")
	sh(t, path, "commit", "-q", "-m", "foreign")
	sh(t, path, "config", "filter.x.clean", "sh -c 'touch "+canary+"; cat'")
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(filepath.Join(path, "f"), future, future)
}

func TestReuseRefusesSeparateGitDirPlant(t *testing.T) {
	err, canary := reuseAfter(t, func(wt, canary string) {
		_ = os.RemoveAll(wt)
		evil := filepath.Join(t.TempDir(), "evil.git")
		sh(t, filepath.Dir(wt), "init", "-q", "-b", "t-023", "--separate-git-dir", evil, filepath.Base(wt))
		plant(t, wt, canary)
	})
	if code(err) != ExitEnv {
		t.Fatalf("planted separate-git-dir repo reused: %v", err)
	}
	if _, e := os.Stat(canary); e == nil {
		t.Fatal("clean filter of the planted repo ran on the host")
	}
}

func TestReuseRefusesInWorktreeGitFileRewrite(t *testing.T) {
	err, canary := reuseAfter(t, func(wt, canary string) {
		scratch := filepath.Join(t.TempDir(), "s")
		sh(t, filepath.Dir(scratch), "init", "-q", "-b", "t-023", filepath.Base(scratch))
		plant(t, scratch, canary)
		_ = os.WriteFile(filepath.Join(scratch, ".git/info/exclude"), []byte(".evil\n"), 0o644)
		for _, n := range []string{".waggle", "internal", ".gitignore", ".git"} {
			_ = os.RemoveAll(filepath.Join(wt, n))
		}
		if err := os.Rename(filepath.Join(scratch, ".git"), filepath.Join(wt, ".evil")); err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: .evil\n"), 0o644)
		for _, f := range []string{".gitattributes", "f"} {
			b, _ := os.ReadFile(filepath.Join(scratch, f))
			_ = os.WriteFile(filepath.Join(wt, f), b, 0o644)
		}
		future := time.Now().Add(time.Hour)
		_ = os.Chtimes(filepath.Join(wt, "f"), future, future)
	})
	if code(err) != ExitEnv {
		t.Fatalf("rewritten .git file reused: %v", err)
	}
	if _, e := os.Stat(canary); e == nil {
		t.Fatal("clean filter of the planted repo ran on the host")
	}
}

func TestReuseAcceptsHonestWorktree(t *testing.T) {
	if err, _ := reuseAfter(t, func(string, string) {}); err != nil {
		t.Fatal(err)
	}
}

func TestNewRunDirRefusesSymlinkedRuns(t *testing.T) {
	dir, _ := newRepo(t)
	elsewhere := t.TempDir()
	wt := filepath.Join(dir, ".waggle/worktrees")
	if err := os.MkdirAll(wt, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(wt, "runs")); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRunDir(dir, "T-023", "builder"); code(err) != ExitEnv {
		t.Fatalf("symlinked runs accepted: %v", err)
	}
	if ents, _ := os.ReadDir(elsewhere); len(ents) != 0 {
		t.Fatal("run record landed in the link target")
	}
}
