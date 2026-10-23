package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/comb"
)

func testPolicy() Policy {
	return Policy{
		Models: Models{
			Adapter: "claude-code",
			Primary: map[Level]string{LevelFrontier: "opus", LevelStandard: "sonnet", LevelFast: "haiku"},
		},
		Minimums: map[string]Level{
			"scout":   LevelFrontier,
			"queen":   LevelFrontier,
			"builder": LevelStandard,
			"guard":   LevelStandard,
			"forager": LevelFast,
		},
	}
}

func tier(t comb.RiskTier) *comb.RiskTier { return &t }

func recorded(ids ...int) func(int) bool {
	return func(id int) bool {
		for _, want := range ids {
			if id == want {
				return true
			}
		}
		return false
	}
}

func mustResolve(t *testing.T, p Policy, role string, task *TaskContext, lookup func(int) bool) Selection {
	t.Helper()
	sel, err := p.Resolve(role, task, lookup)
	if err != nil {
		t.Fatalf("Resolve(%s): %v", role, err)
	}
	if !sel.resolved || sel.Role != role || sel.Adapter != "claude-code" {
		t.Fatalf("Resolve(%s) = %+v; want resolved for role and adapter", role, sel)
	}
	return sel
}

func wantErr(t *testing.T, err, sentinel error, substrs ...string) {
	t.Helper()
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v; want %v", err, sentinel)
	}
	for _, s := range substrs {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("err %q does not mention %q", err, s)
		}
	}
}

func TestParseLevel(t *testing.T) {
	for word, want := range map[string]Level{"fast": LevelFast, "standard": LevelStandard, "frontier": LevelFrontier} {
		got, err := ParseLevel(word)
		if err != nil || got != want || got.String() != word {
			t.Errorf("ParseLevel(%q) = %v, %v; want %v", word, got, err, want)
		}
	}
	for _, bad := range []string{"Frontier", "opus", "", " fast"} {
		if _, err := ParseLevel(bad); !errors.Is(err, ErrInvalidLevel) {
			t.Errorf("ParseLevel(%q) err = %v; want ErrInvalidLevel", bad, err)
		}
	}
	if s := Level(0).String(); s != "invalid(0)" {
		t.Errorf("Level(0).String() = %q", s)
	}
}

func TestValidate(t *testing.T) {
	if err := testPolicy().Validate(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		mutate   func(*Policy)
		sentinel error
	}{
		{"bad minimum", func(p *Policy) { p.Minimums["forager"] = 0 }, ErrInvalidLevel},
		{"missing primary", func(p *Policy) { delete(p.Models.Primary, LevelFast) }, ErrLevelUnmapped},
		{"alternate equals primary", func(p *Policy) { p.Models.Alternate = map[Level]string{LevelFrontier: "opus"} }, ErrInvalidLevel},
		{"no guard", func(p *Policy) { delete(p.Minimums, "guard") }, ErrUnknownRole},
		{"no builder", func(p *Policy) { delete(p.Minimums, "builder") }, ErrUnknownRole},
		{"waggled in roster", func(p *Policy) { p.Minimums["waggled"] = LevelFast }, ErrNoModelRole},
	}
	for _, tc := range cases {
		p := testPolicy()
		tc.mutate(&p)
		wantErr(t, p.Validate(), tc.sentinel)
	}
}

func TestDefaultMinimums(t *testing.T) {
	p := testPolicy()
	for role, want := range map[string]string{"scout": "opus", "queen": "opus", "builder": "sonnet", "forager": "haiku"} {
		sel := mustResolve(t, p, role, nil, nil)
		if sel.Model != want || sel.Level != p.Minimums[role] || sel.SteppedUp {
			t.Errorf("%s: %+v; want model %s at roster minimum", role, sel, want)
		}
	}
}

func TestGuardCrossModel(t *testing.T) {
	p := testPolicy()
	sel := mustResolve(t, p, "guard", &TaskContext{TaskID: "T-1"}, nil)
	if sel.Model != "opus" || sel.Level != LevelStandard {
		t.Errorf("guard with builder at standard: %+v; want opus (walked up from the shared standard model)", sel)
	}

	task := &TaskContext{TaskID: "T-1", GuardMustFix: 2}
	_, err := p.Resolve("guard", task, nil)
	wantErr(t, err, ErrNoCrossModel, "guard", "T-1", "frontier", "alternates.frontier")

	p.Models.Alternate = map[Level]string{LevelFrontier: "other-frontier"}
	sel = mustResolve(t, p, "guard", task, nil)
	if sel.Model != "other-frontier" || sel.Level != LevelFrontier {
		t.Errorf("guard against stepped-up builder: %+v; want the frontier alternate", sel)
	}
}

