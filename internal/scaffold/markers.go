// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"errors"
	"fmt"
	"strings"
)

// Marker pairs for the blocks Waggle owns inside project files.
const (
	GitignoreStart = "# --- waggle:start ---"
	GitignoreEnd   = "# --- waggle:end ---"
	MarkdownStart  = "<!-- waggle:start -->"
	MarkdownEnd    = "<!-- waggle:end -->"
)

// ErrMarkers reports a file whose Waggle markers do not form exactly zero or one
// well-ordered block.
var ErrMarkers = errors.New("scaffold: malformed waggle marker block")

// findBlock splits content into lines (each keeping its newline) and returns
// the indexes of the start and end marker lines, or -1, -1 when neither exists.
// Markers must occupy a whole line; trailing spaces and a CR are tolerated.
func findBlock(content, start, end string) ([]string, int, int, error) {
	lines := strings.SplitAfter(content, "\n")
	si, ei := -1, -1
	for i, l := range lines {
		switch strings.TrimRight(l, " \t\r\n") {
		case start:
			if si >= 0 {
				return nil, 0, 0, fmt.Errorf("two %q lines: %w", start, ErrMarkers)
			}
			si = i
		case end:
			if ei >= 0 {
				return nil, 0, 0, fmt.Errorf("two %q lines: %w", end, ErrMarkers)
			}
			ei = i
		}
	}
	switch {
	case si < 0 && ei < 0:
		return lines, -1, -1, nil
	case si < 0:
		return nil, 0, 0, fmt.Errorf("%q without %q: %w", end, start, ErrMarkers)
	case ei < 0:
		return nil, 0, 0, fmt.Errorf("%q without %q: %w", start, end, ErrMarkers)
	case ei < si:
		return nil, 0, 0, fmt.Errorf("%q before %q: %w", end, start, ErrMarkers)
	}
	return lines, si, ei, nil
}

// bodyLines returns body as whole lines, each ending in a newline.
func bodyLines(body string) string {
	if body == "" || strings.HasSuffix(body, "\n") {
		return body
	}
	return body + "\n"
}

// UpsertBlock returns content with exactly one Waggle block holding body.
// With no block it appends one, first adding a trailing newline to content if
// missing. With one block it replaces only the lines between the markers.
// Malformed markers return ErrMarkers and leave content unchanged.
func UpsertBlock(content, start, end, body string) (string, error) {
	lines, si, ei, err := findBlock(content, start, end)
	if err != nil {
		return content, err
	}
	if si < 0 {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		return content + start + "\n" + bodyLines(body) + end + "\n", nil
	}
	return strings.Join(lines[:si+1], "") + bodyLines(body) + strings.Join(lines[ei:], ""), nil
}

// RemoveBlock is the inverse of UpsertBlock: it deletes the marker lines and
// everything between them, and reports whether a block was found. Content that
// UpsertBlock appended to is restored byte for byte, except that a newline
// UpsertBlock added to unterminated content stays.
func RemoveBlock(content, start, end string) (string, bool, error) {
	lines, si, ei, err := findBlock(content, start, end)
	if err != nil {
		return content, false, err
	}
	if si < 0 {
		return content, false, nil
	}
	return strings.Join(lines[:si], "") + strings.Join(lines[ei+1:], ""), true, nil
}
