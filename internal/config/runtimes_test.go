package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/runtime"
)

const runtimesBase = `version: 1
runtimes:
  default: claude-code
  adapters:
    claude-code:
      binary: claude
      nudge_tokens: 200000
      max_tokens: 300000
      levels:
        frontier: opus
        standard: sonnet
        fast: haiku
      alternates:
        frontier: claude-opus-4-1
`

func TestParseRuntimes(t *testing.T) {
	r, err := ParseRuntimes([]byte(runtimesBase))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Models()
	if err != nil {
		t.Fatal(err)
	}
	want := runtime.Models{
		Adapter:   "claude-code",
		Primary:   map[runtime.Level]string{runtime.LevelFrontier: "opus", runtime.LevelStandard: "sonnet", runtime.LevelFast: "haiku"},
		Alternate: map[runtime.Level]string{runtime.LevelFrontier: "claude-opus-4-1"},
	}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("Models() = %+v; want %+v", m, want)
	}
	if got, want := r.Adapters[r.Default].Config, (runtime.AdapterConfig{Binary: "claude", NudgeTokens: 200000, MaxTokens: 300000}); !reflect.DeepEqual(got, want) {
		t.Errorf("AdapterConfig() = %+v; want %+v", got, want)
	}
	// The real config template parses, with no alternates shipped (Q-T012-2).
	r, err = ParseRuntimes(renderTemplate(t, "direct"))
	if err != nil {
		t.Fatalf("template: %v", err)
	}
	if m, _ := r.Models(); len(m.Alternate) != 0 || len(m.Primary) != 3 {
		t.Errorf("template models = %+v", m)
	}
}

// Test 13: every rejection wraps ErrInvalidConfig and names the key path.
func TestParseRuntimesRejects(t *testing.T) {
	without := func(lines ...string) string {
		s := runtimesBase
		for _, l := range lines {
			s = strings.Replace(s, l+"\n", "", 1)
		}
		return s
	}
	cases := []struct{ name, src, path string }{
		{"missing levels", without("      levels:", "        frontier: opus", "        standard: sonnet", "        fast: haiku"), "runtimes.adapters.claude-code.levels.fast"},
		{"missing levels.fast", without("        fast: haiku"), "runtimes.adapters.claude-code.levels.fast"},
		{"leading dash", strings.Replace(runtimesBase, "frontier: opus", "frontier: -x", 1), "runtimes.adapters.claude-code.levels.frontier"},
		{"space in value", strings.Replace(runtimesBase, "standard: sonnet", `standard: "son net"`, 1), "runtimes.adapters.claude-code.levels.standard"},
		{"alternate equals primary", strings.Replace(runtimesBase, "frontier: claude-opus-4-1", "frontier: opus", 1), "runtimes.adapters.claude-code.alternates.frontier"},
		{"unknown level", strings.Replace(runtimesBase, "        fast: haiku\n", "        fast: haiku\n        ultra: big\n", 1), "runtimes.adapters.claude-code.levels.ultra"},
		{"unknown adapter key", strings.Replace(runtimesBase, "      binary: claude\n", "      binary: claude\n      default_args: x\n", 1), "runtimes.adapters.claude-code.default_args"},
		{"unknown runtimes key", strings.Replace(runtimesBase, "  default: claude-code\n", "  default: claude-code\n  extra: 1\n", 1), "runtimes.extra"},
		{"default names missing adapter", strings.Replace(runtimesBase, "default: claude-code", "default: aider", 1), "runtimes.default"},
		{"quoted tokens", strings.Replace(runtimesBase, "max_tokens: 300000", `max_tokens: "300000"`, 1), "runtimes.adapters.claude-code.max_tokens"},
		{"no runtimes", "version: 1\n", "runtimes"},
		{"scalar adapters", "version: 1\nruntimes:\n  default: claude-code\n  adapters: none\n", "runtimes.default"},
	}
	for _, c := range cases {
		_, err := ParseRuntimes([]byte(c.src))
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("%s: err = %v; want ErrInvalidConfig", c.name, err)
			continue
		}
		if !strings.Contains(err.Error(), c.path+":") {
			t.Errorf("%s: err %q does not name %s", c.name, err, c.path)
		}
		if c.path != "runtimes" && !strings.Contains(err.Error(), "line ") {
			t.Errorf("%s: err %q has no line number", c.name, err)
		}
	}
}
