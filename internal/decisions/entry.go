// Package decisions is the append-only writer for the hive decision log,
// .waggle/hive/DECISIONS.md. It validates one structured entry, assigns the
// next ID, and atomically appends the rendered entry; nothing in this package
// edits, reorders, or deletes an existing entry.
//
// The command cannot verify that a decision was approved. In v0.1 `waggle
// decide` is run by the Keeper, or by the queen only after an explicit Keeper
// answer in the same session. Later waggled calls Append when a decision card
// is answered over run.sock. Cells never get this command.
//
// The package never reads the clock; callers pass the time so tests stay
// deterministic.
package decisions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	// ErrInvalidInput marks a rejected input; the message names the field.
	ErrInvalidInput = errors.New("decisions: invalid input")
	// ErrCorrupt marks a DECISIONS.md that fails the integrity check.
	ErrCorrupt = errors.New("decisions: DECISIONS.md failed integrity check")
	// ErrLocked marks a lock file held by another writer.
	ErrLocked = errors.New("decisions: DECISIONS.md is locked by another writer")
	// ErrNotAppendOnly marks a change that is not a pure append of entries.
	ErrNotAppendOnly = errors.New("decisions: change is not a pure append")
)

// Kind classifies a decision. Exactly the eight constants below are valid.
type Kind string

// The eight decision kinds.
const (
	KindPublish     Kind = "publish"
	KindSpend       Kind = "spend"
	KindDelete      Kind = "delete"
	KindCredentials Kind = "credentials"
	KindStrategy    Kind = "strategy"
	KindPolicy      Kind = "policy"
	KindTechnical   Kind = "technical"
	KindScope       Kind = "scope"
)

// MaxInputBytes is the largest JSON input DecodeInput accepts.
const MaxInputBytes = 64 << 10

// Input is the JSON object `waggle decide --file` reads. Tickets accept
// "T-007" or "7"; supersedes entries are canonical IDs such as "D-007".
type Input struct {
	Kind           string   `json:"kind"`
	Title          string   `json:"title"`
	Question       string   `json:"question"`
	Options        []string `json:"options"`
	Recommendation string   `json:"recommendation"`
	Choice         string   `json:"choice"`
	Rationale      string   `json:"rationale"`
	Tickets        []string `json:"tickets"`
	Supersedes     []string `json:"supersedes"`
}

// Entry is one validated, rendered decision.
type Entry struct {
	ID              int
	Date            string // YYYY-MM-DD, UTC
	Kind            Kind
	Title, Question string
	Options         []string
	Recommendation  string
	Choice          string
	Rationale       string
	Tickets         []int
	Supersedes      []int
}

