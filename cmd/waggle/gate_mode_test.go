// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
)

const (
	openCfg   = "version: 1\ngate:\n  mode: open\n"
	guardCfg  = "version: 1\ngate:\n  mode: guard\n"
	openNote  = "gate: mode open: findings are reported, not enforced"
	wouldFail = "would fail (mode open)"
)

// outsideLease adds a commit that touches a path no claim leases.
func outsideLease(r gateRepo) {
	r.write("other/y.md", []byte("y\n"))
	r.commit("T-031: outside")
}

func TestModeGuardBytesUnchanged(t *testing.T) {
	var outs []string
	for _, cfg := range []string{"", "version: 1\n", "version: 1\ngate:\n  approvers: \"alice\"\n", guardCfg} {
		r := protRepo(t, cfg)
		outsideLease(r)
		code, out := r.runWith(nil, "verify")
		if code != 1 || !strings.Contains(out, "leases: FAIL: other/y.md") || strings.Contains(out, wouldFail) {
			t.Errorf("cfg %q: exit %d\n%s", cfg, code, out)
		}
		if strings.Contains(cfg, "approvers") {
			continue
		}
		outs = append(outs, strings.ReplaceAll(out, r.dir, "DIR"))
	}
	for _, o := range outs[1:] {
		if o != outs[0] {
			t.Errorf("guard output differs:\n%s\n---\n%s", outs[0], o)
		}
	}
}

func TestModeOpenLeaseFinding(t *testing.T) {
	r := protRepo(t, openCfg)
	outsideLease(r)
	code, out := r.runWith(nil, "verify")
	if code != 0 || !strings.Contains(out, openNote) || !strings.Contains(out, "leases: would fail (mode open): other/y.md") || strings.Contains(out, "FAIL") {
		t.Errorf("exit %d\n%s", code, out)
	}
	code, out = r.runWith(map[string]string{"GITHUB_ACTIONS": "true"}, "verify")
	if code != 0 || !strings.Contains(out, "::notice::"+openNote) || !strings.Contains(out, "::warning file=other/y.md::leases: would fail (mode open)") || strings.Contains(out, "::error") {
		t.Errorf("gha exit %d\n%s", code, out)
	}
}

func TestModeOpenOtherChecks(t *testing.T) {
	r := protRepo(t, openCfg)
	r.write(".github/workflows/x.yml", []byte("x\n"))
	r.commit("T-031: ci")
	// Built in the index so a case-insensitive filesystem can hold both names.
	r.index("docs/a.md")
	r.index("docs/A.md")
	r.git("commit", "-q", "-m", "T-031: case")
	// Staged by path, as add -A would drop the index-only case pair.
	r.write(layout.Decisions, []byte("not a log\n"))
	r.git("add", layout.Decisions)
	r.git("commit", "-q", "-m", "T-031: log")
	code, out := r.runWith(nil, "verify")
	if code != 0 || strings.Contains(out, "FAIL") {
		t.Errorf("exit %d\n%s", code, out)
	}
	for _, want := range []string{
		"tiers: would fail (mode open): .github/workflows/x.yml",
		"tiers: would fail (mode open): ",
		"decisions: would fail (mode open): ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q\n%s", want, out)
		}
	}
	if n := strings.Count(out, "tiers: would fail"); n < 2 {
		t.Errorf("want a Tier 3 and a case finding, got %d\n%s", n, out)
	}
	code, out = r.runWith(map[string]string{"GITHUB_ACTIONS": "true"}, "verify")
	if code != 0 || !strings.Contains(out, "::warning file="+layout.Decisions+"::decisions: would fail") || strings.Contains(out, "::error") {
		t.Errorf("gha exit %d\n%s", code, out)
	}
}

func TestModeOpenCleanPR(t *testing.T) {
	r := protRepo(t, openCfg)
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-031: add")
	code, out := r.runWith(nil, "verify")
	if code != 0 || !strings.Contains(out, openNote) {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for _, want := range []string{"leases: ok", "tiers: ok", "decisions: unchanged"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q\n%s", want, out)
		}
	}
}

func TestModeComesFromBase(t *testing.T) {
	// (a) base open, head sets guard and has a violation: passes.
	r := protRepo(t, openCfg)
	r.write(layout.Config, []byte(guardCfg))
	r.commit("T-031: config")
	if code, out := r.runWith(nil, "verify"); code != 0 {
		t.Errorf("base open: exit %d\n%s", code, out)
	}
	// (b) base guard, head sets open and has a violation: fails on the config.
	r = protRepo(t, guardCfg)
	r.write(layout.Config, []byte(openCfg))
	r.commit("T-031: config")
	code, out := r.runWith(nil, "verify")
	if code != 1 || !strings.Contains(out, layout.Config) || strings.Contains(out, wouldFail) {
		t.Errorf("base guard: exit %d\n%s", code, out)
	}
}

