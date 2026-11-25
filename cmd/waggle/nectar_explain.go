// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/signal"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/nectar"
)

const explainUsage = "Usage: waggle nectar explain <task> [--repo <dir>] [--now <rfc3339>] [--main <oid>] [--receipts <oid>] [--no-fetch] [--json]"

const explainHelp = "Shows why one task earned its reputation score: its state, dates, weight, release record, flags and the rule behind each reason, with the pull request author beside the keeper id. It reads the same inputs as waggle nectar and fetches release records, then main, unless --no-fetch is given."

// reasonText is one fixed sentence for each reason code the scorer writes. An
// unknown code prints bare.
var reasonText = map[string]string{
	"no_binding":              "the commit names no task, so no keeper is credited",
	"unconfirmed_revert":      "the commit looks like a revert but no reverted commit was confirmed, so nothing was clawed back",
	"min_changed_lines":       "the change is under the minimum line count, so it weighs as a minimal unit",
	"receipt_unmerged":        "a release record for this task names no merge, and none names this one",
	"no_valid_receipt":        "no release record names this merge, so no keeper is credited",
	"third_party":             "the release record was written by someone other than the keeper",
	"fork_mismatch":           "the pull request author differs from the keeper the record credits",
	"receipt_incomplete":      "the release record was written offline and lacks forge facts, so review is not checked",
	"self_merge_multi_keeper": "the keeper merged their own pull request in a repo with several keepers, so the review factor is halved",
	"direct_push":             "no pull request is recorded, so the 30 day window starts when the record was first seen",
	"restored":                "a revert was itself reverted, so the window restarts at the re-landing",
	"revert_before_maturity":  "the work was reverted before the window ended, so it earns nothing",
	"revert_after_maturity":   "the work was reverted after the window ended, so its weight is clawed back",
	"partial_revert":          "part of the work was reverted, so that share is clawed back or withheld",
	"superseded":              "a later task replaced this work, so survival is not measured",
	"alive_below_ratio":       "too few of the added lines are still alive on main, so the unit is dropped",
	"reciprocal_pair":         "two keepers approve each other's work too often, so the review factor is discounted",
	"daily_cap_carry":         "the daily cap was reached, so part of the weight carries to a later day",
	"orphaned_rewritten":      "the merged commit left main and its base is still on main, so it was rewritten",
	"orphaned_missing":        "the merged commit left main and its base is gone too",
}

