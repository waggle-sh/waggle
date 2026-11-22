// SPDX-License-Identifier: Apache-2.0

package main

import (
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
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/dispatch"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/nectar"
	"github.com/waggle-sh/waggle/internal/quorum"
)

const quorumUsage = `Usage: waggle vote --round <n> <item>=<votes>... [--dry-run] [--key <path>] [--now <rfc3339>] [--repo <dir>] [--json]
       waggle quorum open (--items T-101,issue-7 | --from-roadmap) [--closes <rfc3339>] [--key <path>] [--now <rfc3339>] [--repo <dir>] [--json]
       waggle quorum status [--round <n>] [--next] [--now <rfc3339>] [--repo <dir>] [--json]
       waggle quorum veto --round <n> --item <id|all> --decision D-<n> [--key <path>] [--repo <dir>]
       waggle quorum close --round <n> [--key <path>] [--repo <dir>] [--json]
       waggle quorum verify --round <n> [--receipt <oid>] [--repo <dir>] [--json]`

const quorumHelp = `A round lists tasks and issues. Each keeper spends credits on them; n votes on one item cost n squared credits.
An item's score adds each voter's votes times the square root of that voter's Nectar, fixed when the round opened.
Ballots and records are signed with an SSH key listed in .waggle/control/keepers on main, through ssh-keygen.
Exit codes: 0 done, 1 a check or validation failed, 2 usage, 3 git, signing tool or file error.
--key defaults to git config user.signingkey when gpg.format is ssh. keeper_id comes from .waggle/local.yaml.`

// scoreSource scores Nectar through internal/nectar at the round's pinned inputs
// and returns the RFC-0005 9.2 bytes.
type scoreSource struct{ dir string }

func (s scoreSource) Snapshot(ctx context.Context, at quorum.NectarAt) ([]byte, error) {
	now, err := time.Parse(time.RFC3339, at.Now)
	if err != nil {
		return nil, fmt.Errorf("round nectar_at.now is not a time: %w", err)
	}
	rep, err := nectar.Score(ctx, nectar.Input{Dir: s.dir, Main: at.MainSHA, Receipts: at.ReceiptsSHA,
		RulesVersion: at.RulesVersion, Now: now})
	if err != nil {
		return nil, err
	}
	return nectarJSON(rep)
}

// quorumRun is one command's view of the repository: the backend, the lifecycle
// and the resolved main head that rounds open and close at.
type quorumRun struct {
	root    string
	backend *comb.GitBackend
	git     *dispatch.Git
	life    quorum.Lifecycle
	main    string
}

func qUsage(format string, a ...any) error { return usageError(format, a...) }

// quorumCode maps an error to the exit code: 2 usage, 1 a failed check or
// validation, 3 anything else.
func quorumCode(err error) int {
	var de *dispatch.Error
	var re *quorum.ReasonError
	if errors.As(err, &de) && de.Code == dispatch.ExitUsage {
		return dispatch.ExitUsage
	}
	checks := []error{quorum.ErrNoRound, quorum.ErrClosed, quorum.ErrNotClosed, quorum.ErrNotApprover, quorum.ErrNotHeadKeeper,
		quorum.ErrKeyNotRegistered, quorum.ErrBadRound, quorum.ErrBadRequest, quorum.ErrTallyMismatch, quorum.ErrUnknownVersion,
		quorum.ErrItem, quorum.ErrBadSignature, quorum.ErrRegistry, quorum.ErrNectar, quorum.ErrOverflow, errQuorumRefused}
	for _, c := range checks {
		if errors.Is(err, c) {
			return 1
		}
	}
	if errors.As(err, &re) {
		return 1
	}
	return dispatch.ExitEnv
}

var errQuorumRefused = errors.New("refused")

func quorumFail(stderr io.Writer, name string, err error) int {
	fmt.Fprintf(stderr, "waggle %s: %s\n", name, clean(err.Error()))
	return quorumCode(err)
}

// quorumFlags is a flag set with the options the quorum commands share.
type quorumFlags struct {
	fs     *flag.FlagSet
	repo   *string
	key    *string
	now    *string
	asJSON *bool
}