func TestModeOpenExit3Stays(t *testing.T) {
	for name, cfg := range map[string]string{
		"watch":       "version: 1\ngate:\n  mode: watch\n",
		"empty":       "version: 1\ngate:\n  mode: \"\"\n",
		"unknown key": "version: 1\ngate:\n  mode: open\n  x: y\n",
		"bad version": "version: 2\ngate:\n  mode: open\n",
	} {
		r := protRepo(t, cfg)
		r.write("docs/a.md", []byte("a\n"))
		r.commit("T-031: add")
		if code, out := r.runWith(nil, "verify"); code != 3 || strings.Contains(out, openNote) {
			t.Errorf("%s: exit %d\n%s", name, code, out)
		}
	}
	r := protRepo(t, openCfg)
	outsideLease(r)
	if code, out := r.runWith(nil, "verify", "--base", "no-such-rev"); code != 3 {
		t.Errorf("bad base: exit %d\n%s", code, out)
	}
	r.git("checkout", "-q", "--orphan", "lonely")
	r.write("z.md", []byte("z\n"))
	r.commit("T-031: z")
	if code, out := r.runWith(nil, "verify", "--head", "lonely"); code != 3 {
		t.Errorf("no merge-base: exit %d\n%s", code, out)
	}
}

func TestModeHasNoOverride(t *testing.T) {
	r := protRepo(t, guardCfg)
	outsideLease(r)
	if code, out := r.runWith(nil, "verify", "--mode", "open"); code != 2 {
		t.Errorf("--mode: exit %d\n%s", code, out)
	}
	env := map[string]string{"WAGGLE_GATE_MODE": "open", "GATE_MODE": "open", "GATE_MODE_OPEN": "1"}
	if code, out := r.runWith(env, "verify"); code != 1 || strings.Contains(out, wouldFail) {
		t.Errorf("env: exit %d\n%s", code, out)
	}
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-031: add\n\nWaggle-Mode: open")
	if code, out := r.runWith(nil, "verify"); code != 1 || strings.Contains(out, wouldFail) {
		t.Errorf("trailer: exit %d\n%s", code, out)
	}
}

func TestModeOpenTooLargeStillFails(t *testing.T) {
	r := protRepo(t, openCfg)
	var in strings.Builder
	for i := 1; i <= maxGateCommits+1; i++ {
		msg := "T-031: step\n"
		fmt.Fprintf(&in, "commit refs/heads/branch\ncommitter t <t@example.com> 1760659200 +0000\ndata %d\n%s", len(msg), msg)
		if i == 1 {
			in.WriteString("from refs/heads/main\n")
		}
		fmt.Fprintf(&in, "M 100644 inline docs/a.md\ndata %d\n%d\n\n", len(fmt.Sprint(i))+1, i)
	}
	r.gitIn(in.String(), "fast-import", "--quiet", "--force")
	code, out := r.runWith(nil, "verify")
	if code != 1 || !strings.Contains(out, "leases: FAIL: PR too large for the gate; split it") || strings.Contains(out, wouldFail) {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestModeOpenHostileText(t *testing.T) {
	r := protRepo(t, openCfg)
	r.write("other/a%\n::error::z:,b.md", []byte("a\n"))
	r.commit("T-031: hostile")
	for _, gha := range []bool{true, false} {
		env := map[string]string{}
		if gha {
			env["GITHUB_ACTIONS"] = "true"
		}
		code, out := r.runWith(env, "verify")
		if code != 0 || strings.Contains(out, "\r") {
			t.Errorf("gha %v: exit %d %q", gha, code, out)
		}
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "::error") || strings.HasPrefix(line, "z") {
				t.Errorf("gha %v: injected line %q", gha, line)
			}
		}
	}
}

func TestModeSingleChecksStayStrict(t *testing.T) {
	r := protRepo(t, openCfg)
	outsideLease(r)
	r.write(".github/workflows/x.yml", []byte("x\n"))
	r.write(layout.Decisions, []byte("not a log\n"))
	r.commit("T-031: more")
	for _, check := range []string{"leases", "tiers"} {
		if code, out := r.runWith(nil, check); code != 1 || strings.Contains(out, wouldFail) {
			t.Errorf("%s: exit %d\n%s", check, code, out)
		}
	}
	code, _, errs := gate(nil, "decisions", "--base", "main", "--head", "branch", "--repo", r.dir)
	if code != 1 || strings.Contains(errs, wouldFail) {
		t.Errorf("decisions: exit %d\n%s", code, errs)
	}
}

func TestModeHelpPlainTerms(t *testing.T) {
	var out, errb bytes.Buffer
	runGate([]string{"-h"}, &out, &errb, func(string) string { return "" })
	checkPlain(t, "gate -h", errb.String(), "verify reads gate.mode from the base config.", "open prints each finding as a warning and exits 0.")
}

func TestModeOpenApproverPathUnchanged(t *testing.T) {
	cfg := "version: 1\ngate:\n  protected_paths: \"deploy/\"\n  approvers: \"alice\"\n"
	warnings := func(out string) []string {
		var ws []string
		for _, l := range strings.Split(out, "\n") {
			if strings.Contains(l, "warning") && !strings.Contains(l, wouldFail) {
				ws = append(ws, l)
			}
		}
		return ws
	}
	var got [2][]string
	for i, c := range []string{cfg, cfg + "  mode: open\n"} {
		r := protRepo(t, c)
		r.write("deploy/a.yml", []byte("a\n"))
		r.commit("T-031: deploy")
		code, out := r.runWith(map[string]string{"GITHUB_ACTIONS": "true"}, "verify")
		if (i == 1) != (code == 0) {
			t.Errorf("cfg %d: exit %d\n%s", i, code, out)
		}
		got[i] = warnings(out)
	}
	if strings.Join(got[0], "\n") != strings.Join(got[1], "\n") {
		t.Errorf("warnings differ:\n%q\n%q", got[0], got[1])
	}
}
