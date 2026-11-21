// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/nectar"
)

const nectarNow = "2026-11-20T12:00:00Z"

func nectarCLI(t *testing.T, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	var out, errb bytes.Buffer
	exit = runNectar(args, &out, &errb)
	return out.String(), errb.String(), exit
}

// scoredRepo is a hive with a bare origin, T-023 merged to main and pushed, and
// its gate-run receipt on origin only: this clone holds no receipt ref yet.
func scoredRepo(t *testing.T) (repo, origin string) {
	t.Helper()
	t.Setenv("GITHUB_TOKEN", "")
	repo, origin, before := gateReceiptRepo(t)
	after := commitGateFile(t, repo, "internal/x/s.go", "T-023: squash")
	git(t, repo, "push", "-q", "origin", "main")
	if res, exit, stderr := runGateReceipt(t, repo, before, after); exit != 0 || res.Written != 1 {
		t.Fatalf("gate-receipt: exit %d %+v %q", exit, res, stderr)
	}
	return repo, origin
}

func refOrNone(t *testing.T, repo, ref string) string {
	t.Helper()
	return gitStdout(t, repo, "for-each-ref", "--format=%(objectname)", ref)
}

// A1, A11: the table and the four trailing lines, zeros printed, strings cleaned.
func TestNectarRender(t *testing.T) {
	rep := nectar.Report{Summary: nectar.Summary{SchemaVersion: 1, RulesVersion: 1, Repo: "waggle-sh/waggle",
		MainSHA: strings.Repeat("a", 40), ComputedAt: nectarNow, Keepers: []nectar.KeeperScore{
			{KeeperID: "alice", Nectar: 14250, Matured: 17500, Pending: 3000, ClawedBack: 3250, Units: 6},
			{KeeperID: "bob", Pending: 2500, Units: 1},
			{KeeperID: "ev\x1b[2Jil", Units: 1},
		}},
		Counts: nectar.Counts{Unbound: 3, Unattributed: 2, Incomplete: 1,
			Excluded:        []nectar.ExcludedKeep{{KeeperID: "bot\nx", Weight: 1500}},
			ReciprocalPairs: []nectar.PairShare{{A: "alice", B: "bob", Share: 500}}}}
	var b bytes.Buffer
	renderNectar(&b, nectarRun{Report: rep, Receipt: "none"}, "")
	want := `keeper_id  nectar  matured  pending  clawed_back  units
alice      14.250  17.500   3.000    3.250        6
bob        0.000   0.000    2.500    0.000        1
ev[2Jil    0.000   0.000    0.000    0.000        1
counts: unbound 3, unattributed 2, orphaned 0, unmerged 0, incomplete 1, third_party 0, fork_mismatch 0, direct_push 0, self_merge 0, carried 0, unconfirmed_revert 0
excluded: botx 1.500
reciprocal pairs: alice bob 0.500
scored: main ` + strings.Repeat("a", 40) + `, receipts none, rules_version 1, at ` + nectarNow + `, repo waggle-sh/waggle
`
	if b.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", b.String(), want)
	}
	// --keeper filters the table only.
	b.Reset()
	renderNectar(&b, nectarRun{Report: rep, Receipt: "none", Local: true}, "bob")
	got := b.String()
	if !strings.Contains(got, "\nbob ") || strings.Contains(got, "alice  ") || !strings.Contains(got, "unattributed 2") || !strings.HasSuffix(got, ", local only\n") {
		t.Errorf("filtered:\n%s", got)
	}
	b.Reset()
	renderNectar(&b, nectarRun{Report: rep, Receipt: "none", Rewound: true}, "nobody")
	if !strings.Contains(b.String(), "no attributed units\n") || !strings.HasSuffix(b.String(), ", receipts rewound\n") {
		t.Errorf("unknown keeper:\n%s", b.String())
	}
	b.Reset()
	renderNectar(&b, nectarRun{Report: nectar.Report{}, Receipt: "none"}, "")
	if !strings.Contains(b.String(), "excluded: none\nreciprocal pairs: none\n") || !strings.Contains(b.String(), "repo (none)") {
		t.Errorf("empty:\n%s", b.String())
	}
}