func TestForagerStepUp(t *testing.T) {
	p := testPolicy()
	for mustFix, want := range map[int]Level{0: LevelStandard, 1: LevelStandard, 2: LevelFrontier, 5: LevelFrontier} {
		sel := mustResolve(t, p, "builder", &TaskContext{GuardMustFix: mustFix}, nil)
		if sel.Level != want || sel.SteppedUp != (want == LevelFrontier) {
			t.Errorf("GuardMustFix %d: %+v; want level %s", mustFix, sel, want)
		}
	}
	p.Minimums["builder"] = LevelFrontier
	sel := mustResolve(t, p, "builder", &TaskContext{GuardMustFix: 4}, nil)
	if sel.Level != LevelFrontier || sel.SteppedUp {
		t.Errorf("builder already at frontier: %+v; want frontier without step-up", sel)
	}
}

func TestForagerOverride(t *testing.T) {
	p := testPolicy()
	override := func(id int) *TaskContext {
		return &TaskContext{TaskID: "T-2", Overrides: map[string]Override{"builder": {Level: LevelFast, DecisionID: id}}}
	}
	sel := mustResolve(t, p, "builder", override(31), recorded(31))
	if sel.Model != "haiku" || sel.Level != LevelFast {
		t.Errorf("recorded override to fast: %+v", sel)
	}
	sel = mustResolve(t, p, "builder", &TaskContext{GuardMustFix: 2, Overrides: override(31).Overrides}, recorded(31))
	if sel.Level != LevelStandard || !sel.SteppedUp {
		t.Errorf("override then step-up: %+v; want standard, stepped up", sel)
	}

	_, err := p.Resolve("builder", override(31), nil)
	wantErr(t, err, ErrOverrideUnrecorded, "builder", "T-2", "fast", "D-31")
	_, err = p.Resolve("builder", override(0), recorded(31))
	wantErr(t, err, ErrOverrideUnrecorded)
	_, err = p.Resolve("builder", override(99), recorded(31))
	wantErr(t, err, ErrOverrideUnrecorded)
	_, err = p.Resolve("builder", &TaskContext{Overrides: map[string]Override{"builder": {DecisionID: 31}}}, recorded(31))
	wantErr(t, err, ErrInvalidLevel)
}

func TestGuardFloor(t *testing.T) {
	p := testPolicy()
	task := func(tier *comb.RiskTier) *TaskContext {
		return &TaskContext{TaskID: "T-3", Tier: tier, Overrides: map[string]Override{"guard": {Level: LevelFast, DecisionID: 7}}}
	}
	for _, tr := range []*comb.RiskTier{tier(comb.Tier2Logic), tier(comb.Tier3ControlPlane), nil} {
		_, err := p.Resolve("guard", task(tr), recorded(7))
		wantErr(t, err, ErrGuardFloor, "guard", "T-3", "fast", "standard")
	}
	sel := mustResolve(t, p, "guard", task(tier(comb.Tier1Isolated)), recorded(7))
	builder := mustResolve(t, p, "builder", task(tier(comb.Tier1Isolated)), recorded(7))
	if sel.Level != LevelFast || sel.Model == builder.Model || sel.Model != "haiku" {
		t.Errorf("guard override on tier 1: %+v vs builder %+v", sel, builder)
	}
	_, err := p.Resolve("guard", task(tier(comb.Tier1Isolated)), nil)
	wantErr(t, err, ErrOverrideUnrecorded)

	// An override above the floor is fine on any tier, and the guard still
	// dodges the builder's model.
	up := &TaskContext{Tier: tier(comb.Tier3ControlPlane), Overrides: map[string]Override{"guard": {Level: LevelFrontier, DecisionID: 7}}}
	if sel := mustResolve(t, p, "guard", up, recorded(7)); sel.Model != "opus" {
		t.Errorf("guard override up: %+v", sel)
	}
}

func TestResolveErrors(t *testing.T) {
	p := testPolicy()
	_, err := p.Resolve("waggled", &TaskContext{TaskID: "T-4"}, nil)
	wantErr(t, err, ErrNoModelRole, "waggled", "T-4")
	_, err = p.Resolve("weaver", nil, nil)
	wantErr(t, err, ErrUnknownRole, "weaver")

	delete(p.Models.Primary, LevelFast)
	_, err = p.Resolve("forager", &TaskContext{TaskID: "T-4"}, nil)
	wantErr(t, err, ErrLevelUnmapped, "forager", "T-4", "fast", "runtimes.adapters.claude-code.levels.fast")
	_, err = p.Resolve("guard", &TaskContext{Overrides: map[string]Override{"builder": {Level: LevelFast, DecisionID: 1}}}, recorded(1))
	wantErr(t, err, ErrLevelUnmapped, "builder")
}

func TestCountMustFix(t *testing.T) {
	review := strings.Join([]string{
		"# Review T-9 (verdict: must-fix)",
		"Verdict: must-fix  ",
		"notes",
		"Verdict: pass",
		"## Round 2",
		"Verdict: must-fix",
		"verdict: must-fix",
		" Verdict: must-fix",
		"",
	}, "\r\n")
	if n := CountMustFix([]byte(review)); n != 2 {
		t.Errorf("CountMustFix = %d; want 2", n)
	}
	if n := CountMustFix(nil); n != 0 {
		t.Errorf("CountMustFix(nil) = %d", n)
	}
}

