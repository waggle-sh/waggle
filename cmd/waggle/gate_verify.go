// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
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

// print writes the findings of one check and returns its exit code. With open
// set (gate.mode open), a finding prints as a would-be failure and the code
// stays 0.
func (r *gateReport) print(check string, stdout, stderr io.Writer, gha, open bool) int {
	code := gatePass
	failed := false
	for _, f := range r.findings {
		if f.check != check {
			continue
		}
		msg := annotData.Replace(f.msg)
		switch {
		case open && !f.warning:
			printWouldFail(stderr, gha, check, f.path, msg)
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
			failed = true
			if !open {
				code = gateFail
			}
		}
	}
	if !failed {
		fmt.Fprintf(stdout, "%s: ok\n", check)
	}
	return code
}

// printWouldFail reports a finding that gate.mode open does not enforce. msg
// is already encoded.
func printWouldFail(stderr io.Writer, gha bool, check, path, msg string) {
	switch {
	case gha && path != "":
		fmt.Fprintf(stderr, "::warning file=%s::%s: would fail (mode open): %s\n", annotProp.Replace(path), check, msg)
	case gha:
		fmt.Fprintf(stderr, "::warning::%s: would fail (mode open): %s\n", check, msg)
	case path != "":
		fmt.Fprintf(stderr, "%s: would fail (mode open): %s: %s\n", check, annotData.Replace(path), msg)
	default:
		fmt.Fprintf(stderr, "%s: would fail (mode open): %s\n", check, msg)
	}
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
	counts := new(string)
	if check == "verify" {
		fs.StringVar(counts, "counts", "", "write per-check finding counts as JSON to this path")
	}
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
	prh := prHead{name: branchName, gha: gha}
	switch {
	case ev != nil:
		prh.repo = ev.headRepo
		prh.sameRepo = ev.headRepo != "" && strings.EqualFold(ev.headRepo, ev.baseRepo)
	default:
		prh.sameRepo = !gha || *branch != ""
	}
	in := scanInput{branchID: branchID, head: prh, tree: check != checkLeases}
	pre := slices.Clone(rep.findings)
	out, err := scanRange(g, *base, *head, in, rep)
	code := gatePass
	// Only verify honors gate.mode, and only the exact value "open" loosens it;
	// "hold" fails on a finding exactly like "guard" (T-113).
	// A pass too large to check stays a failure (Q-T076-1).
	open := check == "verify" && out.mode == config.GateModeOpen && !errors.Is(err, errTooLarge)
	if open {
		if gha {
			fmt.Fprintln(stderr, "::notice::gate: mode open: findings are reported, not enforced")
		} else {
			fmt.Fprintln(stderr, "gate: mode open: findings are reported, not enforced")
		}
	}
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
				code = max(code, rep.print(c, stdout, stderr, gha, open))
			}
		}
	}
	decFailed := false
	if check == "verify" {
		var dout bytes.Buffer
		dcode := runGateDecisionsMode([]string{"--base", *base, "--head", *head, "--repo", *repo}, &dout, stderr, getenv, open)
		stdout.Write(dout.Bytes())
		decFailed = decisionsFailed(dcode, open, dout.String())
		code = max(code, dcode)
	}
	// On a gate error no file is written: without the artifact the counts are
	// omitted, never guessed. A write failure only warns; telemetry must never
	// fail the required gate.
	if *counts != "" && code != gateError {
		if err := writeGateCounts(*counts, countGate(rep.findings, open, decFailed)); err != nil {
			fmt.Fprintf(stderr, "gate: warning: cannot write counts file: %s\n", annotData.Replace(err.Error()))
		}
	}
	return code
}

// prHead is the PR head a claim's branch list is matched against. name is the
// raw branch name, "" when unknown. sameRepo is false for a fork and for any
// head under Actions whose repository the gate could not confirm.
type prHead struct {
	name     string
	repo     string // head repository, owner/name; "" when unknown
	sameRepo bool
	gha      bool
}

// scanInput is what one scanRange pass is asked to do.
type scanInput struct {
	branchID string // ticket the PR branch binds, or ""
	head     prHead // the PR head the claims are checked against
	tree     bool   // also check the head tree for case collisions
	filter   bool   // drop tier3 and outside_lease findings on approvable paths
	// keeperFilter drops tier3 and outside_lease findings on hive paths; set
	// only after the Head Keeper's approval counts (T-056).
	keeperFilter bool
}

