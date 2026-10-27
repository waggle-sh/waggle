// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// Test 20 (and 26): host state changes are exit 1, nothing pushed, the worktree
// tainted, and no planted filter runs on the host.
func TestRunHostState(t *testing.T) {
	type hcase struct {
		name, path string
		script     func(f *fixture, canary string) string
		pre        func(f *fixture)
		notInspect bool
	}
	cases := []hcase{
		{name: "new admin entry", path: "worktrees/planted", script: func(f *fixture, _ string) string {
			// A valid-looking entry: git prunes one whose gitdir file is missing.
			return okScenario + "mkdir '" + f.common() + "/worktrees/planted'\nprintf '%s\\n' \"$PWD/.git\" > '" + f.common() + "/worktrees/planted/gitdir'\n"
		}},
		{name: "own commondir rewritten", path: "worktrees/T-023-builder/commondir", script: func(f *fixture, canary string) string {
			return okScenario + "echo '" + filterRepo(f.t, canary) + "' > '" + f.common() + "/worktrees/T-023-builder/commondir'\n"
		}},
		{name: "own gitdir rewritten", path: "worktrees/T-023-builder/gitdir", script: func(f *fixture, _ string) string {
			return okScenario + "echo /nowhere/.git > '" + f.common() + "/worktrees/T-023-builder/gitdir'\n"
		}},
		{name: "another entry HEAD", path: "worktrees/other/HEAD",
			pre: func(f *fixture) {
				f.git("worktree", "add", "-q", filepath.Join(filepath.Dir(f.real), "other"), "-b", "other")
			},
			script: func(f *fixture, _ string) string {
				return okScenario + "echo 'ref: refs/heads/main' > '" + f.common() + "/worktrees/other/HEAD'\n"
			}},
		{name: "dot git rewritten", path: ".git", notInspect: true, script: func(f *fixture, canary string) string {
			return okScenario + "printf 'gitdir: %s\\n' '" + filterRepo(f.t, canary) + "' > .git\n"
		}},
		{name: "clean filter in config", path: "config", notInspect: true, script: func(f *fixture, canary string) string {
			// The agent defines the filter last, so only a host call can fire it.
			return "echo '* filter=x' > .gitattributes\ngit add .gitattributes\ngit -c user.name=a -c user.email=a@e.co commit -q -s -m 'T-023: attr'\n" +
				okScenario + "touch -t 203501010000 internal/x/a.go\ngit config filter.x.clean \"sh -c 'touch " + canary + "; cat'\"\n"
		}},
		{name: "info exclude", path: "info/exclude", notInspect: true, script: func(f *fixture, _ string) string {
			return okScenario + "echo '*' > '" + f.common() + "/info/exclude'\necho x > internal/x/hidden.go\n"
		}},
		{name: "pushurl", path: "config", script: func(*fixture, string) string {
			return okScenario + "git config remote.origin.pushurl /tmp/nowhere\n"
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			eachRepoSpelling(t, func(t *testing.T, link bool) {
				f := newFixture(t, true, link)
				if c.pre != nil {
					c.pre(f)
				}
				canary := filepath.Join(t.TempDir(), "canary")
				res, err := f.run(c.script(f, canary))
				if err != nil || res.ExitCode != 1 || res.Pushed || f.pushed("t-023") || !hasViolation(res.Violations, ruleHost, c.path) {
					t.Fatalf("exit %d err %v violations %v", res.ExitCode, err, res.Violations)
				}
				if _, err := os.Stat(canary); err == nil {
					t.Fatal("a planted filter ran on the host")
				}
				if c.notInspect && !strings.Contains(strings.Join(violationDetails(res.Violations), "|"), "not inspected") {
					t.Fatalf("status was not skipped: %v", res.Violations)
				}
				if _, err := os.Stat(res.Worktree + ".tainted"); err != nil {
					t.Fatal("worktree not tainted")
				}
				_, err = f.run(okScenario)
				if code(err) != ExitEnv || !strings.Contains(err.Error(), "host state was changed") {
					t.Fatalf("next dispatch: %v", err)
				}
			})
		})
	}
}

func violationDetails(vs []comb.Violation) (out []string) {
	for _, v := range vs {
		out = append(out, v.Detail)
	}
	return out
}

