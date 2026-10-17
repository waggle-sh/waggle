package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestMaterializeShims(t *testing.T) {
	dir := t.TempDir()
	roles := filepath.Join(dir, ".waggle", "control", "roles")
	writeFile(t, filepath.Join(roles, "builder.md"), "# Builder\n")
	writeFile(t, filepath.Join(roles, "guard.md"), "# Guard\n")
	writeFile(t, filepath.Join(roles, "notes.txt"), "ignored\n")
	agents := filepath.Join(dir, ".claude", "agents")
	handBuilder := "---\nname: builder\n---\nmine\n"
	writeFile(t, filepath.Join(agents, "builder.md"), handBuilder)
	handGuard := "---\nname: guard\n---\nhand-written at a shim path\n"
	writeFile(t, filepath.Join(agents, "waggle-guard.md"), handGuard)

	a := NewClaudeAdapter(AdapterConfig{})
	for i := 0; i < 2; i++ { // second run overwrites its own generated shim
		if err := a.Materialize(dir); err != nil {
			t.Fatal(err)
		}
	}

	got := readFile(t, filepath.Join(agents, "waggle-builder.md"))
	want := "---\n# waggle:generated\nname: builder\n" +
		"description: Waggle builder role, generated from .waggle/control/roles/builder.md; edit the source, not this file\n" +
		"---\n\n# Builder\n"
	if got != want {
		t.Errorf("waggle-builder.md =\n%s\nwant\n%s", got, want)
	}
	if !IsGeneratedShim([]byte(got)) {
		t.Error("generated shim not recognized")
	}
	if strings.ContainsAny(got, "\u2014\u2013") {
		t.Error("shim contains an em or en dash")
	}
	if s := readFile(t, filepath.Join(agents, "builder.md")); s != handBuilder {
		t.Errorf("hand-written builder.md changed: %q", s)
	}
	if s := readFile(t, filepath.Join(agents, "waggle-guard.md")); s != handGuard {
		t.Errorf("unmarked waggle-guard.md overwritten: %q", s)
	}
	if _, err := os.Stat(filepath.Join(agents, "waggle-notes.txt")); !os.IsNotExist(err) {
		t.Error("non-markdown role produced a shim")
	}
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Error("adapter wrote CLAUDE.md")
	}
}

func TestRunRefusesUnresolvedSelection(t *testing.T) {
	a := NewClaudeAdapter(AdapterConfig{})
	ctx := context.Background()
	_, err := a.Run(ctx, Invocation{Role: "forager", TaskID: "T-5"})
	wantErr(t, err, ErrUnresolved, "forager", "T-5")

	guard := mustResolve(t, testPolicy(), "guard", nil, nil)
	_, err = a.Run(ctx, Invocation{Role: "forager", TaskID: "T-5", Selection: guard})
	wantErr(t, err, ErrUnresolved, "forager")

	_, err = a.Run(ctx, Invocation{Role: "forager", Selection: Selection{Role: "forager", Model: "opus"}})
	wantErr(t, err, ErrUnresolved)

	forager := mustResolve(t, testPolicy(), "forager", nil, nil)
	_, err = a.Run(ctx, Invocation{Role: "forager", Selection: forager})
	if err == nil || errors.Is(err, ErrUnresolved) {
		t.Errorf("Run with a resolved selection: %v; want the waggled error only", err)
	}
}

func TestClaudeArgs(t *testing.T) {
	a := NewClaudeAdapter(AdapterConfig{DefaultArgs: []string{"--print", "--verbose"}})
	sel := mustResolve(t, testPolicy(), "forager", nil, nil)
	got := a.claudeArgs(Invocation{Role: "forager", Selection: sel})
	want := []string{"--model", "sonnet", "--print", "--verbose"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("claudeArgs = %q; want %q", got, want)
	}
	if got := NewClaudeAdapter(AdapterConfig{}).claudeArgs(Invocation{Selection: sel}); len(got) != 2 {
		t.Errorf("claudeArgs without defaults = %q", got)
	}
}

func TestMaterializeMissingRoles(t *testing.T) {
	if err := NewClaudeAdapter(AdapterConfig{}).Materialize(t.TempDir()); err == nil {
		t.Error("Materialize without roles succeeded; want error")
	}
}
