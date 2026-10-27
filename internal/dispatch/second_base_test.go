// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T-034 AC8 (D-254): --second with no --base takes the first guard record's
// base_oid; one that names no commit is exit 3 naming the record, and one that
// is not hex never reaches git.
func TestSecondDefaultBase(t *testing.T) {
	for oid, want := range map[string]string{
		strings.Repeat("ab", 20): "first guard base " + strings.Repeat("ab", 20) + " from run record T-023-guard-1 does not resolve",
		"HEAD":                   "run record T-023-guard-1 has no valid base_oid",
	} {
		t.Run(want, func(t *testing.T) {
			f, _ := secondFixture(t, true)
			p := filepath.Join(f.real, ".waggle/worktrees/runs/T-023-guard-1/result.json")
			var m map[string]any
			b, _ := os.ReadFile(p)
			if json.Unmarshal(b, &m) != nil {
				t.Fatal("guard result.json")
			}
			m["base_oid"] = oid
			b, _ = json.Marshal(m)
			if err := os.WriteFile(p, b, 0o600); err != nil {
				t.Fatal(err)
			}
			res, err := f.second()
			if code(err) != ExitEnv || !strings.Contains(err.Error(), want) || res.RunDir != "" {
				t.Fatalf("want exit 3 containing %q before launch, got exit %d err %v run %q", want, code(err), err, res.RunDir)
			}
		})
	}
}
