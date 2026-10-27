// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
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

// Bounds on the Waggle-Ticket trailer field of one commit (T-031 section 2.1).
const (
	maxTrailerField  = 256
	maxTrailerValues = 8
	maxTrailerValue  = 16
)

var (
	ticketSubject = regexp.MustCompile(`^(T-[0-9]{3,6}): `)
	// ticketBranch is a branch that binds a ticket: waggle/T-<id> or dispatch's
	// t-<id>, each optionally followed by -<anything> (D-237).
	ticketBranch = regexp.MustCompile(`^(?:waggle/T-|t-)([0-9]{3,6})(?:-.*)?$`)
)

// gateCommit is one commit of mb..head with its parents, subject and the raw
// Waggle-Ticket trailer field (comma separated).
type gateCommit struct {
	oid     string
	parents []string
	subject string
	trailer string
}

// branchTicket returns the ticket a PR branch name binds, or "" when it binds
// none. ignored is true for a name that is too long or holds a control byte;
// the name is never echoed.
func branchTicket(name string) (id string, ignored bool) {
	if len(name) > 255 || strings.IndexFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return "", true
	}
	m := ticketBranch.FindStringSubmatch(name)
	if m == nil {
		return "", false
	}
	id, err := comb.ParseTicketID("T-" + m[1])
	if err != nil {
		return "", false
	}
	return id, false
}

