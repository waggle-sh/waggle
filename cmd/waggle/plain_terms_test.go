// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/scaffold"
)

// bareWords are bee names no help text for a gate-only user may hold.
var bareWords = []string{"Keeper", "queen", "Cells", "waggled"}

func checkPlain(t *testing.T, name, text string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(text, w) {
			t.Errorf("%s lacks %q:\n%s", name, w, text)
		}
	}
	for _, bad := range bareWords {
		if strings.Contains(text, bad) {
			t.Errorf("%s holds %q:\n%s", name, bad, text)
		}
	}
	// "hive" may appear only inside the decisions log path.
	if strings.Contains(strings.ReplaceAll(text, ".waggle/hive/", ""), "hive") {
		t.Errorf("%s holds \"hive\":\n%s", name, text)
	}
	if strings.ContainsAny(text, dashes) {
		t.Errorf("%s holds an en or em dash", name)
	}
}

func TestHelpPlainTerms(t *testing.T) {
	var out, errb bytes.Buffer

	runGate([]string{"-h"}, &out, &errb, func(string) string { return "" })
	checkPlain(t, "gate -h", errb.String(), "an admin merge that bypasses the required check.")

	errb.Reset()
	runClaim([]string{"-h"}, &out, &errb)
	checkPlain(t, "claim -h", errb.String(), "claim owner, usually your GitHub login; required without .waggle/local.yaml, else defaults to its keeper_id", "-dirs")

	errb.Reset()
	runTaskRelease([]string{"-h"}, &out, &errb)
	checkPlain(t, "release -h", errb.String(), "claim owner, usually your GitHub login; required without .waggle/local.yaml, else defaults to its keeper_id", "-unmerged")

	errb.Reset()
	runDecide([]string{"-h"}, strings.NewReader(""), &out, &errb)
	checkPlain(t, "decide -h", errb.String(), "Who runs it: a maintainer, after the decision is made. Agents never run this", "Repository root; must contain a .waggle/ directory")

	errb.Reset()
	runUpgrade([]string{"-h"}, &out, &errb)
	checkPlain(t, "upgrade -h", errb.String(), "Permit a release tag lower than the current pin", "Local agent runtime whose shims to regenerate")
}

func TestPrintUsagePlainTerms(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	printUsage()
	w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	text := string(b)
	for _, want := range []string{"waggle - task claims and a merge gate for AI coding agents on Git", "Active claims, daemon state and per-ticket token use"} {
		if !strings.Contains(text, want) {
			t.Errorf("usage lacks %q:\n%s", want, text)
		}
	}
}

func TestStatusGateOnly(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := filepath.Join(t.TempDir(), "gateonly")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-q")
	err := scaffold.InitRepo(scaffold.InitOptions{
		TargetDir: dir, Profile: "cli", Delivery: "installed", Forge: "github", CombMode: "direct",
		Runtime: "none", GateOnly: true, Out: &bytes.Buffer{},
		BuildInfo: func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: strings.Repeat("a", 40)}, {Key: "vcs.modified", Value: "false"},
			}}, true
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The gate-only config names remote origin; ListActiveLeases fetches from it (a fetch, not a push).
	bare := filepath.Join(t.TempDir(), "origin.git")
	git(t, filepath.Dir(bare), "init", "-q", "--bare", bare)
	git(t, dir, "remote", "add", "origin", bare)

	code, out, errb := runStatusT(t, "--repo", dir)
	if code != 0 {
		t.Fatalf("status code %d, stderr %q", code, errb)
	}
	lines := strings.Split(out, "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "Repo: gateonly  pin ") || !strings.HasSuffix(lines[0], "  claim refs direct (origin)") ||
		lines[1] != "daemon: stopped (no .waggle/control/run.sock)" {
		t.Errorf("first lines = %q", lines[:min(2, len(lines))])
	}

	plain := t.TempDir()
	git(t, plain, "init", "-q")
	if code, _, errb := runStatusT(t, "--repo", plain); code != 1 || !strings.Contains(errb, "refused: not a Waggle repo: .waggle/control/config.yaml not found; run waggle init") {
		t.Errorf("outside a Waggle repo: code %d, stderr %q", code, errb)
	}
}
