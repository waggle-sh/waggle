// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/decisions"
	"github.com/waggle-sh/waggle/internal/nectar"
	"github.com/waggle-sh/waggle/internal/quorum"
)

const (
	qOpenAt  = "2100-01-01T00:00:00Z"
	qVoteAt  = "2100-01-02T00:00:00Z"
	qCloseAt = "2100-01-10T00:00:00Z"
)

const qConfig = "version: 1\nforge:\n  provider: none\n  remote: origin\ngate:\n  approvers: alice, bob\nquorum:\n  credits: 20\n  round_days: 7\n"

// qKey makes an ed25519 key with the real ssh-keygen. It fails, never skips,
// when ssh-keygen is missing: the signature tests must run.
func qKey(t *testing.T, name string) (priv, pub string) {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Fatalf("ssh-keygen is required for the quorum signature tests: %v", err)
	}
	priv = filepath.Join(t.TempDir(), name)
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", name, "-f", priv).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v: %s", err, out)
	}
	b, err := os.ReadFile(priv + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	f := strings.Fields(string(b))
	return priv, f[0] + " " + f[1]
}

func qWrite(t *testing.T, repo, rel, body string) {
	t.Helper()
	p := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setClock(t *testing.T, at string) {
	t.Helper()
	now, err := time.Parse(time.RFC3339, at)
	if err != nil {
		t.Fatal(err)
	}
	statusClock = func() time.Time { return now }
	t.Cleanup(func() { statusClock = time.Now })
}

// qHive is a scored hive (T-023 merged, bob holds its Nectar) with the quorum
// config, a registry for alice (first approver) and bob, a roadmap and one
// decision on main. bob is a second clone of the same origin.
type qHive struct {
	alice, bob, origin string
	aliceKey, bobKey   string
}

func newQHive(t *testing.T) qHive {
	t.Helper()
	repo, origin := scoredRepo(t)
	ak, apub := qKey(t, "alice")
	bk, bpub := qKey(t, "bob")
	ns := `namespaces="` + quorum.NamespaceBallot + "," + quorum.NamespaceRound + "," + quorum.NamespaceVeto + `" `
	qWrite(t, repo, ".waggle/control/config.yaml", qConfig)
	qWrite(t, repo, ".waggle/control/keepers", "alice "+ns+apub+"\nbob "+ns+bpub+"\n")
	qWrite(t, repo, ".waggle/hive/ROADMAP.md", "# Roadmap\n\n## Backlog (test)\n\nT-101 · first · S\nT-102 · second · S\nT-103 · third · S\n\n## Done\n\nT-001 · old · S\n")
	qWrite(t, repo, ".waggle/hive/DECISIONS.md", string(decisions.Render([]decisions.Entry{{
		ID: 1, Date: "2026-11-01", Kind: decisions.KindPolicy, Title: "Reorder", Question: "Reorder?",
		Options: []string{"yes", "no"}, Recommendation: "yes", Choice: "yes", Rationale: "test",
	}})))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "fixture: quorum")
	git(t, repo, "push", "-q", "origin", "main")
	clone := filepath.Join(t.TempDir(), "clone")
	git(t, filepath.Dir(clone), "clone", "-q", "-b", "main", origin, clone)
	qWrite(t, clone, ".waggle/control/local.yaml", "keeper_id: bob\n")
	return qHive{alice: repo, bob: clone, origin: origin, aliceKey: ak, bobKey: bk}
}

func qRun(t *testing.T, repo string, fn func([]string, io.Writer, io.Writer) int, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = fn(append(args, "--repo", repo), &out, &errb)
	for _, s := range []string{out.String(), errb.String()} {
		if strings.ContainsAny(s, dashes) {
			t.Errorf("output holds an en or em dash: %q", s)
		}
	}
	return code, out.String(), errb.String()
}

func qExpect(t *testing.T, want int, code int, out, errs string) {
	t.Helper()
	if code != want {
		t.Fatalf("exit %d, want %d\nstdout %s\nstderr %s", code, want, out, errs)
	}
}

func qStatus(t *testing.T, repo string, round string) (rep struct {
	Closed bool `json:"closed"`
	Tally  struct {
		Order   []string `json:"order"`
		Counted []string `json:"counted"`
	} `json:"tally"`
}) {
	t.Helper()
	code, out, errs := qRun(t, repo, runQuorum, "status", "--round", round, "--json", "--now", qVoteAt)
	qExpect(t, 0, code, out, errs)
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("status --json: %v: %q", err, out)
	}
	return rep
}

