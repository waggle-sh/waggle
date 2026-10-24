package scaffold

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/waggle-sh/waggle/internal/layout"
)

var (
	pinA = strings.Repeat("a", 40)
	pinB = strings.Repeat("b", 40)
)

// commitBuild stamps a build with a clean vcs.revision.
func commitBuild(rev string) func() (*debug.BuildInfo, bool) {
	return func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: "false"},
		}}, true
	}
}

// tagBuild stamps a build with a release tag.
func tagBuild(tag string) func() (*debug.BuildInfo, bool) {
	return func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: tag}}, true
	}
}

func unpinnedBuild() (*debug.BuildInfo, bool) { return nil, false }

// templatesA is the embedded set plus one role the embedded set does not ship.
func templatesA(t *testing.T) fs.FS {
	t.Helper()
	m := fstest.MapFS{}
	err := fs.WalkDir(templateFS, "templates", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(templateFS, p)
		m[p] = &fstest.MapFile{Data: b}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	m["templates/roles/oldrole.md"] = &fstest.MapFile{Data: []byte("# Role: Oldrole (Retired)\n")}
	// init resolves a model for every role file (T-012), so the old role needs a roster entry.
	roster := m["templates/roster.json.tmpl"]
	roster.Data = []byte(strings.Replace(string(roster.Data), `"roles": [`, `"roles": [{"id": "oldrole", "min_level": "fast"},`, 1))
	return m
}

// hive runs init with the given build and templates in a fresh repo and commits it.
func hive(t *testing.T, rt string, build func() (*debug.BuildInfo, bool), tfs fs.FS) string {
	t.Helper()
	dir := newRepo(t)
	o, _ := opts(dir, rt)
	o.BuildInfo, o.Templates = build, tfs
	mustInit(t, o)
	commitAll(t, dir, "waggle init")
	return dir
}

func upgradeOpts(dir, rt string, build func() (*debug.BuildInfo, bool), apply bool) (UpgradeOptions, *bytes.Buffer) {
	var out bytes.Buffer
	return UpgradeOptions{Dir: dir, Apply: apply, Runtime: rt, Out: &out, GitEnv: gitEnv, BuildInfo: build}, &out
}

func upgrade(t *testing.T, o UpgradeOptions) string {
	t.Helper()
	if err := Upgrade(o); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	return o.Out.(*bytes.Buffer).String()
}

// setPin rewrites the hive pin line and commits.
func setPin(t *testing.T, dir, pin string) {
	t.Helper()
	updated, err := WritePin(get(t, dir, layout.Config), pin)
	if err != nil {
		t.Fatal(err)
	}
	put(t, dir, layout.Config, updated)
	commitAll(t, dir, "pin "+pin)
}

func names(t *testing.T, dir, rel, prefix string) []string {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), prefix) {
			out = append(out, strings.TrimSuffix(strings.TrimPrefix(e.Name(), prefix), ".md"))
		}
	}
	sort.Strings(out)
	return out
}

func wantLines(t *testing.T, out string, lines ...string) {
	t.Helper()
	for _, l := range lines {
		if !strings.Contains(out, l) {
			t.Errorf("output lacks %q:\n%s", l, out)
		}
	}
}

