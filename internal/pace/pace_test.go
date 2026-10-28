// SPDX-License-Identifier: Apache-2.0

package pace

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func owner(profile, limit Profile) Pace {
	return Pace{Set: true, Profile: profile, Limit: limit, Table: Defaults()}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name      string
		owner     Pace
		pick      string
		want      Profile
		wantKinds []string
		notIn     string
	}{
		{"no pick", owner(Steady, Max), "", Steady, nil, ""},
		{"serial", owner(Steady, Max), "serial", Serial, nil, ""},
		{"steady", owner(Serial, Max), "steady", Steady, nil, ""},
		{"max at limit", owner(Steady, Max), "max", Max, nil, ""},
		{"at limit", owner(Serial, Steady), "steady", Steady, nil, ""},
		{"over limit", owner(Serial, Steady), "max", Steady, []string{KindClamped}, ""},
		{"unknown", owner(Steady, Max), "turbo", Steady, []string{KindUnknownPick}, ""},
		{"unknown with control", owner(Steady, Max), "tu\x1b[31mrbo\n", Steady, []string{KindUnknownPick}, "\x1b"},
		{"unknown with separator", owner(Steady, Max), "a b", Steady, []string{KindUnknownPick}, " "},
		{"case matters", owner(Steady, Max), "Max", Steady, []string{KindUnknownPick}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, warns := Resolve(tc.owner, tc.pick)
			if s.Profile != tc.want || s.ParallelRuns != tc.owner.Table[tc.want].ParallelRuns {
				t.Errorf("settings %+v, want profile %s", s, tc.want)
			}
			if s.Profile > tc.owner.Limit {
				t.Errorf("profile %s is faster than limit %s", s.Profile, tc.owner.Limit)
			}
			if len(warns) != len(tc.wantKinds) {
				t.Fatalf("warnings %v, want kinds %v", warns, tc.wantKinds)
			}
			for i, w := range warns {
				if w.Kind != tc.wantKinds[i] || !strings.HasPrefix(w.Message, "pace: ") {
					t.Errorf("warning %+v", w)
				}
				if tc.notIn != "" && strings.Contains(w.Message, tc.notIn) {
					t.Errorf("message holds %q: %q", tc.notIn, w.Message)
				}
				checkText(t, w.Message)
			}
		})
	}
}

func TestResolveMessages(t *testing.T) {
	_, w := Resolve(owner(Serial, Steady), "max")
	if want := "pace: local.yaml picks max, above the owner's limit steady; using steady"; w[0].Message != want {
		t.Errorf("clamp message %q", w[0].Message)
	}
	_, w = Resolve(owner(Steady, Max), "x")
	if want := "pace: local.yaml profile x is not serial, steady or max; using steady"; w[0].Message != want {
		t.Errorf("unknown message %q", w[0].Message)
	}
}

func TestResolveOverrides(t *testing.T) {
	p := owner(Steady, Max)
	p.Table[Max].ParallelRuns = 3
	s, _ := Resolve(p, "max")
	if s.ParallelRuns != 3 || s.Subagents != 8 {
		t.Errorf("settings %+v", s)
	}
}

func TestLine(t *testing.T) {
	p := owner(Steady, Max)
	s, _ := Resolve(p, "")
	want := "pace: steady (default steady, limit max): 2 runs, 3 subagents, CPU target 70%, memory floor 2048 MiB"
	if got := Line(p, s); got != want {
		t.Errorf("line %q", got)
	}
}

const meminfo = "MemTotal:       16384000 kB\nMemFree:          100000 kB\nMemAvailable:    9535488 kB\nBuffers: 1 kB\n"

