// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/forge"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/pace"
)

// setPace appends a pace: section to the hive's config and writes local.yaml.
func setPace(t *testing.T, dir, section, local string) {
	t.Helper()
	cfg := filepath.Join(dir, layout.Config)
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, append(data, []byte("\n"+section)...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, layout.Local), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixLoad(t *testing.T, r pace.Reading) {
	t.Helper()
	old := readLoad
	readLoad = func() (pace.Reading, error) { return r, nil }
	t.Cleanup(func() { readLoad = old })
}

func runPaceT(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = runPace(args, &out, &errb)
	checkPlain(t, "pace stdout", out.String())
	checkPlain(t, "pace stderr", errb.String())
	return code, out.String(), errb.String()
}

func TestStatusPaceLine(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := newHive(t)
	setPace(t, dir, "pace:\n  limit: steady\n", "keeper_id: paul\npace:\n  profile: max\n")
	code, out, errb := runStatusT(t, "--repo", dir)
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, errb)
	}
	line := "pace: steady (default steady, limit steady): 2 runs, 3 subagents, CPU target 70%, memory floor 2048 MiB\n"
	if strings.Count(out, "pace:") != 1 || !strings.Contains(out, "daemon: stopped (no .waggle/control/run.sock)\n"+line) {
		t.Errorf("stdout:\n%s", out)
	}
	if errb != "warning: pace: local.yaml picks max, above the owner's limit steady; using steady\n" {
		t.Errorf("stderr %q", errb)
	}
	checkPlain(t, "status pace line", line+errb)
}

func TestStatusPaceInvalid(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := newHive(t)
	setPace(t, dir, "pace:\n  speed: fast\n", "keeper_id: paul\n")
	code, out, _ := runStatusT(t, "--repo", dir)
	if code != 0 || !strings.Contains(out, "pace: invalid: ") || !strings.Contains(out, "pace.speed: unknown key") {
		t.Errorf("code %d, stdout:\n%s", code, out)
	}
}

func TestStatusNoPace(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := newHive(t)
	_, out, _ := runStatusT(t, "--repo", dir)
	if strings.Contains(out, "pace") {
		t.Errorf("pace line without a section:\n%s", out)
	}
}

func TestDoctorPace(t *testing.T) {
	fixLoad(t, pace.Reading{Load1: 2, CPUs: 8, AvailMiB: 9312})
	want := "pace: steady; load 2.0 on 8 CPUs (25%), under the 70% target; available memory 9.1 GiB, at or above the 2048 MiB floor\n"

	// Local-only repo.
	dir := doctorRepo(t, "  provider: github\n")
	setPace(t, dir, "pace:\n  profile: steady\n", "")
	seamForge(t, &fakeForge{}, okClaims)
	code, out, _ := runDoc("--repo", dir)
	if code != 0 || !strings.HasPrefix(out, want) || !strings.Contains(out, "Local-only repo") {
		t.Errorf("local-only: code %d\n%s", code, out)
	}

	// Forge repo: the finding is never required, so the exit code is the forge's.
	dir = doctorRepo(t, githubCfg)
	setPace(t, dir, "pace:\n  profile: steady\n", "pace:\n  profile: serial\n")
	seamForge(t, &fakeForge{report: report("up_to_date", okGate, okUpTo)}, okClaims)
	code, out, _ = runDoc("--repo", dir)
	if code != 0 || strings.Count(out, "pace:") != 1 || !strings.Contains(out, "pace: serial; load 2.0 on 8 CPUs (25%), under the 50% target") {
		t.Errorf("forge: code %d\n%s", code, out)
	}
	seamForge(t, &fakeForge{report: report("up_to_date", okGate, badUpTo)}, okClaims)
	if code, _, _ = runDoc("--repo", dir); code != 1 {
		t.Errorf("forge missing: code %d", code)
	}

	// Over the target and unknown memory still exit as before.
	fixLoad(t, pace.Reading{Load1: 8, CPUs: 8, AvailMiB: -1, MemoryNote: "not measured on macOS"})
	seamForge(t, &fakeForge{report: report("up_to_date", okGate, okUpTo)}, okClaims)
	code, out, _ = runDoc("--repo", dir)
	if code != 0 || !strings.Contains(out, "at or over the 50% target; available memory not measured on macOS") {
		t.Errorf("over: code %d\n%s", code, out)
	}
	checkPlain(t, "doctor", out)
}

func TestDoctorPaceInvalid(t *testing.T) {
	fixLoad(t, pace.Reading{})
	dir := doctorRepo(t, "  provider: github\n  remote: origin\n")
	setPace(t, dir, "pace:\n  limit: turbo\n", "")
	seamForge(t, &fakeForge{report: report("up_to_date", okGate, okUpTo)}, forge.Finding{ID: "claim-refs", Status: forge.StatusOK, Required: true, Summary: "ok"})
	code, out, _ := runDoc("--repo", dir)
	if code != 0 || !strings.Contains(out, "pace: invalid: ") {
		t.Errorf("code %d\n%s", code, out)
	}
}

