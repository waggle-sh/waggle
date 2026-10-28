// SPDX-License-Identifier: Apache-2.0

package pace

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Tools are the agent tools waggle pace can render for.
var Tools = []string{"claude-code", "copilot", "gas-town"}

// Render returns the text that applies s in one agent tool. It is pure: it
// writes no file and runs no program. goos is runtime.GOOS of this machine.
func Render(tool string, s Settings, goos string) (string, error) {
	switch tool {
	case "claude-code":
		return renderClaude(s, goos), nil
	case "copilot":
		return "Copilot cloud agent: advice only.\n" +
			"It runs on GitHub's runners, not on this machine, and GitHub documents no setting that limits how many Copilot cloud agent sessions run at once.\n" +
			fmt.Sprintf("Pace is how many issues you assign at once: %d.\n", s.ParallelRuns), nil
	case "gas-town":
		return "Gas Town:\n" +
			fmt.Sprintf("  gt config set scheduler.max_polecats %d\n", s.ParallelRuns) +
			"Waggle prints this command and never runs it.\n" +
			"The CPU target and the memory floor are advice only: Gas Town's scheduler counts polecats and uses no CPU or memory reading.\n", nil
	}
	return "", fmt.Errorf("unknown tool %q; use %s", Printable(tool), strings.Join(Tools, ", "))
}

// ClaudeEnv is the env object for .claude/settings.local.json: plain digits
// and "0" or "1" only.
func ClaudeEnv(s Settings) map[string]string {
	env := map[string]string{
		"CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS":       strconv.Itoa(s.Subagents),
		"CLAUDE_CODE_WORKFLOW_MAX_CONCURRENT_AGENTS": strconv.Itoa(min(s.Subagents, 256)),
	}
	if s.Profile == Serial {
		env["CLAUDE_CODE_DISABLE_BACKGROUND_TASKS"] = "1"
		env["CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"] = "0"
	}
	return env
}

func renderClaude(s Settings, goos string) string {
	env := ClaudeEnv(s)
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("Claude Code: set this object as the value of \"env\" in .claude/settings.local.json:\n{\n")
	for i, k := range keys {
		sep := ","
		if i == len(keys)-1 {
			sep = ""
		}
		fmt.Fprintf(&b, "  %q: %q%s\n", k, env[k], sep)
	}
	b.WriteString("}\n")
	b.WriteString("CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS needs Claude Code v2.1.217 or later and CLAUDE_CODE_WORKFLOW_MAX_CONCURRENT_AGENTS needs v2.1.269 or later.\n")
	if s.Profile == Serial {
		b.WriteString("CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=0 keeps agent teams off; they are experimental and off by default.\n")
	}
	b.WriteString(".claude/settings.local.json overrides .claude/settings.json and is overridden by managed settings and --settings.\n")
	b.WriteString("Claude Code writes that file itself when you choose \"Yes, and don't ask again\", so edit it by hand and keep its other keys.\n")
	b.WriteString("env values in project and local settings may wait until you trust the folder.\n")
	fmt.Fprintf(&b, "Parallel sessions: run at most %d Claude Code sessions yourself. Claude Code has no setting for that, and agent teams have no hard limit on the number of teammates.\n", s.ParallelRuns)
	if goos == "linux" && s.MemoryFloorMiB > 0 {
		fmt.Fprintf(&b, "Memory floor: export CLAUDE_CODE_WARN_MEMORY_FREE_GIB=%d in the shell that starts claude (needs Claude Code v2.1.304 or later). Claude Code reads it from the launch environment only and ignores it in every settings file.\n",
			(s.MemoryFloorMiB+1023)/1024)
	}
	b.WriteString("CPU target: advice only. Claude Code documents no CPU limit setting; the CPU target is advice for how many sessions you start.\n")
	return b.String()
}