func TestLinuxParsers(t *testing.T) {
	if v, ok := ParseLoadavg([]byte("1.80 1.20 0.90 2/512 12345\n")); !ok || v != 1.8 {
		t.Errorf("loadavg %v %v", v, ok)
	}
	for _, bad := range []string{"", "x 1 2", "-1 0 0", "NaN 0 0", "1e99 0 0"} {
		if _, ok := ParseLoadavg([]byte(bad)); ok {
			t.Errorf("loadavg %q parsed", bad)
		}
	}
	if v, ok := ParseMemAvailable([]byte(meminfo)); !ok || v != 9312 {
		t.Errorf("meminfo %v %v", v, ok)
	}
	for _, bad := range []string{"", "MemTotal: 1 kB\n", "MemAvailable: x kB\n", "MemAvailable: 5 MB\n", "MemAvailable: -5 kB\n", "MemAvailable:\n", "\x00\xff"} {
		if _, ok := ParseMemAvailable([]byte(bad)); ok {
			t.Errorf("meminfo %q parsed", bad)
		}
	}
	r := linuxReading([]byte("0.5 0 0 1/1 1"), []byte("MemTotal: 1 kB\n"))
	if r.Load1 != 0.5 || r.AvailMiB != -1 {
		t.Errorf("reading %+v", r)
	}
	r = linuxReading(nil, nil)
	if _, ok := r.Percent(); ok || r.AvailMiB != -1 {
		t.Errorf("empty reading %+v", r)
	}
}

// loadavgBytes is struct loadavg: ldavg 1.5 with fscale 2048, 24 bytes.
func loadavgBytes() []byte {
	b := make([]byte, 24)
	b[0], b[1] = 0x00, 0x0c // 3072 = 1.5 * 2048
	b[17] = 0x08            // fscale 2048
	return b
}

func TestDecodeLoadavg(t *testing.T) {
	full := loadavgBytes()
	a, okA := DecodeLoadavg(full)
	b, okB := DecodeLoadavg(full[:23])
	if !okA || !okB || a != 1.5 || a != b {
		t.Errorf("24 bytes %v %v, 23 bytes %v %v", a, okA, b, okB)
	}
	for _, n := range []int{0, 22, 25} {
		if _, ok := DecodeLoadavg(append(full, make([]byte, 8)...)[:n]); ok {
			t.Errorf("%d bytes decoded", n)
		}
	}
	zero := loadavgBytes()
	zero[17] = 0
	if _, ok := DecodeLoadavg(zero); ok {
		t.Error("fscale 0 decoded")
	}
}

func TestPercent(t *testing.T) {
	for _, tc := range []struct {
		load float64
		cpus int
		want float64
	}{{1, 1, 100}, {4, 8, 50}, {32, 64, 50}, {0, 8, 0}} {
		got, ok := Reading{Load1: tc.load, CPUs: tc.cpus}.Percent()
		if !ok || got != tc.want {
			t.Errorf("%v on %d: %v %v", tc.load, tc.cpus, got, ok)
		}
	}
	if _, ok := (Reading{Load1: -1, CPUs: 8}).Percent(); ok {
		t.Error("unknown load has a percent")
	}
	s := Defaults()[Steady]
	if !(Reading{Load1: 5.6, CPUs: 8}).OverCPU(s) || (Reading{Load1: 5.5, CPUs: 8}).OverCPU(s) {
		t.Error("CPU target edge")
	}
	if !(Reading{AvailMiB: 2047}).UnderMemory(s) || (Reading{AvailMiB: 2048}).UnderMemory(s) || (Reading{AvailMiB: -1}).UnderMemory(s) {
		t.Error("memory floor edge")
	}
}

func TestFinding(t *testing.T) {
	s := Defaults()[Steady]
	s.Profile = Steady
	got := Finding(s, Reading{Load1: 2, CPUs: 8, AvailMiB: 9312})
	want := "pace: steady; load 2.0 on 8 CPUs (25%), under the 70% target; available memory 9.1 GiB, at or above the 2048 MiB floor"
	if got != want {
		t.Errorf("finding %q", got)
	}
	got = Finding(s, Reading{Load1: 1.8, CPUs: 8, AvailMiB: -1, MemoryNote: "not measured on macOS"})
	if !strings.HasSuffix(got, "available memory not measured on macOS") {
		t.Errorf("finding %q", got)
	}
	got = Finding(s, Unknown())
	if !strings.Contains(got, "load unknown") || !strings.HasSuffix(got, "available memory unknown") {
		t.Errorf("finding %q", got)
	}
	checkText(t, got)
}

