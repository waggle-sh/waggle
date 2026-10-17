package runtime

import (
	"errors"
	"testing"

	"github.com/waggle-sh/waggle/internal/comb"
)

// Section 4 of the brief with nudgeTokens = 200000.
var wantSettings = map[Profile]ContextSettings{
	ProfileLean:     {Profile: ProfileLean, HandoffTokens: 100000, MaxSessions: 3, MaxTurns: 40, ToolOutputChars: 10000, OutputLines: 80, InlineReadOnly: false, Thinking: ProfileLean},
	ProfileBalanced: {Profile: ProfileBalanced, HandoffTokens: 150000, MaxSessions: 5, MaxTurns: 80, ToolOutputChars: 20000, OutputLines: 200, InlineReadOnly: false, Thinking: ProfileBalanced},
	ProfileThorough: {Profile: ProfileThorough, HandoffTokens: 200000, MaxSessions: 8, MaxTurns: 150, ToolOutputChars: 30000, OutputLines: 400, InlineReadOnly: true, Thinking: ProfileThorough},
}

func mustSettings(t *testing.T, c ContextPolicy, role string, task *TaskContext) ContextSettings {
	t.Helper()
	s, err := c.Settings(role, task, 200000)
	if err != nil {
		t.Fatalf("Settings(%s): %v", role, err)
	}
	return s
}

func TestParseProfile(t *testing.T) {
	for word, want := range map[string]Profile{"lean": ProfileLean, "balanced": ProfileBalanced, "thorough": ProfileThorough} {
		got, err := ParseProfile(word)
		if err != nil || got != want || got.String() != word {
			t.Errorf("ParseProfile(%q) = %v, %v; want %v", word, got, err, want)
		}
	}
	for _, bad := range []string{"Lean", "", " lean", "max"} {
		if _, err := ParseProfile(bad); !errors.Is(err, ErrInvalidProfile) {
			t.Errorf("ParseProfile(%q) err = %v; want ErrInvalidProfile", bad, err)
		}
	}
	if s := Profile(0).String(); s != "invalid(0)" {
		t.Errorf("Profile(0).String() = %q", s)
	}
}

func TestSettingsPerProfile(t *testing.T) {
	for p, want := range wantSettings {
		if got := mustSettings(t, ContextPolicy{Default: p}, "builder", nil); got != want {
			t.Errorf("Settings(builder, %s) = %+v; want %+v", p, got, want)
		}
	}
	if got := mustSettings(t, ContextPolicy{Default: ProfileLean}, "builder", &TaskContext{Tier: tier(comb.Tier3ControlPlane)}); got != wantSettings[ProfileLean] {
		t.Errorf("builder on Tier 3 was floored: %+v", got)
	}
}

func TestSettingsOverrides(t *testing.T) {
	c := ContextPolicy{Default: ProfileLean, Roles: map[string]Profile{"scout": ProfileThorough}}
	if got := mustSettings(t, c, "scout", nil); got != wantSettings[ProfileThorough] {
		t.Errorf("override: %+v; want thorough", got)
	}
	if got := mustSettings(t, c, "builder", nil); got != wantSettings[ProfileLean] {
		t.Errorf("no override: %+v; want lean", got)
	}
	if got := mustSettings(t, ContextPolicy{}, "builder", nil); got != wantSettings[ProfileBalanced] {
		t.Errorf("zero Default: %+v; want balanced", got)
	}
	if got := mustSettings(t, ContextPolicy{Roles: map[string]Profile{"builder": ProfileLean}}, "forager", nil); got != wantSettings[ProfileBalanced] {
		t.Errorf("zero Default with another role's override: %+v; want balanced", got)
	}
}

func TestSettingsGuardFloor(t *testing.T) {
	lean := ContextPolicy{Default: ProfileLean}
	overridden := ContextPolicy{Default: ProfileThorough, Roles: map[string]Profile{"guard": ProfileLean}}
	balanced := wantSettings[ProfileBalanced]
	balanced.Profile, balanced.Floored = ProfileLean, true
	for name, task := range map[string]*TaskContext{
		"tier2": {Tier: tier(comb.Tier2Logic)},
		"tier3": {Tier: tier(comb.Tier3ControlPlane)},
		"nil":   nil,
		"unset": {},
	} {
		for _, c := range []ContextPolicy{lean, overridden} {
			if got := mustSettings(t, c, "guard", task); got != balanced {
				t.Errorf("guard lean on %s = %+v; want balanced values with Floored", name, got)
			}
		}
	}
	unfloored := wantSettings[ProfileLean]
	for name, task := range map[string]*TaskContext{"tier0": {Tier: tier(comb.Tier0Trivial)}, "tier1": {Tier: tier(comb.Tier1Isolated)}} {
		if got := mustSettings(t, overridden, "guard", task); got != unfloored {
			t.Errorf("guard lean on %s = %+v; want lean, not floored", name, got)
		}
	}
	thorough := ContextPolicy{Default: ProfileLean, Roles: map[string]Profile{"guard": ProfileThorough}}
	for _, task := range []*TaskContext{nil, {Tier: tier(comb.Tier0Trivial)}, {Tier: tier(comb.Tier3ControlPlane)}} {
		if got := mustSettings(t, thorough, "guard", task); got != wantSettings[ProfileThorough] {
			t.Errorf("guard thorough = %+v; want thorough unchanged", got)
		}
	}
	if got := mustSettings(t, ContextPolicy{Default: ProfileBalanced}, "guard", nil); got.Floored {
		t.Errorf("guard balanced reports Floored: %+v", got)
	}
}

func TestSettingsErrors(t *testing.T) {
	c := ContextPolicy{Default: ProfileBalanced}
	_, err := c.Settings("waggled", &TaskContext{TaskID: "T-7"}, 200000)
	wantErr(t, err, ErrNoModelRole, "waggled", "T-7")
	_, err = c.Settings("builder", nil, 999)
	wantErr(t, err, ErrInvalidNudgeTokens, "999")
	if _, err := c.Settings("builder", nil, 1000); err != nil {
		t.Errorf("nudge 1000: %v", err)
	}
	_, err = ContextPolicy{Default: Profile(4)}.Settings("builder", nil, 200000)
	wantErr(t, err, ErrInvalidProfile, "builder", "invalid(4)")
	_, err = ContextPolicy{Roles: map[string]Profile{"guard": 0}}.Settings("guard", nil, 200000)
	wantErr(t, err, ErrInvalidProfile, "guard")
}
