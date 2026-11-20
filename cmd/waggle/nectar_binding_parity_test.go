// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/nectar"
)

// TestBindingParity runs the gate's classify and the internal/nectar copy of the
// task binding over one table (T-051 B1): both must name the table's task for
// every row, so neither binds more than the other.
func TestBindingParity(t *testing.T) {
	data, err := os.ReadFile("../../internal/nectar/testdata/binding.tsv")
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 5 {
			t.Fatalf("bad row %q", line)
		}
		rows++
		t.Run(f[0], func(t *testing.T) {
			if got := gateBind(f[1], f[2], f[3]); got != f[4] {
				t.Errorf("gate binds %q, want %q", got, f[4])
			}
			if got := nectar.Bind(f[1], f[2], f[3]); got != f[4] {
				t.Errorf("nectar binds %q, want %q", got, f[4])
			}
		})
	}
	if rows < 20 {
		t.Fatalf("only %d rows", rows)
	}
	// A branch name the gate ignores (control byte, too long) binds the same in both.
	for _, b := range []string{"t-007\x01", "t-007\x7f", "t-007-" + strings.Repeat("x", 300), "t-007-ok"} {
		if got, want := nectar.BranchTask(b), gateBranchTask(b); got != want {
			t.Errorf("BranchTask(%q) = %q, gate %q", b, got, want)
		}
	}
}

// gateBind runs the gate's own classify on one commit and reads the ticket it
// bound. Every claim the table can name is pre-seeded, so classify never reads a
// repository.
func gateBind(subject, trailer, branch string) string {
	s := &commitScan{rep: &gateReport{}, bound: map[string]string{}, claims: map[string]claimLookup{}}
	s.branch = gateBranchTask(branch)
	for _, text := range []string{subject, trailer, s.branch} {
		for _, id := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' || r == ':' }) {
			s.claims[id] = claimLookup{}
		}
	}
	const oid = "0123456789012345678901234567890123456789"
	if _, err := s.classify(gateCommit{oid: oid, subject: subject, trailer: trailer}); err != nil {
		return "error: " + err.Error()
	}
	return s.bound[oid]
}

func gateBranchTask(name string) string {
	id, _ := branchTicket(name)
	return id
}