// A10: the repo string.
func TestNectarRepoName(t *testing.T) {
	for raw, want := range map[string]string{
		"https://github.com/o/r":          "o/r",
		"https://github.com/o/r.git":      "o/r",
		"git@github.com:o/r":              "o/r",
		"git@github.com:o/r.git":          "o/r",
		"ssh://git@github.com/o/r":        "o/r",
		"ssh://git@github.com/o/r.git":    "o/r",
		"https://gitlab.com/o/r":          "",
		"https://github.com/-bad_owner/r": "",
		"https://github.com/o/..":         "",
		"https://github.com/o/r/extra":    "",
		"":                                "",
	} {
		if got := githubRepoName(raw); got != want {
			t.Errorf("%q: got %q, want %q", raw, got, want)
		}
	}
}

// A12: help is plain, the main.go line has no dash and no explain.
func TestNectarSummaryHelp(t *testing.T) {
	_, errb, exit := nectarCLI(t, "-h")
	if exit != 0 {
		t.Errorf("-h exit %d", exit)
	}
	checkPlain(t, "nectar -h", errb, "reputation score", "--no-fetch", "--receipts", "waggle nectar backfill")
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "nectar ") {
			checkPlain(t, "main.go nectar line", line, "Reputation score")
			if strings.Contains(line, "explain") {
				t.Errorf("line names explain: %s", line)
			}
		}
	}
	if !strings.HasPrefix(nectarUsage, backfillUsage) {
		t.Error("nectarUsage changed")
	}
}

