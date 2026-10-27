// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/forge"
)

const (
	noBranchMsg  = "T-040: claim does not name this pull request's branch"
	noListMsg    = "T-040: claim names no branch; renew it with waggle claim --branch"
	fallbackWarn = "claim for T-040 names no branch; accepted on ticket branch name"
)

// ticketRepo is leaseRepo with one T-040 commit in src. claim writes the claim.
func ticketRepo(t *testing.T, claim func(r gateRepo)) gateRepo {
	t.Helper()
	r := leaseRepo(t)
	claim(r)
	r.write("src/a.go", []byte("package src\n"))
	r.commit("T-040: add src")
	return r
}

func listed(branches ...string) func(r gateRepo) {
	return func(r gateRepo) { r.claimOn(40, branches, "full", active, "src") }
}

// forkEvent writes a pull request event whose head repository is headRepo
// (nil writes null) and whose base repository is o/r.
func forkEvent(t *testing.T, ref string, headRepo any) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"pull_request": map[string]any{
		"number": 7, "body": "",
		"head": map[string]any{"sha": otherHead, "ref": ref, "repo": headRepo},
		"base": map[string]any{"repo": map[string]string{"full_name": "o/r"}}}})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// T-039 AC8.
func TestReadPREventRepos(t *testing.T) {
	read := func(path string) *prEvent {
		t.Helper()
		ev, reason := readPREvent(func(k string) string {
			return map[string]string{"GITHUB_EVENT_NAME": "pull_request", "GITHUB_EVENT_PATH": path}[k]
		})
		if ev == nil {
			t.Fatalf("no event: %s", reason)
		}
		return ev
	}
	ev := read(forkEvent(t, "x", map[string]string{"full_name": "evil/r"}))
	if ev.headRepo != "evil/r" || ev.baseRepo != "o/r" || ev.number != 7 || ev.headRef != "x" || ev.headSHA != otherHead {
		t.Errorf("both repos: %+v", ev)
	}
	if ev = read(forkEvent(t, "x", nil)); ev.headRepo != "" || ev.baseRepo != "o/r" {
		t.Errorf("null head repo: %+v", ev)
	}
	p := filepath.Join(t.TempDir(), "e.json")
	if err := os.WriteFile(p, []byte(`{"pull_request":{"number":3,"body":"b","head":{"sha":"`+otherHead+`","ref":"r"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if ev = read(p); ev.number != 3 || ev.body != "b" || ev.headRef != "r" || ev.headRepo != "" || ev.baseRepo != "" {
		t.Errorf("no repo fields: %+v", ev)
	}
}

// T-039 AC9 (same repo) and AC14.
func TestClaimNamesHead(t *testing.T) {
	r := ticketRepo(t, listed("t-040"))
	if code, out := r.runWith(nil, "verify", "--branch", "t-040"); code != 0 {
		t.Errorf("t-040: exit %d\n%s", code, out)
	}
	for _, name := range []string{"feature/x", "T-040", "t-040-x"} {
		code, out := r.runWith(nil, "verify", "--branch", name)
		if code != 1 || !strings.Contains(out, noBranchMsg) {
			t.Errorf("%s: exit %d\n%s", name, code, out)
		}
		if strings.Contains(strings.Replace(out, noBranchMsg, "", 1), "feature/x") {
			t.Errorf("%s: the head name is echoed\n%s", name, out)
		}
	}
	// The approver rescan with an approval at head still fails the check.
	ar := apprRepo(t, apprConfig, nil)
	head := ar.head()
	useFake(t, &fakeReviews{rv: forge.PRReviews{Reviews: []forge.Review{approval(1, "alice", "APPROVED", head)}}})
	code, out := ar.runWith(prEnv(writeEvent(t, head, "z", apprBody)), "verify")
	if code != 1 || !strings.Contains(out, "T-031: claim does not name this pull request's branch") || !strings.Contains(out, "approved by alice") {
		t.Errorf("approver: exit %d\n%s", code, out)
	}
	// A hostile head reaches no output.
	code, out = r.runWith(nil, "verify", "--branch", "t-040\r::error::x")
	if code != 1 || strings.Contains(out, "\r") || strings.Contains(out, "\n::error::x") || strings.Contains(out, "::error::x") {
		t.Errorf("hostile head: exit %d\n%q", code, out)
	}
}

// T-039 AC10: a PR whose branch binds nothing cannot carry another ticket's
// commits. Before this ticket the same repo passed.
func TestClaimGapClosed(t *testing.T) {
	r := leaseRepo(t)
	r.claimOn(41, []string{"work"}, "full", active, "docs")
	r.claimOn(40, []string{"t-040"}, "full", active, "src")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-041: docs")
	r.write("src/a.go", []byte("package src\n"))
	r.commit("work\n\nWaggle-Ticket: T-040")
	code, out := r.runWith(nil, "verify", "--branch", "work")
	if code != 1 || strings.Count(out, noBranchMsg) != 1 || strings.Contains(out, "T-041") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

// T-039 AC11: forks, and heads the gate cannot place in the repository.
func TestClaimForkHeads(t *testing.T) {
	evil := map[string]string{"full_name": "evil/r"}
	cases := []struct {
		name   string
		claim  string
		ref    string
		repo   any
		passes bool
	}{
		{"plain entry", "t-040", "t-040", evil, false},
		{"fork entry", "evil/r:t-040", "t-040", evil, true},
		{"fork entry case", "EVIL/R:t-040", "t-040", evil, true},
		{"fork branch case", "evil/r:T-040", "t-040", evil, false},
		{"other fork", "other/r:t-040", "t-040", evil, false},
		{"null head repo", "evil/r:t-040", "t-040", nil, false},
	}
	for _, tc := range cases {
		r := ticketRepo(t, listed(tc.claim))
		code, out := r.runWith(prEnv(forkEvent(t, tc.ref, tc.repo)), "verify")
		if (code == 0) != tc.passes || (!tc.passes && !strings.Contains(out, noBranchMsg)) {
			t.Errorf("%s: exit %d\n%s", tc.name, code, out)
		}
	}
	r := ticketRepo(t, listed("t-040"))
	target := map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "pull_request_target", "GITHUB_HEAD_REF": "t-040"}
	if code, out := r.runWith(target, "verify"); code != 1 || !strings.Contains(out, noBranchMsg) {
		t.Errorf("pull_request_target: exit %d\n%s", code, out)
	}
	unreadable := map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "pull_request", "GITHUB_HEAD_REF": "t-040",
		"GITHUB_EVENT_PATH": filepath.Join(t.TempDir(), "none")}
	if code, out := r.runWith(unreadable, "verify"); code != 1 || !strings.Contains(out, noBranchMsg) {
		t.Errorf("unreadable event: exit %d\n%s", code, out)
	}
	if code, out := r.runWith(map[string]string{"GITHUB_ACTIONS": "true"}, "verify", "--branch", "t-040"); code != 0 {
		t.Errorf("--branch with no event: exit %d\n%s", code, out)
	}
}

// T-039 AC12 under Q-T039-1 (a), T-042 AC3.
func TestClaimWithoutBranchList(t *testing.T) {
	r := ticketRepo(t, func(r gateRepo) { r.claim(40, "full", active, "src") })
	for _, name := range []string{"t-040", "t-040-x", "t-040a", "waggle/T-040"} {
		code, out := r.runWith(nil, "verify", "--branch", name)
		if code != 1 || !strings.Contains(out, noListMsg) || strings.Contains(out, fallbackWarn) {
			t.Errorf("%s: exit %d\n%s", name, code, out)
		}
	}
	for _, name := range []string{"t-0040", "t-40", "xt-040", "feature/x", "t-040-" + strings.Repeat("a", 300)} {
		code, out := r.runWith(nil, "verify", "--branch", name)
		if code != 1 || !strings.Contains(out, noListMsg) {
			t.Errorf("%.12s: exit %d\n%s", name, code, out)
		}
	}
	code, out := r.runWith(prEnv(forkEvent(t, "t-040", map[string]string{"full_name": "evil/r"})), "verify")
	if code != 1 || !strings.Contains(out, noListMsg) {
		t.Errorf("fork head: exit %d\n%s", code, out)
	}
}

// T-042 AC3: an expired claim with no list fails the same way, and still warns.
func TestExpiredClaimWithoutBranchList(t *testing.T) {
	r := ticketRepo(t, func(r gateRepo) { r.claim(40, "full", "2020-01-02T00:00:00Z", "src") })
	code, out := r.runWith(nil, "verify", "--branch", "t-040")
	if code != 1 || !strings.Contains(out, noListMsg) || !strings.Contains(out, "claim for T-040 expired at") ||
		strings.Contains(out, fallbackWarn) {
		t.Errorf("exit %d\n%s", code, out)
	}
}

// T-042 AC4: no approval clears a claim with no list.
func TestApproverNoBranchList(t *testing.T) {
	r := apprRepo(t, apprConfig, nil)
	r.claim(31, "full", active, "docs")
	head := r.head()
	useFake(t, &fakeReviews{rv: forge.PRReviews{Reviews: []forge.Review{approval(1, "alice", "APPROVED", head)}}})
	code, out := r.runWith(prEnv(writeEvent(t, head, "x", apprBody)), "verify")
	if code != 1 || !strings.Contains(out, "T-031: claim names no branch; renew it with waggle claim --branch") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

// T-042 AC5: the branch checks that remain.
func TestNoListUnchangedCases(t *testing.T) {
	r := ticketRepo(t, func(r gateRepo) { r.claim(40, "full", active, "src") })
	code, out := r.runWith(nil, "verify")
	if code != 0 || strings.Count(out, "no pull request branch") != 1 {
		t.Errorf("local: exit %d\n%s", code, out)
	}
	code, out = r.runWith(map[string]string{"GITHUB_ACTIONS": "true"}, "verify")
	if code != 1 || !strings.Contains(out, "T-040: pull request branch unknown") || strings.Contains(out, "names no branch") {
		t.Errorf("actions: exit %d\n%s", code, out)
	}
	l := ticketRepo(t, listed("t-040"))
	if code, out := l.runWith(nil, "verify", "--branch", "t-040"); code != 0 {
		t.Errorf("listed: exit %d\n%s", code, out)
	}
	if code, out := l.runWith(nil, "verify", "--branch", "feature/x"); code != 1 || !strings.Contains(out, noBranchMsg) {
		t.Errorf("other branch: exit %d\n%s", code, out)
	}
}

// T-039 AC13: an unknown head warns once locally and fails under Actions.
func TestClaimUnknownHead(t *testing.T) {
	r := leaseRepo(t)
	r.claim(40, "full", active, "src")
	r.claim(41, "full", active, "docs")
	r.write("src/a.go", []byte("package src\n"))
	r.commit("T-040: src")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-041: docs")
	code, out := r.runWith(nil, "verify")
	warn := "leases: warning: no pull request branch; claim branches not checked (pass --branch)"
	if code != 0 || strings.Count(out, warn) != 1 {
		t.Errorf("local: exit %d\n%s", code, out)
	}
	code, out = r.runWith(map[string]string{"GITHUB_ACTIONS": "true"}, "verify")
	if code != 1 || !strings.Contains(out, "T-040: pull request branch unknown") || !strings.Contains(out, "T-041: pull request branch unknown") {
		t.Errorf("actions: exit %d\n%s", code, out)
	}
}
