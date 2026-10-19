package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
)

// Role ids the policy treats specially; every other role is governed only by
// its roster minimum and a recorded override.
const roleBuilder, roleGuard, roleDaemon = "builder", "guard", "waggled"

var (
	// ErrUnknownRole reports a role id missing from the roster minimums.
	ErrUnknownRole = errors.New("unknown role")
	// ErrNoModelRole reports a model lookup for waggled, which is a daemon, not an agent.
	ErrNoModelRole = errors.New("role has no model")
	// ErrLevelUnmapped reports a level with no primary model in config.
	ErrLevelUnmapped = errors.New("level has no model mapped")
	// ErrBelowMinimum reports a level under the roster minimum without a recorded decision.
	ErrBelowMinimum = errors.New("level below roster minimum")
	// ErrGuardFloor reports a guard override under the guard floor on a Tier 2, Tier 3 or unknown-tier task.
	ErrGuardFloor = errors.New("guard level below floor")
	// ErrNoCrossModel reports that no model differs from the builder's (and, for a
	// second guard, the first guard's): none at or above the guard's level, and
	// for a second guard none below it either (D-179).
	ErrNoCrossModel = errors.New("no model differs from the builder's and the first guard's")
	// ErrOverrideUnrecorded reports an override whose decision the lookup could not confirm.
	ErrOverrideUnrecorded = errors.New("override has no recorded decision")
	// ErrUnresolved reports an Invocation whose Selection did not come from Resolve for its role.
	ErrUnresolved = errors.New("model selection not resolved")
)

// Models maps capability levels to concrete model strings for one runtime
// adapter. Adapter names the adapter in config paths such as
// runtimes.adapters.<name>.levels.<level>. Primary must map all three levels;
// Alternate is optional and only serves the guard's different-model rule.
type Models struct {
	Adapter   string
	Primary   map[Level]string
	Alternate map[Level]string
}

// Policy is the model policy for a hive: the adapter's level mapping plus each
// role's roster minimum, keyed by role id.
type Policy struct {
	Models   Models
	Minimums map[string]Level
}

// Override requests a level for one role on one task. It applies only when
// DecisionID names a decision recorded with `waggle decide`.
type Override struct {
	Level      Level
	DecisionID int
}

// TaskContext carries the per-task facts Resolve needs. A nil *TaskContext
// means a base resolution: no step-up, no overrides, unknown tier.
type TaskContext struct {
	TaskID       string
	Tier         *comb.RiskTier      // nil means unknown and is treated as Tier 3
	GuardMustFix int                 // must-fix verdicts on this task so far
	SecondGuard  bool                // guard role only: also avoid the first guard's model (D-100)
	Overrides    map[string]Override // by role id
}

// Selection is a resolved model choice for one role on one task. Only Resolve
// produces a usable Selection; a zero or hand-built value is refused by Prepare.
type Selection struct {
	Role, Adapter, Model string
	Level                Level
	SteppedUp            bool // builder step-up applied
	resolved             bool // set only by Resolve
}

// Validate checks the policy is complete: every minimum is a valid level, all
// three primaries are mapped, no alternate equals its own primary, and the
// builder and guard roles are present. waggled may not appear in the roster.
func (p Policy) Validate() error {
	for role, l := range p.Minimums {
		if role == roleDaemon {
			return fmt.Errorf("%w: role %s may not appear in the roster", ErrNoModelRole, role)
		}
		if levelNames[l] == "" {
			return fmt.Errorf("%w: role %s minimum %s", ErrInvalidLevel, role, l)
		}
	}
	for _, role := range []string{roleBuilder, roleGuard} {
		if _, ok := p.Minimums[role]; !ok {
			return fmt.Errorf("%w: role %s has no roster minimum", ErrUnknownRole, role)
		}
	}
	for l := LevelFast; l <= LevelFrontier; l++ {
		if p.Models.Primary[l] == "" {
			return fmt.Errorf("%w: level %s (set %s)", ErrLevelUnmapped, l, p.levelPath(l))
		}
		if alt, ok := p.Models.Alternate[l]; ok && alt == p.Models.Primary[l] {
			return fmt.Errorf("%w: alternate %q for level %s equals its primary (runtimes.adapters.%s.alternates.%s)", ErrInvalidLevel, alt, l, p.Models.Adapter, l)
		}
	}
	return nil
}

func (p Policy) levelPath(l Level) string {
	return fmt.Sprintf("runtimes.adapters.%s.levels.%s", p.Models.Adapter, l)
}

