// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"

	"github.com/waggle-sh/waggle/internal/dispatch"
)

// The ready list's merged check binds by the gate's rule (Q-T054-4 (a)): a
// subject prefix or a Waggle-Ticket trailer, never a mention.
func TestCommitTask(t *testing.T) {
	cases := []struct{ subject, trailer, want string }{
		{"T-101: add x", "", "T-101"},
		{"T-0101: add x", "", "T-101"},
		{"add x", "T-101", "T-101"},
		{"T-101: add x", "T-101", "T-101"},
		{"T-101: add x", "T-102", ""},
		{"add x", "T-101, T-102", ""},
		{"add x", "not-a-ticket", ""},
		{`Revert "T-101: add x"`, "", ""},
		{"Records: T-101 brief, T-102 status", "", ""},
		{"T-102: build on the T-101 API", "", "T-102"},
		{"Head Keeper: unblock T-101", "", ""},
		{"Head Keeper: unblock T-101", "T-101", ""},
		{"Merge pull request #9 from org/t-101-x", "", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := commitTask(c.subject, c.trailer); got != c.want {
			t.Errorf("commitTask(%q, %q) = %q, want %q", c.subject, c.trailer, got, c.want)
		}
	}
}

// A task counts as merged when its work is on main, including through a PR
// merge commit; a mention, a revert subject or a Head Keeper subject does not
// make it merged.
func TestQuorumMergedBinding(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	commit := func(msg string) { git(t, dir, "commit", "-q", "--allow-empty", "-m", msg) }
	commit("init")
	git(t, dir, "checkout", "-q", "-b", "t-101")
	commit("T-101: add x")
	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "merge", "-q", "--no-ff", "-m", "Merge pull request #1 from org/t-101", "t-101")
	commit("add y\n\nWaggle-Ticket: T-103")
	commit("Records: T-102 brief, T-104 status")
	commit(`Revert "T-105: add z"`)
	commit("Head Keeper: unblock T-106")
	commit("T-107: bound by subject\n\nWaggle-Ticket: T-108")

	r := &quorumRun{root: dir, git: &dispatch.Git{Dir: dir}, main: "main"}
	merged := r.merged(context.Background())
	for task, want := range map[string]bool{
		"T-101": true, "T-0101": true, "T-103": true,
		"T-102": false, "T-104": false, "T-105": false, "T-106": false,
		"T-107": false, "T-108": false, "T-999": false, "issue-7": false,
	} {
		if got := merged(task); got != want {
			t.Errorf("merged(%q) = %v, want %v", task, got, want)
		}
	}

	bad := &quorumRun{root: dir, git: &dispatch.Git{Dir: dir}, main: "no-such-ref"}
	if bad.merged(context.Background())("T-101") {
		t.Error("a failed git read reported T-101 merged")
	}
}

// A veto ID passes only in the form close looks up (D-%03d of a number >= 1).
func TestCanonicalDecision(t *testing.T) {
	for id, want := range map[string]bool{
		"D-001": true, "D-007": true, "D-372": true, "D-1000": true, "D-999999": true,
		"D-1": false, "D-07": false, "D-0007": false, "D-000": false, "D-01000": false,
		"D-+01": false, "D- 01": false, "D-1e3": false, "d-007": false, "D-": false, "": false, "D-0000001": false,
	} {
		if got := canonicalDecision(id); got != want {
			t.Errorf("canonicalDecision(%q) = %v, want %v", id, got, want)
		}
	}
}
