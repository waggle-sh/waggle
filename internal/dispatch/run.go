package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/runtime"
)

// Event is one progress line: claimed, worktree, launched, ended, checked,
// pushed or recorded.
type Event struct{ Kind, Msg string }

// Deps are the seams Run takes from its caller; zero values mean the real thing.
type Deps struct {
	Backend        ClaimBackend         // nil: built from config
	Cell           CellRuntime          // required; the CLI passes HostCell()
	Runtime        runtime.AgentRuntime // nil: the configured Claude Code adapter
	GitBin         string
	Now            func() time.Time // stamps the result; nil means time.Now
	OnEvent        func(Event)
	Timeout        time.Duration // 0 means 120 minutes
	KeepaliveEvery time.Duration // 0 means half the claim TTL
	NoPush         bool
	KeepWorktree   bool
}

func (d Deps) emit(kind, format string, args ...any) {
	if d.OnEvent != nil {
		d.OnEvent(Event{kind, fmt.Sprintf(format, args...)})
	}
}

// seq runs steps in order and stops at the first error. A non-*Error error
// exits 3 (ExitCode), so steps return other packages' errors as they are.
func seq(steps ...func() error) error {
	for _, s := range steps {
		if err := s(); err != nil {
			return err
		}
	}
	return nil
}

type loaded struct {
	cfg     *config.Config
	policy  runtime.Policy
	context runtime.ContextPolicy
	keeper  string
	adapter runtime.AdapterConfig
}

func load(repo string) (l loaded, err error) {
	path := filepath.Join(repo, filepath.FromSlash(layout.Config))
	var data []byte
	var rts *config.Runtimes
	err = seq(
		func() (e error) { l.cfg, e = config.Load(path); return },
		func() (e error) { l.policy, e = config.LoadPolicy(repo); return },
		func() (e error) { l.context, e = config.LoadContext(repo); return },
		func() (e error) { l.keeper, e = config.LoadKeeperID(repo); return },
		func() (e error) { data, e = os.ReadFile(path); return },
		func() (e error) { rts, e = config.ParseRuntimes(data); return })
	if err == nil {
		l.adapter = rts.Adapters[l.policy.Models.Adapter].Config
	}
	return l, err
}

// refuseDirs refuses, before any fetch, a directory that equals, contains or
// sits inside a Tier 3 path or .waggle/, which the Comb would refuse at the claim.
func refuseDirs(dirs []string) error {
	for _, d := range dirs {
		for _, p := range comb.HardcodedTier3Prefixes {
			if comb.DirsOverlap(strings.ToLower(d), strings.ToLower(strings.TrimSuffix(p, "/"))) {
				return failf(ExitClaim, "directory %q: Tier 3 paths and .waggle/ are never leased", d)
			}
		}
	}
	return nil
}

// offBranch refuses a base on the existing branch (section 4): the check base
// merge-base(branch, base) must be an ancestor of the default base, or the
// earlier branch commits would fall outside the checked range. It returns the
// branch point, merge-base(branch, base), which is baseOID for a new branch.
func offBranch(ctx context.Context, g *Git, req Request, baseOID string) (string, error) {
	rev, ok := g.branchRev(ctx, req.Branch)
	if !ok {
		return baseOID, nil
	}
	def, name := baseOID, "main"
	if g.Remote != "" {
		name = g.Remote + "/main"
	}
	var err error
	if req.Base != "" {
		def, err = ResolveBase(ctx, g, "", name)
	}
	mb, merr := g.Run(ctx, "merge-base", rev, baseOID)
	if err = errors.Join(err, merr); err != nil {
		return "", err
	}
	point := strings.TrimSpace(string(mb))
	_, err = g.Run(ctx, "merge-base", "--is-ancestor", point, def)
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return "", failf(ExitUsage, "base is on the branch; earlier branch commits would skip the check")
	}
	return point, err
}

