package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fullHelp lists every token Verify looks for, as the pinned Claude Code does.
const fullHelp = "--print --output-format stream-json --verbose --model --strict-mcp-config " +
	"--setting-sources --settings --permission-mode dontAsk --tools --append-system-prompt --restricted " +
	"--no-session-persistence --disable-slash-commands --debug --max-budget-usd"

// fakeClaude installs an executable `claude` shell script on a temp PATH, so
// no test ever runs the real binary.
func fakeClaude(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "claude"), "#!/bin/sh\n"+script)
	if err := os.Chmod(filepath.Join(dir, "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":/usr/bin:/bin")
}

func verifiedAdapter(t *testing.T, cfg AdapterConfig) *ClaudeAdapter {
	t.Helper()
	fakeClaude(t, "echo '"+fullHelp+"'\n")
	if cfg.NudgeTokens == 0 {
		cfg.NudgeTokens, cfg.MaxTokens = 200000, 300000
	}
	a := NewClaudeAdapter(cfg)
	if err := a.Verify(context.Background()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	return a
}

func testLaunch(t *testing.T, task string) (Invocation, ContextSettings) {
	t.Helper()
	s, err := ContextPolicy{Default: ProfileLean}.Settings("builder", &TaskContext{TaskID: task}, 200000)
	if err != nil {
		t.Fatal(err)
	}
	sel := mustResolve(t, testPolicy(), "builder", nil, nil)
	return Invocation{Role: "builder", TaskID: task, Selection: sel, Commands: []string{"go build", "go test"}}, s
}

func mustPrepare(t *testing.T, a *ClaudeAdapter, task string, variable []byte) LaunchSpec {
	t.Helper()
	inv, s := testLaunch(t, task)
	spec, err := a.Prepare(inv, s, []byte("# Role\n\n# WAGGLE.md\n"), variable)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	return spec
}

func argValue(argv []string, flag string) (string, bool) {
	for _, a := range argv {
		if v, ok := strings.CutPrefix(a, flag+"="); ok {
			return v, true
		}
	}
	return "", false
}

// Test 1: print mode, stream-json, model, strict MCP, max turns, isolation
// flags and settings; never a resume flag; every refused default arg fails.
func TestPrepareArgv(t *testing.T) {
	a := verifiedAdapter(t, AdapterConfig{DefaultArgs: []string{"--max-budget-usd=2.50", "--debug"}})
	argv := mustPrepare(t, a, "T-5", []byte("brief")).Argv
	if argv[0] != "claude" || argv[len(argv)-1] != "--debug" || argv[len(argv)-2] != "--max-budget-usd=2.50" {
		t.Errorf("argv must start with the binary and end with default_args: %q", argv)
	}
	for _, want := range []string{"--print", "--output-format=stream-json", "--verbose", "--model=sonnet",
		"--setting-sources=", "--strict-mcp-config", "--restricted", "--no-session-persistence", "--disable-slash-commands",
		"--permission-mode=dontAsk", "--max-turns=40",
		"--tools=Bash,Edit,Glob,Grep,Read,Write"} {
		if !slices.Contains(argv, want) {
			t.Errorf("argv lacks %q: %q", want, argv)
		}
	}
	for _, arg := range argv {
		flag, _, _ := strings.Cut(arg, "=")
		if slices.Contains([]string{"--continue", "-c", "--resume", "-r", "--session-id", "--dangerously-skip-permissions"}, flag) {
			t.Errorf("argv carries %q", arg)
		}
	}
	raw, _ := argValue(argv, "--settings")
	var settings map[string]map[string]any
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		t.Fatalf("settings JSON: %v", err)
	}
	if len(settings) != 1 || settings["permissions"]["defaultMode"] != "dontAsk" {
		t.Errorf("settings must hold only permissions (no hooks): %s", raw)
	}
	for _, want := range []string{`"Bash(go test *)"`, `"Bash(git commit *)"`, `"Edit(./**)"`, `"Write(./**)"`} {
		if !strings.Contains(raw, want) {
			t.Errorf("settings lacks allow %s: %s", want, raw)
		}
	}
	// Bare Edit and Write rules would allow writes outside the working directory.
	for _, bare := range []string{`"Edit"`, `"Write"`} {
		if strings.Contains(raw, bare) {
			t.Errorf("settings carries unscoped %s: %s", bare, raw)
		}
	}
	for _, want := range []string{`"Bash(git push *)"`, `"Bash(gh *)"`, `"Bash(curl *)"`, `"Bash(wget *)"`, `"Bash(ssh *)"`, `"WebFetch"`, `"WebSearch"`} {
		if !strings.Contains(raw[strings.Index(raw, `"deny"`):], want) {
			t.Errorf("settings lacks deny %s: %s", want, raw)
		}
	}

}

func TestPrepareRefusesBadInputs(t *testing.T) {
	inv, s := testLaunch(t, "T-5")
	if _, err := NewClaudeAdapter(AdapterConfig{NudgeTokens: 200000, MaxTokens: 300000}).Prepare(inv, s, nil, nil); !errors.Is(err, ErrUnverified) {
		t.Errorf("Prepare before Verify: %v; want ErrUnverified", err)
	}
	for _, cfg := range []AdapterConfig{{NudgeTokens: 300000, MaxTokens: 300000}, {NudgeTokens: 999, MaxTokens: 300000},
		{NudgeTokens: 200000, MaxTokens: MaxNudgeTokens + 1}} {
		if _, err := verifiedAdapter(t, cfg).Prepare(inv, s, nil, nil); !errors.Is(err, ErrAdapterConfig) {
			t.Errorf("config %+v: %v; want ErrAdapterConfig", cfg, err)
		}
	}
	a := verifiedAdapter(t, AdapterConfig{})
	if _, err := a.Prepare(inv, ContextSettings{}, nil, nil); !errors.Is(err, ErrAdapterConfig) {
		t.Errorf("zero settings: %v; want ErrAdapterConfig", err)
	}
	if _, err := a.Prepare(inv, s, bytes.Repeat([]byte("x"), MaxStablePrompt+1), nil); !errors.Is(err, ErrAdapterConfig) {
		t.Errorf("oversized stable prompt: %v; want ErrAdapterConfig", err)
	}
	for _, cmd := range []string{"", " go test", "go  test", "go test *", "go test; rm -rf /", "go test && curl x",
		"gh pr list", "curl", "wget x", "ssh host", "git push origin", "go test\nrm", "go $(id)", "Bash(x)"} {
		bad := inv
		bad.Commands = []string{cmd}
		if _, err := a.Prepare(bad, s, nil, nil); !errors.Is(err, ErrAdapterConfig) {
			t.Errorf("command %q: %v; want ErrAdapterConfig", cmd, err)
		}
	}
	if _, err := (ContextPolicy{}).Settings("builder", nil, MaxNudgeTokens+1); !errors.Is(err, ErrInvalidNudgeTokens) {
		t.Errorf("Settings over the nudge cap: %v", err)
	}
}

// Test 2: a zero, hand-built or other-role Selection is ErrUnresolved.
func TestPrepareRefusesUnresolvedSelection(t *testing.T) {
	a := verifiedAdapter(t, AdapterConfig{})
	_, s := testLaunch(t, "T-5")
	guard := mustResolve(t, testPolicy(), "guard", nil, nil)
	invs := []Invocation{
		{Role: "builder", TaskID: "T-5"},
		{Role: "builder", TaskID: "T-5", Selection: guard},
		{Role: "builder", TaskID: "T-5", Selection: Selection{Role: "builder", Model: "opus"}},
	}
	for _, inv := range invs {
		_, err := a.Prepare(inv, s, nil, nil)
		wantErr(t, err, ErrUnresolved, "builder", "T-5")
	}
}

func TestVerify(t *testing.T) {
	fakeClaude(t, "echo '--print --output-format stream-json --verbose --model'\n")
	err := NewClaudeAdapter(AdapterConfig{}).Verify(context.Background())
	wantErr(t, err, ErrUnverified, "--setting-sources", "--strict-mcp-config", "dontAsk", "--restricted")

	fakeClaude(t, "exit 3\n")
	wantErr(t, NewClaudeAdapter(AdapterConfig{}).Verify(context.Background()), ErrUnverified)
}

// Test 3: dropped variables never reach the worker; allowlisted ones and the
// push tripwire do.
func TestPrepareEnv(t *testing.T) {
	a := verifiedAdapter(t, AdapterConfig{})
	for _, k := range []string{"GITHUB_TOKEN", "GH_TOKEN", "SSH_AUTH_SOCK", "GIT_ASKPASS", "OP_SERVICE_ACCOUNT_TOKEN",
		"AWS_SECRET_ACCESS_KEY", "FOO", "GIT_CONFIG_COUNT", "CLAUDE_CODE_DISABLE_CLAUDE_MDS"} {
		t.Setenv(k, "secret-"+k)
	}
	t.Setenv("HOME", "/home/keeper")
	t.Setenv("ANTHROPIC_API_KEY", "model-key")
	env := mustPrepare(t, a, "T-5", nil).Env
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "secret-") {
		t.Errorf("env carries a dropped variable:\n%s", joined)
	}
	for _, want := range []string{"HOME=/home/keeper", "ANTHROPIC_API_KEY=model-key", "GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=14", "GIT_CONFIG_KEY_12=maintenance.auto", "GIT_CONFIG_KEY_13=gc.auto", "GIT_CONFIG_VALUE_12=false", "GIT_CONFIG_VALUE_13=0", "GIT_ATTR_NOSYSTEM=1", "GIT_CONFIG_KEY_10=core.excludesFile", "GIT_CONFIG_KEY_11=core.attributesFile", "GIT_CONFIG_KEY_0=url.file:///dev/null/.pushInsteadOf", "GIT_CONFIG_VALUE_0=https://",
		"GIT_CONFIG_VALUE_1=ssh://", "GIT_CONFIG_VALUE_2=git@", "GIT_CONFIG_VALUE_5=git+ssh://", "GIT_CONFIG_VALUE_6=ssh+git://", "GIT_CONFIG_VALUE_8=/", "GIT_CONFIG_VALUE_9=.", "CLAUDE_CODE_DISABLE_CLAUDE_MDS=1",
		"DISABLE_AUTO_COMPACT=1", "BASH_MAX_OUTPUT_LENGTH=10000", "MAX_THINKING_TOKENS=4000"} {
		if !slices.Contains(env, want) {
			t.Errorf("env lacks %q", want)
		}
	}
	seen := map[string]bool{}
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if seen[k] {
			t.Errorf("env sets %s twice", k)
		}
		seen[k] = true
	}
}

