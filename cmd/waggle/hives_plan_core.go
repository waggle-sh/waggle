// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// This file is the pure core of `waggle hives plan` (T-064, phase 2 of
// docs/hives.md): the plan file, its validation, the check that compares a
// plan with each hive's current state, and the renderings. It does no I/O and
// reads no clock. A plan is advisory (RFC-0004 section 6.4): ordering is
// reported, never enforced, and nothing here writes to a hive. Hive text never
// reaches a brief; only validated fields do (RFC-0004 section 7.4).

const (
	planVersion = 1
	maxSteps    = 200
)

// Step states reported by check.
const (
	stateDone    = "done"
	stateClaimed = "claimed"
	stateOpen    = "open"
	stateBlocked = "blocked"
	stateDraft   = "draft"   // no task in the hive yet
	stateMissing = "missing" // the hive has no such task or claim
	stateStale   = "stale"   // the hive could not be read
	stateRefused = "refused" // the registry no longer allows planning for this hive
)

var stepIDRe = regexp.MustCompile(`^s[1-9][0-9]{0,3}$`)

// PlanStep is one per-hive task in a plan. Task is the hive's ticket id when
// the task exists; Title is the Keeper's own words. A step needs one of them.
type PlanStep struct {
	ID        string   `json:"id"`
	Hive      string   `json:"hive"`
	Task      string   `json:"task,omitempty"`
	Title     string   `json:"title,omitempty"`
	DependsOn []string `json:"depends_on"`
}

// Plan is an ordered list of steps. A step may depend only on earlier steps,
// so a plan cannot have a cycle.
type Plan struct {
	Version int        `json:"version"`
	Name    string     `json:"name"`
	Steps   []PlanStep `json:"steps"`
}

// NewPlan is an empty plan.
func NewPlan(name string) (Plan, error) {
	if !hiveNameRe.MatchString(name) {
		return Plan{}, fmt.Errorf("plan name %q must match %s", name, hiveNameRe)
	}
	return Plan{Version: planVersion, Name: name, Steps: []PlanStep{}}, nil
}

// ParsePlan strictly parses a plan file: unknown fields, trailing data, a
// wrong version, a name other than want, duplicate or malformed step ids,
// malformed tasks, steps with neither task nor title, duplicate hive tasks and
// dependencies that are not earlier steps are all refused.
func ParsePlan(data []byte, want string) (Plan, error) {
	var p Plan
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Plan{}, fmt.Errorf("plan: %v", err)
	}
	if dec.More() {
		return Plan{}, errors.New("plan: trailing data after the JSON object")
	}
	if p.Version != planVersion {
		return Plan{}, fmt.Errorf("plan: version %d is not supported (want %d)", p.Version, planVersion)
	}
	if p.Name != want {
		return Plan{}, fmt.Errorf("plan: file holds plan %q, expected %q", p.Name, want)
	}
	if len(p.Steps) > maxSteps {
		return Plan{}, fmt.Errorf("plan: %d steps, the limit is %d", len(p.Steps), maxSteps)
	}
	if p.Steps == nil {
		p.Steps = []PlanStep{}
	}
	seen := map[string]bool{}
	tasks := map[string]bool{}
	for i := range p.Steps {
		s := &p.Steps[i]
		if !stepIDRe.MatchString(s.ID) || seen[s.ID] {
			return Plan{}, fmt.Errorf("plan: step %d: id %q is malformed or repeated", i+1, s.ID)
		}
		if !hiveNameRe.MatchString(s.Hive) {
			return Plan{}, fmt.Errorf("plan: step %s: hive name %q is malformed", s.ID, s.Hive)
		}
		if s.Task != "" {
			c, err := comb.ParseTicketID(s.Task)
			if err != nil || c != s.Task {
				return Plan{}, fmt.Errorf("plan: step %s: task %q is not a ticket id", s.ID, s.Task)
			}
			if tasks[s.Hive+"/"+s.Task] {
				return Plan{}, fmt.Errorf("plan: step %s: %s %s is already a step", s.ID, s.Hive, s.Task)
			}
			tasks[s.Hive+"/"+s.Task] = true
		}
		if s.Task == "" && s.Title == "" {
			return Plan{}, fmt.Errorf("plan: step %s needs a task or a title", s.ID)
		}
		if s.Title != flatten(s.Title) {
			return Plan{}, fmt.Errorf("plan: step %s: title is not a flat line of at most %d characters", s.ID, maxTitle)
		}
		if s.DependsOn == nil {
			s.DependsOn = []string{}
		}
		deps := map[string]bool{}
		for _, d := range s.DependsOn {
			if !seen[d] || deps[d] {
				return Plan{}, fmt.Errorf("plan: step %s: depends_on %q is not an earlier step, or is repeated", s.ID, d)
			}
			deps[d] = true
		}
		seen[s.ID] = true
	}
	return p, nil
}

