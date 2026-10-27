// SPDX-License-Identifier: Apache-2.0

// Package license holds the SPDX header rules for this repository and the
// check that enforces them. It is a compliance check, not a security
// boundary: anyone who can commit can also edit the rules.
package license

import (
	"bytes"
	"fmt"
	"path"
	"strings"
)

// Form is the comment syntax a header is written in.
type Form int

const (
	// Slash is a Go line comment: "// ".
	Slash Form = iota
	// Hash is a shell or YAML comment: "# ".
	Hash
	// HTML is a Markdown comment: "<!-- ... -->".
	HTML
)

const tag = "SPDX-License-Identifier: "

type kind int

const (
	exempt kind = iota
	notChecked
	checked
)

type rule struct {
	patterns []string
	kind     kind
	id       string
}

// rules is the ordered table of the T-025 brief, section 4. The first match
// wins. Row 5 (CLAUDE.md) was folded into the exempt row by D-334.
var rules = []rule{
	{[]string{"**/testdata/**"}, exempt, ""},
	{[]string{"**/*.json", "internal/scaffold/templates/**/*.json.tmpl"}, exempt, ""},
	{[]string{"LICENSE", "NOTICE", "LICENSES/**", "go.mod", "go.sum", ".gitignore", "CLAUDE.md"}, exempt, ""},
	{[]string{".waggle/hive/**"}, notChecked, ""},
	{[]string{"internal/scaffold/templates/**"}, checked, "MIT-0"},
	{[]string{"cmd/**", "internal/**"}, checked, "Apache-2.0"},
	{[]string{"docs/**", "README.md", "CONTRIBUTING.md"}, checked, "Apache-2.0"},
	{[]string{".github/workflows/waggle-gate.yml"}, checked, "MIT-0"},
	{[]string{".github/**"}, checked, "Apache-2.0"},
	{[]string{"gate/**"}, checked, "Apache-2.0"},
	{[]string{".waggle/control/**"}, checked, "MIT-0"},
	{[]string{".claude/hooks/*.sh"}, checked, "MIT-0"},
}

// Want reports the identifier and comment form a slash-separated path must
// carry. ok is false for exempt and unchecked paths. It returns an error
// for a path no rule matches and for a checked path with an unknown extension.
func Want(p string) (id string, form Form, ok bool, err error) {
	segs := strings.Split(p, "/")
	for _, r := range rules {
		for _, pat := range r.patterns {
			if !matchSegs(strings.Split(pat, "/"), segs) {
				continue
			}
			if r.kind != checked {
				return "", 0, false, nil
			}
			f, err := formOf(p)
			if err != nil {
				return "", 0, false, err
			}
			return r.id, f, true, nil
		}
	}
	return "", 0, false, fmt.Errorf("unknown path: %s: no license rule matches; add a row in internal/license", p)
}

// formOf picks the comment syntax from the extension, ignoring a trailing
// ".tmpl" because a template is written out in its rendered file's syntax.
func formOf(p string) (Form, error) {
	base := path.Base(p)
	if base == "CODEOWNERS" {
		return Hash, nil
	}
	name := strings.TrimSuffix(base, ".tmpl")
	switch path.Ext(name) {
	case ".go":
		return Slash, nil
	case ".sh", ".yml", ".yaml":
		return Hash, nil
	case ".md":
		return HTML, nil
	}
	return 0, fmt.Errorf("unknown extension: %s: no comment form for this file kind", p)
}

// matchSegs matches pattern segments against path segments. "*" and the other
// path.Match metacharacters stay within one segment; "**" matches zero or more
// whole segments.
func matchSegs(pat, segs []string) bool {
	if len(pat) == 0 {
		return len(segs) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if matchSegs(pat[1:], segs[i:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	if ok, err := path.Match(pat[0], segs[0]); err != nil || !ok {
		return false
	}
	return matchSegs(pat[1:], segs[1:])
}

// Line returns the header text for an identifier in a form.
func Line(id string, form Form) string {
	switch form {
	case Hash:
		return "# " + tag + id
	case HTML:
		return "<!-- " + tag + id + " -->"
	}
	return "// " + tag + id
}

// Check verifies the header of one file. It reads only the header line:
// line 1, or line 2 when line 1 starts with "#!". Go files must also have a
// blank line after it.
func Check(p string, content []byte) error {
	id, form, ok, err := Want(p)
	if err != nil || !ok {
		return err
	}
	want := Line(id, form)
	lines := bytes.SplitN(content, []byte("\n"), 4)
	n := 0
	if len(lines) > 0 && bytes.HasPrefix(lines[0], []byte("#!")) {
		n = 1
	}
	if n >= len(lines) || len(content) == 0 {
		return fmt.Errorf("missing SPDX header: %s: want line %d %q", p, n+1, want)
	}
	got := string(lines[n])
	if strings.HasSuffix(got, "\r") {
		return fmt.Errorf("CRLF header line: %s: line %d must end in LF", p, n+1)
	}
	if got != want {
		if strings.Contains(got, tag) {
			return fmt.Errorf("wrong SPDX header: %s: got %q, want %q", p, got, want)
		}
		return fmt.Errorf("missing SPDX header: %s: want line %d %q", p, n+1, want)
	}
	if form == Slash && n+1 < len(lines) && len(lines[n+1]) != 0 {
		return fmt.Errorf("missing blank line after SPDX header: %s: line %d must be empty", p, n+2)
	}
	return nil
}
