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
	"github.com/waggle-sh/waggle/internal/forge"
	"github.com/waggle-sh/waggle/internal/layout"
)

// Injection points for tests, which never run in parallel in this package.
var (
	newForge    = forge.New
	probeClaims = forge.ProbeClaimRefs
)

const doctorTimeout = 2 * time.Minute

// runDoctor implements `waggle doctor`: can this clone push claim refs, and
// is the gate a required check with up-to-date branches. Exit codes: 0 every
// required finding is ok or skipped, 1 a required finding is missing or
// unknown, 2 usage, 3 config, forge or default branch could not be read.
func runDoctor(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, "Usage: waggle doctor [--repo <dir>] [--no-write]"); fs.PrintDefaults() }
	repo := fs.String("repo", "", "Repository directory (default: the current work tree root)")
	noWrite := fs.Bool("no-write", false, "Do not push test refs to the remote")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	exit := func(code int, format string, a ...any) int {
		fmt.Fprintf(stderr, "waggle doctor "+format+"\n", a...)
		return code
	}
	if fs.NArg() > 0 {
		return exit(2, "unexpected argument %q", fs.Arg(0))
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return exit(3, "failed: %v", err)
	}
	cfg, err := config.Load(filepath.Join(root, layout.Config))
	if err != nil {
		return exit(3, "failed: %v", err)
	}
	if line, warns := paceDoctor(root); line != "" {
		fmt.Fprintln(stdout, line)
		for _, w := range warns {
			fmt.Fprintln(stderr, w)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), doctorTimeout)
	defer cancel()
	cf, cline := claimsFinding(ctx, root, cfg)
	if cfg.Forge.Remote == "" {
		fmt.Fprintln(stdout, "Local-only repo: no forge checks. After adding a remote and setting forge.remote, run waggle doctor.")
		printClaims(stdout, cf, cline)
		return 0
	}
	f, err := newForge(cfg.Forge.Provider, forge.Options{RepoDir: root, Remote: cfg.Forge.Remote, Getenv: getenv})
	if err != nil {
		return exit(3, "failed: %v", err)
	}
	report, err := f.Probe(ctx, forge.ProbeOptions{Write: !*noWrite, MergeStrategy: cfg.Forge.MergeStrategy,
		GateCheck: forge.DefaultGateCheck})
	if err != nil {
		return exit(3, "failed: %v", err)
	}
	claims := probeClaims(ctx, root, cfg.Forge.Remote, !*noWrite, stdout)
	report.Findings = append([]forge.Finding{claims}, report.Findings...)
	if err := forge.Render(stdout, report); err != nil {
		return exit(3, "failed: %v", err)
	}
	printClaims(stdout, cf, cline)
	for _, fd := range report.Findings {
		if fd.Required && (fd.Status == forge.StatusMissing || fd.Status == forge.StatusUnknown) {
			return 1
		}
	}
	return 0
}
