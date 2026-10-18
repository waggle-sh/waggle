package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/runtime"
)

const rosterTemplatePath = "../scaffold/templates/roster.json.tmpl"

// sectionOneMinimums is the T-012 section 1 table.
var sectionOneMinimums = map[string]runtime.Level{
	"scout": runtime.LevelFrontier, "queen": runtime.LevelFrontier,
	"builder": runtime.LevelStandard, "guard": runtime.LevelStandard, "forager": runtime.LevelFast,
}

func renderRoster(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(rosterTemplatePath)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(raw), "{{PROFILE}}", "cli")
}

func writeRoster(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "roster.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Test 11: the real templates load, validate, and carry the section 1 minimums.
func TestLoadPolicyFromTemplates(t *testing.T) {
	dir := t.TempDir()
	for rel, content := range map[string]string{layout.Config: string(renderTemplate(t, "direct")), layout.Roster: renderRoster(t)} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := LoadPolicy(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Minimums, sectionOneMinimums) {
		t.Errorf("minimums = %v; want %v", p.Minimums, sectionOneMinimums)
	}
	if p.Models.Adapter != "claude-code" || len(p.Models.Primary) != 3 {
		t.Errorf("models = %+v", p.Models)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(renderRoster(t)), &top); err != nil {
		t.Fatal(err)
	}
	if _, ok := top["mode"]; ok {
		t.Error("rendered roster still has a top-level mode key (D-070)")
	}
}

func TestLoadPolicyMissingFiles(t *testing.T) {
	if _, err := LoadPolicy(t.TempDir()); err == nil {
		t.Error("LoadPolicy on an empty dir succeeded")
	}
}

// Test 14: roster rejections.
func TestLoadRosterRejects(t *testing.T) {
	base := renderRoster(t)
	cases := []struct {
		name, src string
		also      error
	}{
		{"missing min_level", strings.Replace(base, `      "min_level": "fast",`+"\n", "", 1), nil},
		{"model name as level", strings.Replace(base, `"min_level": "fast"`, `"min_level": "opus"`, 1), runtime.ErrInvalidLevel},
		{"wrong key case", strings.Replace(base, `"min_level": "fast"`, `"MIN_LEVEL": "fast"`, 1), nil},
		{"duplicate role id", strings.Replace(base, `"id": "forager"`, `"id": "scout"`, 1), nil},
		{"waggled role", strings.Replace(base, `"id": "forager"`, `"id": "waggled"`, 1), runtime.ErrNoModelRole},
		{"empty id", strings.Replace(base, `"id": "forager"`, `"id": ""`, 1), nil},
		{"no roles", `{"version": 1, "roles": []}`, nil},
		{"not json", `{"roles": [`, nil},
		{"oversize", `{"roles": [], "pad": "` + strings.Repeat("x", MaxFileSize) + `"}`, nil},
	}
	for _, c := range cases {
		if c.src == base {
			t.Fatalf("%s: replacement did not apply", c.name)
		}
		_, err := LoadRoster(writeRoster(t, c.src))
		if !errors.Is(err, ErrInvalidRoster) {
			t.Errorf("%s: err = %v; want ErrInvalidRoster", c.name, err)
		}
		if c.also != nil && !errors.Is(err, c.also) {
			t.Errorf("%s: err = %v; want it to wrap %v", c.name, err, c.also)
		}
	}
}

// Test 14a: a leftover top-level mode key is ignored, whatever its value (D-070).
func TestLoadRosterIgnoresLeftoverMode(t *testing.T) {
	base := renderRoster(t)
	want, err := LoadRoster(writeRoster(t, base))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{`"swarm"`, `"anything"`} {
		src := strings.Replace(base, `"version": 1,`, `"version": 1, "mode": `+mode+`,`, 1)
		if src == base {
			t.Fatal("replacement did not apply")
		}
		got, err := LoadRoster(writeRoster(t, src))
		if err != nil {
			t.Fatalf("mode %s: %v", mode, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("mode %s: minimums = %v; want %v", mode, got, want)
		}
	}
}
