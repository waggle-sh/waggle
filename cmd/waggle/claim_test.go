//go:build unix

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type cliResult struct {
	OK      bool   `json:"ok"`
	Command string `json:"command"`
	Task    string `json:"task"`
	Outcome string `json:"outcome"`
	Claim   struct {
		Task        string   `json:"task"`
		Owner       string   `json:"owner"`
		Gate        string   `json:"gate"`
		Directories []string `json:"directories"`
		ClaimedAt   string   `json:"claimed_at"`
		ExpiresAt   string   `json:"expires_at"`
		Ref         string   `json:"ref"`
		OID         string   `json:"oid"`
	} `json:"claim"`
	OID    string `json:"oid"`
	Merged bool   `json:"merged"`
	Branch string `json:"branch"`
	Error  struct {
		Code    string `json:"code"`
		Exit    int    `json:"exit"`
		Message string `json:"message"`
	} `json:"error"`
}

// runJSON runs fn with --json and decodes the one object on stdout.
func runJSON(t *testing.T, fn func([]string, *bytes.Buffer, *bytes.Buffer) int, args ...string) (res cliResult, exit int) {
	t.Helper()
	var out, errb bytes.Buffer
	exit = fn(append(args, "--json"), &out, &errb)
	if strings.Count(out.String(), "\n") != 1 || !strings.HasSuffix(out.String(), "}\n") {
		t.Fatalf("stdout is not one JSON line: %q (stderr %q)", out.String(), errb.String())
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("stdout is not JSON: %v: %q", err, out.String())
	}
	if !res.OK && res.Error.Exit != exit {
		t.Fatalf("error.exit %d, process exit %d", res.Error.Exit, exit)
	}
	return res, exit
}

func claimCLI(a []string, o, e *bytes.Buffer) int { return runClaim(a, o, e) }
func releaseCLI(a []string, o, e *bytes.Buffer) int {
	return runTaskRelease(a, o, e)
}

