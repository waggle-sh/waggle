// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestNoRepo runs every command that takes --repo in a directory that is not a
// git repository: each exits 3 with the one-line message and no raw git text.
func TestNoRepo(t *testing.T) {
	cmds := []struct {
		name string
		args []string
	}{
		{"claim", []string{"claim", "T-91", "--dirs", "cmd", "--gate", "full", "--owner", "me"}},
		{"release", []string{"release", "T-91", "--owner", "me"}},
		{"status", []string{"status"}},
		{"doctor", []string{"doctor"}},
		{"pace", []string{"pace"}},
		{"nectar", []string{"nectar"}},
		{"nectar", []string{"nectar", "explain", "T-91"}},
		{"quorum open", []string{"quorum", "open", "--items", "a"}},
		{"quorum status", []string{"quorum", "status"}},
		{"vote", []string{"vote", "--round", "1", "a=1"}},
	}
	for _, c := range cmds {
		t.Run(strings.Join(c.args[:min(2, len(c.args))], " "), func(t *testing.T) {
			dir := t.TempDir()
			var out, errb bytes.Buffer
			args := append(append([]string{}, c.args...), "--repo", dir)
			code := runMain(args, strings.NewReader(""), &out, &errb, func(string) string { return "" })
			want := "waggle " + c.name + ": " + dir + " is not a git repository (run it inside your repo or pass --repo " + dir + ")\n"
			if code != 3 || errb.String() != want || out.Len() != 0 {
				t.Fatalf("code %d stdout %q stderr %q, want 3 and %q", code, out.String(), errb.String(), want)
			}
			if strings.Contains(errb.String(), "fatal:") {
				t.Fatalf("raw git text in %q", errb.String())
			}
		})
	}
}
