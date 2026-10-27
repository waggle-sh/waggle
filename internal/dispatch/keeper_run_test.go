// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T-045 AC7: a first guard and a --second guard on a Head Keeper branch with no
// builder run and no claim. Local mode, nothing is pushed.
func TestKeeperCommitGuards(t *testing.T) {
	f := newFixture(t, false, false)
	cfg, _ := os.ReadFile(filepath.Join(f.real, ".waggle/control/config.yaml"))
	write(t, f.real, ".waggle/control/config.yaml", strings.Replace(string(cfg), "        fast: haiku\n", "        fast: haiku\n      alternates:\n        standard: haiku\n", 1))
	f.git("commit", "-q", "-am", "config")
	f.git("branch", "t-023k")
	keeper := func(second bool) func(*Request, *Deps) {
		return func(r *Request, _ *Deps) { r.Branch, r.KeeperCommit, r.Second = "t-023k", true, second }
	}
	scenario := emit(assistant(100), verdictEvent(`Verdict: pass`))
	first, err := f.guard(scenario, keeper(false))
	if err != nil || first.ExitCode != 0 || !first.KeeperCommit || first.ClaimVerb != "none" || first.ClaimOID != "" {
		t.Fatalf("first guard: exit %d verb %q keeper %v err %v violations %v", first.ExitCode, first.ClaimVerb, first.KeeperCommit, err, first.Violations)
	}
	second, err := f.guard(scenario, keeper(true))
	if err != nil || second.ExitCode != 0 || !second.KeeperCommit || !second.Second {
		t.Fatalf("second guard: exit %d err %v violations %v", second.ExitCode, err, second.Violations)
	}
	if second.Model == first.Model || second.Model == "" || second.BaseOID != first.BaseOID {
		t.Fatalf("second guard model %q base %s; first %q base %s", second.Model, second.BaseOID, first.Model, first.BaseOID)
	}
	b, _ := os.ReadFile(filepath.Join(second.RunDir, "result.json"))
	if !strings.Contains(string(b), `"keeper_commit": true`) {
		t.Fatalf("result.json lacks keeper_commit: %s", b)
	}
}
