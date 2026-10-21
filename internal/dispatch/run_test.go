//go:build unix

package dispatch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	stdruntime "runtime"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/runtime"
)

// Test 11 (and 26): the happy path, with the repo reached two ways.
func TestRunHappyPath(t *testing.T) {
	eachRepoSpelling(t, func(t *testing.T, link bool) {
		f := newFixture(t, true, link)
		res, err := f.run(okScenario)
		if err != nil || res.ExitCode != 0 || res.End != "done" {
			t.Fatalf("exit %d end %s err %v violations %v", res.ExitCode, res.End, err, res.Violations)
		}
		head := f.git("rev-parse", "refs/heads/t-023")
		if res.HeadOID != head || res.Branch != "t-023" || res.Ticket != "T-023" || res.Role != "builder" || res.Cell != "host" ||
			res.KeeperID != "alice" || res.ClaimVerb != "claim" || res.ClaimOID != f.git("rev-parse", "refs/waggle/claims/ticket-23") ||
			res.BaseOID != f.git("rev-parse", "origin/main") || res.CheckBaseOID != res.BaseOID || res.RuntimeVersion != "fake-claude 9.9" ||
			res.Model == "" || res.Profile == "" || res.CostUSD == nil || *res.CostUSD != 0.05 || res.Usage.Input != 100 || res.Usage.Output != 10 ||
			len(res.NotEnforced) != len(NotEnforced) || !res.Pushed || res.Schema != 1 {
			t.Fatalf("result fields: %+v", res)
		}
		if !f.pushed("t-023") || sh(t, f.origin, "rev-parse", "refs/heads/t-023") != head {
			t.Fatal("branch not pushed to the remote")
		}
		if _, err := os.Stat(res.Worktree); err == nil {
			t.Fatal("worktree not removed on success")
		}
		if f.git("rev-parse", "--verify", "refs/heads/t-023") != head {
			t.Fatal("branch must stay")
		}
		want, _ := filepath.EvalSymlinks(filepath.Join(f.real, ".waggle/worktrees/runs"))
		if got, _ := filepath.EvalSymlinks(filepath.Dir(res.RunDir)); got != want {
			t.Fatalf("run dir %s not under %s", res.RunDir, want)
		}
		raw, err := os.ReadFile(filepath.Join(res.RunDir, "result.json"))
		if err != nil || string(raw) != string(res.JSON) {
			t.Fatalf("result.json differs from Result.JSON: %v", err)
		}
		if fi, _ := os.Stat(filepath.Join(res.RunDir, "result.json")); fi.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v", fi.Mode())
		}
		for _, n := range []string{"prompt.stable.txt", "prompt.variable.txt", "settings.json", "transcript.jsonl", "stderr.log"} {
			if _, err := os.Stat(filepath.Join(res.RunDir, n)); err != nil {
				t.Fatalf("run record lacks %s", n)
			}
		}
		if b, _ := os.ReadFile(filepath.Join(res.RunDir, "final.md")); string(b) != "all done" {
			t.Fatalf("final.md %q", b)
		}
		if b, _ := os.ReadFile(filepath.Join(res.RunDir, "prompt.variable.txt")); !strings.Contains(string(b), "git commit -s") {
			t.Fatal("task lines lack the sign-off rule")
		}
		if _, err := os.Stat(filepath.Join(f.real, ".waggle/worktrees/dispatch.lock")); err == nil {
			t.Fatal("dispatch.lock left behind")
		}
	})
}

type violationCase struct {
	name, rule, path, detail string
	script                   func(f *fixture) string
	dirs                     string
	local                    bool
}

// plumbing builds a commit with hash-object, mktree and commit-tree, so no
// filesystem (and no case-insensitive volume) is involved. put replaces or adds
// one tree entry; the commit is signed off by hand.
func plumbing(chain string) string {
	return `put() { { git ls-tree "$1" 2>/dev/null | awk -F'\t' -v n="$4" '$2!=n'; printf '%s %s %s\t%s\n' "$2" "$3" "$5" "$4"; } | git mktree; }
b=$(echo x | git hash-object -w --stdin)
` + chain + `c=$(git -c user.name=a -c user.email=a@e.co commit-tree "$r" -p HEAD -m 'T-023: plumbing' -m 'Signed-off-by: a <a@e.co>')
git update-ref refs/heads/t-023 "$c"
`
}

