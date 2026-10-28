// SPDX-License-Identifier: Apache-2.0

package pace

// readSystem reads the two fixed /proc files. A file that cannot be read
// leaves its part of the reading unknown.
func readSystem() (Reading, error) {
	loadavg, _ := readProc("/proc/loadavg")
	meminfo, _ := readProc("/proc/meminfo")
	return linuxReading(loadavg, meminfo), nil
}