func TestUpgradeRoundTripRemovesDroppedRole(t *testing.T) {
	dir := hive(t, "claude-code", commitBuild(pinA), templatesA(t))
	before, state := project(t, dir), repoState(t, dir)
	oldRole, oldShim := layout.RolesDir+"/oldrole.md", layout.ShimPrefix+"oldrole.md"
	for _, rel := range []string{oldRole, oldShim} {
		if !exists(dir, rel) {
			t.Fatalf("%s missing after init", rel)
		}
	}

	o, _ := upgradeOpts(dir, "claude-code", commitBuild(pinB), false)
	out := upgrade(t, o)
	wantLines(t, out, "[pin]     "+pinA+" -> "+pinB, "[remove]  "+oldRole, "[remove]  "+oldShim,
		"[rewrite] "+layout.Roster, "[same]    "+layout.Devcontainer, "[rewrite] "+layout.GateWorkflow,
		"[shims]   regenerate", "Dry run: nothing changed", upgradeLastLine)
	for _, r := range roleNames {
		wantLines(t, out, "[same]    "+layout.RolesDir+"/"+r+".md")
	}
	// T-036 2.4: the gate workflow names the pin's commit, so it is rewritten with the roster.
	if strings.Count(out, "[rewrite]") != 2 || strings.Contains(out, "[create]") {
		t.Errorf("unexpected rewrite or create:\n%s", out)
	}
	if after := project(t, dir); !reflect.DeepEqual(before, after) {
		t.Error("dry run changed files")
	}
	if got := repoState(t, dir); got != state {
		t.Errorf("dry run changed the repository:\n got %s\nwant %s", got, state)
	}

	o, _ = upgradeOpts(dir, "claude-code", commitBuild(pinB), true)
	upgrade(t, o)
	for _, rel := range []string{oldRole, oldShim} {
		if exists(dir, rel) {
			t.Errorf("%s survived apply", rel)
		}
	}
	cfg := get(t, dir, layout.Config)
	if want := strings.Replace(before[layout.Config], pinA, pinB, 1); cfg != want {
		t.Errorf("config.yaml changed beyond the pin value:\n%s", cfg)
	}
	if !strings.Contains(cfg, "  pin: "+pinB+" # the Waggle build this hive runs") {
		t.Errorf("pin line lost its comment:\n%s", cfg)
	}
	if got := names(t, dir, layout.RolesDir, ""); !reflect.DeepEqual(got, roleNames) {
		t.Errorf("roles = %v; want %v", got, roleNames)
	}
	if got := names(t, dir, filepath.Dir(layout.ShimPrefix), filepath.Base(layout.ShimPrefix)); !reflect.DeepEqual(got, roleNames) {
		t.Errorf("shims = %v; want %v", got, roleNames)
	}
	if st := git(t, dir, "status", "--porcelain=v1"); st != "M "+layout.GateWorkflow+"\n M "+layout.Config+"\n D "+oldRole+"\n M "+layout.Roster {
		t.Errorf("status after apply = %q; want the workflow, the pin edit, the role removal and the roster without it", st)
	}

	commitAll(t, dir, "upgrade")
	o, _ = upgradeOpts(dir, "claude-code", commitBuild(pinB), false)
	out = upgrade(t, o)
	wantLines(t, out, "[pin]     "+pinB+" (unchanged)")
	if strings.Contains(out, "[remove]") || strings.Contains(out, "[rewrite]") {
		t.Errorf("rerun still has work:\n%s", out)
	}
}

func TestUpgradeUnchangedPinIsNoop(t *testing.T) {
	dir := hive(t, "none", fixedBuild, nil)
	before, state := project(t, dir), repoState(t, dir)
	files, err := templateFiles(templateFS)
	if err != nil {
		t.Fatal(err)
	}
	for _, apply := range []bool{false, true} {
		o, _ := upgradeOpts(dir, "none", fixedBuild, apply)
		out := upgrade(t, o)
		for _, f := range files {
			verb := "[same]    "
			if f.seeded {
				verb = "[skip]    "
			}
			wantLines(t, out, verb+f.dst)
		}
		wantLines(t, out, "[pin]     "+testPin+" (unchanged)", "[keep]    refs/waggle/* and "+layout.HiveDir+"/**")
		if strings.Contains(out, "[shims]") {
			t.Errorf("runtime none regenerated shims:\n%s", out)
		}
		if after := project(t, dir); !reflect.DeepEqual(before, after) {
			t.Errorf("apply=%v changed files", apply)
		}
		if got := repoState(t, dir); got != state {
			t.Errorf("apply=%v changed the repository", apply)
		}
	}
}

func TestUpgradeRestoresHandEditedManagedFile(t *testing.T) {
	dir := hive(t, "none", fixedBuild, nil)
	want := get(t, dir, layout.Roster)
	put(t, dir, layout.Roster, want+"x")
	commitAll(t, dir, "hand edit")

	o, _ := upgradeOpts(dir, "none", fixedBuild, false)
	o.Diff = true
	out := upgrade(t, o)
	wantLines(t, out, "[rewrite] "+layout.Roster, "\n--- a/"+layout.Roster+"\n", "\n+++ b/"+layout.Roster+"\n", "\n-x\n")
	if got := get(t, dir, layout.Roster); got != want+"x" {
		t.Error("dry run wrote roster.json")
	}
	o, _ = upgradeOpts(dir, "none", fixedBuild, true)
	upgrade(t, o)
	if got := get(t, dir, layout.Roster); got != want {
		t.Errorf("roster.json not restored:\n%s", got)
	}
}