// D2: open, votes, a recast, a veto with its decision entry, close, verify from
// a second clone, status --next.
func TestQuorumEndToEnd(t *testing.T) {
	h := newQHive(t)

	setClock(t, qOpenAt)
	code, out, errs := qRun(t, h.alice, runQuorum, "open", "--from-roadmap", "--key", h.aliceKey, "--now", qOpenAt)
	qExpect(t, 0, code, out, errs)
	if !strings.Contains(out, "opened round 1 with 3 items, 20 credits each") {
		t.Fatalf("open: %q", out)
	}

	setClock(t, qVoteAt)
	// bob has no registered key configured for git: --key is needed, then the
	// git config default is used for the recast.
	code, out, errs = qRun(t, h.bob, runVote, "--round", "1", "T-101=3", "T-102=3")
	qExpect(t, 2, code, out, errs)
	code, out, errs = qRun(t, h.bob, runVote, "--round", "1", "T-101=3", "T-102=3", "--key", h.bobKey)
	qExpect(t, 0, code, out, errs)
	first := strings.Fields(strings.SplitN(out, "\n", 2)[0])[1]
	if !strings.Contains(out, "cost 18 of 20 credits") {
		t.Fatalf("vote: %q", out)
	}
	git(t, h.bob, "config", "gpg.format", "ssh")
	git(t, h.bob, "config", "user.signingkey", h.bobKey)
	code, out, errs = qRun(t, h.bob, runVote, "--round", "1", "T-101=2", "T-102=3", "T-103=2")
	qExpect(t, 0, code, out, errs)
	second := strings.Fields(strings.SplitN(out, "\n", 2)[0])[1]
	if first == second {
		t.Fatal("recast gave the same receipt")
	}
	code, out, errs = qRun(t, h.alice, runVote, "--round", "1", "T-103=4", "--key", h.aliceKey)
	qExpect(t, 0, code, out, errs)

	rep := qStatus(t, h.alice, "1")
	if rep.Closed || !reflect.DeepEqual(rep.Tally.Order, []string{"T-102", "T-101", "T-103"}) {
		t.Fatalf("provisional status: %+v", rep)
	}
	// Only the Head Keeper (the first approver) may veto.
	code, out, errs = qRun(t, h.bob, runQuorum, "veto", "--round", "1", "--item", "T-102", "--decision", "D-001")
	qExpect(t, 1, code, out, errs)
	code, out, errs = qRun(t, h.alice, runQuorum, "veto", "--round", "1", "--item", "T-102", "--decision", "D-001", "--key", h.aliceKey)
	qExpect(t, 0, code, out, errs)

	// Close is refused before the close time.
	code, out, errs = qRun(t, h.alice, runQuorum, "close", "--round", "1", "--key", h.aliceKey)
	qExpect(t, 1, code, out, errs)
	setClock(t, qCloseAt)
	code, out, errs = qRun(t, h.alice, runQuorum, "close", "--round", "1", "--key", h.aliceKey)
	qExpect(t, 0, code, out, errs)
	if !strings.Contains(out, "order: T-101, T-103") {
		t.Fatalf("close: %q", out)
	}
	code, out, errs = qRun(t, h.alice, runQuorum, "close", "--round", "1", "--key", h.aliceKey)
	qExpect(t, 1, code, out, errs)

	// A second clone recomputes the tally from refs.
	code, out, errs = qRun(t, h.bob, runQuorum, "verify", "--round", "1", "--receipt", second)
	qExpect(t, 0, code, out, errs)
	if !strings.Contains(out, "byte for byte") || !strings.Contains(out, ": counted") {
		t.Fatalf("verify: %q", out)
	}
	code, out, errs = qRun(t, h.bob, runQuorum, "verify", "--round", "1", "--receipt", first)
	qExpect(t, 0, code, out, errs)
	if !strings.Contains(out, "not counted (superseded by "+second+")") {
		t.Fatalf("verify first receipt: %q", out)
	}

	code, out, errs = qRun(t, h.bob, runQuorum, "status", "--round", "1", "--next")
	qExpect(t, 0, code, out, errs)
	if out != "T-101\nT-103\n" {
		t.Fatalf("status --next: %q", out)
	}
	code, out, errs = qRun(t, h.bob, runQuorum, "status")
	qExpect(t, 0, code, out, errs)
	if !strings.Contains(out, "round 1: closed") || !strings.Contains(out, "veto: T-102 under D-001") {
		t.Fatalf("status: %q", out)
	}
}

