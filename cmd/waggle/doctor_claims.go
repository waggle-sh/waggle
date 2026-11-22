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
	f := forge.Finding{ID: "claims", Status: forge.StatusOK, Summary: "No malformed claim."}
	opts, err := cfg.CombOptions(root)
	var b *comb.GitBackend
	if err == nil {
		opts.Remote = ""
		b, err = comb.NewGitBackend(opts)
	}
	if err == nil {
		_, err = b.ListActiveLeases(ctx)
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
	first, base, err := dispatch.FirstApprover(ctx, root, cfg.Forge.Remote, "")
	if err != nil || first == "" {
		return f, "force-release: nobody; no first login in gate.approvers at the base commit"
	}
	if id, err := config.LoadKeeperID(root); err == nil && strings.EqualFold(id, first) {
		return f, "force-release: allowed for " + id
	}
	return f, fmt.Sprintf("force-release: only %s (first login in gate.approvers at %.12s)", first, base)
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