// RenderPlanFile is the bytes written to a plan file.
func RenderPlanFile(p Plan) []byte {
	out, _ := json.MarshalIndent(p, "", "  ")
	return append(out, '\n')
}

// planEntry returns the registry entry for a hive a step may name: it must be
// listed, and its level must be plan or work, never view.
func planEntry(reg []RegistryEntry, hive string) (RegistryEntry, error) {
	for _, e := range reg {
		if e.Name == hive {
			if e.Level == levelView {
				return e, fmt.Errorf("hive %q is at level view in the registry; raise it to plan first", hive)
			}
			return e, nil
		}
	}
	return RegistryEntry{}, fmt.Errorf("hive %q is not in the registry", hive)
}

// AddStep returns p with one more step. It refuses a hive that is not in the
// registry or is at level view, a malformed task, a missing task and title, a
// task already in the plan, and a dependency that is not an existing step.
func AddStep(p Plan, reg []RegistryEntry, hive, task, title string, deps []string) (Plan, PlanStep, error) {
	if _, err := planEntry(reg, hive); err != nil {
		return p, PlanStep{}, err
	}
	if task != "" {
		c, err := comb.ParseTicketID(task)
		if err != nil {
			return p, PlanStep{}, fmt.Errorf("task %q is not a ticket id like T-020", task)
		}
		task = c
	}
	title = flatten(title)
	if task == "" && title == "" {
		return p, PlanStep{}, errors.New("a step needs --task, --title or both")
	}
	if len(p.Steps) >= maxSteps {
		return p, PlanStep{}, fmt.Errorf("a plan holds at most %d steps", maxSteps)
	}
	next := 1
	for _, s := range p.Steps {
		if n, _ := strconv.Atoi(s.ID[1:]); n >= next {
			next = n + 1
		}
		if task != "" && s.Hive == hive && s.Task == task {
			return p, PlanStep{}, fmt.Errorf("%s %s is already step %s", hive, task, s.ID)
		}
	}
	step := PlanStep{ID: fmt.Sprintf("s%d", next), Hive: hive, Task: task, Title: title, DependsOn: []string{}}
	for _, d := range deps {
		if findStep(p, d) < 0 {
			return p, PlanStep{}, fmt.Errorf("depends-on %q is not a step of this plan", d)
		}
		for _, have := range step.DependsOn {
			if have == d {
				return p, PlanStep{}, fmt.Errorf("depends-on %q is repeated", d)
			}
		}
		step.DependsOn = append(step.DependsOn, d)
	}
	out := p
	out.Steps = append(append([]PlanStep{}, p.Steps...), step)
	return out, step, nil
}

// RemoveStep returns p without step id. It refuses while another step still
// depends on it.
func RemoveStep(p Plan, id string) (Plan, error) {
	i := findStep(p, id)
	if i < 0 {
		return p, fmt.Errorf("no step %q in plan %q", id, p.Name)
	}
	for _, s := range p.Steps {
		for _, d := range s.DependsOn {
			if d == id {
				return p, fmt.Errorf("step %s depends on %s; remove it first", s.ID, id)
			}
		}
	}
	out := p
	out.Steps = append(append([]PlanStep{}, p.Steps[:i]...), p.Steps[i+1:]...)
	return out, nil
}

func findStep(p Plan, id string) int {
	for i, s := range p.Steps {
		if s.ID == id {
			return i
		}
	}
	return -1
}

// StepStatus is one step as check reports it.
type StepStatus struct {
	ID         string   `json:"id"`
	Hive       string   `json:"hive"`
	Level      string   `json:"level,omitempty"`
	Task       string   `json:"task,omitempty"`
	Title      string   `json:"title,omitempty"`
	DependsOn  []string `json:"depends_on"`
	State      string   `json:"state"`
	Reason     string   `json:"reason,omitempty"`
	BlockedBy  []string `json:"blocked_by,omitempty"` // plan steps not yet done
	Unverified []string `json:"unverified,omitempty"` // dependency steps whose state could not be read
}