const (
	underInternal = "t=$(put HEAD:internal/Comb 100644 blob x.go $b)\ni=$(put HEAD:internal 040000 tree Comb $t)\nr=$(put HEAD 040000 tree internal $i)\n"
	dotdotPath    = "t1=$(put HEAD:no/such 100644 blob x $b)\nt2=$(put HEAD:no/such 040000 tree workflows $t1)\nt3=$(put HEAD:no/such 040000 tree .github $t2)\nt4=$(put HEAD:no/such 040000 tree .. $t3)\nr=$(put HEAD 040000 tree docs $t4)\n"
	backslash     = "x=$(put HEAD:internal/x 100644 blob 'a\\b.go' $b)\ni=$(put HEAD:internal 040000 tree x $x)\nr=$(put HEAD 040000 tree internal $i)\n"
)

func violationCases() []violationCase {
	sign := "git -c user.name=a -c user.email=a@e.co "
	return []violationCase{
		{name: "outside lease", rule: "outside_lease", path: "docs/a.md", script: func(*fixture) string { return commitFile("docs/a.md", "x") }},
		{name: "deletion outside", rule: "outside_lease", path: "README", script: func(*fixture) string { return "git rm -q README\n" + sign + "commit -q -s -m 'T-023: rm'\n" }},
		{name: "case differs", rule: "case_differs", path: "internal/Comb/x.go", dirs: "internal/comb", script: func(*fixture) string { return plumbing(underInternal) }},
		{name: "symlink", rule: "link_mode", path: "internal/x/lnk", script: func(*fixture) string {
			return "ln -s /etc/hosts internal/x/lnk\ngit add internal/x/lnk\n" + sign + "commit -q -s -m 'T-023: link'\n"
		}},
		{name: "no subject prefix", rule: "commit", detail: "prefix", script: func(*fixture) string {
			return "echo x > internal/x/c.go\ngit add -A\n" + sign + "commit -q -s -m oops\n"
		}},
		{name: "merge commit", rule: "commit", detail: "merge commit", script: func(*fixture) string {
			return commitFile("internal/x/one.go", "1") + "git checkout -q -b tmp HEAD~1\n" + commitFile("internal/x/two.go", "2") +
				"git checkout -q t-023\n" + sign + "merge -q --no-ff --no-edit tmp\ngit branch -q -D tmp\n"
		}},
		{name: "uncommitted file", rule: "dirty", path: ".", script: func(*fixture) string { return okScenario + "echo x > internal/x/loose.go\n" }},
		{name: "main checkout write", rule: "host_state", path: ".", script: func(f *fixture) string {
			return okScenario + "echo x > '" + f.real + "/stray.txt'\n"
		}},
		{name: "new ref", rule: "host_state", path: "refs/heads/evil", script: func(*fixture) string { return okScenario + "git update-ref refs/heads/evil HEAD\n" }},
		{name: "other claim moved", rule: "host_state", path: "refs/waggle/claims/ticket-24", local: true, script: func(*fixture) string {
			return okScenario + "git update-ref refs/waggle/claims/ticket-24 HEAD\n"
		}},
		{name: "hook edited", rule: "host_state", path: "hooks/pre-commit", script: func(f *fixture) string {
			return okScenario + "printf '#!/bin/sh\\n' > '" + f.common() + "/hooks/pre-commit'\n"
		}},
		{name: "hooksPath set", rule: "host_state", path: "config", script: func(*fixture) string { return okScenario + "git config core.hooksPath /tmp/none\n" }},
		{name: "unsigned commit", rule: "dco", script: func(*fixture) string {
			return "echo x > internal/x/d.go\ngit add -A\n" + sign + "commit -q -m 'T-023: unsigned'\n"
		}},
		{name: "dotdot path", rule: "outside_lease", path: "docs/../.github/workflows/x", detail: "not canonical", script: func(*fixture) string { return plumbing(dotdotPath) }},
		{name: "backslash path", rule: "outside_lease", path: `internal/x/a\b.go`, detail: "not canonical", script: func(*fixture) string { return plumbing(backslash) }},
	}
}

