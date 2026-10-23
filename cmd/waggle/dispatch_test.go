//go:build unix

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/waggle-sh/waggle/internal/scaffold"
)

const dispatchHelp = "--print --output-format stream-json --verbose --model --strict-mcp-config --setting-sources --settings " +
	"--permission-mode dontAsk --tools --append-system-prompt --restricted --no-session-persistence --disable-slash-commands\n"

const dispatchScript = `#!/bin/sh
d=$(dirname "$0")
case "$1" in
--version) echo "fake-claude 9.9"; exit 0;;
--help) cat "$d/help.txt"; exit 0;;
esac
cat >/dev/null
ok='{"type":"result","subtype":"success","is_error":false,"result":"done","session_id":"s1","usage":{"input_tokens":10,"output_tokens":1}}'
case $(basename "$PWD") in
T-023-builder)
  mkdir -p internal/x; echo ok > internal/x/b.go; git add -A
  git -c user.name=a -c user.email=a@e.co commit -q -s -m 'T-023: b'
  printf '%s\n' "$ok";;
T-024-builder) printf '%s\n' "$ok";;
T-025-builder) printf '%s\n' '{"type":"result","subtype":"error_max_turns","is_error":true}'; exit 1;;
esac
`

// dispatchRepo is a scaffolded hive with briefs for tickets 23 to 25 and a fake claude.
func dispatchRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	repo, bin := filepath.Join(t.TempDir(), "repo"), t.TempDir()
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"claude": dispatchScript, "help.txt": dispatchHelp, "scenario.sh": ""} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	git(t, repo, "init", "-q", "-b", "main")
	git(t, repo, "config", "--local", "user.name", "Fixture Keeper")
	git(t, repo, "config", "--local", "user.email", "keeper@example.com")
	old := os.Stdout
	os.Stdout, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	err := scaffold.InitRepo(scaffold.InitOptions{TargetDir: repo, Profile: "cli", Delivery: "installed", Forge: "github", CombMode: "direct", Runtime: "claude-code", Out: io.Discard})
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := os.ReadFile(filepath.Join(repo, ".waggle/control/config.yaml"))
	files := map[string]string{
		".waggle/control/config.yaml": strings.Replace(string(cfg), "binary: claude", "binary: "+filepath.Join(bin, "claude"), 1),
		".waggle/control/local.yaml":  "keeper_id: alice\n",
		"internal/x/a.go":             "package x\n",
	}
	for _, n := range []string{"023", "024", "025"} {
		files[".waggle/hive/tasks/T-"+n+".md"] = "# brief\n"
	}
	for rel, body := range files {
		p := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "init")
	return repo
}

// Test 15: every exit code, --json equals the file, no U+2013 or U+2014 anywhere.
func TestDispatchCLI(t *testing.T) {
	repo := dispatchRepo(t)
	base := func(n, dir string, extra ...string) []string {
		return append([]string{n, "--role", "builder", "--dir", dir, "--gate", "full", "--no-cell", "--repo", repo}, extra...)
	}
	cases := []struct {
		name string
		args []string
		exit int
	}{
		{"done", base("23", "internal/x"), 0},
		{"no commits", base("24", "internal/y"), 1},
		{"usage: bad role", append(base("26", "internal/w"), "--role", "queen"), 2},
		{"usage: no ticket", []string{"--role", "builder"}, 2},
		{"environment: no config", []string{"23", "--role", "builder", "--dir", "internal/x", "--gate", "full", "--no-cell", "--repo", t.TempDir()}, 3},
		{"claim refused", base("27", ".waggle/hive"), 4},
		{"max turns", base("25", "internal/z"), 5},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		if got := runDispatch(c.args, &out, &errb); got != c.exit {
			t.Fatalf("%s: exit %d, want %d\nstdout: %s\nstderr: %s", c.name, got, c.exit, out.String(), errb.String())
		}
		if all := out.String() + errb.String(); strings.ContainsAny(all, "\u2013\u2014") {
			t.Fatalf("%s: output has an en or em dash: %s", c.name, all)
		}
	}

	// --no-cell is required and the refusal lists what is not enforced.
	var out, errb bytes.Buffer
	if got := runDispatch([]string{"23", "--role", "builder", "--dir", "internal/x", "--gate", "full", "--repo", repo}, &out, &errb); got != 2 ||
		!strings.Contains(errb.String(), "read-only root") || !strings.Contains(errb.String(), "network egress") {
		t.Fatalf("missing --no-cell: exit %d stderr %s", got, errb.String())
	}

	// --json prints only result.json, byte for byte. Ticket 23 ran above; its
	// branch holds the commit, so a rerun on a fresh ticket is the cleaner probe.
	out.Reset()
	errb.Reset()
	git(t, repo, "branch", "-q", "-D", "t-023")
	if got := runDispatch(append(base("23", "internal/x", "--json"), "--keep-worktree"), &out, &errb); got != 0 {
		t.Fatalf("--json run: exit %d stderr %s", got, errb.String())
	}
	var res struct {
		RunDir string `json:"run_dir"`
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("--json output is not json: %v\n%s", err, out.String())
	}
	if file, err := os.ReadFile(filepath.Join(res.RunDir, "result.json")); err != nil || !bytes.Equal(file, out.Bytes()) {
		t.Fatalf("--json output differs from result.json: %v", err)
	}
}

