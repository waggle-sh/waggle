//go:build unix

package dispatch

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func provisionAt(t *testing.T, dir string, g *Git, role, head string) (*Worktree, error) {
	t.Helper()
	base, err := ResolveBase(context.Background(), g, "", "")
	if err != nil {
		t.Fatal(err)
	}
	req := mustValid(t, okReq(dir))
	req.Role, req.Head = role, head
	return Provision(context.Background(), g, req, base, t0)
}

// D2b-T5: provision never prunes. T-024's stale entry stays, T-023's own stale
// entry is exit 3 with the clean hint, and Clean lets provision through.
func TestProvisionLeavesOtherStaleEntriesAndHintsClean(t *testing.T) {
	dir, g := newRepo(t)
	trace := filepath.Join(t.TempDir(), "trace")
	t.Setenv("GIT_TRACE", trace)
	w23 := filepath.Join(dir, ".waggle/worktrees/T-023-builder")
	w24 := filepath.Join(dir, ".waggle/worktrees/T-024-builder")
	sh(t, dir, "worktree", "add", "-q", "-b", "t-024", w24, "main")
	w, err := provisionAt(t, dir, g, "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Unlock()
	if os.RemoveAll(w23) != nil || os.RemoveAll(w24) != nil {
		t.Fatal("cannot remove the worktrees")
	}
	common := filepath.Join(dir, ".git")
	_, err = provisionAt(t, dir, g, "builder", "")
	if code(err) != ExitEnv || err == nil || !strings.Contains(err.Error(), "waggle dispatch clean T-023 --role builder") {
		t.Fatalf("stale own entry: %v", err)
	}
	if !exists(filepath.Join(common, "worktrees/T-024-builder")) {
		t.Fatal("provision dropped T-024's stale entry")
	}
	if b, _ := os.ReadFile(trace); strings.Contains(string(b), "worktree prune") {
		t.Fatal("provision ran worktree prune")
	}
	if err := Clean(context.Background(), CleanRequest{RepoDir: dir, Ticket: "23"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	w, err = provisionAt(t, dir, g, "builder", "")
	if err != nil {
		t.Fatalf("after clean: %v", err)
	}
	_ = w.Unlock()
	if !exists(filepath.Join(common, "worktrees/T-024-builder")) {
		t.Fatal("T-024's stale entry is gone")
	}
}

// D2b-T5: a smudge filter in the Keeper's global config never runs during
// provision, whichever worktree add form is used.
func TestProvisionRunsNoGlobalFilter(t *testing.T) {
	for _, form := range []string{"new branch", "existing branch", "remote branch", "detach"} {
		t.Run(form, func(t *testing.T) {
			dir, g := newRepo(t)
			write(t, dir, ".gitattributes", "* filter=canary\n")
			write(t, dir, "f.txt", "content\n")
			sh(t, dir, "add", "-A")
			sh(t, dir, "commit", "-q", "-m", "attrs")
			canary := filepath.Join(t.TempDir(), "canary")
			global := filepath.Join(t.TempDir(), "gitconfig")
			if err := os.WriteFile(global, []byte("[filter \"canary\"]\n\tsmudge = \"sh -c 'touch "+canary+"; cat'\"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GIT_CONFIG_GLOBAL", global)
			head := ""
			switch form {
			case "existing branch":
				sh(t, dir, "branch", "t-023", "main")
			case "remote branch":
				sh(t, dir, "update-ref", "refs/remotes/origin/t-023", "main")
				g.Remote = "origin"
			case "detach":
				head = strings.TrimSpace(sh(t, dir, "rev-parse", "HEAD"))
			}
			role := "builder"
			if head != "" {
				role = "guard"
			}
			w, err := provisionAt(t, dir, g, role, head)
			if err != nil {
				t.Fatal(err)
			}
			_ = w.Unlock()
			if !exists(filepath.Join(w.Path, "f.txt")) {
				t.Fatal("the worktree was not checked out")
			}
			if exists(canary) {
				t.Fatal("the global smudge filter ran on the host")
			}
		})
	}
}
