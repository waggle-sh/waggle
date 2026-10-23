//go:build unix

package dispatch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
)

func taskReq(f *fixture, mut ...func(*ClaimTaskRequest)) ClaimTaskRequest {
	r := ClaimTaskRequest{RepoDir: f.repo, Ticket: "32", Dirs: []string{"internal/y", "internal/x"}, Gate: "full", Owner: "alice"}
	for _, m := range mut {
		m(&r)
	}
	return r
}

func noClaimRefs(f *fixture) bool { return f.git("for-each-ref", "refs/waggle/claims/") == "" }

// fakeClaims is a ClaimBackend that holds no claims and counts the writes.
type fakeClaims struct {
	ClaimBackend
	acquired int
}

func (f *fakeClaims) ClaimOID(context.Context, string) (string, error) { return "", comb.ErrNotFound }
func (f *fakeClaims) ReadClaim(context.Context, string) (comb.Lease, error) {
	return comb.Lease{}, comb.ErrNotFound
}

func (f *fakeClaims) AcquireLease(context.Context, comb.Lease) error { f.acquired++; return nil }

// commitGate lists entries as gate.protected_paths in the working tree config
// and commits it.
func commitGate(f *fixture, entries string, commit bool) {
	cfg, _ := os.ReadFile(filepath.Join(f.real, ".waggle/control/config.yaml"))
	write(f.t, f.real, ".waggle/control/config.yaml", string(cfg)+"\ngate:\n  protected_paths: \""+entries+"\"\n")
	if commit {
		f.git("commit", "-q", "-a", "-m", "gate")
	}
}

