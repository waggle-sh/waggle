// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"os"
	"strings"
	"testing"
)

// TestBindTable runs Bind over testdata/binding.tsv, the table the gate's parity
// test in cmd/waggle reads as well. Columns: name, subject, trailer, branch, want.
func TestBindTable(t *testing.T) {
	data, err := os.ReadFile("testdata/binding.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 5 {
			t.Fatalf("bad row %q", line)
		}
		t.Run(f[0], func(t *testing.T) {
			if got := Bind(f[1], f[2], f[3]); got != f[4] {
				t.Fatalf("Bind = %q, want %q", got, f[4])
			}
		})
	}
	for _, name := range []string{"t-007\x01", "t-007\x7f", "t-007-" + strings.Repeat("x", 300)} {
		if BranchTask(name) != "" {
			t.Errorf("BranchTask(%q) bound", name)
		}
	}
}
