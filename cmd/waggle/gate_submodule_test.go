// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
)

// TestVerifyGitlink adds a gitlink (mode 160000) under the claim, built in the
// index so no submodule checkout is needed, and expects the gate's message.
func TestVerifyGitlink(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "full", active, "docs")
	r.git("update-index", "--add", "--cacheinfo", "160000,1111111111111111111111111111111111111111,docs/sub")
	r.git("commit", "-q", "-m", "T-001: submodule")
	out := r.expect(t, 1, "docs/sub: T-001: symlink or submodule mode 160000")
	if !strings.Contains(out, "leases: FAIL") {
		t.Errorf("want a leases failure\n%s", out)
	}
}

// TestVerifyRenameOutOfTier3 renames a hardcoded Tier 3 file into the claim.
// The rename target is inside the lease, so only the deletion side is
// reported, and it must still fail as Tier 3 with the old path named.
func TestVerifyRenameOutOfTier3(t *testing.T) {
	const old = ".github/workflows/x.yml"
	r := newGateRepo(t)
	r.write("README.md", []byte("hive\n"))
	r.write(old, []byte("on: push\n"))
	r.write(layout.Decisions, gateLog(1))
	r.commit("init")
	r.git("checkout", "-q", "-b", "branch")
	r.claim(1, "full", active, "docs")
	if err := os.MkdirAll(filepath.Join(r.dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	r.git("mv", old, "docs/x.yml")
	r.commit("T-001: move")

	out := r.expect(t, 1, "tiers: FAIL: "+old+": Tier 3 path; only an admin bypass merges it, approval does not")
	if !strings.Contains(out, "leases: FAIL: "+old+":") {
		t.Errorf("want the old path outside the lease\n%s", out)
	}
	if strings.Contains(out, "docs/x.yml") {
		t.Errorf("rename target inside the claim was reported\n%s", out)
	}
}
