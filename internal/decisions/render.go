// SPDX-License-Identifier: Apache-2.0

package decisions

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// header opens every rendered log; one blank line precedes each entry.
const header = "<!-- SPDX-License-Identifier: MIT-0 -->\n" +
	"<!-- GENERATED FILE: append-only, written by `waggle decide`. Never edit or delete an entry; record a new one that supersedes it. -->\n" +
	"# DECISIONS\n"

// Render returns the whole log for entries: header, then each entry
// preceded by one blank line. LF endings, single trailing newline.
func Render(entries []Entry) []byte {
	var b bytes.Buffer
	b.WriteString(header)
	for _, e := range entries {
		b.WriteByte('\n')
		b.Write(RenderEntry(e))
	}
	return b.Bytes()
}

// RenderEntry returns one entry in the fixed section format, ending in a
// single newline. Every text field sits after a prefix on its own line, so
// no input can start a line or forge a heading.
func RenderEntry(e Entry) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "## %s: %s\n\n", formatID(e.ID), e.Title)
	fmt.Fprintf(&b, "- **Date:** %s\n- **Kind:** %s\n", e.Date, e.Kind)
	fmt.Fprintf(&b, "- **Tickets:** %s\n- **Supersedes:** %s\n\n", joinIDs("T-", e.Tickets), joinIDs("D-", e.Supersedes))
	fmt.Fprintf(&b, "**Question:** %s\n\n**Options considered:**\n", e.Question)
	for i, o := range e.Options {
		fmt.Fprintf(&b, "%d. %s\n", i+1, o)
	}
	fmt.Fprintf(&b, "\n**Hive recommendation:** %s\n\n**Keeper choice:** %s\n\n**Rationale:** %s\n", e.Recommendation, e.Choice, e.Rationale)
	return b.Bytes()
}

func joinIDs(prefix string, ns []int) string {
	if len(ns) == 0 {
		return "none"
	}
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = fmt.Sprintf("%s%03d", prefix, n)
	}
	return strings.Join(parts, ", ")
}

// Parse reads a log in the exact Render format. It enforces that IDs run
// 1..n, dates are valid and non-decreasing, every supersedes points to a
// lower ID, and Render(Parse(data)) == data byte for byte. Any failure is
// ErrCorrupt naming the first offending line. Empty data is zero entries.
func Parse(data []byte) ([]Entry, error) {
	if len(data) == 0 {
		return nil, nil
	}
	got := strings.Split(string(data), "\n")
	if data[len(data)-1] != '\n' {
		return nil, corrupt(len(got), "missing trailing newline")
	}
	p := &parser{lines: got[:len(got)-1]}
	for _, h := range strings.Split(strings.TrimSuffix(header, "\n"), "\n") {
		if _, err := p.line(h, false); err != nil {
			return nil, err
		}
	}
	var entries []Entry
	for p.pos < len(p.lines) {
		e, err := p.entry(entries)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if rendered := Render(entries); !bytes.Equal(rendered, data) {
		want := strings.Split(string(rendered), "\n")
		n := 0
		for n < len(got) && n < len(want) && got[n] == want[n] {
			n++
		}
		return nil, corrupt(n+1, "line differs from the canonical rendering")
	}
	return entries, nil
}

func corrupt(line int, msg string) error {
	return fmt.Errorf("%w: line %d: %s", ErrCorrupt, line, msg)
}

type parser struct {
	lines []string
	pos   int // lines consumed; also the 1-based number of the last line read
}

// line consumes one line. With rest, the line must start with prefix and
// the remainder is returned; otherwise the line must equal prefix.
func (p *parser) line(prefix string, rest bool) (string, error) {
	if p.pos >= len(p.lines) {
		return "", corrupt(p.pos+1, fmt.Sprintf("unexpected end of file, expected %q", prefix))
	}
	p.pos++
	l := p.lines[p.pos-1]
	if !strings.HasPrefix(l, prefix) || (!rest && l != prefix) {
		return "", corrupt(p.pos, fmt.Sprintf("expected %q", prefix))
	}
	return l[len(prefix):], nil
}

type step struct {
	prefix string
	dst    *string // nil: the line must equal prefix exactly
}

func (p *parser) steps(steps []step) error {
	for _, s := range steps {
		v, err := p.line(s.prefix, s.dst != nil)
		if err != nil {
			return err
		}
		if s.dst != nil {
			*s.dst = v
		}
	}
	return nil
}

// entry parses one section following the entries parsed so far. The heading
// prefix carries the expected ID, which enforces contiguity.
func (p *parser) entry(prev []Entry) (Entry, error) {
	e := Entry{ID: len(prev) + 1}
	var kind, tickets, sup string
	heading := p.pos + 2
	err := p.steps([]step{
		{"", nil}, {"## " + formatID(e.ID) + ": ", &e.Title}, {"", nil},
		{"- **Date:** ", &e.Date}, {"- **Kind:** ", &kind},
		{"- **Tickets:** ", &tickets}, {"- **Supersedes:** ", &sup},
		{"", nil}, {"**Question:** ", &e.Question}, {"", nil}, {"**Options considered:**", nil},
	})
	if err != nil {
		return e, err
	}
	for {
		l, err := p.line("", true)
		if err != nil {
			return e, err
		}
		if l == "" {
			break
		}
		prefix := strconv.Itoa(len(e.Options)+1) + ". "
		if !strings.HasPrefix(l, prefix) {
			return e, corrupt(p.pos, fmt.Sprintf("expected option %q", prefix+"<text>"))
		}
		e.Options = append(e.Options, l[len(prefix):])
	}
	if len(e.Options) == 0 {
		return e, corrupt(p.pos, "entry has no options")
	}
	err = p.steps([]step{
		{"**Hive recommendation:** ", &e.Recommendation}, {"", nil},
		{"**Keeper choice:** ", &e.Choice}, {"", nil}, {"**Rationale:** ", &e.Rationale},
	})
	if err != nil {
		return e, err
	}
	e.Kind = Kind(kind)
	if _, err := time.Parse("2006-01-02", e.Date); err != nil {
		return e, corrupt(heading+2, "date is not YYYY-MM-DD")
	}
	if len(prev) > 0 && e.Date < prev[len(prev)-1].Date {
		return e, corrupt(heading+2, "date is earlier than the previous entry")
	}
	if e.Tickets, err = splitIDs("T-", tickets); err != nil {
		return e, corrupt(heading+4, err.Error())
	}
	if e.Supersedes, err = splitIDs("D-", sup); err != nil {
		return e, corrupt(heading+5, err.Error())
	}
	for _, s := range e.Supersedes {
		if s >= e.ID {
			return e, corrupt(heading+5, "supersedes must point to an earlier entry")
		}
	}
	return e, nil
}

// splitIDs parses "none" or a ", "-joined ID list. Non-canonical forms and
// ordering are caught by the round-trip check in Parse.
func splitIDs(prefix, s string) ([]int, error) {
	if s == "none" {
		return nil, nil
	}
	var ns []int
	for _, part := range strings.Split(s, ", ") {
		n, err := strconv.Atoi(strings.TrimPrefix(part, prefix))
		if err != nil || n < 1 {
			return nil, fmt.Errorf("%q is not a %sNNN ID", part, prefix)
		}
		ns = append(ns, n)
	}
	return ns, nil
}
