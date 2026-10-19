package dispatch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/runtime"
)

var t0 = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

// sh runs git in dir with an isolated config and returns trimmed stdout.
func sh(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return shIn(t, dir, "", args...)
}

func shIn(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepo returns a repo on main holding a brief, role prompt and WAGGLE.md.
func newRepo(t *testing.T) (dir string, g *Git) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir = t.TempDir()
	sh(t, dir, "init", "-q", "-b", "main")
	write(t, dir, ".waggle/hive/tasks/T-023.md", "# brief at base\n")
	write(t, dir, ".waggle/control/roles/builder.md", "builder role\n")
	write(t, dir, ".waggle/control/WAGGLE.md", "waggle md\n")
	write(t, dir, "internal/x/a.go", "package x\n")
	write(t, dir, ".gitignore", ".waggle/worktrees/\n")
	sh(t, dir, "add", "-A")
	sh(t, dir, "commit", "-q", "-m", "init")
	return dir, &Git{Dir: dir}
}

func okReq(dir string) Request {
	return Request{RepoDir: dir, Ticket: "23", Role: "builder", Dirs: []string{"internal/x"}, Gate: "full"}
}

func code(err error) int { return ExitCode(err) }

func TestValidate(t *testing.T) {
	tier := comb.RiskTier(4)
	cases := []struct {
		name string
		mut  func(*Request)
		want int
	}{
		{"ok", func(r *Request) {}, 0},
		{"ticket T-23", func(r *Request) { r.Ticket = "T-23" }, 0},
		{"ticket bad", func(r *Request) { r.Ticket = "abc" }, ExitUsage},
		{"ticket zero", func(r *Request) { r.Ticket = "0" }, ExitUsage},
		{"role queen", func(r *Request) { r.Role = "queen" }, ExitUsage},
		{"role scout", func(r *Request) { r.Role = "scout" }, ExitUsage},
		{"role waggled", func(r *Request) { r.Role = "waggled" }, ExitUsage},
		{"role weaver", func(r *Request) { r.Role = "weaver" }, ExitUsage},
		{"guard ok", func(r *Request) { r.Role = "guard"; r.Branch = "t-023" }, 0},
		{"dir hive", func(r *Request) { r.Dirs = []string{".waggle/hive"} }, 0}, // refused by the Comb at claim time
		{"dir root", func(r *Request) { r.Dirs = []string{"."} }, ExitClaim},
		{"dir escapes", func(r *Request) { r.Dirs = []string{"../x"} }, ExitUsage},
		{"no dirs", func(r *Request) { r.Dirs = nil }, ExitUsage},
		{"gate", func(r *Request) { r.Gate = "heavy" }, ExitUsage},
		{"branch main", func(r *Request) { r.Branch = "main" }, ExitUsage},
		{"branch other ticket", func(r *Request) { r.Branch = "t-024" }, ExitUsage},
		{"branch longer number", func(r *Request) { r.Branch = "t-0231" }, ExitUsage},
		{"branch upper", func(r *Request) { r.Branch = "T-023" }, ExitUsage},
		{"branch suffix ok", func(r *Request) { r.Branch = "t-023b" }, 0},
		{"branch dotdot", func(r *Request) { r.Branch = "t-023..x" }, ExitUsage},
		{"branch lock", func(r *Request) { r.Branch = "t-023.lock" }, ExitUsage},
		{"branch slash", func(r *Request) { r.Branch = "t-023/x" }, ExitUsage},
		{"base is branch", func(r *Request) { r.Base = "t-023" }, ExitUsage},
		{"base is refs/heads branch", func(r *Request) { r.Base = "refs/heads/t-023" }, ExitUsage},
		{"read dot", func(r *Request) { r.ReadPaths = []string{"."} }, ExitUsage},
		{"read pathspec magic", func(r *Request) { r.ReadPaths = []string{":(top)x"} }, ExitUsage},
		{"base flag", func(r *Request) { r.Base = "--upload-pack=x" }, ExitUsage},
		{"tier 4", func(r *Request) { r.Tier = &tier }, ExitUsage},
		{"read abs", func(r *Request) { r.ReadPaths = []string{"/etc/passwd"} }, ExitUsage},
		{"read dotdot", func(r *Request) { r.ReadPaths = []string{"../x"} }, ExitUsage},
		{"read ok", func(r *Request) { r.ReadPaths = []string{"internal/x/a.go"} }, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := okReq("/repo")
			c.mut(&r)
			got, err := r.Validate()
			if code(err) != c.want {
				t.Fatalf("exit %d (%v), want %d", code(err), err, c.want)
			}
			if err == nil && got.Ticket != "T-023" {
				t.Fatalf("ticket %q not canonical", got.Ticket)
			}
		})
	}
	r, _ := okReq("/repo").Validate()
	if r.Branch != "t-023" {
		t.Fatalf("default branch %q", r.Branch)
	}
}

