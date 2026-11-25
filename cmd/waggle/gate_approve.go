// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/forge"
)

// maxEventFile caps the GitHub event payload the gate reads.
const maxEventFile = 2 << 20

var headSHAPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// prEvent is the part of a pull_request or pull_request_review payload the
// gate reads. Every field is written by GitHub except the body and head ref.
type prEvent struct {
	number  int
	headSHA string
	headRef string
	body    string
	// headRepo and baseRepo are owner/name, or "" when the payload has none
	// (a deleted fork reads as "").
	headRepo, baseRepo string
}

// readPREvent reads the event file named by GITHUB_EVENT_PATH. It returns nil
// and a plain-terms reason when there is no usable pull request event.
func readPREvent(getenv func(string) string) (*prEvent, string) {
	if name := getenv("GITHUB_EVENT_NAME"); name != "pull_request" && name != "pull_request_review" {
		return nil, "event is not a pull request event"
	}
	path := getenv("GITHUB_EVENT_PATH")
	if path == "" {
		return nil, "no GITHUB_EVENT_PATH"
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "event file unreadable"
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxEventFile+1))
	switch {
	case err != nil:
		return nil, "event file unreadable"
	case len(data) > maxEventFile:
		return nil, "event file over 2 MiB"
	}
	var raw struct {
		PullRequest struct {
			Number int `json:"number"`
			Head   struct {
				SHA  string `json:"sha"`
				Ref  string `json:"ref"`
				Repo *struct {
					FullName string `json:"full_name"`
				} `json:"repo"`
			} `json:"head"`
			Base struct {
				Repo *struct {
					FullName string `json:"full_name"`
				} `json:"repo"`
			} `json:"base"`
			Body *string `json:"body"`
		} `json:"pull_request"`
	}
	pr := &raw.PullRequest
	if json.Unmarshal(data, &raw) != nil || pr.Number <= 0 || !headSHAPattern.MatchString(pr.Head.SHA) {
		return nil, "event file malformed"
	}
	ev := &prEvent{number: pr.Number, headSHA: pr.Head.SHA, headRef: pr.Head.Ref}
	if pr.Body != nil {
		ev.body = *pr.Body
	}
	if pr.Head.Repo != nil {
		ev.headRepo = pr.Head.Repo.FullName
	}
	if pr.Base.Repo != nil {
		ev.baseRepo = pr.Base.Repo.FullName
	}
	return ev, ""
}

// reviewSource reads the reviews and conversation comments of one pull
// request. Tests replace newReviewSource with a fake; no gate test talks HTTP.
type reviewSource interface {
	Reviews(ctx context.Context, number int) (forge.PRReviews, error)
	Comments(ctx context.Context, number int) ([]forge.Comment, error)
	ReviewApproves(ctx context.Context, id, login string) ([]string, error)
}

var newReviewSource = func(repository, token string) (reviewSource, error) {
	return forge.NewReviewReader(repository, token)
}

// approvablePath reports the paths a listed approver can pass: listed in the
// base config and outside every hardcoded Tier 3 prefix, so a listing stays
// raise-only. It repeats comb's equal-or-prefix rule over the exported list.
func approvablePath(p comb.TierPolicy) func(string) bool {
	return func(path string) bool {
		if p.RequiredMinTier(path) != comb.Tier3ControlPlane {
			return false
		}
		lower := strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
		for _, h := range comb.HardcodedTier3Prefixes {
			lp := strings.ToLower(h)
			if lower == strings.TrimSuffix(lp, "/") || strings.HasPrefix(lower, lp) {
				return false
			}
		}
		return true
	}
}

// keeperPath reports the paths only the Head Keeper's approval can pass: the
// exact-case .waggle/hive/ carve-out, which no claim can lease. A case variant
// such as .waggle/HIVE/ resolves to Tier 3 and stays out (T-056).
func keeperPath(path string) bool {
	return strings.HasPrefix(path, comb.HiveCarveOut) && comb.RequiredMinTier(path) == comb.Tier2Logic
}

// isHeadKeeper reports whether entry, as approvedBy returns it, is the first
// entry of gate.approvers. approvedBy walks the list in order, so it returns
// the first entry whenever that entry's approval counts.
func isHeadKeeper(entry string, approvers []string) bool {
	return entry != "" && len(approvers) > 0 && strings.EqualFold(entry, approvers[0])
}