// A2, A3, A4, A8, A9: the fetch path, determinism and the pins.
func TestNectarScoreInputs(t *testing.T) {
	repo, origin := scoredRepo(t)
	local := func() string { return refOrNone(t, repo, "refs/waggle/receipts/") }
	base := []string{"--repo", repo, "--now", nectarNow}
	run := func(extra ...string) (string, string, int) {
		return nectarCLI(t, append(append([]string{}, base...), extra...)...)
	}

	// --no-fetch scores this clone as it is and moves no ref. Here the clone
	// holds no receipt (the task reads unattributed), so it is a finding.
	held := gitStdout(t, repo, "rev-parse", "refs/waggle/receipts/main")
	git(t, repo, "update-ref", "-d", "refs/waggle/receipts/main")
	out, _, exit := run("--no-fetch")
	if local() != "" || !strings.Contains(out, "receipts none") || !strings.HasSuffix(strings.TrimSpace(out), "local only") || exit != 1 {
		t.Errorf("--no-fetch: exit %d, local %q:\n%s", exit, local(), out)
	}
	git(t, repo, "update-ref", "-d", "refs/remotes/origin/main")
	if out, _, exit := run("--no-fetch", "--json"); exit != 3 || out != "" {
		t.Errorf("--no-fetch without a tracking ref: exit %d %q", exit, out)
	}

	// The default run fetches the receipts first, then main.
	json1, errb, exit := run("--json")
	if local() == "" || exit != 0 && exit != 1 || errb != "" {
		t.Fatalf("json: exit %d stderr %q local %q", exit, errb, local())
	}
	if got := gitStdout(t, repo, "rev-parse", "refs/waggle/receipts/main"); got != held {
		t.Errorf("fetched receipt head %s, want %s", got, held)
	}
	if !strings.HasSuffix(json1, "}\n") || strings.Count(json1, "\n") != 1 {
		t.Errorf("stdout is not one line: %q", json1)
	}
	var sum testSummary
	if err := json.Unmarshal([]byte(json1), &sum); err != nil || sum.SchemaVersion != 1 || sum.RulesVersion != 1 || sum.ComputedAt != nectarNow {
		t.Fatalf("summary %+v: %v", sum, err)
	}
	wantMain := gitStdout(t, repo, "rev-parse", "refs/remotes/origin/main")
	if sum.MainSHA != wantMain || len(sum.Keepers) != 1 || sum.Keepers[0].KeeperID != "bob" || sum.Keepers[0].Units != 1 {
		t.Errorf("summary %+v, want main %s and one keeper bob", sum, wantMain)
	}
	if k := sum.Keepers[0]; k.Nectar != 0 || k.Pending == 0 {
		t.Errorf("pending must stay out of nectar: %+v", k)
	}
	if sum.Repo != "" {
		t.Errorf("a local bare origin names no GitHub repo, got %q", sum.Repo)
	}

	// Two runs and --recompute give the same bytes, and so does a later commit.
	for _, extra := range [][]string{{"--json"}, {"--json", "--recompute"}, {"--json", "--no-fetch"}} {
		if got, _, _ := run(extra...); got != json1 {
			t.Errorf("%v differs:\n%s\n%s", extra, got, json1)
		}
	}
	rcHead := gitStdout(t, repo, "rev-parse", "refs/waggle/receipts/main")

	// A9: pins reproduce a cold run at the older inputs.
	commitGateFile(t, repo, "internal/x/t.go", "T-023: later")
	git(t, repo, "push", "-q", "origin", "main")
	if got, _, _ := run("--json", "--no-fetch", "--main", wantMain, "--receipts", rcHead); got != json1 {
		t.Errorf("pinned run differs:\n%s\n%s", got, json1)
	}
	newer, _, _ := run("--json")
	if newer == json1 {
		t.Error("a fetched run after a push should score the new main")
	}
	zero := strings.Repeat("0", 40)
	out, _, _ = run("--no-fetch", "--receipts", zero)
	if !strings.Contains(out, "receipts none") || !strings.Contains(out, "no attributed units") {
		t.Errorf("zero receipts:\n%s", out)
	}
	for _, bad := range [][]string{{"--main", "abc123"}, {"--main", strings.Repeat("g", 40)}, {"--receipts", "xyz"}, {"--receipts", strings.Repeat("a", 39)}, {"--main", strings.Repeat("a", 64)}} {
		if out, _, exit := run(append([]string{"--json"}, bad...)...); exit != 2 || out != "" {
			t.Errorf("%v: exit %d stdout %q", bad, exit, out)
		}
	}
	for _, bad := range [][]string{{"--main", strings.Repeat("a", 40)}, {"--receipts", strings.Repeat("b", 40)}, {"--receipts", wantMain}} {
		out, errb, exit := run(append([]string{"--json"}, bad...)...)
		if exit != 3 || out != "" || !strings.Contains(errb, bad[1]) {
			t.Errorf("%v: exit %d stdout %q stderr %q", bad, exit, out, errb)
		}
	}

	// A remote that cannot be fetched warns once and scores local state.
	git(t, repo, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	out, errb, exit = run()
	if exit != 0 && exit != 1 || strings.Count(errb, "warning") != 1 || !strings.HasSuffix(strings.TrimSpace(out), "local only") {
		t.Errorf("missing remote: exit %d stderr %q\n%s", exit, errb, out)
	}
	git(t, repo, "remote", "set-url", "origin", origin)

	// A rewound remote chain is not taken: exit 1, said so, local chain scored.
	before := local()
	other := gitStdout(t, origin, "commit-tree", "-m", "rewound", "refs/waggle/receipts/main^{tree}")
	git(t, origin, "update-ref", "refs/waggle/receipts/main", other)
	out, errb, exit = run()
	if exit != 1 || !strings.Contains(out, "receipts rewound") || !strings.Contains(errb, "rewound") || local() != before {
		t.Errorf("rewound: exit %d stderr %q local moved %v\n%s", exit, errb, local() != before, out)
	}
	if out, _, exit = run("--json"); exit != 1 || strings.Contains(out, "rewound") || !strings.HasPrefix(out, "{") {
		t.Errorf("rewound --json: exit %d %q", exit, out)
	}
}

// A8: --no-fetch with a remote needs the tracking ref.
func TestNectarNoFetchNeedsTrackingRef(t *testing.T) {
	repo, _ := scoredRepo(t)
	git(t, repo, "update-ref", "-d", "refs/remotes/origin/main")
	out, errb, exit := nectarCLI(t, "--repo", repo, "--now", nectarNow, "--no-fetch", "--json")
	if exit != 3 || out != "" || errb == "" {
		t.Errorf("exit %d stdout %q stderr %q", exit, out, errb)
	}
}

// A5, A6, A7: flags, exit codes and the empty stdout of a failed --json run.
func TestNectarSummaryExits(t *testing.T) {
	repo, _ := scoredRepo(t)
	base := []string{"--repo", repo, "--now", nectarNow}
	with := func(extra ...string) (string, string, int) {
		return nectarCLI(t, append(append([]string{}, base...), extra...)...)
	}
	out, _, exit := with("--keeper", "bob")
	if exit != 0 && exit != 1 || strings.Count(strings.SplitN(out, "counts:", 2)[0], "\n") != 2 {
		t.Errorf("--keeper bob: exit %d\n%s", exit, out)
	}
	out, _, _ = with("--keeper", "nobody")
	if !strings.Contains(out, "no attributed units") || !strings.Contains(out, "counts:") {
		t.Errorf("unknown keeper:\n%s", out)
	}
	for _, args := range [][]string{{"--keeper", "bad id!", "--json"}, {"--keeper", "bad id!"}, {"--keeper", "bob", "--json"},
		{"--bogus", "--json"}, {"extra", "--json"}, {"--now", "yesterday", "--json"}} {
		a := append([]string{}, args...)
		if a[len(a)-1] != "--json" {
			a = append(a, "--json")
		}
		if out, errb, exit := nectarCLI(t, append([]string{"--repo", repo}, a...)...); exit != 2 || out != "" {
			t.Errorf("%v: exit %d stdout %q stderr %q", args, exit, out, errb)
		}
	}
	if _, _, exit := nectarCLI(t, "--repo", repo, "--now", "nope"); exit != 2 {
		t.Errorf("bad --now exit %d", exit)
	}

	// A6: day 30 moves the unit from pending to matured.
	var early, late testSummary
	mt, err := time.Parse(time.RFC3339, gitStdout(t, repo, "log", "-1", "--format=%cI", "origin/main"))
	if err != nil {
		t.Fatal(err)
	}
	at := func(d time.Duration) string { return mt.Add(d).UTC().Format(time.RFC3339) }
	o1, _, _ := nectarCLI(t, "--repo", repo, "--json", "--now", at(29*24*time.Hour))
	o2, _, _ := nectarCLI(t, "--repo", repo, "--json", "--now", at(31*24*time.Hour))
	if json.Unmarshal([]byte(o1), &early) != nil || json.Unmarshal([]byte(o2), &late) != nil || len(early.Keepers) != 1 || len(late.Keepers) != 1 {
		t.Fatalf("%q %q", o1, o2)
	}
	if e, l := early.Keepers[0], late.Keepers[0]; e.Matured != 0 || e.Pending == 0 || l.Matured == 0 || l.Pending != 0 || l.Nectar != l.Matured {
		t.Errorf("window: early %+v late %+v", e, l)
	}

	// Not a Waggle repo, invalid config and relay mode exit 3 with empty stdout.
	plain := t.TempDir()
	git(t, plain, "init", "-q", "-b", "main")
	if out, _, exit := nectarCLI(t, "--repo", plain, "--json"); exit != 3 || out != "" {
		t.Errorf("not a repo: exit %d %q", exit, out)
	}
	cfgPath := filepath.Join(repo, ".waggle/control/config.yaml")
	orig, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"invalid": "forge: [\n", "relay": strings.Replace(string(orig), "comb_mode: direct", "comb_mode: relay", 1)} {
		if body == string(orig) {
			t.Fatalf("%s: config has no comb_mode line", name)
		}
		if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, _, exit := nectarCLI(t, "--repo", repo, "--json"); exit != 3 || out != "" {
			t.Errorf("%s: exit %d %q", name, exit, out)
		}
	}
}

