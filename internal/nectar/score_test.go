// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

const w10 = Milli(1485) // 10 added code lines at tier 0: 1 + log2(1.4)

var late = t0.Add(400 * day)

func TestUnits(t *testing.T) {
	t.Run("squash, merge and rebase each give one unit", func(t *testing.T) {
		f := newFx(t)
		a := f.squash("T-001", nil)
		b := f.pr("t-002-x", "T-002: work", map[string]string{"src/b.txt": lines(10, "b")})
		f.squash("T-003", map[string]string{"src/c1.txt": lines(5, "c")})
		f.commit("T-003: more", map[string]string{"src/c2.txt": lines(5, "c")})
		c := f.head()
		f.addReceipt(f.receipt("T-001", "alice", a), "")
		f.addReceipt(f.receipt("T-002", "bob", b), "")
		f.addReceipt(f.receipt("T-003", "carol", c), "")
		rep := f.score(late)
		if len(rep.Units) != 3 {
			t.Fatalf("want 3 units, got %d: %+v", len(rep.Units), rep.Units)
		}
		for _, id := range []string{"alice", "bob", "carol"} {
			if k := keeper(rep, id); k == nil || k.Units != 1 {
				t.Fatalf("%s: %+v", id, k)
			}
		}
		if u := unitFor(t, rep, "T-003"); len(u.Commits) != 2 || u.MergedSHA != c {
			t.Fatalf("rebase run: %+v", u)
		}
	})
	t.Run("a task in two PRs is one unit completed by the later", func(t *testing.T) {
		f := newFx(t)
		first := f.squash("T-001", map[string]string{"a.txt": lines(10, "a")})
		second := f.squash("T-001", map[string]string{"b.txt": lines(10, "b")})
		f.addReceipt(f.receipt("T-001", "alice", second), "")
		rep := f.score(late)
		u := unitFor(t, rep, "T-001")
		if u.MergedSHA != second || len(u.Commits) != 2 || u.Commits[0] != first || u.Factors.CodeLines != 20 {
			t.Fatalf("%+v", u)
		}
		if k := keeper(rep, "alice"); k == nil || k.Units != 1 {
			t.Fatalf("%+v", k)
		}
	})
	t.Run("a receipt for the earlier merge of the task credits it", func(t *testing.T) {
		f := newFx(t)
		first := f.squash("T-001", nil)
		f.squash("T-001", map[string]string{"b.txt": lines(10, "b")})
		f.addReceipt(f.receipt("T-001", "alice", first), "")
		if u := unitFor(t, f.score(late), "T-001"); u.KeeperID == nil || *u.KeeperID != "alice" {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("a commit bound only through a receipt head_branch binds", func(t *testing.T) {
		f := newFx(t)
		sha := f.commit("tidy things", map[string]string{"x.txt": lines(10, "x")})
		f.addReceipt(f.receipt("T-009", "alice", sha), "")
		rep := f.score(late)
		if u := unitFor(t, rep, "T-009"); u.KeeperID == nil || *u.KeeperID != "alice" {
			t.Fatalf("%+v", u)
		}
		// The same commit with a receipt whose head_branch is not in branches stays unbound.
		g := newFx(t)
		sha = g.commit("tidy things", map[string]string{"x.txt": lines(10, "x")})
		g.addReceipt(g.receipt("T-009", "alice", sha, func(r *comb.Receipt) { r.Branches = []string{"other"} }), "")
		if rep := g.score(late); rep.Counts.Unbound != 1 {
			t.Fatalf("%+v", rep.Counts)
		}
	})
	t.Run("no binding, a maintainer commit and a conflict are unbound", func(t *testing.T) {
		f := newFx(t)
		f.commit("fix typo", map[string]string{"a.txt": "a\n"})
		f.commit("Head Keeper: T-004: pin", map[string]string{"b.txt": "b\n"})
		f.commit("T-005: x\n\nWaggle-Ticket: T-006", map[string]string{"c.txt": "c\n"})
		rep := f.score(late)
		if rep.Counts.Unbound != 3 || len(rep.Units) != 3 {
			t.Fatalf("%+v", rep.Counts)
		}
		for _, u := range rep.Units {
			if u.State != StateUnbound || u.Task != "" {
				t.Fatalf("%+v", u)
			}
		}
	})
	t.Run("a merge of mixed tasks is unbound; a merge subject with no task takes its commits", func(t *testing.T) {
		f := newFx(t)
		f.commit("base", map[string]string{"base.txt": "x\n"})
		f.git("checkout", "-q", "-b", "mixed")
		f.commit("T-001: a", map[string]string{"a.txt": "a\n"})
		f.commit("T-002: b", map[string]string{"b.txt": "b\n"})
		f.git("checkout", "-q", "main")
		f.commit("main moves", map[string]string{"m.txt": "x\n"})
		f.git("merge", "-q", "--no-ff", "-m", "Merge mixed", "mixed")
		if rep := f.score(late); rep.Counts.Unbound != 3 {
			t.Fatalf("base, main and the mixed merge are unbound: %+v", rep.Counts)
		}
	})
}

func TestAttribution(t *testing.T) {
	t.Run("a bot commit follows the claim", func(t *testing.T) {
		f := newFx(t)
		f.git("config", "user.name", "dependabot")
		sha := f.squash("T-001", nil)
		f.addReceipt(f.receipt("T-001", "alice", sha), "")
		if keeper(f.score(late), "alice") == nil {
			t.Fatal("alice not credited")
		}
	})
	t.Run("no valid receipt is unattributed", func(t *testing.T) {
		f := newFx(t)
		f.squash("T-001", nil)
		rep := f.score(late)
		if rep.Summary.Unattributed != 1 || rep.Counts.Unattributed != 1 || len(rep.Summary.Keepers) != 0 {
			t.Fatalf("%+v", rep)
		}
		if u := unitFor(t, rep, "T-001"); u.State != StateUnattributed || u.Weight != w10 {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("an unmerged receipt credits nothing; a later merged one supersedes it", func(t *testing.T) {
		f := newFx(t)
		sha := f.squash("T-001", nil)
		un := f.receipt("T-001", "alice", sha, func(r *comb.Receipt) {
			r.Status, r.MergeProof, r.MergedSHA, r.MergedAt, r.BaseSHA = comb.ReceiptUnmerged, comb.ProofNone, nil, nil, nil
		})
		f.addReceipt(un, "")
		rep := f.score(late)
		if rep.Counts.Unmerged != 1 || rep.Counts.Unattributed != 1 {
			t.Fatalf("%+v", rep.Counts)
		}
		f.addReceipt(f.receipt("T-001", "alice", sha), "")
		if rep = f.score(late); keeper(rep, "alice") == nil || rep.Counts.Unattributed != 0 {
			t.Fatalf("%+v", rep)
		}
	})
	t.Run("invalid receipts credit nothing", func(t *testing.T) {
		f := newFx(t)
		sha := f.squash("T-001", nil)
		other := f.squash("T-002", nil)
		f.addReceipt(f.receipt("T-001", "mallory", other), "") // names another task's merge
		f.addReceipt(f.receipt("T-001", "eve", sha, func(r *comb.Receipt) { r.Branches = []string{"nope"} }), "")
		f.addReceipt(f.receipt("T-002", "eve", sha), "") // ticket differs from the unit's task
		rep := f.score(late)
		if rep.Summary.Unattributed != 2 || len(rep.Summary.Keepers) != 0 {
			t.Fatalf("%+v", rep.Summary)
		}
	})
	t.Run("a release beats gate_run for the same merge; newest wins otherwise", func(t *testing.T) {
		f := newFx(t)
		sha := f.squash("T-001", nil)
		f.addReceipt(f.receipt("T-001", "alice", sha), "")
		f.addReceipt(f.receipt("T-001", "gate", sha, func(r *comb.Receipt) { r.Source = comb.ReceiptSourceGateRun }), "")
		if u := unitFor(t, f.score(late), "T-001"); *u.KeeperID != "alice" || u.Receipt.Source != "release" {
			t.Fatalf("%+v", u)
		}
		f.addReceipt(f.receipt("T-001", "carol", sha), "")
		if u := unitFor(t, f.score(late), "T-001"); *u.KeeperID != "carol" {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("co-keepers split equally and sum to the weight", func(t *testing.T) {
		f := newFx(t)
		sha := f.squash("T-001", nil)
		f.addReceipt(f.receipt("T-001", "alice", sha, func(r *comb.Receipt) { r.CoKeepers = []string{"bob", "carol"} }), "")
		rep := f.score(late)
		var sum Milli
		for _, id := range []string{"alice", "bob", "carol"} {
			sum += keeper(rep, id).Matured
		}
		if sum != w10 || keeper(rep, "bob").Matured != 495 || keeper(rep, "alice").Matured != 495 {
			t.Fatalf("%+v", rep.Summary.Keepers)
		}
		// Three shares of exactly 1.000: a unit of weight 3.000 (tier 1 path at size 1).
		g := newFx(t)
		g.commit("Head Keeper: seed", map[string]string{".waggle/hive/x.md": "x\n"})
		sha = g.commit("T-001: hive", map[string]string{".waggle/hive/x.md": del})
		g.addReceipt(g.receipt("T-001", "alice", sha, func(r *comb.Receipt) { r.CoKeepers = []string{"bob", "carol"} }), "")
		rep = g.score(late)
		if u := unitFor(t, rep, "T-001"); u.Weight != 3000 || u.KeeperShare != 1000 || u.CoKeepers[0].Share != 1000 || u.CoKeepers[1].Share != 1000 {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("third party, maintainer, relay and fork mismatch signals", func(t *testing.T) {
		f := newFx(t)
		f.commit("base", map[string]string{".waggle/control/config.yaml": "version: 1\ngate:\n  approvers: maint\n"})
		a := f.squash("T-001", nil)
		b := f.squash("T-002", nil)
		c := f.squash("T-003", nil)
		d := f.squash("T-004", nil)
		f.addReceipt(f.receipt("T-001", "fork-dev", a, func(r *comb.Receipt) { r.ReleasedBy = "maint" }), "")
		f.addReceiptAs("someone", f.receipt("T-002", "bob", b), "")
		claim := f.commit("claim\n\nWaggle-Relay-Requester: ext\nWaggle-Relay-Approved-By: maint", nil)
		f.addReceipt(f.receipt("T-003", "carol", c), claim)
		f.addReceipt(f.receipt("T-004", "dave", d, func(r *comb.Receipt) { x := "someone-else"; r.PRAuthor = &x }), "")
		rep := f.score(late)
		u1, u2, u3, u4 := unitFor(t, rep, "T-001"), unitFor(t, rep, "T-002"), unitFor(t, rep, "T-003"), unitFor(t, rep, "T-004")
		if !u1.ThirdParty || !u1.Maintainer || *u1.KeeperID != "fork-dev" {
			t.Fatalf("T-001 %+v", u1)
		}
		if !u2.ThirdParty || u2.Maintainer {
			t.Fatalf("T-002 %+v", u2)
		}
		if u3.ThirdParty || !u3.Relay {
			t.Fatalf("T-003 %+v", u3)
		}
		if !u4.ForkMismatch || rep.Counts.ForkMismatch != 1 || rep.Counts.ThirdParty != 2 {
			t.Fatalf("T-004 %+v %+v", u4, rep.Counts)
		}
	})
	t.Run("exclude_keepers drops a keeper and lists its weight", func(t *testing.T) {
		f := newFx(t)
		f.commit("base", map[string]string{".waggle/control/config.yaml": "version: 1\nnectar:\n  exclude_keepers: bot\n"})
		a := f.squash("T-001", nil)
		b := f.squash("T-002", nil)
		f.addReceipt(f.receipt("T-001", "bot", a), "")
		f.addReceipt(f.receipt("T-002", "alice", b), "")
		rep := f.score(late)
		if keeper(rep, "bot") != nil || keeper(rep, "alice") == nil {
			t.Fatalf("%+v", rep.Summary.Keepers)
		}
		if len(rep.Counts.Excluded) != 1 || rep.Counts.Excluded[0] != (ExcludedKeep{"bot", w10}) {
			t.Fatalf("%+v", rep.Counts.Excluded)
		}
	})
	t.Run("the config is read from Main, not the worktree", func(t *testing.T) {
		f := newFx(t)
		a := f.commit("T-001: x", map[string]string{"s.txt": lines(10, "s"), ".waggle/control/config.yaml": "version: 1\n"})
		f.addReceipt(f.receipt("T-001", "bot", a), "")
		f.write(map[string]string{".waggle/control/config.yaml": "version: 1\nnectar:\n  exclude_keepers: bot\n"})
		if keeper(f.score(late), "bot") == nil {
			t.Fatal("the worktree config was read")
		}
	})
	t.Run("a bad config at Main is an error", func(t *testing.T) {
		f := newFx(t)
		f.commit("base", map[string]string{".waggle/control/config.yaml": "version: 1\nnectar:\n  nope: 1\n"})
		if _, err := Score(context.Background(), f.in(late)); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestWeight(t *testing.T) {
	score1 := func(t *testing.T, f *fx, files map[string]string) Unit {
		t.Helper()
		sha := f.squash("T-001", files)
		f.addReceipt(f.receipt("T-001", "alice", sha), "")
		return unitFor(t, f.score(late), "T-001")
	}
	t.Run("a 1,000-line unit caps at 400", func(t *testing.T) {
		u := score1(t, newFx(t), map[string]string{"a.txt": lines(1000, "a")})
		if u.Factors.Size != 5087 || u.Weight != 5087 || u.Factors.CodeLines != 1000 {
			t.Fatalf("%+v", u.Factors)
		}
	})
	t.Run("docs count at half and hive units are docs", func(t *testing.T) {
		u := score1(t, newFx(t), map[string]string{"a.go": lines(100, "a"), "README.md": lines(100, "d")})
		if u.Factors.CodeLines != 100 || u.Factors.DocsLines != 100 || u.Factors.ChangedLines != 150000 || u.Factors.Size != 3807 {
			t.Fatalf("%+v", u.Factors)
		}
		h := score1(t, newFx(t), map[string]string{".waggle/hive/n.md": lines(100, "h")})
		if h.Factors.ChangedLines != 50000 || h.Factors.Tier != 2 || h.Weight != mulMilli(3000, 2585) {
			t.Fatalf("%+v weight %d", h.Factors, h.Weight)
		}
	})
	t.Run("lockfiles and generated files are left out and listed", func(t *testing.T) {
		f := newFx(t)
		f.commit("base", map[string]string{".gitattributes": "gen/** linguist-generated\n"})
		u := score1(t, f, map[string]string{"a.txt": lines(10, "a"), "go.sum": lines(500, "s"), "gen/x.go": lines(50, "g"), "gen/sub/y.go": lines(5, "g")})
		if u.Factors.CodeLines != 10 || strings.Join(u.Factors.ExcludedFiles, ",") != "gen/sub/y.go,gen/x.go,go.sum" {
			t.Fatalf("%+v", u.Factors)
		}
	})
	t.Run("a pure deletion weighs as a minimal unit", func(t *testing.T) {
		f := newFx(t)
		f.commit("base", map[string]string{"a.txt": lines(50, "a")})
		u := score1(t, f, map[string]string{"a.txt": del})
		if u.Factors.Size != 1000 || u.Weight != 1000 || u.Factors.CodeLines != 50 {
			t.Fatalf("%+v", u.Factors)
		}
	})
	t.Run("light and full claims weigh the same", func(t *testing.T) {
		f, g := newFx(t), newFx(t)
		a, b := f.squash("T-001", nil), g.squash("T-001", nil)
		f.addReceipt(f.receipt("T-001", "alice", a), "")
		g.addReceipt(g.receipt("T-001", "alice", b, func(r *comb.Receipt) { r.Gate = "light" }), "")
		if unitFor(t, f.score(late), "T-001").Weight != unitFor(t, g.score(late), "T-001").Weight {
			t.Fatal("differs")
		}
	})
	t.Run("tier comes from the config at the unit's base", func(t *testing.T) {
		f := newFx(t)
		f.commit("base", map[string]string{".waggle/control/config.yaml": "version: 1\n"})
		old := f.squash("T-001", map[string]string{"secure/a.txt": lines(10, "a")})
		f.addReceipt(f.receipt("T-001", "alice", old), "")
		f.commit("cfg", map[string]string{".waggle/control/config.yaml": "version: 1\ngate:\n  protected_paths: secure/\n"})
		neu := f.squash("T-002", map[string]string{"secure/b.txt": lines(10, "b")})
		f.addReceipt(f.receipt("T-002", "alice", neu), "")
		rep := f.score(late)
		if u := unitFor(t, rep, "T-001"); u.Factors.Tier != 0 || u.Weight != w10 {
			t.Fatalf("older unit changed: %+v", u.Factors)
		}
		if u := unitFor(t, rep, "T-002"); u.Factors.Tier != 3 || u.Weight != mulMilli(4000, w10) {
			t.Fatalf("newer unit: %+v", u.Factors)
		}
	})
}

func TestWindow(t *testing.T) {
	f := newFx(t)
	sha := f.squash("T-001", nil)
	f.addReceipt(f.receipt("T-001", "alice", sha), "")
	merged := f.clock.Add(-time.Minute) // the receipt commit tick is after the squash
	merged = time.Unix(0, 0).UTC()
	for _, u := range f.score(late).Units {
		merged, _ = time.Parse(time.RFC3339, u.MergedAt)
	}
	edge := merged.Add(720 * time.Hour)
	for _, tc := range []struct {
		now   time.Time
		state string
	}{{edge.Add(-time.Second), StatePending}, {edge, StateMatured}} {
		rep := f.score(tc.now)
		u := unitFor(t, rep, "T-001")
		if u.State != tc.state || *u.MaturesAt != rfc3339(edge) {
			t.Fatalf("at %v: %+v", tc.now, u)
		}
		k := keeper(rep, "alice")
		if (tc.state == StatePending) != (k.Pending == w10 && k.Nectar == 0) {
			t.Fatalf("%+v", k)
		}
	}
	t.Run("a committer clock that moves back does not reorder units", func(t *testing.T) {
		f := newFx(t)
		a := f.squash("T-001", nil)
		f.clock = f.clock.Add(-10 * day)
		b := f.squash("T-002", nil)
		f.addReceipt(f.receipt("T-001", "alice", a), "")
		f.addReceipt(f.receipt("T-002", "bob", b), "")
		rep := f.score(late)
		if rep.Units[0].MergedSHA != b || rep.Units[1].MergedSHA != a {
			t.Fatalf("units sort by (merged_at, sha) only: %+v", rep.Units)
		}
		if keeper(rep, "alice").Units != 1 || keeper(rep, "bob").Units != 1 {
			t.Fatal("units lost")
		}
	})
	t.Run("a direct push starts at the later of merged_at and first-seen; incomplete delays", func(t *testing.T) {
		f := newFx(t)
		sha := f.squash("T-001", nil)
		f.tick(100 * day)
		f.addReceipt(f.receipt("T-001", "alice", sha, func(r *comb.Receipt) {
			r.PRNumber, r.PRAuthor, r.HeadBranch, r.Approvers, r.Incomplete = nil, nil, nil, []string{}, true
		}), "")
		rep := f.score(f.clock.Add(29 * day))
		u := unitFor(t, rep, "T-001")
		if !u.DirectPush || u.State != StatePending || u.FirstSeen == nil || rep.Counts.DirectPush != 1 || rep.Counts.Incomplete != 1 {
			t.Fatalf("%+v %+v", u, rep.Counts)
		}
		if keeper(rep, "alice") == nil {
			t.Fatal("an incomplete receipt must delay, not remove")
		}
		if u := unitFor(t, f.score(f.clock.Add(31*day)), "T-001"); u.State != StateMatured {
			t.Fatalf("%+v", u)
		}
	})
}

func TestReverts(t *testing.T) {
	revert := func(f *fx, sha string, extra string) string {
		return f.commit("Revert \"x\"\n\nThis reverts commit "+sha+"."+extra, map[string]string{"src/T-001.txt": del})
	}
	t.Run("before maturity the unit never pays", func(t *testing.T) {
		f := newFx(t)
		sha := f.squash("T-001", nil)
		f.addReceipt(f.receipt("T-001", "alice", sha), "")
		f.tick(5 * day)
		r := revert(f, sha, "")
		rep := f.score(late)
		u := unitFor(t, rep, "T-001")
		if u.State != StateDropped || keeper(rep, "alice").Nectar != 0 || keeper(rep, "alice").ClawedBack != 0 || keeper(rep, "alice").Matured != 0 {
			t.Fatalf("%+v %+v", u, keeper(rep, "alice"))
		}
		var rv Unit
		for _, x := range rep.Units {
			if x.Kind == KindRevert {
				rv = x
			}
		}
		if rv.MergedSHA != r || rv.Weight != 0 || rv.KeeperID != nil {
			t.Fatalf("revert entry %+v", rv)
		}
	})
	for _, after := range []time.Duration{31 * day, 400 * day} {
		t.Run("after maturity claws back in full, "+after.String(), func(t *testing.T) {
			f := newFx(t)
			sha := f.squash("T-001", nil)
			other := f.squash("T-002", nil)
			f.addReceipt(f.receipt("T-001", "alice", sha), "")
			f.addReceipt(f.receipt("T-002", "bob", other), "")
			f.tick(after)
			revert(f, sha, "")
			rep := f.score(f.clock.Add(time.Hour))
			k := keeper(rep, "alice")
			if k.Matured != w10 || k.ClawedBack != w10 || k.Nectar != 0 {
				t.Fatalf("%+v", k)
			}
			if b := keeper(rep, "bob"); b.Nectar != w10 {
				t.Fatalf("the reverter's own score changed: %+v", b)
			}
			if u := unitFor(t, rep, "T-001"); u.State != StateClawedBack || u.ClawedBack != w10 {
				t.Fatalf("%+v", u)
			}
		})
	}
	t.Run("a revert -m 1 of a merge commit names the unit", func(t *testing.T) {
		f := newFx(t)
		f.commit("Head Keeper: base", map[string]string{"base.txt": "x\n"})
		m := f.pr("t-001-x", "T-001: work", map[string]string{"src/a.txt": lines(10, "a")})
		f.addReceipt(f.receipt("T-001", "alice", m), "")
		f.tick(40 * day)
		f.git("revert", "-m", "1", "--no-edit", m)
		rep := f.score(f.clock.Add(time.Hour))
		if u := unitFor(t, rep, "T-001"); u.State != StateClawedBack {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("a revert of a revert restores the unit and restarts the window", func(t *testing.T) {
		f := newFx(t)
		sha := f.squash("T-001", nil)
		f.addReceipt(f.receipt("T-001", "alice", sha), "")
		f.tick(40 * day)
		r1 := revert(f, sha, "")
		f.tick(day)
		r2 := f.commit("Revert \"Revert\"\n\nThis reverts commit "+r1+".", map[string]string{"src/T-001.txt": lines(10, "T-001")})
		land := f.clock
		rep := f.score(land.Add(29 * day))
		u := unitFor(t, rep, "T-001")
		if u.State != StatePending || u.Weight != w10 || u.MergedAt != rfc3339(land) || *u.MaturesAt != rfc3339(land.Add(30*day)) {
			t.Fatalf("%+v (r2 %s)", u, r2)
		}
		if k := keeper(rep, "alice"); k.Pending != w10 || k.ClawedBack != 0 || k.Matured != 0 {
			t.Fatalf("%+v", k)
		}
		if u := unitFor(t, f.score(land.Add(30*day)), "T-001"); u.State != StateMatured {
			t.Fatalf("%+v", u)
		}
		// Reverting the restore reverts the unit again.
		f.tick(40 * day)
		f.commit("Revert again\n\nThis reverts commit "+r2+".", map[string]string{"src/T-001.txt": del})
		if u := unitFor(t, f.score(f.clock.Add(time.Hour)), "T-001"); u.State != StateClawedBack {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("a new task re-adding the lines is a new unit and the original stays clawed back", func(t *testing.T) {
		f := newFx(t)
		sha := f.squash("T-001", nil)
		f.addReceipt(f.receipt("T-001", "alice", sha), "")
		f.tick(40 * day)
		revert(f, sha, "")
		again := f.squash("T-002", map[string]string{"src/T-001.txt": lines(10, "T-001")})
		f.addReceipt(f.receipt("T-002", "bob", again), "")
		rep := f.score(f.clock.Add(40 * day))
		if keeper(rep, "alice").Nectar != 0 || keeper(rep, "bob").Nectar != w10 || unitFor(t, rep, "T-001").State != StateClawedBack {
			t.Fatalf("%+v", rep.Summary.Keepers)
		}
	})
	t.Run("a subject-only revert is not a revert", func(t *testing.T) {
		f := newFx(t)
		sha := f.squash("T-001", nil)
		f.addReceipt(f.receipt("T-001", "alice", sha), "")
		f.commit("Revert \"T-001: work\"", map[string]string{"src/T-001.txt": del})
		rep := f.score(late)
		if unitFor(t, rep, "T-001").State != StateMatured || rep.Counts.Unbound != 1 {
			t.Fatalf("%+v", rep.Counts)
		}
	})
}

func TestOrphans(t *testing.T) {
	t.Run("a receipt whose merged_sha left main suspends the unit", func(t *testing.T) {
		f := newFx(t)
		base := f.commit("base", map[string]string{"b.txt": "b\n"})
		gone := f.squash("T-001", nil)
		f.addReceipt(f.receipt("T-001", "alice", gone), "")
		f.git("reset", "-q", "--hard", base)
		f.tick(time.Hour)
		rep := f.score(late)
		if rep.Summary.Orphaned != 1 || rep.Counts.Orphaned != 1 || len(rep.Summary.Keepers) != 0 {
			t.Fatalf("%+v", rep)
		}
		var o Unit
		for _, u := range rep.Units {
			if u.State == StateOrphaned {
				o = u
			}
		}
		if !o.Rewritten || o.Missing || o.MergedSHA != gone {
			t.Fatalf("%+v", o)
		}
		// A later valid receipt for the re-landed task resolves it.
		again := f.squash("T-001", nil)
		f.addReceipt(f.receipt("T-001", "alice", again), "")
		rep = f.score(late)
		if rep.Summary.Orphaned != 0 || keeper(rep, "alice") == nil {
			t.Fatalf("%+v", rep)
		}
	})
	t.Run("a base_sha that is gone too says missing", func(t *testing.T) {
		g := newFx(t)
		g.commit("Head Keeper: base", map[string]string{"b.txt": "b\n"})
		x := strings.Repeat("a", 40)
		g.addReceipt(g.receipt("T-001", "alice", x, func(r *comb.Receipt) { r.BaseSHA = &x }), "")
		rep := g.score(late)
		var o Unit
		for _, u := range rep.Units {
			if u.State == StateOrphaned {
				o = u
			}
		}
		if rep.Summary.Orphaned != 1 || !o.Missing || o.Rewritten || o.Task != "T-001" {
			t.Fatalf("%+v", rep.Units)
		}
	})
}

func TestReplay(t *testing.T) {
	build := func(t *testing.T) *fx {
		f := newFx(t)
		a := f.squash("T-001", nil)
		b := f.pr("t-002-x", "T-002: m", map[string]string{"m.txt": lines(30, "m")})
		f.commit("untracked", map[string]string{"u.txt": "u\n"})
		f.addReceipt(f.receipt("T-001", "alice", a), "")
		f.addReceipt(f.receipt("T-002", "bob", b), "")
		return f
	}
	f := build(t)
	in := f.in(late)
	run := func(in Input) (string, string) {
		rep, err := Score(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		s, _ := rep.MarshalJSON()
		d, err := rep.Detail()
		if err != nil {
			t.Fatal(err)
		}
		return string(s), string(d)
	}
	s1, d1 := run(in)
	if s2, d2 := run(in); s1 != s2 || d1 != d2 {
		t.Fatal("same input, different bytes")
	}
	// Land a commit, append a receipt and move both live heads: the pinned input scores the same.
	f.squash("T-003", nil)
	f.addReceipt(f.receipt("T-003", "carol", f.head()), "")
	if s2, d2 := run(in); s1 != s2 || d1 != d2 {
		t.Fatal("moving the live heads changed a pinned score")
	}
	if !strings.Contains(s1, `"main_sha":"`+in.Main+`"`) || !strings.Contains(s1, `"repo":"acme/repo"`) || !strings.Contains(s1, `"computed_at":"`+rfc3339(late)+`"`) {
		t.Fatal(s1)
	}
	// A second clone that has the later heads.
	clone := t.TempDir()
	f.git("clone", "-q", "--no-local", f.dir, clone+"/c")
	f.git("-C", clone+"/c", "fetch", "-q", f.dir, "+"+f.rc+":refs/receipts")
	cin := in
	cin.Dir = clone + "/c"
	if s2, d2 := run(cin); s1 != s2 || d1 != d2 {
		t.Fatal("a second clone scored differently")
	}
	t.Run("unknown OIDs are errors that name them", func(t *testing.T) {
		bad := in
		bad.Main = strings.Repeat("1", 40)
		if _, err := Score(context.Background(), bad); err == nil || !strings.Contains(err.Error(), bad.Main) {
			t.Fatalf("%v", err)
		}
		bad = in
		bad.Receipts = strings.Repeat("2", 40)
		if _, err := Score(context.Background(), bad); err == nil || !strings.Contains(err.Error(), bad.Receipts) {
			t.Fatalf("%v", err)
		}
		bad.Receipts = in.Main // a main commit is not a receipt commit
		if _, err := Score(context.Background(), bad); err == nil || !strings.Contains(err.Error(), bad.Receipts) {
			t.Fatalf("%v", err)
		}
		bad = in
		bad.RulesVersion = 2
		if _, err := Score(context.Background(), bad); err == nil {
			t.Fatal("want an error for an unknown rules version")
		}
	})
	t.Run("the zero receipts OID scores with no receipts", func(t *testing.T) {
		z := in
		z.Receipts = strings.Repeat("0", 40)
		rep, err := Score(context.Background(), z)
		if err != nil || len(rep.Summary.Keepers) != 0 || rep.Summary.Unattributed != 2 {
			t.Fatalf("%v %+v", err, rep.Summary)
		}
	})
}
