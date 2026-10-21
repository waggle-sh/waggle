package dispatch

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Test 27 (D-166): the package and its CLI keep building and vetting for
// Windows, where dispatch refuses to run. This is a Go test, not a CI step.
func TestCrossCompileWindows(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		if parent := filepath.Dir(root); parent != root {
			root = parent
			continue
		}
		t.Fatal("no go.mod above the test directory")
	}
	for _, target := range []string{"windows", "solaris", "illumos"} {
		cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "vet", "./internal/dispatch/", "./cmd/waggle/")
		cmd.Dir, cmd.Env = root, append(os.Environ(), "GOOS="+target, "GOARCH=amd64", "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("GOOS=%s go vet: %v\n%s", target, err, out)
		}
	}
}

// Test 28: redirectVars is the list the Comb scrubs, read from its source
// because it is unexported (dispatch has no internal/comb lease).
func TestRedirectVarsMatchComb(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "comb", "git.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var comb []string
	ast.Inspect(f, func(n ast.Node) bool {
		if vs, ok := n.(*ast.ValueSpec); ok && len(vs.Names) == 1 && vs.Names[0].Name == "scrubbedGitEnv" {
			for _, e := range vs.Values[0].(*ast.CompositeLit).Elts {
				s, _ := strconv.Unquote(e.(*ast.BasicLit).Value)
				comb = append(comb, s)
			}
		}
		return true
	})
	ours := strings.Fields(redirectVars)
	slices.Sort(comb)
	slices.Sort(ours)
	if len(comb) == 0 || !slices.Equal(comb, ours) {
		t.Fatalf("redirectVars %v, comb scrubbedGitEnv %v", ours, comb)
	}
}

func TestPinnedEnv(t *testing.T) {
	in := []string{"PATH=/bin", "GIT_CONFIG_PARAMETERS='a.b=c'", "git_config_count=1", "GIT_CONFIG_KEY_0=k", "GIT_CONFIG_VALUE_0=v",
		"GIT_CONFIG_KEY_12=k", "GIT_EXEC_PATH=/x", "GIT_SSH_COMMAND=ssh", "GIT_CONFIG_GLOBAL=/home/me/.gitconfig"}
	got := strings.Join(pinnedEnv(in), "\n")
	for _, bad := range []string{"PARAMETERS", "count", "KEY_0", "VALUE_0", "KEY_12", "EXEC_PATH", "/home/me"} {
		if strings.Contains(got, bad) {
			t.Fatalf("%s survived: %s", bad, got)
		}
	}
	for _, keep := range []string{"PATH=/bin", "GIT_SSH_COMMAND=ssh", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_ATTR_NOSYSTEM=1"} {
		if !strings.Contains(got, keep) {
			t.Fatalf("%s missing: %s", keep, got)
		}
	}
}

func TestCanonicalPaths(t *testing.T) {
	for p, want := range map[string]bool{"a/b.go": true, "": false, "/a": false, `a\b`: false, "a//b": false, "a/./b": false, "a/../b": false,
		".git/config": false, "x/.GIT/y": false, "a/.github/b": true, "a/": false, "docs/../.github/x": false} {
		if canonical(p) != want {
			t.Fatalf("canonical(%q) = %v", p, !want)
		}
	}
}
