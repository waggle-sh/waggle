// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// MergedPR is what the forge says about the merged pull request of a task
// (RFC-0005 section 6.2). HeadBranch must be the entry of the claim's branch
// list that matched (a fork entry keeps its <owner>/<repo>: prefix), MergeSHA
// the commit the merge left on main (the last commit of a rebase run), and
// Approvers every login whose latest review is APPROVED at the merged head and
// not dismissed.
type MergedPR struct {
	Number                       int
	Author, HeadBranch, MergeSHA string
	Approvers                    []string
}

// PRLookup is the forge read a release makes once, when it writes the receipt.
// The merge commit it names is checked against main here, so an adapter need
// not. ErrNoForge means there is no forge to ask (provider none, no auth); any
// other error is a failed read. found is false when the forge has no merged PR
// for any of the branches.
type PRLookup interface {
	MergedPRByHead(ctx context.Context, branches []string) (pr MergedPR, found bool, err error)
}

// ErrNoForge is a PRLookup answer for "nothing to ask".
var ErrNoForge = errors.New("no forge to ask")

// mergeFacts is what the resolver found for one ticket (T-051 section 3).
type mergeFacts struct {
	Status, Proof      string
	MergedSHA, BaseSHA string
	MergedAt           time.Time
	PR                 *MergedPR
	Incomplete         bool
	ForgeWarning       bool // the forge read failed: one warning, the receipt is incomplete
}

var bindSubject = regexp.MustCompile(`^(T-[0-9]{3,6}): `)

// bindTask is the task one commit binds to by the gate's rules for a subject
// prefix and a Waggle-Ticket trailer (cmd/waggle/gate_commits.go: ticketSubject,
// trailerTickets, classify): a "Head Keeper: " subject binds none, a bad
// trailer binds none, and a subject and trailer that name different tasks bind
// none. The gate's copy is in package main and cannot be imported; keep the two
// the same. The ticket-branch form needs a PR branch, which main history does
// not hold, so it is not read here.
func bindTask(subject, trailer string) string {
	if strings.HasPrefix(subject, "Head Keeper: ") {
		return ""
	}
	id := ""
	if m := bindSubject.FindStringSubmatch(subject); m != nil {
		id, _ = comb.ParseTicketID(m[1])
	}
	if trailer == "" {
		return id
	}
	if len(trailer) > 256 || strings.Count(trailer, ",")+1 > 8 {
		return ""
	}
	// A trailer naming more than one ticket binds to none, on purpose: the gate
	// rejects such a commit, so no one task can be credited with it.
	for _, v := range strings.Split(trailer, ",") {
		v = strings.Trim(v, " \t")
		t, err := comb.ParseTicketID(v)
		if err != nil || len(v) > 16 || (id != "" && t != id) {
			return ""
		}
		id = t
	}
	return id
}

// bindLogFormat reads hash, parents, committer time, subject and the trailer field.
const bindLogFormat = "--format=%H%x1f%P%x1f%ct%x1f%s%x1f%(trailers:key=Waggle-Ticket,valueonly,unfold,separator=%x2C)%x1e"

type logCommit struct {
	oid     string
	parents []string
	when    time.Time
	task    string // what the commit binds to by itself
}

func parseLog(out string) ([]logCommit, error) {
	var cs []logCommit
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		f := strings.Split(rec, "\x1f")
		if len(f) != 5 {
			return nil, errors.New("unexpected git log record")
		}
		ct, err := strconv.ParseInt(f[2], 10, 64)
		if err != nil {
			return nil, err
		}
		cs = append(cs, logCommit{oid: f[0], parents: strings.Fields(f[1]), when: time.Unix(ct, 0).UTC(), task: bindTask(f[3], strings.TrimSpace(f[4]))})
	}
	return cs, nil
}

// mergeTask is the task a first-parent commit of main binds to as a unit: its
// own binding, and for a merge commit the one task all of its non-merge
// commits bind to (a merge commit binds through them, as the gate rejects
// mixed tasks).
func mergeTask(ctx context.Context, pg *Git, c logCommit) (string, error) {
	if len(c.parents) < 2 {
		return c.task, nil
	}
	out, err := pg.Run(ctx, "log", "--no-merges", bindLogFormat, c.parents[0]+".."+c.oid)
	if err != nil {
		return "", err
	}
	side, err := parseLog(string(out))
	if err != nil {
		return "", err
	}
	id := ""
	for _, s := range side {
		if s.task == "" || (id != "" && s.task != id) {
			return "", nil
		}
		id = s.task
	}
	switch {
	case id == "":
		return c.task, nil
	case c.task != "" && c.task != id:
		return "", nil
	}
	return id, nil
}

// bindingRun is the git-only resolver step (RFC-0005 6.2 step 2): the runs of
// consecutive first-parent commits on main, committed at or after claimedAt,
// that bind to task. One run gives its last commit and the first parent of its
// first commit; none, or more than one (separate runs differ in PR), gives
// found false.
func bindingRun(ctx context.Context, pg *Git, main, task string, claimedAt time.Time) (last logCommit, base string, found bool, err error) {
	out, err := pg.Run(ctx, "log", "--first-parent", "--reverse", bindLogFormat, main)
	if err != nil {
		return logCommit{}, "", false, err
	}
	cs, err := parseLog(string(out))
	if err != nil {
		return logCommit{}, "", false, err
	}
	runs, inRun := 0, false
	for _, c := range cs {
		bound := false
		if !c.when.Before(claimedAt) {
			t, err := mergeTask(ctx, pg, c)
			if err != nil {
				return logCommit{}, "", false, err
			}
			bound = t == task
		}
		if !bound {
			inRun = false
			continue
		}
		if !inRun {
			if runs++; runs == 1 && len(c.parents) > 0 {
				base = c.parents[0]
			}
		}
		inRun, last = true, c
	}
	return last, base, runs == 1 && base != "", nil
}