// withBranchPoint adds the Branch point line after the Base line when the
// branch forks somewhere other than the base. Both OIDs are ones dispatch
// resolved itself.
func withBranchPoint(lines []string, baseOID, point string) []string {
	if point == baseOID {
		return lines
	}
	i := slices.Index(lines, "Base: "+baseOID)
	if i < 0 {
		return lines
	}
	return slices.Insert(slices.Clone(lines), i+1, "Branch point: "+point+" (this branch was not rebased onto Base; Base is where the brief and role were read from)")
}

// cutOf names the limit or timeout that ended the session, empty when none did.
func cutOf(s session) string {
	switch {
	case s.TimedOut:
		return "timeout"
	case s.Subtype == "error_max_budget_usd":
		return "max_budget"
	case s.End == runtime.EndMaxTurns || s.End == runtime.EndMaxTokens:
		return s.End
	}
	return ""
}

// shortOID is the first 7 characters of an OID.
func shortOID(oid string) string {
	return oid[:min(len(oid), 7)]
}

// waggleCommands returns the commands of the WAGGLE.md section titled "Build
// and Verification Commands" (the first backtick span of each bullet) that the
// worker may run through Bash. An unfilled placeholder such as <build command>
// is skipped. The section is found by its title, so renumbering is harmless.
func waggleCommands(md []byte) (cmds []string) {
	in := false
	for _, line := range strings.Split(string(md), "\n") {
		if strings.HasPrefix(line, "## ") {
			in = strings.Contains(line, "Build and Verification Commands")
		} else if _, rest, ok := strings.Cut(line, "`"); ok && in && strings.HasPrefix(line, "- ") {
			if cmd, _, ok := strings.Cut(rest, "`"); ok && !strings.HasPrefix(cmd, "<") {
				cmds = append(cmds, cmd)
			}
		}
	}
	return cmds
}

// combEnv is the Comb's extra environment. The agent controls repo config, so
// the claim writes run no hooks and no auto maintenance. The Keeper's own
// GIT_CONFIG_COUNT entries are replaced (the last value of a variable wins in
// os/exec), and GIT_CONFIG_PARAMETERS is emptied so a parent's cannot add keys.
var combEnv = []string{"GIT_CONFIG_PARAMETERS=", "GIT_CONFIG_COUNT=3", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0=/dev/null",
	"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false", "GIT_CONFIG_KEY_2=gc.auto", "GIT_CONFIG_VALUE_2=0"}

