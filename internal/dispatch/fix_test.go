//go:build unix

package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// canaryScript returns a script that records its own run and then execs argv
// (or fails, when exec is empty), and the canary file it creates.
func canaryScript(t *testing.T, exec string) (script, canary string) {
	t.Helper()
	dir := t.TempDir()
	script, canary = filepath.Join(dir, "canary.sh"), filepath.Join(dir, "ran")
	body := "#!/bin/sh\ntouch '" + canary + "'\n" + exec + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script, canary
}

func noCanary(t *testing.T, canary, what string) {
	t.Helper()
	if _, err := os.Stat(canary); err == nil {
		t.Fatalf("%s ran on the host", what)
	}
}

// Fix 1: the OID that was checked is the OID that is recorded and pushed, even
// when the branch moves after the checks started (an escaped child).
func TestFixPushesCheckedOID(t *testing.T) {
	f := newFixture(t, true, false)
	dir := t.TempDir()
	late, mark, wrap := filepath.Join(dir, "late"), filepath.Join(dir, "mark"), filepath.Join(dir, "gitwrap")
	body := "#!/bin/sh\ncase \"$*\" in *diff-tree*) if [ ! -e '" + mark + "' ]; then touch '" + mark + "'; git -C '" + f.real +
		"' update-ref refs/heads/t-023 $(cat '" + late + "'); fi;; esac\nexec git \"$@\"\n"
	if err := os.WriteFile(wrap, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	script := okScenario + "git -c user.name=a -c user.email=a@e.co commit-tree 'HEAD^{tree}' -p HEAD -m 'T-023: late' -m 'Signed-off-by: a <a@e.co>' > '" + late + "'\n"
	res, err := f.run(script, func(_ *Request, d *Deps) { d.GitBin = wrap })
	if err != nil || res.ExitCode != 0 || !res.Pushed {
		t.Fatalf("exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
	moved, _ := os.ReadFile(late)
	if f.git("rev-parse", "refs/heads/t-023") != strings.TrimSpace(string(moved)) {
		t.Fatal("the branch was not moved after the check; the test proves nothing")
	}
	if got := sh(t, f.origin, "rev-parse", "refs/heads/t-023"); got != res.HeadOID || got == strings.TrimSpace(string(moved)) {
		t.Fatalf("remote holds %s, checked %s, moved %s", got, res.HeadOID, moved)
	}
}

// Fix 2: a config change by the worker skips the final claim sync, so a planted
// uploadpack never runs.
func TestFixClaimSyncSkipped(t *testing.T) {
	f := newFixture(t, true, false)
	up, canary := canaryScript(t, `exec git-upload-pack "$@"`)
	res, err := f.run(okScenario + "git config remote.origin.uploadpack '" + up + "'\n")
	if err != nil || res.ExitCode != 1 || !hasViolation(res.Violations, ruleHost, "config") || !strings.Contains(strings.Join(res.Warnings, "|"), "claim_sync_skipped") {
		t.Fatalf("exit %d err %v warnings %v violations %v", res.ExitCode, err, res.Warnings, res.Violations)
	}
	noCanary(t, canary, "the planted uploadpack")
}

// Fix 3: the dispatch-side Comb backend runs no hooks, even one the Keeper's
// repo already has.
func TestFixCombRunsNoHooks(t *testing.T) {
	f := newFixture(t, false, false)
	hook, canary := canaryScript(t, "")
	if err := os.Rename(hook, filepath.Join(f.common(), "hooks", "reference-transaction")); err != nil {
		t.Fatal(err)
	}
	res, err := f.run("sleep 0.5\n"+emit(assistant(100), okResult), func(_ *Request, d *Deps) { d.KeepaliveEvery = 100 * time.Millisecond })
	if err != nil || res.End != "no_change" { // the worker commits nothing: its own git would run the hook
		t.Fatalf("end %s err %v violations %v", res.End, err, res.Violations)
	}
	noCanary(t, canary, "reference-transaction")
}

// Fix 4: once the worker changed a host file, claim renewals stop.
func TestFixKeepaliveGate(t *testing.T) {
	f := newFixture(t, false, false)
	b, _ := claimEnv(t, f.real)
	var cb = &countBackend{ClaimBackend: b}
	res, err := f.run("echo x > '"+f.common()+"/hooks/zz'\nsleep 1\n"+okScenario, func(_ *Request, d *Deps) { d.Backend, d.KeepaliveEvery = cb, 100*time.Millisecond })
	if err != nil || res.ExitCode != 1 || cb.leases.Load() != 1 || !strings.Contains(strings.Join(res.Warnings, "|"), "renewal skipped") {
		t.Fatalf("exit %d leases %d warnings %v err %v", res.ExitCode, cb.leases.Load(), res.Warnings, err)
	}
}

// Fix 5: the global ignore and attributes files are hashed, so ignoring the
// whole main checkout is a host_state violation, not a hidden stray file.
func TestFixGlobalIgnore(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	f := newFixture(t, true, false)
	res, err := f.run(okScenario + "mkdir -p '" + xdg + "/git'\necho '*' > '" + xdg + "/git/ignore'\necho x > '" + f.real + "/stray.txt'\n")
	if err != nil || res.ExitCode != 1 || res.Pushed || !hasViolation(res.Violations, ruleHost, "global:"+xdg+"/git/ignore") {
		t.Fatalf("exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
}

// Fix 6: log.showSignature from the repo config never runs the worker's gpg.program.
func TestFixShowSignature(t *testing.T) {
	f := newFixture(t, false, false)
	gpg, canary := canaryScript(t, "exit 1")
	c := filepath.Join(t.TempDir(), "c.txt")
	script := commitFile("internal/x/b.go", "ok") + "git cat-file commit HEAD > '" + c + "'\n" +
		`awk 'BEGIN{d=0} !d && /^$/ {print "gpgsig -----BEGIN PGP SIGNATURE-----"; print " "; print " AAAA"; print " -----END PGP SIGNATURE-----"; d=1} {print}' '` + c + "' > '" + c + ".y'\n" +
		"git update-ref refs/heads/t-023 $(git hash-object -t commit -w '" + c + ".y')\n" +
		"git config log.showSignature true\ngit config gpg.program '" + gpg + "'\n" + emit(assistant(100), okResult)
	res, err := f.run(script)
	if err != nil || res.ExitCode != 1 || !hasViolation(res.Violations, ruleHost, "config") {
		t.Fatalf("exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
	noCanary(t, canary, "gpg.program")
}

// Fix 7: a staged nested repo is not entered by the status calls, so its clean
// filter never fires, and the staged gitlink still fails the dirty rule.
func TestFixNestedRepoFilter(t *testing.T) {
	f := newFixture(t, false, false)
	_, canary := canaryScript(t, "")
	script := commitFile("internal/x/b.go", "ok") + "git init -q nested\ncd nested\necho '* filter=x' > .gitattributes\necho a > f\ngit add .\n" +
		"git -c user.name=a -c user.email=a@e.co commit -q -m n\ntouch -t 203501010000 f\n" +
		"git config filter.x.clean \"sh -c 'touch " + canary + "; cat'\"\ncd ..\ngit add nested 2>/dev/null\n" + emit(assistant(100), okResult)
	res, err := f.run(script)
	if err != nil || res.ExitCode != 1 || !hasViolation(res.Violations, ruleDirty, ".") {
		t.Fatalf("exit %d err %v violations %v", res.ExitCode, err, res.Violations)
	}
	noCanary(t, canary, "the nested repo's clean filter")
}

// Fix 8: a symlink planted at result.json or final.md is replaced, not followed.
func TestFixResultSymlink(t *testing.T) {
	f := newFixture(t, false, false)
	target := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(target, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(f.real, ".waggle/worktrees/runs/T-023-builder-1")
	res, err := f.run("ln -s '" + target + "' '" + runDir + "/result.json'\nln -s '" + target + "' '" + runDir + "/final.md'\n" + okScenario)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("exit %d err %v", res.ExitCode, err)
	}
	if b, _ := os.ReadFile(target); string(b) != "keep" {
		t.Fatalf("the planted symlink was followed: %q", b)
	}
	if fi, err := os.Lstat(filepath.Join(runDir, "result.json")); err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("result.json is not a regular file: %v", err)
	}
}

// Fix 9: a SIGTERM answered with a result event (Collect keeps its end) still reads as a timeout.
func TestFixTimeoutWithErrorResult(t *testing.T) {
	old := killGrace
	killGrace = 300 * time.Millisecond
	t.Cleanup(func() { killGrace = old })
	f := newFixture(t, false, false)
	script := `r() { printf '%s\n' "$1"; exit 1; }` + "\n" +
		`trap 'r "{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false}"' TERM` + "\nsleep 30 &\nwait\n"
	var n atomic.Int32
	res, err := f.run(script, func(_ *Request, d *Deps) { d.Timeout = 400 * time.Millisecond; n.Add(1) })
	if err != nil || res.ExitCode != 5 || res.End != "timeout" {
		t.Fatalf("exit %d end %s err %v", res.ExitCode, res.End, err)
	}
}
