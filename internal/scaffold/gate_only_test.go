package scaffold

import (
	"errors"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/decisions"
	"github.com/waggle-sh/waggle/internal/layout"
)

// tagRevBuild stamps a build with a release tag and a clean revision.
func tagRevBuild(tag, rev string) func() (*debug.BuildInfo, bool) {
	return func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: tag}, Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: "false"},
		}}, true
	}
}

func dirtyBuild() (*debug.BuildInfo, bool) {
	return &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: pinA}, {Key: "vcs.modified", Value: "true"},
	}}, true
}

const gateWorkflowRel = ".github/workflows/waggle-gate.yml"

func gateOpts(dir string, build func() (*debug.BuildInfo, bool)) InitOptions {
	o, _ := opts(dir, "claude-code")
	o.GateOnly, o.BuildInfo = true, build
	return o
}

func TestGateRefTable(t *testing.T) {
	tagWarn := "[warn]   gate Action pinned by tag v1.2.3, not by commit; replace it with the tag's commit SHA"
	cases := []struct {
		name  string
		build func() (*debug.BuildInfo, bool)
		uses  string
		warn  bool
	}{
		{"clean revision", commitBuild(pinA), "gate@" + pinA + " # waggle.pin " + pinA, false},
		{"tag and revision", tagRevBuild("v1.2.3", pinB), "gate@" + pinB + " # waggle.pin v1.2.3", false},
		{"tag only", fixedBuild, "gate@v1.2.3 # waggle.pin v1.2.3", true},
	}
	for _, c := range cases {
		for _, gateOnly := range []bool{false, true} {
			dir := t.TempDir()
			o, out := opts(dir, "none")
			o.BuildInfo, o.GateOnly = c.build, gateOnly
			mustInit(t, o)
			if got := get(t, dir, gateWorkflowRel); !strings.Contains(got, "uses: waggle-sh/waggle/"+c.uses+"\n") {
				t.Errorf("%s gateOnly=%v: workflow lacks %q:\n%s", c.name, gateOnly, c.uses, got)
			}
			if strings.Contains(out.String(), tagWarn) != c.warn {
				t.Errorf("%s gateOnly=%v: tag warning = %v, want %v:\n%s", c.name, gateOnly, !c.warn, c.warn, out.String())
			}
		}
	}

	// A hive upgrade moves the Action ref with the pin.
	dir := hive(t, "none", commitBuild(pinA), nil)
	o, _ := upgradeOpts(dir, "none", tagRevBuild("v1.2.3", pinB), true)
	upgrade(t, o)
	if got := get(t, dir, gateWorkflowRel); !strings.Contains(got, "gate@"+pinB+" # waggle.pin v1.2.3\n") {
		t.Errorf("upgrade did not move the ref:\n%s", got)
	}
	commitAll(t, dir, "upgrade")
	o, _ = upgradeOpts(dir, "none", fixedBuild, true)
	o.AllowDowngrade = true
	out := upgrade(t, o)
	if !strings.Contains(out, tagWarn) || !strings.HasSuffix(out, upgradeLastLine+"\n") {
		t.Errorf("tag-only upgrade output:\n%s", out)
	}
	if !strings.Contains(get(t, dir, gateWorkflowRel), "gate@v1.2.3 # waggle.pin v1.2.3\n") {
		t.Error("tag-only upgrade did not write the tag ref")
	}

	// Unpinned builds: a full init skips the workflow, --gate-only and upgrade refuse.
	for name, build := range map[string]func() (*debug.BuildInfo, bool){"dirty": dirtyBuild, "no build info": unpinnedBuild} {
		dir := t.TempDir()
		o, out := opts(dir, "none")
		o.BuildInfo = build
		mustInit(t, o)
		if exists(dir, gateWorkflowRel) || !exists(dir, layout.Roster) {
			t.Errorf("%s: full init should skip only the workflow", name)
		}
		if !strings.Contains(out.String(), "[warn]   "+gateWorkflowRel+" not written: this build has no commit or release tag to pin the gate Action to") {
			t.Errorf("%s: no skip warning:\n%s", name, out.String())
		}

		dir = t.TempDir()
		go1 := gateOpts(dir, build)
		if err := InitRepo(go1); err == nil || len(tree(t, dir)) != 0 {
			t.Errorf("%s: --gate-only err = %v, files %v", name, err, tree(t, dir))
		}

		dir = hive(t, "none", commitBuild(pinA), nil)
		uo, _ := upgradeOpts(dir, "none", build, true)
		if err := Upgrade(uo); !errors.Is(err, ErrRefused) {
			t.Errorf("%s: upgrade err = %v, want ErrRefused", name, err)
		}
	}
}

