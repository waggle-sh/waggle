// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

// subreaper: dispatch adopts the orphans of the worker's tree (prctl
// PR_SET_CHILD_SUBREAPER) so they stay visible after their parent exits.
const subreaper = true

func setSubreaper() { _, _, _ = syscall.RawSyscall(syscall.SYS_PRCTL, 36, 1, 0) }

// startedAfter compares /proc start times (clock ticks since boot).
func startedAfter(start, root string) bool {
	a, e1 := strconv.ParseUint(start, 10, 64)
	b, e2 := strconv.ParseUint(root, 10, 64)
	return e1 == nil && e2 == nil && a >= b
}

// procTable reads /proc/<pid>/stat: pid (comm) state ppid ... starttime (field 22).
func procTable() (tab []proc, err error) {
	ents, err := os.ReadDir("/proc")
	for _, e := range ents {
		pid, perr := strconv.Atoi(e.Name())
		b, rerr := os.ReadFile("/proc/" + e.Name() + "/stat")
		s := string(b)
		open, end := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
		if perr != nil || rerr != nil || open < 0 || end < open {
			continue
		}
		if f := strings.Fields(s[end+1:]); len(f) > 19 { // state ppid pgrp ..., starttime is f[19]
			ppid, _ := strconv.Atoi(f[1])
			pgid, _ := strconv.Atoi(f[2])
			tab = append(tab, proc{pid: pid, ppid: ppid, pgid: pgid, start: f[19], comm: s[open+1 : end], zombie: f[0] == "Z"})
		}
	}
	return tab, err
}
