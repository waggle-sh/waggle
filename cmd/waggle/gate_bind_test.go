package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
)

// runWith runs a gate check on main..branch with extra args and an environment.
func (r gateRepo) runWith(env map[string]string, check string, extra ...string) (int, string) {
	getenv := func(k string) string { return env[k] }
	args := append([]string{check, "--base", "main", "--head", "branch", "--repo", r.dir}, extra...)
	code, out, errs := gate(getenv, args...)
	return code, out + errs
}

// protRepo is main with a README, a log and the given base config (when not
// empty), then a branch. Claim 31 leases docs, deploy, infra and src.
func protRepo(t *testing.T, config string) gateRepo {
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
	r.claim(31, "full", active, "docs", "deploy", "infra", "src")
	return r
}

func TestBindSources(t *testing.T) {
	cases := []struct {
		name  string
		msg   string
		extra []string
		env   map[string]string
	}{
		{"trailer", "add docs\n\nWaggle-Ticket: T-031", nil, nil},
		{"trailer short id", "add docs\n\nWaggle-Ticket: 31", nil, nil},
		{"flag", "add docs", []string{"--branch", "t-031"}, nil},
		{"flag suffix", "add docs", []string{"--branch", "t-031-fix"}, nil},
		{"flag waggle", "add docs", []string{"--branch", "waggle/T-031"}, nil},
		// T-039 2.3: no event under Actions is a fork head, so the row reads a
		// same-repo event (set below) whose head ref is t-031.
		{"env", "add docs", nil, nil},
	}
	for _, tc := range cases {
		r := leaseRepo(t)
		switch {
		case len(tc.extra) > 0:
			r.claimOn(31, []string{tc.extra[1]}, "full", active, "docs")
		case tc.name == "env":
			r.claimOn(31, []string{"t-031"}, "full", active, "docs")
		default:
			r.claim(31, "full", active, "docs")
		}
		r.write("docs/a.md", []byte("a\n"))
		r.commit(tc.msg)
		env := tc.env
		if tc.name == "env" {
			env = prEnv(writeEvent(t, otherHead, "t-031", ""))
		}
		if code, out := r.runWith(env, "verify", tc.extra...); code != 0 {
			t.Errorf("%s inside: exit %d\n%s", tc.name, code, out)
		}
		r.write("src/a.go", []byte("package src\n"))
		r.commit(tc.msg)
		if code, out := r.runWith(env, "verify", tc.extra...); code != 1 || !strings.Contains(out, "T-031: path is outside") {
			t.Errorf("%s outside: exit %d\n%s", tc.name, code, out)
		}
	}
	// The environment is read only under GitHub Actions; other names bind nothing.
	for name, c := range map[string]struct {
		env   map[string]string
		extra []string
	}{
		"env without actions": {map[string]string{"GITHUB_HEAD_REF": "t-031"}, nil},
		"hive records":        {nil, []string{"--branch", "hive-records-20"}},
		"zero":                {nil, []string{"--branch", "t-000"}},
		"embedded":            {nil, []string{"--branch", "revert-T-031-in-x"}},
		"two digits":          {nil, []string{"--branch", "t-31"}},
	} {
		r := leaseRepo(t)
		r.claim(31, "full", active, "docs")
		r.write("docs/a.md", []byte("a\n"))
		r.commit("add docs")
		code, out := r.runWith(c.env, "verify", c.extra...)
		if code != 1 || !strings.Contains(out, "commit has no T-<id>: prefix, Waggle-Ticket: trailer or ticket branch") {
			t.Errorf("%s: exit %d\n%s", name, code, out)
		}
	}
}