// Test 4: argv and env do not depend on the task; the variable prompt is on stdin only.
func TestPrepareStableAcrossTasks(t *testing.T) {
	a := verifiedAdapter(t, AdapterConfig{})
	one := mustPrepare(t, a, "T-5", []byte("brief five VARIABLE-5"))
	two := mustPrepare(t, a, "T-6", []byte("brief six VARIABLE-6"))
	if !slices.Equal(one.Argv, two.Argv) || !slices.Equal(one.Env, two.Env) {
		t.Errorf("argv or env differ across tasks:\n%q\n%q", one.Argv, two.Argv)
	}
	if string(one.Stdin) != "brief five VARIABLE-5" || strings.Contains(strings.Join(one.Argv, " "), "VARIABLE") {
		t.Errorf("variable prompt must be on stdin only: stdin %q", one.Stdin)
	}
	if v, _ := argValue(one.Argv, "--append-system-prompt"); v != "# Role\n\n# WAGGLE.md\n" {
		t.Errorf("stable prompt = %q", v)
	}
}

// AC7: the repo's Stop hook never runs in a worker. The fake binary follows
// Claude Code's documented rule: project settings (and their hooks) load
// unless --setting-sources leaves "project" out. The control run without the
// flag proves the fake would have fired the hook.
func TestLaunchSkipsRepoHooks(t *testing.T) {
	a := verifiedAdapter(t, AdapterConfig{})
	repo := t.TempDir()
	marker := filepath.Join(t.TempDir(), "hook-ran")
	writeFile(t, filepath.Join(repo, ".claude", "settings.json"),
		`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"sh .claude/hooks/handoff-check.sh"}]}]}}`)
	writeFile(t, filepath.Join(repo, ".claude", "hooks", "handoff-check.sh"), "echo ran > \""+marker+"\"\n")
	fakeClaude(t, `src=user,project,local
for a in "$@"; do case "$a" in --setting-sources=*) src="${a#--setting-sources=}";; esac; done
cat > /dev/null
echo '{"type":"result","subtype":"success","is_error":false,"result":"ok","session_id":"s1"}'
case ",$src," in *,project,*) [ -f .claude/settings.json ] && sh .claude/hooks/handoff-check.sh;; esac
`)
	spec := mustPrepare(t, a, "T-5", []byte("go"))
	run := func(argv []string) RunResult {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir, cmd.Env, cmd.Stdin = repo, spec.Env, bytes.NewReader(spec.Stdin)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("fake claude: %v", err)
		}
		res, err := a.Collect(context.Background(), bytes.NewReader(out), Limits{MaxTokens: 300000}, func() {})
		if err != nil || res.End != EndDone {
			t.Fatalf("Collect = %+v, %v", res, err)
		}
		return res
	}
	run(spec.Argv)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("repo Stop hook ran under the worker argv")
	}
	run(slices.DeleteFunc(slices.Clone(spec.Argv), func(s string) bool { return s == "--setting-sources=" }))
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("control run without --setting-sources did not fire the hook; the test proves nothing")
	}
}

