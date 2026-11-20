// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitStdout(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(append(os.Environ(), "LC_ALL=C"), gitEnv...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func commitGateFile(t *testing.T, repo, rel, msg string) string {
	t.Helper()
	p := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("// " + msg + "\n")
	f.Close()
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", msg)
	return gitStdout(t, repo, "rev-parse", "HEAD")
}

// gateReceiptRepo is a hive whose origin is a local bare repository, with live claims
// on T-023 and T-024 and none on T-025. It returns the repo, the bare origin and
// main's head before the pushes under test.
func gateReceiptRepo(t *testing.T) (repo, origin, before string) {
	t.Helper()
	repo = dispatchRepo(t)
	origin = filepath.Join(t.TempDir(), "origin.git")
	git(t, repo, "init", "-q", "--bare", origin)
	p := filepath.Join(repo, ".waggle/control/config.yaml")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "forge:\n", "forge:\n  remote: origin\n", 1))
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "fixture: remote")
	git(t, repo, "remote", "add", "origin", origin)
	git(t, repo, "push", "-q", "origin", "main")
	for _, c := range [][]string{{"23", "internal/x"}, {"24", "internal/y"}} {
		if res, exit := runJSON(t, claimCLI, c[0], "--dirs", c[1], "--gate", "full", "--owner", "bob", "--repo", repo); exit != 0 {
			t.Fatalf("claim %s: %+v", c[0], res)
		}
	}
	return repo, origin, gitStdout(t, repo, "rev-parse", "HEAD")
}

type gateReceiptOut struct {
	OK      bool `json:"ok"`
	Written int  `json:"written"`
	Items   []struct {
		Task      string `json:"task"`
		MergedSHA string `json:"merged_sha"`
		State     string `json:"state"`
		Proof     string `json:"merge_proof"`
	} `json:"items"`
}

func runGateReceipt(t *testing.T, repo, before, after string, extra ...string) (gateReceiptOut, int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	args := append([]string{"gate-receipt", "--before", before, "--after", after, "--actor", "push-actor", "--repo", repo, "--json"}, extra...)
	exit := runNectar(args, &out, &errb)
	var res gateReceiptOut
	if exit == 0 {
		if err := json.Unmarshal(out.Bytes(), &res); err != nil || !res.OK {
			t.Fatalf("json %q: %v", out.String(), err)
		}
	}
	return res, exit, errb.String()
}

// D1: one receipt for each claimed task that landed in the push, written once
// and atomically to the origin; an unclaimed task stays unattributed; a rerun
// writes nothing; a rebase run is named by its last commit.
func TestNectarGateReceipt(t *testing.T) {
	repo, origin, before := gateReceiptRepo(t)
	t.Setenv("GITHUB_TOKEN", "")
	commitGateFile(t, repo, "internal/x/s.go", "T-023: squash")
	commitGateFile(t, repo, "internal/y/a.go", "T-024: first of a run")
	last24 := commitGateFile(t, repo, "internal/y/b.go", "T-024: second of a run")
	commitGateFile(t, repo, "internal/z/c.go", "T-025: no claim")
	commitGateFile(t, repo, "internal/w/d.go", "Head Keeper: binds none")
	after := commitGateFile(t, repo, "README.md", "unbound")
	git(t, repo, "push", "-q", "origin", "main")
	receipts := func() string { return gitStdout(t, origin, "for-each-ref", "refs/waggle/receipts/") }
	if receipts() != "" {
		t.Fatal("receipt ref exists before the run")
	}

	res, exit, stderr := runGateReceipt(t, repo, before, after)
	if exit != 0 || res.Written != 2 || len(res.Items) != 3 {
		t.Fatalf("exit %d res %+v stderr %q", exit, res, stderr)
	}
	states := map[string]string{}
	for _, it := range res.Items {
		states[it.Task] = it.State
		if it.Task == "T-024" && it.MergedSHA != last24 {
			t.Errorf("T-024 merged_sha %s, want the last commit of its run %s", it.MergedSHA, last24)
		}
	}
	if states["T-023"] != "written" || states["T-024"] != "written" || states["T-025"] != "no_claim" {
		t.Errorf("states %v", states)
	}
	head := gitStdout(t, origin, "rev-parse", "refs/waggle/receipts/main")
	var raw map[string]any
	if err := json.Unmarshal([]byte(gitStdout(t, origin, "show", head+":receipt.json")), &raw); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]any{"source": "gate_run", "ticket_id": "T-024", "keeper_id": "bob", "released_by": "push-actor",
		"merge_proof": "binding", "status": "merged", "merged_sha": last24, "incomplete": true, "pr_number": nil} {
		if raw[k] != want {
			t.Errorf("receipt %s = %v, want %v", k, raw[k], want)
		}
	}
	if parents := strings.Fields(gitStdout(t, origin, "rev-list", "--parents", "-n", "1", head)); len(parents) != 3 {
		t.Errorf("receipt commit has %d parents, want the previous head and the claim commit", len(parents)-1)
	}
	// The claim itself is untouched.
	if refs := gitStdout(t, origin, "for-each-ref", "--format=%(refname)", "refs/waggle/claims/"); !strings.Contains(refs, "ticket-23") || !strings.Contains(refs, "ticket-24") {
		t.Errorf("claim refs %q", refs)
	}

	// Idempotent: a rerun, and a rerun over a wider range, write nothing.
	tip := receipts()
	for _, b := range []string{before, strings.Repeat("0", 40)} {
		res, exit, stderr = runGateReceipt(t, repo, b, after)
		if exit != 0 || res.Written != 0 || receipts() != tip {
			t.Errorf("rerun from %s: exit %d res %+v stderr %q", b[:7], exit, res, stderr)
		}
	}
}

