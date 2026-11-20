// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNectarConfig(t *testing.T) {
	parse := func(block string) (*Config, error) {
		return Parse([]byte("version: 1\n" + block))
	}
	t.Run("absent block gives defaults", func(t *testing.T) {
		c, err := parse("")
		if err != nil {
			t.Fatal(err)
		}
		want := NectarConfig{DailyCapMilli: 12000, SelfMergeFactorMilli: 500, ReciprocityDiscountMilli: 500}
		if !reflect.DeepEqual(c.Nectar, want) {
			t.Fatalf("got %+v, want %+v", c.Nectar, want)
		}
	})
	t.Run("values", func(t *testing.T) {
		c, err := parse("nectar:\n  exclude_keepers: bot-a, bot-b\n  min_changed_lines: 5\n  daily_cap: 7.25\n  self_merge_factor: 1\n  reciprocity_discount: 0.001\n")
		if err != nil {
			t.Fatal(err)
		}
		want := NectarConfig{ExcludeKeepers: []string{"bot-a", "bot-b"}, MinChangedLines: 5, DailyCapMilli: 7250, SelfMergeFactorMilli: 1000, ReciprocityDiscountMilli: 1}
		if !reflect.DeepEqual(c.Nectar, want) {
			t.Fatalf("got %+v, want %+v", c.Nectar, want)
		}
	})
	t.Run("unknown key names its line", func(t *testing.T) {
		_, err := parse("nectar:\n  daily_cap: 3\n  allow_self_merge: true\n")
		if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "line 4") || !strings.Contains(err.Error(), "nectar.allow_self_merge") {
			t.Fatalf("got %v", err)
		}
	})
	for _, bad := range []string{
		"self_merge_factor: 0", "reciprocity_discount: 0", "daily_cap: 0", "daily_cap: 0.000",
		"self_merge_factor: 1.5", "reciprocity_discount: -1", "daily_cap: 1.2345", "daily_cap: abc",
		"min_changed_lines: -1", "min_changed_lines: 1.5", "exclude_keepers: -bad",
	} {
		t.Run("rejects "+bad, func(t *testing.T) {
			if _, err := parse("nectar:\n  " + bad + "\n"); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("got %v", err)
			}
		})
	}
	t.Run("not a mapping", func(t *testing.T) {
		if _, err := parse("nectar: 1\n"); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("got %v", err)
		}
	})
}
