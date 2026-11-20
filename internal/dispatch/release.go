// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
)

// ReleaseRequest asks to give back one ticket's claim.
type ReleaseRequest struct {
	RepoDir, GitBin, Ticket string
	Branch                  string       // default t-<nnn>, else waggle/T-<nnn> (see ReleaseTask)
	Unmerged                bool         // release without the merged check: the ticket makes no PR
	Owner                   string       // default keeper_id from .waggle/local.yaml
	Backend                 ClaimBackend // nil: built from config
	CoKeepers               []string     // credited with the owner in the receipt (T-051): at most 4, not the owner
	Forge                   PRLookup     // read once for the receipt's PR fields; nil: none (the receipt is incomplete)
}

// ReleaseTaskResult is what ReleaseTask did.
type ReleaseTaskResult struct {
	Task     string
	Outcome  string // released or no_claim
	OID      string // the claim commit that was released; empty for no_claim
	Merged   bool   // the merged check passed; false with Unmerged
	Branch   string // the head that passed the merged check, or ""
	Receipt  string // the receipt commit written with the release; "" when the backend writes none
	Warnings []string
}

// receiptBackend is a ClaimBackend that records a receipt with the release.
type receiptBackend interface {
	ClaimBackend
	ReleaseWithReceipt(ctx context.Context, ticketID, keeperID string, rec comb.Receipt) (string, error)
}

// errNotMerged and errTaskBusy tag the two refusals ErrorCode names. They are
// wrapped by taggedError, which keeps the message text as it always was.
var (
	errNotMerged = errors.New("branch not merged")
	errTaskBusy  = errors.New("task worktree or marker present")
)

type taggedError struct {
	msg string
	tag error
}

func (e taggedError) Error() string { return e.msg }
func (e taggedError) Unwrap() error { return e.tag }

// ReleaseLine is the one line the text output prints for a successful release.
func ReleaseLine(res ReleaseTaskResult) string {
	if res.Outcome == "no_claim" {
		return "no claim for " + res.Task
	}
	state := "unmerged"
	if res.Merged {
		state = "merged into main"
	}
	return fmt.Sprintf("released %s (claim %s, %s)", res.Task, printable(res.OID[:min(len(res.OID), 12)]), state)
}