func TestInitGateOnly(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "CLAUDE.md", "mine\n")
	put(t, dir, "AGENTS.md", "also mine\n")
	put(t, dir, ".gitignore", "node_modules/\n")
	before := snapshot(t, dir)
	o := gateOpts(dir, commitBuild(pinA))
	mustInit(t, o)

	want := []string{".gitignore", ".github/workflows/waggle-gate.yml", ".waggle/control/config.yaml", ".waggle/hive/DECISIONS.md", "AGENTS.md", "CLAUDE.md"}
	sort.Strings(want)
	if got := tree(t, dir); !reflect.DeepEqual(got, want) {
		t.Errorf("file set:\n got %v\nwant %v", got, want)
	}
	for _, rel := range []string{"CLAUDE.md", "AGENTS.md", ".gitignore"} {
		if get(t, dir, rel) != before[rel] {
			t.Errorf("%s changed", rel)
		}
	}
	cfg, err := config.Parse([]byte(get(t, dir, layout.Config)))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Forge.Provider != "github" || cfg.Forge.Remote != "origin" || len(cfg.Gate.ProtectedPaths) != 0 || len(cfg.Gate.Approvers) != 0 {
		t.Errorf("gate-only config = %+v", cfg)
	}
	if got := get(t, dir, layout.Decisions); got != string(decisions.Render(nil)) {
		t.Errorf("DECISIONS.md = %q", got)
	}
	for _, rel := range []string{gateWorkflowRel, layout.Config, layout.Decisions} {
		c := get(t, dir, rel)
		if strings.ContainsAny(c, dashes) || strings.Contains(strings.ReplaceAll(c, "${{", ""), "{{") {
			t.Errorf("%s has a dash or an unrendered placeholder", rel)
		}
	}
	if !strings.Contains(get(t, dir, layout.Config), "\n  pin: "+pinA+" # ") {
		t.Error("config lacks the pin line")
	}

	// A rerun changes nothing; --force rewrites only the workflow.
	snap := snapshot(t, dir)
	mustInit(t, o)
	if !reflect.DeepEqual(snap, snapshot(t, dir)) {
		t.Error("rerun changed files")
	}
	put(t, dir, gateWorkflowRel, "hand edit\n")
	put(t, dir, layout.Config, "version: 1\nhand: edit\n")
	put(t, dir, layout.Decisions, "hand edit\n")
	o.Force = true
	mustInit(t, o)
	if get(t, dir, gateWorkflowRel) != snap[gateWorkflowRel] {
		t.Error("--force did not rewrite the workflow")
	}
	if get(t, dir, layout.Config) != "version: 1\nhand: edit\n" || get(t, dir, layout.Decisions) != "hand edit\n" {
		t.Error("--force rewrote a seeded file")
	}

	// A forge other than github writes nothing.
	dir = t.TempDir()
	o = gateOpts(dir, commitBuild(pinA))
	o.Forge = "none"
	if err := InitRepo(o); err == nil || len(tree(t, dir)) != 0 {
		t.Errorf("--forge none: err = %v, files %v", err, tree(t, dir))
	}
}

func TestInitGateOnlyInFullHive(t *testing.T) {
	dir := t.TempDir()
	o, _ := opts(dir, "none")
	mustInit(t, o)
	before := snapshot(t, dir)
	g := gateOpts(dir, fixedBuild)
	mustInit(t, g)
	after := snapshot(t, dir)
	if after[layout.Decisions] != string(decisions.Render(nil)) {
		t.Error("DECISIONS.md not created")
	}
	delete(after, layout.Decisions)
	if !reflect.DeepEqual(before, after) {
		t.Error("--gate-only in a full hive changed more than DECISIONS.md")
	}
	snap := snapshot(t, dir)
	mustInit(t, g)
	if !reflect.DeepEqual(snap, snapshot(t, dir)) {
		t.Error("rerun changed files")
	}
}

