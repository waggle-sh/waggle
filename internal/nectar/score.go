// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// Input fixes every byte of the output: Score reads only the commits Main and
// Receipts name, never a live ref, the worktree, the wall clock or the network.
type Input struct {
	Dir          string // repository directory
	Repo         string // the repo string of RFC-0005 9.2, supplied by the caller
	Main         string // full commit OID of main
	Receipts     string // full commit OID on the receipt chain, or the zero OID
	RulesVersion int
	Now          time.Time
	Recompute    bool // RFC-0005 9.1 --recompute; the phase C cache reads it
}

var revertRe = regexp.MustCompile(`(?m)^This reverts commit ([0-9a-f]{40}|[0-9a-f]{64})[.,]`)

// permuteReceipts is a test seam: it reorders the receipt slice in place after it
// is read, to prove the score depends on each receipt's chain index (rc.idx) and
// not on slice order (T-051 B11).
var permuteReceipts func([]receipt)

// act is one revert commit applied to a unit.
type act struct {
	oid string
	at  time.Time
}

// bu is a unit under construction.
type bu struct {
	*Unit
	merges   []*commit // credited merges in history order
	all      map[string]bool
	mergeSet map[string]bool
	active   []act // reverts in force, oldest first
	restored time.Time
	weight   Milli
	rc       *receipt
}

type scorer struct {
	ctx    context.Context
	g      repo
	now    time.Time
	oidLen int
	hist   []commit
	onLine map[string]int
	rcs    []receipt
	byTask map[string][]*receipt
	cfgAt  map[string]comb.TierPolicy
	counts *Counts
}

// Score computes the report for the given pinned input.
func Score(ctx context.Context, in Input) (Report, error) {
	if in.RulesVersion != RulesVersion1 {
		return Report{}, fmt.Errorf("nectar: unknown rules version %d", in.RulesVersion)
	}
	g := repo{dir: in.Dir}
	if err := g.resolveCommit(ctx, "main", in.Main); err != nil {
		return Report{}, err
	}
	s := &scorer{ctx: ctx, g: g, now: in.Now.UTC(), oidLen: len(in.Main), onLine: map[string]int{},
		byTask: map[string][]*receipt{}, cfgAt: map[string]comb.TierPolicy{}}
	var err error
	if s.rcs, err = g.readReceipts(ctx, in.Receipts, s.oidLen); err != nil {
		return Report{}, err
	}
	if permuteReceipts != nil {
		permuteReceipts(s.rcs)
	}
	if s.hist, err = g.log(ctx, "--first-parent", "--reverse", in.Main); err != nil {
		return Report{}, err
	}
	// The nectar: config block is cut from phase B (T-051 section 9, planned cut B):
	// exclude_keepers is the rules table's empty default until t-051b2.
	excluded := rulesV1.excludeKeepers
	var approvers []string
	if cfg, err := g.configAt(ctx, in.Main); err != nil {
		return Report{}, err
	} else if cfg != nil {
		approvers = cfg.Gate.Approvers
	}
	for i := range s.hist {
		s.onLine[s.hist[i].oid] = i
	}
	for i := range s.rcs {
		rc := &s.rcs[i]
		s.byTask[rc.r.TicketID] = append(s.byTask[rc.r.TicketID], rc)
	}

	rep := Report{Summary: Summary{SchemaVersion: SchemaVersion, RulesVersion: in.RulesVersion, Repo: in.Repo,
		MainSHA: in.Main, ComputedAt: rfc3339(s.now), Keepers: []KeeperScore{}},
		Counts: Counts{Excluded: []ExcludedKeep{}, ReciprocalPairs: []PairShare{}}, Units: []Unit{}}
	s.counts = &rep.Counts
	units, extra, err := s.build(ctx, &rep)
	if err != nil {
		return Report{}, err
	}
	tasks := make([]string, 0, len(units))
	for t := range units {
		tasks = append(tasks, t)
	}
	for t := range s.byTask {
		if units[t] == nil {
			tasks = append(tasks, t)
		}
	}
	sort.Strings(tasks)
	byCommit := map[string]*bu{}
	for _, u := range units {
		for o := range u.all {
			byCommit[o] = u
		}
	}
	s.applyReverts(extra, byCommit)
	keep := map[string]*KeeperScore{}
	excl := map[string]Milli{}
	for _, t := range tasks {
		u := units[t]
		if o, own := s.orphan(t, u); o != nil {
			rep.Counts.Orphaned++
			rep.Summary.Orphaned++
			if own {
				extra = append(extra, o)
			}
			continue
		}
		if u == nil {
			continue
		}
		s.attribute(u, approvers, &rep.Counts)
		s.lifecycle(u)
		if u.KeeperID == nil {
			rep.Counts.Unattributed++
			rep.Summary.Unattributed++
			continue
		}
		shares := append([]Share{{*u.KeeperID, u.KeeperShare}}, u.CoKeepers...)
		for _, sh := range shares {
			if slices.Contains(excluded, sh.KeeperID) {
				excl[sh.KeeperID] += sh.Share
				continue
			}
			k := keep[sh.KeeperID]
			if k == nil {
				k = &KeeperScore{KeeperID: sh.KeeperID}
				keep[sh.KeeperID] = k
			}
			k.Units++
			switch u.State {
			case StatePending:
				k.Pending += sh.Share
			case StateMatured:
				k.Matured += sh.Share
			case StateClawedBack:
				k.Matured += sh.Share
				k.ClawedBack += sh.Share
			}
		}
	}
	for _, k := range keep {
		k.Nectar = max(0, k.Matured-k.ClawedBack)
		rep.Summary.Keepers = append(rep.Summary.Keepers, *k)
	}
	sort.Slice(rep.Summary.Keepers, func(i, j int) bool { return rep.Summary.Keepers[i].KeeperID < rep.Summary.Keepers[j].KeeperID })
	for id, w := range excl {
		rep.Counts.Excluded = append(rep.Counts.Excluded, ExcludedKeep{id, w})
	}
	sort.Slice(rep.Counts.Excluded, func(i, j int) bool { return rep.Counts.Excluded[i].KeeperID < rep.Counts.Excluded[j].KeeperID })
	for _, u := range units {
		rep.Units = append(rep.Units, *u.Unit)
	}
	for _, u := range extra {
		rep.Units = append(rep.Units, *u)
	}
	sort.Slice(rep.Units, func(i, j int) bool {
		a, b := &rep.Units[i], &rep.Units[j]
		if !a.at.Equal(b.at) {
			return a.at.Before(b.at)
		}
		return a.MergedSHA < b.MergedSHA
	})
	return rep, nil
}

