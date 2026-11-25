// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

// T-087: top-level help and version flags, usage on the right stream.
func TestRunTopLevel(t *testing.T) {
	ver := "waggle v" + version + "\n"
	const usageHead = "waggle - task claims and a merge gate"
	tests := []struct {
		name       string
		args       []string
		code       int
		outHas     string
		outExact   string
		errHas     []string
		errNotHas  string
		outIsEmpty bool
	}{
		{name: "dash h", args: []string{"-h"}, outHas: usageHead},
		{name: "long help", args: []string{"--help"}, outHas: usageHead},
		{name: "help word", args: []string{"help"}, outHas: usageHead},
		{name: "dash v", args: []string{"-v"}, outExact: ver},
		{name: "long version", args: []string{"--version"}, outExact: ver},
		{name: "version", args: []string{"version"}, outExact: ver},
		{name: "version extra", args: []string{"version", "extra"}, code: 2, outIsEmpty: true,
			errHas: []string{"waggle version takes no arguments"}},
		{name: "no args", args: nil, code: 2, outIsEmpty: true, errHas: []string{usageHead}},
		{name: "unknown", args: []string{"frobnicate"}, code: 2, outIsEmpty: true,
			errHas: []string{`Unknown subcommand "frobnicate"`, usageHead}, errNotHas: "did you mean"},
		{name: "typo", args: []string{"claime"}, code: 2, outIsEmpty: true,
			errHas: []string{`Unknown subcommand "claime"`, `did you mean "claim"?`, usageHead}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			code := runMain(tc.args, strings.NewReader(""), &out, &errb, func(string) string { return "" })
			if code != tc.code {
				t.Errorf("code = %d, want %d (stderr %q)", code, tc.code, errb.String())
			}
			if tc.outExact != "" && out.String() != tc.outExact {
				t.Errorf("stdout = %q, want %q", out.String(), tc.outExact)
			}
			if tc.outHas != "" && !strings.Contains(out.String(), tc.outHas) {
				t.Errorf("stdout lacks %q", tc.outHas)
			}
			if tc.outIsEmpty && out.Len() != 0 {
				t.Errorf("stdout = %q, want empty", out.String())
			}
			for _, w := range tc.errHas {
				if !strings.Contains(errb.String(), w) {
					t.Errorf("stderr lacks %q: %q", w, errb.String())
				}
			}
			if tc.errNotHas != "" && strings.Contains(errb.String(), tc.errNotHas) {
				t.Errorf("stderr has %q: %q", tc.errNotHas, errb.String())
			}
			if tc.code == 0 && errb.Len() != 0 {
				t.Errorf("stderr = %q, want empty", errb.String())
			}
		})
	}
}

func TestUsageHeadingNoBootstrap(t *testing.T) {
	var b bytes.Buffer
	printUsageTo(&b)
	if strings.Contains(b.String(), "Bootstrap") || !strings.Contains(b.String(), "\nCommands:\n") {
		t.Errorf("usage heading wrong:\n%s", b.String())
	}
}

func TestEditDistance(t *testing.T) {
	for _, c := range []struct {
		a, b string
		d    int
	}{{"", "abc", 3}, {"claim", "claim", 0}, {"claime", "claim", 1}, {"kitten", "sitting", 3}} {
		if got := editDistance(c.a, c.b); got != c.d {
			t.Errorf("editDistance(%q,%q) = %d, want %d", c.a, c.b, got, c.d)
		}
	}
}