// claimEnv builds a backend with a settable clock.
func claimEnv(t *testing.T, dir string) (*comb.GitBackend, *time.Time) {
	t.Helper()
	now := t0
	b, err := comb.NewGitBackend(comb.Options{
		RepoDir: dir, Now: func() time.Time { return now }, DefaultTTL: time.Hour,
		ExtraEnv: []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b, &now
}

func mustValid(t *testing.T, r Request) Request {
	t.Helper()
	r, err := r.Validate()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAcquireClaim(t *testing.T) {
	dir, _ := newRepo(t)
	b, now := claimEnv(t, dir)
	ctx := context.Background()
	req := mustValid(t, okReq(dir))

	c, err := AcquireClaim(ctx, b, &Git{Dir: dir}, req, "alice", *now)
	if err != nil || c.Verb != "claim" || c.OID == "" || len(c.Notes) != 0 || c.Lease.KeeperID != "alice" {
		t.Fatalf("fresh: %+v %v", c, err)
	}

	// Same Keeper renews; a changed gate prints old and new.
	req2 := req
	req2.Gate = "light"
	c, err = AcquireClaim(ctx, b, &Git{Dir: dir}, req2, "alice", *now)
	if err != nil || c.Verb != "renew" || len(c.Notes) != 1 || !strings.Contains(c.Notes[0], "full") || !strings.Contains(c.Notes[0], "light") {
		t.Fatalf("renew: %+v %v", c, err)
	}
	c, err = AcquireClaim(ctx, b, &Git{Dir: dir}, req2, "alice", *now)
	if err != nil || c.Verb != "renew" || len(c.Notes) != 0 {
		t.Fatalf("plain renew: %+v %v", c, err)
	}

	// Another Keeper's active claim: exit 4, nothing else touched.
	_, err = AcquireClaim(ctx, b, &Git{Dir: dir}, req, "bob", *now)
	if code(err) != ExitClaim || !errors.Is(err, comb.ErrAlreadyClaimed) || !strings.Contains(err.Error(), "alice") {
		t.Fatalf("held: %v", err)
	}
	if _, serr := os.Stat(filepath.Join(dir, ".waggle/worktrees")); serr == nil {
		t.Fatal("a refused claim created worktree state")
	}

	// Overlap with another ticket.
	other := mustValid(t, Request{RepoDir: dir, Ticket: "24", Role: "builder", Dirs: []string{"internal"}, Gate: "full"})
	if _, err = AcquireClaim(ctx, b, &Git{Dir: dir}, other, "bob", *now); code(err) != ExitClaim || !errors.Is(err, comb.ErrLeaseOverlap) {
		t.Fatalf("overlap: %v", err)
	}

	// Expired takeover names the previous owner.
	*now = t0.Add(3 * time.Hour)
	c, err = AcquireClaim(ctx, b, &Git{Dir: dir}, req, "bob", *now)
	if err != nil || c.Verb != "takeover" || len(c.Notes) != 1 || !strings.Contains(c.Notes[0], "alice") || c.Lease.KeeperID != "bob" {
		t.Fatalf("takeover: %+v %v", c, err)
	}

	// Tier 3 directory and guard are refused.
	for _, d := range []string{".github", ".waggle/hive"} {
		bad := mustValid(t, Request{RepoDir: dir, Ticket: "25", Role: "builder", Dirs: []string{d}, Gate: "full"})
		if _, err = AcquireClaim(ctx, b, &Git{Dir: dir}, bad, "bob", *now); code(err) != ExitClaim {
			t.Fatalf("%s: %v", d, err)
		}
		if refs := sh(t, dir, "for-each-ref", "refs/waggle/claims/ticket-25"); refs != "" {
			t.Fatalf("%s: a refused claim wrote %s", d, refs)
		}
	}
	g := req
	g.Role = "guard"
	if _, err = AcquireClaim(ctx, b, &Git{Dir: dir}, g, "bob", *now); code(err) != ExitUsage {
		t.Fatalf("guard: %v", err)
	}
}

func TestAcquireClaimMalformed(t *testing.T) {
	dir, _ := newRepo(t)
	b, now := claimEnv(t, dir)
	blob := shIn(t, dir, "not json", "hash-object", "-w", "--stdin")
	tree := shIn(t, dir, "100644 blob "+blob+"\tlease.json\n", "mktree")
	commit := sh(t, dir, "commit-tree", tree, "-m", "junk")
	sh(t, dir, "update-ref", "refs/waggle/claims/ticket-23", commit)
	_, err := AcquireClaim(context.Background(), b, &Git{Dir: dir}, mustValid(t, okReq(dir)), "alice", *now)
	if code(err) != ExitClaim || !errors.Is(err, comb.ErrMalformedClaim) || !strings.Contains(err.Error(), "force-release") {
		t.Fatalf("malformed: %v", err)
	}
	if got := sh(t, dir, "rev-parse", "refs/waggle/claims/ticket-23"); got != commit {
		t.Fatal("malformed claim was overwritten")
	}
}

func TestProvisionNewBranch(t *testing.T) {
	dir, g := newRepo(t)
	ctx := context.Background()
	base, err := ResolveBase(ctx, g, "", "")
	if err != nil {
		t.Fatal(err)
	}
	w, err := Provision(ctx, g, mustValid(t, okReq(dir)), base, t0)
	if err != nil {
		t.Fatal(err)
	}
	if w.Reused || w.CheckBase != base || w.Branch != "t-023" || !strings.HasSuffix(w.Path, "T-023-builder") {
		t.Fatalf("%+v", w)
	}
	if got := sh(t, w.Path, "rev-parse", "HEAD"); got != base {
		t.Fatalf("HEAD %s, base %s", got, base)
	}
	if got := sh(t, w.Path, "symbolic-ref", "--short", "HEAD"); got != "t-023" {
		t.Fatalf("branch %s", got)
	}
	if _, err := os.Stat(w.LockPath); err != nil {
		t.Fatal("lock missing")
	}
	// A second run while the lock is live: exit 3.
	if _, err := Provision(ctx, g, mustValid(t, okReq(dir)), base, t0); code(err) != ExitEnv || !strings.Contains(err.Error(), "run in progress") {
		t.Fatalf("live lock: %v", err)
	}
	if err := w.Unlock(); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove(ctx, g); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.Path); err == nil {
		t.Fatal("worktree not removed")
	}
	if sh(t, dir, "rev-parse", "--verify", "refs/heads/t-023") != base {
		t.Fatal("branch must stay")
	}
}

func TestProvisionExistingBranch(t *testing.T) {
	dir, g := newRepo(t)
	ctx := context.Background()
	oldBase := sh(t, dir, "rev-parse", "HEAD")
	sh(t, dir, "branch", "t-023", oldBase)
	wt := t.TempDir()
	sh(t, dir, "worktree", "add", filepath.Join(wt, "tmp"), "t-023")
	write(t, filepath.Join(wt, "tmp"), "internal/x/b.go", "package x\n")
	sh(t, filepath.Join(wt, "tmp"), "add", "-A")
	sh(t, filepath.Join(wt, "tmp"), "commit", "-q", "-m", "T-023: work")
	sh(t, dir, "worktree", "remove", filepath.Join(wt, "tmp"))
	write(t, dir, "README", "later\n")
	sh(t, dir, "add", "-A")
	sh(t, dir, "commit", "-q", "-m", "main moves")
	newBase := sh(t, dir, "rev-parse", "HEAD")

	req := mustValid(t, okReq(dir))
	w, err := Provision(ctx, g, req, newBase, t0)
	if err != nil {
		t.Fatal(err)
	}
	if w.CheckBase != oldBase || w.CheckBase == newBase {
		t.Fatalf("check base %s, want merge-base %s", w.CheckBase, oldBase)
	}
	if _, err := os.Stat(filepath.Join(w.Path, "internal/x/b.go")); err != nil {
		t.Fatal("existing branch content missing")
	}
	_ = w.Unlock()

	// A clean existing worktree is reused.
	w2, err := Provision(ctx, g, req, newBase, t0)
	if err != nil || !w2.Reused || w2.CheckBase != oldBase {
		t.Fatalf("reuse: %+v %v", w2, err)
	}
	_ = w2.Unlock()

	// A dirty one is exit 3 naming it, and the lock is released.
	write(t, w.Path, "internal/x/dirty.go", "package x\n")
	_, err = Provision(ctx, g, req, newBase, t0)
	if code(err) != ExitEnv || !strings.Contains(err.Error(), w.Path) || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("dirty: %v", err)
	}
	if _, serr := os.Stat(w.LockPath); serr == nil {
		t.Fatal("failed provision left its lock")
	}
}

func TestStaleLock(t *testing.T) {
	dir, g := newRepo(t)
	base, _ := ResolveBase(context.Background(), g, "", "")
	req := mustValid(t, okReq(dir))
	lock := filepath.Join(dir, ".waggle/worktrees/T-023-builder.lock")
	host, _ := os.Hostname()
	put := func(body string) {
		t.Helper()
		write(t, dir, ".waggle/worktrees/T-023-builder.lock", body)
	}
	old := pidAlive
	t.Cleanup(func() { pidAlive = old })
	pidAlive = func(int) bool { return false }

	put(`{"pid":4242,"host":"` + host + `","started":"2026-10-18T10:00:00Z"}`)
	w, err := Provision(context.Background(), g, req, base, t0)
	if err != nil || len(w.Warnings) != 1 || !strings.Contains(w.Warnings[0], "4242") {
		t.Fatalf("dead pid: %+v %v", w, err)
	}
	_ = w.Unlock()
	if _, err := os.Stat(lock); err == nil {
		t.Fatal("lock not removed")
	}

	pidAlive = func(int) bool { return true }
	put(`{"pid":4242,"host":"` + host + `","started":"2026-10-18T10:00:00Z"}`)
	if _, err := Provision(context.Background(), g, req, base, t0); code(err) != ExitEnv {
		t.Fatalf("live pid: %v", err)
	}
	pidAlive = func(int) bool { return false }
	put(`{"pid":4242,"host":"elsewhere","started":"2026-10-18T10:00:00Z"}`)
	if _, err := Provision(context.Background(), g, req, base, t0); code(err) != ExitEnv {
		t.Fatalf("other host: %v", err)
	}
	put("garbage")
	if _, err := Provision(context.Background(), g, req, base, t0); code(err) != ExitEnv {
		t.Fatalf("garbage lock: %v", err)
	}
}

func TestProcessAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Fatal("own pid reported dead")
	}
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if processAlive(cmd.Process.Pid) {
		t.Fatal("reaped pid reported alive")
	}
}