func TestUpgradeSeededFilesKeepTheirBytes(t *testing.T) {
	dir := hive(t, "none", commitBuild(pinA), nil)
	edited := get(t, dir, layout.WaggleMD) + "\n## 4. Mine\n"
	put(t, dir, layout.WaggleMD, edited)
	put(t, dir, layout.Local, "runtime: none\nmine: yes\n")
	commitAll(t, dir, "seeded edits")
	cfgBefore := get(t, dir, layout.Config)

	o, _ := upgradeOpts(dir, "none", commitBuild(pinB), true)
	out := upgrade(t, o)
	wantLines(t, out, "[skip]    "+layout.Config+" (seeded)", "[skip]    "+layout.WaggleMD+" (seeded)", "[skip]    "+layout.Local+" (seeded)")
	if got := get(t, dir, layout.WaggleMD); got != edited {
		t.Error("WAGGLE.md edit lost")
	}
	if got := get(t, dir, layout.Local); got != "runtime: none\nmine: yes\n" {
		t.Error("local.yaml changed")
	}
	if got := get(t, dir, layout.Config); got != strings.Replace(cfgBefore, pinA, pinB, 1) {
		t.Errorf("config.yaml changed beyond the pin:\n%s", got)
	}
	if strings.Contains(out, "[note]") {
		t.Errorf("unexpected schema note:\n%s", out)
	}
}

func TestUpgradeNotesMissingConfigKeys(t *testing.T) {
	dir := hive(t, "none", fixedBuild, nil)
	cfg := get(t, dir, layout.Config)
	i := strings.Index(cfg, "\nleases:")
	put(t, dir, layout.Config, cfg[:i+1])
	commitAll(t, dir, "drop leases")
	o, _ := upgradeOpts(dir, "none", fixedBuild, false)
	out := upgrade(t, o)
	wantLines(t, out, `[note]    `+layout.Config+` lacks top-level key "leases"`)
	if strings.Count(out, "[note]") != 1 {
		t.Errorf("want one note:\n%s", out)
	}
}

func TestUpgradeRefusesWithoutWriting(t *testing.T) {
	type tc struct {
		setup   func(t *testing.T) (dir, runDir string)
		build   func() (*debug.BuildInfo, bool)
		wantErr error
		wantMsg string
	}
	fresh := func(t *testing.T) (string, string) {
		dir := hive(t, "none", commitBuild(pinA), nil)
		return dir, dir
	}
	withConfig := func(edit func(string) string) func(t *testing.T) (string, string) {
		return func(t *testing.T) (string, string) {
			dir, _ := fresh(t)
			put(t, dir, layout.Config, edit(get(t, dir, layout.Config)))
			commitAll(t, dir, "config edit")
			return dir, dir
		}
	}
	cases := map[string]tc{
		"subdirectory": {setup: func(t *testing.T) (string, string) {
			dir, _ := fresh(t)
			put(t, dir, "src/x.go", "package x\n")
			commitAll(t, dir, "src")
			return dir, filepath.Join(dir, "src")
		}, wantMsg: "work tree root"},
		"untracked file": {setup: func(t *testing.T) (string, string) {
			dir, _ := fresh(t)
			put(t, dir, "stray.txt", "x\n")
			return dir, dir
		}, wantMsg: "first dirty path: stray.txt"},
		"modified file": {setup: func(t *testing.T) (string, string) {
			dir, _ := fresh(t)
			put(t, dir, "CLAUDE.md", "edited\n")
			return dir, dir
		}, wantMsg: "first dirty path: CLAUDE.md"},
		"legacy layout": {setup: func(t *testing.T) (string, string) {
			dir := newRepo(t)
			put(t, dir, ".waggle/config.yaml", "x\n")
			commitAll(t, dir, "legacy")
			return dir, dir
		}, wantErr: ErrLegacyLayout},
		"not a hive": {setup: func(t *testing.T) (string, string) {
			dir := newRepo(t)
			commitAll(t, dir, "empty")
			return dir, dir
		}, wantMsg: "run waggle init first"},
		"socket present": {setup: func(t *testing.T) (string, string) {
			dir, _ := fresh(t)
			put(t, dir, layout.Socket, "")
			return dir, dir
		}, wantMsg: "stop waggled first"},
		"bad markers": {setup: func(t *testing.T) (string, string) {
			dir, _ := fresh(t)
			put(t, dir, "AGENTS.md", "<!-- waggle:end -->\nx\n")
			commitAll(t, dir, "broken markers")
			return dir, dir
		}, wantErr: ErrMarkers},
		"unpinned build": {setup: fresh, build: unpinnedBuild, wantMsg: "clean, stamped checkout"},
		"no pin line": {setup: withConfig(func(c string) string {
			return strings.Replace(c, "  pin: ", "  pinned: ", 1)
		}), wantMsg: "no pin: line under waggle:"},
		"two pin lines": {setup: withConfig(func(c string) string {
			return strings.Replace(c, "waggle:\n", "waggle:\n  pin: "+pinB+"\n", 1)
		}), wantMsg: "2 pin: lines under waggle:"},
		"short hash": {setup: withConfig(func(c string) string {
			return strings.Replace(c, pinA, pinA[:7], 1)
		}), wantMsg: "set waggle.pin to the full 40-hex commit"},
		"trailing text": {setup: withConfig(func(c string) string {
			return strings.Replace(c, pinA, "v1.0.0 beta", 1)
		}), wantMsg: `pin "v1.0.0 beta" is not a full 40-hex commit`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir, runDir := c.setup(t)
			build := c.build
			if build == nil {
				build = commitBuild(pinB)
			}
			before, state := project(t, dir), repoState(t, dir)
			o, _ := upgradeOpts(runDir, "none", build, true)
			err := Upgrade(o)
			if !errors.Is(err, ErrRefused) {
				t.Fatalf("err = %v; want ErrRefused", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Errorf("err = %v; want %v", err, c.wantErr)
			}
			if c.wantMsg != "" && !strings.Contains(err.Error(), c.wantMsg) {
				t.Errorf("err = %v; want substring %q", err, c.wantMsg)
			}
			if after := project(t, dir); !reflect.DeepEqual(before, after) {
				t.Error("refused upgrade changed files")
			}
			if got := repoState(t, dir); got != state {
				t.Errorf("refused upgrade changed the repository:\n got %s\nwant %s", got, state)
			}
		})
	}
}