// Phase C0 (D-137, D-162): the default_args allowlist, each form and each refusal class.
func TestDefaultArgsAllowlist(t *testing.T) {
	inv, s := testLaunch(t, "T-5")
	for _, ok := range []string{"--debug", "--max-budget-usd=5", "--max-budget-usd=0.25", "--max-budget-usd=10.50"} {
		a := verifiedAdapter(t, AdapterConfig{DefaultArgs: []string{ok}})
		if _, err := a.Prepare(inv, s, nil, nil); err != nil {
			t.Errorf("default arg %q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"--continue", "-c", "--resume", "-r", "-rabc", "-pc", "-w", "-d", "--session-id=1",
		"--max-turns=9999", "--model", "--model=opus", "--print", "-p", "--output-format", "--settings",
		"--setting-sources=user", "--permission-mode", "--dangerously-skip-permissions", "--allowedTools",
		"--tools", "--append-system-prompt", "--add-dir", "--plugin-dir", "--agents", "--", "--effort=high",
		"--effort", "--fallback-model=opus", "--remote-control", "--chrome", "--ide", "--restricted",
		"--strict-mcp-config", "--no-session-persistence", "--disable-slash-commands", "--verbose",
		"--max-budget-usd", "--max-budget-usd=", "--max-budget-usd=0", "--max-budget-usd=0.0", "--max-budget-usd=-1",
		"--max-budget-usd=+1", "--max-budget-usd=1e3", "--max-budget-usd=inf", "--max-budget-usd=NaN",
		"--max-budget-usd=1.", "--max-budget-usd=.5", "--max-budget-usd= 1", "--max-budget-usd=1 2", "--max-budget-usd=abc",
		"--debug=api", "--debug=", "--Debug", "high", "fix the bug", ""} {
		a := verifiedAdapter(t, AdapterConfig{DefaultArgs: []string{"--debug", bad}})
		if _, err := a.Prepare(inv, s, nil, nil); !errors.Is(err, ErrRefusedArg) || !strings.Contains(err.Error(), `"`+bad+`"`) {
			t.Errorf("default arg %q: err = %v; want ErrRefusedArg naming it", bad, err)
		}
	}
}

// An allowed flag the --help output does not list fails closed (a renamed flag).
func TestDefaultArgsMissingFromHelp(t *testing.T) {
	fakeClaude(t, "echo '"+strings.ReplaceAll(fullHelp, "--max-budget-usd", "--max-cost")+"'\n")
	a := NewClaudeAdapter(AdapterConfig{DefaultArgs: []string{"--max-budget-usd=3"}, NudgeTokens: 200000, MaxTokens: 300000})
	wantErr(t, a.Verify(context.Background()), ErrUnverified, "--max-budget-usd")
	inv, s := testLaunch(t, "T-5")
	if _, err := a.Prepare(inv, s, nil, nil); !errors.Is(err, ErrUnverified) {
		t.Errorf("Prepare after failed Verify = %v; want ErrUnverified", err)
	}
	// The same help is fine when the flag is not used.
	b := NewClaudeAdapter(AdapterConfig{NudgeTokens: 200000, MaxTokens: 300000})
	if err := b.Verify(context.Background()); err != nil {
		t.Errorf("Verify without default_args: %v", err)
	}
}

// The caller mutating its DefaultArgs after Verify cannot skip the help check.
func TestDefaultArgsMutatedAfterVerify(t *testing.T) {
	fakeClaude(t, "echo '"+strings.ReplaceAll(fullHelp, "--max-budget-usd", "--max-cost")+"'\n")
	args := []string{"--debug"}
	a := NewClaudeAdapter(AdapterConfig{DefaultArgs: args, NudgeTokens: 200000, MaxTokens: 300000})
	if err := a.Verify(context.Background()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	args[0] = "--max-budget-usd=3"
	argv := mustPrepare(t, a, "T-5", nil).Argv
	if slices.Contains(argv, "--max-budget-usd=3") || !slices.Contains(argv, "--debug") {
		t.Errorf("Prepare used the mutated slice: %q", argv)
	}
}

// The tripwire holds under the exact worker env: a push to an http, git, absolute
// path or ../ relative path remote fails and the bare remote stays empty.
func TestTripwireBlocksPush(t *testing.T) {
	env := append(mustPrepare(t, verifiedAdapter(t, AdapterConfig{}), "T-5", nil).Env,
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	git := func(dir string, env []string, args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	root := t.TempDir()
	work, bare := filepath.Join(root, "work"), filepath.Join(root, "bare.git")
	setup := append(slices.Clone(env), "GIT_CONFIG_COUNT=0")
	for _, c := range [][]string{{"init", "-q", bare, "--bare"}, {"init", "-q", work}} {
		if out, err := git(root, setup, c...); err != nil {
			t.Fatalf("git %v: %v\n%s", c, err, out)
		}
	}
	writeFile(t, filepath.Join(work, "f.txt"), "x\n")
	for _, c := range [][]string{{"add", "f.txt"}, {"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "m"}} {
		if out, err := git(work, setup, c...); err != nil {
			t.Fatalf("git %v: %v\n%s", c, err, out)
		}
	}
	for name, url := range map[string]string{"http": "http://127.0.0.1:1/x.git", "git": "git://127.0.0.1:1/x.git",
		"absolute": bare, "relative": "../bare.git"} {
		out, err := git(work, env, "push", url, "HEAD:refs/heads/pushed")
		if err == nil || !strings.Contains(out, "/dev/null/") {
			t.Errorf("%s push: err = %v, output %q; want a failure at the file:///dev/null/ target", name, err, out)
		}
		if refs, _ := git(bare, setup, "for-each-ref"); refs != "" {
			t.Errorf("%s push moved a ref in the bare remote: %s", name, refs)
		}
	}
}

// Item 16: editing Config().DefaultArgs after Verify leaves the next argv unchanged.
func TestConfigDoesNotAliasDefaultArgs(t *testing.T) {
	a := verifiedAdapter(t, AdapterConfig{DefaultArgs: []string{"--debug"}})
	a.Config().DefaultArgs[0] = "--max-budget-usd=3"
	if argv := mustPrepare(t, a, "T-5", nil).Argv; slices.Contains(argv, "--max-budget-usd=3") || !slices.Contains(argv, "--debug") {
		t.Errorf("argv follows the edited copy: %q", argv)
	}
}

// Item 17: --debug-file in the help does not satisfy --debug, and a quote or
// parenthesis after a token (as in the real help) still counts.
func TestVerifyHelpTokenBoundary(t *testing.T) {
	fakeClaude(t, "echo '"+strings.ReplaceAll(fullHelp, "--debug", "--debug-file")+"'\n")
	a := NewClaudeAdapter(AdapterConfig{DefaultArgs: []string{"--debug"}})
	wantErr(t, a.Verify(context.Background()), ErrUnverified, "--debug")
	if !helpHas([]byte("(--debug, \"dontAsk\")"), "dontAsk") || helpHas([]byte("--debug-file"), "--debug") {
		t.Error("helpHas boundary wrong")
	}
}

// Item 19, runtime half: partly set or control-character identities are refused;
// a zero identity sets both GIT_CONFIG_* values and no author or committer.
func TestPrepareGitIdentity(t *testing.T) {
	a := verifiedAdapter(t, AdapterConfig{})
	inv, s := testLaunch(t, "T-5")
	for _, id := range []GitIdentity{{Name: "A"}, {Email: "a@example.test"}, {Name: "A\nB", Email: "a@example.test"}, {Name: "A", Email: "a@example.test\x00"},
		{Name: "A\u0085B", Email: "a@example.test"}, {Name: "A\u009bB", Email: "a@example.test"}, {Name: "A\xffB", Email: "a@example.test"},
		{Name: "A\u2028B", Email: "a@example.test"}, {Name: "A", Email: "a@example.test\u2029"}, {Name: "  ", Email: "a@example.test"}, {Name: "A", Email: " "}} {
		inv.GitIdentity = id
		_, err := a.Prepare(inv, s, nil, nil)
		wantErr(t, err, ErrAdapterConfig, "git identity")
	}
	for _, id := range []GitIdentity{{}, {Name: "  ", Email: " "}} {
		inv.GitIdentity = id
		spec, err := a.Prepare(inv, s, nil, nil)
		if err != nil {
			t.Fatalf("identity %q: %v", id, err)
		}
		for _, kv := range spec.Env {
			if strings.HasPrefix(kv, "GIT_AUTHOR_") || strings.HasPrefix(kv, "GIT_COMMITTER_") {
				t.Errorf("zero identity %q sets %s", id, kv)
			}
		}
	}
	spec, err := a.Prepare(inv, s, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"} {
		if !slices.Contains(spec.Env, want) {
			t.Errorf("env lacks %q", want)
		}
	}
	inv.GitIdentity = GitIdentity{Name: "Hive Worker", Email: "worker@example.test"}
	spec, err = a.Prepare(inv, s, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"GIT_AUTHOR_NAME=Hive Worker", "GIT_AUTHOR_EMAIL=worker@example.test", "GIT_COMMITTER_NAME=Hive Worker", "GIT_COMMITTER_EMAIL=worker@example.test"} {
		if !slices.Contains(spec.Env, want) {
			t.Errorf("env lacks %q", want)
		}
	}
}

// Item 20: the guard gets no git add or git commit rule; the builder keeps both.
func TestGuardSettingsDropGitWrites(t *testing.T) {
	guard, err := workerSettings("guard", []string{"go test"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(guard), "Bash(git add") || strings.Contains(string(guard), "Bash(git commit") || !strings.Contains(string(guard), "Bash(git diff") {
		t.Errorf("guard settings: %s", guard)
	}
	builder, _ := workerSettings("builder", []string{"go test"})
	if !strings.Contains(string(builder), "Bash(git add") || !strings.Contains(string(builder), "Bash(git commit") {
		t.Errorf("builder settings: %s", builder)
	}
}

// Fable N2: the home ignore and attributes files do not apply under the produced env.
func TestEnvIgnoresHomeIgnoreAndAttributes(t *testing.T) {
	home, repo := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home) // Prepare passes HOME through
	writeFile(t, filepath.Join(home, ".config", "git", "ignore"), "hidden.txt\n")
	writeFile(t, filepath.Join(home, ".config", "git", "attributes"), "a.txt filter=probe\n")
	run := func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = repo, env
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	base := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "XDG_CONFIG_HOME="}
	if out, err := run(base, "init", "-q"); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if _, err := run(base, "check-ignore", "-q", "hidden.txt"); err != nil {
		t.Fatalf("control: home ignore does not apply to a plain env: %v", err)
	}
	if out, _ := run(base, "check-attr", "filter", "a.txt"); !strings.HasSuffix(out, "probe") {
		t.Fatalf("control: home attributes do not apply to a plain env: %q", out)
	}
	env := append(base, mustPrepare(t, verifiedAdapter(t, AdapterConfig{}), "T-5", nil).Env...)
	if _, err := run(env, "check-ignore", "-q", "hidden.txt"); err == nil {
		t.Error("home ignore file still applies under the worker env")
	}
	if out, _ := run(env, "check-attr", "filter", "a.txt"); !strings.HasSuffix(out, "unspecified") {
		t.Errorf("home attributes still apply under the worker env: %q", out)
	}
}