func newQuorumFlags(name string, stderr io.Writer, withKey, withNow, withJSON bool) quorumFlags {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, quorumUsage)
		fmt.Fprintln(stderr, quorumHelp)
		fs.PrintDefaults()
	}
	q := quorumFlags{fs: fs, repo: fs.String("repo", ".", "repository"), key: new(string), now: new(string), asJSON: new(bool)}
	if withKey {
		q.key = fs.String("key", "", "SSH private key file to sign with (default: git config user.signingkey when gpg.format is ssh)")
	}
	if withNow {
		q.now = fs.String("now", "", "act as of this RFC 3339 time (for tests and replay)")
	}
	if withJSON {
		q.asJSON = fs.Bool("json", false, "print one JSON object and nothing else on stdout")
	}
	return q
}

// parse reads flags and positional arguments in any order. done is true when
// the command is finished (help was asked for, or the flags were bad).
func (q quorumFlags) parse(args []string) (pos []string, code int, done bool) {
	for {
		if err := q.fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, 0, true
			}
			return nil, dispatch.ExitUsage, true
		}
		if q.fs.NArg() == 0 {
			return pos, 0, false
		}
		pos, args = append(pos, q.fs.Arg(0)), q.fs.Args()[1:]
	}
}

func (q quorumFlags) clock() (time.Time, error) {
	if *q.now == "" {
		return statusClock(), nil
	}
	t, err := time.Parse(time.RFC3339, *q.now)
	if err != nil {
		return t, qUsage("--now is not an RFC 3339 time")
	}
	return t, nil
}

func printJSON(w io.Writer, v any) {
	data, _ := json.Marshal(v)
	fmt.Fprintf(w, "%s\n", data)
}

// openQuorum resolves the repository, fetches release records and main when
// there is a remote, and builds the lifecycle.
func openQuorum(ctx context.Context, repo string, stderr io.Writer) (*quorumRun, error) {
	root, err := repoRoot(repo)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(filepath.Join(root, layout.Config))
	if err != nil {
		return nil, err
	}
	if cfg.Forge.CombMode == comb.CombModeRelay {
		return nil, comb.ErrRelayNotImplemented
	}
	opts, err := cfg.CombOptions(root)
	if err != nil {
		return nil, err
	}
	opts.Now = statusClock
	backend, err := comb.NewGitBackend(opts)
	if err != nil {
		return nil, err
	}
	r := &quorumRun{root: root, backend: backend, git: &dispatch.Git{Dir: root, Remote: opts.Remote}}
	if opts.Remote != "" {
		if _, err := backend.ReadReceipts(ctx); err != nil {
			fmt.Fprintf(stderr, "waggle quorum: warning: release records: %s\n", clean(err.Error()))
		}
	}
	if r.main, err = dispatch.ResolveBase(ctx, r.git, opts.Remote, ""); err != nil {
		return nil, err
	}
	r.life = quorum.Lifecycle{Comb: backend, RepoDir: root, Nectar: scoreSource{dir: root}}
	return r, nil
}

