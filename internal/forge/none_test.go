package forge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// requireGit skips the test when git is not on PATH and isolates git from the
// developer's configuration for the rest of the test.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME": "test", "GIT_AUTHOR_EMAIL": "test@localhost",
		"GIT_COMMITTER_NAME": "test", "GIT_COMMITTER_EMAIL": "test@localhost",
	} {
		t.Setenv(k, v)
	}
}

// runGit runs git in dir for test setup and inspection.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, errb.String())
	}
	return strings.TrimSpace(out.String())
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepo returns a work tree where main and feat diverged from one root
// commit. With conflict both branches change the same file.
func newRepo(t *testing.T, conflict bool) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	writeFile(t, dir, "f", "root\n")
	runGit(t, dir, "add", "f")
	runGit(t, dir, "commit", "-q", "-m", "root")
	runGit(t, dir, "branch", "feat")
	if conflict {
		writeFile(t, dir, "f", "main side\n")
	} else {
		writeFile(t, dir, "m", "main side\n")
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "main change")
	runGit(t, dir, "checkout", "-q", "feat")
	if conflict {
		writeFile(t, dir, "f", "feat side\n")
	} else {
		writeFile(t, dir, "h", "feat side\n")
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "feat change")
	runGit(t, dir, "checkout", "-q", "main")
	return dir
}

func newNone(t *testing.T, dir, remote string) *none {
	t.Helper()
	f, err := New("none", Options{RepoDir: dir, Remote: remote})
	if err != nil {
		t.Fatal(err)
	}
	return f.(*none)
}

