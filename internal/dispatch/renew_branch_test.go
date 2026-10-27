// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// dispatchReq is a validated builder request on branch, flagged as a dispatch run.
func dispatchReq(t *testing.T, dir, branch string) Request {
	t.Helper()
	r := okReq(dir)
	r.Branch = branch
	r = mustValid(t, r)
	r.addBranch = true
	return r
}

// T-044 AC1 and AC2: a dispatch renewal adds a branch the list lacks.
func TestDispatchRenewalBranches(t *testing.T) {
	dir, _ := newRepo(t)
	b, now := claimEnv(t, dir)
	ctx := context.Background()
	g := &Git{Dir: dir}

	c, err := AcquireClaim(ctx, b, g, dispatchReq(t, dir, "t-023"), "alice", *now)
	if err != nil || c.Verb != "claim" || !slices.Equal(c.Lease.Branches, []string{"t-023"}) {
		t.Fatalf("fresh: %+v %v", c, err)
	}
	c, err = AcquireClaim(ctx, b, g, dispatchReq(t, dir, "t-023"), "alice", *now)
	if err != nil || c.Verb != "renew" || len(c.Notes) != 0 || !slices.Equal(c.Lease.Branches, []string{"t-023"}) {
		t.Fatalf("same branch: %+v %v", c, err)
	}
	light := dispatchReq(t, dir, "t-023")
	light.Gate = "light"
	c, err = AcquireClaim(ctx, b, g, light, "alice", *now)
	if err != nil || len(c.Notes) != 1 || !strings.Contains(c.Notes[0], "full") || !strings.Contains(c.Notes[0], "light") ||
		strings.Count(c.Notes[0], "[t-023]") != 2 || !slices.Equal(c.Lease.Branches, []string{"t-023"}) {
		t.Fatalf("gate change: %+v %v", c, err)
	}
	c, err = AcquireClaim(ctx, b, g, dispatchReq(t, dir, "t-023b"), "alice", *now)
	if err != nil || c.Verb != "renew" || !slices.Equal(c.Lease.Branches, []string{"t-023", "t-023b"}) || len(c.Notes) != 1 ||
		!strings.Contains(c.Notes[0], "[t-023]") || !strings.Contains(c.Notes[0], "[t-023 t-023b]") {
		t.Fatalf("append: %+v %v", c, err)
	}
	c, err = AcquireClaim(ctx, b, g, dispatchReq(t, dir, "t-023"), "alice", *now)
	if err != nil || len(c.Notes) != 0 || !slices.Equal(c.Lease.Branches, []string{"t-023", "t-023b"}) {
		t.Fatalf("listed again: %+v %v", c, err)
	}

	// A fork entry is not the plain branch name.
	given := dispatchReq(t, dir, "t-023")
	given.ClaimBranches = []string{"copilot/x", "owner/repo:t-023"}
	if _, err = AcquireClaim(ctx, b, g, given, "alice", *now); err != nil {
		t.Fatal(err)
	}
	c, err = AcquireClaim(ctx, b, g, dispatchReq(t, dir, "t-023"), "alice", *now)
	if err != nil || !slices.Equal(c.Lease.Branches, []string{"copilot/x", "owner/repo:t-023", "t-023"}) {
		t.Fatalf("fork entry: %+v %v", c, err)
	}
}

// T-044 AC3: a full list that lacks the run's branch refuses before any write.
func TestDispatchRenewalFullList(t *testing.T) {
	dir, _ := newRepo(t)
	b, now := claimEnv(t, dir)
	ctx := context.Background()
	g := &Git{Dir: dir}
	list := []string{"t-023a", "t-023b", "copilot/x", "owner/repo:t-023z"}
	given := dispatchReq(t, dir, "t-023a")
	given.ClaimBranches = list
	if _, err := AcquireClaim(ctx, b, g, given, "alice", *now); err != nil {
		t.Fatal(err)
	}
	before, err := b.ClaimOID(ctx, "T-023")
	if err != nil {
		t.Fatal(err)
	}
	_, err = AcquireClaim(ctx, b, g, dispatchReq(t, dir, "t-023c"), "alice", *now)
	if code(err) != ExitClaim || !errors.Is(err, comb.ErrInvalidLease) || !strings.Contains(err.Error(), "already names 4 branches") {
		t.Fatalf("full list: %v", err)
	}
	for _, s := range append([]string{"t-023c"}, list...) {
		if strings.Contains(err.Error(), s) {
			t.Errorf("message %q names %q", err, s)
		}
	}
	if after, _ := b.ClaimOID(ctx, "T-023"); after != before {
		t.Fatalf("claim ref moved: %s -> %s", before, after)
	}
	c, err := AcquireClaim(ctx, b, g, dispatchReq(t, dir, "t-023b"), "alice", *now)
	if err != nil || !slices.Equal(c.Lease.Branches, list) {
		t.Fatalf("listed branch: %+v %v", c, err)
	}
}

