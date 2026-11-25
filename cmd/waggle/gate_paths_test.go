// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// claimRaw forges a claim like gateRepo.claim, but writes each directory
// through encoding/json so a quote, backslash or tab survives into the lease.
func (r gateRepo) claimRaw(n int, dirs ...string) {
	r.t.Helper()
	list, err := json.Marshal(dirs)
	if err != nil {
		r.t.Fatal(err)
	}
	lease := fmt.Sprintf(`{"version":2,"ticket_id":"T-%03d","keeper_id":"keeper","role":"forager",`+
		`"gate":"full","directories":%s,"claimed_at":"2020-01-01T00:00:00Z","expires_at":%q}`,
		n, list, active)
	blob := r.gitIn(lease, "hash-object", "-w", "--stdin")
	tree := r.gitIn("100644 blob "+blob+"\tlease.json\n", "mktree")
	oid := r.git("commit-tree", tree, "-m", "claim")
	r.git("update-ref", fmt.Sprintf("refs/waggle/claims/ticket-%d", n), oid)
}

// pathsCommit stages paths in the index and commits them under T-001.
func (r gateRepo) pathsCommit(paths ...string) {
	r.t.Helper()
	for _, p := range paths {
		r.index(p)
	}
	r.git("commit", "-q", "-m", "T-001: paths")
}

// awkwardNames are file names the gate must handle. The tab and the
// backslash are the two the encoder leaves as they are.
var awkwardNames = map[string]string{
	"space":     "a b.txt",
	"dquote":    `a"b.txt`,
	"squote":    "a'b.txt",
	"backslash": `a\b.txt`,
	"dash":      "-rf",
	"nonascii":  "café.txt",
	"tab":       "a\tb.txt",
}

const outsideMsg = ": T-001: path is outside the claim directories\n"

// Awkward names inside the claim directory pass the lease check.
func TestPathsAwkwardInsideClaim(t *testing.T) {
	for name, file := range awkwardNames {
		t.Run(name, func(t *testing.T) {
			r := leaseRepo(t)
			r.claim(1, "full", active, "src")
			r.pathsCommit("src/" + file)
			r.expect(t, 0, "leases: ok\n", "tiers: ok\n")
		})
	}
}

// The same names one directory over fail, and the finding prints the path
// through the encoder, in both output modes. Pinned as is: the encoder keeps
// a tab and a backslash raw (reported in the PR body).
func TestPathsAwkwardOutsideClaim(t *testing.T) {
	for name, file := range awkwardNames {
		t.Run(name, func(t *testing.T) {
			r := leaseRepo(t)
			r.claim(1, "full", active, "src")
			r.pathsCommit("lib/" + file)
			code, out := r.run("verify", false)
			want := "leases: FAIL: lib/" + file + outsideMsg
			if code != 1 || !strings.Contains(out, want) || strings.Contains(out, "\r") {
				t.Errorf("exit %d, want line %q\n%q", code, want, out)
			}
			code, out = r.run("verify", true)
			want = "::error file=lib/" + file + "::leases: T-001: path is outside the claim directories\n"
			if code != 1 || !strings.Contains(out, want) || strings.Contains(out, "\r") {
				t.Errorf("gha: exit %d, want line %q\n%q", code, want, out)
			}
		})
	}
}

// Every spelling of the directory src is read as src: the stored claim is
// normalized, a prefix sibling fails, and a different case fails with its own
// finding.
func TestPathsClaimSpellings(t *testing.T) {
	same := []string{"src", "src/", "./src", "./src/", "src//", "src/."}
	for _, d := range same {
		t.Run(d, func(t *testing.T) {
			r := leaseRepo(t)
			r.claimRaw(1, d)
			r.pathsCommit("src/a.go", "src/sub/b.go", "srcfoo/x", "lib/y")
			out := r.expect(t, 1,
				"leases: FAIL: srcfoo/x"+outsideMsg,
				"leases: FAIL: lib/y"+outsideMsg)
			if strings.Contains(out, "src/a.go") || strings.Contains(out, "src/sub/b.go") {
				t.Errorf("a path under the claim was reported\n%s", out)
			}
		})
	}
	t.Run("inner dot", func(t *testing.T) {
		// src/./x is the directory src/x, not src.
		r := leaseRepo(t)
		r.claimRaw(1, "src/./x")
		r.pathsCommit("src/x/a.go", "src/y.go")
		out := r.expect(t, 1, "leases: FAIL: src/y.go"+outsideMsg)
		if strings.Contains(out, "src/x/a.go") {
			t.Errorf("a path under src/x was reported\n%s", out)
		}
	})
	t.Run("case", func(t *testing.T) {
		r := leaseRepo(t)
		r.claimRaw(1, "SRC")
		r.pathsCommit("src/a.go", "srcfoo/x")
		r.expect(t, 1,
			"leases: FAIL: src/a.go: T-001: case differs from lease\n",
			"leases: FAIL: srcfoo/x"+outsideMsg)
	})
}

// A claim directory with a space, quote, backslash, leading dash, non-ASCII
// byte or tab is not a valid directory. The gate fails the commit, quoting the
// directory so no raw byte reaches the output.
func TestPathsInvalidClaimDirs(t *testing.T) {
	const allow = " contains a character outside [A-Za-z0-9._/-]"
	cases := map[string]struct{ dir, why string }{
		"space":     {"my dir", allow},
		"dquote":    {`a"b`, allow},
		"squote":    {"it's", allow},
		"backslash": {`a\b`, allow},
		"nonascii":  {"café", allow},
		"tab":       {"a\tb", allow},
		"dash":      {"-x", `: segment "-x" starts with a dash`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := leaseRepo(t)
			r.claimRaw(1, c.dir)
			r.pathsCommit("src/x")
			msg := fmt.Sprintf("T-001: refs/waggle/claims/ticket-1: comb: malformed claim ref: directory %q%s: comb: invalid lease\n", c.dir, c.why)
			code, out := r.run("verify", false)
			if code != 1 || !strings.Contains(out, "leases: FAIL: "+msg) || strings.Contains(out, "\t") {
				t.Errorf("exit %d, want %q\n%q", code, msg, out)
			}
			code, out = r.run("verify", true)
			if code != 1 || !strings.Contains(out, "::error::leases: "+msg) || strings.Contains(out, "\t") {
				t.Errorf("gha: exit %d, want %q\n%q", code, msg, out)
			}
		})
	}
}
