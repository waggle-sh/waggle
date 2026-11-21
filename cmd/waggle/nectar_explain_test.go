// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/nectar"
)

func sp(s string) *string { return &s }

func explainText(units ...nectar.Unit) string {
	var b bytes.Buffer
	renderExplain(&b, "T-001", units)
	return b.String()
}

func wantLines(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains("\n"+got+"\n", "\n"+w+"\n") {
			t.Errorf("missing line %q in:\n%s", w, got)
		}
	}
}

// B2, B3: built reports render every flag, the receipt, pr_author beside
// keeper_id, and unknown reason codes bare.
func TestNectarExplainRender(t *testing.T) {
	pr := 12
	alive := nectar.Milli(250)
	u := nectar.Unit{Task: "T-001", Kind: nectar.KindUnit, Commits: []string{"aaa", "bbb"}, MergedSHA: "bbb", BaseSHA: sp("ccc"),
		MergedAt: "2026-01-02T00:00:00Z", MaturesAt: sp("2026-02-01T00:00:00Z"), State: nectar.StateDropped,
		KeeperID: sp("alice"), CoKeepers: []nectar.Share{{KeeperID: "alice", Share: 6000}, {KeeperID: "bob", Share: 4000}},
		Weight: 10000, Factors: nectar.Factors{Tier: 2, Size: 10000, Review: 500, ChangedLines: 120000, CodeLines: 100, DocsLines: 40, ExcludedFiles: []string{"go.sum"}},
		AliveRatio: &alive, Carried: 1500,
		Receipt: &nectar.ReceiptInfo{OID: "rcp", Source: "release", Status: "merged", MergeProof: "forge", ReleasedBy: "mal\x1b[2Jlory",
			PRNumber: &pr, PRAuthor: sp("carol"), HeadBranch: sp("t-001"), Approvers: []string{"dan"}},
		ThirdParty: true, Maintainer: true, Relay: true, ForkMismatch: true, DirectPush: true, SelfMerge: true,
		Reasons: []string{"alive_below_ratio", "self_merge_multi_keeper", "reciprocal_pair", "brand_new_code"}}
	got := explainText(u)
	wantLines(t, got,
		"state: dropped",
		"keeper_id: alice, pr_author carol",
		"weight: 10.000, tier 2, size 10.000, review 0.500",
		"alive_ratio: 0.250",
		"carried: 1.500",
		"co_keepers: alice 6.000",
		"co_keepers: bob 4.000",
		"excluded_files: go.sum",
		"flag: third_party, released by a maintainer",
		"flag: relay", "flag: fork_mismatch", "flag: direct_push", "flag: self_merge",
		"reason: brand_new_code",
		"first_seen: none",
		"receipt: oid rcp, source release, status merged, merge_proof forge, incomplete false, released_by mal[2Jlory, pr_number 12, head_branch t-001, approvers dan",
	)
	if !strings.Contains(got, "reason: alive_below_ratio: ") || !strings.Contains(got, "reason: reciprocal_pair: ") {
		t.Errorf("known codes lack a sentence:\n%s", got)
	}
	if strings.Contains(got, "\x1b") {
		t.Error("control character printed")
	}
	// An incomplete record has no pr_author.
	u2 := nectar.Unit{Task: "T-001", Kind: nectar.KindUnit, State: nectar.StatePending, KeeperID: sp("alice"), MergedSHA: "x",
		Receipt: &nectar.ReceiptInfo{OID: "r", Incomplete: true}, DirectPush: true, FirstSeen: sp("2026-01-03T00:00:00Z")}
	got = explainText(u2)
	wantLines(t, got, "keeper_id: alice, pr_author none", "first_seen: 2026-01-03T00:00:00Z", "flag: direct_push", "reasons: none")
	if !strings.Contains(got, "incomplete true") {
		t.Errorf("incomplete not shown:\n%s", got)
	}
	// Orphaned states and a revert entry.
	o := nectar.Unit{Task: "T-001", Kind: nectar.KindUnit, State: nectar.StateOrphaned, Rewritten: true, MergedSHA: "x", Reasons: []string{"orphaned_rewritten"}}
	wantLines(t, explainText(o), "state: orphaned (rewritten)", "keeper_id: none, pr_author none")
	o.Rewritten, o.Missing = false, true
	wantLines(t, explainText(o), "state: orphaned (missing)")
	rv := nectar.Unit{Task: "T-001", Kind: nectar.KindRevert, MergedSHA: "rrr", MergedAt: "2026-03-01T00:00:00Z"}
	wantLines(t, explainText(u2, rv), "revert: commit rrr, merged_at 2026-03-01T00:00:00Z, clawed_back 0.000")
	if got := explainText(); got != "no merged unit for T-001\n" {
		t.Errorf("empty: %q", got)
	}
	// A task whose only entries are reverts still lists them.
	if got, want := explainText(rv), "no merged unit for T-001\nrevert: commit rrr, merged_at 2026-03-01T00:00:00Z, clawed_back 0.000\n"; got != want {
		t.Errorf("reverts only: %q", got)
	}
}

