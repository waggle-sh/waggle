package usage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	ledgerPath = ".waggle/control/usage.jsonl"
	dashes     = "\u2013\u2014"
)

// Test 8: the golden covers two roles with several sessions, a floored guard
// record, a null cost and tickets out of order in the file.
func TestRenderGolden(t *testing.T) {
	recs, bad, err := Read("testdata/usage.jsonl")
	if err != nil || len(bad) != 0 {
		t.Fatalf("read: %v, bad %v", err, bad)
	}
	got := Render(Summarize(recs), ledgerPath)
	want, err := os.ReadFile("testdata/usage.golden")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if strings.ContainsAny(string(got), dashes) {
		t.Error("output contains an en or em dash")
	}
}

// Test 9: token count formatting.
func TestTokens(t *testing.T) {
	for n, want := range map[int64]string{
		0: "0", 999: "999", 1000: "1k", 52381: "52k", 999499: "999k", 999500: "1.00M",
		999950: "1.00M", 1840000: "1.84M", 2150000: "2.15M",
	} {
		if got := Tokens(n); got != want {
			t.Errorf("Tokens(%d) = %q, want %q", n, got, want)
		}
	}
}

// Test 10: the cached share is omitted when in is 0 and rounds to a whole percent.
func TestCachedShare(t *testing.T) {
	cost := 0.0
	out := string(Render(Summarize([]Record{
		{Ticket: "T-1", Role: "r", Session: "a", OutputTokens: 5, CostUSD: &cost},
		{Ticket: "T-2", Role: "r", Session: "a", InputTokens: 1, CacheReadTokens: 2, CostUSD: &cost},
		{Ticket: "T-3", Role: "r", Session: "a", InputTokens: 2, CacheReadTokens: 1, CostUSD: &cost},
	}), ledgerPath))
	for _, want := range []string{"T-001  r 1 sess  in 0  ", "in 3 (67% cached)", "in 3 (33% cached)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "in 0 (") {
		t.Errorf("percentage printed for in 0:\n%s", out)
	}
}

// Test 11: bad lines are reported with line numbers; the rest load.
func TestReadBadLines(t *testing.T) {
	recs, bad, err := Read("testdata/bad.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 || recs[0].Ticket != "T-001" || recs[1].Ticket != "T-002" {
		t.Errorf("records: %+v", recs)
	}
	if len(bad) != 2 || bad[0].Line != 2 || bad[1].Line != 4 || bad[0].Err == nil || bad[1].Err == nil {
		t.Errorf("bad lines: %+v", bad)
	}
}

// A line over 1 MiB is an I/O error, not a silent skip.
func TestReadLongLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.jsonl")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxLine+10)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Read(path); err == nil {
		t.Error("want an error for an oversized line")
	}
}

// Test 12: an absent file is an empty summary, no error.
func TestReadAbsent(t *testing.T) {
	recs, bad, err := Read(filepath.Join(t.TempDir(), "none.jsonl"))
	if recs != nil || bad != nil || err != nil {
		t.Fatalf("got %v, %v, %v", recs, bad, err)
	}
	want := "Usage: none recorded (" + ledgerPath + " absent)\n"
	if got := string(Render(Summarize(recs), ledgerPath)); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Test 13: Append then Read round-trips; a missing directory fails and creates nothing.
func TestAppendRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage.jsonl")
	c := 0.62
	r := Record{
		TS: time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC), Ticket: "T-016", Role: "r", Session: "s1",
		Adapter: "claude", Model: "m", Profile: "standard", Floored: true,
		InputTokens: 1, CacheWriteTokens: 2, CacheReadTokens: 3, OutputTokens: 4, CostUSD: &c, End: "done",
	}
	nilCost := r
	nilCost.CostUSD = nil
	for _, rec := range []Record{r, nilCost} {
		if err := Append(path, rec); err != nil {
			t.Fatal(err)
		}
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("stat: %v, %v", info, err)
	}
	recs, bad, err := Read(path)
	if err != nil || len(bad) != 0 || len(recs) != 2 {
		t.Fatalf("read: %v, %v, %v", recs, bad, err)
	}
	for i, want := range []Record{r, nilCost} {
		a, _ := json.Marshal(recs[i])
		b, _ := json.Marshal(want)
		if string(a) != string(b) {
			t.Errorf("record %d:\n got %s\nwant %s", i, a, b)
		}
	}
	data, _ := os.ReadFile(path)
	if strings.Count(string(data), "\n") != 2 || !strings.Contains(string(data), `"cost_usd":null`) {
		t.Errorf("ledger bytes: %q", data)
	}

	missing := filepath.Join(dir, "no-such-dir", "usage.jsonl")
	err = Append(missing, r)
	if err == nil || !errors.Is(err, os.ErrNotExist) || !strings.HasPrefix(err.Error(), "usage: ") {
		t.Errorf("want wrapped not-exist error, got %v", err)
	}
	if _, err := os.Stat(filepath.Dir(missing)); err == nil {
		t.Error("Append created the directory")
	}
}

// Sessions count once per role per distinct id; tickets sort numerically and
// canonicalize; roles keep first-appearance order.
func TestSummarize(t *testing.T) {
	s := Summarize([]Record{
		{Ticket: "T-10", Role: "b", Session: "1"},
		{Ticket: "T-9", Role: "a", Session: "1"},
		{Ticket: "T-009", Role: "a", Session: "1"},
		{Ticket: "T-009", Role: "a", Session: "2"},
		{Ticket: "T-9", Role: "b", Session: "1"},
	})
	if len(s) != 2 || s[0].Ticket != "T-009" || s[1].Ticket != "T-010" {
		t.Fatalf("summaries: %+v", s)
	}
	if got := s[0].Roles; len(got) != 2 || got[0] != (RoleSessions{"a", 2}) || got[1] != (RoleSessions{"b", 1}) {
		t.Errorf("roles: %+v", got)
	}
}
