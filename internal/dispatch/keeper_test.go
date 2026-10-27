// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/runtime"
)

const (
	keeperHead = "1111111111111111111111111111111111111111"
	keeperBase = "2222222222222222222222222222222222222222"
)

// keeperRuns is a first guard record of T-045 on t-045b at keeperHead, no builder.
func keeperRuns(t *testing.T, model string) string {
	t.Helper()
	runs := t.TempDir()
	keeperRec(t, runs, "T-045-guard-1", Result{Model: model, HeadOID: keeperHead, BaseOID: keeperBase, Verdict: "pass"})
	return runs
}

func keeperRec(t *testing.T, runs, name string, r Result) {
	t.Helper()
	r.Ticket, r.Branch = "T-045", "t-045b"
	rec(t, runs, name, r)
}

func keeperTask() runtime.TaskContext { return runtime.TaskContext{TaskID: "T-045", SecondGuard: true} }

func wantExit3(t *testing.T, err error, sub string) {
	t.Helper()
	if code(err) != ExitEnv || err == nil || !strings.Contains(err.Error(), sub) {
		t.Fatalf("exit %d err %v; want exit 3 containing %q", code(err), err, sub)
	}
}

// T-045 AC1: --second with the flag needs only the first guard's record.
func TestKeeperSecondModel(t *testing.T) {
	for _, c := range []struct{ first, want string }{{"opus", "fable"}, {"fable", "opus"}} {
		sel, _, err := guardModel(keeperRuns(t, c.first), hiveShape(true), keeperTask(), "t-045b", keeperHead, true, true)
		if err != nil || sel.Model != c.want {
			t.Fatalf("first guard %s: model %q err %v; want %s", c.first, sel.Model, err, c.want)
		}
	}
	_, _, err := guardModel(keeperRuns(t, "opus"), hiveShape(true), keeperTask(), "t-045b", keeperHead, true, false)
	wantExit3(t, err, "--second needs a builder run and a first guard run of T-045 on head 1111111 in ")
	// AC3: a first guard on another head or a second: true record is not this round's.
	_, _, err = guardModel(keeperRuns(t, "opus"), hiveShape(true), keeperTask(), "t-045b", strings.Repeat("3", 40), true, true)
	wantExit3(t, err, "--second needs a first guard run of T-045 on head 3333333 in ")
	runs := t.TempDir()
	keeperRec(t, runs, "T-045-guard-1", Result{Model: "opus", HeadOID: keeperHead, BaseOID: keeperBase, Verdict: "pass", Second: true})
	_, _, err = guardModel(runs, hiveShape(true), keeperTask(), "t-045b", keeperHead, true, true)
	wantExit3(t, err, "--second needs a first guard run of T-045 on head 1111111")
	keeperRec(t, runs, "T-045-guard-2", Result{HeadOID: keeperHead, BaseOID: keeperBase, Verdict: "pass"})
	_, _, err = guardModel(runs, hiveShape(true), keeperTask(), "t-045b", keeperHead, true, true)
	wantExit3(t, err, "--second: run record T-045-guard-2 has no model")
}

// T-045 AC4: no fable, so no other model: ErrNoCrossModel before launch.
func TestKeeperSecondNoAlternate(t *testing.T) {
	_, _, err := guardModel(keeperRuns(t, "opus"), hiveShape(false), keeperTask(), "t-045b", keeperHead, true, true)
	if !errors.Is(err, runtime.ErrNoCrossModel) || code(err) != ExitEnv {
		t.Fatalf("exit %d err %v; want ErrNoCrossModel, exit 3", code(err), err)
	}
}

