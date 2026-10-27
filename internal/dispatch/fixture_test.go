// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/scaffold"
)

const fakeHelp = "--print --output-format stream-json --verbose --model --strict-mcp-config --setting-sources --settings " +
	"--permission-mode dontAsk --tools --append-system-prompt --restricted --no-session-persistence --disable-slash-commands\n"

const fakeScript = `#!/bin/sh
d=$(dirname "$0")
case "$1" in
--version) echo "fake-claude 9.9"; exit 0;;
--help) cat "$d/help.txt"; exit 0;;
esac
cat >/dev/null
. "$d/scenario.sh"
`

const okResult = `{"type":"result","subtype":"success","is_error":false,"result":"all done","total_cost_usd":0.05,"session_id":"s1","usage":{"input_tokens":100,"output_tokens":10,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`

// emit returns shell lines that print stream-json lines.
func emit(lines ...string) string {
	s := ""
	for _, l := range lines {
		s += "printf '%s\\n' '" + l + "'\n"
	}
	return s
}

func assistant(inputTokens int) string {
	return fmt.Sprintf(`{"type":"assistant","message":{"id":"m1","usage":{"input_tokens":%d,"output_tokens":10}}}`, inputTokens)
}

// commitFile adds path with body and commits it signed, as the worker would.
func commitFile(path, body string) string {
	return fmt.Sprintf("mkdir -p $(dirname %s)\necho %s > %s\ngit add %s\ngit -c user.name=a -c user.email=a@e.co commit -q -s -m 'T-023: %s'\n",
		path, body, path, path, strings.ReplaceAll(path, "/", "-"))
}

// okScenario is a worker that commits one file inside internal/x and reports.
var okScenario = commitFile("internal/x/b.go", "ok") + emit(assistant(100), okResult)

// fixture is a hive repo with the scaffolded config, a fake claude binary and
// optionally a bare remote.
type fixture struct {
	t              *testing.T
	repo, real     string // repo is what Run gets, possibly through a symlink
	origin, binDir string
}

func newFixture(t *testing.T, remote, link bool) *fixture {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	f := &fixture{t: t, real: filepath.Join(root, "repo"), binDir: filepath.Join(root, "bin"), origin: filepath.Join(root, "origin.git")}
	for _, d := range []string{f.real, f.binDir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.repo = f.real
	if link {
		f.repo = filepath.Join(t.TempDir(), "link")
		if err := os.Symlink(f.real, f.repo); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"claude": fakeScript, "help.txt": fakeHelp} {
		if err := os.WriteFile(filepath.Join(f.binDir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	sh(t, f.real, "init", "-q", "-b", "main")
	quietGit(t, f.real)
	sh(t, f.real, "config", "--local", "user.name", "Fixture Keeper")
	sh(t, f.real, "config", "--local", "user.email", "keeper@example.com")
	opts := scaffold.InitOptions{TargetDir: f.real, Profile: "cli", Delivery: "installed", Forge: "github", CombMode: "direct", Runtime: "claude-code", Out: io.Discard}
	quiet(t, func() {
		if err := scaffold.InitRepo(opts); err != nil {
			t.Fatal(err)
		}
	})
	cfgPath := filepath.Join(f.real, ".waggle/control/config.yaml")
	cfg, _ := os.ReadFile(cfgPath)
	c := strings.Replace(string(cfg), "binary: claude", "binary: "+filepath.Join(f.binDir, "claude"), 1)
	if remote {
		c = strings.Replace(c, "  provider: github", "  provider: github\n  remote: origin", 1)
	}
	write(t, f.real, ".waggle/control/config.yaml", c)
	write(t, f.real, ".waggle/control/local.yaml", "keeper_id: alice\n")
	for _, n := range []string{"023", "024"} {
		write(t, f.real, ".waggle/hive/tasks/T-"+n+".md", "# brief "+n+"\n")
	}
	write(t, f.real, "internal/x/a.go", "package x\n")
	write(t, f.real, "README", "readme\n")
	sh(t, f.real, "add", "-A")
	sh(t, f.real, "commit", "-q", "-m", "init")
	if remote {
		sh(t, root, "init", "-q", "--bare", "-b", "main", f.origin)
		quietGit(t, f.origin)
		sh(t, f.real, "remote", "add", "origin", f.origin)
		sh(t, f.real, "push", "-q", "origin", "main")
		sh(t, f.real, "fetch", "-q", "origin")
	}
	return f
}

func (f *fixture) git(args ...string) string { f.t.Helper(); return sh(f.t, f.real, args...) }

func (f *fixture) common() string { return filepath.Join(f.real, ".git") }

// run installs scenario as the worker's script and calls Run for ticket 23.
func (f *fixture) run(scenario string, mut ...func(*Request, *Deps)) (Result, error) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.binDir, "scenario.sh"), []byte(scenario), 0o644); err != nil {
		f.t.Fatal(err)
	}
	req, d := okReq(f.repo), Deps{Cell: HostCell()}
	for _, m := range mut {
		m(&req, &d)
	}
	return Run(context.Background(), req, d)
}

// pushed reports whether the bare remote holds the branch.
func (f *fixture) pushed(branch string) bool {
	_, err := os.Stat(f.origin)
	return err == nil && sh(f.t, f.origin, "for-each-ref", "refs/heads/"+branch) != ""
}

func withDirs(dirs ...string) func(*Request, *Deps) {
	return func(r *Request, _ *Deps) { r.Dirs = dirs }
}

func hasViolation(vs []comb.Violation, rule, path string) bool {
	for _, v := range vs {
		if v.Rule == rule && (path == "" || v.Path == path) {
			return true
		}
	}
	return false
}

// eachRepoSpelling runs fn for a plain repo path and for one reached through a
// symlinked parent (test 26): macOS hides path bugs that ubuntu CI shows.
func eachRepoSpelling(t *testing.T, fn func(t *testing.T, link bool)) {
	for _, link := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "symlinked"}[link], func(t *testing.T) { fn(t, link) })
	}
}

// quiet runs fn with stdout discarded: scaffold prints its shim list there.
func quiet(t *testing.T, fn func()) {
	t.Helper()
	old := os.Stdout
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = null
	defer func() { os.Stdout = old; null.Close() }()
	fn()
}

// wait polls cond for up to 5 seconds.
func wait(cond func() bool) bool {
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if cond() {
			return true
		}
	}
	return cond()
}
