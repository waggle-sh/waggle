// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/runtime"
)

// verdictEvent is a result event whose report is text (no single quote, and
// "\n" is the JSON escape).
func verdictEvent(text string) string {
	return `{"type":"result","subtype":"success","is_error":false,"result":"` + text + `","session_id":"s1","usage":{"input_tokens":100,"output_tokens":10}}`
}

// guardFixture is a fixture after a clean builder run: branch t-023 and its claim.
func guardFixture(t *testing.T, remote bool) *fixture {
	t.Helper()
	f := newFixture(t, remote, false)
	if res, err := f.run(okScenario); err != nil || res.ExitCode != 0 {
		t.Fatalf("builder: exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
	return f
}

// guard runs a guard for ticket 23; it takes no directories and no gate.
func (f *fixture) guard(scenario string, mut ...func(*Request, *Deps)) (Result, error) {
	f.t.Helper()
	return f.run(scenario, append([]func(*Request, *Deps){func(r *Request, _ *Deps) { r.Role, r.Dirs, r.Gate = "guard", nil, "" }}, mut...)...)
}

func (f *fixture) claimRef() string { return f.git("rev-parse", "refs/waggle/claims/ticket-23") }

// Test 16: the guard path. The verdict is the last matching line; a verdict
// of either kind is exit 0; none is no_verdict, exit 5; a final cut at
// MaxFinal is never parsed, even when an early line says pass (item 12).
func TestGuardVerdicts(t *testing.T) {
	long := strings.Repeat("x", runtime.MaxFinal+1000)
	cases := []struct {
		name, report, verdict, end string
		exit                       int
	}{
		{"pass", `looks fine\nVerdict: pass`, "pass", "done", 0},
		{"must-fix", `Verdict: pass\nbut no\nVerdict: must-fix`, "must-fix", "done", 0},
		{"none", `all good, I think`, "", "no_verdict", 5},
		{"heading text", `Findings (verdict: must-fix) and Verdict: pass here`, "", "no_verdict", 5},
		{"truncated", `Verdict: pass\n` + long, "", "no_verdict", 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := guardFixture(t, false)
			claim := f.claimRef()
			head := f.git("rev-parse", "refs/heads/t-023")
			res, err := f.guard(emit(assistant(100), verdictEvent(c.report)))
			if err != nil || res.ExitCode != c.exit || res.End != c.end || res.Verdict != c.verdict {
				t.Fatalf("exit %d end %s verdict %q err %v violations %v", res.ExitCode, res.End, res.Verdict, err, res.Violations)
			}
			review, rerr := os.ReadFile(filepath.Join(res.RunDir, "review.md"))
			if rerr != nil || len(review) == 0 {
				t.Fatalf("review.md: %v", rerr)
			}
			if res.HeadOID != head || res.ClaimVerb != "read" || res.Pushed || res.Second || f.claimRef() != claim || f.git("rev-parse", "refs/heads/t-023") != head {
				t.Fatalf("head %s claim verb %s pushed %v: the guard must read the claim and move nothing", res.HeadOID, res.ClaimVerb, res.Pushed)
			}
		})
	}
}

// A guard needs the builder's claim and an existing branch, and a refusal writes nothing.
func TestGuardRefusals(t *testing.T) {
	f := newFixture(t, false, false)
	for name, want := range map[string]int{"no claim": ExitClaim} {
		if _, err := f.guard(okScenario); code(err) != want {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if sh(t, f.real, "for-each-ref", "refs/waggle/claims/", "refs/heads/t-023") != "" {
		t.Fatal("a refused guard wrote a ref")
	}
	g := guardFixture(t, false)
	if _, err := g.guard(okScenario, func(r *Request, _ *Deps) { r.Branch = "t-023-nowhere" }); code(err) != ExitUsage {
		t.Fatalf("missing branch: %v", err)
	}
}

// The second guard sees a worktree reset to head_oid with every untracked file
// gone, so the first guard's probe cannot reach the second review; a probe left
// behind fails rule 3 (exit 1) with the verdict still recorded.
func TestGuardSecondResetsWorktree(t *testing.T) {
	f := guardFixture(t, false)
	cfg, _ := os.ReadFile(filepath.Join(f.real, ".waggle/control/config.yaml"))
	write(t, f.real, ".waggle/control/config.yaml", strings.Replace(string(cfg), "        fast: haiku\n", "        fast: haiku\n      alternates:\n        standard: haiku\n", 1))
	seen, seenA := filepath.Join(t.TempDir(), "seen"), filepath.Join(t.TempDir(), "seenA")
	pass := emit(assistant(100), verdictEvent(`Verdict: pass`))
	res, err := f.guard("echo probe > probe.txt\nmkdir -p sub && echo p > sub/q.txt\necho x >> internal/x/b.go\ngit update-index --skip-worktree internal/x/a.go\necho POISON >> internal/x/a.go\n" + pass)
	if err != nil || res.ExitCode != 1 || res.Verdict != "pass" || !hasViolation(res.Violations, ruleDirty, ".") {
		t.Fatalf("first guard: exit %d verdict %q err %v violations %v", res.ExitCode, res.Verdict, err, res.Violations)
	}
	res, err = f.guard("git status --porcelain=v1 --ignored > '"+seen+"'\ngit rev-parse HEAD >> '"+seen+"'\ncat internal/x/a.go > '"+seenA+"'\n"+pass, func(r *Request, _ *Deps) { r.Second = true })
	if err != nil || res.ExitCode != 0 || !res.Second {
		t.Fatalf("second guard: exit %d second %v err %v violations %v", res.ExitCode, res.Second, err, res.Violations)
	}
	got, _ := os.ReadFile(seen)
	if a, _ := os.ReadFile(seenA); !strings.Contains(string(a), "package x") || strings.Contains(string(a), "POISON") {
		t.Fatalf("a skip-worktree edit survived the reset: %s", a)
	}
	if want := f.git("rev-parse", "refs/heads/t-023"); strings.TrimSpace(string(got)) != want {
		t.Fatalf("second guard saw a dirty or moved worktree:\n%s\nwant only HEAD %s", got, want)
	}
}

// Rule 5: a guard commits nothing and moves no ref, and its worktree HEAD must
// still equal head_oid after the run (carry: HEAD check).
func TestGuardZeroChange(t *testing.T) {
	cases := map[string]struct{ script, path string }{
		"commit":       {"git -c user.name=a -c user.email=a@e.co commit -q --allow-empty -m probe\n", "HEAD"},
		"detach moved": {"git checkout -q --detach HEAD~1\n", "HEAD"},
		"branch moved": {"git update-ref refs/heads/t-023 HEAD~1\n", "refs/heads/t-023"},
		"claim moved":  {"git update-ref refs/waggle/claims/ticket-23 HEAD~1\n", "refs/waggle/claims/ticket-23"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := guardFixture(t, false)
			res, err := f.guard(c.script + emit(assistant(100), verdictEvent(`Verdict: pass`)))
			if err != nil || res.ExitCode != 1 || res.Verdict != "pass" || !hasViolation(res.Violations, ruleHost, c.path) {
				t.Fatalf("exit %d verdict %q err %v violations %v", res.ExitCode, res.Verdict, err, res.Violations)
			}
		})
	}
}

// Item 22 and opus N6: the must-fix count comes from this phase's review files
// at base (t-023b reads T-023b-*.md), so two guards' must-fix files step the
// builder up and a file for another phase does not. The count is read at the
// run's base, so two guards see the same count only on the same base: a --second
// run after a committed review runs on the first guard's base, which checkSecond
// enforces (TestSecondGuardRefusals).
func TestMustFixCountPerPhase(t *testing.T) {
	commit := func(f *fixture, name, body string) {
		write(t, f.real, ".waggle/hive/reviews/"+name, body)
		f.git("add", "-A")
		f.git("commit", "-q", "-m", "review "+name)
	}
	model := func(f *fixture, branch string, mut ...func(*Request, *Deps)) string {
		res, err := f.run(okScenario, append([]func(*Request, *Deps){func(r *Request, _ *Deps) { r.Branch = branch }}, mut...)...)
		if err != nil || res.ExitCode != 0 {
			t.Fatalf("exit %d err %v", res.ExitCode, err)
		}
		return res.Model
	}
	base := model(newFixture(t, false, false), "t-023b")

	f := newFixture(t, false, false)
	commit(f, "T-023b-opus.md", "# r\nVerdict: must-fix\n")
	commit(f, "T-023b-fable.md", "# r\nVerdict: must-fix\n")
	commit(f, "T-023d-opus.md", "# other phase\nVerdict: must-fix\nVerdict: must-fix\nVerdict: must-fix\n")
	if got := model(f, "t-023b"); got == base {
		t.Fatalf("two must-fix files for the phase left the builder on %s", got)
	}
	g := newFixture(t, false, false)
	commit(g, "T-023b-opus.md", "Verdict: must-fix\n")
	commit(g, "T-023d-opus.md", "Verdict: must-fix\nVerdict: must-fix\n")
	if got := model(g, "t-023b"); got != base {
		t.Fatalf("one phase file and one other-phase file stepped the builder up to %s", got)
	}
}

// Item 14 and the ledger: every end maps as section 9.1 says, a session writes
// one line, and a failing append is a warning in result.json.
func TestLedger(t *testing.T) {
	for end, want := range map[string]string{"done": "done", "handoff": "handoff", "max_turns": "max_turns", "max_sessions": "max_sessions",
		"max_tokens": "max_tokens", "max_budget": "max_budget", "timeout": "error", "no_change": "error", "lease_violation": "error",
		"no_verdict": "error", "bad_handoff": "error", "interrupted": "error", "error": "error"} {
		if got := ledgerEnd(end); got != want {
			t.Errorf("%s maps to %s, want %s", end, got, want)
		}
	}
	f := newFixture(t, false, false)
	res, err := f.run(emit(assistant(100), `{"type":"result","subtype":"error_max_budget_usd","is_error":true,"session_id":"s9","usage":{"input_tokens":7,"output_tokens":3}}`))
	if err != nil || res.End != "max_budget" {
		t.Fatalf("end %s err %v", res.End, err)
	}
	line, _ := os.ReadFile(filepath.Join(f.real, ".waggle/control/usage.jsonl"))
	for _, want := range []string{`"ticket":"T-023"`, `"role":"builder"`, `"session":"s9"`, `"end":"max_budget"`, `"input_tokens":7`, `"adapter":"claude-code"`} {
		if !strings.Contains(string(line), want) {
			t.Fatalf("ledger line lacks %s: %s", want, line)
		}
	}
	if n := strings.Count(string(line), "\n"); n != 1 {
		t.Fatalf("%d ledger lines for one session", n)
	}
	h := newFixture(t, false, false)
	if err := os.Mkdir(filepath.Join(h.real, ".waggle/control/usage.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err = h.run(okScenario)
	b, _ := os.ReadFile(filepath.Join(res.RunDir, "result.json"))
	if err != nil || res.ExitCode != 0 || !strings.Contains(string(b), "ledger_append_failed") {
		t.Fatalf("exit %d err %v: a failing append must be a warning in result.json", res.ExitCode, err)
	}
}
