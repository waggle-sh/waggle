// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/dispatch"
	"github.com/waggle-sh/waggle/internal/layout"
)

// jsonVersion is the schema version of the JSON that waggle claim and waggle
// release print. Fields are only ever added.
const jsonVersion = 1

type taskErrorJSON struct {
	Code    string `json:"code"`
	Exit    int    `json:"exit"`
	Message string `json:"message"`
}

type taskFailJSON struct {
	Version int           `json:"version"`
	Command string        `json:"command"`
	OK      bool          `json:"ok"`
	Task    string        `json:"task"`
	Error   taskErrorJSON `json:"error"`
}

type claimBodyJSON struct {
	Task        string   `json:"task"`
	Owner       string   `json:"owner"`
	Gate        string   `json:"gate"`
	Directories []string `json:"directories"`
	ClaimedAt   string   `json:"claimed_at"`
	ExpiresAt   string   `json:"expires_at"`
	Ref         string   `json:"ref"`
	OID         string   `json:"oid"`
	Branches    []string `json:"branches"`
}

// repeatFlag collects every use of a repeatable string flag.
type repeatFlag []string

func (r *repeatFlag) String() string { return strings.Join(*r, ",") }

func (r *repeatFlag) Set(v string) error {
	*r = append(*r, v)
	return nil
}

type claimOKJSON struct {
	Version  int           `json:"version"`
	Command  string        `json:"command"`
	OK       bool          `json:"ok"`
	Task     string        `json:"task"`
	Outcome  string        `json:"outcome"`
	Claim    claimBodyJSON `json:"claim"`
	Notes    []string      `json:"notes"`
	Warnings []string      `json:"warnings"`
}

type releaseOKJSON struct {
	Version  int      `json:"version"`
	Command  string   `json:"command"`
	OK       bool     `json:"ok"`
	Task     string   `json:"task"`
	Outcome  string   `json:"outcome"`
	OID      string   `json:"oid"`
	Merged   bool     `json:"merged"`
	Branch   string   `json:"branch"`
	Receipt  string   `json:"receipt"`
	Warnings []string `json:"warnings"`
}

// taskOut prints the result of one waggle claim or waggle release run: text, or
// with --json exactly one JSON object on stdout.
type taskOut struct {
	command        string
	asJSON         bool
	stdout, stderr io.Writer
}

// clean drops control and format characters and the line and paragraph
// separators, so a value from a ref, the remote or config cannot move the
// cursor or start a new line.
func clean(s string) string {
	return strings.ToValidUTF8(strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) {
			return -1
		}
		return r
	}, s), "")
}

func (o taskOut) json(v any) {
	data, err := json.Marshal(v)
	if err != nil { // the structs above always encode
		data = []byte(`{"version":1,"ok":false}`)
	}
	fmt.Fprintf(o.stdout, "%s\n", data)
}

// fail prints err and returns the exit code. Human text always goes to stderr.
func (o taskOut) fail(task string, err error) int {
	code := dispatch.ExitCode(err)
	if o.asJSON {
		o.json(taskFailJSON{jsonVersion, o.command, false, task, taskErrorJSON{dispatch.ErrorCode(err), code, err.Error()}})
	}
	fmt.Fprintf(o.stderr, "waggle %s: %s\n", o.command, clean(err.Error()))
	return code
}

func (o taskOut) warn(ws []string) {
	for _, w := range ws {
		fmt.Fprintf(o.stderr, "waggle %s: warning: %s\n", o.command, clean(w))
	}
}

// missingRemoteLine returns the one-line message for cmd when err is a
// dispatch.MissingRemoteError (forge.remote names a remote the repo lacks).
// claim and release print it through taskOut.fail; doctor prints it directly.
func missingRemoteLine(cmd string, err error) (string, bool) {
	var mr *dispatch.MissingRemoteError
	if !errors.As(err, &mr) {
		return "", false
	}
	return "waggle " + cmd + ": " + clean(mr.Error()), true
}

