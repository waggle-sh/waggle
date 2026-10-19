package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
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

// allowedDefaultArgs are the only default_args entries a worker may carry
// (D-137, D-162), each in its exact form. The value says whether the flag takes
// a value: --max-budget-usd=<positive decimal> needs one, --debug takes none
// (its logs land under the Keeper's HOME, outside the worktree).
// Everything else is refused: other flags, bare words (Claude Code appends them
// to the prompt), short flags, "--", and every flag the fixed argv sets or
// T-012 owns (--effort). Verify also requires each allowed flag in --help.
var allowedDefaultArgs = map[string]bool{"--max-budget-usd": true, "--debug": false}

var decimalRe = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

// checkDefaultArg returns nil when a is an allowed default_args entry.
func checkDefaultArg(a string) error {
	flag, val, hasVal := strings.Cut(a, "=")
	needsVal, ok := allowedDefaultArgs[flag]
	if ok && needsVal == hasVal {
		if !hasVal {
			return nil
		}
		if f, err := strconv.ParseFloat(val, 64); decimalRe.MatchString(val) && err == nil && f > 0 {
			return nil
		}
	}
	return fmt.Errorf("%w: default_args contains %q", ErrRefusedArg, a)
}

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

// pushBlocked are the remote prefixes the tripwire rewrites to a dead target.
// It is best effort, defense in depth behind the Bash deny rules and 7.2 rule 4.
// It does not catch: bare relative paths (including one reached through a
// worker-made symlink), scp forms without git@ (host:path, user@host:path),
// upper-case schemes (the prefix match is case-sensitive), <transport>::<address>
// forms, remote.<name>.pushurl, and a longer pushInsteadOf from git -c or from
// the Keeper's global git config.
var pushBlocked = []string{"https://", "ssh://", "git@", "http://", "git://", "git+ssh://", "ssh+git://", "file://", "/", "."}