// Resolve picks the model for role on task: roster minimum, recorded override
// (decisionExists must confirm its decision ID; a nil lookup rejects every
// override), builder step-up, guard floor, model lookup, cross-model rule.
func (p Policy) Resolve(role string, task *TaskContext, decisionExists func(id int) bool) (Selection, error) {
	if task == nil {
		task = &TaskContext{}
	}
	if role == roleDaemon {
		return Selection{}, fmt.Errorf("%w: role %s task %q", ErrNoModelRole, role, task.TaskID)
	}
	level, stepped, err := p.effectiveLevel(role, task, decisionExists)
	if err != nil {
		return Selection{}, err
	}
	var avoid []string
	if role == roleGuard {
		var builderModel string
		if level, builderModel, err = p.guardLevel(level, task, decisionExists); err != nil {
			return Selection{}, err
		}
		avoid = []string{builderModel}
		if task.SecondGuard { // the first guard is re-resolved from policy, never read from a run record
			first, err := p.pick(role, level, task, builderModel)
			if err != nil {
				return Selection{}, err
			}
			avoid = append(avoid, first)
		}
	}
	model, err := p.pick(role, level, task, avoid...)
	if err != nil && task.SecondGuard && role == roleGuard && errors.Is(err, ErrNoCrossModel) {
		// D-179: nothing unused at or above the level, so take the highest unused model below it.
		for l := level - 1; l >= LevelFast && model == ""; l-- {
			for _, m := range []string{p.Models.Primary[l], p.Models.Alternate[l]} {
				if m != "" && !slices.Contains(avoid, m) {
					model, level, err = m, l, nil
					break
				}
			}
		}
	}
	if err != nil {
		return Selection{}, err
	}
	return Selection{Role: role, Adapter: p.Models.Adapter, Model: model, Level: level, SteppedUp: stepped, resolved: true}, nil
}

// effectiveLevel applies the roster minimum, a recorded override, and the
// builder step-up (rules 1 to 4). It never consults the guard floor.
func (p Policy) effectiveLevel(role string, task *TaskContext, decisionExists func(int) bool) (Level, bool, error) {
	level, ok := p.Minimums[role]
	if !ok {
		return 0, false, fmt.Errorf("%w: role %s task %q", ErrUnknownRole, role, task.TaskID)
	}
	if ov, ok := task.Overrides[role]; ok {
		if levelNames[ov.Level] == "" {
			return 0, false, fmt.Errorf("%w: role %s task %q override level %s", ErrInvalidLevel, role, task.TaskID, ov.Level)
		}
		if decisionExists == nil || ov.DecisionID <= 0 || !decisionExists(ov.DecisionID) {
			return 0, false, fmt.Errorf("%w: role %s task %q level %s decision D-%d", ErrOverrideUnrecorded, role, task.TaskID, ov.Level, ov.DecisionID)
		}
		level = ov.Level
	}
	if role == roleBuilder && task.GuardMustFix >= 2 && level < LevelFrontier {
		return level + 1, true, nil
	}
	return level, false, nil
}

// guardLevel applies the guard floor (rule 5) and returns the level to resolve
// plus the builder's model, which the guard must not share (rule 7).
func (p Policy) guardLevel(level Level, task *TaskContext, decisionExists func(int) bool) (Level, string, error) {
	builderLevel, _, err := p.effectiveLevel(roleBuilder, task, decisionExists)
	if err != nil {
		return 0, "", err
	}
	floor := max(p.Minimums[roleGuard], builderLevel)
	if _, overridden := task.Overrides[roleGuard]; !overridden {
		level = floor
	} else if level < floor && !lowTier(task.Tier) {
		return 0, "", fmt.Errorf("%w: role %s task %q level %s is below floor %s", ErrGuardFloor, roleGuard, task.TaskID, level, floor)
	}
	builderModel, err := p.pick(roleBuilder, builderLevel, task)
	return level, builderModel, err
}

// pick returns the primary model for level (rule 6). With avoid set it walks
// from level up to frontier, trying each primary then alternate, and returns
// the first model not in avoid (rule 7).
func (p Policy) pick(role string, level Level, task *TaskContext, avoid ...string) (string, error) {
	if len(avoid) == 0 {
		if m := p.Models.Primary[level]; m != "" {
			return m, nil
		}
		return "", fmt.Errorf("%w: role %s task %q level %s (set %s)", ErrLevelUnmapped, role, task.TaskID, level, p.levelPath(level))
	}
	for l := level; l <= LevelFrontier; l++ {
		for _, m := range []string{p.Models.Primary[l], p.Models.Alternate[l]} {
			if m != "" && !slices.Contains(avoid, m) {
				return m, nil
			}
		}
	}
	return "", fmt.Errorf("%w: role %s task %q level %s shares %q with the builder or first guard (map a model not already used, for example runtimes.adapters.%s.alternates.%s)",
		ErrNoCrossModel, role, task.TaskID, level, avoid, p.Models.Adapter, level)
}

// lowTier reports whether a recorded override may lower the guard: only on a
// known Tier 0 or Tier 1 task. Unknown (nil) and every other tier enforce the floor.
func lowTier(tier *comb.RiskTier) bool {
	return tier != nil && (*tier == comb.Tier0Trivial || *tier == comb.Tier1Isolated)
}

// CountMustFix counts the lines of a review file that equal "Verdict: must-fix"
// after trailing spaces and carriage returns are trimmed. Heading text such as
// "(verdict: must-fix)" does not count. It assumes the guard appends one
// Verdict line per review round and never rewrites earlier ones.
func CountMustFix(review []byte) int {
	n := 0
	for _, line := range bytes.Split(review, []byte("\n")) {
		if strings.TrimRight(string(line), " \r") == "Verdict: must-fix" {
			n++
		}
	}
	return n
}
