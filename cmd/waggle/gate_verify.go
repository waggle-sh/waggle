package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
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
	err := scanRange(g, *base, *head, check != checkLeases, &rep)
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
// checks the head tree for case collisions.
func scanRange(g gitRunner, base, head string, tree bool, rep *gateReport) error {
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
	s := commitScan{g: g, comb: cb, now: time.Now(), rep: rep, claims: map[string]claimLookup{}}
	if err := s.scan(mb, headOID); err != nil {
		return err
	}
	if tree {
		return caseCollisions(g, headOID, rep)
	}
	return nil
}
