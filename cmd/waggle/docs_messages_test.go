// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// troubleshootingHeading matches an entry heading of docs/troubleshooting.md:
// a level 3 heading that is one inline code span, which holds the quoted message.
var troubleshootingHeading = regexp.MustCompile("^### `([^`]+)`$")

// placeholder matches a <name> that stands for a value in a quoted message.
var placeholder = regexp.MustCompile(`<[^<>]*>`)

// staticParts splits a quoted message at its placeholders and returns the
// fixed text between them, trimmed of the punctuation the code prints around a value.
func staticParts(msg string) []string {
	var out []string
	for _, p := range placeholder.Split(msg, -1) {
		if p = strings.Trim(p, " :\"'"); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// goSources reads every non-test Go file under cmd/ and internal/ of the repo.
func goSources(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return err
			}
			data, err := os.ReadFile(p)
			b.Write(data)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return b.String()
}

func TestStaticParts(t *testing.T) {
	got := staticParts("<id>: claim names no branch; renew it")
	if len(got) != 1 || got[0] != "claim names no branch; renew it" {
		t.Fatalf("staticParts = %q", got)
	}
	if got := staticParts("No rule on <branch> requires the <check> check."); len(got) != 3 || got[1] != "requires the" {
		t.Fatalf("staticParts = %q", got)
	}
}

// TestTroubleshootingMessagesExist fails when a message quoted in
// docs/troubleshooting.md is not in the Go source, so a reworded message
// cannot leave the page stale.
func TestTroubleshootingMessagesExist(t *testing.T) {
	root := filepath.Join("..", "..")
	page, err := os.ReadFile(filepath.Join(root, "docs", "troubleshooting.md"))
	if err != nil {
		t.Fatal(err)
	}
	src := goSources(t, root)
	n := 0
	for _, line := range strings.Split(string(page), "\n") {
		m := troubleshootingHeading.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n++
		parts := staticParts(m[1])
		if len(parts) == 0 {
			t.Errorf("message %q has no fixed text to check", m[1])
		}
		for _, p := range parts {
			if !strings.Contains(src, p) {
				t.Errorf("message %q: %q is in no non-test Go file under cmd/ or internal/", m[1], p)
			}
		}
	}
	if n < 10 {
		t.Errorf("docs/troubleshooting.md quotes %d messages, want at least 10", n)
	}
}