// trailerTickets parses a Waggle-Ticket trailer field into distinct canonical
// IDs. bad is a plain-terms reason when the field is over a bound or holds a
// value that is not a ticket ID; nothing of the value is returned then.
func trailerTickets(raw string) (ids []string, bad string) {
	if raw == "" {
		return nil, ""
	}
	if len(raw) > maxTrailerField || strings.Count(raw, ",")+1 > maxTrailerValues {
		return nil, "Waggle-Ticket trailer too long"
	}
	for _, v := range strings.Split(raw, ",") {
		v = strings.Trim(v, " \t")
		if len(v) > maxTrailerValue {
			return nil, "Waggle-Ticket trailer too long"
		}
		id, err := comb.ParseTicketID(v)
		if err != nil {
			return nil, "malformed Waggle-Ticket trailer"
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids, ""
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

	baseOID string
	policy  comb.TierPolicy // repo-defined Tier 3 paths, read from baseOID
	branch  string          // ticket the PR branch binds, or ""
	bound   map[string]string
	head    prHead // the PR head every claim's branch list is checked against
	noHead  bool   // the local no-branch warning was filed

	approvable func(string) bool // paths a listed approver can pass; nil means none
	filter     bool              // drop tier3 and outside_lease findings on approvable paths
	hit        bool              // a tier3 finding on an approvable path was seen
	maxTier    comb.RiskTier     // highest tier the changed paths require
}

// listCommits returns mb..head oldest first. One commit past the limit is
// read so the caller can tell the range is too large.
func listCommits(g gitRunner, mb, head string) ([]gateCommit, error) {
	// git omits an empty format line, so a leading "." keeps an empty subject
	// or trailer field on its own line. The unfolded trailer values are joined
	// with a comma, so each commit stays a fixed number of lines.
	out, err := g.run("rev-list", "--reverse", "--parents",
		"--format=.%s%n.%(trailers:key=Waggle-Ticket,valueonly,unfold,separator=%x2C)",
		fmt.Sprintf("--max-count=%d", maxGateCommits+1), mb+".."+head)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	// %s never contains a newline and the trailer is unfolded, so each commit
	// is exactly three lines.
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines)%3 != 0 {
		return nil, fmt.Errorf("%w rev-list: odd line count %d", errGit, len(lines))
	}
	var cs []gateCommit
	for i := 0; i < len(lines); i += 3 {
		f := strings.Fields(lines[i])
		subject, dot := strings.CutPrefix(lines[i+1], ".")
		trailer, tdot := strings.CutPrefix(lines[i+2], ".")
		if len(f) < 2 || f[0] != "commit" || !oidPattern.MatchString(f[1]) || !dot || !tdot {
			return nil, fmt.Errorf("%w rev-list: unexpected line %q", errGit, lines[i])
		}
		cs = append(cs, gateCommit{oid: f[1], parents: f[2:], subject: subject, trailer: trailer})
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
		diffFrom := c
		if len(c.parents) > 1 {
			base, err := s.isBaseMerge(c)
			if err != nil {
				return err
			}
			if !base {
				s.rep.fail(checkLeases, "", fmt.Sprintf("merge commit %.12s in PR; rebase onto main (only an update from the base branch is accepted)", c.oid))
				continue
			}
			// What the PR brings to base is the diff from the base side.
			diffFrom.parents = []string{c.parents[1]}
		}
		cl, err := s.classify(c)
		if err != nil {
			return err
		}
		changes, err := commitChanges(s.g, diffFrom)
		if err != nil {
			return err
		}
		for _, ch := range changes {
			s.maxTier = max(s.maxTier, s.policy.RequiredMinTier(ch.Path))
		}
		for _, v := range comb.CheckChangesPolicy(cl.lease, changes, s.policy) {
			if s.approvable != nil && s.approvable(v.Path) && (v.Rule == comb.RuleTier3 || v.Rule == comb.RuleOutsideLease) {
				s.hit = s.hit || v.Rule == comb.RuleTier3
				if s.filter {
					continue
				}
			}
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

// isBaseMerge reports whether c is an update from the base branch: two
// parents, and the only merge-base of c and the base tip is its second parent.
// That rules out a second parent that is older than base commits already in
// the first parent, or a side-branch commit base merged later (T-031 2.3).
func (s *commitScan) isBaseMerge(c gateCommit) (bool, error) {
	if len(c.parents) != 2 {
		return false, nil
	}
	out, err := s.g.run("merge-base", "--all", c.oid, s.baseOID)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == c.parents[1], nil
}

// classify binds a commit to a ticket (T-031 section 2.1): subject prefix,
// Waggle-Ticket trailer, then PR branch; the first present source wins and a
// source naming another ticket is a conflict. Only a bound commit with a
// readable claim gets per-path lease rules; every other class fails leases here
// and still runs the tier rules with an empty lease.
func (s *commitScan) classify(c gateCommit) (claimLookup, error) {
	subj := c.subject
	if len(subj) > maxSubjectEcho {
		subj = subj[:maxSubjectEcho]
	}
	if strings.HasPrefix(c.subject, "Head Keeper: ") {
		s.rep.fail(checkLeases, "", fmt.Sprintf("commit %.12s %q: maintainer commit (subject starts \"Head Keeper: \"): merge with an admin bypass", c.oid, subj))
		return claimLookup{}, nil
	}
	var subjectID string
	if m := ticketSubject.FindStringSubmatch(c.subject); m != nil {
		subjectID, _ = comb.ParseTicketID(m[1])
	}
	trailerIDs, bad := trailerTickets(c.trailer)
	if bad != "" {
		s.rep.fail(checkLeases, "", fmt.Sprintf("commit %.12s: %s", c.oid, bad))
		return claimLookup{}, nil
	}
	var id string
	for _, cand := range append(append([]string{subjectID}, trailerIDs...), s.branch) {
		if cand == "" {
			continue
		}
		if id == "" {
			id = cand
		} else if cand != id {
			var parts []string
			if subjectID != "" {
				parts = append(parts, "subject "+subjectID)
			}
			if len(trailerIDs) > 0 {
				parts = append(parts, "trailer "+strings.Join(trailerIDs, ", "))
			}
			if s.branch != "" {
				parts = append(parts, "branch "+s.branch)
			}
			s.rep.fail(checkLeases, "", fmt.Sprintf("commit %.12s: ticket conflict: %s", c.oid, strings.Join(parts, ", ")))
			return claimLookup{}, nil
		}
	}
	if id == "" && len(c.parents) == 2 {
		// A base merge with nothing to bind takes its first parent's ticket.
		id = s.bound[c.parents[0]]
	}
	if id == "" {
		s.rep.fail(checkLeases, "", fmt.Sprintf("commit %.12s %q: commit has no T-<id>: prefix, Waggle-Ticket: trailer or ticket branch", c.oid, subj))
		return claimLookup{}, nil
	}
	s.bound[c.oid] = id
	return s.claimFor(id)
}

// claimFor reads the claim of one ticket, once. It is the one place leases
// reads a claim, so the signature check of section 9.1 adds one call here. A
// readable claim is also checked against the PR head (checkHead).
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
		s.checkHead(id, l)
	}
	s.claims[id] = cl
	return cl, nil
}

// checkHead fails a bound ticket whose claim does not name the PR head
// (T-039 section 2.4). The finding has no path, so no approval clears it, and
// it never echoes the head name, a repository or a claim entry.
func (s *commitScan) checkHead(id string, l comb.Lease) {
	h := s.head
	if h.name == "" {
		switch {
		case h.gha:
			s.rep.fail(checkLeases, "", id+": pull request branch unknown")
		case !s.noHead:
			s.noHead = true
			s.rep.warn(checkLeases, "no pull request branch; claim branches not checked (pass --branch)")
		}
		return
	}
	if len(l.Branches) == 0 {
		s.rep.fail(checkLeases, "", id+": claim names no branch; renew it with waggle claim --branch")
		return
	}
	for _, e := range l.Branches {
		repo, branch, fork := strings.Cut(e, ":")
		switch {
		case !fork && h.sameRepo && e == h.name:
			return
		case fork && !h.sameRepo && h.repo != "" && strings.EqualFold(repo, h.repo) && branch == h.name:
			return
		}
	}
	s.rep.fail(checkLeases, "", id+": claim does not name this pull request's branch")
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