// runNectarExplain implements `waggle nectar explain <task>`. Exit codes: 0
// explained (or no unit), 1 a finding for this task or a rewound receipt chain,
// 2 usage, 3 not a Waggle repo, config invalid, relay mode or any other failure.
// Stdout holds only the explanation; with --json a failure leaves it empty.
func runNectarExplain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("nectar explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, explainUsage)
		fmt.Fprintln(stderr, explainHelp)
		fs.PrintDefaults()
	}
	repo := fs.String("repo", "", "repository directory (default: the current work tree root)")
	nowFlag := fs.String("now", "", "score as of this RFC 3339 time (default: the current time)")
	mainPin := fs.String("main", "", "score at this main commit (full hex id)")
	rcPin := fs.String("receipts", "", "score with the release records up to this commit (full hex id, or all zeros for none)")
	noFetch := fs.Bool("no-fetch", false, "do not fetch; score what this clone holds")
	asJSON := fs.Bool("json", false, "print the detail object for this task and nothing else")
	// The task may come before or after the flags.
	var rest []string
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return nectarUsageEx
		}
		if fs.NArg() == 0 {
			break
		}
		// After a "--" every argument is positional; do not reparse it.
		if i := len(args) - fs.NArg() - 1; i >= 0 && args[i] == "--" {
			rest = append(rest, fs.Args()...)
			break
		}
		rest = append(rest, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if len(rest) != 1 {
		return nectarFail(stderr, nectarUsageEx, "%s", explainUsage)
	}
	task, err := comb.ParseTicketID(rest[0])
	if err != nil {
		return nectarFail(stderr, nectarUsageEx, "%q is not a task id like T-052", rest[0])
	}
	var now time.Time
	if *nowFlag != "" {
		t, err := time.Parse(time.RFC3339, *nowFlag)
		if err != nil {
			return nectarFail(stderr, nectarUsageEx, "--now %q is not an RFC 3339 time", *nowFlag)
		}
		now = t
	}
	for _, p := range [][2]string{{"--main", *mainPin}, {"--receipts", *rcPin}} {
		if p[1] != "" && !nectarOIDRe.MatchString(p[1]) {
			return nectarFail(stderr, nectarUsageEx, "%s %q is not a full hex object id", p[0], p[1])
		}
	}
	root, err := repoRoot(*repo)
	if err != nil {
		if code, ok := noRepoExit(stderr, "nectar", err); ok {
			return code
		}
		return nectarFail(stderr, nectarEnvEx, "failed: %v", err)
	}
	if now.IsZero() {
		now = statusClock()
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	run, code := nectarScore(ctx, root, *mainPin, *rcPin, now, *noFetch, false, stderr)
	if code != 0 {
		return code
	}
	var units []nectar.Unit
	for _, u := range run.Report.Units {
		if u.Task == task {
			units = append(units, u)
		}
	}
	exit := 0
	if run.Rewound {
		exit = nectarFinding
	}
	for _, u := range units {
		if u.Kind == nectar.KindUnit && (u.State == nectar.StateOrphaned || u.State == nectar.StateUnattributed) {
			exit = nectarFinding
		}
	}
	if *asJSON {
		narrowed := run.Report
		narrowed.Units = append([]nectar.Unit{}, units...)
		data, err := narrowed.Detail()
		if err != nil {
			return nectarFail(stderr, nectarEnvEx, "encode: %v", err)
		}
		stdout.Write(append(data, '\n'))
		return exit
	}
	renderExplain(stdout, task, units)
	if run.Rewound {
		fmt.Fprintln(stdout, "receipts rewound: the remote release record chain does not hold this clone's records, so the local records were scored")
	}
	return exit
}

// orNone prints a cleaned string, or none when it is empty.
func orNone(s string) string {
	if s = clean(s); s == "" {
		return "none"
	}
	return s
}

func strOrNone(s *string) string {
	if s == nil {
		return "none"
	}
	return orNone(*s)
}

// renderExplain writes the labeled lines for the task's unit and then its
// revert entries. With no unit it says so before the revert entries.
func renderExplain(w io.Writer, task string, units []nectar.Unit) {
	shown := false
	for _, u := range units {
		if u.Kind != nectar.KindUnit {
			continue
		}
		shown = true
		explainUnit(w, u)
	}
	if !shown {
		fmt.Fprintf(w, "no merged unit for %s\n", task)
	}
	for _, u := range units {
		if u.Kind == nectar.KindRevert {
			fmt.Fprintf(w, "revert: commit %s, merged_at %s, clawed_back %s\n", clean(u.MergedSHA), orNone(u.MergedAt), milli(u.ClawedBack))
		}
	}
}

func explainUnit(w io.Writer, u nectar.Unit) {
	line := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
	state := u.State
	switch {
	case u.Rewritten:
		state += " (rewritten)"
	case u.Missing:
		state += " (missing)"
	}
	line("task: %s", orNone(u.Task))
	line("kind: %s", orNone(u.Kind))
	line("state: %s", state)
	line("commits: %s", orNone(strings.Join(u.Commits, ", ")))
	line("merged_sha: %s", orNone(u.MergedSHA))
	line("base_sha: %s", strOrNone(u.BaseSHA))
	line("merged_at: %s", orNone(u.MergedAt))
	line("first_seen: %s", strOrNone(u.FirstSeen))
	line("matures_at: %s", strOrNone(u.MaturesAt))
	f := u.Factors
	// Factors.Tier is the risk tier, not its factor, so the line lists the
	// factors rather than writing a product.
	line("weight: %s, tier %d, size %s, review %s", milli(u.Weight), f.Tier, milli(f.Size), milli(f.Review))
	line("changed_lines: %s", milli(f.ChangedLines))
	line("code_lines: %d", f.CodeLines)
	line("docs_lines: %d", f.DocsLines)
	if len(f.ExcludedFiles) == 0 {
		line("excluded_files: none")
	}
	for _, x := range f.ExcludedFiles {
		line("excluded_files: %s", orNone(x))
	}
	// pr_author sits beside keeper_id (RFC-0005 10).
	author := "none"
	if u.Receipt != nil {
		author = strOrNone(u.Receipt.PRAuthor)
	}
	line("keeper_id: %s, pr_author %s", strOrNone(u.KeeperID), author)
	line("keeper_share: %s", milli(u.KeeperShare))
	if len(u.CoKeepers) == 0 {
		line("co_keepers: none")
	}
	for _, c := range u.CoKeepers {
		line("co_keepers: %s %s", orNone(c.KeeperID), milli(c.Share))
	}
	if u.AliveRatio == nil {
		line("alive_ratio: none")
	} else {
		line("alive_ratio: %s", milli(*u.AliveRatio))
	}
	line("clawed_back: %s", milli(u.ClawedBack))
	line("carried: %s", milli(u.Carried))
	if r := u.Receipt; r == nil {
		line("receipt: none")
	} else {
		pr := "none"
		if r.PRNumber != nil {
			pr = fmt.Sprint(*r.PRNumber)
		}
		approvers := make([]string, 0, len(r.Approvers))
		for _, a := range r.Approvers {
			approvers = append(approvers, clean(a))
		}
		line("receipt: oid %s, source %s, status %s, merge_proof %s, incomplete %t, released_by %s, pr_number %s, head_branch %s, approvers %s",
			orNone(r.OID), orNone(r.Source), orNone(r.Status), orNone(r.MergeProof), r.Incomplete, orNone(r.ReleasedBy), pr, strOrNone(r.HeadBranch), joinOrNone(approvers))
	}
	if u.ThirdParty {
		s := "flag: third_party"
		if u.Maintainer {
			s += ", released by a maintainer"
		}
		line("%s", s)
	}
	for _, fl := range []struct {
		on   bool
		name string
	}{{u.Relay, "relay"}, {u.ForkMismatch, "fork_mismatch"}, {u.DirectPush, "direct_push"}, {u.SelfMerge, "self_merge"}} {
		if fl.on {
			line("flag: %s", fl.name)
		}
	}
	if len(u.Reasons) == 0 {
		line("reasons: none")
	}
	for _, code := range u.Reasons {
		if text, ok := reasonText[code]; ok {
			line("reason: %s: %s", clean(code), text)
		} else {
			line("reason: %s", clean(code))
		}
	}
}
