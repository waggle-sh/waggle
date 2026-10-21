package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// registration is <common>/worktrees/T-023-builder for the repo at dir.
func registration(t *testing.T, dir string) string {
	t.Helper()
	common := strings.TrimSpace(sh(t, dir, "rev-parse", "--path-format=absolute", "--git-common-dir"))
	return filepath.Join(common, "worktrees", "T-023-builder")
}

// filterRepo makes a repo whose clean filter touches canary, tracking a
// .gitattributes that selects it. It returns the repo's gitdir.
func filterRepo(t *testing.T, canary string) string {
	t.Helper()
	r := filepath.Join(t.TempDir(), "foreign")
	if err := os.MkdirAll(r, 0o755); err != nil {
		t.Fatal(err)
	}
	sh(t, r, "init", "-q", "-b", "t-023")
	write(t, r, ".gitattributes", "* filter=x\n")
	sh(t, r, "add", "-A")
	sh(t, r, "commit", "-q", "-m", "f")
	sh(t, r, "config", "filter.x.clean", "sh -c 'touch "+canary+"; cat'")
	return filepath.Join(r, ".git")
}

// dirtyViaFilter commits a .gitattributes selecting filter x in wt and makes it
// stat-dirty, so a status that loads a foreign config hashes it through x.
func dirtyViaFilter(t *testing.T, wt string) {
	t.Helper()
	write(t, wt, ".gitattributes", "* filter=x\n")
	sh(t, wt, "add", ".gitattributes")
	sh(t, wt, "commit", "-q", "-m", "attr")
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(filepath.Join(wt, ".gitattributes"), future, future)
}

func repoOfWorktree(wt string) string { return filepath.Dir(filepath.Dir(filepath.Dir(wt))) }

func TestReuseRefusesCommondirRewrite(t *testing.T) {
	err, canary := reuseAfter(t, func(wt, canary string) {
		dirtyViaFilter(t, wt)
		foreign := filterRepo(t, canary)
		admin := registration(t, repoOfWorktree(wt))
		if err := os.WriteFile(filepath.Join(admin, "commondir"), []byte(foreign+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	if code(err) != ExitEnv || !strings.Contains(err.Error(), "commondir") {
		t.Fatalf("rewritten commondir accepted: %v", err)
	}
	if _, e := os.Stat(canary); e == nil {
		t.Fatal("clean filter of the foreign gitdir ran on the host")
	}
}

// extraRegistrationRefused plants a second registration AAA for the worktree
// path, with its gitdir file written by gitdirOf, and points the worktree's
// .git at it. Depending on how prune treats the planted entry, the reuse guard
// refuses on the registration count or on the commondir pin; either is a
// refusal, and the foreign clean filter must never run.
func extraRegistrationRefused(t *testing.T, gitdirOf func(wt, aaa string) string) {
	t.Helper()
	err, canary := reuseAfter(t, func(wt, canary string) {
		dirtyViaFilter(t, wt)
		aaa := filepath.Join(filepath.Dir(registration(t, repoOfWorktree(wt))), "AAA")
		if err := os.Rename(filterRepo(t, canary), aaa); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(aaa, "gitdir"), []byte(gitdirOf(wt, aaa)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+aaa+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("refusal: %v", err)
	if code(err) != ExitEnv || !(strings.Contains(err.Error(), "registrations") || strings.Contains(err.Error(), "commondir")) {
		t.Fatalf("second registration for the path accepted: %v", err)
	}
	if _, e := os.Stat(canary); e == nil {
		t.Fatal("clean filter of the extra registration ran on the host")
	}
}

func TestReuseRefusesExtraRegistration(t *testing.T) {
	// Byte-identical to the real registration: provision runs no prune, so the
	// real entry stays and both entries are refused (registrations refusal).
	t.Run("absolute", func(t *testing.T) {
		extraRegistrationRefused(t, func(wt, aaa string) string { return filepath.Join(wt, ".git") })
	})
	// A relative spelling of the same path, so the outcome does not hinge on
	// the OS temp directory layout.
	t.Run("relative", func(t *testing.T) {
		extraRegistrationRefused(t, func(wt, aaa string) string {
			rel, err := filepath.Rel(aaa, filepath.Join(wt, ".git"))
			if err != nil {
				t.Fatal(err)
			}
			return rel
		})
	})
}

// The filter canary only proves the refusals above if the same dirty state
// reaches the filter when nothing is tampered with.
func TestFilterCanaryFiresWithoutTamper(t *testing.T) {
	dir, g := newRepo(t)
	ctx := context.Background()
	base, _ := ResolveBase(ctx, g, "", "")
	w, err := Provision(ctx, g, mustValid(t, okReq(dir)), base, t0)
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Unlock()
	canary := filepath.Join(t.TempDir(), "filter-ran")
	sh(t, dir, "config", "filter.x.clean", "sh -c 'touch "+canary+"; cat'")
	dirtyViaFilter(t, w.Path)
	_ = os.Remove(canary)
	_ = sh(t, w.Path, "status", "--porcelain")
	if _, err := os.Stat(canary); err != nil {
		t.Fatal("filter did not fire on the control path; the canary proves nothing")
	}
}

func TestReuseAcceptsRelativePathWorktree(t *testing.T) {
	dir, g := newRepo(t)
	sh(t, dir, "config", "worktree.useRelativePaths", "true")
	ctx := context.Background()
	base, _ := ResolveBase(ctx, g, "", "")
	w, err := Provision(ctx, g, mustValid(t, okReq(dir)), base, t0)
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Unlock()
	reg, _ := os.ReadFile(filepath.Join(registration(t, dir), "gitdir"))
	if filepath.IsAbs(strings.TrimSpace(string(reg))) {
		t.Fatalf("git older than 2.48 ignores worktree.useRelativePaths and writes an absolute registration; this test requires git 2.48 or newer (CI runs 2.55)")
	}
	// A different working directory makes a cwd-relative resolve fail.
	old, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	w2, err := Provision(ctx, g, mustValid(t, okReq(dir)), base, t0)
	if err != nil {
		t.Fatalf("honest relative-path worktree refused: %v", err)
	}
	_ = w2.Unlock()
	if !w2.Reused {
		t.Fatal("worktree not reused")
	}
}