// D4: exit codes 0, 1, 2 and 3 for each command, and --json prints only JSON.
func TestQuorumExitCodes(t *testing.T) {
	h := newQHive(t)
	notRepo := t.TempDir()

	setClock(t, qOpenAt)
	code, out, errs := qRun(t, h.alice, runQuorum, "open", "--items", "T-101,issue-7", "--key", h.aliceKey, "--json")
	qExpect(t, 0, code, out, errs)
	var opened struct {
		Round struct {
			Round    int      `json:"round"`
			ClosesAt string   `json:"closes_at"`
			Items    []string `json:"items"`
		} `json:"round"`
	}
	if err := json.Unmarshal([]byte(out), &opened); err != nil || opened.Round.Round != 1 || opened.Round.ClosesAt != "2100-01-08T00:00:00Z" {
		t.Fatalf("open --json: %v: %q", err, out)
	}

	type step struct {
		name string
		fn   func([]string, io.Writer, io.Writer) int
		repo string
		args []string
		want int
	}
	steps := []step{
		{"vote dry run", runVote, h.alice, []string{"--round", "1", "T-101=2", "--dry-run", "--now", qVoteAt}, 0},
		{"vote over budget", runVote, h.alice, []string{"--round", "1", "T-101=9", "--dry-run", "--now", qVoteAt}, 1},
		{"vote unknown item", runVote, h.alice, []string{"--round", "1", "T-999=1", "--dry-run", "--now", qVoteAt}, 1},
		{"vote unknown round", runVote, h.alice, []string{"--round", "7", "T-101=1", "--dry-run"}, 1},
		{"vote no arguments", runVote, h.alice, nil, 2},
		{"vote bad pair", runVote, h.alice, []string{"--round", "1", "T-101"}, 2},
		{"vote not a repo", runVote, notRepo, []string{"--round", "1", "T-101=1"}, 3},
		{"open neither", runQuorum, h.alice, []string{"open"}, 2},
		{"open both", runQuorum, h.alice, []string{"open", "--items", "T-101", "--from-roadmap"}, 2},
		{"open bad item", runQuorum, h.alice, []string{"open", "--items", "t-101", "--key", h.aliceKey}, 1},
		{"open not a repo", runQuorum, notRepo, []string{"open", "--items", "T-101", "--key", h.aliceKey}, 3},
		{"status ok", runQuorum, h.alice, []string{"status", "--round", "1", "--now", qVoteAt}, 0},
		{"status unknown round", runQuorum, h.alice, []string{"status", "--round", "9"}, 1},
		{"status next of an open round", runQuorum, h.alice, []string{"status", "--round", "1", "--next"}, 1},
		{"status usage", runQuorum, h.alice, []string{"status", "--round", "-1"}, 2},
		{"status not a repo", runQuorum, notRepo, []string{"status"}, 3},
		{"veto no decision", runQuorum, h.alice, []string{"veto", "--round", "1", "--item", "T-101"}, 2},
		{"veto short decision", runQuorum, h.alice, []string{"veto", "--round", "1", "--item", "T-101", "--decision", "D-7", "--key", h.aliceKey}, 2},
		{"veto two digit decision", runQuorum, h.alice, []string{"veto", "--round", "1", "--item", "T-101", "--decision", "D-07", "--key", h.aliceKey}, 2},
		{"veto ok", runQuorum, h.alice, []string{"veto", "--round", "1", "--item", "T-101", "--decision", "D-001", "--key", h.aliceKey}, 0},
		{"veto unknown item", runQuorum, h.alice, []string{"veto", "--round", "1", "--item", "T-555", "--decision", "D-001", "--key", h.aliceKey}, 1},
		{"veto not a repo", runQuorum, notRepo, []string{"veto", "--round", "1", "--item", "T-101", "--decision", "D-001", "--key", h.aliceKey}, 3},
		{"close usage", runQuorum, h.alice, []string{"close"}, 2},
		{"close unknown round", runQuorum, h.alice, []string{"close", "--round", "9", "--key", h.aliceKey}, 1},
		{"close not a repo", runQuorum, notRepo, []string{"close", "--round", "1", "--key", h.aliceKey}, 3},
		{"verify open round", runQuorum, h.alice, []string{"verify", "--round", "1"}, 1},
		{"verify usage", runQuorum, h.alice, []string{"verify"}, 2},
		{"verify not a repo", runQuorum, notRepo, []string{"verify", "--round", "1"}, 3},
		{"unknown subcommand", runQuorum, h.alice, []string{"tally"}, 2},
		{"no subcommand", runQuorum, h.alice, nil, 2},
	}
	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			args := s.args
			if s.repo != "" && len(args) > 0 {
				args = append(append([]string{}, args...), "--repo", s.repo)
			}
			if code := s.fn(args, &out, &errb); code != s.want {
				t.Fatalf("exit %d, want %d\nstdout %s\nstderr %s", code, s.want, out.String(), errb.String())
			}
			if s.want != 0 && strings.HasSuffix(strings.Join(args, " "), "--json") && out.Len() != 0 {
				t.Fatalf("a failed --json run wrote stdout: %q", out.String())
			}
		})
	}

	// --json prints one JSON object and nothing else, on success and on failure.
	setClock(t, qVoteAt)
	for _, c := range []struct {
		name string
		fn   func([]string, io.Writer, io.Writer) int
		args []string
		want int
	}{
		{"vote", runVote, []string{"--round", "1", "T-101=2", "--dry-run", "--json"}, 0},
		{"status", runQuorum, []string{"status", "--round", "1", "--json"}, 0},
		{"close early", runQuorum, []string{"close", "--round", "1", "--key", h.aliceKey, "--json"}, 1},
		{"verify open", runQuorum, []string{"verify", "--round", "1", "--json"}, 1},
	} {
		code, out, errs := qRun(t, h.alice, c.fn, c.args...)
		qExpect(t, c.want, code, out, errs)
		if c.want == 0 {
			var v map[string]any
			if err := json.Unmarshal([]byte(out), &v); err != nil || strings.Count(out, "\n") != 1 {
				t.Errorf("%s --json: %v: %q", c.name, err, out)
			}
		} else if out != "" {
			t.Errorf("%s --json failed with stdout %q", c.name, out)
		}
	}
	setClock(t, qCloseAt)
	code, out, errs = qRun(t, h.alice, runQuorum, "close", "--round", "1", "--key", h.aliceKey, "--json")
	qExpect(t, 0, code, out, errs)
	var closed map[string]any
	if err := json.Unmarshal([]byte(out), &closed); err != nil || closed["oid"] == "" {
		t.Fatalf("close --json: %v: %q", err, out)
	}
	code, out, errs = qRun(t, h.alice, runQuorum, "verify", "--round", "1", "--json")
	qExpect(t, 0, code, out, errs)
	if err := json.Unmarshal([]byte(out), &closed); err != nil || closed["verified"] != true {
		t.Fatalf("verify --json: %v: %q", err, out)
	}
	// A vote after the close time is refused.
	code, out, errs = qRun(t, h.alice, runVote, "--round", "1", "T-101=1", "--key", h.aliceKey)
	qExpect(t, 1, code, out, errs)
}

