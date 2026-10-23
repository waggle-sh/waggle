package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/runtime"
)

// rec writes runs/<name>/result.json for ticket T-034, branch t-034 unless set.
func rec(t *testing.T, runs, name string, r Result) {
	t.Helper()
	if r.Ticket == "" {
		r.Ticket = "T-034"
	}
	if r.Branch == "" {
		r.Branch = "t-034"
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	write(t, runs, name+"/result.json", string(b))
}

// hiveShape is the hive's policy since D-224: opus, sonnet, haiku, alternates.frontier
// fable, builder and guard minimum standard. alt false drops the alternate.
func hiveShape(alt bool) runtime.Policy {
	p := runtime.Policy{
		Models: runtime.Models{Adapter: "claude-code",
			Primary: map[runtime.Level]string{runtime.LevelFrontier: "opus", runtime.LevelStandard: "sonnet", runtime.LevelFast: "haiku"}},
		Minimums: map[string]runtime.Level{"builder": runtime.LevelStandard, "guard": runtime.LevelStandard, "forager": runtime.LevelFast},
	}
	if alt {
		p.Models.Alternate = map[runtime.Level]string{runtime.LevelFrontier: "fable"}
	}
	return p
}

// T-034 AC4: which records count as the previous round, this round's first
// guard and the builder record.
func TestGuardRecordLookup(t *testing.T) {
	const head = "C"
	cases := []struct {
		name        string
		setup       func(t *testing.T, runs string)
		prev, first string
	}{
		{"no records", func(*testing.T, string) {}, "", ""},
		{"another head", func(t *testing.T, runs string) {
			rec(t, runs, "T-034-guard-1", Result{HeadOID: "B", Verdict: "pass", Model: "opus"})
		}, "T-034-guard-1", ""},
		{"same head", func(t *testing.T, runs string) {
			rec(t, runs, "T-034-guard-1", Result{HeadOID: "B", Verdict: "pass"})
			rec(t, runs, "T-034-guard-2", Result{HeadOID: head, Verdict: "must-fix"})
		}, "T-034-guard-1", "T-034-guard-2"},
		{"skipped records", func(t *testing.T, runs string) {
			rec(t, runs, "T-034-guard-1", Result{HeadOID: "B", Verdict: "pass"})
			rec(t, runs, "T-034-guard-2", Result{HeadOID: "B", Verdict: "pass", Second: true})
			rec(t, runs, "T-034-guard-3", Result{HeadOID: "B"})
			rec(t, runs, "T-034-guard-4", Result{HeadOID: "B", Verdict: "pass", Branch: "t-034b"})
			rec(t, runs, "T-0340-guard-5", Result{HeadOID: "B", Verdict: "pass", Ticket: "T-0340", Branch: "t-0340"})
			rec(t, runs, "T-034-guard-6", Result{HeadOID: "B", Verdict: "pass", Ticket: "T-0340"})
			write(t, runs, "T-034-guard-7/result.json", `{"ticket":`)
			big, _ := json.Marshal(Result{Ticket: "T-034", Branch: "t-034", HeadOID: "B", Verdict: "pass"})
			write(t, runs, "T-034-guard-8/result.json", string(big)+strings.Repeat(" ", maxBrief))
			if err := os.MkdirAll(filepath.Join(runs, "T-034-guard-9"), 0o700); err != nil {
				t.Fatal(err)
			}
			src := filepath.Join(runs, "T-034-guard-1", "result.json")
			if err := os.Symlink(src, filepath.Join(runs, "T-034-guard-9", "result.json")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(runs, "T-034-guard-1"), filepath.Join(runs, "T-034-guard-11")); err != nil {
				t.Fatal(err)
			}
			rec(t, runs, "T-034-guard-+12", Result{HeadOID: "B", Verdict: "pass"})
		}, "T-034-guard-1", ""},
		{"numeric order", func(t *testing.T, runs string) {
			rec(t, runs, "T-034-guard-9", Result{HeadOID: "B", Verdict: "pass"})
			rec(t, runs, "T-034-guard-10", Result{HeadOID: "B", Verdict: "pass"})
		}, "T-034-guard-10", ""},
		{"lease violation counts", func(t *testing.T, runs string) {
			rec(t, runs, "T-034-guard-1", Result{HeadOID: "B", Verdict: "pass", End: "lease_violation"})
			rec(t, runs, "T-034-guard-2", Result{HeadOID: head, Verdict: "pass", End: "lease_violation"})
		}, "T-034-guard-1", "T-034-guard-2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runs := t.TempDir()
			c.setup(t, runs)
			_, prev, _ := guardRun(runs, "T-034", "t-034", func(r Result) bool { return r.HeadOID != head })
			_, first, _ := guardRun(runs, "T-034", "t-034", func(r Result) bool { return r.HeadOID == head })
			if prev != c.prev || first != c.first {
				t.Fatalf("previous %q first %q; want %q and %q", prev, first, c.prev, c.first)
			}
		})
	}
	runs := t.TempDir()
	rec(t, runs, "T-034-builder-1", Result{Model: "sonnet", End: "error"})
	rec(t, runs, "T-034-builder-2", Result{Model: "opus", Branch: "t-034b"})
	rec(t, runs, "T-034-builder-3", Result{Model: "haiku", Second: true})
	if b, name, ok := builderRun(runs, "T-034", "t-034"); !ok || name != "T-034-builder-1" || b.Model != "sonnet" {
		t.Fatalf("builder record %q %+v", name, b)
	}
}