func TestBindConflicts(t *testing.T) {
	cases := []struct {
		name, msg, branch string
		code              int
		want              string
	}{
		{"subject vs trailer", "T-031: x\n\nWaggle-Ticket: T-032", "", 1, "ticket conflict: subject T-031, trailer T-032"},
		{"subject vs branch", "T-031: x", "t-032", 1, "ticket conflict: subject T-031, branch T-032"},
		{"two trailers", "x\n\nWaggle-Ticket: T-031\nWaggle-Ticket: T-032", "", 1, "ticket conflict: trailer T-031, T-032"},
		{"trailer vs branch", "x\n\nWaggle-Ticket: T-031", "t-032-x", 1, "ticket conflict"},
		{"all agree", "T-031: x\n\nWaggle-Ticket: T-31", "t-031-x", 0, "leases: ok"},
		{"malformed", "x\n\nWaggle-Ticket: ev%0A::q,r\r", "", 1, "malformed Waggle-Ticket trailer"},
		{"empty value", "x\n\nWaggle-Ticket: T-031,", "", 1, "malformed Waggle-Ticket trailer"},
		{"too many values", "x\n\nWaggle-Ticket: 31,31,31,31,31,31,31,31,31", "", 1, "Waggle-Ticket trailer too long"},
		{"long value", "x\n\nWaggle-Ticket: T-0000000000000031", "", 1, "Waggle-Ticket trailer too long"},
		{"long field", "x\n\nWaggle-Ticket: 31," + strings.Repeat(" ", 300) + "31", "", 1, "Waggle-Ticket trailer too long"},
	}
	for _, tc := range cases {
		r := leaseRepo(t)
		if tc.name == "all agree" {
			r.claimOn(31, []string{tc.branch}, "full", active, "docs")
		} else {
			r.claim(31, "full", active, "docs")
		}
		r.write("docs/a.md", []byte("a\n"))
		r.commit(tc.msg)
		var extra []string
		if tc.branch != "" {
			extra = []string{"--branch", tc.branch}
		}
		code, out := r.runWith(nil, "leases", extra...)
		if code != tc.code || !strings.Contains(out, tc.want) {
			t.Errorf("%s: exit %d\n%s", tc.name, code, out)
		}
		if strings.Contains(out, "ev%") || strings.Contains(out, "0000") {
			t.Errorf("%s: trailer value echoed\n%s", tc.name, out)
		}
	}
}