// T-044 AC4: Run's renewal adds its branch and the keepalive repeats it.
func TestRunRenewalAddsBranch(t *testing.T) {
	f := newFixture(t, false, false)
	b, now := claimEnv(t, f.real)
	ctx := context.Background()
	first := mustValid(t, okReq(f.repo))
	if _, err := AcquireClaim(ctx, b, &Git{Dir: f.repo}, first, "alice", *now); err != nil {
		t.Fatal(err)
	}
	rb := &recordBackend{ClaimBackend: b}
	res, err := f.run("sleep 1\n"+okScenario, func(r *Request, d *Deps) {
		r.Branch = "t-023b"
		d.Backend, d.KeepaliveEvery, d.Now = rb, 100*time.Millisecond, func() time.Time { return *now }
	})
	if err != nil {
		t.Fatalf("exit %d err %v warnings %v", res.ExitCode, err, res.Warnings)
	}
	if res.ClaimVerb != "renew" || len(res.ClaimNotes) != 1 || !strings.Contains(res.ClaimNotes[0], "[t-023 t-023b]") ||
		!strings.Contains(res.ClaimNotes[0], "[t-023]") {
		t.Fatalf("verb %q notes %q", res.ClaimVerb, res.ClaimNotes)
	}
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if len(rb.leases) < 2 {
		t.Fatalf("%d leases written; want a claim and a renewal", len(rb.leases))
	}
	for i, l := range rb.leases {
		if !slices.Equal(l.Branches, []string{"t-023", "t-023b"}) {
			t.Errorf("lease %d branches %v", i, l.Branches)
		}
	}
}

// T-044 AC5: waggle claim with no branch keeps the stored list (D-292).
func TestClaimTaskRenewalKeepsList(t *testing.T) {
	f := newFixture(t, false, false)
	b, now := claimEnv(t, f.real)
	ctx := context.Background()
	mut := func(r *ClaimTaskRequest) {
		r.Ticket, r.Dirs, r.Backend, r.Now = "23", []string{"internal/x"}, b, func() time.Time { return *now }
	}
	if _, err := ClaimTask(ctx, taskReq(f, mut, func(r *ClaimTaskRequest) { r.Branches = []string{"t-023b"} })); err != nil {
		t.Fatal(err)
	}
	res, err := ClaimTask(ctx, taskReq(f, mut))
	if err != nil || res.Outcome != "renewed" || !slices.Equal(res.Lease.Branches, []string{"t-023b"}) {
		t.Fatalf("%+v %v", res, err)
	}
}

// T-044 AC6: a takeover or a renewal with no stored list records the run's branch.
func TestDispatchClaimResetsList(t *testing.T) {
	dir, _ := newRepo(t)
	b, now := claimEnv(t, dir)
	ctx := context.Background()
	g := &Git{Dir: dir}
	given := dispatchReq(t, dir, "t-023")
	given.ClaimBranches = []string{"t-023", "x"}
	if _, err := AcquireClaim(ctx, b, g, given, "alice", *now); err != nil {
		t.Fatal(err)
	}
	*now = t0.Add(3 * time.Hour)
	c, err := AcquireClaim(ctx, b, g, dispatchReq(t, dir, "t-023b"), "alice", *now)
	if err != nil || c.Verb != "takeover" || !slices.Equal(c.Lease.Branches, []string{"t-023b"}) {
		t.Fatalf("takeover: %+v %v", c, err)
	}

	dir2, _ := newRepo(t)
	b2, now2 := claimEnv(t, dir2)
	if err := b2.AcquireLease(ctx, comb.Lease{TicketID: "T-023", KeeperID: "alice", Role: "builder", Gate: "full", Directories: []string{"internal/x"}}); err != nil {
		t.Fatal(err)
	}
	c, err = AcquireClaim(ctx, b2, &Git{Dir: dir2}, dispatchReq(t, dir2, "t-023b"), "alice", *now2)
	if err != nil || c.Verb != "renew" || !slices.Equal(c.Lease.Branches, []string{"t-023b"}) {
		t.Fatalf("no stored list: %+v %v", c, err)
	}
}

// T-044 AC7: the guard never claims, flag or not.
func TestAcquireClaimGuardRefused(t *testing.T) {
	dir, _ := newRepo(t)
	b, now := claimEnv(t, dir)
	for _, flag := range []bool{false, true} {
		req := mustValid(t, Request{RepoDir: dir, Ticket: "23", Role: "guard"})
		req.addBranch = flag
		if _, err := AcquireClaim(context.Background(), b, &Git{Dir: dir}, req, "alice", *now); code(err) != ExitUsage {
			t.Fatalf("flag %v: %v", flag, err)
		}
		if _, err := b.ClaimOID(context.Background(), "T-023"); !errors.Is(err, comb.ErrNotFound) {
			t.Fatalf("flag %v: claim written: %v", flag, err)
		}
	}
}