type pick struct {
	model string
	warn  string // a substring of the only warning, "" for none
}

func wantPick(t *testing.T, sel runtime.Selection, warns []string, err error, want pick) {
	t.Helper()
	if err != nil || sel.Model != want.model {
		t.Fatalf("model %q err %v; want %s", sel.Model, err, want.model)
	}
	if want.warn == "" && len(warns) > 0 || want.warn != "" && (len(warns) != 1 || !strings.Contains(warns[0], want.warn)) {
		t.Fatalf("warnings %q; want %q", warns, want.warn)
	}
}

func task34(mustFix int, tr *comb.RiskTier) runtime.TaskContext {
	return runtime.TaskContext{TaskID: "T-034", GuardMustFix: mustFix, Tier: tr}
}

// T-034 AC5: confirming guards alternate over a fabricated history (D-205).
func TestGuardAlternates(t *testing.T) {
	runs := t.TempDir()
	p := hiveShape(true)
	rec(t, runs, "T-034-builder-1", Result{Model: "sonnet"})
	rec(t, runs, "T-034-guard-1", Result{HeadOID: "A", Verdict: "must-fix", Model: "opus"})
	rec(t, runs, "T-034-guard-2", Result{HeadOID: "A", Verdict: "pass", Model: "fable", Second: true})
	sel, warns, err := guardModel(runs, p, task34(1, nil), "t-034", "B", false)
	wantPick(t, sel, warns, err, pick{"fable", ""})
	tier2 := comb.Tier2Logic
	sel, warns, err = guardModel(runs, p, task34(1, &tier2), "t-034", "B", false)
	wantPick(t, sel, warns, err, pick{"opus", ""})

	rec(t, runs, "T-034-guard-3", Result{HeadOID: "B", Verdict: "must-fix", Model: "fable"})
	sel, warns, err = guardModel(runs, p, task34(1, nil), "t-034", "C", false)
	wantPick(t, sel, warns, err, pick{"opus", ""})
	rec(t, runs, "T-034-guard-4", Result{HeadOID: "C", Verdict: "pass", Model: "opus"})
	sel, warns, err = guardModel(runs, p, task34(1, nil), "t-034", "C", true)
	wantPick(t, sel, warns, err, pick{"fable", ""})

	// After a builder step-up: builder opus, so the walk gives sonnet after fable.
	stepped := t.TempDir()
	rec(t, stepped, "T-034-builder-1", Result{Model: "sonnet"})
	rec(t, stepped, "T-034-guard-1", Result{HeadOID: "B", Verdict: "must-fix", Model: "fable"})
	rec(t, stepped, "T-034-builder-2", Result{Model: "opus"})
	sel, warns, err = guardModel(stepped, p, task34(2, nil), "t-034", "C", false)
	wantPick(t, sel, warns, err, pick{"sonnet", ""})

	// One guard model only: the guard repeats it and says so.
	sel, warns, err = guardModel(runs, hiveShape(false), task34(1, nil), "t-034", "D", false)
	wantPick(t, sel, warns, err, pick{"opus", "confirm_model_repeated: opus also reviewed C; no other guard model is available (D-205)"})

	empty := t.TempDir()
	sel, warns, err = guardModel(empty, p, task34(1, nil), "t-034", "abcdef0123", false)
	wantPick(t, sel, warns, err, pick{"opus", "confirm_model_unknown: no guard run record for t-034 before abcdef0; the model did not alternate (D-205)"})
	sel, warns, err = guardModel(empty, p, task34(0, nil), "t-034", "abcdef0123", false)
	wantPick(t, sel, warns, err, pick{"opus", ""})
}

