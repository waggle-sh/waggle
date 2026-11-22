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
	"regexp"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
)

// ForceReleaseRequest asks to inspect or clear one claim ref regardless of its
// owner or shape (waggle force-release, T-072). Exactly one of Ticket and Ref
// names the target. Without OID it only inspects; with OID it deletes the ref
// by compare-and-swap against that OID and records Reason in the audit chain.
type ForceReleaseRequest struct {
	RepoDir, GitBin string
	Ticket          string // comb.ParseTicketID forms
	Ref             string // full ref under refs/waggle/claims/, possibly non-canonical
	OID             string // the inspected object id; empty means inspect
	Reason          string // required with OID, never echoed
	Owner           string // caller id; default keeper_id from .waggle/local.yaml
	Now             func() time.Time
}

// ForceReleaseResult is what ForceReleaseTask saw and did. It is filled as far
// as it got, also on error.
type ForceReleaseResult struct {
	Task          string      // canonical ticket id when the ref is canonical
	Ref           string      // the target ref
	Outcome       string      // inspected, released or no_claim
	OID           string      // the target's object id
	Lease         *comb.Lease // the decoded claim; nil when malformed or absent
	Expired       bool        // Lease has expired
	Malformed     string      // malformation reason; "" for a valid claim
	Caller        string      // the caller id checked
	FirstApprover string      // gate.approvers[0] at Base; "" when none
	Authorized    bool        // Caller may run the execute step
	Base          string      // the base commit the approvers were read from
	AuditOID      string      // the audit commit written; "" unless released
	Warnings      []string
}

var (
	fullOID      = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	canonicalRef = regexp.MustCompile(`^` + regexp.QuoteMeta(comb.ClaimRefPrefix) + `ticket-([1-9][0-9]{0,5})$`)
)

// baseHeadKeeper authorizes only gate.approvers[0] read from the base commit,
// compared by ASCII case folding like the gate's Head Keeper rule. With no
// approvers it denies everyone. Like every comb.HeadKeeperAuthorizer it is
// advisory against anyone with raw push access to the remote (D-009).
type baseHeadKeeper struct {
	first, base string
}

// AuthorizeHeadKeeper implements comb.HeadKeeperAuthorizer.
func (h baseHeadKeeper) AuthorizeHeadKeeper(_ context.Context, id string) error {
	short := h.base[:min(len(h.base), 12)]
	switch {
	case h.first == "":
		return fmt.Errorf("no first approver in gate.approvers at %s; nobody can force-release: %w", short, comb.ErrNotHeadKeeper)
	case id == "" || !strings.EqualFold(id, h.first):
		return fmt.Errorf("%q is not %s, the first login in gate.approvers at %s: %w", id, h.first, short, comb.ErrNotHeadKeeper)
	}
	return nil
}

// loadBaseHeadKeeper reads gate.approvers from the config at baseOID, never
// from the working tree. A missing file or key is no approver; an unreadable
// or invalid file is ExitEnv (fail closed).
func loadBaseHeadKeeper(ctx context.Context, g *Git, baseOID string) (baseHeadKeeper, error) {
	h := baseHeadKeeper{base: baseOID}
	data, found, err := blobAt(ctx, g, baseOID, layout.Config, config.MaxFileSize)
	if err != nil || !found {
		return h, err
	}
	cfg, err := config.Parse(data)
	if err != nil {
		return h, failf(ExitEnv, "base: %w", err)
	}
	if len(cfg.Gate.Approvers) > 0 {
		h.first = cfg.Gate.Approvers[0]
	}
	return h, nil
}

// validate checks the request shape before any git call. All failures are ExitUsage.
func (r *ForceReleaseRequest) validate() (ref, task string, err error) {
	switch {
	case (r.Ticket == "") == (r.Ref == ""):
		return "", "", failf(ExitUsage, "name the claim with exactly one of <task> or --ref")
	case r.OID != "" && !fullOID.MatchString(r.OID):
		return "", "", failf(ExitUsage, "--oid %q must be the full lowercase hex object id that inspect printed", r.OID)
	case r.OID != "" && r.Reason == "":
		return "", "", failf(ExitUsage, "--oid needs --reason <text>: say why the claim is cleared")
	case r.OID == "" && r.Reason != "":
		return "", "", failf(ExitUsage, "--reason needs --oid: inspect first, then run the line it prints")
	}
	if r.Ticket != "" {
		if task, err = comb.ParseTicketID(r.Ticket); err != nil {
			return "", "", failf(ExitUsage, "%w", err)
		}
		ref, _ = comb.RefName(task)
		return ref, task, nil
	}
	if m := canonicalRef.FindStringSubmatch(r.Ref); m != nil {
		task, _ = comb.ParseTicketID(m[1])
	}
	return r.Ref, task, nil
}

