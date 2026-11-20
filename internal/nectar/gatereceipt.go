// SPDX-License-Identifier: Apache-2.0

// Package nectar holds the Nectar code that is not the scorer yet: the
// post-merge gate-run receipt writer of T-051 phase D (RFC-0005 6.2).
package nectar

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/dispatch"
	"github.com/waggle-sh/waggle/internal/forge"
	"github.com/waggle-sh/waggle/internal/layout"
)

// ErrNoForge is a CommitLookup answer for "nothing to ask" (provider none, no
// auth, a forge without the lookup).
var ErrNoForge = errors.New("no forge to ask")

// CommitLookup is the forge read a gate-run receipt makes once per merge: the
// pull request whose merge commit is sha. found is false when the forge has no
// such merged pull request. HeadBranch has a "<owner>/<repo>:" prefix for a
// fork head, as a claim's branch list does.
type CommitLookup interface {
	MergedPRByCommit(ctx context.Context, sha string) (forge.MergedPull, bool, error)
}

// Backend is the Comb as the gate-run writer uses it.
type Backend interface {
	ReadClaimRecords(ctx context.Context) ([]comb.ClaimRecord, error)
	ReadReceipts(ctx context.Context) ([]comb.ReceiptEntry, error)
	AppendReceipts(ctx context.Context, recs []comb.ReceiptAppend) ([]string, error)
}

// Gate-run item states.
const (
	GateWritten    = "written"   // appended by this run
	GatePresent    = "present"   // a receipt for the commit or its task is already there
	GateNoClaim    = "no_claim"  // the task has no live claim; the merge stays unattributed
	GateNotCredit  = "not_valid" // the receipt would not pass its checks
	gateMaxCommits = 5000        // first-parent commits read from one push
)

// GateReceiptRequest is one `waggle nectar gate-receipt` run.
type GateReceiptRequest struct {
	RepoDir, GitBin string
	Before, After   string       // the push event's before and after SHAs
	Actor           string       // released_by: the push event's actor (github.actor)
	Backend         Backend      // nil: built from config
	Forge           CommitLookup // nil: built from config
}

// GateItem is one merge of the push.
type GateItem struct {
	Task      string
	MergedSHA string
	State     string
	Evidence  string
	Proof     string
	PR        int // 0 when the forge gave none
}

// GateReceiptResult is what a run found and wrote.
type GateReceiptResult struct {
	Items    []GateItem
	Written  int
	Warnings []string
}

var shaArg = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// Error classes the CLI maps to exit codes.
var (
	ErrUsage = errors.New("usage")
	ErrEnv   = errors.New("environment")
)

func usagef(format string, a ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, a...), ErrUsage)
}

func envf(format string, a ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, a...), ErrEnv)
}

// forgeByCommit is the CommitLookup built from config, on first use.
type forgeByCommit struct {
	provider string
	opt      forge.Options
	timeout  time.Duration
}

