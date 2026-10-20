package dispatch

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"time"
)

// No subreaper here: a tracked process that outlives its parent is found by
// its own pid and start time, but its later children are not.
const subreaper = false

func setSubreaper() {}

func startedAfter(string, string) bool { return false }

// The table is read in-process (D-191), so a 25 ms poll is cheap.
func init() { pollEvery = 25 * time.Millisecond }

// kinfoSize is sizeof(struct kinfo_proc) on arm64 and amd64 macOS, little-endian.
const kinfoSize = 648

// procTable is sysctl kern.proc.all. Per entry: p_starttime (sec int64 at 0,
// usec int32 at 8, the start half of a process identity at microsecond
// resolution), p_stat at 36, p_pid at 40, p_comm at 243, e_ppid at 560 and
// e_pgid at 564. An empty name is a name like any other.
func procTable() (tab []proc, err error) {
	var s string
	for range 5 { // the table can grow between the size query and the read
		if s, err = syscall.Sysctl("kern.proc.all"); err == nil {
			break
		}
	}
	b := make([]byte, (len(s)+kinfoSize-1)/kinfoSize*kinfoSize) // Sysctl drops one trailing NUL byte
	copy(b, s)
	for ; len(b) >= kinfoSize; b = b[kinfoSize:] {
		le, comm := binary.LittleEndian, b[243:260]
		for i, c := range comm {
			if c == 0 {
				comm = comm[:i]
				break
			}
		}
		tab = append(tab, proc{pid: int(le.Uint32(b[40:])), ppid: int(int32(le.Uint32(b[560:]))), pgid: int(int32(le.Uint32(b[564:]))),
			start: fmt.Sprintf("%d.%06d", le.Uint64(b[0:]), le.Uint32(b[8:])), comm: string(comm), zombie: b[36] == 5})
	}
	return tab, err
}