func TestUpgradeRefusesSymlinks(t *testing.T) {
	for _, rel := range []string{layout.Roster, layout.RolesDir, layout.Config} {
		t.Run(rel, func(t *testing.T) {
			dir := hive(t, "claude-code", commitBuild(pinA), nil)
			outside := filepath.Join(t.TempDir(), "target")
			inside := filepath.Join(dir, filepath.FromSlash(rel))
			if err := os.Rename(inside, outside); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, inside); err != nil {
				t.Fatal(err)
			}
			commitAll(t, dir, "symlink "+rel)
			state, target := repoState(t, dir), outsideState(t, outside)
			o, _ := upgradeOpts(dir, "claude-code", commitBuild(pinB), true)
			err := Upgrade(o)
			if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), rel+" is a symlink") {
				t.Fatalf("err = %v; want ErrRefused naming %s", err, rel)
			}
			if got := repoState(t, dir); got != state {
				t.Errorf("refused upgrade changed the repository:\n got %s\nwant %s", got, state)
			}
			if got := outsideState(t, outside); !reflect.DeepEqual(got, target) {
				t.Errorf("refused upgrade wrote through the symlink:\n got %v\nwant %v", got, target)
			}
		})
	}
	t.Run("hand-added link inside roles", func(t *testing.T) {
		dir := hive(t, "none", commitBuild(pinA), nil)
		outside := filepath.Join(t.TempDir(), "precious.md")
		put(t, filepath.Dir(outside), "precious.md", "keep\n")
		link := filepath.Join(dir, filepath.FromSlash(layout.RolesDir), "evil.md")
		if err := os.Symlink(outside, link); err != nil {
			t.Fatal(err)
		}
		commitAll(t, dir, "hand-added link")
		o, _ := upgradeOpts(dir, "none", commitBuild(pinB), true)
		wantLines(t, upgrade(t, o), "[remove]  "+layout.RolesDir+"/evil.md")
		if _, err := os.Lstat(link); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("link still present: %v", err)
		}
		if got, err := os.ReadFile(outside); err != nil || string(got) != "keep\n" {
			t.Errorf("link target = %q, %v; want untouched", got, err)
		}
	})
}

