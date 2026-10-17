package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Caps on a handoff file: more than this and the launcher escalates instead
// of seeding a fresh session with it.
const (
	handoffMaxLines = 150
	handoffMaxBytes = 16 << 10
)

// ErrBadHandoff reports a handoff that is over the caps or not in the fixed format.
var ErrBadHandoff = errors.New("bad handoff")

// handoffHeadings are the five section headings, in the only order allowed.
var handoffHeadings = [...]string{"Done", "Next", "Files touched", "Open questions", "Verification"}

// Handoff is what a session leaves for its successor when its context reaches
// the profile's handoff threshold. Seq numbers the handoffs of one role on one
// task, starting at 1. Each list renders as bullets under a fixed heading.
type Handoff struct {
	Task, Role                                            string
	Seq                                                   int
	Done, Next, FilesTouched, OpenQuestions, Verification []string
}

func (h *Handoff) sections() []*[]string {
	return []*[]string{&h.Done, &h.Next, &h.FilesTouched, &h.OpenQuestions, &h.Verification}
}

// RenderHandoff writes h in the fixed format ParseHandoff reads: a title line
// "# Handoff: <task> <role> <seq>", then each heading preceded by a blank line
// and followed by its bullets. An empty list renders as a bare heading.
func RenderHandoff(h Handoff) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Handoff: %s %s %d\n", h.Task, h.Role, h.Seq)
	for i, items := range h.sections() {
		fmt.Fprintf(&b, "\n## %s\n", handoffHeadings[i])
		for _, item := range *items {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	return b.Bytes()
}

// ParseHandoff reads a handoff in RenderHandoff's format. It is strict: the
// caps, the title line, every heading in order, bullets only, exactly one
// blank line between sections and a final newline, so that
// RenderHandoff(ParseHandoff(b)) == b for every b it accepts. Any deviation
// wraps ErrBadHandoff with the line number.
func ParseHandoff(b []byte) (Handoff, error) {
	var h Handoff
	if len(b) > handoffMaxBytes {
		return h, fmt.Errorf("%w: %d bytes, cap %d", ErrBadHandoff, len(b), handoffMaxBytes)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		return h, fmt.Errorf("%w: missing final newline", ErrBadHandoff)
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(lines) > handoffMaxLines {
		return h, fmt.Errorf("%w: %d lines, cap %d", ErrBadHandoff, len(lines), handoffMaxLines)
	}
	title, ok := strings.CutPrefix(lines[0], "# Handoff: ")
	fields := strings.Fields(title)
	if !ok || len(fields) != 3 || title != strings.Join(fields, " ") {
		return h, fmt.Errorf("%w: line 1 is not \"# Handoff: <task> <role> <seq>\"", ErrBadHandoff)
	}
	seq, err := strconv.Atoi(fields[2])
	if err != nil || seq < 1 || strconv.Itoa(seq) != fields[2] {
		return h, fmt.Errorf("%w: line 1 sequence %q is not a positive integer", ErrBadHandoff, fields[2])
	}
	h.Task, h.Role, h.Seq = fields[0], fields[1], seq
	i := 1
	for si, items := range h.sections() {
		heading := "## " + handoffHeadings[si]
		if i+1 >= len(lines) || lines[i] != "" || lines[i+1] != heading {
			return Handoff{}, fmt.Errorf("%w: expected a blank line then %q at line %d", ErrBadHandoff, heading, i+1)
		}
		for i += 2; i < len(lines) && lines[i] != ""; i++ {
			text, ok := strings.CutPrefix(lines[i], "- ")
			if !ok {
				return Handoff{}, fmt.Errorf("%w: line %d is not a bullet", ErrBadHandoff, i+1)
			}
			*items = append(*items, text)
		}
	}
	if i != len(lines) {
		return Handoff{}, fmt.Errorf("%w: unexpected content at line %d", ErrBadHandoff, i+1)
	}
	return h, nil
}