// Violation is an ordering violation: step is claimed or done while a step it
// depends on is known not to be done. It is a report; nothing enforces it.
type Violation struct {
	Step      string `json:"step"`
	StepState string `json:"step_state"`
	DependsOn string `json:"depends_on"`
	DepState  string `json:"dependency_state"`
}

// PlanCheck is the result of comparing a plan with the hives' current state.
type PlanCheck struct {
	Plan       string       `json:"plan"`
	CheckedAt  string       `json:"checked_at"`
	Steps      []StepStatus `json:"steps"`
	Violations []Violation  `json:"violations"`
}

// CheckPlan rebuilds each step's state from the hive views (keyed by hive
// name) and reports ordering violations. A hive with no view, or one that was
// unreachable, makes its steps stale: the state is not guessed, and a
// dependency in that state is listed as unverified, never as a violation.
func CheckPlan(p Plan, reg []RegistryEntry, views map[string]HiveView, now time.Time) PlanCheck {
	out := PlanCheck{Plan: p.Name, CheckedAt: now.UTC().Format(time.RFC3339), Steps: []StepStatus{}, Violations: []Violation{}}
	state := map[string]string{}
	for _, s := range p.Steps {
		st := StepStatus{ID: s.ID, Hive: s.Hive, Task: s.Task, Title: s.Title, DependsOn: append([]string{}, s.DependsOn...)}
		e, err := planEntry(reg, s.Hive)
		if err != nil {
			st.State, st.Reason = stateRefused, err.Error()
		} else {
			st.Level = e.Level
			v, ok := views[s.Hive]
			switch {
			case !ok || !v.Reachable:
				st.State = stateStale
				st.Reason = "unreachable"
				if ok && v.Error != "" {
					st.Reason = "unreachable: " + v.Error
				}
			default:
				st.State, st.Title = hiveStepState(s, v)
				if st.Title == "" {
					st.Title = s.Title
				}
			}
		}
		state[s.ID] = st.State
		out.Steps = append(out.Steps, st)
	}
	for i := range out.Steps {
		st := &out.Steps[i]
		for _, d := range st.DependsOn {
			switch ds := state[d]; ds {
			case stateDone:
			case stateStale, stateRefused, stateMissing:
				st.Unverified = append(st.Unverified, d)
			default:
				st.BlockedBy = append(st.BlockedBy, d)
				if st.State == stateClaimed || st.State == stateDone {
					out.Violations = append(out.Violations, Violation{Step: st.ID, StepState: st.State, DependsOn: d, DepState: ds})
				}
			}
		}
		if st.State == stateOpen && len(st.BlockedBy) > 0 {
			st.State = stateBlocked
		}
		if st.State == stateDone || st.State == stateClaimed {
			st.BlockedBy = nil
		}
	}
	return out
}

// hiveStepState classifies one step against its hive's view and returns the
// hive's own (flattened) title for a task step when the Keeper gave none.
func hiveStepState(s PlanStep, v HiveView) (string, string) {
	if s.Task == "" {
		return stateDraft, ""
	}
	for _, d := range v.Done {
		if d == s.Task {
			return stateDone, ""
		}
	}
	for _, c := range v.Claims {
		if canonTicket(c.Ticket) == s.Task {
			return stateClaimed, ""
		}
	}
	for _, t := range v.Blocked {
		if t.Ticket == s.Task {
			return stateBlocked, t.Title
		}
	}
	for _, t := range v.Open {
		if t.Ticket == s.Task {
			return stateOpen, t.Title
		}
	}
	return stateMissing, ""
}

// RenderPlanJSON renders a plan as indented JSON.
func RenderPlanJSON(p Plan) []byte { return RenderPlanFile(p) }

// RenderPlanText renders a plan for a terminal.
func RenderPlanText(p Plan) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "Plan %s: %d steps. Advisory: nothing is written to any hive.\n", p.Name, len(p.Steps))
	for _, s := range p.Steps {
		fmt.Fprintf(&b, "  %s  %s  %s", s.ID, s.Hive, stepLabel(s.Task, s.Title))
		if len(s.DependsOn) > 0 {
			fmt.Fprintf(&b, "  (after %s)", strings.Join(s.DependsOn, ", "))
		}
		b.WriteByte('\n')
	}
	return b.Bytes()
}

func stepLabel(task, title string) string {
	switch {
	case task != "" && title != "":
		return task + "  " + title
	case task != "":
		return task
	}
	return "(no task yet)  " + title
}

