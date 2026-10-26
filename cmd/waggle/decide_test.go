package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
)

const decideInput = `{
  "kind": "technical",
  "title": "Config parser is a hand-written YAML subset",
  "question": "Which parser reads .waggle/config.yaml?",
  "options": ["A: add a YAML dependency", "B: hand-written subset parser"],
  "recommendation": "B",
  "choice": "B: hand-written YAML subset parser, stdlib only, fuzz tested",
  "rationale": "Config is machine-written and tiny.",
  "tickets": ["T-004"],
  "supersedes": []
}
`

// decideRepo creates a hive root with a .waggle/ directory and an input file.
func decideRepo(t *testing.T) (repo, input string) {
	t.Helper()
	repo = t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, layout.Root), 0o755); err != nil {
		t.Fatal(err)
	}
	input = filepath.Join(repo, "in.json")
	if err := os.WriteFile(input, []byte(decideInput), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, input
}

func decide(stdin string, args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = runDecide(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func logPath(repo string) string {
	return filepath.Join(repo, filepath.FromSlash(layout.Decisions))
}

func TestRunDecideRecords(t *testing.T) {
	repo, input := decideRepo(t)
	code, out, errb := decide("", "--file", input, "--repo", repo)
	if code != 0 || out != "recorded D-001 in "+layout.Decisions+"\n" || errb != "" {
		t.Fatalf("code %d, stdout %q, stderr %q", code, out, errb)
	}
	data, err := os.ReadFile(logPath(repo))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "## D-001: Config parser is a hand-written YAML subset\n") {
		t.Errorf("log:\n%s", data)
	}
	code, out, _ = decide(decideInput, "--file", "-", "--repo", repo)
	if code != 0 || !strings.HasPrefix(out, "recorded D-002 in ") {
		t.Errorf("stdin: code %d, stdout %q", code, out)
	}
}

func TestRunDecideDryRun(t *testing.T) {
	repo, input := decideRepo(t)
	code, out, errb := decide("", "--file", input, "--repo", repo, "--dry-run")
	if code != 0 || !strings.HasPrefix(out, "## D-001: Config parser") || !strings.Contains(out, "**Rationale:** Config is machine-written and tiny.\n") {
		t.Fatalf("code %d, stdout %q, stderr %q", code, out, errb)
	}
	if _, err := os.Stat(logPath(repo)); !errors.Is(err, os.ErrNotExist) {
		t.Error("dry run wrote the log")
	}
}

func TestRunDecideUsage(t *testing.T) {
	repo, input := decideRepo(t)
	cases := map[string][]string{
		"missing --file": {"--repo", repo},
		"unknown --date": {"--file", input, "--repo", repo, "--date", "2020-01-01"},
		"unknown --id":   {"--file", input, "--repo", repo, "--id", "D-009"},
		"unknown --edit": {"--file", input, "--repo", repo, "--edit"},
		"extra argument": {"--file", input, "--repo", repo, "extra"},
	}
	for name, args := range cases {
		if code, _, errb := decide("", args...); code != 2 || !strings.Contains(errb, "Usage: waggle decide") {
			t.Errorf("%s: code %d, stderr %q", name, code, errb)
		}
	}
	if code, _, errb := decide("", "-h"); code != 0 || !strings.Contains(errb, "Agents never run this") {
		t.Errorf("help: code %d, stderr %q", code, errb)
	}
	if _, err := os.Stat(logPath(repo)); !errors.Is(err, os.ErrNotExist) {
		t.Error("a usage error wrote the log")
	}
}

func TestRunDecideFailures(t *testing.T) {
	repo, input := decideRepo(t)
	noHive := t.TempDir()
	if code, _, errb := decide("", "--file", input, "--repo", noHive); code != 1 || !strings.Contains(errb, "not a Waggle repo") {
		t.Errorf("no .waggle/: code %d, stderr %q", code, errb)
	}
	bad := strings.Replace(decideInput, `"technical"`, `"wish"`, 1)
	if code, _, errb := decide(bad, "--file", "-", "--repo", repo); code != 1 || !strings.Contains(errb, "kind") {
		t.Errorf("invalid input: code %d, stderr %q", code, errb)
	}
	if code, _, errb := decide("", "--file", filepath.Join(repo, "missing.json"), "--repo", repo); code != 1 || errb == "" {
		t.Errorf("missing input file: code %d, stderr %q", code, errb)
	}
	if _, err := os.Stat(logPath(repo)); !errors.Is(err, os.ErrNotExist) {
		t.Error("a failed decide wrote the log")
	}
	if entries, _ := os.ReadDir(filepath.Join(repo, layout.HiveDir)); len(entries) != 0 {
		t.Errorf("leftovers in hive dir: %v", entries)
	}
}
