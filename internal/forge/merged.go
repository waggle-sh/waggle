// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

// MergedPull is a merged pull request as a release receipt records it (T-051,
// RFC-0005 6.2). HeadBranch is the entry of the branch list that matched,
// MergeSHA the commit the merge left on the base branch, and Approvers every
// login whose latest review is APPROVED at the merged head, not dismissed, and
// names that head on a waggle-approve line in its body (T-121).
type MergedPull struct {
	Number                       int
	Author, HeadBranch, MergeSHA string
	Approvers                    []string
}

// PRFinder is implemented by a forge that can find the merged pull request of
// a task's branches. Only the github forge does.
type PRFinder interface {
	MergedPRByHead(ctx context.Context, branches []string) (MergedPull, bool, error)
}

// mergedPull is the part of a closed pull request object read here.
type mergedPull struct {
	Number   int        `json:"number"`
	MergedAt *time.Time `json:"merged_at"`
	MergeSHA string     `json:"merge_commit_sha"`
	User     struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

// MergedPRByHead returns the newest merged pull request whose head is the
// first of branches that has one. A "<owner>/<repo>:<branch>" entry looks up
// that fork's head. found is false when no branch has a merged pull request.
func (g *github) MergedPRByHead(ctx context.Context, branches []string) (MergedPull, bool, error) {
	for _, b := range branches {
		owner, name := g.owner, b
		if fork, br, ok := strings.Cut(b, ":"); ok {
			owner, name = strings.SplitN(fork, "/", 2)[0], br
		}
		var prs []mergedPull
		q := "?head=" + url.QueryEscape(owner+":"+name) + "&state=closed&per_page=100"
		if _, err := g.t.do(ctx, "GET", g.repoAPI+"/pulls"+q, nil, &prs); err != nil {
			return MergedPull{}, false, fmt.Errorf("forge: merged pull request for %s: %w", b, err)
		}
		var best *mergedPull
		for i, p := range prs {
			if p.MergedAt != nil && p.MergeSHA != "" && (best == nil || p.MergedAt.After(*best.MergedAt)) {
				best = &prs[i]
			}
		}
		if best == nil {
			continue
		}
		rr := &ReviewReader{t: g.t, owner: g.owner, repo: g.repo}
		rv, err := rr.Reviews(ctx, best.Number)
		if err != nil {
			return MergedPull{}, false, fmt.Errorf("forge: reviews of pull request %d: %w", best.Number, err)
		}
		approvers, err := rr.approversAt(ctx, rv.Reviews, best.Head.SHA)
		if err != nil {
			return MergedPull{}, false, fmt.Errorf("forge: review bodies of pull request %d: %w", best.Number, err)
		}
		return MergedPull{Number: best.Number, Author: best.User.Login, HeadBranch: b, MergeSHA: best.MergeSHA,
			Approvers: approvers}, true, nil
	}
	return MergedPull{}, false, nil
}

// NewTokenPRFinder returns a PRFinder for repository ("owner/repo") that talks
// to apiBase with token and never runs gh. It exists for callers that must
// point the read at another API host, such as a test server.
func NewTokenPRFinder(repository, token, apiBase string) (PRFinder, error) {
	owner, repo, ok := strings.Cut(repository, "/")
	if !ok || !ownerPattern.MatchString(owner) || !repoPattern.MatchString(repo) {
		return nil, fmt.Errorf("forge: repository must be owner/repo: %w", ErrForgeRemote)
	}
	if token == "" {
		return nil, fmt.Errorf("forge: no token: %w", ErrNoForgeAuth)
	}
	return &github{t: &tokenTransport{base: apiBase, token: token, client: tokenClient()},
		owner: owner, repo: repo, repoAPI: "/repos/" + owner + "/" + repo}, nil
}

// approversAt is every login whose latest deciding review (reviews come in ID
// order; a comment or pending review decides nothing) is APPROVED at head and
// whose body names head on a waggle-approve line. A dismissed review is
// DISMISSED, so it leaves its login out. The commit GitHub reports for a
// review does not count alone, as in the merge gate (T-120, T-121), so the
// body of each such review is read; a failed read is an error.
func (r *ReviewReader) approversAt(ctx context.Context, reviews []Review, head string) ([]string, error) {
	latest := map[string]Review{}
	for _, rv := range reviews {
		if rv.State != "COMMENTED" && rv.State != "PENDING" {
			latest[rv.Login] = rv
		}
	}
	logins := []string{}
	for login, rv := range latest {
		if rv.State == "APPROVED" && rv.CommitID == head {
			logins = append(logins, login)
		}
	}
	slices.Sort(logins)
	out := []string{}
	for _, login := range logins {
		rv := latest[login]
		shas, err := r.ReviewApproves(ctx, rv.NodeID, rv.Login)
		if err != nil {
			return nil, err
		}
		if slices.Contains(shas, head) {
			out = append(out, login)
		}
	}
	return out, nil
}