// checkMissingRemote returns the missing-remote error when forge.remote names a
// remote the repo lacks, so release prints one line before the comb fetch can
// fail with git's own text. Any other problem (no config, not a repo) is left
// to ReleaseTask to report.
func checkMissingRemote(ctx context.Context, repo string) error {
	root, err := repoRoot(repo)
	if err != nil {
		return nil
	}
	cfg, err := config.Load(filepath.Join(root, layout.Config))
	if err != nil {
		return nil
	}
	err = dispatch.CheckRemote(ctx, &dispatch.Git{Dir: root}, cfg.Forge.Remote)
	var mr *dispatch.MissingRemoteError
	if errors.As(err, &mr) {
		return err
	}
	return nil
}

func usageError(format string, args ...any) error {
	return &dispatch.Error{Code: dispatch.ExitUsage, Err: fmt.Errorf(format, args...)}
}

// maxClaimBranches is the most --branch values a claim accepts.
const maxClaimBranches = 4

// errHint rewords a failure so it names the fix: a bad task id gets the accepted
// form, and a missing owner gets both ways out. The error keeps its code and exit.
func errHint(err error) error {
	if errors.Is(err, comb.ErrInvalidTicketID) {
		return fmt.Errorf("%w; use T-<digits>, for example T-90", err)
	}
	var de *dispatch.Error
	if !errors.As(err, &de) || de.Code != dispatch.ExitUsage {
		return err
	}
	msg, ok := strings.CutSuffix(err.Error(), "; or pass --owner <id>")
	if !ok {
		return err
	}
	return usageError("%s; pass --owner <your GitHub login> or create .waggle/control/local.yaml with keeper_id: <your GitHub login>", msg)
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// parseTaskArgs parses a flag set whose task argument comes before or after the
// flags. ok is false when the flags did not parse (usage is printed, code is 2)
// or help was asked for (code 0). extra counts the arguments after the task.
func parseTaskArgs(fs *flag.FlagSet, args []string) (task string, extra int, code int, ok bool) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		task, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return "", 0, 0, false
		}
		return "", 0, dispatch.ExitUsage, false
	}
	if task == "" && fs.NArg() > 0 {
		// flag stops at the first non-flag: take it as the task, parse the rest again.
		task, args = fs.Arg(0), fs.Args()[1:]
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return "", 0, 0, false
			}
			return "", 0, dispatch.ExitUsage, false
		}
	}
	return task, fs.NArg(), 0, true
}

