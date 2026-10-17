package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// MaxNudgeTokens caps an adapter's nudge_tokens and max_tokens, so token
// arithmetic in Settings and Collect cannot overflow.
const MaxNudgeTokens = 10_000_000

// MaxStablePrompt caps the stable prompt Prepare passes on argv. Linux limits
// one argument to 128 KiB.
const MaxStablePrompt = 100 << 10

var (
	// ErrRefusedArg reports a default_args entry that would override a flag
	// the adapter owns or continue an earlier session.
	ErrRefusedArg = errors.New("default argument refused")
	// ErrAdapterConfig reports adapter thresholds or launch inputs Prepare cannot use.
	ErrAdapterConfig = errors.New("invalid adapter launch config")
	// ErrUnverified reports a Prepare call before Verify confirmed the
	// installed binary supports every security flag.
	ErrUnverified = errors.New("runtime binary not verified")
)

// LaunchSpec is everything a Cell needs to start one session: the argv
// (Argv[0] is the binary), the complete environment, and the bytes for stdin.
type LaunchSpec struct {
	Argv, Env []string
	Stdin     []byte
}

// refusedArgs are flags default_args may not carry: resume paths (T-021 S1),
// flags the adapter sets itself, and ways to widen tools, settings, MCP or plugins.
var refusedArgs = toSet("--continue", "--resume", "--session-id", "--fork-session", "--from-pr",
	"--teleport", "--max-turns", "--model", "--mcp-config", "--print", "--output-format",
	"--input-format", "--settings", "--setting-sources", "--permission-mode",
	"--dangerously-skip-permissions", "--allow-dangerously-skip-permissions", "--allowedTools",
	"--allowed-tools", "--disallowedTools", "--disallowed-tools", "--tools", "--append-system-prompt",
	"--append-system-prompt-file", "--system-prompt", "--system-prompt-file", "--add-dir",
	"--agent", "--agents", "--plugin-dir", "--plugin-url", "--bg", "--background", "--cloud",
	"--worktree", "--strict-mcp-config", "--")

// workerTools are the only built-in tools a worker gets (D-110).
const workerTools = "Bash,Edit,Glob,Grep,Read,Write"

// deniedPrograms may not start an allowed hive command.
var deniedPrograms = toSet("gh", "curl", "wget", "ssh")

// gitAllowed are the git subcommands a worker may run besides the hive's commands.
var gitAllowed = []string{"git add", "git commit", "git status", "git diff", "git log", "git show"}

// deniedRules always win over allowed ones (D-110).
var deniedRules = []string{"Bash(git push)", "Bash(git push *)", "Bash(gh)", "Bash(gh *)",
	"Bash(curl)", "Bash(curl *)", "Bash(wget)", "Bash(wget *)", "Bash(ssh)", "Bash(ssh *)",
	"WebFetch", "WebSearch"}

// envAllowed are the parent variables a worker inherits when set (section 8).
var envAllowed = []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TMPDIR", "TERM", "TZ",
	"LANG", "LC_ALL", "LC_CTYPE", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL"}

// envFixed isolates the worker: no CLAUDE.md or auto-memory from any scope,
// no claude.ai connectors, no auto-compaction (handoffs replace it, T-021 S5),
// no git prompts, and a tripwire that rewrites every push URL to a dead scheme.
var envFixed = []string{"CLAUDE_CODE_DISABLE_CLAUDE_MDS=1", "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1",
	"ENABLE_CLAUDEAI_MCP_SERVERS=false", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
	"DISABLE_AUTO_COMPACT=1", "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_COUNT=3",
	"GIT_CONFIG_KEY_0=url.waggle-push-blocked:.pushInsteadOf", "GIT_CONFIG_VALUE_0=https://",
	"GIT_CONFIG_KEY_1=url.waggle-push-blocked:.pushInsteadOf", "GIT_CONFIG_VALUE_1=ssh://",
	"GIT_CONFIG_KEY_2=url.waggle-push-blocked:.pushInsteadOf", "GIT_CONFIG_VALUE_2=git@"}

// helpTokens must all appear in `claude --help` before Prepare runs. --max-turns
// is hidden from help; an unknown flag makes Claude Code exit, so it fails loudly.
var helpTokens = []string{"--print", "--output-format", "stream-json", "--verbose", "--model",
	"--strict-mcp-config", "--setting-sources", "--settings", "--permission-mode", "dontAsk",
	"--tools", "--append-system-prompt"}

// thinkingTokens maps the Thinking step to Claude Code's MAX_THINKING_TOKENS.
var thinkingTokens = map[Profile]int{ProfileLean: 4000, ProfileBalanced: 12000, ProfileThorough: 32000}

func toSet(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}

// Verify runs `<binary> --help` with the worker environment and checks it
// lists every flag Prepare depends on. Prepare refuses to run until Verify
// has succeeded once, so a Claude Code version without a security flag fails closed.
func (c *ClaudeAdapter) Verify(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var out capWriter
	cmd := exec.CommandContext(ctx, c.cfg.Binary, "--help")
	cmd.Env, cmd.Stdout, cmd.WaitDelay = workerEnv(ContextSettings{}), &out, time.Second
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s --help: %v", ErrUnverified, c.cfg.Binary, err)
	}
	var missing []string
	for _, tok := range helpTokens {
		if !bytes.Contains(out.buf.Bytes(), []byte(tok)) {
			missing = append(missing, tok)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s --help lacks %s", ErrUnverified, c.cfg.Binary, strings.Join(missing, ", "))
	}
	c.verified.Store(true)
	return nil
}

