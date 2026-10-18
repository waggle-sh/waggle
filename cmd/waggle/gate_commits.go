package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// Limits on what the gate inspects (T-014 section 4.2). Past them the PR fails.
const (
	maxGateCommits = 500
	maxGatePaths   = 5000
	maxSubjectEcho = 72
	maxTreePath    = 4096
)

// errTooLarge is a check failure (exit 1), not a git error.
var errTooLarge = errors.New("PR too large for the gate; split it")

var ticketSubject = regexp.MustCompile(`^(T-[0-9]{3,6}): `)

// gateCommit is one commit of mb..head with its parents and subject.
type gateCommit struct {
	oid     string
	parents []string
	subject string
}

// claimLookup is the lease a ticket commit is checked against; ok is false
// when the claim is missing or malformed (already reported).
type claimLookup struct {
	lease comb.Lease
	ok    bool
}

// commitScan walks every commit of a PR once and records lease and tier
// findings into rep.
type commitScan struct {
	g      gitRunner
	comb   *comb.GitBackend
	now    time.Time
	rep    *gateReport
	claims map[string]claimLookup
}

// listCommits returns mb..head oldest first. One commit past the limit is
// read so the caller can tell the range is too large.
func listCommits(g gitRunner, mb, head string) ([]gateCommit, error) {
	out, err := g.run("rev-list", "--reverse", "--parents", "--format=%s",
		fmt.Sprintf("--max-count=%d", maxGateCommits+1), mb+".."+head)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	// %s never contains a newline, so each commit is exactly two lines.
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines)%2 != 0 {
		return nil, fmt.Errorf("%w rev-list: odd line count %d", errGit, len(lines))
	}
	var cs []gateCommit
	for i := 0; i < len(lines); i += 2 {
		f := strings.Fields(lines[i])
		if len(f) < 2 || f[0] != "commit" || !oidPattern.MatchString(f[1]) {
			return nil, fmt.Errorf("%w rev-list: unexpected line %q", errGit, lines[i])
		}
		cs = append(cs, gateCommit{oid: f[1], parents: f[2:], subject: lines[i+1]})
	}
	return cs, nil
}

// commitChanges lists every path c changes against its single parent, or
// against the empty tree for a root commit. Deletions and both sides of a
// rename are separate entries because renames are off.
func commitChanges(g gitRunner, c gateCommit) ([]comb.Change, error) {
	args := []string{"diff-tree", "-r", "-z", "--no-renames", "--raw", "--no-commit-id"}
	if len(c.parents) == 0 {
		args = append(args, "--root", c.oid)
	} else {
		args = append(args, c.parents[0], c.oid)
	}
	out, err := g.run(args...)
	if err != nil || len(out) == 0 {
		return nil, err
	}
	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("%w diff-tree: odd field count", errGit)
	}
	if len(fields)/2 > maxGatePaths {
		return nil, errTooLarge
	}
	ch := make([]comb.Change, 0, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		m := strings.Fields(strings.TrimPrefix(fields[i], ":"))
		if len(m) != 5 {
			return nil, fmt.Errorf("%w diff-tree: unexpected record %q", errGit, fields[i])
		}
		ch = append(ch, comb.Change{Path: fields[i+1], OldMode: m[0], NewMode: m[1]})
	}
	return ch, nil
}

// scan checks every commit of mb..head. A non-nil error is a git failure or
// errTooLarge; check findings go to s.rep.
func (s *commitScan) scan(mb, head string) error {
	cs, err := listCommits(s.g, mb, head)
	if err != nil {
		return err
	}
	if len(cs) > maxGateCommits {
		return errTooLarge
	}
	for _, c := range cs {
		if len(c.parents) > 1 {
			s.rep.fail(checkLeases, "", fmt.Sprintf("merge commit %.12s in PR; rebase onto main", c.oid))
			continue
		}
		cl, err := s.classify(c)
		if err != nil {
			return err
		}
		changes, err := commitChanges(s.g, c)
		if err != nil {
			return err
		}
		for _, v := range comb.CheckChanges(cl.lease, changes) {
			switch {
			case comb.IsTierRule(v.Rule):
				s.rep.fail(checkTiers, v.Path, v.Detail)
			case cl.ok:
				s.rep.fail(checkLeases, v.Path, cl.lease.TicketID+": "+v.Detail)
			}
		}
	}
	return nil
}

// classify sorts a commit by subject (T-014 section 4.2). Only a ticket commit
// with a readable claim gets per-path lease rules; every other class fails
// leases here and still runs the tier rules with an empty lease.
func (s *commitScan) classify(c gateCommit) (claimLookup, error) {
	if m := ticketSubject.FindStringSubmatch(c.subject); m != nil {
		if id, err := comb.ParseTicketID(m[1]); err == nil {
			return s.claimFor(id)
		}
	}
	subj := c.subject
	if len(subj) > maxSubjectEcho {
		subj = subj[:maxSubjectEcho]
	}
	reason := "commit has no T-<id>: prefix"
	if strings.HasPrefix(c.subject, "Head Keeper: ") {
		reason = "Head Keeper commit: merge with a Head Keeper bypass"
	}
	s.rep.fail(checkLeases, "", fmt.Sprintf("commit %.12s %q: %s", c.oid, subj, reason))
	return claimLookup{}, nil
}

// claimFor reads the claim of one ticket, once. It is the one place leases
// reads a claim, so the signature check of section 9.1 adds one call here.
func (s *commitScan) claimFor(id string) (claimLookup, error) {
	if cl, found := s.claims[id]; found {
		return cl, nil
	}
	var cl claimLookup
	l, err := s.comb.ReadClaim(s.g.ctx, id)
	switch {
	case errors.Is(err, comb.ErrNotFound):
		s.rep.fail(checkLeases, "", "no claim for "+id+"; claims must stay until merge")
	case errors.Is(err, comb.ErrMalformedClaim):
		s.rep.fail(checkLeases, "", id+": "+err.Error())
	case err != nil:
		return cl, fmt.Errorf("%w: read claim %s: %v", errGit, id, err)
	default:
		cl = claimLookup{lease: l, ok: true}
		if !l.ExpiresAt.After(s.now) {
			s.rep.warn(checkLeases, fmt.Sprintf("claim for %s expired at %s; accepted",
				id, l.ExpiresAt.Format(time.RFC3339)))
		}
	}
	s.claims[id] = cl
	return cl, nil
}

// caseCollisions fails head tree paths that are equal under case folding but
// differ in bytes, and paths over maxTreePath bytes (T-014 section 4.4).
func caseCollisions(g gitRunner, head string, rep *gateReport) error {
	out, err := g.run("ls-tree", "-r", "-z", "--name-only", "--full-tree", head)
	if err != nil {
		return err
	}
	seen := map[string]string{}
	for _, p := range strings.Split(string(out), "\x00") {
		switch {
		case p == "":
			continue
		case len(p) > maxTreePath:
			rep.fail(checkTiers, p[:256], fmt.Sprintf("path is %d bytes, over the %d byte limit", len(p), maxTreePath))
			continue
		}
		k := strings.ToLower(p)
		if prev, dup := seen[k]; dup && prev != p && strings.EqualFold(prev, p) {
			rep.fail(checkTiers, p, "path collides with "+prev+" on case-insensitive checkouts")
			continue
		}
		seen[k] = p
	}
	return nil
}
