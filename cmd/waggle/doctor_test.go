// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/forge"
)

// fakeForge answers Probe with a canned report or error.
type fakeForge struct {
	forge.Forge
	report forge.Report
	err    error
	opt    forge.ProbeOptions
}

func (f *fakeForge) Probe(ctx context.Context, opt forge.ProbeOptions) (forge.Report, error) {
	f.opt = opt
	return f.report, f.err
}

// doctorRepo is a git repo with a gate-only style config.
func doctorRepo(t *testing.T, forgeYAML string) string {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	cfg := filepath.Join(dir, ".waggle", "control", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("version: 1\nforge:\n"+forgeYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// seamForge installs fakes for newForge and probeClaims for the test.
func seamForge(t *testing.T, f *fakeForge, claims forge.Finding) (calls *[]string) {
	t.Helper()
	var log []string
	oldNew, oldClaims := newForge, probeClaims
	t.Cleanup(func() { newForge, probeClaims = oldNew, oldClaims })
	newForge = func(provider string, opt forge.Options) (forge.Forge, error) {
		log = append(log, "new "+provider+" "+opt.Remote)
		if f == nil {
			return nil, errors.New("no auth")
		}
		return f, nil
	}
	probeClaims = func(ctx context.Context, dir, remote string, write bool, notice io.Writer) forge.Finding {
		log = append(log, "claims "+remote)
		if write {
			io.WriteString(notice, "NOTICE\n")
		}
		return claims
	}
	return &log
}

func runDoc(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = runDoctor(args, &out, &errb, func(string) string { return "" })
	return code, out.String(), errb.String()
}

var (
	okClaims = forge.Finding{ID: "claim-refs", Status: forge.StatusOK, Required: true, Summary: "Pushes work."}
	okGate   = forge.Finding{ID: "required-gate", Status: forge.StatusOK, Required: true, Summary: "Required."}
	okUpTo   = forge.Finding{ID: "up-to-date", Status: forge.StatusOK, Required: true, Summary: "Strict."}
	badUpTo  = forge.Finding{ID: "up-to-date", Status: forge.StatusMissing, Required: true, Summary: "Not strict.",
		NotEnforced: "a pull request can merge after main moved.", Fix: "turn on strict."}
)

func report(strategy string, fs ...forge.Finding) forge.Report {
	return forge.Report{Provider: "github", Repo: "o/r", Remote: "origin", Auth: "GITHUB_TOKEN",
		Claims: "refs/waggle/claims/* on origin", MergeStrategy: strategy, Findings: fs}
}

const githubCfg = "  provider: github\n  remote: origin\n"

func TestDoctorLocalOnly(t *testing.T) {
	dir := doctorRepo(t, "  provider: github\n")
	calls := seamForge(t, &fakeForge{}, okClaims)
	code, out, _ := runDoc("--repo", dir)
	want := "Local-only repo: no forge checks. After adding a remote and setting forge.remote, run waggle doctor.\n"
	if code != 0 || out != want || len(*calls) != 0 {
		t.Fatalf("got %d %q %q", code, out, *calls)
	}
}

func TestDoctorExitCodes(t *testing.T) {
	dir := doctorRepo(t, githubCfg)
	tests := []struct {
		name  string
		f     *fakeForge
		claim forge.Finding
		args  []string
		want  int
	}{
		{"all ok", &fakeForge{report: report("up_to_date", okGate, okUpTo)}, okClaims, nil, 0},
		{"missing required", &fakeForge{report: report("up_to_date", okGate, badUpTo)}, okClaims, nil, 1},
		{"claims unknown", &fakeForge{report: report("up_to_date", okGate, okUpTo)},
			forge.Finding{ID: "claim-refs", Status: forge.StatusUnknown, Required: true, Summary: "x", NotEnforced: "n", Fix: "f"}, nil, 1},
		{"claims skipped", &fakeForge{report: report("up_to_date", okGate, okUpTo)},
			forge.Finding{ID: "claim-refs", Status: forge.StatusSkipped, Required: true, Summary: "Not tested."}, []string{"--no-write"}, 0},
		{"usage", &fakeForge{}, okClaims, []string{"--bogus"}, 2},
		{"extra arg", &fakeForge{}, okClaims, []string{"extra"}, 2},
		{"probe error", &fakeForge{err: errors.New("default branch")}, okClaims, nil, 3},
		{"no forge", nil, okClaims, nil, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seamForge(t, tc.f, tc.claim)
			code, _, _ := runDoc(append([]string{"--repo", dir}, tc.args...)...)
			if code != tc.want {
				t.Errorf("exit %d, want %d", code, tc.want)
			}
		})
	}
	t.Run("no config", func(t *testing.T) {
		seamForge(t, &fakeForge{}, okClaims)
		requireGit(t)
		empty := t.TempDir()
		git(t, empty, "init", "-q")
		if code, _, _ := runDoc("--repo", empty); code != 3 {
			t.Errorf("exit %d, want 3", code)
		}
	})
}

func TestDoctorNotice(t *testing.T) {
	dir := doctorRepo(t, githubCfg)
	f := &fakeForge{report: report("up_to_date", okGate, okUpTo)}
	seamForge(t, f, okClaims)
	_, out, _ := runDoc("--repo", dir)
	if !strings.HasPrefix(out, "NOTICE\nWaggle setup check:") || !f.opt.Write {
		t.Errorf("write run: %q write=%v", out, f.opt.Write)
	}
	_, out, _ = runDoc("--repo", dir, "--no-write")
	if strings.Contains(out, "NOTICE") || f.opt.Write {
		t.Errorf("--no-write run: %q write=%v", out, f.opt.Write)
	}
	if f.opt.GateCheck != "verify-leases-and-tiers" {
		t.Errorf("gate check %q", f.opt.GateCheck)
	}
}

const mixedGolden = `Waggle setup check: github o/r (remote origin)
Auth: GITHUB_TOKEN
Claims: refs/waggle/claims/* on origin
Merge strategy: up_to_date
Enforcement: advisory (up-to-date is missing)

  ok        claim-refs      Pushes work.
  ok        required-gate   Required.
  missing   up-to-date      Not strict.
                            Not enforced: a pull request can merge after main moved.
                            Fix: turn on strict.

Not enforced on this repo (1):
  - A pull request can merge after main moved.
Waggle runs here, but it cannot stop the merges listed above.
`

func TestDoctorGoldenMixed(t *testing.T) {
	dir := doctorRepo(t, githubCfg)
	seamForge(t, &fakeForge{report: report("up_to_date", okGate, badUpTo)}, okClaims)
	_, out, _ := runDoc("--repo", dir, "--no-write")
	if out != mixedGolden {
		t.Errorf("got:\n%s\nwant:\n%s", out, mixedGolden)
	}
}

const queueGolden = `Waggle setup check: github o/r (remote origin)
Auth: GITHUB_TOKEN
Claims: refs/waggle/claims/* on origin
Merge strategy: native_queue
Enforcement: advisory (merge-queue is unknown)

  ok        claim-refs      Pushes work.
  ok        required-gate   Required.
  skipped   up-to-date      Not used: merge_strategy is native_queue.
  unknown   merge-queue     This build does not check the merge queue rule.
                            Not enforced: this build does not check the merge queue rule.
                            Fix: check that a merge queue rule is active on main.

Not enforced on this repo (1):
  - This build does not check the merge queue rule.
Waggle runs here, but it cannot stop the merges listed above.
`

func TestDoctorGoldenNativeQueue(t *testing.T) {
	dir := doctorRepo(t, githubCfg+"  merge_strategy: native_queue\n")
	queue := []forge.Finding{okGate,
		{ID: "up-to-date", Status: forge.StatusSkipped, Summary: "Not used: merge_strategy is native_queue."},
		{ID: "merge-queue", Status: forge.StatusUnknown, Summary: "This build does not check the merge queue rule.",
			NotEnforced: "this build does not check the merge queue rule.", Fix: "check that a merge queue rule is active on main."}}
	seamForge(t, &fakeForge{report: report("native_queue", queue...)}, okClaims)
	code, out, _ := runDoc("--repo", dir, "--no-write")
	if code != 0 || out != queueGolden || strings.Contains(out, "found missing") {
		t.Errorf("exit %d, got:\n%s", code, out)
	}
}