// ReleaseTask deletes the owner's own claim for a ticket whose branch has merged
// into main (or, with Unmerged, whose branch never will). When
// .waggle/worktrees/ exists it runs under dispatch.lock and refuses while a
// worktree, lock, tainted marker or takeover guard of the ticket exists; when it
// is absent no dispatch run can be there, so it takes no lock and creates
// nothing. The delete goes through ReleaseLease, so the owner check and the
// remote compare-and-swap stay the Comb's. A claim that is not there is not an
// error. A backend that can record one (the Comb) also appends a receipt to
// refs/waggle/receipts/main in the same transaction (RFC-0005 6.2): merged_sha
// is found by the forge, then by the task binding on main, then by the merged
// tip, else the receipt is unmerged; a forge failure marks the receipt
// incomplete with one warning and never fails the release. Unmerged skips the
// merged check, not the receipt. Refusals are ExitClaim, a held lock or git trouble is ExitEnv. The
// result is filled as far as it got, also on error. A cancelled ctx turns an
// ExitEnv failure into ExitFailed (interrupted).
func ReleaseTask(ctx context.Context, r ReleaseRequest) (res ReleaseTaskResult, err error) {
	defer func() { err = interruptedEnv(ctx, err) }()
	id, err := comb.ParseTicketID(r.Ticket)
	if err != nil {
		return res, failf(ExitUsage, "%w", err)
	}
	res.Task = id
	if r.Branch != "" {
		if err = checkReleaseBranch(r.Branch, id); err != nil {
			return res, failf(ExitUsage, "%w", err)
		}
	}
	repo, err := filepath.Abs(r.RepoDir)
	if err != nil {
		return res, failf(ExitEnv, "%w", err)
	}
	pg, common, dir := (&Git{Bin: r.GitBin, Dir: repo}).pinned(), "", ""
	if _, serr := os.Lstat(filepath.Join(repo, filepath.FromSlash(layout.Worktrees))); !errors.Is(serr, fs.ErrNotExist) {
		var warns bytes.Buffer
		var unlock func()
		repo, pg, common, dir, unlock, err = lockRepo(ctx, r.RepoDir, r.GitBin, &warns)
		for _, l := range strings.Split(strings.TrimSpace(warns.String()), "\n") {
			if l != "" {
				res.Warnings = append(res.Warnings, strings.TrimPrefix(l, "warning: "))
			}
		}
		if err != nil {
			return res, err
		}
		defer unlock()
	}
	cfg, err := config.Load(filepath.Join(repo, filepath.FromSlash(layout.Config)))
	if err != nil {
		return res, failf(ExitEnv, "%w", err)
	}
	owner := r.Owner
	if owner == "" {
		if owner, err = config.LoadKeeperID(repo); err != nil {
			return res, failf(ExitUsage, "%w; or pass --owner <id>", err)
		}
	}
	if err = checkCoKeepers(r.CoKeepers, owner); err != nil {
		return res, err
	}
	if dir != "" {
		c := &cleaner{repo: repo, common: common}
		for _, role := range []string{"builder", "forager", "guard"} {
			wt := filepath.Join(dir, id+"-"+role)
			entry, err := c.entryFor(c.wants(wt))
			if err != nil {
				return res, err
			}
			if entry != "" || slices.ContainsFunc([]string{wt, wt + ".lock", wt + ".tainted", wt + ".lock.takeover"}, func(p string) bool { _, e := os.Lstat(p); return e == nil }) {
				return res, failf(ExitClaim, "%w", taggedError{fmt.Sprintf("%s-%s is still there; run waggle dispatch clean %s first", id, role, id), errTaskBusy})
			}
		}
	}
	cb := r.Backend
	if cb == nil {
		opts, err := cfg.CombOptions(repo)
		opts.GitBinary = r.GitBin
		opts.ExtraEnv = combEnv
		if err == nil {
			cb, err = comb.NewGitBackend(opts)
		}
		if err != nil {
			return res, failf(ExitEnv, "%w", err)
		}
	}
	oid, err := cb.ClaimOID(ctx, id)
	if errors.Is(err, comb.ErrNotFound) {
		res.Outcome = "no_claim"
		return res, nil
	} else if err != nil {
		return res, releaseError(err)
	}
	g := &Git{Bin: r.GitBin, Dir: repo, Remote: cfg.Forge.Remote}
	main, mergeCommit := "", ""
	if !r.Unmerged {
		branches := []string{r.Branch}
		if r.Branch == "" {
			branches = []string{"t-" + strings.TrimPrefix(id, "T-"), "waggle/" + id}
		}
		if main, err = ResolveBase(ctx, g, g.Remote, ""); err != nil {
			return res, err
		}
		head, merge, err := branchMerged(ctx, g, pg, main, branches)
		if err != nil {
			return res, err
		}
		if head == "" {
			return res, failf(ExitClaim, "%w", taggedError{fmt.Sprintf("branch %s is not merged into main (or is absent); the claim stays until it is; if the task merged by squash or rebase, or makes no PR, release it with --unmerged", branches[0]), errNotMerged})
		}
		res.Merged, res.Branch, mergeCommit = true, head, merge
	}
	rb, ok := cb.(receiptBackend)
	if !ok {
		if err = cb.ReleaseLease(ctx, id, owner); err != nil {
			return res, releaseError(err)
		}
		res.Outcome, res.OID = "released", oid
		return res, nil
	}
	if r.Unmerged {
		// The receipt still looks for the merge (forge, then binding); main
		// failing to resolve is not a refusal, the receipt is then unmerged.
		if m, merr := ResolveBase(ctx, g, g.Remote, ""); merr == nil {
			main = m
		}
	}
	lease, err := rb.ReadClaim(ctx, id)
	if err != nil {
		return res, releaseError(err)
	}
	facts := resolveMerge(ctx, pg, main, lease, firstClaimed(ctx, pg, oid, lease.ClaimedAt), r.Forge, mergeCommit)
	if facts.ForgeWarning {
		res.Warnings = append(res.Warnings, "could not read the forge for "+id+"; the receipt is marked incomplete")
	}
	res.Receipt, err = rb.ReleaseWithReceipt(ctx, id, owner, facts.receipt(owner, r.CoKeepers))
	if errors.Is(err, comb.ErrInvalidReceipt) && facts.PR != nil {
		// A forge answer the receipt rules refuse is dropped: it is no reason to keep the claim.
		facts.PR, facts.Incomplete = nil, true
		res.Warnings = append(res.Warnings, "the forge answer for "+id+" was not usable; the receipt is marked incomplete")
		res.Receipt, err = rb.ReleaseWithReceipt(ctx, id, owner, facts.receipt(owner, r.CoKeepers))
	}
	if err != nil {
		res.Receipt = ""
		return res, releaseError(err)
	}
	res.Outcome, res.OID = "released", oid
	return res, nil
}