// B1, B4, B5: through the real scorer.
func TestNectarExplainScored(t *testing.T) {
	repo, _ := scoredRepo(t)
	if _, _, exit := nectarCLI(t, "--repo", repo, "--now", nectarNow); exit != 0 {
		t.Fatalf("first run exit %d", exit)
	}
	exp := func(extra ...string) (string, string, int) {
		return nectarCLI(t, append([]string{"explain", "--repo", repo, "--no-fetch"}, extra...)...)
	}

	// Pending, then matured.
	out, errb, exit := exp("T-023", "--now", nectarNow)
	if exit != 0 || errb != "" {
		t.Fatalf("pending: exit %d %q\n%s", exit, errb, out)
	}
	wantLines(t, out, "task: T-023", "kind: unit", "state: pending", "keeper_id: bob, pr_author none")
	// B2: the gate's record shows its source.
	if !strings.Contains(out, "\nreceipt: oid ") || !strings.Contains(out, ", source gate_run, ") {
		t.Errorf("gate_run source not shown:\n%s", out)
	}
	lateT := time.Now().UTC().Add(60 * 24 * time.Hour).Truncate(time.Second)
	late := lateT.Format(time.RFC3339)
	out, _, exit = exp("23", "--now", late)
	if exit != 0 || !strings.Contains(out, "state: matured\n") {
		t.Errorf("matured: exit %d\n%s", exit, out)
	}

	// B4: --json is Report.Detail narrowed to the task plus one newline, with
	// the counts whole: score the same inputs once and compare the bytes.
	run, code := nectarScore(context.Background(), repo, "", "", lateT, true, false, io.Discard)
	if code != 0 {
		t.Fatalf("nectarScore exit %d", code)
	}
	detail := func(task string) string {
		t.Helper()
		narrowed := run.Report
		narrowed.Units = []nectar.Unit{}
		for _, u := range run.Report.Units {
			if u.Task == task {
				narrowed.Units = append(narrowed.Units, u)
			}
		}
		data, err := narrowed.Detail()
		if err != nil {
			t.Fatal(err)
		}
		return string(data) + "\n"
	}
	for _, task := range []string{"T-023", "T-099"} {
		j, errb, exit := exp(task, "--now", late, "--json")
		if want := detail(task); exit != 0 || errb != "" || j != want {
			t.Errorf("%s json: exit %d %q\ngot  %s\nwant %s", task, exit, errb, j, want)
		}
	}
	if run.Report.Counts.Unbound == 0 {
		t.Error("the counts should be whole: the fixture has unbound commits")
	}
	// The keeper's share is shown and equals the weight with no co-keepers.
	for _, u := range run.Report.Units {
		if u.Task == "T-023" && u.Kind == nectar.KindUnit {
			out, _, _ := exp("T-023", "--now", late)
			wantLines(t, out, "keeper_share: "+milli(u.Weight), "co_keepers: none")
		}
	}

	// B5: no unit; a malformed id and extra arguments are usage.
	out, _, exit = exp("T-099", "--now", late)
	if exit != 0 || out != "no merged unit for T-099\n" {
		t.Errorf("no unit: exit %d %q", exit, out)
	}
	for _, args := range [][]string{{"nope"}, {"T-1x"}, {}, {"T-023", "T-024"}, {"T-023", "--bogus"}, {"T-023", "--now", "soon"}, {"T-023", "--main", "xyz"}, {"--", "--now"}, {"--", "T-023", "T-024"}} {
		out, _, exit := exp(args...)
		if exit != 2 || out != "" {
			t.Errorf("%v: exit %d stdout %q", args, exit, out)
		}
	}
	if out, _, exit := nectarCLI(t, "explain", "T-023", "--repo", t.TempDir(), "--json"); exit != 3 || out != "" {
		t.Errorf("not a repo: exit %d %q", exit, out)
	}

	// Orphaned exits 1.
	parent := gitStdout(t, repo, "rev-parse", "refs/remotes/origin/main^")
	out, _, exit = exp("T-023", "--now", nectarNow, "--main", parent)
	if exit != 1 {
		t.Errorf("orphaned: exit %d\n%s", exit, out)
	}
	wantLines(t, out, "state: orphaned (rewritten)")
	if !strings.Contains(out, "reason: orphaned_rewritten: ") {
		t.Errorf("rewritten reason:\n%s", out)
	}
	// With the base gone from main too, the unit is missing.
	grand := gitStdout(t, repo, "rev-parse", parent+"^")
	out, _, exit = exp("T-023", "--now", nectarNow, "--main", grand)
	if exit != 1 || !strings.Contains(out, "state: orphaned (missing)\n") || !strings.Contains(out, "reason: orphaned_missing: ") {
		t.Errorf("missing: exit %d\n%s", exit, out)
	}

	// Unattributed exits 1 for this task only.
	git(t, repo, "update-ref", "-d", "refs/waggle/receipts/main")
	out, _, exit = exp("T-023", "--now", nectarNow)
	if exit != 1 || !strings.Contains(out, "state: unattributed\n") || !strings.Contains(out, "reason: no_valid_receipt: ") {
		t.Errorf("unattributed: exit %d\n%s", exit, out)
	}
}

