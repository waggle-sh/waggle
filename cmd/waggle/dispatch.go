package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/dispatch"
)

// listFlag collects a repeatable flag.
type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

// stopSignals end a run like Ctrl-C: SIGHUP is a closed terminal.
var stopSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}

// runDispatch implements `waggle dispatch <ticket>` (T-023 section 4). Exit
// codes: 0 done, 1 run failed, 2 usage, 3 environment, 4 claim refused, 5 needs
// the Keeper. There is no flag that skips a check, lowers a model or forces.
func runDispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "clean" {
		return runClean(args[1:], stdout, stderr)
	}
	fs := flag.NewFlagSet("dispatch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: waggle dispatch <ticket> --role builder|forager --dir <path>... --gate light|full --no-cell")
		fmt.Fprintln(stderr, "       waggle dispatch <ticket> --role guard --branch <branch> --no-cell [--second] [--base <rev>] [--tier 0-3] [--json]")
		fmt.Fprintln(stderr, "       waggle dispatch clean <ticket> [--role builder|forager|guard] [--runs] [--repo <dir>]")
		fmt.Fprintln(stderr, "       waggle dispatch clean --host-record [--accept <digest>] [--repo <dir>]")
		fmt.Fprintln(stderr, "       [--branch <b>] [--base <rev>] [--tier 0-3] [--read <path>...] [--timeout 120m] [--no-push] [--keep-worktree] [--json] [--repo <dir>]")
	}
	var dirs, reads listFlag
	role, gate := fs.String("role", "", "builder, forager or guard"), fs.String("gate", "", "light or full")
	second := fs.Bool("second", false, "guard: a model different from the builder's and the first guard's")
	branch, base, repo := fs.String("branch", "", "branch, default t-<nnn>"), fs.String("base", "", "base revision"), fs.String("repo", ".", "repository")
	tier, timeout := fs.Int("tier", -1, "risk tier 0 to 3"), fs.Duration("timeout", 120*time.Minute, "session timeout")
	noCell, noPush := fs.Bool("no-cell", false, "run on the host without a Cell (required)"), fs.Bool("no-push", false, "do not push the branch")
	keep, asJSON := fs.Bool("keep-worktree", false, "keep the worktree after success"), fs.Bool("json", false, "print only result.json")
	fs.Var(&dirs, "dir", "leased directory (repeatable)")
	fs.Var(&reads, "read", "read-only context path (repeatable)")
	ticket := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		ticket, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return dispatch.ExitUsage
	}
	if ticket == "" && fs.NArg() == 1 {
		ticket = fs.Arg(0)
	} else if ticket == "" || fs.NArg() > 0 {
		fs.Usage()
		return dispatch.ExitUsage
	}
	if !*noCell {
		fmt.Fprintln(stderr, "waggle dispatch: --no-cell is required: no Cell runtime exists yet, so the worker runs as your own user. Not enforced:")
		for _, s := range dispatch.NotEnforced {
			fmt.Fprintln(stderr, "  - "+s)
		}
		return dispatch.ExitUsage
	}
	req := dispatch.Request{RepoDir: *repo, Ticket: ticket, Role: *role, Dirs: dirs, Gate: *gate, Branch: *branch, Base: *base, ReadPaths: reads, Second: *second}
	if *tier != -1 {
		t := comb.RiskTier(*tier)
		req.Tier = &t
	}
	d := dispatch.Deps{Cell: dispatch.HostCell(), Timeout: *timeout, NoPush: *noPush, KeepWorktree: *keep}
	if !*asJSON {
		d.OnEvent = func(e dispatch.Event) { fmt.Fprintf(stdout, "%s: %s\n", e.Kind, e.Msg) }
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	res, err := dispatch.Run(ctx, req, d)
	if err != nil {
		fmt.Fprintf(stderr, "waggle dispatch: %v\n", err)
		return dispatch.ExitCode(err)
	}
	if res.Error != "" {
		fmt.Fprintf(stderr, "waggle dispatch: %s\n", res.Error)
	}
	if *asJSON {
		_, _ = stdout.Write(res.JSON)
		return res.ExitCode
	}
	if res.Verdict != "" {
		fmt.Fprintf(stdout, "verdict: %s (head %s)\n", res.Verdict, res.HeadOID)
	}
	for _, v := range res.Violations {
		fmt.Fprintf(stdout, "violation: %s %s: %s\n", v.Rule, v.Path, v.Detail)
	}
	for _, w := range append(res.Warnings, res.NotEnforced...) {
		fmt.Fprintf(stdout, "note: %s\n", w)
	}
	fmt.Fprintf(stdout, "%s %s ended %s (exit %d), record %s\n", res.Ticket, res.Role, res.End, res.ExitCode, res.RunDir)
	return res.ExitCode
}

// runClean implements `waggle dispatch clean <ticket>` (T-023 10.7.3 items 5
// and 7). It never deletes a branch or touches a claim.
func runClean(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dispatch clean", flag.ContinueOnError)
	fs.SetOutput(stderr)
	role, repo := fs.String("role", "", "builder, forager or guard; default every role"), fs.String("repo", ".", "repository")
	runs := fs.Bool("runs", false, "also remove this ticket's run records")
	hostRec, accept := fs.Bool("host-record", false, "list the host files that differ from the record"), fs.String("accept", "", "with --host-record: record the state this digest was listed for")
	ticket := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		ticket, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return dispatch.ExitUsage
	}
	accepting := false
	fs.Visit(func(f *flag.Flag) { accepting = accepting || f.Name == "accept" })
	if ticket == "" && fs.NArg() == 1 && !*hostRec {
		ticket = fs.Arg(0)
	} else if (ticket == "") != *hostRec || fs.NArg() > 0 || accepting && !*hostRec || *hostRec && (*role != "" || *runs) {
		fmt.Fprintln(stderr, "Usage: waggle dispatch clean <ticket> [--role builder|forager|guard] [--runs] [--repo <dir>]")
		fmt.Fprintln(stderr, "       waggle dispatch clean --host-record [--accept <digest>] [--repo <dir>]")
		return dispatch.ExitUsage
	}
	var err error
	if *hostRec {
		err = dispatch.HostRecord(context.Background(), dispatch.HostRecordRequest{RepoDir: *repo, Accept: *accept, Accepting: accepting}, stdout)
	} else {
		err = dispatch.Clean(context.Background(), dispatch.CleanRequest{RepoDir: *repo, Ticket: ticket, Role: *role, Runs: *runs}, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "waggle dispatch: %v\n", err)
	}
	return dispatch.ExitCode(err)
}
