// Package dispatch is the library core of waggle dispatch (T-023): request
// validation, the Comb claim, the worktree lifecycle and the prompt inputs
// read from the base revision. Phase B is inert: nothing here launches an
// agent, and the post-run checks arrive with the run phase.
package dispatch

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
)

// Exit codes shared by the library and the CLI (T-023 section 4).
const (
	ExitFailed      = 1 // run failed
	ExitUsage       = 2 // usage error, before any git write
	ExitEnv         = 3 // git, config, I/O, brief absent at base
	ExitClaim       = 4 // claim refused
	ExitNeedsKeeper = 5 // needs the Keeper
)

// Error carries the exit code a failure maps to.
type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// failf returns an *Error with code. The format may use %w.
func failf(code int, format string, args ...any) error {
	return &Error{Code: code, Err: fmt.Errorf(format, args...)}
}

// ExitCode returns the exit code for err: 0 for nil, the code of the first
// *Error in the chain, otherwise ExitEnv.
func ExitCode(err error) int {
	var e *Error
	switch {
	case err == nil:
		return 0
	case errors.As(err, &e):
		return e.Code
	}
	return ExitEnv
}

// Request is what the queen asks for. Validate fills in the defaults.
type Request struct {
	RepoDir   string
	Ticket    string // comb.ParseTicketID forms
	Role      string // builder, forager or guard
	Dirs      []string
	Gate      string // light or full
	Branch    string // default t-<nnn>
	Base      string // revision; empty means <forge.remote>/main or main
	Tier      *comb.RiskTier
	ReadPaths []string // read-only context, repo-relative, read from base
	Second    bool     // guard only: also avoid the first guard's model (D-100)
	Head      string   // set by Run for a guard: the OID under review, where its worktree is detached
}

const maxReadPaths = 50

var branchRest = regexp.MustCompile(`^[A-Za-z0-9._-]*$`)

// Validate checks every field that needs no git or config and returns the
// request with canonical ticket, normalized directories and the default
// branch. Errors are *Error with ExitUsage, or ExitClaim for a lease
// directory the Comb would refuse outright. It never touches git.
func (r Request) Validate() (Request, error) {
	if strings.TrimSpace(r.RepoDir) == "" {
		return r, failf(ExitUsage, "repo directory is required")
	}
	id, err := comb.ParseTicketID(r.Ticket)
	if err != nil {
		return r, failf(ExitUsage, "%w", err)
	}
	r.Ticket = id
	switch r.Role {
	case "builder", "forager", "guard":
	case "queen", "scout", "waggled":
		return r, failf(ExitUsage, "role %q stays outside dispatch (hive files no lease can cover)", r.Role)
	default:
		return r, failf(ExitUsage, "role %q is not a claim role; use builder, forager or guard", r.Role)
	}
	guard := r.Role == "guard"
	if guard { // a guard takes its directories and gate from the claim
		r.Dirs, r.Gate = nil, ""
	}
	switch {
	case r.Second && !guard:
		return r, failf(ExitUsage, "--second is for the guard role")
	case !guard && r.Gate != "light" && r.Gate != "full":
		return r, failf(ExitUsage, "gate %q: must be light or full", r.Gate)
	}
	if !guard {
		dirs, err := comb.NormalizeDirectories(r.Dirs, config.MaxMaxDirectoriesPerClaim)
		switch {
		case errors.Is(err, comb.ErrTier3Path):
			return r, failf(ExitClaim, "%w", err)
		case err != nil:
			return r, failf(ExitUsage, "%w", err)
		}
		r.Dirs = dirs
	}
	if r.Branch == "" {
		r.Branch = "t-" + strings.TrimPrefix(id, "T-")
	}
	if err := checkBranch(r.Branch, id); err != nil {
		return r, failf(ExitUsage, "%w", err)
	}
	if r.Base != "" && (r.Base == r.Branch || r.Base == "refs/heads/"+r.Branch) {
		return r, failf(ExitUsage, "base %q is the branch being created", r.Base)
	}
	if strings.HasPrefix(r.Base, "-") || strings.ContainsAny(r.Base, " \t\r\n\x00") {
		return r, failf(ExitUsage, "base %q is not a revision", r.Base)
	}
	if r.Tier != nil && (*r.Tier < comb.Tier0Trivial || *r.Tier > comb.Tier3ControlPlane) {
		return r, failf(ExitUsage, "tier %d: must be 0 to 3", *r.Tier)
	}
	if len(r.ReadPaths) > maxReadPaths {
		return r, failf(ExitUsage, "%d read paths exceeds %d", len(r.ReadPaths), maxReadPaths)
	}
	for _, p := range r.ReadPaths {
		if path.Clean(p) != p || p == "." || strings.HasPrefix(p, ":") || p == ".." || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "../") {
			return r, failf(ExitUsage, "read path %q must be a clean repo-relative path", p)
		}
	}
	return r, nil
}

// checkBranch accepts t-<nnn> for this ticket plus an optional suffix. The
// next character after the number must not be a digit (t-0123 is ticket 123).
// It covers the rules of git check-ref-format for this character set.
func checkBranch(branch, ticket string) error {
	prefix := "t-" + strings.TrimPrefix(ticket, "T-")
	rest, ok := strings.CutPrefix(branch, prefix)
	switch {
	case !ok:
		return fmt.Errorf("branch %q must start with %s for ticket %s", branch, prefix, ticket)
	case rest != "" && rest[0] >= '0' && rest[0] <= '9':
		return fmt.Errorf("branch %q belongs to another ticket", branch)
	case len(branch) > 100 || !branchRest.MatchString(rest):
		return fmt.Errorf("branch %q: suffix must be at most 100 characters of [A-Za-z0-9._-]", branch)
	case strings.Contains(branch, "..") || strings.HasSuffix(branch, ".") || strings.HasSuffix(branch, ".lock"):
		return fmt.Errorf("branch %q is not a valid ref name", branch)
	}
	return nil
}
