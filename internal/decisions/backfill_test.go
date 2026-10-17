package decisions

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// appendixA is the hive brief that holds the backfill source of truth. It is
// hive-layer state, so the test skips when a checkout does not carry it.
var appendixA = filepath.Join("..", "..", ".waggle", "hive", "tasks", "T-006-backfill.md")

var (
	itemHead  = regexp.MustCompile("^\\*\\*(D-[0-9]{3})\\*\\* \\| kind `([a-z]+)` \\| tickets (.*)$")
	itemField = regexp.MustCompile(`^- (Title|Question|Options|Recommendation|Choice|Rationale|Supersedes): (.*)$`)
)

// parseAppendixA reads Appendix A into one Input per item, in order, using the
// mapping rules stated at the top of the appendix.
func parseAppendixA(t *testing.T, text string) ([]string, []Input) {
	t.Helper()
	_, body, ok := strings.Cut(text, "## Appendix A")
	if !ok {
		t.Fatal("no Appendix A heading")
	}
	body, _, _ = strings.Cut(body, "## Appendix B")
	var ids []string
	var ins []Input
	for _, line := range strings.Split(body, "\n") {
		if m := itemHead.FindStringSubmatch(line); m != nil {
			in := Input{Kind: m[2], Tickets: []string{}, Supersedes: []string{}}
			if m[3] != "none" {
				in.Tickets = splitList(strings.Trim(m[3], "`"), ",")
			}
			ids, ins = append(ids, m[1]), append(ins, in)
			continue
		}
		m := itemField.FindStringSubmatch(line)
		if m == nil || len(ins) == 0 {
			continue
		}
		in := &ins[len(ins)-1]
		switch v := m[2]; m[1] {
		case "Title":
			in.Title = v
		case "Question":
			in.Question = v
		case "Options":
			in.Options = strings.Split(v, " || ")
		case "Recommendation":
			in.Recommendation = v
		case "Choice":
			in.Choice = v
		case "Rationale":
			in.Rationale = v
		case "Supersedes":
			in.Supersedes = splitList(v, ",")
		}
	}
	return ids, ins
}

func splitList(s, sep string) []string {
	parts := strings.Split(s, sep)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// TestBackfillMatchesAppendixA checks that every testdata/backfill input
// exists, decodes, validates against the entry rules, and equals its
// Appendix A item field for field.
func TestBackfillMatchesAppendixA(t *testing.T) {
	text, err := os.ReadFile(appendixA)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("%s not present", appendixA)
	}
	if err != nil {
		t.Fatal(err)
	}
	ids, want := parseAppendixA(t, string(text))
	files, err := filepath.Glob(filepath.Join("testdata", "backfill", "D-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 || len(files) != len(ids) {
		t.Fatalf("Appendix A has %d items, testdata/backfill has %d files", len(ids), len(files))
	}
	for i, id := range ids {
		if wantID := fmt.Sprintf("D-%03d", i+1); id != wantID {
			t.Fatalf("Appendix A item %d is %s, want %s", i+1, id, wantID)
		}
		f, err := os.Open(filepath.Join("testdata", "backfill", id+".json"))
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeInput(f)
		_ = f.Close()
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		if _, err := got.entry(i); err != nil {
			t.Errorf("%s: %v", id, err)
		}
		if !reflect.DeepEqual(got, want[i]) {
			t.Errorf("%s differs from Appendix A:\n got %+v\nwant %+v", id, got, want[i])
		}
	}
}
