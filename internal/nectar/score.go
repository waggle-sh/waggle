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

// act is one revert commit applied to a unit; share is the part of the unit's
// weighted lines it undoes, 1000 for a full revert (RFC-0005 4.3).
type act struct {
	oid   string
	at    time.Time
	share Milli
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

	lc            lineCount
	base, rev     Milli // tier * size, and the review factor (self-merge, reciprocity)
	baseApprovers []string
	clawShare     Milli            // share of the weight reverted, 0 to 1000
	carryBy       map[string]Milli // per Keeper, weight held back by the daily cap
	recip         bool
	key           string // cache key: the task and its credited merges
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
	cfgAt  map[string]baseCfg
	counts *Counts

	byOID    map[string]*commit // main's line and the sides of its merges
	credited map[string]bool    // first-parent commits bound to a unit
	supers   map[string]int     // task -> newest history index of a commit that names it in Waggle-Supersedes
	rpid     map[string]string  // commit -> patch-id of its reversed diff
	pidIdx   map[string][]string
	cache    *nectarCache
	ignore   map[string]string         // survival commit -> blame ignore-revs file, "" for none
	steps    map[int]map[string]string // history index -> renames that commit made
	blames   map[string]map[string]int64
}

// baseCfg is the config read at a unit's base commit.
type baseCfg struct {
	policy    comb.TierPolicy
	approvers []string
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
		byTask: map[string][]*receipt{}, cfgAt: map[string]baseCfg{}, byOID: map[string]*commit{},
		credited: map[string]bool{}, supers: map[string]int{}, ignore: map[string]string{}, steps: map[int]map[string]string{}, blames: map[string]map[string]int64{}}
	defer s.cleanup()
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
		s.byOID[s.hist[i].oid] = &s.hist[i]
	}
	s.cache = loadCache(ctx, g, in)
	if err := s.patchIDs(ctx, in.Main); err != nil {
		return Report{}, err
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
	if err := s.applyReverts(ctx, extra, byCommit); err != nil {
		return Report{}, err
	}
	var live []*bu
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
		if err := s.lifecycle(ctx, u); err != nil {
			return Report{}, err
		}
		if u.KeeperID == nil {
			rep.Counts.Unattributed++
			rep.Summary.Unattributed++
			continue
		}
		live = append(live, u)
	}
	s.reciprocity(live)
	for _, u := range live {
		s.finalize(u)
	}
	s.dailyCap(live)
	keep := map[string]*KeeperScore{}
	excl := map[string]Milli{}
	for _, u := range live {
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
			s.credit(u, sh, k)
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
	s.saveCache(in)
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
		for j := range sides {
			s.byOID[sides[j].oid] = &sides[j]
		}
		targets := s.revertTargets(c, sides, known)
		if len(targets) == 0 {
			targets = s.manualRevert(c)
		}
		if len(targets) > 0 {
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
		unconfirmed := unconfirmedRevert(c, sides)
		if unconfirmed {
			rep.Counts.UnconfirmedRevert++
		}
		if task == "" {
			known[c.oid] = true
			for _, sc := range sides {
				known[sc.oid] = true
			}
			u := newUnit("", KindUnit, c.oid, c.at)
			u.State = StateUnbound
			u.Reasons = append(u.Reasons, "no_binding")
			if unconfirmed {
				u.Reasons = append(u.Reasons, "unconfirmed_revert")
			}
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
		if unconfirmed {
			u.Reasons = append(u.Reasons, "unconfirmed_revert")
		}
		s.credited[c.oid] = true
		s.noteSupersedes(i, c, sides)
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
		bc, ok := s.cfgAt[first.parents[0]]
		if !ok {
			// A missing, oversized or unparsable file at an old base means no extra
			// paths and no approvers, as an absent file does for the gate.
			if cfg, err := s.g.configAt(ctx, first.parents[0]); err == nil && cfg != nil {
				bc = baseCfg{comb.TierPolicy{ExtraTier3: cfg.Gate.ProtectedPaths}, cfg.Gate.Approvers}
			}
			s.cfgAt[first.parents[0]] = bc
		}
		policy, u.baseApprovers = bc.policy, bc.approvers
	}
	u.key = unitKey(u)
	lc, hit := s.cache.lines(u.key)
	if !hit {
		for _, m := range u.merges {
			if err := r.countLines(ctx, s.g, m, policy, &lc); err != nil {
				return err
			}
		}
		slices.Sort(lc.excluded)
		slices.Sort(lc.paths)
		lc.paths = slices.Compact(lc.paths)
		s.cache.putLines(u.key, lc)
	}
	u.lc = lc
	changed := lc.code*1000 + lc.docs*int64(r.docsFactor)
	size := r.sizeFactor(changed)
	switch {
	case lc.added == 0:
		size = 1000 // a pure deletion weighs as a minimal unit
	case r.minChangedLines > 0 && changed < r.minChangedLines*1000:
		size = 1000 // under nectar.min_changed_lines: minimal, never zero
		u.Reasons = append(u.Reasons, "min_changed_lines")
	}
	u.base, u.rev = mulMilli(r.tierFactor[lc.tier], size), 1000
	u.weight = u.base
	u.Weight = u.weight
	u.Factors = Factors{Tier: int(lc.tier), Size: size, Review: 1000, ChangedLines: Milli(changed),
		CodeLines: lc.code, DocsLines: lc.docs, ExcludedFiles: slices.Compact(append([]string{}, lc.excluded...))}
	return nil
}

// applyReverts replays revert commits in history order. A revert of a revert
// restores the units it undid, each at the share it had; a later revert of that
// restore reverts them again.
func (s *scorer) applyReverts(ctx context.Context, extra []*Unit, byCommit map[string]*bu) error {
	type hit struct {
		u     *bu
		share Milli
	}
	type eff struct{ reverts, undoes []hit }
	effects := map[string]*eff{}
	for _, x := range extra {
		if x.Kind != KindRevert {
			continue
		}
		e := &eff{}
		effects[x.MergedSHA] = e
		apply := func(u *bu, share Milli) {
			u.active = append(u.active, act{x.MergedSHA, x.at, share})
			e.reverts = append(e.reverts, hit{u, share})
		}
		// A rebase run is reverted commit by commit; each target claws back its
		// share by line, so the shares of one revert on one unit add up.
		shares := map[*bu]Milli{}
		var order []*bu
		for _, t := range x.reverts {
			prev := effects[t]
			switch u := byCommit[t]; {
			case prev != nil:
				for _, h := range prev.reverts {
					h.u.active = slices.DeleteFunc(h.u.active, func(a act) bool { return a.oid == t })
					h.u.restored = x.at
					h.u.all[x.MergedSHA] = true // the re-landing commit owns the restored lines
					e.undoes = append(e.undoes, h)
				}
				for _, h := range prev.undoes {
					apply(h.u, h.share)
				}
			case u != nil:
				sh, err := s.revertShare(ctx, x, t, u)
				if err != nil {
					return err
				}
				if _, seen := shares[u]; !seen {
					order = append(order, u)
				}
				shares[u] += sh
			}
		}
		for _, u := range order {
			apply(u, min(shares[u], 1000))
		}
	}
	return nil
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
		// Approval is unknown, so the review factor stays 1 (Q-T051-2 (a)); the
		// unit is counted and listed.
		counts.Incomplete++
		u.Reasons = append(u.Reasons, "receipt_incomplete")
	} else if selfMerge(r) && multiKeeper(u.baseApprovers) {
		u.SelfMerge = true
		u.rev = mulMilli(u.rev, rulesV1.selfMergeFactor)
		counts.SelfMerge++
		u.Reasons = append(u.Reasons, "self_merge_multi_keeper")
	}
}

