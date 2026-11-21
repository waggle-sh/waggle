// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math/bits"
)

// Reason codes. They are fixed strings and never echo ballot text.
const (
	ReasonDecode        = "decode"
	ReasonVersion       = "version"
	ReasonKeeperID      = "keeper_id"
	ReasonRound         = "round"
	ReasonItem          = "item"
	ReasonUnknownItem   = "unknown_item"
	ReasonDuplicateItem = "duplicate_item"
	ReasonNegativeVote  = "negative_vote"
	ReasonNonInteger    = "non_integer_vote"
	ReasonOverBudget    = "over_budget"
	ReasonCredits       = "credits_used"
	ReasonDuplicateKeep = "duplicate_keeper"
	ReasonVetoRound     = "veto_round"
	ReasonVetoItem      = "veto_item"
)

// ReasonError is an error with a fixed reason code.
type ReasonError struct{ Reason string }

func (e *ReasonError) Error() string { return "quorum: malformed: " + e.Reason }

func reason(code string) error { return &ReasonError{Reason: code} }

// ReasonOf returns the reason code of err, or "" if it has none.
func ReasonOf(err error) string {
	var re *ReasonError
	if errors.As(err, &re) {
		return re.Reason
	}
	return ""
}

// DecodeBallot decodes ballot.json bytes. Vote counts are read from their
// JSON text without a float; a fractional or exponent form is
// non_integer_vote.
func DecodeBallot(data []byte) (Ballot, error) {
	var raw struct {
		Version     int    `json:"version"`
		Round       int    `json:"round"`
		KeeperID    string `json:"keeper_id"`
		CreditsUsed uint64 `json:"credits_used"`
		Prev        string `json:"prev"`
		Votes       []struct {
			Item  string          `json:"item"`
			Votes json.RawMessage `json:"votes"`
		} `json:"votes"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&raw); err != nil {
		return Ballot{}, reason(ReasonDecode)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Ballot{}, reason(ReasonDecode)
	}
	b := Ballot{Version: raw.Version, Round: raw.Round, KeeperID: raw.KeeperID,
		CreditsUsed: raw.CreditsUsed, Prev: raw.Prev, Votes: []Vote{}}
	for _, v := range raw.Votes {
		n, code := parseWhole(string(v.Votes))
		if code != "" {
			return Ballot{}, reason(code)
		}
		b.Votes = append(b.Votes, Vote{Item: v.Item, Votes: n})
	}
	return b, nil
}

// parseWhole reads a JSON number token as a whole int64. A magnitude past
// 2^62 is over_budget: no valid ballot can hold it.
func parseWhole(s string) (int64, string) {
	neg := false
	if s != "" && s[0] == '-' {
		neg, s = true, s[1:]
	}
	if s == "" {
		return 0, ReasonDecode
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' || c == 'e' || c == 'E' {
			return 0, ReasonNonInteger
		}
		if c < '0' || c > '9' {
			return 0, ReasonDecode
		}
		hi, lo := bits.Mul64(n, 10)
		sum, carry := bits.Add64(lo, uint64(c-'0'), 0)
		if hi != 0 || carry != 0 || sum > 1<<62 {
			return 0, ReasonOverBudget
		}
		n = sum
	}
	if neg {
		return -int64(n), ""
	}
	return int64(n), ""
}

// ValidateBallot checks a ballot against its round (RFC-0006 5.1, 6.3). It
// needs no Nectar. The error carries a fixed reason code; read it with
// ReasonOf. Item grammar is checked first.
func ValidateBallot(r Round, b Ballot) error {
	for _, v := range b.Votes {
		if err := CheckItem(v.Item); err != nil {
			return reason(ReasonItem)
		}
	}
	if b.Version != Version {
		return reason(ReasonVersion)
	}
	if !keeperIDPattern.MatchString(b.KeeperID) {
		return reason(ReasonKeeperID)
	}
	if b.Round != r.Round {
		return reason(ReasonRound)
	}
	inSet := make(map[string]bool, len(r.Items))
	for _, it := range r.Items {
		inSet[it] = true
	}
	seen := make(map[string]bool, len(b.Votes))
	var sum uint64
	for _, v := range b.Votes {
		if !inSet[v.Item] {
			return reason(ReasonUnknownItem)
		}
		if seen[v.Item] {
			return reason(ReasonDuplicateItem)
		}
		seen[v.Item] = true
		if v.Votes < 0 {
			return reason(ReasonNegativeVote)
		}
		hi, lo := bits.Mul64(uint64(v.Votes), uint64(v.Votes))
		if hi != 0 {
			return reason(ReasonOverBudget)
		}
		var carry uint64
		sum, carry = bits.Add64(sum, lo, 0)
		if carry != 0 || sum > r.Credits {
			return reason(ReasonOverBudget)
		}
	}
	if b.CreditsUsed != sum {
		return reason(ReasonCredits)
	}
	return nil
}
