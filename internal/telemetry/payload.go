// SPDX-License-Identifier: Apache-2.0

// Package telemetry computes the weekly telemetry payload (docs/telemetry.md,
// schema v1) from local data. It never touches the network and never writes.
// A field whose source is missing is left out, never guessed.
package telemetry

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"time"
)

// SchemaVersion is the payload schema version.
const SchemaVersion = 1

// Pair is a p50 and p90 pair, in rounded hours.
type Pair struct {
	P50 int `json:"p50"`
	P90 int `json:"p90"`
}

// Payload is one weekly report. Field order follows the table in
// docs/telemetry.md. A pointer or omitempty field is omitted when its source
// is missing. The stage, gate count and forge fields have no local source
// today, so Collect never sets them (T-106, T-107, D-405).
type Payload struct {
	ClaimRate         *int   `json:"claim_rate,omitempty"`
	ClaimToMergeHours *Pair  `json:"claim_to_merge_hours,omitempty"`
	Contributors      string `json:"contributors"`
	GateMode          string `json:"gate_mode"`
	InstallID         string `json:"install_id,omitempty"`
	Keepers           string `json:"keepers"`
	PRsMerged         string `json:"prs_merged"`
	ReceiptRate       *int   `json:"receipt_rate,omitempty"`
	ReleasesNinety    int    `json:"releases_90d"`
	Schema            int    `json:"schema"`
	TicketToMerge     *Pair  `json:"ticket_to_merge_hours,omitempty"`
	WaggleVersion     string `json:"waggle_version"`
	Week              string `json:"week"`
}

// Merged is one receipt reduced to what the payload needs. It holds no login,
// path, URL, SHA or ticket text.
type Merged struct {
	// Released is true for a receipt written by a release (not a backfill).
	Released bool
	// ClaimedAt is zero when the receipt records no claim time.
	ClaimedAt, MergedAt time.Time
	// TicketCreated is when the task file first appeared; zero when unknown.
	TicketCreated time.Time
}

// Inputs is everything Compute needs. Collect fills it from git.
type Inputs struct {
	Now          time.Time
	Version      string // build version, for example "0.1.0-dev"
	GateMode     string
	Approvers    int
	Contributors int // distinct authors on the default branch in 90 days
	Releases90d  int
	InstallID    string // "" when no id file exists
	// MergedInWeek are the merged receipts whose merged_at is in the 7 days
	// before Now.
	MergedInWeek []Merged
}

var versionMinor = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)`)

// Minor returns version cut to major.minor, or an error when it has none.
func Minor(version string) (string, error) {
	m := versionMinor.FindStringSubmatch(version)
	if m == nil {
		return "", fmt.Errorf("build version %q has no major.minor", version)
	}
	return m[1] + "." + m[2], nil
}

// Week returns the ISO week of t in UTC, for example 2026-W48.
func Week(t time.Time) string {
	y, w := t.UTC().ISOWeek()
	return fmt.Sprintf("%04d-W%02d", y, w)
}

// PeopleBucket buckets a head count. Zero reads as "1": a hive has at least
// one person.
func PeopleBucket(n int) string {
	switch {
	case n <= 1:
		return "1"
	case n <= 5:
		return "2-5"
	case n <= 20:
		return "6-20"
	}
	return "21+"
}

// CountBucket buckets a count.
func CountBucket(n int) string {
	switch {
	case n <= 0:
		return "0"
	case n <= 5:
		return "1-5"
	case n <= 20:
		return "6-20"
	case n <= 50:
		return "21-50"
	}
	return "51+"
}

var hourEdges = []int{1, 4, 12, 24, 72, 168, 336}

// RoundHours rounds h up to the next of 1, 4, 12, 24, 72, 168 and 336. 336
// means 336 or more.
func RoundHours(h float64) int {
	for _, e := range hourEdges {
		if h <= float64(e) {
			return e
		}
	}
	return 336
}

// percent returns num/den as a percent rounded to the nearest 10.
func percent(num, den int) int {
	return int(math.Round(float64(num)*10/float64(den))) * 10
}

// nearestRank returns the p-th percentile (0 < p <= 1) of sorted values.
func nearestRank(sorted []float64, p float64) float64 {
	i := int(math.Ceil(p*float64(len(sorted)))) - 1
	return sorted[max(i, 0)]
}

// pairOf returns the rounded p50 and p90 of hours, or nil when there are none.
func pairOf(hours []float64) *Pair {
	if len(hours) == 0 {
		return nil
	}
	sort.Float64s(hours)
	return &Pair{P50: RoundHours(nearestRank(hours, 0.5)), P90: RoundHours(nearestRank(hours, 0.9))}
}

// Compute builds the payload. Fields with no source (no merged receipt in the
// week, no install id) are left out.
func Compute(in Inputs) (Payload, error) {
	minor, err := Minor(in.Version)
	if err != nil {
		return Payload{}, err
	}
	p := Payload{
		Schema:         SchemaVersion,
		Week:           Week(in.Now),
		WaggleVersion:  minor,
		GateMode:       in.GateMode,
		Contributors:   PeopleBucket(in.Contributors),
		Keepers:        PeopleBucket(in.Approvers),
		PRsMerged:      CountBucket(len(in.MergedInWeek)),
		ReleasesNinety: min(in.Releases90d, 12),
		InstallID:      in.InstallID,
	}
	var claimToMerge, ticketToMerge []float64
	claimed, released := 0, 0
	for _, m := range in.MergedInWeek {
		if !m.ClaimedAt.IsZero() {
			claimed++
			if d := m.MergedAt.Sub(m.ClaimedAt).Hours(); d >= 0 {
				claimToMerge = append(claimToMerge, d)
			}
		}
		if m.Released {
			released++
		}
		if !m.TicketCreated.IsZero() {
			if d := m.MergedAt.Sub(m.TicketCreated).Hours(); d >= 0 {
				ticketToMerge = append(ticketToMerge, d)
			}
		}
	}
	p.ClaimToMergeHours = pairOf(claimToMerge)
	p.TicketToMerge = pairOf(ticketToMerge)
	if n := len(in.MergedInWeek); n > 0 {
		c, r := percent(claimed, n), percent(released, n)
		p.ClaimRate, p.ReceiptRate = &c, &r
	}
	return p, nil
}

// Missing lists the schema-required fields the payload lacks, so a caller can
// say why it would not be sent. An empty list means every required field is set.
func (p Payload) Missing() []string {
	var m []string
	if p.ClaimRate == nil {
		m = append(m, "claim_rate")
	}
	if p.ClaimToMergeHours == nil {
		m = append(m, "claim_to_merge_hours")
	}
	if p.InstallID == "" {
		m = append(m, "install_id")
	}
	if p.ReceiptRate == nil {
		m = append(m, "receipt_rate")
	}
	if p.TicketToMerge == nil {
		m = append(m, "ticket_to_merge_hours")
	}
	return m
}

// JSON returns the exact bytes `waggle telemetry show` prints: indented, keys
// in payload order, one trailing newline.
func (p Payload) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