func (r *quorumRun) gitConfig(ctx context.Context, key string) string {
	out, err := r.git.Run(ctx, "config", "--get", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// keyPath returns the signing key file: --key, else git config user.signingkey
// when gpg.format is ssh, else a usage error.
func (r *quorumRun) keyPath(ctx context.Context, key string) (string, error) {
	if key == "" {
		if r.gitConfig(ctx, "gpg.format") != "ssh" {
			return "", qUsage("no --key given and git config gpg.format is not ssh")
		}
		if key = r.gitConfig(ctx, "user.signingkey"); key == "" {
			return "", qUsage("no --key given and git config user.signingkey is empty")
		}
	}
	if strings.HasPrefix(key, "key::") || strings.HasPrefix(key, "ssh-") {
		return "", qUsage("the signing key is a literal key, not a file; give --key <path>")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(key, "~/") {
		key = filepath.Join(home, key[2:])
	}
	return key, nil
}

func (r *quorumRun) actor(ctx context.Context, key string) (quorum.Actor, error) {
	path, err := r.keyPath(ctx, key)
	if err != nil {
		return quorum.Actor{}, err
	}
	id, err := config.LoadKeeperID(r.root)
	if err != nil {
		return quorum.Actor{}, err
	}
	return quorum.Actor{KeeperID: id, Signer: quorum.Signer{KeyPath: path}}, nil
}

// latestRound is the highest round number in the local copy of the namespace.
func (r *quorumRun) latestRound(ctx context.Context) (int, error) {
	if err := r.backend.FetchObjectNamespace(ctx, quorum.RefPrefix); err != nil {
		return 0, err
	}
	refs, err := r.backend.ListObjectRefs(ctx, quorum.RefPrefix)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, ref := range refs {
		if _, rn, _ := quorum.ParseRef(ref.Ref); rn > n {
			n = rn
		}
	}
	if n == 0 {
		return 0, fmt.Errorf("%w: there are no rounds yet", quorum.ErrNoRound)
	}
	return n, nil
}

// merged reports whether a first-parent commit on main names the task, in a
// subject such as "T-101: ..." or "Merge pull request #1 from org/t-101-x".
func (r *quorumRun) merged(ctx context.Context) func(string) bool {
	out, _ := r.git.Run(ctx, "log", "--first-parent", "--format=%s", r.main)
	subjects := string(out)
	return func(task string) bool {
		n := strings.TrimLeft(strings.TrimPrefix(task, "T-"), "0")
		return regexp.MustCompile(`(?im)(^|[^a-z0-9])t-0*` + n + `([^0-9]|$)`).MatchString(subjects)
	}
}

func qMilli(u uint64) string { return fmt.Sprintf("%d.%03d", u/1000, u%1000) }

func short(oid string) string { return oid[:min(len(oid), 12)] }

// runQuorum implements `waggle quorum <open|status|veto|close|verify>`.
func runQuorum(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(stderr, quorumUsage)
		fmt.Fprintln(stderr, quorumHelp)
		if len(args) > 0 {
			return 0
		}
		return dispatch.ExitUsage
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	switch args[0] {
	case "open":
		return quorumOpen(ctx, args[1:], stdout, stderr)
	case "status":
		return quorumStatus(ctx, args[1:], stdout, stderr)
	case "veto":
		return quorumVeto(ctx, args[1:], stdout, stderr)
	case "close":
		return quorumClose(ctx, args[1:], stdout, stderr)
	case "verify":
		return quorumVerify(ctx, args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "waggle quorum: unknown subcommand %q\n%s\n", clean(args[0]), quorumUsage)
	return dispatch.ExitUsage
}

func quorumOpen(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	q := newQuorumFlags("quorum open", stderr, true, true, true)
	items := q.fs.String("items", "", "comma-separated items, such as T-101,issue-7")
	roadmap := q.fs.Bool("from-roadmap", false, "use the T- lines under the Backlog heading of ROADMAP.md on main")
	closes := q.fs.String("closes", "", "RFC 3339 close time (default: now plus quorum.round_days)")
	pos, code, done := q.parse(args)
	if done {
		return code
	}
	if len(pos) > 0 || (*items == "") == !*roadmap {
		return quorumFail(stderr, "quorum open", qUsage("give exactly one of --items and --from-roadmap"))
	}
	now, err := q.clock()
	if err != nil {
		return quorumFail(stderr, "quorum open", err)
	}
	var closesAt time.Time
	if *closes != "" {
		if closesAt, err = time.Parse(time.RFC3339, *closes); err != nil {
			return quorumFail(stderr, "quorum open", qUsage("--closes is not an RFC 3339 time"))
		}
	}
	r, err := openQuorum(ctx, *q.repo, stderr)
	if err != nil {
		return quorumFail(stderr, "quorum open", err)
	}
	actor, err := r.actor(ctx, *q.key)
	if err != nil {
		return quorumFail(stderr, "quorum open", err)
	}
	if closesAt.IsZero() {
		data, err := r.backend.ReadBlobAt(ctx, r.main, layout.Config, config.MaxFileSize)
		if err != nil {
			return quorumFail(stderr, "quorum open", err)
		}
		bc, err := config.Parse(data)
		if err != nil {
			return quorumFail(stderr, "quorum open", err)
		}
		closesAt = now.Add(time.Duration(bc.Quorum.RoundDays) * 24 * time.Hour)
	}
	req := quorum.OpenRequest{FromRoadmap: *roadmap, Now: now, ClosesAt: closesAt, Base: r.main, MainSHA: r.main}
	if *items != "" {
		req.Items = strings.Split(*items, ",")
	}
	rd, oid, err := r.life.Open(ctx, req, actor)
	if err != nil {
		return quorumFail(stderr, "quorum open", err)
	}
	if *q.asJSON {
		printJSON(stdout, map[string]any{"version": 1, "round": rd, "oid": oid})
		return 0
	}
	fmt.Fprintf(stdout, "opened round %d with %d items, %d credits each, closes %s, record %s\n", rd.Round, len(rd.Items), rd.Credits, clean(rd.ClosesAt), short(oid))
	return 0
}

// Vote is a top-level command: waggle vote --round <n> T-101=8 ...
func runVote(args []string, stdout, stderr io.Writer) int {
	q := newQuorumFlags("vote", stderr, true, true, true)
	round := q.fs.Int("round", 0, "round number")
	dry := q.fs.Bool("dry-run", false, "check the votes and print their cost; write nothing")
	pos, code, done := q.parse(args)
	if done {
		return code
	}
	if *round < 1 || len(pos) == 0 {
		return quorumFail(stderr, "vote", qUsage("usage: waggle vote --round <n> <item>=<votes>..."))
	}
	var votes []quorum.Vote
	for _, p := range pos {
		item, num, ok := strings.Cut(p, "=")
		n, err := strconv.ParseInt(num, 10, 64)
		if !ok || err != nil {
			return quorumFail(stderr, "vote", qUsage("%q is not in the form <item>=<votes>", clean(p)))
		}
		votes = append(votes, quorum.Vote{Item: item, Votes: n})
	}
	now, err := q.clock()
	if err != nil {
		return quorumFail(stderr, "vote", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	r, err := openQuorum(ctx, *q.repo, stderr)
	if err != nil {
		return quorumFail(stderr, "vote", err)
	}
	id, err := config.LoadKeeperID(r.root)
	if err != nil {
		return quorumFail(stderr, "vote", err)
	}
	rd, err := r.life.LoadRound(ctx, *round)
	if err != nil {
		return quorumFail(stderr, "vote", err)
	}
	if t, err := time.Parse(time.RFC3339, rd.ClosesAt); err != nil || !now.Before(t) {
		return quorumFail(stderr, "vote", fmt.Errorf("%w: round %d closed at %s", errQuorumRefused, rd.Round, clean(rd.ClosesAt)))
	}
	b, err := quorum.CheckVotes(rd, id, votes)
	if err != nil {
		return quorumFail(stderr, "vote", err)
	}
	receipt := ""
	if !*dry {
		key, err := r.keyPath(ctx, *q.key)
		if err != nil {
			return quorumFail(stderr, "vote", err)
		}
		s := quorum.Store{Comb: r.backend, Signer: quorum.Signer{KeyPath: key}}
		if receipt, err = s.Cast(ctx, rd, id, votes); err != nil {
			return quorumFail(stderr, "vote", err)
		}
	}
	if *q.asJSON {
		printJSON(stdout, map[string]any{"version": 1, "round": rd.Round, "keeper_id": id, "credits_used": b.CreditsUsed,
			"credits": rd.Credits, "dry_run": *dry, "receipt": receipt})
		return 0
	}
	if *dry {
		fmt.Fprintf(stdout, "dry run: these votes cost %d of %d credits; nothing was written\n", b.CreditsUsed, rd.Credits)
		return 0
	}
	fmt.Fprintf(stdout, "receipt %s\ncost %d of %d credits\n", receipt, b.CreditsUsed, rd.Credits)
	return 0
}

type qCredit struct {
	KeeperID string `json:"keeper_id"`
	Used     uint64 `json:"used"`
	Left     uint64 `json:"left"`
}

func quorumStatus(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	q := newQuorumFlags("quorum status", stderr, false, true, true)
	round := q.fs.Int("round", 0, "round number (default: the latest round)")
	next := q.fs.Bool("next", false, "print the ready list of a closed round: not vetoed, not claimed, dependencies merged")
	pos, code, done := q.parse(args)
	if done {
		return code
	}
	if len(pos) > 0 || *round < 0 {
		return quorumFail(stderr, "quorum status", qUsage("usage: waggle quorum status [--round <n>] [--next]"))
	}
	now, err := q.clock()
	if err != nil {
		return quorumFail(stderr, "quorum status", err)
	}
	r, err := openQuorum(ctx, *q.repo, stderr)
	if err != nil {
		return quorumFail(stderr, "quorum status", err)
	}
	if *round == 0 {
		if *round, err = r.latestRound(ctx); err != nil {
			return quorumFail(stderr, "quorum status", err)
		}
	}
	rep, err := r.life.Status(ctx, *round, r.main)
	if err != nil {
		return quorumFail(stderr, "quorum status", err)
	}
	if *next {
		return quorumNext(ctx, r, rep, *q.asJSON, stdout, stderr)
	}
	credits := make([]qCredit, 0, len(rep.Credits))
	for _, c := range rep.Credits {
		credits = append(credits, qCredit{c.KeeperID, c.Used, c.Left})
	}
	if *q.asJSON {
		printJSON(stdout, map[string]any{"version": 1, "round": rep.Round, "closed": rep.Closed, "tally": rep.Tally, "credits": credits})
		return 0
	}
	state := "open"
	if rep.Closed {
		state = "closed"
	} else if t, err := time.Parse(time.RFC3339, rep.Round.ClosesAt); err == nil && !now.Before(t) {
		state = "open, past its close time"
	}
	fmt.Fprintf(stdout, "round %d: %s, closes %s, %d credits each\n", rep.Round.Round, state, clean(rep.Round.ClosesAt), rep.Round.Credits)
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "item\tscore\traw votes\tvetoed")
	for _, s := range rep.Tally.Items {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%v\n", clean(s.Item), qMilli(s.ScoreMilli), s.RawVotes, s.Vetoed)
	}
	tw.Flush()
	fmt.Fprintf(stdout, "order: %s\n", clean(strings.Join(rep.Tally.Order, ", ")))
	if rep.Tally.Rejected {
		fmt.Fprintln(stdout, "rejected: a veto of all items; the previous order stands")
	}
	for _, v := range rep.Tally.Vetoes {
		fmt.Fprintf(stdout, "veto: %s under %s\n", clean(v.Item), clean(v.Decision))
	}
	for _, v := range rep.Tally.Ignored {
		fmt.Fprintf(stdout, "ignored veto: %s under %s (%s)\n", clean(v.Item), clean(v.Decision), clean(v.Reason))
	}
	for _, c := range credits {
		fmt.Fprintf(stdout, "credits: %s used %d, left %d\n", clean(c.KeeperID), c.Used, c.Left)
	}
	for _, m := range rep.Tally.Malformed {
		fmt.Fprintf(stdout, "malformed: %s (%s)\n", clean(m.Ref), clean(m.Reason))
	}
	return 0
}

func quorumNext(ctx context.Context, r *quorumRun, rep quorum.StatusReport, asJSON bool, stdout, stderr io.Writer) int {
	if !rep.Closed {
		return quorumFail(stderr, "quorum status", quorum.ErrNotClosed)
	}
	ready, err := quorum.Ready(ctx, r.backend, rep.Round, rep.Tally, r.merged(ctx))
	if err != nil {
		return quorumFail(stderr, "quorum status", err)
	}
	if asJSON {
		printJSON(stdout, map[string]any{"version": 1, "round": rep.Round.Round, "ready": ready})
		return 0
	}
	deps := map[string]string{}
	for _, d := range rep.Round.Depends {
		deps[d.Item] = " (after " + strings.Join(d.On, ", ") + ")"
	}
	for _, it := range ready {
		fmt.Fprintf(stdout, "%s%s\n", clean(it), clean(deps[it]))
	}
	return 0
}

func quorumVeto(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	q := newQuorumFlags("quorum veto", stderr, true, false, false)
	round := q.fs.Int("round", 0, "round number")
	item := q.fs.String("item", "", "item to veto, or all")
	decision := q.fs.String("decision", "", "decision entry that records the veto, such as D-001, three digits or more")
	pos, code, done := q.parse(args)
	if done {
		return code
	}
	if len(pos) > 0 || *round < 1 || *item == "" || *decision == "" {
		return quorumFail(stderr, "quorum veto", qUsage("usage: waggle quorum veto --round <n> --item <id|all> --decision D-<n>"))
	}
	r, err := openQuorum(ctx, *q.repo, stderr)
	if err != nil {
		return quorumFail(stderr, "quorum veto", err)
	}
	actor, err := r.actor(ctx, *q.key)
	if err != nil {
		return quorumFail(stderr, "quorum veto", err)
	}
	oid, err := r.life.Veto(ctx, *round, *item, *decision, actor)
	if err != nil {
		return quorumFail(stderr, "quorum veto", err)
	}
	fmt.Fprintf(stdout, "veto recorded in round %d, record %s\n", *round, short(oid))
	return 0
}

func quorumClose(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	q := newQuorumFlags("quorum close", stderr, true, false, true)
	round := q.fs.Int("round", 0, "round number")
	pos, code, done := q.parse(args)
	if done {
		return code
	}
	if len(pos) > 0 || *round < 1 {
		return quorumFail(stderr, "quorum close", qUsage("usage: waggle quorum close --round <n>"))
	}
	r, err := openQuorum(ctx, *q.repo, stderr)
	if err != nil {
		return quorumFail(stderr, "quorum close", err)
	}
	rd, err := r.life.LoadRound(ctx, *round)
	if err != nil {
		return quorumFail(stderr, "quorum close", err)
	}
	// The library has no clock; the close time is checked here.
	if t, err := time.Parse(time.RFC3339, rd.ClosesAt); err != nil || statusClock().Before(t) {
		return quorumFail(stderr, "quorum close", fmt.Errorf("%w: round %d closes at %s", errQuorumRefused, rd.Round, clean(rd.ClosesAt)))
	}
	actor, err := r.actor(ctx, *q.key)
	if err != nil {
		return quorumFail(stderr, "quorum close", err)
	}
	t, oid, err := r.life.Close(ctx, *round, r.main, actor)
	if err != nil {
		return quorumFail(stderr, "quorum close", err)
	}
	if *q.asJSON {
		printJSON(stdout, map[string]any{"version": 1, "round": *round, "oid": oid, "tally": t})
		return 0
	}
	fmt.Fprintf(stdout, "closed round %d, tally %s\norder: %s\n", *round, oid, clean(strings.Join(t.Order, ", ")))
	return 0
}

func quorumVerify(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	q := newQuorumFlags("quorum verify", stderr, false, false, true)
	round := q.fs.Int("round", 0, "round number")
	receipt := q.fs.String("receipt", "", "ballot commit id to look up in the tally")
	pos, code, done := q.parse(args)
	if done {
		return code
	}
	if len(pos) > 0 || *round < 1 {
		return quorumFail(stderr, "quorum verify", qUsage("usage: waggle quorum verify --round <n> [--receipt <oid>]"))
	}
	r, err := openQuorum(ctx, *q.repo, stderr)
	if err != nil {
		return quorumFail(stderr, "quorum verify", err)
	}
	res, err := r.life.Verify(ctx, *round, *receipt)
	if err != nil {
		return quorumFail(stderr, "quorum verify", err)
	}
	if *q.asJSON {
		printJSON(stdout, map[string]any{"version": 1, "round": *round, "verified": true, "receipt": res.Receipt})
		return 0
	}
	fmt.Fprintf(stdout, "verified: the tally of round %d matches a recomputation byte for byte\n", *round)
	if *receipt != "" {
		fmt.Fprintf(stdout, "receipt %s: %s\n", short(*receipt), clean(res.Receipt))
	}
	return 0
}
