// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var updateClaims = flag.Bool("update-claims", false, "regenerate testdata/claims_read.golden")

// T-081: golden output of ListActiveLeases and CaseCollisions for
// representative claim sets, captured before the one-sync refactor.
func TestClaimsReadGolden(t *testing.T) {
	requireGit(t)
	scenarios := []struct {
		name   string
		claims map[string]string // ref suffix to lease JSON
		expire bool              // advance the clock past every claim
	}{
		{"none", nil, false},
		{"several", map[string]string{
			"ticket-10": forgedJSON("T-010", "ben", `"docs", "pkg/a"`),
			"ticket-2":  forgedJSON("T-002", "carl", `"cmd"`),
			"ticket-3":  forgedJSON("T-003", "ana", `"internal/x"`),
		}, false},
		{"case-collision", map[string]string{
			"ticket-1":  forgedJSON("T-001", "ben", `"docs"`),
			"ticket-2":  forgedJSON("T-002", "carl", `"Docs/api"`),
			"ticket-10": forgedJSON("T-010", "ana", `"Docs", "pkg"`),
			"ticket-3":  forgedJSON("T-003", "dee", `"PKG/x", "other"`),
		}, false},
		{"expired", map[string]string{
			"ticket-1": forgedJSON("T-001", "ben", `"docs"`),
			"ticket-2": forgedJSON("T-002", "carl", `"Docs"`),
		}, true},
		{"malformed", map[string]string{
			"ticket-1": forgedJSON("T-001", "ben", `"docs"`),
			"ticket-2": `{"not": "a lease"}`,
			"ticket-3": forgedJSON("T-003", "carl", `"DOCS/x"`),
		}, false},
	}
	var sb strings.Builder
	for _, s := range scenarios {
		for _, remote := range []bool{false, true} {
			var dir string
			var b *GitBackend
			clock := newClock()
			if remote {
				origin, a, _ := remoteSetup(t)
				dir = origin
				b = newBackend(t, a, "origin", clock)
			} else {
				dir = newRepo(t, false)
				b = newBackend(t, dir, "", clock)
			}
			for suffix, js := range s.claims {
				forgeClaim(t, dir, "refs/waggle/claims/"+suffix, js)
			}
			if s.expire {
				clock.Advance(3 * time.Hour)
			}
			ctx := context.Background()
			leases, lerr := b.ListActiveLeases(ctx)
			cols, cerr := b.CaseCollisions(ctx)
			lj, _ := json.MarshalIndent(leases, "", "  ")
			cj, _ := json.MarshalIndent(cols, "", "  ")
			mode := "local"
			if remote {
				mode = "remote"
			}
			fmt.Fprintf(&sb, "== %s/%s\nleases err: %v\nleases: %s\ncollisions err: %v\ncollisions: %s\n\n",
				s.name, mode, lerr, lj, cerr, cj)
		}
	}
	golden := filepath.Join("testdata", "claims_read.golden")
	if *updateClaims {
		if err := os.WriteFile(golden, []byte(sb.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if sb.String() != string(want) {
		t.Errorf("claims read output changed from %s:\n%s", golden, sb.String())
	}
}
