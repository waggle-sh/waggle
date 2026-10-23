package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/signal"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/waggle-sh/waggle/internal/dispatch"
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

func usageError(format string, args ...any) error {
	return &dispatch.Error{Code: dispatch.ExitUsage, Err: fmt.Errorf(format, args...)}
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
		return fs.Arg(0), fs.NArg() - 1, 0, true
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
		fmt.Fprintln(stderr, "Usage: waggle claim <task> --dirs <a,b> --gate light|full [--ttl <duration>] [--owner <id>] [--base <rev>] [--repo <dir>] [--json]")
	}
	dirs, gate := fs.String("dirs", "", "comma-separated directories to lease"), fs.String("gate", "", "light or full")
	ttl, owner := fs.Duration("ttl", 0, "claim lifetime in whole minutes; default leases.default_ttl_minutes"), fs.String("owner", "", "claim owner; default keeper_id in .waggle/local.yaml")
	base, repo := fs.String("base", "", "revision whose config lists protected paths; default main on the remote"), fs.String("repo", ".", "repository")
	asJSON := fs.Bool("json", false, "print one JSON object")
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
	switch {
	case task == "" || extra > 0:
		return o.fail("", usageError("usage: waggle claim <task> --dirs <a,b> --gate light|full"))
	case len(list) == 0 || slices.Contains(list, ""):
		return o.fail("", usageError("--dirs is required: a comma-separated list with no empty entry"))
	case ttlSet && *ttl == 0:
		return o.fail("", usageError("ttl 0s: must be a whole number of minutes, at least 1"))
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	res, err := dispatch.ClaimTask(ctx, dispatch.ClaimTaskRequest{RepoDir: *repo, Ticket: task, Dirs: list, Gate: *gate, TTL: *ttl, Owner: *owner, Base: *base})
	if err != nil {
		return o.fail(res.Task, err)
	}
	l := res.Lease
	o.warn(res.Warnings)
	if *asJSON {
		o.json(claimOKJSON{jsonVersion, "claim", true, res.Task, res.Outcome,
			claimBodyJSON{l.TicketID, l.KeeperID, l.Gate, nonNil(l.Directories), rfc3339(l.ClaimedAt), rfc3339(l.ExpiresAt), res.Ref, res.OID},
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

// runTaskRelease implements `waggle release <task>`: the same release as
// waggle dispatch release, with --owner and JSON output.
func runTaskRelease(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: waggle release <task> [--branch <name>] [--unmerged] [--owner <id>] [--repo <dir>] [--json]")
	}
	branch, repo := fs.String("branch", "", "branch, t-<nnn> or waggle/T-<nnn> with an optional suffix"), fs.String("repo", ".", "repository")
	unmerged, owner := fs.Bool("unmerged", false, "release without the merged check (a task that makes no PR)"), fs.String("owner", "", "claim owner; default keeper_id in .waggle/local.yaml")
	asJSON := fs.Bool("json", false, "print one JSON object")
	task, extra, code, ok := parseTaskArgs(fs, args)
	if !ok {
		return code
	}
	o := taskOut{"release", *asJSON, stdout, stderr}
	if task == "" || extra > 0 {
		return o.fail("", usageError("usage: waggle release <task>"))
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	res, err := dispatch.ReleaseTask(ctx, dispatch.ReleaseRequest{RepoDir: *repo, Ticket: task, Branch: *branch, Unmerged: *unmerged, Owner: *owner})
	o.warn(res.Warnings)
	if err != nil {
		return o.fail(res.Task, err)
	}
	if *asJSON {
		o.json(releaseOKJSON{jsonVersion, "release", true, res.Task, res.Outcome, res.OID, res.Merged, res.Branch, nonNil(res.Warnings)})
		return 0
	}
	fmt.Fprintln(stdout, clean(dispatch.ReleaseLine(res)))
	return 0
}