// runClaim implements `waggle claim <task>`: it writes a claim for any human or
// orchestrator. Exit codes: 0 done, 1 interrupted, 2 usage, 3 environment, 4
// refused.
func runClaim(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("claim", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: waggle claim <task> --dirs <a,b> --gate light|full [--branch <name>]... [--ttl <duration>] [--owner <id>] [--base <rev>] [--repo <dir>] [--json]")
		fs.PrintDefaults()
	}
	dirs, gate := fs.String("dirs", "", "comma-separated directories to lease"), fs.String("gate", "", "light or full")
	ttl, owner := fs.Duration("ttl", 0, "claim lifetime in whole minutes; default leases.default_ttl_minutes"), fs.String("owner", "", "claim owner, usually your GitHub login; required without .waggle/local.yaml, else defaults to its keeper_id")
	base, repo := fs.String("base", "", "revision whose config lists protected paths; default main on the remote"), fs.String("repo", ".", "repository")
	asJSON := fs.Bool("json", false, "print one JSON object")
	var branches repeatFlag
	fs.Var(&branches, "branch", "PR branch this claim accepts; repeatable, up to 4")
	task, extra, code, ok := parseTaskArgs(fs, args)
	if !ok {
		return code
	}
	o := taskOut{"claim", *asJSON, stdout, stderr}
	ttlSet := false
	fs.Visit(func(f *flag.Flag) { ttlSet = ttlSet || f.Name == "ttl" })
	var list []string
	if *dirs != "" {
		list = strings.Split(*dirs, ",")
	}
	id, _ := comb.ParseTicketID(task)
	switch {
	case task == "" || extra > 0:
		return o.fail(id, usageError("usage: waggle claim <task> --dirs <a,b> --gate light|full [--branch <name>]..."))
	case len(list) == 0 || slices.Contains(list, ""):
		return o.fail(id, usageError("--dirs is required: a comma-separated list with no empty entry"))
	case *gate == "":
		return o.fail(id, usageError("--gate is required: light or full"))
	case *gate != "light" && *gate != "full":
		return o.fail(id, usageError("--gate %q: must be light or full", *gate))
	case len(branches) > maxClaimBranches:
		return o.fail(id, usageError("--branch given %d times; at most %d", len(branches), maxClaimBranches))
	case ttlSet && *ttl == 0:
		return o.fail(id, usageError("ttl 0s: must be a whole number of minutes, at least 1"))
	}
	if err := checkRepo(*repo); err != nil {
		return o.fail(id, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	res, err := dispatch.ClaimTask(ctx, dispatch.ClaimTaskRequest{RepoDir: *repo, Ticket: task, Dirs: list, Gate: *gate, Branches: branches, TTL: *ttl, Owner: *owner, Base: *base})
	if err != nil {
		return o.fail(res.Task, errHint(err))
	}
	l := res.Lease
	o.warn(res.Warnings)
	if *asJSON {
		o.json(claimOKJSON{jsonVersion, "claim", true, res.Task, res.Outcome,
			claimBodyJSON{l.TicketID, l.KeeperID, l.Gate, nonNil(l.Directories), rfc3339(l.ClaimedAt), rfc3339(l.ExpiresAt), res.Ref, res.OID, nonNil(l.Branches)},
			nonNil(res.Notes), nonNil(res.Warnings)})
		return 0
	}
	ds := make([]string, len(l.Directories))
	for i, d := range l.Directories {
		ds[i] = clean(d)
	}
	claim := ""
	if res.OID != "" {
		claim = ", claim " + clean(res.OID[:min(len(res.OID), 12)])
	}
	fmt.Fprintf(stdout, "%s %s for %s until %s: %s (gate %s%s)\n", strings.ReplaceAll(res.Outcome, "_", " "), res.Task, clean(l.KeeperID),
		rfc3339(l.ExpiresAt), strings.Join(ds, ", "), clean(l.Gate), claim)
	for _, n := range res.Notes {
		fmt.Fprintf(stdout, "note: %s\n", clean(n))
	}
	return 0
}

// runTaskRelease implements `waggle release <task>`: it gives back your claim
// once the ticket's branch has merged, with --owner and JSON output.
func runTaskRelease(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: waggle release <task> [--branch <name>] [--unmerged] [--co <id>]... [--owner <id>] [--repo <dir>] [--json]")
		fs.PrintDefaults()
	}
	var co repeatFlag
	fs.Var(&co, "co", "another keeper who shares the credit with you in the release record; repeatable, up to 4")
	branch, repo := fs.String("branch", "", "branch, t-<nnn> or waggle/T-<nnn> with an optional suffix"), fs.String("repo", ".", "repository")
	unmerged, owner := fs.Bool("unmerged", false, "release without the merged check (a task that makes no PR)"), fs.String("owner", "", "claim owner, usually your GitHub login; required without .waggle/local.yaml, else defaults to its keeper_id")
	asJSON := fs.Bool("json", false, "print one JSON object")
	task, extra, code, ok := parseTaskArgs(fs, args)
	if !ok {
		return code
	}
	o := taskOut{"release", *asJSON, stdout, stderr}
	if task == "" || extra > 0 {
		id, _ := comb.ParseTicketID(task)
		return o.fail(id, usageError("usage: waggle release <task>"))
	}
	if err := checkRepo(*repo); err != nil {
		id, _ := comb.ParseTicketID(task)
		return o.fail(id, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	if err := checkMissingRemote(ctx, *repo); err != nil {
		return o.fail(task, err)
	}
	res, err := dispatch.ReleaseTask(ctx, dispatch.ReleaseRequest{RepoDir: *repo, Ticket: task, Branch: *branch, Unmerged: *unmerged, Owner: *owner, CoKeepers: co})
	o.warn(res.Warnings)
	if err != nil {
		return o.fail(res.Task, errHint(err))
	}
	if *asJSON {
		o.json(releaseOKJSON{jsonVersion, "release", true, res.Task, res.Outcome, res.OID, res.Merged, res.Branch, res.Receipt, nonNil(res.Warnings)})
		return 0
	}
	line := dispatch.ReleaseLine(res)
	if ref, err := comb.RefName(res.Task); err == nil && res.Outcome == "no_claim" {
		line += ": no claim exists, " + ref + " was not found"
	}
	fmt.Fprintln(stdout, clean(line))
	return 0
}