func TestUntrackedPathIsNotAWorktree(t *testing.T) {
	dir, g := newRepo(t)
	base, _ := ResolveBase(context.Background(), g, "", "")
	write(t, dir, ".waggle/worktrees/T-023-builder/file", "x")
	if _, err := Provision(context.Background(), g, mustValid(t, okReq(dir)), base, t0); code(err) != ExitEnv {
		t.Fatalf("%v", err)
	}
}

func TestInputsComeFromBase(t *testing.T) {
	dir, g := newRepo(t)
	ctx := context.Background()
	base, _ := ResolveBase(ctx, g, "", "")
	write(t, dir, ".waggle/hive/tasks/T-023.md", "# INJECTED uncommitted edit\n")
	write(t, dir, ".waggle/control/roles/builder.md", "INJECTED role\n")
	req := mustValid(t, okReq(dir))
	req.ReadPaths = []string{"internal/x/a.go"}
	in, err := ReadInputs(ctx, g, base, req)
	if err != nil {
		t.Fatal(err)
	}
	lease := comb.Lease{Directories: req.Dirs, Gate: "full"}
	runDir, err := NewRunDir(dir, req.Ticket, req.Role)
	if err != nil {
		t.Fatal(err)
	}
	stable, variable, err := WritePrompts(runDir, in, TaskLines(req, base, lease), runtime.ContextSettings{InlineReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(runDir, "prompt.variable.txt"))
	if string(got) != string(variable) || !strings.Contains(string(variable), "# brief at base") ||
		strings.Contains(string(variable), "INJECTED") || strings.Contains(string(stable), "INJECTED") {
		t.Fatalf("variable:\n%s\nstable:\n%s", variable, stable)
	}
	for _, want := range []string{"Ticket: T-023", "Branch: t-023", "Base: " + base, "internal/x", "package x", `"T-023: "`, "do not push"} {
		if !strings.Contains(string(variable), want) {
			t.Fatalf("variable lacks %q:\n%s", want, variable)
		}
	}
	if fi, _ := os.Stat(filepath.Join(runDir, "prompt.stable.txt")); fi == nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("prompt file mode %v", fi)
	}
}

