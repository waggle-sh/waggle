// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// parserRejections lists every construct section 4 rejects, with the line
// the error must name. Shared with FuzzParse as seed input.
var parserRejections = []struct {
	name, src string
	line      int
}{
	{"BOM", "\xEF\xBB\xBFversion: 1\n", 1},
	{"invalid UTF-8", "version: 1\nforge:\n  remote: \xff\n", 0},
	{"block sequence", "version: 1\nlist:\n  - item\n", 3},
	{"lone dash", "version: 1\n-\n", 2},
	{"sequence value", "version: 1\nkey: - item\n", 2},
	{"flow sequence", "version: 1\nkey: [a, b]\n", 2},
	{"flow mapping", "version: 1\nkey: {}\n", 2},
	{"anchor", "version: 1\nkey: &anchor value\n", 2},
	{"alias", "version: 1\nkey: *anchor\n", 2},
	{"tag", "version: 1\nkey: !!str value\n", 2},
	{"literal block scalar", "version: 1\nkey: |\n  text\n", 2},
	{"folded block scalar", "version: 1\nkey: >\n  text\n", 2},
	{"document start", "---\nversion: 1\n", 1},
	{"document end", "version: 1\n...\n", 2},
	{"merge key", "version: 1\n<<: *base\n", 2},
	{"duplicate key", "version: 1\nversion: 1\n", 2},
	{"key without value at end", "version: 1\nempty:\n", 2},
	{"key without value before dedent", "version: 1\nforge:\n  empty:\n  comb_mode: direct\n", 3},
	{"key without value before sibling", "version: 1\nempty:\nforge:\n  comb_mode: direct\n", 2},
	{"tab indentation", "version: 1\nforge:\n\tcomb_mode: direct\n", 3},
	{"mixed tab indentation", "version: 1\nforge:\n \tcomb_mode: direct\n", 3},
	{"first line indented", "  version: 1\n", 1},
	{"indent without parent", "version: 1\n  forge: x\n", 2},
	{"dedent to unknown level", "version: 1\na:\n    b:\n      c: 1\n  d: 2\n", 5},
	{"child under scalar", "version: 1\nforge: github\n  comb_mode: direct\n", 3},
	{"invalid key character", "version: 1\nfor ge: x\n", 2},
	{"key without colon", "version: 1\nforge\n", 2},
	{"no space after colon", "version: 1\nforge:github\n", 2},
	{"empty key", "version: 1\n: value\n", 2},
	{"unterminated double quote", "version: 1\nkey: \"abc\n", 2},
	{"unterminated single quote", "version: 1\nkey: 'abc\n", 2},
	{"bad escape", "version: 1\nkey: \"a\\x41\"\n", 2},
	{"escape at end", "version: 1\nkey: \"abc\\\n", 2},
	{"text after double quote", "version: 1\nkey: \"abc\"def\n", 2},
	{"text after single quote", "version: 1\nkey: 'abc'def\n", 2},
	{"nested mapping on value line", "version: 1\nkey: a: b\n", 2},
	{"trailing colon on value", "version: 1\nkey: value:\n", 2},
	{"stray carriage return", "version: 1\r\rforge:\n", 1},
	{"control character", "version: 1\nkey: a\x01b\n", 2},
}

func TestParserRejections(t *testing.T) {
	for _, tc := range parserRejections {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.src))
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("err = %v, want ErrInvalidConfig", err)
			}
			if tc.line > 0 && !strings.Contains(err.Error(), fmt.Sprintf("line %d:", tc.line)) {
				t.Fatalf("error %q does not name line %d", err, tc.line)
			}
		})
	}
}

func TestParserAccepts(t *testing.T) {
	src := strings.Join([]string{
		"# leading comment",
		"",
		"version: 1 # trailing comment",
		"a:",
		"  b:   spaced   ",
		"  c: \"quoted # not a comment\\n\\t\\\\\\\"\"",
		"  d: 'it''s'",
		"  e: value#notacomment",
		"  f: https://example.com/x",
		"  g: -5",
		"  h: true",
		"  i:  # comment after an empty value",
		"    j: 1",
		"  k: 2",
		"\t# tab-indented comment",
		"l: ''",
		"",
	}, "\r\n")
	tree, err := parseTree([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	a := tree["a"]
	if a == nil || !a.isMapping() || a.line != 4 {
		t.Fatalf("a = %+v", a)
	}
	want := map[string]node{
		"b": {line: 5, scalar: "spaced"},
		"c": {line: 6, scalar: "quoted # not a comment\n\t\\\"", quoted: true},
		"d": {line: 7, scalar: "it's", quoted: true},
		"e": {line: 8, scalar: "value#notacomment"},
		"f": {line: 9, scalar: "https://example.com/x"},
		"g": {line: 10, scalar: "-5"},
		"h": {line: 11, scalar: "true"},
		"k": {line: 14, scalar: "2"},
	}
	for k, w := range want {
		got := a.children[k]
		if got == nil || got.children != nil || got.line != w.line || got.scalar != w.scalar || got.quoted != w.quoted {
			t.Errorf("a.%s = %+v, want %+v", k, got, w)
		}
	}
	if i := a.children["i"]; i == nil || i.line != 12 || i.children["j"] == nil || i.children["j"].scalar != "1" {
		t.Errorf("a.i = %+v", i)
	}
	if l := tree["l"]; l == nil || l.line != 16 || !l.quoted || l.scalar != "" {
		t.Errorf("l = %+v", l)
	}
	if len(tree) != 3 || len(a.children) != 9 {
		t.Errorf("tree has %d top-level and %d a.* keys", len(tree), len(a.children))
	}

	if _, err := parseTree(nil); err != nil {
		t.Fatalf("empty input: %v", err)
	}
	if _, err := parseTree([]byte("# only a comment")); err != nil {
		t.Fatalf("comment only, no newline: %v", err)
	}
}

func FuzzParse(f *testing.F) {
	if raw, err := os.ReadFile(templatePath); err == nil {
		r := strings.NewReplacer("{{PIN}}", "unpinned", "{{PROFILE}}", "cli", "{{DELIVERY}}", "installed",
			"{{FORGE}}", "github", "{{COMB_MODE}}", "direct", "{{RUNTIME}}", "claude-code")
		f.Add([]byte(r.Replace(string(raw))))
	}
	if raw, err := os.ReadFile("../../.waggle/control/config.yaml"); err == nil {
		f.Add(raw)
	}
	f.Add([]byte("version: 1\n"))
	f.Add([]byte("version: 1\r\nforge:\r\n  remote: \"origin\" # c\r\n"))
	for _, tc := range rejections {
		f.Add([]byte(tc.src))
	}
	for _, tc := range parserRejections {
		f.Add([]byte(tc.src))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		c1, err1 := Parse(data)
		c2, err2 := Parse(data)
		switch {
		case err1 == nil && err2 == nil:
			if !reflect.DeepEqual(c1, c2) {
				t.Fatalf("nondeterministic result: %+v vs %+v", c1, c2)
			}
			if err := validate(c1, nil); err != nil {
				t.Fatalf("Parse returned a config that fails validate: %v", err)
			}
		case err1 != nil && err2 != nil:
			if err1.Error() != err2.Error() {
				t.Fatalf("nondeterministic error: %q vs %q", err1, err2)
			}
			if !errors.Is(err1, ErrInvalidConfig) {
				t.Fatalf("error does not wrap ErrInvalidConfig: %v", err1)
			}
		default:
			t.Fatalf("nondeterministic outcome: %v vs %v", err1, err2)
		}
	})
}
