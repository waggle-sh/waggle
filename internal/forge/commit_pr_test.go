// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"slices"
	"strings"
	"testing"
)

const commitPulls = `[
 {"number":7,"merged_at":"2026-10-23T10:00:00Z","merge_commit_sha":"` + headOld + `","user":{"login":"x"},"head":{"sha":"` + headOld + `","ref":"other","repo":{"full_name":"o/r"}}},
 {"number":9,"merged_at":"2026-10-23T11:00:00Z","merge_commit_sha":"` + mergeOK + `","user":{"login":"alice"},"head":{"sha":"` + headNew + `","ref":"t-051","repo":{"full_name":"O/R"}}}]`

// The merged pull request is the one whose merge commit is the commit, not one
// that merely contains it; approvers are read at its merged head.
func TestMergedPRByCommit(t *testing.T) {
	g, log := fakeGitHub(t, map[string]reply{
		"GET /repos/o/r/commits/" + mergeOK + "/pulls": {200, commitPulls},
		"POST /graphql": {200, reviewsBody(
			reviewNode(1, "APPROVED", headNew, "bob"),
			reviewNode(2, "APPROVED", headOld, "dave"),
		)},
	})
	pr, found, err := g.MergedPRByCommit(context.Background(), mergeOK)
	if err != nil || !found {
		t.Fatalf("found %v err %v", found, err)
	}
	if pr.Number != 9 || pr.Author != "alice" || pr.HeadBranch != "t-051" || pr.MergeSHA != mergeOK || !slices.Equal(pr.Approvers, []string{"bob"}) {
		t.Errorf("pr = %+v", pr)
	}
	if len(*log) == 0 || !strings.Contains((*log)[0].query, "per_page=100") {
		t.Errorf("log = %+v", *log)
	}
}

// A head in a fork keeps its owner/repo prefix, the form a claim records.
func TestMergedPRByCommitFork(t *testing.T) {
	body := `[{"number":4,"merged_at":"2026-10-23T11:00:00Z","merge_commit_sha":"` + mergeOK + `","user":{"login":"zed"},"head":{"sha":"` + headNew + `","ref":"t-051","repo":{"full_name":"zed/r"}}}]`
	g, _ := fakeGitHub(t, map[string]reply{
		"GET /repos/o/r/commits/" + mergeOK + "/pulls": {200, body},
		"POST /graphql": {200, reviewsBody()},
	})
	pr, found, err := g.MergedPRByCommit(context.Background(), mergeOK)
	if err != nil || !found || pr.HeadBranch != "zed/r:t-051" {
		t.Fatalf("pr %+v found %v err %v", pr, found, err)
	}
}

func TestMergedPRByCommitNone(t *testing.T) {
	g, _ := fakeGitHub(t, map[string]reply{
		"GET /repos/o/r/commits/" + mergeOK + "/pulls": {200, `[{"number":7,"merged_at":null,"merge_commit_sha":"` + mergeOK + `","user":{"login":"x"},"head":{"sha":"` + headOld + `","ref":"a"}}]`},
	})
	if _, found, err := g.MergedPRByCommit(context.Background(), mergeOK); err != nil || found {
		t.Errorf("found %v err %v", found, err)
	}
	if _, found, err := g.MergedPRByCommit(context.Background(), "not-a-sha"); err == nil || found {
		t.Errorf("bad sha: found %v err %v", found, err)
	}
}

func TestMergedPRByCommitServerError(t *testing.T) {
	g, _ := fakeGitHub(t, map[string]reply{
		"GET /repos/o/r/commits/" + mergeOK + "/pulls": {500, `{}`},
	})
	if _, found, err := g.MergedPRByCommit(context.Background(), mergeOK); err == nil || found {
		t.Errorf("found %v err %v", found, err)
	}
}