// checkCoKeepers applies the receipt's co-keeper rules before anything is
// written: at most 4, well-formed ids, no repeat, never the owner (RFC-0005 6.3).
func checkCoKeepers(co []string, owner string) error {
	if len(co) > 4 {
		return failf(ExitUsage, "--co names %d keepers; at most 4", len(co))
	}
	for i, c := range co {
		switch {
		case !comb.ValidKeeperID(c):
			return failf(ExitUsage, "--co %q is not a keeper id", c)
		case c == owner:
			return failf(ExitUsage, "--co names the owner %s, who is always credited", owner)
		case slices.Index(co, c) != i:
			return failf(ExitUsage, "--co names %s twice", c)
		}
	}
	return nil
}

// checkReleaseBranch accepts the branch forms of checkBranch plus
// waggle/T-<nnn>[-suffix], each for this ticket's number only.
func checkReleaseBranch(branch, ticket string) error {
	rest, ok := strings.CutPrefix(branch, "waggle/T-")
	if !ok {
		return checkBranch(branch, ticket)
	}
	if checkBranch("t-"+rest, ticket) != nil || len(branch) > 100 {
		return fmt.Errorf("branch %q must be waggle/%s with an optional suffix of [A-Za-z0-9._-], at most 100 characters", branch, ticket)
	}
	return nil
}

// branchMerged checks the first of branches that exists against main (the
// commit the caller resolved after fetching the remote host side, with the
// Keeper's git auth): it returns that branch's name and the merge commit when
// its head is the second or a later parent of a merge commit on main's
// first-parent history, else "". Only that proves the head itself merged: a head
// whose tip is just a first parent has no commits of its own in main, and a
// merge off that history (another branch updated from main) proves nothing. Only
// the first existing head is checked, so a branch someone pushes at another PR's
// merged head cannot stand in for one that exists unmerged. The head is the
// remote's branch when it exists, else the local one; it is never read from a
// run record. The head and merge reads run pinned.
func branchMerged(ctx context.Context, g, pg *Git, main string, branches []string) (branch, merge string, err error) {
	head := ""
	for _, b := range branches {
		refs := []string{"refs/heads/" + b}
		if g.Remote != "" {
			refs = []string{"refs/remotes/" + g.Remote + "/" + b, refs[0]}
		}
		for _, ref := range refs {
			if out, err := pg.Run(ctx, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err == nil {
				branch, head = b, strings.TrimSpace(string(out))
				break
			}
		}
		if head != "" {
			break
		}
	}
	if head == "" {
		return "", "", nil
	}
	out, err := pg.Run(ctx, "rev-list", "--first-parent", "--parents", "--merges", head+".."+main)
	if err != nil {
		return "", "", failf(ExitEnv, "%w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) > 2 && slices.Contains(f[2:], head) {
			return branch, f[0], nil
		}
	}
	return "", "", nil
}

// releaseError maps a Comb error from the release to its dispatch exit code.
func releaseError(err error) error {
	switch {
	case errors.Is(err, comb.ErrNotOwner), errors.Is(err, comb.ErrConcurrentUpdate):
		return failf(ExitClaim, "%w", err)
	}
	return claimError(err)
}
