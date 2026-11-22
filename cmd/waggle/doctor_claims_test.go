// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// B2: the claims finding is informational, names the first malformed ref and
// who may force-release. It writes no ref, with or without --no-write.
func TestDoctorClaimsFinding(t *testing.T) {
	dir := doctorRepo(t, githubCfg)
	seamForge(t, &fakeForge{report: report("up_to_date", okGate, okUpTo)}, okClaims)
	git(t, dir, "checkout", "-q", "-b", "main")
	cfg := filepath.Join(dir, ".waggle", "control", "config.yaml")
	if err := os.WriteFile(cfg, []byte("version: 1\nforge:\n"+githubCfg+"gate:\n  approvers: Ana, ben\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "base")
	git(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	_, out, _ := runDoc("--repo", dir, "--no-write")
	if !strings.Contains(out, "ok        claims") || !strings.Contains(out, "force-release: only Ana (first login in gate.approvers at ") {
		t.Errorf("no malformed claim:\n%s", out)
	}
	forgeClaim(t, dir, "refs/waggle/claims/ticket-7", "not json")
	before := gitOut(t, dir, "", "for-each-ref")
	code, out, _ := runDoc("--repo", dir, "--no-write")
	for _, want := range []string{"missing   claims", "1 malformed claim(s), the first is refs/waggle/claims/ticket-7.",
		"force-release --ref refs/waggle/claims/ticket-7"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	if code != 0 {
		t.Errorf("exit %d, want 0: claims is not required", code)
	}
	summary, tail, _ := strings.Cut(out, "Claims (informational")
	if strings.Contains(summary, "  missing   claims") || strings.Contains(summary, "malformed") || !strings.Contains(summary, "No required setting above was found missing") {
		t.Errorf("claims leaked into the enforcement summary:\n%s", out)
	}
	if !strings.Contains(tail, "missing   claims") {
		t.Errorf("claims finding not after the summary:\n%s", out)
	}
	if err := os.WriteFile(filepath.Join(dir, ".waggle", "control", "local.yaml"), []byte("keeper_id: ana\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, out, _ = runDoc("--repo", dir); !strings.Contains(out, "force-release: allowed for ana\n") {
		t.Errorf("caller line:\n%s", out)
	}
	if after := gitOut(t, dir, "", "for-each-ref"); after != before {
		t.Errorf("doctor wrote a ref:\n%s\nvs\n%s", before, after)
	}
}

func TestDoctorClaimsLocalOnly(t *testing.T) {
	dir := doctorRepo(t, "  provider: github\n")
	seamForge(t, &fakeForge{}, okClaims)
	code, out, _ := runDoc("--repo", dir)
	if code != 0 || !strings.Contains(out, "claims") || !strings.Contains(out, "force-release: nobody") {
		t.Fatalf("got %d %q", code, out)
	}
	for _, bad := range []string{"Enforcement:", "Not enforced", "cannot stop the merges"} {
		if strings.Contains(out, bad) {
			t.Errorf("local-only output has %q:\n%s", bad, out)
		}
	}
}
