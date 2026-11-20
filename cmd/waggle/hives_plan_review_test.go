// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlansDirInsideHiveIsRefused(t *testing.T) {
	requireGit(t)
	fixClock(t)
	e := newPlanEnv(t)
	before := e.snapshots(t)
	inside := filepath.Join(e.lib, ".waggle", "plans")

	// --plans-dir, for a write and for reads.
	for _, c := range [][]string{{"new", "leak"}, {"list"}, {"show", "leak"}} {
		full := append([]string{"plan"}, c...)
		full = append(full, "--registry", e.registry, "--plans-dir", inside)
		if code, _, errs := runHivesT(t, full...); code != 1 || !strings.Contains(errs, "inside hive") {
			t.Errorf("--plans-dir %v: exit %d: %s", c, code, errs)
		}
	}

	// WAGGLE_HIVES_PLANS.
	var out, errb bytes.Buffer
	get := func(k string) string {
		if k == plansEnv {
			return inside
		}
		return ""
	}
	if code := runHives([]string{"plan", "new", "leak", "--registry", e.registry}, &out, &errb, get); code != 1 || !strings.Contains(errb.String(), "inside hive") {
		t.Errorf("env plans dir: exit %d: %s", code, errb.String())
	}
	if code := runHives([]string{"plan", "list", "--registry", e.registry}, &out, &errb, get); code != 1 {
		t.Errorf("env plans dir, list: exit %d", code)
	}

	// A symlink to a hive directory, and a path below a symlink into a hive.
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(e.lib, link); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{link, filepath.Join(link, "deeper", "plans")} {
		full := []string{"plan", "new", "leak", "--registry", e.registry, "--plans-dir", dir}
		if code, _, _ := runHivesT(t, full...); code != 1 {
			t.Errorf("plans dir %s: exit %d, want 1", dir, code)
		}
	}
	for i, s := range e.snapshots(t) {
		if s != before[i] {
			t.Errorf("hive %d changed", i)
		}
	}
}

func TestDefaultPlansDirInsideRepoIsRefused(t *testing.T) {
	requireGit(t)
	fixClock(t)
	e := newPlanEnv(t)
	// A registry file inside a hive makes the default plans/ dir sit inside it.
	reg := filepath.Join(e.cli, ".waggle", "hives.json")
	data, err := os.ReadFile(e.registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reg, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runHivesT(t, "plan", "new", "leak", "--registry", reg); code != 1 || !strings.Contains(errs, "inside hive") {
		t.Errorf("exit %d: %s", code, errs)
	}
	if _, err := os.Stat(filepath.Join(e.cli, ".waggle", "plans")); err == nil {
		t.Error("plans dir was created inside the hive")
	}
}

func TestPlansDirInUnregisteredGitWorkTreeIsRefused(t *testing.T) {
	requireGit(t)
	fixClock(t)
	e := newPlanEnv(t)
	other := newHive(t) // a git work tree that is not in the registry
	dir := filepath.Join(other, "plans")
	code, _, errs := runHivesT(t, "plan", "new", "leak", "--registry", e.registry, "--plans-dir", dir)
	if code != 1 || !strings.Contains(errs, "git work tree") {
		t.Errorf("exit %d: %s", code, errs)
	}
}

func TestPlansDirSymlinkAndModeAreRefused(t *testing.T) {
	requireGit(t)
	fixClock(t)
	e := newPlanEnv(t)
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][]string{{"new", "p"}, {"list"}} {
		full := append([]string{"plan"}, c...)
		full = append(full, "--registry", e.registry, "--plans-dir", link)
		if code, _, errs := runHivesT(t, full...); code != 1 || !strings.Contains(errs, "symlink") {
			t.Errorf("symlinked plans dir %v: exit %d: %s", c, code, errs)
		}
	}
	open := filepath.Join(base, "open")
	if err := os.Mkdir(open, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(open, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][]string{{"new", "p"}, {"list"}} {
		full := append([]string{"plan"}, c...)
		full = append(full, "--registry", e.registry, "--plans-dir", open)
		if code, _, errs := runHivesT(t, full...); code != 1 || !strings.Contains(errs, "mode 0755") {
			t.Errorf("open plans dir %v: exit %d: %s", c, code, errs)
		}
	}
}

func TestPlanSymlinkedPlanFileIsRefused(t *testing.T) {
	requireGit(t)
	fixClock(t)
	e := newPlanEnv(t)
	e.mustRun(t, "new", "feat")
	target := filepath.Join(t.TempDir(), "elsewhere.json")
	if err := os.WriteFile(target, RenderPlanFile(Plan{Version: planVersion, Name: "feat", Steps: []PlanStep{}}), 0o600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(e.plans, "feat.json")
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, file); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][]string{{"show", "feat"}, {"check", "feat"}, {"add", "feat", "--hive", "lib", "--task", "T-020"}} {
		if code, _, errs := e.run(t, c...); code != 1 || !strings.Contains(errs, "symlink") {
			t.Errorf("%v: exit %d: %s", c, code, errs)
		}
	}
}

