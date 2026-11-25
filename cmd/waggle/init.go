// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/scaffold"
)

// initBuildInfo is the build stamp source for init. Nil means the running
// binary's build info; tests set it to stand in for a stamped build.
var initBuildInfo func() (*debug.BuildInfo, bool)

// checkInitChoices rejects an invalid --profile, --delivery, --forge or
// --comb-mode before anything is written. The wording is the scaffold's.
func checkInitChoices(profile, delivery, forge, combMode string) error {
	if profile != "cli" {
		return fmt.Errorf("unsupported profile %q in v0.1 (only 'cli' is supported; others arrive in v0.2)", profile)
	}
	if delivery != "installed" && delivery != "hosted" {
		return fmt.Errorf("invalid --delivery=%q (must be 'installed' or 'hosted')", delivery)
	}
	if forge != "github" && forge != "none" {
		return fmt.Errorf("invalid --forge=%q (must be 'github' or 'none')", forge)
	}
	if combMode != "direct" && combMode != "relay" {
		return fmt.Errorf("invalid --comb-mode=%q (must be 'direct' or 'relay')", combMode)
	}
	return nil
}

// runInit scaffolds a hive in cwd or, with --gate-only, just the gate. Exit
// codes: 0 done, 1 init failed, 2 usage (bad flag, stray argument, invalid
// value).
func runInit(args []string, cwd string, stdout, stderr io.Writer) int {
	initCmd := flag.NewFlagSet("init", flag.ContinueOnError)
	initCmd.SetOutput(stderr)
	profile := initCmd.String("profile", "cli", "Project profile (v0.1: 'cli')")
	delivery := initCmd.String("delivery", "installed", "Delivery model: 'installed' or 'hosted'")
	forge := initCmd.String("forge", "github", "Forge adapter (v0.1: 'github')")
	combMode := initCmd.String("comb-mode", "direct", "Claim ref mode: 'direct' ('relay' is accepted but not implemented)")
	rt := initCmd.String("runtime", "claude-code", "Local agent runtime ('claude-code' or 'none')")
	force := initCmd.Bool("force", false, "Rewrite managed Waggle files (seeded files are never overwritten)")
	gateOnly := initCmd.Bool("gate-only", false, "Write only the gate workflow, a small config and DECISIONS.md")
	if err := initCmd.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return exitUsage
	}
	if initCmd.NArg() > 0 {
		fmt.Fprintln(stderr, "waggle init takes no arguments")
		return exitUsage
	}
	if err := checkInitChoices(*profile, *delivery, *forge, *combMode); err != nil {
		fmt.Fprintf(stderr, "waggle init failed: %v\n", err)
		return exitUsage
	}

	opts := scaffold.InitOptions{
		TargetDir: cwd,
		Profile:   *profile,
		Delivery:  *delivery,
		Forge:     *forge,
		CombMode:  *combMode,
		Runtime:   *rt,
		Force:     *force,
		GateOnly:  *gateOnly,
		BuildInfo: initBuildInfo,
		Out:       stdout,
	}

	if opts.GateOnly {
		fmt.Fprintln(stdout, "Initializing Waggle gate (gate-only)...")
		if err := scaffold.InitRepo(opts); err != nil {
			fmt.Fprintf(stderr, "waggle init failed: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Gate initialized. Next: commit %s, %s and %s, then require the check verify-leases-and-tiers on your default branch.\n",
			layout.GateWorkflow, layout.Config, layout.Decisions)
		return 0
	}
	fmt.Fprintf(stdout, "Initializing Waggle Hive (profile=%s, delivery=%s, comb=%s, runtime=%s)...\n",
		opts.Profile, opts.Delivery, opts.CombMode, opts.Runtime)
	if err := scaffold.InitRepo(opts); err != nil {
		fmt.Fprintf(stderr, "waggle init failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Hive initialized. Next: edit %s, then commit CLAUDE.md (or AGENTS.md) and .gitignore with the rest.\n", layout.WaggleMD)
	return 0
}
