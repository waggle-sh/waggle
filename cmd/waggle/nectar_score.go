// SPDX-License-Identifier: Apache-2.0

package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/dispatch"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/nectar"
)

const summaryUsage = "Usage: waggle nectar [--repo <dir>] [--keeper <id>] [--now <rfc3339>] [--main <oid>] [--receipts <oid>] [--no-fetch] [--recompute] [--json]\n       waggle nectar explain <task> [--repo <dir>] [--now <rfc3339>] [--main <oid>] [--receipts <oid>] [--no-fetch] [--json]"

const summaryHelp = "Prints every keeper's reputation score from merged tasks that last 30 days on main without a revert, every count of a rule that gave less than full credit, and the exact inputs it was scored at. It fetches release records, then main, unless --no-fetch is given."

// Exit codes of `waggle nectar` (RFC-0005 9.1).
const (
	nectarFinding = 1
	nectarUsageEx = dispatch.ExitUsage
	nectarEnvEx   = dispatch.ExitEnv
)

var (
	// The URL and name patterns copy internal/forge/remote.go, whose parser is unexported.
	githubURLRe = regexp.MustCompile(`^(?:https://github\.com/|git@github\.com:|ssh://git@github\.com/)([^/]+)/([^/]+?)(?:\.git)?$`)
	ownerNameRe = regexp.MustCompile(`^[A-Za-z0-9-]{1,39}$`)
	repoNameRe  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
	nectarOIDRe = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
)

// nectarSeam is a test seam called after the receipt head is read ("receipts")
// and after main is resolved ("main"); it is a no-op outside tests.
var nectarSeam = func(stage string) {}

// nectarRun is the result of resolving the inputs once and scoring them.
type nectarRun struct {
	Report  nectar.Report
	Rewound bool // the remote receipt chain was rewound; local state was scored
	Local   bool // a fetch failed or --no-fetch was given; local state was scored
	Receipt string
}

// githubRepoName returns owner/repo for a GitHub remote URL, else "".
func githubRepoName(raw string) string {
	m := githubURLRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil || !ownerNameRe.MatchString(m[1]) || !repoNameRe.MatchString(m[2]) || m[2] == "." || m[2] == ".." {
		return ""
	}
	return m[1] + "/" + m[2]
}

// nectarFail prints a message to stderr and returns code. Stdout is never written.
func nectarFail(stderr io.Writer, code int, format string, a ...any) int {
	fmt.Fprintf(stderr, "waggle nectar: %s\n", clean(fmt.Sprintf(format, a...)))
	return code
}

// runNectarSummary implements `waggle nectar`. Exit codes: 0 scored, 1 a
// finding (an orphaned or unattributed unit, or a rewound receipt chain), 2
// usage, 3 not a Waggle repo, config invalid, relay mode or any other failure.
// Stdout holds only the report; with --json a failure leaves it empty.
func runNectarSummary(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("nectar", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, summaryUsage)
		fmt.Fprintln(stderr, summaryHelp)
		fs.PrintDefaults()
		fmt.Fprintln(stderr, "Other commands:")
		fmt.Fprintln(stderr, "  "+backfillUsage[len("Usage: "):])
		fmt.Fprintln(stderr, "  "+nectarUsage[strings.Index(nectarUsage, "\n")+1:])
	}
	repo := fs.String("repo", "", "repository directory (default: the current work tree root)")
	keeper := fs.String("keeper", "", "show only this keeper's table row; the counts stay whole")
	nowFlag := fs.String("now", "", "score as of this RFC 3339 time (default: the current time)")
	mainPin := fs.String("main", "", "score at this main commit (full hex id)")
	rcPin := fs.String("receipts", "", "score with the release records up to this commit (full hex id, or all zeros for none)")
	noFetch := fs.Bool("no-fetch", false, "do not fetch; score what this clone holds")
	recompute := fs.Bool("recompute", false, "ignore any cached result")
	asJSON := fs.Bool("json", false, "print the RFC-0005 9.2 object and nothing else")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return nectarUsageEx
	}
	if fs.NArg() > 0 {
		return nectarFail(stderr, nectarUsageEx, "unexpected argument %q\n%s", fs.Arg(0), summaryUsage)
	}
	var now time.Time
	if *nowFlag != "" {
		t, err := time.Parse(time.RFC3339, *nowFlag)
		if err != nil {
			return nectarFail(stderr, nectarUsageEx, "--now %q is not an RFC 3339 time", *nowFlag)
		}
		now = t
	}
	if *keeper != "" {
		if !comb.ValidKeeperID(*keeper) {
			return nectarFail(stderr, nectarUsageEx, "--keeper %q is not a valid keeper id", *keeper)
		}
		if *asJSON {
			return nectarFail(stderr, nectarUsageEx, "--keeper filters the table only; it cannot be used with --json")
		}
	}
	for _, p := range [][2]string{{"--main", *mainPin}, {"--receipts", *rcPin}} {
		if p[1] != "" && !nectarOIDRe.MatchString(p[1]) {
			return nectarFail(stderr, nectarUsageEx, "%s %q is not a full hex object id", p[0], p[1])
		}
	}
	root, err := repoRoot(*repo)
	if err != nil {
		if code, ok := noRepoExit(stderr, "nectar", err); ok {
			return code
		}
		return nectarFail(stderr, nectarEnvEx, "failed: %v", err)
	}
	if now.IsZero() {
		now = statusClock()
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	run, code := nectarScore(ctx, root, *mainPin, *rcPin, now, *noFetch, *recompute, stderr)
	if code != 0 {
		return code
	}
	exit := 0
	if run.Report.Summary.Orphaned > 0 || run.Report.Summary.Unattributed > 0 || run.Rewound {
		exit = nectarFinding
	}
	if *asJSON {
		data, err := nectarJSON(run.Report)
		if err != nil {
			return nectarFail(stderr, nectarEnvEx, "encode: %v", err)
		}
		stdout.Write(data)
		return exit
	}
	renderNectar(stdout, run, *keeper)
	return exit
}

