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
	Tier0Trivial      RiskTier = 0 // Comments, formatting, non-steering typo fixes
	Tier1Isolated     RiskTier = 1 // Light Gate: <150 lines, isolated unit tests, lane-eligible
	Tier2Logic        RiskTier = 2 // Full Gate: standard logic within lease, or steering docs
	Tier3ControlPlane RiskTier = 3 // Control plane, security, cross-module, dependencies (Head Keeper only)
)

// HardcodedTier3Prefixes define the hive's control plane, execution sandbox and CI.
// Any modification is a potential prompt-injection or sandbox-escape vector and
// MUST require Head Keeper approval. Not configurable. Matched case-insensitively.
// ".waggle/" covers every current and future subfolder except HiveCarveOut.
var HardcodedTier3Prefixes = []string{
	".waggle/",
	".github/workflows/",
	".gitlab-ci.yml",
	".devcontainer/",
	"go.mod",
	"go.sum",
	"go.work",
	"go.work.sum",
	"vendor/",
}

// HiveCarveOut is the only part of .waggle/ classified below Tier 3. It matches
// exact case only, so a case variant such as ".waggle/HIVE/" resolves upward to Tier 3.
const HiveCarveOut = ".waggle/hive/"

// HardcodedTier2MinPrefixes define agent-read steering context.
// Because an injected brief, decision log or agent instruction file steers
// downstream Builders, these paths can never be downgraded to Tier 0 or Tier 1
// (Light Gate). Matched case-insensitively.
var HardcodedTier2MinPrefixes = []string{
	".waggle/hive/",
	"CLAUDE.md",
	"AGENTS.md",
}

// matchesPrefix applies the equal-or-prefix rule to an already lowercased path.
func matchesPrefix(lower string, prefixes []string) bool {
	for _, p := range prefixes {
		lp := strings.ToLower(p)
		if lower == strings.TrimSuffix(lp, "/") || strings.HasPrefix(lower, lp) {
			return true
		}
	}
	return false
}

// RequiredMinTier returns the minimum allowable RiskTier for a given repo-relative path.
// The path is normalized via filepath.Clean and ToSlash to prevent "../" traversal
// bypasses. A path inside HiveCarveOut (exact case) is Tier 2; otherwise the
// lowercased path is compared against the lowercased tier lists.
func RequiredMinTier(relPath string) RiskTier {
	cleaned := filepath.ToSlash(filepath.Clean(relPath))
	if cleaned == strings.TrimSuffix(HiveCarveOut, "/") || strings.HasPrefix(cleaned, HiveCarveOut) {
		return Tier2Logic
	}
	lower := strings.ToLower(cleaned)
	if matchesPrefix(lower, HardcodedTier3Prefixes) {
		return Tier3ControlPlane
	}
	if matchesPrefix(lower, HardcodedTier2MinPrefixes) {
		return Tier2Logic
	}
	return Tier0Trivial
}

// LeaseVersion is the lease.json version written by new claims. Version 1
// claims predate the role rename (D-080) and carry the old role names; they are
// still read, with their roles mapped through legacyRoles (see parseLease).
const LeaseVersion = 2

// legacyRoles maps a version 1 role name to its current name (D-080). Before
// the rename, "scout" was the triage role and "forager" the implementation
// role; both words still exist with different meanings, so a version 1 claim
// is never read with its role name taken literally.
var legacyRoles = map[string]string{
	"scout":   "forager",
	"forager": "builder",
	"guard":   "guard",
}