// Test 22 (and 26): a base on the branch is refused before anything is
// created; an older main commit is accepted; a branch known only as
// <remote>/<branch> is recreated locally and pushed without force.
func TestRunBaseAndBranch(t *testing.T) {
	eachRepoSpelling(t, func(t *testing.T, link bool) {
		f := newFixture(t, true, link)
		old := f.git("rev-parse", "HEAD")
		f.git("checkout", "-q", "-b", "t-023")
		write(t, f.real, "internal/x/first.go", "package x\n")
		f.git("add", "-A")
		f.git("commit", "-q", "-s", "-m", "T-023: first")
		first := f.git("rev-parse", "HEAD")
		f.git("checkout", "-q", "main")
		write(t, f.real, "README", "moved\n")
		f.git("commit", "-q", "-a", "-m", "main moves")
		f.git("push", "-q", "origin", "main", "t-023")
		f.git("fetch", "-q", "origin")
		for _, base := range []string{"heads/t-023", "t-023~0", first, "origin/t-023"} {
			_, err := f.run(okScenario, func(r *Request, _ *Deps) { r.Base = base })
			if code(err) != ExitUsage || !strings.Contains(err.Error(), "base is on the branch") {
				t.Fatalf("base %s: %v", base, err)
			}
			if _, serr := os.Stat(filepath.Join(f.real, ".waggle/worktrees/T-023-builder")); serr == nil {
				t.Fatalf("base %s created a worktree", base)
			}
		}
		if sh(t, f.real, "for-each-ref", "refs/waggle/claims/") != "" {
			t.Fatal("a refused base wrote a claim")
		}
		// An older main commit is accepted.
		res, err := f.run(okScenario, func(r *Request, _ *Deps) { r.Base = old })
		if err != nil || res.ExitCode != 0 || res.CheckBaseOID != old {
			t.Fatalf("older base: exit %d check base %s err %v violations %v", res.ExitCode, res.CheckBaseOID, err, res.Violations)
		}
	})
	t.Run("remote tracking only", func(t *testing.T) {
		f := newFixture(t, true, false)
		f.git("checkout", "-q", "-b", "t-023")
		write(t, f.real, "internal/x/first.go", "package x\n")
		f.git("add", "-A")
		f.git("commit", "-q", "-s", "-m", "T-023: first")
		first := f.git("rev-parse", "HEAD")
		f.git("push", "-q", "origin", "t-023")
		f.git("checkout", "-q", "main")
		f.git("branch", "-q", "-D", "t-023")
		res, err := f.run(okScenario)
		if err != nil || res.ExitCode != 0 || !res.Pushed {
			t.Fatalf("exit %d err %v violations %v", res.ExitCode, err, res.Violations)
		}
		if sh(t, f.origin, "merge-base", "--is-ancestor", first, "refs/heads/t-023") != "" {
			t.Fatal("first commit lost")
		}
		if sh(t, f.origin, "rev-parse", "refs/heads/t-023") != res.HeadOID || f.git("rev-parse", "refs/heads/t-023") != res.HeadOID {
			t.Fatal("remote and local branch differ from the result")
		}
	})
}

// gitWrapper logs every git call to log and runs the real git.
func gitWrapper(t *testing.T) (bin, log string) {
	dir := t.TempDir()
	bin, log = filepath.Join(dir, "gitwrap"), filepath.Join(dir, "calls.log")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho \"$@\" >> '"+log+"'\nexec git \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, log
}

func claimFetches(log string) int {
	b, _ := os.ReadFile(log)
	n := 0
	for _, l := range strings.Split(string(b), "\n") {
		if strings.Contains(l, "fetch") && strings.Contains(l, "refs/waggle/claims") {
			n++
		}
	}
	return n
}

