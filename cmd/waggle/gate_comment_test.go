// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/forge"
	"github.com/waggle-sh/waggle/internal/layout"
)

// TestApproveCommentHivePaths is T-057: GitHub refuses a review on your own
// PR, so a listed approver's comment with a whole line "waggle-approve <head>"
// counts as that approver's approval of head, under the T-056 rules.
func TestApproveCommentHivePaths(t *testing.T) {
	const brief = "file=.waggle/hive/tasks/T-099.md::leases: T-031: path is outside the claim directories"
	const passed = "hive paths approved by the Head Keeper alice at "
	const needKeeper = "hive paths need an approving review or waggle-approve comment from the Head Keeper (the first entry of gate.approvers)"
	type input struct {
		reviews  []forge.Review
		comments []forge.Comment
		base     time.Time
		err      error
	}
	comment := func(login, sha string) forge.Comment {
		return forge.Comment{ID: 1, Login: login, Created: apprTime, Approves: []string{sha}}
	}
	cases := []struct {
		name string
		more func(r gateRepo)
		in   func(head, prev string) input
		code int
		want []string
		not  []string
	}{
		{name: "head keeper comment at head passes hive",
			in:   func(head, _ string) input { return input{comments: []forge.Comment{comment("alice", head)}} },
			code: 0, want: []string{"leases: ok", "tiers: ok", passed}, not: []string{"::error"}},
		{name: "other approver comment does not pass hive",
			in:   func(head, _ string) input { return input{comments: []forge.Comment{comment("bob", head)}} },
			code: 1, want: []string{brief, needKeeper}, not: []string{passed}},
		{name: "unlisted commenter does not count",
			in:   func(head, _ string) input { return input{comments: []forge.Comment{comment("carol", head)}} },
			code: 1, want: []string{brief, noApproval}},
		{name: "comment with the wrong sha fails",
			in:   func(_, _ string) input { return input{comments: []forge.Comment{comment("alice", otherHead)}} },
			code: 1, want: []string{brief, noApproval}, not: []string{passed}},
		{name: "comment at the previous head fails after a new push",
			more: func(r gateRepo) { r.write("docs/b.md", []byte("b\n")); r.commit("T-031: more") },
			in:   func(_, prev string) input { return input{comments: []forge.Comment{comment("alice", prev)}} },
			code: 1, want: []string{brief, noApproval}, not: []string{passed}},
		{name: "comment before the last base change fails",
			in: func(head, _ string) input {
				return input{comments: []forge.Comment{comment("alice", head)}, base: apprTime.Add(time.Minute)}
			}, code: 1, want: []string{brief, noApproval}},
		{name: "comment with no approve line fails",
			in: func(_, _ string) input {
				return input{comments: []forge.Comment{{ID: 1, Login: "alice", Created: apprTime}}}
			}, code: 1, want: []string{brief, noApproval}},
		{name: "no comment and no review fails",
			in: func(_, _ string) input { return input{} }, code: 1, want: []string{brief, noApproval}},
		{name: "unreadable comments leave reviews deciding",
			in:   func(_, _ string) input { return input{err: errors.New("boom")} },
			code: 1, want: []string{brief, noApproval, "PR comments could not be read; only reviews were checked"}},
		{name: "unreadable comments keep the review path",
			in: func(head, _ string) input {
				return input{reviews: []forge.Review{approval(1, "alice", "APPROVED", head)}, err: errors.New("boom")}
			}, code: 0, want: []string{passed}, not: []string{"could not be read"}},
		{name: "head keeper review path still passes",
			in: func(head, _ string) input {
				return input{reviews: []forge.Review{approval(1, "alice", "APPROVED", head)}}
			}, code: 0, want: []string{passed}},
		{name: "changes requested by another approver blocks the comment",
			in: func(head, _ string) input {
				return input{reviews: []forge.Review{approval(1, "bob", "CHANGES_REQUESTED", head)},
					comments: []forge.Comment{comment("alice", head)}}
			}, code: 1, want: []string{brief, noApproval}, not: []string{passed}},
		{name: "changes requested blocks a later comment by the same approver",
			in: func(head, _ string) input {
				c := comment("alice", head)
				c.Created = apprTime.Add(time.Hour)
				return input{reviews: []forge.Review{approval(1, "alice", "CHANGES_REQUESTED", head)},
					comments: []forge.Comment{c}}
			}, code: 1, want: []string{brief, noApproval}, not: []string{passed}},
		{name: "tier 3 control paths still fail",
			more: func(r gateRepo) { r.write(".waggle/control/x.md", []byte("x\n")); r.commit("T-031: control") },
			in:   func(head, _ string) input { return input{comments: []forge.Comment{comment("alice", head)}} },
			code: 1, want: []string{"file=.waggle/control/x.md::tiers: Tier 3 path", passed}, not: []string{brief}},
		{name: "decisions non-append still fails",
			more: func(r gateRepo) {
				r.write(layout.Decisions, []byte(strings.Replace(string(gateLog(2)), "tiny.", "tidy.", 1)))
				r.commit("T-031: edit D-001")
			},
			in:   func(head, _ string) input { return input{comments: []forge.Comment{comment("alice", head)}} },
			code: 1, want: []string{"::error file=" + layout.Decisions + "::", passed}},
		{name: "light claim still fails hive paths",
			more: func(r gateRepo) { r.claimOn(31, []string{"x"}, "light", active, "docs") },
			in:   func(head, _ string) input { return input{comments: []forge.Comment{comment("alice", head)}} },
			code: 1, want: []string{"file=.waggle/hive/tasks/T-099.md::tiers: Tier 2 path under a light claim", passed}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := keeperRepo(t, keeperConfig, tc.more)
			head, prev := r.head(), r.git("rev-parse", "branch~1")
			in := tc.in(head, prev)
			useFake(t, &fakeReviews{rv: forge.PRReviews{Reviews: in.reviews, LastBaseChange: in.base},
				comments: in.comments, commentsErr: in.err})
			code, out := r.runWith(prEnv(writeEvent(t, head, "x", apprBody)), "verify")
			bad := code != tc.code
			for _, w := range tc.want {
				bad = bad || !strings.Contains(out, w)
			}
			for _, w := range tc.not {
				bad = bad || strings.Contains(out, w)
			}
			if bad {
				t.Errorf("exit %d, want %d\n%s", code, tc.code, out)
			}
		})
	}
}

// TestApproveCommentProtectedPaths: a listed approver's comment at head passes
// a repo-listed protected path, as an approving review does.
func TestApproveCommentProtectedPaths(t *testing.T) {
	r := apprRepo(t, apprConfig, nil)
	head := r.head()
	useFake(t, &fakeReviews{comments: []forge.Comment{{ID: 1, Login: "alice", Created: apprTime, Approves: []string{head}}}})
	code, out := r.runWith(prEnv(writeEvent(t, head, "x", apprBody)), "verify")
	if code != 0 || !strings.Contains(out, "protected paths approved by alice at "+head[:12]) {
		t.Errorf("exit %d\n%s", code, out)
	}
}
