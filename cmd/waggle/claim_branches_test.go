//go:build unix

package main

import (
	"bytes"
	"strings"
	"testing"
)

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
