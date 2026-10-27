// SPDX-License-Identifier: Apache-2.0

package decisions

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var clock = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

func valid() Input {
	return Input{
		Kind:           "technical",
		Title:          "Config parser is a hand-written YAML subset",
		Question:       "Which parser reads .waggle/config.yaml?",
		Options:        []string{"A: add a YAML dependency", "B: hand-written subset parser"},
		Recommendation: "B",
		Choice:         "B: hand-written YAML subset parser, stdlib only, fuzz tested",
		Rationale:      "Config is machine-written and tiny.",
		Tickets:        []string{"T-004"},
		Supersedes:     []string{},
	}
}

func rep(n int) string { return strings.Repeat("x", n) }

func TestValidationTable(t *testing.T) {
	cases := []struct {
		name, field string
		mut         func(*Input)
	}{
		{"unknown kind", "kind", func(in *Input) { in.Kind = "wish" }},
		{"kind case", "kind", func(in *Input) { in.Kind = "Technical" }},
		{"empty title", "title", func(in *Input) { in.Title = "" }},
		{"201-byte title", "title", func(in *Input) { in.Title = rep(201) }},
		{"1001-byte question", "question", func(in *Input) { in.Question = rep(1001) }},
		{"501-byte option", "options[0]", func(in *Input) { in.Options[0] = rep(501) }},
		{"1001-byte recommendation", "recommendation", func(in *Input) { in.Recommendation = rep(1001) }},
		{"1001-byte choice", "choice", func(in *Input) { in.Choice = rep(1001) }},
		{"2001-byte rationale", "rationale", func(in *Input) { in.Rationale = rep(2001) }},
		{"newline", "question", func(in *Input) { in.Question = "a\nb" }},
		{"carriage return", "choice", func(in *Input) { in.Choice = "a\rb" }},
		{"tab", "rationale", func(in *Input) { in.Rationale = "a\tb" }},
		{"NUL", "title", func(in *Input) { in.Title = "a\x00b" }},
		{"leading space", "title", func(in *Input) { in.Title = " a" }},
		{"trailing space", "recommendation", func(in *Input) { in.Recommendation = "a " }},
		{"invalid UTF-8", "title", func(in *Input) { in.Title = "a\xffb" }},
		{"empty option", "options[1]", func(in *Input) { in.Options[1] = "" }},
		{"0 options", "options", func(in *Input) { in.Options = nil }},
		{"11 options", "options", func(in *Input) { in.Options = strings.Split(rep(11), "") }},
		{"21 tickets", "tickets", func(in *Input) { in.Tickets = strings.Split(strings.TrimSuffix(strings.Repeat("1,", 21), ","), ",") }},
		{"ticket t-1", "tickets", func(in *Input) { in.Tickets = []string{"t-1"} }},
		{"ticket T-0", "tickets", func(in *Input) { in.Tickets = []string{"T-0"} }},
		{"ticket 0", "tickets", func(in *Input) { in.Tickets = []string{"0"} }},
		{"ticket T-1234567", "tickets", func(in *Input) { in.Tickets = []string{"T-1234567"} }},
		{"ticket T-", "tickets", func(in *Input) { in.Tickets = []string{"T-"} }},
		{"ticket 1.5", "tickets", func(in *Input) { in.Tickets = []string{"1.5"} }},
		{"11 supersedes", "supersedes", func(in *Input) { in.Supersedes = strings.Split(strings.Repeat("D-001,", 11), ",") }},
		{"supersedes D-1", "supersedes", func(in *Input) { in.Supersedes = []string{"D-1"} }},
		{"supersedes D-0001", "supersedes", func(in *Input) { in.Supersedes = []string{"D-0001"} }},
		{"supersedes D-000", "supersedes", func(in *Input) { in.Supersedes = []string{"D-000"} }},
		{"supersedes 1", "supersedes", func(in *Input) { in.Supersedes = []string{"1"} }},
		{"supersedes missing", "supersedes", func(in *Input) { in.Supersedes = []string{"D-002"} }},
	}
	path := filepath.Join(t.TempDir(), "DECISIONS.md")
	if _, err := Append(path, valid(), clock); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		in := valid()
		c.mut(&in)
		_, err := Next(path, in, clock)
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: got %v, want ErrInvalidInput", c.name, err)
			continue
		}
		if !strings.Contains(err.Error(), c.field) {
			t.Errorf("%s: message %q does not name field %q", c.name, err, c.field)
		}
	}
}

func TestDecodeInput(t *testing.T) {
	good := `{"kind":"scope","title":"t","question":"q","options":["o"],"recommendation":"r","choice":"c","rationale":"why","tickets":["3","T-002","T-3"],"supersedes":[]}`
	in, err := DecodeInput(strings.NewReader(good + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if in.Kind != "scope" || len(in.Tickets) != 3 {
		t.Errorf("decoded %+v", in)
	}
	bad := map[string]string{
		"unknown field": `{"kind":"scope","title":"t","extra":1}`,
		"two objects":   good + good,
		"trailing text": good + " x",
		"array":         "[" + good + "]",
		"over 64 KiB":   `{"kind":"scope","title":"` + rep(MaxInputBytes) + `"}`,
		"empty":         "",
	}
	for name, src := range bad {
		if _, err := DecodeInput(strings.NewReader(src)); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: got %v, want ErrInvalidInput", name, err)
		}
	}
	if _, err := DecodeInput(strings.NewReader(`{"kind":"scope","title":"t","extra":1}`)); err == nil || !strings.Contains(err.Error(), "extra") {
		t.Errorf("unknown field error should name it: %v", err)
	}
}

func TestTicketCanonicalization(t *testing.T) {
	in := valid()
	in.Tickets = []string{"3", "T-002", "T-3"}
	e, err := Next(filepath.Join(t.TempDir(), "DECISIONS.md"), in, clock)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(RenderEntry(e)); !strings.Contains(got, "- **Tickets:** T-002, T-003\n") {
		t.Errorf("rendered:\n%s", got)
	}
	if e.ID != 1 || e.Date != "2026-10-17" {
		t.Errorf("ID %d date %s", e.ID, e.Date)
	}
}

func TestSupersedesCanonicalization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "DECISIONS.md")
	for i := 0; i < 3; i++ {
		if _, err := Append(path, valid(), clock); err != nil {
			t.Fatal(err)
		}
	}
	in := valid()
	in.Supersedes = []string{"D-003", "D-001", "D-003"}
	e, err := Next(path, in, clock)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(RenderEntry(e)); !strings.Contains(got, "- **Supersedes:** D-001, D-003\n") {
		t.Errorf("rendered:\n%s", got)
	}
}
