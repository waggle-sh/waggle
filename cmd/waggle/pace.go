// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"

	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/pace"
)

// readLoad is the load reader; tests replace it.
var readLoad = pace.ReadLoad

// resolvePace reads the owner's pace and the keeper's pick under root. It
// returns owner.Set false when config.yaml has no pace: section.
func resolvePace(root string) (owner pace.Pace, s pace.Settings, warns []pace.Warning, err error) {
	owner, err = config.LoadPace(root)
	if err != nil || !owner.Set {
		return owner, s, nil, err
	}
	pick, err := config.LoadPacePick(root)
	if err != nil {
		return owner, s, nil, err
	}
	s, warns = pace.Resolve(owner, pick)
	return owner, s, warns, nil
}

// paceStatus is the status pace line ("" when no section is set) and the
// warnings to print on stderr.
func paceStatus(root string) (line string, warnings []string) {
	owner, s, warns, err := resolvePace(root)
	switch {
	case err != nil:
		return "pace: invalid: " + err.Error(), nil
	case !owner.Set:
		return "", nil
	}
	for _, w := range warns {
		warnings = append(warnings, "warning: "+w.Message)
	}
	return pace.Line(owner, s), warnings
}

// paceDoctor is the doctor pace finding ("" when no section is set) and the
// warnings to print on stderr. It never affects doctor's exit code.
func paceDoctor(root string) (line string, warnings []string) {
	owner, s, warns, err := resolvePace(root)
	switch {
	case err != nil:
		return "pace: invalid: " + err.Error(), nil
	case !owner.Set:
		return "", nil
	}
	for _, w := range warns {
		warnings = append(warnings, "warning: "+w.Message)
	}
	r, err := readLoad()
	if err != nil {
		r = pace.Unknown()
	}
	return pace.Finding(s, r), warnings
}

// runPace implements `waggle pace`: print the resolved pace and, for each
// agent tool, what applies it. It writes no file and runs no program. Exit
// codes: 0 printed, 1 invalid pace section, 2 usage, 3 repo not found.
func runPace(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pace", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: waggle pace [--repo <dir>] [--tool claude-code|copilot|gas-town]")
		fs.PrintDefaults()
	}
	repo := fs.String("repo", "", "Repository directory (default: the current work tree root)")
	tool := fs.String("tool", "", "Show only this tool: "+strings.Join(pace.Tools, ", "))
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	exit := func(code int, format string, a ...any) int {
		fmt.Fprintf(stderr, "waggle pace "+format+"\n", a...)
		return code
	}
	if fs.NArg() > 0 {
		return exit(2, "unexpected argument %q", fs.Arg(0))
	}
	tools := pace.Tools
	if *tool != "" {
		if !slices.Contains(pace.Tools, *tool) {
			return exit(2, "unknown tool %q; use %s", pace.Printable(*tool), strings.Join(pace.Tools, ", "))
		}
		tools = []string{*tool}
	}
	root, err := repoRoot(*repo)
	if err != nil {
		if code, ok := noRepoExit(stderr, "pace", err); ok {
			return code
		}
		return exit(3, "failed: %v", err)
	}
	if _, err := os.Stat(root + "/" + layout.Config); err != nil {
		return exit(1, "refused: not a Waggle repo: %s not found; run waggle init", layout.Config)
	}
	owner, s, warns, err := resolvePace(root)
	if err != nil {
		return exit(1, "refused: pace: invalid: %v", err)
	}
	if !owner.Set {
		fmt.Fprintf(stdout, "No pace is set. The owner adds a pace: section to %s.\n", layout.Config)
		return 0
	}
	for _, w := range warns {
		fmt.Fprintf(stderr, "warning: %s\n", w.Message)
	}
	fmt.Fprintln(stdout, pace.Line(owner, s))
	for _, t := range tools {
		text, err := pace.Render(t, s, runtime.GOOS)
		if err != nil {
			return exit(2, "%v", err)
		}
		fmt.Fprintf(stdout, "\n%s", text)
	}
	return 0
}