// A release that landed first wins: the task already has a receipt, so the
// gate run leaves it alone.
func TestNectarGateReceiptAfterRelease(t *testing.T) {
	repo, origin, before := gateReceiptRepo(t)
	t.Setenv("GITHUB_TOKEN", "")
	after := commitGateFile(t, repo, "internal/x/s.go", "T-023: squash")
	git(t, repo, "push", "-q", "origin", "main")
	if res, exit := runJSON(t, releaseCLI, "23", "--unmerged", "--owner", "bob", "--repo", repo); exit != 0 {
		t.Fatalf("release: %+v", res)
	}
	tip := gitStdout(t, origin, "rev-parse", "refs/waggle/receipts/main")
	res, exit, stderr := runGateReceipt(t, repo, before, after)
	if exit != 0 || res.Written != 0 || len(res.Items) != 1 || res.Items[0].State != "present" || gitStdout(t, origin, "rev-parse", "refs/waggle/receipts/main") != tip {
		t.Errorf("exit %d res %+v stderr %q", exit, res, stderr)
	}
}

// With no live claim and with nothing bound, no receipt ref is created.
func TestNectarGateReceiptNothingToWrite(t *testing.T) {
	repo, origin, before := gateReceiptRepo(t)
	after := commitGateFile(t, repo, "internal/z/c.go", "T-025: no claim")
	res, exit, stderr := runGateReceipt(t, repo, before, after)
	if exit != 0 || res.Written != 0 || len(res.Items) != 1 || res.Items[0].State != "no_claim" || gitStdout(t, origin, "for-each-ref", "refs/waggle/receipts/") != "" {
		t.Errorf("exit %d res %+v stderr %q", exit, res, stderr)
	}
}

func TestNectarGateReceiptUsage(t *testing.T) {
	var out, errb bytes.Buffer
	sha := strings.Repeat("a", 40)
	for _, args := range [][]string{
		{"gate-receipt"},
		{"gate-receipt", "--before", sha},
		{"gate-receipt", "--before", "main", "--after", sha, "--actor", "x"},
		{"gate-receipt", "--before", sha, "--after", "HEAD", "--actor", "x"},
		{"gate-receipt", "--before", sha, "--after", sha, "--actor", "bad[bot]"},
		{"gate-receipt", "--before", sha, "--after", sha, "--actor", "x", "extra"},
	} {
		out.Reset()
		errb.Reset()
		if exit := runNectar(args, &out, &errb); exit != 2 {
			t.Errorf("%v: exit %d, stderr %q", args, exit, errb.String())
		}
	}
	errb.Reset()
	if exit := runNectar([]string{"gate-receipt", "-h"}, &out, &errb); exit != 0 {
		t.Errorf("-h exit %d", exit)
	}
	checkPlain(t, "nectar gate-receipt -h", errb.String(), "-before", "-after", "refs/waggle/receipts/*")
}