// A7: a finding exits 1 with full output, the same JSON bytes as a clean run would give.
func TestNectarSummaryFindings(t *testing.T) {
	repo, _ := scoredRepo(t)
	// A merge of a task with no claim and no receipt is attributed to nobody.
	commitGateFile(t, repo, "internal/z/c.go", "T-025: no claim")
	git(t, repo, "push", "-q", "origin", "main")
	out, _, exit := nectarCLI(t, "--repo", repo, "--now", nectarNow)
	if exit != 1 || !strings.Contains(out, "unattributed 1") {
		t.Errorf("unattributed: exit %d\n%s", exit, out)
	}
	j, _, jexit := nectarCLI(t, "--repo", repo, "--now", nectarNow, "--json")
	var sum testSummary
	if jexit != 1 || json.Unmarshal([]byte(j), &sum) != nil || sum.Unattributed != 1 {
		t.Errorf("json: exit %d %q", jexit, j)
	}
}

// testSummary reads the 9.2 object; nectar.Milli only marshals.
type testSummary struct {
	SchemaVersion int    `json:"schema_version"`
	RulesVersion  int    `json:"rules_version"`
	Repo          string `json:"repo"`
	MainSHA       string `json:"main_sha"`
	ComputedAt    string `json:"computed_at"`
	Keepers       []struct {
		KeeperID   string  `json:"keeper_id"`
		Nectar     float64 `json:"nectar"`
		Matured    float64 `json:"matured"`
		Pending    float64 `json:"pending"`
		ClawedBack float64 `json:"clawed_back"`
		Units      int     `json:"units"`
	} `json:"keepers"`
	Unattributed int `json:"unattributed"`
	Orphaned     int `json:"orphaned"`
}