// scanOutcome is what a pass learned beyond its findings.
type scanOutcome struct {
	baseOID, headOID string
	approvers        []string      // gate.approvers of the base config
	mode             string        // gate.mode of the base config
	hit              bool          // a tier3 finding on an approvable path
	keeperHit        bool          // a tier3 or outside_lease finding on a hive path
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
	policy, approvers, mode, err := baseTierPolicy(g, baseOID)
	if err != nil {
		return scanOutcome{}, err
	}
	s := commitScan{g: g, comb: cb, now: time.Now(), rep: rep, claims: map[string]claimLookup{},
		baseOID: baseOID, policy: policy, branch: in.branchID, head: in.head, bound: map[string]string{},
		approvable: approvablePath(policy), filter: in.filter,
		keeper: keeperPath, keeperFilter: in.keeperFilter,
		signed: func() (keeperSig, error) { return headKeeperSigned(g, baseOID, headOID, approvers) }}
	res := scanOutcome{baseOID: baseOID, headOID: headOID, approvers: approvers, mode: mode}
	if err := s.scan(mb, headOID); err != nil {
		return res, err
	}
	short := headOID[:12]
	if s.signedTiers {
		rep.warn(checkTiers, "Tier 3 paths approved by the Head Keeper's signature on head commit "+short)
	}
	if s.signedLease {
		rep.warn(checkLeases, "Tier 3 paths outside the claim approved by the Head Keeper's signature on head commit "+short)
	}
	if s.sigChecked && s.sig.why != "" {
		rep.warn(checkTiers, "no Head Keeper signature counts for head commit "+short+": "+s.sig.why)
	}
	res.hit, res.keeperHit, res.maxTier = s.hit, s.keeperHit, s.maxTier
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

// baseTierPolicy reads gate.protected_paths, gate.approvers and gate.mode from the config
// at the PR base, never from the head tree or the worktree. A missing file
// means no entries;
// a symlink, tree, submodule, oversized or invalid file is an error.
func baseTierPolicy(g gitRunner, baseOID string) (comb.TierPolicy, []string, string, error) {
	out, err := g.run("ls-tree", "-z", "--full-tree", baseOID, "--", layout.Config)
	if err != nil {
		return comb.TierPolicy{}, nil, config.GateModeGuard, err
	}
	if len(out) == 0 {
		return comb.TierPolicy{}, nil, config.GateModeGuard, nil
	}
	recs := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	meta, _, _ := strings.Cut(recs[0], "\t")
	f := strings.Fields(meta)
	if len(f) != 3 || !oidPattern.MatchString(f[2]) {
		return comb.TierPolicy{}, nil, "", fmt.Errorf("%w ls-tree: unexpected output %q", errGit, recs[0])
	}
	if len(recs) != 1 || f[1] != "blob" || (f[0] != "100644" && f[0] != "100755") {
		return comb.TierPolicy{}, nil, "", fmt.Errorf("base: %s is mode %s type %s; it must be a regular file", layout.Config, f[0], f[1])
	}
	out, err = g.run("cat-file", "-s", f[2])
	if err != nil {
		return comb.TierPolicy{}, nil, "", err
	}
	size, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return comb.TierPolicy{}, nil, "", fmt.Errorf("%w cat-file -s: unexpected output %q", errGit, out)
	}
	if size > config.MaxFileSize {
		return comb.TierPolicy{}, nil, "", fmt.Errorf("base: %s is %d bytes, over the %d byte cap", layout.Config, size, config.MaxFileSize)
	}
	data, err := g.run("cat-file", "blob", f[2])
	if err != nil {
		return comb.TierPolicy{}, nil, "", err
	}
	cfg, err := config.Parse(data)
	if err != nil {
		return comb.TierPolicy{}, nil, "", fmt.Errorf("base: %w", err)
	}
	return comb.TierPolicy{ExtraTier3: cfg.Gate.ProtectedPaths}, cfg.Gate.Approvers, cfg.Gate.Mode, nil
}
