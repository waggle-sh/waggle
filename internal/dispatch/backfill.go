// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/decisions"
	"github.com/waggle-sh/waggle/internal/layout"
)

// Backfill item states.
const (
	BackfillPlanned      = "planned"      // a receipt the plan would write
	BackfillWritten      = "written"      // appended by --apply
	BackfillPresent      = "present"      // a receipt for the task landed first; nothing written
	BackfillUnattributed = "unattributed" // no source names an owner and a merge; no receipt
)

// Where a backfilled owner came from.
const (
	BackfillLiveClaim    = "live claim"
	BackfillClaimHistory = "claim history"
	BackfillDecisions    = "DECISIONS.md"
)

// BackfillBackend is what backfill reads and writes: the Comb.
type BackfillBackend interface {
	ReadClaimRecords(ctx context.Context) ([]comb.ClaimRecord, error)
	ReadReceipts(ctx context.Context) ([]comb.ReceiptEntry, error)
	AppendReceipts(ctx context.Context, recs []comb.ReceiptAppend) ([]string, error)
}

// BackfillRequest is one `waggle nectar backfill` run.
type BackfillRequest struct {
	RepoDir, GitBin string
	Apply           bool
	Owner           string          // keeper_id credited for a task only DECISIONS.md names; "" leaves those unattributed
	ReleasedBy      string          // default keeper_id from .waggle/local.yaml (needed with Apply only)
	Backend         BackfillBackend // nil: built from config
	Forge           PRLookup        // read once per task with a claim; nil: none (those receipts are incomplete)
}

// BackfillItem is one task of the plan.
type BackfillItem struct {
	Task      string
	State     string // a Backfill state constant
	Owner     string // keeper_id; "" when unattributed
	Origin    string // BackfillLiveClaim, BackfillClaimHistory or BackfillDecisions; "" when unattributed
	MergedSHA string // "" when unattributed
	Proof     string // merge_proof; "" when unattributed
	Evidence  string
}

// BackfillResult is the plan, and with Apply what was written.
type BackfillResult struct {
	Items    []BackfillItem
	Applied  bool
	Written  int
	Warnings []string
}

