// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"errors"
	"fmt"
	"math/bits"
	"sort"
)

// ErrOverflow reports a score or weight that does not fit in uint64. It is
// malformed input and stops the tally; nothing wraps.
var ErrOverflow = errors.New("quorum: tally overflow")

// Tally computes tally.json for a round (RFC-0006 5.2, 5.3, 5.5, 8).
//
// nectar is the parsed keeper_id to milli-Nectar map from ParseNectar; a
// keeper absent from it has weight 0. Ballots that fail ValidateBallot, and
// every ballot of a keeper_id seen twice, are listed in Malformed and not
// counted. Vetoes of another round or of an item outside the set are listed
// and ignored; a valid veto never changes a score. Input order does not
// matter: the result is canonical.
func Tally(r Round, ballots []Ballot, nectar map[string]uint64, vetoes []Veto) (TallyRecord, error) {
	t := TallyRecord{
		Version:   Version,
		Round:     r.Round,
		Credits:   r.Credits,
		NectarAt:  r.NectarAt,
		Nectar:    map[string]uint64{},
		Counted:   []string{},
		Items:     []ItemScore{},
		Order:     []string{},
		Vetoes:    []Veto{},
		Malformed: []Malformed{},
	}
	for k, m := range nectar {
		t.Nectar[k] = m
	}

	scores := make(map[string]*ItemScore, len(r.Items))
	for _, it := range r.Items {
		if CheckItem(it) != nil {
			return TallyRecord{}, fmt.Errorf("quorum: round holds an invalid item")
		}
		if scores[it] == nil {
			scores[it] = &ItemScore{Item: it}
		}
	}

	count := map[string]int{}
	for _, b := range ballots {
		count[b.KeeperID]++
	}
	for _, b := range ballots {
		ref := ballotRef(b)
		if count[b.KeeperID] > 1 {
			t.Malformed = append(t.Malformed, Malformed{Ref: ref, Reason: ReasonDuplicateKeep})
			continue
		}
		if err := ValidateBallot(r, b); err != nil {
			t.Malformed = append(t.Malformed, Malformed{Ref: ref, Reason: ReasonOf(err)})
			continue
		}
		w, ok := weightMilli(nectar[b.KeeperID])
		if !ok {
			return TallyRecord{}, ErrOverflow
		}
		for _, v := range b.Votes {
			s := scores[v.Item]
			hi, lo := bits.Mul64(w, uint64(v.Votes))
			if hi != 0 {
				return TallyRecord{}, ErrOverflow
			}
			var carry uint64
			s.ScoreMilli, carry = bits.Add64(s.ScoreMilli, lo, 0)
			if carry != 0 {
				return TallyRecord{}, ErrOverflow
			}
			s.RawVotes += uint64(v.Votes) // bounded by credits, cannot overflow in practice
		}
		t.Counted = append(t.Counted, b.KeeperID)
	}

	vetoed := map[string]bool{}
	for _, v := range vetoes {
		ref := vetoRef(v)
		switch {
		case v.Round != r.Round:
			t.Malformed = append(t.Malformed, Malformed{Ref: ref, Reason: ReasonVetoRound})
		case v.Item == VetoAll:
			t.Rejected = true
			t.Vetoes = append(t.Vetoes, v)
		case scores[v.Item] == nil:
			t.Malformed = append(t.Malformed, Malformed{Ref: ref, Reason: ReasonVetoItem})
		default:
			vetoed[v.Item] = true
			t.Vetoes = append(t.Vetoes, v)
		}
	}

	for _, s := range scores {
		s.Vetoed = vetoed[s.Item]
		t.Items = append(t.Items, *s)
	}
	sort.Slice(t.Items, func(i, j int) bool { return scoreLess(t.Items[i], t.Items[j]) })
	if !t.Rejected {
		for _, s := range t.Items {
			if !s.Vetoed {
				t.Order = append(t.Order, s.Item)
			}
		}
	}
	return canonTally(t), nil
}

// ballotRef names a ballot in a malformed list. A keeper_id that fails the
// pattern is never echoed. The ref depends only on the ballot, never on its
// input position, so shuffled input gives the same bytes.
func ballotRef(b Ballot) string {
	if keeperIDPattern.MatchString(b.KeeperID) {
		return "ballot:" + b.KeeperID
	}
	return "ballot:invalid_keeper_id"
}

// vetoRef names a veto in a malformed list by round and item. An item that
// fails the grammar is never echoed.
func vetoRef(v Veto) string {
	item := v.Item
	if item != VetoAll && CheckItem(item) != nil {
		item = "invalid_item"
	}
	return fmt.Sprintf("veto:round-%d:%s", v.Round, item)
}