// Item 21 (test 17): the second guard avoids the builder's model and the first
// guard's, and none left is ErrNoCrossModel naming the alternates key.
func TestSecondGuard(t *testing.T) {
	p := testPolicy()
	p.Models.Alternate = map[Level]string{LevelFrontier: "other-frontier"}
	first := mustResolve(t, p, "guard", &TaskContext{TaskID: "T-1"}, nil)
	second := mustResolve(t, p, "guard", &TaskContext{TaskID: "T-1", SecondGuard: true}, nil)
	if first.Model != "opus" || second.Model != "other-frontier" {
		t.Errorf("first %q second %q; want opus then other-frontier", first.Model, second.Model)
	}
	// D-179: a frontier builder leaves frontier taken by builder and first guard,
	// so the second guard takes the highest unused model below, at its own level.
	ctx := func(second bool) *TaskContext {
		return &TaskContext{TaskID: "T-1", GuardMustFix: 2, SecondGuard: second}
	}
	builder, first2 := mustResolve(t, p, "builder", ctx(false), nil), mustResolve(t, p, "guard", ctx(false), nil)
	second2 := mustResolve(t, p, "guard", ctx(true), nil)
	var err error
	if builder.Model != "opus" || first2.Model != "other-frontier" || second2.Model != "sonnet" || second2.Level != LevelStandard {
		t.Errorf("builder %q first %q second %q (%s); want opus, other-frontier, sonnet (standard)", builder.Model, first2.Model, second2.Model, second2.Level)
	}
	// Every model taken (one model at all three levels): still an error.
	p.Models.Primary = map[Level]string{LevelFrontier: "m", LevelStandard: "m", LevelFast: "m"}
	p.Models.Alternate = nil
	_, err = p.Resolve("guard", &TaskContext{TaskID: "T-1", SecondGuard: true}, nil)
	wantErr(t, err, ErrNoCrossModel, "T-1", "alternates.standard", "first guard")
}

// D-180: this hive's shape (opus/sonnet/haiku, builder and guard minimum
// standard, no alternates), unstepped: the second guard never falls to haiku,
// and the hint names frontier, where the first guard's opus sits (T-034 2.3).
func TestSecondGuardHiveShapeRefuses(t *testing.T) {
	p := testPolicy()
	_, err := p.Resolve("guard", &TaskContext{TaskID: "T-1", SecondGuard: true}, nil)
	wantErr(t, err, ErrNoCrossModel, "alternates.frontier")
}

// The D-179 walk: builder stepped to frontier, first guard on alternates.frontier,
// the second guard at standard, bounded at the guard minimum (D-180).
func walkPolicy(standard, fast, altStandard string) Policy {
	p := testPolicy()
	p.Minimums["builder"] = LevelFrontier
	p.Models.Primary = map[Level]string{LevelFrontier: "opus", LevelStandard: standard, LevelFast: fast}
	p.Models.Alternate = map[Level]string{LevelFrontier: "fable"}
	if altStandard != "" {
		p.Models.Alternate[LevelStandard] = altStandard
	}
	return p
}

func TestSecondGuardWalk(t *testing.T) {
	second := &TaskContext{TaskID: "T-1", SecondGuard: true}
	t.Run("motivating case", func(t *testing.T) {
		p := walkPolicy("sonnet", "haiku", "")
		b := mustResolve(t, p, "builder", &TaskContext{TaskID: "T-1"}, nil)
		f := mustResolve(t, p, "guard", &TaskContext{TaskID: "T-1"}, nil)
		s := mustResolve(t, p, "guard", second, nil)
		if b.Model != "opus" || f.Model != "fable" || s.Model != "sonnet" || s.Level != LevelStandard {
			t.Errorf("builder %q first %q second %q (%s); want opus, fable, sonnet (standard)", b.Model, f.Model, s.Model, s.Level)
		}
	})
	t.Run("primary before alternate", func(t *testing.T) {
		s := mustResolve(t, walkPolicy("sonnet", "haiku", "s2"), "guard", second, nil)
		if s.Model != "sonnet" {
			t.Errorf("second = %q; want sonnet", s.Model)
		}
	})
	t.Run("avoid list inside the walk", func(t *testing.T) {
		s := mustResolve(t, walkPolicy("opus", "haiku", "s2"), "guard", second, nil)
		if s.Model != "s2" {
			t.Errorf("second = %q; want s2", s.Model)
		}
	})
	t.Run("walk exhaustion", func(t *testing.T) {
		p := walkPolicy("opus", "fable", "")
		p.Minimums["guard"] = LevelFast
		_, err := p.Resolve("guard", second, nil)
		wantErr(t, err, ErrNoCrossModel)
	})
	t.Run("floor", func(t *testing.T) {
		p := walkPolicy("opus", "haiku", "")
		_, err := p.Resolve("guard", second, nil)
		wantErr(t, err, ErrNoCrossModel, "alternates.standard")
	})
}