// Tests 12 and 21: every violation is exit 1 and nothing is pushed.
func TestRunViolations(t *testing.T) {
	for _, c := range violationCases() {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, !c.local, false)
			if c.local {
				f.git("update-ref", "refs/waggle/claims/ticket-24", f.git("rev-parse", "HEAD"))
			}
			dirs := c.dirs
			if dirs == "" {
				dirs = "internal/x"
			}
			res, err := f.run(c.script(f), withDirs(dirs))
			if err != nil || res.ExitCode != 1 || res.Pushed || f.pushed("t-023") {
				t.Fatalf("exit %d pushed %v err %v violations %v", res.ExitCode, res.Pushed, err, res.Violations)
			}
			ok := false
			for _, v := range res.Violations {
				ok = ok || (v.Rule == c.rule && (c.path == "" || v.Path == c.path) && strings.Contains(v.Detail, c.detail))
			}
			if !ok {
				t.Fatalf("no %s %q violation in %v", c.rule, c.path, res.Violations)
			}
			_, tainted := os.Stat(res.Worktree + ".tainted")
			if (c.rule == "host_state") != (tainted == nil) {
				t.Fatalf("tainted marker (%v) does not match rule %s", tainted, c.rule)
			}
		})
	}
}

func TestRunZeroCommits(t *testing.T) {
	f := newFixture(t, true, false)
	res, err := f.run(emit(assistant(100), okResult))
	if err != nil || res.ExitCode != 1 || res.End != "no_change" || len(res.Violations) != 0 || res.Pushed {
		t.Fatalf("%+v %v", res, err)
	}
}

// Test 13: a limit stop ends exit 5 and the whole process group is gone.
func TestRunLimitsKillTheGroup(t *testing.T) {
	old := killGrace
	killGrace = 300 * time.Millisecond
	t.Cleanup(func() { killGrace = old })
	cases := []struct {
		name, end string
		stream    string
		timeout   time.Duration
	}{
		{"max_tokens", "max_tokens", emit(assistant(400000)), 0},
		{"timeout", "timeout", emit(assistant(100)), 400 * time.Millisecond},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, false, false)
			pidFile := filepath.Join(t.TempDir(), "pid")
			script := "sleep 300 </dev/null >/dev/null 2>&1 &\necho $! > '" + pidFile + "'\n" + c.stream + "sleep 60\n"
			res, err := f.run(script, func(_ *Request, d *Deps) { d.Timeout = c.timeout })
			if err != nil || res.ExitCode != 5 || res.End != c.end {
				t.Fatalf("exit %d end %s err %v", res.ExitCode, res.End, err)
			}
			b, _ := os.ReadFile(pidFile)
			var pid int
			for _, ch := range strings.TrimSpace(string(b)) {
				pid = pid*10 + int(ch-'0')
			}
			if pid == 0 || !wait(func() bool { return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) }) {
				t.Fatalf("grandchild %d survived", pid)
			}
		})
	}
}

// failing makes AcquireLease fail after the first (the claim itself).
type countBackend struct {
	ClaimBackend
	leases, reads atomic.Int32
	failRenew     bool
	failReadBack  bool
}

func (c *countBackend) AcquireLease(ctx context.Context, l comb.Lease) error {
	if c.leases.Add(1) > 1 && c.failRenew {
		return errors.New("renewal refused")
	}
	return c.ClaimBackend.AcquireLease(ctx, l)
}

