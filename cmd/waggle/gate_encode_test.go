// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
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

// hostileDecisionsRepo makes the decisions check fail with a tickets field whose
// quoted form in the error holds a backslash and a %. A tab in the field is
// quoted by %q, so it reaches the reason as a backslash, not a raw tab.
func hostileDecisionsRepo(t *testing.T, r gateRepo) gateRepo {
	t.Helper()
	log := string(gateLog(2))
	i := strings.LastIndex(log, "- **Tickets:** none")
	log = log[:i] + "- **Tickets:** a\tb\\c%" + log[i+len("- **Tickets:** none"):]
	r.write(layout.Decisions, []byte(log))
	r.commit("D-002")
	return r
}

// The decisions FAIL line encodes the reason, plain and GitHub Actions.
func TestDecisionsFailEncodesReason(t *testing.T) {
	const enc = `"a%5Ctb%5C%5Cc%25" is not a T-NNN ID`
	for _, gha := range []bool{false, true} {
		r := hostileDecisionsRepo(t, leaseRepo(t))
		code, out := r.run("decisions", gha)
		want := "decisions: FAIL: "
		if gha {
			want = "::error file=" + layout.Decisions + "::"
		}
		if code != 1 || !strings.Contains(out, want) || !strings.Contains(out, enc) {
			t.Errorf("gha=%v: exit %d, want %q and %q\n%q", gha, code, want, enc, out)
		}
		noRaw(t, out)
		if strings.Contains(out, "%\n") {
			t.Errorf("gha=%v: raw %%\n%q", gha, out)
		}
	}
}

// The decisions error line (a git failure) encodes a tab, a backslash and a %
// that git echoes from the --repo path.
func TestDecisionsErrorEncodesReason(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a\tb\\c%d")
	for _, gha := range []bool{false, true} {
		env := func(string) string { return "" }
		if gha {
			env = ghaEnv
		}
		code, out, errs := gate(env, "decisions", "--base", "main", "--head", "branch", "--repo", dir)
		all := out + errs
		if code != gateError || !strings.Contains(all, "decisions: error: ") || !strings.Contains(all, "a%09b%5Cc%25d") {
			t.Errorf("gha=%v: exit %d\n%q", gha, code, all)
		}
		noRaw(t, all)
	}
}

// The open-mode would-fail line carries the same encoded reason, through the
// real decisions check (gate.mode open).
func TestDecisionsOpenModeEncodesReason(t *testing.T) {
	for _, gha := range []bool{false, true} {
		r := hostileDecisionsRepo(t, protRepo(t, openCfg))
		env := map[string]string{}
		if gha {
			env["GITHUB_ACTIONS"] = "true"
		}
		code, out := r.runWith(env, "verify")
		if code != 0 || !strings.Contains(out, `"a%5Ctb%5C%5Cc%25" is not a T-NNN ID`) {
			t.Errorf("gha=%v: exit %d\n%q", gha, code, out)
		}
		noRaw(t, out)
	}
}
