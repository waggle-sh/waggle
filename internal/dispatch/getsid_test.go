// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin || freebsd

package dispatch

import "syscall"

// getsid is the calling process's session id (fable D1 N1: no syscall.Getsid).
func getsid() int {
	r, _, _ := syscall.Syscall(syscall.SYS_GETSID, 0, 0, 0)
	return int(r)
}

// getpgrp is the calling process's group (solaris and illumos lack syscall.Getpgrp).
func getpgrp() int { return syscall.Getpgrp() }
