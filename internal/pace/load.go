// SPDX-License-Identifier: Apache-2.0

package pace

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// readCap bounds a read of a /proc file.
const readCap = 64 << 10

// Reading is one system reading. Load1 is negative and AvailMiB is -1 when
// unknown. MemoryNote says why memory is unknown, for a system that does not
// document it.
type Reading struct {
	Load1      float64
	CPUs       int
	AvailMiB   int64
	MemoryNote string
}

// Unknown is a reading with nothing measured.
func Unknown() Reading { return Reading{Load1: -1, CPUs: runtime.NumCPU(), AvailMiB: -1} }

// ReadLoad reads the 1-minute load average and, where documented, available memory.
func ReadLoad() (Reading, error) { return readSystem() }

// Percent is 100 * load1 / CPUs, or false when the load or CPU count is unknown.
func (r Reading) Percent() (float64, bool) {
	if r.Load1 < 0 || r.CPUs < 1 {
		return 0, false
	}
	return 100 * r.Load1 / float64(r.CPUs), true
}

// OverCPU reports whether load percent is at or over the target. Unknown is false.
func (r Reading) OverCPU(s Settings) bool {
	pct, ok := r.Percent()
	return ok && pct >= float64(s.CPUTargetPercent)
}

// UnderMemory reports whether available memory is under the floor. Unknown is false.
func (r Reading) UnderMemory(s Settings) bool {
	return r.AvailMiB >= 0 && r.AvailMiB < int64(s.MemoryFloorMiB)
}

// Finding is the doctor text for a resolved profile and a reading.
func Finding(s Settings, r Reading) string {
	var b strings.Builder
	fmt.Fprintf(&b, "pace: %s; ", s.Profile)
	if pct, ok := r.Percent(); ok {
		verdict := "under"
		if r.OverCPU(s) {
			verdict = "at or over"
		}
		fmt.Fprintf(&b, "load %.1f on %d CPUs (%.0f%%), %s the %d%% target; ", r.Load1, r.CPUs, pct, verdict, s.CPUTargetPercent)
	} else {
		b.WriteString("load unknown; ")
	}
	switch {
	case r.AvailMiB >= 0:
		verdict := "at or above"
		if r.UnderMemory(s) {
			verdict = "under"
		}
		fmt.Fprintf(&b, "available memory %.1f GiB, %s the %d MiB floor", float64(r.AvailMiB)/1024, verdict, s.MemoryFloorMiB)
	case r.MemoryNote != "":
		b.WriteString("available memory " + r.MemoryNote)
	default:
		b.WriteString("available memory unknown")
	}
	return b.String()
}

// ParseLoadavg reads the first field of /proc/loadavg.
func ParseLoadavg(b []byte) (float64, bool) {
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(f[0], 64)
	if err != nil || v < 0 || v != v || v > 1e9 {
		return 0, false
	}
	return v, true
}

// ParseMemAvailable reads the MemAvailable line of /proc/meminfo, in MiB.
func ParseMemAvailable(b []byte) (int64, bool) {
	for _, line := range strings.Split(string(b), "\n") {
		rest, ok := strings.CutPrefix(line, "MemAvailable:")
		if !ok {
			continue
		}
		f := strings.Fields(rest)
		if len(f) != 2 || f[1] != "kB" {
			return 0, false
		}
		kib, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil || kib < 0 {
			return 0, false
		}
		return kib / 1024, true
	}
	return 0, false
}

// DecodeLoadavg decodes the value of the macOS vm.loadavg sysctl, a struct
// loadavg: three uint32 values, 4 bytes of padding and an int64 fscale, 24
// bytes little-endian. syscall.Sysctl drops one trailing NUL, so 23 bytes are
// padded with a zero byte. Any other length and an fscale of 0 are unknown.
func DecodeLoadavg(b []byte) (float64, bool) {
	if len(b) != 23 && len(b) != 24 {
		return 0, false
	}
	buf := make([]byte, 24)
	copy(buf, b)
	ld := binary.LittleEndian.Uint32(buf[0:4])
	fscale := int64(binary.LittleEndian.Uint64(buf[16:24]))
	if fscale <= 0 {
		return 0, false
	}
	return float64(ld) / float64(fscale), true
}

// readProc reads a fixed /proc path read-only, capped at readCap bytes.
func readProc(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, readCap))
}

// linuxReading builds a Reading from the contents of the two /proc files;
// either may be nil.
func linuxReading(loadavg, meminfo []byte) Reading {
	r := Unknown()
	if v, ok := ParseLoadavg(bytes.TrimSpace(loadavg)); ok {
		r.Load1 = v
	}
	if v, ok := ParseMemAvailable(meminfo); ok {
		r.AvailMiB = v
	}
	return r
}
