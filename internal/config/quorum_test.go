// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"strings"
	"testing"
)

// D1: the quorum block.
func TestQuorumBlock(t *testing.T) {
	t.Run("absent gives defaults", func(t *testing.T) {
		c := mustParse(t, "version: 1\n")
		if c.Quorum.Credits != 100 || c.Quorum.RoundDays != 14 {
			t.Fatalf("quorum = %+v", c.Quorum)
		}
	})
	t.Run("both keys parse", func(t *testing.T) {
		c := mustParse(t, "version: 1\nquorum:\n  credits: 40\n  round_days: 7\n")
		if c.Quorum.Credits != 40 || c.Quorum.RoundDays != 7 {
			t.Fatalf("quorum = %+v", c.Quorum)
		}
	})
	t.Run("one key keeps the other default", func(t *testing.T) {
		c := mustParse(t, "version: 1\nquorum:\n  round_days: 3\n")
		if c.Quorum.Credits != 100 || c.Quorum.RoundDays != 3 {
			t.Fatalf("quorum = %+v", c.Quorum)
		}
	})
	bad := []struct{ name, src, want string }{
		{"unknown key", "version: 1\nquorum:\n  credits: 5\n  bonus: 1\n", "line 4"},
		{"zero credits", "version: 1\nquorum:\n  credits: 0\n", "quorum.credits"},
		{"negative days", "version: 1\nquorum:\n  round_days: -2\n", "quorum.round_days"},
		{"not an integer", "version: 1\nquorum:\n  credits: many\n", "quorum.credits"},
		{"not a mapping", "version: 1\nquorum: yes\n", "quorum"},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.src))
			if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want a config error naming %q", err, c.want)
			}
		})
	}
}