// T-045 AC2: a builder record on the branch refuses the flag, plain or second.
func TestKeeperRefusesBuilderRecord(t *testing.T) {
	g := &Git{}
	for _, end := range []string{"done", "error"} {
		runs := keeperRuns(t, "opus")
		keeperRec(t, runs, "T-045-builder-1", Result{Model: "sonnet", BaseOID: keeperBase, HeadOID: "A", End: end})
		const want = "--keeper-commit: builder run T-045-builder-1 is on branch t-045b; drop --keeper-commit"
		for _, second := range []bool{false, true} {
			task := keeperTask()
			task.SecondGuard = second
			_, _, err := guardModel(runs, hiveShape(true), task, "t-045b", keeperHead, second, true)
			wantExit3(t, err, want)
		}
		wantExit3(t, checkSecond(context.Background(), g, runs, "T-045", "t-045b", keeperHead, keeperBase, "fable", true, "sonnet"), want)
		// without the flag the builder record is today's input
		if _, _, err := guardModel(runs, hiveShape(true), keeperTask(), "t-045b", keeperHead, true, false); err != nil {
			t.Fatalf("no flag: %v", err)
		}
	}
	other := keeperRuns(t, "opus")
	rec(t, other, "T-045-builder-1", Result{Ticket: "T-045", Branch: "t-045", Model: "sonnet"})
	if sel, _, err := guardModel(other, hiveShape(true), keeperTask(), "t-045b", keeperHead, true, true); err != nil || sel.Model != "fable" {
		t.Fatalf("builder on another branch: %q %v", sel.Model, err)
	}
}

// T-045 AC3: the five checkSecond rows with the flag; no merge-base runs (the
// Git has no repository, so any git call would fail with another text).
func TestKeeperCheckSecond(t *testing.T) {
	cases := []struct {
		name, base, model, builder, want string
		first                            Result
		head                             string
	}{
		{"ok", keeperBase, "fable", "sonnet", "", Result{Model: "opus", BaseOID: keeperBase}, keeperHead},
		{"no first guard on head", keeperBase, "fable", "sonnet", "first guard run of T-045 on head 3333333", Result{Model: "opus", BaseOID: keeperBase}, strings.Repeat("3", 40)},
		{"first guard base not hex", keeperBase, "fable", "sonnet", "run record T-045-guard-1 has no valid base_oid", Result{Model: "opus", BaseOID: "HEAD"}, keeperHead},
		{"base differs", strings.Repeat("4", 40), "fable", "sonnet", "pass --base " + keeperBase, Result{Model: "opus", BaseOID: keeperBase}, keeperHead},
		{"first guard model", keeperBase, "opus", "sonnet", "the model of the first guard (opus)", Result{Model: "opus", BaseOID: keeperBase}, keeperHead},
		{"policy builder model", keeperBase, "sonnet", "sonnet", "--second would use sonnet, the policy's builder model", Result{Model: "opus", BaseOID: keeperBase}, keeperHead},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runs := t.TempDir()
			c.first.HeadOID, c.first.Verdict = keeperHead, "pass"
			keeperRec(t, runs, "T-045-guard-1", c.first)
			keeperRec(t, runs, "T-045-guard-2", Result{Model: "x", BaseOID: keeperBase, HeadOID: c.head, Verdict: "pass", Second: true})
			err := checkSecond(context.Background(), &Git{}, runs, "T-045", "t-045b", c.head, c.base, c.model, true, c.builder)
			if c.want == "" {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			wantExit3(t, err, c.want)
		})
	}
}

type claimStub struct {
	comb.Backend
	lease comb.Lease
	oid   string
	err   error
}

func (c claimStub) ReadClaim(context.Context, string) (comb.Lease, error) { return c.lease, c.err }
func (c claimStub) ClaimOID(context.Context, string) (string, error)      { return c.oid, c.err }