func newUnit(task, kind, sha string, at time.Time) *Unit {
	return &Unit{Task: task, Kind: kind, Commits: []string{sha}, MergedSHA: sha, MergedAt: rfc3339(at), CoKeepers: []Share{},
		Factors: Factors{Review: 1000, ExcludedFiles: []string{}}, Reasons: []string{}, at: at}
}

// build walks main's first-parent line once: reverts, units by task and the
// commits no task binds.
func (s *scorer) build(ctx context.Context, rep *Report) (map[string]*bu, []*Unit, error) {
	units := map[string]*bu{}
	var extra []*Unit
	// known holds the commits a revert may name: those of units and revert entries
	// already walked on main's line, never the sides of the merge being read.
	known := map[string]bool{}
	for i := range s.hist {
		c := &s.hist[i]
		var sides []commit
		if len(c.parents) >= 2 {
			var err error
			if sides, err = s.g.log(ctx, "--no-merges", c.parents[0]+".."+c.parents[1]); err != nil {
				return nil, nil, err
			}
		}
		if targets := s.revertTargets(c, sides, known); len(targets) > 0 {
			u := newUnit(Bind(c.subject, c.trailer, ""), KindRevert, c.oid, c.at)
			u.reverts = targets
			known[c.oid] = true
			for _, sc := range sides {
				known[sc.oid] = true
			}
			extra = append(extra, u)
			continue
		}
		task := s.bindCommit(c, sides)
		if task == "" {
			known[c.oid] = true
			for _, sc := range sides {
				known[sc.oid] = true
			}
			u := newUnit("", KindUnit, c.oid, c.at)
			u.State = StateUnbound
			u.Reasons = append(u.Reasons, "no_binding")
			rep.Counts.Unbound++
			extra = append(extra, u)
			continue
		}
		u := units[task]
		if u == nil {
			u = &bu{Unit: newUnit(task, KindUnit, c.oid, c.at), all: map[string]bool{}, mergeSet: map[string]bool{}}
			u.Commits = nil
			units[task] = u
		}
		u.merges = append(u.merges, c)
		u.MergedSHA, u.at, u.MergedAt = c.oid, c.at, rfc3339(c.at)
		u.Commits = append(u.Commits, c.oid)
		u.mergeSet[c.oid], u.all[c.oid] = true, true
		for _, sc := range sides {
			u.all[sc.oid] = true
		}
		for o := range u.all {
			known[o] = true
		}
	}
	for _, u := range units {
		if err := s.weigh(ctx, u); err != nil {
			return nil, nil, err
		}
	}
	return units, extra, nil
}

