//go:build linux

package dispatch

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// D-193, D-195: tests that call sweep() directly run on linux only (the Colima
// container), never on the Keeper's Mac.

// Pid reuse guard: a tracked pid whose start time differs from the live
// process is another process and is never killed.
func TestSweepSkipsReusedPid(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	var real proc
	if !wait(func() bool {
		tab, _ := procTable()
		i := slices.IndexFunc(tab, func(p proc) bool { return p.pid == cmd.Process.Pid })
		if i >= 0 {
			real = tab[i]
		}
		return i >= 0
	}) {
		t.Fatal("the process table does not list the helper")
	}
	tr := newTracker(cmd.Process.Pid)
	tr.seen[pkey{real.pid, real.start + "x"}] = "sleep"
	if k := tr.sweep(); len(k) != 0 || syscall.Kill(real.pid, 0) != nil {
		t.Fatalf("a start time mismatch was killed: %v", k)
	}
	tr.seen[pkey{real.pid, real.start}] = "sleep"
	k := tr.sweep()
	dead := make(chan error, 1)
	go func() { dead <- cmd.Wait() }()
	select {
	case <-dead:
	case <-time.After(5 * time.Second):
		t.Fatal("the matching process was not killed")
	}
	if len(k) != 1 || k[0].PID != real.pid {
		t.Fatalf("killed %v", k)
	}
}

// The sweep also signals each tracked process group: a child that was
// orphaned (never seen) but stayed in a tracked process's group dies too.
func TestSweepKillsTrackedGroup(t *testing.T) {
	cmd := exec.Command("sh", "-c", "(sleep 61 &); sleep 62")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); _ = cmd.Wait() })
	var tab []proc
	var orphan, leader proc
	if !wait(func() bool {
		tab, _ = procTable()
		orphan, leader = proc{}, proc{}
		for _, p := range tab {
			if p.pid == cmd.Process.Pid {
				leader = p
			} else if p.pgid == cmd.Process.Pid && p.ppid != cmd.Process.Pid && !p.zombie {
				orphan = p
			}
		}
		return leader.pid != 0 && orphan.pid != 0
	}) {
		t.Fatalf("no orphan in the group: %v", tab)
	}
	tr := newTracker(cmd.Process.Pid)
	// rootStart lets the sweep reap the killed orphan once the earlier tests in
	// the package have made this process a subreaper (a zombie still answers kill 0).
	tr.rootStart = leader.start
	tr.seen[pkey{leader.pid, leader.start}] = "sh"
	tr.leaders = map[pkey]bool{{leader.pid, leader.start}: true}
	tr.sweep()
	if !gone(orphan.pid) {
		t.Fatalf("the unseen group member %d survived the sweep", orphan.pid)
	}
	_ = cmd.Wait()
}

type step struct {
	tab []proc
	err error
}

type sent struct {
	pid int
	sig syscall.Signal
}

// fakeSweep returns a tracker that has learned targetTable and whose table read
// and kill are a fabricated sequence (the last step repeats) and a recorder.
// Nothing real is read or signalled (D-193).
func fakeSweep(t *testing.T, steps ...step) (tr *tracker, kills *[]sent, reads *int) {
	t.Helper()
	tr, kills, reads = &tracker{root: 500, self: 4242, group: 4000, seen: map[pkey]string{}}, new([]sent), new(int)
	tr.observe(targetTable(), nil)
	tr.read = func() ([]proc, error) {
		s := steps[min(*reads, len(steps)-1)]
		*reads++
		return s.tab, s.err
	}
	tr.kill = func(pid int, sig syscall.Signal) error { *kills = append(*kills, sent{pid, sig}); return nil }
	if tr.read == nil || tr.kill == nil {
		t.Fatal("a sweep seam is nil")
	}
	return tr, kills, reads
}

// D2-T3: the signals equal targets round by round (groups, then pids), the
// untracked member of a killed group is recorded once, and the loop ends when
// targets is empty.
func TestSweepSignalsExactlyTargets(t *testing.T) {
	round := append(targetTable(), pr(503, 1, 501, "503"))
	tr, kills, reads := fakeSweep(t, step{tab: round}, step{tab: round}, step{tab: without(round, 501, 502, 503, 510, 511)})
	got := tr.sweep()
	want := []sent{{-501, 9}, {-510, 9}, {501, 9}, {502, 9}, {510, 9}, {511, 9}}
	want = append(want, want...)
	if !slices.Equal(*kills, want) || *reads != 3 {
		t.Fatalf("kills %v reads %d", *kills, *reads)
	}
	pids := []int{}
	for _, s := range got {
		pids = append(pids, s.PID)
	}
	slices.Sort(pids)
	if !slices.Equal(pids, []int{501, 502, 503, 510, 511}) {
		t.Fatalf("survivors %v, each pid once", got)
	}
}

func TestSweepStopsAfter40Rounds(t *testing.T) {
	tr, kills, reads := fakeSweep(t, step{tab: targetTable()})
	tr.sweep()
	if *reads != 40 || len(*kills) != 40*6 {
		t.Fatalf("reads %d kills %d", *reads, len(*kills))
	}
}

// A failed read signals nothing, is retried up to 3 times and leaves blind set.
func TestSweepReadRetry(t *testing.T) {
	bad := step{err: errors.New("no table")}
	tr, kills, reads := fakeSweep(t, bad, bad, bad, step{tab: without(targetTable(), 501, 502, 510, 511)})
	tr.sweep()
	if len(*kills) != 0 || *reads != 4 || !tr.blind {
		t.Fatalf("kills %v reads %d blind %v", *kills, *reads, tr.blind)
	}
	tr, kills, reads = fakeSweep(t, bad)
	tr.sweep()
	if len(*kills) != 0 || *reads != 4 {
		t.Fatalf("a persistent failure: kills %v reads %d, want 4 reads", *kills, *reads)
	}
	tr, kills, reads = fakeSweep(t, bad, step{tab: targetTable()}, step{tab: without(targetTable(), 501, 502, 510, 511)})
	tr.sweep()
	if len(*kills) != 6 || *reads != 3 {
		t.Fatalf("a read that recovers: kills %v reads %d", *kills, *reads)
	}
}

// D2-T4: a tool-shell stand-in that starts its own session, backgrounds
// sleep 30 and exits leaves nothing alive after Run, and the result names it.
func TestToolShellChildKilled(t *testing.T) {
	fastPoll(t, 30*time.Millisecond)
	f := newFixture(t, false, false)
	res, err := f.run(helper(t, "toolshell") + " &\nsleep 0.7\n" + okScenario)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("exit %d err %v", res.ExitCode, err)
	}
	i := slices.IndexFunc(res.Survivors, func(s Survivor) bool { return strings.Contains(s.Command, "sleep") })
	if i < 0 || !gone(res.Survivors[i].PID) {
		t.Fatalf("survivors %v: want the sleep child, gone after Run", res.Survivors)
	}
}
