package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
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
	_, err := a.Run(ctx, Invocation{Role: "builder", TaskID: "T-5"})
	wantErr(t, err, ErrUnresolved, "builder", "T-5")

	guard := mustResolve(t, testPolicy(), "guard", nil, nil)
	_, err = a.Run(ctx, Invocation{Role: "builder", TaskID: "T-5", Selection: guard})
	wantErr(t, err, ErrUnresolved, "builder")

	_, err = a.Run(ctx, Invocation{Role: "builder", Selection: Selection{Role: "builder", Model: "opus"}})
	wantErr(t, err, ErrUnresolved)

	builder := mustResolve(t, testPolicy(), "builder", nil, nil)
	_, err = a.Run(ctx, Invocation{Role: "builder", Selection: builder})
	if err == nil || errors.Is(err, ErrUnresolved) {
		t.Errorf("Run with a resolved selection: %v; want the waggled error only", err)
	}
}

func mustArgs(t *testing.T, a *ClaudeAdapter, inv Invocation, s *ContextSettings) []string {
	t.Helper()
	got, err := a.claudeArgs(inv, s)
	if err != nil {
		t.Fatalf("claudeArgs: %v", err)
	}
	for _, arg := range got {
		if flag, _, _ := strings.Cut(arg, "="); resumeFlags[flag] {
			t.Errorf("claudeArgs emitted resume flag %q in %q", arg, got)
		}
	}
	if !slices.Contains(got, "--strict-mcp-config") {
		t.Errorf("claudeArgs = %q; missing --strict-mcp-config", got)
	}
	return got
}

func TestClaudeArgs(t *testing.T) {
	a := NewClaudeAdapter(AdapterConfig{DefaultArgs: []string{"--print", "--verbose"}})
	sel := mustResolve(t, testPolicy(), "builder", nil, nil)
	inv := Invocation{Role: "builder", Selection: sel}

	// Without settings: T-012's argv plus the S6 flag.
	got := mustArgs(t, a, inv, nil)
	want := []string{"--model", "sonnet", "--strict-mcp-config", "--print", "--verbose"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("claudeArgs = %q; want %q", got, want)
	}
	if got := mustArgs(t, NewClaudeAdapter(AdapterConfig{}), Invocation{Selection: sel}, nil); len(got) != 3 {
		t.Errorf("claudeArgs without defaults = %q", got)
	}

	s, err := ContextPolicy{Default: ProfileLean}.Settings("builder", nil, 200000)
	if err != nil {
		t.Fatal(err)
	}
	got = mustArgs(t, a, inv, &s)
	want = []string{"--model", "sonnet", "--strict-mcp-config", "--max-turns", "40", "--print", "--verbose"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("claudeArgs with settings = %q; want %q", got, want)
	}

	for _, bad := range []string{"--continue", "-c", "--resume", "-r", "--session-id", "--resume=abc", "--session-id=123"} {
		a := NewClaudeAdapter(AdapterConfig{DefaultArgs: []string{"--print", bad, "x"}})
		if _, err := a.claudeArgs(inv, &s); !errors.Is(err, ErrResumeFlag) {
			t.Errorf("claudeArgs with default arg %q err = %v; want ErrResumeFlag", bad, err)
		}
	}
}

func TestClaudeEnv(t *testing.T) {
	if env := claudeEnv(nil); env != nil {
		t.Errorf("claudeEnv(nil) = %q; want nil", env)
	}
	want := map[Profile]string{
		ProfileLean:     "BASH_MAX_OUTPUT_LENGTH=10000 MAX_THINKING_TOKENS=4000",
		ProfileBalanced: "BASH_MAX_OUTPUT_LENGTH=20000 MAX_THINKING_TOKENS=12000",
		ProfileThorough: "BASH_MAX_OUTPUT_LENGTH=30000 MAX_THINKING_TOKENS=32000",
	}
	for p, w := range want {
		s, err := ContextPolicy{Default: p}.Settings("builder", nil, 200000)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(claudeEnv(&s), " "); got != w {
			t.Errorf("claudeEnv(%s) = %q; want %q", p, got, w)
		}
	}
	if got := claudeEnv(&ContextSettings{ToolOutputChars: 5}); len(got) != 1 || got[0] != "BASH_MAX_OUTPUT_LENGTH=5" {
		t.Errorf("claudeEnv with unknown thinking step = %q", got)
	}
}

func TestMaterializeMissingRoles(t *testing.T) {
	if err := NewClaudeAdapter(AdapterConfig{}).Materialize(t.TempDir()); err == nil {
		t.Error("Materialize without roles succeeded; want error")
	}
}