// revertTargets reads the commits named by "This reverts commit <sha>." (or "<sha>, reversing", as git revert -m writes it) in a
// main commit or, for a merge, in the commits it brought in. Only a target in
// known counts; a sha on a branch only or off main is dropped, and a commit left
// with no target is an ordinary commit.
func (s *scorer) revertTargets(c *commit, sides []commit, known map[string]bool) []string {
	var out []string
	for _, body := range append([]string{c.body}, sideBodies(sides)...) {
		for _, m := range revertRe.FindAllStringSubmatch(body, -1) {
			if len(m[1]) == s.oidLen && known[m[1]] && !slices.Contains(out, m[1]) {
				out = append(out, m[1])
			}
		}
	}
	return out
}

func sideBodies(cs []commit) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.body
	}
	return out
}

// bindCommit binds a main commit to one task by the gate's rules: a conflict,
// a maintainer subject or a commit in a merge that names another task binds
// none. The receipt's head branch is consulted only when nothing else names a task.
func (s *scorer) bindCommit(c *commit, sides []commit) string {
	var ids []string
	named := false
	for _, x := range append([]commit{*c}, sides...) {
		if !explicit(x.subject, x.trailer) {
			continue
		}
		named = true
		id := Bind(x.subject, x.trailer, "")
		if id == "" {
			return ""
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	switch {
	case len(ids) > 1:
		return ""
	case len(ids) == 1:
		return ids[0]
	case named:
		return ""
	}
	var best *receipt
	for i := range s.rcs {
		rc := &s.rcs[i]
		if rc.r.MergedSHA != nil && *rc.r.MergedSHA == c.oid && branchOK(rc) && rc.r.HeadBranch != nil &&
			rc.r.TicketID == BranchTask(*rc.r.HeadBranch) && (best == nil || rc.idx > best.idx) {
			best = rc
		}
	}
	if best == nil {
		return ""
	}
	return BranchTask(*best.r.HeadBranch)
}

func branchOK(rc *receipt) bool {
	return rc.r.HeadBranch == nil || slices.Contains(rc.r.Branches, *rc.r.HeadBranch)
}

// weigh counts the unit's lines and sets its weight (RFC-0005 3.3). All integer
// thousandths.
func (s *scorer) weigh(ctx context.Context, u *bu) error {
	r := &rulesV1
	first := u.merges[0]
	u.BaseSHA = nil
	var policy comb.TierPolicy
	if len(first.parents) > 0 {
		u.BaseSHA = strp(first.parents[0])
		p, ok := s.cfgAt[first.parents[0]]
		if !ok {
			var err error
			if p, err = s.g.tierPolicyAt(ctx, first.parents[0]); err != nil {
				return err
			}
			s.cfgAt[first.parents[0]] = p
		}
		policy = p
	}
	var lc lineCount
	for _, m := range u.merges {
		if err := r.countLines(ctx, s.g, m, policy, &lc); err != nil {
			return err
		}
	}
	changed := lc.code*1000 + lc.docs*int64(r.docsFactor)
	size := r.sizeFactor(changed)
	if lc.added == 0 {
		size = 1000 // a pure deletion weighs as a minimal unit
	}
	slices.Sort(lc.excluded)
	u.weight = mulMilli(r.tierFactor[lc.tier], size)
	u.Weight = u.weight
	u.Factors = Factors{Tier: int(lc.tier), Size: size, Review: 1000, ChangedLines: Milli(changed),
		CodeLines: lc.code, DocsLines: lc.docs, ExcludedFiles: slices.Compact(append([]string{}, lc.excluded...))}
	return nil
}

// applyReverts replays revert commits in history order. A revert of a revert
// restores the units it undid; a later revert of that restore reverts them again.
func (s *scorer) applyReverts(extra []*Unit, byCommit map[string]*bu) {
	type eff struct{ reverts, undoes []*bu }
	effects := map[string]*eff{}
	for _, x := range extra {
		if x.Kind != KindRevert {
			continue
		}
		e := &eff{}
		effects[x.MergedSHA] = e
		hit := func(u *bu) {
			u.active = append(u.active, act{x.MergedSHA, x.at})
			e.reverts = append(e.reverts, u)
		}
		for _, t := range x.reverts {
			prev := effects[t]
			switch u := byCommit[t]; {
			case prev != nil:
				for _, u := range prev.reverts {
					u.active = slices.DeleteFunc(u.active, func(a act) bool { return a.oid == t })
					u.restored = x.at
					e.undoes = append(e.undoes, u)
				}
				for _, u := range prev.undoes {
					hit(u)
				}
			case u != nil:
				hit(u)
			}
		}
	}
}

// orphan finds the task's orphaned record: its newest well-formed merged receipt
// whose merged_sha left main, newer than every valid receipt for the unit. For a
// unit on main it marks the unit; otherwise it returns a unit of its own.
func (s *scorer) orphan(task string, u *bu) (*Unit, bool) {
	maxValid := -1
	var orphan *receipt
	for _, rc := range s.byTask[task] {
		if rc.r.MergedSHA == nil || !branchOK(rc) {
			continue
		}
		if _, on := s.onLine[*rc.r.MergedSHA]; !on {
			if orphan == nil || rc.idx > orphan.idx {
				orphan = rc
			}
		} else if u != nil && u.mergeSet[*rc.r.MergedSHA] {
			maxValid = max(maxValid, rc.idx)
		}
	}
	if orphan == nil || orphan.idx < maxValid {
		return nil, false
	}
	o := u
	if u == nil {
		at := orphan.commit.at
		if orphan.r.MergedAt != nil {
			at = orphan.r.MergedAt.UTC()
		}
		o = &bu{Unit: newUnit(task, KindUnit, *orphan.r.MergedSHA, at)}
		o.BaseSHA = orphan.r.BaseSHA
		o.KeeperID = strp(orphan.r.KeeperID)
	}
	o.State = StateOrphaned
	o.Receipt = receiptInfo(orphan)
	reason := "orphaned_missing"
	if orphan.r.BaseSHA != nil {
		if _, on := s.onLine[*orphan.r.BaseSHA]; on {
			o.Rewritten, reason = true, "orphaned_rewritten"
		}
	}
	o.Missing = !o.Rewritten
	o.Reasons = append(o.Reasons, reason)
	return o.Unit, u == nil
}

func receiptInfo(rc *receipt) *ReceiptInfo {
	a := append([]string{}, rc.r.Approvers...)
	return &ReceiptInfo{OID: rc.oid, Source: rc.r.Source, Status: rc.r.Status, MergeProof: rc.r.MergeProof,
		Incomplete: rc.r.Incomplete, ReleasedBy: rc.r.ReleasedBy, PRNumber: rc.r.PRNumber, PRAuthor: rc.r.PRAuthor,
		HeadBranch: rc.r.HeadBranch, Approvers: a}
}

// attribute picks the receipt that credits the unit and splits its weight
// (RFC-0005 6.1 to 6.4).
func (s *scorer) attribute(u *bu, approvers []string, counts *Counts) {
	last := u.merges[len(u.merges)-1].oid
	var best *receipt
	// Naming the completing commit beats naming an earlier merge; among those a
	// release beats gate_run and backfill; then the newest by receipt order.
	rank := func(rc *receipt) int {
		switch {
		case *rc.r.MergedSHA != last:
			return 0
		case rc.r.Source == comb.ReceiptSourceRelease:
			return 2
		}
		return 1
	}
	unmerged := false
	for _, rc := range s.byTask[u.Task] {
		if rc.r.MergedSHA == nil {
			unmerged = true
			continue
		}
		if !u.mergeSet[*rc.r.MergedSHA] || !branchOK(rc) {
			continue
		}
		if best == nil || rank(rc) > rank(best) || rank(rc) == rank(best) && rc.idx > best.idx {
			best = rc
		}
	}
	if best == nil {
		if unmerged {
			counts.Unmerged++
			u.Reasons = append(u.Reasons, "receipt_unmerged")
		}
		u.State = StateUnattributed
		u.Reasons = append(u.Reasons, "no_valid_receipt")
		return
	}
	u.rc = best
	r := best.r
	u.Receipt = receiptInfo(best)
	u.KeeperID = strp(r.KeeperID)
	n := Milli(1 + len(r.CoKeepers))
	each := u.weight / n
	u.KeeperShare = u.weight - each*Milli(len(r.CoKeepers))
	for _, c := range r.CoKeepers {
		u.CoKeepers = append(u.CoKeepers, Share{c, each})
	}
	if best.thirdParty() {
		u.ThirdParty = true
		counts.ThirdParty++
		u.Reasons = append(u.Reasons, "third_party")
	}
	for _, a := range approvers {
		if strings.EqualFold(a, r.ReleasedBy) {
			u.Maintainer = true
		}
	}
	if rel, err := s.g.isRelay(s.ctx, best.claimOID); err == nil && rel {
		u.Relay = true
	}
	if r.PRAuthor != nil && *r.PRAuthor != r.KeeperID {
		u.ForkMismatch = true
		counts.ForkMismatch++
		u.Reasons = append(u.Reasons, "fork_mismatch")
	}
	if r.Incomplete {
		counts.Incomplete++
		u.Reasons = append(u.Reasons, "receipt_incomplete")
	}
}

// lifecycle sets the window and the state (RFC-0005 4.1, 5.2). An orphaned or
// unattributed unit keeps its state; a unit is direct-push unless a valid
// receipt for it carries a pr_number.
func (s *scorer) lifecycle(u *bu) {
	r := &rulesV1
	if u.State == StateOrphaned || u.State == StateUnattributed {
		return
	}
	last := u.merges[len(u.merges)-1]
	mergedAt := last.at
	start := mergedAt
	if !u.restored.After(last.at) {
		direct := true
		var seen *time.Time
		seenIdx := int(^uint(0) >> 1)
		for _, rc := range s.byTask[u.Task] {
			if rc.r.MergedSHA == nil || !u.mergeSet[*rc.r.MergedSHA] || !branchOK(rc) {
				continue
			}
			if rc.r.PRNumber != nil {
				direct = false
			}
			if *rc.r.MergedSHA == last.oid && rc.idx < seenIdx {
				t := rc.commit.at
				seen, seenIdx = &t, rc.idx
			}
		}
		if direct {
			u.DirectPush = true
			s.counts.DirectPush++
			u.Reasons = append(u.Reasons, "direct_push")
			if seen != nil {
				if seen.After(start) {
					start = *seen
				}
				u.FirstSeen = strp(rfc3339(*seen))
			}
		}
	} else {
		mergedAt, start = u.restored, u.restored
		u.Reasons = append(u.Reasons, "restored")
	}
	u.MergedAt = rfc3339(mergedAt)
	u.at = mergedAt
	matures := start.Add(r.window)
	u.MaturesAt = strp(rfc3339(matures))
	switch {
	case len(u.active) > 0 && u.active[0].at.Before(matures):
		u.State = StateDropped
		u.Reasons = append(u.Reasons, "revert_before_maturity")
	case len(u.active) > 0:
		u.State = StateClawedBack
		u.ClawedBack = u.weight
		u.Reasons = append(u.Reasons, "revert_after_maturity")
	case s.now.Before(matures):
		u.State = StatePending
	default:
		u.State = StateMatured
	}
}
