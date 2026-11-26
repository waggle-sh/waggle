// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
)

const (
	rulesPerPage = 100
	rulesMaxPage = 10
)

// failWhy names a failed read in fixed words and the status code. GitHub's
// message text is never returned.
func failWhy(status int, err error) string {
	var ae *apiError
	switch {
	case errors.As(err, &ae):
		return fmt.Sprintf("GitHub answered %d", ae.Status)
	case status >= 200 && status <= 299:
		return "the answer could not be read"
	}
	return "the request failed"
}

// gateRead is what the probe learned about the required check on a branch.
// A non-empty why means the read was inconclusive.
type gateRead struct {
	matched, strict bool
	nonFF           bool // a ruleset rule blocks non-fast-forward updates
	forbidden       bool // the rules read answered 403
	upgrade         bool // the 403 message holds the word upgrade
	why             string
}

// Probe reads the default branch's rules: is the gate check required, and
// must branches be up to date. It writes nothing. An error means no report
// could be built (the default branch could not be read); a failed rules read
// makes the affected findings unknown instead.
func (g *github) Probe(ctx context.Context, opt ProbeOptions) (Report, error) {
	gate := cmp.Or(opt.GateCheck, DefaultGateCheck)
	var info struct {
		DefaultBranch string `json:"default_branch"`
		Private       bool   `json:"private"`
	}
	if status, err := g.t.do(ctx, "GET", g.repoAPI, nil, &info); err != nil {
		return Report{}, fmt.Errorf("forge: probe %s/%s: reading the default branch: %s", g.owner, g.repo, failWhy(status, err))
	}
	branch := info.DefaultBranch
	if err := checkBranch(ctx, g.repoDir, branch); err != nil {
		return Report{}, fmt.Errorf("forge: probe %s/%s: GitHub returned a default branch name git rejects", g.owner, g.repo)
	}
	read := g.readRules(ctx, branch, gate)
	capab, capWhy := capability(read, info.Private)
	if read.why == "" && !read.matched {
		nonFF := read.nonFF
		read = g.readClassic(ctx, branch, gate)
		read.nonFF = nonFF // a ruleset still blocks force-pushes when classic protection holds the check
	}
	claims := "refs/waggle/claims/* in this repository only"
	if g.remote != "" {
		claims = "refs/waggle/claims/* on " + g.remote
	}
	force := g.forcePushFinding(ctx, read, branch)
	r := Report{Provider: "github", Repo: g.owner + "/" + g.repo, Remote: g.remote, Auth: g.auth,
		Claims: claims, MergeStrategy: opt.MergeStrategy, Capability: capab,
		Findings: []Finding{gateFinding(read, branch, gate)}}
	if opt.MergeStrategy == "native_queue" {
		r.Findings = append(r.Findings,
			Finding{ID: "up-to-date", Status: StatusSkipped, Summary: "Not used: merge_strategy is native_queue."},
			Finding{ID: "merge-queue", Status: StatusUnknown,
				Summary:     "This build does not check the merge queue rule.",
				NotEnforced: "this build does not check the merge queue rule, so it cannot say whether merges wait in a queue.",
				Fix:         "check that a merge queue rule is active on " + branch + "."})
	} else {
		r.Findings = append(r.Findings, upToDateFinding(read, branch, gate))
	}
	r.Findings = append(r.Findings, force, capabilityFinding(capab, capWhy, branch), g.writeHolders(ctx))
	return r, nil
}

// readRules pages through the rules that apply to branch.
func (g *github) readRules(ctx context.Context, branch, gate string) gateRead {
	var read gateRead
	for page := 1; ; page++ {
		if page > rulesMaxPage {
			return gateRead{why: "the branch has more rules than this build reads"}
		}
		var rules []struct {
			Type       string          `json:"type"`
			Parameters json.RawMessage `json:"parameters"`
		}
		path := g.repoAPI + "/rules/branches/" + url.PathEscape(branch) +
			"?per_page=" + strconv.Itoa(rulesPerPage) + "&page=" + strconv.Itoa(page)
		if status, err := g.t.do(ctx, "GET", path, nil, &rules); err != nil {
			forbidden, upgrade := upgradeAnswer(err)
			return gateRead{why: failWhy(status, err), forbidden: forbidden, upgrade: upgrade}
		}
		for _, rule := range rules {
			if rule.Type == "non_fast_forward" {
				read.nonFF = true
			}
			if rule.Type != "required_status_checks" {
				continue
			}
			var p struct {
				Checks []struct {
					Context string `json:"context"`
				} `json:"required_status_checks"`
				Strict bool `json:"strict_required_status_checks_policy"`
			}
			if json.Unmarshal(rule.Parameters, &p) != nil {
				return gateRead{why: "the answer could not be read"}
			}
			if slices.ContainsFunc(p.Checks, func(c struct {
				Context string `json:"context"`
			}) bool {
				return c.Context == gate
			}) {
				read.matched = true
				read.strict = read.strict || p.Strict
			}
		}
		if len(rules) < rulesPerPage {
			return read
		}
	}
}

