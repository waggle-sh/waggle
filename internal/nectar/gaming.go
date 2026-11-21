// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"sort"
	"strings"
	"time"
)

// reciprocity finds Keeper pairs that approve each other's units (RFC-0005 8).
// For each pair, over the pair's units (both owners' units on Main with a known
// approver list), it measures the share approved by the other member. Over
// reciprocity_over every unit of the pair weighs reciprocity_discount; at
// exactly that share, full weight. Pairs with any cross approval are listed.
func (s *scorer) reciprocity(live []*bu) {
	n := map[string]int64{}
	approved := map[string]map[string]int64{} // owner -> approver -> units
	for _, u := range live {
		if u.rc.r.Incomplete {
			continue
		}
		o := strings.ToLower(*u.KeeperID)
		n[o]++
		if approved[o] == nil {
			approved[o] = map[string]int64{}
		}
		seen := map[string]bool{}
		for _, a := range u.rc.r.Approvers {
			if a = strings.ToLower(a); !seen[a] {
				seen[a] = true
				approved[o][a]++
			}
		}
	}
	owners := make([]string, 0, len(n))
	for o := range n {
		owners = append(owners, o)
	}
	sort.Strings(owners)
	over := map[string]bool{}
	for i, a := range owners {
		for _, b := range owners[i+1:] {
			units, k := n[a]+n[b], approved[a][b]+approved[b][a]
			if k == 0 {
				continue
			}
			s.counts.ReciprocalPairs = append(s.counts.ReciprocalPairs, PairShare{a, b, Milli((k*1000 + units/2) / units)})
			if k*1000 > rulesV1.reciprocityOver*units {
				over[a], over[b] = true, true
			}
		}
	}
	for _, u := range live {
		if o := strings.ToLower(*u.KeeperID); over[o] && !u.rc.r.Incomplete {
			u.recip = true
			u.rev = mulMilli(u.rev, rulesV1.reciprocityDiscount)
			u.Reasons = append(u.Reasons, "reciprocal_pair")
		}
	}
}

// finalize sets the unit's final weight from its factors, splits it among its
// Keepers and sets the reverted part (RFC-0005 3.3, 6.4, 4.3).
func (s *scorer) finalize(u *bu) {
	u.weight = mulMilli(u.base, u.rev)
	u.Weight = u.weight
	u.Factors.Review = u.rev
	r := u.rc.r
	n := Milli(1 + len(r.CoKeepers))
	each := u.weight / n
	u.KeeperShare = u.weight - each*Milli(len(r.CoKeepers))
	u.CoKeepers = []Share{}
	for _, c := range r.CoKeepers {
		u.CoKeepers = append(u.CoKeepers, Share{c, each})
	}
	if u.State == StateClawedBack {
		u.clawShare = 1000
	}
	u.ClawedBack = mulMilli(u.weight, u.clawShare)
}

// dailyCap holds back the weight that would mature past nectar.daily_cap per
// Keeper per UTC day of matures_at (RFC-0005 8). Excess carries forward to the
// next days with room, in (merged_at, sha) order, never past the UTC day of now,
// and counts as pending until then. Total credit is unchanged, only delayed.
func (s *scorer) dailyCap(live []*bu) {
	type row struct {
		u    *bu
		who  string
		left Milli
		day  time.Time
	}
	byKeeper := map[string][]*row{}
	for _, u := range live {
		if u.State != StateMatured && u.State != StateClawedBack {
			continue
		}
		day := mustTime(*u.MaturesAt).Truncate(24 * time.Hour)
		for _, sh := range append([]Share{{*u.KeeperID, u.KeeperShare}}, u.CoKeepers...) {
			byKeeper[sh.KeeperID] = append(byKeeper[sh.KeeperID], &row{u: u, who: sh.KeeperID, left: sh.Share, day: day})
		}
	}
	nowDay := s.now.Truncate(24 * time.Hour)
	less := func(a, b *row) bool {
		if !a.u.at.Equal(b.u.at) {
			return a.u.at.Before(b.u.at)
		}
		return a.u.MergedSHA < b.u.MergedSHA
	}
	for _, rows := range byKeeper {
		sort.SliceStable(rows, func(i, j int) bool {
			if !rows[i].day.Equal(rows[j].day) {
				return rows[i].day.Before(rows[j].day)
			}
			return less(rows[i], rows[j])
		})
		var queue []*row
		i := 0
		d := rows[0].day
		for len(queue) > 0 || i < len(rows) {
			if len(queue) == 0 && rows[i].day.After(d) {
				d = rows[i].day
			}
			if d.After(nowDay) {
				break
			}
			for ; i < len(rows) && !rows[i].day.After(d); i++ {
				queue = append(queue, rows[i])
			}
			sort.SliceStable(queue, func(a, b int) bool { return less(queue[a], queue[b]) })
			room := rulesV1.dailyCap
			kept := queue[:0]
			for _, r := range queue {
				take := min(r.left, room)
				r.left -= take
				room -= take
				if r.left > 0 {
					kept = append(kept, r)
				}
			}
			queue = kept
			d = d.Add(24 * time.Hour)
		}
		for _, r := range rows {
			if r.left == 0 {
				continue
			}
			if r.u.carryBy == nil {
				r.u.carryBy = map[string]Milli{}
			}
			r.u.carryBy[r.who] = r.left
			r.u.Carried += r.left
		}
	}
	for _, u := range live {
		if u.Carried == 0 {
			continue
		}
		s.counts.Carried++
		u.Reasons = append(u.Reasons, "daily_cap_carry")
		if u.State == StateMatured && u.Carried >= u.weight {
			u.State = StatePending
		}
	}
}

func mustTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// credit adds one Keeper's part of a unit to that Keeper's totals. Carried
// weight is pending, not matured; a clawback removes it from pending first.
func (s *scorer) credit(u *bu, sh Share, k *KeeperScore) {
	cl := mulDivMilli(sh.Share, u.ClawedBack, u.weight)
	switch u.State {
	case StateDropped:
	case StatePending:
		k.Pending += sh.Share - min(cl, sh.Share)
	default:
		c := min(u.carryBy[sh.KeeperID], sh.Share)
		fromPending := min(cl, c)
		k.Pending += c - fromPending
		k.Matured += sh.Share - c
		k.ClawedBack += min(cl-fromPending, sh.Share-c)
	}
}

// mulDivMilli is a*b/c rounded half up.
func mulDivMilli(a, b, c Milli) Milli {
	if c == 0 {
		return 0
	}
	return Milli((int64(a)*int64(b) + int64(c)/2) / int64(c))
}
