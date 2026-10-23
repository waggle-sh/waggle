package scaffold

import (
	"strings"
	"testing"
)

// T-033: the guard writes no file in the repo, and the gate workflow keeps
// read-only permissions outside the relay job.
func TestGuardTemplateWritesNothing(t *testing.T) {
	dir := t.TempDir()
	o, _ := opts(dir, "claude-code")
	mustInit(t, o)
	guard := get(t, dir, ".waggle/control/roles/guard.md")
	for _, s := range []string{".waggle/hive/reviews/", ".waggle/hive/media/"} {
		if strings.Contains(guard, s) {
			t.Errorf("guard.md contains %q", s)
		}
	}
	if !strings.Contains(guard, "final message") {
		t.Error("guard.md does not name the final message as the review")
	}
	roster := get(t, dir, ".waggle/control/roster.json")
	i := strings.Index(roster, `"id": "guard"`)
	if i < 0 || !strings.Contains(roster[i:], `"owns": []`) {
		t.Error(`roster guard entry lacks "owns": []`)
	}
	for rel, content := range snapshot(t, dir) {
		if strings.Contains(content, "hive/reviews/T-<id>.md") && strings.Contains(rel, ".waggle/control/") {
			t.Errorf("%s names a review file path", rel)
		}
	}
}

func TestGateTemplateBaseBuilt(t *testing.T) {
	dir := t.TempDir()
	o, _ := opts(dir, "claude-code")
	mustInit(t, o)
	gate := get(t, dir, ".github/workflows/waggle-gate.yml")
	if !strings.HasPrefix(gate[strings.Index(gate, "\npermissions:"):], "\npermissions:\n  contents: read\n\njobs:") {
		t.Error("workflow-level permissions are not contents: read only")
	}
	relay := strings.Index(gate, "  relay-claim:")
	verify := strings.Index(gate, "  verify-leases-and-tiers:")
	for _, w := range []string{"contents: write", "pull-requests: write", "issues: write"} {
		if n := strings.Count(gate, w); n != 1 || strings.Index(gate, w) < relay || strings.Index(gate, w) > verify {
			t.Errorf("%q must appear once, under relay-claim", w)
		}
	}
	if !strings.Contains(gate[verify:], "    permissions:\n      contents: read\n") {
		t.Error("verify job lacks job-level contents: read")
	}
	if strings.Contains(gate, "|| true") {
		t.Error("claim fetch swallows its exit")
	}
	for _, s := range []string{
		`gate verify --base "$BASE_SHA" --head "$HEAD_SHA"`,
		`$BASE_SHA:cmd/waggle/gate.go`,
		"id: src",
		"        env:\n          BASE_SHA: ${{ github.event.pull_request.base.sha }}\n        run: |\n          if ! git cat-file",
	} {
		if !strings.Contains(gate, s) {
			t.Errorf("gate lacks %q", s)
		}
	}
	if n := strings.Count(gate, "if: steps.src.outputs.present == 'true'"); n != 2 {
		t.Errorf("present guard on %d steps, want 2 (setup-go, gate)", n)
	}
	for _, s := range []string{"gate decisions", "pull_request_target", "github.event.pull_request.head.ref"} {
		if strings.Contains(gate, s) {
			t.Errorf("gate contains %q", s)
		}
	}
}
