package runtime

import (
	"context"
	"io"
)

// AdapterConfig holds model- and runtime-specific thresholds loaded from .waggle/control/config.yaml.
// Keeping token limits in config rather than hardcoded in the binary allows operators to
// tune thresholds as model context windows evolve.
type AdapterConfig struct {
	Binary      string   `json:"binary" yaml:"binary"`
	DefaultArgs []string `json:"default_args,omitempty" yaml:"default_args,omitempty"`
	NudgeTokens int      `json:"nudge_tokens" yaml:"nudge_tokens"`
	MaxTokens   int      `json:"max_tokens" yaml:"max_tokens"`
}

// Invocation defines a single isolated agent execution inside a Cell or main checkout.
type Invocation struct {
	Role        string
	TaskID      string
	WorktreeDir string
	PromptFile  string
	BriefFile   string
	SocketPath  string    // Path to waggled's .waggle/control/run.sock
	Selection   Selection // Resolved model for Role; Prepare refuses a zero value
	Commands    []string  // build, test, vet and format commands from WAGGLE.md section 3, allowed in Bash
}

// UsageStats reports token consumption back to waggled so the daemon can enforce
// NudgeTokens and MaxTokens boundaries between turns. TotalTokens is the sum
// of the other four.
type UsageStats struct {
	InputTokens      int
	OutputTokens     int
	TotalTokens      int
	CacheWriteTokens int
	CacheReadTokens  int
}

// AgentRuntime abstracts the underlying CLI coding agent (Claude Code in v0.1;
// Aider, Codex, Gemini CLI in v0.2+).
type AgentRuntime interface {
	Name() string
	Config() AdapterConfig
	// Materialize generates gitignored vendor-specific files (e.g., .claude/agents/waggle-*.md)
	// from the canonical .waggle/control/roles/*.md sources. It never writes CLAUDE.md.
	Materialize(repoDir string) error
	// Verify checks the installed runtime supports every security flag
	// Prepare emits; Prepare fails closed until it has succeeded.
	Verify(ctx context.Context) error
	// Prepare builds the argv, environment and stdin for one fresh session.
	// The caller (the Cell seam) owns the process.
	Prepare(inv Invocation, s ContextSettings, stable, variable []byte) (LaunchSpec, error)
	// Collect reads the session's output stream until EOF, calling stop at
	// most once when a limit is hit or ctx ends.
	Collect(ctx context.Context, r io.Reader, lim Limits, stop func()) (RunResult, error)
}
