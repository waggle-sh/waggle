// SPDX-License-Identifier: Apache-2.0

// Package quorum holds the Quorum tally core (RFC-0006, T-054 phase A): record
// types, item and ballot validation, Nectar parsing and the integer tally.
// Everything here is a pure function over in-memory values: no git, no file
// I/O and no clock.
package quorum

import (
	"encoding/json"
	"sort"
)

// Version is the schema version of every record.
const Version = 1

// NectarAt pins the Nectar snapshot a round reads (RFC-0006 5.4).
type NectarAt struct {
	MainSHA      string `json:"main_sha"`
	Now          string `json:"now"`
	ReceiptsSHA  string `json:"receipts_sha"`
	RulesVersion int    `json:"rules_version"`
}

// Round is round.json (RFC-0006 4.2, 5.1, 5.4, 6.1). Times are RFC 3339 text
// set by the caller; this package never reads a clock.
type Round struct {
	Version   int      `json:"version"`
	Round     int      `json:"round"`
	Credits   uint64   `json:"credits"`
	RoundDays int      `json:"round_days"`
	Base      string   `json:"base"`
	OpensAt   string   `json:"opens_at"`
	ClosesAt  string   `json:"closes_at"`
	Items     []string `json:"items"`
	NectarAt  NectarAt `json:"nectar_at"`
	// Depends records each item's task dependencies read at open (Q-T054-4).
	// Items without dependencies are left out.
	Depends []Dependency `json:"depends,omitempty"`
}

// Dependency lists the tasks an item waits for.
type Dependency struct {
	Item string   `json:"item"`
	On   []string `json:"on"`
}

// Vote is one item and its whole vote count.
type Vote struct {
	Item  string `json:"item"`
	Votes int64  `json:"votes"`
}

// Ballot is ballot.json (RFC-0006 6.3).
type Ballot struct {
	Version     int    `json:"version"`
	Round       int    `json:"round"`
	KeeperID    string `json:"keeper_id"`
	Votes       []Vote `json:"votes"`
	CreditsUsed uint64 `json:"credits_used"`
	Prev        string `json:"prev"`
}

// Veto is veto.json (RFC-0006 8). Item is an item id or VetoAll. The caller
// has already checked authority and the decision entry (phase C).
type Veto struct {
	Version  int    `json:"version"`
	Round    int    `json:"round"`
	Item     string `json:"item"`
	Decision string `json:"decision"`
}

// VetoAll rejects the whole round so the previous order stands.
const VetoAll = "all"

// ItemScore is one item's line in a tally. Scores are in milli units.
type ItemScore struct {
	Item       string `json:"item"`
	ScoreMilli uint64 `json:"score_milli"`
	RawVotes   uint64 `json:"raw_votes"`
	Vetoed     bool   `json:"vetoed"`
}

// Malformed names an input that was not counted. Reason is a fixed code and
// never carries text from a ballot.
type Malformed struct {
	Ref    string `json:"ref"`
	Reason string `json:"reason"`
}

// TallyRecord is tally.json. Nectar is the parsed keeper_id to milli-Nectar map
// (Q-T054-2), not the raw report. Order lists the ready order after vetoes.
type TallyRecord struct {
	Version  int               `json:"version"`
	Round    int               `json:"round"`
	Credits  uint64            `json:"credits"`
	NectarAt NectarAt          `json:"nectar_at"`
	Nectar   map[string]uint64 `json:"nectar_milli"`
	// Counted lists the commit OIDs of the counted ballots, one per Keeper
	// (the chain tip at close), sorted. They are not keeper_ids.
	Counted   []string    `json:"counted"`
	Items     []ItemScore `json:"items"`
	Order     []string    `json:"order"`
	Vetoes    []Veto      `json:"vetoes"`
	Rejected  bool        `json:"rejected"`
	Malformed []Malformed `json:"malformed"`
	// CloseCommit is the commit whose DECISIONS.md the close read (phase C).
	CloseCommit string `json:"close_commit,omitempty"`
	// Ignored lists vetoes that were not applied, with a fixed reason code.
	Ignored []IgnoredVeto `json:"ignored_vetoes,omitempty"`
}

// IgnoredVeto is a veto record that close did not apply.
type IgnoredVeto struct {
	Item     string `json:"item"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// Encode returns the canonical bytes of a record: two-space indent, a
// trailing newline and every list sorted, so no map or input order reaches
// the output (Q-T054-7). It accepts Round, Ballot and TallyRecord values or
// pointers; any other type is marshaled as given. The input is not modified.
func Encode(record any) []byte {
	switch r := record.(type) {
	case Round:
		record = canonRound(r)
	case *Round:
		record = canonRound(*r)
	case Ballot:
		record = canonBallot(r)
	case *Ballot:
		record = canonBallot(*r)
	case TallyRecord:
		record = canonTally(r)
	case *TallyRecord:
		record = canonTally(*r)
	}
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		// Every record type here is plain strings and integers.
		panic("quorum: encode: " + err.Error())
	}
	return append(b, '\n')
}

func canonRound(r Round) Round {
	items := append([]string{}, r.Items...)
	sort.SliceStable(items, func(i, j int) bool { return itemLess(items[i], items[j]) })
	r.Items = items
	if len(r.Depends) > 0 {
		deps := make([]Dependency, len(r.Depends))
		for i, d := range r.Depends {
			on := append([]string{}, d.On...)
			sort.SliceStable(on, func(a, b int) bool { return itemLess(on[a], on[b]) })
			deps[i] = Dependency{Item: d.Item, On: on}
		}
		sort.SliceStable(deps, func(i, j int) bool { return itemLess(deps[i].Item, deps[j].Item) })
		r.Depends = deps
	}
	return r
}

func canonBallot(b Ballot) Ballot {
	v := append([]Vote{}, b.Votes...)
	sort.SliceStable(v, func(i, j int) bool { return itemLess(v[i].Item, v[j].Item) })
	b.Votes = v
	return b
}

func canonTally(t TallyRecord) TallyRecord {
	t.Counted = append([]string{}, t.Counted...)
	sort.Strings(t.Counted)
	t.Items = append([]ItemScore{}, t.Items...)
	sort.SliceStable(t.Items, func(i, j int) bool { return scoreLess(t.Items[i], t.Items[j]) })
	t.Order = append([]string{}, t.Order...)
	t.Vetoes = append([]Veto{}, t.Vetoes...)
	sort.SliceStable(t.Vetoes, func(i, j int) bool {
		if t.Vetoes[i].Item != t.Vetoes[j].Item {
			return t.Vetoes[i].Item < t.Vetoes[j].Item
		}
		return t.Vetoes[i].Decision < t.Vetoes[j].Decision
	})
	t.Malformed = append([]Malformed{}, t.Malformed...)
	sort.SliceStable(t.Malformed, func(i, j int) bool {
		if t.Malformed[i].Ref != t.Malformed[j].Ref {
			return t.Malformed[i].Ref < t.Malformed[j].Ref
		}
		return t.Malformed[i].Reason < t.Malformed[j].Reason
	})
	t.Ignored = append([]IgnoredVeto(nil), t.Ignored...)
	sort.SliceStable(t.Ignored, func(i, j int) bool {
		a, b := t.Ignored[i], t.Ignored[j]
		if a.Item != b.Item {
			return a.Item < b.Item
		}
		if a.Decision != b.Decision {
			return a.Decision < b.Decision
		}
		return a.Reason < b.Reason
	})
	if t.Nectar == nil {
		t.Nectar = map[string]uint64{}
	}
	return t
}