// DecodeInput reads exactly one JSON object from r. Unknown fields, trailing
// data, and input over MaxInputBytes are rejected with ErrInvalidInput.
func DecodeInput(r io.Reader) (Input, error) {
	var in Input
	data, err := io.ReadAll(io.LimitReader(r, MaxInputBytes+1))
	if err != nil {
		return in, fmt.Errorf("%w: reading: %v", ErrInvalidInput, err)
	}
	if len(data) > MaxInputBytes {
		return in, fmt.Errorf("%w: input exceeds %d bytes", ErrInvalidInput, MaxInputBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return in, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if err := dec.Decode(new(json.RawMessage)); err != io.EOF {
		return in, fmt.Errorf("%w: trailing data after the JSON object", ErrInvalidInput)
	}
	return in, nil
}

// Text field byte limits (section 4.2 of the T-006 brief, title widened to
// 200 so the backfill titles fit).
const (
	maxTitle          = 200
	maxQuestion       = 1000
	maxOption         = 500
	maxRecommendation = 1000
	maxChoice         = 1000
	maxRationale      = 2000
)

// entry validates in against a log holding IDs 1..known and returns the
// entry with every field but ID and Date set.
func (in Input) entry(known int) (Entry, error) {
	e := Entry{Kind: Kind(in.Kind)}
	switch e.Kind {
	case KindPublish, KindSpend, KindDelete, KindCredentials, KindStrategy, KindPolicy, KindTechnical, KindScope:
	default:
		return e, fmt.Errorf("%w: kind %q is not one of publish, spend, delete, credentials, strategy, policy, technical, scope", ErrInvalidInput, in.Kind)
	}
	texts := []struct {
		field string
		dst   *string
		src   string
		max   int
	}{
		{"title", &e.Title, in.Title, maxTitle},
		{"question", &e.Question, in.Question, maxQuestion},
		{"recommendation", &e.Recommendation, in.Recommendation, maxRecommendation},
		{"choice", &e.Choice, in.Choice, maxChoice},
		{"rationale", &e.Rationale, in.Rationale, maxRationale},
	}
	for _, t := range texts {
		if err := checkText(t.field, t.src, t.max); err != nil {
			return e, err
		}
		*t.dst = t.src
	}
	if n := len(in.Options); n < 1 || n > 10 {
		return e, fmt.Errorf("%w: options must have 1 to 10 entries, got %d", ErrInvalidInput, n)
	}
	for i, o := range in.Options {
		if err := checkText(fmt.Sprintf("options[%d]", i), o, maxOption); err != nil {
			return e, err
		}
	}
	e.Options = slices.Clone(in.Options)
	if n := len(in.Tickets); n > 20 {
		return e, fmt.Errorf("%w: tickets must have at most 20 entries, got %d", ErrInvalidInput, n)
	}
	for _, t := range in.Tickets {
		n, err := parseTicket(t)
		if err != nil {
			return e, fmt.Errorf("%w: tickets: %v", ErrInvalidInput, err)
		}
		e.Tickets = append(e.Tickets, n)
	}
	if n := len(in.Supersedes); n > 10 {
		return e, fmt.Errorf("%w: supersedes must have at most 10 entries, got %d", ErrInvalidInput, n)
	}
	for _, s := range in.Supersedes {
		n, err := parseDecisionID(s)
		if err != nil {
			return e, fmt.Errorf("%w: supersedes: %v", ErrInvalidInput, err)
		}
		if n > known {
			return e, fmt.Errorf("%w: supersedes: %s does not exist (last entry is %s)", ErrInvalidInput, s, formatID(known))
		}
		e.Supersedes = append(e.Supersedes, n)
	}
	e.Tickets, e.Supersedes = sortUnique(e.Tickets), sortUnique(e.Supersedes)
	return e, nil
}

// checkText enforces the single-line, control-free, untrimmed rule so that
// input bytes equal rendered bytes.
func checkText(field, s string, max int) error {
	if n := len(s); n < 1 || n > max {
		return fmt.Errorf("%w: %s must be 1 to %d bytes, got %d", ErrInvalidInput, field, max, n)
	}
	if !utf8.ValidString(s) {
		return fmt.Errorf("%w: %s is not valid UTF-8", ErrInvalidInput, field)
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: %s contains a control character (U+%04X); fields are single-line", ErrInvalidInput, field, r)
		}
	}
	if strings.TrimSpace(s) != s {
		return fmt.Errorf("%w: %s has leading or trailing whitespace", ErrInvalidInput, field)
	}
	return nil
}

// digits parses 1 to 6 ASCII digits with value >= 1.
func digits(s string) (int, bool) {
	if len(s) < 1 || len(s) > 6 || strings.Trim(s, "0123456789") != "" {
		return 0, false
	}
	n, _ := strconv.Atoi(s)
	return n, n >= 1
}

// parseTicket accepts ^(T-)?[0-9]{1,6}$ with value >= 1 (D-003).
// TODO(T-006): unify with comb.ParseTicketID after T-002 merges
func parseTicket(s string) (int, error) {
	n, ok := digits(strings.TrimPrefix(s, "T-"))
	if !ok {
		return 0, fmt.Errorf("%q is not a ticket ID (T-<n> or <n>, 1 to 6 digits, value >= 1)", s)
	}
	return n, nil
}

// parseDecisionID accepts only the canonical D-%03d form.
func parseDecisionID(s string) (int, error) {
	n, ok := digits(strings.TrimPrefix(s, "D-"))
	if !ok || formatID(n) != s {
		return 0, fmt.Errorf("%q is not a canonical decision ID (D-%%03d)", s)
	}
	return n, nil
}

func formatID(n int) string { return fmt.Sprintf("D-%03d", n) }

func sortUnique(ns []int) []int {
	slices.Sort(ns)
	return slices.Compact(ns)
}
