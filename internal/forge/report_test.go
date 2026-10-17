package forge

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

// githubFindings mirrors the brief's example report, section 7.1.
func githubFindings(gateOK bool) []Finding {
	gate := Finding{ID: "required-gate", Status: StatusMissing, Required: true,
		Summary:     "No rule on main requires the verify-leases-and-tiers check.",
		NotEnforced: "a pull request can merge with a failing or missing Waggle gate.",
		Fix:         "add a branch ruleset on main that requires verify-leases-and-tiers."}
	bypass := Finding{ID: "bypass", Status: StatusUnknown,
		Summary:     "Ruleset bypass list is not readable with this login.",
		NotEnforced: "cannot confirm only the Head Keeper can merge Tier 3 changes.",
		Fix:         "rerun waggle doctor as a repo admin."}
	if gateOK {
		gate = Finding{ID: "required-gate", Status: StatusOK, Required: true,
			Summary: "A rule on main requires the verify-leases-and-tiers check."}
		bypass = Finding{ID: "bypass", Status: StatusOK, Summary: "Only the Head Keeper can bypass rulesets."}
	}
	return []Finding{
		{ID: "custom-refs", Status: StatusOK, Required: true, Summary: "refs/waggle/* can be pushed and deleted."},
		{ID: "atomic-push", Status: StatusOK, Required: true, Summary: "The remote accepts atomic multi-ref pushes."},
		gate,
		{ID: "up-to-date", Status: StatusOK, Required: true, Summary: "Branches must be up to date before merging."},
		bypass,
		{ID: "merge-queue", Status: StatusSkipped, Summary: "Not used: merge_strategy is up_to_date."},
	}
}

func golden(t *testing.T, name string, r Report) string {
	t.Helper()
	var b bytes.Buffer
	if err := Render(&b, r); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := b.String(); got != string(want) {
		t.Errorf("%s: rendered report differs from golden file:\n%s", name, got)
	}
	for _, line := range strings.Split(b.String(), "\n") {
		if len(line) >= 100 {
			t.Errorf("%s: line is %d columns: %q", name, len(line), line)
		}
		if strings.ContainsAny(line, "\u2013\u2014") {
			t.Errorf("%s: line contains a dash: %q", name, line)
		}
	}
	return b.String()
}

func TestRenderGithubAdvisory(t *testing.T) {
	out := golden(t, "github_advisory", Report{Provider: "github", Repo: "waggle-sh/waggle", Remote: "origin",
		Auth: "gh CLI as paulternate (admin)", Claims: "refs/waggle/claims/* on origin",
		MergeStrategy: "up_to_date", Findings: githubFindings(false)})
	for _, want := range []string{"Enforcement: advisory (required-gate is missing)",
		"\n  missing   required-gate   No rule on main", "\n                            Not enforced: a pull request",
		"\n                            Fix: add a branch ruleset", "\nNot enforced on this repo (2):\n",
		"\n  - A pull request can merge", "\n  - Cannot confirm only", "cannot stop the merges listed above.\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
}

func TestRenderGithubEnforced(t *testing.T) {
	out := golden(t, "github_enforced", Report{Provider: "github", Repo: "waggle-sh/waggle", Remote: "origin",
		Auth: "GITHUB_TOKEN as paulternate (admin)", Claims: "refs/waggle/claims/* on origin",
		MergeStrategy: "up_to_date", Findings: githubFindings(true)})
	if !strings.Contains(out, "Enforcement: enforced\n") || strings.Contains(out, "Not enforced") {
		t.Errorf("enforced report wrong:\n%s", out)
	}
}

func TestRenderNone(t *testing.T) {
	for name, remote := range map[string]string{"none_remote": "origin", "none_local": ""} {
		r, err := (&none{remote: remote}).Probe(context.Background(), ProbeOptions{MergeStrategy: "up_to_date"})
		if err != nil {
			t.Fatal(err)
		}
		out := golden(t, name, r)
		if !strings.Contains(out, "Enforcement: advisory (required-gate is not checked)") ||
			!strings.Contains(out, "\n  missing   code-host       No code host.\n") ||
			!strings.Contains(out, "Not enforced: nothing outside Waggle") {
			t.Errorf("%s: unexpected report:\n%s", name, out)
		}
	}
}

func TestRenderRefusesSilentFindings(t *testing.T) {
	for _, s := range []Status{StatusMissing, StatusUnknown} {
		err := Render(&bytes.Buffer{}, Report{Provider: "none", Findings: []Finding{{ID: "x", Status: s, Summary: "s", NotEnforced: "n"}}})
		if err == nil || !strings.Contains(err.Error(), "without Not enforced and Fix") {
			t.Errorf("%s without Fix: got %v", s, err)
		}
	}
}

func TestEnforcementNativeQueue(t *testing.T) {
	fs := githubFindings(true)
	r := Report{MergeStrategy: "native_queue", Findings: fs}
	if got := r.Enforcement(); got != "advisory (merge-queue is skipped)" {
		t.Errorf("native_queue without a queue: %q", got)
	}
	fs[5].Status = StatusOK
	if got := r.Enforcement(); got != "enforced" {
		t.Errorf("native_queue with a queue: %q", got)
	}
}
