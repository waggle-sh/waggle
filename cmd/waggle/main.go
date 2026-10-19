package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/scaffold"
)

const version = "0.1.0-bootstrap"

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
		combMode := initCmd.String("comb-mode", "direct", "Comb ref mode: 'direct' or 'relay'")
		rt := initCmd.String("runtime", "claude-code", "Local Keeper agent runtime ('claude-code' or 'none')")
		force := initCmd.Bool("force", false, "Rewrite managed Waggle files (seeded files are never overwritten)")
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

	case "status":
		os.Exit(runStatus(os.Args[2:], os.Stdout, os.Stderr))

	case "upgrade":
		os.Exit(runUpgrade(os.Args[2:], os.Stdout, os.Stderr))

	case "version":
		fmt.Printf("waggle v%s\n", version)

	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand %q\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`waggle - Distributed AI Swarm Coordination on Git

Usage:
  waggle <command> [flags]

Commands (v0.1 Bootstrap):
  init      Scaffold .waggle/control/, .waggle/hive/, the gate workflow, and local runtime shims
  decide    Append one decision entry to .waggle/hive/DECISIONS.md from a JSON file (--file)
  gate      Merge-gate checks CI runs against a pull request: verify, decisions, leases, tiers
  dispatch  Claim a ticket, run a worker in a worktree, check its changes against the claim (waggle dispatch <ticket> --help)
  uninstall Print the plan to remove everything init installed; --apply executes it (clean tree required)
  status    Active claims, waggled state, and per-ticket token use
  upgrade   Print the plan to re-render managed files from this build and move waggle.pin; --apply executes it
  version   Print version`)
}
