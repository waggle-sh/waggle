// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// updateGolden rewrites the files under testdata/json instead of comparing
// against them: go test ./cmd/waggle -run TestJSONGolden -update
var updateGolden = flag.Bool("update", false, "rewrite the --json golden files under testdata/json")

var (
	goldenTime = regexp.MustCompile(`\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ`)
	goldenOID  = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
)

// goldenForm turns one line of --json output into the text a golden holds:
// the repository path, object ids and timestamps become placeholders, and the
// object is indented so a difference reads as a line diff. Key order is the
// command's own, so a renamed or reordered key shows up as a difference.
func goldenForm(t *testing.T, raw, repo string) string {
	t.Helper()
	if strings.Count(raw, "\n") != 1 || !strings.HasSuffix(raw, "}\n") {
		t.Fatalf("stdout is not one JSON line: %q", raw)
	}
	s := raw
	if repo != "" {
		s = strings.ReplaceAll(s, repo, "<REPO>")
	}
	s = goldenOID.ReplaceAllString(s, "<OID>")
	s = goldenTime.ReplaceAllString(s, "<TIME>")
	var b bytes.Buffer
	if err := json.Indent(&b, []byte(strings.TrimSuffix(s, "\n")), "", "  "); err != nil {
		t.Fatalf("not JSON: %v: %q", err, s)
	}
	b.WriteByte('\n')
	return b.String()
}

// checkGolden compares got with testdata/json/<name>.json, or rewrites that
// file under -update.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "json", name+".json")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run: go test ./cmd/waggle -run TestJSONGolden -update)", err)
	}
	if string(want) == got {
		return
	}
	gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	var diff strings.Builder
	for i := 0; i < max(len(gl), len(wl)); i++ {
		g, w := "", ""
		if i < len(gl) {
			g = gl[i]
		}
		if i < len(wl) {
			w = wl[i]
		}
		if g != w {
			diff.WriteString("line " + strconv.Itoa(i+1) + "\n  want: " + w + "\n  got:  " + g + "\n")
		}
	}
	t.Errorf("%s differs from %s (run with -update to accept):\n%s", name, path, diff.String())
}

// goldenRun runs fn with --json and returns the golden form of its stdout and
// the exit code.
func goldenRun(t *testing.T, fn func([]string, *bytes.Buffer, *bytes.Buffer) int, repo string, args ...string) (string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	exit := fn(append(args, "--json"), &out, &errb)
	return goldenForm(t, out.String(), repo), exit
}

// The --json commands today: claim, release, force-release, nectar (the summary),
// nectar explain, nectar backfill, nectar gate-receipt, vote, quorum open, status,
// close and verify, hives status, hives plan and dispatch. waggle status and
// waggle doctor have no --json. This test pins claim, release, force-release and
// the nectar summary; the others are not pinned here.
//
// claim, release and force-release have no --now flag, so their timestamps come
// from the clock and are replaced by <TIME>. Every object id becomes <OID>.
func TestJSONGolden(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Run("claim and release", func(t *testing.T) {
		repo := dispatchRepo(t)
		steps := []struct {
			name string
			fn   func([]string, *bytes.Buffer, *bytes.Buffer) int
			exit int
			args []string
		}{
			{"claim", claimCLI, 0, []string{"T-32", "--dirs", "internal/y,internal/x", "--gate", "full", "--branch", "t-032", "--owner", "alice", "--repo", repo}},
			{"claim_renewed", claimCLI, 0, []string{"T-32", "--dirs", "internal/x,internal/y", "--gate", "full", "--branch", "t-032", "--owner", "alice", "--repo", repo}},
			{"claim_error", claimCLI, 2, []string{"T-32", "--gate", "full", "--owner", "alice", "--repo", repo}},
			{"release", releaseCLI, 0, []string{"T-32", "--unmerged", "--owner", "alice", "--repo", repo}},
			{"release_no_claim", releaseCLI, 0, []string{"T-32", "--unmerged", "--owner", "alice", "--repo", repo}},
		}
		for _, s := range steps {
			got, exit := goldenRun(t, s.fn, repo, s.args...)
			if exit != s.exit {
				t.Fatalf("%s: exit %d, want %d\n%s", s.name, exit, s.exit, got)
			}
			checkGolden(t, s.name, got)
		}
	})

	t.Run("force-release inspect", func(t *testing.T) {
		repo := forceRepo(t, "alice")
		got, exit := goldenRun(t, forceCLI, repo, "T-23", "--repo", repo)
		if exit != 0 {
			t.Fatalf("exit %d\n%s", exit, got)
		}
		checkGolden(t, "force_release_inspect", got)
		got, exit = goldenRun(t, forceCLI, repo, "T-99", "--repo", repo)
		if exit != 0 {
			t.Fatalf("no claim: exit %d\n%s", exit, got)
		}
		checkGolden(t, "force_release_no_claim", got)
	})

	t.Run("nectar", func(t *testing.T) {
		repo, _ := scoredRepo(t)
		var out, errb bytes.Buffer
		exit := runNectar([]string{"--repo", repo, "--now", nectarNow, "--no-fetch", "--json"}, &out, &errb)
		if exit != 0 && exit != nectarFinding {
			t.Fatalf("exit %d stderr %q", exit, errb.String())
		}
		checkGolden(t, "nectar", goldenForm(t, out.String(), repo))
	})
}