func TestNoneOpenChange(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	n := newNone(t, dir, "")
	ctx := context.Background()
	c, err := n.OpenChange(ctx, ChangeRequest{Head: "feat", Base: "main", Title: "T-001: work", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if c != (Change{Head: "feat", Base: "main"}) {
		t.Fatalf("got %+v, want the branches only", c)
	}
	bad := []ChangeRequest{
		{Head: "bad..name", Base: "main", Title: "t"},
		{Head: "-x", Base: "main", Title: "t"},
		{Head: "", Base: "main", Title: "t"},
		{Head: "feat", Base: "a b", Title: "t"},
		{Head: "feat", Base: "main", Title: ""},
		{Head: "feat", Base: "main", Title: "two\nlines"},
		{Head: "feat", Base: "main", Title: strings.Repeat("x", 257)},
		{Head: "feat", Base: "main", Title: "t", Body: strings.Repeat("x", 64<<10+1)},
	}
	for _, req := range bad {
		if _, err := n.OpenChange(ctx, req); err == nil || !strings.HasPrefix(err.Error(), "forge: open change: ") {
			t.Errorf("%q/%q/%q: got %v, want an open change error", req.Head, req.Base, req.Title, err)
		}
	}
	if n.Name() != "none" {
		t.Errorf("Name() = %q", n.Name())
	}
	if _, ok := n.CIShim(); ok {
		t.Error("CIShim() reported a shim")
	}
}

func TestNoneMergeWritesMergeCommit(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	n := newNone(t, dir, "")
	base, head := runGit(t, dir, "rev-parse", "main"), runGit(t, dir, "rev-parse", "feat")
	indexBefore := runGit(t, dir, "write-tree")
	req := MergeRequest{Subject: "Merge T-001: work", Body: "Intent: do the thing\n\nTicket: T-001", ExpectedHeadOID: head}
	if err := n.Merge(context.Background(), Change{Head: "feat", Base: "main"}, req); err != nil {
		t.Fatal(err)
	}
	merged := runGit(t, dir, "rev-parse", "main")
	if got, want := runGit(t, dir, "rev-list", "--parents", "-n1", "main"), merged+" "+base+" "+head; got != want {
		t.Errorf("parents: got %q, want %q", got, want)
	}
	if got := runGit(t, dir, "log", "-1", "--format=%s", "main"); got != req.Subject {
		t.Errorf("subject: got %q", got)
	}
	if got := runGit(t, dir, "log", "-1", "--format=%b", "main"); got != req.Body {
		t.Errorf("body: got %q, want %q", got, req.Body)
	}
	if got := runGit(t, dir, "write-tree"); got != indexBefore {
		t.Errorf("index changed: %s to %s", indexBefore, got)
	}
	if _, err := os.Stat(filepath.Join(dir, "h")); err == nil {
		t.Error("working tree changed: h appeared")
	}
	if runGit(t, dir, "symbolic-ref", "HEAD") != "refs/heads/main" {
		t.Error("HEAD moved")
	}
	if runGit(t, dir, "rev-parse", "feat") != head {
		t.Error("feat moved")
	}
}

func TestNoneMergeHeadMoved(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	n := newNone(t, dir, "")
	ctx := context.Background()
	base, head := runGit(t, dir, "rev-parse", "main"), runGit(t, dir, "rev-parse", "feat")
	c := Change{Head: "feat", Base: "main"}
	err := n.Merge(ctx, c, MergeRequest{Subject: "s", ExpectedHeadOID: base})
	if !errors.Is(err, ErrHeadMoved) {
		t.Fatalf("stale ExpectedHeadOID: got %v, want ErrHeadMoved", err)
	}
	if runGit(t, dir, "rev-parse", "main") != base {
		t.Fatal("main changed after a refused merge")
	}
	n.beforeWrite = func() { runGit(t, dir, "update-ref", "refs/heads/main", head) }
	err = n.Merge(ctx, c, MergeRequest{Subject: "s", ExpectedHeadOID: head})
	if !errors.Is(err, ErrHeadMoved) {
		t.Fatalf("base moved during merge: got %v, want ErrHeadMoved", err)
	}
	if runGit(t, dir, "rev-parse", "main") != head {
		t.Fatal("main changed after a lost race")
	}
}

func TestNoneMergeConflictAndUnsupported(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, true)
	n := newNone(t, dir, "")
	ctx := context.Background()
	base, head := runGit(t, dir, "rev-parse", "main"), runGit(t, dir, "rev-parse", "feat")
	c := Change{Head: "feat", Base: "main"}
	err := n.Merge(ctx, c, MergeRequest{Subject: "s", ExpectedHeadOID: head})
	if !errors.Is(err, ErrNotMergeable) || !strings.Contains(err.Error(), "conflicts in f:") {
		t.Fatalf("conflict: got %v, want ErrNotMergeable naming f", err)
	}
	if runGit(t, dir, "rev-parse", "main") != base {
		t.Fatal("main changed after a conflict")
	}
	if err := n.Merge(ctx, c, MergeRequest{Subject: "s", ExpectedHeadOID: head, Auto: true}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Auto: got %v, want ErrUnsupported", err)
	}
	if err := n.Comment(ctx, c, "hello"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Comment: got %v, want ErrUnsupported", err)
	}
}

func TestNoneMergePushesWithLease(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	setup := func() (dir, bare string) {
		dir, bare = newRepo(t, false), t.TempDir()
		runGit(t, bare, "init", "-q", "--bare")
		runGit(t, dir, "remote", "add", "origin", bare)
		runGit(t, dir, "push", "-q", "origin", "main", "feat")
		return dir, bare
	}
	dir, bare := setup()
	head := runGit(t, dir, "rev-parse", "feat")
	n := newNone(t, dir, "origin")
	if err := n.Merge(ctx, Change{Head: "feat", Base: "main"}, MergeRequest{Subject: "s", ExpectedHeadOID: head}); err != nil {
		t.Fatal(err)
	}
	if local, remote := runGit(t, dir, "rev-parse", "main"), runGit(t, bare, "rev-parse", "main"); local != remote {
		t.Fatalf("remote main %s, local main %s", remote, local)
	}

	dir, bare = setup()
	base, head := runGit(t, dir, "rev-parse", "main"), runGit(t, dir, "rev-parse", "feat")
	other := runGit(t, dir, "commit-tree", base+"^{tree}", "-p", base, "-m", "someone else")
	runGit(t, dir, "push", "-q", "origin", other+":refs/heads/main")
	n = newNone(t, dir, "origin")
	err := n.Merge(ctx, Change{Head: "feat", Base: "main"}, MergeRequest{Subject: "s", ExpectedHeadOID: head})
	if !errors.Is(err, ErrHeadMoved) {
		t.Fatalf("concurrent remote push: got %v, want ErrHeadMoved", err)
	}
	if got := runGit(t, bare, "rev-parse", "main"); got != other {
		t.Errorf("remote main overwritten: %s", got)
	}
	if got := runGit(t, dir, "rev-parse", "main"); got != base {
		t.Errorf("local main not restored: %s", got)
	}
}

func TestNoneMergeUnreachableRemote(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	runGit(t, dir, "remote", "add", "origin", filepath.Join(t.TempDir(), "gone"))
	base, head := runGit(t, dir, "rev-parse", "main"), runGit(t, dir, "rev-parse", "feat")
	n := newNone(t, dir, "origin")
	err := n.Merge(context.Background(), Change{Head: "feat", Base: "main"}, MergeRequest{Subject: "s", ExpectedHeadOID: head})
	if err == nil || errors.Is(err, ErrHeadMoved) {
		t.Fatalf("unreachable remote: got %v, want a plain push error", err)
	}
	merged := runGit(t, dir, "rev-parse", "main")
	if merged == base {
		t.Fatal("local main was not advanced")
	}
	if want := "local refs/heads/main advanced to " + merged + " but the push to origin did not: git push:"; !strings.Contains(err.Error(), want) {
		t.Errorf("got %q, want it to contain %q", err, want)
	}
}

// TestNoneMergeIgnoresInheritedGitDir proves an inherited GIT_DIR (a git hook
// starting waggled) cannot redirect the merge into another repository.
func TestNoneMergeIgnoresInheritedGitDir(t *testing.T) {
	requireGit(t)
	dir, decoy := newRepo(t, false), newRepo(t, false)
	runGit(t, decoy, "commit", "-q", "--allow-empty", "-m", "decoy") // distinct OIDs from dir
	decoyBase, base, head := runGit(t, decoy, "rev-parse", "main"), runGit(t, dir, "rev-parse", "main"), runGit(t, dir, "rev-parse", "feat")
	t.Setenv("GIT_DIR", filepath.Join(decoy, ".git"))
	t.Setenv("git_work_tree", decoy)
	n := newNone(t, dir, "")
	if err := n.Merge(context.Background(), Change{Head: "feat", Base: "main"}, MergeRequest{Subject: "s", ExpectedHeadOID: head}); err != nil {
		t.Fatal(err)
	}
	os.Unsetenv("GIT_DIR")
	os.Unsetenv("git_work_tree")
	if got := runGit(t, decoy, "rev-parse", "main"); got != decoyBase {
		t.Errorf("decoy main moved to %s", got)
	}
	merged := runGit(t, dir, "rev-parse", "main")
	if got, want := runGit(t, dir, "rev-list", "--parents", "-n1", "main"), merged+" "+base+" "+head; got != want {
		t.Errorf("parents: got %q, want %q", got, want)
	}
	env := childEnv([]string{"GIT_DIR=x", "Git_Namespace=y", "GIT_CONFIG_GLOBAL=z", "PATH=p", "GIT_NO_REPLACE_OBJECTS=0"})
	if want := []string{"GIT_CONFIG_GLOBAL=z", "PATH=p", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "GIT_NO_REPLACE_OBJECTS=1"}; !slices.Equal(env, want) {
		t.Errorf("childEnv: got %q, want %q", env, want)
	}
}

func TestNoneProbe(t *testing.T) {
	for _, remote := range []string{"", "origin"} {
		r, err := (&none{remote: remote}).Probe(context.Background(), ProbeOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Provider != "none" || r.Remote != remote || len(r.Findings) != 1 || r.Findings[0].ID != "code-host" {
			t.Errorf("remote %q: report %+v", remote, r)
		}
	}
}

func TestNew(t *testing.T) {
	if _, err := New("none", Options{RepoDir: "."}); err != nil {
		t.Errorf("none: %v", err)
	}
	if f, err := New("github", Options{RepoDir: "."}); !errors.Is(err, ErrForgeRemote) || f != nil {
		t.Errorf("github without a remote: got %v, %v, want a nil Forge and ErrForgeRemote", f, err)
	}
	for _, o := range []struct{ provider, dir, remote string }{{"gitlab", ".", ""}, {"none", "", ""}, {"none", ".", "-bad"}} {
		if _, err := New(o.provider, Options{RepoDir: o.dir, Remote: o.remote}); err == nil {
			t.Errorf("%+v: want an error", o)
		}
	}
}
