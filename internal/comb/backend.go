package comb

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

// Lease represents a Keeper's claim on a single ticket and its scoped directories.
// In v0.1 directories are literal repo-relative paths, never globs (see
// NormalizeDirectories). TicketID is always stored in canonical display form
// (T-%03d, see ParseTicketID).
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
	// The overlap check is optimistic; the merge gate is authoritative.
	AcquireLease(ctx context.Context, lease Lease) error

	// ReleaseLease removes refs/waggle/claims/ticket-<ID>. Only the owning Keeper may
	// release a claim.
	ReleaseLease(ctx context.Context, ticketID string, keeperID string) error

	// ListActiveLeases reads all non-expired claims from refs/waggle/claims/*.
	// When some claim refs are malformed it returns the valid leases together with a
	// *MalformedClaimsError.
	ListActiveLeases(ctx context.Context) ([]Lease, error)
}

// Sentinel errors returned by Comb backends. Returned errors wrap these with
// context, so callers must compare with errors.Is.
var (
	// ErrInvalidTicketID reports a ticket ID that does not match ^(T-)?[0-9]{1,6}$ or is zero.
	ErrInvalidTicketID = errors.New("comb: invalid ticket id")
	// ErrInvalidLease reports a lease that fails field or directory validation.
	ErrInvalidLease = errors.New("comb: invalid lease")
	// ErrTier3Path reports a lease directory that equals, is contained by, or contains a Tier 3 path.
	ErrTier3Path = errors.New("comb: lease covers a Tier 3 control-plane path")
	// ErrGateTooLow reports a Light gate lease that touches a Tier 2 steering path.
	ErrGateTooLow = errors.New("comb: light gate cannot lease a Tier 2 steering path")
	// ErrLeaseOverlap reports that requested directories overlap another active lease.
	ErrLeaseOverlap = errors.New("comb: directories overlap an active lease")
	// ErrAlreadyClaimed reports that another Keeper holds an active claim, or won a race for it.
	ErrAlreadyClaimed = errors.New("comb: ticket already claimed")
	// ErrNotFound reports that no claim ref exists for the ticket.
	ErrNotFound = errors.New("comb: claim not found")
	// ErrNotOwner reports a release attempt by a Keeper that does not own the claim.
	ErrNotOwner = errors.New("comb: claim owned by another keeper")
	// ErrConcurrentUpdate reports that the claim ref changed between read and delete.
	ErrConcurrentUpdate = errors.New("comb: claim changed during release")
	// ErrMalformedClaim reports a claim ref that cannot be decoded into a valid Lease.
	ErrMalformedClaim = errors.New("comb: malformed claim ref")
	// ErrRelayNotImplemented is returned by every Backend method in relay comb mode.
	ErrRelayNotImplemented = errors.New("comb: relay comb mode not implemented in v0.1")
)

// ClaimRefPrefix is the ref namespace holding one claim ref per ticket.
const ClaimRefPrefix = "refs/waggle/claims/"

// DefaultMaxDirectoriesPerClaim is the default for leases.max_directories_per_claim.
const DefaultMaxDirectoriesPerClaim = 5

