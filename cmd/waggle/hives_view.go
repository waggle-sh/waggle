// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/decisions"
)

// This file is the pure core of `waggle hives status` (T-064, phase 1 of
// docs/hives.md): registry parsing, the per-hive view, the merged card list
// and both renderings. It does no I/O and reads no clock; hives.go feeds it.
// Everything read from a hive is untrusted data (RFC-0004 section 7.4): only
// validated fields are shown and free text is flattened and truncated.

// Access levels of a registry entry (RFC-0004 section 7.1). Phase 1 never
// writes at any level; the level is recorded and shown, nothing more.
const (
	levelView = "view"
	levelPlan = "plan"
	levelWork = "work"
)

// maxHives bounds a registry file; maxTitle bounds hive text shown to the Keeper.
const (
	maxHives = 64
	maxTitle = 100
)

var hiveNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// RegistryEntry is one home hive the Keeper listed.
type RegistryEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path,omitempty"` // absolute path of a local checkout
	Repo  string `json:"repo,omitempty"` // owner/repo label; phase 1 cannot read it remotely
	Level string `json:"level"`
}

type registryFile struct {
	Hives []RegistryEntry `json:"hives"`
}

// ParseRegistry validates the registry file bytes. It rejects unknown fields,
// duplicate or malformed names, relative paths and unknown levels. An empty
// level means view. An entry with only a repo has no checkout and is reported
// as unreachable by the view.
func ParseRegistry(data []byte) ([]RegistryEntry, error) {
	var f registryFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("registry: %v", err)
	}
	if dec.More() {
		return nil, errors.New("registry: trailing data after the JSON object")
	}
	if len(f.Hives) > maxHives {
		return nil, fmt.Errorf("registry: %d hives listed, the limit is %d", len(f.Hives), maxHives)
	}
	seen := map[string]bool{}
	for i := range f.Hives {
		e := &f.Hives[i]
		if !hiveNameRe.MatchString(e.Name) {
			return nil, fmt.Errorf("registry: hive %d: name %q must match %s", i+1, e.Name, hiveNameRe)
		}
		if seen[e.Name] {
			return nil, fmt.Errorf("registry: duplicate hive name %q", e.Name)
		}
		seen[e.Name] = true
		switch e.Level {
		case "":
			e.Level = levelView
		case levelView, levelPlan, levelWork:
		default:
			return nil, fmt.Errorf("registry: hive %q: level %q must be view, plan or work", e.Name, e.Level)
		}
		if e.Path == "" && e.Repo == "" {
			return nil, fmt.Errorf("registry: hive %q needs a path or a repo", e.Name)
		}
		if e.Path != "" && !strings.HasPrefix(e.Path, "/") {
			return nil, fmt.Errorf("registry: hive %q: path must be absolute", e.Name)
		}
	}
	return f.Hives, nil
}

// Task is one brief under .waggle/hive/tasks/, reduced to validated fields.
type Task struct {
	ID        string
	Title     string
	Done      bool
	Dropped   bool // done by a word that means the work will not happen (cancelled, rejected, dropped...)
	DependsOn []string
}

// HiveInput is what hives.go read from one reachable hive.
type HiveInput struct {
	Leases      []comb.Lease
	Malformed   int
	Collisions  []comb.CaseCollision
	Tasks       []Task
	Decisions   []decisions.Entry
	DecisionErr string // non-empty when the decision log could not be parsed
}

// ClaimView is one active claim, as `waggle status` lists it.
type ClaimView struct {
	Ticket      string   `json:"ticket"`
	Role        string   `json:"role"`
	Keeper      string   `json:"keeper"`
	Gate        string   `json:"gate"`
	Directories []string `json:"directories"`
	ExpiresIn   string   `json:"expires_in"`
	ExpiresAt   string   `json:"expires_at"`
}

// TaskView is an open or blocked task.
type TaskView struct {
	Ticket    string   `json:"ticket"`
	Title     string   `json:"title"`
	BlockedBy []string `json:"blocked_by,omitempty"`
}

// CollisionView is one pair of active claims whose directories differ only in
// letter case, as `waggle status` warns about it.
type CollisionView struct {
	TicketA string `json:"ticket_a"`
	DirA    string `json:"dir_a"`
	TicketB string `json:"ticket_b"`
	DirB    string `json:"dir_b"`
}

