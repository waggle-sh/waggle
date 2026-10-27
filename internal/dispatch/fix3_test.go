// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// D-192: a branch section that git push -u writes does not trip the record;
// pushRemote, a path remote and a global branch setting do.
func TestHostRecordBranchKeysStrict(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  [][]string
		trip bool
	}{
		{"push -u section", [][]string{{"branch.t-023.remote", "origin"}, {"branch.t-023.merge", "refs/heads/t-023"}}, false},
		{"pushRemote", [][]string{{"branch.t-023.pushRemote", "origin"}}, true},
		{"path remote", [][]string{{"branch.t-023.remote", "/tmp/planted.git"}}, true},
		{"unconfigured remote", [][]string{{"branch.t-023.remote", "nowhere"}}, true},
		{"global setting", [][]string{{"branch.autoSetupMerge", "always"}}, true},
		{"per-branch rebase", [][]string{{"branch.t-023.rebase", "true"}}, true},
	} {
		f := newFixture(t, false, false)
		f.git("remote", "add", "origin", "https://example.invalid/o.git")
		if res, err := f.run(okScenario); err != nil || res.ExitCode != 0 {
			t.Fatalf("%s: run 1: exit %d err %v", tc.name, res.ExitCode, err)
		}
		for _, kv := range tc.cfg {
			f.git("config", "--local", kv[0], kv[1])
		}
		res, err := f.run(commitFile("internal/x/c.go", "ok") + emit(assistant(100), okResult))
		if tripped := code(err) == ExitEnv && strings.Contains(err.Error(), "config.keys"); tripped != tc.trip || (!tc.trip && (err != nil || res.ExitCode != 0)) {
			t.Fatalf("%s: tripped %v want %v: exit %d %v", tc.name, tripped, tc.trip, res.ExitCode, err)
		}
	}
}

// D-190 CB1: a setsid child whose command name is blank is tracked and killed.
func TestBlankNameChildKilled(t *testing.T) {
	fastPoll(t, 30*time.Millisecond)
	f := newFixture(t, false, false)
	dir := t.TempDir()
	pidFile, marker, release := filepath.Join(dir, "pid"), filepath.Join(dir, "marker"), filepath.Join(dir, "release")
	script := `use POSIX; $0 = " "; POSIX::setsid(); open F, ">", $ARGV[0]; print F $$; close F; for (1..600) { if (-e $ARGV[2]) { open F, ">", $ARGV[1]; close F; last } select(undef, undef, undef, 0.1) }`
	res, err := f.run("perl -e '" + script + "' " + pidFile + " " + marker + " " + release + " &\nsleep 0.7\n" + okScenario)
	pid := pidFrom(t, pidFile)
	if err != nil || res.ExitCode != 0 || len(res.Survivors) != 1 || res.Survivors[0].PID != pid || !gone(pid) {
		t.Fatalf("exit %d err %v survivors %v pid %d", res.ExitCode, err, res.Survivors, pid)
	}
	_ = os.WriteFile(release, nil, 0o644)
	time.Sleep(500 * time.Millisecond)
	if _, e := os.Stat(marker); e == nil {
		t.Fatal("the blank-named survivor wrote its marker")
	}
}

// stallBackend stalls every renewal until its context ends and records whether
// the context had a deadline.
type stallBackend struct {
	ClaimBackend
	calls    atomic.Int32
	deadline atomic.Bool
}

func (s *stallBackend) AcquireLease(ctx context.Context, l comb.Lease) error {
	if s.calls.Add(1) == 1 {
		return s.ClaimBackend.AcquireLease(ctx, l)
	}
	_, ok := ctx.Deadline()
	s.deadline.Store(ok)
	<-ctx.Done()
	return ctx.Err()
}

// D-190 CB3: a renewal that outlives the session is bounded, so a stalled
// remote cannot hold Run.
func TestRenewalBounded(t *testing.T) {
	old := renewBound
	renewBound = 300 * time.Millisecond
	t.Cleanup(func() { renewBound = old })
	f := newFixture(t, false, false)
	b, _ := claimEnv(t, f.real)
	sb := &stallBackend{ClaimBackend: b}
	done := make(chan error, 1)
	go func() {
		_, err := f.run("sleep 1\n"+okScenario, func(_ *Request, d *Deps) { d.Backend, d.KeepaliveEvery = sb, 50*time.Millisecond })
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil || !sb.deadline.Load() {
			t.Fatalf("err %v, renewal context had a deadline: %v", err, sb.deadline.Load())
		}
	case <-time.After(20 * time.Second):
		t.Fatal("a stalled renewal held Run")
	}
}

// ownChildBackend runs a child of dispatch as the renewal's work.
type ownChildBackend struct {
	ClaimBackend
	calls atomic.Int32
	err   atomic.Value
}

func (o *ownChildBackend) AcquireLease(ctx context.Context, l comb.Lease) error {
	if o.calls.Add(1) == 1 {
		return o.ClaimBackend.AcquireLease(ctx, l)
	}
	err := exec.Command("sleep", "0.6").Run()
	o.err.Store(map[bool]string{true: "ok", false: "disturbed"}[err == nil])
	return nil
}

// D-190 N11: the sweep waits for a renewal in flight; it never kills the
// renewal's own child (on linux it adopts children of dispatch younger than the worker).
func TestSweepWaitsForRenewal(t *testing.T) {
	f := newFixture(t, false, false)
	b, _ := claimEnv(t, f.real)
	ob := &ownChildBackend{ClaimBackend: b}
	if _, err := f.run("sleep 0.5\n"+okScenario, func(_ *Request, d *Deps) { d.Backend, d.KeepaliveEvery = ob, 200*time.Millisecond }); err != nil {
		t.Fatal(err)
	}
	if got, _ := ob.err.Load().(string); got != "ok" {
		t.Fatalf("the renewal's child was %q", got)
	}
}
