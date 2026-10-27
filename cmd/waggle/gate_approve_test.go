// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/forge"
	"github.com/waggle-sh/waggle/internal/layout"
)

// fakeReviews is the review source tests install; it makes no HTTP call.
type fakeReviews struct {
	rv    forge.PRReviews
	err   error
	calls int
}

func (f *fakeReviews) Reviews(ctx context.Context, number int) (forge.PRReviews, error) {
	f.calls++
	return f.rv, f.err
}

// useFake installs f as the review source. It keeps the real constructor's
// repository and token validation.
func useFake(t *testing.T, f *fakeReviews) {
	t.Helper()
	old := newReviewSource
	newReviewSource = func(repository, token string) (reviewSource, error) {
		if _, err := forge.NewReviewReader(repository, token); err != nil {
			return nil, err
		}
		return f, nil
	}
	t.Cleanup(func() { newReviewSource = old })
}

const (
	apprConfig = "version: 1\ngate:\n  protected_paths: deploy/\n  approvers: alice\n"
	apprBody   = "Change\n\nWaggle-Ticket: T-031\nWaggle-Risk-Tier: 3\nWaggle-Verdict: pass\n"
	otherHead  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

var apprTime = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

// apprRepo is main with a base config, then a branch with a T-031 commit that
// adds deploy/x and docs/a.md under a claim on docs. more adds commits.
func apprRepo(t *testing.T, config string, more func(r gateRepo)) gateRepo {
	t.Helper()
	r := newGateRepo(t)
	r.write("README.md", []byte("hive\n"))
	r.write("other/x.md", []byte("x\n"))
	r.write(layout.Decisions, gateLog(1))
	if config != "" {
		r.write(layout.Config, []byte(config))
	}
	r.commit("init")
	r.git("checkout", "-q", "-b", "branch")
	r.claimOn(31, []string{"x"}, "full", active, "docs")
	r.write("deploy/x", []byte("x\n"))
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-031: change")
	if more != nil {
		more(r)
	}
	return r
}

func (r gateRepo) head() string { return r.git("rev-parse", "branch") }

// writeEvent writes a pull request event payload and returns its path.
func writeEvent(t *testing.T, sha, ref, body string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"pull_request": map[string]any{
		"number": 7, "body": body,
		"head": map[string]any{"sha": sha, "ref": ref, "repo": map[string]string{"full_name": "o/r"}},
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

func prEnv(path string) map[string]string {
	return map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "pull_request", "GITHUB_EVENT_PATH": path,
		"GITHUB_TOKEN": "tok", "GITHUB_REPOSITORY": "o/r"}
}

func approval(id int64, login, state, commit string) forge.Review {
	return forge.Review{ID: id, Login: login, State: state, CommitID: commit, Submitted: apprTime}
}

func errLines(out string) string {
	var ls []string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "::error") {
			ls = append(ls, l)
		}
	}
	return strings.Join(ls, "\n")
}

