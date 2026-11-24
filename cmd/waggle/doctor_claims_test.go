// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/comb"
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

// liveLease is a stored lease that stays active at the real clock, which
// doctor reads.
func liveLease(ticket, dir, expires string) string {
	return `{"version": 2, "ticket_id": "` + ticket + `", "keeper_id": "ben", "role": "builder", "gate": "full",` +
		` "directories": ["` + dir + `"], "claimed_at": "2026-10-17T12:00:00Z", "expires_at": "` + expires + `"}`
}

const farFuture = "2099-01-01T00:00:00Z"

func collisionRepo(t *testing.T, claims ...[3]string) string {
	t.Helper()
	dir := doctorRepo(t, githubCfg)
	seamForge(t, &fakeForge{report: report("up_to_date", okGate, okUpTo)}, okClaims)
	for _, c := range claims {
		forgeClaim(t, dir, "refs/waggle/claims/ticket-"+strconv.Itoa(ticketNum(c[0])), liveLease(c[0], c[1], c[2]))
	}
	return dir
}

// B2, B9, B10: one pair prints a missing finding after the summary, exit 0, no ref written.
func TestDoctorClaimsCollision(t *testing.T) {
	dir := collisionRepo(t, [3]string{"T-054", "internal/comb", farFuture}, [3]string{"T-099", "Internal/Comb", farFuture})
	before := gitOut(t, dir, "", "for-each-ref")
	for _, args := range [][]string{{"--repo", dir, "--no-write"}, {"--repo", dir}} {
		code, out, _ := runDoc(args...)
		if code != 0 {
			t.Errorf("exit %d, want 0", code)
		}
		summary, tail, _ := strings.Cut(out, "Claims (informational")
		if strings.Contains(summary, "letter case") {
			t.Errorf("collision leaked into the enforcement summary:\n%s", out)
		}
		for _, want := range []string{"missing   claims",
			"1 pair(s) of claims differ only in letter case, the first is T-054 internal/comb and T-099 Internal/Comb.",
			"    Impact: they are one directory on macOS and Windows, so two tasks can edit the same files under two claims.\n",
			"    Fix: release one, or ask the first login in gate.approvers to clear it with waggle force-release.\n"} {
			if !strings.Contains(tail, want) {
				t.Errorf("claims block lacks %q:\n%s", want, out)
			}
		}
		checkPlain(t, "doctor collision", out)
	}
	checkPlain(t, "shared strings", collisionImpact+collisionFix)
	if after := gitOut(t, dir, "", "for-each-ref"); after != before {
		t.Errorf("doctor wrote a ref:\n%s\nvs\n%s", before, after)
	}
}

// B3, B6: no collision, or a pair with an expired claim, is ok.
func TestDoctorClaimsNoCollision(t *testing.T) {
	dir := collisionRepo(t, [3]string{"T-054", "internal/comb", farFuture}, [3]string{"T-099", "internal/other", farFuture})
	forgeClaim(t, dir, "refs/waggle/claims/ticket-70", liveLease("T-070", "Internal/Comb", "2026-10-18T00:00:00Z"))
	code, out, _ := runDoc("--repo", dir, "--no-write")
	if code != 0 || !strings.Contains(out, "ok        claims          No malformed claim or case collision.") {
		t.Errorf("got %d:\n%s", code, out)
	}
}

// B5: the count is directory pairs and the first named is the lowest ticket.
func TestDoctorClaimsThreePairs(t *testing.T) {
	dir := collisionRepo(t, [3]string{"T-099", "Internal/Comb", farFuture}, [3]string{"T-054", "internal/comb", farFuture},
		[3]string{"T-070", "INTERNAL/comb", farFuture})
	_, out, _ := runDoc("--repo", dir, "--no-write")
	want := "3 pair(s) of claims differ only in letter case, the first is T-054 internal/comb and T-070 INTERNAL/comb."
	if !strings.Contains(out, want) {
		t.Errorf("lacks %q:\n%s", want, out)
	}
}

// B4: a collision with a malformed ref is one finding, malformed text first.
func TestDoctorClaimsCollisionAndMalformed(t *testing.T) {
	dir := collisionRepo(t, [3]string{"T-054", "internal/comb", farFuture}, [3]string{"T-099", "Internal/Comb", farFuture})
	forgeClaim(t, dir, "refs/waggle/claims/ticket-7", "not json")
	_, out, _ := runDoc("--repo", dir, "--no-write")
	if strings.Count(out, "claims          ") != 1 || strings.Count(out, "force-release:") != 1 {
		t.Errorf("want one finding and one force-release line:\n%s", out)
	}
	if !strings.Contains(out, "missing   claims          1 malformed claim(s), the first is refs/waggle/claims/ticket-7; 1 pair(s) of claims differ") {
		t.Errorf("summary:\n%s", out)
	}
	i, j := strings.Index(out, "force-release --ref refs/waggle/claims/ticket-7"), strings.Index(out, "for the pairs, release one")
	if i < 0 || j < i {
		t.Errorf("malformed Fix text must come before the collision Fix text:\n%s", out)
	}
	checkPlain(t, "doctor both", out)
}

// B7: a failed collision scan is skipped, and never hides a malformed finding.
func TestDoctorClaimsScanFailure(t *testing.T) {
	orig := scanCollisions
	scanCollisions = func(context.Context, *comb.GitBackend) ([]comb.CaseCollision, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() { scanCollisions = orig })
	dir := collisionRepo(t)
	code, out, _ := runDoc("--repo", dir, "--no-write")
	if code != 0 || !strings.Contains(out, "skipped   claims          Could not check claims for directories that differ only in letter case.") {
		t.Errorf("got %d:\n%s", code, out)
	}
	checkPlain(t, "scan failure", out)
	forgeClaim(t, dir, "refs/waggle/claims/ticket-7", "not json")
	_, out, _ = runDoc("--repo", dir, "--no-write")
	if !strings.Contains(out, "missing   claims          1 malformed claim(s)") || strings.Contains(out, "letter case") {
		t.Errorf("malformed finding hidden:\n%s", out)
	}
}

// B8: a local-only repo still prints the collision finding and exits 0.
func TestDoctorClaimsCollisionLocalOnly(t *testing.T) {
	dir := doctorRepo(t, "  provider: github\n")
	seamForge(t, &fakeForge{}, okClaims)
	forgeClaim(t, dir, "refs/waggle/claims/ticket-54", liveLease("T-054", "internal/comb", farFuture))
	forgeClaim(t, dir, "refs/waggle/claims/ticket-99", liveLease("T-099", "Internal/Comb", farFuture))
	code, out, _ := runDoc("--repo", dir)
	if code != 0 || !strings.Contains(out, "1 pair(s) of claims differ only in letter case") {
		t.Fatalf("got %d %q", code, out)
	}
}

// B11: the docs sentence names the doctor finding.
func TestMultiKeeperDocsDoctorClause(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "multi-keeper.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "prints a warning for each such pair and `waggle doctor` counts them in its `claims` finding") {
		t.Error("docs/multi-keeper.md lacks the doctor clause")
	}
}
