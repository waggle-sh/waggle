// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
)

// The allow rules are built from BashCommands, each command once as Bash(cmd) and Bash(cmd *).
func TestBashCommandsMatchSettings(t *testing.T) {
	hive := []string{"go build ./...", "go test -race ./..."}
	for _, role := range []string{"builder", "guard"} {
		cmds, err := BashCommands(role, hive)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := workerSettings(role, hive)
		if err != nil {
			t.Fatal(err)
		}
		var s struct {
			Permissions struct{ Allow []string }
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			t.Fatal(err)
		}
		var want []string
		for _, c := range cmds {
			want = append(want, "Bash("+c+")", "Bash("+c+" *)")
		}
		if got := s.Permissions.Allow[5:]; !slices.Equal(got, want) {
			t.Errorf("%s: allow rules %q, want %q", role, got, want)
		}
		if role == "guard" && (slices.Contains(cmds, "git add") || slices.Contains(cmds, "git commit")) {
			t.Errorf("guard commands carry a git write: %q", cmds)
		}
		if role == "builder" && !slices.Contains(cmds, "git commit") {
			t.Errorf("builder commands lack git commit: %q", cmds)
		}
	}
}

func TestBashCommandsRefusals(t *testing.T) {
	for name, cmd := range map[string]string{
		"wildcard":        "go test *",
		"quote":           `go test "x"`,
		"shell operator":  "go build && ls",
		"control char":    "go build\x01",
		"denied program":  "curl http://x",
		"git push":        "git push",
		"extra space":     "go  build",
		"empty":           "",
		"rule syntax":     "go build (x)",
		"denied gh":       "gh pr list",
		"git push origin": "git push origin",
	} {
		for _, role := range []string{"builder", "guard"} {
			if _, err := BashCommands(role, []string{cmd}); !errors.Is(err, ErrAdapterConfig) {
				t.Errorf("%s/%s: BashCommands error %v", name, role, err)
			}
			if _, err := workerSettings(role, []string{cmd}); !errors.Is(err, ErrAdapterConfig) {
				t.Errorf("%s/%s: workerSettings error %v", name, role, err)
			}
		}
	}
}