func TestApprovedBy(t *testing.T) {
	const h, o = "1111111111111111111111111111111111111111", "2222222222222222222222222222222222222222"
	list := []string{"alice", "bob"}
	at := func(r forge.Review, d time.Duration) forge.Review { r.Submitted = apprTime.Add(d); return r }
	cases := []struct {
		name string
		rv   []forge.Review
		base time.Time
		want string
	}{
		{"approved at head", []forge.Review{approval(1, "alice", "APPROVED", h)}, time.Time{}, "alice"},
		{"approved at old head", []forge.Review{approval(1, "alice", "APPROVED", o)}, time.Time{}, ""},
		{"old then head", []forge.Review{approval(1, "alice", "APPROVED", o), approval(2, "alice", "APPROVED", h)}, time.Time{}, "alice"},
		{"dismissed in place", []forge.Review{approval(1, "alice", "DISMISSED", h)}, time.Time{}, ""},
		{"approved then changes", []forge.Review{approval(1, "alice", "APPROVED", h), approval(2, "alice", "CHANGES_REQUESTED", h)}, time.Time{}, ""},
		{"dismissed request then approved", []forge.Review{approval(1, "alice", "DISMISSED", o), approval(2, "alice", "APPROVED", h)}, time.Time{}, "alice"},
		{"approved then commented", []forge.Review{approval(1, "alice", "APPROVED", h), approval(2, "alice", "COMMENTED", h)}, time.Time{}, "alice"},
		{"pending", []forge.Review{approval(1, "alice", "PENDING", h)}, time.Time{}, ""},
		{"unlisted", []forge.Review{approval(1, "carol", "APPROVED", h)}, time.Time{}, ""},
		{"case folded", []forge.Review{approval(1, "ALICE", "APPROVED", h)}, time.Time{}, "alice"},
		{"suffix", []forge.Review{approval(1, "alice2", "APPROVED", h)}, time.Time{}, ""},
		{"prefix", []forge.Review{approval(1, "xalice", "APPROVED", h)}, time.Time{}, ""},
		{"other approver blocks", []forge.Review{approval(1, "alice", "APPROVED", h), approval(2, "bob", "CHANGES_REQUESTED", o)}, time.Time{}, ""},
		{"before base change", []forge.Review{at(approval(1, "alice", "APPROVED", h), -time.Second)}, apprTime, ""},
		{"same second as base change", []forge.Review{approval(1, "alice", "APPROVED", h)}, apprTime, ""},
		{"after base change", []forge.Review{at(approval(1, "alice", "APPROVED", h), time.Second)}, apprTime, "alice"},
		{"no time with base change", []forge.Review{{ID: 1, Login: "alice", State: "APPROVED", CommitID: h}}, apprTime, ""},
		{"id order not slice order", []forge.Review{approval(1<<31+1, "alice", "APPROVED", h), approval(1<<31-1, "alice", "CHANGES_REQUESTED", h)}, time.Time{}, "alice"},
		{"changes after approval by id", []forge.Review{approval(1<<31-1, "alice", "CHANGES_REQUESTED", h), approval(1<<31+1, "alice", "APPROVED", h)}, time.Time{}, "alice"},
		{"none", nil, time.Time{}, ""},
	}
	for _, tc := range cases {
		if got := approvedBy(forge.PRReviews{Reviews: tc.rv, LastBaseChange: tc.base}, list, h); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestApproverPasses(t *testing.T) {
	r := apprRepo(t, apprConfig, nil)
	head := r.head()
	f := &fakeReviews{rv: forge.PRReviews{Reviews: []forge.Review{approval(1, "alice", "APPROVED", head)}}}
	useFake(t, f)
	env := prEnv(writeEvent(t, head, "x", apprBody))
	code, out := r.runWith(env, "verify")
	if code != 0 || !strings.Contains(out, "tiers: ok") || !strings.Contains(out, "leases: ok") ||
		strings.Count(out, "approved by alice") != 1 || strings.Contains(out, "Intent Packet") {
		t.Errorf("verify exit %d\n%s", code, out)
	}
	for _, c := range []string{checkTiers, checkLeases} {
		code, out := r.runWith(env, c)
		if code != 0 || !strings.Contains(out, "::warning::"+c+": protected paths approved by alice at "+head[:12]) {
			t.Errorf("%s exit %d\n%s", c, code, out)
		}
	}
}

func TestApproverFailsClosed(t *testing.T) {
	t.Run("older head", func(t *testing.T) {
		r := apprRepo(t, apprConfig, nil)
		head := r.head()
		useFake(t, &fakeReviews{rv: forge.PRReviews{Reviews: []forge.Review{approval(1, "alice", "APPROVED", otherHead)}}})
		base, bout := r.runWith(map[string]string{"GITHUB_ACTIONS": "true"}, "verify", "--branch", "x") // T-039 2.4: no head fails closed
		code, out := r.runWith(prEnv(writeEvent(t, head, "x", apprBody)), "verify")
		if code != 1 || base != 1 || errLines(out) != errLines(bout) || errLines(out) == "" ||
			!strings.Contains(out, "no approving review from a listed approver at "+head[:12]) {
			t.Errorf("exit %d\n%s\nbaseline:\n%s", code, out, bout)
		}
	})
	t.Run("head config adds approver", func(t *testing.T) {
		r := apprRepo(t, apprConfig, func(r gateRepo) {
			r.write(layout.Config, []byte(strings.Replace(apprConfig, "alice", "alice, carol", 1)))
			r.commit("T-031: config")
		})
		head := r.head()
		useFake(t, &fakeReviews{rv: forge.PRReviews{Reviews: []forge.Review{approval(1, "carol", "APPROVED", head)}}})
		code, out := r.runWith(prEnv(writeEvent(t, head, "x", apprBody)), "verify")
		if code != 1 || !strings.Contains(out, "deploy/x") {
			t.Errorf("exit %d\n%s", code, out)
		}
	})
	extra := map[string]struct {
		config string
		more   func(r gateRepo)
		want   string
	}{
		"hardcoded path": {apprConfig, func(r gateRepo) { r.write("go.sum", []byte("x\n")); r.commit("T-031: sum") }, "go.sum"},
		"hive path listed": {"version: 1\ngate:\n  protected_paths: deploy/, .waggle/hive/tasks/\n  approvers: alice\n",
			func(r gateRepo) { r.write(".waggle/hive/tasks/T-099.md", []byte("b\n")); r.commit("T-031: brief") }, ".waggle/hive/tasks/T-099.md"},
		"unbound commit": {apprConfig, func(r gateRepo) { r.write("deploy/y", []byte("y\n")); r.commit("other work") }, "commit has no T-<id>"},
		"symlink": {apprConfig, func(r gateRepo) {
			blob := r.gitIn("t", "hash-object", "-w", "--stdin")
			r.git("update-index", "--add", "--cacheinfo", "120000,"+blob+",deploy/l")
			r.git("commit", "-q", "-m", "T-031: link")
		}, "symlink or submodule mode 120000"},
	}
	for name, tc := range extra {
		t.Run(name, func(t *testing.T) {
			r := apprRepo(t, tc.config, tc.more)
			head := r.head()
			useFake(t, &fakeReviews{rv: forge.PRReviews{Reviews: []forge.Review{approval(1, "alice", "APPROVED", head)}}})
			code, out := r.runWith(prEnv(writeEvent(t, head, "x", apprBody)), "verify")
			if code != 1 || !strings.Contains(out, tc.want) || !strings.Contains(out, "approved by alice") {
				t.Errorf("exit %d\n%s", code, out)
			}
		})
	}
	t.Run("branch name ignored stays", func(t *testing.T) {
		r := apprRepo(t, apprConfig, nil)
		head := r.head()
		useFake(t, &fakeReviews{rv: forge.PRReviews{Reviews: []forge.Review{approval(1, "alice", "APPROVED", head)}}})
		code, out := r.runWith(prEnv(writeEvent(t, head, "x", apprBody)), "verify", "--branch", "a\x01b")
		// T-039 2.3: the ignored name is a known head that matches no claim entry.
		if code != 1 || !strings.Contains(out, "branch name ignored") || !strings.Contains(out, "T-031: claim does not name") {
			t.Errorf("exit %d\n%s", code, out)
		}
	})
	t.Run("event head differs", func(t *testing.T) {
		r := apprRepo(t, apprConfig, nil)
		f := &fakeReviews{rv: forge.PRReviews{Reviews: []forge.Review{approval(1, "alice", "APPROVED", r.head())}}}
		useFake(t, f)
		code, out := r.runWith(prEnv(writeEvent(t, otherHead, "x", apprBody)), "verify")
		if code != 1 || f.calls != 0 || !strings.Contains(out, "approver check skipped: no pull request context (event head differs from --head)") {
			t.Errorf("exit %d calls %d\n%s", code, f.calls, out)
		}
	})
}

func TestApproverDegrades(t *testing.T) {
	r := apprRepo(t, apprConfig, nil)
	head := r.head()
	_, bout := r.runWith(map[string]string{"GITHUB_ACTIONS": "true"}, "verify")
	good := func() *fakeReviews {
		return &fakeReviews{rv: forge.PRReviews{Reviews: []forge.Review{approval(1, "alice", "APPROVED", head)}}}
	}
	big := filepath.Join(t.TempDir(), "big.json")
	if err := os.WriteFile(big, []byte(strings.Repeat(" ", maxEventFile+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	zero := filepath.Join(t.TempDir(), "zero.json")
	if err := os.WriteFile(zero, []byte(`{"pull_request":{"number":0,"head":{"sha":"`+head+`","ref":"x"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(env map[string]string, f *fakeReviews)
		phrase string
	}{
		{"push", func(e map[string]string, f *fakeReviews) { e["GITHUB_EVENT_NAME"] = "push" }, "no pull request context (event is not a pull request event)"},
		{"no path", func(e map[string]string, f *fakeReviews) { delete(e, "GITHUB_EVENT_PATH") }, "no pull request context (no GITHUB_EVENT_PATH)"},
		{"unreadable", func(e map[string]string, f *fakeReviews) { e["GITHUB_EVENT_PATH"] = filepath.Join(t.TempDir(), "none") }, "no pull request context (event file unreadable)"},
		{"over cap", func(e map[string]string, f *fakeReviews) { e["GITHUB_EVENT_PATH"] = big }, "no pull request context (event file over 2 MiB)"},
		{"malformed", func(e map[string]string, f *fakeReviews) { e["GITHUB_EVENT_PATH"] = bad }, "no pull request context (event file malformed)"},
		{"number 0", func(e map[string]string, f *fakeReviews) { e["GITHUB_EVENT_PATH"] = zero }, "no pull request context (event file malformed)"},
		{"no token", func(e map[string]string, f *fakeReviews) { e["GITHUB_TOKEN"] = "" }, "no GITHUB_TOKEN"},
		{"bad repository", func(e map[string]string, f *fakeReviews) { e["GITHUB_REPOSITORY"] = "bad" }, "invalid GITHUB_REPOSITORY"},
		{"403", func(e map[string]string, f *fakeReviews) { f.err = &forge.StatusError{Status: 403} }, "GitHub answered 403"},
		{"500", func(e map[string]string, f *fakeReviews) { f.err = &forge.StatusError{Status: 500} }, "GitHub answered 500"},
		{"timeout", func(e map[string]string, f *fakeReviews) { f.err = context.DeadlineExceeded }, "GitHub reviews unavailable"},
		{"read error", func(e map[string]string, f *fakeReviews) { f.err = errors.New("bad node") }, "GitHub reviews unavailable"},
	}
	for _, tc := range cases {
		f := good()
		useFake(t, f)
		env := prEnv(writeEvent(t, head, "x", apprBody))
		tc.mutate(env, f)
		code, out := r.runWith(env, "verify")
		// T-039 2.4: a read event gives a head, so the baseline's unknown-head line goes.
		want := errLines(bout)
		if !map[string]bool{"push": true, "no path": true, "unreadable": true, "over cap": true, "malformed": true, "number 0": true}[tc.name] {
			want = strings.ReplaceAll(want, "::error::leases: T-031: pull request branch unknown\n", "")
		}
		if code != 1 || errLines(out) != want || strings.Count(out, "approver check skipped") != 1 ||
			!strings.Contains(out, "approver check skipped: "+tc.phrase) {
			t.Errorf("%s: exit %d\n%s", tc.name, code, out)
		}
		if strings.Contains(out, "push") {
			t.Errorf("%s: the event name is echoed\n%s", tc.name, out)
		}
	}
	// Off Actions nothing about approvers prints.
	f := good()
	useFake(t, f)
	code, out := r.runWith(map[string]string{}, "verify")
	if code != 1 || strings.Contains(out, "approver") || f.calls != 0 {
		t.Errorf("no GITHUB_ACTIONS: exit %d\n%s", code, out)
	}
}

func TestApproverInertWithoutApprovers(t *testing.T) {
	fake := &fakeReviews{}
	useFake(t, fake)
	ghaOnly := map[string]string{"GITHUB_ACTIONS": "true"}
	// (i) an empty list against no list at all.
	a := apprRepo(t, "version: 1\ngate:\n  protected_paths: deploy/\n  approvers: \"\"\n", nil)
	b := apprRepo(t, "version: 1\ngate:\n  protected_paths: deploy/\n", nil)
	ca, oa := a.runWith(ghaOnly, "verify", "--branch", "x") // T-039 2.4: no head fails closed
	cb, ob := b.runWith(ghaOnly, "verify", "--branch", "x")
	if ca != 1 || ca != cb || oa != ob {
		t.Errorf("empty list differs from no list:\n%s\n---\n%s", oa, ob)
	}
	// (ii) no list, with an event file of valid trailers, against no event.
	ev := prEnv(writeEvent(t, b.head(), "x", apprBody))
	ce, oe := b.runWith(ev, "verify")
	if ce != cb || oe != ob {
		t.Errorf("event file changed the output:\n%s\n---\n%s", oe, ob)
	}
	// Approvers set, no approvable path touched.
	c := apprRepo(t, "version: 1\ngate:\n  protected_paths: infra/\n  approvers: alice\n", nil)
	cc, oc := c.runWith(prEnv(writeEvent(t, c.head(), "x", apprBody)), "verify")
	if cc != 1 || strings.Contains(oc, "approver") { // deploy/x still fails leases: outside the claim
		t.Errorf("exit %d\n%s", cc, oc)
	}
	if fake.calls != 0 {
		t.Errorf("review source called %d times", fake.calls)
	}
}

func TestBranchDefaultFromEvent(t *testing.T) {
	r := leaseRepo(t)
	r.claimOn(31, []string{"t-031"}, "full", active, "docs")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("add docs")
	head := r.git("rev-parse", "branch")
	env := func(ref, sha string) map[string]string {
		e := prEnv(writeEvent(t, sha, ref, apprBody))
		e["GITHUB_EVENT_NAME"] = "pull_request_review"
		return e
	}
	if code, out := r.runWith(env("t-031", head), "verify"); code != 0 {
		t.Errorf("review event: exit %d\n%s", code, out)
	}
	if code, out := r.runWith(env("t-031", otherHead), "verify"); code != 0 {
		t.Errorf("review event, other head: exit %d\n%s", code, out)
	}
	e := env("t-031", head)
	e["GITHUB_HEAD_REF"] = "t-099"
	if code, _ := r.runWith(e, "verify"); code != 1 {
		t.Errorf("GITHUB_HEAD_REF should win over the event ref: exit %d", code)
	}
	code, out := r.runWith(env("t-031\r::error::injected", head), "verify")
	if code != 1 || !strings.Contains(out, "branch name ignored") || strings.Contains(out, "\n::error::injected") || strings.Contains(out, "\r") {
		t.Errorf("hostile ref: exit %d\n%q", code, out)
	}
}

func TestTrailerWarnings(t *testing.T) {
	bound := []string{"T-031"}
	cases := []struct {
		name string
		body string
		tier int
		want []string
	}{
		{"valid", apprBody, 3, nil},
		{"valid CRLF and case", "x\r\n\r\nwaggle-ticket: T-031\r\nWAGGLE-RISK-TIER: 3\r\nWaggle-Verdict: pass\r\n\r\n", 3, nil},
		{"other trailers ignored", apprBody + "Signed-off-by: a <a@b.c>\n", 3, nil},
		{"none", "just text", 3, []string{"no Waggle-Ticket"}},
		{"middle paragraph only", "Waggle-Ticket: T-031\nWaggle-Risk-Tier: 3\nWaggle-Verdict: pass\n\nmore text\n", 3, []string{"no Waggle-Ticket"}},
		{"missing verdict", "x\n\nWaggle-Ticket: T-031\nWaggle-Risk-Tier: 3\n", 3, []string{"Waggle-Verdict trailer missing"}},
		{"bad tier", "x\n\nWaggle-Ticket: T-031\nWaggle-Risk-Tier: high\nWaggle-Verdict: pass\n", 3, []string{"Waggle-Risk-Tier trailer malformed"}},
		{"other ticket", "x\n\nWaggle-Ticket: T-032\nWaggle-Risk-Tier: 3\nWaggle-Verdict: pass\n", 3, []string{"Waggle-Ticket names T-032; commits bind T-031"}},
		{"low tier", "x\n\nWaggle-Ticket: T-031\nWaggle-Risk-Tier: 1\nWaggle-Verdict: pass\n", 3, []string{"Waggle-Risk-Tier 1 is below the Tier 3 the changed paths require"}},
		{"repeated key", "x\n\nWaggle-Ticket: T-031\nWaggle-Ticket: T-031\nWaggle-Risk-Tier: 3\nWaggle-Verdict: pass\n", 3, []string{"Waggle-Ticket trailer malformed"}},
		{"must-fix", "x\n\nWaggle-Ticket: T-031\nWaggle-Risk-Tier: 3\nWaggle-Verdict: must-fix\n", 3, []string{"Waggle-Verdict is must-fix"}},
		{"bad verdict", "x\n\nWaggle-Ticket: T-031\nWaggle-Risk-Tier: 3\nWaggle-Verdict: maybe\n", 3, []string{"Waggle-Verdict trailer malformed"}},
		{"long value", "x\n\nWaggle-Ticket: T-031\nWaggle-Risk-Tier: 3\nWaggle-Verdict: " + strings.Repeat("p", 257) + "\n", 3, []string{"Waggle-Verdict trailer malformed"}},
		{"over 64 KiB", strings.Repeat("x", maxPRBody+1), 3, []string{"PR body over 64 KiB; trailers not checked"}},
	}
	for _, tc := range cases {
		got := trailerWarnings(tc.body, bound, 3)
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
			continue
		}
		for i, w := range tc.want {
			if !strings.HasPrefix(got[i], "Intent Packet: ") || !strings.Contains(got[i], w) {
				t.Errorf("%s: got %q, want %q", tc.name, got[i], w)
			}
		}
	}
	for _, w := range trailerWarnings("x\n\nWaggle-Ticket: T-031\nWaggle-Risk-Tier: high\r::error::z\nWaggle-Verdict: pass\n", bound, 3) {
		if strings.ContainsAny(w, "\r\n") || strings.Contains(w, "high") {
			t.Errorf("hostile value echoed: %q", w)
		}
	}
}

func TestTrailerCheckKeepsExit(t *testing.T) {
	r := leaseRepo(t)
	r.claimOn(31, []string{"x"}, "full", active, "docs")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-031: docs")
	head := r.git("rev-parse", "branch")
	useFake(t, &fakeReviews{})
	gha := map[string]string{"GITHUB_ACTIONS": "true"}
	bc, bout := r.runWith(gha, "verify", "--branch", "x") // T-039 2.4: no head fails closed
	code, out := r.runWith(prEnv(writeEvent(t, head, "x", "no trailers\r\n::error::x")), "verify")
	if code != bc || bc != 0 || strings.Count(out, "Intent Packet:") != 1 || strings.Contains(out, "\n::error::x") ||
		!strings.Contains(out, "::warning::leases: Intent Packet: no Waggle-Ticket") || strings.Contains(bout, "Intent Packet") {
		t.Errorf("exit %d (baseline %d)\n%s", code, bc, out)
	}
	// A matching body adds nothing; the other checks never run it.
	if code, out := r.runWith(prEnv(writeEvent(t, head, "x", "x\n\nWaggle-Ticket: T-031\nWaggle-Risk-Tier: 0\nWaggle-Verdict: pass\n")), "verify"); code != 0 || strings.Contains(out, "Intent Packet") {
		t.Errorf("matching body: exit %d\n%s", code, out)
	}
	if code, out := r.runWith(prEnv(writeEvent(t, head, "x", "none")), "tiers"); code != 0 || strings.Contains(out, "Intent Packet") {
		t.Errorf("tiers: exit %d\n%s", code, out)
	}
}
