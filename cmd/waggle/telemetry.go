// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/telemetry"
)

const telemetryUsage = "Usage: waggle telemetry show [--repo <dir>]"

// runTelemetry implements `waggle telemetry`. Only show exists today; send
// ships with T-108. Exit codes: 0 done, 1 refused (bad config), 2 usage, 3 failed (including not a git repo).
func runTelemetry(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "show" {
		fmt.Fprintln(stderr, telemetryUsage)
		return exitUsage
	}
	fs := flag.NewFlagSet("telemetry show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, telemetryUsage)
		fmt.Fprintln(stderr, "Prints the exact JSON this week's telemetry report would hold. It needs no network, writes nothing and works with telemetry off. A field with no local source is left out.")
		fs.PrintDefaults()
	}
	repo := fs.String("repo", ".", "repository")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return exitUsage
	}
	if fs.NArg() > 0 {
		fs.Usage()
		return exitUsage
	}
	root, err := repoRoot(*repo)
	if err != nil {
		if code, ok := noRepoExit(stderr, "telemetry", err); ok {
			return code
		}
		fmt.Fprintf(stderr, "waggle telemetry failed: %v\n", err)
		return 3
	}
	cfg, err := config.Load(filepath.Join(root, layout.Config))
	if err != nil {
		fmt.Fprintf(stderr, "waggle telemetry refused: %v\n", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p, notes, err := telemetry.Collect(ctx, root, cfg, version, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "waggle telemetry failed: %v\n", err)
		return 3
	}
	out, err := p.JSON()
	if err != nil {
		fmt.Fprintf(stderr, "waggle telemetry failed: %v\n", err)
		return 3
	}
	stdout.Write(out)
	for _, n := range notes {
		fmt.Fprintln(stderr, n)
	}
	return 0
}
