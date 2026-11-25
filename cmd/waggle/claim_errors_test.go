// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type errCase struct {
	name string
	fn   func([]string, *bytes.Buffer, *bytes.Buffer) int
	args []string
	exit int
	want []string
}

// TestClaimReleaseErrors is the T-090 table: each failure exits as documented and
// its stderr names the flag, the allowed values or the fix.
func TestClaimReleaseErrors(t *testing.T) {
	repo := dispatchRepo(t)
	gateOnly := dispatchRepo(t)
	if err := os.Remove(filepath.Join(gateOnly, ".waggle/control/local.yaml")); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if exit := runClaim([]string{"T-5", "--dirs", "internal/x", "--gate", "full", "--repo", repo}, &out, &errb); exit != 0 {
		t.Fatalf("setup claim: exit %d %q", exit, errb.String())
	}
	var branches, cos []string
	for _, b := range []string{"t-1", "t-1-a", "t-1-b", "t-1-c", "t-1-d"} {
		branches = append(branches, "--branch", b)
	}
	for _, c := range []string{"a", "b", "c", "d", "e"} {
		cos = append(cos, "--co", c)
	}
	claim := func(task string, args ...string) []string {
		return append([]string{task, "--repo", repo}, args...)
	}
	cases := []errCase{
		{"missing dirs", claimCLI, claim("T-1", "--gate", "full"), 2, []string{"--dirs is required"}},
		{"missing gate", claimCLI, claim("T-1", "--dirs", "a"), 2, []string{"--gate is required", "light or full"}},
		{"bad gate", claimCLI, claim("T-1", "--dirs", "a", "--gate", "heavy"), 2, []string{`--gate "heavy"`, "light or full"}},
		{"fifth branch", claimCLI, append(claim("T-1", "--dirs", "a", "--gate", "full"), branches...), 2, []string{"--branch given 5 times", "at most 4"}},
		{"fifth co", releaseCLI, append(claim("T-1"), cos...), 2, []string{"--co names 5 keepers", "at most 4"}},
		{"overlap", claimCLI, claim("T-6", "--dirs", "internal/x/sub", "--gate", "full"), 4, []string{"T-005", `"internal/x"`, "overlaps"}},
		{"owner missing claim", claimCLI, []string{"T-1", "--dirs", "a", "--gate", "full", "--repo", gateOnly}, 2,
			[]string{"--owner <your GitHub login>", ".waggle/control/local.yaml"}},
		{"owner missing release", releaseCLI, []string{"T-1", "--repo", gateOnly}, 2,
			[]string{"--owner <your GitHub login>", ".waggle/control/local.yaml"}},
	}
	for _, id := range []string{"t-1", "T-", "1a", "../x"} {
		cases = append(cases,
			errCase{"claim task " + id, claimCLI, claim(id, "--dirs", "a", "--gate", "full"), 2, []string{"ticket id", "T-<digits>"}},
			errCase{"release task " + id, releaseCLI, claim(id), 2, []string{"ticket id", "T-<digits>"}})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if exit := c.fn(c.args, &out, &errb); exit != c.exit || out.Len() != 0 {
				t.Fatalf("exit %d (want %d), stdout %q, stderr %q", exit, c.exit, out.String(), errb.String())
			}
			for _, w := range c.want {
				if !strings.Contains(errb.String(), w) {
					t.Errorf("stderr %q lacks %q", errb.String(), w)
				}
			}
		})
	}
}

// TestReleaseNoClaimNamesRef: releasing a ticket nobody claimed succeeds and says
// which ref it looked for.
func TestReleaseNoClaimNamesRef(t *testing.T) {
	repo := dispatchRepo(t)
	var out, errb bytes.Buffer
	if exit := runTaskRelease([]string{"T-7", "--repo", repo}, &out, &errb); exit != 0 || errb.Len() != 0 {
		t.Fatalf("exit %d, stderr %q", exit, errb.String())
	}
	for _, w := range []string{"no claim", "T-007", "refs/waggle/claims/ticket-7"} {
		if !strings.Contains(out.String(), w) {
			t.Errorf("stdout %q lacks %q", out.String(), w)
		}
	}
}
