// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
)

// B4: a hive whose queen.md predates the delete-card rule gets it from waggle
// upgrade, listed as a managed rewrite of the role file.
func TestUpgradePlansQueenDeleteCardRule(t *testing.T) {
	dir := hive(t, "claude-code", commitBuild(pinA), templatesA(t))
	rel := layout.RolesDir + "/queen.md"
	var kept []string
	for _, l := range strings.Split(get(t, dir, rel), "\n") {
		if !strings.Contains(l, "force-release") {
			kept = append(kept, l)
		}
	}
	old := strings.Join(kept, "\n")
	if strings.Contains(old, "`delete` card") {
		t.Fatal("test setup: delete-card line still in the old queen.md")
	}
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, dir, "old queen")

	o, _ := upgradeOpts(dir, "claude-code", commitBuild(pinA), false)
	out := upgrade(t, o)
	wantLines(t, out, "[rewrite] "+rel, "Dry run: nothing changed")
	if got := get(t, dir, rel); got != old {
		t.Error("dry run changed queen.md")
	}
}

// B4: the queen template other hives get carries the force-release delete-card rule.
func TestQueenTemplateForceReleaseCardRule(t *testing.T) {
	data, err := templateFS.ReadFile("templates/roles/queen.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"`waggle force-release`", "`delete` card", "Never run its execute step", "from their own shell"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("queen template lacks %q", want)
		}
	}
}
