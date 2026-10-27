// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/runtime"
)

var bashTick = regexp.MustCompile("`([^`]*)`")

// bashLineCommands returns the backticked commands of the one Bash: line.
func bashLineCommands(t *testing.T, lines []string) (cmds []string) {
	t.Helper()
	var found []string
	for _, l := range lines {
		if strings.HasPrefix(l, "Bash: ") {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want one Bash: line, got %q", found)
	}
	for _, m := range bashTick.FindAllStringSubmatch(found[0], -1) {
		cmds = append(cmds, m[1])
	}
	return cmds
}

func TestBashLineNamesAllowedCommands(t *testing.T) {
	hive := []string{"go build ./...", "go vet ./...", "go test -race ./..."}
	lease := comb.Lease{Directories: []string{"internal/x"}, Gate: "full"}
	for _, role := range []string{"builder", "guard"} {
		bash, err := runtime.BashCommands(role, hive)
		if err != nil {
			t.Fatal(err)
		}
		req := Request{Ticket: "T-1", Role: role, Branch: "t-1"}
		lines := TaskLines(req, "abc", lease, bash)
		if role == "guard" {
			lines = guardTaskLines(req, "abc", lease, bash)
		}
		got := bashLineCommands(t, lines)
		if !slices.Equal(got, bash) {
			t.Errorf("%s: Bash: line names %q, want %q", role, got, bash)
		}
		if role == "guard" && (slices.Contains(got, "git add") || slices.Contains(got, "git commit")) {
			t.Errorf("guard Bash: line carries a git write: %q", got)
		}
		if role == "builder" && !slices.Contains(got, "git commit") {
			t.Errorf("builder Bash: line lacks git commit: %q", got)
		}
	}
}

func TestBashLineForms(t *testing.T) {
	line := bashLine([]string{"go test -race ./..."})
	if !strings.Contains(line, "-run 'TestX'") || !strings.Contains(line, "not that Bash is off") {
		t.Errorf("line: %s", line)
	}
	if line := bashLine([]string{"go build ./..."}); strings.Contains(line, "TestX") {
		t.Errorf("go test example without a go test command: %s", line)
	}
	if got := bashLine(nil); !strings.HasPrefix(got, "Bash: no commands are allowed") {
		t.Errorf("empty list: %s", got)
	}
}

// The Bash line sits in the variable half, so the stable prompt does not change.
func TestBashLineStablePromptUnchanged(t *testing.T) {
	in := Inputs{RolePrompt: []byte("role"), WaggleMD: []byte("md"), Brief: []byte("brief")}
	lease := comb.Lease{Directories: []string{"d"}, Gate: "full"}
	req := Request{Ticket: "T-1", Role: "builder", Branch: "t-1"}
	var stables [][]byte
	var variables []string
	for _, cmds := range [][]string{{"go build ./..."}, {"make all", "go test -race ./..."}} {
		bash, err := runtime.BashCommands("builder", cmds)
		if err != nil {
			t.Fatal(err)
		}
		stable, variable := BuildPrompts(in, TaskLines(req, "abc", lease, bash), runtime.ContextSettings{})
		stables = append(stables, stable)
		variables = append(variables, string(variable))
	}
	if string(stables[0]) != string(stables[1]) {
		t.Error("stable prompt differs between tasks with different commands")
	}
	if variables[0] == variables[1] || !strings.Contains(variables[1], "`make all`") {
		t.Error("variable prompt does not carry the commands")
	}
}

func TestTrailerCountWarnsWithoutText(t *testing.T) {
	dir, g := newRepo(t)
	base := sh(t, dir, "rev-parse", "HEAD")
	write(t, dir, "a.txt", "1\n")
	sh(t, dir, "add", "-A")
	sh(t, dir, "commit", "-q", "-m", "T-1: one\n\nbody\n\nCO-AUTHORED-BY: Secret Name <s@example.com>")
	write(t, dir, "a.txt", "2\n")
	sh(t, dir, "commit", "-q", "-a", "-m", "T-1: two\n\nSigned-off-by: t <t@example.com>")
	write(t, dir, "a.txt", "3\n")
	sh(t, dir, "commit", "-q", "-a", "-m", "T-1: three\n\nco-authored-by: Another <a@example.com>")
	ck := &checker{g: g}
	if n := ck.countTrailers(context.Background(), base+"..HEAD"); n != 2 {
		t.Errorf("count %d, want 2", n)
	}
	if n := ck.countTrailers(context.Background(), base+"..HEAD~1"); n != 1 {
		t.Errorf("count %d, want 1", n)
	}
	if n := ck.countTrailers(context.Background(), "HEAD..HEAD"); n != 0 {
		t.Errorf("count %d, want 0", n)
	}
}