func TestInputsMissing(t *testing.T) {
	dir, g := newRepo(t)
	ctx := context.Background()
	base, _ := ResolveBase(ctx, g, "", "")
	// Brief only in the working tree, not on base.
	write(t, dir, ".waggle/hive/tasks/T-030.md", "# uncommitted\n")
	r := okReq(dir)
	r.Ticket, r.Branch = "30", "t-030"
	_, err := ReadInputs(ctx, g, base, mustValid(t, r))
	if code(err) != ExitEnv || !strings.Contains(err.Error(), "T-030.md is not on base") {
		t.Fatalf("brief: %v", err)
	}
	r = okReq(dir)
	r.ReadPaths = []string{"nope.go"}
	if _, err = ReadInputs(ctx, g, base, mustValid(t, r)); code(err) != ExitEnv {
		t.Fatalf("read path: %v", err)
	}
	r.ReadPaths = []string{"internal"} // a directory
	if _, err = ReadInputs(ctx, g, base, mustValid(t, r)); code(err) != ExitEnv {
		t.Fatalf("dir read path: %v", err)
	}
	r = okReq(dir)
	r.Role = "forager" // no forager prompt at base
	if _, err = ReadInputs(ctx, g, base, mustValid(t, r)); code(err) != ExitEnv {
		t.Fatalf("role prompt: %v", err)
	}
	big := strings.Repeat("x", maxBrief+1)
	write(t, dir, ".waggle/hive/tasks/T-023.md", big)
	sh(t, dir, "add", "-A")
	sh(t, dir, "commit", "-q", "-m", "big")
	base2, _ := ResolveBase(ctx, g, "", "")
	if _, err = ReadInputs(ctx, g, base2, mustValid(t, okReq(dir))); code(err) != ExitEnv || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("over cap: %v", err)
	}
}

