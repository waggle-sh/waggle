package dispatch

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// ClaimBackend is the slice of *comb.GitBackend that dispatch uses.
type ClaimBackend interface {
	comb.Backend
	ReadClaim(ctx context.Context, ticketID string) (comb.Lease, error)
	ClaimOID(ctx context.Context, ticketID string) (string, error)
}

// Claim is the outcome of AcquireClaim. Notes are lines the caller prints
// before it continues: a takeover names the previous owner, and a renewal that
// changes role, gate or directories shows old and new.
type Claim struct {
	Lease    comb.Lease // as requested; times are the backend's
	OID      string     // claim commit
	Verb     string     // claim, renew or takeover
	Notes    []string
	Warnings []string // read-back failures after the claim was written
}

// AcquireClaim claims the ticket for a builder or forager (T-023 section 6).
// The guard reads the claim instead and never calls this. req must have passed
// Validate. The pre-read (after a sync in remote mode) only builds the
// notes; AcquireLease stays the authority. Failures are *Error: ExitClaim for a held, overlapping,
// malformed or refused claim, ExitEnv for git trouble. Nothing is written
// when the claim is refused.
func AcquireClaim(ctx context.Context, cb ClaimBackend, g *Git, req Request, keeperID string, now time.Time) (Claim, error) {
	if req.Role == "guard" {
		return Claim{}, failf(ExitUsage, "guard does not claim; it reads the builder's claim")
	}
	branches := req.ClaimBranches
	if len(branches) == 0 {
		branches = []string{req.Branch}
	}
	want := comb.Lease{TicketID: req.Ticket, KeeperID: keeperID, Role: req.Role, Gate: req.Gate, Directories: req.Dirs, Branches: branches}
	c := Claim{Lease: want, Verb: "claim"}
	// ClaimOID fetches in remote mode, so the notes and the malformed-ref exit
	// below describe the remote claim, not a stale local mirror.
	if _, err := cb.ClaimOID(ctx, req.Ticket); err != nil && !errors.Is(err, comb.ErrNotFound) {
		return Claim{}, claimError(err)
	}
	old, err := cb.ReadClaim(ctx, req.Ticket)
	switch {
	case errors.Is(err, comb.ErrNotFound):
	case err != nil:
		return Claim{}, claimError(err)
	case !old.ExpiresAt.After(now):
		c.Verb = "takeover"
		c.Notes = append(c.Notes, fmt.Sprintf("taking over %s from %s (expired %s)", req.Ticket, old.KeeperID, old.ExpiresAt.UTC().Format(time.RFC3339)))
	case old.KeeperID == keeperID:
		c.Verb = "renew"
		if len(req.ClaimBranches) == 0 && len(old.Branches) > 0 {
			want.Branches = old.Branches
		}
		if old.Role != want.Role || old.Gate != want.Gate || !slices.Equal(old.Directories, want.Directories) ||
			!slices.Equal(old.Branches, want.Branches) {
			c.Notes = append(c.Notes, fmt.Sprintf("renewal changes the claim: %s %s %v %v -> %s %s %v %v",
				old.Role, old.Gate, old.Directories, old.Branches, want.Role, want.Gate, want.Directories, want.Branches))
		}
	}
	if err := cb.AcquireLease(ctx, want); err != nil {
		return Claim{}, claimError(err)
	}
	// The claim is written. A failing read-back is a warning, never exit 4: the
	// requested lease stands and the OID stays empty. The OID comes from the
	// local ref (a rev-parse, no third fetch in remote mode).
	if c.Lease, err = cb.ReadClaim(ctx, req.Ticket); err != nil {
		c.Lease = want
		c.Warnings = append(c.Warnings, "claim_read_failed: "+err.Error())
	}
	ref, _ := comb.RefName(req.Ticket)
	if out, err := g.Run(ctx, "rev-parse", "--verify", "--quiet", ref); err != nil {
		c.Warnings = append(c.Warnings, "claim_oid_failed: "+err.Error())
	} else {
		c.OID = strings.TrimSpace(string(out))
	}
	return c, nil
}

// claimError maps a Comb error to its dispatch exit code.
func claimError(err error) error {
	switch {
	case errors.Is(err, comb.ErrMalformedClaim):
		return failf(ExitClaim, "%w; a maintainer must delete the malformed claim ref by hand, on the remote if the repo has one", err)
	case errors.Is(err, comb.ErrAlreadyClaimed), errors.Is(err, comb.ErrLeaseOverlap),
		errors.Is(err, comb.ErrTier3Path), errors.Is(err, comb.ErrGateTooLow), errors.Is(err, comb.ErrInvalidLease):
		return failf(ExitClaim, "%w", err)
	case errors.Is(err, comb.ErrInvalidTicketID):
		return failf(ExitUsage, "%w", err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return failf(ExitFailed, "interrupted: %w", err)
	}
	return failf(ExitEnv, "%w", err)
}