// Test 23: refused directories cost no fetch and write nothing; a remote run
// does exactly two claim fetches before launch; another ticket's claim mirror
// moved during the run is restored, not a violation.
func TestRunClaims(t *testing.T) {
	f := newFixture(t, true, false)
	bin, log := gitWrapper(t)
	for _, d := range []string{".github/workflows", ".waggle/hive"} {
		_, err := f.run(okScenario, withDirs(d), func(_ *Request, dd *Deps) { dd.GitBin = bin })
		if code(err) != ExitClaim {
			t.Fatalf("%s: %v", d, err)
		}
	}
	if n := claimFetches(log); n != 0 {
		t.Fatalf("%d claim fetches for refused directories", n)
	}
	if b, _ := os.ReadFile(log); strings.Contains(string(b), "fetch") {
		t.Fatal("a refused directory fetched")
	}
	if _, err := os.Stat(filepath.Join(f.real, ".waggle/worktrees")); err == nil {
		t.Fatal("a refused directory created .waggle/worktrees")
	}

	// Another keeper's claim for ticket 24, mirrored locally.
	bob, err := comb.NewGitBackend(comb.Options{RepoDir: f.real, Remote: "origin", DefaultTTL: 3 * time.Hour,
		ExtraEnv: []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := bob.AcquireLease(context.Background(), comb.Lease{TicketID: "T-024", KeeperID: "bob", Role: "builder", Gate: "full", Directories: []string{"internal/y"}}); err != nil {
		t.Fatal(err)
	}
	mirror := f.git("rev-parse", "refs/waggle/claims/ticket-24")
	snap := filepath.Join(t.TempDir(), "snap.log")
	res, err := f.run("cp '"+log+"' '"+snap+"'\n"+okScenario+"git update-ref refs/waggle/claims/ticket-24 HEAD\n",
		func(_ *Request, dd *Deps) { dd.GitBin = bin })
	if err != nil || res.ExitCode != 0 || len(res.Violations) != 0 {
		t.Fatalf("exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
	if n := claimFetches(snap); n != 2 {
		t.Fatalf("%d claim fetches before launch, want 2", n)
	}
	if got := f.git("rev-parse", "refs/waggle/claims/ticket-24"); got != mirror {
		t.Fatalf("mirror not restored: %s, want %s", got, mirror)
	}
}

// Test 24: one dispatch run per repo at a time.
func TestRunDispatchLock(t *testing.T) {
	f := newFixture(t, false, false)
	host, _ := os.Hostname()
	lock := filepath.Join(f.real, ".waggle/worktrees/dispatch.lock")
	body := func(pid string) string {
		return `{"pid":` + pid + `,"host":"` + host + `","started":"2026-10-18T12:00:00Z"}`
	}
	write(t, f.real, ".waggle/worktrees/dispatch.lock", body(itoa(os.Getpid())))
	_, err := f.run(okScenario)
	if code(err) != ExitEnv || !strings.Contains(err.Error(), "another dispatch run is in progress") {
		t.Fatalf("held lock: %v", err)
	}
	if sh(t, f.real, "for-each-ref", "refs/waggle/claims/", "refs/heads/t-023") != "" {
		t.Fatal("a refused run wrote a ref")
	}
	if _, serr := os.Stat(filepath.Join(f.real, ".waggle/worktrees/T-023-builder")); serr == nil {
		t.Fatal("a refused run created a worktree")
	}
	old := pidAlive
	t.Cleanup(func() { pidAlive = old })
	pidAlive = func(int) bool { return false }
	write(t, f.real, ".waggle/worktrees/dispatch.lock", body("999999"))
	res, err := f.run(okScenario)
	if err != nil || res.ExitCode != 0 || !strings.Contains(strings.Join(res.Warnings, "|"), "took over stale run lock") {
		t.Fatalf("stale lock: exit %d warnings %v err %v", res.ExitCode, res.Warnings, err)
	}
	if _, serr := os.Stat(lock); serr == nil {
		t.Fatal("dispatch.lock left behind")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}

// Test 28 (integration half): the Keeper's GIT_CONFIG_* variables never reach
// a post-run git call, so a filter they name never fires on a stat-dirty file.
func TestRunGitEnvDropped(t *testing.T) {
	f := newFixture(t, false, false)
	canary := filepath.Join(t.TempDir(), "canary")
	write(t, f.real, ".gitattributes", "* filter=x\n")
	f.git("add", ".gitattributes")
	f.git("commit", "-q", "-m", "attributes")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "filter.x.clean")
	t.Setenv("GIT_CONFIG_VALUE_0", "sh -c 'touch "+canary+"; cat'")
	t.Setenv("GIT_EXEC_PATH", f.git("--exec-path"))
	res, err := f.run("touch -t 203501010000 internal/x/a.go\n" + okScenario)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
	if _, err := os.Stat(canary); err == nil {
		t.Fatal("a filter named by the Keeper's environment ran in a post-run git call")
	}
}