// outsideState maps every file under p (or p itself) to its content.
func outsideState(t *testing.T, p string) map[string]string {
	t.Helper()
	m := map[string]string{}
	err := filepath.WalkDir(p, func(f string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(f)
		m[f] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestUpgradeDowngradeRule(t *testing.T) {
	cases := []struct {
		from, to string
		allow    bool
		refused  bool
	}{
		{"v0.2.0", "v0.1.0", false, true},
		{"v0.2.0", "v0.1.0", true, false},
		{"v0.1.0", "v0.2.0", false, false},
		{"v0.1.0-rc1", "v0.1.0", false, false},
		{"v0.1.0", "v0.1.0-rc1", false, true},
		{"v0.1.0", "v0.1.0", false, false},
		{pinA, "v0.1.0", false, false},
		{"v9.0.0", pinA, false, false},
		{Unpinned, "v0.1.0", false, false},
	}
	for _, c := range cases {
		dir := hive(t, "none", tagBuild("v0.0.1"), nil)
		setPin(t, dir, c.from)
		build := tagBuild(c.to)
		if c.to == pinA {
			build = commitBuild(pinA)
		}
		o, _ := upgradeOpts(dir, "none", build, false)
		o.AllowDowngrade = c.allow
		err := Upgrade(o)
		if got := errors.Is(err, ErrRefused); got != c.refused || (err != nil && !got) {
			t.Errorf("%s -> %s allow=%v: err = %v; want refused %v", c.from, c.to, c.allow, err, c.refused)
		}
		if c.refused && !strings.Contains(err.Error(), "--allow-downgrade") {
			t.Errorf("err = %v; want the flag hint", err)
		}
	}
}

func TestCompareTagsAndValidPin(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"v1.0.0", "v1.0.0", 0}, {"v1.0.0", "v0.9.9", 1}, {"v0.10.0", "v0.9.0", 1},
		{"v1.0.0-rc1", "v1.0.0", -1}, {"v1.0.0-rc1", "v1.0.0-rc2", -1}, {"v1.0.1", "v1.0.0", 1},
	} {
		if got := compareTags(c.a, c.b); got != c.want {
			t.Errorf("compareTags(%s, %s) = %d; want %d", c.a, c.b, got, c.want)
		}
	}
	for pin, want := range map[string]bool{
		pinA: true, "v0.1.0": true, "v0.1.0-rc.1": true, Unpinned: true,
		pinA[:7]: false, "0.1.0": false, "v1": false, "": false, strings.ToUpper(pinA): false,
	} {
		if got := ValidPin(pin); got != want {
			t.Errorf("ValidPin(%q) = %v; want %v", pin, got, want)
		}
	}
}

func TestUpgradeNeverRemovesProjectContent(t *testing.T) {
	dir := hive(t, "claude-code", commitBuild(pinA), nil)
	workflow := "name: mine\non: push\n"
	put(t, dir, layout.GateWorkflow, workflow)
	put(t, dir, ".claude/agents/foo.md", "---\nname: foo\n---\n")
	put(t, dir, layout.ShimPrefix+"foo.md", "---\nname: waggle-foo\n---\n")
	put(t, dir, layout.HiveDir+"/notes.md", "keep\n")
	put(t, dir, "CLAUDE.md", "# My project\n\n"+get(t, dir, "CLAUDE.md")+"\nMore.\n")
	put(t, dir, ".gitignore", get(t, dir, ".gitignore")+"node_modules/\n")
	commitAll(t, dir, "project content")
	before := project(t, dir)

	o, _ := upgradeOpts(dir, "claude-code", commitBuild(pinB), true)
	out := upgrade(t, o)
	wantLines(t, out, "[skip]    "+layout.GateWorkflow+" (warning: no \"# waggle:managed\" line, project file)",
		"[skip]    "+layout.ShimPrefix+"foo.md (not generated by waggle)",
		"[same]    .gitignore (waggle block)", "[same]    CLAUDE.md (waggle block)")
	if strings.Contains(out, "[remove]") {
		t.Errorf("unexpected removal:\n%s", out)
	}
	after := project(t, dir)
	for _, rel := range []string{layout.GateWorkflow, ".claude/agents/foo.md", layout.ShimPrefix + "foo.md",
		layout.HiveDir + "/notes.md", "CLAUDE.md", ".gitignore", layout.WaggleMD} {
		if after[rel] != before[rel] {
			t.Errorf("%s changed:\n%s", rel, after[rel])
		}
	}
	if !strings.Contains(get(t, dir, layout.Config), pinB) {
		t.Error("pin not moved")
	}
}

func TestUpgradeMarkedBlocks(t *testing.T) {
	dir := hive(t, "none", fixedBuild, nil)
	put(t, dir, ".gitignore", GitignoreStart+"\nold\n"+GitignoreEnd+"\n")
	commitAll(t, dir, "stale block")
	o, _ := upgradeOpts(dir, "none", fixedBuild, true)
	out := upgrade(t, o)
	wantLines(t, out, "[edit]    .gitignore (waggle block)", "[same]    AGENTS.md (waggle block)")
	if got := get(t, dir, ".gitignore"); got != GitignoreStart+"\n"+gitignoreBody+GitignoreEnd+"\n" {
		t.Errorf(".gitignore = %q", got)
	}
	if exists(dir, "CLAUDE.md") {
		t.Error("an AGENTS.md hive gained a CLAUDE.md")
	}
}

func TestUpgradeDecisionInput(t *testing.T) {
	dir := hive(t, "none", commitBuild(pinA), nil)
	o, _ := upgradeOpts(dir, "none", commitBuild(pinB), false)
	out := upgrade(t, o)
	_, rest, ok := strings.Cut(out, "--- waggle decide input (save as upgrade-decision.json) ---\n")
	body, _, ok2 := strings.Cut(rest, "--- end ---\n")
	if !ok || !ok2 {
		t.Fatalf("decision block missing:\n%s", out)
	}
	var in struct {
		Kind, Title, Question, Recommendation, Choice, Rationale string
		Options, Tickets, Supersedes                             []string
	}
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		t.Fatalf("decode: %v\n%s", err, body)
	}
	if in.Kind != "technical" || in.Title != "Upgrade hive pin to "+pinB || len(in.Options) != 2 ||
		in.Tickets[0] != "T-016" || in.Supersedes == nil || len(in.Supersedes) != 0 {
		t.Errorf("input = %+v", in)
	}
	for name, s := range map[string]struct {
		v   string
		max int
	}{"title": {in.Title, 100}, "question": {in.Question, 1000}, "recommendation": {in.Recommendation, 1000},
		"choice": {in.Choice, 1000}, "rationale": {in.Rationale, 2000}, "option0": {in.Options[0], 500}, "option1": {in.Options[1], 500}} {
		if s.v == "" || len(s.v) > s.max || strings.ContainsAny(s.v, "\n\r\t") || strings.TrimSpace(s.v) != s.v {
			t.Errorf("%s = %q violates T-006 4.2", name, s.v)
		}
	}
	wantLines(t, out, "Run: waggle decide --file upgrade-decision.json")
}

