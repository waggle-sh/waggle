// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"strings"
	"testing"
)

// B4: the queen template other hives get carries the force-release delete-card rule.
func TestQueenTemplateForceReleaseCardRule(t *testing.T) {
	data, err := templateFS.ReadFile("templates/roles/queen.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"`waggle force-release`", "`delete` card", "Never run its execute step", "from their own shell"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("queen template lacks %q", want)
		}
	}
}