// D3: the adapter gives the bytes internal/nectar.Score gives for the same
// pinned inputs, and the parsed map holds the Nectar of the merged task.
func TestQuorumNectarParity(t *testing.T) {
	repo, _ := scoredRepo(t)
	if _, errb, exit := nectarCLI(t, "--repo", repo, "--now", nectarNow, "--json"); exit != 0 {
		t.Fatalf("nectar: %d %s", exit, errb)
	}
	main := gitStdout(t, repo, "rev-parse", "refs/remotes/origin/main")
	receipts := gitStdout(t, repo, "rev-parse", comb.ReceiptRef)
	at := quorum.NectarAt{MainSHA: main, Now: qOpenAt, ReceiptsSHA: receipts, RulesVersion: nectar.RulesVersion1}
	got, err := scoreSource{dir: repo}.Snapshot(context.Background(), at)
	if err != nil {
		t.Fatal(err)
	}
	now, _ := time.Parse(time.RFC3339, qOpenAt)
	rep, err := nectar.Score(context.Background(), nectar.Input{Dir: repo, Main: main, Receipts: receipts, RulesVersion: nectar.RulesVersion1, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	want, err := nectarJSON(rep)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("adapter bytes differ from Score:\n%s\n%s\n%v", got, want, err)
	}
	m, err := quorum.ParseNectar(got)
	if err != nil {
		t.Fatal(err)
	}
	var wantMilli uint64
	for _, k := range rep.Summary.Keepers {
		if k.KeeperID == "bob" {
			wantMilli = uint64(k.Nectar)
		}
	}
	if wantMilli == 0 || m["bob"] != wantMilli {
		t.Fatalf("parsed Nectar %v, want bob %d (nonzero)", m, wantMilli)
	}
}

// D5: the help text of vote and quorum is plain.
func TestQuorumHelpPlainTerms(t *testing.T) {
	var out, errb bytes.Buffer
	if code := runVote([]string{"-h"}, &out, &errb); code != 0 {
		t.Fatalf("vote -h: %d", code)
	}
	checkPlain(t, "vote -h", errb.String(), "waggle vote --round <n>", "squared", "-dry-run")
	for _, sub := range []string{"-h", "open", "status", "veto", "close", "verify"} {
		errb.Reset()
		args := []string{sub, "-h"}
		if sub == "-h" {
			args = []string{"-h"}
		}
		if code := runQuorum(args, &out, &errb); code != 0 {
			t.Fatalf("quorum %v: %d", args, code)
		}
		checkPlain(t, "quorum "+sub+" -h", errb.String(), "waggle quorum open", "waggle quorum verify")
	}
}