// refuseDispatchWorktree refuses a repo whose root sits inside the dispatch
// worktree directory of its main clone: a dispatched worker inherits the
// Keeper's local.yaml, so the command is for a person at the main clone.
func refuseDispatchWorktree(ctx context.Context, g *Git) error {
	out, err := g.Run(ctx, "rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir")
	if err != nil {
		return failf(ExitEnv, "%w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		return failf(ExitEnv, "unexpected git rev-parse output %q", out)
	}
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return filepath.Clean(p)
	}
	top, common := resolve(lines[0]), resolve(lines[1])
	wt := filepath.Join(filepath.Dir(common), filepath.FromSlash(layout.Worktrees))
	if rel, err := filepath.Rel(wt, top); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return failf(ExitClaim, "force-release is for a person at the main clone, not a dispatch worktree (%s)", top)
	}
	return nil
}

// ForceReleaseTask inspects or force-releases one claim ref (T-072). Inspect
// (no OID) syncs, reads and classifies the ref, checks the caller against
// gate.approvers[0] at the base commit and writes nothing; it succeeds also
// when the caller is not authorized, so the facts can be handed on. Execute
// (OID and Reason) re-inspects, refuses a changed ref, then calls
// comb.GitBackend.ForceRelease, which deletes the ref by compare-and-swap and
// appends the audit entry in one transaction or atomic push. An absent ref is
// outcome no_claim. Failures are *Error: ExitUsage for a bad request,
// ExitClaim for a refusal (not authorized, stale OID, invalid request, a
// dispatch worktree), ExitEnv for git, config or remote trouble.
func ForceReleaseTask(ctx context.Context, r ForceReleaseRequest) (res ForceReleaseResult, err error) {
	defer func() { err = interruptedEnv(ctx, err) }()
	ref, task, err := r.validate()
	res.Ref, res.Task = ref, task
	if err != nil {
		return res, err
	}
	repo, err := filepath.Abs(r.RepoDir)
	if err != nil {
		return res, failf(ExitEnv, "%w", err)
	}
	g := &Git{Bin: r.GitBin, Dir: repo}
	if err = refuseDispatchWorktree(ctx, g.pinned()); err != nil {
		return res, err
	}
	cfg, err := config.Load(filepath.Join(repo, filepath.FromSlash(layout.Config)))
	if err != nil {
		return res, failf(ExitEnv, "%w", err)
	}
	if res.Caller = r.Owner; res.Caller == "" {
		if res.Caller, err = config.LoadKeeperID(repo); err != nil {
			return res, failf(ExitUsage, "%w; or pass --owner <id>", err)
		}
	}
	g.Remote = cfg.Forge.Remote
	if res.Base, err = ResolveBase(ctx, g, g.Remote, ""); err != nil {
		return res, err
	}
	hk, err := loadBaseHeadKeeper(ctx, g, res.Base)
	if err != nil {
		return res, err
	}
	res.FirstApprover = hk.first
	authErr := hk.AuthorizeHeadKeeper(ctx, res.Caller)
	res.Authorized = authErr == nil
	if r.OID != "" && authErr != nil {
		return res, failf(ExitClaim, "%w", authErr)
	}
	if r.OID != "" {
		// Like waggle release: no delete while a dispatch run holds this repo.
		if _, serr := os.Lstat(filepath.Join(repo, filepath.FromSlash(layout.Worktrees))); !errors.Is(serr, fs.ErrNotExist) {
			var warns bytes.Buffer
			_, _, _, _, unlock, lerr := lockRepo(ctx, repo, r.GitBin, &warns)
			for _, l := range strings.Split(strings.TrimSpace(warns.String()), "\n") {
				if l != "" {
					res.Warnings = append(res.Warnings, strings.TrimPrefix(l, "warning: "))
				}
			}
			if lerr != nil {
				return res, lerr
			}
			defer unlock()
		}
	}
	opts, err := cfg.CombOptions(repo)
	opts.GitBinary, opts.ExtraEnv, opts.Now, opts.HeadKeeper = r.GitBin, combEnv, r.Now, hk
	var b *comb.GitBackend
	if err == nil {
		b, err = comb.NewGitBackend(opts)
	}
	if err != nil {
		return res, failf(ExitEnv, "%w", err)
	}
	if err = inspectRef(ctx, b, &res, r.Now); err != nil {
		if errors.Is(err, comb.ErrNotFound) {
			res.Outcome = "no_claim"
			return res, nil
		}
		return res, forceError(err)
	}
	if r.OID == "" {
		res.Outcome = "inspected"
		return res, nil
	}
	if res.OID != r.OID {
		return res, forceError(fmt.Errorf("%s is at %s, not %s: %w", ref, res.OID, r.OID, comb.ErrConcurrentUpdate))
	}
	err = b.ForceRelease(ctx, comb.ForceRelease{Ref: ref, ExpectedOID: r.OID, HeadKeeperID: res.Caller, Reason: r.Reason})
	if errors.Is(err, comb.ErrNotFound) {
		res.Outcome = "no_claim"
		return res, nil
	} else if err != nil {
		return res, forceError(err)
	}
	res.Outcome = "released"
	if out, rerr := g.pinned().Run(ctx, "rev-parse", "--verify", "--quiet", comb.ForceReleaseAuditRef); rerr != nil {
		res.Warnings = append(res.Warnings, "audit_oid_failed: "+rerr.Error())
	} else {
		res.AuditOID = strings.TrimSpace(string(out))
	}
	return res, nil
}