// AC3: claim, renew, refuse another owner, take over after expiry.
func TestClaimTask(t *testing.T) {
	f := newFixture(t, false, false)
	ctx := context.Background()
	cfg, err := config.Load(filepath.Join(f.real, ".waggle/control/config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	ttl := time.Duration(cfg.Leases.DefaultTTLMinutes) * time.Minute
	now := time.Now().UTC().Truncate(time.Second)
	at := func(d time.Duration) func(*ClaimTaskRequest) {
		return func(r *ClaimTaskRequest) { r.Now = func() time.Time { return now.Add(d) } }
	}
	res, err := ClaimTask(ctx, taskReq(f, at(0)))
	if err != nil || res.Outcome != "claimed" || res.Task != "T-032" || res.Ref != "refs/waggle/claims/ticket-32" || len(res.OID) != 40 ||
		!slices.Equal(res.Lease.Directories, []string{"internal/x", "internal/y"}) || res.Lease.ExpiresAt.Sub(res.Lease.ClaimedAt) != ttl || res.Lease.Role != "builder" {
		t.Fatalf("claim: %+v %v", res, err)
	}
	ninety := func(r *ClaimTaskRequest) { r.Ticket, r.Dirs = "T-033", []string{"docs/a"}; r.TTL = 90 * time.Minute }
	res, err = ClaimTask(ctx, taskReq(f, ninety, at(0)))
	if err != nil || res.Lease.ExpiresAt.Sub(res.Lease.ClaimedAt) != 90*time.Minute {
		t.Fatalf("ttl: %+v %v", res, err)
	}
	res, err = ClaimTask(ctx, taskReq(f, at(time.Minute), func(r *ClaimTaskRequest) { r.Dirs = []string{"internal/x"} }))
	if err != nil || res.Outcome != "renewed" || len(res.Notes) == 0 {
		t.Fatalf("renew: %+v %v", res, err)
	}
	bob := func(r *ClaimTaskRequest) { r.Owner = "bob" }
	if _, err = ClaimTask(ctx, taskReq(f, at(2*time.Minute), bob)); code(err) != ExitClaim || !errors.Is(err, comb.ErrAlreadyClaimed) || ErrorCode(err) != "already_claimed" {
		t.Fatalf("other owner: %v", err)
	}
	if res, err = ClaimTask(ctx, taskReq(f, at(ttl+10*time.Minute), bob)); err != nil || res.Outcome != "taken_over" || res.Lease.KeeperID != "bob" {
		t.Fatalf("takeover: %+v %v", res, err)
	}
}

// AC4: every refusal writes nothing and carries its code and exit.
func TestClaimTaskRefusals(t *testing.T) {
	f := newFixture(t, false, false)
	ctx := context.Background()
	if _, err := ClaimTask(ctx, taskReq(f, func(r *ClaimTaskRequest) { r.Ticket, r.Dirs = "40", []string{"internal/x"} })); err != nil {
		t.Fatal(err)
	}
	before := f.git("for-each-ref", "refs/waggle/claims/")
	cases := []struct {
		name string
		mut  func(*ClaimTaskRequest)
		exit int
		code string
	}{
		{"overlap", func(r *ClaimTaskRequest) { r.Ticket, r.Dirs = "41", []string{"internal/x/sub"} }, ExitClaim, "overlap"},
		{"vendor", func(r *ClaimTaskRequest) { r.Dirs = []string{"vendor"} }, ExitClaim, "protected_path"},
		{"workflows", func(r *ClaimTaskRequest) { r.Dirs = []string{".github/workflows"} }, ExitClaim, "protected_path"},
		{"light gate on CLAUDE.md", func(r *ClaimTaskRequest) { r.Gate, r.Dirs = "light", []string{"CLAUDE.md"} }, ExitClaim, "gate_too_low"},
		{"ttl 30s", func(r *ClaimTaskRequest) { r.TTL = 30 * time.Second }, ExitUsage, "usage"},
		{"ttl 169h", func(r *ClaimTaskRequest) { r.TTL = 169 * time.Hour }, ExitUsage, "usage"},
		{"ttl 90m30s", func(r *ClaimTaskRequest) { r.TTL = 90*time.Minute + 30*time.Second }, ExitUsage, "usage"},
		{"ttl negative", func(r *ClaimTaskRequest) { r.TTL = -time.Minute }, ExitUsage, "usage"},
		{"no gate", func(r *ClaimTaskRequest) { r.Gate = "" }, ExitUsage, "usage"},
		{"bad task", func(r *ClaimTaskRequest) { r.Ticket = "x" }, ExitUsage, "invalid_task"},
		{"bad base", func(r *ClaimTaskRequest) { r.Base = "-x" }, ExitUsage, "usage"},
		{"no dirs", func(r *ClaimTaskRequest) { r.Dirs = nil }, ExitUsage, "usage"},
	}
	for _, c := range cases {
		_, err := ClaimTask(ctx, taskReq(f, c.mut))
		if code(err) != c.exit || ErrorCode(err) != c.code {
			t.Errorf("%s: exit %d code %q err %v; want %d %q", c.name, code(err), ErrorCode(err), err, c.exit, c.code)
		}
	}
	if _, err := ClaimTask(ctx, taskReq(f, func(r *ClaimTaskRequest) { r.Dirs = []string{"vendor"} })); !errors.Is(err, comb.ErrTier3Path) {
		t.Errorf("hardcoded refusal does not wrap ErrTier3Path: %v", err)
	}
	if got := f.git("for-each-ref", "refs/waggle/claims/"); got != before {
		t.Fatalf("a refusal wrote a ref:\n%s\nbefore:\n%s", got, before)
	}
	if err := os.Remove(filepath.Join(f.real, ".waggle/control/local.yaml")); err != nil {
		t.Fatal(err)
	}
	_, err := ClaimTask(ctx, taskReq(f, func(r *ClaimTaskRequest) { r.Owner = "" }))
	if code(err) != ExitUsage || !strings.Contains(err.Error(), "or pass --owner <id>") {
		t.Fatalf("no owner: %v", err)
	}
}

// AC5: protected paths come from the base revision, raise-only.
func TestClaimTaskProtected(t *testing.T) {
	f := newFixture(t, false, false)
	commitGate(f, "deploy,Infra/terraform/", true)
	for _, d := range []string{"deploy", "deploy/app", "deployment", "infra", "INFRA/terraform/mod"} {
		_, err := ClaimTask(context.Background(), taskReq(f, func(r *ClaimTaskRequest) { r.Dirs = []string{d} }))
		if code(err) != ExitClaim || ErrorCode(err) != "protected_path" || !errors.Is(err, comb.ErrProtectedPath) {
			t.Errorf("%s: exit %d code %q err %v", d, code(err), ErrorCode(err), err)
		}
	}
	if !noClaimRefs(f) {
		t.Fatal("a refusal wrote a ref")
	}
	if _, err := ClaimTask(context.Background(), taskReq(f, func(r *ClaimTaskRequest) { r.Dirs = []string{"docs"} })); err != nil {
		t.Fatalf("docs: %v", err)
	}

	// Only the base counts: the working tree and another branch do not apply.
	g := newFixture(t, false, false)
	commitGate(g, "deploy", false)
	if _, err := ClaimTask(context.Background(), taskReq(g, func(r *ClaimTaskRequest) { r.Dirs = []string{"deploy"} })); err != nil {
		t.Fatalf("working tree entry applied: %v", err)
	}
	h := newFixture(t, false, false)
	h.git("checkout", "-q", "-b", "other")
	commitGate(h, "deploy", true)
	h.git("checkout", "-q", "main")
	if _, err := ClaimTask(context.Background(), taskReq(h, func(r *ClaimTaskRequest) { r.Dirs = []string{"deploy"} })); err != nil {
		t.Fatalf("other branch entry applied: %v", err)
	}
	// A base config that is a symlink is exit 3 through the verb.
	s := newFixture(t, false, false)
	cfgPath := filepath.Join(s.real, ".waggle/control/config.yaml")
	data, _ := os.ReadFile(cfgPath)
	write(t, s.real, ".waggle/control/real.yaml", string(data))
	if err := os.Remove(cfgPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.yaml", cfgPath); err != nil {
		t.Fatal(err)
	}
	s.git("add", "-A")
	s.git("commit", "-q", "-m", "link")
	if _, err := ClaimTask(context.Background(), taskReq(s)); code(err) != ExitEnv || ErrorCode(err) != "environment" || !noClaimRefs(s) {
		t.Fatalf("symlink config: %v", err)
	}
}

// AC6: basePolicy reads the base blob and fails closed.
func TestBasePolicy(t *testing.T) {
	ctx := context.Background()
	cfgRel := ".waggle/control/config.yaml"
	policyAt := func(f *fixture) (comb.TierPolicy, error) {
		return basePolicy(ctx, &Git{Dir: f.real}, f.git("rev-parse", "HEAD"))
	}
	f := newFixture(t, false, false)
	if p, err := policyAt(f); err != nil || len(p.ExtraTier3) != 0 {
		t.Fatalf("no gate section: %+v %v", p, err)
	}
	commitGate(f, "deploy,Infra/terraform/", true)
	if p, err := policyAt(f); err != nil || !slices.Equal(p.ExtraTier3, []string{"deploy", "Infra/terraform/"}) {
		t.Fatalf("entries: %+v %v", p, err)
	}
	f.git("rm", "-q", cfgRel)
	f.git("commit", "-q", "-m", "no config")
	if p, err := policyAt(f); err != nil || len(p.ExtraTier3) != 0 {
		t.Fatalf("missing: %+v %v", p, err)
	}
	for _, name := range []string{"symlink", "directory", "oversized", "invalid"} {
		f := newFixture(t, false, false)
		switch name {
		case "symlink":
			f.git("rm", "-q", cfgRel)
			if err := os.Symlink("local.yaml", filepath.Join(f.real, cfgRel)); err != nil {
				t.Fatal(err)
			}
		case "directory":
			f.git("rm", "-q", cfgRel)
			write(t, f.real, cfgRel+"/x", "x\n")
		case "oversized":
			write(t, f.real, cfgRel, "version: 1\n# "+strings.Repeat("x", config.MaxFileSize)+"\n")
		case "invalid":
			write(t, f.real, cfgRel, "version: 2\n")
		}
		f.git("add", "-A")
		f.git("commit", "-q", "-m", name)
		if _, err := policyAt(f); code(err) != ExitEnv {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// AC6: the covered-directory refusal comes before any claim write, also with an
// injected backend.
func TestProtectedRefusedBeforeClaim(t *testing.T) {
	p := comb.TierPolicy{ExtraTier3: []string{"deploy"}}
	if err := refuseProtected(p, []string{"docs", "deploy/app"}); code(err) != ExitClaim || !errors.Is(err, comb.ErrProtectedPath) || !errors.Is(err, comb.ErrTier3Path) {
		t.Fatalf("refuseProtected: %v", err)
	}
	if err := refuseProtected(p, []string{"docs"}); err != nil {
		t.Fatal(err)
	}
	f := newFixture(t, false, false)
	commitGate(f, "deploy", true)
	fake := &fakeClaims{}
	_, err := ClaimTask(context.Background(), taskReq(f, func(r *ClaimTaskRequest) { r.Backend, r.Dirs = fake, []string{"deploy/app"} }))
	if code(err) != ExitClaim || fake.acquired != 0 {
		t.Fatalf("err %v, writes %d", err, fake.acquired)
	}
}

// AC6: the post-run check applies the base policy, and an empty policy changes nothing.
func TestCheckerPolicy(t *testing.T) {
	f := newFixture(t, false, false)
	ctx := context.Background()
	req, err := okReq(f.real).Validate()
	if err != nil {
		t.Fatal(err)
	}
	g := &Git{Dir: f.real}
	base, err := ResolveBase(ctx, g, "", "")
	if err != nil {
		t.Fatal(err)
	}
	w, err := Provision(ctx, g, req, base, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	defer w.Unlock()
	pg := g.pinned()
	lease := comb.Lease{TicketID: "T-023", Gate: "full", Directories: []string{"deploy", "go.mod", "internal/x"}}
	write(t, w.Path, "deploy/a.txt", "a\n")
	write(t, w.Path, "go.mod", "module x\n")
	write(t, w.Path, "internal/x/b.go", "package x\n")
	ck := func(p comb.TierPolicy) *checker {
		c := &checker{g: pg, common: f.common(), w: w, lease: lease, policy: p}
		if c.before, err = takeSnap(ctx, pg, f.common(), w.Admin); err != nil {
			t.Fatal(err)
		}
		return c
	}
	withPolicy, without := ck(comb.TierPolicy{ExtraTier3: []string{"deploy"}}), ck(comb.TierPolicy{})
	sh(t, w.Path, "add", "-A")
	sh(t, w.Path, "commit", "-q", "-s", "-m", "T-023: files")
	tiers := func(c *checker) (vs []comb.Violation) {
		all, _ := c.run(ctx)
		for _, v := range all {
			if hasViolation([]comb.Violation{v}, ruleHost, "") {
				t.Errorf("unexpected host violation %+v", v)
			}
			if comb.IsTierRule(v.Rule) {
				vs = append(vs, v)
			}
		}
		return vs
	}
	got := tiers(withPolicy)
	if len(got) != 2 || got[0].Path != "deploy/a.txt" || got[0].Rule != comb.RuleTier3 ||
		got[0].Detail != "protected path (listed in .waggle/control/config.yaml); needs an admin bypass to merge" || got[1].Path != "go.mod" {
		t.Fatalf("with policy: %+v", got)
	}
	want := comb.CheckChanges(lease, []comb.Change{{Path: "deploy/a.txt", NewMode: "100644"}, {Path: "go.mod", NewMode: "100644"}, {Path: "internal/x/b.go", NewMode: "100644"}})
	if got = tiers(without); !slices.Equal(got, want) {
		t.Fatalf("empty policy: %+v, CheckChanges %+v", got, want)
	}
}

// release fixtures: ticket 23 claimed by alice; branches merge into main.
func mergeBranch(f *fixture, branch string) {
	branchWith(f, branch, "internal/x/"+strings.ReplaceAll(branch, "/", "-")+".go")
	f.git("merge", "-q", "--no-ff", "-m", "merge "+branch, branch)
}

func releaseTask(f *fixture, mut ...func(*ReleaseRequest)) (ReleaseTaskResult, error) {
	r := ReleaseRequest{RepoDir: f.repo, Ticket: "23"}
	for _, m := range mut {
		m(&r)
	}
	return ReleaseTask(context.Background(), r)
}

// AC7: branch forms, owner, merged check and the no-worktrees path.
func TestReleaseTask(t *testing.T) {
	merged := func(branch string) func(*fixture) { return func(f *fixture) { mergeBranch(f, branch) } }
	branchArg := func(b string) func(*ReleaseRequest) { return func(r *ReleaseRequest) { r.Branch = b } }
	cases := []struct {
		name   string
		setup  func(*fixture)
		mut    func(*ReleaseRequest)
		exit   int
		code   string
		branch string
	}{
		{"t-023 merged", merged("t-023"), nil, 0, "", "t-023"},
		{"waggle/T-023 merged", merged("waggle/T-023"), nil, 0, "", "waggle/T-023"},
		{"suffix form", merged("waggle/T-023-x"), branchArg("waggle/T-023-x"), 0, "", "waggle/T-023-x"},
		{"other ticket's waggle branch", func(f *fixture) {}, branchArg("waggle/T-024"), ExitUsage, "usage", ""},
		{"foreign branch", func(f *fixture) {}, branchArg("feature/x"), ExitUsage, "usage", ""},
		{"unmerged", func(f *fixture) { branchWith(f, "t-023", "internal/x/b.go") }, nil, ExitClaim, "not_merged", ""},
		{"unmerged t-023, merged waggle/T-023", func(f *fixture) {
			branchWith(f, "t-023", "internal/x/b.go")
			mergeBranch(f, "waggle/T-023")
		}, nil, ExitClaim, "not_merged", ""},
		{"another owner", func(f *fixture) {}, func(r *ReleaseRequest) { r.Unmerged, r.Owner = true, "bob" }, ExitClaim, "not_owner", ""},
		{"worktree marker", func(f *fixture) {
			write(f.t, f.real, ".waggle/worktrees/T-023-builder.tainted", "run-1")
		}, func(r *ReleaseRequest) { r.Unmerged = true }, ExitClaim, "task_busy", ""},
		{"held dispatch.lock", func(f *fixture) {
			write(f.t, f.real, ".waggle/worktrees/keep", "")
			writeLock(f.t, filepath.Join(f.real, ".waggle/worktrees/dispatch.lock"), os.Getpid())
		}, func(r *ReleaseRequest) { r.Unmerged = true }, ExitEnv, "environment", ""},
		{"unmerged flag", func(f *fixture) {}, func(r *ReleaseRequest) { r.Unmerged = true }, 0, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, false, false)
			releaseClaim(t, f, "alice")
			c.setup(f)
			var mut []func(*ReleaseRequest)
			if c.mut != nil {
				mut = append(mut, c.mut)
			}
			res, err := releaseTask(f, mut...)
			if code(err) != c.exit || (err != nil && ErrorCode(err) != c.code) {
				t.Fatalf("res %+v err %v, want exit %d code %q", res, err, c.exit, c.code)
			}
			if err == nil && (res.Outcome != "released" || res.Branch != c.branch || res.Merged != (c.branch != "") || claimKept(t, f)) {
				t.Fatalf("res %+v, claim kept %v", res, claimKept(t, f))
			}
			if err != nil && !claimKept(t, f) {
				t.Fatal("a refusal deleted the claim")
			}
		})
	}
}

func TestReleaseTaskNoWorktreesDir(t *testing.T) {
	f := newFixture(t, false, false)
	releaseClaim(t, f, "alice")
	dir := filepath.Join(f.real, ".waggle/worktrees")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	mergeBranch(f, "t-023")
	if res, err := releaseTask(f); err != nil || res.Outcome != "released" {
		t.Fatalf("%+v %v", res, err)
	}
	if exists(dir) {
		t.Fatal("release created .waggle/worktrees")
	}
	if res, err := releaseTask(f); err != nil || res.Outcome != "no_claim" || res.OID != "" || ReleaseLine(res) != "no claim for T-023" {
		t.Fatalf("second release: %+v %v", res, err)
	}
	// A file at that path is not absent: lockRepo refuses it.
	write(t, f.real, ".waggle/worktrees", "a file")
	if _, err := releaseTask(f); code(err) != ExitEnv {
		t.Fatalf("file at .waggle/worktrees: %v", err)
	}
}

// AC8, mapping: a lost compare-and-swap is exit 4 concurrent_update.
func TestReleaseTaskConcurrentUpdate(t *testing.T) {
	f := newFixture(t, false, false)
	releaseClaim(t, f, "alice")
	_, err := releaseTask(f, unmerged, func(r *ReleaseRequest) { r.Backend = fakeRelease{err: comb.ErrConcurrentUpdate} })
	if code(err) != ExitClaim || ErrorCode(err) != "concurrent_update" {
		t.Fatalf("%v", err)
	}
}

// AC12 helpers: a root commit whose config lists docs as protected.
func evilCommit(f *fixture) string {
	cfg, _ := os.ReadFile(filepath.Join(f.real, ".waggle/control/config.yaml"))
	blob := shIn(f.t, f.real, string(cfg)+"\ngate:\n  protected_paths: \"docs\"\n", "hash-object", "-w", "--stdin")
	control := shIn(f.t, f.real, "100644 blob "+blob+"\tconfig.yaml\n", "mktree")
	waggle := shIn(f.t, f.real, "040000 tree "+control+"\tcontrol\n", "mktree")
	root := shIn(f.t, f.real, "040000 tree "+waggle+"\t.waggle\n", "mktree")
	return sh(f.t, f.real, "commit-tree", "-m", "evil", root)
}

// AC12: a tag named like the default base never picks the base.
func TestDefaultBaseIgnoresTags(t *testing.T) {
	for _, remote := range []bool{false, true} {
		name := map[bool]string{false: "local", true: "remote"}[remote]
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, remote, false)
			ctx := context.Background()
			c0 := f.git("rev-parse", "HEAD")
			mergeBranch(f, "t-023")
			evil, short, qualified, rem := evilCommit(f), "main", "refs/heads/main", ""
			if remote {
				f.git("push", "-q", "origin", "main")
				f.git("push", "-q", "origin", evil+":refs/heads/evil")
				f.git("--git-dir="+f.origin, "tag", "origin/main", evil)
				f.git("fetch", "-q", "--tags", "origin")
				short, qualified, rem = "origin/main", "refs/remotes/origin/main", "origin"
			} else {
				f.git("tag", "main", evil)
			}
			if got := f.git("-c", "core.warnAmbiguousRefs=false", "rev-parse", short+"^{commit}"); got != evil {
				t.Fatalf("setup: %s resolves to %s, not the tag's commit", short, got)
			}
			main := f.git("rev-parse", qualified)
			g := &Git{Dir: f.real, Remote: rem}
			if got, err := ResolveBase(ctx, g, rem, ""); err != nil || got != main {
				t.Fatalf("ResolveBase = %q %v, want %s", got, err, main)
			}
			fake := &fakeClaims{}
			if _, err := ClaimTask(ctx, taskReq(f, func(r *ClaimTaskRequest) { r.Backend, r.Dirs = fake, []string{"docs"} })); err != nil || fake.acquired != 1 {
				t.Fatalf("claim on docs read the tag's config: %v, writes %d", err, fake.acquired)
			}
			if res, err := ReleaseTask(ctx, ReleaseRequest{RepoDir: f.repo, Ticket: "23", Owner: "alice", Backend: fakeRelease{}}); err != nil || res.Branch != "t-023" {
				t.Fatalf("merged check read the tag's history: %+v %v", res, err)
			}
			f.git("branch", "-q", "t-024", c0)
			req := mustValid(t, Request{RepoDir: f.real, Ticket: "24", Role: "builder", Dirs: []string{"internal/x"}, Gate: "full", Base: c0})
			if got, err := offBranch(ctx, g, req, c0); err != nil || got != c0 {
				t.Fatalf("offBranch = %q %v, want %s", got, err, c0)
			}
		})
	}
}

// AC8: against a local bare origin both verbs write and delete the remote ref.
func TestClaimReleaseRemote(t *testing.T) {
	f := newFixture(t, true, false)
	ctx := context.Background()
	remoteClaim := func() string { return sh(t, f.origin, "for-each-ref", "refs/waggle/claims/ticket-23") }
	res, err := ClaimTask(ctx, taskReq(f, func(r *ClaimTaskRequest) { r.Ticket, r.Dirs = "23", []string{"internal/x"} }))
	if err != nil || res.Outcome != "claimed" || remoteClaim() == "" {
		t.Fatalf("claim: %+v %v, claim on origin %q", res, err, remoteClaim())
	}
	mergeBranch(f, "t-023")
	f.git("push", "-q", "origin", "main")
	if rel, err := releaseTask(f, func(r *ReleaseRequest) { r.Owner = "alice" }); err != nil || rel.Outcome != "released" || remoteClaim() != "" {
		t.Fatalf("release: %+v %v, claim on origin %q", rel, err, remoteClaim())
	}
}

func TestErrorCode(t *testing.T) {
	for want, err := range map[string]error{
		"invalid_task":      failf(ExitUsage, "%w", comb.ErrInvalidTicketID),
		"usage":             failf(ExitUsage, "flag"),
		"protected_path":    failf(ExitClaim, "x: %w: %w", comb.ErrWaggleDir, comb.ErrTier3Path),
		"gate_too_low":      claimError(comb.ErrGateTooLow),
		"invalid_lease":     claimError(comb.ErrInvalidLease),
		"overlap":           claimError(comb.ErrLeaseOverlap),
		"already_claimed":   claimError(comb.ErrAlreadyClaimed),
		"malformed_claim":   claimError(comb.ErrMalformedClaim),
		"not_owner":         releaseError(comb.ErrNotOwner),
		"concurrent_update": releaseError(comb.ErrConcurrentUpdate),
		"not_merged":        failf(ExitClaim, "%w", taggedError{"m", errNotMerged}),
		"task_busy":         failf(ExitClaim, "%w", taggedError{"m", errTaskBusy}),
		"interrupted":       claimError(context.Canceled),
		"environment":       errors.New("git exploded"),
	} {
		if got := ErrorCode(err); got != want {
			t.Errorf("%v: code %q, want %q", err, got, want)
		}
	}
}