// B5: a matured task exits 0 while another task is unattributed, and a task
// named only by unbound commits has no unit.
func TestNectarExplainOtherFindings(t *testing.T) {
	repo, _ := scoredRepo(t)
	commitGateFile(t, repo, "internal/z/c.go", "T-025: no claim")
	commitGateFile(t, repo, "internal/w/u.go", "tidy: notes for T-099")
	git(t, repo, "push", "-q", "origin", "main")
	late := time.Now().UTC().Add(60 * 24 * time.Hour).Format(time.RFC3339)
	exp := func(task string) (string, string, int) {
		return nectarCLI(t, "explain", task, "--repo", repo, "--now", late)
	}
	if out, _, exit := nectarCLI(t, "--repo", repo, "--now", late); exit != 1 || !strings.Contains(out, "unattributed 1") {
		t.Fatalf("summary: exit %d\n%s", exit, out)
	}
	out, errb, exit := exp("T-023")
	if exit != 0 || errb != "" || !strings.Contains(out, "state: matured\n") {
		t.Errorf("matured beside an unattributed task: exit %d %q\n%s", exit, errb, out)
	}
	if out, _, exit := exp("T-025"); exit != 1 || !strings.Contains(out, "state: unattributed\n") {
		t.Errorf("T-025: exit %d\n%s", exit, out)
	}
	if out, _, exit := exp("T-099"); exit != 0 || out != "no merged unit for T-099\n" {
		t.Errorf("unbound only: exit %d %q", exit, out)
	}
}

