//go:build linux

package dispatch

import (
	"os/exec"
	"slices"
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
	tr.sweep()
	if !gone(orphan.pid) {
		t.Fatalf("the unseen group member %d survived the sweep", orphan.pid)
	}
	_ = cmd.Wait()
}
