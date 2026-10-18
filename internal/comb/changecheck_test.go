package comb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCheckChanges(t *testing.T) {
	full := Lease{TicketID: "T-001", Gate: "full", Directories: []string{"internal/comb", "docs"}}
	light := full
	light.Gate = "light"
	// The tier 2 cases use a lease no claim could hold (validation forbids
	// .waggle), so the tier rule is tested on its own.
	hive := Lease{Gate: "light", Directories: []string{".waggle/hive"}}
	hiveFull := hive
	hiveFull.Gate = "full"
	ch := func(p string) Change { return Change{Path: p, OldMode: "100644", NewMode: "100644"} }
	cases := []struct {
		name  string
		lease Lease
		c     Change
		want  []string
	}{
		{"inside", full, ch("internal/comb/x.go"), nil},
		{"dir itself", full, ch("docs"), nil},
		{"sibling prefix", full, ch("internal/combx/x.go"), []string{RuleOutsideLease}},
		{"outside", full, ch("internal/layout/x.go"), []string{RuleOutsideLease}},
		{"case differs", full, ch("internal/Comb/x.go"), []string{RuleCaseDiffers}},
		{"case differs dir", full, ch("DOCS"), []string{RuleCaseDiffers}},
		{"symlink", full, Change{"docs/l", "000000", "120000"}, []string{RuleLinkMode}},
		{"submodule", full, Change{"docs/s", "000000", "160000"}, []string{RuleLinkMode}},
		{"symlink deleted", full, Change{"docs/l", "120000", "000000"}, nil},
		{"tier 3", full, ch(".github/workflows/x.yml"), []string{RuleOutsideLease, RuleTier3}},
		{"tier 3 case", full, ch(".WAGGLE/control/waggle.md"), []string{RuleOutsideLease, RuleTier3}},
		{"tier 2 light", hive, ch(".waggle/hive/tasks/T-099.md"), []string{RuleTier2Light}},
		{"tier 2 full", hiveFull, ch(".waggle/hive/tasks/T-099.md"), nil},
		{"tier 2 light outside", light, ch("CLAUDE.md"), []string{RuleOutsideLease, RuleTier2Light}},
		{"empty lease", Lease{}, ch("docs/a"), []string{RuleOutsideLease}},
	}
	for _, tc := range cases {
		var got []string
		for _, v := range CheckChanges(tc.lease, []Change{tc.c}) {
			if v.Path != tc.c.Path || v.Detail == "" {
				t.Errorf("%s: bad violation %+v", tc.name, v)
			}
			got = append(got, v.Rule)
		}
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("%s: rules %v; want %v", tc.name, got, tc.want)
		}
	}
	if !IsTierRule(RuleTier3) || !IsTierRule(RuleTier2Light) || IsTierRule(RuleOutsideLease) {
		t.Error("IsTierRule")
	}
}

// D-107: a lease.json over MaxLeaseBytes is a malformed claim and is not
// decoded; the claims around it in the same batch still decode.
func TestLoadClaimsSizeCap(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	lease := func(n int, pad int) string {
		return fmt.Sprintf(`{"version":2,"ticket_id":"T-%03d","keeper_id":"keeper","role":"forager",`+
			`"gate":"full","directories":["docs"],"claimed_at":"2026-10-17T00:00:00Z",`+
			`"expires_at":"2026-10-17T02:00:00Z"%s}`, n, strings.Repeat(" ", pad))
	}
	forgeClaim(t, dir, "refs/waggle/claims/ticket-1", lease(1, 0))
	forgeClaim(t, dir, "refs/waggle/claims/ticket-2", lease(2, MaxLeaseBytes))
	forgeClaim(t, dir, "refs/waggle/claims/ticket-3", lease(3, 0))
	b := newBackend(t, dir, "", nil)
	ctx := context.Background()
	if _, err := b.ReadClaim(ctx, "T-002"); !errors.Is(err, ErrMalformedClaim) || !strings.Contains(err.Error(), "byte cap") {
		t.Errorf("oversized claim: %v", err)
	}
	for _, id := range []string{"T-001", "T-003"} {
		if l, err := b.ReadClaim(ctx, id); err != nil || l.TicketID != id {
			t.Errorf("%s: %+v %v", id, l, err)
		}
	}
}