// RenderCheckJSON renders a check as indented JSON.
func RenderCheckJSON(c PlanCheck) []byte {
	out, _ := json.MarshalIndent(c, "", "  ")
	return append(out, '\n')
}

// RenderCheckText renders a check for a terminal.
func RenderCheckText(c PlanCheck) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "Plan %s, checked at %s. Read only: nothing is written to any hive; ordering is reported, not enforced.\n", c.Plan, c.CheckedAt)
	for _, s := range c.Steps {
		fmt.Fprintf(&b, "  %s  %s  %s  [%s]", s.ID, s.Hive, stepLabel(s.Task, s.Title), s.State)
		switch s.State {
		case stateStale:
			fmt.Fprintf(&b, "  stale since %s: %s", c.CheckedAt, s.Reason)
		case stateRefused:
			fmt.Fprintf(&b, "  %s", s.Reason)
		}
		if len(s.BlockedBy) > 0 {
			fmt.Fprintf(&b, "  (waiting on %s)", strings.Join(s.BlockedBy, ", "))
		}
		if len(s.Unverified) > 0 {
			fmt.Fprintf(&b, "  (cannot read %s)", strings.Join(s.Unverified, ", "))
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "Ordering violations (%d):\n", len(c.Violations))
	for _, v := range c.Violations {
		fmt.Fprintf(&b, "  %s is %s but %s, which it depends on, is %s\n", v.Step, v.StepState, v.DependsOn, v.DepState)
	}
	return b.Bytes()
}

// RenderBrief drafts a per-hive brief for one step. It is built from the plan
// (the Keeper's own words) and validated fields only; no hive text is copied
// into it. The Keeper edits it and lands it through that hive's normal path.
func RenderBrief(p Plan, c PlanCheck, step string) ([]byte, error) {
	i := findStep(p, step)
	if i < 0 {
		return nil, fmt.Errorf("no step %q in plan %q", step, p.Name)
	}
	s := p.Steps[i]
	title := s.Title
	if title == "" {
		title = "(title to be set)"
	}
	var b bytes.Buffer
	b.WriteString("<!-- DRAFT: written by `waggle hives plan brief`. Edit it, then land it in the hive through that hive's normal path. Nothing has been written to the hive. -->\n")
	if s.Task != "" {
		fmt.Fprintf(&b, "# %s: %s\n\n", s.Task, title)
	} else {
		fmt.Fprintf(&b, "# %s\n\n", title)
	}
	b.WriteString("- **Status:** Draft\n")
	fmt.Fprintf(&b, "- **Plan:** %s, step %s, hive %s\n", p.Name, s.ID, s.Hive)
	var same []string
	for _, d := range s.DependsOn {
		j := findStep(p, d)
		if j >= 0 && p.Steps[j].Hive == s.Hive && p.Steps[j].Task != "" {
			same = append(same, p.Steps[j].Task)
		}
	}
	if len(same) > 0 {
		fmt.Fprintf(&b, "- **Depends on:** %s\n", strings.Join(same, ", "))
	}
	b.WriteString("\n## Ordering across hives (advisory, from the plan)\n\n")
	n := 0
	for _, d := range s.DependsOn {
		j := findStep(p, d)
		if j < 0 || p.Steps[j].Hive == s.Hive {
			continue
		}
		n++
		fmt.Fprintf(&b, "- After step %s in hive %s%s (state at check: %s)\n", d, p.Steps[j].Hive, taskSuffix(p.Steps[j].Task), stepState(c, d))
	}
	for _, o := range p.Steps {
		for _, d := range o.DependsOn {
			if d == s.ID && o.Hive != s.Hive {
				n++
				fmt.Fprintf(&b, "- Before step %s in hive %s%s\n", o.ID, o.Hive, taskSuffix(o.Task))
			}
		}
	}
	if n == 0 {
		b.WriteString("- None.\n")
	}
	b.WriteString("\n## Goal\n\n(for the Keeper to write)\n\n## Lease\n\n(for the Keeper to write; a lease names directories in this hive only)\n")
	return b.Bytes(), nil
}

func taskSuffix(task string) string {
	if task == "" {
		return ""
	}
	return ", task " + task
}

func stepState(c PlanCheck, id string) string {
	for _, s := range c.Steps {
		if s.ID == id {
			return s.State
		}
	}
	return "unknown"
}