func TestInputsReview(t *testing.T) {
	dir, g := newRepo(t)
	ctx := context.Background()
	write(t, dir, ".waggle/hive/reviews/T-023.md", "x\nVerdict: must-fix\nmore\nVerdict: must-fix\n")
	sh(t, dir, "add", "-A")
	sh(t, dir, "commit", "-q", "-m", "review")
	base, _ := ResolveBase(ctx, g, "", "")
	in, err := ReadInputs(ctx, g, base, mustValid(t, okReq(dir)))
	if err != nil || in.MustFix != 2 {
		t.Fatalf("must-fix %d, %v", in.MustFix, err)
	}
}

func TestResolveBase(t *testing.T) {
	dir, g := newRepo(t)
	ctx := context.Background()
	head := sh(t, dir, "rev-parse", "HEAD")
	if got, err := ResolveBase(ctx, g, "", "HEAD"); err != nil || got != head {
		t.Fatalf("explicit: %s %v", got, err)
	}
	if _, err := ResolveBase(ctx, g, "", "nosuchrev"); code(err) != ExitUsage {
		t.Fatalf("explicit bad: %v", err)
	}

	// With a remote, the default base is <remote>/main after a fetch.
	bare := t.TempDir()
	sh(t, bare, "init", "-q", "--bare", "-b", "main")
	sh(t, dir, "remote", "add", "origin", bare)
	sh(t, dir, "push", "-q", "origin", "main")
	sh(t, dir, "fetch", "-q", "origin")
	write(t, dir, "later.txt", "x")
	sh(t, dir, "add", "-A")
	sh(t, dir, "commit", "-q", "-m", "later")
	// Another clone advances the remote.
	other := t.TempDir()
	sh(t, other, "clone", "-q", bare, "c")
	c := filepath.Join(other, "c")
	write(t, c, "remote.txt", "x")
	sh(t, c, "add", "-A")
	sh(t, c, "commit", "-q", "-m", "remote")
	sh(t, c, "push", "-q", "origin", "main")
	want := sh(t, c, "rev-parse", "HEAD")
	got, err := ResolveBase(ctx, g, "origin", "")
	if err != nil || got != want {
		t.Fatalf("remote default: %s want %s, %v", got, want, err)
	}
	if _, err := ResolveBase(ctx, g, "nosuchremote", ""); code(err) != ExitEnv {
		t.Fatalf("bad remote: %v", err)
	}
}