// T-045 AC5: guardTarget reads the claim under the flag.
func TestKeeperGuardTarget(t *testing.T) {
	dir, g := newRepo(t)
	sh(t, dir, "branch", "t-045b")
	ctx := context.Background()
	req := Request{Ticket: "T-045", Branch: "t-045b", KeeperCommit: true}
	plain := req
	plain.KeeperCommit = false

	lease, head, oid, err := guardTarget(ctx, claimStub{err: comb.ErrNotFound}, g, req)
	if err != nil || oid != "" || lease.TicketID != "T-045" || head != sh(t, dir, "rev-parse", "t-045b") {
		t.Fatalf("no claim: lease %+v head %s oid %q err %v", lease, head, oid, err)
	}
	_, _, _, err = guardTarget(ctx, claimStub{err: comb.ErrNotFound}, g, plain)
	if code(err) != ExitClaim || !strings.Contains(err.Error(), "no claim for T-045: the guard reads the builder's claim") {
		t.Fatalf("no flag: exit %d %v", code(err), err)
	}
	listed := claimStub{lease: comb.Lease{TicketID: "T-045", Directories: []string{"internal/x"}, Gate: "light", Branches: []string{"t-045", "t-045b"}}, oid: "abc"}
	_, _, _, err = guardTarget(ctx, listed, g, req)
	wantExit3(t, err, "--keeper-commit: the claim for T-045 lists branch t-045b; drop --keeper-commit")
	if _, _, _, err = guardTarget(ctx, listed, g, plain); err != nil {
		t.Fatalf("no flag, listed branch: %v", err)
	}
	listed.lease.Branches = []string{"t-045"}
	if lease, _, oid, err = guardTarget(ctx, listed, g, req); err != nil || oid != "abc" || lease.TicketID != "T-045" {
		t.Fatalf("other branch listed: lease %+v oid %q err %v", lease, oid, err)
	}
	bad := errors.New("malformed claim")
	_, _, _, err = guardTarget(ctx, claimStub{err: bad}, g, req)
	_, _, _, errPlain := guardTarget(ctx, claimStub{err: bad}, g, plain)
	if err == nil || errPlain == nil || code(err) != code(errPlain) || err.Error() != errPlain.Error() {
		t.Fatalf("malformed claim: %v and %v", err, errPlain)
	}
}

// T-045 AC6: the prompt shows no builder phase's directories under the flag.
func TestKeeperTaskLines(t *testing.T) {
	lease := comb.Lease{Directories: []string{"internal/x"}, Gate: "light"}
	req := Request{Ticket: "T-045", Branch: "t-045b", Head: keeperHead}
	plain := guardTaskLines(req, "abc", lease, nil)
	req.KeeperCommit = true
	keeper := guardTaskLines(req, "abc", lease, nil)
	for i := range plain {
		switch {
		case strings.HasPrefix(plain[i], "Lease directories: "):
			if plain[i] != "Lease directories: internal/x" || keeper[i] != "Lease directories: none (Head Keeper commit)" {
				t.Fatalf("directories: %q and %q", plain[i], keeper[i])
			}
		case strings.HasPrefix(plain[i], "Gate: "):
			if plain[i] != "Gate: light" || keeper[i] != "Gate: full (Head Keeper commit)" {
				t.Fatalf("gate: %q and %q", plain[i], keeper[i])
			}
		case plain[i] != keeper[i]:
			t.Fatalf("line %d changed: %q and %q", i, plain[i], keeper[i])
		}
	}
}

// T-045 AC8: the flag is for the guard role.
func TestKeeperValidate(t *testing.T) {
	for _, role := range []string{"builder", "forager"} {
		_, err := Request{RepoDir: ".", Ticket: "T-045", Role: role, Dirs: []string{"internal/x"}, Gate: "light", KeeperCommit: true}.Validate()
		if code(err) != ExitUsage || err == nil || !strings.Contains(err.Error(), "--keeper-commit is for the guard role") {
			t.Fatalf("%s: exit %d err %v", role, code(err), err)
		}
	}
	if _, err := (Request{RepoDir: ".", Ticket: "T-045", Role: "guard", Branch: "t-045b", KeeperCommit: true}).Validate(); err != nil {
		t.Fatal(err)
	}
}