// onFirstParentLine reports whether oid is on main's first-parent history.
func onFirstParentLine(ctx context.Context, pg *Git, main, oid string) (bool, error) {
	out, err := pg.Run(ctx, "rev-list", "--first-parent", main)
	if err != nil {
		return false, err
	}
	return slices.Contains(strings.Fields(string(out)), oid), nil
}

// commitFacts returns the first parent and committer time of oid.
func commitFacts(ctx context.Context, pg *Git, oid string) (base string, at time.Time, err error) {
	out, err := pg.Run(ctx, "log", "-1", bindLogFormat, oid)
	if err != nil {
		return "", time.Time{}, err
	}
	cs, err := parseLog(string(out))
	if err != nil || len(cs) != 1 || len(cs[0].parents) == 0 {
		return "", time.Time{}, fmt.Errorf("commit %s has no parent to use as base", oid)
	}
	return cs[0].parents[0], cs[0].when, nil
}

// firstClaimed is when the claim chain at claimOID began. A renewal or takeover
// is an AcquireLease that chains to the previous claim commit and, unless the
// caller passes ClaimedAt, sets claimed_at to now, so the binding step looks
// back to the oldest commit of the chain. since is the fallback.
func firstClaimed(ctx context.Context, pg *Git, claimOID string, since time.Time) time.Time {
	out, err := pg.Run(ctx, "log", "--format=%ct", claimOID)
	if err != nil {
		return since
	}
	for _, s := range strings.Fields(string(out)) {
		if ct, err := strconv.ParseInt(s, 10, 64); err == nil && time.Unix(ct, 0).Before(since) {
			since = time.Unix(ct, 0).UTC()
		}
	}
	return since
}

// resolveMerge finds merged_sha in RFC-0005 6.2 order: forge, binding, the
// merged tip (mergeCommit, "" when branchMerged did not run or proved nothing),
// none. main is main's commit OID, "" when it could not be resolved: then only
// "none" is possible. It never fails the release: a git or forge trouble
// falls through to the next step, and a forge failure sets ForgeWarning. The
// receipt is Incomplete unless the forge answered. base_sha is the first parent
// of the merged commit, or of the first commit of its run for the binding step.
func resolveMerge(ctx context.Context, pg *Git, main string, lease comb.Lease, since time.Time, forge PRLookup, mergeCommit string) mergeFacts {
	f := mergeFacts{Status: comb.ReceiptUnmerged, Proof: comb.ProofNone, Incomplete: true}
	if main == "" {
		return f
	}
	set := func(sha, base, proof string, at time.Time) {
		f.Status, f.Proof, f.MergedSHA, f.BaseSHA, f.MergedAt = comb.ReceiptMerged, proof, sha, base, at
	}
	if forge != nil && len(lease.Branches) > 0 {
		pr, ok, err := forge.MergedPRByHead(ctx, lease.Branches)
		switch {
		case errors.Is(err, ErrNoForge):
		case err != nil:
			f.ForgeWarning = true
		default:
			f.Incomplete = false
			if ok && slices.Contains(lease.Branches, pr.HeadBranch) {
				on, err := onFirstParentLine(ctx, pg, main, pr.MergeSHA)
				if base, at, ferr := commitFacts(ctx, pg, pr.MergeSHA); err == nil && on && ferr == nil {
					set(pr.MergeSHA, base, comb.ProofForge, at)
					f.PR = &pr
					return f
				}
			}
		}
	}
	if last, base, ok, err := bindingRun(ctx, pg, main, lease.TicketID, since); err == nil && ok {
		set(last.oid, base, comb.ProofBinding, last.when)
		return f
	}
	if mergeCommit != "" {
		if base, at, err := commitFacts(ctx, pg, mergeCommit); err == nil {
			set(mergeCommit, base, comb.ProofMergedTip, at)
		}
	}
	return f
}

// receipt builds the receipt of a release from f. The claim fields are filled
// by the Comb from the claim itself.
func (f mergeFacts) receipt(owner string, co []string) comb.Receipt {
	r := comb.Receipt{
		Version: comb.ReceiptVersion, Source: comb.ReceiptSourceRelease,
		CoKeepers: append([]string{}, co...), Approvers: []string{}, ReleasedBy: owner,
		Status: f.Status, MergeProof: f.Proof, Incomplete: f.Incomplete,
	}
	if f.MergedSHA != "" {
		sha, base, at := f.MergedSHA, f.BaseSHA, f.MergedAt.UTC()
		r.MergedSHA, r.BaseSHA, r.MergedAt = &sha, &base, &at
	}
	if pr := f.PR; pr != nil {
		n, author, head := pr.Number, pr.Author, pr.HeadBranch
		r.PRNumber, r.PRAuthor, r.HeadBranch = &n, &author, &head
		r.Approvers = append([]string{}, pr.Approvers...)
	}
	return r
}
