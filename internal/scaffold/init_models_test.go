package scaffold

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
)

var modelLine = regexp.MustCompile(`(?m)^model: (\S+)$`)

func shimModel(t *testing.T, dir, role string) string {
	t.Helper()
	m := modelLine.FindStringSubmatch(get(t, dir, layout.ShimPrefix+role+".md"))
	if m == nil {
		t.Fatalf("waggle-%s.md has no model: line", role)
	}
	return m[1]
}

// Test 15: init writes model lines; guard differs from builder, scout is frontier.
func TestInitWritesShimModels(t *testing.T) {
	dir := t.TempDir()
	o, _ := opts(dir, "claude-code")
	mustInit(t, o)
	o.Force = true
	mustInit(t, o)
	if g, b := shimModel(t, dir, "guard"), shimModel(t, dir, "builder"); g == b {
		t.Errorf("guard and builder share model %q", g)
	}
	if s := shimModel(t, dir, "scout"); s != "opus" {
		t.Errorf("scout model = %q; want the frontier model opus", s)
	}
}

func TestInitFailsWithoutLevels(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, layout.Config, "version: 1\nruntimes:\n  default: claude-code\n  adapters:\n    claude-code:\n      binary: claude\n")
	o, _ := opts(dir, "claude-code")
	if err := InitRepo(o); err == nil || !strings.Contains(err.Error(), "runtimes.adapters.claude-code.levels") {
		t.Fatalf("err = %v; want a missing levels error", err)
	}
	if exists(dir, layout.ShimPrefix+"builder.md") {
		t.Error("shim written without a model policy")
	}
}

func TestPlaceholdersEscapeJSON(t *testing.T) {
	evil := `cli", "entry": "x\`
	kv := []string{"{{PROFILE}}", evil}
	var doc map[string]string
	if err := json.Unmarshal([]byte(placeholders(layout.Roster, kv).Replace(`{"profile": "{{PROFILE}}"}`)), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["profile"] != evil || len(doc) != 1 {
		t.Errorf("decoded %v; want profile %q only", doc, evil)
	}
	if got := placeholders(layout.WaggleMD, kv).Replace("{{PROFILE}}"); got != evil {
		t.Errorf("non-JSON destination escaped: %q", got)
	}
	if kv[1] != evil {
		t.Error("placeholders changed its input")
	}
}

func TestInitRejectsUnknownForge(t *testing.T) {
	dir := t.TempDir()
	o, _ := opts(dir, "none")
	o.Forge = `github", "x": "y`
	if err := InitRepo(o); err == nil || !strings.Contains(err.Error(), "--forge") {
		t.Fatalf("err = %v; want a --forge error", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("init wrote %d entries before refusing", len(entries))
	}
}

func TestUpgradeEscapesSeededProfile(t *testing.T) {
	dir := hive(t, "none", commitBuild(pinA), nil)
	evil := `cli","entry":"x`
	put(t, dir, layout.Config, strings.Replace(get(t, dir, layout.Config), "profile: cli", "profile: "+evil, 1))
	commitAll(t, dir, "hand-edit profile")
	o, _ := upgradeOpts(dir, "none", commitBuild(pinB), true)
	upgrade(t, o)
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(get(t, dir, layout.Roster)), &top); err != nil {
		t.Fatalf("roster.json is not valid JSON: %v", err)
	}
	var profile, entry string
	if json.Unmarshal(top["profile"], &profile) != nil || profile != evil {
		t.Errorf("profile = %s; want %q", top["profile"], evil)
	}
	if json.Unmarshal(top["entry"], &entry) != nil || entry != "queen" {
		t.Errorf("entry = %s; want queen", top["entry"])
	}
}

func TestInitRefusesSymlinks(t *testing.T) {
	for _, rel := range []string{layout.ControlDir, layout.HiveDir, layout.TasksDir, ".claude/agents", "CLAUDE.md", ".github"} {
		dir, outside := t.TempDir(), t.TempDir()
		target := outside
		if rel == "CLAUDE.md" {
			target = filepath.Join(outside, "notes.md")
			put(t, outside, "notes.md", "outside\n")
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, filepath.FromSlash(rel))), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
		o, _ := opts(dir, "claude-code")
		err := InitRepo(o)
		if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), rel+" is a symlink") {
			t.Errorf("%s: err = %v; want ErrRefused naming it", rel, err)
		}
		if got := tree(t, outside); len(got) > 1 || (len(got) == 1 && get(t, outside, "notes.md") != "outside\n") {
			t.Errorf("%s: init wrote through the symlink: %v", rel, got)
		}
		if got := tree(t, dir); len(got) != 1 {
			t.Errorf("%s: init wrote %v before refusing", rel, got)
		}
	}
}
