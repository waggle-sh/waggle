// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"encoding/json"
	"time"
)

// SchemaVersion is the RFC-0005 9.2 schema; DetailVersion is the detail view's.
const (
	SchemaVersion = 1
	DetailVersion = 1
)

// Unit states.
const (
	StatePending      = "pending"
	StateMatured      = "matured"
	StateClawedBack   = "clawed_back"
	StateDropped      = "dropped"
	StateOrphaned     = "orphaned"
	StateUnbound      = "unbound"
	StateUnattributed = "unattributed"
)

// Unit kinds.
const (
	KindUnit   = "unit"
	KindRevert = "revert"
)

// Report is the result of Score. json.Marshal gives the RFC-0005 9.2 summary
// and nothing else; Detail gives the counts and units.
type Report struct {
	Summary Summary
	Counts  Counts
	Units   []Unit
}

// MarshalJSON returns the summary, so T-052's --json and T-054's Nectar source
// read the same bytes.
func (r Report) MarshalJSON() ([]byte, error) { return json.Marshal(r.Summary) }

// Detail marshals the counts and units under detail_version 1.
func (r Report) Detail() ([]byte, error) {
	return json.MarshalIndent(struct {
		DetailVersion int    `json:"detail_version"`
		Counts        Counts `json:"counts"`
		Units         []Unit `json:"units"`
	}{DetailVersion, r.Counts, r.Units}, "", "  ")
}

// Summary is RFC-0005 9.2.
type Summary struct {
	SchemaVersion int           `json:"schema_version"`
	RulesVersion  int           `json:"rules_version"`
	Repo          string        `json:"repo"`
	MainSHA       string        `json:"main_sha"`
	ComputedAt    string        `json:"computed_at"`
	Keepers       []KeeperScore `json:"keepers"`
	Unattributed  int           `json:"unattributed"`
	Orphaned      int           `json:"orphaned"`
}

// KeeperScore is one Keeper's totals. Nectar is max(0, Matured - ClawedBack);
// Pending is never part of it.
type KeeperScore struct {
	KeeperID   string `json:"keeper_id"`
	Nectar     Milli  `json:"nectar"`
	Matured    Milli  `json:"matured"`
	Pending    Milli  `json:"pending"`
	ClawedBack Milli  `json:"clawed_back"`
	Units      int    `json:"units"`
}

// Counts lists every rule that gave a unit less than full credit or none
// (RFC-0005 section 1, "Nothing is excluded silently"). Counts that phase C
// builds are 0 here.
type Counts struct {
	Unbound           int            `json:"unbound"`
	Unattributed      int            `json:"unattributed"`
	Orphaned          int            `json:"orphaned"`
	Unmerged          int            `json:"unmerged"`
	Incomplete        int            `json:"incomplete"`
	ThirdParty        int            `json:"third_party"`
	ForkMismatch      int            `json:"fork_mismatch"`
	DirectPush        int            `json:"direct_push"`
	SelfMerge         int            `json:"self_merge"`
	Carried           int            `json:"carried"`
	UnconfirmedRevert int            `json:"unconfirmed_revert"`
	Excluded          []ExcludedKeep `json:"excluded"`
	ReciprocalPairs   []PairShare    `json:"reciprocal_pairs"`
}

// ExcludedKeep is a keeper dropped by nectar.exclude_keepers and its weight.
type ExcludedKeep struct {
	KeeperID string `json:"keeper_id"`
	Weight   Milli  `json:"weight"`
}

// PairShare is a Keeper pair and the share of its units each approved (phase C).
type PairShare struct {
	A     string `json:"a"`
	B     string `json:"b"`
	Share Milli  `json:"share"`
}

// Share is one Keeper's part of a unit's weight.
type Share struct {
	KeeperID string `json:"keeper_id"`
	Share    Milli  `json:"share"`
}

// Factors are the parts of a unit's weight (RFC-0005 3.3).
type Factors struct {
	Tier          int      `json:"tier"`
	Size          Milli    `json:"size"`
	Review        Milli    `json:"review"`
	ChangedLines  Milli    `json:"changed_lines"`
	CodeLines     int64    `json:"code_lines"`
	DocsLines     int64    `json:"docs_lines"`
	ExcludedFiles []string `json:"excluded_files"`
}

// ReceiptInfo is the receipt chosen for a unit.
type ReceiptInfo struct {
	OID        string   `json:"oid"`
	Source     string   `json:"source"`
	Status     string   `json:"status"`
	MergeProof string   `json:"merge_proof"`
	Incomplete bool     `json:"incomplete"`
	ReleasedBy string   `json:"released_by"`
	PRNumber   *int     `json:"pr_number"`
	PRAuthor   *string  `json:"pr_author"`
	HeadBranch *string  `json:"head_branch"`
	Approvers  []string `json:"approvers"`
}

// Unit is one unit of credit, one revert commit, or one commit with no task.
// A revert has an empty state: it earns nothing and has no window.
type Unit struct {
	Task         string       `json:"task"`
	Kind         string       `json:"kind"`
	Commits      []string     `json:"commits"`
	MergedSHA    string       `json:"merged_sha"`
	BaseSHA      *string      `json:"base_sha"`
	MergedAt     string       `json:"merged_at"`
	FirstSeen    *string      `json:"first_seen"`
	MaturesAt    *string      `json:"matures_at"`
	State        string       `json:"state"`
	KeeperID     *string      `json:"keeper_id"`
	KeeperShare  Milli        `json:"keeper_share"`
	CoKeepers    []Share      `json:"co_keepers"`
	Weight       Milli        `json:"weight"`
	Factors      Factors      `json:"factors"`
	AliveRatio   *Milli       `json:"alive_ratio"`
	ClawedBack   Milli        `json:"clawed_back"`
	Carried      Milli        `json:"carried"`
	Receipt      *ReceiptInfo `json:"receipt"`
	ThirdParty   bool         `json:"third_party"`
	Maintainer   bool         `json:"maintainer"`
	Relay        bool         `json:"relay"`
	ForkMismatch bool         `json:"fork_mismatch"`
	DirectPush   bool         `json:"direct_push"`
	SelfMerge    bool         `json:"self_merge"`
	Rewritten    bool         `json:"rewritten"`
	Missing      bool         `json:"missing"`
	Reasons      []string     `json:"reasons"`

	at      time.Time // merged_at, for ordering
	reverts []string  // revert kind: target commit OIDs
}

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func strp(s string) *string { return &s }