// AC3 and AC4 through the verb: JSON for success and for every failure after flag parsing.
func TestClaimCLI(t *testing.T) {
	repo := dispatchRepo(t)
	res, exit := runJSON(t, claimCLI, "T-32", "--dirs", "internal/y,internal/x", "--gate", "full", "--owner", "alice", "--repo", repo)
	claimed, err1 := time.Parse(time.RFC3339, res.Claim.ClaimedAt)
	expires, err2 := time.Parse(time.RFC3339, res.Claim.ExpiresAt)
	if exit != 0 || !res.OK || res.Command != "claim" || res.Task != "T-032" || res.Outcome != "claimed" || res.Claim.Owner != "alice" || res.Claim.Gate != "full" ||
		strings.Join(res.Claim.Directories, ",") != "internal/x,internal/y" || res.Claim.Ref != "refs/waggle/claims/ticket-32" || len(res.Claim.OID) != 40 ||
		err1 != nil || err2 != nil || expires.Sub(claimed) <= 0 {
		t.Fatalf("claim: exit %d %+v", exit, res)
	}
	def := expires.Sub(claimed)
	if res, exit = runJSON(t, claimCLI, "32", "--dirs", "internal/x", "--gate", "full", "--owner", "alice", "--ttl", "90m", "--repo", repo); exit != 0 || res.Outcome != "renewed" {
		t.Fatalf("renew: exit %d %+v", exit, res)
	}
	claimed, _ = time.Parse(time.RFC3339, res.Claim.ClaimedAt)
	if expires, _ = time.Parse(time.RFC3339, res.Claim.ExpiresAt); expires.Sub(claimed) != 90*time.Minute || def == 90*time.Minute {
		t.Fatalf("ttl: %v (default %v)", expires.Sub(claimed), def)
	}
	if res, exit = runJSON(t, claimCLI, "T-32", "--dirs", "internal/x", "--gate", "full", "--owner", "bob", "--repo", repo); exit != 4 || res.OK || res.Error.Code != "already_claimed" || res.Task != "T-032" {
		t.Fatalf("other owner: exit %d %+v", exit, res)
	}
	if res, exit = runJSON(t, claimCLI, "T-33", "--dirs", "internal/x/sub", "--gate", "full", "--owner", "bob", "--repo", repo); exit != 4 || res.Error.Code != "overlap" {
		t.Fatalf("overlap: exit %d %+v", exit, res)
	}
	if res, exit = runJSON(t, claimCLI, "T-33", "--dirs", "vendor", "--gate", "full", "--owner", "bob", "--repo", repo); exit != 4 || res.Error.Code != "protected_path" {
		t.Fatalf("vendor: exit %d %+v", exit, res)
	}
	if res, exit = runJSON(t, claimCLI, "T-33", "--dirs", "CLAUDE.md", "--gate", "light", "--owner", "bob", "--repo", repo); exit != 4 || res.Error.Code != "gate_too_low" {
		t.Fatalf("light gate: exit %d %+v", exit, res)
	}
	usage := map[string][]string{
		"ttl 0s":         {"T-34", "--dirs", "docs", "--gate", "full", "--ttl", "0s"},
		"ttl 30s":        {"T-34", "--dirs", "docs", "--gate", "full", "--ttl", "30s"},
		"ttl 169h":       {"T-34", "--dirs", "docs", "--gate", "full", "--ttl", "169h"},
		"ttl 90m30s":     {"T-34", "--dirs", "docs", "--gate", "full", "--ttl", "90m30s"},
		"empty entry":    {"T-34", "--dirs", "a,,b", "--gate", "full"},
		"trailing comma": {"T-34", "--dirs", "a,", "--gate", "full"},
		"no dirs":        {"T-34", "--gate", "full"},
		"no gate":        {"T-34", "--dirs", "docs"},
		"no task":        {"--dirs", "docs", "--gate", "full"},
	}
	for name, args := range usage {
		if res, exit = runJSON(t, claimCLI, append(args, "--owner", "alice", "--repo", repo)...); exit != 2 || res.Error.Code != "usage" {
			t.Errorf("%s: exit %d %+v", name, exit, res)
		}
	}
	if res, exit = runJSON(t, claimCLI, "x", "--dirs", "docs", "--gate", "full", "--owner", "alice", "--repo", repo); exit != 2 || res.Error.Code != "invalid_task" || res.Task != "" {
		t.Fatalf("bad task: exit %d %+v", exit, res)
	}
	if err := os.Remove(filepath.Join(repo, ".waggle/control/local.yaml")); err != nil {
		t.Fatal(err)
	}
	if res, exit = runJSON(t, claimCLI, "T-34", "--dirs", "docs", "--gate", "full", "--repo", repo); exit != 2 || !strings.Contains(res.Error.Message, "--owner") {
		t.Fatalf("no owner: exit %d %+v", exit, res)
	}
	// A flag-parse failure cannot know --json: usage on stderr, nothing on stdout.
	var out, errb bytes.Buffer
	if exit = runClaim([]string{"T-34", "--bogus", "--json"}, &out, &errb); exit != 2 || out.Len() != 0 || !strings.Contains(errb.String(), "Usage: waggle claim") {
		t.Fatalf("unknown flag: exit %d stdout %q stderr %q", exit, out.String(), errb.String())
	}
}

