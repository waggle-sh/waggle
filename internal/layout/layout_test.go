// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"path"
	"strings"
	"testing"
)

// allPaths is every path constant, named for failure messages.
var allPaths = map[string]string{
	"Root": Root, "ControlDir": ControlDir, "HiveDir": HiveDir, "Worktrees": Worktrees,
	"Config": Config, "Roster": Roster, "RolesDir": RolesDir, "WaggleMD": WaggleMD,
	"Devcontainer": Devcontainer, "Local": Local, "Socket": Socket, "Handoffs": Handoffs,
	"Usage": Usage, "TasksDir": TasksDir, "ReviewsDir": ReviewsDir, "MediaDir": MediaDir,
	"Decisions": Decisions, "Workplan": Workplan, "Keepers": Keepers, "Roadmap": Roadmap,
	"SpecsDir": SpecsDir, "GateWorkflow": GateWorkflow, "ShimPrefix": ShimPrefix,
}

// Every value is repo-relative and slash-separated, and is already clean
// apart from the trailing slash of Handoffs.
func TestPathsAreRepoRelativeAndSlashed(t *testing.T) {
	for name, p := range allPaths {
		if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) || strings.Contains(p, "..") {
			t.Errorf("%s = %q: want a repo-relative slash path", name, p)
		}
		if path.Clean(strings.TrimSuffix(p, "/")) != strings.TrimSuffix(p, "/") {
			t.Errorf("%s = %q is not a clean path", name, p)
		}
	}
}

// Everything Waggle owns under .waggle sits in the control or hive tree, apart
// from the worktrees directory and the roots themselves.
func TestWagglePathsLiveUnderTheirTree(t *testing.T) {
	control := []string{Config, Roster, RolesDir, WaggleMD, Devcontainer, Local, Socket, Handoffs, Usage, Keepers}
	hive := []string{TasksDir, ReviewsDir, MediaDir, Decisions, Workplan, Roadmap, SpecsDir}
	for _, p := range control {
		if !strings.HasPrefix(p, ControlDir+"/") {
			t.Errorf("%q is not under %s", p, ControlDir)
		}
	}
	for _, p := range hive {
		if !strings.HasPrefix(p, HiveDir+"/") {
			t.Errorf("%q is not under %s", p, HiveDir)
		}
	}
	for _, p := range []string{ControlDir, HiveDir, Worktrees} {
		if !strings.HasPrefix(p, Root+"/") {
			t.Errorf("%q is not under %s", p, Root)
		}
	}
}

// Managed files are rewritten by init --force and seeded files never are, so
// no path may be both.
func TestManagedAndSeededAreDisjoint(t *testing.T) {
	seeded := map[string]bool{}
	for _, p := range Seeded {
		seeded[p] = true
	}
	for _, p := range Managed {
		if seeded[p] {
			t.Errorf("%q is both Managed and Seeded", p)
		}
	}
}
