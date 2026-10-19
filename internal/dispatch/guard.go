package dispatch

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/runtime"
)

var verdictLine = regexp.MustCompile(`(?m)^Verdict: (pass|must-fix)$`)

// parseVerdict is the last line of the guard's final message that matches
// "Verdict: pass" or "Verdict: must-fix", or "" for none. A final of MaxFinal
// bytes or more was cut by the adapter, so its last line proves nothing: it is
// never parsed (10.6 item 12).
func parseVerdict(final []byte) string {
	m := verdictLine.FindAllSubmatch(final, -1)
	if len(m) == 0 || len(final) >= runtime.MaxFinal {
		return ""
	}
	return string(m[len(m)-1][1])
}

// guardTaskLines replace the commit rules of TaskLines (T-023 10.2).
func guardTaskLines(req Request, baseOID string, lease comb.Lease) []string {
	return []string{
		"Ticket: " + req.Ticket,
		"Role: guard",
		"Branch: " + req.Branch,
		"Base: " + baseOID,
		"Head under review: " + req.Head,
		"Lease directories: " + strings.Join(lease.Directories, ", "),
		"Gate: " + lease.Gate,
		"Rules: do not commit; delete any probe files before your final message; end with `Verdict: pass` or `Verdict: must-fix` as the last line of your final message.",
	}
}

// guardTarget reads the builder's claim (active or expired; the guard writes
// none) and resolves the branch head once. The review is of that OID.
func guardTarget(ctx context.Context, cb ClaimBackend, g *Git, req Request) (lease comb.Lease, head string, err error) {
	if _, e := cb.ClaimOID(ctx, req.Ticket); e != nil && !errors.Is(e, comb.ErrNotFound) {
		return lease, "", claimError(e)
	}
	switch lease, err = cb.ReadClaim(ctx, req.Ticket); {
	case errors.Is(err, comb.ErrNotFound):
		return lease, "", failf(ExitClaim, "no claim for %s: the guard reads the builder's claim", req.Ticket)
	case err != nil:
		return lease, "", claimError(err)
	}
	rev, ok := g.branchRev(ctx, req.Branch)
	if !ok {
		return lease, "", failf(ExitUsage, "branch %s does not exist", req.Branch)
	}
	out, err := g.Run(ctx, "rev-parse", "--verify", rev+"^{commit}")
	return lease, strings.TrimSpace(string(out)), err
}
