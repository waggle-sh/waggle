// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/dispatch"
	"github.com/waggle-sh/waggle/internal/forge"
)

// claimsFinding is the informational "claims" finding of waggle doctor and
// the force-release line. It reads the local claim refs and the last fetched
// base, so it writes no ref, with or without --no-write. It is never
// Required, so it cannot change doctor's exit code.
func claimsFinding(ctx context.Context, root string, cfg *config.Config) (forge.Finding, string) {
	f := forge.Finding{ID: "claims", Status: forge.StatusOK, Summary: "No malformed claim or case collision."}
	opts, err := cfg.CombOptions(root)
	var b *comb.GitBackend
	if err == nil {
		opts.Remote = ""
		b, err = comb.NewGitBackend(opts)
	}
	var collisions []comb.CaseCollision
	var collErr error
	if err == nil {
		_, err = b.ListActiveLeases(ctx)
		collisions, collErr = scanCollisions(ctx, b)
	}
	var m *comb.MalformedClaimsError
	switch {
	case errors.As(err, &m) && len(m.Claims) > 0:
		ref := m.Claims[0].Ref
		f.Status = forge.StatusMissing
		f.Summary = fmt.Sprintf("%d malformed claim(s), the first is %s.", len(m.Claims), ref)
		f.NotEnforced = "a malformed claim stays until someone clears it, and it blocks claims on its ticket."
		f.Fix = "the first login in gate.approvers can clear it: waggle force-release --ref " + ref
	case err != nil && m == nil:
		f.Status, f.Summary = forge.StatusSkipped, "Could not read the claim refs."
	}
	if collErr != nil && f.Status == forge.StatusOK {
		f.Status, f.Summary = forge.StatusSkipped, "Could not check claims for directories that differ only in letter case."
	}
	if len(collisions) > 0 {
		addCollisions(&f, collisions)
	}
	first, base, err := dispatch.FirstApprover(ctx, root, cfg.Forge.Remote, "")
	if err != nil || first == "" {
		return f, "force-release: nobody; no first login in gate.approvers at the base commit"
	}
	if id, err := config.LoadKeeperID(root); err == nil && strings.EqualFold(id, first) {
		return f, "force-release: allowed for " + id
	}
	return f, fmt.Sprintf("force-release: only %s (first login in gate.approvers at %.12s)", first, base)
}

// scanCollisions is the case collision read; tests replace it to force a failure.
var scanCollisions = func(ctx context.Context, b *comb.GitBackend) ([]comb.CaseCollision, error) {
	return b.CaseCollisions(ctx)
}

// addCollisions adds the case collision text to the finding. Malformed text
// stays first: it blocks claiming and releasing, a collision does not.
func addCollisions(f *forge.Finding, cs []comb.CaseCollision) {
	first := strings.TrimSuffix(collisionPair(cs[0]), " differ only in letter case")
	summary := fmt.Sprintf("%d pair(s) of claims differ only in letter case, the first is %s.", len(cs), first)
	impact := collisionImpact + ", so two tasks can edit the same files under two claims."
	fix := collisionFix + "."
	if f.Status == forge.StatusMissing {
		summary = strings.TrimSuffix(f.Summary, ".") + "; " + summary
		impact = f.NotEnforced + " " + strings.ToUpper(impact[:1]) + impact[1:]
		fix = f.Fix + "; for the pairs, " + fix
	}
	f.Status, f.Summary, f.NotEnforced, f.Fix = forge.StatusMissing, summary, impact, fix
}

// printClaims writes the claims finding and the force-release line after the
// enforcement summary. They stay out of forge.Render's findings: a claim is
// not a merge rule, so it must not count among the merges Waggle cannot stop.
func printClaims(w io.Writer, f forge.Finding, line string) {
	fmt.Fprintf(w, "\nClaims (informational, not a merge rule):\n  %-9s %-15s %s\n", f.Status, f.ID, f.Summary)
	if f.NotEnforced != "" {
		fmt.Fprintln(w, "    Impact: "+f.NotEnforced)
	}
	if f.Fix != "" {
		fmt.Fprintln(w, "    Fix: "+f.Fix)
	}
	fmt.Fprintln(w, line)
}
