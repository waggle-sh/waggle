// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// CommitPRFinder is implemented by a forge that can find the merged pull
// request whose merge commit is a given commit (T-051 phase D, RFC-0005 6.2:
// the gate-run receipt knows the commit, not the branch). Only the github
// forge does. MergedPull.HeadBranch is the pull request's head branch, with a
// "<owner>/<repo>:" prefix when the head lives in a fork, the form a claim's
// branch list uses.
type CommitPRFinder interface {
	MergedPRByCommit(ctx context.Context, sha string) (MergedPull, bool, error)
}

// commitPull is the part of a pull request object read by commit.
type commitPull struct {
	Number   int        `json:"number"`
	MergedAt *time.Time `json:"merged_at"`
	MergeSHA string     `json:"merge_commit_sha"`
	User     struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		SHA  string `json:"sha"`
		Ref  string `json:"ref"`
		Repo *struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
}

// MergedPRByCommit returns the merged pull request whose merge commit is sha.
// found is false when the forge knows of none; a pull request that merely
// contains sha is not a match. The head's reviews are read once, as
// MergedPRByHead does.
func (g *github) MergedPRByCommit(ctx context.Context, sha string) (MergedPull, bool, error) {
	if !oidPattern.MatchString(sha) {
		return MergedPull{}, false, fmt.Errorf("forge: %q is not a commit id", sha)
	}
	var prs []commitPull
	if _, err := g.t.do(ctx, "GET", g.repoAPI+"/commits/"+url.PathEscape(sha)+"/pulls?per_page=100", nil, &prs); err != nil {
		return MergedPull{}, false, fmt.Errorf("forge: pull requests of commit %s: %w", sha, err)
	}
	for _, p := range prs {
		if p.MergedAt == nil || p.MergeSHA != sha {
			continue
		}
		head := p.Head.Ref
		if r := p.Head.Repo; r != nil && !strings.EqualFold(r.FullName, g.owner+"/"+g.repo) {
			head = r.FullName + ":" + p.Head.Ref
		}
		rv, err := (&ReviewReader{t: g.t, owner: g.owner, repo: g.repo}).Reviews(ctx, p.Number)
		if err != nil {
			return MergedPull{}, false, fmt.Errorf("forge: reviews of pull request %d: %w", p.Number, err)
		}
		return MergedPull{Number: p.Number, Author: p.User.Login, HeadBranch: head, MergeSHA: sha,
			Approvers: approversAt(rv.Reviews, p.Head.SHA)}, true, nil
	}
	return MergedPull{}, false, nil
}