// Backfill builds receipts for tasks that have a credited merge on main and no
// receipt (RFC-0005 6.2 "Backfill"), marked source backfill. Owners come from
// three sources, in this order: a live claim ref; a reachable claim, that is a
// second parent on the force-release audit chain or the receipt chain; and a
// DECISIONS.md entry that names the task in its Tickets field, read at main,
// which credits Owner and only when Owner is set. merged_sha is found by the
// release resolver (forge, then the task binding on main) and never from the
// merged tip. A task no source names, or whose merge the resolver cannot find,
// is unattributed and gets no receipt. Without Apply nothing is written. With
// Apply the planned receipts go in one atomic append; a task that has a
// receipt by then is left alone, and an existing receipt is never changed.
func Backfill(ctx context.Context, r BackfillRequest) (res BackfillResult, err error) {
	defer func() { err = interruptedEnv(ctx, err) }()
	if r.Owner != "" && !comb.ValidKeeperID(r.Owner) {
		return res, failf(ExitUsage, "--owner %q is not a keeper id", r.Owner)
	}
	repo, err := filepath.Abs(r.RepoDir)
	if err != nil {
		return res, failf(ExitEnv, "%w", err)
	}
	cfg, err := config.Load(filepath.Join(repo, filepath.FromSlash(layout.Config)))
	if err != nil {
		return res, failf(ExitEnv, "%w", err)
	}
	releasedBy := r.ReleasedBy
	if r.Apply && releasedBy == "" {
		if releasedBy, err = config.LoadKeeperID(repo); err != nil {
			return res, failf(ExitUsage, "%w; --apply records who ran it", err)
		}
	}
	if r.Apply && !comb.ValidKeeperID(releasedBy) {
		return res, failf(ExitUsage, "keeper id %q is not valid", releasedBy)
	}
	if !r.Apply && !comb.ValidKeeperID(releasedBy) {
		releasedBy = "plan-only" // a plan writes nothing; this lets its receipts be checked
	}
	cb := r.Backend
	if cb == nil {
		opts, oerr := cfg.CombOptions(repo)
		opts.GitBinary = r.GitBin
		opts.ExtraEnv = combEnv
		var gb *comb.GitBackend
		if oerr == nil {
			gb, oerr = comb.NewGitBackend(opts)
		}
		if oerr != nil {
			return res, failf(ExitEnv, "%w", oerr)
		}
		cb = gb
	}
	g := &Git{Bin: r.GitBin, Dir: repo, Remote: cfg.Forge.Remote}
	pg := g.pinned()
	main, err := ResolveBase(ctx, g, g.Remote, "")
	if err != nil {
		return res, err
	}
	receipts, err := cb.ReadReceipts(ctx)
	if err != nil {
		return res, failf(ExitEnv, "%w", err)
	}
	have := map[string]bool{}
	for _, e := range receipts {
		have[e.Receipt.TicketID] = true
	}
	records, err := cb.ReadClaimRecords(ctx)
	if err != nil {
		return res, failf(ExitEnv, "%w", err)
	}
	tasks, err := tasksOnMain(ctx, pg, main)
	if err != nil {
		return res, failf(ExitEnv, "%w", err)
	}
	named, warn := decisionTickets(ctx, pg, main)
	if warn != "" {
		res.Warnings = append(res.Warnings, warn)
	}

	var appends []comb.ReceiptAppend
	forgeFailed := 0
	for _, task := range tasks {
		if have[task] {
			continue
		}
		item := BackfillItem{Task: task, State: BackfillUnattributed}
		var rec comb.Receipt
		claimOID := ""
		built := false
		if cr, ok := pickClaim(records, task); ok {
			since := firstClaimed(ctx, pg, cr.OID, cr.Lease.ClaimedAt)
			facts := resolveMerge(ctx, pg, main, cr.Lease, since, r.Forge, "")
			if facts.ForgeWarning {
				forgeFailed++
			}
			item.Origin = BackfillClaimHistory
			if cr.Source == comb.ClaimFromLive {
				item.Origin = BackfillLiveClaim
			}
			item.Evidence = fmt.Sprintf("%s by %s at claim commit %s", item.Origin, cr.Lease.KeeperID, short(cr.OID))
			if facts.Status == comb.ReceiptUnmerged {
				item.Origin = ""
				item.Evidence += "; the merge on main could not be matched to the claim, so no receipt"
			} else {
				rec = facts.receipt(releasedBy, nil)
				rec.KeeperID, rec.Role, rec.Gate = cr.Lease.KeeperID, cr.Lease.Role, cr.Lease.Gate
				claimed := cr.Lease.ClaimedAt
				rec.ClaimedAt, rec.Branches = &claimed, append([]string{}, cr.Lease.Branches...)
				claimOID, built = cr.OID, true
			}
		} else if ds := named[task]; len(ds) > 0 {
			item.Evidence = "named by " + strings.Join(ds, ", ")
			if r.Owner == "" {
				item.Evidence += "; pass --owner <id> to credit it"
			} else {
				// No claim, so no branches: the forge is not asked and the receipt is incomplete.
				facts := resolveMerge(ctx, pg, main, comb.Lease{TicketID: task}, time.Time{}, nil, "")
				if facts.Status == comb.ReceiptUnmerged {
					item.Evidence += "; the merge on main could not be found, so no receipt"
				} else {
					item.Origin = BackfillDecisions
					rec = facts.receipt(releasedBy, nil)
					rec.KeeperID, rec.Branches = r.Owner, []string{}
					built = true
				}
			}
		} else {
			item.Evidence = "no claim, no receipt and no DECISIONS.md entry names it"
		}
		if built {
			rec.Version, rec.Source, rec.TicketID = comb.ReceiptVersion, comb.ReceiptSourceBackfill, task
			if err := rec.Validate(len(main)); err != nil && rec.PRNumber != nil {
				// A forge answer the receipt rules refuse is dropped, as a release does.
				rec.PRNumber, rec.PRAuthor, rec.HeadBranch, rec.Approvers, rec.Incomplete = nil, nil, nil, []string{}, true
			}
			if err := rec.Validate(len(main)); err != nil {
				item.Origin = ""
				item.Evidence += "; the receipt would not pass its checks (" + err.Error() + "), so no receipt"
			} else {
				item.State, item.Owner = BackfillPlanned, rec.KeeperID
				item.MergedSHA, item.Proof = *rec.MergedSHA, rec.MergeProof
				appends = append(appends, comb.ReceiptAppend{Receipt: rec, ClaimOID: claimOID})
			}
		}
		res.Items = append(res.Items, item)
	}
	if forgeFailed > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("could not read the forge for %d task(s); their receipts are marked incomplete", forgeFailed))
	}
	if !r.Apply {
		return res, nil
	}
	res.Applied = true
	written, err := cb.AppendReceipts(ctx, appends)
	if err != nil {
		return res, failf(ExitEnv, "%w", err)
	}
	for i := range res.Items {
		if res.Items[i].State != BackfillPlanned {
			continue
		}
		if slices.Contains(written, res.Items[i].Task) {
			res.Items[i].State = BackfillWritten
			res.Written++
		} else {
			res.Items[i].State = BackfillPresent
		}
	}
	return res, nil
}

