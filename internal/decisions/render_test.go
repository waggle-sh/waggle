package decisions

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// threeEntries returns a valid three-entry log and its parsed entries.
func threeEntries(t *testing.T) ([]byte, []Entry) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "DECISIONS.md")
	for i := 0; i < 3; i++ {
		in := valid()
		if i == 2 {
			in.Supersedes = []string{"D-001"}
		}
		if _, err := Append(path, in, clock.AddDate(0, 0, i)); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return data, entries
}

func section(data []byte, id string) (start, end int) {
	start = bytes.Index(data, []byte("\n## "+id+": "))
	end = bytes.Index(data[start+1:], []byte("\n## "))
	if end < 0 {
		return start, len(data)
	}
	return start, start + 1 + end
}

func TestParseEmptyAndRoundTrip(t *testing.T) {
	if entries, err := Parse(nil); err != nil || len(entries) != 0 {
		t.Errorf("empty: %v %v", entries, err)
	}
	data, entries := threeEntries(t)
	if len(entries) != 3 || entries[2].ID != 3 || entries[2].Supersedes[0] != 1 || entries[1].Date != "2026-10-18" {
		t.Errorf("parsed %+v", entries)
	}
	if !bytes.Equal(Render(entries), data) {
		t.Error("Render(Parse(data)) != data")
	}
	if got := string(Render(nil)); got != header {
		t.Errorf("Render(nil) = %q", got)
	}
}

func TestCorruptionRefusal(t *testing.T) {
	good, entries := threeEntries(t)
	s2, e2 := section(good, "D-002")
	s3, e3 := section(good, "D-003")
	rev := []Entry{entries[0], entries[1]}
	rev[0].Date, rev[1].Date = "2026-10-18", "2026-10-17"
	fwd := []Entry{entries[0]}
	fwd[0].Supersedes = []int{2}
	bad := map[string][]byte{
		"gap":                  append(append([]byte{}, good[:s2]...), good[e2:]...),
		"swapped":              append(append(append([]byte{}, good[:s2]...), good[s3:e3]...), good[s2:e2]...),
		"field removed":        bytes.Replace(good, []byte("- **Kind:** technical\n"), nil, 1),
		"extra blank line":     bytes.Replace(good, []byte("\n## D-002"), []byte("\n\n## D-002"), 1),
		"CRLF":                 bytes.ReplaceAll(good, []byte("\n"), []byte("\r\n")),
		"no trailing newline":  good[:len(good)-1],
		"duplicate ID":         append(append([]byte{}, good...), good[s3:e3]...),
		"date backwards":       Render(rev),
		"supersedes forward":   Render(fwd),
		"missing header line":  bytes.Replace(good, []byte("# DECISIONS\n"), nil, 1),
		"heading padded":       bytes.Replace(good, []byte("## D-002:"), []byte("## D-0002:"), 1),
		"option renumbered":    bytes.Replace(good, []byte("\n2. B:"), []byte("\n3. B:"), 1),
		"ticket not canonical": bytes.Replace(good, []byte("T-004"), []byte("T-4"), 1),
		"bad date":             bytes.Replace(good, []byte("2026-10-17"), []byte("2026-13-17"), 1),
		"trailing junk":        append(append([]byte{}, good...), "\nx\n"...),
	}
	for name, data := range bad {
		_, err := Parse(data)
		if !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: Parse got %v, want ErrCorrupt", name, err)
			continue
		}
		if !strings.Contains(err.Error(), "line ") {
			t.Errorf("%s: error lacks a line number: %v", name, err)
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "DECISIONS.md")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Append(path, valid(), clock); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: Append got %v, want ErrCorrupt", name, err)
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(after, data) {
			t.Errorf("%s: file changed after refused append", name)
		}
		assertClean(t, dir)
	}
}

func TestCorruptionSymlinkAndSize(t *testing.T) {
	dir := t.TempDir()
	good, _ := threeEntries(t)
	real := filepath.Join(dir, "real.md")
	if err := os.WriteFile(real, good, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "DECISIONS.md")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(link, valid(), clock); !errors.Is(err, ErrCorrupt) {
		t.Errorf("symlink: got %v", err)
	}
	if after, _ := os.ReadFile(real); !bytes.Equal(after, good) {
		t.Error("symlink target changed")
	}
	big := filepath.Join(dir, "big.md")
	if err := os.WriteFile(big, make([]byte, maxFileBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(big, valid(), clock); !errors.Is(err, ErrCorrupt) {
		t.Errorf("over 4 MiB: got %v", err)
	}
	if _, err := Append(filepath.Join(dir), valid(), clock); !errors.Is(err, ErrCorrupt) {
		t.Errorf("directory: got %v", err)
	}
	assertClean(t, dir)
}

func TestVerifyAppendOnly(t *testing.T) {
	good, entries := threeEntries(t)
	old := Render(entries[:2])
	e4, e5 := entries[2], entries[2]
	e4.ID, e5.ID = 4, 5
	plusThree := Render(append(entries, e4, e5))
	accept := map[string][]byte{"one entry": good, "three entries": plusThree, "from empty": good}
	for name, next := range accept {
		before := old
		if name == "from empty" {
			before = nil
		}
		if err := VerifyAppendOnly(before, next); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	s1, e1 := section(good, "D-001")
	e4.ID = 5
	reject := map[string][]byte{
		"changed byte":      bytes.Replace(good, []byte("- **Kind:** technical"), []byte("- **Kind:** policy"), 1),
		"removed entry":     append(append([]byte{}, good[:s1]...), good[e1:]...),
		"reordered":         Render([]Entry{entries[1], entries[0], entries[2]}),
		"truncated":         good[:len(good)-20],
		"non-contiguous ID": append(append([]byte{}, old...), append([]byte{'\n'}, RenderEntry(e4)...)...),
		"unchanged":         old,
		"empty":             nil,
	}
	for name, next := range reject {
		if err := VerifyAppendOnly(old, next); !errors.Is(err, ErrNotAppendOnly) {
			t.Errorf("%s: got %v, want ErrNotAppendOnly", name, err)
		}
	}
	if err := VerifyAppendOnly(old[:len(old)-1], good); !errors.Is(err, ErrCorrupt) {
		t.Errorf("corrupt old: got %v", err)
	}
}