func TestPlanOverSizeAndTooManySteps(t *testing.T) {
	requireGit(t)
	fixClock(t)
	e := newPlanEnv(t)
	e.mustRun(t, "new", "feat")
	file := filepath.Join(e.plans, "feat.json")

	big := `{"version":1,"name":"feat","steps":[],"pad":"` + strings.Repeat("x", maxPlanSize) + `"}`
	if err := os.WriteFile(file, []byte(big), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := e.run(t, "show", "feat"); code != 1 {
		t.Errorf("over size: exit %d: %s", code, errs)
	}

	var steps []string
	for i := 1; i <= maxSteps+1; i++ {
		steps = append(steps, fmt.Sprintf(`{"id":"s%d","hive":"lib","title":"t%d","depends_on":[]}`, i, i))
	}
	many := `{"version":1,"name":"feat","steps":[` + strings.Join(steps, ",") + `]}`
	if err := os.WriteFile(file, []byte(many), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := e.run(t, "show", "feat"); code != 1 || !strings.Contains(errs, "limit") {
		t.Errorf("too many steps: exit %d: %s", code, errs)
	}

	// add refuses the step after the last allowed one.
	many = `{"version":1,"name":"feat","steps":[` + strings.Join(steps[:maxSteps], ",") + `]}`
	if err := os.WriteFile(file, []byte(many), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := e.run(t, "add", "feat", "--hive", "lib", "--title", "one more"); code != 1 || !strings.Contains(errs, "at most") {
		t.Errorf("add past the limit: exit %d: %s", code, errs)
	}
}

func TestPlanTitleNewlinesAreFlattened(t *testing.T) {
	requireGit(t)
	fixClock(t)
	e := newPlanEnv(t)
	e.mustRun(t, "new", "feat")
	e.mustRun(t, "add", "feat", "--hive", "lib", "--title", "line one\nline two\r\n# heading")
	var p Plan
	if err := json.Unmarshal([]byte(e.mustRun(t, "show", "feat", "--json")), &p); err != nil {
		t.Fatal(err)
	}
	if got := p.Steps[0].Title; strings.ContainsAny(got, "\r\n") || got != flatten(got) || got == "" {
		t.Errorf("title not flattened: %q", got)
	}
	// A plan file holding a multi-line title is refused.
	file := filepath.Join(e.plans, "feat.json")
	body := `{"version":1,"name":"feat","steps":[{"id":"s1","hive":"lib","title":"a\nb","depends_on":[]}]}`
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := e.run(t, "show", "feat"); code != 1 {
		t.Errorf("multi-line title in file: exit %d", code)
	}
}

func TestPlanDependsOnTrimsSpaces(t *testing.T) {
	requireGit(t)
	fixClock(t)
	e := newPlanEnv(t)
	e.mustRun(t, "new", "feat")
	e.mustRun(t, "add", "feat", "--hive", "lib", "--task", "T-020")
	e.mustRun(t, "add", "feat", "--hive", "lib", "--title", "b")
	e.mustRun(t, "add", "feat", "--hive", "cli", "--title", "c", "--depends-on", "s1, s2")
	var p Plan
	if err := json.Unmarshal([]byte(e.mustRun(t, "show", "feat", "--json")), &p); err != nil {
		t.Fatal(err)
	}
	if d := p.Steps[2].DependsOn; len(d) != 2 || d[0] != "s1" || d[1] != "s2" {
		t.Errorf("depends_on: %v", d)
	}
}

func TestPlanRmRemovesCorruptPlan(t *testing.T) {
	requireGit(t)
	fixClock(t)
	e := newPlanEnv(t)
	e.mustRun(t, "new", "feat")
	file := filepath.Join(e.plans, "feat.json")
	if err := os.WriteFile(file, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := e.run(t, "show", "feat"); code != 1 {
		t.Fatalf("corrupt plan was read: exit %d", code)
	}
	if code, _, _ := e.run(t, "rm", "feat", "--step", "s1"); code != 1 {
		t.Errorf("rm --step on a corrupt plan: exit %d, want 1", code)
	}
	e.mustRun(t, "rm", "feat")
	if _, err := os.Lstat(file); err == nil {
		t.Error("corrupt plan file is still there")
	}
	if code, _, _ := e.run(t, "rm", "feat"); code != 1 {
		t.Errorf("rm of a missing plan: exit %d, want 1", code)
	}
}

func TestPlanCheckMissingAndDroppedDependencies(t *testing.T) {
	requireGit(t)
	fixClock(t)
	e := newPlanEnv(t)
	for _, f := range []struct{ id, status string }{{"T-041", "Cancelled"}, {"T-042", "Rejected"}, {"T-043", "Dropped"}} {
		if err := os.WriteFile(filepath.Join(e.lib, ".waggle/hive/tasks/"+f.id+".md"), []byte(brief(f.id, "gone", f.status, "")), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e.mustRun(t, "new", "feat")
	e.mustRun(t, "add", "feat", "--hive", "lib", "--task", "T-099")                       // s1 missing
	e.mustRun(t, "add", "feat", "--hive", "lib", "--task", "T-041")                       // s2 cancelled
	e.mustRun(t, "add", "feat", "--hive", "lib", "--task", "T-042")                       // s3 rejected
	e.mustRun(t, "add", "feat", "--hive", "lib", "--task", "T-043")                       // s4 dropped
	e.mustRun(t, "add", "feat", "--hive", "cli", "--task", "T-030", "--depends-on", "s1") // s5 claimed over a missing dep
	e.mustRun(t, "add", "feat", "--hive", "cli", "--title", "after", "--depends-on", "s2, s3, s4")

	c := checkJSON(t, e, "feat")
	want := []string{stateMissing, stateDropped, stateDropped, stateDropped, stateClaimed, stateDraft}
	for i, s := range c.Steps {
		if s.State != want[i] {
			t.Errorf("%s: state %s, want %s", s.ID, s.State, want[i])
		}
	}
	if len(c.Steps[4].Unverified) != 1 || c.Steps[4].Unverified[0] != "s1" {
		t.Errorf("s5 must list the missing dependency as unverified: %+v", c.Steps[4])
	}
	if d := c.Steps[5].Dropped; len(d) != 3 {
		t.Errorf("s6 must name its dropped dependencies: %+v", c.Steps[5])
	}
	if len(c.Violations) != 0 {
		t.Errorf("violations: %+v", c.Violations)
	}
	text := e.mustRun(t, "check", "feat")
	for _, w := range []string{"[missing]", "[dropped]", "(cannot read s1)", "(depends on s2, s3, s4, which the hive dropped)"} {
		if !strings.Contains(text, w) {
			t.Errorf("text lacks %q:\n%s", w, text)
		}
	}
}

func TestPlanCheckHiveTitleIsSeparateFromKeeperTitle(t *testing.T) {
	requireGit(t)
	fixClock(t)
	e := newPlanEnv(t)
	e.mustRun(t, "new", "feat")
	e.mustRun(t, "add", "feat", "--hive", "lib", "--task", "T-020", "--title", "my words")
	e.mustRun(t, "add", "feat", "--hive", "lib", "--task", "T-022")
	c := checkJSON(t, e, "feat")
	if s := c.Steps[0]; s.Title != "my words" || s.HiveTitle != "" {
		t.Errorf("s1: %+v", s)
	}
	if s := c.Steps[1]; s.Title != "" || s.HiveTitle != "later library change" {
		t.Errorf("s2: %+v", s)
	}
}

func TestPlanRemoteOnlyEntryIsStale(t *testing.T) {
	requireGit(t)
	fixClock(t)
	e := newPlanEnv(t)
	reg := fmt.Sprintf(`{"hives":[{"name":"lib","path":%q,"level":"plan"},{"name":"far","repo":"owner/far","level":"plan"}]}`, e.lib)
	if err := os.WriteFile(e.registry, []byte(reg), 0o600); err != nil {
		t.Fatal(err)
	}
	e.mustRun(t, "new", "feat")
	e.mustRun(t, "add", "feat", "--hive", "far", "--task", "T-001")
	e.mustRun(t, "add", "feat", "--hive", "lib", "--task", "T-020", "--depends-on", "s1")
	c := checkJSON(t, e, "feat")
	if c.Steps[0].State != stateStale || !strings.HasPrefix(c.Steps[0].Reason, "unreachable") {
		t.Errorf("s1: %+v", c.Steps[0])
	}
	if len(c.Steps[1].Unverified) != 1 || len(c.Violations) != 0 {
		t.Errorf("s2: %+v / %+v", c.Steps[1], c.Violations)
	}
}
