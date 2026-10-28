// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/pace"
)

func TestParsePaceAbsent(t *testing.T) {
	p, err := ParsePace([]byte("version: 1\n"))
	if err != nil || p.Set {
		t.Fatalf("absent section: %+v %v", p, err)
	}
}

func TestParsePaceDefaults(t *testing.T) {
	p, err := ParsePace([]byte("pace:\n  profile: serial\n"))
	if err != nil || !p.Set || p.Profile != pace.Serial || p.Limit != pace.Max || p.Table != pace.Defaults() {
		t.Fatalf("%+v %v", p, err)
	}
	p, err = ParsePace([]byte("pace:\n  limit: steady\n"))
	if err != nil || p.Profile != pace.Steady || p.Limit != pace.Steady {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestParsePaceOverrides(t *testing.T) {
	p, err := ParsePace([]byte("pace:\n  max:\n    parallel_runs: 3\n    memory_floor_mib: 0\n  serial:\n    cpu_target_percent: 40\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Table[pace.Max].ParallelRuns != 3 || p.Table[pace.Max].MemoryFloorMiB != 0 || p.Table[pace.Max].Subagents != 8 ||
		p.Table[pace.Serial].CPUTargetPercent != 40 {
		t.Errorf("table %+v", p.Table)
	}
}

func TestParsePaceBounds(t *testing.T) {
	// Each key at its bounds, set on max for the high end and on serial for the low end.
	for _, k := range paceKeys {
		hi := "pace:\n  max:\n    " + k.name + ": " + itoa(k.hi) + "\n"
		lo := "pace:\n  serial:\n    " + k.name + ": " + itoa(k.lo) + "\n"
		if k.lowerFast {
			hi, lo = "pace:\n  serial:\n    "+k.name+": "+itoa(k.hi)+"\n", "pace:\n  max:\n    "+k.name+": "+itoa(k.lo)+"\n"
		}
		for _, in := range []string{hi, lo} {
			if _, err := ParsePace([]byte(in)); err != nil {
				t.Errorf("%s at bound: %v\n%s", k.name, err, in)
			}
		}
		for _, v := range []int{k.hi + 1, k.lo - 1} {
			in := "pace:\n  steady:\n    " + k.name + ": " + itoa(v) + "\n"
			if _, err := ParsePace([]byte(in)); !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "pace.steady."+k.name) {
				t.Errorf("%s = %d: %v", k.name, v, err)
			}
		}
	}
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

func TestParsePaceErrors(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"unknown key", "pace:\n  speed: fast\n", "line 2: pace.speed: unknown key"},
		{"unknown raw key", "pace:\n  max:\n    threads: 3\n", "line 3: pace.max.threads: unknown key"},
		{"bad word", "pace:\n  profile: turbo\n", "line 2: pace.profile: must be serial, steady or max"},
		{"bad limit", "pace:\n  limit: Max\n", "line 2: pace.limit: must be serial, steady or max"},
		{"profile over limit", "pace:\n  profile: max\n  limit: steady\n", "line 2: pace.profile: max is faster than pace.limit steady"},
		{"default over limit", "pace:\n  limit: serial\n", "line 2: pace.profile: steady is faster than pace.limit serial"},
		{"not a mapping", "pace: fast\n", "line 1: pace: must be a mapping"},
		{"profile mapping", "pace:\n  serial: 3\n", "line 2: pace.serial: must be a mapping"},
		{"quoted int", "pace:\n  max:\n    subagents: \"4\"\n", "pace.max.subagents: must be a plain integer"},
		{"word int", "pace:\n  max:\n    subagents: many\n", "pace.max.subagents: must be a base-10 integer"},
		{"serial above steady", "pace:\n  serial:\n    parallel_runs: 3\n", "line 3: pace.serial.parallel_runs: serial is faster than steady"},
		{"steady above max", "pace:\n  steady:\n    subagents: 9\n", "line 3: pace.steady.subagents: steady is faster than max"},
		{"max below steady", "pace:\n  max:\n    cpu_target_percent: 60\n", "line 3: pace.max.cpu_target_percent: steady is faster than max"},
		{"floor unordered", "pace:\n  max:\n    memory_floor_mib: 3000\n", "line 3: pace.max.memory_floor_mib: steady is faster than max"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParsePace([]byte(tc.in))
			if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %v, want %q", err, tc.want)
			}
			// The semantic error never reaches config.Parse (the gate).
			if _, err := Parse([]byte("version: 1\n" + tc.in)); err != nil {
				t.Errorf("config.Parse failed on a semantic pace error: %v", err)
			}
		})
	}
}

func TestParsePaceSyntaxErrorFailsParse(t *testing.T) {
	in := "version: 1\npace:\n\tprofile: steady\n"
	if _, err := ParsePace([]byte(in)); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("ParsePace: %v", err)
	}
	if _, err := Parse([]byte(in)); err == nil {
		t.Error("config.Parse accepted a tab in the pace block")
	}
}

func TestPaceFromTemplate(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "scaffold", "templates", "config.yaml.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if p, _ := ParsePace([]byte(text)); p.Set {
		t.Error("template sets pace")
	}
	// The commented example, uncommented, is a valid section.
	var b strings.Builder
	in := false
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "# pace:"):
			in = true
			b.WriteString("pace:\n")
		case in && strings.HasPrefix(line, "#   "):
			b.WriteString(strings.TrimPrefix(line, "# ") + "\n")
		default:
			in = false
		}
	}
	p, err := ParsePace([]byte(b.String()))
	if err != nil || !p.Set || p.Table[pace.Max].ParallelRuns != 3 {
		t.Errorf("template example: %+v %v\n%s", p, err, b.String())
	}
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadPacePick(t *testing.T) {
	dir := t.TempDir()
	if v, err := LoadPacePick(dir); v != "" || err != nil {
		t.Errorf("missing file: %q %v", v, err)
	}
	cases := []struct{ in, want string }{
		{"keeper_id: paul\n", ""},
		{"keeper_id: paul\npace:\n  other: 1\n", ""},
		{"keeper_id: paul\npace:\n  profile: serial # slow\n", "serial"},
		{"pace:\n  profile: turbo\n  max:\n    parallel_runs: 9\n", "turbo"},
	}
	for _, tc := range cases {
		writeFile(t, dir, layout.Local, tc.in)
		if v, err := LoadPacePick(dir); v != tc.want || err != nil {
			t.Errorf("%q: got %q %v", tc.in, v, err)
		}
	}
	for _, bad := range []string{"pace: serial\n", "pace:\n  profile:\n    x: 1\n", "pace:\n\tprofile: serial\n"} {
		writeFile(t, dir, layout.Local, bad)
		if _, err := LoadPacePick(dir); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func TestLoadPace(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadPace(dir); err == nil {
		t.Error("missing config loaded")
	}
	writeFile(t, dir, layout.Config, "version: 1\npace:\n  limit: steady\n")
	if p, err := LoadPace(dir); err != nil || p.Limit != pace.Steady {
		t.Errorf("%+v %v", p, err)
	}
}
