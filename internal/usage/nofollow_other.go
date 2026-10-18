//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package usage

// openFlags is zero where O_NOFOLLOW is unavailable; openRegular still
// checks with Lstat and fstat.
const openFlags = 0
