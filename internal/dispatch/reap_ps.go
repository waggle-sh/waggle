//go:build unix && !linux

package dispatch

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// No subreaper here: a tracked process that outlives its parent is found by
// its own pid and start time, but its later children are not.
const subreaper = false

func setSubreaper() {}

func startedAfter(string, string) bool { return false }

// procTable is `ps` pinned to the C locale: pid, ppid, the fixed five-field
// lstart (one-second resolution, the start-time half of a process identity),
// state and command name.
func procTable() (tab []proc, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/ps", "-A", "-o", "pid=,ppid=,lstart=,stat=,comm=")
	cmd.Env = []string{"LC_ALL=C", "TZ=UTC"}
	out, err := cmd.Output()
	for _, l := range strings.Split(string(out), "\n") {
		f := strings.Fields(l) // pid ppid 5 lstart fields state comm...
		if len(f) < 9 {
			continue
		}
		pid, e1 := strconv.Atoi(f[0])
		ppid, e2 := strconv.Atoi(f[1])
		if e1 == nil && e2 == nil {
			tab = append(tab, proc{pid: pid, ppid: ppid, start: strings.Join(f[2:7], " "), comm: filepath.Base(strings.Join(f[8:], " ")), zombie: strings.HasPrefix(f[7], "Z")})
		}
	}
	return tab, err
}