// readClassic reads classic branch protection, the fallback when no ruleset
// rule names the check.
func (g *github) readClassic(ctx context.Context, branch, gate string) gateRead {
	var p struct {
		Strict   bool     `json:"strict"`
		Contexts []string `json:"contexts"`
		Checks   []struct {
			Context string `json:"context"`
		} `json:"checks"`
	}
	path := g.repoAPI + "/branches/" + url.PathEscape(branch) + "/protection/required_status_checks"
	status, err := g.t.do(ctx, "GET", path, nil, &p)
	var ae *apiError
	switch {
	case err == nil:
		matched := slices.Contains(p.Contexts, gate) || slices.ContainsFunc(p.Checks, func(c struct {
			Context string `json:"context"`
		}) bool {
			return c.Context == gate
		})
		return gateRead{matched: matched, strict: matched && p.Strict}
	case errors.As(err, &ae) && ae.Status == 404 &&
		(ae.Message == "Branch not protected" || ae.Message == "Required status checks not enabled"):
		return gateRead{}
	case errors.As(err, &ae) && (ae.Status == 403 || ae.Status == 404):
		return gateRead{why: "needs repo admin to read branch protection"}
	}
	return gateRead{why: failWhy(status, err)}
}

func gateFinding(r gateRead, branch, gate string) Finding {
	f := Finding{ID: "required-gate", Required: true}
	switch {
	case r.why != "":
		f.Status = StatusUnknown
		f.Summary = "Could not read the required checks on " + branch + " (" + r.why + ")."
		f.NotEnforced = "this run could not confirm that " + gate + " is a required check, so a merge may skip the gate."
		f.Fix = "run waggle doctor with a token that can read branch rules and protection, or check " + branch + " by hand."
	case r.matched:
		f.Status, f.Summary = StatusOK, "A rule on "+branch+" requires the "+gate+" check."
	default:
		f.Status = StatusMissing
		f.Summary = "No rule on " + branch + " requires the " + gate + " check."
		f.NotEnforced = "a pull request can merge without the gate passing."
		f.Fix = "add a branch ruleset on " + branch + " that requires " + gate + "."
	}
	return f
}

func upToDateFinding(r gateRead, branch, gate string) Finding {
	f := Finding{ID: "up-to-date", Required: true}
	switch {
	case r.why != "":
		f.Status = StatusUnknown
		f.Summary = "Could not read whether " + branch + " must be up to date (" + r.why + ")."
		f.NotEnforced = "this run could not confirm that a pull request must be up to date before it merges."
		f.Fix = "run waggle doctor with a token that can read branch rules and protection, or check " + branch + " by hand."
	case r.matched && r.strict:
		f.Status, f.Summary = StatusOK, "Pull requests into "+branch+" must be up to date before they merge."
	default:
		f.Status = StatusMissing
		f.Summary = "Pull requests into " + branch + " need not be up to date with it before they merge."
		f.NotEnforced = "a pull request can merge after " + branch + " moved, so the gate never saw the merged result."
		f.Fix = "turn on require branches to be up to date before merging for the " + gate + " check on " + branch + "."
	}
	return f
}

// forcePushFinding says whether the default branch can be force-pushed (T-051
// phase C, RFC-0005 4.5). It is a warning, never required, so it does not change
// doctor's exit code. A ruleset rule that blocks non-fast-forward updates is
// enough; otherwise classic protection's allow_force_pushes decides.
func (g *github) forcePushFinding(ctx context.Context, read gateRead, branch string) Finding {
	f := Finding{ID: "force-push"}
	why := read.why
	allowed := false
	switch {
	case read.nonFF:
		f.Status, f.Summary = StatusOK, "A rule on "+branch+" blocks force-pushes."
		return f
	case why == "":
		var p struct {
			AllowForcePushes struct {
				Enabled bool `json:"enabled"`
			} `json:"allow_force_pushes"`
		}
		status, err := g.t.do(ctx, "GET", g.repoAPI+"/branches/"+url.PathEscape(branch)+"/protection", nil, &p)
		var ae *apiError
		switch {
		case err == nil:
			allowed = p.AllowForcePushes.Enabled
			if !allowed {
				f.Status, f.Summary = StatusOK, "Branch protection on "+branch+" blocks force-pushes."
				return f
			}
		case errors.As(err, &ae) && ae.Status == 404 && ae.Message == "Branch not protected":
			allowed = true
		case errors.As(err, &ae) && (ae.Status == 403 || ae.Status == 404):
			why = "needs repo admin to read branch protection"
		default:
			why = failWhy(status, err)
		}
	}
	if why != "" && !allowed {
		f.Status = StatusUnknown
		f.Summary = "Could not read whether " + branch + " allows force-pushes (" + why + ")."
	} else {
		f.Status = StatusMissing
		f.Summary = branch + " allows force-pushes."
	}
	f.NotEnforced = "the reputation score can be rewritten, because history on " + branch + " can be force-pushed."
	f.Fix = "block force-pushes on " + branch + " with a ruleset rule or branch protection."
	return f
}
