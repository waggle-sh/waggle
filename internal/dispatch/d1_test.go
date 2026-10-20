//go:build unix

package dispatch

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/runtime"
)

// nothingWritten fails when a refused run left a claim, a branch, a worktree or a run record.
func nothingWritten(t *testing.T, f *fixture) {
	t.Helper()
	if got := sh(t, f.real, "for-each-ref", "refs/waggle/claims/", "refs/heads/t-023"); got != "" {
		t.Fatalf("a refused run wrote refs: %s", got)
	}
	for _, p := range []string{"T-023-builder", "runs"} {
		if _, err := os.Stat(filepath.Join(f.real, ".waggle/worktrees", p)); err == nil {
			t.Fatalf("a refused run created .waggle/worktrees/%s", p)
		}
	}
}

// Item 1: a hook planted after a clean run makes the next dispatch exit 3,
// naming the earlier run, before any fetch.
func TestHostRecordAcrossRuns(t *testing.T) {
	f := newFixture(t, true, false)
	if res, err := f.run(okScenario); err != nil || res.ExitCode != 0 {
		t.Fatalf("first run: exit %d err %v", res.ExitCode, err)
	}
	if _, err := os.Stat(filepath.Join(f.real, ".waggle/worktrees/host-files.json")); err != nil {
		t.Fatal(err)
	}
	bin, log := gitWrapper(t)
	write(t, f.common(), "hooks/post-checkout", "#!/bin/sh\n")
	_, err := f.run(okScenario, func(_ *Request, d *Deps) { d.GitBin = bin })
	if code(err) != ExitEnv || !strings.Contains(err.Error(), "since run T-023-builder-1") || !strings.Contains(err.Error(), "hooks/post-checkout") {
		t.Fatalf("exit %d: %v", code(err), err)
	}
	if b, _ := os.ReadFile(log); strings.Contains(string(b), "fetch") {
		t.Fatalf("a fetch ran before the host record was compared:\n%s", b)
	}
	if err := os.Remove(filepath.Join(f.common(), "hooks/post-checkout")); err != nil {
		t.Fatal(err)
	}
	if res, err := f.run(commitFile("internal/x/c.go", "ok") + emit(assistant(100), okResult)); err != nil || res.ExitCode != 0 {
		t.Fatalf("after restoring the host: exit %d err %v", res.ExitCode, err)
	}
}

