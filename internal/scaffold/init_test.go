package scaffold

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
)

const testPin = "v1.2.3"

func fixedBuild() (*debug.BuildInfo, bool) {
	return &debug.BuildInfo{Main: debug.Module{Version: testPin}}, true
}

func opts(dir, rt string) (InitOptions, *bytes.Buffer) {
	var out bytes.Buffer
	return InitOptions{
		TargetDir: dir, Profile: "cli", Delivery: "installed", Forge: "github",
		CombMode: "direct", Runtime: rt, BuildInfo: fixedBuild, Out: &out,
	}, &out
}

func mustInit(t *testing.T, o InitOptions) {
	t.Helper()
	if err := InitRepo(o); err != nil {
		t.Fatal(err)
	}
}

func put(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// tree returns every file (and every empty directory, with a trailing slash) under dir.
func tree(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == dir {
			return err
		}
		rel := filepath.ToSlash(p[len(dir)+1:])
		if !d.IsDir() {
			out = append(out, rel)
			return nil
		}
		if ents, _ := os.ReadDir(p); len(ents) == 0 {
			out = append(out, rel+"/")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

var roleNames = []string{"scout", "builder", "guard", "queen", "forager"}

const wantGitignore = "# --- waggle:start ---\n" +
	".waggle/control/local.yaml\n.waggle/control/run.sock\n.waggle/worktrees/\n.claude/agents/waggle-*.md\n" +
	"# --- waggle:end ---\n"

const wantClaudeBlock = "<!-- waggle:start -->\n@.waggle/control/WAGGLE.md\n<!-- waggle:end -->\n"

func TestInitFileSet(t *testing.T) {
	dir := t.TempDir()
	o, _ := opts(dir, "claude-code")
	mustInit(t, o)

	want := []string{
		".github/workflows/waggle-gate.yml", ".gitignore", ".waggle/control/WAGGLE.md",
		".waggle/control/config.yaml", ".waggle/control/devcontainer.json", ".waggle/control/local.yaml",
		".waggle/control/roster.json", ".waggle/hive/media/", ".waggle/hive/reviews/", ".waggle/hive/tasks/", "CLAUDE.md",
	}
	for _, r := range roleNames {
		want = append(want, ".claude/agents/waggle-"+r+".md", ".waggle/control/roles/"+r+".md")
	}
	sort.Strings(want)
	if got := tree(t, dir); !reflect.DeepEqual(got, want) {
		t.Errorf("file set:\n got %v\nwant %v", got, want)
	}
	if got := get(t, dir, ".gitignore"); got != wantGitignore {
		t.Errorf(".gitignore = %q", got)
	}
	if got := get(t, dir, "CLAUDE.md"); got != wantClaudeBlock {
		t.Errorf("CLAUDE.md = %q", got)
	}
	if cfg := get(t, dir, ".waggle/control/config.yaml"); !strings.Contains(cfg, "\n  pin: "+testPin+" # ") {
		t.Errorf("config.yaml missing pin:\n%s", cfg)
	}
	if !reflect.DeepEqual(tree(t, dir), want) {
		t.Fatal("unstable")
	}
	// Rerunning changes nothing.
	before := snapshot(t, dir)
	mustInit(t, o)
	if after := snapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Error("second init changed files")
	}
}

func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, rel := range tree(t, dir) {
		if strings.HasSuffix(rel, "/") {
			m[rel] = ""
			continue
		}
		m[rel] = get(t, dir, rel)
	}
	return m
}

const legacyGitignore = `
# --- waggle:start ---
# Local Keeper preferences & runtime daemon state
.waggle/local.yaml
.waggle/run.sock
.waggle/worktrees/
.dashboard/

# Materialized vendor/IDE shims (generated from .waggle/roles/ and WAGGLE.md)
.claude/
CLAUDE.md
AGENTS.md
GEMINI.md
.cursor/
.aider*
# --- waggle:end ---
`

func TestInitPreservesProjectFiles(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "CLAUDE.md", "# Project rules\n\nBe nice.\n")
	put(t, dir, ".gitignore", "node_modules/"+legacyGitignore+"dist/\n")
	put(t, dir, ".claude/agents/builder.md", "mine\n")
	o, _ := opts(dir, "claude-code")
	mustInit(t, o)

	if got := get(t, dir, "CLAUDE.md"); got != "# Project rules\n\nBe nice.\n"+wantClaudeBlock {
		t.Errorf("CLAUDE.md = %q", got)
	}
	want := "node_modules/\n" + wantGitignore + "dist/\n"
	if got := get(t, dir, ".gitignore"); got != want {
		t.Errorf(".gitignore = %q; want %q", got, want)
	}
	if got := get(t, dir, ".claude/agents/builder.md"); got != "mine\n" {
		t.Errorf("project agent changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Error("AGENTS.md created alongside CLAUDE.md")
	}
}

func TestInitImportTarget(t *testing.T) {
	agentsBlock := "<!-- waggle:start -->\nRead .waggle/control/WAGGLE.md before working in this repo.\n<!-- waggle:end -->\n"

	dir := t.TempDir()
	o, _ := opts(dir, "none")
	mustInit(t, o)
	if got := get(t, dir, "AGENTS.md"); got != agentsBlock {
		t.Errorf("runtime none: AGENTS.md = %q", got)
	}
	for _, p := range []string{"CLAUDE.md", ".claude"} {
		if _, err := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(err) {
			t.Errorf("runtime none created %s", p)
		}
	}

	dir = t.TempDir()
	put(t, dir, "AGENTS.md", "# Agents\n")
	o, out := opts(dir, "claude-code")
	mustInit(t, o)
	if got := get(t, dir, "AGENTS.md"); got != "# Agents\n"+agentsBlock {
		t.Errorf("existing AGENTS.md = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Error("CLAUDE.md created alongside AGENTS.md")
	}
	if !strings.Contains(out.String(), "Claude Code reads CLAUDE.md") {
		t.Error("missing CLAUDE.md note")
	}
}

func TestInitForce(t *testing.T) {
	dir := t.TempDir()
	o, _ := opts(dir, "none")
	mustInit(t, o)
	roster := get(t, dir, ".waggle/control/roster.json")
	put(t, dir, ".waggle/control/roster.json", "hand edit\n")
	put(t, dir, ".waggle/control/config.yaml", "hive owned\n")
	put(t, dir, ".waggle/control/WAGGLE.md", "ours\n")

	mustInit(t, o)
	if got := get(t, dir, ".waggle/control/roster.json"); got != "hand edit\n" {
		t.Error("managed file rewritten without --force")
	}
	o.Force = true
	mustInit(t, o)
	if got := get(t, dir, ".waggle/control/roster.json"); got != roster {
		t.Error("--force did not rewrite managed roster.json")
	}
	if get(t, dir, ".waggle/control/config.yaml") != "hive owned\n" || get(t, dir, ".waggle/control/WAGGLE.md") != "ours\n" {
		t.Error("--force overwrote a seeded file")
	}
}

func TestInitRefusesWithoutWriting(t *testing.T) {
	setups := map[string]func(dir string){
		"legacy config": func(dir string) { put(t, dir, ".waggle/config.yaml", "x\n") },
		"legacy roles":  func(dir string) { put(t, dir, ".waggle/roles/queen.md", "x\n") },
		"bad markers":   func(dir string) { put(t, dir, ".gitignore", "# --- waggle:start ---\nx\n") },
	}
	for name, setup := range setups {
		dir := t.TempDir()
		setup(dir)
		before := snapshot(t, dir)
		o, _ := opts(dir, "claude-code")
		err := InitRepo(o)
		if err == nil {
			t.Errorf("%s: init succeeded", name)
		}
		if strings.HasPrefix(name, "legacy") && !errors.Is(err, ErrLegacyLayout) {
			t.Errorf("%s: %v; want ErrLegacyLayout", name, err)
		}
		if after := snapshot(t, dir); !reflect.DeepEqual(before, after) {
			t.Errorf("%s: files changed: %v", name, after)
		}
	}
}

func TestResolvePin(t *testing.T) {
	rev := strings.Repeat("ab", 20)
	bi := func(v string, settings ...string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			b := &debug.BuildInfo{Main: debug.Module{Version: v}}
			for i := 0; i+1 < len(settings); i += 2 {
				b.Settings = append(b.Settings, debug.BuildSetting{Key: settings[i], Value: settings[i+1]})
			}
			return b, true
		}
	}
	cases := []struct {
		name string
		read func() (*debug.BuildInfo, bool)
		want string
		ok   bool
	}{
		{"release", bi("v0.2.0"), "v0.2.0", true},
		{"prerelease", bi("v0.2.0-rc.1"), "v0.2.0-rc.1", true},
		{"pseudo clean", bi("v0.0.0-20261017120000-abcdefabcdef", "vcs.revision", rev, "vcs.modified", "false"), rev, true},
		{"devel clean", bi("(devel)", "vcs.revision", rev, "vcs.modified", "false"), rev, true},
		{"dirty", bi("v0.0.0-20261017120000-abcdefabcdef+dirty", "vcs.revision", rev, "vcs.modified", "true"), Unpinned, false},
		{"no vcs", bi("(devel)"), Unpinned, false},
		{"short rev", bi("(devel)", "vcs.revision", "abc", "vcs.modified", "false"), Unpinned, false},
		{"no info", func() (*debug.BuildInfo, bool) { return nil, false }, Unpinned, false},
	}
	for _, tc := range cases {
		if got, ok := resolvePin(tc.read); got != tc.want || ok != tc.ok {
			t.Errorf("%s: got %q, %v; want %q, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}

	dir := t.TempDir()
	o, out := opts(dir, "none")
	o.BuildInfo = bi("(devel)")
	mustInit(t, o)
	if !strings.Contains(get(t, dir, ".waggle/control/config.yaml"), "pin: unpinned #") {
		t.Error("unpinned not rendered")
	}
	if !strings.Contains(out.String(), "[warn]   built from a modified or unstamped tree; set waggle.pin by hand") {
		t.Errorf("missing warning in %q", out.String())
	}
}

func TestRenderedTemplatesClean(t *testing.T) {
	dir := t.TempDir()
	o, _ := opts(dir, "claude-code")
	mustInit(t, o)
	forbidden := []string{"docs/tasks", "docs/reviews", "docs/media", "Master" + " Keeper", "Control" + "ler", "\u2014", "\u2013", "internal/comb"}
	for rel, content := range snapshot(t, dir) {
		for _, f := range forbidden {
			if strings.Contains(content, f) {
				t.Errorf("%s contains %q", rel, f)
			}
		}
		if strings.Contains(strings.ReplaceAll(content, "${{", ""), "{{") {
			t.Errorf("%s has an unrendered placeholder", rel)
		}
	}
	gate := get(t, dir, ".github/workflows/waggle-gate.yml")
	if lines := strings.SplitN(gate, "\n", 3); lines[1] != "# waggle:managed: installed by waggle init; removed by waggle uninstall. Do not edit by hand." {
		t.Errorf("gate second line = %q", lines[1])
	}
	hive := strings.Index(gate, ".waggle/hive|.waggle/hive/*)")
	ctl := strings.Index(gate, ".waggle|.waggle/*|.github/workflows/*|.gitlab-ci.yml|.devcontainer/*)")
	steer := strings.Index(gate, "CLAUDE.md|AGENTS.md)")
	if !(hive >= 0 && hive < ctl && ctl < steer) || !strings.Contains(gate, "--no-renames") {
		t.Error("gate case order or --no-renames wrong")
	}
	if strings.Contains(gate, "gate decisions") {
		t.Error("gate has a decisions step")
	}
	if !strings.Contains(get(t, dir, ".waggle/control/devcontainer.json"), `"/workspace/.waggle/control/run.sock"`) {
		t.Error("devcontainer socket path")
	}
	cfg := get(t, dir, ".waggle/control/config.yaml")
	for _, s := range []string{"socket_path: .waggle/control/run.sock", "derived_workplan: .waggle/hive/WORKPLAN.md", "merge_strategy: up_to_date"} {
		if !strings.Contains(cfg, s) {
			t.Errorf("config.yaml missing %q", s)
		}
	}
}
