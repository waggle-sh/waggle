// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// T-039 AC5: AcquireClaim records the given list, keeps it on a renewal and
// never lets a takeover inherit it.
func TestAcquireClaimBranches(t *testing.T) {
	dir, _ := newRepo(t)
	b, now := claimEnv(t, dir)
	ctx := context.Background()
	req := mustValid(t, okReq(dir))
	g := &Git{Dir: dir}

	c, err := AcquireClaim(ctx, b, g, req, "alice", *now)
	if err != nil || !slices.Equal(c.Lease.Branches, []string{"t-023"}) {
		t.Fatalf("fresh: %+v %v", c, err)
	}
	given := req
	given.ClaimBranches = []string{"copilot/x", "t-023"}
	c, err = AcquireClaim(ctx, b, g, given, "alice", *now)
	if err != nil || c.Verb != "renew" || !slices.Equal(c.Lease.Branches, given.ClaimBranches) || len(c.Notes) != 1 ||
		!strings.Contains(c.Notes[0], "[t-023]") || !strings.Contains(c.Notes[0], "[copilot/x t-023]") {
		t.Fatalf("given list: %+v %v", c, err)
	}
	c, err = AcquireClaim(ctx, b, g, req, "alice", *now)
	if err != nil || c.Verb != "renew" || len(c.Notes) != 0 || !slices.Equal(c.Lease.Branches, given.ClaimBranches) {
		t.Fatalf("plain renewal: %+v %v", c, err)
	}
	*now = t0.Add(3 * time.Hour)
	c, err = AcquireClaim(ctx, b, g, req, "bob", *now)
	if err != nil || c.Verb != "takeover" || !slices.Equal(c.Lease.Branches, []string{"t-023"}) {
		t.Fatalf("takeover: %+v %v", c, err)
	}
}

// T-039 AC6: ClaimTask records, defaults and refuses a branch list.
func TestClaimTaskBranches(t *testing.T) {
	f := newFixture(t, false, false)
	ctx := context.Background()
	res, err := ClaimTask(ctx, taskReq(f, func(r *ClaimTaskRequest) { r.Branches = []string{"feature/x"} }))
	if err != nil || !slices.Equal(res.Lease.Branches, []string{"feature/x"}) {
		t.Fatalf("given: %+v %v", res, err)
	}
	res, err = ClaimTask(ctx, taskReq(f, func(r *ClaimTaskRequest) { r.Ticket, r.Dirs = "33", []string{"docs/a"} }))
	if err != nil || !slices.Equal(res.Lease.Branches, []string{"t-033"}) {
		t.Fatalf("default: %+v %v", res, err)
	}
	before := f.git("for-each-ref", "refs/waggle/claims/")
	_, err = ClaimTask(ctx, taskReq(f, func(r *ClaimTaskRequest) {
		r.Ticket, r.Dirs, r.Branches = "34", []string{"docs/b"}, []string{"a..b"}
	}))
	if code(err) != ExitUsage || ErrorCode(err) != "usage" || !errors.Is(err, comb.ErrInvalidLease) {
		t.Fatalf("bad list: %v", err)
	}
	if got := f.git("for-each-ref", "refs/waggle/claims/"); got != before {
		t.Fatalf("a refused list wrote a ref:\n%s", got)
	}
}

// recordBackend forwards every call and keeps each lease it is asked to write.
type recordBackend struct {
	ClaimBackend
	mu     sync.Mutex
	leases []comb.Lease
}

func (r *recordBackend) AcquireLease(ctx context.Context, l comb.Lease) error {
	r.mu.Lock()
	r.leases = append(r.leases, l)
	r.mu.Unlock()
	return r.ClaimBackend.AcquireLease(ctx, l)
}

// T-039 AC7: the keepalive repeats the list the claim recorded.
func TestRunKeepaliveKeepsBranches(t *testing.T) {
	f := newFixture(t, false, false)
	b, _ := claimEnv(t, f.real)
	rb := &recordBackend{ClaimBackend: b}
	res, err := f.run("sleep 1\n"+okScenario, func(_ *Request, d *Deps) { d.Backend, d.KeepaliveEvery = rb, 100*time.Millisecond })
	if err != nil {
		t.Fatalf("exit %d err %v warnings %v", res.ExitCode, err, res.Warnings)
	}
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if len(rb.leases) < 2 {
		t.Fatalf("%d leases written; want a claim and a renewal", len(rb.leases))
	}
	want := []string{"t-023"}
	for i, l := range rb.leases {
		if !slices.Equal(l.Branches, want) {
			t.Errorf("lease %d branches %v; want %v", i, l.Branches, want)
		}
	}
}
