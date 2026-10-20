package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/layout"
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
func guardTarget(ctx context.Context, cb ClaimBackend, g *Git, req Request) (lease comb.Lease, head, claimOID string, err error) {
	if claimOID, err = cb.ClaimOID(ctx, req.Ticket); err != nil && !errors.Is(err, comb.ErrNotFound) {
		return lease, "", "", claimError(err)
	}
	switch lease, err = cb.ReadClaim(ctx, req.Ticket); {
	case errors.Is(err, comb.ErrNotFound):
		return lease, "", "", failf(ExitClaim, "no claim for %s: the guard reads the builder's claim", req.Ticket)
	case err != nil:
		return lease, "", "", claimError(err)
	}
	rev, ok := g.branchRev(ctx, req.Branch)
	if !ok {
		return lease, "", "", failf(ExitUsage, "branch %s does not exist", req.Branch)
	}
	out, err := g.Run(ctx, "rev-parse", "--verify", rev+"^{commit}")
	return lease, strings.TrimSpace(string(out)), claimOID, err
}

// checkSecond is D-188: a second guard runs only after a builder run and a
// first guard run of this ticket, on the same base, and on a model that
// neither of them used. The must-fix count moves between guard runs when the
// queen commits a review, so the policy alone cannot promise that.
func checkSecond(repo, ticket, baseOID, model string) error {
	runs := filepath.Join(repo, filepath.FromSlash(layout.Worktrees), "runs")
	ents, _ := os.ReadDir(runs)
	latest := func(role string) (r Result, ok bool) {
		best := 0
		for _, e := range ents {
			n, found := strings.CutPrefix(e.Name(), ticket+"-"+role+"-")
			seq, err := strconv.Atoi(n)
			b, rerr := readUpTo(filepath.Join(runs, e.Name(), "result.json"), maxBrief)
			var c Result
			if found && err == nil && seq > best && rerr == nil && json.Unmarshal(b, &c) == nil && !c.Second {
				best, r, ok = seq, c, true
			}
		}
		return r, ok
	}
	b, okB := latest("builder")
	g, okG := latest("guard")
	switch {
	case !okB || !okG:
		return failf(ExitEnv, "--second needs a builder run and a first guard run of %s in %s/runs", ticket, layout.Worktrees)
	case b.BaseOID != baseOID || g.BaseOID != baseOID:
		return failf(ExitEnv, "--second: base %s differs from the recorded runs (builder %s, first guard %s); pass --base %s", baseOID, b.BaseOID, g.BaseOID, b.BaseOID)
	case model == b.Model || model == g.Model:
		return failf(ExitEnv, "--second would use %s, the model of the builder (%s) or the first guard (%s)", model, b.Model, g.Model)
	}
	return nil
}
