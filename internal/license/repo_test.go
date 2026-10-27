// SPDX-License-Identifier: Apache-2.0

package license

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// moduleRoot walks up from the working directory to the directory with go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test directory")
		}
		dir = parent
	}
}

func TestRepoHeaders(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	root := moduleRoot(t)
	top, err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not in a git checkout")
	}
	a, errA := filepath.EvalSymlinks(strings.TrimSpace(string(top)))
	b, errB := filepath.EvalSymlinks(root)
	if errA != nil || errB != nil || a != b {
		t.Skip("module root is not the git top level")
	}
	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var bad []string
	for _, p := range bytes.Split(out, []byte{0}) {
		if len(p) == 0 {
			continue
		}
		name := string(p)
		full := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		content, err := os.ReadFile(full)
		if err != nil {
			t.Fatal(err)
		}
		if err := Check(name, content); err != nil {
			bad = append(bad, err.Error())
		}
	}
	if len(bad) > 0 {
		t.Errorf("%d license offenders:\n%s", len(bad), strings.Join(bad, "\n"))
	}
}