// nectarJSON is the --json output: json.Marshal(report), the RFC-0005 9.2
// object, and one newline.
func nectarJSON(rep nectar.Report) ([]byte, error) {
	data, err := json.Marshal(rep)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// nectarScore resolves the inputs once, in the order of T-052 2.1 item 2, and
// calls Score once. A nonzero code is the exit code; the message is printed.
func nectarScore(ctx context.Context, root, mainPin, rcPin string, now time.Time, noFetch, recompute bool, stderr io.Writer) (nectarRun, int) {
	var run nectarRun
	fail := func(code int, format string, a ...any) (nectarRun, int) {
		return run, nectarFail(stderr, code, format, a...)
	}
	cfgPath := filepath.Join(root, layout.Config)
	if _, err := os.Stat(cfgPath); err != nil {
		return fail(nectarEnvEx, "not a Waggle repo: %s not found; run waggle init", layout.Config)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fail(nectarEnvEx, "%v", err)
	}
	if cfg.Forge.CombMode == comb.CombModeRelay {
		return fail(nectarEnvEx, "%v", comb.ErrRelayNotImplemented)
	}
	opts, err := cfg.CombOptions(root)
	if err != nil {
		return fail(nectarEnvEx, "%v", err)
	}
	remote := opts.Remote
	if noFetch {
		opts.Remote = ""
	}
	opts.Now = statusClock
	backend, err := comb.NewGitBackend(opts)
	if err != nil {
		return fail(nectarEnvEx, "%v", err)
	}
	g := &dispatch.Git{Dir: root, Remote: remote}
	format, err := g.Run(ctx, "rev-parse", "--show-object-format")
	if err != nil {
		return fail(nectarEnvEx, "%v", err)
	}
	oidLen := 40
	if strings.TrimSpace(string(format)) == "sha256" {
		oidLen = 64
	}
	for _, p := range [][2]string{{"--main", mainPin}, {"--receipts", rcPin}} {
		if p[1] != "" && len(p[1]) != oidLen {
			return fail(nectarUsageEx, "%s %q is not a %d digit object id", p[0], p[1], oidLen)
		}
	}
	zero := strings.Repeat("0", oidLen)

	// Receipts first, then main: a receipt is written after its merge, so the
	// main fetched next holds every merged_sha the receipts name.
	fetchMain := remote != "" && !noFetch
	if fetchMain {
		if _, err := backend.ReadReceipts(ctx); err != nil {
			fetchMain = false
			if errors.Is(err, comb.ErrReceiptsRewound) {
				run.Rewound = true
				fmt.Fprintf(stderr, "waggle nectar: warning: %s\n", clean(err.Error()))
			} else {
				run.Local = true
				fmt.Fprintf(stderr, "waggle nectar: warning: could not fetch release records, scoring local state: %s\n", clean(err.Error()))
			}
		}
	}
	// The receipt head is read now, before main is fetched, so a receipt
	// synced later cannot name a merge the resolved main lacks.
	rcOID := rcPin
	if rcOID == "" {
		rcOID = zero
		if out, err := g.Run(ctx, "rev-parse", "--verify", "--quiet", comb.ReceiptRef+"^{commit}"); err == nil {
			rcOID = strings.TrimSpace(string(out))
		}
	} else if rcOID != zero {
		if _, err := g.Run(ctx, "merge-base", "--is-ancestor", rcOID, comb.ReceiptRef); err != nil {
			return fail(nectarEnvEx, "--receipts %s is not on the release record chain %s", rcOID, comb.ReceiptRef)
		}
	}
	nectarSeam("receipts")
	local := ""
	if remote != "" {
		local = "refs/remotes/" + remote + "/main"
	}
	var mainOID string
	if fetchMain {
		mainOID, err = dispatch.ResolveBase(ctx, g, remote, "")
		if err != nil {
			// Neither API marks network errors: if the tracking ref resolves
			// without a fetch, the fetch failed and local state is scored.
			m, lerr := dispatch.ResolveBase(ctx, g, "", local)
			if lerr != nil {
				return fail(nectarEnvEx, "%v", err)
			}
			mainOID, run.Local = m, true
			fmt.Fprintf(stderr, "waggle nectar: warning: could not fetch %s, scoring local state: %s\n", clean(remote), clean(err.Error()))
		}
	} else {
		// --no-fetch, or no remote to fetch from: local state is scored.
		if !run.Rewound {
			run.Local = true
		}
		mainOID, err = dispatch.ResolveBase(ctx, g, "", local)
		if err != nil {
			return fail(nectarEnvEx, "%v", err)
		}
	}
	if mainPin != "" {
		out, err := g.Run(ctx, "rev-parse", "--verify", "--quiet", mainPin+"^{commit}")
		if err != nil {
			return fail(nectarEnvEx, "--main %s is not a commit in this repository", mainPin)
		}
		mainOID = strings.TrimSpace(string(out))
	}
	nectarSeam("main")
	repoName := ""
	if remote != "" {
		if out, err := g.Run(ctx, "remote", "get-url", remote); err == nil {
			repoName = githubRepoName(string(out))
		}
	}
	rep, err := nectar.Score(ctx, nectar.Input{Dir: root, Repo: repoName, Main: mainOID, Receipts: rcOID,
		RulesVersion: nectar.RulesVersion1, Now: now, Recompute: recompute})
	if err != nil {
		return fail(nectarEnvEx, "score at main %s, receipts %s: %v", mainOID, rcOID, err)
	}
	run.Report = rep
	run.Receipt = rcOID
	if strings.Trim(rcOID, "0") == "" {
		run.Receipt = "none"
	}
	return run, 0
}

// milli prints a thousandths value with 3 decimals.
func milli(m nectar.Milli) string { return fmt.Sprintf("%d.%03d", int64(m)/1000, int64(m)%1000) }

// renderNectar writes the table and the four trailing lines. keeper, when set,
// limits the table to that row; the trailing lines stay whole.
func renderNectar(w io.Writer, run nectarRun, keeper string) {
	rep := run.Report
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "keeper_id\tnectar\tmatured\tpending\tclawed_back\tunits")
	rows := 0
	for _, k := range rep.Summary.Keepers {
		if keeper != "" && k.KeeperID != keeper {
			continue
		}
		rows++
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\n", clean(k.KeeperID), milli(k.Nectar), milli(k.Matured), milli(k.Pending), milli(k.ClawedBack), k.Units)
	}
	tw.Flush()
	if rows == 0 {
		fmt.Fprintln(w, "no attributed units")
	}
	c := rep.Counts
	fmt.Fprintf(w, "counts: unbound %d, unattributed %d, orphaned %d, unmerged %d, incomplete %d, third_party %d, fork_mismatch %d, direct_push %d, self_merge %d, carried %d, unconfirmed_revert %d\n",
		c.Unbound, c.Unattributed, c.Orphaned, c.Unmerged, c.Incomplete, c.ThirdParty, c.ForkMismatch, c.DirectPush, c.SelfMerge, c.Carried, c.UnconfirmedRevert)
	ex := make([]string, 0, len(c.Excluded))
	for _, e := range c.Excluded {
		ex = append(ex, clean(e.KeeperID)+" "+milli(e.Weight))
	}
	fmt.Fprintf(w, "excluded: %s\n", joinOrNone(ex))
	rp := make([]string, 0, len(c.ReciprocalPairs))
	for _, p := range c.ReciprocalPairs {
		rp = append(rp, clean(p.A)+" "+clean(p.B)+" "+milli(p.Share))
	}
	fmt.Fprintf(w, "reciprocal pairs: %s\n", joinOrNone(rp))
	line := fmt.Sprintf("scored: main %s, receipts %s, rules_version %d, at %s, repo %s",
		clean(rep.Summary.MainSHA), run.Receipt, rep.Summary.RulesVersion, clean(rep.Summary.ComputedAt), cmp.Or(clean(rep.Summary.Repo), "(none)"))
	switch {
	case run.Rewound:
		line += ", receipts rewound"
	case run.Local:
		line += ", local only"
	}
	fmt.Fprintln(w, line)
}

func joinOrNone(s []string) string {
	if len(s) == 0 {
		return "none"
	}
	return strings.Join(s, ", ")
}