// Card is a decision still waiting on the Keeper, tagged with its hive. The
// view lists it; it never answers it (RFC-0004 open question 4).
type Card struct {
	Hive  string `json:"hive"`
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Date  string `json:"date"`
	Title string `json:"title"`
}

// HiveView is the read-only view of one home hive.
type HiveView struct {
	Name           string          `json:"name"`
	Path           string          `json:"path,omitempty"`
	Repo           string          `json:"repo,omitempty"`
	Level          string          `json:"level"`
	Reachable      bool            `json:"reachable"`
	Error          string          `json:"error,omitempty"`
	Claims         []ClaimView     `json:"claims"`
	MalformedCount int             `json:"malformed_claims"`
	Collisions     []CollisionView `json:"case_collisions"`
	Open           []TaskView      `json:"open"`
	Blocked        []TaskView      `json:"blocked"`
	Waiting        []Card          `json:"waiting_on_keeper"`
	DecisionError  string          `json:"decision_log_error,omitempty"`
	Done           []string        `json:"-"` // tasks the hive's briefs mark done; used by plans, not in the status report
	Dropped        []string        `json:"-"` // the subset of Done that was cancelled, rejected or dropped; used by plans
}

// HivesReport is the whole answer: one view per registry entry, in registry
// order, and the merged card list.
type HivesReport struct {
	Hives []HiveView `json:"hives"`
	Cards []Card     `json:"cards"`
}

// UnreachableView is the view of a hive that could not be read. It does not
// guess: every list is empty and the reason is stated.
func UnreachableView(e RegistryEntry, reason string) HiveView {
	return HiveView{Name: e.Name, Path: e.Path, Repo: e.Repo, Level: e.Level, Error: flatten(reason),
		Claims: []ClaimView{}, Collisions: []CollisionView{}, Open: []TaskView{}, Blocked: []TaskView{}, Waiting: []Card{}}
}

// BuildHiveView derives a hive's view from what was read. Claims are the
// active leases in ticket order, as `waggle status` prints them. A task is
// done, claimed (listed under claims only), blocked (it depends on a task of
// the same hive that is not done) or open. A card is a decision whose Keeper
// choice is still not decided.
func BuildHiveView(e RegistryEntry, in HiveInput, now time.Time) HiveView {
	v := UnreachableView(e, "")
	v.Reachable = true
	v.MalformedCount = in.Malformed
	for _, c := range in.Collisions {
		v.Collisions = append(v.Collisions, CollisionView{TicketA: c.TicketA, DirA: c.DirA, TicketB: c.TicketB, DirB: c.DirB})
	}

	leases := slices.Clone(in.Leases)
	sort.SliceStable(leases, func(i, j int) bool { return ticketNum(leases[i].TicketID) < ticketNum(leases[j].TicketID) })
	claimed := map[string]bool{}
	for _, l := range leases {
		claimed[canonTicket(l.TicketID)] = true
		v.Claims = append(v.Claims, ClaimView{
			Ticket: l.TicketID, Role: l.Role, Keeper: l.KeeperID, Gate: l.Gate,
			Directories: slices.Clone(l.Directories), ExpiresIn: expiresIn(l.ExpiresAt.Sub(now)),
			ExpiresAt: l.ExpiresAt.UTC().Format(time.RFC3339),
		})
	}

	known := map[string]Task{}
	for _, t := range in.Tasks {
		known[t.ID] = t
	}
	tasks := slices.Clone(in.Tasks)
	sort.SliceStable(tasks, func(i, j int) bool { return ticketNum(tasks[i].ID) < ticketNum(tasks[j].ID) })
	for _, t := range tasks {
		if t.Done {
			v.Done = append(v.Done, t.ID)
			if t.Dropped {
				v.Dropped = append(v.Dropped, t.ID)
			}
		}
		if t.Done || claimed[t.ID] {
			continue
		}
		var by []string
		for _, d := range t.DependsOn {
			if dep, ok := known[d]; ok && !dep.Done && d != t.ID {
				by = append(by, d)
			}
		}
		if len(by) > 0 {
			v.Blocked = append(v.Blocked, TaskView{Ticket: t.ID, Title: flatten(t.Title), BlockedBy: by})
		} else {
			v.Open = append(v.Open, TaskView{Ticket: t.ID, Title: flatten(t.Title)})
		}
	}

	v.DecisionError = flatten(in.DecisionErr)
	for _, d := range in.Decisions {
		if !undecided(d.Choice) {
			continue
		}
		v.Waiting = append(v.Waiting, Card{Hive: e.Name, ID: fmt.Sprintf("D-%03d", d.ID), Kind: string(d.Kind),
			Date: d.Date, Title: flatten(d.Title)})
	}
	return v
}