// B1: a revert that is itself reverted re-lands the task; the window restarts
// at the re-landing and both revert entries list under the unit.
func TestNectarExplainRelanded(t *testing.T) {
	repo, _ := scoredRepo(t)
	sha := gitStdout(t, repo, "rev-parse", "HEAD")
	start := time.Now().UTC()
	revert := func(target, subject string, days int) string {
		when := start.Add(time.Duration(days) * 24 * time.Hour).Format(time.RFC3339)
		t.Setenv("GIT_AUTHOR_DATE", when)
		t.Setenv("GIT_COMMITTER_DATE", when)
		git(t, repo, "revert", "--no-commit", target)
		git(t, repo, "commit", "-q", "-m", "Revert \""+subject+"\"\n\nThis reverts commit "+target+".\n\nWaggle-Ticket: T-023")
		return gitStdout(t, repo, "rev-parse", "HEAD")
	}
	r1 := revert(sha, "T-023: squash", 5)
	r2 := revert(r1, "Revert T-023", 10)
	git(t, repo, "push", "-q", "origin", "main")
	relanded, err := time.Parse(time.RFC3339, gitStdout(t, repo, "log", "-1", "--format=%cI", r2))
	if err != nil {
		t.Fatal(err)
	}
	at := func(d time.Duration) string { return relanded.Add(d).UTC().Format(time.RFC3339) }
	out, errb, exit := nectarCLI(t, "explain", "T-023", "--repo", repo, "--now", at(29*24*time.Hour))
	if exit != 0 || errb != "" {
		t.Fatalf("exit %d %q\n%s", exit, errb, out)
	}
	wantLines(t, out, "state: pending", "merged_at: "+at(0), "matures_at: "+at(30*24*time.Hour), "clawed_back: 0.000")
	if !strings.Contains(out, "reason: restored: ") || !strings.Contains(out, "\nrevert: commit "+r1+",") || !strings.Contains(out, "\nrevert: commit "+r2+",") {
		t.Errorf("re-landing not explained:\n%s", out)
	}
	out, _, _ = nectarCLI(t, "explain", "T-023", "--repo", repo, "--now", at(31*24*time.Hour))
	wantLines(t, out, "state: matured")
}

// B2: a backfilled record shows its source.
func TestNectarExplainBackfill(t *testing.T) {
	repo, _ := scoredRepo(t)
	if _, _, exit := nectarCLI(t, "--repo", repo, "--now", nectarNow); exit != 0 {
		t.Fatalf("first run exit %d", exit)
	}
	commitGateFile(t, repo, "internal/y/b.go", "T-024: land")
	git(t, repo, "push", "-q", "origin", "main")
	if out, errb, exit := nectarCLI(t, "backfill", "--apply", "--repo", repo); exit != 0 {
		t.Fatalf("backfill: exit %d %q %q", exit, out, errb)
	}
	out, errb, exit := nectarCLI(t, "explain", "T-024", "--repo", repo, "--now", nectarNow, "--no-fetch")
	if exit != 0 || errb != "" || !strings.Contains(out, ", source backfill, ") || !strings.Contains(out, "keeper_id: bob, ") {
		t.Errorf("backfill: exit %d %q\n%s", exit, errb, out)
	}
}

// A rewound remote chain exits 1 and says why on the last stdout line.
func TestNectarExplainRewound(t *testing.T) {
	repo, origin := scoredRepo(t)
	if _, _, exit := nectarCLI(t, "--repo", repo, "--now", nectarNow); exit != 0 {
		t.Fatalf("first run exit %d", exit)
	}
	other := gitStdout(t, origin, "commit-tree", "-m", "rewound", "refs/waggle/receipts/main^{tree}")
	git(t, origin, "update-ref", "refs/waggle/receipts/main", other)
	out, errb, exit := nectarCLI(t, "explain", "T-023", "--repo", repo, "--now", nectarNow)
	if exit != 1 || !strings.Contains(errb, "rewound") || !strings.Contains(out, "state: pending\n") ||
		!strings.HasSuffix(out, "\nreceipts rewound: the remote release record chain does not hold this clone's records, so the local records were scored\n") {
		t.Errorf("exit %d stderr %q\n%s", exit, errb, out)
	}
	checkPlain(t, "rewound line", out)
	if j, _, exit := nectarCLI(t, "explain", "T-023", "--repo", repo, "--now", nectarNow, "--json"); exit != 1 || strings.Contains(j, "rewound") {
		t.Errorf("--json: exit %d %q", exit, j)
	}
}

