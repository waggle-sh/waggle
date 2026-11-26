// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"sort"
)

// version is set by the release build: -ldflags "-X main.version=<tag without v>".
// It feeds the version command only, never waggle.pin.
var version = "0.1.0-dev"

// printVersion writes the version line to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "waggle v%s\n", version)
}

// Exit codes shared by run: 0 done, 1 failed, 2 usage.
const exitUsage = 2

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
}

// runMain dispatches one waggle invocation and returns its exit code. args
// excludes the program name.
func runMain(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) < 1 {
		printUsageTo(stderr)
		return exitUsage
	}
	rest := args[1:]

	switch args[0] {
	case "-h", "--help", "help":
		printUsageTo(stdout)
		return 0
	case "-v", "--version":
		printVersion(stdout)
		return 0
	case "version":
		if len(rest) > 0 {
			fmt.Fprintln(stderr, "waggle version takes no arguments")
			return exitUsage
		}
		printVersion(stdout)
		return 0
	case "init":
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "error resolving working directory: %v\n", err)
			return 1
		}
		return runInit(rest, cwd, stdout, stderr)
	case "decide":
		return runDecide(rest, stdin, stdout, stderr)
	case "gate":
		return runGate(rest, stdout, stderr, getenv)
	case "uninstall":
		return runUninstall(rest, stdout, stderr)
	case "dispatch":
		return runDispatch(rest, stdout, stderr)
	case "claim":
		return runClaim(rest, stdout, stderr)
	case "release":
		return runTaskRelease(rest, stdout, stderr)
	case "force-release":
		return runForceRelease(rest, stdout, stderr)
	case "nectar":
		return runNectar(rest, stdout, stderr)
	case "vote":
		return runVote(rest, stdout, stderr)
	case "quorum":
		return runQuorum(rest, stdout, stderr)
	case "status":
		return runStatus(rest, stdout, stderr)
	case "hives":
		return runHives(rest, stdout, stderr, getenv)
	case "pace":
		return runPace(rest, stdout, stderr)
	case "upgrade":
		return runUpgrade(rest, stdout, stderr)
	case "doctor":
		return runDoctor(rest, stdout, stderr, getenv)
	case "telemetry":
		return runTelemetry(rest, stdout, stderr)
	}
	fmt.Fprintf(stderr, "Unknown subcommand %q\n", args[0])
	if near := nearCommand(args[0]); near != "" {
		fmt.Fprintf(stderr, "did you mean %q?\n", near)
	}
	fmt.Fprintln(stderr)
	printUsageTo(stderr)
	return exitUsage
}

// commandNames lists every subcommand, for "did you mean" hints.
var commandNames = []string{"init", "decide", "gate", "dispatch", "claim", "release", "force-release", "nectar", "vote", "quorum", "uninstall", "status", "hives", "pace", "upgrade", "doctor", "telemetry", "version", "help"}

// nearCommand returns the command closest to word within edit distance 2,
// or "" when none is that close.
func nearCommand(word string) string {
	names := append([]string(nil), commandNames...)
	sort.Strings(names)
	best, bestD := "", 3
	for _, n := range names {
		if d := editDistance(word, n); d < bestD {
			best, bestD = n, d
		}
	}
	return best
}

// editDistance is the Levenshtein distance between a and b.
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

// printUsage writes the usage to stdout.
func printUsage() { printUsageTo(os.Stdout) }

// printUsageTo writes the usage to w.
func printUsageTo(w io.Writer) {
	fmt.Fprintln(w, `waggle - task claims and a merge gate for AI coding agents on Git

Usage:
  waggle <command> [flags]

Commands:
  init      Scaffold .waggle/control/, .waggle/hive/, the gate workflow, and local runtime shims (--gate-only: the gate workflow, a small config and DECISIONS.md)
  decide    Append one decision entry to .waggle/hive/DECISIONS.md from a JSON file (--file)
  gate      Merge-gate checks CI runs against a pull request: verify, decisions, leases, tiers
  dispatch  Claim a ticket, run a worker in a worktree, check its changes against the claim (waggle dispatch <ticket> --help)
  claim     Lease directories for a task: waggle claim <task> --dirs <a,b> --gate light|full [--json]
  release   Give back your claim for a task once its branch has merged [--json]
  force-release  Clear a stuck or malformed claim: inspect first, then run again with --oid and --reason (first login in gate.approvers only)
  nectar    Reputation score from merged tasks that last 30 days on main without a revert: waggle nectar [--json] [explain <task>]; backfill adds missing release records
  vote      Spend your credits on task and issue priority in an open round: waggle vote --round <n> T-101=3
  quorum    Open, show, close and recheck priority rounds weighted by Nectar: waggle quorum open|status|veto|close|verify
  uninstall Print the plan to remove everything init installed; --apply executes it (clean tree required)
  status    Active claims, daemon state and per-ticket token use
  hives     Read-only view across your hives from a registry file: waggle hives status [--registry <file>] [--json]; cross-repo plans: waggle hives plan
  pace      Show the agent pace set for this repo and how to apply it in each agent tool [--tool]
  upgrade   Print the plan to re-render managed files from this build and move waggle.pin; --apply executes it
  doctor    Check that claim refs can be pushed and the gate is a required check [--no-write]
  telemetry Print the exact weekly telemetry report, with no network: waggle telemetry show [--repo <dir>]
  version   Print version`)
}