// releaseOnOrigin claims task for bob on dir, lands one commit for it on main,
// pushes it and writes its gate-run receipt to origin only. It returns the commit.
func releaseOnOrigin(t *testing.T, repo, task, dir string) string {
	t.Helper()
	if res, exit := runJSON(t, claimCLI, task, "--dirs", dir, "--gate", "full", "--owner", "bob", "--repo", repo); exit != 0 {
		t.Fatalf("claim %s: %+v", task, res)
	}
	before := gitStdout(t, repo, "rev-parse", "HEAD")
	after := commitGateFile(t, repo, dir+"/f.go", "T-0"+task+": land")
	git(t, repo, "push", "-q", "origin", "main")
	if res, exit, stderr := runGateReceipt(t, repo, before, after); exit != 0 || res.Written != 1 {
		t.Fatalf("gate-receipt %s: exit %d %+v %q", task, exit, res, stderr)
	}
	return after
}

// A3, A8: the seams. Work that lands between the two fetches, or after the
// inputs are resolved, changes nothing; work on the remote before the run is scored.
func TestNectarSeams(t *testing.T) {
	// seamed runs the CLI once with f called at stage.
	seamed := func(t *testing.T, stage string, f func(), args ...string) (string, string, int) {
		t.Cleanup(func() { nectarSeam = func(string) {} })
		nectarSeam = func(s string) {
			if s == stage {
				f()
			}
		}
		defer func() { nectarSeam = func(string) {} }()
		return nectarCLI(t, args...)
	}

	t.Run("remote work is scored by the default run", func(t *testing.T) {
		repo, origin := scoredRepo(t)
		tracked := gitStdout(t, repo, "rev-parse", "refs/remotes/origin/main")
		m := releaseOnOrigin(t, repo, "26", "internal/v")
		git(t, repo, "update-ref", "refs/remotes/origin/main", tracked)
		git(t, repo, "update-ref", "-d", "refs/waggle/receipts/main")
		if refOrNone(t, repo, "refs/waggle/receipts/") != "" {
			t.Fatal("this clone holds a receipt before the run")
		}
		rc := gitStdout(t, origin, "rev-parse", "refs/waggle/receipts/main")
		out, errb, exit := nectarCLI(t, "--repo", repo, "--now", nectarNow)
		if exit != 0 || errb != "" {
			t.Fatalf("exit %d stderr %q\n%s", exit, errb, out)
		}
		if !strings.Contains(out, "\nbob ") || !strings.Contains(out, "        2\n") || !strings.Contains(out, "unattributed 0, orphaned 0") ||
			!strings.Contains(out, "scored: main "+m+", receipts "+rc+",") || strings.Contains(out, "local only") {
			t.Errorf("want main %s and receipts %s scored, bob with 2 units:\n%s", m, rc, out)
		}
	})

	t.Run("a main commit and its receipt between the fetches", func(t *testing.T) {
		repo, origin := scoredRepo(t)
		if _, _, exit := nectarCLI(t, "--repo", repo, "--now", nectarNow); exit != 0 {
			t.Fatalf("first run exit %d", exit)
		}
		held := gitStdout(t, repo, "rev-parse", "refs/waggle/receipts/main")
		var m string
		j, errb, exit := seamed(t, "receipts", func() { m = releaseOnOrigin(t, repo, "26", "internal/v") },
			"--repo", repo, "--now", nectarNow, "--json")
		var sum testSummary
		if err := json.Unmarshal([]byte(j), &sum); err != nil {
			t.Fatalf("%q %q: %v", j, errb, err)
		}
		if gitStdout(t, origin, "rev-parse", "refs/waggle/receipts/main") == held {
			t.Fatal("the seam wrote no receipt to the origin")
		}
		// main holds the new commit, the receipts do not: unattributed, never orphaned.
		if sum.Orphaned != 0 || sum.MainSHA != m || sum.Unattributed != 1 || exit != 1 || len(sum.Keepers) != 1 || sum.Keepers[0].Units != 1 {
			t.Errorf("exit %d, want main %s, orphaned 0, unattributed 1: %+v", exit, m, sum)
		}
		out, _, _ := nectarCLI(t, "--repo", repo, "--now", nectarNow, "--no-fetch", "--receipts", held)
		if !strings.Contains(out, "receipts "+held+",") {
			t.Errorf("the pinned receipts line:\n%s", out)
		}
	})

	t.Run("a commit and a receipt after resolution", func(t *testing.T) {
		repo, _ := scoredRepo(t)
		want, _, exit := nectarCLI(t, "--repo", repo, "--now", nectarNow, "--json")
		if exit != 0 {
			t.Fatalf("first run exit %d %q", exit, want)
		}
		got, errb, exit := seamed(t, "main", func() {
			releaseOnOrigin(t, repo, "26", "internal/v")
			// A concurrent sync moves both local refs past the resolved inputs.
			git(t, repo, "fetch", "-q", "origin", "+refs/waggle/receipts/main:refs/waggle/receipts/main", "+refs/heads/main:refs/remotes/origin/main")
		}, "--repo", repo, "--now", nectarNow, "--json")
		if got != want || exit != 0 {
			t.Errorf("exit %d stderr %q\ngot  %s\nwant %s", exit, errb, got, want)
		}
	})
}