// Lease represents a Keeper's claim on a single ticket and its scoped directories.
// In v0.1 directories are literal repo-relative paths, never globs (see
// NormalizeDirectories). TicketID is always stored in canonical display form
// (T-%03d, see ParseTicketID).
//
// Version is the stored lease.json version. A lease read from a version 1
// claim keeps Version 1 but carries the current role name.
//
// CONCURRENCY INVARIANT (RFC-0003 Section 3.1):
// Updating refs/waggle/claims/ticket-<ID> provides atomic Compare-And-Swap (CAS)
// for the ticket itself, but directory overlap checks across different ticket refs
// have a TOCTOU window. waggle-gate.yml is the authoritative merge-time check.
type Lease struct {
	Version     int       `json:"version"`
	TicketID    string    `json:"ticket_id"`
	KeeperID    string    `json:"keeper_id"`
	Role        string    `json:"role"` // "forager" | "builder" | "guard" (see isClaimRole); version 1 names are mapped on read
	Gate        string    `json:"gate"` // "light" | "full"
	Directories []string  `json:"directories"`
	ClaimedAt   time.Time `json:"claimed_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	// Branches are the pull request branches the gate accepts for this claim
	// (see ValidateBranches); empty means the claim names none.
	Branches []string `json:"branches,omitempty"`
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
	// ErrWaggleDir reports a lease directory that equals, is inside, or contains
	// .waggle (any case, hive included). Returned errors also wrap ErrTier3Path.
	ErrWaggleDir = errors.New("comb: leases never include .waggle/; files there change only through waggle commands or an admin merge")
	// ErrProtectedPath reports a lease directory that covers a path listed in
	// gate.protected_paths. Returned errors also wrap ErrTier3Path.
	ErrProtectedPath = errors.New("protected path listed in .waggle/control/config.yaml; leases never cover it")
	// ErrGateTooLow reports a Light gate lease that touches a Tier 2 steering path.
	ErrGateTooLow = errors.New("comb: light gate cannot lease a Tier 2 steering path")
	// ErrLeaseOverlap reports that requested directories overlap another active lease.
	ErrLeaseOverlap = errors.New("comb: directories overlap an active lease")
	// ErrAlreadyClaimed reports that another Keeper holds an active claim, or won a race for it.
	ErrAlreadyClaimed = errors.New("comb: ticket already claimed")
	// ErrNotFound reports that no claim ref exists for the ticket.
	ErrNotFound = errors.New("comb: claim not found")
	// ErrNotOwner reports a release attempt by an owner that does not hold the claim.
	ErrNotOwner = errors.New("comb: claim held by another owner")
	// ErrConcurrentUpdate reports that the claim ref changed between read and delete.
	ErrConcurrentUpdate = errors.New("comb: claim changed during release")
	// ErrMalformedClaim reports a claim ref that cannot be decoded into a valid Lease.
	ErrMalformedClaim = errors.New("comb: malformed claim ref")
	// ErrRelayNotImplemented is returned by every Backend method in relay comb mode.
	ErrRelayNotImplemented = errors.New("comb: relay comb mode not implemented in v0.1")
	// ErrNotHeadKeeper reports a ForceRelease by a caller Options.HeadKeeper did
	// not authorize, including every call when no authorizer is configured.
	ErrNotHeadKeeper = errors.New("comb: caller is not an authorized head keeper")
	// ErrInvalidForceRelease reports a ForceRelease request that fails field validation.
	ErrInvalidForceRelease = errors.New("comb: invalid force-release request")
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
// Each entry must be a literal, repo-relative directory of at most 1024 bytes
// made only of [A-Za-z0-9._/-]: no spaces, control characters, backslashes,
// glob metacharacters, backticks, angle brackets, colons, tildes, quotes or
// non-ASCII (Q-T009-2). It must not be absolute, escape with "..", or lie
// under .git (case-insensitive). After cleaning, every path segment must be at
// most 255 bytes, must not end with "." (Windows drops trailing dots, so
// ".waggle." would resolve to ".waggle"), must not start with "-" (never
// flag-shaped), and must not be a Windows reserved device name (CON, PRN, AUX,
// NUL, COM0 to COM9, LPT0 to LPT9 before the first ".", any case; Q-T009-1).
// Entries are cleaned to slash form without a trailing slash, deduplicated and
// sorted ascending. An entry that cleans to the repo root returns ErrTier3Path
// because the root contains Tier 3 files; every other failure returns
// ErrInvalidLease. An empty list or more than maxDirs distinct entries is
// rejected. maxDirs <= 0 means DefaultMaxDirectoriesPerClaim.
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

// Byte limits on a raw directory entry and on each cleaned path segment.
const (
	maxDirBytes     = 1024
	maxSegmentBytes = 255
)

// isDirByte reports whether c is in the directory allow-list [A-Za-z0-9._/-].
func isDirByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '.' || c == '_' || c == '/' || c == '-'
}

// isWindowsDevice reports whether the part of a segment before its first "."
// is a Windows reserved device name, compared case-insensitively.
func isWindowsDevice(seg string) bool {
	stem, _, _ := strings.Cut(seg, ".")
	stem = strings.ToUpper(stem)
	switch stem {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) {
		return stem[3] >= '0' && stem[3] <= '9'
	}
	return false
}

// checkSegment applies the per-segment rules of NormalizeDirectories.
func checkSegment(seg string) error {
	switch {
	case strings.HasSuffix(seg, "."):
		return fmt.Errorf("segment %q ends with a dot: %w", seg, ErrInvalidLease)
	case strings.HasPrefix(seg, "-"):
		return fmt.Errorf("segment %q starts with a dash: %w", seg, ErrInvalidLease)
	case len(seg) > maxSegmentBytes:
		return fmt.Errorf("segment of %d bytes exceeds %d: %w", len(seg), maxSegmentBytes, ErrInvalidLease)
	case isWindowsDevice(seg):
		return fmt.Errorf("segment %q is a Windows reserved device name: %w", seg, ErrInvalidLease)
	}
	return nil
}

// isCanonicalDir reports whether d passes normalizeDir and is already in
// canonical form, so it can be emitted verbatim.
func isCanonicalDir(d string) bool {
	c, err := normalizeDir(d)
	return err == nil && c == d
}

func normalizeDir(d string) (string, error) {
	if strings.TrimSpace(d) == "" {
		return "", fmt.Errorf("empty directory: %w", ErrInvalidLease)
	}
	if len(d) > maxDirBytes {
		return "", fmt.Errorf("directory of %d bytes exceeds %d: %w", len(d), maxDirBytes, ErrInvalidLease)
	}
	for i := 0; i < len(d); i++ {
		if !isDirByte(d[i]) {
			return "", fmt.Errorf("directory %q contains a character outside [A-Za-z0-9._/-]: %w", d, ErrInvalidLease)
		}
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
	for _, seg := range strings.Split(cleaned, "/") {
		if err := checkSegment(seg); err != nil {
			return "", fmt.Errorf("directory %q: %w", d, err)
		}
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
// matching mirrors RequiredMinTier, so "CLAUDE.md.bak" also matches "CLAUDE.md".
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
	return RequiredMinTier(dir) == Tier3ControlPlane || touchesPrefixes(dir, HardcodedTier3Prefixes)
}

// isWaggleDir reports whether a normalized directory equals, is inside, or
// contains .waggle, comparing case-insensitively (D-043). The hive carve-out
// does not apply: worker leases never include any part of .waggle/.
func isWaggleDir(dir string) bool {
	return touchesPrefixes(dir, []string{".waggle/"})
}

// isTier2Dir reports whether a normalized directory equals, is contained by, or
// contains a Tier 2 steering path.
func isTier2Dir(dir string) bool {
	return RequiredMinTier(dir) == Tier2Logic || touchesPrefixes(dir, HardcodedTier2MinPrefixes)
}

// isClaimRole reports whether role may hold a claim. The claimable roles are the
// three worker roles that act on a ticket: forager (triage and repro), builder
// (implementation) and guard (review). The queen and the scout never claim.
func isClaimRole(role string) bool {
	switch role {
	case "forager", "builder", "guard":
		return true
	}
	return false
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
		return Lease{}, fmt.Errorf("owner %q: %w", l.KeeperID, ErrInvalidLease)
	}
	if !isClaimRole(l.Role) {
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
		if isWaggleDir(d) {
			return Lease{}, fmt.Errorf("directory %q: %w: %w", d, ErrWaggleDir, ErrTier3Path)
		}
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
	if err := ValidateBranches(l.Branches); err != nil {
		return Lease{}, err
	}
	l.Directories = dirs
	return l, nil
}

var (
	forkOwnerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
	forkRepoPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
)

// ValidateBranches checks a claim's pull request branch list: at most 4 distinct
// entries, each a short branch name or an <owner>/<repo>:<branch> fork branch.
// Errors name an entry by position and rule, never by its bytes, because a
// stored claim's error reaches gate output.
func ValidateBranches(list []string) error {
	if len(list) > 4 {
		return fmt.Errorf("branches: at most 4: %w", ErrInvalidLease)
	}
	seen := map[string]bool{}
	for i, e := range list {
		if seen[e] {
			return fmt.Errorf("branches entry %d: duplicate: %w", i+1, ErrInvalidLease)
		}
		seen[e] = true
		if len(e) > 255 {
			return fmt.Errorf("branches entry %d: over 255 bytes: %w", i+1, ErrInvalidLease)
		}
		name := e
		if repo, branch, ok := strings.Cut(e, ":"); ok {
			if !validForkRepo(repo) {
				return fmt.Errorf("branches entry %d: not a valid fork repository: %w", i+1, ErrInvalidLease)
			}
			name = branch
		}
		if !validBranchName(name) {
			return fmt.Errorf("branches entry %d: not a valid branch name: %w", i+1, ErrInvalidLease)
		}
	}
	return nil
}

func validForkRepo(s string) bool {
	owner, repo, ok := strings.Cut(s, "/")
	return ok && forkOwnerPattern.MatchString(owner) && forkRepoPattern.MatchString(repo) &&
		repo != "." && repo != ".."
}

// validBranchName reports whether s is a short branch name git accepts, limited
// to printable ASCII.
func validBranchName(s string) bool {
	if s == "" || s == "@" || strings.HasPrefix(s, "refs/") ||
		strings.ContainsAny(s[:1], "-/.") || strings.ContainsAny(s[len(s)-1:], "/.") ||
		strings.HasSuffix(s, ".lock") || strings.Contains(s, "..") ||
		strings.Contains(s, "@{") || strings.Contains(s, "//") {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x21 || c > 0x7e || strings.IndexByte("~^:?*[\\", c) >= 0 {
			return false
		}
	}
	for _, part := range strings.Split(s, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
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
