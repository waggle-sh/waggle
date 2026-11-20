// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/signal"

	"github.com/waggle-sh/waggle/internal/dispatch"
)

const nectarUsage = "Usage: waggle nectar backfill [--apply] [--owner <id>] [--repo <dir>] [--json]"

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

// runNectar implements `waggle nectar`. Its only subcommand here is backfill.
func runNectar(args []string, stdout, stderr io.Writer) int {
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
		fmt.Fprintln(stderr, nectarUsage)
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
		return o.fail("", usageError("%s", nectarUsage[len("Usage: "):]))
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
