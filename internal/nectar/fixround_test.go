// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// Fix round for the first guard review of #246 (B6, B7, B11).
func TestRevertScope(t *testing.T) {
	noRevertEntry := func(t *testing.T, rep Report) {
		t.Helper()
		for _, u := range rep.Units {
			if u.Kind == KindRevert {
				t.Fatalf("revert entry %+v", u)
			}
		}
	}
	t.Run("a branch-local revert line in a squash body is not a revert", func(t *testing.T) {
		f := newFx(t)
		f.commit("base", map[string]string{"base.txt": "x\n"})
		f.git("checkout", "-q", "-b", "wip")
		wip := f.commit("wip", map[string]string{"w.txt": "w\n"})
		f.git("checkout", "-q", "main")
		sha := f.commit("T-001: work\n\nThis reverts commit "+wip+".", map[string]string{"src/T-001.txt": lines(10, "T-001")})
		f.addReceipt(f.receipt("T-001", "alice", sha), "")
		rep := f.score(f.clock.Add(40 * day))
		if u := unitFor(t, rep, "T-001"); u.State != StateMatured || keeper(rep, "alice").Nectar != w10 {
			t.Fatalf("%+v %+v", u, rep.Summary.Keepers)
		}
		noRevertEntry(t, rep)
	})
	t.Run("a branch revert of its own wip commit merged with no-ff is not a revert", func(t *testing.T) {
		f := newFx(t)
		f.commit("Head Keeper: base", map[string]string{"base.txt": "x\n"})
		f.git("checkout", "-q", "-b", "t-001-x")
		wip := f.commit("wip", map[string]string{"w.txt": "w\n"})
		f.commit("T-001: work", map[string]string{"src/T-001.txt": lines(10, "T-001")})
		f.tick(time.Hour)
		f.git("revert", "--no-edit", wip)
		f.git("checkout", "-q", "main")
		f.tick(time.Hour)
		f.git("merge", "-q", "--no-ff", "-m", "Merge pull request #1 from x/t-001-x", "t-001-x")
		f.addReceipt(f.receipt("T-001", "alice", f.head()), "")
		rep := f.score(f.clock.Add(40 * day))
		if u := unitFor(t, rep, "T-001"); u.State != StateMatured || keeper(rep, "alice") == nil {
			t.Fatalf("%+v", rep.Units)
		}
		noRevertEntry(t, rep)
	})
	t.Run("a revert before the completing merge drops the unit", func(t *testing.T) {
		f := newFx(t)
		big := f.squash("T-001", map[string]string{"src/big.txt": lines(300, "big")})
		f.addReceipt(f.receipt("T-001", "alice", big), "")
		f.tick(day)
		f.commit("Revert big\n\nThis reverts commit "+big+".", map[string]string{"src/big.txt": del})
		f.tick(day)
		small := f.squash("T-001", map[string]string{"src/small.txt": "s\n"})
		f.addReceipt(f.receipt("T-001", "alice", small), "")
		rep := f.score(f.clock.Add(40 * day))
		u := unitFor(t, rep, "T-001")
		if u.State != StateDropped || !slices.Contains(u.Reasons, "revert_before_maturity") ||
			(keeper(rep, "alice") != nil && keeper(rep, "alice").Nectar != 0) {
			t.Fatalf("%+v %+v", u, rep.Summary.Keepers)
		}
	})
	t.Run("a restore after a revert before the completing merge restores the unit", func(t *testing.T) {
		f := newFx(t)
		big := f.squash("T-001", map[string]string{"src/big.txt": lines(20, "big")})
		f.addReceipt(f.receipt("T-001", "alice", big), "")
		f.tick(day)
		r1 := f.commit("Revert big\n\nThis reverts commit "+big+".", map[string]string{"src/big.txt": del})
		f.tick(day)
		f.commit("Revert Revert\n\nThis reverts commit "+r1+".", map[string]string{"src/big.txt": lines(20, "big")})
		rep := f.score(f.clock.Add(40 * day))
		if u := unitFor(t, rep, "T-001"); u.State != StateMatured {
			t.Fatalf("%+v", u)
		}
	})
}

func TestBranchFallbackNeedsTicketMatch(t *testing.T) {
	f := newFx(t)
	f.commit("Head Keeper: base", map[string]string{"base.txt": "x\n"})
	f.git("checkout", "-q", "-b", "t-002-x")
	f.commit("work", map[string]string{"src/a.txt": lines(10, "a")})
	f.git("checkout", "-q", "main")
	f.tick(time.Hour)
	f.git("merge", "-q", "--no-ff", "-m", "Merge pull request #1 from x/t-002-x", "t-002-x")
	// The receipt names T-001 but its head branch says T-002: the branch does not bind.
	f.addReceipt(f.receipt("T-001", "alice", f.head(), func(r *comb.Receipt) {
		b := "t-002-x"
		r.HeadBranch, r.Branches = &b, []string{b}
	}), "")
	rep := f.score(late)
	// Two unbound: the base commit and the merge.
	if rep.Counts.Unbound != 2 || len(rep.Summary.Keepers) != 0 {
		t.Fatalf("%+v %+v", rep.Counts, rep.Summary)
	}
}

func TestReceiptOrderIndependence(t *testing.T) {
	f := newFx(t)
	a := f.squash("T-001", nil)
	f.addReceipt(f.receipt("T-001", "alice", a), "")
	f.addReceipt(f.receipt("T-001", "bob", a, func(r *comb.Receipt) { r.Source = comb.ReceiptSourceGateRun }), "")
	f.git("checkout", "-q", "-b", "t-002-x")
	f.commit("work", map[string]string{"m.txt": lines(30, "m")})
	f.git("checkout", "-q", "main")
	f.tick(time.Hour)
	f.git("merge", "-q", "--no-ff", "-m", "Merge pull request #2 from x/t-002-x", "t-002-x")
	f.addReceipt(f.receipt("T-002", "carol", f.head()), "")
	f.addReceipt(f.receipt("T-003", "dave", strings.Repeat("c", 40)), "")
	f.addReceipt(f.receipt("T-003", "erin", strings.Repeat("d", 40)), "")
	run := func() string {
		rep := f.score(late)
		s, _ := rep.MarshalJSON()
		d, err := rep.Detail()
		if err != nil {
			t.Fatal(err)
		}
		return string(s) + string(d)
	}
	want := run()
	defer func() { permuteReceipts = nil }()
	perms := map[string]func([]receipt){
		"reverse": slices.Reverse[[]receipt],
		"rotate":  func(r []receipt) { r[0], r[len(r)-1] = r[len(r)-1], r[0]; slices.Reverse(r[1:]) },
	}
	for name, p := range perms {
		permuteReceipts = p
		if got := run(); got != want {
			t.Fatalf("%s: slice order changed the score", name)
		}
	}
}
