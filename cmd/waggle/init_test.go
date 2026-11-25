// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
)

// stampedBuild stands in for a clean, commit-stamped build, which gate-only
// init requires.
func stampedBuild(t *testing.T) {
	t.Helper()
	initBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: strings.Repeat("a", 40)}, {Key: "vcs.modified", Value: "false"},
		}}, true
	}
	t.Cleanup(func() { initBuildInfo = nil })
}

func runInitT(t *testing.T, dir string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runInit(args, dir, &out, &errb)
	return code, out.String(), errb.String()
}

func TestInitArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"positional", []string{"foo"}, "waggle init takes no arguments"},
		{"positional after flag", []string{"--gate-only", "foo"}, "waggle init takes no arguments"},
		{"unknown flag", []string{"--nope"}, "flag provided but not defined"},
		{"profile", []string{"--profile", "web"}, "(only 'cli' is supported"},
		{"delivery", []string{"--delivery", "x"}, "(must be 'installed' or 'hosted')"},
		{"forge", []string{"--forge", "gitlab"}, "(must be 'github' or 'none')"},
		{"comb mode", []string{"--comb-mode", "x"}, "(must be 'direct' or 'relay')"},
		{"profile with gate-only", []string{"--gate-only", "--profile", "web"}, "(only 'cli' is supported"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			code, _, errb := runInitT(t, dir, tc.args...)
			if code != 2 {
				t.Errorf("code %d, want 2; stderr %q", code, errb)
			}
			if !strings.Contains(errb, tc.want) {
				t.Errorf("stderr %q lacks %q", errb, tc.want)
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Errorf("wrote %d entries on a usage error", len(entries))
			}
		})
	}
}

func TestInitHelp(t *testing.T) {
	code, _, errb := runInitT(t, t.TempDir(), "-h")
	if code != 0 || !strings.Contains(errb, "-gate-only") {
		t.Errorf("code %d, stderr %q", code, errb)
	}
}

// files lists every regular file under dir except .git, slash-separated.
func files(t *testing.T, dir string) map[string]string {
	t.Helper()
	got := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		got[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestInitGateOnly(t *testing.T) {
	requireGit(t)
	stampedBuild(t)
	dir := t.TempDir()
	git(t, dir, "init", "-q")

	code, out, errb := runInitT(t, dir, "--gate-only")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, errb)
	}
	want := []string{layout.GateWorkflow, layout.Config, layout.Decisions}
	sort.Strings(want)
	first := files(t, dir)
	var got []string
	for p := range first {
		got = append(got, p)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("wrote %v, want %v", got, want)
	}
	success := "Gate initialized. Next: commit " + layout.GateWorkflow + ", " + layout.Config + " and " + layout.Decisions + ", then require the check verify-leases-and-tiers on your default branch.\n"
	if !strings.Contains(out, success) {
		t.Errorf("stdout %q lacks the success line", out)
	}

	code, _, errb = runInitT(t, dir, "--gate-only")
	if code != 0 {
		t.Fatalf("second run code %d, stderr %q", code, errb)
	}
	second := files(t, dir)
	if len(second) != len(first) {
		t.Errorf("second run changed the file set: %v", second)
	}
	for p, body := range first {
		if second[p] != body {
			t.Errorf("second run changed %s", p)
		}
	}
}

func TestInitGateOnlyUnstamped(t *testing.T) {
	requireGit(t)
	initBuildInfo = func() (*debug.BuildInfo, bool) { return &debug.BuildInfo{}, true }
	t.Cleanup(func() { initBuildInfo = nil })
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	code, _, errb := runInitT(t, dir, "--gate-only")
	if code != 1 || !strings.Contains(errb, "nothing was written") {
		t.Errorf("code %d, stderr %q", code, errb)
	}
	if got := files(t, dir); len(got) != 0 {
		t.Errorf("wrote %v", got)
	}
}
