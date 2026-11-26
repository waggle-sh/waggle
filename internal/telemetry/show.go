// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
)

const showUsage = "Usage: waggle telemetry show [--repo <dir>]"

// Exit codes of RunShow.
const (
	exitRefused = 1 // bad config
	exitUsage   = 2
	exitFailed  = 3 // includes not a git repository
)

// RunShow implements `waggle telemetry <args>`. Only show exists today; send
// ships with T-108. repoDir is the default for --repo. It prints the payload
// JSON on stdout and notes on stderr, and returns the exit code: 0 done,
// 1 refused (bad config), 2 usage, 3 failed (including not a git repository).
func RunShow(args []string, repoDir, version string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "show" {
		fmt.Fprintln(stderr, showUsage)
		return exitUsage
	}
	fs := flag.NewFlagSet("telemetry show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, showUsage)
		fmt.Fprintln(stderr, "Prints the exact JSON this week's telemetry report would hold. It needs no network, writes nothing and works with telemetry off. A field with no local source is left out.")
		fs.PrintDefaults()
	}
	repo := fs.String("repo", cmp.Or(repoDir, "."), "repository")
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
	root, notRepo, err := topLevel(*repo)
	if err != nil {
		if notRepo {
			fmt.Fprintln(stderr, "waggle telemetry: "+cleanLine(err.Error()))
		} else {
			fmt.Fprintf(stderr, "waggle telemetry failed: %v\n", err)
		}
		return exitFailed
	}
	cfg, err := config.Load(filepath.Join(root, layout.Config))
	if err != nil {
		fmt.Fprintf(stderr, "waggle telemetry refused: %v\n", err)
		return exitRefused
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p, notes, err := Collect(ctx, root, cfg, version, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "waggle telemetry failed: %v\n", err)
		return exitFailed
	}
	out, err := p.JSON()
	if err != nil {
		fmt.Fprintf(stderr, "waggle telemetry failed: %v\n", err)
		return exitFailed
	}
	stdout.Write(out)
	for _, n := range notes {
		fmt.Fprintln(stderr, n)
	}
	return 0
}

// topLevel returns the work tree root of dir. notRepo is true when dir is not
// inside a git work tree, and err then holds the one-line message.
func topLevel(dir string) (root string, notRepo bool, err error) {
	dir = cmp.Or(dir, ".")
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	if runErr != nil {
		s := stderr.String()
		if strings.Contains(s, "not a git repository") || strings.Contains(s, "cannot change to") {
			return "", true, fmt.Errorf("%s is not a git repository (run it inside your repo or pass --repo %s)", dir, dir)
		}
		return "", false, fmt.Errorf("git rev-parse --show-toplevel in %s: %v: %s", dir, runErr, strings.TrimSpace(s))
	}
	return strings.TrimSpace(string(out)), false, nil
}

// cleanLine drops control characters so a hostile --repo cannot rewrite the
// terminal.
func cleanLine(s string) string {
	return strings.ToValidUTF8(strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) {
			return -1
		}
		return r
	}, s), "")
}
