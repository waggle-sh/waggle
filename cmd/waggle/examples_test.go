// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestGateWalkthrough builds the binary outside the clone, runs
// examples/gate-walkthrough/run.sh and checks the two gate exit codes it prints.
func TestGateWalkthrough(t *testing.T) {
	for _, tool := range []string{"git", "sh"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not found", tool)
		}
	}
	bin := filepath.Join(t.TempDir(), "waggle")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	// init --gate-only pins the Action to a clean commit stamp or a release
	// tag, so a build from a modified tree cannot run the walkthrough.
	info, err := exec.Command("go", "version", "-m", bin).CombinedOutput()
	if err != nil {
		t.Fatalf("go version -m: %v\n%s", err, info)
	}
	if !strings.Contains(string(info), "vcs.modified=false") {
		t.Skip("build has no clean commit stamp (modified tree or no git metadata); init --gate-only needs one")
	}

	script := filepath.Join("..", "..", "examples", "gate-walkthrough", "run.sh")
	out, err := exec.Command("sh", script, bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run.sh: %v\n%s", err, out)
	}
	got := regexp.MustCompile(`(?m)^exit (\d+)$`).FindAllStringSubmatch(string(out), -1)
	if len(got) != 2 || got[0][1] != "0" || got[1][1] != "1" {
		t.Fatalf("want exit 0 then exit 1, got %v\n%s", got, out)
	}
	if !strings.Contains(string(out), "leases: FAIL: README.md: T-001: path is outside the claim directories") {
		t.Errorf("missing the failing lease line\n%s", out)
	}
}
