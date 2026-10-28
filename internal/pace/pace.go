// SPDX-License-Identifier: Apache-2.0

// Package pace holds agent pace profiles (T-043): the owner's bounds from
// config.yaml, a keeper's pick from local.yaml, their resolution into four raw
// values, the system load reading and the per-tool render. It imports nothing
// from internal/config.
package pace

import (
	"fmt"
	"strings"
	"unicode"
)

// Profile is a pace word. A higher value is a faster pace.
type Profile int

const (
	Serial Profile = iota
	Steady
	Max
)

var profileWords = [...]string{"serial", "steady", "max"}

func (p Profile) String() string {
	if p < Serial || p > Max {
		return "unknown"
	}
	return profileWords[p]
}

// ParseProfile matches a profile word exactly.
func ParseProfile(s string) (Profile, bool) {
	for i, w := range profileWords {
		if s == w {
			return Profile(i), true
		}
	}
	return 0, false
}

// Settings are the four raw values of one profile. Profile is set on a
// resolved Settings and is the zero value on a table entry.
type Settings struct {
	Profile          Profile
	ParallelRuns     int // agent sessions or dispatch runs at once
	Subagents        int // subagents one session runs at once
	CPUTargetPercent int // load ceiling a new run must start under
	MemoryFloorMiB   int // available memory a new run needs
}

// Pace is the owner's bounds. Set is false when config.yaml has no pace: section.
type Pace struct {
	Set     bool
	Profile Profile // default for a keeper who picks none
	Limit   Profile // the fastest profile a keeper may pick
	Table   [3]Settings
}

// Defaults is the profile table of section 2.4.
func Defaults() [3]Settings {
	return [3]Settings{
		Serial: {ParallelRuns: 1, Subagents: 1, CPUTargetPercent: 50, MemoryFloorMiB: 4096},
		Steady: {ParallelRuns: 2, Subagents: 3, CPUTargetPercent: 70, MemoryFloorMiB: 2048},
		Max:    {ParallelRuns: 4, Subagents: 8, CPUTargetPercent: 85, MemoryFloorMiB: 1024},
	}
}

// Warning kinds.
const (
	KindClamped     = "clamped"
	KindUnknownPick = "unknown_pick"
)

// Warning is a Resolve note. Callers map Kind, never Message text.
type Warning struct {
	Kind    string
	Message string
}

// Resolve applies a keeper's pick (the empty string means none) within the
// owner's bounds. It is pure and never returns a value faster than owner.Limit.
func Resolve(owner Pace, pick string) (Settings, []Warning) {
	chosen := owner.Profile
	var warns []Warning
	if pick != "" {
		p, ok := ParseProfile(pick)
		switch {
		case !ok:
			warns = append(warns, Warning{KindUnknownPick, fmt.Sprintf(
				"pace: local.yaml profile %s is not serial, steady or max; using %s", Printable(pick), owner.Profile)})
		case p > owner.Limit:
			warns = append(warns, Warning{KindClamped, fmt.Sprintf(
				"pace: local.yaml picks %s, above the owner's limit %s; using %s", p, owner.Limit, owner.Limit)})
			chosen = owner.Limit
		default:
			chosen = p
		}
	}
	if chosen > owner.Limit {
		chosen = owner.Limit
	}
	s := owner.Table[chosen]
	s.Profile = chosen
	return s, warns
}

// Printable drops control, format and line or paragraph separator characters
// and caps s at 100 bytes, so a value from local.yaml never reaches output raw.
func Printable(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			return -1
		}
		return r
	}, s)
	return strings.ToValidUTF8(s[:min(len(s), 100)], "")
}

// Line is the shared pace line of status, doctor and waggle pace.
func Line(owner Pace, s Settings) string {
	return fmt.Sprintf("pace: %s (default %s, limit %s): %d runs, %d subagents, CPU target %d%%, memory floor %d MiB",
		s.Profile, owner.Profile, owner.Limit, s.ParallelRuns, s.Subagents, s.CPUTargetPercent, s.MemoryFloorMiB)
}