func (c *countBackend) ReadClaim(ctx context.Context, id string) (comb.Lease, error) {
	if c.reads.Add(1) > 1 && c.failReadBack {
		return comb.Lease{}, errors.New("read-back refused")
	}
	return c.ClaimBackend.ReadClaim(ctx, id)
}

// Test 14: the claim is renewed while the session runs, and a failing renewal
// is a warning.
func TestRunKeepalive(t *testing.T) {
	for _, fail := range []bool{false, true} {
		f := newFixture(t, false, false)
		b, _ := claimEnv(t, f.real)
		cb := &countBackend{ClaimBackend: b, failRenew: fail}
		gate, opened := filepath.Join(t.TempDir(), "gate"), make(chan struct{})
		go func() { // the worker is held until three leases were written, at most 20 s
			defer close(opened)
			for end := time.Now().Add(20 * time.Second); cb.leases.Load() < 3 && time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
			}
			_ = os.WriteFile(gate, nil, 0o644)
		}()
		hold := "i=0; while [ ! -e '" + gate + "' ] && [ $i -lt 250 ]; do sleep 0.1; i=$((i+1)); done\n"
		res, err := f.run(hold+okScenario, func(_ *Request, d *Deps) { d.Backend, d.KeepaliveEvery = cb, 100*time.Millisecond })
		<-opened
		if err != nil || res.ExitCode != 0 || cb.leases.Load() < 3 {
			t.Fatalf("fail=%v: exit %d leases %d err %v v=%v w=%v", fail, res.ExitCode, cb.leases.Load(), err, res.Violations, res.Warnings)
		}
		warned := false
		for _, w := range res.Warnings {
			warned = warned || strings.HasPrefix(w, "claim_renew_failed")
		}
		if warned != fail {
			t.Fatalf("fail=%v warnings %v", fail, res.Warnings)
		}
	}
}

// A failing read-back after the claim was written is a warning, never exit 4.
func TestRunClaimReadBackWarning(t *testing.T) {
	f := newFixture(t, false, false)
	b, _ := claimEnv(t, f.real)
	res, err := f.run(okScenario, func(_ *Request, d *Deps) { d.Backend = &countBackend{ClaimBackend: b, failReadBack: true} })
	if err != nil || res.ExitCode != 0 || !strings.HasPrefix(strings.Join(res.Warnings, "|"), "claim_read_failed") {
		t.Fatalf("exit %d warnings %v err %v", res.ExitCode, res.Warnings, err)
	}
}

// Test 25: the end comes from the stream (the worker exits 1 after
// error_max_turns), and no goroutine outlives Run.
func TestRunProcessEnds(t *testing.T) {
	f := newFixture(t, false, false)
	before := stdruntime.NumGoroutine()
	res, err := f.run(emit(assistant(100), `{"type":"result","subtype":"error_max_turns","is_error":true,"session_id":"s1"}`) + "exit 1\n")
	if err != nil || res.ExitCode != 5 || res.End != "max_turns" {
		t.Fatalf("exit %d end %s err %v", res.ExitCode, res.End, err)
	}
	if !wait(func() bool { return stdruntime.NumGoroutine() <= before }) {
		t.Fatalf("goroutines %d, was %d", stdruntime.NumGoroutine(), before)
	}
}

// A budget stop is named in result.json, not mistaken for a crash.
func TestRunBudgetStop(t *testing.T) {
	f := newFixture(t, false, false)
	res, err := f.run(emit(assistant(100), `{"type":"result","subtype":"error_max_budget_usd","is_error":true,"session_id":"s1"}`))
	if err != nil || res.ExitCode != 5 || res.End != "max_budget" || res.ResultSubtype != "error_max_budget_usd" {
		t.Fatalf("%+v %v", res, err)
	}
}

type countRuntime struct {
	runtime.AgentRuntime
	verifies atomic.Int32
}

func (c *countRuntime) Verify(ctx context.Context) error {
	c.verifies.Add(1)
	return c.AgentRuntime.Verify(ctx)
}