// FirstApprover returns gate.approvers[0] and the base commit it was read
// from, for waggle doctor. It never fetches and writes nothing: the base is
// refs/remotes/<remote>/main as last fetched, or refs/heads/main with no remote.
func FirstApprover(ctx context.Context, repo, remote, gitBin string) (first, base string, err error) {
	g := &Git{Bin: gitBin, Dir: repo, Remote: remote}
	ref := "refs/heads/main"
	if remote != "" {
		ref = "refs/remotes/" + remote + "/main"
	}
	out, err := g.Run(ctx, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", "", failf(ExitEnv, "base %q does not resolve to a commit: %w", ref, err)
	}
	base = strings.TrimSpace(string(out))
	hk, err := loadBaseHeadKeeper(ctx, g, base)
	return hk.first, base, err
}

// inspectRef fills res.OID and either the lease or the malformation reason.
// RefOID syncs in remote mode, so the facts are the remote's.
func inspectRef(ctx context.Context, b *comb.GitBackend, res *ForceReleaseResult, now func() time.Time) error {
	oid, err := b.RefOID(ctx, res.Ref)
	if err != nil {
		return err
	}
	res.OID = oid
	if res.Task != "" {
		l, err := b.ReadClaim(ctx, res.Task)
		switch {
		case err == nil:
			if now == nil {
				now = time.Now
			}
			res.Lease, res.Expired = &l, !l.ExpiresAt.After(now())
			return nil
		case !errors.Is(err, comb.ErrMalformedClaim):
			return err
		}
	}
	// A malformed or non-canonical ref: the reason comes from the claim scan.
	_, err = b.ListActiveLeases(ctx)
	var me *comb.MalformedClaimsError
	if errors.As(err, &me) {
		for _, c := range me.Claims {
			if c.Ref == res.Ref {
				res.Malformed = c.Reason
				return nil
			}
		}
	} else if err != nil {
		return err
	}
	res.Malformed = "unreadable claim"
	return nil
}

// forceError maps a Comb error from force-release to its exit code.
func forceError(err error) error {
	switch {
	case errors.Is(err, comb.ErrConcurrentUpdate):
		return failf(ExitClaim, "re-inspect: the claim changed since you looked: %w", err)
	case errors.Is(err, comb.ErrNotHeadKeeper), errors.Is(err, comb.ErrInvalidForceRelease):
		return failf(ExitClaim, "%w", err)
	}
	return claimError(err)
}