// selfMerge reports whether the receipt's approvers hold no login other than its
// keeper_id (RFC-0005 8, T-051 finding F7).
func selfMerge(r comb.Receipt) bool {
	return !slices.ContainsFunc(r.Approvers, func(a string) bool { return !strings.EqualFold(a, r.KeeperID) })
}

// multiKeeper reports whether the approver list at the unit's base has more than
// one distinct login; at most one is the default solo case, which counts a
// self-merge at full weight.
func multiKeeper(approvers []string) bool {
	seen := map[string]bool{}
	for _, a := range approvers {
		seen[strings.ToLower(a)] = true
	}
	return len(seen) > 1
}

// lifecycle sets the window and the state (RFC-0005 4.1, 5.2). An orphaned or
// unattributed unit keeps its state; a unit is direct-push unless a valid
// receipt for it carries a pr_number.
func (s *scorer) lifecycle(ctx context.Context, u *bu) error {
	r := &rulesV1
	if u.State == StateOrphaned || u.State == StateUnattributed {
		return nil
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
	var before, after Milli
	for _, a := range u.active {
		if a.at.Before(matures) {
			before += a.share
		} else {
			after += a.share
		}
	}
	before = min(before, 1000)
	after = min(after, 1000-before)
	partial := before + after
	u.clawShare = partial
	switch {
	case before >= 1000:
		u.State = StateDropped
		u.clawShare = 0
		u.Reasons = append(u.Reasons, "revert_before_maturity")
		return nil
	case s.now.Before(matures):
		u.State = StatePending
	case partial >= 1000:
		u.State = StateClawedBack
	default:
		u.State = StateMatured
	}
	if partial >= 1000 && u.State == StateClawedBack {
		u.Reasons = append(u.Reasons, "revert_after_maturity")
	} else if partial > 0 {
		u.Reasons = append(u.Reasons, "partial_revert")
	}
	if u.State == StatePending {
		return nil
	}
	return s.survive(ctx, u, matures)
}
