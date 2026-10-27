// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"errors"
	"fmt"
)

// Profile is a context profile: how much context and how many turns a role
// gets on a task. It never chooses a model or level (T-012 owns those) and
// never skips or weakens the guard. The zero value is invalid.
type Profile int

// The three profiles, least to most generous. ProfileBalanced is the default.
const (
	ProfileLean     Profile = 1
	ProfileBalanced Profile = 2
	ProfileThorough Profile = 3
)

// InlineReadOnlyLines caps the read-only context lines AssemblePrompt inlines
// for a role whose settings carry InlineReadOnly.
const InlineReadOnlyLines = 1500

var (
	// ErrInvalidProfile reports a profile word or value outside lean, balanced, thorough.
	ErrInvalidProfile = errors.New("invalid context profile")
	// ErrInvalidNudgeTokens reports an adapter nudge_tokens value under 1000 or over MaxNudgeTokens.
	ErrInvalidNudgeTokens = errors.New("nudge_tokens out of range")
)

var profileNames = map[Profile]string{
	ProfileLean:     "lean",
	ProfileBalanced: "balanced",
	ProfileThorough: "thorough",
}

// ParseProfile converts one of the exact words lean, balanced or thorough to a
// Profile. It is case-sensitive and does not trim whitespace.
func ParseProfile(s string) (Profile, error) {
	for p, name := range profileNames {
		if s == name {
			return p, nil
		}
	}
	return 0, fmt.Errorf("%w: %q (want lean, balanced or thorough)", ErrInvalidProfile, s)
}

// String returns the profile word, or "invalid(<n>)" for a value outside the
// three defined profiles.
func (p Profile) String() string {
	if name, ok := profileNames[p]; ok {
		return name
	}
	return fmt.Sprintf("invalid(%d)", int(p))
}

// ContextSettings is one role's resolved context budget for one task.
type ContextSettings struct {
	Profile         Profile // the profile the role asked for
	Floored         bool    // guard floor raised at least one value
	HandoffTokens   int     // context size at which the launcher asks for a handoff
	MaxSessions     int     // fresh sessions per task and role before escalating
	MaxTurns        int     // turns per session
	ToolOutputChars int     // the runtime's own tool output cap
	OutputLines     int     // TrimOutput cap for waggle's own output
	InlineReadOnly  bool    // inline read-only context instead of listing paths
	Thinking        Profile // thinking budget step: lean low, balanced medium, thorough high
}

// profileTable holds the starting values per profile. HandoffTokens is a
// share of nudge_tokens in percent until settingsFor scales it.
var profileTable = map[Profile]ContextSettings{
	ProfileLean:     {HandoffTokens: 50, MaxSessions: 3, MaxTurns: 40, ToolOutputChars: 10000, OutputLines: 80},
	ProfileBalanced: {HandoffTokens: 75, MaxSessions: 5, MaxTurns: 80, ToolOutputChars: 20000, OutputLines: 200},
	ProfileThorough: {HandoffTokens: 100, MaxSessions: 8, MaxTurns: 150, ToolOutputChars: 30000, OutputLines: 400, InlineReadOnly: true},
}

func settingsFor(p Profile, nudgeTokens int) ContextSettings {
	s := profileTable[p]
	s.Profile, s.Thinking, s.HandoffTokens = p, p, nudgeTokens*s.HandoffTokens/100
	return s
}

// ContextPolicy is the hive's context.profile setting: a default profile and
// per-role overrides keyed by roster id. Whether a key names a roster role is
// checked when config loads, not here.
type ContextPolicy struct {
	Default Profile            // zero means balanced
	Roles   map[string]Profile // per-role overrides, keys are roster ids
}

// Settings resolves one role's settings for one task. A nil task means an
// unknown tier. nudgeTokens is the adapter's nudge_tokens, from 1000 to
// MaxNudgeTokens. For the guard on a Tier 2, Tier 3 or unknown-tier task every value is
// raised to at least the balanced value and Floored records whether any was.
func (c ContextPolicy) Settings(role string, task *TaskContext, nudgeTokens int) (ContextSettings, error) {
	if task == nil {
		task = &TaskContext{}
	}
	if role == roleDaemon {
		return ContextSettings{}, fmt.Errorf("%w: role %s task %q", ErrNoModelRole, role, task.TaskID)
	}
	if nudgeTokens < 1000 || nudgeTokens > MaxNudgeTokens {
		return ContextSettings{}, fmt.Errorf("%w: got %d", ErrInvalidNudgeTokens, nudgeTokens)
	}
	p := c.Default
	if p == 0 {
		p = ProfileBalanced
	}
	if override, ok := c.Roles[role]; ok {
		p = override
	}
	if profileNames[p] == "" {
		return ContextSettings{}, fmt.Errorf("%w: role %s task %q profile %s", ErrInvalidProfile, role, task.TaskID, p)
	}
	s := settingsFor(p, nudgeTokens)
	if role == roleGuard && !lowTier(task.Tier) {
		s = floorSettings(s, settingsFor(ProfileBalanced, nudgeTokens))
	}
	return s, nil
}

// floorSettings raises every value of s to at least the matching value of
// floor and marks the result Floored when anything changed.
func floorSettings(s, floor ContextSettings) ContextSettings {
	raise := func(v *int, min int) {
		if *v < min {
			*v, s.Floored = min, true
		}
	}
	raise(&s.HandoffTokens, floor.HandoffTokens)
	raise(&s.MaxSessions, floor.MaxSessions)
	raise(&s.MaxTurns, floor.MaxTurns)
	raise(&s.ToolOutputChars, floor.ToolOutputChars)
	raise(&s.OutputLines, floor.OutputLines)
	if s.Thinking < floor.Thinking {
		s.Thinking, s.Floored = floor.Thinking, true
	}
	if floor.InlineReadOnly && !s.InlineReadOnly {
		s.InlineReadOnly, s.Floored = true, true
	}
	return s
}
