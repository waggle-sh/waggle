package comb

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// RiskTier defines the semantic review requirement for a task or diff.
type RiskTier int

const (
	Tier0Trivial     RiskTier = 0 // Comments, formatting, non-steering typo fixes
	Tier1Isolated    RiskTier = 1 // Light Gate: <150 lines, isolated unit tests, lane-eligible
	Tier2Logic       RiskTier = 2 // Full Gate: standard logic within lease, or steering docs
	Tier3ControlAuth RiskTier = 3 // Control plane, security, cross-module, dependencies (Controller-only)
)

// HardcodedTier3Prefixes define the hive's control plane and execution sandbox.
// Any modification is a potential prompt-injection or sandbox-escape vector and
// MUST require Master Keeper (Controller) approval. Not configurable.
var HardcodedTier3Prefixes = []string{
	".waggle/",
	"WAGGLE.md",
	".devcontainer/",
	".github/workflows/",
	".gitlab-ci.yml",
}

// HardcodedTier2MinPrefixes define agent-read steering context.
// Because an injected roadmap or decision log steers downstream Foragers,
// these paths can never be downgraded to Tier 0 or Tier 1 (Light Gate).
var HardcodedTier2MinPrefixes = []string{
	"DECISIONS.md",
	"ROADMAP.md",
	"WORKPLAN.md",
	"docs/specs/",
	"docs/tasks/",
}

// RequiredMinTier returns the minimum allowable RiskTier for a given repo-relative path.
// We normalize via filepath.Clean and ToSlash to prevent "../" traversal bypasses.
func RequiredMinTier(relPath string) RiskTier {
	cleaned := filepath.ToSlash(filepath.Clean(relPath))

	for _, prefix := range HardcodedTier3Prefixes {
		if cleaned == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(cleaned, prefix) {
			return Tier3ControlAuth
		}
	}
	for _, prefix := range HardcodedTier2MinPrefixes {
		if cleaned == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(cleaned, prefix) {
			return Tier2Logic
		}
	}
	return Tier0Trivial
}

// RefName returns the canonical Git ref path for a ticket claim.
// Using per-ticket refs on both local and remote repositories ensures a single
// codepath from 1 Keeper (local .git) to 500 Keepers (remote Forge).
func RefName(ticketID string) string {
	return fmt.Sprintf("refs/waggle/claims/ticket-%s", strings.TrimPrefix(ticketID, "T-"))
}

// Lease represents a Keeper's claim on a single ticket and its scoped directory globs.
//
// CONCURRENCY INVARIANT (RFC-0003 Section 3.1):
// Updating refs/waggle/claims/ticket-<ID> provides atomic Compare-And-Swap (CAS)
// for the ticket itself, but directory overlap checks across different ticket refs
// have a TOCTOU window. waggle-gate.yml is the authoritative merge-time check.
type Lease struct {
	Version     int       `json:"version"`
	TicketID    string    `json:"ticket_id"`
	KeeperID    string    `json:"keeper_id"`
	Role        string    `json:"role"` // "scout" | "forager" | "guard"
	Gate        string    `json:"gate"` // "light" | "full"
	Directories []string  `json:"directories"`
	ClaimedAt   time.Time `json:"claimed_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Backend defines the single Comb interface used for both solo local repos and remotes.
type Backend interface {
	// AcquireLease checks existing refs/waggle/claims/* for directory overlap and writes
	// the new claim ref atomically via git update-ref (local) or git push --atomic (remote).
	AcquireLease(ctx context.Context, lease Lease) error

	// ReleaseLease removes refs/waggle/claims/ticket-<ID>.
	ReleaseLease(ctx context.Context, ticketID string, keeperID string) error

	// ListActiveLeases reads all non-expired claims from refs/waggle/claims/*.
	ListActiveLeases(ctx context.Context) ([]Lease, error)
}