// Text mode: one line, notes after it, no JSON.
func TestClaimCLIText(t *testing.T) {
	repo := dispatchRepo(t)
	var out, errb bytes.Buffer
	if exit := runClaim([]string{"32", "--dirs", "internal/y,internal/x", "--gate", "full", "--repo", repo}, &out, &errb); exit != 0 ||
		!strings.HasPrefix(out.String(), "claimed T-032 for alice until ") || !strings.Contains(out.String(), ": internal/x, internal/y (gate full, claim ") || errb.Len() != 0 {
		t.Fatalf("exit %d stdout %q stderr %q", exit, out.String(), errb.String())
	}
	out.Reset()
	if exit := runClaim([]string{"32", "--dirs", "internal/x", "--gate", "full", "--repo", repo}, &out, &errb); exit != 0 ||
		!strings.HasPrefix(out.String(), "renewed T-032 for alice") || !strings.Contains(out.String(), "\nnote: renewal changes the claim") {
		t.Fatalf("renew: exit %d stdout %q", exit, out.String())
	}
	out.Reset()
	if exit := runClaim([]string{"--dirs", "docs", "--gate", "full", "T-34", "T-35", "--repo", repo}, &out, &errb); exit != 2 || out.Len() != 0 {
		t.Fatalf("two tasks: exit %d stdout %q", exit, out.String())
	}
	if exit := runClaim([]string{"33", "--dirs", "vendor", "--gate", "full", "--repo", repo}, &out, &errb); exit != 4 || out.Len() != 0 || !strings.HasPrefix(errb.String(), "waggle claim: ") {
		t.Fatalf("refused: exit %d stdout %q stderr %q", exit, out.String(), errb.String())
	}
}

// AC7 through the verb.
func TestReleaseCLI(t *testing.T) {
	repo := dispatchRepo(t)
	if res, exit := runJSON(t, releaseCLI, "23", "--repo", repo); exit != 0 || res.Outcome != "no_claim" || res.OID != "" || res.Task != "T-023" || res.Command != "release" {
		t.Fatalf("no claim: exit %d %+v", exit, res)
	}
	if _, exit := runJSON(t, claimCLI, "23", "--dirs", "internal/x", "--gate", "full", "--repo", repo); exit != 0 {
		t.Fatal("claim failed")
	}
	if res, exit := runJSON(t, releaseCLI, "23", "--owner", "bob", "--unmerged", "--repo", repo); exit != 4 || res.Error.Code != "not_owner" {
		t.Fatalf("other owner: exit %d %+v", exit, res)
	}
	if res, exit := runJSON(t, releaseCLI, "23", "--repo", repo); exit != 4 || res.Error.Code != "not_merged" {
		t.Fatalf("unmerged: exit %d %+v", exit, res)
	}
	for _, b := range []string{"waggle/T-024", "feature/x"} {
		if res, exit := runJSON(t, releaseCLI, "23", "--branch", b, "--repo", repo); exit != 2 || res.Error.Code != "usage" {
			t.Fatalf("branch %s: exit %d %+v", b, exit, res)
		}
	}
	var out, errb bytes.Buffer
	if exit := runTaskRelease([]string{"23", "--unmerged", "--repo", repo}, &out, &errb); exit != 0 || !strings.HasPrefix(out.String(), "released T-023 (claim ") || !strings.HasSuffix(out.String(), ", unmerged)\n") {
		t.Fatalf("text: exit %d stdout %q stderr %q", exit, out.String(), errb.String())
	}
	if exit := runTaskRelease([]string{"23", "--bogus"}, &out, &errb); exit != 2 {
		t.Fatalf("unknown flag: exit %d", exit)
	}
}