// capWriter keeps the first 1 MiB written to it and drops the rest.
type capWriter struct{ buf bytes.Buffer }

func (w *capWriter) Write(p []byte) (int, error) {
	if room := 1<<20 - w.buf.Len(); room > 0 {
		w.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

// Prepare builds one headless session: print mode with stream-json output,
// the resolved model, no setting sources (no user, project or local settings,
// hooks, plugins or CLAUDE.md), the D-110 tool allowlist and deny list as an
// inline settings file, strict MCP, --max-turns, and the stable prompt appended
// to the system prompt. The variable prompt goes on stdin only. It refuses an
// unresolved Selection, an unverified binary, bad thresholds, a refused
// default argument, and an allowlisted command that could widen the rules.
func (c *ClaudeAdapter) Prepare(inv Invocation, s ContextSettings, stable, variable []byte) (LaunchSpec, error) {
	if !inv.Selection.resolved || inv.Selection.Role != inv.Role {
		return LaunchSpec{}, fmt.Errorf("%w: role %s task %s", ErrUnresolved, inv.Role, inv.TaskID)
	}
	if !c.verified.Load() {
		return LaunchSpec{}, ErrUnverified
	}
	cfg := c.cfg
	switch {
	case cfg.NudgeTokens < 1000 || cfg.MaxTokens > MaxNudgeTokens || cfg.NudgeTokens >= cfg.MaxTokens:
		return LaunchSpec{}, fmt.Errorf("%w: need 1000 <= nudge_tokens (%d) < max_tokens (%d) <= %d",
			ErrAdapterConfig, cfg.NudgeTokens, cfg.MaxTokens, MaxNudgeTokens)
	case s.MaxTurns < 1 || s.ToolOutputChars < 1:
		return LaunchSpec{}, fmt.Errorf("%w: settings not resolved (max turns %d)", ErrAdapterConfig, s.MaxTurns)
	case len(stable) > MaxStablePrompt || bytes.IndexByte(stable, 0) >= 0:
		return LaunchSpec{}, fmt.Errorf("%w: stable prompt over %d bytes or holds NUL", ErrAdapterConfig, MaxStablePrompt)
	}
	settings, err := workerSettings(inv.Commands)
	if err != nil {
		return LaunchSpec{}, err
	}
	argv, err := c.claudeArgs(inv, s, settings, stable)
	if err != nil {
		return LaunchSpec{}, err
	}
	return LaunchSpec{Argv: argv, Env: workerEnv(s), Stdin: variable}, nil
}

// claudeArgs assembles the argv. Every value flag uses the --flag=value form
// so a value can never be read as a flag. It never emits a resume flag.
func (c *ClaudeAdapter) claudeArgs(inv Invocation, s ContextSettings, settings, stable []byte) ([]string, error) {
	for _, a := range c.cfg.DefaultArgs {
		flag, _, _ := strings.Cut(a, "=")
		short := len(flag) > 1 && flag[0] == '-' && flag[1] != '-' && strings.ContainsAny(flag[1:], "cprw")
		if refusedArgs[flag] || short {
			return nil, fmt.Errorf("%w: default_args contains %q", ErrRefusedArg, a)
		}
	}
	args := []string{c.cfg.Binary, "--print", "--output-format=stream-json", "--verbose",
		"--model=" + inv.Selection.Model, "--setting-sources=", "--settings=" + string(settings),
		"--strict-mcp-config", "--tools=" + workerTools, "--permission-mode=dontAsk",
		"--max-turns=" + strconv.Itoa(s.MaxTurns), "--append-system-prompt=" + string(stable)}
	return append(args, c.cfg.DefaultArgs...), nil
}

// workerSettings renders the settings JSON: allow rules for the read, edit and
// write tools, each hive command in commands, and gitAllowed; deny rules from
// deniedRules. A command must be plain words: no wildcard, quote, control
// character, shell operator or rule syntax, and none of the denied programs.
func workerSettings(commands []string) ([]byte, error) {
	allow := []string{"Read", "Glob", "Grep", "Edit", "Write"}
	for _, cmd := range append(append([]string(nil), commands...), gitAllowed...) {
		f := strings.Fields(cmd)
		if len(f) == 0 || strings.Join(f, " ") != cmd || strings.ContainsAny(cmd, "*()\"'`$;&|<>\\") ||
			strings.ContainsFunc(cmd, func(r rune) bool { return r < 0x20 || r == 0x7f }) ||
			deniedPrograms[f[0]] || (len(f) > 1 && f[0] == "git" && f[1] == "push") {
			return nil, fmt.Errorf("%w: allowed command %q", ErrAdapterConfig, cmd)
		}
		allow = append(allow, "Bash("+cmd+")", "Bash("+cmd+" *)")
	}
	type perms struct {
		Allow       []string `json:"allow"`
		Deny        []string `json:"deny"`
		DefaultMode string   `json:"defaultMode"`
	}
	return json.Marshal(struct {
		Permissions perms `json:"permissions"`
	}{perms{allow, deniedRules, "dontAsk"}})
}

// workerEnv builds the environment from envAllowed (only when set in the
// parent), envFixed and the settings values. It never copies os.Environ().
func workerEnv(s ContextSettings) []string {
	var env []string
	for _, k := range envAllowed {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	env = append(env, envFixed...)
	if s.ToolOutputChars > 0 {
		env = append(env, "BASH_MAX_OUTPUT_LENGTH="+strconv.Itoa(s.ToolOutputChars))
	}
	if n := thinkingTokens[s.Thinking]; n > 0 {
		env = append(env, "MAX_THINKING_TOKENS="+strconv.Itoa(n))
	}
	return env
}
