// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
)

// countsRun runs verify with --counts and returns the exit code, the combined
// output and the file text.
func countsRun(t *testing.T, r gateRepo) (int, string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "counts.json")
	code, out := r.runWith(nil, "verify", "--counts", path)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no counts file (exit %d): %v\n%s", code, err, out)
	}
	return code, out, string(b)
}

const zeros = `{"binding":0,"decisions":0,"leases":0,"tiers":0}`

func countsJSON(failed, would string) string {
	return `{"failed":` + failed + `,"schema":1,"would_fail":` + would + "}\n"
}

// twoLease adds two paths outside every lease, a Tier 3 path and a bad log.
func twoLease(r gateRepo) {
	r.write("other/y.md", []byte("y\n"))
	r.write("other/z.md", []byte("z\n"))
	r.write(".github/workflows/x.yml", []byte("x\n"))
	r.write(layout.Decisions, []byte("not a log\n"))
	r.commit("T-031: mixed")
}

func TestCountsFailedGuardAndHold(t *testing.T) {
	for _, cfg := range []string{guardCfg, holdCfg, ""} {
		r := protRepo(t, cfg)
		twoLease(r)
		code, _, got := countsRun(t, r)
		want := countsJSON(`{"binding":0,"decisions":1,"leases":4,"tiers":1}`, zeros)
		if code != 1 || got != want {
			t.Errorf("cfg %q: exit %d, got %q want %q", cfg, code, got, want)
		}
	}
}

func TestCountsWouldFailOpen(t *testing.T) {
	r := protRepo(t, openCfg)
	twoLease(r)
	code, _, got := countsRun(t, r)
	want := countsJSON(zeros, `{"binding":0,"decisions":1,"leases":4,"tiers":1}`)
	if code != 0 || got != want {
		t.Errorf("exit %d, got %q want %q", code, got, want)
	}
}

func TestCountsPassIsZeros(t *testing.T) {
	for _, cfg := range []string{guardCfg, openCfg} {
		r := protRepo(t, cfg)
		r.write("docs/a.md", []byte("a\n"))
		r.commit("T-031: add")
		code, _, got := countsRun(t, r)
		if code != 0 || got != countsJSON(zeros, zeros) {
			t.Errorf("cfg %q: exit %d, got %q", cfg, code, got)
		}
	}
}

func TestCountsLeaveNoFixtureStrings(t *testing.T) {
	r := protRepo(t, guardCfg)
	twoLease(r)
	_, _, got := countsRun(t, r)
	for _, s := range []string{r.dir, r.git("rev-parse", "HEAD"), "other", "y.md", "workflows", "branch", "T-031", "alice", "t@example.com"} {
		if strings.Contains(got, s) {
			t.Errorf("counts file contains %q: %s", s, got)
		}
	}
}

func TestCountsWarningsNotCounted(t *testing.T) {
	rep := &gateReport{}
	rep.warn(checkLeases, "branch name ignored")
	rep.warn(checkTiers, "note")
	rep.fail(checkLeases, "a", "m")
	got := countGate(rep.findings, false, false)
	if got.Failed.Leases != 1 || got.Failed.Tiers != 0 || got.WouldFail != (checkCounts{}) {
		t.Errorf("got %+v", got)
	}
}

func TestCountsNoFlagNoFile(t *testing.T) {
	r := protRepo(t, guardCfg)
	twoLease(r)
	dir := t.TempDir()
	// The module is go1.23, so t.Chdir (go1.24) is not available.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Error(err)
		}
	})
	r.runWith(nil, "verify")
	if ents, _ := os.ReadDir(dir); len(ents) != 0 {
		t.Errorf("verify without --counts wrote %d files", len(ents))
	}
	if code, _ := r.runWith(nil, "leases", "--counts", filepath.Join(dir, "c.json")); code != gateUsage {
		t.Errorf("leases accepted --counts: exit %d", code)
	}
	if code, _ := r.runWith(nil, "tiers", "--counts", filepath.Join(dir, "c.json")); code != gateUsage {
		t.Errorf("tiers accepted --counts: exit %d", code)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 0 {
		t.Errorf("a usage error wrote %d files", len(ents))
	}
}

