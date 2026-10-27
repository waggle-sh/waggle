// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"bytes"
	"strings"
	"testing"
)

// T-042 AC6: a plain same-owner renewal gives a claim with no list the
// ticket's own branch, and that clears the no-list failure.
func TestRenewalClearsNoBranchList(t *testing.T) {
	repo := dispatchRepo(t)
	r := gateRepo{t, repo}
	lease := `{"version":2,"ticket_id":"T-023","keeper_id":"alice","role":"forager","gate":"full",` +
		`"directories":["internal/x"],"claimed_at":"2020-01-01T00:00:00Z","expires_at":"2999-01-01T00:00:00Z"}`
	blob := r.gitIn(lease, "hash-object", "-w", "--stdin")
	tree := r.gitIn("100644 blob "+blob+"\tlease.json\n", "mktree")
	r.git("update-ref", "refs/waggle/claims/ticket-23", r.git("commit-tree", tree, "-m", "claim"))
	r.git("checkout", "-q", "-b", "t-023")
	r.write("internal/x/b.go", []byte("package x\n"))
	r.commit("T-023: b")
	run := func() (int, string) {
		code, out, errs := gate(func(string) string { return "" }, "leases", "--base", "main", "--head", "t-023", "--repo", repo, "--branch", "t-023")
		return code, out + errs
	}
	const msg = "T-023: claim names no branch; renew it with waggle claim --branch"
	if code, out := run(); code != 1 || !strings.Contains(out, msg) {
		t.Fatalf("before renewal: exit %d\n%s", code, out)
	}
	var out, errb bytes.Buffer
	args := []string{"T-023", "--dirs", "internal/x", "--gate", "full", "--owner", "alice", "--repo", repo, "--json"}
	if exit := runClaim(args, &out, &errb); exit != 0 || !strings.Contains(out.String(), `"branches":["t-023"]`) {
		t.Fatalf("renewal: exit %d stdout %q stderr %q", exit, out.String(), errb.String())
	}
	if code, o := run(); code != 0 || strings.Contains(o, msg) {
		t.Errorf("after renewal: exit %d\n%s", code, o)
	}
}

// T-039 AC6: waggle claim --branch prints and keeps the list.
func TestClaimCLIBranches(t *testing.T) {
	repo := dispatchRepo(t)
	base := []string{"T-32", "--dirs", "internal/x", "--gate", "full", "--owner", "alice", "--repo", repo, "--json"}
	var out, errb bytes.Buffer
	args := append(append([]string{}, base...), "--branch", "feature/x", "--branch", "t-032")
	if exit := runClaim(args, &out, &errb); exit != 0 || !strings.Contains(out.String(), `"branches":["feature/x","t-032"]`) {
		t.Fatalf("exit %d stdout %q stderr %q", exit, out.String(), errb.String())
	}
	out.Reset()
	if exit := runClaim(base, &out, &errb); exit != 0 || !strings.Contains(out.String(), `"branches":["feature/x","t-032"]`) {
		t.Fatalf("renewal: exit %d stdout %q stderr %q", exit, out.String(), errb.String())
	}
	out.Reset()
	args = append([]string{}, base...)
	for _, b := range []string{"a", "b", "c", "d", "e"} {
		args = append(args, "--branch", b)
	}
	if exit := runClaim(args, &out, &errb); exit != 2 {
		t.Fatalf("five branches: exit %d stdout %q", exit, out.String())
	}
}
