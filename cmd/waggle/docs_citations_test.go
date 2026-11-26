// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// docCite is one path.go:N or path.go:N-M citation found in a doc.
type docCite struct {
	doc        string // repo-relative doc that holds the citation
	lineNo     int    // 1-based line of the doc
	path       string // cited Go file, repo-relative
	start, end int    // cited lines; end == start for a single line
	ident      string // backticked identifier right before the cite, or ""
}

var (
	// A citation is a repo-relative .go path, a colon and a line or range.
	// Further numbers after a comma, as in `a.go:181, 246`, cite the same file.
	citeRe = regexp.MustCompile("([A-Za-z0-9_./-]+\\.go):(\\d+(?:-\\d+)?(?:, \\d+(?:-\\d+)?)*)")
	// A backticked identifier directly before the cite, as in
	// "test `TestX`, `cmd/waggle/x_test.go:12`".
	identBeforeRe = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_]*)`[,;:]? ?\\(?$")
)

// citationDocs lists the docs the audit scans, repo-relative.
func citationDocs(t *testing.T, root string) []string {
	t.Helper()
	docs := []string{"README.md", "docs/gate.md", "docs/nectar.md", "docs/quorum.md"}
	site, err := filepath.Glob(filepath.Join(root, "site", "content", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(site)
	for _, p := range site {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			t.Fatal(err)
		}
		docs = append(docs, filepath.ToSlash(rel))
	}
	return docs
}

// parseDocCites extracts every citation from one doc's text.
func parseDocCites(doc, text string) []docCite {
	var out []docCite
	for i, line := range strings.Split(text, "\n") {
		for _, m := range citeRe.FindAllStringSubmatchIndex(line, -1) {
			path := line[m[2]:m[3]]
			ident := ""
			// The cite sits inside backticks, so drop the opening one first.
			if im := identBeforeRe.FindStringSubmatch(strings.TrimSuffix(line[:m[0]], "`")); im != nil {
				ident = im[1]
			}
			for _, part := range strings.Split(line[m[4]:m[5]], ", ") {
				lo, hi, _ := strings.Cut(part, "-")
				s, _ := strconv.Atoi(lo)
				e := s
				if hi != "" {
					e, _ = strconv.Atoi(hi)
				}
				out = append(out, docCite{doc: doc, lineNo: i + 1, path: path, start: s, end: e, ident: ident})
			}
		}
	}
	return out
}

// checkDocCite returns a failure reason for the cite, or "" when it holds.
// lines is the cited file split into lines, or nil when it is missing.
func checkDocCite(c docCite, lines []string) string {
	if lines == nil {
		return "file does not exist"
	}
	if c.start < 1 || c.end < c.start {
		return fmt.Sprintf("bad line range %d-%d", c.start, c.end)
	}
	if c.end > len(lines) {
		return fmt.Sprintf("line %d is past the end of the file (%d lines)", c.end, len(lines))
	}
	if c.ident == "" {
		return ""
	}
	// The identifier must appear on the cited line (or range) or within the
	// 20 lines before it.
	lo := c.start - 20
	if lo < 1 {
		lo = 1
	}
	for n := lo; n <= c.end; n++ {
		if strings.Contains(lines[n-1], c.ident) {
			return ""
		}
	}
	return fmt.Sprintf("identifier %q not found on lines %d-%d", c.ident, lo, c.end)
}

// TestDocsCiteCurrentLines fails when a doc cites a Go file line that moved:
// a missing file, a line past the end, or an identifier named next to the
// cite that no longer sits near it. All failures are listed at once.
func TestDocsCiteCurrentLines(t *testing.T) {
	root := driftRepoRoot(t)
	files := map[string][]string{}
	load := func(rel string) []string {
		if l, ok := files[rel]; ok {
			return l
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		var l []string
		if err == nil {
			l = strings.Split(string(data), "\n")
			if len(l) > 0 && l[len(l)-1] == "" {
				l = l[:len(l)-1]
			}
		}
		files[rel] = l
		return l
	}
	var failures []string
	total := 0
	for _, doc := range citationDocs(t, root) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(doc)))
		if err != nil {
			t.Errorf("%s: %v", doc, err)
			continue
		}
		for _, c := range parseDocCites(doc, string(data)) {
			total++
			if why := checkDocCite(c, load(c.path)); why != "" {
				cite := fmt.Sprintf("%s:%d", c.path, c.start)
				if c.end != c.start {
					cite = fmt.Sprintf("%s:%d-%d", c.path, c.start, c.end)
				}
				failures = append(failures, fmt.Sprintf("%s:%d: cite %s: %s", c.doc, c.lineNo, cite, why))
			}
		}
	}
	if total == 0 {
		t.Fatal("found no path.go:N citations in the docs; the scan is broken")
	}
	if len(failures) > 0 {
		t.Errorf("%d of %d doc citations are stale:\n%s", len(failures), total, strings.Join(failures, "\n"))
	}
}

// TestDocsCitationParser pins the citation syntax the audit understands.
func TestDocsCitationParser(t *testing.T) {
	text := "see `Foo`, `a/b.go:3-5, 9` and (`c.go:7`) plus `x.go:1`\nno cite here"
	got := parseDocCites("d.md", text)
	want := []docCite{
		{"d.md", 1, "a/b.go", 3, 5, "Foo"},
		{"d.md", 1, "a/b.go", 9, 9, "Foo"},
		{"d.md", 1, "c.go", 7, 7, ""},
		{"d.md", 1, "x.go", 1, 1, ""},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d cites, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cite %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	lines := []string{"package x", "func Foo() {}", "", "x"}
	if why := checkDocCite(docCite{path: "a.go", start: 4, end: 9}, lines); why == "" {
		t.Error("a range past the end should fail")
	}
	if why := checkDocCite(docCite{path: "a.go", start: 4, end: 4, ident: "Foo"}, lines); why != "" {
		t.Errorf("identifier within 20 lines should pass: %s", why)
	}
	if why := checkDocCite(docCite{path: "a.go", start: 4, end: 4, ident: "Bar"}, lines); why == "" {
		t.Error("a missing identifier should fail")
	}
	if why := checkDocCite(docCite{path: "a.go", start: 1, end: 1}, nil); why == "" {
		t.Error("a missing file should fail")
	}
}