func TestDoctorNoPace(t *testing.T) {
	fixLoad(t, pace.Reading{})
	dir := doctorRepo(t, "  provider: github\n")
	seamForge(t, &fakeForge{}, okClaims)
	_, out, _ := runDoc("--repo", dir)
	if strings.Contains(out, "pace") {
		t.Errorf("pace finding without a section:\n%s", out)
	}
}

// snapshot lists every file under dir with its size and mtime.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out[p] = info.Mode().String() + info.ModTime().Format(time.RFC3339Nano) + string(rune(info.Size()))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPaceCommand(t *testing.T) {
	requireGit(t)
	dir := newHive(t)
	setPace(t, dir, "pace:\n  profile: serial\n", "keeper_id: paul\n")
	before := snapshot(t, dir)

	code, out, errb := runPaceT(t, "--repo", dir, "--tool", "claude-code")
	if code != 0 || errb != "" {
		t.Fatalf("code %d, stderr %q", code, errb)
	}
	if !strings.HasPrefix(out, "pace: serial (default serial, limit max): 1 runs, 1 subagents, CPU target 50%, memory floor 4096 MiB\n") {
		t.Errorf("first line:\n%s", out)
	}
	var env map[string]string
	if err := json.Unmarshal([]byte(out[strings.Index(out, "{"):strings.Index(out, "}")+1]), &env); err != nil {
		t.Fatalf("env object: %v\n%s", err, out)
	}
	if env["CLAUDE_CODE_DISABLE_BACKGROUND_TASKS"] != "1" || env["CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"] != "0" || len(env) != 4 {
		t.Errorf("serial env %v", env)
	}

	code, out, _ = runPaceT(t, "--repo", dir)
	if code != 0 || !strings.Contains(out, "scheduler.max_polecats 1\n") || !strings.Contains(out, "Copilot cloud agent: advice only") ||
		!strings.Contains(out, "Claude Code:") {
		t.Errorf("all tools: code %d\n%s", code, out)
	}

	after := snapshot(t, dir)
	if len(before) != len(after) {
		t.Errorf("file count changed: %d to %d", len(before), len(after))
	}
	for p, v := range before {
		if after[p] != v {
			t.Errorf("%s changed", p)
		}
	}
}

func TestPaceCommandErrors(t *testing.T) {
	requireGit(t)
	dir := newHive(t)
	if code, out, _ := runPaceT(t, "--repo", dir); code != 0 || !strings.Contains(out, "No pace is set") {
		t.Errorf("no section: code %d\n%s", code, out)
	}
	if code, _, _ := runPaceT(t, "--repo", dir, "--tool", "emacs"); code != 2 {
		t.Errorf("unknown tool: code %d", code)
	}
	if code, _, _ := runPaceT(t, "--bogus"); code != 2 {
		t.Errorf("bad flag: code %d", code)
	}
	if code, _, _ := runPaceT(t, "--repo", dir, "extra"); code != 2 {
		t.Errorf("extra arg: code %d", code)
	}
	setPace(t, dir, "pace:\n  speed: fast\n", "")
	if code, _, errb := runPaceT(t, "--repo", dir); code != 1 || !strings.Contains(errb, "pace: invalid") {
		t.Errorf("invalid: code %d, stderr %q", code, errb)
	}
}

func TestPaceClampWarning(t *testing.T) {
	requireGit(t)
	dir := newHive(t)
	setPace(t, dir, "pace:\n  limit: serial\n  profile: serial\n", "pace:\n  profile: max\n")
	code, out, errb := runPaceT(t, "--repo", dir, "--tool", "gas-town")
	if code != 0 || !strings.Contains(errb, "picks max, above the owner's limit serial") || !strings.Contains(out, "max_polecats 1") {
		t.Errorf("code %d, stdout %q, stderr %q", code, out, errb)
	}
}

func TestUsageHasPace(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	printUsage()
	w.Close()
	os.Stdout = old
	var b bytes.Buffer
	b.ReadFrom(r)
	checkPlain(t, "usage", b.String(), "  pace      Show the agent pace")
}

// A semantic pace error leaves a gate run unchanged.
func TestGateIgnoresPaceSection(t *testing.T) {
	run := func(cfg string) (int, string) {
		r := newGateRepo(t)
		r.write("README.md", []byte("hive\n"))
		r.write(layout.Config, []byte(cfg))
		r.write(layout.Decisions, gateLog(1))
		r.commit("init")
		r.git("checkout", "-q", "-b", "branch")
		r.claim(1, "full", active, "docs")
		r.write("docs/a.md", []byte("a\n"))
		r.commit("T-001: add docs")
		return r.run("verify", false)
	}
	baseCode, baseOut := run("version: 1\n")
	for _, bad := range []string{"pace:\n  speed: fast\n", "pace:\n  profile: max\n  limit: serial\n", "pace:\n  max:\n    subagents: 9999\n"} {
		code, out := run("version: 1\n" + bad)
		if code != baseCode || out != baseOut {
			t.Errorf("%q changed the gate: exit %d\n%s\nwant exit %d\n%s", bad, code, out, baseCode, baseOut)
		}
	}
}
