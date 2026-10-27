// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/waggle-sh/waggle/internal/decisions"
	"github.com/waggle-sh/waggle/internal/layout"
)

// runDecide implements `waggle decide`. It appends exactly one entry from a
// JSON file to the hive decision log. Exit codes: 0 recorded, 1 validation,
// integrity or I/O failure (nothing written), 2 usage error.
func runDecide(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("decide", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: waggle decide --file <path | -> [--repo <dir>] [--dry-run]")
		fmt.Fprintf(stderr, "Append one decision entry from a JSON object to %s.\n", layout.Decisions)
		fmt.Fprintln(stderr, "Entries are never edited or deleted; record a new one that supersedes an old one.")
		fmt.Fprintln(stderr, "The ID and date are always assigned; there is no flag to set them.")
		fmt.Fprintln(stderr, "Who runs it: a maintainer, after the decision is made. Agents never run this")
		fmt.Fprintln(stderr, "command on their own. The command cannot verify approval.")
		fs.PrintDefaults()
	}
	file := fs.String("file", "", "JSON input path, or - for stdin (required)")
	repo := fs.String("repo", ".", "Repository root; must contain a .waggle/ directory")
	dryRun := fs.Bool("dry-run", false, "Validate, assign the next ID, print the entry, write nothing")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "waggle decide: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return 2
	}
	if *file == "" {
		fmt.Fprintln(stderr, "waggle decide: --file is required")
		fs.Usage()
		return 2
	}
	if fi, err := os.Stat(filepath.Join(*repo, layout.Root)); err != nil || !fi.IsDir() {
		fmt.Fprintf(stderr, "waggle decide: %s is not a Waggle repo (no %s directory)\n", *repo, layout.Root)
		return 1
	}

	in, err := readInput(*file, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "waggle decide: %v\n", err)
		return 1
	}
	path := filepath.Join(*repo, filepath.FromSlash(layout.Decisions))
	if *dryRun {
		e, err := decisions.Next(path, in, time.Now())
		if err != nil {
			fmt.Fprintf(stderr, "waggle decide: %v\n", err)
			return 1
		}
		_, _ = stdout.Write(decisions.RenderEntry(e))
		return 0
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(stderr, "waggle decide: %v\n", err)
		return 1
	}
	e, err := decisions.Append(path, in, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "waggle decide: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "recorded D-%03d in %s\n", e.ID, layout.Decisions)
	return 0
}

func readInput(file string, stdin io.Reader) (decisions.Input, error) {
	r := stdin
	if file != "-" {
		f, err := os.Open(file)
		if err != nil {
			return decisions.Input{}, err
		}
		defer f.Close()
		r = f
	}
	return decisions.DecodeInput(r)
}
