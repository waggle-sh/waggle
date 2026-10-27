// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/runtime"
)

var contextRoster = map[string]runtime.Level{
	"queen": runtime.LevelFrontier, "builder": runtime.LevelStandard, "guard": runtime.LevelStandard,
}

// withContext replaces the template's context block with block.
func withContext(t *testing.T, block string) []byte {
	t.Helper()
	cfg := string(renderTemplate(t, "direct"))
	const tmplBlock = "context:\n  profile: balanced # lean | balanced | thorough\n"
	if !strings.Contains(cfg, tmplBlock) {
		t.Fatal("template lacks the context block")
	}
	return []byte(strings.Replace(cfg, tmplBlock, block, 1))
}

// Test 10: the rendered template parses to balanced with no overrides, and
// LoadContext reads it alongside the roster template.
func TestContextFromTemplate(t *testing.T) {
	want := runtime.ContextPolicy{Default: runtime.ProfileBalanced, Roles: map[string]runtime.Profile{}}
	got, err := ParseContext(renderTemplate(t, "direct"), contextRoster)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseContext(template) = %+v, %v; want %+v", got, err, want)
	}
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
	if got, err := LoadContext(dir); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("LoadContext = %+v, %v; want %+v", got, err, want)
	}
	if _, err := LoadContext(t.TempDir()); err == nil {
		t.Error("LoadContext on an empty dir succeeded")
	}
}

func TestContextAbsentAndOverrides(t *testing.T) {
	got, err := ParseContext(withContext(t, ""), contextRoster)
	if err != nil || got.Default != runtime.ProfileBalanced || len(got.Roles) != 0 {
		t.Errorf("absent block = %+v, %v; want balanced, no roles", got, err)
	}
	got, err = ParseContext(withContext(t, "context:\n  profile: lean\n  roles:\n    guard: thorough\n    builder: lean\n"), contextRoster)
	want := runtime.ContextPolicy{Default: runtime.ProfileLean, Roles: map[string]runtime.Profile{"guard": runtime.ProfileThorough, "builder": runtime.ProfileLean}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("overrides = %+v, %v; want %+v", got, err, want)
	}
}

// Test 11: every rejection wraps ErrInvalidConfig and names the key path and line.
func TestContextRejections(t *testing.T) {
	cases := map[string]struct{ block, want string }{
		"capitalized":     {"context:\n  profile: Lean\n", "context.profile"},
		"unknown word":    {"context:\n  profile: max\n", "context.profile"},
		"empty":           {"context:\n  profile: \"\"\n", "context.profile"},
		"unknown key":     {"context:\n  profile: lean\n  budget: 5\n", "context.budget: unknown key"},
		"not in roster":   {"context:\n  roles:\n    scout: lean\n", "context.roles.scout: unknown role"},
		"waggled":         {"context:\n  roles:\n    waggled: lean\n", "context.roles.waggled"},
		"bad role value":  {"context:\n  roles:\n    guard: max\n", "context.roles.guard"},
		"roles scalar":    {"context:\n  roles: guard\n", "context.roles: must be a mapping"},
		"context scalar":  {"context: lean\n", "context: must be a mapping"},
		"profile mapping": {"context:\n  profile:\n    a: b\n", "context.profile: must be a scalar"},
	}
	for name, c := range cases {
		_, err := ParseContext(withContext(t, c.block), contextRoster)
		if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "line ") {
			t.Errorf("%s: err = %v; want ErrInvalidConfig with %q and a line number", name, err, c.want)
		}
	}
}

// LoadPolicy and ParsePolicy reject a bad context block too, so a hive cannot
// carry a context error past init or upgrade.
func TestPolicyChecksContext(t *testing.T) {
	cfg := withContext(t, "context:\n  roles:\n    nobody: lean\n")
	if _, err := ParsePolicy(cfg, []byte(renderRoster(t))); !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "context.roles.nobody") {
		t.Errorf("ParsePolicy with a stale role key: err = %v; want ErrInvalidConfig naming context.roles.nobody", err)
	}
	cfg = withContext(t, "context:\n  roles:\n    guard: lean\n")
	if _, err := ParsePolicy(cfg, []byte(renderRoster(t))); err != nil {
		t.Errorf("ParsePolicy with guard: lean: %v", err)
	}
}