// AC11 and AC12: the task may come before, between or after flags; usage errors carry it.
func TestTaskArgPosition(t *testing.T) {
	repo := dispatchRepo(t)
	for i, args := range [][]string{
		{"--json", "T-32", "--dirs", "internal/x", "--gate", "full", "--owner", "alice", "--repo", repo},
		{"--dirs", "internal/x", "T-32", "--gate", "full", "--owner", "alice", "--repo", repo},
		{"--dirs", "internal/x", "--gate", "full", "--owner", "alice", "--repo", repo, "T-32"},
	} {
		var out, errb bytes.Buffer
		if exit := runClaim(append([]string{"--json"}, args...), &out, &errb); exit != 0 || !strings.Contains(out.String(), `"task":"T-032"`) {
			t.Fatalf("claim %d: exit %d stdout %q stderr %q", i, exit, out.String(), errb.String())
		}
	}
	if res, exit := runJSON(t, releaseCLI, "--unmerged", "23", "--repo", repo); exit != 0 || res.Command != "release" {
		t.Fatalf("release: exit %d %+v", exit, res)
	}
	var out, errb bytes.Buffer
	if exit := runClaim([]string{"--dirs", "a", "T-32", "-h"}, &out, &errb); exit != 0 || !strings.Contains(errb.String(), "Usage: waggle claim") {
		t.Fatalf("help: exit %d stderr %q", exit, errb.String())
	}
	out.Reset()
	if exit := runClaim([]string{"--dirs", "docs", "T-34", "--gate", "full", "T-35", "--repo", repo}, &out, &errb); exit != 2 || out.Len() != 0 {
		t.Fatalf("two tasks: exit %d stdout %q", exit, out.String())
	}
	if exit := runClaim([]string{"T-34", "--json", "--dirs", "a", "--bogus"}, &out, &errb); exit != 2 || out.Len() != 0 {
		t.Fatalf("unknown flag after task: exit %d stdout %q", exit, out.String())
	}
	if res, exit := runJSON(t, claimCLI, "T-34", "--dirs", "a,,b", "--gate", "full", "--repo", repo); exit != 2 || res.Error.Code != "usage" || res.Task != "T-034" {
		t.Fatalf("dirs: exit %d %+v", exit, res)
	}
	if res, exit := runJSON(t, claimCLI, "--dirs", "docs", "--gate", "full", "--repo", repo); exit != 2 || res.Task != "" {
		t.Fatalf("no task: exit %d %+v", exit, res)
	}
	if res, exit := runJSON(t, releaseCLI, "--json", "--repo", repo, "23", "24"); exit != 2 || res.Error.Code != "usage" || res.Task != "T-023" {
		t.Fatalf("two release tasks: exit %d %+v", exit, res)
	}
}

// AC10: values at the validator's limits and hostile error text stay on one line.
func TestTaskOutputHostileText(t *testing.T) {
	repo := dispatchRepo(t)
	seg := strings.Repeat("d", 255)
	dir := strings.Join([]string{seg, seg, seg, seg[:253]}, "/") // 1023 bytes
	owner := "o" + strings.Repeat("w", 127)
	res, exit := runJSON(t, claimCLI, "T-40", "--dirs", dir, "--gate", "full", "--owner", owner, "--repo", repo)
	if exit != 0 || res.Claim.Owner != owner || len(res.Claim.Directories) != 1 || res.Claim.Directories[0] != dir {
		t.Fatalf("limits: exit %d", exit)
	}
	var out, errb bytes.Buffer
	if exit = runClaim([]string{"T-41", "--dirs", dir + "/x", "--gate", "full", "--owner", owner, "--repo", repo}, &out, &errb); exit == 0 {
		// 1025 bytes is over the limit, so the verb refuses it; text mode still has no control bytes.
		t.Fatalf("over the limit was accepted: %q", out.String())
	}
	if exit = runClaim([]string{"T-40", "--dirs", dir, "--gate", "full", "--owner", owner, "--repo", repo}, &out, &errb); exit != 0 {
		t.Fatalf("renew at the limits: %d %q", exit, errb.String())
	}
	hostile := errors.New("bad\r\nvalue\x1b[31m end")
	for _, asJSON := range []bool{true, false} {
		var o, e bytes.Buffer
		taskOut{"claim", asJSON, &o, &e}.fail("T-040", hostile)
		if asJSON {
			var got cliResult
			if err := json.Unmarshal(o.Bytes(), &got); err != nil || got.Error.Message != hostile.Error() || strings.Count(o.String(), "\n") != 1 {
				t.Fatalf("json: %v %q", err, o.String())
			}
		}
		for _, s := range []string{e.String(), o.String()} {
			if strings.ContainsAny(strings.TrimSuffix(s, "\n"), "\r\n\x1b ") && !asJSON {
				t.Fatalf("control bytes in %q", s)
			}
		}
	}
	for _, s := range []string{out.String(), errb.String()} {
		if strings.ContainsAny(strings.TrimSuffix(s, "\n"), "\r\x1b ") {
			t.Fatalf("control bytes in %q", s)
		}
	}
}
