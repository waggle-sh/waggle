// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"context"
	"strings"
)

// patchIDs reads the stable patch-id of every first-parent commit's diff and of
// its reverse, for manual-revert detection (RFC-0005 5.1 item 2). A commit with
// an empty diff has none.
func (s *scorer) patchIDs(ctx context.Context, main string) error {
	s.pidIdx, s.rpid = map[string][]string{}, map[string]string{}
	if len(s.hist) < 2 {
		return nil
	}
	read := func(reverse bool, add func(pid, oid string)) error {
		args := []string{"log", "--first-parent", "--reverse", "--diff-merges=first-parent", "--no-renames",
			"--no-ext-diff", "--no-color", "--no-prefix", "-p"}
		if reverse {
			args = append(args, "-R")
		}
		patch, err := s.g.run(ctx, nil, append(args, main)...)
		if err != nil {
			return err
		}
		out, err := s.g.run(ctx, patch, "patch-id", "--stable")
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(out), "\n") {
			if f := strings.Fields(line); len(f) == 2 {
				add(f[0], f[1])
			}
		}
		return nil
	}
	if err := read(false, func(pid, oid string) { s.pidIdx[pid] = append(s.pidIdx[pid], oid) }); err != nil {
		return err
	}
	return read(true, func(pid, oid string) { s.rpid[oid] = pid })
}

// manualRevert finds the credited commit a commit with no trailer reverts: the
// newest earlier first-parent commit bound to a unit whose patch-id equals the
// patch-id of this commit's reversed diff.
func (s *scorer) manualRevert(c *commit) []string {
	pid := s.rpid[c.oid]
	if pid == "" {
		return nil
	}
	cands := s.pidIdx[pid]
	for i := len(cands) - 1; i >= 0; i-- {
		if s.credited[cands[i]] && cands[i] != c.oid {
			return []string{cands[i]}
		}
	}
	return nil
}

// unconfirmedRevert reports a commit, or a commit of its merge, whose subject
// reads Revert "..." although no trailer or patch-id names what it reverts
// (RFC-0005 5.1 item 4). It changes no score.
func unconfirmedRevert(c *commit, sides []commit) bool {
	if strings.HasPrefix(c.subject, `Revert "`) {
		return true
	}
	for _, sc := range sides {
		if strings.HasPrefix(sc.subject, `Revert "`) {
			return true
		}
	}
	return false
}

// revertShare is the share of unit u's weighted lines that revert commit x
// undoes through its target t (RFC-0005 4.3): per counted path, the lines x
// removes of what t added plus the lines x restores of what t removed, weighted
// like the unit's lines, over the unit's weighted changed lines. A full revert
// gives 1000.
func (s *scorer) revertShare(ctx context.Context, x *Unit, t string, u *bu) (Milli, error) {
	total := u.Factors.ChangedLines
	tc, xc := s.byOID[t], s.byOID[x.MergedSHA]
	if total == 0 || tc == nil || xc == nil {
		return 1000, nil
	}
	r := &rulesV1
	rt, err := r.countedRows(ctx, s.g, tc)
	if err != nil {
		return 0, err
	}
	rx, err := r.countedRows(ctx, s.g, xc)
	if err != nil {
		return 0, err
	}
	var undone int64
	for p, a := range rt {
		w := int64(1000)
		if a.docs {
			w = int64(rulesV1.docsFactor)
		}
		b := rx[p]
		undone += w * (min(b.deleted, a.added) + min(b.added, a.deleted))
	}
	return min(1000, Milli((undone*1000+int64(total)/2)/int64(total))), nil
}
