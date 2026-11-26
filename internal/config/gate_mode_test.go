// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"strings"
	"testing"
)

func TestGateModeDefault(t *testing.T) {
	for name, in := range map[string]string{
		"no gate":        "version: 1\n",
		"gate no mode":   "version: 1\ngate:\n  approvers: \"alice\"\n",
		"gate other key": "version: 1\ngate:\n  protected_paths: \"deploy/\"\n",
	} {
		if m := mustParse(t, in).Gate.Mode; m != GateModeGuard {
			t.Errorf("%s: mode %q", name, m)
		}
	}
}

func TestGateModeValues(t *testing.T) {
	for _, tc := range []struct{ val, want string }{
		{"open", "open"}, {"guard", "guard"}, {`"open"`, "open"}, {`'guard'`, "guard"},
		{"open # note", "open"}, {`"guard" # note`, "guard"},
		{"hold", "hold"}, {`"hold"`, "hold"}, {`'hold'`, "hold"}, {"hold # note", "hold"},
	} {
		if m := mustParse(t, "version: 1\ngate:\n  mode: "+tc.val+"\n").Gate.Mode; m != tc.want {
			t.Errorf("%s: got %q", tc.val, m)
		}
	}
}

func TestGateModeInvalid(t *testing.T) {
	for _, val := range []string{"watch", "Open", "OPEN", "Hold", "HOLD", `""`, "all", `"open,guard"`, "on", "true", "\n    x: y"} {
		_, err := Parse([]byte("version: 1\ngate:\n  mode: " + val + "\n"))
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("%q: err %v", val, err)
			continue
		}
		if msg := err.Error(); !strings.Contains(msg, "gate.mode") || !strings.Contains(msg, "3") {
			t.Errorf("%q: message %q lacks the key or line", val, msg)
		}
	}
}

func TestGateModeWithOtherKeys(t *testing.T) {
	c := mustParse(t, "version: 1\ngate:\n  protected_paths: \"deploy/\"\n  approvers: \"alice\"\n  mode: open\n")
	if c.Gate.Mode != GateModeOpen || len(c.Gate.ProtectedPaths) != 1 || len(c.Gate.Approvers) != 1 {
		t.Errorf("got %+v", c.Gate)
	}
}
