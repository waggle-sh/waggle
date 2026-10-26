package decisions

import (
	"bytes"
	"fmt"
	"strings"
)

// MaxLogBytes is the largest DECISIONS.md Append and the merge gate will read.
const MaxLogBytes = maxFileBytes

// VerifyTransition reports whether a DECISIONS.md change from old to next is a
// pure append of well-formed entries. oldExists and nextExists say whether the
// file is present at each revision. It returns the IDs of the appended entries.
//
// Every failure wraps ErrNotAppendOnly or ErrCorrupt and names the side (base
// or head) and, where known, the entry ID or line number. A corrupt base fails
// every change: the gate never accepts a transition whose starting point
// already fails integrity, so a repair needs an admin merge.
// The byte rule itself (old is an exact prefix of next, and the rest is the
// rendering of the new entries) is VerifyAppendOnly's; this function adds the
// existence cases, the unchanged case, and the diagnostics.
func VerifyTransition(old []byte, oldExists bool, next []byte, nextExists bool) (added []int, err error) {
	switch {
	case !oldExists && !nextExists:
		return nil, nil
	case oldExists && !nextExists:
		return nil, fmt.Errorf("%w: head: DECISIONS.md deleted", ErrNotAppendOnly)
	case !oldExists:
		old = nil
	}
	oldEntries, err := Parse(old)
	if err != nil {
		return nil, fmt.Errorf("base DECISIONS.md fails integrity; repair needs an admin merge: %w", err)
	}
	nextEntries, err := Parse(next)
	if err != nil {
		// A head that no longer starts with the base bytes is reported as the
		// base change it is, which names the line, rather than as corruption.
		if len(old) > 0 && !bytes.HasPrefix(next, old) {
			return nil, prefixError(old, next, oldEntries)
		}
		return nil, fmt.Errorf("head DECISIONS.md: %w", err)
	}
	if bytes.Equal(old, next) {
		return nil, nil
	}
	o, n := len(oldEntries), len(nextEntries)
	if n <= o {
		return nil, fmt.Errorf("%w: head: entries removed or rewritten (base has %d entries, head has %d)", ErrNotAppendOnly, o, n)
	}
	if err := VerifyAppendOnly(old, next); err != nil {
		if !bytes.HasPrefix(next, old) {
			return nil, prefixError(old, next, oldEntries)
		}
		return nil, fmt.Errorf("head: %w", err)
	}
	// Parse already guarantees both of these; assert them so a regression
	// there still names the bad entry here.
	for i, e := range nextEntries {
		if e.ID != i+1 {
			return nil, fmt.Errorf("%w: head: entry %d has ID %s, expected %s", ErrCorrupt, i+1, formatID(e.ID), formatID(i+1))
		}
	}
	if o > 0 && nextEntries[o].Date < oldEntries[o-1].Date {
		return nil, fmt.Errorf("%w: head: %s is dated %s, earlier than %s (%s)", ErrCorrupt, formatID(o+1), nextEntries[o].Date, formatID(o), oldEntries[o-1].Date)
	}
	for id := o + 1; id <= n; id++ {
		added = append(added, id)
	}
	return added, nil
}

// prefixError names the first head line that differs from base and the base
// entry (or the header) whose rendering contains that line.
func prefixError(old, next []byte, oldEntries []Entry) error {
	i := 0
	for i < len(old) && i < len(next) && old[i] == next[i] {
		i++
	}
	line := bytes.Count(next[:i], []byte{'\n'}) + 1
	where, end := "the header", strings.Count(header, "\n")
	for _, e := range oldEntries {
		if line <= end {
			break
		}
		where, end = formatID(e.ID), end+1+bytes.Count(RenderEntry(e), []byte{'\n'})
	}
	return fmt.Errorf("%w: head line %d differs from base inside %s; existing entries must be byte-identical", ErrNotAppendOnly, line, where)
}
