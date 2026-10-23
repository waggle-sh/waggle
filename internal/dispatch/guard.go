package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/runtime"
)

var verdictLine = regexp.MustCompile(`(?m)^Verdict: (pass|must-fix)$`)

// parseVerdict is the last line of the guard's final message that matches
// "Verdict: pass" or "Verdict: must-fix", or "" for none. A final of MaxFinal
// bytes or more was cut by the adapter, so its last line proves nothing: it is
// never parsed (10.6 item 12).
func parseVerdict(final []byte) string {
	m := verdictLine.FindAllSubmatch(final, -1)
	if len(m) == 0 || len(final) >= runtime.MaxFinal {
		return ""
	}
	return string(m[len(m)-1][1])
}

const markerPrefix = "<!-- waggle-review"

var (
	markerLine = regexp.MustCompile(`^<!-- waggle-review v1 verdict=(pass|must-fix) head=(?:[0-9a-f]{40}|[0-9a-f]{64}) -->$`)
	hexHead    = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
)

// reviewFile is the content of a run's review.md: the final message with every
// column-0 line that opens like a round marker indented by one space, so the
// guard cannot add a round, and, when a verdict was parsed and head is a valid
// OID, one marker line and a blank line in front (T-033 2.2).
func reviewFile(final []byte, verdict, head string) []byte {
	var out []byte
	if verdict != "" && hexHead.MatchString(head) {
		out = []byte(markerPrefix + " v1 verdict=" + verdict + " head=" + head + " -->\n\n")
	}
	for _, line := range strings.SplitAfter(string(final), "\n") {
		if strings.HasPrefix(line, markerPrefix) {
			out = append(out, ' ')
		}
		out = append(out, line...)
	}
	return out
}

// countReviewMustFix counts the must-fix rounds of one review file. Text before
// the first round marker is legacy and counts by runtime.CountMustFix; after
// it only marker lines count, one per must-fix marker.
func countReviewMustFix(data []byte) int {
	var legacy []byte
	n, seen := 0, false
	for _, line := range strings.SplitAfter(string(data), "\n") {
		m := markerLine.FindStringSubmatch(strings.TrimRight(line, " \r\n"))
		switch {
		case m != nil:
			seen = true
			if m[1] == "must-fix" {
				n++
			}
		case !seen:
			legacy = append(legacy, line...)
		}
	}
	return n + runtime.CountMustFix(legacy)
}

// guardTaskLines replace the commit rules of TaskLines (T-023 10.2). bash is the
// guard's runtime.BashCommands list, without git add or git commit.
func guardTaskLines(req Request, baseOID string, lease comb.Lease, bash []string) []string {
	return []string{
		"Ticket: " + req.Ticket,
		"Role: guard",
		"Branch: " + req.Branch,
		"Base: " + baseOID,
		"Head under review: " + req.Head,
		"Lease directories: " + strings.Join(lease.Directories, ", "),
		"Gate: " + lease.Gate,
		bashLine(bash),
		"Rules: do not commit; write no file in the repo; your final message is the review; delete any probe files before your final message; end with `Verdict: pass` or `Verdict: must-fix` as the last line of your final message.",
	}
}

// guardTarget reads the builder's claim (active or expired; the guard writes
// none) and resolves the branch head once. The review is of that OID.
func guardTarget(ctx context.Context, cb ClaimBackend, g *Git, req Request) (lease comb.Lease, head, claimOID string, err error) {
	if claimOID, err = cb.ClaimOID(ctx, req.Ticket); err != nil && !errors.Is(err, comb.ErrNotFound) {
		return lease, "", "", claimError(err)
	}
	switch lease, err = cb.ReadClaim(ctx, req.Ticket); {
	case errors.Is(err, comb.ErrNotFound):
		return lease, "", "", failf(ExitClaim, "no claim for %s: the guard reads the builder's claim", req.Ticket)
	case err != nil:
		return lease, "", "", claimError(err)
	}
	rev, ok := g.branchRev(ctx, req.Branch)
	if !ok {
		return lease, "", "", failf(ExitUsage, "branch %s does not exist", req.Branch)
	}
	out, err := g.Run(ctx, "rev-parse", "--verify", rev+"^{commit}")
	return lease, strings.TrimSpace(string(out)), claimOID, err
}

// runsDir is where dispatch writes its run records.
func runsDir(repo string) string {
	return filepath.Join(repo, filepath.FromSlash(layout.Worktrees), "runs")
}

// latestRun is the record with the highest <seq> among runs/<ticket>-<role>-<seq>/result.json
// of this ticket whose Second is false and that keep accepts, with its directory
// name. A symlink, an unreadable, oversize or malformed record is skipped.
func latestRun(runs, ticket, role string, keep func(Result) bool) (r Result, name string, ok bool) {
	ents, _ := os.ReadDir(runs)
	best := 0
	for _, e := range ents {
		n, found := strings.CutPrefix(e.Name(), ticket+"-"+role+"-")
		seq, err := strconv.Atoi(n)
		if !found || err != nil || strconv.Itoa(seq) != n || seq <= best || !e.IsDir() {
			continue
		}
		b, rerr := readUpTo(filepath.Join(runs, e.Name(), "result.json"), maxBrief)
		var c Result
		if rerr == nil && json.Unmarshal(b, &c) == nil && c.Ticket == ticket && !c.Second && keep(c) {
			best, r, name, ok = seq, c, e.Name(), true
		}
	}
	return r, name, ok
}

// builderRun is the latest builder record on branch, whatever its end: a
// failed run may still have pushed commits (T-034 2.1).
func builderRun(runs, ticket, branch string) (Result, string, bool) {
	return latestRun(runs, ticket, "builder", func(r Result) bool { return r.Branch == branch })
}