func short(oid string) string { return oid[:min(len(oid), 12)] }

// pickClaim chooses the claim that names the owner of task: a live claim, else
// the newest claim by claimed_at from the history (the first listed on a tie).
func pickClaim(records []comb.ClaimRecord, task string) (comb.ClaimRecord, bool) {
	var best comb.ClaimRecord
	found := false
	for _, c := range records {
		if c.Lease.TicketID != task {
			continue
		}
		switch {
		case !found:
		case c.Source == comb.ClaimFromLive && best.Source != comb.ClaimFromLive:
		case best.Source != comb.ClaimFromLive && c.Lease.ClaimedAt.After(best.Lease.ClaimedAt):
		default:
			continue
		}
		best, found = c, true
	}
	return best, found
}

// tasksOnMain lists the tasks that bind to a commit on main's first-parent
// line, in the order of their first commit.
func tasksOnMain(ctx context.Context, pg *Git, main string) ([]string, error) {
	out, err := pg.Run(ctx, "log", "--first-parent", "--reverse", bindLogFormat, main)
	if err != nil {
		return nil, err
	}
	cs, err := parseLog(string(out))
	if err != nil {
		return nil, err
	}
	var tasks []string
	seen := map[string]bool{}
	for _, c := range cs {
		t, err := mergeTask(ctx, pg, c)
		if err != nil {
			return nil, err
		}
		if t != "" && !seen[t] {
			seen[t] = true
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

// decisionTickets reads DECISIONS.md at main and returns, per task, the
// decision ids whose Tickets field names it. A missing file is no entries; a
// file that does not parse is no entries and a warning.
func decisionTickets(ctx context.Context, pg *Git, main string) (map[string][]string, string) {
	spec := main + ":" + layout.Decisions
	if _, err := pg.Run(ctx, "cat-file", "-e", spec); err != nil {
		return nil, ""
	}
	data, err := pg.Run(ctx, "show", spec)
	if err != nil {
		return nil, "could not read " + layout.Decisions + " at main; it is not used as a source"
	}
	entries, err := decisions.Parse(data)
	if err != nil {
		return nil, layout.Decisions + " does not parse; it is not used as a source"
	}
	named := map[string][]string{}
	for _, e := range entries {
		for _, n := range e.Tickets {
			if id, err := comb.ParseTicketID(fmt.Sprintf("T-%d", n)); err == nil {
				named[id] = append(named[id], fmt.Sprintf("D-%03d", e.ID))
			}
		}
	}
	for _, ds := range named {
		sort.Strings(ds)
	}
	return named, ""
}