// B1: a revert after the window claws the unit back, and the revert entry lists
// under it.
func TestNectarExplainRevert(t *testing.T) {
	repo, _ := scoredRepo(t)
	sha := gitStdout(t, repo, "rev-parse", "HEAD")
	when := time.Now().UTC().Add(400 * 24 * time.Hour).Format(time.RFC3339)
	t.Setenv("GIT_AUTHOR_DATE", when)
	t.Setenv("GIT_COMMITTER_DATE", when)
	if err := os.Remove(filepath.Join(repo, "internal/x/s.go")); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "Revert \"T-023: squash\"\n\nThis reverts commit "+sha+".\n\nWaggle-Ticket: T-023")
	git(t, repo, "push", "-q", "origin", "main")
	now := time.Now().UTC().Add(401 * 24 * time.Hour).Format(time.RFC3339)
	out, errb, exit := nectarCLI(t, "explain", "T-023", "--repo", repo, "--now", now)
	if exit != 0 || errb != "" {
		t.Fatalf("exit %d %q\n%s", exit, errb, out)
	}
	wantLines(t, out, "state: clawed_back")
	if !strings.Contains(out, "reason: revert_after_maturity: ") || !strings.Contains(out, "\nrevert: commit ") {
		t.Errorf("revert not explained:\n%s", out)
	}
}

// B6: explain -h is plain, the main.go line names explain, backfill -h is unchanged.
func TestNectarExplainHelp(t *testing.T) {
	_, errb, exit := nectarCLI(t, "explain", "-h")
	if exit != 0 {
		t.Errorf("-h exit %d", exit)
	}
	checkPlain(t, "nectar explain -h", errb, "reputation score", "--no-fetch", "--receipts")
	_, errb, _ = nectarCLI(t, "-h")
	checkPlain(t, "nectar -h", errb, "waggle nectar explain <task>")
	_, errb, _ = nectarCLI(t, "backfill", "-h")
	if !strings.HasPrefix(errb, backfillUsage+"\nAdds a release record for each task") {
		t.Errorf("backfill -h changed:\n%s", errb)
	}
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "nectar ") {
			found = true
			checkPlain(t, "main.go nectar line", line, "Reputation score", "explain <task>")
		}
	}
	if !found {
		t.Error("no nectar line in main.go")
	}
}

// B2: a release record with co-keepers splits the weight, and the keeper's
// share and the co-keeper shares sum to it.
func TestNectarExplainCoKeepers(t *testing.T) {
	repo, _ := scoredRepo(t)
	git(t, repo, "checkout", "-q", "-b", "t-024")
	commitGateFile(t, repo, "internal/y/b.go", "T-024: land")
	git(t, repo, "checkout", "-q", "main")
	git(t, repo, "merge", "-q", "--no-ff", "-m", "T-024: merge", "t-024")
	git(t, repo, "push", "-q", "origin", "main", "t-024")
	if res, exit := runJSON(t, releaseCLI, "24", "--owner", "bob", "--co", "alice", "--co", "carol", "--repo", repo); exit != 0 {
		t.Fatalf("release: exit %d %+v", exit, res)
	}
	now := time.Now().UTC().Truncate(time.Second)
	run, code := nectarScore(context.Background(), repo, "", "", now, true, false, io.Discard)
	if code != 0 {
		t.Fatalf("nectarScore exit %d", code)
	}
	var u *nectar.Unit
	for i := range run.Report.Units {
		if run.Report.Units[i].Task == "T-024" && run.Report.Units[i].Kind == nectar.KindUnit {
			u = &run.Report.Units[i]
		}
	}
	if u == nil || len(u.CoKeepers) != 2 {
		t.Fatalf("unit %+v", u)
	}
	sum := u.KeeperShare
	for _, c := range u.CoKeepers {
		sum += c.Share
	}
	if sum != u.Weight || u.Weight == 0 {
		t.Errorf("shares %d + %+v sum to %d, weight %d", u.KeeperShare, u.CoKeepers, sum, u.Weight)
	}
	out, errb, exit := nectarCLI(t, "explain", "T-024", "--repo", repo, "--no-fetch", "--now", now.Format(time.RFC3339))
	if exit != 0 || errb != "" {
		t.Fatalf("exit %d %q\n%s", exit, errb, out)
	}
	wantLines(t, out, "keeper_id: bob, pr_author none", "keeper_share: "+milli(u.KeeperShare),
		"co_keepers: "+u.CoKeepers[0].KeeperID+" "+milli(u.CoKeepers[0].Share),
		"co_keepers: "+u.CoKeepers[1].KeeperID+" "+milli(u.CoKeepers[1].Share))
	if !strings.Contains(out, ", source release, ") {
		t.Errorf("release source not shown:\n%s", out)
	}
}
