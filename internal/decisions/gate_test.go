package decisions

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// gateEntries returns n valid entries with distinct titles and dates one day
// apart, built through the package's own next so they match Append's output.
func gateEntries(t *testing.T, n int) []Entry {
	t.Helper()
	var entries []Entry
	for i := 0; i < n; i++ {
		in := valid()
		in.Title = fmt.Sprintf("Decision number %d", i+1)
		e, err := next(entries, in, clock.AddDate(0, 0, i))
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	return entries
}

func withID(e Entry, id int) Entry { e.ID = id; return e }

// appended returns old plus the rendered entry, the shape Append writes.
func appended(old []byte, e Entry) []byte {
	return append(append(slices.Clone(old), '\n'), RenderEntry(e)...)
}

func TestVerifyTransition(t *testing.T) {
	es := gateEntries(t, 18)
	logN := func(n int) []byte { return Render(es[:n]) }
	log2, log3, log4 := logN(2), logN(3), logN(4)
	s2, e2 := section(log4, "D-002")
	editedD2 := slices.Concat(log4[:s2], bytes.Replace(log4[s2:e2], []byte("tiny."), []byte("tidy."), 1), log4[e2:])
	removedD2 := Render([]Entry{es[0], withID(es[2], 2), withID(es[3], 3), withID(es[4], 4)})
	inserted := Render([]Entry{es[0], withID(es[3], 2), withID(es[1], 3), withID(es[2], 4)})
	swapped := Render([]Entry{withID(es[1], 1), withID(es[0], 2), es[2], es[3]})
	early := es[2]
	early.Date = "2026-10-01"
	truncated := log3[:bytes.LastIndexByte(log3[:len(log3)-1], '\n')+1]
	extraBlank := append(append(slices.Clone(log2), '\n'), appended(nil, es[2])...)
	crlf := append(append(slices.Clone(log2), '\n'), bytes.ReplaceAll(RenderEntry(es[2]), []byte("\n"), []byte("\r\n"))...)
	cases := []struct {
		name                 string
		old, next            []byte
		oldExists, nextExist bool
		want                 error // nil for pass
		added                []int
		msg                  string // substring the failure message must contain
	}{
		{"both absent", nil, nil, false, false, nil, nil, ""},
		{"absent to one", nil, logN(1), false, true, nil, []int{1}, ""},
		{"absent to eighteen", nil, logN(18), false, true, nil, seq(1, 18), ""},
		{"absent to empty", nil, []byte{}, false, true, nil, nil, ""},
		{"unchanged", log3, log3, true, true, nil, nil, ""},
		{"append one", log3, log4, true, true, nil, []int{4}, ""},
		{"append three", log3, logN(6), true, true, nil, []int{4, 5, 6}, ""},
		{"empty to one", []byte{}, logN(1), true, true, nil, []int{1}, ""},
		{"deleted", log3, nil, true, false, ErrNotAppendOnly, nil, "deleted"},
		{"edited old entry plus append", log3, editedD2, true, true, ErrNotAppendOnly, nil, "D-002"},
		{"removed and renumbered", log3, removedD2, true, true, ErrNotAppendOnly, nil, "D-002"},
		{"removed, no append", log3, Render([]Entry{es[0], withID(es[2], 2)}), true, true, ErrNotAppendOnly, nil, "removed or rewritten"},
		{"inserted in the middle", log3, inserted, true, true, ErrNotAppendOnly, nil, "D-002"},
		{"ID gap", log3, appended(log3, withID(es[4], 5)), true, true, ErrCorrupt, nil, "head"},
		{"duplicate last ID", log2, appended(log2, withID(es[2], 2)), true, true, ErrCorrupt, nil, "head"},
		{"dated before last old", log2, appended(log2, early), true, true, ErrCorrupt, nil, "head"},
		{"extra blank line", log2, extraBlank, true, true, ErrCorrupt, nil, "head"},
		{"CRLF entry", log2, crlf, true, true, ErrCorrupt, nil, "head"},
		{"truncated new entry", log2, truncated, true, true, ErrCorrupt, nil, "head"},
		{"header edited", log2, bytes.Replace(log3, []byte("# DECISIONS\n"), []byte("# DECISIONS LOG\n"), 1), true, true, ErrNotAppendOnly, nil, "header"},
		{"corrupt base", log2[:len(log2)-1], log3, true, true, ErrCorrupt, nil, "base"},
	}
	for _, c := range cases {
		added, err := VerifyTransition(c.old, c.oldExists, c.next, c.nextExist)
		if c.want == nil {
			if err != nil || !slices.Equal(added, c.added) {
				t.Errorf("%s: added %v, err %v; want %v, nil", c.name, added, err, c.added)
			}
			continue
		}
		if !errors.Is(err, c.want) || added != nil {
			t.Errorf("%s: added %v, err %v; want %v", c.name, added, err, c.want)
		} else if !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%s: %q does not mention %q", c.name, err, c.msg)
		}
	}
	// Two old entries swapped: either sentinel is acceptable, never a pass.
	if _, err := VerifyTransition(log3, true, swapped, true); !errors.Is(err, ErrNotAppendOnly) && !errors.Is(err, ErrCorrupt) {
		t.Errorf("swapped: got %v", err)
	}
}

func seq(lo, hi int) []int {
	var ns []int
	for i := lo; i <= hi; i++ {
		ns = append(ns, i)
	}
	return ns
}

// TestPrefixErrorNamesLine pins the line diagnostic: the header is three
// lines, so the first entry's heading is line 5 and an edit there names D-001.
func TestPrefixErrorNamesLine(t *testing.T) {
	es := gateEntries(t, 2)
	old := Render(es[:1])
	edited := bytes.Replace(Render(es), []byte("## D-001: Decision number 1"), []byte("## D-001: Decision number one"), 1)
	_, err := VerifyTransition(old, true, edited, true)
	if err == nil || !strings.Contains(err.Error(), "line 5 ") || !strings.Contains(err.Error(), "D-001") {
		t.Errorf("got %v", err)
	}
	_, err = VerifyTransition(old, true, bytes.Replace(Render(es), []byte("MIT-0"), []byte("MIT"), 1), true)
	if err == nil || !strings.Contains(err.Error(), "line 1 ") || !strings.Contains(err.Error(), "header") {
		t.Errorf("got %v", err)
	}
}
