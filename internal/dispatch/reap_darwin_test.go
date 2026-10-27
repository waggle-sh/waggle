// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
)

// The kinfo_proc offsets are checked against the process's own identity and a
// child that has exited but is not waited for.
func TestProcTableLayout(t *testing.T) {
	tab, err := procTable()
	if err != nil {
		t.Fatal(err)
	}
	var me proc
	for _, p := range tab {
		if p.pid == os.Getpid() {
			me = p
		}
	}
	if me.ppid != os.Getppid() || me.pgid != syscall.Getpgrp() || me.zombie || me.start == "" || me.comm == "" {
		t.Fatalf("own entry %+v, want ppid %d pgid %d", me, os.Getppid(), syscall.Getpgrp())
	}
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Wait() })
	if !wait(func() bool {
		tab, _ := procTable()
		for _, p := range tab {
			if p.pid == cmd.Process.Pid && p.zombie && p.ppid == os.Getpid() {
				return true
			}
		}
		return false
	}) {
		t.Fatalf("child %d not listed as a zombie", cmd.Process.Pid)
	}
}

// One read of the process table, for the report.
func BenchmarkProcTable(b *testing.B) {
	for range b.N {
		if _, err := procTable(); err != nil {
			b.Fatal(err)
		}
	}
}
