// SPDX-License-Identifier: Apache-2.0

// Package relay is the pure core of the `/waggle` comment relay (T-065): the
// request grammar, the decoded GitHub event, the approval digest and the
// policy that stands between attacker text and a claim write.
//
// Nothing here does I/O. A comment body, a PR title, a branch name and every
// event field other than GitHub's own identity fields are attacker text, so
// the parser accepts printable ASCII tokens only, never builds a shell line,
// and every refusal reason is fixed text that does not quote the request.
package relay

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/waggle-sh/waggle/internal/comb"
)

// MaxBodyBytes caps a comment body the parser looks at.
const MaxBodyBytes = 4096

// Commands the grammar accepts.
const (
	CommandClaim   = "claim"
	CommandRelease = "release"
	CommandApprove = "approve-claim"
)

// Gate values a claim request may name.
const (
	GateLight = "light"
	GateFull  = "full"
)

// ErrNoRequest means the body holds no line that starts with "/waggle ". The
// comment is not for the relay; it is ignored, not answered.
var ErrNoRequest = errors.New("relay: no /waggle request line")

// Refusal is a request the relay refuses. Reason is fixed text safe to post
// as a reply: it never quotes the request.
type Refusal struct{ Reason string }

// Error implements error.
func (r *Refusal) Error() string { return "relay: refused: " + r.Reason }

func refuse(format string, args ...any) *Refusal {
	return &Refusal{Reason: fmt.Sprintf(format, args...)}
}

// Request is one parsed command. The owner is never part of it: a claim is
// always owned by the commenter.
type Request struct {
	Command string
	// Task is the canonical ticket id (T-001). Set for claim and release.
	Task string
	// Dirs are normalized and sorted. Set for claim.
	Dirs []string
	// Branches are the --branch entries in the order given. Set for claim.
	Branches []string
	// TTLMinutes is the requested lease length, 0 when the request names none.
	TTLMinutes int
	// Gate is "light" or "full" (default "full"). Set for claim.
	Gate string
	// Digest is the 12 hex digits an approval names. Set for approve-claim.
	Digest string
}

var (
	ttlPattern    = regexp.MustCompile(`^[0-9]{1,6}m$`)
	digestPattern = regexp.MustCompile(`^[0-9a-f]{12}$`)
)

// flagsRefused are options `waggle claim` has that a relay request must not
// carry: the owner is the commenter and the base and repo come from the
// workflow.
var flagsRefused = map[string]bool{"--owner": true, "--base": true, "--repo": true, "--json": true}

// Parse reads the first line of body that starts with "/waggle " (after
// trimming spaces) and parses it. Other lines are ignored; the gate's
// `waggle-approve <sha>` comment never matches. maxDirs is the limit
// `leases.max_directories_per_claim` sets, 0 for the default.
//
// No such line returns ErrNoRequest. Everything else wrong returns *Refusal.
func Parse(body string, maxDirs int) (Request, error) {
	if len(body) > MaxBodyBytes {
		return Request{}, refuse("the comment is over %d bytes", MaxBodyBytes)
	}
	if !utf8.ValidString(body) || strings.ContainsRune(body, 0) {
		return Request{}, refuse("the comment is not valid text")
	}
	var line string
	found := false
	for _, l := range strings.Split(body, "\n") {
		l = strings.Trim(strings.TrimSuffix(l, "\r"), " ")
		if strings.HasPrefix(l, "/waggle ") {
			line, found = l, true
			break
		}
	}
	if !found {
		return Request{}, ErrNoRequest
	}
	for i := 0; i < len(line); i++ {
		if c := line[i]; c < 0x20 || c > 0x7e {
			return Request{}, refuse("the command line must be printable ASCII with spaces between words")
		}
	}
	if strings.ContainsAny(line, "\"'`\\") {
		return Request{}, refuse("quotes and escapes are not supported")
	}
	toks := strings.Fields(line)[1:]
	if len(toks) == 0 {
		return Request{}, refuse("no command; use claim, release or approve-claim")
	}
	cmd, args := toks[0], toks[1:]
	switch cmd {
	case CommandClaim:
		return parseClaim(args, maxDirs)
	case CommandRelease:
		if len(args) != 1 || strings.HasPrefix(args[0], "-") {
			return Request{}, refuse("usage: /waggle release <task>")
		}
		task, err := comb.ParseTicketID(args[0])
		if err != nil {
			return Request{}, refuse("the task id is not valid")
		}
		return Request{Command: CommandRelease, Task: task}, nil
	case CommandApprove:
		if len(args) != 1 || !digestPattern.MatchString(args[0]) {
			return Request{}, refuse("usage: /waggle approve-claim <12 hex digit digest>")
		}
		return Request{Command: CommandApprove, Digest: args[0]}, nil
	}
	return Request{}, refuse("unknown command; use claim, release or approve-claim")
}

func parseClaim(args []string, maxDirs int) (Request, error) {
	const usage = "usage: /waggle claim <task> --dirs <a,b> --branch <owner>/<repo>:<branch> [--ttl <minutes>m] [--gate light|full]"
	req := Request{Command: CommandClaim, Gate: GateFull}
	var haveDirs, haveTTL, haveGate bool
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			if req.Task != "" {
				return Request{}, refuse("%s", usage)
			}
			task, err := comb.ParseTicketID(a)
			if err != nil {
				return Request{}, refuse("the task id is not valid")
			}
			req.Task = task
			continue
		}
		if flagsRefused[a] {
			return Request{}, refuse("%s is not allowed in a relay request: the owner is the commenter", a)
		}
		switch a {
		case "--dirs", "--branch", "--ttl", "--gate":
		default:
			return Request{}, refuse("unknown option; %s", usage)
		}
		i++
		if i >= len(args) || strings.HasPrefix(args[i], "-") {
			return Request{}, refuse("%s needs a value", a)
		}
		v := args[i]
		switch a {
		case "--dirs":
			if haveDirs {
				return Request{}, refuse("--dirs given twice")
			}
			haveDirs = true
			dirs, err := comb.NormalizeDirectories(strings.Split(v, ","), maxDirs)
			switch {
			case errors.Is(err, comb.ErrTier3Path):
				return Request{}, refuse("the repository root cannot be leased")
			case err != nil:
				return Request{}, refuse("--dirs must be literal repo-relative directories, at most the allowed number")
			}
			req.Dirs = dirs
		case "--branch":
			req.Branches = append(req.Branches, v)
		case "--ttl":
			if haveTTL {
				return Request{}, refuse("--ttl given twice")
			}
			haveTTL = true
			if !ttlPattern.MatchString(v) {
				return Request{}, refuse("--ttl must be a number of minutes such as 120m")
			}
			n, _ := strconv.Atoi(strings.TrimSuffix(v, "m"))
			if n < 1 {
				return Request{}, refuse("--ttl must be at least 1m")
			}
			req.TTLMinutes = n
		case "--gate":
			if haveGate {
				return Request{}, refuse("--gate given twice")
			}
			haveGate = true
			if v != GateLight && v != GateFull {
				return Request{}, refuse("--gate must be light or full")
			}
			req.Gate = v
		}
	}
	if req.Task == "" || !haveDirs {
		return Request{}, refuse("%s", usage)
	}
	if err := comb.ValidateBranches(req.Branches); err != nil {
		return Request{}, refuse("--branch must be <owner>/<repo>:<branch>, at most 4, no repeats")
	}
	return req, nil
}
