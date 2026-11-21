// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/dispatch"
)

// This file is the commit-binding parser of T-051 section 3 step 2, copied from
// internal/dispatch/merged.go (phase A) as the brief allows: internal/dispatch
// is outside this slice's lease. Keep the two the same.

var bindSubject = regexp.MustCompile(`^(T-[0-9]{3,6}): `)

// bindTask is the task one commit binds to by the gate's rules for a subject
// prefix and a Waggle-Ticket trailer: a "Head Keeper: " subject binds none, a
// bad trailer binds none, and a subject and trailer that name different tasks
// bind none. The ticket-branch form needs a PR branch, which main history does
// not hold, so it is not read here.
func bindTask(subject, trailer string) string {
	if strings.HasPrefix(subject, "Head Keeper: ") {
		return ""
	}
	id := ""
	if m := bindSubject.FindStringSubmatch(subject); m != nil {
		id, _ = comb.ParseTicketID(m[1])
	}
	if trailer == "" {
		return id
	}
	if len(trailer) > 256 || strings.Count(trailer, ",")+1 > 8 {
		return ""
	}
	for _, v := range strings.Split(trailer, ",") {
		v = strings.Trim(v, " \t")
		t, err := comb.ParseTicketID(v)
		if err != nil || len(v) > 16 || (id != "" && t != id) {
			return ""
		}
		id = t
	}
	return id
}

const bindLogFormat = "--format=%H%x1f%P%x1f%ct%x1f%s%x1f%(trailers:key=Waggle-Ticket,valueonly,unfold,separator=%x2C)%x1e"

type logCommit struct {
	oid     string
	parents []string
	when    time.Time
	task    string // what the commit binds to by itself
}

func parseLog(out string) ([]logCommit, error) {
	var cs []logCommit
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		f := strings.Split(rec, "\x1f")
		if len(f) != 5 {
			return nil, errors.New("unexpected git log record")
		}
		ct, err := strconv.ParseInt(f[2], 10, 64)
		if err != nil {
			return nil, err
		}
		cs = append(cs, logCommit{oid: f[0], parents: strings.Fields(f[1]), when: time.Unix(ct, 0).UTC(), task: bindTask(f[3], strings.TrimSpace(f[4]))})
	}
	return cs, nil
}

// mergeTask is the task a first-parent commit of main binds to as a unit: its
// own binding, and for a merge commit the one task all of its non-merge
// commits bind to.
func mergeTask(ctx context.Context, g *dispatch.Git, c logCommit) (string, error) {
	if len(c.parents) < 2 {
		return c.task, nil
	}
	out, err := g.Run(ctx, "log", "--no-merges", bindLogFormat, c.parents[0]+".."+c.oid)
	if err != nil {
		return "", err
	}
	side, err := parseLog(string(out))
	if err != nil {
		return "", err
	}
	id := ""
	for _, s := range side {
		if s.task == "" || (id != "" && s.task != id) {
			return "", nil
		}
		id = s.task
	}
	switch {
	case id == "":
		return c.task, nil
	case c.task != "" && c.task != id:
		return "", nil
	}
	return id, nil
}

// firstClaimed is when the claim chain at claimOID began (a renewal or takeover
// chains to the previous claim commit and resets claimed_at). since is the
// fallback.
func firstClaimed(ctx context.Context, g *dispatch.Git, claimOID string, since time.Time) time.Time {
	out, err := g.Run(ctx, "log", "--format=%ct", claimOID)
	if err != nil {
		return since
	}
	for _, s := range strings.Fields(string(out)) {
		if ct, err := strconv.ParseInt(s, 10, 64); err == nil && time.Unix(ct, 0).Before(since) {
			since = time.Unix(ct, 0).UTC()
		}
	}
	return since
}
