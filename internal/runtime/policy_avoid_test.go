package runtime

import (
	"slices"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/comb"
)

// hivePolicy is this hive's shape since D-224: opus, sonnet, haiku, with
// alternates.frontier fable; builder and guard minimum standard.
func hivePolicy() Policy {
	p := testPolicy()
	p.Models.Alternate = map[Level]string{LevelFrontier: "fable"}
	return p
}

// T-034 AC3: a guard avoids every AvoidModels entry besides the policy's
// builder model; a Tier 3 or unknown-tier plain guard walks down (D-255).
func TestGuardAvoidModels(t *testing.T) {
	noAlt := testPolicy()
	cases := []struct {
		name    string
		p       Policy
		task    TaskContext
		want    string // "" means ErrNoCrossModel
		builder string
	}{
		{"none", hivePolicy(), TaskContext{}, "opus", "sonnet"},
		{"opus", hivePolicy(), TaskContext{AvoidModels: []string{"opus"}}, "fable", "sonnet"},
		{"fable", hivePolicy(), TaskContext{AvoidModels: []string{"fable"}}, "opus", "sonnet"},
		{"unmapped", hivePolicy(), TaskContext{AvoidModels: []string{"x"}}, "opus", "sonnet"},
		{"stepped fable", hivePolicy(), TaskContext{GuardMustFix: 2, AvoidModels: []string{"fable"}}, "sonnet", "opus"},
		{"stepped sonnet", hivePolicy(), TaskContext{GuardMustFix: 2, AvoidModels: []string{"sonnet"}}, "fable", "opus"},
		{"stepped opus", hivePolicy(), TaskContext{GuardMustFix: 2, AvoidModels: []string{"opus"}}, "fable", "opus"},
		{"stepped sonnet and fable", hivePolicy(), TaskContext{GuardMustFix: 2, AvoidModels: []string{"sonnet", "fable"}}, "", "opus"},
		{"stepped tier 2 no walk", hivePolicy(), TaskContext{GuardMustFix: 2, Tier: tier(comb.Tier2Logic), AvoidModels: []string{"fable"}}, "", "opus"},
		{"stepped tier 3 walks", hivePolicy(), TaskContext{GuardMustFix: 2, Tier: tier(comb.Tier3ControlPlane), AvoidModels: []string{"fable"}}, "sonnet", "opus"},
		{"no alternates", noAlt, TaskContext{AvoidModels: []string{"opus"}}, "", "sonnet"},
		{"second fable", hivePolicy(), TaskContext{SecondGuard: true, AvoidModels: []string{"fable"}}, "opus", "sonnet"},
		{"second as today", hivePolicy(), TaskContext{SecondGuard: true}, "fable", "sonnet"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.task.TaskID = "T-34"
			sel, err := c.p.Resolve("guard", &c.task, nil)
			if c.want == "" {
				wantErr(t, err, ErrNoCrossModel)
				return
			}
			if err != nil || sel.Model != c.want {
				t.Fatalf("model %q err %v; want %s", sel.Model, err, c.want)
			}
			if sel.Model == c.builder || slices.Contains(c.task.AvoidModels, sel.Model) {
				t.Fatalf("model %q is the builder's (%s) or in %v", sel.Model, c.builder, c.task.AvoidModels)
			}
		})
	}
	b := mustResolve(t, hivePolicy(), "builder", &TaskContext{GuardMustFix: 2}, nil)
	if b.Model != "opus" {
		t.Fatalf("stepped builder %q; want opus", b.Model)
	}
}

// T-034 AC7: a second guard's hint names a level whose alternate would resolve
// and keeps the first guard where it is (2.3).
func TestSecondGuardHint(t *testing.T) {
	second := func(avoid []string, builder string) *TaskContext {
		return &TaskContext{TaskID: "T-34", SecondGuard: true, AvoidModels: avoid, RecordedBuilder: builder}
	}
	unmapped := testPolicy()
	unmapped.Models.Primary[LevelFrontier] = "sonnet"
	cases := []struct {
		name string
		p    Policy
		task *TaskContext
		want []string
		not  string
	}{
		{"first guard not mapped", unmapped, second([]string{"haiku"}, ""), []string{`first guard "haiku"`, "is not mapped at or above level standard", "levels"}, "alternates."},
		{"hive before D-224", testPolicy(), second(nil, ""), []string{`builder "sonnet", first guard "opus"`, "alternates.frontier", "keeps the first guard on opus"}, "alternates.standard"},
		{"walk floor", walkPolicy("opus", "haiku", ""), second(nil, ""), []string{`first guard "fable"`, "alternates.standard"}, "alternates.frontier"},
		{"every slot taken", walkPolicy("opus", "haiku", "fable"), second(nil, ""), []string{"is taken", "runtimes.adapters.claude-code.levels"}, "alternates."},
		{"no builder record", testPolicy(), second([]string{"opus"}, ""), []string{`builder "sonnet", first guard "opus"`, "alternates.frontier"}, `builder "opus"`},
		{"builder record", testPolicy(), second([]string{"opus", "haiku"}, "opus"), []string{`builder "opus", first guard "haiku"`}, `builder "sonnet"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := c.p.Resolve("guard", c.task, nil)
			wantErr(t, err, ErrNoCrossModel, c.want...)
			if err != nil && strings.Contains(err.Error(), c.not) {
				t.Errorf("err %q mentions %q", err, c.not)
			}
			if !strings.HasPrefix(err.Error(), ErrNoCrossModel.Error()) {
				t.Errorf("err %q does not lead with the sentinel", err)
			}
		})
	}
	if sel := mustResolve(t, hivePolicy(), "guard", second(nil, ""), nil); sel.Model != "fable" {
		t.Errorf("D-224 shape: second guard %q; want fable", sel.Model)
	}
	// A plain guard keeps pick's text, with or without AvoidModels.
	_, err := testPolicy().Resolve("guard", &TaskContext{TaskID: "T-34", AvoidModels: []string{"opus"}}, nil)
	wantErr(t, err, ErrNoCrossModel, `shares ["sonnet" "opus"] with the builder or first guard`, "alternates.standard")
}
