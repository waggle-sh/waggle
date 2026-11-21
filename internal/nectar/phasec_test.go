// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// mixed is n lines of tag with the first k replaced.
func mixed(k, n int, tag string) string {
	var b strings.Builder
	b.WriteString(lines(k, "new"))
	for i := k; i < n; i++ {
		fmt.Fprintf(&b, "%s %d\n", tag, i)
	}
	return b.String()
}

// drop3 is the 10 lines of T-001 without the first 3: a revert of 30 percent.
var drop3 = lines(10, "T-001")[len("T-001 0\nT-001 1\nT-001 2\n"):]

// one lands T-001 with 10 code lines and a receipt for alice.
func one(f *fx, files map[string]string) string {
	sha := f.squash("T-001", files)
	f.addReceipt(f.receipt("T-001", "alice", sha), "")
	return sha
}

func detail(t *testing.T, r Report) string {
	t.Helper()
	b, err := r.Detail()
	if err != nil {
		t.Fatal(err)
	}
	sum, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(sum) + string(b)
}

func TestSurvival(t *testing.T) {
	rewrite := func(t *testing.T, k int) Unit {
		f := newFx(t)
		one(f, nil)
		f.tick(day)
		f.commit("T-002: rewrite", map[string]string{"src/T-001.txt": mixed(k, 10, "T-001")})
		return unitFor(t, f.score(late), "T-001")
	}
	t.Run("a later task rewriting over half the lines drops the unit", func(t *testing.T) {
		u := rewrite(t, 6)
		if u.State != StateDropped || !slices.Contains(u.Reasons, "alive_below_ratio") || u.AliveRatio == nil || *u.AliveRatio != 400 {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("exactly half alive survives", func(t *testing.T) {
		u := rewrite(t, 5)
		if u.State != StateMatured || u.AliveRatio == nil || *u.AliveRatio != 500 {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("a move, a rename and a whitespace reformat keep their blame", func(t *testing.T) {
		f := newFx(t)
		one(f, nil)
		f.tick(day)
		f.commit("T-002: move", map[string]string{"src/T-001.txt": del, "src/moved.txt": lines(10, "T-001")})
		f.commit("T-003: reformat", map[string]string{"src/moved.txt": strings.ReplaceAll(lines(10, "T-001"), "T-001", "  T-001")})
		u := unitFor(t, f.score(late), "T-001")
		if u.State != StateMatured || u.AliveRatio == nil || *u.AliveRatio != 1000 {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("lines moved into another file keep their blame", func(t *testing.T) {
		f := newFx(t)
		one(f, nil)
		f.tick(day)
		f.commit("T-002: split", map[string]string{"src/T-001.txt": "", "src/other.txt": lines(10, "T-001") + lines(5, "x")})
		u := unitFor(t, f.score(late), "T-001")
		if u.State != StateMatured || u.AliveRatio == nil || *u.AliveRatio != 1000 {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("a rev in .git-blame-ignore-revs is ignored", func(t *testing.T) {
		build := func(t *testing.T, ignore bool) Unit {
			f := newFx(t)
			one(f, nil)
			f.tick(day)
			r := f.commit("T-002: format", map[string]string{"src/T-001.txt": strings.ReplaceAll(lines(10, "T-001"), "\n", " ;\n")})
			if ignore {
				f.commit("T-003: ignore", map[string]string{".git-blame-ignore-revs": "# format\n" + r + "\n0000000000000000000000000000000000000001\n"})
			} else {
				f.commit("T-003: note", map[string]string{"NOTES": "x\n"})
			}
			return unitFor(t, f.score(late), "T-001")
		}
		if u := build(t, false); u.State != StateDropped {
			t.Fatalf("without the file: %+v", u)
		}
		if u := build(t, true); u.State != StateMatured || *u.AliveRatio != 1000 {
			t.Fatalf("with the file: %+v", u)
		}
	})
	t.Run("lines in generated and binary files do not count as alive", func(t *testing.T) {
		f := newFx(t)
		f.commit("base", map[string]string{".gitattributes": "gen/** linguist-generated\n"})
		one(f, map[string]string{"src/T-001.txt": lines(10, "T-001"), "gen/out.txt": lines(50, "g"), "bin/b.dat": "\x00" + lines(50, "b")})
		f.tick(day)
		f.commit("T-002: rewrite", map[string]string{"src/T-001.txt": lines(10, "new"), "gen/out.txt": lines(51, "g"),
			"bin/b.dat": "\x00" + lines(51, "b")})
		if u := unitFor(t, f.score(late), "T-001"); u.State != StateDropped || u.AliveRatio == nil || *u.AliveRatio != 0 {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("the clone's own git config does not change survival", func(t *testing.T) {
		f := newFx(t)
		one(f, nil)
		f.tick(day)
		r := f.commit("T-002: format", map[string]string{"src/T-001.txt": strings.ReplaceAll(lines(10, "T-001"), "\n", " ;\n")})
		ign := filepath.Join(f.dir, ".git", "local-ignore-revs")
		if err := os.WriteFile(ign, []byte(r+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		f.git("config", "blame.ignoreRevsFile", ign)
		f.git("config", "blame.markIgnoredLines", "true")
		f.git("config", "diff.algorithm", "patience")
		f.git("config", "diff.noprefix", "true")
		f.git("config", "diff.context", "0")
		if u := unitFor(t, f.score(late), "T-001"); u.State != StateDropped || *u.AliveRatio != 0 {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("docs lines count in the ratio", func(t *testing.T) {
		files := map[string]string{"src/T-001.txt": lines(2, "c"), "docs/a.md": lines(10, "d")}
		f := newFx(t)
		one(f, files)
		f.tick(day)
		f.commit("T-002: docs", map[string]string{"docs/a.md": lines(10, "e")})
		u := unitFor(t, f.score(late), "T-001")
		if u.State != StateDropped || *u.AliveRatio != 167 {
			t.Fatalf("%+v", u)
		}
		g := newFx(t)
		one(g, files)
		g.tick(day)
		g.commit("T-002: code", map[string]string{"src/T-001.txt": lines(2, "e")})
		if u := unitFor(t, g.score(late), "T-001"); u.State != StateMatured || *u.AliveRatio != 833 {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("hive-only lines count too", func(t *testing.T) {
		f := newFx(t)
		one(f, map[string]string{".waggle/hive/notes.md": lines(10, "n")})
		f.tick(day)
		f.commit("T-002: gone", map[string]string{".waggle/hive/notes.md": del})
		if u := unitFor(t, f.score(late), "T-001"); u.State != StateDropped {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("a rewrite after the window does not drop it", func(t *testing.T) {
		f := newFx(t)
		one(f, nil)
		f.tick(40 * day)
		f.commit("T-002: rewrite", map[string]string{"src/T-001.txt": lines(10, "new")})
		if u := unitFor(t, f.score(late), "T-001"); u.State != StateMatured || *u.AliveRatio != 1000 {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("a pending unit has no ratio yet", func(t *testing.T) {
		f := newFx(t)
		one(f, nil)
		if u := unitFor(t, f.score(t0.Add(day)), "T-001"); u.State != StatePending || u.AliveRatio != nil {
			t.Fatalf("%+v", u)
		}
	})
}

func TestSupersedes(t *testing.T) {
	f := newFx(t)
	sha := one(f, nil)
	f.tick(day)
	f.commit("T-002: rewrite\n\nWaggle-Supersedes: T-001", map[string]string{"src/T-001.txt": lines(10, "new")})
	u := unitFor(t, f.score(late), "T-001")
	if u.State != StateMatured || u.AliveRatio != nil || !slices.Contains(u.Reasons, "superseded") {
		t.Fatalf("exempt from rule 2: %+v", u)
	}
	// Not from rule 1: a revert after the window still claws it back.
	f.tick(40 * day)
	f.commit("Revert\n\nThis reverts commit "+sha+".", map[string]string{"src/T-001.txt": del})
	if u := unitFor(t, f.score(late), "T-001"); u.State != StateClawedBack || u.ClawedBack != u.Weight {
		t.Fatalf("revert: %+v", u)
	}
	// An earlier commit naming the task does not exempt it.
	g := newFx(t)
	g.commit("T-009: early\n\nWaggle-Supersedes: T-001", map[string]string{"e.txt": "e\n"})
	one(g, nil)
	g.tick(day)
	g.commit("T-002: rewrite", map[string]string{"src/T-001.txt": lines(10, "new")})
	if u := unitFor(t, g.score(late), "T-001"); u.State != StateDropped {
		t.Fatalf("earlier supersede: %+v", u)
	}
}

func TestPartialAndManualReverts(t *testing.T) {
	t.Run("a manual revert by patch-id after the window claws back", func(t *testing.T) {
		f := newFx(t)
		one(f, nil)
		f.tick(40 * day)
		f.commit("undo that", map[string]string{"src/T-001.txt": del})
		rep := f.score(late)
		u := unitFor(t, rep, "T-001")
		if u.State != StateClawedBack || u.ClawedBack != w10 {
			t.Fatalf("%+v", u)
		}
		if k := keeper(rep, "alice"); k.Nectar != 0 || k.ClawedBack != w10 {
			t.Fatalf("%+v", k)
		}
		var rv int
		for _, x := range rep.Units {
			if x.Kind == KindRevert {
				rv++
			}
		}
		if rv != 1 {
			t.Fatalf("revert entries: %d", rv)
		}
	})
	t.Run("a manual revert inside the window drops the unit", func(t *testing.T) {
		f := newFx(t)
		one(f, nil)
		f.tick(day)
		f.commit("undo that", map[string]string{"src/T-001.txt": del})
		if u := unitFor(t, f.score(late), "T-001"); u.State != StateDropped {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("a commit that only looks like a revert changes nothing", func(t *testing.T) {
		f := newFx(t)
		one(f, nil)
		f.tick(40 * day)
		f.commit("rewrite", map[string]string{"src/T-001.txt": lines(10, "other")})
		if u := unitFor(t, f.score(late), "T-001"); u.State != StateMatured || u.ClawedBack != 0 {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("a partial revert of 30 percent claws back 0.3 of the weight", func(t *testing.T) {
		f := newFx(t)
		sha := one(f, nil)
		f.tick(40 * day)
		f.commit("Revert part\n\nThis reverts commit "+sha+".", map[string]string{"src/T-001.txt": drop3})
		rep := f.score(late)
		u := unitFor(t, rep, "T-001")
		want := mulMilli(w10, 300)
		if u.State != StateMatured || u.ClawedBack != want || !slices.Contains(u.Reasons, "partial_revert") {
			t.Fatalf("%+v want %d", u, want)
		}
		if k := keeper(rep, "alice"); k.Matured != w10 || k.ClawedBack != want || k.Nectar != w10-want {
			t.Fatalf("%+v", k)
		}
	})
	t.Run("a partial revert inside the window reduces what matures", func(t *testing.T) {
		f := newFx(t)
		sha := one(f, nil)
		f.tick(day)
		f.commit("Revert part\n\nThis reverts commit "+sha+".", map[string]string{"src/T-001.txt": drop3})
		rep := f.score(late)
		if k := keeper(rep, "alice"); k.Nectar != w10-mulMilli(w10, 300) {
			t.Fatalf("%+v", k)
		}
	})
	for _, to := range []string{"src/renamed.txt", "lib/moved/T-001.txt"} {
		t.Run("a trailer revert after a rename or move to "+to+" claws back in full", func(t *testing.T) {
			build := func(gap time.Duration) Unit {
				f := newFx(t)
				sha := one(f, nil)
				f.tick(day)
				f.commit("T-002: move", map[string]string{"src/T-001.txt": del, to: lines(10, "T-001")})
				f.tick(gap)
				f.commit("Revert T-001\n\nThis reverts commit "+sha+".", map[string]string{to: del})
				return unitFor(t, f.score(late), "T-001")
			}
			if u := build(40 * day); u.State != StateClawedBack || u.ClawedBack != w10 {
				t.Fatalf("after the window: %+v", u)
			}
			if u := build(0); u.State != StateDropped {
				t.Fatalf("inside the window: %+v", u)
			}
		})
	}
	t.Run("a rebase run reverted one commit at a time claws back its share by line", func(t *testing.T) {
		f := newFx(t)
		c1 := f.squash("T-003", map[string]string{"src/p.txt": lines(5, "p")})
		c2 := f.commit("T-003: more", map[string]string{"src/q.txt": lines(5, "q")})
		f.addReceipt(f.receipt("T-003", "carol", c2), "")
		f.tick(40 * day)
		f.commit("Revert p\n\nThis reverts commit "+c1+".", map[string]string{"src/p.txt": del})
		u := unitFor(t, f.score(late), "T-003")
		if u.State != StateMatured || u.ClawedBack != mulMilli(u.Weight, 500) {
			t.Fatalf("%+v", u)
		}
		f.commit("Revert q\n\nThis reverts commit "+c2+".", map[string]string{"src/q.txt": del})
		if u := unitFor(t, f.score(late), "T-003"); u.State != StateClawedBack || u.ClawedBack != u.Weight {
			t.Fatalf("both: %+v", u)
		}
	})
}

func TestUnconfirmedRevert(t *testing.T) {
	f := newFx(t)
	one(f, nil)
	before := f.score(late)
	f.commit("Revert \"T-001: work\"", map[string]string{"src/other.txt": lines(3, "o")})
	rep := f.score(late)
	if rep.Counts.UnconfirmedRevert != 1 {
		t.Fatalf("%+v", rep.Counts)
	}
	if a, b := keeper(before, "alice"), keeper(rep, "alice"); *a != *b {
		t.Fatalf("score changed: %+v %+v", a, b)
	}
	var found bool
	for _, u := range rep.Units {
		found = found || slices.Contains(u.Reasons, "unconfirmed_revert")
	}
	if !found {
		t.Fatal("no unit carries unconfirmed_revert")
	}
}

func TestDailyCap(t *testing.T) {
	build := func(t *testing.T) *fx {
		f := newFx(t)
		for i := 1; i <= 20; i++ {
			task := fmt.Sprintf("T-%03d", i)
			f.addReceipt(f.receipt(task, "alice", f.squash(task, nil)), "")
		}
		return f
	}
	end := t0.Add(30*day + 21*time.Hour) // Jan 31, after the last of the 20 matures
	t.Run("twelve mature, the rest wait", func(t *testing.T) {
		f := build(t)
		rep := f.score(end)
		k := keeper(rep, "alice")
		if k.Matured != 12000 || k.Pending != 20*w10-12000 || k.Nectar != 12000 {
			t.Fatalf("%+v", k)
		}
		if rep.Counts.Carried != 12 {
			t.Fatalf("carried units: %d", rep.Counts.Carried)
		}
		first, ninth, tenth := unitFor(t, rep, "T-001"), unitFor(t, rep, "T-009"), unitFor(t, rep, "T-010")
		if first.Carried != 0 || first.State != StateMatured || ninth.Carried != w10-(12000-8*w10) ||
			ninth.State != StateMatured || tenth.Carried != w10 || tenth.State != StatePending {
			t.Fatalf("%+v %+v %+v", first, ninth, tenth)
		}
	})
	t.Run("the rest matures on the next days with room and the total is unchanged", func(t *testing.T) {
		f := build(t)
		if k := keeper(f.score(end.Add(day)), "alice"); k.Matured != 24000 || k.Pending != 20*w10-24000 {
			t.Fatalf("next day: %+v", k)
		}
		if k := keeper(f.score(end.Add(2*day)), "alice"); k.Matured != 20*w10 || k.Pending != 0 || k.Nectar != 20*w10 {
			t.Fatalf("two days: %+v", k)
		}
		if k := keeper(f.score(end.Add(60*day)), "alice"); k.Nectar != 20*w10 {
			t.Fatalf("late: %+v", k)
		}
	})
	t.Run("nothing carries onto a day after now", func(t *testing.T) {
		f := build(t)
		// Day 1 and day 2 are processed up to now's day and no further.
		if k := keeper(f.score(end.Add(day)), "alice"); k.Matured > 24000 {
			t.Fatalf("%+v", k)
		}
	})
	t.Run("a clawback of a partly carried unit removes the carried part from pending first", func(t *testing.T) {
		f := build(t)
		f.tick(30*day - 8*time.Hour)
		f.commit("Revert nine\n\nThis reverts commit "+f.git("log", "--format=%H", "--grep=T-009: work")+".", map[string]string{"src/T-009.txt": del})
		rep := f.score(end)
		k := keeper(rep, "alice")
		carried := w10 - (12000 - 8*w10) // unit 9 straddles the cap
		if k.Matured != 12000 || k.ClawedBack != w10-carried || k.Nectar != 12000-(w10-carried) ||
			k.Pending != 20*w10-12000-carried {
			t.Fatalf("%+v", k)
		}
	})
}

func TestSelfMerge(t *testing.T) {
	cfg := func(approvers string) map[string]string {
		return map[string]string{".waggle/control/config.yaml": "version: 1\ngate:\n  approvers: " + approvers + "\n"}
	}
	score := func(t *testing.T, approvers string, mut ...func(*comb.Receipt)) (Report, Unit) {
		f := newFx(t)
		f.commit("base", cfg(approvers))
		sha := f.squash("T-001", nil)
		f.addReceipt(f.receipt("T-001", "alice", sha, mut...), "")
		rep := f.score(late)
		return rep, unitFor(t, rep, "T-001")
	}
	half := mulMilli(w10, 500)
	none := func(r *comb.Receipt) { r.Approvers = []string{} }
	self := func(r *comb.Receipt) { r.Approvers = []string{"alice"} }
	t.Run("no approval in a multi-Keeper repo halves the weight", func(t *testing.T) {
		rep, u := score(t, "maint, bob", none)
		if u.Weight != half || u.Factors.Review != 500 || !u.SelfMerge || rep.Counts.SelfMerge != 1 ||
			!slices.Contains(u.Reasons, "self_merge_multi_keeper") {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("approval only by the keeper halves it", func(t *testing.T) {
		if _, u := score(t, "maint, bob", self); u.Weight != half {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("an approval by another login gives full weight", func(t *testing.T) {
		if rep, u := score(t, "maint, bob"); u.Weight != w10 || u.SelfMerge || rep.Counts.SelfMerge != 0 {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("a solo repo counts a self-merge at full weight", func(t *testing.T) {
		if _, u := score(t, "alice", none); u.Weight != w10 || u.SelfMerge {
			t.Fatalf("%+v", u)
		}
		f := newFx(t)
		sha := f.squash("T-001", nil)
		f.addReceipt(f.receipt("T-001", "alice", sha, none), "")
		if u := unitFor(t, f.score(late), "T-001"); u.Weight != w10 {
			t.Fatalf("no config: %+v", u)
		}
	})
	t.Run("an incomplete receipt keeps full weight and is counted", func(t *testing.T) {
		rep, u := score(t, "maint, bob", none, func(r *comb.Receipt) { r.Incomplete = true })
		if u.Weight != w10 || u.SelfMerge || rep.Counts.Incomplete != 1 {
			t.Fatalf("%+v", u)
		}
	})
	t.Run("the config is read at the unit's base", func(t *testing.T) {
		f := newFx(t)
		f.commit("base", cfg("alice"))
		sha := f.squash("T-001", nil)
		f.addReceipt(f.receipt("T-001", "alice", sha, none), "")
		f.commit("later", cfg("maint, bob"))
		if u := unitFor(t, f.score(late), "T-001"); u.Weight != w10 {
			t.Fatalf("%+v", u)
		}
	})
}

func TestReciprocity(t *testing.T) {
	// Owners alice and bob, five units in all. approvedBy lists who approved each.
	build := func(t *testing.T, approvedBy map[string]string) Report {
		f := newFx(t)
		f.commit("base", map[string]string{".waggle/control/config.yaml": "version: 1\ngate:\n  approvers: alice, bob, maint\n"})
		for _, row := range []struct{ task, owner string }{
			{"T-001", "alice"}, {"T-002", "alice"}, {"T-003", "alice"}, {"T-004", "bob"}, {"T-005", "bob"}} {
			sha := f.squash(row.task, nil)
			who := approvedBy[row.task]
			f.addReceipt(f.receipt(row.task, row.owner, sha, func(r *comb.Receipt) { r.Approvers = []string{who} }), "")
		}
		return f.score(late)
	}
	t.Run("over 60 percent halves every unit of the pair", func(t *testing.T) {
		rep := build(t, map[string]string{"T-001": "bob", "T-002": "bob", "T-003": "bob", "T-004": "alice", "T-005": "maint"})
		if len(rep.Counts.ReciprocalPairs) != 1 || rep.Counts.ReciprocalPairs[0] != (PairShare{"alice", "bob", 800}) {
			t.Fatalf("%+v", rep.Counts.ReciprocalPairs)
		}
		for _, task := range []string{"T-001", "T-004", "T-005"} {
			if u := unitFor(t, rep, task); u.Weight != mulMilli(w10, 500) || !slices.Contains(u.Reasons, "reciprocal_pair") {
				t.Fatalf("%s %+v", task, u)
			}
		}
	})
	t.Run("exactly 60 percent keeps full weight and lists the pair", func(t *testing.T) {
		rep := build(t, map[string]string{"T-001": "bob", "T-002": "bob", "T-003": "maint", "T-004": "alice", "T-005": "maint"})
		if len(rep.Counts.ReciprocalPairs) != 1 || rep.Counts.ReciprocalPairs[0].Share != 600 {
			t.Fatalf("%+v", rep.Counts.ReciprocalPairs)
		}
		for _, u := range rep.Units {
			if u.KeeperID != nil && u.Weight != w10 {
				t.Fatalf("%+v", u)
			}
		}
	})
}

func TestMinChangedLines(t *testing.T) {
	small := map[string]string{"src/T-001.txt": lines(3, "s")}
	score := func(t *testing.T, min int64) Unit {
		old := rulesV1.minChangedLines
		rulesV1.minChangedLines = min
		t.Cleanup(func() { rulesV1.minChangedLines = old })
		f := newFx(t)
		one(f, small)
		return unitFor(t, f.score(late), "T-001")
	}
	if u := score(t, 5); u.Weight != 1000 || u.Factors.Size != 1000 || !slices.Contains(u.Reasons, "min_changed_lines") {
		t.Fatalf("set: %+v", u)
	}
	want := rulesV1.sizeFactor(3000)
	if u := score(t, 0); u.Weight != want || want <= 1000 || slices.Contains(u.Reasons, "min_changed_lines") {
		t.Fatalf("default: %+v want %d", u, want)
	}
	if u := score(t, 3); u.Weight != want {
		t.Fatalf("at the minimum: %+v", u)
	}
}

func TestPureDeletion(t *testing.T) {
	f := newFx(t)
	f.commit("base", map[string]string{"old.txt": lines(50, "o")})
	sha := f.squash("T-001", map[string]string{"old.txt": del})
	f.addReceipt(f.receipt("T-001", "alice", sha), "")
	u := unitFor(t, f.score(late), "T-001")
	if u.State != StateMatured || u.Weight != 1000 || u.AliveRatio != nil {
		t.Fatalf("%+v", u)
	}
}

func TestCache(t *testing.T) {
	build := func(t *testing.T) *fx {
		f := newFx(t)
		one(f, nil)
		f.squash("T-002", map[string]string{"src/b.txt": lines(12, "b")})
		f.addReceipt(f.receipt("T-002", "bob", f.head()), "")
		f.tick(day)
		f.commit("T-003: rewrite", map[string]string{"src/b.txt": mixed(8, 12, "b")})
		return f
	}
	path := func(f *fx) string { return filepath.Join(f.dir, ".git", "waggle", "nectar", "1.json") }
	score := func(t *testing.T, f *fx, mut func(*Input)) string {
		in := f.in(late)
		if mut != nil {
			mut(&in)
		}
		rep, err := Score(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		return detail(t, rep)
	}
	f := build(t)
	cold := score(t, f, nil)
	if _, err := os.Stat(path(f)); err != nil {
		t.Fatalf("no cache file after a run: %v", err)
	}
	cases := map[string]func(){
		"warm":    func() {},
		"deleted": func() { _ = os.Remove(path(f)) },
		"corrupt": func() { _ = os.WriteFile(path(f), []byte("{not json"), 0o644) },
		"other rules version": func() {
			_ = os.WriteFile(path(f), []byte(`{"rules_version":2,"main_sha":"`+f.head()+`","units":{"x":{}}}`), 0o644)
		},
		"a main the history does not contain": func() {
			_ = os.WriteFile(path(f), []byte(`{"rules_version":1,"main_sha":"`+strings.Repeat("ab", 20)+`","units":{}}`), 0o644)
		},
		"poisoned entries": func() {
			b, _ := os.ReadFile(path(f))
			var c cacheFile
			_ = json.Unmarshal(b, &c)
			for k, u := range c.Units {
				u.SurvivalAt = "someothercommit"
				u.Alive = 0
				c.Units[k] = u
			}
			b, _ = json.Marshal(c)
			_ = os.WriteFile(path(f), b, 0o644)
		},
	}
	for name, prep := range cases {
		t.Run(name, func(t *testing.T) {
			prep()
			if got := score(t, f, nil); got != cold {
				t.Fatalf("differs from the cold run")
			}
			if b, err := os.ReadFile(path(f)); err != nil || !json.Valid(b) {
				t.Fatalf("cache not rewritten: %v", err)
			}
		})
	}
	t.Run("recompute", func(t *testing.T) {
		if got := score(t, f, func(in *Input) { in.Recompute = true }); got != cold {
			t.Fatal("differs from the cold run")
		}
	})
	t.Run("the cache holds nothing read from receipts", func(t *testing.T) {
		b, _ := os.ReadFile(path(f))
		for _, banned := range []string{"alice", "bob", "keeper", "approvers", "first_seen"} {
			if strings.Contains(string(b), banned) {
				t.Fatalf("cache holds %q", banned)
			}
		}
	})
	t.Run("a pinned older replay matches a cold run and leaves the cache alone", func(t *testing.T) {
		g := build(t)
		m1, r1 := g.git("rev-parse", "main"), g.rc
		pinned := func(in *Input) { in.Main, in.Receipts = m1, r1 }
		cold1 := score(t, g, pinned)
		_ = os.Remove(path(g))
		g.tick(day)
		g.squash("T-004", map[string]string{"src/d.txt": lines(10, "d")})
		g.addReceipt(g.receipt("T-004", "carol", g.head()), "")
		g.addReceipt(g.receipt("T-001", "dave", g.git("log", "--format=%H", "--grep=T-001: work")), "")
		score(t, g, nil)
		before, err := os.ReadFile(path(g))
		if err != nil {
			t.Fatal(err)
		}
		if got := score(t, g, pinned); got != cold1 {
			t.Fatal("pinned replay differs from a cold run at the same OIDs")
		}
		if after, _ := os.ReadFile(path(g)); string(after) != string(before) {
			t.Fatal("the pinned replay moved the cache")
		}
	})
	t.Run("worktrees share the cache in the common dir", func(t *testing.T) {
		g := build(t)
		wt := filepath.Join(t.TempDir(), "wt")
		g.git("worktree", "add", "-q", "--detach", wt)
		in := g.in(late)
		in.Dir = wt
		if _, err := Score(context.Background(), in); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path(g)); err != nil {
			t.Fatalf("cache not in the common dir: %v", err)
		}
	})
}