// envFixed isolates the worker: no CLAUDE.md or auto-memory from any scope,
// no claude.ai connectors, no auto-compaction (handoffs replace it, T-021 S5),
// no git prompts, no global or system git config, and a tripwire (tripwireEnv)
// that rewrites every push URL it recognizes (see pushBlocked) to
// file:///dev/null/, a dead target that makes no network or ssh attempt.
var envFixed = append([]string{"CLAUDE_CODE_DISABLE_CLAUDE_MDS=1", "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1",
	"ENABLE_CLAUDEAI_MCP_SERVERS=false", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
	"DISABLE_AUTO_COMPACT=1", "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}, tripwireEnv()...)

func tripwireEnv() []string {
	env := []string{"GIT_CONFIG_COUNT=" + strconv.Itoa(len(pushBlocked))}
	for i, prefix := range pushBlocked {
		n := strconv.Itoa(i)
		env = append(env, "GIT_CONFIG_KEY_"+n+"=url.file:///dev/null/.pushInsteadOf", "GIT_CONFIG_VALUE_"+n+"="+prefix)
	}
	return env
}

// helpTokens must all appear in `claude --help` before Prepare runs. --max-turns
// is hidden from help; an unknown flag makes Claude Code exit, so it fails loudly.
var helpTokens = []string{"--print", "--output-format", "stream-json", "--verbose", "--model",
	"--strict-mcp-config", "--setting-sources", "--settings", "--permission-mode", "dontAsk",
	"--tools", "--append-system-prompt", "--restricted", "--no-session-persistence", "--disable-slash-commands"}

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
	cmd.Env, cmd.Stdout, cmd.WaitDelay = workerEnv(ContextSettings{}, GitIdentity{}), &out, time.Second
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s --help: %v", ErrUnverified, c.cfg.Binary, err)
	}
	var missing []string
	// An allowed default_args flag the help does not list is a renamed flag: fail closed.
	// (The adapter has no field for the help text, so the check lives here.)
	tokens := slices.Clone(helpTokens)
	for _, a := range c.cfg.DefaultArgs {
		flag, _, _ := strings.Cut(a, "=")
		if _, ok := allowedDefaultArgs[flag]; ok {
			tokens = append(tokens, flag)
		}
	}
	for _, tok := range tokens {
		if !helpHas(out.buf.Bytes(), tok) {
			missing = append(missing, tok)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s --help lacks %s", ErrUnverified, c.cfg.Binary, strings.Join(missing, ", "))
	}
	// Later changes to the caller's slice must not skip the help check above.
	c.cfg.DefaultArgs = slices.Clone(c.cfg.DefaultArgs)
	c.verified.Store(true)
	return nil
}

// helpHas reports whether help holds tok not followed by a flag-name character,
// so --debug-file does not satisfy --debug. A quote or parenthesis may follow:
// the real help prints "stream-json" and "dontAsk" quoted.
func helpHas(help []byte, tok string) bool {
	for rest := help; ; {
		i := bytes.Index(rest, []byte(tok))
		if i < 0 {
			return false
		}
		rest = rest[i+len(tok):]
		if len(rest) == 0 || !strings.ContainsRune("-_0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", rune(rest[0])) {
			return true
		}
	}
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
// inline settings file, strict MCP, --restricted (file tools confined to the
// working directory), --max-turns, and the stable prompt appended
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
	if err := inv.GitIdentity.check(); err != nil {
		return LaunchSpec{}, err
	}
	settings, err := workerSettings(inv.Role, inv.Commands)
	if err != nil {
		return LaunchSpec{}, err
	}
	argv, err := c.claudeArgs(inv, s, settings, stable)
	if err != nil {
		return LaunchSpec{}, err
	}
	return LaunchSpec{Argv: argv, Env: workerEnv(s, inv.GitIdentity), Stdin: variable}, nil
}

// claudeArgs assembles the argv. Every value flag uses the --flag=value form
// so a value can never be read as a flag. It never emits a resume flag.
func (c *ClaudeAdapter) claudeArgs(inv Invocation, s ContextSettings, settings, stable []byte) ([]string, error) {
	for _, a := range c.cfg.DefaultArgs {
		if err := checkDefaultArg(a); err != nil {
			return nil, err
		}
	}
	args := []string{c.cfg.Binary, "--print", "--output-format=stream-json", "--verbose",
		"--model=" + inv.Selection.Model, "--setting-sources=", "--settings=" + string(settings),
		"--strict-mcp-config", "--restricted", "--no-session-persistence", "--disable-slash-commands", "--tools=" + workerTools, "--permission-mode=dontAsk",
		"--max-turns=" + strconv.Itoa(s.MaxTurns), "--append-system-prompt=" + string(stable)}
	return append(args, c.cfg.DefaultArgs...), nil
}

// check refuses a partly set identity or a value with a control character. The
// zero value is allowed: the worker then gets no identity variables.
func (g GitIdentity) check() error {
	if (g.Name == "") != (g.Email == "") ||
		strings.ContainsFunc(g.Name+g.Email, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return fmt.Errorf("%w: git identity needs a name and an email without control characters", ErrAdapterConfig)
	}
	return nil
}

// workerSettings renders the settings JSON: allow rules for the read tools, the
// edit and write tools scoped to the working directory, each hive command in commands, and gitAllowed; deny rules from
// deniedRules. A command must be plain words: no wildcard, quote, control
// character, shell operator or rule syntax, and none of the denied programs.
func workerSettings(role string, commands []string) ([]byte, error) {
	allow := []string{"Read", "Glob", "Grep", "Edit(./**)", "Write(./**)"}
	for _, cmd := range append(append([]string(nil), commands...), gitAllowed...) {
		if role == roleGuard && (cmd == "git add" || cmd == "git commit") {
			continue // a guard reviews and never commits
		}
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
func workerEnv(s ContextSettings, id GitIdentity) []string {
	var env []string
	for _, k := range envAllowed {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	env = append(env, envFixed...)
	if id != (GitIdentity{}) {
		env = append(env, "GIT_AUTHOR_NAME="+id.Name, "GIT_AUTHOR_EMAIL="+id.Email,
			"GIT_COMMITTER_NAME="+id.Name, "GIT_COMMITTER_EMAIL="+id.Email)
	}
	if s.ToolOutputChars > 0 {
		env = append(env, "BASH_MAX_OUTPUT_LENGTH="+strconv.Itoa(s.ToolOutputChars))
	}
	if n := thinkingTokens[s.Thinking]; n > 0 {
		env = append(env, "MAX_THINKING_TOKENS="+strconv.Itoa(n))
	}
	return env
}