// Run is the dispatch sequence of T-023 section 5 for a builder or forager. It
// returns an *Error before a run record exists. After that it returns the
// Result with its ExitCode, whatever happened to the session: the checks of 7.2
// always run after the agent and nothing is pushed unless all pass. Run
// returns only after the worker's process group is gone and every goroutine it
// started has exited.
func Run(ctx context.Context, req Request, d Deps) (res Result, err error) {
	if req, err = req.Validate(); err != nil {
		return res, err
	}
	switch {
	case d.Cell == nil:
		return res, failf(ExitEnv, "no cell runtime; the CLI passes the host cell")
	}
	if err = refuseDirs(req.Dirs); err != nil {
		return res, err
	}
	var l loaded
	if req.RepoDir, err = filepath.Abs(req.RepoDir); err == nil {
		l, err = load(req.RepoDir)
	}
	if err != nil {
		return res, err
	}
	now := time.Now
	if d.Now != nil {
		now = d.Now
	}
	g := &Git{Bin: d.GitBin, Dir: req.RepoDir, Remote: l.cfg.Forge.Remote}
	pg := g.pinned()
	rt := d.Runtime
	if rt == nil {
		a := l.adapter
		a.DefaultArgs = slices.Clone(a.DefaultArgs) // dispatch keeps no reference
		rt = runtime.NewClaudeAdapter(a)
	}
	// Verify runs exactly once per adapter, before any Prepare. Every runtime
	// refusal (unverified binary, refused argument, bad config) exits 3.
	// Everything that can refuse a run comes before the first write (item 4).
	var id runtime.GitIdentity
	var dir, common string
	if err = seq(
		func() error { return rt.Verify(ctx) },
		func() error { return ownGroup(new(exec.Cmd)) },
		func() error { return checkIgnored(ctx, pg) },
		func() (e error) { id, e = resolveIdentity(ctx, g); return },
		func() (e error) { dir, e = worktreesDir(req.RepoDir); return }); err != nil {
		return res, err
	}
	lock := filepath.Join(dir, "dispatch.lock")
	lockWarns, err := takeLock(lock, time.Now())
	if err != nil {
		return res, failf(ExitEnv, "another dispatch run is in progress in this repo: %w", err)
	}
	defer os.Remove(lock)
	wtPath := filepath.Join(dir, req.Ticket+"-"+req.Role)
	if err = (&Worktree{Path: wtPath}).taintedErr(); err != nil {
		return res, err
	}
	out, err := pg.Run(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if common = strings.TrimSpace(string(out)); err == nil {
		err = checkHostRecord(ctx, pg, dir, common) // before ResolveBase's fetch, the first unpinned call
	}
	if err != nil {
		return res, err
	}
	guard := req.Role == "guard"
	ownRef, _ := comb.RefName(req.Ticket)
	var (
		baseOID, runDir  string
		point            string
		in               Inputs
		cmds             []string
		want             = comb.Lease{TicketID: req.Ticket, Directories: req.Dirs, Gate: req.Gate}
		task             = &runtime.TaskContext{TaskID: req.Ticket, Tier: req.Tier, SecondGuard: req.Second}
		sel              runtime.Selection
		cs               runtime.ContextSettings
		backend          = d.Backend
		scratch          string
		claim            Claim
		w                *Worktree
		stable, variable []byte
		spec             runtime.LaunchSpec
		ck               = &checker{g: pg, remoteMode: g.Remote != "", common: common, guard: guard}
	)
	defer func() { _ = os.Remove(scratch) }() // empty only; a no-op once the record step removed it
	err = seq(
		func() (e error) { baseOID, e = ResolveBase(ctx, g, g.Remote, req.Base); return },
		func() (e error) { point, e = offBranch(ctx, g, req, baseOID); return },
		func() (e error) { in, e = ReadInputs(ctx, g, baseOID, req); task.GuardMustFix = in.MustFix; return },
		func() (e error) { sel, e = l.policy.Resolve(req.Role, task, nil); return },
		func() error {
			if req.Second {
				return checkSecond(req.RepoDir, req.Ticket, baseOID, sel.Model)
			}
			return nil
		},
		func() (e error) { cs, e = l.context.Settings(req.Role, task, rt.Config().NudgeTokens); return },
		func() error {
			if backend != nil {
				return nil
			}
			opts, e := l.cfg.CombOptions(req.RepoDir)
			opts.GitBinary = d.GitBin
			opts.ExtraEnv = combEnv
			if e == nil {
				backend, e = comb.NewGitBackend(opts)
			}
			return e
		},
		func() (e error) { // the guard reads the builder's claim and writes none
			if guard {
				want, req.Head, claim.OID, e = guardTarget(ctx, backend, g, req)
				claim.Verb = "read"
			}
			return
		},
		func() (e error) { scratch, e = newScratch(os.TempDir(), req.Ticket, req.Role); return },
		func() (e error) {
			cmds = waggleCommands(in.WaggleMD)
			bash, e := runtime.BashCommands(req.Role, cmds)
			if e != nil {
				return e
			}
			lines := TaskLines(req, baseOID, want, bash)
			if guard {
				lines = guardTaskLines(req, baseOID, want, bash)
			}
			lines = withBranchPoint(lines, baseOID, point)
			stable, variable = BuildPrompts(in, lines, cs)
			spec, e = rt.Prepare(runtime.Invocation{Role: req.Role, TaskID: req.Ticket, WorktreeDir: wtPath,
				Selection: sel, Commands: cmds, GitIdentity: id, ScratchDir: scratch}, cs, stable, variable)
			return
		},
		func() (e error) {
			if !guard {
				claim, e = AcquireClaim(ctx, backend, g, req, l.keeper, now())
				want = claim.Lease
			}
			return
		},
		func() (e error) { w, e = Provision(ctx, g, req, baseOID, time.Now()); return },
		func() (e error) { runDir, e = NewRunDir(req.RepoDir, req.Ticket, req.Role); return },
		func() error { return SavePrompts(runDir, stable, variable) },
		func() (e error) {
			ck.w, ck.lease, ck.claimOID, ck.head = w, want, claim.OID, req.Head
			ck.before, e = takeSnap(ctx, pg, common, w.Admin)
			return
		})
	if w != nil {
		defer w.Unlock()
	}
	if err != nil {
		return res, err
	}
	for _, a := range spec.Argv {
		if s, ok := strings.CutPrefix(a, "--settings="); ok {
			_ = os.WriteFile(filepath.Join(runDir, "settings.json"), []byte(s), 0o600)
		}
	}
	res = Result{Schema: 1, Ticket: req.Ticket, Role: req.Role, Branch: req.Branch, BaseOID: baseOID, CheckBaseOID: w.CheckBase,
		Worktree: w.Path, RunDir: runDir, KeeperID: l.keeper, ClaimOID: claim.OID, ClaimVerb: claim.Verb, ClaimNotes: claim.Notes,
		Second: req.Second, Cell: d.Cell.Name(), RuntimeVersion: runtimeVersion(ctx, rt.Config().Binary), Model: sel.Model, Profile: cs.Profile.String(),
		Floored: cs.Floored, StartedAt: now().UTC(), Final: "final.md", Transcript: "transcript.jsonl", StderrLog: "stderr.log"}
	res.Warnings = slices.Concat(lockWarns, w.Warnings, claim.Warnings)
	if point != w.CheckBase { // the branch moved between the two reads; Provision's value stays
		res.Warnings = append(res.Warnings, "branch_moved: branch point was "+shortOID(point)+" when the prompts were built, "+shortOID(w.CheckBase)+" at checkout")
	}
	if w.CheckBase != baseOID {
		res.Warnings = append(res.Warnings, "branch_behind_base: branch forks at "+shortOID(w.CheckBase)+", base is "+shortOID(baseOID)+"; rebase before merge")
	}
	if len(cmds) == 0 {
		res.Warnings = append(res.Warnings, "no_waggle_commands: WAGGLE.md has no \"Build and Verification Commands\" bullets; the worker may run no build or test command")
	}
	d.emit("claimed", "%s %s by %s", claim.Verb, req.Ticket, l.keeper)
	d.emit("worktree", "%s on %s", filepath.Base(w.Path), w.Branch)
	every := d.KeepaliveEvery
	if every <= 0 {
		every = time.Duration(l.cfg.Leases.DefaultTTLMinutes) * time.Minute / 2
	}
	renew := func(ctx context.Context) error {
		if len(diffKeys(ck.before.files, hostFiles(ck.common, w.Admin))) > 0 { // the agent changed what git trusts
			return errors.New("host files changed; renewal skipped")
		}
		if guard {
			return nil
		}
		err := backend.AcquireLease(ctx, comb.Lease{TicketID: req.Ticket, KeeperID: l.keeper, Role: req.Role, Gate: req.Gate, Directories: req.Dirs})
		if oid, e := pg.Run(ctx, "rev-parse", "--verify", "--quiet", ownRef); err == nil && e == nil {
			ck.claimOID = strings.TrimSpace(string(oid)) // the renewal's own write; the check wants this ref at the end
		}
		return err
	}
	d.emit("launched", "%s in %s", sel.Model, d.Cell.Name())
	s, err := runSession(ctx, d, CellSpec{Worktree: w.Path, LeaseRW: req.Dirs}, spec, rt, runDir, renew, every)
	if err != nil {
		return res, err
	}
	res.EndedAt, res.NotEnforced, res.Warnings = now().UTC(), s.NotEnforced, append(res.Warnings, s.Warnings...)
	res.ResultSubtype, res.Compacted, res.CostUSD, res.Survivors, res.MustFix = s.Subtype, s.Compacted, s.CostUSD, s.Survivors, in.MustFix
	u := s.Usage
	res.Usage = Usage{Input: u.InputTokens, CacheWrite: u.CacheWriteTokens, CacheRead: u.CacheReadTokens, Output: u.OutputTokens}
	end := cutOf(s)
	if end == "" {
		end = s.End
	}
	d.emit("ended", "%s, %d tokens", end, u.TotalTokens)
	_ = writeFile(runDir, "final.md", s.Final)
	if guard { // the review is the final message; the queen commits it (D-057)
		res.Verdict = parseVerdict(s.Final)
		_ = writeFile(runDir, "review.md", s.Final)
	}
	pctx := context.WithoutCancel(ctx) // the checks run even when ctx was cancelled
	if g.Remote != "" {                // the final sync overwrites a tampered claim mirror, but never runs on changed host files
		if len(diffKeys(ck.before.files, hostFiles(ck.common, w.Admin))) > 0 {
			res.Warnings = append(res.Warnings, "claim_sync_skipped: host files changed")
		} else if _, err := backend.ClaimOID(pctx, req.Ticket); err != nil && !errors.Is(err, comb.ErrNotFound) {
			res.Warnings = append(res.Warnings, "claim_sync_failed: "+err.Error())
		} else {
			ck.synced = true
		}
	}
	var commits int
	res.Violations, commits = ck.run(pctx)
	res.HeadOID = ck.head
	if ck.trailers > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("attribution_trailer: %d commit(s) carry a Co-authored-by trailer; drop it before merge", ck.trailers))
	}
	if slices.ContainsFunc(res.Violations, func(v comb.Violation) bool { return v.Rule == ruleHost }) {
		if err := writeFile(dir, filepath.Base(w.Path)+".tainted", []byte(filepath.Base(runDir))); err != nil {
			res.Warnings = append(res.Warnings, "tainted_marker_failed: "+err.Error())
		}
	}
	if guard {
		commits = 1 // a guard commits nothing: only the verdict decides between done and no_verdict
	}
	res.End, res.ExitCode = outcome(s, res.Violations, commits)
	if guard && res.End == "done" && res.Verdict == "" {
		res.End, res.ExitCode = "no_verdict", ExitNeedsKeeper
	}
	d.emit("checked", "%d violations, %d commits", len(res.Violations), commits)
	if ref := "refs/heads/" + req.Branch; res.ExitCode == 0 && g.Remote != "" && !d.NoPush && !guard {
		if _, err := g.Run(pctx, "push", g.Remote, ck.head+":"+ref); err != nil { // the checked OID, never the ref
			res.End, res.ExitCode, res.Error = "error", ExitEnv, err.Error()
		} else {
			res.Pushed = true
			d.emit("pushed", "%s to %s", req.Branch, g.Remote)
		}
	}
	if res.ExitCode == 0 && !d.KeepWorktree {
		if err := w.Remove(pctx, pg); err != nil {
			res.Warnings = append(res.Warnings, "worktree_remove_failed: "+err.Error())
		}
	}
	// the next dispatch of any ticket compares the host files with what the checks verified
	if err := saveHostRecord(dir, ck.before, filepath.Base(runDir)); err != nil {
		res.Warnings = append(res.Warnings, "host_record_failed: "+err.Error())
	}
	if warn := dropScratch(scratch); warn != "" {
		res.Warnings = append(res.Warnings, warn)
	}
	err = res.Write()
	if aerr := appendLedger(req.RepoDir, l.policy.Models.Adapter, s.SessionID, res); aerr != nil && err == nil {
		res.Warnings = append(res.Warnings, "ledger_append_failed: "+aerr.Error())
		err = res.Write()
	}
	if err != nil {
		return res, err
	}
	d.emit("recorded", "%s", filepath.Base(runDir))
	return res, nil
}