// Verify runs exactly once per run, and a renamed flag (a missing help token)
// is exit 3 with ErrUnverified before any git write.
func TestRunVerifyOnce(t *testing.T) {
	f := newFixture(t, false, false)
	rt := &countRuntime{AgentRuntime: runtime.NewClaudeAdapter(runtime.AdapterConfig{
		Binary: filepath.Join(f.binDir, "claude"), NudgeTokens: 200000, MaxTokens: 300000})}
	if res, err := f.run(okScenario, func(_ *Request, d *Deps) { d.Runtime = rt }); err != nil || res.ExitCode != 0 || rt.verifies.Load() != 1 {
		t.Fatalf("verifies %d exit %d err %v", rt.verifies.Load(), res.ExitCode, err)
	}
	h := filepath.Join(f.binDir, "help.txt")
	if err := os.WriteFile(h, []byte(strings.ReplaceAll(fakeHelp, "--restricted", "--renamed")), 0o644); err != nil {
		t.Fatal(err)
	}
	f.git("update-ref", "-d", "refs/waggle/claims/ticket-23")
	_, err := f.run(okScenario, withDirs("internal/y"))
	if code(err) != ExitEnv || !errors.Is(err, runtime.ErrUnverified) {
		t.Fatalf("renamed flag: %v", err)
	}
	if sh(t, f.real, "for-each-ref", "refs/waggle/claims/") != "" {
		t.Fatal("a refused runtime wrote a claim")
	}
}

// A refused default argument surfaces from Prepare as ErrRefusedArg and exits
// like ErrUnverified: 3, never 4 or 1.
func TestRunRefusedArgExitsLikeUnverified(t *testing.T) {
	f := newFixture(t, false, false)
	rt := runtime.NewClaudeAdapter(runtime.AdapterConfig{Binary: filepath.Join(f.binDir, "claude"), NudgeTokens: 200000,
		MaxTokens: 300000, DefaultArgs: []string{"--effort=high"}})
	_, err := f.run(okScenario, func(_ *Request, d *Deps) { d.Runtime = rt })
	if !errors.Is(err, runtime.ErrRefusedArg) || code(err) != ExitEnv {
		t.Fatalf("exit %d: %v", code(err), err)
	}
}

func TestWaggleCommandsAndOutcome(t *testing.T) {
	md := "## 2. x\n- `nope`\n## 3. Build and Verification Commands\n- **Build:** `go build ./...`\n- **T:** `<test command>`\n- **V:** `go vet ./...` and `more`\n## 4. y\n- `late`\n"
	if got := strings.Join(waggleCommands([]byte(md)), "|"); got != "go build ./...|go vet ./..." {
		t.Fatalf("commands %q", got)
	}
	dirty := []comb.Violation{{Rule: ruleDirty}}
	cases := []struct {
		s       session
		vs      []comb.Violation
		commits int
		end     string
		exit    int
	}{
		{session{RunResult: runtime.RunResult{End: runtime.EndDone}}, nil, 1, "done", 0},
		{session{RunResult: runtime.RunResult{End: runtime.EndDone}}, dirty, 1, "lease_violation", 1},
		{session{RunResult: runtime.RunResult{End: runtime.EndMaxTokens}}, dirty, 0, "max_tokens", 5},
		{session{RunResult: runtime.RunResult{End: runtime.EndMaxTokens}}, []comb.Violation{{Rule: ruleHost}}, 0, "lease_violation", 1},
		{session{TimedOut: true, RunResult: runtime.RunResult{End: runtime.EndInterrupted}}, nil, 0, "timeout", 5},
		{session{RunResult: runtime.RunResult{End: runtime.EndInterrupted}}, nil, 0, "interrupted", 1},
		{session{RunResult: runtime.RunResult{End: runtime.EndError}}, nil, 1, "error", 1},
	}
	for i, c := range cases {
		if end, exit := outcome(c.s, c.vs, c.commits); end != c.end || exit != c.exit {
			t.Fatalf("case %d: %s %d, want %s %d", i, end, exit, c.end, c.exit)
		}
	}
}