func TestBindHeadKeeperIgnoresBranch(t *testing.T) {
	r := leaseRepo(t)
	r.claim(31, "full", active, "docs")
	r.write("src/a.go", []byte("package src\n"))
	r.commit("Head Keeper: tweak\n\nWaggle-Ticket: T-031")
	code, out := r.runWith(nil, "verify", "--branch", "t-031")
	if code != 1 || !strings.Contains(out, "merge with an admin bypass") || strings.Contains(out, "T-031") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestBindIgnoresBadBranch(t *testing.T) {
	r := leaseRepo(t)
	r.claim(31, "full", active, "docs")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("add docs")
	for _, b := range []string{"t-031-" + strings.Repeat("x", 300), "t-031-a\x01b"} {
		code, out := r.runWith(nil, "leases", "--branch", b)
		if code != 1 || !strings.Contains(out, "leases: warning: branch name ignored") || strings.Contains(out, "xxxx") {
			t.Errorf("branch %q: exit %d\n%s", b[:6], code, out)
		}
	}
}

// AC15: the warning is filed under the check that ran.
func TestBadBranchWarningPerCheck(t *testing.T) {
	r := leaseRepo(t)
	r.claim(31, "full", active, "docs")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("add docs")
	const b = "t-031-a\x01b"
	if _, out := r.runWith(nil, "tiers", "--branch", b); !strings.Contains(out, "tiers: warning: branch name ignored") {
		t.Errorf("tiers:\n%s", out)
	}
	for _, check := range []string{"leases", "verify"} {
		_, out := r.runWith(nil, check, "--branch", b)
		if !strings.Contains(out, "leases: warning: branch name ignored") || strings.Contains(out, "tiers: warning") {
			t.Errorf("%s:\n%s", check, out)
		}
	}
}

// AC14: the field length is checked before the split, with the same results.
func TestTrailerTicketsBounds(t *testing.T) {
	cases := []struct {
		name, raw string
		ids       int
		bad       string
	}{
		{"empty", "", 0, ""},
		{"one", "T-031", 1, ""},
		{"eight", "1,2,3,4,5,6,7,8", 8, ""},
		{"nine", "1,2,3,4,5,6,7,8,9", 0, "Waggle-Ticket trailer too long"},
		{"16 bytes", "0000000000000031", 0, "malformed Waggle-Ticket trailer"},
		{"17 bytes", "00000000000000031", 0, "Waggle-Ticket trailer too long"},
		{"256 bytes", "31," + strings.Repeat(" ", 251) + "31", 1, ""},
		{"257 bytes", "31," + strings.Repeat(" ", 252) + "31", 0, "Waggle-Ticket trailer too long"},
		{"300 spaces", "31," + strings.Repeat(" ", 300) + "31", 0, "Waggle-Ticket trailer too long"},
		{"malformed", "T-031,x", 0, "malformed Waggle-Ticket trailer"},
	}
	for _, tc := range cases {
		ids, bad := trailerTickets(tc.raw)
		if len(ids) != tc.ids || bad != tc.bad {
			t.Errorf("%s: %v %q", tc.name, ids, bad)
		}
	}
}

func TestProtectedPaths(t *testing.T) {
	const cfg = "version: 1\ngate:\n  protected_paths: \"deploy/,Infra/\"\n"
	const msg = "protected path (listed in .waggle/control/config.yaml); needs an admin bypass to merge"
	r := protRepo(t, cfg)
	r.write("deploy/x", []byte("x\n"))
	r.write("infra/y", []byte("y\n"))
	r.write("src/ok.go", []byte("package src\n"))
	r.commit("T-031: edit")
	code, out := r.runWith(nil, "verify")
	for _, p := range []string{"deploy/x", "infra/y"} {
		if !strings.Contains(out, "tiers: FAIL: "+p+": "+msg) {
			t.Errorf("%s not reported\n%s", p, out)
		}
	}
	if code != 1 || strings.Contains(out, "src/ok.go") {
		t.Errorf("exit %d\n%s", code, out)
	}

	// An entry added only on the head side does not apply.
	r = protRepo(t, "")
	r.write(layout.Config, []byte(cfg))
	r.write("deploy/x", []byte("x\n"))
	r.commit("T-031: edit")
	_, out = r.runWith(nil, "tiers")
	if strings.Contains(out, "FAIL: deploy/x") || !strings.Contains(out, "FAIL: "+layout.Config) {
		t.Errorf("head-side entry applied\n%s", out)
	}

	// An entry inside the hive raises a hive file to Tier 3; others stay Tier 2.
	r = protRepo(t, "version: 1\ngate:\n  protected_paths: \".waggle/hive/tasks/\"\n")
	r.write(".waggle/hive/tasks/T-099.md", []byte("brief\n"))
	r.write(".waggle/hive/notes.md", []byte("n\n"))
	r.commit("T-031: briefs")
	_, out = r.runWith(nil, "tiers")
	if !strings.Contains(out, "FAIL: .waggle/hive/tasks/T-099.md: "+msg) || strings.Contains(out, "notes.md") {
		t.Errorf("hive entry\n%s", out)
	}

	// A hardcoded hit keeps its text even when also listed.
	r = protRepo(t, "version: 1\ngate:\n  protected_paths: \".github/\"\n")
	r.write(".github/workflows/x.yml", []byte("x\n"))
	r.commit("T-031: ci")
	if _, out = r.runWith(nil, "tiers"); !strings.Contains(out, "Tier 3 path; only an admin bypass merges it, approval does not") || strings.Contains(out, msg) {
		t.Errorf("hardcoded text\n%s", out)
	}
}

func TestBaseConfigFaultsExit3(t *testing.T) {
	good := "version: 1\ngate:\n  protected_paths: \"deploy/\"\n"
	cases := map[string]func(r gateRepo){
		"invalid": func(r gateRepo) { r.write(layout.Config, []byte("version: 2\n")) },
		"unknown gate key": func(r gateRepo) {
			r.write(layout.Config, []byte("version: 1\ngate:\n  x: y\n"))
		},
		"symlink": func(r gateRepo) {
			r.write("real.yaml", []byte(good))
			if err := os.MkdirAll(filepath.Join(r.dir, filepath.Dir(layout.Config)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("../../real.yaml", filepath.Join(r.dir, layout.Config)); err != nil {
				t.Skip(err)
			}
		},
		"directory": func(r gateRepo) { r.write(layout.Config+"/inner", []byte(good)) },
		"oversize": func(r gateRepo) {
			r.write(layout.Config, []byte("version: 1\n"+strings.Repeat("# padding padding padding\n", 3000)))
		},
	}
	for name, setup := range cases {
		r := newGateRepo(t)
		r.write("README.md", []byte("hive\n"))
		r.write(layout.Decisions, gateLog(1))
		setup(r)
		r.commit("init")
		r.git("checkout", "-q", "-b", "branch")
		r.claim(31, "full", active, "docs")
		r.write("docs/a.md", []byte("a\n"))
		r.commit("T-031: add")
		code, out := r.runWith(nil, "verify")
		if code != 3 || !strings.Contains(out, "error: ") {
			t.Errorf("%s: exit %d\n%s", name, code, out)
		}
	}
	// A missing config is not an error.
	r := protRepo(t, "")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-031: add")
	if code, out := r.runWith(nil, "verify"); code != 0 {
		t.Errorf("missing config: exit %d\n%s", code, out)
	}
}

func TestDependencyInputsFailTiers(t *testing.T) {
	r := leaseRepo(t)
	r.claim(31, "full", active, ".", "docs")
	r.write("go.sum", []byte("x\n"))
	r.commit("T-031: deps")
	if _, out := r.runWith(nil, "tiers"); !strings.Contains(out, "tiers: FAIL: go.sum: Tier 3 path") {
		t.Errorf("go.sum\n%s", out)
	}
}

func TestBindHostileText(t *testing.T) {
	r := leaseRepo(t)
	r.claim(31, "full", active, "docs")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("x\n\nWaggle-Ticket: %0A::warning::x,y\r::error::z")
	for _, gha := range []bool{true, false} {
		env := map[string]string{}
		if gha {
			env["GITHUB_ACTIONS"] = "true"
		}
		for _, branch := range []string{"t-031-%0A::warning::x,y\r\n::error::z", "evil%\r\n::error::z,y"} {
			code, out := r.runWith(env, "verify", "--branch", branch)
			if code != 1 || strings.Contains(out, "\r") || strings.Contains(out, "evil") {
				t.Errorf("gha %v: exit %d %q", gha, code, out)
			}
			for _, line := range strings.Split(out, "\n") {
				if strings.HasPrefix(line, "::warning::x") || strings.HasPrefix(line, "::error::z") || strings.HasPrefix(line, "x,y") {
					t.Errorf("gha %v: injected line %q", gha, line)
				}
			}
		}
	}
}

// baseMergeRepo is main with a go.sum, a claim for docs on T-001 and a branch
// with one ticket commit.
func baseMergeRepo(t *testing.T) gateRepo {
	t.Helper()
	r := leaseRepo(t)
	r.git("checkout", "-q", "main")
	r.write("go.sum", []byte("v0\n"))
	r.commit("base: go.sum")
	r.git("checkout", "-q", "branch")
	r.git("reset", "-q", "--hard", "main")
	r.claim(1, "full", active, "docs")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-001: a")
	return r
}

func (r gateRepo) advanceMain(file, content string) string {
	r.t.Helper()
	r.git("checkout", "-q", "main")
	r.write(file, []byte(content))
	r.commit("base: " + file)
	oid := r.git("rev-parse", "HEAD")
	r.git("checkout", "-q", "branch")
	return oid
}

func TestBaseMerges(t *testing.T) {
	// An "Update branch" merge passes; so do two in a row.
	r := baseMergeRepo(t)
	r.advanceMain("go.sum", "v1\n")
	r.git("merge", "-q", "--no-ff", "-m", "Merge branch 'main' into branch", "main")
	r.expect(t, 0, "leases: ok\n", "tiers: ok\n")
	r.advanceMain("go.sum", "v2\n")
	r.git("merge", "-q", "--no-ff", "-m", "Merge branch 'main' into branch", "main")
	r.expect(t, 0, "leases: ok\n", "tiers: ok\n")
	r.write("docs/b.md", []byte("b\n"))
	r.commit("T-001: b")
	r.expect(t, 0, "leases: ok\n")

	// A resolution that edits a base-side file outside the lease fails.
	r = baseMergeRepo(t)
	r.advanceMain("other/y.md", "y\n")
	r.git("merge", "-q", "--no-ff", "--no-commit", "main")
	r.write("other/x.md", []byte("edited in the merge\n"))
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "Merge branch 'main' into branch")
	r.expect(t, 1, "leases: FAIL: other/x.md: T-001: path is outside")

	// A merge with nothing to bind and no first-parent ticket fails as unbound.
	r = leaseRepo(t)
	r.claim(1, "full", active, "docs")
	r.advanceMain("other/y.md", "y\n")
	r.git("merge", "-q", "--no-ff", "-m", "Merge main", "main")
	r.expect(t, 1, "commit has no T-<id>: prefix")

	// A merge whose second parent is not on base, and an octopus merge.
	const refuse = "in PR; rebase onto main (only an update from the base branch is accepted)"
	r = baseMergeRepo(t)
	r.git("checkout", "-q", "-b", "side1", "main")
	r.write("docs/s1.md", []byte("1\n"))
	r.commit("T-001: s1")
	r.git("checkout", "-q", "-b", "side2", "main")
	r.write("docs/s2.md", []byte("2\n"))
	r.commit("T-001: s2")
	r.git("checkout", "-q", "branch")
	r.git("merge", "-q", "--no-ff", "-m", "T-001: merge side", "side1")
	r.expect(t, 1, refuse)
	r = baseMergeRepo(t)
	r.git("checkout", "-q", "-b", "side1", "main")
	r.write("docs/s1.md", []byte("1\n"))
	r.commit("T-001: s1")
	r.git("checkout", "-q", "-b", "side2", "main")
	r.write("docs/s2.md", []byte("2\n"))
	r.commit("T-001: s2")
	r.git("checkout", "-q", "branch")
	r.git("merge", "-q", "--no-ff", "-m", "T-001: octopus", "side1", "side2")
	r.expect(t, 1, refuse)
}

// Crafted merges whose tree reverts a base-side Tier 3 file must be refused.
func TestBaseMergeCraftedShapes(t *testing.T) {
	const refuse = "in PR; rebase onto main (only an update from the base branch is accepted)"

	// (i) The PR merges a newer base commit, then makes a second merge whose
	// second parent is an older base commit.
	r := baseMergeRepo(t)
	older := r.git("rev-parse", "main")
	r.advanceMain("go.sum", "v1\n")
	r.git("merge", "-q", "--no-ff", "-m", "Merge branch 'main' into branch", "main")
	r.git("read-tree", "HEAD")
	r.git("update-index", "--cacheinfo", "100644,"+r.git("rev-parse", older+":go.sum")+",go.sum")
	tree := r.git("write-tree")
	head := r.git("rev-parse", "HEAD")
	merge := r.git("commit-tree", tree, "-p", head, "-p", older, "-m", "Merge older base")
	r.git("update-ref", "refs/heads/branch", merge)
	r.git("reset", "-q", "--hard", "branch")
	if got := r.git("show", "HEAD:go.sum"); got != "v0" {
		t.Fatalf("crafted tree go.sum = %q", got)
	}
	r.expect(t, 1, refuse)

	// (ii) The PR sits on an older base commit; the second parent is a side
	// commit that base merged later. Its merge-bases with base are two commits.
	r = leaseRepo(t)
	r.git("checkout", "-q", "main")
	r.write("go.sum", []byte("v0\n"))
	r.commit("base: go.sum")
	r.git("checkout", "-q", "-b", "side", "main~1")
	r.write("docs/s.md", []byte("s\n"))
	r.commit("T-001: side")
	r.git("checkout", "-q", "branch")
	r.git("reset", "-q", "--hard", "main")
	r.claim(1, "full", active, "docs")
	r.write("docs/a.md", []byte("a\n"))
	r.commit("T-001: a")
	r.git("checkout", "-q", "main")
	r.git("merge", "-q", "--no-ff", "-m", "base: merge side", "side")
	r.write("go.sum", []byte("v2\n"))
	r.commit("base: go.sum v2")
	r.git("checkout", "-q", "branch")
	r.git("merge", "-q", "--no-ff", "-m", "Merge side", "side")
	if n := len(strings.Fields(r.git("merge-base", "--all", "HEAD", "main"))); n != 2 {
		t.Fatalf("setup: %d merge-bases; want 2", n)
	}
	r.expect(t, 1, refuse)
}
