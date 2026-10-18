package scaffold

import (
	"errors"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/runtime"
)

var (
	rosterMinLevel = regexp.MustCompile(`(?m)^\s*"min_level": "[a-z]+",\n`)
	configLevels   = regexp.MustCompile(`(?m)^      # Keeper-assigned.*\n      levels:\n(        .*\n)+`)
)

// preT012Hive is a claude-code hive whose committed roster has no min_level,
// as before T-012. edit rewrites its config.yaml before the commit.
func preT012Hive(t *testing.T, edit func(string) string) string {
	t.Helper()
	dir := hive(t, "claude-code", commitBuild(pinA), nil)
	roster := rosterMinLevel.ReplaceAllString(get(t, dir, layout.Roster), "")
	if strings.Contains(roster, "min_level") {
		t.Fatal("min_level not stripped from the roster")
	}
	put(t, dir, layout.Roster, roster)
	cfg := get(t, dir, layout.Config)
	if edit(cfg) == cfg {
		t.Fatal("config edit did not apply")
	}
	put(t, dir, layout.Config, edit(cfg))
	commitAll(t, dir, "pre-T-012 hive")
	return dir
}

func dropLevels(cfg string) string { return configLevels.ReplaceAllString(cfg, "") }

func shimNames(t *testing.T, dir string) []string {
	t.Helper()
	return names(t, dir, filepath.Dir(layout.ShimPrefix), filepath.Base(layout.ShimPrefix))
}

// Test 16: upgrade resolves the policy from the rendered roster, not the one on disk.
func TestUpgradeWritesShimModelsFromRenderedRoster(t *testing.T) {
	dir := preT012Hive(t, func(cfg string) string { return cfg + "# pre-T-012 hive with levels added\n" })
	for _, r := range shimNames(t, dir) {
		put(t, dir, layout.ShimPrefix+r+".md", "---\n"+runtime.GeneratedMarker+"\nname: "+r+"\n---\nstale shim, no model line\n")
	}
	o, _ := upgradeOpts(dir, "claude-code", commitBuild(pinB), false)
	wantLines(t, upgrade(t, o), "[rewrite] "+layout.Roster, "[shims]   regenerate")
	o, _ = upgradeOpts(dir, "claude-code", commitBuild(pinB), true)
	upgrade(t, o)
	if got := shimNames(t, dir); !reflect.DeepEqual(got, roleNames) {
		t.Fatalf("shims = %v; want %v", got, roleNames)
	}
	for _, r := range roleNames {
		shimModel(t, dir, r)
	}
	if g, b := shimModel(t, dir, "guard"), shimModel(t, dir, "builder"); g == b {
		t.Errorf("guard and builder share model %q", g)
	}
	if s := shimModel(t, dir, "scout"); s != "opus" {
		t.Errorf("scout model = %q; want the frontier model opus", s)
	}
	if !strings.Contains(get(t, dir, layout.Roster), `"min_level"`) {
		t.Error("roster on disk was not rewritten with min_level")
	}
}

// refusedUpgrade checks that the dry run and --apply both refuse, wrapping
// ErrRefused and every sentinel in want, and leave the work tree, the shims
// and the repository as they were. It returns the --apply error.
func refusedUpgrade(t *testing.T, dir string, want ...error) error {
	t.Helper()
	before, state := project(t, dir), repoState(t, dir)
	var last error
	for _, apply := range []bool{false, true} {
		o, _ := upgradeOpts(dir, "claude-code", commitBuild(pinB), apply)
		last = Upgrade(o)
		for _, w := range append(want, ErrRefused) {
			if !errors.Is(last, w) {
				t.Errorf("apply=%v: err = %v; want it to wrap %v", apply, last, w)
			}
		}
	}
	if after := project(t, dir); !reflect.DeepEqual(before, after) {
		t.Error("refused upgrade changed files")
	}
	if got := repoState(t, dir); got != state {
		t.Errorf("refused upgrade changed the repository:\n got %s\nwant %s", got, state)
	}
	if st := git(t, dir, "status", "--porcelain"); st != "" {
		t.Errorf("status = %q; want clean", st)
	}
	if pin, err := ReadPin(filepath.Join(dir, filepath.FromSlash(layout.Config))); err != nil || pin != pinA {
		t.Errorf("pin = %q, %v; want %s", pin, err, pinA)
	}
	return last
}

// Test 17: no levels block refuses before any write, naming the key path.
func TestUpgradeRefusesWithoutLevels(t *testing.T) {
	err := refusedUpgrade(t, preT012Hive(t, dropLevels))
	for _, s := range []string{"runtimes.adapters.claude-code.levels", "T-012 section 3.2", layout.Config} {
		if err == nil || !strings.Contains(err.Error(), s) {
			t.Errorf("err = %v; want it to name %q", err, s)
		}
	}
}

// Test 18: one model for every level leaves the guard no cross model.
func TestUpgradeRefusesNoCrossModel(t *testing.T) {
	dir := preT012Hive(t, func(cfg string) string {
		return strings.NewReplacer("standard: sonnet", "standard: opus", "fast: haiku", "fast: opus").Replace(cfg)
	})
	refusedUpgrade(t, dir, runtime.ErrNoCrossModel)
}

// Test 19: --runtime none loads no policy and leaves the shims alone.
func TestUpgradeRuntimeNoneIgnoresPolicy(t *testing.T) {
	dir := preT012Hive(t, dropLevels)
	shims := map[string]string{}
	for _, r := range shimNames(t, dir) {
		shims[r] = get(t, dir, layout.ShimPrefix+r+".md")
	}
	if len(shims) != len(roleNames) {
		t.Fatalf("init wrote %d shims; want %d", len(shims), len(roleNames))
	}
	o, _ := upgradeOpts(dir, "none", commitBuild(pinB), true)
	if out := upgrade(t, o); strings.Contains(out, "[shims]") {
		t.Errorf("runtime none planned shims:\n%s", out)
	}
	for r, want := range shims {
		if got := get(t, dir, layout.ShimPrefix+r+".md"); got != want {
			t.Errorf("shim %s changed", r)
		}
	}
	if got := shimNames(t, dir); len(got) != len(shims) {
		t.Errorf("shims = %v; want the %d from init", got, len(shims))
	}
}
