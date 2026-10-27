// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func rolesDir(t *testing.T, roles ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, r := range roles {
		writeFile(t, filepath.Join(dir, ".waggle", "control", "roles", r+".md"), "# "+r+"\n")
	}
	return dir
}

func TestMaterializeWritesModelLines(t *testing.T) {
	dir := rolesDir(t, "scout", "queen", "builder", "guard", "forager")
	if err := NewClaudeAdapter(AdapterConfig{}).WithPolicy(testPolicy()).Materialize(dir); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"scout": "opus", "queen": "opus", "builder": "sonnet", "guard": "opus", "forager": "haiku"}
	for role, model := range want {
		got := readFile(t, filepath.Join(dir, ".claude", "agents", "waggle-"+role+".md"))
		head := "---\n" + GeneratedMarker + "\nname: " + role + "\nmodel: " + model + "\ndescription: "
		if !strings.HasPrefix(got, head) {
			t.Errorf("waggle-%s.md starts %q; want %q", role, got[:min(len(got), len(head))], head)
		}
	}
}

func TestMaterializePolicyFailsClosed(t *testing.T) {
	agents := func(dir string) bool {
		_, err := os.Stat(filepath.Join(dir, ".claude", "agents"))
		return err == nil
	}
	// A role file without a roster entry fails before any shim is written.
	dir := rolesDir(t, "builder", "guard", "weaver")
	err := NewClaudeAdapter(AdapterConfig{}).WithPolicy(testPolicy()).Materialize(dir)
	wantErr(t, err, ErrUnknownRole, "weaver")
	if agents(dir) {
		t.Error("shims written despite a resolve error")
	}

	// A guard with no model different from the builder's fails too.
	p := testPolicy()
	p.Models.Primary = map[Level]string{LevelFrontier: "same", LevelStandard: "same", LevelFast: "haiku"}
	dir = rolesDir(t, "builder", "guard")
	wantErr(t, NewClaudeAdapter(AdapterConfig{}).WithPolicy(p).Materialize(dir), ErrNoCrossModel)

	// An invalid policy is refused before resolution.
	p = testPolicy()
	delete(p.Models.Primary, LevelFast)
	wantErr(t, NewClaudeAdapter(AdapterConfig{}).WithPolicy(p).Materialize(rolesDir(t, "builder")), ErrLevelUnmapped)

	// A model string with a line break never reaches the frontmatter.
	p = testPolicy()
	p.Models.Primary[LevelFast] = "haiku\ntools: Bash"
	if err := NewClaudeAdapter(AdapterConfig{}).WithPolicy(p).Materialize(rolesDir(t, "forager")); err == nil || !strings.Contains(err.Error(), "line break") {
		t.Errorf("err = %v; want a line break refusal", err)
	}
}
