// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package dispatch

import "syscall"

// openFlags make open refuse a symlink and never block on a FIFO.
const openFlags = syscall.O_NOFOLLOW | syscall.O_NONBLOCK
