//go:build unix

package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestHelperProcess is the helper-process pattern: the test binary, started
// with GO_WANT_HELPER_PROCESS=1, plays a worker's child. It never runs as a test.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	a := os.Args[slices.Index(os.Args, "--")+1:]
	pid := []byte(strconv.Itoa(os.Getpid()))
	switch a[0] {
	case "session": // setsid like Claude Code's Bash tool, record the pid, outlive the leader, then leave a marker
		_, _ = syscall.Setsid()
		_ = os.WriteFile(a[1], pid, 0o644)
		for range 600 { // the marker follows only a release the test writes after Run; a survivor never outlives 60 s (D-195)
			if _, err := os.Stat(a[3]); err == nil {
				_ = os.WriteFile(a[2], pid, 0o644)
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	case "blink": // a session leader that is gone before any poll
		_, _ = syscall.Setsid()
		time.Sleep(20 * time.Millisecond)
	case "daemon": // start a session leader that holds stdout, then exit at once
		c := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$", "--", "sleeper")
		c.Env, c.Stdout, c.SysProcAttr = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1"), os.Stdout, &syscall.SysProcAttr{Setsid: true}
		if err := c.Start(); err == nil {
			_ = os.WriteFile(a[1], []byte(strconv.Itoa(c.Process.Pid)), 0o644)
		}
	case "sid": // report the session and the leader's pid, and whether joining the test's group fails
		g, _ := strconv.Atoi(a[2])
		_ = os.WriteFile(a[1], []byte(fmt.Sprint(getsid(), " ", os.Getppid(), " ", syscall.Setpgid(0, g) == syscall.EPERM)), 0o644)
	case "toolshell": // a tool shell: own session, a background child, then exit
		_, _ = syscall.Setsid()
		_ = exec.Command("sleep", "30").Start()
	case "sleeper":
		time.Sleep(60 * time.Second)
	}
	os.Exit(0)
}

// helper is the shell command line for one helper mode.
func helper(t *testing.T, mode string, args ...string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return "GO_WANT_HELPER_PROCESS=1 '" + exe + "' -test.run='^TestHelperProcess$' -- " + mode + " " + strings.Join(args, " ")
}

// pidFrom reads a pid file the helper wrote; a missing file fails the test (never a skip).
func pidFrom(t *testing.T, path string) int {
	t.Helper()
	var pid int
	if !wait(func() bool { b, _ := os.ReadFile(path); pid, _ = strconv.Atoi(string(b)); return pid > 1 }) {
		t.Fatalf("the helper process did not start: no pid in %s", path)
	}
	return pid
}

func gone(pid int) bool { return wait(func() bool { return syscall.Kill(pid, 0) != nil }) }

func fastPoll(t *testing.T, d time.Duration) {
	old := pollEvery
	pollEvery = d
	t.Cleanup(func() { pollEvery = old })
}

// D-187: a child that setsid()s out of the worker's group and outlives the
// leader is killed after the leader is reaped, and recorded.
func TestSurvivorKilledAndRecorded(t *testing.T) {
	fastPoll(t, 30*time.Millisecond)
	f := newFixture(t, false, false)
	dir := t.TempDir()
	pidFile, marker, release := filepath.Join(dir, "pid"), filepath.Join(dir, "marker"), filepath.Join(dir, "release")
	res, err := f.run(helper(t, "session", pidFile, marker, release) + " &\nsleep 0.7\n" + okScenario)
	pid := pidFrom(t, pidFile)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
	if len(res.Survivors) != 1 || res.Survivors[0].PID != pid || !strings.Contains(res.Survivors[0].Command, "dispatch") {
		t.Fatalf("survivors %v, want pid %d", res.Survivors, pid)
	}
	if !gone(pid) {
		t.Fatalf("survivor %d still runs after Run", pid)
	}
	b, _ := os.ReadFile(filepath.Join(res.RunDir, "result.json"))
	if !strings.Contains(string(b), `"survivors_killed": [`) || !strings.Contains(strings.Join(res.Warnings, "|"), "survivors_killed") {
		t.Fatalf("survivor missing from result.json or the warnings: %s", b)
	}
	_ = os.WriteFile(release, nil, 0o644)
	time.Sleep(500 * time.Millisecond)
	if _, e := os.Stat(marker); e == nil {
		t.Fatal("the killed survivor still wrote its marker")
	}
}

// A child spawned and exited between two polls is gone before the sweep: nothing
// to kill, nothing recorded.
func TestShortLivedChildHarmless(t *testing.T) {
	fastPoll(t, time.Hour) // only the first poll, before the child exists
	f := newFixture(t, false, false)
	res, err := f.run(helper(t, "blink") + "\n" + okScenario)
	if err != nil || res.ExitCode != 0 || len(res.Survivors) != 0 || strings.Contains(strings.Join(res.Warnings, "|"), "survivors_killed") {
		t.Fatalf("exit %d survivors %v warnings %v err %v", res.ExitCode, res.Survivors, res.Warnings, err)
	}
}

// Item 5 and fable N2/B2: a process that escaped unseen (a daemon whose parent
// is gone before any poll) holds stdout. Run returns about a second after the
// leader is reaped, not at the 120 minute default timeout. Linux adopts the
// orphan as a subreaper and kills it; elsewhere the pipe is cut and warned.
// The test fails, never skips, when the helper cannot start.
func TestStdoutHolder(t *testing.T) {
	fastPoll(t, time.Hour)
	f := newFixture(t, false, false)
	pidFile := filepath.Join(t.TempDir(), "pid")
	start := time.Now()
	res, err := f.run(helper(t, "daemon", pidFile) + "\nsleep 0.3\n" + okScenario)
	took := time.Since(start)
	pidFrom(t, pidFile) // a survivor that was not killed exits on its own after 60 s (D-195)
	held := strings.Contains(strings.Join(res.Warnings, "|"), "stdout_held")
	if err != nil || took > 8*time.Second || (goruntime.GOOS == "linux") == held || (goruntime.GOOS == "linux" && len(res.Survivors) != 1) {
		t.Fatalf("took %v exit %d held %v survivors %v warnings %v err %v", took, res.ExitCode, held, res.Survivors, res.Warnings, err)
	}
}

// secondFixture is a builder run and a first guard run on one base, with an
// alternate model so the second guard has somewhere to go.
func secondFixture(t *testing.T, firstGuard bool) (*fixture, Result) {
	f := guardFixture(t, false)
	cfg, _ := os.ReadFile(filepath.Join(f.real, ".waggle/control/config.yaml"))
	write(t, f.real, ".waggle/control/config.yaml", strings.Replace(string(cfg), "        fast: haiku\n", "        fast: haiku\n      alternates:\n        standard: haiku\n", 1))
	if !firstGuard {
		return f, Result{}
	}
	first, err := f.guard(emit(assistant(100), verdictEvent(`Verdict: must-fix`)))
	if err != nil || first.ExitCode != 0 {
		t.Fatalf("first guard: exit %d err %v", first.ExitCode, err)
	}
	return f, first
}

func (f *fixture) second(mut ...func(*Request, *Deps)) (Result, error) {
	return f.guard(emit(assistant(100), verdictEvent(`Verdict: pass`)), append([]func(*Request, *Deps){func(r *Request, _ *Deps) { r.Second = true }}, mut...)...)
}

// D-188: --second exits 3 before launch when the builder or first guard record
// is missing, when the base moved, or when the model repeats; result.json
// carries must_fix.
func TestSecondGuardRefusals(t *testing.T) {
	refused := func(t *testing.T, f *fixture, want string, mut ...func(*Request, *Deps)) {
		t.Helper()
		runs := len(sh(t, f.real, "for-each-ref", "refs/"))
		res, err := f.second(mut...)
		if code(err) != ExitEnv || !strings.Contains(err.Error(), want) || res.RunDir != "" || len(sh(t, f.real, "for-each-ref", "refs/")) != runs {
			t.Fatalf("want exit 3 containing %q before launch, got exit %d err %v run %q", want, code(err), err, res.RunDir)
		}
	}
	t.Run("no first guard", func(t *testing.T) {
		f, _ := secondFixture(t, false)
		refused(t, f, "first guard run")
	})
	t.Run("no builder", func(t *testing.T) {
		f, _ := secondFixture(t, true)
		if err := os.RemoveAll(filepath.Join(f.real, ".waggle/worktrees/runs/T-023-builder-1")); err != nil {
			t.Fatal(err)
		}
		refused(t, f, "builder run")
	})
	t.Run("base moved", func(t *testing.T) {
		f, _ := secondFixture(t, true)
		write(t, f.real, "README", "moved\n")
		f.git("commit", "-q", "-am", "move main")
		refused(t, f, "differs from the recorded runs")
	})
	t.Run("model repeats", func(t *testing.T) {
		f, _ := secondFixture(t, true)
		res, err := f.second()
		if err != nil || res.ExitCode != 0 || !res.Second {
			t.Fatalf("second guard: exit %d err %v", res.ExitCode, err)
		}
		p := filepath.Join(f.real, ".waggle/worktrees/runs/T-023-builder-1/result.json")
		var m map[string]any
		b, _ := os.ReadFile(p)
		if json.Unmarshal(b, &m) != nil {
			t.Fatal("builder result.json")
		}
		m["model"] = res.Model
		b, _ = json.Marshal(m)
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		refused(t, f, "would use "+res.Model)
	})
	// Opus P8: the queen commits the first guard's must-fix review and dispatches
	// --second with the default base. The count grew, so the models would have
	// walked down to the builder's; now it is exit 3, and the builder's base works.
	t.Run("review committed between the guards", func(t *testing.T) {
		f, first := secondFixture(t, true)
		write(t, f.real, ".waggle/hive/reviews/T-023-opus.md", "Verdict: must-fix\n")
		f.git("add", "-A")
		f.git("commit", "-q", "-m", "review")
		refused(t, f, "differs from the recorded runs")
		res, err := f.second(func(r *Request, _ *Deps) { r.Base = first.BaseOID })
		if err != nil || res.ExitCode != 0 || res.MustFix != first.MustFix {
			t.Fatalf("pinned base: exit %d must_fix %d (first %d) err %v", res.ExitCode, res.MustFix, first.MustFix, err)
		}
	})
}

// D-188: result.json records the must-fix count the step-up used.
func TestResultMustFix(t *testing.T) {
	f := newFixture(t, false, false)
	write(t, f.real, ".waggle/hive/reviews/T-023.md", "Verdict: must-fix\n")
	f.git("add", "-A")
	f.git("commit", "-q", "-m", "review")
	res, err := f.run(okScenario)
	b, _ := os.ReadFile(filepath.Join(res.RunDir, "result.json"))
	if err != nil || res.MustFix != 1 || !strings.Contains(string(b), `"must_fix": 1`) {
		t.Fatalf("must_fix %d err %v: %s", res.MustFix, err, b)
	}
}

// A guard takes no --dir or --gate (exit 2), and Head is only for guards.
func TestGuardValidate(t *testing.T) {
	for name, mut := range map[string]func(*Request){"dir": func(r *Request) { r.Dirs = []string{"internal/x"} }, "gate": func(r *Request) { r.Gate = "light" }} {
		req := Request{RepoDir: ".", Ticket: "23", Role: "guard"}
		mut(&req)
		if _, err := req.Validate(); code(err) != ExitUsage {
			t.Errorf("guard with %s: exit %d %v", name, code(err), err)
		}
	}
	b := okReq(".")
	b.Head = "abc"
	if v, err := b.Validate(); err != nil || v.Head != "" {
		t.Fatalf("builder Head %q err %v", v.Head, err)
	}
}

// The guard records the builder's claim OID it read.
func TestGuardRecordsClaimOID(t *testing.T) {
	f := guardFixture(t, false)
	claim := f.claimRef()
	res, err := f.guard(emit(assistant(100), verdictEvent(`Verdict: pass`)))
	if err != nil || res.ClaimOID != claim {
		t.Fatalf("claim_oid %q, want %q (err %v)", res.ClaimOID, claim, err)
	}
}

// Fable B1: a process that writes after the compare (here, while the worktree is
// removed) is not absorbed into the baseline: the next dispatch exits 3.
func TestHostRecordKeepsVerifiedSnapshot(t *testing.T) {
	f := newFixture(t, false, false)
	hook, cfg := filepath.Join(f.common(), "hooks/post-checkout"), filepath.Join(f.common(), "config")
	bin := filepath.Join(t.TempDir(), "gitwrap")
	script := "#!/bin/sh\ncase \"$*\" in *worktree*remove*) mkdir -p '" + filepath.Dir(hook) + "'; printf '#!/bin/sh\\n' > '" + hook +
		"'; printf '[remote \"evil\"]\\n\\turl = x\\n' >> '" + cfg + "';; esac\nexec git \"$@\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if res, err := f.run(okScenario, func(_ *Request, d *Deps) { d.GitBin = bin }); err != nil || res.ExitCode != 0 {
		t.Fatalf("run 1: exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
	_, err := f.run(commitFile("internal/x/c.go", "ok") + emit(assistant(100), okResult))
	if code(err) != ExitEnv || !strings.Contains(err.Error(), "hooks/post-checkout") || !strings.Contains(err.Error(), "config.keys") {
		t.Fatalf("run 2 must exit 3 naming the late writes: exit %d %v", code(err), err)
	}
}

// Opus N1: after a run that failed the host check the record still holds the
// verified snapshot, so the next dispatch of any ticket exits 3.
func TestHostRecordAfterViolation(t *testing.T) {
	f := newFixture(t, false, false)
	hook := filepath.Join(f.common(), "hooks/post-checkout")
	res, err := f.run("mkdir -p '" + filepath.Dir(hook) + "'\nprintf '#!/bin/sh\\n' > '" + hook + "'\n" + okScenario)
	if err != nil || res.ExitCode != 1 || !hasViolation(res.Violations, ruleHost, "hooks/post-checkout") {
		t.Fatalf("run 1: exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
	_, err = f.run(okScenario, func(r *Request, _ *Deps) { r.Ticket, r.Branch = "24", "" })
	if code(err) != ExitEnv || !strings.Contains(err.Error(), "since run T-023-builder-1") {
		t.Fatalf("run 2 (T-024): exit %d %v", code(err), err)
	}
}

// D-189: branch.<name>.* in the common config does not trip the record; any
// other key (a remote) does.
func TestHostRecordIgnoresBranchKeys(t *testing.T) {
	f := newFixture(t, false, false)
	f.git("remote", "add", "origin", "https://example.invalid/o.git")
	if res, err := f.run(okScenario); err != nil || res.ExitCode != 0 {
		t.Fatalf("run 1: exit %d err %v", res.ExitCode, err)
	}
	f.git("config", "--local", "branch.t-023.remote", "origin")
	f.git("config", "--local", "branch.t-023.merge", "refs/heads/t-023")
	if res, err := f.run(commitFile("internal/x/c.go", "ok") + emit(assistant(100), okResult)); err != nil || res.ExitCode != 0 {
		t.Fatalf("run 2 after a branch section: exit %d err %v", res.ExitCode, err)
	}
	f.git("remote", "add", "evil", "https://example.invalid/x.git")
	_, err := f.run(commitFile("internal/x/d.go", "ok") + emit(assistant(100), okResult))
	if code(err) != ExitEnv || !strings.Contains(err.Error(), "config.keys") {
		t.Fatalf("run 3 after a new remote: exit %d %v", code(err), err)
	}
}
