// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"bytes"
	"strings"
	"testing"
)

func lines(n int, prefix string) []byte {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(prefix)
		b.WriteString("\n")
	}
	return []byte(b.String())
}

func TestAssemblePromptStablePrefix(t *testing.T) {
	role, waggle := []byte("# Role: builder\nDo the task.\n"), []byte("# WAGGLE.md\nBoundaries.") // no final newline
	a := PromptParts{RolePrompt: role, WaggleMD: waggle, Brief: []byte("# T-101\nbuild it\n"),
		TaskLines: []string{"Task: T-101", "Lease: internal/a/"}, ReadOnly: []ReadOnlyFile{{Path: "docs/x.md", Content: []byte("x\n")}}}
	b := PromptParts{RolePrompt: role, WaggleMD: waggle, Brief: []byte("# T-202\nfix it\n"),
		TaskLines: []string{"Task: T-202", "Lease: internal/b/", "Handoff: .waggle/control/handoffs/T-202-builder-1.md"},
		Handoff:   RenderHandoff(Handoff{Task: "T-202", Role: "builder", Seq: 1, Done: []string{"half"}})}
	balanced := ContextSettings{}
	stableA, varA := AssemblePrompt(a, balanced)
	stableB, varB := AssemblePrompt(b, balanced)
	if !bytes.Equal(stableA, stableB) {
		t.Errorf("stable differs across tasks:\n%s\n---\n%s", stableA, stableB)
	}
	if want := "# Role: builder\nDo the task.\n\n# WAGGLE.md\nBoundaries.\n\n"; string(stableA) != want {
		t.Errorf("stable = %q; want %q", stableA, want)
	}
	for _, id := range []string{"T-101", "T-202", "handoffs"} {
		if bytes.Contains(stableA, []byte(id)) {
			t.Errorf("stable contains %q", id)
		}
	}
	wantA := "# T-101\nbuild it\n\nRead-only context (open as needed): docs/x.md\n\nTask: T-101\nLease: internal/a/\n\n"
	if string(varA) != wantA {
		t.Errorf("variable A = %q; want %q", varA, wantA)
	}
	if bytes.Contains(varA, []byte("x\n\n")) {
		t.Error("read-only content inlined without InlineReadOnly")
	}
	if !strings.HasSuffix(string(varB), string(b.Handoff)+"\n") || !bytes.Contains(varB, []byte("Task: T-202\nLease: internal/b/\nHandoff: ")) {
		t.Errorf("variable B = %q", varB)
	}
	if bytes.Index(varB, []byte("# T-202")) > bytes.Index(varB, []byte("Task: T-202")) {
		t.Error("brief does not precede task lines")
	}
}

func TestAssemblePromptInlineCap(t *testing.T) {
	inline := ContextSettings{InlineReadOnly: true}
	p := PromptParts{Brief: []byte("brief\n"), ReadOnly: []ReadOnlyFile{
		{Path: "a.md", Content: lines(1000, "a")},
		{Path: "b.md", Content: bytes.TrimSuffix(lines(500, "b"), []byte("\n"))}, // 500 lines, no final newline
		{Path: "c.md", Content: lines(1, "c")},                                   // over the cap: a path
		{Path: "d.md", Content: lines(1, "d")},                                   // after a path: a path
	}}
	_, v := AssemblePrompt(p, inline)
	s := string(v)
	if !strings.HasPrefix(s, "brief\n\n## Read-only: a.md\n\na\n") || !strings.Contains(s, "## Read-only: b.md\n\nb\n") {
		t.Errorf("inlined files missing:\n%.80s", s)
	}
	if strings.Count(s, "a\n") != 1000 || strings.Count(s, "b\n") != 500 {
		t.Errorf("inlined line counts: a=%d b=%d", strings.Count(s, "a\n"), strings.Count(s, "b\n"))
	}
	if strings.Contains(s, "## Read-only: c.md") || strings.Contains(s, "## Read-only: d.md") {
		t.Error("file beyond the cap was inlined")
	}
	if !strings.HasSuffix(s, "Read-only context (open as needed): c.md\nRead-only context (open as needed): d.md\n\n") {
		t.Errorf("paths for the rest missing:\n%s", s[len(s)-120:])
	}
	if c := strings.Count(s, "\n"); c > InlineReadOnlyLines+20 {
		t.Errorf("variable has %d lines; inline cap not respected", c)
	}

	p.ReadOnly[2].Content = lines(1501, "c")
	_, v = AssemblePrompt(PromptParts{ReadOnly: p.ReadOnly[2:3]}, inline)
	if want := "Read-only context (open as needed): c.md\n\n"; string(v) != want {
		t.Errorf("single file over the cap = %q; want %q", v, want)
	}
	_, v = AssemblePrompt(PromptParts{ReadOnly: []ReadOnlyFile{{Path: "e.md", Content: lines(1500, "e")}}}, inline)
	if !strings.HasPrefix(string(v), "## Read-only: e.md\n\ne\n") {
		t.Errorf("file at exactly the cap was not inlined: %.40q", v)
	}
	if s, v := AssemblePrompt(PromptParts{}, inline); len(s) != 0 || len(v) != 0 {
		t.Errorf("empty parts = %q, %q", s, v)
	}
}