// newScratch creates the run's own scratch directory under parent (mode 0700).
// It becomes the worker's TMPDIR and the one directory the push tripwire lets
// through (T-028); nothing about it comes from the worker or the repo.
func newScratch(parent, ticket, role string) (string, error) {
	dir, err := os.MkdirTemp(parent, "waggle-"+ticket+"-"+role+"-")
	if err != nil {
		return "", failf(ExitEnv, "scratch directory: %w", err)
	}
	return dir, nil
}

// dropScratch removes the scratch directory only when it is empty and returns
// the scratch_left warning when it is not. It never removes anything inside it.
func dropScratch(dir string) string {
	if dir == "" {
		return ""
	}
	if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
		return "scratch_left: " + filepath.Base(dir)
	}
	return ""
}

// outcome maps the session and the checks to result.end and the exit code. A
// violation other than a dirty tree always wins: it blocks the push and is the
// Keeper's to read. A session cut short by a limit leaves work half done, so a
// dirty tree alone does not hide that it needs the Keeper.
func outcome(s session, vs []comb.Violation, commits int) (string, int) {
	serious := slices.ContainsFunc(vs, func(v comb.Violation) bool { return v.Rule != ruleDirty })
	cut := cutOf(s)
	switch {
	case serious || (len(vs) > 0 && cut == "" && s.End == runtime.EndDone):
		return "lease_violation", ExitFailed
	case cut != "":
		return cut, ExitNeedsKeeper
	case s.End == runtime.EndDone && commits > 0:
		return "done", 0
	case s.End == runtime.EndDone:
		return "no_change", ExitFailed
	case s.End == runtime.EndInterrupted:
		return "interrupted", ExitFailed
	}
	return "error", ExitFailed
}

