package dispatch

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
)

// ClaimTaskRequest asks to claim directories for a task on behalf of a human or
// an orchestrator (waggle claim). The claim is written with role builder, so a
// later waggle dispatch of the task by the same owner renews it.
type ClaimTaskRequest struct {
	RepoDir, GitBin, Ticket string
	Dirs                    []string
	Gate                    string        // light or full
	Branches                []string      // PR branches the claim accepts; empty: t-<nnn> on a fresh claim, the stored list on a renewal
	TTL                     time.Duration // 0: leases.default_ttl_minutes; else whole minutes in the config bounds
	Owner                   string        // default keeper_id from .waggle/local.yaml
	Base                    string        // revision whose config lists the protected paths; default as ResolveBase
	Now                     func() time.Time
	Backend                 ClaimBackend // nil: built from config
}

// ClaimTaskResult is what ClaimTask wrote.
type ClaimTaskResult struct {
	Task     string
	Outcome  string     // claimed, renewed or taken_over
	Lease    comb.Lease // as stored; zero times when the read-back failed
	Ref, OID string     // OID is empty when the read-back failed
	Notes    []string
	Warnings []string
}

// ClaimTask validates the request, refuses what no lease may cover, reads the
// protected paths from the base revision and writes the claim through
// AcquireClaim. Failures are *Error: ExitUsage for a bad request, ExitClaim for
// a refused claim, ExitEnv for git or config trouble. Nothing is written when it
// fails. A cancelled ctx turns an ExitEnv failure into ExitFailed (interrupted).
func ClaimTask(ctx context.Context, r ClaimTaskRequest) (res ClaimTaskResult, err error) {
	defer func() { err = interruptedEnv(ctx, err) }()
	if id, perr := comb.ParseTicketID(r.Ticket); perr == nil {
		res.Task = id
	}
	req, err := Request{RepoDir: r.RepoDir, Ticket: r.Ticket, Role: "builder", Dirs: r.Dirs, Gate: r.Gate, Base: r.Base, ClaimBranches: r.Branches}.Validate()
	if err != nil {
		return res, err
	}
	res.Task = req.Ticket
	if err = comb.ValidateBranches(r.Branches); err != nil {
		return res, failf(ExitUsage, "%w", err)
	}
	if err = refuseHardcoded(req.Dirs); err != nil {
		return res, err
	}
	if lo, hi := config.MinTTLMinutes*time.Minute, config.MaxTTLMinutes*time.Minute; r.TTL != 0 && (r.TTL < lo || r.TTL > hi || r.TTL%time.Minute != 0) {
		return res, failf(ExitUsage, "ttl %s: must be a whole number of minutes from %s to %s", r.TTL, lo, hi)
	}
	repo, err := filepath.Abs(r.RepoDir)
	if err != nil {
		return res, failf(ExitEnv, "%w", err)
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
	now := r.Now
	if now == nil {
		now = time.Now
	}
	g := &Git{Bin: r.GitBin, Dir: repo, Remote: cfg.Forge.Remote}
	baseOID, err := ResolveBase(ctx, g, g.Remote, req.Base)
	if err != nil {
		return res, err
	}
	policy, err := basePolicy(ctx, g, baseOID)
	if err != nil {
		return res, err
	}
	if err = refuseProtected(policy, req.Dirs); err != nil {
		return res, err
	}
	cb := r.Backend
	if cb == nil {
		opts, err := cfg.CombOptions(repo)
		opts.GitBinary, opts.ExtraEnv, opts.Protected, opts.Now = r.GitBin, combEnv, policy, r.Now
		if r.TTL != 0 {
			opts.DefaultTTL = r.TTL
		}
		if err == nil {
			cb, err = comb.NewGitBackend(opts)
		}
		if err != nil {
			return res, failf(ExitEnv, "%w", err)
		}
	}
	c, err := AcquireClaim(ctx, cb, g, req, owner, now())
	if err != nil {
		return res, err
	}
	res.Outcome = map[string]string{"claim": "claimed", "renew": "renewed", "takeover": "taken_over"}[c.Verb]
	res.Lease, res.OID, res.Notes, res.Warnings = c.Lease, c.OID, c.Notes, c.Warnings
	res.Ref, _ = comb.RefName(req.Ticket)
	return res, nil
}

// basePolicy reads gate.protected_paths from .waggle/control/config.yaml at
// baseOID, never from the working tree, the head branch or a merge-base. A
// missing file is an empty policy; a file that is not a regular file, is over the
// cap or does not parse is ExitEnv (fail closed).
func basePolicy(ctx context.Context, g *Git, baseOID string) (comb.TierPolicy, error) {
	data, found, err := blobAt(ctx, g, baseOID, layout.Config, config.MaxFileSize)
	if err != nil || !found {
		return comb.TierPolicy{}, err
	}
	cfg, err := config.Parse(data)
	if err != nil {
		return comb.TierPolicy{}, failf(ExitEnv, "base: %w", err)
	}
	return comb.TierPolicy{ExtraTier3: cfg.Gate.ProtectedPaths}, nil
}

// refuseProtected refuses a directory that covers a protected path of p.
func refuseProtected(p comb.TierPolicy, dirs []string) error {
	for _, d := range dirs {
		if p.CoversDir(d) {
			return failf(ExitClaim, "directory %q: %w: %w", d, comb.ErrProtectedPath, comb.ErrTier3Path)
		}
	}
	return nil
}

// refuseHardcoded is refuseDirs with an error that wraps comb.ErrTier3Path: it
// refuses, before any fetch, a directory that equals, contains or sits inside a
// Tier 3 path or .waggle/.
func refuseHardcoded(dirs []string) error {
	for _, d := range dirs {
		for _, p := range comb.HardcodedTier3Prefixes {
			if comb.DirsOverlap(strings.ToLower(d), strings.ToLower(strings.TrimSuffix(p, "/"))) {
				return failf(ExitClaim, "directory %q: %w", d, comb.ErrTier3Path)
			}
		}
	}
	return nil
}

// ErrorCode names err for the JSON output of waggle claim and waggle release:
// the first match in a fixed order, so every error has exactly one code.
func ErrorCode(err error) string {
	exit := ExitCode(err)
	for _, c := range []struct {
		target error
		code   string
	}{
		{comb.ErrInvalidTicketID, "invalid_task"}, {nil, "usage"}, {comb.ErrTier3Path, "protected_path"},
		{comb.ErrGateTooLow, "gate_too_low"}, {comb.ErrInvalidLease, "invalid_lease"}, {comb.ErrLeaseOverlap, "overlap"},
		{comb.ErrAlreadyClaimed, "already_claimed"}, {comb.ErrMalformedClaim, "malformed_claim"}, {comb.ErrNotOwner, "not_owner"},
		{comb.ErrConcurrentUpdate, "concurrent_update"}, {errNotMerged, "not_merged"}, {errTaskBusy, "task_busy"},
	} {
		switch {
		case c.target == nil && exit == ExitUsage, c.target != nil && errors.Is(err, c.target):
			return c.code
		}
	}
	switch exit {
	case ExitFailed:
		return "interrupted"
	case ExitClaim:
		return "refused"
	}
	return "environment"
}

// interruptedEnv maps an ExitEnv error to ExitFailed once ctx is done: a
// cancelled git call reports as a killed process, not as context.Canceled.
// Usage and refusal errors keep their codes.
func interruptedEnv(ctx context.Context, err error) error {
	if err != nil && ctx.Err() != nil && ExitCode(err) == ExitEnv {
		return failf(ExitFailed, "interrupted: %w", err)
	}
	return err
}
