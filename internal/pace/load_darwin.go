// SPDX-License-Identifier: Apache-2.0

package pace

import "syscall"

// readSystem reads vm.loadavg. Available memory is not measured on macOS.
func readSystem() (Reading, error) {
	r := Unknown()
	r.MemoryNote = "not measured on macOS"
	s, err := syscall.Sysctl("vm.loadavg")
	if err != nil {
		return r, nil
	}
	if v, ok := DecodeLoadavg([]byte(s)); ok {
		r.Load1 = v
	}
	return r, nil
}
