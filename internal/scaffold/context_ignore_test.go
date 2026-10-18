package scaffold

import (
	"strings"
	"testing"
)

// T-021 test 12: init --force rewrites a pre-T-021 ignore block so that it
// lists the handoff folder and the usage ledger.
func TestInitForceIgnoresHandoffsAndUsage(t *testing.T) {
	dir := t.TempDir()
	o, _ := opts(dir, "none")
	mustInit(t, o)
	put(t, dir, ".gitignore", "node_modules/\n# --- waggle:start ---\n"+
		".waggle/control/local.yaml\n.waggle/control/run.sock\n.waggle/worktrees/\n.claude/agents/waggle-*.md\n"+
		"# --- waggle:end ---\n")
	o.Force = true
	mustInit(t, o)
	got := get(t, dir, ".gitignore")
	if got != "node_modules/\n"+wantGitignore {
		t.Fatalf(".gitignore = %q; want %q", got, "node_modules/\n"+wantGitignore)
	}
	for _, line := range []string{".waggle/control/handoffs/\n", ".waggle/control/usage.jsonl\n"} {
		if !strings.Contains(got, line) {
			t.Errorf(".gitignore lacks %q", line)
		}
	}
}