func TestReadWritePin(t *testing.T) {
	dir := t.TempDir()
	content := "version: 1\n\nwaggle:\n  # comment\n  pin: \"" + pinA + "\"  # keep me\n\nhive:\n  pin: v9.9.9\n"
	put(t, dir, "c.yaml", content)
	pin, err := ReadPin(filepath.Join(dir, "c.yaml"))
	if err != nil || pin != pinA {
		t.Fatalf("ReadPin = %q, %v", pin, err)
	}
	got, err := WritePin(content, "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Replace(content, "\""+pinA+"\"", "v0.1.0", 1); got != want {
		t.Errorf("WritePin = %q; want %q", got, want)
	}
	if _, err := WritePin(content, "abc"); err == nil {
		t.Error("WritePin accepted a short hash")
	}
	if _, err := ReadPin(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("ReadPin of a missing file succeeded")
	}
	for _, v := range []string{"v1.0.0 beta", "\"" + pinA + "\" beta", pinA + " junk", "v1.0.0#x", "\"v1.0.0 beta\""} {
		put(t, dir, "bad.yaml", "waggle:\n  pin: "+v+"  # comment\n")
		if pin, err := ReadPin(filepath.Join(dir, "bad.yaml")); err == nil || !strings.Contains(err.Error(), "line 2") {
			t.Errorf("ReadPin(pin: %s) = %q, %v; want refusal naming line 2", v, pin, err)
		}
	}
	put(t, dir, "ok.yaml", "waggle:\n  pin: "+pinB+"\t \n")
	if pin, err := ReadPin(filepath.Join(dir, "ok.yaml")); err != nil || pin != pinB {
		t.Errorf("ReadPin with trailing whitespace = %q, %v", pin, err)
	}
}
