// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/waggle-sh/waggle/internal/scaffold"
)

// upgradeBuildInfo is the build stamp reader; tests inject a pinned one.
var upgradeBuildInfo = debug.ReadBuildInfo

// runUpgrade implements `waggle upgrade`. It prints the plan to re-render the
// managed files from this build and move waggle.pin, and, with --apply,
// executes it. Exit codes: 0 ok (dry run or applied), 1 refused (a
// precondition failed, nothing changed), 2 usage, 3 git or I/O failure.
func runUpgrade(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: waggle upgrade [--apply] [--diff] [--allow-downgrade] [--runtime=claude-code|none]")
		fmt.Fprintln(stderr, "Print the plan to re-render managed files from this build and move waggle.pin; --apply executes it.")
		fs.PrintDefaults()
	}
	apply := fs.Bool("apply", false, "Execute the plan (default: dry run, print it and change nothing)")
	diff := fs.Bool("diff", false, "Print a unified diff under every [rewrite] line")
	allowDowngrade := fs.Bool("allow-downgrade", false, "Permit a release tag lower than the current pin")
	rt := fs.String("runtime", "claude-code", "Local agent runtime whose shims to regenerate ('claude-code' or 'none')")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "waggle upgrade: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return 2
	}
	if *rt != "claude-code" && *rt != "none" {
		fmt.Fprintf(stderr, "waggle upgrade: unsupported --runtime=%q (supported: 'claude-code', 'none')\n", *rt)
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "waggle upgrade: resolving working directory: %v\n", err)
		return 3
	}
	err = scaffold.Upgrade(scaffold.UpgradeOptions{
		Dir: cwd, Apply: *apply, Diff: *diff, AllowDowngrade: *allowDowngrade, Runtime: *rt, Out: stdout,
		BuildInfo: upgradeBuildInfo,
	})
	switch {
	case err == nil:
		return 0
	case errors.Is(err, scaffold.ErrRefused):
		fmt.Fprintf(stderr, "waggle upgrade refused: %v\n", err)
		return 1
	default:
		fmt.Fprintf(stderr, "waggle upgrade failed: %v\n", err)
		return 3
	}
}
