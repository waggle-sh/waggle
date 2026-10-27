// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/waggle-sh/waggle/internal/scaffold"
)

// runUninstall implements `waggle uninstall`. It prints the removal plan and,
// with --apply, executes it. Exit codes: 0 ok (dry run or applied), 1 refused
// (a precondition failed, nothing changed), 2 usage, 3 git or I/O failure.
func runUninstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: waggle uninstall [--apply]")
		fmt.Fprintln(stderr, "Print the plan to remove everything waggle init installed; --apply executes it.")
		fs.PrintDefaults()
	}
	apply := fs.Bool("apply", false, "Execute the plan (default: dry run, print it and change nothing)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "waggle uninstall: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "waggle uninstall: resolving working directory: %v\n", err)
		return 3
	}
	err = scaffold.Uninstall(scaffold.UninstallOptions{Dir: cwd, Apply: *apply, Out: stdout})
	switch {
	case err == nil:
		return 0
	case errors.Is(err, scaffold.ErrRefused):
		fmt.Fprintf(stderr, "waggle uninstall refused: %v\n", err)
		return 1
	default:
		fmt.Fprintf(stderr, "waggle uninstall failed: %v\n", err)
		return 3
	}
}
