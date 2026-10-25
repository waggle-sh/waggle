package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
)

// Check names, in the order verify runs and reports them.
const (
	checkLeases = "leases"
	checkTiers  = "tiers"
)

// Workflow-command encoders (GitHub's rules). Every path, subject and reason
// that reaches output goes through gateReport.print, which applies them, so
// attacker-chosen text can never start a new line or a new command.
var (
	annotData = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	annotProp = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

type gateFinding struct {
	check, path, msg string
	warning          bool
}

// gateReport collects findings. A path fails each check at most once, however
// many commits touch it.
type gateReport struct {
	findings []gateFinding
	seen     map[string]bool
}

func (r *gateReport) fail(check, path, msg string) {
	key := check + "\x00" + path
	if path == "" {
		key += "\x00" + msg
	}
	if r.seen == nil {
		r.seen = map[string]bool{}
	}
	if r.seen[key] {
		return
	}
	r.seen[key] = true
	r.findings = append(r.findings, gateFinding{check: check, path: path, msg: msg})
}

func (r *gateReport) warn(check, msg string) {
	r.findings = append(r.findings, gateFinding{check: check, msg: msg, warning: true})
}

// print writes the findings of one check and returns its exit code.
func (r *gateReport) print(check string, stdout, stderr io.Writer, gha bool) int {
	code := gatePass
	for _, f := range r.findings {
		if f.check != check {
			continue
		}
		msg := annotData.Replace(f.msg)
		switch {
		case f.warning && gha:
			fmt.Fprintf(stderr, "::warning::%s: %s\n", check, msg)
		case f.warning:
			fmt.Fprintf(stderr, "%s: warning: %s\n", check, msg)
		case gha && f.path != "":
			fmt.Fprintf(stderr, "::error file=%s::%s: %s\n", annotProp.Replace(f.path), check, msg)
		case gha:
			fmt.Fprintf(stderr, "::error::%s: %s\n", check, msg)
		case f.path != "":
			fmt.Fprintf(stderr, "%s: FAIL: %s: %s\n", check, annotData.Replace(f.path), msg)
		default:
			fmt.Fprintf(stderr, "%s: FAIL: %s\n", check, msg)
		}
		if !f.warning {
			code = gateFail
		}
	}
	if code == gatePass {
		fmt.Fprintf(stdout, "%s: ok\n", check)
	}
	return code
}

// runGateRange implements `waggle gate verify|leases|tiers`. verify runs
// leases, tiers, then decisions against the same merge-base and head, reports
// every failure, and returns the worst exit code under 0 < 1 < 3.
func runGateRange(check string, args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("gate "+check, flag.ContinueOnError)
	fs.SetOutput(stderr)
	base := fs.String("base", "", "PR base revision (required)")
	head := fs.String("head", "", "PR head revision (required)")
	repo := fs.String("repo", ".", "Git repository to inspect")
	branch := fs.String("branch", "", "PR branch name (default: GITHUB_HEAD_REF under GitHub Actions)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return gatePass
		}
		return gateUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "waggle gate %s: unexpected argument %q\n", check, fs.Arg(0))
		return gateUsage
	}
	for _, f := range []struct{ name, rev string }{{"--base", *base}, {"--head", *head}} {
		if err := checkRev(f.rev); err != nil {
			fmt.Fprintf(stderr, "waggle gate %s: %s %v\n", check, f.name, err)
			return gateUsage
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	g := gitRunner{ctx, *repo}
	gha := getenv("GITHUB_ACTIONS") == "true"

	rep := &gateReport{}
	var ev *prEvent
	var evReason string
	if gha {
		ev, evReason = readPREvent(getenv)
	}
	branchName := *branch
	if branchName == "" && gha {
		branchName = getenv("GITHUB_HEAD_REF")
		if branchName == "" && ev != nil {
			branchName = ev.headRef
		}
	}
	branchID, ignored := branchTicket(branchName)
	if ignored {
		if check == checkTiers {
			rep.warn(checkTiers, "branch name ignored")
		} else {
			rep.warn(checkLeases, "branch name ignored")
		}
	}
	in := scanInput{branchID: branchID, tree: check != checkLeases}
	pre := slices.Clone(rep.findings)
	out, err := scanRange(g, *base, *head, in, rep)
	code := gatePass
	switch {
	case errors.Is(err, errTooLarge) && check == checkTiers:
		rep.fail(checkTiers, "", err.Error())
	case errors.Is(err, errTooLarge):
		rep.fail(checkLeases, "", err.Error())
	case err != nil:
		fmt.Fprintf(stderr, "%s: error: %s\n", check, annotData.Replace(err.Error()))
		code = gateError
	case err == nil:
		pr := prRun{g: g, check: check, in: in, gha: gha, getenv: getenv, ev: ev, evReason: evReason, pre: pre}
		rep = pr.finish(rep, out)
	}
	if code == gatePass {
		for _, c := range []string{checkLeases, checkTiers} {
			if check == c || check == "verify" {
				code = max(code, rep.print(c, stdout, stderr, gha))
			}
		}
	}
	if check == "verify" {
		code = max(code, runGateDecisions([]string{"--base", *base, "--head", *head, "--repo", *repo}, stdout, stderr, getenv))
	}
	return code
}

// scanInput is what one scanRange pass is asked to do.
type scanInput struct {
	branchID string // ticket the PR branch binds, or ""
	tree     bool   // also check the head tree for case collisions
	filter   bool   // drop tier3 and outside_lease findings on approvable paths
}

// scanOutcome is what a pass learned beyond its findings.
type scanOutcome struct {
	baseOID, headOID string
	approvers        []string      // gate.approvers of the base config
	hit              bool          // a tier3 finding on an approvable path
	bound            []string      // distinct tickets the commits bind, sorted
	maxTier          comb.RiskTier // highest tier the changed paths require
}

// scanRange resolves base and head, takes their merge-base exactly as
// gateDecisions does, checks every commit in between, and with in.tree set also
// checks the head tree for case collisions. Repo-defined protected paths and
// approvers are read from base only.
func scanRange(g gitRunner, base, head string, in scanInput, rep *gateReport) (scanOutcome, error) {
	baseOID, err := g.resolve(base)
	if err != nil {
		return scanOutcome{}, err
	}
	headOID, err := g.resolve(head)
	if err != nil {
		return scanOutcome{}, err
	}
	out, err := g.run("merge-base", baseOID, headOID)
	mb := strings.TrimSpace(string(out))
	if err != nil || !oidPattern.MatchString(mb) {
		return scanOutcome{}, fmt.Errorf("%w: no merge-base; is the clone shallow? use fetch-depth: 0 (%v)", errGit, err)
	}
	cb, err := comb.NewGitBackend(comb.Options{RepoDir: g.repo})
	if err != nil {
		return scanOutcome{}, fmt.Errorf("%w: %v", errGit, err)
	}
	policy, approvers, err := baseTierPolicy(g, baseOID)
	if err != nil {
		return scanOutcome{}, err
	}
	s := commitScan{g: g, comb: cb, now: time.Now(), rep: rep, claims: map[string]claimLookup{},
		baseOID: baseOID, policy: policy, branch: in.branchID, bound: map[string]string{},
		approvable: approvablePath(policy), filter: in.filter}
	res := scanOutcome{baseOID: baseOID, headOID: headOID, approvers: approvers}
	if err := s.scan(mb, headOID); err != nil {
		return res, err
	}
	res.hit, res.maxTier = s.hit, s.maxTier
	for _, id := range s.bound {
		if !slices.Contains(res.bound, id) {
			res.bound = append(res.bound, id)
		}
	}
	slices.Sort(res.bound)
	if in.tree {
		return res, caseCollisions(g, headOID, rep)
	}
	return res, nil
}

// baseTierPolicy reads gate.protected_paths and gate.approvers from the config
// at the PR base, never from the head tree or the worktree. A missing file
// means no entries;
// a symlink, tree, submodule, oversized or invalid file is an error.
func baseTierPolicy(g gitRunner, baseOID string) (comb.TierPolicy, []string, error) {
	out, err := g.run("ls-tree", "-z", "--full-tree", baseOID, "--", layout.Config)
	if err != nil || len(out) == 0 {
		return comb.TierPolicy{}, nil, err
	}
	recs := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	meta, _, _ := strings.Cut(recs[0], "\t")
	f := strings.Fields(meta)
	if len(f) != 3 || !oidPattern.MatchString(f[2]) {
		return comb.TierPolicy{}, nil, fmt.Errorf("%w ls-tree: unexpected output %q", errGit, recs[0])
	}
	if len(recs) != 1 || f[1] != "blob" || (f[0] != "100644" && f[0] != "100755") {
		return comb.TierPolicy{}, nil, fmt.Errorf("base: %s is mode %s type %s; it must be a regular file", layout.Config, f[0], f[1])
	}
	out, err = g.run("cat-file", "-s", f[2])
	if err != nil {
		return comb.TierPolicy{}, nil, err
	}
	size, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return comb.TierPolicy{}, nil, fmt.Errorf("%w cat-file -s: unexpected output %q", errGit, out)
	}
	if size > config.MaxFileSize {
		return comb.TierPolicy{}, nil, fmt.Errorf("base: %s is %d bytes, over the %d byte cap", layout.Config, size, config.MaxFileSize)
	}
	data, err := g.run("cat-file", "blob", f[2])
	if err != nil {
		return comb.TierPolicy{}, nil, err
	}
	cfg, err := config.Parse(data)
	if err != nil {
		return comb.TierPolicy{}, nil, fmt.Errorf("base: %w", err)
	}
	return comb.TierPolicy{ExtraTier3: cfg.Gate.ProtectedPaths}, cfg.Gate.Approvers, nil
}