func TestFullInitOverGateOnly(t *testing.T) {
	dir := t.TempDir()
	mustInit(t, gateOpts(dir, fixedBuild))
	before := snapshot(t, dir)

	o, _ := opts(dir, "claude-code")
	err := InitRepo(o)
	if err == nil || !strings.Contains(err.Error(), ".waggle/control/config.yaml has no runtimes: block (a gate-only config); add the runtimes: block a full init writes, or rerun with --runtime none") {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(before, snapshot(t, dir)) {
		t.Error("refused init wrote files")
	}

	o, _ = opts(dir, "none")
	mustInit(t, o)
	empty := t.TempDir()
	e, _ := opts(empty, "none")
	mustInit(t, e)
	want := append(tree(t, empty), layout.Decisions)
	sort.Strings(want)
	if got := tree(t, dir); !reflect.DeepEqual(got, want) {
		t.Errorf("file set:\n got %v\nwant %v", got, want)
	}
	if get(t, dir, layout.Config) != before[layout.Config] {
		t.Error("gate-only config was not kept")
	}
}

// gateOnlyRepo is a committed gate-only repo built at rev.
func gateOnlyRepo(t *testing.T, rev string) string {
	t.Helper()
	dir := newRepo(t)
	mustInit(t, gateOpts(dir, commitBuild(rev)))
	commitAll(t, dir, "waggle init --gate-only")
	return dir
}

func TestUpgradeGateOnly(t *testing.T) {
	for _, rt := range []string{"claude-code", "none"} {
		dir := gateOnlyRepo(t, pinA)
		before := project(t, dir)
		o, _ := upgradeOpts(dir, rt, commitBuild(pinB), false)
		out := upgrade(t, o)
		wantLines(t, out, "[note]    gate-only repo: only waggle.pin and the gate workflow are upgraded",
			"[pin]     "+pinA+" -> "+pinB, "[rewrite] "+layout.GateWorkflow, "[skip]    "+layout.Config+" (seeded)")
		for _, bad := range []string{"[create]", "[edit]", "[shims]", "[remove]", "lacks top-level key", "model policy"} {
			if strings.Contains(out, bad) {
				t.Errorf("%s: output has %q:\n%s", rt, bad, out)
			}
		}
		if !reflect.DeepEqual(before, project(t, dir)) {
			t.Errorf("%s: dry run changed files", rt)
		}

		o, _ = upgradeOpts(dir, rt, commitBuild(pinB), true)
		upgrade(t, o)
		if st := git(t, dir, "status", "--porcelain=v1"); st != "M "+layout.GateWorkflow+"\n M "+layout.Config {
			t.Errorf("%s: status after apply = %q", rt, st)
		}
		if p := project(t, dir); len(p) != 3 {
			t.Errorf("%s: file set after apply = %v", rt, p)
		}
		commitAll(t, dir, "upgrade")

		state := repoState(t, dir)
		o, _ = upgradeOpts(dir, rt, commitBuild(pinB), true)
		out = upgrade(t, o)
		wantLines(t, out, "[pin]     "+pinB+" (unchanged)", "[same]    "+layout.GateWorkflow)
		if got := repoState(t, dir); got != state {
			t.Errorf("%s: unchanged pin is not a no-op", rt)
		}
	}

	// A gate-only config without gate: gets a key note.
	dir := gateOnlyRepo(t, pinA)
	cfg := get(t, dir, layout.Config)
	put(t, dir, layout.Config, cfg[:strings.Index(cfg, "gate:")])
	commitAll(t, dir, "drop gate")
	o, _ := upgradeOpts(dir, "claude-code", commitBuild(pinA), false)
	wantLines(t, upgrade(t, o), `[note]    `+layout.Config+` lacks top-level key "gate"`)
}

func TestUninstallGateOnly(t *testing.T) {
	dir := newRepo(t)
	put(t, dir, "README.md", "project\n")
	mustInit(t, gateOpts(dir, fixedBuild))
	commitAll(t, dir, "waggle init --gate-only")
	before, state := project(t, dir), repoState(t, dir)

	if _, err := uninstall(t, dir, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, project(t, dir)) || repoState(t, dir) != state {
		t.Error("dry run changed the tree")
	}
	if _, err := uninstall(t, dir, true); err != nil {
		t.Fatal(err)
	}
	if got := project(t, dir); !reflect.DeepEqual(got, map[string]string{"README.md": "project\n"}) {
		t.Errorf("tree after uninstall = %v", got)
	}
}