func TestCountsOutputUnchanged(t *testing.T) {
	for _, cfg := range []string{guardCfg, openCfg} {
		for _, gha := range []bool{false, true} {
			r := protRepo(t, cfg)
			twoLease(r)
			getenv := func(string) string { return "" }
			if gha {
				getenv = func(k string) string {
					if k == "GITHUB_ACTIONS" {
						return "true"
					}
					return ""
				}
			}
			run := func(extra ...string) (int, string, string) {
				args := append([]string{"verify", "--base", "main", "--head", "branch", "--repo", r.dir}, extra...)
				return gate(getenv, args...)
			}
			c1, o1, e1 := run()
			c2, o2, e2 := run("--counts", filepath.Join(t.TempDir(), "c.json"))
			if c1 != c2 || o1 != o2 || e1 != e2 {
				t.Errorf("cfg %q gha %v: output differs: %d %q %q vs %d %q %q", cfg, gha, c1, o1, e1, c2, o2, e2)
			}
		}
	}
	// A passing run prints its three ok lines, with the flag too.
	p := protRepo(t, guardCfg)
	p.write("docs/a.md", []byte("a\n"))
	p.commit("T-031: add")
	_, out, _ := gate(nil, "verify", "--base", "main", "--head", "branch", "--repo", p.dir, "--counts", filepath.Join(t.TempDir(), "c.json"))
	if out != "leases: ok\ntiers: ok\ndecisions: unchanged\n" {
		t.Errorf("stdout with --counts: %q", out)
	}
}

func TestCountsWriteFailure(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "missing", "c.json")
	r := protRepo(t, guardCfg)
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-031: add")
	code, _, errs := gate(nil, "verify", "--base", "main", "--head", "branch", "--repo", r.dir, "--counts", bad)
	if code != gatePass || strings.Count(errs, "gate: warning: cannot write counts file: ") != 1 {
		t.Errorf("pass: exit %d stderr %q", code, errs)
	}
	r2 := protRepo(t, guardCfg)
	twoLease(r2)
	code, _, errs = gate(nil, "verify", "--base", "main", "--head", "branch", "--repo", r2.dir, "--counts", bad)
	if code != gateFail || strings.Count(errs, "gate: warning: cannot write counts file: ") != 1 || !strings.Contains(errs, "leases: FAIL") {
		t.Errorf("fail: exit %d stderr %q", code, errs)
	}
}

func TestCountsRenameFailureLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.json")
	if err := os.MkdirAll(filepath.Join(path, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeGateCounts(path, gateCounts{Schema: gateCountsSchema}); err == nil {
		t.Fatal("rename onto a non-empty directory succeeded")
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".waggle-gate-counts-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestCountsFileMode(t *testing.T) {
	r := protRepo(t, guardCfg)
	path := filepath.Join(t.TempDir(), "c.json")
	r.runWith(nil, "verify", "--counts", path)
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0o644 {
		t.Errorf("stat %v mode %v", err, st)
	}
	if ents, _ := os.ReadDir(filepath.Dir(path)); len(ents) != 1 {
		t.Errorf("temp file left behind: %d entries", len(ents))
	}
}

func TestCountsGateError(t *testing.T) {
	r := protRepo(t, guardCfg)
	path := filepath.Join(t.TempDir(), "c.json")
	code, _, _ := gate(nil, "verify", "--base", "main", "--head", "nosuchref", "--repo", r.dir, "--counts", path)
	_, err := os.Stat(path)
	if code != gateError || !os.IsNotExist(err) {
		t.Errorf("exit %d stat err %v: want exit 3 and no counts file", code, err)
	}
}
