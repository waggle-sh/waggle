// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/waggle-sh/waggle/internal/dispatch"
	"github.com/waggle-sh/waggle/internal/nectar"
)

const backfillUsage = "Usage: waggle nectar backfill [--apply] [--owner <id>] [--repo <dir>] [--json]"
const gateReceiptUsage = "Usage: waggle nectar gate-receipt --before <sha> --after <sha> [--actor <id>] [--repo <dir>] [--json]"
const nectarUsage = backfillUsage + "\n       waggle nectar gate-receipt --before <sha> --after <sha> [--actor <id>] [--repo <dir>] [--json]"

// backfillItemJSON is one task of the backfill plan.
type backfillItemJSON struct {
	Task      string  `json:"task"`
	State     string  `json:"state"`
	Owner     *string `json:"owner"`
	Origin    *string `json:"origin"`
	MergedSHA *string `json:"merged_sha"`
	Proof     *string `json:"merge_proof"`
	Evidence  string  `json:"evidence"`
}

type backfillJSON struct {
	Version  int                `json:"version"`
	Command  string             `json:"command"`
	OK       bool               `json:"ok"`
	Applied  bool               `json:"applied"`
	Written  int                `json:"written"`
	Items    []backfillItemJSON `json:"items"`
	Warnings []string           `json:"warnings"`
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// runNectar implements `waggle nectar`: backfill and gate-receipt.
func runNectar(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "gate-receipt" {
		return runNectarGateReceipt(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "backfill" {
		return runNectarBackfill(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, nectarUsage)
	if len(args) > 0 && (args[0] == "-h" || args[0] == "-help" || args[0] == "--help") {
		return 0
	}
	return dispatch.ExitUsage
}

// runNectarBackfill implements `waggle nectar backfill`: it plans a receipt for
// each task that merged to main and has none, and with --apply writes them.
// Exit codes: 0 done, 1 interrupted, 2 usage, 3 environment.
func runNectarBackfill(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("nectar backfill", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, backfillUsage)
		fmt.Fprintln(stderr, "Adds a release record for each task that merged to main and has none, from claims that still exist, claims kept in the audit and release records, and tasks named in the decisions log. Without --apply it prints the plan and writes nothing.")
		fs.PrintDefaults()
	}
	apply := fs.Bool("apply", false, "write the planned records; without it the plan is only printed")
	owner := fs.String("owner", "", "keeper id credited for a task that only the decisions log names; without it those tasks stay unattributed")
	repo := fs.String("repo", ".", "repository")
	asJSON := fs.Bool("json", false, "print one JSON object")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return dispatch.ExitUsage
	}
	o := taskOut{"nectar backfill", *asJSON, stdout, stderr}
	if fs.NArg() > 0 {
		return o.fail("", usageError("%s", backfillUsage[len("Usage: "):]))
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	res, err := dispatch.Backfill(ctx, dispatch.BackfillRequest{RepoDir: *repo, Apply: *apply, Owner: *owner})
	o.warn(res.Warnings)
	if err != nil {
		return o.fail("", err)
	}
	if *asJSON {
		items := make([]backfillItemJSON, 0, len(res.Items))
		for _, it := range res.Items {
			items = append(items, backfillItemJSON{it.Task, it.State, nilIfEmpty(it.Owner), nilIfEmpty(it.Origin), nilIfEmpty(it.MergedSHA), nilIfEmpty(it.Proof), it.Evidence})
		}
		o.json(backfillJSON{jsonVersion, "nectar backfill", true, res.Applied, res.Written, items, nonNil(res.Warnings)})
		return 0
	}
	planned, unattributed := 0, 0
	for _, it := range res.Items {
		if it.State == dispatch.BackfillUnattributed {
			unattributed++
			fmt.Fprintf(stdout, "%s unattributed: %s\n", it.Task, clean(it.Evidence))
			continue
		}
		planned++
		fmt.Fprintf(stdout, "%s %s: owner %s from %s, merged %s (%s); %s\n", it.Task, it.State, clean(it.Owner), clean(it.Origin), it.MergedSHA[:min(len(it.MergedSHA), 12)], clean(it.Proof), clean(it.Evidence))
	}
	if res.Applied {
		fmt.Fprintf(stdout, "wrote %d record(s); %d task(s) unattributed\n", res.Written, unattributed)
	} else {
		fmt.Fprintf(stdout, "plan: %d record(s) to write, %d task(s) unattributed; nothing written, run again with --apply to write\n", planned, unattributed)
	}
	return 0
}

type gateItemJSON struct {
	Task      string  `json:"task"`
	MergedSHA string  `json:"merged_sha"`
	State     string  `json:"state"`
	Proof     *string `json:"merge_proof"`
	PR        *int    `json:"pr_number"`
	Evidence  string  `json:"evidence"`
}

type gateReceiptJSON struct {
	Version  int            `json:"version"`
	Command  string         `json:"command"`
	OK       bool           `json:"ok"`
	Written  int            `json:"written"`
	Items    []gateItemJSON `json:"items"`
	Warnings []string       `json:"warnings"`
}

// runNectarGateReceipt implements `waggle nectar gate-receipt`: the post-merge
// gate-run receipt writer (T-051 phase D). It writes one receipt for each task
// that landed on main in before..after with a live claim and no receipt, and
// changes no claim. Exit codes: 0 done (including nothing to write), 1
// interrupted, 2 usage, 3 environment.
func runNectarGateReceipt(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("nectar gate-receipt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, gateReceiptUsage)
		fmt.Fprintln(stderr, "Writes a gate-run release record for each task that landed on main between the two commits, has a live claim and has no record yet. It never releases or changes a claim, and needs push rights to refs/waggle/receipts/*.")
		fs.PrintDefaults()
	}
	before := fs.String("before", "", "main before the push (full commit id)")
	after := fs.String("after", "", "main after the push (full commit id)")
	actor := fs.String("actor", "", "who ran the push, recorded as released_by; default $GITHUB_ACTOR")
	repo := fs.String("repo", ".", "repository")
	asJSON := fs.Bool("json", false, "print one JSON object")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return dispatch.ExitUsage
	}
	o := taskOut{"nectar gate-receipt", *asJSON, stdout, stderr}
	if fs.NArg() > 0 || *before == "" || *after == "" {
		return o.fail("", usageError("%s", gateReceiptUsage[len("Usage: "):]))
	}
	who := *actor
	if who == "" {
		who = os.Getenv("GITHUB_ACTOR")
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	res, err := nectar.GateReceipt(ctx, nectar.GateReceiptRequest{RepoDir: *repo, Before: *before, After: *after, Actor: who})
	o.warn(res.Warnings)
	if err != nil {
		switch {
		case errors.Is(err, nectar.ErrUsage):
			err = &dispatch.Error{Code: dispatch.ExitUsage, Err: err}
		case ctx.Err() != nil:
			err = &dispatch.Error{Code: dispatch.ExitFailed, Err: fmt.Errorf("interrupted: %w", err)}
		}
		return o.fail("", err)
	}
	if *asJSON {
		items := make([]gateItemJSON, 0, len(res.Items))
		for _, it := range res.Items {
			j := gateItemJSON{Task: it.Task, MergedSHA: it.MergedSHA, State: it.State, Proof: nilIfEmpty(it.Proof), Evidence: it.Evidence}
			if it.PR > 0 {
				n := it.PR
				j.PR = &n
			}
			items = append(items, j)
		}
		o.json(gateReceiptJSON{jsonVersion, "nectar gate-receipt", true, res.Written, items, nonNil(res.Warnings)})
		return 0
	}
	for _, it := range res.Items {
		fmt.Fprintf(stdout, "%s %s: merged %s; %s\n", it.Task, it.State, it.MergedSHA[:min(len(it.MergedSHA), 12)], clean(it.Evidence))
	}
	fmt.Fprintf(stdout, "wrote %d record(s)\n", res.Written)
	return 0
}
