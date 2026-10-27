// SPDX-License-Identifier: Apache-2.0

//go:build unix && !darwin && !linux

package dispatch

import "errors"

// No process table reader exists for this platform, so the descendant tracker
// is unavailable: poll finds nothing and the sweep kills nothing. Only the
// worker's own process group is signalled at stop; a process that left the
// group (setsid) is not found. This gap is named in NotEnforced.
const subreaper = false

func setSubreaper() {}

func startedAfter(string, string) bool { return false }

func procTable() ([]proc, error) { return nil, errors.New("no process table reader on this platform") }