// undecided reports whether a decision's Keeper choice records no answer yet.
func undecided(choice string) bool {
	c := strings.ToLower(strings.TrimSpace(choice))
	return c == "" || strings.HasPrefix(c, "not decided") || strings.HasPrefix(c, "pending")
}

// canonTicket is the canonical T-<nnn> form of id, or id itself when unparsable.
func canonTicket(id string) string {
	if c, err := comb.ParseTicketID(id); err == nil {
		return c
	}
	return id
}

// flatten makes hive text safe to show: control, format and line-separator
// characters become spaces, runs of space collapse, and the result is cut to
// maxTitle runes.
func flatten(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) ||
			unicode.Is(unicode.Zp, r) || r == utf8.RuneError {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > maxTitle {
		s = string(r[:maxTitle-3]) + "..."
	}
	return s
}

// BuildReport assembles the report from views in registry order. The merged
// card list keeps registry order, then decision ID, so it is deterministic.
func BuildReport(views []HiveView) HivesReport {
	r := HivesReport{Hives: views, Cards: []Card{}}
	for _, v := range views {
		r.Cards = append(r.Cards, v.Waiting...)
	}
	return r
}

// RenderHivesJSON renders the report as indented JSON with a trailing newline.
func RenderHivesJSON(r HivesReport) []byte {
	out, _ := json.MarshalIndent(r, "", "  ")
	return append(out, '\n')
}

// RenderHivesText renders the report for a terminal. Claim lines use the same
// fields, in the same order, as the claims table of `waggle status`.
func RenderHivesText(r HivesReport) []byte {
	var b bytes.Buffer
	reached := 0
	for _, v := range r.Hives {
		if v.Reachable {
			reached++
		}
	}
	fmt.Fprintf(&b, "Hives: %d registered, %d read. Read only: nothing is written to any hive.\n", len(r.Hives), reached)
	for _, v := range r.Hives {
		fmt.Fprintf(&b, "\n%s  (%s)\n", v.Name, v.Level)
		if !v.Reachable {
			fmt.Fprintf(&b, "  unreachable: %s\n", v.Error)
			continue
		}
		fmt.Fprintf(&b, "  claimed (%d):\n", len(v.Claims))
		tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		for _, c := range v.Claims {
			fmt.Fprintf(tw, "    %s\t%s\t%s\t%s\t%s/\texpires in %s\n", c.Ticket, c.Role, c.Keeper, c.Gate,
				strings.Join(c.Directories, "/ "), c.ExpiresIn)
		}
		tw.Flush()
		if v.MalformedCount > 0 {
			fmt.Fprintf(&b, "    malformed: %d\n", v.MalformedCount)
		}
		for _, c := range v.Collisions {
			fmt.Fprintf(&b, "    %s\n", collisionWarning(comb.CaseCollision{TicketA: c.TicketA, DirA: c.DirA, TicketB: c.TicketB, DirB: c.DirB}))
		}
		fmt.Fprintf(&b, "  open (%d):\n", len(v.Open))
		for _, t := range v.Open {
			fmt.Fprintf(&b, "    %s  %s\n", t.Ticket, t.Title)
		}
		fmt.Fprintf(&b, "  blocked (%d):\n", len(v.Blocked))
		for _, t := range v.Blocked {
			fmt.Fprintf(&b, "    %s  %s  (waiting on %s)\n", t.Ticket, t.Title, strings.Join(t.BlockedBy, ", "))
		}
		fmt.Fprintf(&b, "  waiting on the Keeper (%d):\n", len(v.Waiting))
		for _, c := range v.Waiting {
			fmt.Fprintf(&b, "    %s  %s\n", c.ID, c.Title)
		}
		if v.DecisionError != "" {
			fmt.Fprintf(&b, "  decision log not read: %s\n", v.DecisionError)
		}
	}
	fmt.Fprintf(&b, "\nDecision cards waiting on the Keeper, all hives (%d). Answer each in its own hive:\n", len(r.Cards))
	for _, c := range r.Cards {
		fmt.Fprintf(&b, "  %s  %s  %s  %s\n", c.Hive, c.ID, c.Kind, c.Title)
	}
	return b.Bytes()
}
