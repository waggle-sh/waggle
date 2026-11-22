// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/scaffold"
)

// version is set by the release build: -ldflags "-X main.version=<tag without v>".
// It feeds the version command only, never waggle.pin.
var version = "0.1.0-dev"

// printVersion writes the version line to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "waggle v%s\n", version)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		// Use a dedicated FlagSet for subcommand flags so global args don't pollute `init`.
		initCmd := flag.NewFlagSet("init", flag.ExitOnError)
		profile := initCmd.String("profile", "cli", "Project profile (v0.1: 'cli')")
		delivery := initCmd.String("delivery", "installed", "Delivery model: 'installed' or 'hosted'")
		forge := initCmd.String("forge", "github", "Forge adapter (v0.1: 'github')")
		combMode := initCmd.String("comb-mode", "direct", "Claim ref mode: 'direct' ('relay' is accepted but not implemented)")
		rt := initCmd.String("runtime", "claude-code", "Local agent runtime ('claude-code' or 'none')")
		force := initCmd.Bool("force", false, "Rewrite managed Waggle files (seeded files are never overwritten)")
		gateOnly := initCmd.Bool("gate-only", false, "Write only the gate workflow, a small config and DECISIONS.md")
		_ = initCmd.Parse(os.Args[2:])

		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error resolving working directory: %v\n", err)
			os.Exit(1)
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
		}

		if opts.GateOnly {
			fmt.Println("Initializing Waggle gate (gate-only)...")
			if err := scaffold.InitRepo(opts); err != nil {
				fmt.Fprintf(os.Stderr, "waggle init failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Gate initialized. Next: commit %s, %s and %s, then require the check verify-leases-and-tiers on your default branch.\n",
				layout.GateWorkflow, layout.Config, layout.Decisions)
			break
		}
		fmt.Printf("Initializing Waggle Hive (profile=%s, delivery=%s, comb=%s, runtime=%s)...\n",
			opts.Profile, opts.Delivery, opts.CombMode, opts.Runtime)
		if err := scaffold.InitRepo(opts); err != nil {
			fmt.Fprintf(os.Stderr, "waggle init failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Hive initialized. Next: edit %s, then commit CLAUDE.md (or AGENTS.md) and .gitignore with the rest.\n", layout.WaggleMD)

	case "decide":
		os.Exit(runDecide(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))

	case "gate":
		os.Exit(runGate(os.Args[2:], os.Stdout, os.Stderr, os.Getenv))

	case "uninstall":
		os.Exit(runUninstall(os.Args[2:], os.Stdout, os.Stderr))

	case "dispatch":
		os.Exit(runDispatch(os.Args[2:], os.Stdout, os.Stderr))

	case "claim":
		os.Exit(runClaim(os.Args[2:], os.Stdout, os.Stderr))

	case "release":
		os.Exit(runTaskRelease(os.Args[2:], os.Stdout, os.Stderr))

	case "force-release":
		os.Exit(runForceRelease(os.Args[2:], os.Stdout, os.Stderr))

	case "nectar":
		os.Exit(runNectar(os.Args[2:], os.Stdout, os.Stderr))

	case "status":
		os.Exit(runStatus(os.Args[2:], os.Stdout, os.Stderr))

	case "hives":
		os.Exit(runHives(os.Args[2:], os.Stdout, os.Stderr, os.Getenv))

	case "pace":
		os.Exit(runPace(os.Args[2:], os.Stdout, os.Stderr))

	case "upgrade":
		os.Exit(runUpgrade(os.Args[2:], os.Stdout, os.Stderr))

	case "doctor":
		os.Exit(runDoctor(os.Args[2:], os.Stdout, os.Stderr, os.Getenv))

	case "version":
		printVersion(os.Stdout)

	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand %q\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`waggle - task claims and a merge gate for AI coding agents on Git

Usage:
  waggle <command> [flags]

Commands (v0.1 Bootstrap):
  init      Scaffold .waggle/control/, .waggle/hive/, the gate workflow, and local runtime shims (--gate-only: the gate workflow, a small config and DECISIONS.md)
  decide    Append one decision entry to .waggle/hive/DECISIONS.md from a JSON file (--file)
  gate      Merge-gate checks CI runs against a pull request: verify, decisions, leases, tiers
  dispatch  Claim a ticket, run a worker in a worktree, check its changes against the claim (waggle dispatch <ticket> --help)
  claim     Lease directories for a task: waggle claim <task> --dirs <a,b> --gate light|full [--json]
  release   Give back your claim for a task once its branch has merged [--json]
  force-release  Clear a stuck or malformed claim: inspect first, then run again with --oid and --reason (first login in gate.approvers only)
  nectar    Reputation score from merged tasks that last 30 days on main without a revert: waggle nectar [--json] [explain <task>]; backfill adds missing release records
  uninstall Print the plan to remove everything init installed; --apply executes it (clean tree required)
  status    Active claims, daemon state and per-ticket token use
  hives     Read-only view across your hives from a registry file: waggle hives status [--registry <file>] [--json]; cross-repo plans: waggle hives plan
  pace      Show the agent pace set for this repo and how to apply it in each agent tool [--tool]
  upgrade   Print the plan to re-render managed files from this build and move waggle.pin; --apply executes it
  doctor    Check that claim refs can be pushed and the gate is a required check [--no-write]
  version   Print version`)
}