// D2-T8: the clean subcommand's exit codes, and no U+2013 or U+2014 in its output.
func TestDispatchCleanCLI(t *testing.T) {
	repo := dispatchRepo(t)
	cases := []struct {
		name string
		args []string
		exit int
	}{
		{"nothing to clean", []string{"clean", "23", "--repo", repo}, 0},
		{"ticket after the flags", []string{"clean", "--repo", repo, "--runs", "23"}, 0},
		{"usage: no ticket", []string{"clean", "--repo", repo}, 2},
		{"usage: two tickets", []string{"clean", "23", "24", "--repo", repo}, 2},
		{"usage: bad ticket", []string{"clean", "x", "--repo", repo}, 2},
		{"usage: bad role", []string{"clean", "23", "--role", "queen", "--repo", repo}, 2},
		{"usage: unknown flag", []string{"clean", "23", "--force", "--repo", repo}, 2},
		{"host record: absent", []string{"clean", "--host-record", "--repo", repo}, 0},
		{"usage: host record with a ticket", []string{"clean", "23", "--host-record", "--repo", repo}, 2},
		{"usage: accept without host record", []string{"clean", "23", "--accept", "0123456789abcdef", "--repo", repo}, 2},
		{"usage: host record with a role", []string{"clean", "--host-record", "--role", "builder", "--repo", repo}, 2},
		{"usage: host record with runs", []string{"clean", "--host-record", "--runs", "--repo", repo}, 2},
		{"usage: bad digest", []string{"clean", "--host-record", "--accept", "xyz", "--repo", repo}, 2},
		{"usage: empty digest", []string{"clean", "--host-record", "--accept", "", "--repo", repo}, 2},
		{"environment: not a repo", []string{"clean", "23", "--repo", t.TempDir()}, 3},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		if got := runDispatch(c.args, &out, &errb); got != c.exit {
			t.Fatalf("%s: exit %d, want %d\nstdout: %s\nstderr: %s", c.name, got, c.exit, out.String(), errb.String())
		}
		if all := out.String() + errb.String(); strings.ContainsAny(all, "\u2013\u2014") {
			t.Fatalf("%s: output has an en or em dash: %s", c.name, all)
		}
	}
	var out, errb bytes.Buffer
	if runDispatch([]string{"--help"}, &out, &errb); !strings.Contains(errb.String(), "waggle dispatch clean <ticket>") || !strings.Contains(errb.String(), "waggle dispatch clean --host-record [--accept <digest>] [--repo <dir>]") {
		t.Fatalf("usage does not list clean: %s", errb.String())
	}
}

// T-030: the release subcommand's exit codes and usage line. It never goes through Run.
func TestDispatchReleaseCLI(t *testing.T) {
	repo := dispatchRepo(t)
	cases := []struct {
		name string
		args []string
		exit int
	}{
		{"no claim", []string{"release", "23", "--repo", repo}, 0},
		{"no claim, ticket after the flags", []string{"release", "--repo", repo, "--unmerged", "23"}, 0},
		{"usage: no ticket", []string{"release", "--repo", repo}, 2},
		{"usage: two tickets", []string{"release", "23", "24", "--repo", repo}, 2},
		{"usage: bad ticket", []string{"release", "x", "--repo", repo}, 2},
		{"usage: other ticket's branch", []string{"release", "23", "--branch", "t-024", "--repo", repo}, 2},
		{"usage: unknown flag", []string{"release", "23", "--force", "--repo", repo}, 2},
		{"environment: not a repo", []string{"release", "23", "--repo", t.TempDir()}, 3},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		if got := runDispatch(c.args, &out, &errb); got != c.exit {
			t.Fatalf("%s: exit %d, want %d\nstdout: %s\nstderr: %s", c.name, got, c.exit, out.String(), errb.String())
		}
		if all := out.String() + errb.String(); strings.ContainsAny(all, string([]rune{0x2013, 0x2014})) {
			t.Fatalf("%s: output has an en or em dash: %s", c.name, all)
		}
	}
	var out, errb bytes.Buffer
	if runDispatch([]string{"--help"}, &out, &errb); !strings.Contains(errb.String(), "waggle dispatch release <ticket> [--branch <name>] [--unmerged] [--repo <dir>]") {
		t.Fatalf("usage does not list release: %s", errb.String())
	}
}

// D2b-T7: a closed terminal stops dispatch like Ctrl-C. No signal is sent.
func TestStopSignals(t *testing.T) {
	for _, want := range []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP} {
		if !slices.Contains(stopSignals, want) {
			t.Errorf("stopSignals lacks %v", want)
		}
	}
}
