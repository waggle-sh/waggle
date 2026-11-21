// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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
		"weight: 10.000 = tier 2 x size 10.000 x review 0.500",
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
}

// Every reason code the scorer writes has a plain sentence.
func TestNectarReasonTable(t *testing.T) {
	for _, code := range []string{"no_binding", "unconfirmed_revert", "min_changed_lines", "receipt_unmerged", "no_valid_receipt", "third_party",
		"fork_mismatch", "receipt_incomplete", "self_merge_multi_keeper", "direct_push", "restored", "revert_before_maturity",
		"revert_after_maturity", "partial_revert", "superseded", "alive_below_ratio", "reciprocal_pair", "daily_cap_carry",
		"orphaned_rewritten", "orphaned_missing"} {
		s, ok := reasonText[code]
		if !ok || s == "" {
			t.Errorf("no sentence for %s", code)
		}
		checkPlain(t, code, s)
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
	late := time.Now().UTC().Add(60 * 24 * time.Hour).Format(time.RFC3339)
	out, _, exit = exp("23", "--now", late)
	if exit != 0 || !strings.Contains(out, "state: matured\n") {
		t.Errorf("matured: exit %d\n%s", exit, out)
	}

	// B4: --json is Report.Detail narrowed to the task, with the counts whole.
	j, errb, exit := exp("T-023", "--now", late, "--json")
	if exit != 0 || errb != "" || !strings.HasSuffix(j, "}\n") || strings.HasSuffix(j, "\n\n") {
		t.Fatalf("json: exit %d %q %q", exit, errb, j)
	}
	var d struct {
		DetailVersion int `json:"detail_version"`
		Counts        struct {
			Unbound int `json:"unbound"`
		} `json:"counts"`
		Units []struct {
			Task string `json:"task"`
		} `json:"units"`
	}
	if err := json.Unmarshal([]byte(j), &d); err != nil || d.DetailVersion != 1 || d.Counts.Unbound == 0 || len(d.Units) != 1 || d.Units[0].Task != "T-023" {
		t.Errorf("detail %+v: %v\n%s", d, err, j)
	}
	j2, _, exit := exp("T-099", "--now", late, "--json")
	if exit != 0 || !strings.Contains(j2, `"units": []`) || !strings.Contains(j2, `"unbound"`) {
		t.Errorf("no unit json: exit %d\n%s", exit, j2)
	}

	// B5: no unit; a malformed id and extra arguments are usage.
	out, _, exit = exp("T-099", "--now", late)
	if exit != 0 || out != "no merged unit for T-099\n" {
		t.Errorf("no unit: exit %d %q", exit, out)
	}
	for _, args := range [][]string{{"nope"}, {"T-1x"}, {}, {"T-023", "T-024"}, {"T-023", "--bogus"}, {"T-023", "--now", "soon"}, {"T-023", "--main", "xyz"}} {
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

	// Unattributed exits 1 for this task only.
	git(t, repo, "update-ref", "-d", "refs/waggle/receipts/main")
	out, _, exit = exp("T-023", "--now", nectarNow)
	if exit != 1 || !strings.Contains(out, "state: unattributed\n") || !strings.Contains(out, "reason: no_valid_receipt: ") {
		t.Errorf("unattributed: exit %d\n%s", exit, out)
	}
	if _, _, exit := exp("T-099", "--now", nectarNow); exit != 0 {
		t.Errorf("another task's explain exits %d, want 0", exit)
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

// B7: docs/nectar.md names every flag and exit code of both commands.
func TestNectarDocNamesFlags(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "nectar.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	for _, usage := range []string{summaryUsage, explainUsage} {
		for _, f := range regexp.MustCompile(`--[a-z-]+`).FindAllString(usage, -1) {
			if !strings.Contains(doc, "`"+f) {
				t.Errorf("doc lacks flag %s", f)
			}
		}
	}
	for _, code := range []string{"`0`", "`1`", "`2`", "`3`"} {
		if strings.Count(doc, code) < 2 {
			t.Errorf("doc names exit code %s fewer than twice", code)
		}
	}
	if strings.ContainsAny(doc, dashes) {
		t.Error("doc holds an en or em dash")
	}
}
