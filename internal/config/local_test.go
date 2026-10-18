package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
)

// keeperRepo writes layout.Config and, when local is not nil, layout.Local.
func keeperRepo(t *testing.T, remote string, local *string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := "version: 1\n"
	if remote != "" {
		cfg += "forge:\n  remote: " + remote + "\n"
	}
	files := map[string]string{layout.Config: cfg}
	if local != nil {
		files[layout.Local] = *local
	}
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadKeeperID(t *testing.T) {
	str := func(s string) *string { return &s }
	seeded := "# Local Keeper Preferences\nkeeper_id: local-keeper\nruntime: claude\nclaude_code:\n  model_overrides:\n    builder: sonnet\n"
	cases := []struct {
		name   string
		remote string
		local  *string
		want   string
		errHas string
	}{
		{"present", "origin", str("keeper_id: paulternate\nruntime: claude\nide: vscode\n"), "paulternate", ""},
		{"quoted with comment", "", str("keeper_id: \"paulternate\" # me\n"), "paulternate", ""},
		{"other keys ignored", "origin", str("claude_code:\n  model_overrides:\n    builder: sonnet\nkeeper_id: bee-1\n"), "bee-1", ""},
		{"file absent", "", nil, "", layout.Local},
		{"key absent", "", str("runtime: claude\n"), "", "keeper_id is missing"},
		{"key empty", "", str("keeper_id: \"\"\n"), "", "keeper_id is empty"},
		{"nested not scalar", "", str("keeper_id:\n  a: b\n"), "", "keeper_id"},
		{"placeholder local-only", "", str(seeded), LocalKeeperPlaceholder, ""},
		{"placeholder with remote", "origin", str(seeded), "", "collide"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, err := LoadKeeperID(keeperRepo(t, c.remote, c.local))
			if c.errHas == "" {
				if err != nil || id != c.want {
					t.Fatalf("got %q, %v; want %q", id, err, c.want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.errHas) {
				t.Fatalf("err %v, want it to contain %q", err, c.errHas)
			}
			if c.name != "nested not scalar" && !errors.Is(err, ErrNoKeeperID) {
				t.Fatalf("err %v does not wrap ErrNoKeeperID", err)
			}
		})
	}
}

// The placeholder check needs a readable config.
func TestLoadKeeperIDPlaceholderNeedsConfig(t *testing.T) {
	dir := keeperRepo(t, "", nil)
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(layout.Local)), []byte("keeper_id: local-keeper\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(layout.Config))); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKeeperID(dir); err == nil {
		t.Fatal("placeholder accepted without a readable config")
	}
}
