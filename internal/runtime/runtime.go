package runtime

import "context"

// AdapterConfig holds model- and runtime-specific thresholds loaded from .waggle/config.yaml.
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
	SocketPath  string // Path to waggled's .waggle/run.sock
}

// UsageStats reports token consumption back to waggled so the daemon can enforce
// NudgeTokens and MaxTokens boundaries between turns.
type UsageStats struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// AgentRuntime abstracts the underlying CLI coding agent (Claude Code in v0.1;
// Aider, Codex, Gemini CLI in v0.2+).
type AgentRuntime interface {
	Name() string
	Config() AdapterConfig
	// Materialize generates gitignored vendor-specific files (e.g., .claude/agents/*.md,
	// CLAUDE.md) from the canonical .waggle/roles/*.md and WAGGLE.md sources of truth.
	Materialize(repoDir string) error
	Run(ctx context.Context, inv Invocation) (UsageStats, error)
}