package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
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

	var rep gateReport
	branchName := *branch
	if branchName == "" && gha {
		branchName = getenv("GITHUB_HEAD_REF")
	}
	branchID, ignored := branchTicket(branchName)
	if ignored {
		rep.warn(checkLeases, "branch name ignored")
	}
	err := scanRange(g, *base, *head, branchID, check != checkLeases, &rep)
	code := gatePass
	switch {
	case errors.Is(err, errTooLarge) && check == checkTiers:
		rep.fail(checkTiers, "", err.Error())
	case errors.Is(err, errTooLarge):
		rep.fail(checkLeases, "", err.Error())
	case err != nil:
		fmt.Fprintf(stderr, "%s: error: %s\n", check, annotData.Replace(err.Error()))
		code = gateError
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

// scanRange resolves base and head, takes their merge-base exactly as
// gateDecisions does, checks every commit in between, and with tree set also
// checks the head tree for case collisions. branchID is the ticket the PR
// branch binds, or "". Repo-defined protected paths are read from base only.
func scanRange(g gitRunner, base, head, branchID string, tree bool, rep *gateReport) error {
	baseOID, err := g.resolve(base)
	if err != nil {
		return err
	}
	headOID, err := g.resolve(head)
	if err != nil {
		return err
	}
	out, err := g.run("merge-base", baseOID, headOID)
	mb := strings.TrimSpace(string(out))
	if err != nil || !oidPattern.MatchString(mb) {
		return fmt.Errorf("%w: no merge-base; is the clone shallow? use fetch-depth: 0 (%v)", errGit, err)
	}
	cb, err := comb.NewGitBackend(comb.Options{RepoDir: g.repo})
	if err != nil {
		return fmt.Errorf("%w: %v", errGit, err)
	}
	policy, err := baseTierPolicy(g, baseOID)
	if err != nil {
		return err
	}
	s := commitScan{g: g, comb: cb, now: time.Now(), rep: rep, claims: map[string]claimLookup{},
		baseOID: baseOID, policy: policy, branch: branchID, bound: map[string]string{}}
	if err := s.scan(mb, headOID); err != nil {
		return err
	}
	if tree {
		return caseCollisions(g, headOID, rep)
	}
	return nil
}

// baseTierPolicy reads gate.protected_paths from the config at the PR base,
// never from the head tree or the worktree. A missing file means no entries;
// a symlink, tree, submodule, oversized or invalid file is an error.
func baseTierPolicy(g gitRunner, baseOID string) (comb.TierPolicy, error) {
	out, err := g.run("ls-tree", "-z", "--full-tree", baseOID, "--", layout.Config)
	if err != nil || len(out) == 0 {
		return comb.TierPolicy{}, err
	}
	recs := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	meta, _, _ := strings.Cut(recs[0], "\t")
	f := strings.Fields(meta)
	if len(f) != 3 || !oidPattern.MatchString(f[2]) {
		return comb.TierPolicy{}, fmt.Errorf("%w ls-tree: unexpected output %q", errGit, recs[0])
	}
	if len(recs) != 1 || f[1] != "blob" || (f[0] != "100644" && f[0] != "100755") {
		return comb.TierPolicy{}, fmt.Errorf("base: %s is mode %s type %s; it must be a regular file", layout.Config, f[0], f[1])
	}
	out, err = g.run("cat-file", "-s", f[2])
	if err != nil {
		return comb.TierPolicy{}, err
	}
	size, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return comb.TierPolicy{}, fmt.Errorf("%w cat-file -s: unexpected output %q", errGit, out)
	}
	if size > config.MaxFileSize {
		return comb.TierPolicy{}, fmt.Errorf("base: %s is %d bytes, over the %d byte cap", layout.Config, size, config.MaxFileSize)
	}
	data, err := g.run("cat-file", "blob", f[2])
	if err != nil {
		return comb.TierPolicy{}, err
	}
	cfg, err := config.Parse(data)
	if err != nil {
		return comb.TierPolicy{}, fmt.Errorf("base: %w", err)
	}
	return comb.TierPolicy{ExtraTier3: cfg.Gate.ProtectedPaths}, nil
}