func TestNewRunDir(t *testing.T) {
	dir := t.TempDir()
	a, err := NewRunDir(dir, "T-023", "builder")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewRunDir(dir, "T-023", "builder")
	c, _ := NewRunDir(dir, "T-023", "guard")
	if filepath.Base(a) != "T-023-builder-1" || filepath.Base(b) != "T-023-builder-2" || filepath.Base(c) != "T-023-guard-1" {
		t.Fatalf("%s %s %s", a, b, c)
	}
	if fi, _ := os.Stat(a); fi.Mode().Perm() != 0o700 {
		t.Fatalf("mode %v", fi.Mode())
	}
}

func TestGitEnvScrubs(t *testing.T) {
	out := gitEnv([]string{"PATH=/bin", "GIT_DIR=/x", "git_work_tree=/y", "GIT_SSH_COMMAND=ssh"})
	joined := strings.Join(out, "\n")
	if strings.Contains(joined, "GIT_DIR") || strings.Contains(strings.ToLower(joined), "git_work_tree") ||
		!strings.Contains(joined, "GIT_SSH_COMMAND=ssh") || !strings.Contains(joined, "GIT_TERMINAL_PROMPT=0") {
		t.Fatal(joined)
	}
}

func TestExitCode(t *testing.T) {
	if ExitCode(nil) != 0 || ExitCode(errors.New("x")) != ExitEnv || ExitCode(failf(ExitClaim, "x")) != ExitClaim {
		t.Fatal("ExitCode")
	}
}
