package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestGateApprovers(t *testing.T) {
	many := strings.TrimSuffix(strings.Repeat("a,", 32), ",")
	ok := []struct {
		val  string
		want []string
	}{
		{`"alice, Bob-2"`, []string{"alice", "Bob-2"}},
		{`""`, nil},
		{`"a,a"`, []string{"a", "a"}},
		{`"` + many + `"`, strings.Split(many, ",")},
	}
	for _, tc := range ok {
		c := mustParse(t, "version: 1\ngate:\n  approvers: "+tc.val+"\n")
		if !reflect.DeepEqual(c.Gate.Approvers, tc.want) {
			t.Errorf("%s: got %q, want %q", tc.val, c.Gate.Approvers, tc.want)
		}
	}
	bad := map[string]string{
		"at sign":        `"@alice"`,
		"team":           `"org/team"`,
		"leading hyphen": `"-x"`,
		"too long":       `"` + strings.Repeat("a", 40) + `"`,
		"control byte":   `"a\u0001b"`,
		"empty entry":    `"a,,b"`,
		"trailing comma": `"a,"`,
		"too many":       `"` + strings.Repeat("a,", 32) + `a"`,
		"space inside":   `"a b"`,
	}
	for name, val := range bad {
		_, err := Parse([]byte("version: 1\ngate:\n  approvers: " + val + "\n"))
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("%s: err = %v, want ErrInvalidConfig", name, err)
		}
	}
	if _, err := Parse([]byte("version: 1\ngate:\n  approvers:\n    nested: x\n")); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("nested mapping: err = %v, want ErrInvalidConfig", err)
	}
	if c := mustParse(t, "version: 1\n"); c.Gate.Approvers != nil {
		t.Errorf("no gate section: got %q", c.Gate.Approvers)
	}
}
