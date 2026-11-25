// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// driftRepoRoot returns the repo root, found by walking up from the working
// directory to the nearest go.mod.
func driftRepoRoot(t *testing.T) string {
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
			t.Fatal("no go.mod above the test's working directory")
		}
		dir = parent
	}
}

// driftRead reads a repo-relative file.
func driftRead(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// driftHelp runs `waggle <args> -h` in process and returns stdout and stderr
// together, since commands differ in which stream they print help on.
func driftHelp(t *testing.T, args ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	code := runMain(append(append([]string(nil), args...), "-h"), strings.NewReader(""), &out, &errb, func(string) string { return "" })
	if code != 0 {
		t.Fatalf("waggle %s -h exited %d: %s", strings.Join(args, " "), code, errb.String())
	}
	return out.String() + errb.String()
}

// driftUsageLine returns the first line of help that starts with "Usage:".
func driftUsageLine(help string) string {
	for _, l := range strings.Split(help, "\n") {
		if strings.HasPrefix(l, "Usage: ") {
			return l
		}
	}
	return ""
}

// driftSection returns the text from the heading line that starts with
// heading up to the next heading of the same or a higher level.
func driftSection(doc, heading string) string {
	level := len(heading) - len(strings.TrimLeft(heading, "#"))
	lines := strings.Split(doc, "\n")
	var out []string
	in := false
	for _, l := range lines {
		if !in {
			if strings.HasPrefix(l, heading) {
				in = true
				out = append(out, l)
			}
			continue
		}
		if h := len(l) - len(strings.TrimLeft(l, "#")); h > 0 && h <= level && strings.HasPrefix(l[h:], " ") {
			break
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// TestDocsUsageLinesMatchHelp fails when the Usage line of claim, release or
// force-release is not in the README "Writing a claim" block and in cli.md.
func TestDocsUsageLinesMatchHelp(t *testing.T) {
	root := driftRepoRoot(t)
	readme := driftSection(driftRead(t, root, "README.md"), "## Writing a claim")
	if readme == "" {
		t.Fatal(`README.md: no "## Writing a claim" section`)
	}
	cli := driftRead(t, root, "site/content/cli.md")
	for _, cmd := range []string{"claim", "release", "force-release"} {
		usage := driftUsageLine(driftHelp(t, cmd))
		if usage == "" {
			t.Fatalf("waggle %s -h prints no Usage line", cmd)
		}
		if !strings.Contains(readme, usage) {
			t.Errorf("README.md, Writing a claim: missing the line %q", usage)
		}
		if !strings.Contains(cli, strings.TrimPrefix(usage, "Usage: ")) {
			t.Errorf("site/content/cli.md: missing the usage %q", strings.TrimPrefix(usage, "Usage: "))
		}
	}
}

// helpCommand matches an entry of the Commands list in the top-level usage.
var helpCommand = regexp.MustCompile(`^  ([a-z][a-z-]*)\s`)

// TestDocsListEveryCommand fails when a command printed by the top-level usage
// is neither a heading nor a table row in cli.md.
func TestDocsListEveryCommand(t *testing.T) {
	root := driftRepoRoot(t)
	cli := driftRead(t, root, "site/content/cli.md")
	var out, errb bytes.Buffer
	if code := runMain([]string{"-h"}, strings.NewReader(""), &out, &errb, func(string) string { return "" }); code != 0 {
		t.Fatalf("waggle -h exited %d", code)
	}
	_, list, ok := strings.Cut(out.String(), "Commands:\n")
	if !ok {
		t.Fatal("waggle -h has no Commands: list")
	}
	n := 0
	for _, l := range strings.Split(list, "\n") {
		m := helpCommand.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		n++
		heading := regexp.MustCompile("(?m)^#+ .*`waggle " + regexp.QuoteMeta(m[1]) + "`")
		row := regexp.MustCompile("(?m)^\\| `waggle " + regexp.QuoteMeta(m[1]) + "`")
		if !heading.MatchString(cli) && !row.MatchString(cli) {
			t.Errorf("site/content/cli.md: command %q is neither a heading nor a table row", m[1])
		}
	}
	if n < 10 {
		t.Errorf("waggle -h lists %d commands, want at least 10", n)
	}
}

var (
	docFlag     = regexp.MustCompile("--([a-z][a-z0-9-]*)")
	helpFlagDef = regexp.MustCompile(`(?m)^\s+-([a-z][a-z0-9-]*)\b`)
)

// TestDocsFlagsExistInHelp fails when a flag in a nectar, vote or quorum row
// of cli.md is not in that command's -h output.
func TestDocsFlagsExistInHelp(t *testing.T) {
	root := driftRepoRoot(t)
	cli := driftRead(t, root, "site/content/cli.md")
	rows := regexp.MustCompile("(?m)^\\| `waggle (nectar|vote|quorum)\\b[^`]*` \\|.*$")
	helps := map[string]string{}
	n := 0
	for _, m := range rows.FindAllStringSubmatch(cli, -1) {
		n++
		cmd := m[1]
		if _, ok := helps[cmd]; !ok {
			helps[cmd] = driftHelp(t, cmd)
		}
		have := map[string]bool{}
		for _, f := range docFlag.FindAllStringSubmatch(helps[cmd], -1) {
			have[f[1]] = true
		}
		for _, f := range helpFlagDef.FindAllStringSubmatch(helps[cmd], -1) {
			have[f[1]] = true
		}
		row := m[0]
		if len(row) > 60 {
			row = row[:60] + "..."
		}
		for _, f := range docFlag.FindAllStringSubmatch(m[0], -1) {
			if !have[f[1]] {
				t.Errorf("site/content/cli.md, row %q: --%s is not in `waggle %s -h`", row, f[1], cmd)
			}
		}
	}
	if n < 8 {
		t.Errorf("site/content/cli.md has %d nectar, vote and quorum rows, want at least 8", n)
	}
}
