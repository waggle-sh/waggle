// SPDX-License-Identifier: Apache-2.0

//go:build unix && !linux && !darwin && !freebsd

package dispatch

// getsid is -1 where the syscall number is not known; the session test then
// relies on setpgid failing with EPERM alone.
func getsid() int { return -1 }

func getpgrp() int { return 1 }