// runtimeVersion is `<binary> --version`, empty when it cannot be read.
func runtimeVersion(ctx context.Context, binary string) string {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--version")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

// session is what one worker process produced.
type session struct {
	runtime.RunResult
	Subtype     string // the stream's own result subtype
	TimedOut    bool
	NotEnforced []string
	Warnings    []string
	Survivors   []Survivor
}

// tap copies the worker's output into transcript.jsonl and remembers the
// subtype of the last result event, which the adapter does not expose (a
// budget stop must not look like a crash).
type tap struct {
	r       io.Reader
	f       *os.File
	line    []byte
	subtype string
}

func (t *tap) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	_, _ = t.f.Write(p[:n])
	for _, b := range p[:n] {
		if b != '\n' {
			if len(t.line) < runtime.MaxStreamLine {
				t.line = append(t.line, b)
			}
			continue
		}
		var ev struct{ Type, Subtype string }
		if json.Unmarshal(t.line, &ev) == nil && ev.Type == "result" {
			t.subtype = ev.Subtype
		}
		t.line = t.line[:0]
	}
	return n, err
}

// runSession starts the worker through the Cell, streams it into Collect,
// renews the claim every `every`, and returns when the process group is gone
// and every goroutine has exited. The end comes from the stream, never from the
// exit code (Claude Code exits 1 on error_max_turns). stop is non-blocking and
// goroutine-safe: SIGTERM to the group, SIGKILL after killGrace.
func runSession(ctx context.Context, d Deps, cs CellSpec, spec runtime.LaunchSpec, rt runtime.AgentRuntime, runDir string,
	renew func(context.Context) error, every time.Duration) (s session, err error) {
	cmd, ne, err := d.Cell.Command(ctx, cs, spec)
	if err != nil {
		return s, err
	}
	s.NotEnforced = ne
	open := func(name string) *os.File {
		f, e := os.OpenFile(filepath.Join(runDir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		err = errors.Join(err, e)
		return f
	}
	tf, ef := open("transcript.jsonl"), open("stderr.log")
	pr, pw, perr := os.Pipe()
	if err = errors.Join(err, perr); err != nil {
		return s, err
	}
	defer tf.Close()
	defer ef.Close()
	defer pr.Close()
	cmd.Stdout, cmd.Stderr = pw, ef
	err = cmd.Start()
	pw.Close()
	if err != nil {
		return s, failf(ExitEnv, "starting %s: %w", spec.Argv[0], err)
	}
	// The tracker's first table read lists the leader before cmd.Wait can reap
	// it, so its start time is known. The pipe is the stream's only way to EOF:
	// when the leader exits, the whole group is killed, so a lingering
	// grandchild cannot hold it open.
	tr := newTracker(cmd.Process.Pid)
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); signalGroup(cmd, true); close(exited) }()
	var mu sync.Mutex
	var killer *time.Timer
	finished := false
	stop := func() {
		mu.Lock()
		defer mu.Unlock()
		if killer == nil && !finished {
			signalGroup(cmd, false)
			killer = time.AfterFunc(killGrace, func() { signalGroup(cmd, true) })
		}
	}
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Minute
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// A renewal that has started finishes (it may be mid-write) but is bounded;
	// none starts once the session or the leader is over, and the sweep waits.
	renewed := make(chan struct{})
	go func() {
		defer close(renewed)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-tctx.Done():
				return
			case <-exited:
				return
			case <-t.C:
				if tctx.Err() != nil {
					return
				}
				rctx, rcancel := context.WithTimeout(context.WithoutCancel(tctx), renewBound)
				e := renew(rctx)
				rcancel()
				if e != nil && tctx.Err() == nil && len(s.Warnings) < 10 {
					s.Warnings = append(s.Warnings, "claim_renew_failed: "+e.Error())
				}
			}
		}
	}()
	// The tree is sampled while the worker runs. Once the leader is reaped,
	// everything tracked that still runs is killed (D-187) and the read end is
	// closed after a short grace, so a process that escaped unseen and holds the
	// pipe cannot hold Run until the timeout.
	tracked, collected, swept, held := make(chan struct{}), make(chan struct{}), make(chan struct{}), false
	go func() {
		defer close(tracked)
		for t := time.NewTicker(pollEvery); ; {
			select {
			case <-exited:
				t.Stop()
				return
			case <-t.C:
				tr.poll()
			}
		}
	}()
	go func() {
		defer close(swept)
		<-exited
		<-tracked
		<-renewed
		s.Survivors = tr.sweep()
		select {
		case <-collected:
		case <-time.After(time.Second):
			held = true
			pr.Close()
		}
	}()
	tp := &tap{r: pr, f: tf}
	s.RunResult, err = rt.Collect(tctx, tp, runtime.Limits{MaxTokens: rt.Config().MaxTokens}, stop)
	close(collected)
	cancel()
	<-renewed
	select {
	case <-exited:
	default:
		stop() // the stream ended or failed while the process still runs
	}
	<-exited
	<-swept
	if held {
		s.Warnings = append(s.Warnings, "stdout_held: a process outside the group held the output open; its stream was cut")
	}
	if tr.blind {
		s.Warnings = append(s.Warnings, "survivors_unchecked: the process table could not be read, so leftover processes may remain")
	}
	if len(s.Survivors) > 0 {
		s.Warnings = append(s.Warnings, fmt.Sprintf("survivors_killed: %d process(es) outlived the worker and were killed: %v", len(s.Survivors), s.Survivors))
	}
	mu.Lock()
	finished = true
	if killer != nil {
		killer.Stop()
	}
	mu.Unlock()
	s.Subtype = tp.subtype
	s.TimedOut = errors.Is(tctx.Err(), context.DeadlineExceeded) && ctx.Err() == nil
	if err != nil {
		s.Warnings = append(s.Warnings, "collect: "+err.Error())
	}
	return s, nil
}