// T-034 AC4b (opus brief guard M1): a plain guard never lands on the builder
// record's model, even when the policy's builder stepped up since.
func TestGuardAvoidsBuilderRecord(t *testing.T) {
	runs := t.TempDir()
	rec(t, runs, "T-034-builder-1", Result{Model: "sonnet"})
	rec(t, runs, "T-034-guard-1", Result{HeadOID: "B", Verdict: "must-fix", Model: "fable"})
	sel, warns, err := guardModel(runs, hiveShape(true), task34(2, nil), "t-034", "C", false)
	wantPick(t, sel, warns, err, pick{"fable", "confirm_model_repeated: fable also reviewed B"})

	sel, warns, err = guardModel(runs, hiveShape(false), task34(2, nil), "t-034", "C", false)
	if !errors.Is(err, runtime.ErrNoCrossModel) || !strings.Contains(err.Error(), "shares") || sel.Model != "" || warns != nil {
		t.Fatalf("only the builder record's model left: model %q warnings %q err %v", sel.Model, warns, err)
	}

	tier2 := comb.Tier2Logic
	sel, warns, err = guardModel(runs, hiveShape(true), task34(2, &tier2), "t-034", "C", false)
	wantPick(t, sel, warns, err, pick{"fable", ""})
	sel, warns, err = guardModel(runs, hiveShape(false), task34(2, &tier2), "t-034", "C", false)
	if !errors.Is(err, runtime.ErrNoCrossModel) || !strings.Contains(err.Error(), `level frontier shares ["opus" "sonnet"]`) || sel.Model != "" {
		t.Fatalf("tier 2, no alternates: model %q err %v", sel.Model, err)
	}

	// No builder record: only the policy's builder model is avoided (section 5 note).
	if err := os.RemoveAll(filepath.Join(runs, "T-034-builder-1")); err != nil {
		t.Fatal(err)
	}
	sel, warns, err = guardModel(runs, hiveShape(true), task34(2, nil), "t-034", "C", false)
	wantPick(t, sel, warns, err, pick{"sonnet", ""})

	err = checkBuilderModel("sonnet", Result{Model: "sonnet"}, "T-034-builder-1", true)
	if code(err) != ExitEnv || !strings.Contains(err.Error(), "guard would use sonnet, the model of builder run T-034-builder-1") {
		t.Fatalf("post-resolution check: exit %d %v", code(err), err)
	}
	if err := checkBuilderModel("opus", Result{Model: "sonnet"}, "T-034-builder-1", true); err != nil {
		t.Fatal(err)
	}
}

// T-034 AC6: --second needs the first guard's base exactly and a builder base
// equal to it or an ancestor of it (the T-029 shape), checked by git.
func TestCheckSecondAncestry(t *testing.T) {
	dir, g := newRepo(t)
	oid := func() string { return sh(t, dir, "rev-parse", "HEAD") }
	b := oid()
	sh(t, dir, "commit", "-q", "--allow-empty", "-m", "m1")
	sh(t, dir, "commit", "-q", "--allow-empty", "-m", "f")
	f := oid()
	sh(t, dir, "checkout", "-q", "-b", "side", b)
	sh(t, dir, "commit", "-q", "--allow-empty", "-m", "s")
	s := oid()
	missing := strings.Repeat("ab", 20)
	const head = "1111111111111111111111111111111111111111"
	cases := []struct {
		name, builder, first, second, model, want string
		firstHead                                 string
	}{
		{"ancestor", b, f, f, "fable", "", head},
		{"equal", f, f, f, "fable", "", head},
		{"sibling", s, f, f, "fable", "is not an ancestor", head},
		{"descendant", f, b, b, "fable", "is not an ancestor", head},
		{"missing object", missing, f, f, "fable", "cannot check builder base " + missing, head},
		{"option", "-x", f, f, "fable", "run record T-034-builder-1 has no valid base_oid", head},
		{"symbolic", "HEAD", f, f, "fable", "run record T-034-builder-1 has no valid base_oid", head},
		{"first guard symbolic", b, "HEAD", f, "fable", "run record T-034-guard-1 has no valid base_oid", head},
		{"second base differs", b, f, b, "fable", "pass --base " + f, head},
		{"another head", b, f, f, "fable", "first guard run of T-034 on head 1111111", "2222222222222222222222222222222222222222"},
		{"builder model", b, f, f, "sonnet", "--second would use sonnet", head},
		{"first guard model", b, f, f, "opus", "--second would use opus", head},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runs := t.TempDir()
			rec(t, runs, "T-034-builder-1", Result{Model: "sonnet", BaseOID: c.builder})
			rec(t, runs, "T-034-guard-1", Result{Model: "opus", BaseOID: c.first, HeadOID: c.firstHead, Verdict: "must-fix"})
			err := checkSecond(context.Background(), g, runs, "T-034", "t-034", head, c.second, c.model)
			switch {
			case c.want == "" && err != nil:
				t.Fatalf("refused: %v", err)
			case c.want != "" && (code(err) != ExitEnv || !strings.Contains(err.Error(), c.want)):
				t.Fatalf("exit %d err %v; want exit 3 containing %q", code(err), err, c.want)
			}
		})
	}
}
