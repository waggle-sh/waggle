// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/usage"
)

// forgedCollisionLease is a stored lease.json for the collision tests: from
// the status clock for two hours, so it is active at fixedNow.
func forgedCollisionLease(ticket, owner, dir string) string {
	return `{"version": 2, "ticket_id": "` + ticket + `", "keeper_id": "` + owner + `", "role": "builder", "gate": "full",` +
		` "directories": ["` + dir + `"], "claimed_at": "2026-10-17T12:00:00Z", "expires_at": "2026-10-17T14:00:00Z"}`
}

const collisionLine = "  warning: T-054 internal/comb and T-099 Internal/Comb differ only in letter case; they are one directory on macOS and Windows. Release one, or ask the first login in gate.approvers to clear it with waggle force-release.\n"

// T-073 tests B1 and B3: one collision prints one warning line after the
// claims table; none leaves the output as it was.
func TestRenderStatusCollisions(t *testing.T) {
	h := header{Name: "waggle", Pin: "v1", Mode: "direct", Remote: "origin"}
	leases := []comb.Lease{{TicketID: "T-054", KeeperID: "ben", Role: "builder", Gate: "full", Directories: []string{"internal/comb"}, ExpiresAt: fixedNow.Add(time.Hour)}}
	block := usage.Render(nil, layout.Usage)
	none := string(renderStatus(h, leases, nil, nil, block, fixedNow))
	one := string(renderStatus(h, leases, nil, []comb.CaseCollision{
		{TicketA: "T-054", DirA: "internal/comb", OwnerA: "ben", TicketB: "T-099", DirB: "Internal/Comb", OwnerB: "ana"},
	}, block, fixedNow))
	if strings.Contains(none, "warning") {
		t.Errorf("no collisions printed a warning:\n%s", none)
	}
	want := strings.Replace(none, "\n\nUsage:", "\n"+collisionLine+"\nUsage:", 1)
	if one != want {
		t.Errorf("got:\n%s\nwant:\n%s", one, want)
	}
	checkPlain(t, "collision line", one)
}

// T-073 tests B1 and B3 end to end: two stored claims that differ only in case
// make status print the warning and exit 0.
func TestStatusCaseCollision(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := newHive(t)
	forgeClaim(t, dir, "refs/waggle/claims/ticket-54", forgedCollisionLease("T-054", "ben", "internal/comb"))
	forgeClaim(t, dir, "refs/waggle/claims/ticket-99", forgedCollisionLease("T-099", "ana", "Internal/Comb"))
	code, out, errb := runStatusT(t, "--repo", dir)
	if code != 0 || errb != "" {
		t.Fatalf("code %d, stderr %q", code, errb)
	}
	if strings.Count(out, "warning:") != 1 || !strings.Contains(out, collisionLine) {
		t.Errorf("stdout:\n%s\nwant one line %q", out, collisionLine)
	}
	checkPlain(t, "status warning line", collisionLine)
}

// T-082: a failed claims read (here the claim sync from a missing remote)
// fails status with exit 3 before any collision result exists, so stdout is
// empty and neither stream carries a collision warning.
func TestStatusClaimsReadFails(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := newHive(t)
	git(t, dir, "remote", "add", "origin", filepath.Join(t.TempDir(), "missing.git"))
	cfg := filepath.Join(dir, layout.Config)
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "forge:\n") {
		t.Fatalf("config has no forge section:\n%s", b)
	}
	if err := os.WriteFile(cfg, []byte(strings.Replace(string(b), "forge:\n", "forge:\n  remote: origin\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errb := runStatusT(t, "--repo", dir)
	if code != 3 {
		t.Fatalf("code %d, want 3; stderr %q", code, errb)
	}
	if out != "" {
		t.Errorf("stdout %q, want empty", out)
	}
	if !strings.HasPrefix(errb, "waggle status failed: ") {
		t.Errorf("stderr %q, want prefix %q", errb, "waggle status failed: ")
	}
	if strings.Contains(out+errb, "letter case") {
		t.Errorf("collision warning printed on a failed claims read: %q", errb)
	}
}