// guardRun is the latest plain guard record on branch with a verdict, whatever
// its end, that keep accepts: a guard that parsed a verdict read the code.
func guardRun(runs, ticket, branch string, keep func(Result) bool) (Result, string, bool) {
	return latestRun(runs, ticket, "guard", func(r Result) bool { return r.Branch == branch && r.Verdict != "" && keep(r) })
}

// guardModel resolves a guard's model from the policy and the run records of
// this ticket on branch, for a review of head (T-034 2.1, 2.2). Every guard
// avoids the builder record's model. A Tier 3 or unknown-tier plain guard also
// avoids the previous round's guard model (D-205), and repeats it with a
// warning only when no other model is left. A second guard avoids this round's
// first guard model and needs both records.
func guardModel(runs string, p runtime.Policy, task runtime.TaskContext, branch, head string, second bool) (runtime.Selection, []string, error) {
	b, bName, okB := builderRun(runs, task.TaskID, branch)
	if okB {
		task.AvoidModels, task.RecordedBuilder = []string{b.Model}, b.Model
	}
	if second {
		f, _, okF := guardRun(runs, task.TaskID, branch, func(r Result) bool { return r.HeadOID == head })
		if !okB || !okF {
			return runtime.Selection{}, nil, secondMissing(task.TaskID, head)
		}
		task.AvoidModels = append(task.AvoidModels, f.Model)
		sel, err := p.Resolve("guard", &task, nil)
		return sel, nil, err
	}
	var warns []string
	prev, _, okP := guardRun(runs, task.TaskID, branch, func(r Result) bool { return r.HeadOID != head })
	alternate := task.Tier == nil || *task.Tier == comb.Tier3ControlPlane
	var sel runtime.Selection
	var err error
	switch {
	case alternate && okP:
		alt := task
		alt.AvoidModels = append(slices.Clone(task.AvoidModels), prev.Model)
		if sel, err = p.Resolve("guard", &alt, nil); errors.Is(err, runtime.ErrNoCrossModel) {
			if sel, err = p.Resolve("guard", &task, nil); err == nil {
				warns = append(warns, fmt.Sprintf("confirm_model_repeated: %s also reviewed %s; no other guard model is available (D-205)", sel.Model, shortOID(prev.HeadOID)))
			}
		}
	case alternate && task.GuardMustFix > 0:
		warns = append(warns, fmt.Sprintf("confirm_model_unknown: no guard run record for %s before %s; the model did not alternate (D-205)", branch, shortOID(head)))
		fallthrough
	default:
		sel, err = p.Resolve("guard", &task, nil)
	}
	if err == nil {
		err = checkBuilderModel(sel.Model, b, bName, okB)
	}
	if err != nil {
		return runtime.Selection{}, nil, err
	}
	return sel, warns, nil
}

// checkBuilderModel refuses a plain guard on the builder record's model. Resolve
// avoids it already; this keeps the rule if that ever changes.
func checkBuilderModel(model string, b Result, name string, ok bool) error {
	if ok && model == b.Model {
		return failf(ExitEnv, "guard would use %s, the model of builder run %s", model, name)
	}
	return nil
}

func secondMissing(ticket, head string) error {
	return failf(ExitEnv, "--second needs a builder run and a first guard run of %s on head %s in %s/runs", ticket, shortOID(head), layout.Worktrees)
}

// secondBase is the base a --second run takes with no --base: the base_oid of
// the latest plain guard record with a verdict on branch, any head (D-254).
// Empty when there is none, and the default base applies.
func secondBase(runs, ticket, branch string) (oid, name string, err error) {
	g, name, ok := guardRun(runs, ticket, branch, func(Result) bool { return true })
	switch {
	case !ok:
		return "", "", nil
	case !hexHead.MatchString(g.BaseOID):
		return "", "", failf(ExitEnv, "--second: run record %s has no valid base_oid", name)
	}
	return g.BaseOID, name, nil
}

// checkSecond is D-188 as T-034 amends it: a second guard runs only after a
// builder run and a first guard run of this ticket on head, on the first
// guard's base exactly (its brief, reviews and must-fix count), with the
// builder's base equal to it or an ancestor of it, and on a model that neither
// record used. No record value reaches git before its hex check.
func checkSecond(ctx context.Context, g *Git, runs, ticket, branch, head, baseOID, model string) error {
	b, bName, okB := builderRun(runs, ticket, branch)
	f, fName, okF := guardRun(runs, ticket, branch, func(r Result) bool { return r.HeadOID == head })
	if !okB || !okF {
		return secondMissing(ticket, head)
	}
	for _, r := range []struct{ name, oid string }{{bName, b.BaseOID}, {fName, f.BaseOID}} {
		if !hexHead.MatchString(r.oid) {
			return failf(ExitEnv, "--second: run record %s has no valid base_oid", r.name)
		}
	}
	if baseOID != f.BaseOID {
		return failf(ExitEnv, "--second: base %s differs from the first guard's base %s; pass --base %s", baseOID, f.BaseOID, f.BaseOID)
	}
	_, err := g.Run(ctx, "merge-base", "--is-ancestor", b.BaseOID, f.BaseOID)
	var ee *exec.ExitError
	switch {
	case errors.As(err, &ee) && ee.ExitCode() == 1:
		return failf(ExitEnv, "--second: builder base %s is not an ancestor of the first guard's base %s; rerun the builder and both guards on one base", b.BaseOID, f.BaseOID)
	case err != nil:
		return failf(ExitEnv, "--second: cannot check builder base %s against %s: %v", b.BaseOID, f.BaseOID, err)
	case model == b.Model || model == f.Model:
		return failf(ExitEnv, "--second would use %s, the model of the builder (%s) or the first guard (%s)", model, b.Model, f.Model)
	}
	return nil
}
