// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"context"
	"strings"
	"testing"
)

// T-038 A1: brief reviews do not count as build rounds and use no slot.
func TestRoundMustFixSkipsBriefs(t *testing.T) {
	dir, g := newRepo(t)
	ctx := context.Background()
	round := mk("must-fix") + "\n\nVerdict: must-fix\nbody\n"
	count := func(branch string) int {
		t.Helper()
		sh(t, dir, "add", "-A")
		sh(t, dir, "commit", "-q", "--allow-empty", "-m", "reviews")
		base, _ := ResolveBase(ctx, g, "", "")
		req := mustValid(t, okReq(dir))
		req.Branch = branch
		n, err := roundMustFix(ctx, g, base, req)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	put := func(name string) { write(t, dir, ".waggle/hive/reviews/"+name, round) }
	put("T-023-opus.md")
	if n := count("t-023"); n != 1 {
		t.Fatalf("build review: %d, want 1", n)
	}
	for _, n := range []string{"T-023-brief-opus.md", "T-023-brief-fable.md", "T-023-brief24-opus.md", "T-023-brief.md", "brief-T-023-opus.md"} {
		put(n)
	}
	if n := count("t-023"); n != 1 {
		t.Fatalf("with brief reviews: %d, want 1", n)
	}
	put("T-023-ci.md")
	put("T-023-briefing-opus.md")
	put("T-023-Brief-sonnet.md") // not T-023-Brief-opus.md: it would collide with brief-opus on a case-insensitive disk
	if n := count("t-023"); n != 4 {
		t.Fatalf("ci, briefing and Brief names count: %d, want 4", n)
	}
	if n := count("t-023-brief"); n != 2 {
		t.Fatalf("branch t-023-brief reads T-023-brief-{opus,fable}.md after the prefix: %d, want 2", n)
	}
}

func TestRoundMustFixBriefsUseNoSlot(t *testing.T) {
	dir, g := newRepo(t)
	ctx := context.Background()
	round := mk("must-fix") + "\n\nVerdict: must-fix\nbody\n"
	for i := 0; i < maxReviews; i++ {
		write(t, dir, ".waggle/hive/reviews/T-023-brief"+strings.Repeat("1", i)+"-a.md", round)
	}
	write(t, dir, ".waggle/hive/reviews/T-023-zzz.md", round)
	sh(t, dir, "add", "-A")
	sh(t, dir, "commit", "-q", "-m", "reviews")
	base, _ := ResolveBase(ctx, g, "", "")
	if n, err := roundMustFix(ctx, g, base, mustValid(t, okReq(dir))); err != nil || n != 1 {
		t.Fatalf("build file after %d briefs: %d, %v", maxReviews, n, err)
	}
}

// T-038 A2.
func TestErrorCodeUnknownClaim(t *testing.T) {
	if got := ErrorCode(failf(ExitClaim, "x")); got != "refused" {
		t.Fatalf("got %q", got)
	}
}

// T-038 A3: a context cancelled before the call maps environment failures to exit 1.
func TestClaimReleaseCancelled(t *testing.T) {
	f := newFixture(t, false, false)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ClaimTask(cancelled, taskReq(f))
	if code(err) != ExitFailed || ErrorCode(err) != "interrupted" || !strings.HasPrefix(err.Error(), "interrupted: ") || !noClaimRefs(f) {
		t.Fatalf("claim: exit %d code %q err %v", code(err), ErrorCode(err), err)
	}
	if _, err = ClaimTask(cancelled, taskReq(f, func(r *ClaimTaskRequest) { r.Ticket = "x" })); code(err) != ExitUsage {
		t.Fatalf("cancelled bad ticket: %v", err)
	}
	releaseClaim(t, f, "alice")
	_, err = ReleaseTask(cancelled, ReleaseRequest{RepoDir: f.repo, Ticket: "23", Unmerged: true})
	if code(err) != ExitFailed || ErrorCode(err) != "interrupted" || !strings.HasPrefix(err.Error(), "interrupted: ") || !claimKept(t, f) {
		t.Fatalf("release: exit %d code %q err %v", code(err), ErrorCode(err), err)
	}
	if _, err = ReleaseTask(cancelled, ReleaseRequest{RepoDir: f.repo, Ticket: "x"}); code(err) != ExitUsage {
		t.Fatalf("cancelled bad ticket release: %v", err)
	}
}

// T-038 A4.
func TestCheckReleaseBranchLength(t *testing.T) {
	for _, c := range []struct {
		prefix string
		n      int
		ok     bool
	}{
		{"waggle/T-023-", 100, true}, {"waggle/T-023-", 101, false}, {"waggle/T-023-", 107, false},
		{"t-023-", 100, true}, {"t-023-", 101, false},
	} {
		b := c.prefix + strings.Repeat("a", c.n-len(c.prefix))
		err := checkReleaseBranch(b, "T-023")
		if (err == nil) != c.ok || (err != nil && !strings.Contains(err.Error(), "at most 100 characters")) {
			t.Errorf("%s len %d: %v, want ok %v", c.prefix, c.n, err, c.ok)
		}
	}
}

// T-038 A6: --second refuses a record with no model.
func TestSecondRefusesEmptyModel(t *testing.T) {
	p := hiveShape(true)
	for _, c := range []struct{ builder, guard, name string }{
		{"sonnet", "", "T-034-guard-1"}, {"", "opus", "T-034-builder-1"},
	} {
		runs := t.TempDir()
		rec(t, runs, "T-034-builder-1", Result{Model: c.builder})
		rec(t, runs, "T-034-guard-1", Result{HeadOID: "A", Verdict: "pass", Model: c.guard})
		_, _, err := guardModel(runs, p, task34(1, nil), "t-034", "A", true, false)
		if code(err) != ExitEnv || err == nil || !strings.Contains(err.Error(), "--second: run record "+c.name+" has no model") {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

// T-038 A8.
func TestClaimTaskNamesTaskBeforeValidate(t *testing.T) {
	f := newFixture(t, false, false)
	ctx := context.Background()
	res, err := ClaimTask(ctx, taskReq(f, func(r *ClaimTaskRequest) { r.Ticket, r.Gate = "34", "" }))
	if code(err) != ExitUsage || res.Task != "T-034" {
		t.Fatalf("no gate: %+v %v", res, err)
	}
	res, err = ClaimTask(ctx, taskReq(f, func(r *ClaimTaskRequest) { r.Ticket = "x" }))
	if res.Task != "" || ErrorCode(err) != "invalid_task" {
		t.Fatalf("bad ticket: %+v %v", res, err)
	}
	res, err = ClaimTask(ctx, ClaimTaskRequest{Ticket: "34", Dirs: []string{"a"}, Gate: "full"})
	if code(err) != ExitUsage || res.Task != "T-034" {
		t.Fatalf("empty repo: %+v %v", res, err)
	}
}
