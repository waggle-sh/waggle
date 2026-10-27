// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/decisions"
	"github.com/waggle-sh/waggle/internal/layout"
)

const decisionStart = "--- waggle decide input (save as upgrade-decision.json) ---\n"

// decisionBlock returns the JSON body between the markers and the line after the end marker.
func decisionBlock(t *testing.T, out string) (body, after string) {
	t.Helper()
	_, rest, ok := strings.Cut(out, decisionStart)
	body, tail, ok2 := strings.Cut(rest, "--- end ---\n")
	if !ok || !ok2 {
		t.Fatalf("decision block missing:\n%s", out)
	}
	after, _, _ = strings.Cut(tail, "\n")
	return body, after
}

func TestUpgradeDecisionInputGateOnly(t *testing.T) {
	dir := gateOnlyRepo(t, pinA)
	o, _ := upgradeOpts(dir, "none", commitBuild(pinB), false)
	out := upgrade(t, o)
	body, after := decisionBlock(t, out)

	in, err := decisions.DecodeInput(strings.NewReader(body))
	if err != nil {
		t.Fatalf("decode: %v\n%s", err, body)
	}
	if in.Title != "Upgrade the Waggle pin to "+pinB || in.Question != "Which Waggle build does this repo's gate run?" ||
		in.Rationale != pinB+" is the build the maintainers chose for this repo." || len(in.Tickets) != 0 {
		t.Errorf("input = %+v", in)
	}
	for _, bad := range []string{"T-0", "D-0", "Keeper", "hive", "queen"} {
		if strings.Contains(body, bad) {
			t.Errorf("gate-only block holds %q:\n%s", bad, body)
		}
	}
	if _, err := decisions.Next(filepath.Join(dir, layout.Decisions), in, time.Now()); err != nil {
		t.Errorf("decisions.Next refused the gate-only input: %v", err)
	}
	if want := "Run: waggle decide --file upgrade-decision.json and commit the entry in the same PR."; after != want {
		t.Errorf("run line = %q, want %q", after, want)
	}
}

func TestUpgradeDecisionBlockFullHiveGolden(t *testing.T) {
	dir := hive(t, "none", commitBuild(pinA), nil)
	o, _ := upgradeOpts(dir, "none", commitBuild(pinB), false)
	body, after := decisionBlock(t, upgrade(t, o))
	want := `{
  "kind": "technical",
  "title": "Upgrade hive pin to ` + pinB + `",
  "question": "Which Waggle build does this hive run?",
  "options": [
    "Stay on ` + pinA + `",
    "Move to ` + pinB + `"
  ],
  "recommendation": "Move to ` + pinB + `",
  "choice": "Move the pin from ` + pinA + ` to ` + pinB + `; managed files re-rendered by waggle upgrade from the target build",
  "rationale": "` + pinB + ` is the known-good build chosen by the Head Keeper (D-041).",
  "tickets": [
    "T-016"
  ],
  "supersedes": []
}
`
	if body != want {
		t.Errorf("full-hive block changed:\n%s\nwant:\n%s", body, want)
	}
	if wantRun := "Run: waggle decide --file upgrade-decision.json (same hive PR, Head Keeper or queen after an explicit Keeper answer, T-006 section 8)."; after != wantRun {
		t.Errorf("run line = %q", after)
	}
}

func TestLastLinesPlainTerms(t *testing.T) {
	for name, dir := range map[string]string{"hive": hive(t, "none", commitBuild(pinA), nil), "gate-only": gateOnlyRepo(t, pinA)} {
		o, _ := upgradeOpts(dir, "none", commitBuild(pinB), false)
		if out := upgrade(t, o); !strings.HasSuffix(out, "Review with git status, then open a PR. This changes .waggle/control/ (Tier 3), so only an admin bypass merges it.\n") {
			t.Errorf("%s: upgrade last line:\n%s", name, out)
		}
		if uninstallLastLine != "Review with git status, then commit. Removing .waggle/control/ is Tier 3, so only an admin bypass merges it." {
			t.Errorf("uninstallLastLine = %q", uninstallLastLine)
		}
	}
}

func TestGateTemplatesPlainTerms(t *testing.T) {
	dir := newRepo(t)
	mustInit(t, gateOpts(dir, commitBuild(pinA)))
	wf := get(t, dir, gateWorkflowRel)
	if !strings.Contains(wf, "      - name: Fetch claim refs\n        run: git fetch origin 'refs/waggle/claims/*:refs/waggle/claims/*'\n") || strings.Contains(wf, "Comb") {
		t.Errorf("workflow step name:\n%s", wf)
	}
	cfg := get(t, dir, layout.Config)
	for _, want := range []string{"and a full install adds more keys later.", "change only with a listed approver's approval or an admin bypass."} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config lacks %q:\n%s", want, cfg)
		}
	}
	for _, bad := range []string{"hive", "Keeper"} {
		if strings.Contains(cfg, bad) {
			t.Errorf("config holds %q:\n%s", bad, cfg)
		}
	}
}