// A7: an orphaned receipt, one whose merge is not on the scored main, exits 1
// with the whole report.
func TestNectarSummaryOrphaned(t *testing.T) {
	repo, _ := scoredRepo(t)
	if _, _, exit := nectarCLI(t, "--repo", repo, "--now", nectarNow); exit != 0 {
		t.Fatalf("first run exit %d", exit)
	}
	parent := gitStdout(t, repo, "rev-parse", "refs/remotes/origin/main^")
	args := []string{"--repo", repo, "--now", nectarNow, "--no-fetch", "--main", parent}
	out, errb, exit := nectarCLI(t, args...)
	rc := gitStdout(t, repo, "rev-parse", "refs/waggle/receipts/main")
	want := `keeper_id  nectar  matured  pending  clawed_back  units
no attributed units
counts: unbound 2, unattributed 0, orphaned 1, unmerged 0, incomplete 0, third_party 0, fork_mismatch 0, direct_push 0, self_merge 0, carried 0, unconfirmed_revert 0
excluded: none
reciprocal pairs: none
scored: main ` + parent + `, receipts ` + rc + `, rules_version 1, at ` + nectarNow + `, repo (none), local only
`
	if exit != 1 || out != want {
		t.Errorf("exit %d stderr %q\ngot:\n%s\nwant:\n%s", exit, errb, out, want)
	}
	j, _, jexit := nectarCLI(t, append(args, "--json")...)
	wantJSON := `{"schema_version":1,"rules_version":1,"repo":"","main_sha":"` + parent + `","computed_at":"` + nectarNow + `","keepers":[],"unattributed":0,"orphaned":1}` + "\n"
	if jexit != 1 || j != wantJSON {
		t.Errorf("json: exit %d\ngot  %s\nwant %s", jexit, j, wantJSON)
	}
}

