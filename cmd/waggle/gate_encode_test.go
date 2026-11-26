// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

// hostilePath holds a tab and a backslash. Its encoded form is encodedPath.
const (
	hostilePath = "lib/a\tb\\c.txt"
	encodedPath = "lib/a%09b%5Cc.txt"
)

// noRaw fails when out still holds a tab or a backslash.
func noRaw(t *testing.T, out string) {
	t.Helper()
	if strings.ContainsAny(out, "\t\\") {
		t.Errorf("raw tab or backslash in output\n%q", out)
	}
}

// A finding line prints a tab as %09 and a backslash as %5C, in the data
// position (plain) and the property position (GitHub Actions).
func TestFindingEncodesTabAndBackslash(t *testing.T) {
	for _, gha := range []bool{false, true} {
		r := leaseRepo(t)
		r.claim(1, "full", active, "src")
		r.pathsCommit(hostilePath)
		code, out := r.run("verify", gha)
		want := "leases: FAIL: " + encodedPath + outsideMsg
		if gha {
			want = "::error file=" + encodedPath + "::leases: T-001: path is outside the claim directories\n"
		}
		if code != 1 || !strings.Contains(out, want) {
			t.Errorf("gha=%v: exit %d, want line %q\n%q", gha, code, want, out)
		}
		noRaw(t, out)
	}
}

// A would-fail line (gate.mode open) encodes the path the same way.
func TestWouldFailEncodesTabAndBackslash(t *testing.T) {
	for _, gha := range []bool{false, true} {
		var buf bytes.Buffer
		printWouldFail(&buf, gha, "leases", hostilePath, "msg")
		want := "leases: would fail (mode open): " + encodedPath + ": msg\n"
		if gha {
			want = "::warning file=" + encodedPath + "::leases: would fail (mode open): msg\n"
		}
		if buf.String() != want {
			t.Errorf("gha=%v: got %q, want %q", gha, buf.String(), want)
		}
		noRaw(t, buf.String())
	}
}

// A message in the data position under GitHub Actions is encoded too.
func TestAnnotDataEncodesTabAndBackslash(t *testing.T) {
	var buf bytes.Buffer
	printWouldFail(&buf, true, "leases", "", annotData.Replace("a\tb\\c"))
	want := "::warning::leases: would fail (mode open): a%09b%5Cc\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
	noRaw(t, buf.String())
}
