//go:build unix

package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/runtime"
)

const (
	head40 = "0123456789abcdef0123456789abcdef01234567"
	// mk is a valid round marker line for the given verdict.
	mkTail = " head=" + head40 + " -->"
)

func mk(verdict string) string { return "<!-- waggle-review v1 verdict=" + verdict + mkTail }

func TestCountReviewMustFix(t *testing.T) {
	legacy := strings.Join([]string{
		"# Review T-9 (verdict: must-fix)", "Verdict: must-fix  ", "notes", "Verdict: pass", "## Round 2",
		"Verdict: must-fix", "verdict: must-fix", " Verdict: must-fix", "",
	}, "\r\n")
	cases := []struct {
		name, data string
		want       int
	}{
		{"legacy table", legacy, 2},
		{"empty", "", 0},
		{"three rounds", mk("must-fix") + "\n\nVerdict: must-fix\nx\nVerdict: must-fix\n\n" + mk("pass") + "\n\nVerdict: pass\n\n" + mk("must-fix") + "\n\nVerdict: must-fix\n", 2},
		{"legacy then marker", "Verdict: must-fix\n\n" + mk("must-fix") + "\n\nVerdict: must-fix\n", 2},
		{"marker then marker-less round", mk("pass") + "\n\nVerdict: pass\n\nVerdict: must-fix\n", 0},
		{"leading space", " " + mk("must-fix") + "\n", 0},
		{"uppercase hex", "<!-- waggle-review v1 verdict=must-fix head=" + strings.ToUpper(head40) + " -->\n", 0},
		{"39 hex", "<!-- waggle-review v1 verdict=must-fix head=" + head40[1:] + " -->\n", 0},
		{"v2", "<!-- waggle-review v2 verdict=must-fix head=" + head40 + " -->\n", 0},
		{"verdict fail", "<!-- waggle-review v1 verdict=fail head=" + head40 + " -->\n", 0},
		{"trailing text", mk("must-fix") + " x\n", 0},
		{"CR and spaces", mk("must-fix") + "  \r\n" + mk("must-fix") + "\r\n", 2},
		{"64 hex", "<!-- waggle-review v1 verdict=must-fix head=" + strings.Repeat("ab", 32) + " -->\n", 1},
	}
	for _, c := range cases {
		if got := countReviewMustFix([]byte(c.data)); got != c.want {
			t.Errorf("%s: count %d, want %d", c.name, got, c.want)
		}
		if c.name == "legacy table" {
			if got := runtime.CountMustFix([]byte(c.data)); got != countReviewMustFix([]byte(c.data)) {
				t.Errorf("legacy count differs from runtime.CountMustFix")
			}
		}
	}
	// A quoted marker written by reviewFile does not count when read back.
	for _, verdict := range []string{"must-fix", ""} {
		out := reviewFile([]byte("quote:\n"+mk("must-fix")+"\nVerdict: pass\n"), verdict, head40)
		want := 0
		if verdict != "" {
			want = 1
		}
		if got := countReviewMustFix(out); got != want {
			t.Errorf("escaped (verdict %q): count %d, want %d", verdict, got, want)
		}
	}
}

func TestGuardTaskLinesRules(t *testing.T) {
	lines := guardTaskLines(Request{Ticket: "T-023", Branch: "t-023", Head: head40}, "abc", comb.Lease{}, nil)
	rules := lines[len(lines)-1]
	for _, s := range []string{"write no file in the repo; your final message is the review", "do not commit",
		"end with `Verdict: pass` or `Verdict: must-fix` as the last line of your final message."} {
		if !strings.Contains(rules, s) {
			t.Errorf("Rules line lacks %q: %s", s, rules)
		}
	}
}

func TestReviewFile(t *testing.T) {
	final := "body\nVerdict: must-fix"
	if got, want := string(reviewFile([]byte(final), "must-fix", head40)), mk("must-fix")+"\n\n"+final; got != want {
		t.Errorf("verdict run: %q", got)
	}
	if got := string(reviewFile([]byte(final), "", head40)); got != final {
		t.Errorf("no verdict: %q", got)
	}
	if got := string(reviewFile([]byte(final), "pass", "HEAD")); got != final {
		t.Errorf("bad head: %q", got)
	}
}

func TestGuardReviewMarker(t *testing.T) {
	quoted := mk("must-fix")
	cases := []struct {
		name, report, verdict, want string
	}{
		{"verdict", `notes\nVerdict: must-fix`, "must-fix", "notes\nVerdict: must-fix"},
		{"quoted marker", `see\n` + quoted + `\nVerdict: pass`, "pass", "see\n " + quoted + "\nVerdict: pass"},
		{"no verdict", `see\n` + quoted + `\nno verdict`, "", "see\n " + quoted + "\nno verdict"},
		{"cut final", `Verdict: pass\n` + strings.Repeat("x", runtime.MaxFinal+10), "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := guardFixture(t, false)
			head := f.git("rev-parse", "refs/heads/t-023")
			res, err := f.guard(emit(assistant(100), verdictEvent(c.report)))
			if err != nil || res.Verdict != c.verdict {
				t.Fatalf("verdict %q err %v", res.Verdict, err)
			}
			review, _ := os.ReadFile(filepath.Join(res.RunDir, "review.md"))
			final, _ := os.ReadFile(filepath.Join(res.RunDir, "final.md"))
			if c.name != "cut final" {
				if got := strings.ReplaceAll(c.report, `\n`, "\n"); string(final) != got {
					t.Fatalf("final.md = %q, want %q", final, got)
				}
				want := c.want
				if c.verdict != "" {
					want = "<!-- waggle-review v1 verdict=" + c.verdict + " head=" + head + " -->\n\n" + want
				}
				if string(review) != want {
					t.Fatalf("review.md = %q, want %q", review, want)
				}
			} else if strings.HasPrefix(string(review), markerPrefix) {
				t.Fatalf("marker written without a verdict")
			}
		})
	}
}

func TestInputsMarkerRounds(t *testing.T) {
	dir, g := newRepo(t)
	ctx := context.Background()
	round := func(v string) string { return mk(v) + "\n\nVerdict: " + v + "\nbody\nVerdict: " + v + "\n" }
	write(t, dir, ".waggle/hive/reviews/T-023.md", round("must-fix")+"\n"+round("must-fix"))
	// Two phase files for another ticket branch: each one must-fix round.
	write(t, dir, ".waggle/hive/reviews/T-023-opus.md", round("must-fix"))
	write(t, dir, ".waggle/hive/reviews/T-023-fable.md", round("must-fix"))
	sh(t, dir, "add", "-A")
	sh(t, dir, "commit", "-q", "-m", "review")
	base, _ := ResolveBase(ctx, g, "", "")
	req := mustValid(t, okReq(dir))
	in, err := ReadInputs(ctx, g, base, req)
	if err != nil || in.MustFix != 2+2 {
		t.Fatalf("ReadInputs must-fix %d, %v", in.MustFix, err)
	}
	if n, err := roundMustFix(ctx, g, base, req); err != nil || n != 2 {
		t.Fatalf("roundMustFix %d, %v (the old rule gives 4)", n, err)
	}
}
