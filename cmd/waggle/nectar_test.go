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

func nectarBackfillCLI(a []string, o, e *bytes.Buffer) int {
	return runNectar(append([]string{"backfill"}, a...), o, e)
}

// A10: nectar backfill -h is plain, and a bad command line is a usage error.
func TestNectarBackfillHelpAndUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if exit := runNectar([]string{"backfill", "-h"}, &out, &errb); exit != 0 {
		t.Errorf("-h exit %d", exit)
	}
	checkPlain(t, "nectar backfill -h", errb.String(), "-apply", "-owner", "Without --apply it prints the plan and writes nothing.")
	for _, args := range [][]string{nil, {"score"}, {"backfill", "extra"}, {"backfill", "--bogus"}} {
		out.Reset()
		errb.Reset()
		if exit := runNectar(args, &out, &errb); exit != 2 {
			t.Errorf("%v: exit %d, stderr %q", args, exit, errb.String())
		}
	}
	errb.Reset()
	if exit := runNectar([]string{"-h"}, &out, &errb); exit != 0 || !strings.Contains(errb.String(), "waggle nectar backfill") {
		t.Errorf("nectar -h: exit %d %q", exit, errb.String())
	}
}

// A8 through the command: the dry run writes nothing, --apply writes the record
// once, and --json prints one object.
func TestNectarBackfillCLI(t *testing.T) {
	repo := dispatchRepo(t)
	if _, exit := runJSON(t, claimCLI, "23", "--dirs", "internal/x", "--gate", "full", "--owner", "bob", "--repo", repo); exit != 0 {
		t.Fatal("claim failed")
	}
	if err := os.WriteFile(filepath.Join(repo, "internal/x/s.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "T-023: squash")
	receipts := func() string {
		out, err := exec.Command("git", "-C", repo, "for-each-ref", "refs/waggle/receipts/").Output()
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}

	var out, errb bytes.Buffer
	if exit := runNectar([]string{"backfill", "--repo", repo}, &out, &errb); exit != 0 ||
		!strings.Contains(out.String(), "T-023 planned: owner bob from live claim") || !strings.Contains(out.String(), "nothing written") || receipts() != "" {
		t.Fatalf("dry run: exit %d stdout %q stderr %q receipts %q", exit, out.String(), errb.String(), receipts())
	}
	out.Reset()
	if exit := runNectar([]string{"backfill", "--apply", "--json", "--repo", repo}, &out, &errb); exit != 0 {
		t.Fatalf("apply: exit %d stdout %q stderr %q", exit, out.String(), errb.String())
	}
	var res struct {
		OK      bool `json:"ok"`
		Applied bool `json:"applied"`
		Written int  `json:"written"`
		Items   []struct {
			Task      string
			State     string
			Owner     *string
			MergedSHA *string `json:"merged_sha"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil || !res.OK || !res.Applied || res.Written != 1 || len(res.Items) != 1 ||
		res.Items[0].State != "written" || res.Items[0].Owner == nil || *res.Items[0].Owner != "bob" || res.Items[0].MergedSHA == nil || receipts() == "" {
		t.Fatalf("apply json %q: %v", out.String(), err)
	}
	out.Reset()
	if exit := runNectar([]string{"backfill", "--apply", "--repo", repo}, &out, &errb); exit != 0 || !strings.Contains(out.String(), "wrote 0 record(s); 0 task(s) unattributed") {
		t.Errorf("second apply: exit %d %q", exit, out.String())
	}
	if res, exit := runJSON(t, nectarBackfillCLI, "--owner", "bad owner", "--repo", repo); exit != 2 || res.Error.Code != "usage" {
		t.Errorf("bad owner: exit %d %+v", exit, res)
	}
}