// Item 2: global files are keyed by role, so no violation, result.json or
// stdout line names a $HOME-like path.
func TestHostKeysPathFree(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	f := newFixture(t, true, false)
	res, err := f.run(okScenario + "mkdir -p '" + xdg + "/git'\necho '*' > '" + xdg + "/git/ignore'\n")
	if err != nil || res.ExitCode != 1 || !hasViolation(res.Violations, ruleHost, "global:xdg/ignore") {
		t.Fatalf("exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
	if b, _ := os.ReadFile(filepath.Join(res.RunDir, "result.json")); strings.Contains(string(b), xdg) || strings.Contains(string(res.JSON), xdg) {
		t.Fatal("result.json names the XDG path")
	}
}

// Item 3: the agent rewrites its own claim ref after the last renewal.
func TestOwnClaimRefRewritten(t *testing.T) {
	f := newFixture(t, false, false)
	res, err := f.run(okScenario + "git update-ref refs/waggle/claims/ticket-23 HEAD\n")
	if err != nil || res.ExitCode != 1 || !hasViolation(res.Violations, ruleHost, "refs/waggle/claims/ticket-23") {
		t.Fatalf("exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
}

// Item 4: a refused default argument and a refused WAGGLE.md command each exit
// 3 with no claim ref, worktree or run directory.
func TestRefusalsWriteNothing(t *testing.T) {
	f := newFixture(t, false, false)
	rt := runtime.NewClaudeAdapter(runtime.AdapterConfig{Binary: filepath.Join(f.binDir, "claude"), NudgeTokens: 200000,
		MaxTokens: 300000, DefaultArgs: []string{"--effort=high"}})
	if _, err := f.run(okScenario, func(_ *Request, d *Deps) { d.Runtime = rt }); !errorsIs(err, runtime.ErrRefusedArg) || code(err) != ExitEnv {
		t.Fatalf("refused arg: exit %d %v", code(err), err)
	}
	nothingWritten(t, f)
	md, _ := os.ReadFile(filepath.Join(f.real, ".waggle/control/WAGGLE.md"))
	write(t, f.real, ".waggle/control/WAGGLE.md", strings.Replace(string(md), "## 3. Build and Verification Commands\n", "## 3. Build and Verification Commands\n- **Evil:** `git push origin`\n", 1))
	f.git("add", "-A")
	f.git("commit", "-q", "-m", "evil command")
	if _, err := f.run(okScenario); code(err) != ExitEnv || !errorsIs(err, runtime.ErrAdapterConfig) {
		t.Fatalf("refused command: exit %d %v", code(err), err)
	}
	nothingWritten(t, f)
}

func errorsIs(err, target error) bool {
	return err != nil && strings.Contains(err.Error(), target.Error())
}

// Item 6: the runtime directory and the ledger must be gitignored, or nothing is written.
func TestIgnoreCheck(t *testing.T) {
	for _, line := range []string{".waggle/worktrees/", ".waggle/control/usage.jsonl"} {
		t.Run(line, func(t *testing.T) {
			f := newFixture(t, false, false)
			gi, _ := os.ReadFile(filepath.Join(f.real, ".gitignore"))
			write(t, f.real, ".gitignore", strings.Replace(string(gi), line+"\n", "", 1))
			_, err := f.run(okScenario)
			if code(err) != ExitEnv || !strings.Contains(err.Error(), line) {
				t.Fatalf("exit %d: %v", code(err), err)
			}
			nothingWritten(t, f)
			if _, e := os.Stat(filepath.Join(f.real, ".waggle/worktrees")); e == nil {
				t.Fatal("a refused run created .waggle/worktrees")
			}
		})
	}
}

// Item 7: a budget-stop result event with a 300 KiB report still ends max_budget.
func TestTapLongResultLine(t *testing.T) {
	f := newFixture(t, false, false)
	big := `{"type":"result","subtype":"error_max_budget_usd","is_error":true,"result":"` + strings.Repeat("y", 300<<10) + `","session_id":"s1"}`
	res, err := f.run(emit(assistant(100), big))
	if err != nil || res.End != "max_budget" || res.ExitCode != 5 || res.ResultSubtype != "error_max_budget_usd" {
		t.Fatalf("end %s subtype %q exit %d err %v", res.End, res.ResultSubtype, res.ExitCode, err)
	}
}

// Item 8: the files step runs no git. The first git process after the session
// ends is the first pinned call, for-each-ref.
func TestFilesStepRunsNoGit(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "trace")
	t.Setenv("GIT_TRACE", trace)
	f := newFixture(t, false, false)
	if res, err := f.run(okScenario + "echo SESSION-END >> '" + trace + "'\n"); err != nil || res.ExitCode != 0 {
		t.Fatalf("exit %d err %v", res.ExitCode, err)
	}
	b, _ := os.ReadFile(trace)
	_, after, ok := strings.Cut(string(b), "SESSION-END\n")
	if !ok {
		t.Fatal("no session marker in the trace")
	}
	for _, l := range strings.Split(after, "\n") {
		if strings.Contains(l, "run_command: git") || strings.Contains(l, "built-in: git") {
			if !strings.Contains(l, "for-each-ref") {
				t.Fatalf("a git process ran before the first pinned call: %s", l)
			}
			return
		}
	}
	t.Fatal("no git process after the session")
}

// Item 9: the section is found by its title, and none is a warning.
func TestWaggleCommandsByTitle(t *testing.T) {
	md := "## 2. x\n- `nope`\n## 7. Build and Verification Commands\n- **Build:** `go build ./...`\n## 8. y\n- `late`\n"
	if got := strings.Join(waggleCommands([]byte(md)), "|"); got != "go build ./..." {
		t.Fatalf("commands %q", got)
	}
	if got := waggleCommands([]byte("## 3. Build\n- `go build ./...`\n")); len(got) != 0 {
		t.Fatalf("a section with another title yielded %q", got)
	}
	f := newFixture(t, false, false)
	res, err := f.run(okScenario)
	if err != nil || !strings.Contains(strings.Join(res.Warnings, "|"), "no_waggle_commands") {
		t.Fatalf("a WAGGLE.md with placeholder commands must warn: %v %v", res.Warnings, err)
	}
}

// Item 10: a parent GIT_CONFIG_PARAMETERS naming a hook directory runs no hook
// during a claim write, and the comb environment carries the maintenance keys.
func TestCombEnvIgnoresParentParameters(t *testing.T) {
	for _, want := range []string{"GIT_CONFIG_PARAMETERS=", "GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false", "GIT_CONFIG_KEY_2=gc.auto", "GIT_CONFIG_VALUE_2=0"} {
		if !slices.Contains(combEnv, want) {
			t.Fatalf("combEnv lacks %s", want)
		}
	}
	hook, canary := canaryScript(t, "")
	hooks := t.TempDir()
	if err := os.Rename(hook, filepath.Join(hooks, "reference-transaction")); err != nil {
		t.Fatal(err)
	}
	f := newFixture(t, false, false)
	t.Setenv("GIT_CONFIG_PARAMETERS", "'core.hooksPath'='"+hooks+"'")
	if _, err := f.run(emit(assistant(100), okResult)); err != nil {
		t.Fatal(err)
	}
	noCanary(t, canary, "a hook named by the parent's GIT_CONFIG_PARAMETERS")
}

// Item 11: the nested-repo filter canary on the main-checkout status call.
func TestNestedRepoInMainCheckout(t *testing.T) {
	f := newFixture(t, false, false)
	_, canary := canaryScript(t, "")
	script := okScenario + "git init -q '" + f.real + "/nested'\ncd '" + f.real + "/nested'\necho '* filter=x' > .gitattributes\necho a > f\ngit add .\n" +
		"git -c user.name=a -c user.email=a@e.co commit -q -m n\ntouch -t 203501010000 f\n" +
		"git config filter.x.clean \"sh -c 'touch " + canary + "; cat'\"\n"
	res, err := f.run(script)
	if err != nil || res.ExitCode != 1 || !hasViolation(res.Violations, ruleHost, ".") {
		t.Fatalf("exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
	noCanary(t, canary, "the nested repo's clean filter (main checkout status)")
}

// Item 11: the same nested repo in a worktree a failed run left behind; the
// reuse path's status call (checkClean) must not fire it either.
func TestNestedRepoOnReuse(t *testing.T) {
	f := newFixture(t, false, false)
	_, canary := canaryScript(t, "")
	script := okScenario + "git init -q nested\ncd nested\necho '* filter=x' > .gitattributes\necho a > f\ngit add .\n" +
		"git -c user.name=a -c user.email=a@e.co commit -q -m n\ntouch -t 203501010000 f\n" +
		"git config filter.x.clean \"sh -c 'touch " + canary + "; cat'\"\n"
	if res, err := f.run(script); err != nil || res.ExitCode != 1 {
		t.Fatalf("first run: exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
	if _, err := f.run(okScenario); code(err) != ExitEnv || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("reuse: %v", err)
	}
	noCanary(t, canary, "the nested repo's clean filter (reuse status)")
}

// Item 11: after a failed final sync another ticket's claim mirror is compared strictly.
func TestFailedSyncIsStrict(t *testing.T) {
	f := newFixture(t, true, false)
	res, err := f.run(okScenario + "mv '" + f.origin + "' '" + f.origin + ".gone'\ngit update-ref refs/waggle/claims/ticket-24 HEAD\n")
	if err != nil || res.ExitCode != 1 || !hasViolation(res.Violations, ruleHost, "refs/waggle/claims/ticket-24") ||
		!strings.Contains(strings.Join(res.Warnings, "|"), "claim_sync_failed") {
		t.Fatalf("exit %d err %v warnings %v violations %v", res.ExitCode, err, res.Warnings, res.Violations)
	}
}

// Item 19, dispatch half: the commit's author and sign-off name the repo
// identity, the Keeper's global pushInsteadOf no longer reaches the worker, a
// global identity is the fallback, and none is exit 3 with nothing written.
func TestWorkerIdentity(t *testing.T) {
	commit := "mkdir -p internal/x\necho ok > internal/x/b.go\ngit add -A\ngit commit -q -s -m 'T-023: id'\n"
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	write(t, filepath.Dir(cfg), filepath.Base(cfg), "[user]\n\tname = Global Keeper\n\temail = global@example.com\n[url \"git@elsewhere:\"]\n\tpushInsteadOf = https://github.com/\n")
	seen := filepath.Join(t.TempDir(), "seen")
	script := commit + "git config --list > '" + seen + "'\n" + emit(assistant(100), okResult)

	f := newFixture(t, false, false)
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	res, err := f.run(script)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
	if got := f.git("log", "-1", "--format=%an <%ae>|%b", "refs/heads/t-023"); !strings.HasPrefix(got, "Fixture Keeper <keeper@example.com>|") || !strings.Contains(got, "Signed-off-by: Fixture Keeper <keeper@example.com>") {
		t.Fatalf("commit identity: %q", got)
	}
	if b, _ := os.ReadFile(seen); strings.Contains(string(b), "elsewhere") {
		t.Fatalf("the Keeper's global pushInsteadOf reached the worker:\n%s", b)
	}

	g := newFixture(t, false, false)
	g.git("config", "--local", "--unset", "user.name")
	g.git("config", "--local", "--unset", "user.email")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	if res, err := g.run(commit + emit(assistant(100), okResult)); err != nil || res.ExitCode != 0 {
		t.Fatalf("global fallback: exit %d err %v", res.ExitCode, err)
	} else if got := g.git("log", "-1", "--format=%an <%ae>", "refs/heads/t-023"); got != "Global Keeper <global@example.com>" {
		t.Fatalf("global identity: %q", got)
	}

	h := newFixture(t, false, false)
	h.git("config", "--local", "--unset", "user.email")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	if _, err := h.run(okScenario); code(err) != ExitEnv || !strings.Contains(err.Error(), "no git identity") {
		t.Fatalf("no identity: exit %d %v", code(err), err)
	}
	nothingWritten(t, h)
}