func (l forgeByCommit) MergedPRByCommit(ctx context.Context, sha string) (forge.MergedPull, bool, error) {
	if l.timeout == 0 {
		l.timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()
	f, err := forge.New(l.provider, l.opt)
	if errors.Is(err, forge.ErrNoForgeAuth) {
		return forge.MergedPull{}, false, ErrNoForge
	} else if err != nil {
		return forge.MergedPull{}, false, err
	}
	cf, ok := f.(forge.CommitPRFinder)
	if !ok {
		return forge.MergedPull{}, false, ErrNoForge
	}
	return cf.MergedPRByCommit(ctx, sha)
}

// gateRun is a run of consecutive first-parent commits of one task: the
// rebase-merge form lands several commits, and the receipt names the last.
type gateRun struct {
	task        string
	first, last logCommit
}

// GateReceipt writes a gate_run receipt for each task that landed on main in
// before..after, binds by commit subject or trailer, has a live claim and has
// no receipt yet (RFC-0005 6.2 "Claims that expire or are never released"). It
// never releases or changes a claim. All receipts go in one atomic append; a
// task that has a receipt by then (a release that landed first) is left alone.
// The forge is read once per merge; when it cannot be read the receipt's forge
// fields stay null and it is marked incomplete.
func GateReceipt(ctx context.Context, r GateReceiptRequest) (res GateReceiptResult, err error) {
	if !shaArg.MatchString(r.After) {
		return res, usagef("--after %q is not a full commit id", r.After)
	}
	zeroBefore := strings.Trim(r.Before, "0") == "" && r.Before != ""
	if !zeroBefore && !shaArg.MatchString(r.Before) {
		return res, usagef("--before %q is not a full commit id", r.Before)
	}
	if !comb.ValidKeeperID(r.Actor) {
		return res, usagef("actor %q is not a valid keeper id (released_by)", r.Actor)
	}
	repo, err := filepath.Abs(r.RepoDir)
	if err != nil {
		return res, envf("%v", err)
	}
	cfg, err := config.Load(filepath.Join(repo, filepath.FromSlash(layout.Config)))
	if err != nil {
		return res, envf("%v", err)
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
			return res, envf("%v", oerr)
		}
		cb = gb
	}
	lookup := r.Forge
	if lookup == nil {
		lookup = forgeByCommit{provider: cfg.Forge.Provider, opt: forge.Options{RepoDir: repo, Remote: cfg.Forge.Remote}}
	}
	g := &dispatch.Git{Bin: r.GitBin, Dir: repo, Remote: cfg.Forge.Remote}

	// A new branch push has no before: only the pushed commit is looked at.
	rng := []string{r.Before + ".." + r.After}
	if zeroBefore {
		rng = []string{"-n", "1", r.After}
	}
	out, err := g.Run(ctx, append([]string{"log", "--first-parent", "--reverse", fmt.Sprintf("--max-count=%d", gateMaxCommits), bindLogFormat}, rng...)...)
	if err != nil {
		return res, envf("%v", err)
	}
	commits, err := parseLog(string(out))
	if err != nil {
		return res, envf("%v", err)
	}
	var runs []gateRun
	for _, c := range commits {
		task, err := mergeTask(ctx, g, c)
		if err != nil {
			return res, envf("%v", err)
		}
		if task == "" {
			continue
		}
		if n := len(runs); n > 0 && runs[n-1].task == task && runs[n-1].last.oid == lastOf(commits, c.oid) {
			runs[n-1].last = c
			continue
		}
		runs = append(runs, gateRun{task, c, c})
	}
	if len(runs) == 0 {
		return res, nil
	}

	receipts, err := cb.ReadReceipts(ctx)
	if err != nil {
		return res, envf("%v", err)
	}
	merged, tasked := map[string]bool{}, map[string]bool{}
	for _, e := range receipts {
		tasked[e.Receipt.TicketID] = true
		if e.Receipt.MergedSHA != nil {
			merged[*e.Receipt.MergedSHA] = true
		}
	}
	records, err := cb.ReadClaimRecords(ctx)
	if err != nil {
		return res, envf("%v", err)
	}

	var appends []comb.ReceiptAppend
	forgeFailed := 0
	for _, run := range runs {
		item := GateItem{Task: run.task, MergedSHA: run.last.oid}
		switch {
		case merged[run.last.oid] || tasked[run.task]:
			item.State, item.Evidence = GatePresent, "a receipt already names this commit or this task"
			res.Items = append(res.Items, item)
			continue
		}
		var claim *comb.ClaimRecord
		for i, c := range records {
			if c.Source == comb.ClaimFromLive && c.Lease.TicketID == run.task {
				claim = &records[i]
				break
			}
		}
		if claim == nil {
			item.State, item.Evidence = GateNoClaim, "no live claim for the task, so the merge stays unattributed"
			res.Items = append(res.Items, item)
			continue
		}
		l := claim.Lease
		if run.last.when.Before(firstClaimed(ctx, g, claim.OID, l.ClaimedAt)) {
			item.State, item.Evidence = GateNoClaim, "the commit is older than the live claim, so it is not the claim's merge"
			res.Items = append(res.Items, item)
			continue
		}
		claimed, at := l.ClaimedAt, run.last.when
		sha, base := run.last.oid, ""
		if len(run.first.parents) > 0 {
			base = run.first.parents[0]
		}
		rec := comb.Receipt{
			Version: comb.ReceiptVersion, Source: comb.ReceiptSourceGateRun, TicketID: run.task,
			KeeperID: l.KeeperID, CoKeepers: []string{}, Role: l.Role, Gate: l.Gate,
			ClaimedAt: &claimed, Branches: append([]string{}, l.Branches...),
			BaseSHA: &base, MergedSHA: &sha, MergedAt: &at,
			Approvers: []string{}, ReleasedBy: r.Actor,
			Status: comb.ReceiptMerged, MergeProof: comb.ProofBinding, Incomplete: true,
		}
		if base == "" {
			item.State, item.Evidence = GateNotCredit, "the commit has no parent to use as base_sha"
			res.Items = append(res.Items, item)
			continue
		}
		pr, ok, ferr := lookup.MergedPRByCommit(ctx, sha)
		switch {
		case errors.Is(ferr, ErrNoForge):
		case ferr != nil:
			forgeFailed++
		default:
			rec.Incomplete = false
			if ok && slices.Contains(l.Branches, pr.HeadBranch) {
				n, author, head := pr.Number, pr.Author, pr.HeadBranch
				rec.PRNumber, rec.PRAuthor, rec.HeadBranch = &n, &author, &head
				rec.Approvers = append([]string{}, pr.Approvers...)
				item.PR = n
			}
		}
		if err := rec.Validate(len(sha)); err != nil && rec.PRNumber != nil {
			// A forge answer the receipt rules refuse is dropped, as a release does.
			rec.PRNumber, rec.PRAuthor, rec.HeadBranch, rec.Approvers, rec.Incomplete = nil, nil, nil, []string{}, true
			item.PR = 0
		}
		if err := rec.Validate(len(sha)); err != nil {
			item.State, item.Evidence = GateNotCredit, "the receipt would not pass its checks ("+err.Error()+")"
			res.Items = append(res.Items, item)
			continue
		}
		item.State, item.Proof = GateWritten, comb.ProofBinding
		item.Evidence = fmt.Sprintf("live claim by %s at claim commit %s", l.KeeperID, claim.OID[:min(len(claim.OID), 12)])
		tasked[run.task] = true
		appends = append(appends, comb.ReceiptAppend{Receipt: rec, ClaimOID: claim.OID})
		res.Items = append(res.Items, item)
	}
	if forgeFailed > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("could not read the forge for %d merge(s); their receipts are marked incomplete", forgeFailed))
	}
	if len(appends) == 0 {
		return res, nil
	}
	written, err := cb.AppendReceipts(ctx, appends)
	if err != nil {
		return res, envf("%v", err)
	}
	for i := range res.Items {
		if res.Items[i].State != GateWritten {
			continue
		}
		if slices.Contains(written, res.Items[i].Task) {
			res.Written++
		} else {
			res.Items[i].State, res.Items[i].Evidence = GatePresent, "a release wrote a receipt first"
		}
	}
	return res, nil
}

// lastOf returns the oid of the commit that precedes oid in commits ("" for the first).
func lastOf(commits []logCommit, oid string) string {
	for i, c := range commits {
		if c.oid == oid && i > 0 {
			return commits[i-1].oid
		}
	}
	return ""
}

// combEnv is the Comb's extra environment: no hooks and no auto maintenance
// from the repo's own config (same as internal/dispatch).
var combEnv = []string{"GIT_CONFIG_PARAMETERS=", "GIT_CONFIG_COUNT=3", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0=/dev/null",
	"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false", "GIT_CONFIG_KEY_2=gc.auto", "GIT_CONFIG_VALUE_2=0"}