// A2: --json is the 9.2 object byte for byte: T-051's golden through the CLI's
// encoder, and a scored run pinned exactly.
func TestNectarJSONBytes(t *testing.T) {
	t.Run("T-051 golden", func(t *testing.T) {
		golden, err := os.ReadFile("../../internal/nectar/testdata/summary.golden.json")
		if err != nil {
			t.Fatal(err)
		}
		rep := nectar.Report{Summary: nectar.Summary{SchemaVersion: 1, RulesVersion: 1, Repo: "acme/repo",
			MainSHA: "29310fe6b638f849a02c9bf6e65f66c98a10150f", ComputedAt: "2026-02-11T05:03:00Z",
			Keepers: []nectar.KeeperScore{
				{KeeperID: "alice", Nectar: 6819, Matured: 6819, Units: 2},
				{KeeperID: "bob", Nectar: 5334, Matured: 5334, Units: 1},
				{KeeperID: "carol", Pending: 1057, Units: 1},
			}},
			// Counts and units never reach the 9.2 object.
			Counts: nectar.Counts{Unbound: 4, Excluded: []nectar.ExcludedKeep{{KeeperID: "x", Weight: 1}}},
			Units:  []nectar.Unit{{}}}
		got, err := nectarJSON(rep)
		if err != nil || !bytes.Equal(got, golden) {
			t.Errorf("err %v\ngot  %q\nwant %q", err, got, golden)
		}
	})

	t.Run("scored run", func(t *testing.T) {
		repo, _ := scoredRepo(t)
		out, errb, exit := nectarCLI(t, "--repo", repo, "--now", nectarNow, "--json")
		main := gitStdout(t, repo, "rev-parse", "refs/remotes/origin/main")
		want := `{"schema_version":1,"rules_version":1,"repo":"","main_sha":"` + main + `","computed_at":"` + nectarNow +
			`","keepers":[{"keeper_id":"bob","nectar":0.000,"matured":0.000,"pending":1.057,"clawed_back":0.000,"units":1}],"unattributed":0,"orphaned":0}` + "\n"
		if exit != 0 || errb != "" || out != want {
			t.Errorf("exit %d stderr %q\ngot  %s\nwant %s", exit, errb, out, want)
		}
	})
}

// A5, A9: failures named one by one.
func TestNectarSummaryRejects(t *testing.T) {
	repo, origin := scoredRepo(t)
	if _, _, exit := nectarCLI(t, "--repo", repo, "--now", nectarNow); exit != 0 {
		t.Fatalf("first run exit %d", exit)
	}
	mainOID := gitStdout(t, repo, "rev-parse", "refs/remotes/origin/main")
	offChain := gitStdout(t, origin, "commit-tree", "-m", "not on the chain", "refs/waggle/receipts/main^{tree}")
	git(t, repo, "fetch", "-q", "origin", offChain)
	for _, tc := range []struct {
		name string
		args []string
		exit int
		says string
	}{
		{"A5 bad --keeper with --json", []string{"--keeper", "bad id!", "--json"}, 2, "bad id!"},
		{"A9 --receipts a main commit", []string{"--no-fetch", "--receipts", mainOID, "--json"}, 3, mainOID},
		{"A9 --receipts a receipt commit off the chain", []string{"--no-fetch", "--receipts", offChain, "--json"}, 3, offChain},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, errb, exit := nectarCLI(t, append([]string{"--repo", repo, "--now", nectarNow}, tc.args...)...)
			if exit != tc.exit || out != "" || !strings.Contains(errb, tc.says) {
				t.Errorf("exit %d stdout %q stderr %q", exit, out, errb)
			}
			checkPlain(t, tc.name, errb)
		})
	}
}

// The scored line says local only when there is no remote to fetch from.
func TestNectarNoRemoteLocalOnly(t *testing.T) {
	repo, _ := scoredRepo(t)
	git(t, repo, "remote", "remove", "origin")
	p := filepath.Join(repo, ".waggle/control/config.yaml")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(strings.Replace(string(data), "  remote: origin\n", "", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errb, _ := nectarCLI(t, "--repo", repo, "--now", nectarNow)
	if !strings.HasSuffix(out, ", repo (none), local only\n") || errb != "" {
		t.Errorf("stderr %q\n%s", errb, out)
	}
}