var (
	ticketIDPattern   = regexp.MustCompile(`^(T-)?[0-9]{1,6}$`)
	claimRefSuffixPat = regexp.MustCompile(`^ticket-([1-9][0-9]{0,5})$`)
	keeperIDPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@+-]{0,127}$`)
)

// parseTicketNum validates a ticket ID and returns its numeric value.
func parseTicketNum(s string) (int, error) {
	if !ticketIDPattern.MatchString(s) {
		return 0, fmt.Errorf("ticket id %q: %w", s, ErrInvalidTicketID)
	}
	n, err := strconv.Atoi(strings.TrimPrefix(s, "T-"))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("ticket id %q: %w", s, ErrInvalidTicketID)
	}
	return n, nil
}

func ticketDisplay(n int) string { return fmt.Sprintf("T-%03d", n) }

func ticketRef(n int) string { return fmt.Sprintf("%sticket-%d", ClaimRefPrefix, n) }

// ParseTicketID accepts "T-<digits>" or "<digits>" and returns the canonical ID.
// The input must match ^(T-)?[0-9]{1,6}$ (uppercase "T-" only) and have a value
// of at least 1. The canonical display form is T-%03d, so "1", "001", "T-1" and
// "T-001" all become "T-001". Any other input returns ErrInvalidTicketID.
func ParseTicketID(s string) (canonical string, err error) {
	n, err := parseTicketNum(s)
	if err != nil {
		return "", err
	}
	return ticketDisplay(n), nil
}

// RefName returns refs/waggle/claims/ticket-<n> for a valid ticket ID, where n is
// the decimal ticket number without zero padding (T-001 maps to ticket-1).
// Using per-ticket refs on both local and remote repositories ensures a single
// codepath from 1 Keeper (local .git) to 500 Keepers (remote Forge).
func RefName(ticketID string) (string, error) {
	n, err := parseTicketNum(ticketID)
	if err != nil {
		return "", err
	}
	return ticketRef(n), nil
}

// parseClaimRef returns the ticket number of a canonical claim ref name, or false
// when the ref is not refs/waggle/claims/ticket-<n> with no leading zeros.
func parseClaimRef(ref string) (int, bool) {
	if !strings.HasPrefix(ref, ClaimRefPrefix) {
		return 0, false
	}
	m := claimRefSuffixPat.FindStringSubmatch(strings.TrimPrefix(ref, ClaimRefPrefix))
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// NormalizeDirectories validates and canonicalizes a lease's directory list.
//
// Each entry must be a literal, repo-relative directory: no empty or
// whitespace-padded entries, no NUL or control characters, no backslashes, no
// glob metacharacters (* ? [), no absolute or volume-prefixed paths, no ".."
// escapes, and no path under .git (case-insensitive). Entries are cleaned to
// slash form without a trailing slash, deduplicated and sorted ascending.
// An entry that cleans to the repo root returns ErrTier3Path because the root
// contains Tier 3 files; every other failure returns ErrInvalidLease. An empty
// list or more than maxDirs distinct entries is rejected. maxDirs <= 0 means
// DefaultMaxDirectoriesPerClaim.
func NormalizeDirectories(dirs []string, maxDirs int) ([]string, error) {
	if maxDirs <= 0 {
		maxDirs = DefaultMaxDirectoriesPerClaim
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("no directories: %w", ErrInvalidLease)
	}
	seen := make(map[string]bool, len(dirs))
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		c, err := normalizeDir(d)
		if err != nil {
			return nil, err
		}
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	if len(out) > maxDirs {
		return nil, fmt.Errorf("%d directories exceeds limit %d: %w", len(out), maxDirs, ErrInvalidLease)
	}
	sort.Strings(out)
	return out, nil
}

func normalizeDir(d string) (string, error) {
	if strings.TrimSpace(d) == "" {
		return "", fmt.Errorf("empty directory: %w", ErrInvalidLease)
	}
	if strings.TrimSpace(d) != d {
		return "", fmt.Errorf("directory %q has leading or trailing whitespace: %w", d, ErrInvalidLease)
	}
	for _, r := range d {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("directory %q contains a control character: %w", d, ErrInvalidLease)
		}
	}
	if strings.ContainsAny(d, "\\*?[") {
		return "", fmt.Errorf("directory %q contains a backslash or glob metacharacter: %w", d, ErrInvalidLease)
	}
	if strings.HasPrefix(d, "/") || filepath.IsAbs(d) || filepath.VolumeName(d) != "" || hasDriveLetter(d) {
		return "", fmt.Errorf("directory %q is absolute: %w", d, ErrInvalidLease)
	}
	cleaned := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(d)), "/")
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("directory %q is the repo root: %w", d, ErrTier3Path)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("directory %q escapes the repo: %w", d, ErrInvalidLease)
	}
	first, _, _ := strings.Cut(cleaned, "/")
	if strings.EqualFold(first, ".git") {
		return "", fmt.Errorf("directory %q is inside .git: %w", d, ErrInvalidLease)
	}
	return cleaned, nil
}

// hasDriveLetter reports a Windows drive prefix such as "C:" on any platform.
func hasDriveLetter(d string) bool {
	if len(d) < 2 || d[1] != ':' {
		return false
	}
	c := d[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// DirsOverlap reports whether two normalized directories overlap on a path-segment boundary.
// "internal" and "internal/comb" overlap; "internal/comb" and "internal/combx" do not.
func DirsOverlap(a, b string) bool {
	return a == b || strings.HasPrefix(b, a+"/") || strings.HasPrefix(a, b+"/")
}

// touchesPrefixes reports whether a normalized directory equals, is contained by,
// or contains any of the given prefixes, comparing case-insensitively. Prefix
// matching mirrors RequiredMinTier, so "WAGGLE.md.bak" also matches "WAGGLE.md".
func touchesPrefixes(dir string, prefixes []string) bool {
	ld := strings.ToLower(dir)
	for _, p := range prefixes {
		lp := strings.ToLower(p)
		bare := strings.TrimSuffix(lp, "/")
		if strings.HasPrefix(ld, lp) || DirsOverlap(ld, bare) {
			return true
		}
	}
	return false
}

// isTier3Dir reports whether a normalized directory equals, is contained by, or
// contains a Tier 3 control-plane path.
func isTier3Dir(dir string) bool {
	return RequiredMinTier(dir) == Tier3ControlAuth || touchesPrefixes(dir, HardcodedTier3Prefixes)
}

// isTier2Dir reports whether a normalized directory equals, is contained by, or
// contains a Tier 2 steering path.
func isTier2Dir(dir string) bool {
	return RequiredMinTier(dir) == Tier2Logic || touchesPrefixes(dir, HardcodedTier2MinPrefixes)
}

// validateLeaseFields checks every non-time field of a lease and returns a copy
// with the canonical ticket ID and normalized directories.
func validateLeaseFields(l Lease, maxDirs int) (Lease, error) {
	id, err := ParseTicketID(l.TicketID)
	if err != nil {
		return Lease{}, err
	}
	l.TicketID = id
	if !keeperIDPattern.MatchString(l.KeeperID) {
		return Lease{}, fmt.Errorf("keeper id %q: %w", l.KeeperID, ErrInvalidLease)
	}
	switch l.Role {
	case "scout", "forager", "guard":
	default:
		return Lease{}, fmt.Errorf("role %q: %w", l.Role, ErrInvalidLease)
	}
	switch l.Gate {
	case "light", "full":
	default:
		return Lease{}, fmt.Errorf("gate %q: %w", l.Gate, ErrInvalidLease)
	}
	dirs, err := NormalizeDirectories(l.Directories, maxDirs)
	if err != nil {
		return Lease{}, err
	}
	for _, d := range dirs {
		if isTier3Dir(d) {
			return Lease{}, fmt.Errorf("directory %q: %w", d, ErrTier3Path)
		}
	}
	if l.Gate == "light" {
		for _, d := range dirs {
			if isTier2Dir(d) {
				return Lease{}, fmt.Errorf("directory %q: %w", d, ErrGateTooLow)
			}
		}
	}
	l.Directories = dirs
	return l, nil
}

// MalformedClaim describes one claim ref that could not be decoded.
type MalformedClaim struct {
	// Ref is the full ref name, for example refs/waggle/claims/ticket-7.
	Ref string
	// Reason is a short human-readable explanation.
	Reason string
}

// MalformedClaimsError lists claim refs that ListActiveLeases skipped. It wraps
// ErrMalformedClaim; use errors.As to inspect it. When it is returned, the
// accompanying lease slice is valid and holds every well-formed active lease.
type MalformedClaimsError struct {
	Claims []MalformedClaim
}

// Error implements error.
func (e *MalformedClaimsError) Error() string {
	parts := make([]string, len(e.Claims))
	for i, c := range e.Claims {
		parts[i] = c.Ref + " (" + c.Reason + ")"
	}
	return fmt.Sprintf("%v: %d malformed: %s", ErrMalformedClaim, len(e.Claims), strings.Join(parts, "; "))
}

// Unwrap returns ErrMalformedClaim so errors.Is(err, ErrMalformedClaim) holds.
func (e *MalformedClaimsError) Unwrap() error { return ErrMalformedClaim }
