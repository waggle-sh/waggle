package forge

import (
	"bytes"
	"strings"
	"testing"
)

func env(vals map[string]string) func(string) string {
	return func(k string) string { return vals[k] }
}

// TestAnnotateGitHubEncodes is T-014 test 15: injection strings in the path
// and the message produce no unencoded newline and no second "::" command.
func TestAnnotateGitHubEncodes(t *testing.T) {
	a := AnnotatorFromEnv(env(map[string]string{"GITHUB_ACTIONS": "true"}))
	var b bytes.Buffer
	a.Annotate(&b, "error", "docs/a.md\n::warning::x,y:z", "bad %0A\r\n::warning::x")
	out := b.String()
	want := "::error file=docs/a.md%0A%3A%3Awarning%3A%3Ax%2Cy%3Az::bad %250A%0D%0A::warning::x\n"
	if out != want {
		t.Errorf("got  %q\nwant %q", out, want)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Errorf("unencoded newline in %q", out)
	}
	if strings.Count("\n"+out, "\n::") != 1 {
		t.Errorf("want exactly one line starting a command in %q", out)
	}
	b.Reset()
	a.Annotate(&b, "bogus", "", "plain")
	if b.String() != "::error::plain\n" {
		t.Errorf("no file, unknown level: %q", b.String())
	}
}

func TestAnnotatePlain(t *testing.T) {
	for _, g := range []func(string) string{env(nil), env(map[string]string{"GITHUB_ACTIONS": "1"}), nil} {
		t.Setenv("GITHUB_ACTIONS", "")
		var b bytes.Buffer
		AnnotatorFromEnv(g).Annotate(&b, "error", "docs/a.md", "bad thing")
		if b.String() != "error: docs/a.md: bad thing\n" {
			t.Errorf("plain: %q", b.String())
		}
		b.Reset()
		AnnotatorFromEnv(g).Annotate(&b, "warning", "", "no file")
		if b.String() != "warning: no file\n" {
			t.Errorf("plain without file: %q", b.String())
		}
	}
}
