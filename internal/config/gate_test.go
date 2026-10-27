// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestGateProtectedPaths(t *testing.T) {
	ok := []struct {
		val  string
		want []string
	}{
		{`"deploy/,infra/terraform/,package-lock.json"`, []string{"deploy/", "infra/terraform/", "package-lock.json"}},
		{`"deploy/ , Infra"`, []string{"deploy/", "Infra"}},
		{`"deploy,deploy"`, []string{"deploy", "deploy"}},
		{`""`, nil},
		{strings.TrimSuffix(strings.Repeat("a,", 64), ","), strings.Split(strings.TrimSuffix(strings.Repeat("a,", 64), ","), ",")},
	}
	for _, tc := range ok {
		c := mustParse(t, "version: 1\ngate:\n  protected_paths: "+tc.val+"\n")
		if !reflect.DeepEqual(c.Gate.ProtectedPaths, tc.want) {
			t.Errorf("%s: got %q, want %q", tc.val, c.Gate.ProtectedPaths, tc.want)
		}
	}
	bad := map[string]string{
		"empty entry":      `"a,,b"`,
		"trailing comma":   `"a,"`,
		"dot":              `"."`,
		"dot slash":        `"./"`,
		"leading slash":    `"/etc"`,
		"backslash":        `"a\\b"`,
		"dotdot":           `"a/../b"`,
		"star":             `"a/*"`,
		"question":         `"a?"`,
		"bracket":          `"a[1]"`,
		"control byte":     `"a\u0001b"`,
		"unclean":          `"a//b"`,
		"unclean dot":      `"a/./b"`,
		"too long":         `"` + strings.Repeat("a", 257) + `"`,
		"too many entries": `"` + strings.Repeat("a,", 64) + `a"`,
	}
	for name, val := range bad {
		_, err := Parse([]byte("version: 1\ngate:\n  protected_paths: " + val + "\n"))
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("%s: err = %v, want ErrInvalidConfig", name, err)
		}
	}
	for _, src := range []string{
		"version: 1\ngate:\n  owners: x\n",
		"version: 1\ngate: x\n",
		"version: 1\ngate:\n  protected_paths:\n    nested: x\n",
	} {
		if _, err := Parse([]byte(src)); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("%q: err = %v, want ErrInvalidConfig", src, err)
		}
	}
	if c := mustParse(t, "version: 1\n"); c.Gate.ProtectedPaths != nil {
		t.Errorf("no gate section: got %q", c.Gate.ProtectedPaths)
	}
}
