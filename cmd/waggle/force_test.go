// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// forceRepo is dispatchRepo with gate.approvers committed and T-023 claimed by carol.
func forceRepo(t *testing.T, approvers string) string {
	t.Helper()
	repo := dispatchRepo(t)
	p := filepath.Join(repo, ".waggle/control/config.yaml")
	cfg, _ := os.ReadFile(p)
	if err := os.WriteFile(p, append(cfg, []byte("gate:\n  approvers: "+approvers+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "commit", "-q", "-am", "approvers")
	if _, exit := runJSON(t, claimCLI, "23", "--dirs", "internal/x", "--gate", "full", "--owner", "carol", "--repo", repo); exit != 0 {
		t.Fatal("claim failed")
	}
	return repo
}

func forceCLI(a []string, o, e *bytes.Buffer) int { return runForceRelease(a, o, e) }

func runForce(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	exit := runForceRelease(args, &out, &errb)
	return exit, out.String(), errb.String()
}

type forceResult struct {
	OK              bool     `json:"ok"`
	Command         string   `json:"command"`
	Task            string   `json:"task"`
	Outcome         string   `json:"outcome"`
	Ref             string   `json:"ref"`
	OID             string   `json:"oid"`
	Owner           string   `json:"owner"`
	Directories     []string `json:"directories"`
	MalformedReason string   `json:"malformed_reason"`
	Authorized      bool     `json:"authorized"`
	FirstApprover   string   `json:"first_approver"`
	Next            string   `json:"next"`
	AuditOID        string   `json:"audit_oid"`
	Warnings        []string `json:"warnings"`
	Error           struct {
		Code string `json:"code"`
		Exit int    `json:"exit"`
	} `json:"error"`
}

func forceJSON(t *testing.T, args ...string) (forceResult, int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	exit := runForceRelease(append(args, "--json"), &out, &errb)
	var res forceResult
	if strings.Count(out.String(), "\n") != 1 || json.Unmarshal(out.Bytes(), &res) != nil {
		t.Fatalf("stdout is not one JSON line: %q (stderr %q)", out.String(), errb.String())
	}
	return res, exit, out.String() + errb.String()
}

// A13: the JSON shape of each outcome; the reason is printed nowhere.
func TestForceReleaseCLIJSON(t *testing.T) {
	repo := forceRepo(t, "Alice")
	const reason = "carol left the project xyzzy"
	res, exit, all := forceJSON(t, "23", "--repo", repo)
	if exit != 0 || !res.OK || res.Command != "force-release" || res.Outcome != "inspected" || res.Task != "T-023" || res.Ref != "refs/waggle/claims/ticket-23" ||
		len(res.OID) != 40 || res.Owner != "carol" || strings.Join(res.Directories, ",") != "internal/x" || !res.Authorized || res.FirstApprover != "Alice" ||
		res.Next != "waggle force-release T-023 --oid "+res.OID+` --reason "<why>" --repo `+repo || res.Warnings == nil {
		t.Fatalf("inspect: exit %d %+v", exit, res)
	}
	oid := res.OID
	if res, exit, all = forceJSON(t, "23", "--oid", oid, "--reason", reason, "--owner", "bob", "--repo", repo); exit != 4 || res.OK || res.Error.Exit != 4 || strings.Contains(all, reason) {
		t.Fatalf("not authorized: exit %d %+v", exit, res)
	}
	if res, exit, all = forceJSON(t, "23", "--oid", oid, "--reason", reason, "--repo", repo); exit != 0 || res.Outcome != "released" || res.OID != oid ||
		res.Owner != "carol" || len(res.AuditOID) != 40 || res.Next != "" || strings.Contains(all, reason) {
		t.Fatalf("execute: exit %d %+v %q", exit, res, all)
	}
	if res, exit, _ = forceJSON(t, "23", "--repo", repo); exit != 0 || res.Outcome != "no_claim" || res.OID != "" {
		t.Fatalf("no claim: exit %d %+v", exit, res)
	}
	if res, exit, _ = forceJSON(t, "--ref", "refs/waggle/claims/ticket-23", "23", "--repo", repo); exit != 2 || res.Error.Code != "usage" {
		t.Fatalf("both targets: exit %d %+v", exit, res)
	}
}

// Text output for inspect and execute of a malformed claim, and the refusal line.
func TestForceReleaseCLIText(t *testing.T) {
	repo := forceRepo(t, "bob,alice")
	git(t, repo, "update-ref", "refs/waggle/claims/ticket-024", "HEAD")
	exit, out, errb := runForce("--ref", "refs/waggle/claims/ticket-024", "--repo", repo)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if exit != 0 || len(lines) != 5 || !strings.HasPrefix(lines[0], "ref: refs/waggle/claims/ticket-024") || !strings.HasPrefix(lines[2], "malformed: ref name is not canonical") ||
		!strings.HasPrefix(lines[3], "allowed: only bob (first login in gate.approvers at ") || !strings.HasSuffix(lines[3], "), not alice") ||
		!strings.HasPrefix(lines[4], "next: waggle force-release --ref refs/waggle/claims/ticket-024 --oid ") {
		t.Fatalf("inspect: exit %d stdout %q stderr %q", exit, out, errb)
	}
	oid := strings.TrimPrefix(lines[1], "oid: ")
	if exit, out, errb = runForce("--ref", "refs/waggle/claims/ticket-024", "--oid", oid, "--reason", "x", "--repo", repo); exit != 4 || out != "" ||
		!strings.Contains(errb, "is not bob, the first login in gate.approvers") {
		t.Fatalf("refused: exit %d stdout %q stderr %q", exit, out, errb)
	}
	exit, out, errb = runForce("--ref", "refs/waggle/claims/ticket-024", "--oid", oid, "--reason", "x", "--owner", "BOB", "--repo", repo)
	if exit != 0 || !strings.HasPrefix(out, "force-released refs/waggle/claims/ticket-024 (was "+oid[:12]+", malformed: ref name is not canonical") || !strings.Contains(out, "); audit ") {
		t.Fatalf("execute: exit %d stdout %q stderr %q", exit, out, errb)
	}
	exit, out, _ = runForce("23", "--owner", "bob", "--repo", repo)
	if exit != 0 || !strings.Contains(out, "owner: carol\ngate: full\ndirectories: internal/x\nexpires: ") || !strings.Contains(out, "\nallowed: you (bob), the first login in gate.approvers at ") ||
		!strings.Contains(out, "\nnext: waggle force-release T-023 --oid ") {
		t.Fatalf("valid claim: exit %d stdout %q", exit, out)
	}
	if exit, _, _ = runForce("--repo", repo); exit != 2 {
		t.Fatalf("no target: exit %d", exit)
	}
	if exit, _, _ = runForce("23", "extra", "--repo", repo); exit != 2 {
		t.Fatalf("extra argument: exit %d", exit)
	}
}

// A10: the flag set is pinned; there is no flag that skips the inspect step.
func TestForceReleaseFlagSet(t *testing.T) {
	fs, _ := newForceFlags(io.Discard)
	var names []string
	fs.VisitAll(func(f *flag.Flag) { names = append(names, f.Name) })
	sort.Strings(names)
	if got := strings.Join(names, ","); got != "json,oid,owner,reason,ref,repo" {
		t.Fatalf("flags = %s", got)
	}
	for _, bad := range []string{"yes", "y", "force", "f", "latest"} {
		if fs.Lookup(bad) != nil {
			t.Errorf("flag %s exists", bad)
		}
	}
	if exit, _, _ := runForce("23", "--yes"); exit != 2 {
		t.Errorf("--yes: exit %d", exit)
	}
}