func TestRenderClaude(t *testing.T) {
	for _, prof := range []Profile{Serial, Steady, Max} {
		s, _ := Resolve(owner(prof, Max), "")
		text, err := Render("claude-code", s, "linux")
		if err != nil {
			t.Fatal(err)
		}
		checkText(t, text)
		start, end := strings.Index(text, "{"), strings.Index(text, "}")
		var env map[string]string
		if err := json.Unmarshal([]byte(text[start:end+1]), &env); err != nil {
			t.Fatalf("%s: %v\n%s", prof, err, text)
		}
		allowed := map[string]bool{
			"CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS": true, "CLAUDE_CODE_WORKFLOW_MAX_CONCURRENT_AGENTS": true,
			"CLAUDE_CODE_DISABLE_BACKGROUND_TASKS": true, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": true,
		}
		for k := range env {
			if !allowed[k] {
				t.Errorf("key %s", k)
			}
		}
		_, bg := env["CLAUDE_CODE_DISABLE_BACKGROUND_TASKS"]
		_, teams := env["CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"]
		if bg != (prof == Serial) || teams != (prof == Serial) {
			t.Errorf("%s: serial-only keys: background %v, teams %v", prof, bg, teams)
		}
		if prof == Serial && (env["CLAUDE_CODE_DISABLE_BACKGROUND_TASKS"] != "1" || env["CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"] != "0") {
			t.Errorf("serial values %v", env)
		}
		if env["CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS"] != strconv.Itoa(s.Subagents) {
			t.Errorf("subagents %v", env)
		}
		if !strings.Contains(text, "export CLAUDE_CODE_WARN_MEMORY_FREE_GIB=") {
			t.Errorf("%s: no memory export on linux", prof)
		}
	}
	s, _ := Resolve(owner(Steady, Max), "")
	if text, _ := Render("claude-code", s, "darwin"); strings.Contains(text, "export ") {
		t.Error("memory export on darwin")
	}
	s.MemoryFloorMiB = 0
	if text, _ := Render("claude-code", s, "linux"); strings.Contains(text, "export ") {
		t.Error("memory export with a zero floor")
	}
	s.MemoryFloorMiB = 2049
	if text, _ := Render("claude-code", s, "linux"); !strings.Contains(text, "FREE_GIB=3 ") {
		t.Errorf("GiB not rounded up:\n%s", text)
	}
}

func TestRenderOthers(t *testing.T) {
	s, _ := Resolve(owner(Steady, Max), "")
	text, err := Render("gas-town", s, "linux")
	if err != nil || !strings.Contains(text, "gt config set scheduler.max_polecats 2\n") {
		t.Errorf("gas town %q %v", text, err)
	}
	checkText(t, text)
	text, err = Render("copilot", s, "linux")
	if err != nil || !strings.Contains(text, "advice only") {
		t.Errorf("copilot %q %v", text, err)
	}
	checkText(t, text)
	if _, err := Render("emacs", s, "linux"); err == nil {
		t.Error("unknown tool rendered")
	}
}

// checkText fails on an en or em dash or one of the bare bee words.
func checkText(t *testing.T, text string) {
	t.Helper()
	if strings.ContainsAny(text, string([]rune{0x2013, 0x2014})) {
		t.Errorf("dash in %q", text)
	}
	for _, w := range []string{"Keeper", "queen", "Cells", "waggled"} {
		if strings.Contains(text, w) {
			t.Errorf("%q holds %q", text, w)
		}
	}
}