// approvedBy returns the config entry of a listed approver whose approval
// passes the PR, or "". Reviews are ordered by ID. For each listed approver
// the counted state is that login's last APPROVED, CHANGES_REQUESTED or
// DISMISSED review. Any counted CHANGES_REQUESTED blocks, and no comment
// clears it. An approval counts only at exactly head and strictly after the
// last base change, and only when it names head on a whole line
// "waggle-approve <head>": in the body of a counted APPROVED review that
// GitHub reports at head, or, since GitHub refuses a review on your own PR, in
// a comment by that login (T-057). The commit GitHub reports for a review
// does not count alone (T-120).
func approvedBy(rv forge.PRReviews, approvers []string, head string) string {
	reviews := sortedReviews(rv.Reviews)
	winner := ""
	for _, entry := range approvers {
		last := lastDeciding(reviews, entry)
		switch {
		case last != nil && last.State == "CHANGES_REQUESTED":
			return ""
		case winner != "":
		case reviewCandidate(last, head, rv.LastBaseChange) && slices.Contains(last.Approves, head):
			winner = entry
		case commentApproves(rv, entry, head):
			winner = entry
		}
	}
	return winner
}

// sortedReviews returns a copy of reviews ordered by ID.
func sortedReviews(reviews []forge.Review) []forge.Review {
	return slices.SortedFunc(slices.Values(reviews), func(a, b forge.Review) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
}

// lastDeciding returns entry's last APPROVED, CHANGES_REQUESTED or DISMISSED
// review in reviews, which are in ID order, or nil.
func lastDeciding(reviews []forge.Review, entry string) *forge.Review {
	var last *forge.Review
	for i, r := range reviews {
		switch r.State {
		case "APPROVED", "CHANGES_REQUESTED", "DISMISSED":
			if strings.EqualFold(r.Login, entry) {
				last = &reviews[i]
			}
		}
	}
	return last
}

// reviewCandidate reports whether r is an APPROVED review that GitHub reports
// at head, submitted strictly after the last base change. Only such a review
// can count, and only once its body names head (approvedBy).
func reviewCandidate(r *forge.Review, head string, lastBase time.Time) bool {
	return r != nil && r.State == "APPROVED" && r.CommitID == head && r.Submitted.After(lastBase)
}

// fillReviewApproves sorts rv.Reviews by ID, clears every Approves, and reads
// the waggle-approve lines of each listed approver's candidate review. It
// returns how many candidates there were and how many bodies could not be
// read; an unread body approves nothing.
func fillReviewApproves(ctx context.Context, src reviewSource, rv *forge.PRReviews, approvers []string, head string) (candidates, unread int) {
	rv.Reviews = sortedReviews(rv.Reviews)
	for i := range rv.Reviews {
		rv.Reviews[i].Approves = nil
	}
	for _, entry := range approvers {
		last := lastDeciding(rv.Reviews, entry)
		if !reviewCandidate(last, head, rv.LastBaseChange) {
			continue
		}
		candidates++
		shas, err := src.ReviewApproves(ctx, last.NodeID, last.Login)
		if err != nil {
			unread++
			continue
		}
		last.Approves = shas
	}
	return candidates, unread
}

// commentApproves reports whether a comment by entry names exactly head on a
// waggle-approve line and was made strictly after the last base change.
func commentApproves(rv forge.PRReviews, entry, head string) bool {
	for _, c := range rv.Comments {
		if strings.EqualFold(c.Login, entry) && c.Created.After(rv.LastBaseChange) && slices.Contains(c.Approves, head) {
			return true
		}
	}
	return false
}

// prRun carries what the approver and trailer checks need after the first scan.
type prRun struct {
	g        gitRunner
	check    string
	in       scanInput
	gha      bool
	getenv   func(string) string
	ev       *prEvent
	evReason string
	pre      []gateFinding // warnings made before the first scan
}

// finish runs the approver check and then the trailer check on a first scan
// that returned nil, and returns the report to print. It never fails the gate:
// every problem leaves the findings as they are.
func (p prRun) finish(rep *gateReport, out scanOutcome) *gateReport {
	reason := p.evReason
	if p.ev != nil && p.ev.headSHA != out.headOID {
		p.ev, reason = nil, "event head differs from --head"
	}
	rep = p.approve(rep, out, reason)
	if p.ev != nil && p.check == "verify" {
		for _, w := range trailerWarnings(p.ev.body, out.bound, out.maxTier) {
			rep.warn(checkLeases, w)
		}
	}
	return rep
}

func (p prRun) approve(rep *gateReport, out scanOutcome, reason string) *gateReport {
	if len(out.approvers) == 0 || (!out.hit && !out.keeperHit) {
		return rep
	}
	chk := checkTiers
	if p.check == checkLeases {
		chk = checkLeases
	}
	skip := func(why string) *gateReport {
		if p.gha {
			rep.warn(chk, "approver check skipped: "+why)
		}
		return rep
	}
	if p.ev == nil {
		return skip("no pull request context (" + reason + ")")
	}
	token := p.getenv("GITHUB_TOKEN")
	if token == "" {
		return skip("no GITHUB_TOKEN")
	}
	src, err := newReviewSource(p.getenv("GITHUB_REPOSITORY"), token)
	if err != nil {
		if errors.Is(err, forge.ErrNoForgeAuth) {
			return skip("no GITHUB_TOKEN")
		}
		return skip("invalid GITHUB_REPOSITORY")
	}
	ctx, cancel := context.WithTimeout(p.g.ctx, 60*time.Second)
	defer cancel()
	rv, err := src.Reviews(ctx, p.ev.number)
	if err != nil {
		var se *forge.StatusError
		if errors.As(err, &se) {
			return skip(fmt.Sprintf("GitHub answered %d", se.Status))
		}
		return skip("GitHub reviews unavailable")
	}
	// A review counts only when its body names the head (T-120). Bodies are
	// read for candidate reviews only and never printed, before the
	// comments so a long comment read cannot use up their time.
	candidates, bodiesUnread := fillReviewApproves(ctx, src, &rv, out.approvers, out.headOID)
	// Comments are a second approval signal; a failed read leaves them empty
	// and the reviews decide alone. Bodies are never printed.
	rv.Comments = nil
	commentsRead := true
	if cs, err := src.Comments(ctx, p.ev.number); err == nil {
		rv.Comments = cs
	} else {
		commentsRead = false
	}
	// unread says so when an approval may have been missed for that reason.
	unread := func(r *gateReport) {
		if !commentsRead {
			r.warn(chk, "PR comments could not be read; only reviews were checked")
		}
		if bodiesUnread > 0 {
			r.warn(chk, "an approving review's body could not be read; that review was not counted")
		}
	}
	short := out.headOID[:12]
	entry := approvedBy(rv, out.approvers, out.headOID)
	if entry == "" {
		rep.warn(chk, "no approving review or waggle-approve comment from a listed approver at "+short)
		if candidates > bodiesUnread {
			rep.warn(chk, "an approving review counts only when its body has the line waggle-approve "+out.headOID)
		}
		unread(rep)
		return rep
	}
	keeper := isHeadKeeper(entry, out.approvers)
	needKeeper := "hive paths need an approving review or waggle-approve comment from the Head Keeper (the first entry of gate.approvers) at " + short
	if !out.hit && !keeper {
		rep.warn(chk, needKeeper)
		unread(rep)
		return rep
	}
	// Rescan at the resolved OIDs with the approvable findings dropped: listed
	// paths for any listed approver, hive paths for the Head Keeper only. The
	// first report stays as it is when this fails.
	again := &gateReport{findings: slices.Clone(p.pre)}
	in := p.in
	in.filter = true
	in.keeperFilter = keeper
	if _, err := scanRange(p.g, out.baseOID, out.headOID, in, again); err != nil {
		return skip("rescan failed")
	}
	if out.hit {
		again.warn(chk, "protected paths approved by "+entry+" at "+short)
	}
	switch {
	case out.keeperHit && keeper:
		again.warn(chk, "hive paths approved by the Head Keeper "+entry+" at "+short)
	case out.keeperHit:
		again.warn(chk, needKeeper)
		unread(again)
	}
	return again
}
