// SPDX-License-Identifier: Apache-2.0

//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package dispatch

// openFlags is zero where O_NOFOLLOW is unavailable; readSmall still checks
// with Lstat and fstat.
const openFlags = 0
