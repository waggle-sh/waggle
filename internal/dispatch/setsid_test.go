//go:build unix

package dispatch

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// D2-T5: the worker leads its own session: a process it starts reports the
// leader's pid as its session id, and cannot join a group of the test process's
// session. No signal is sent.
func TestWorkerLeadsOwnSession(t *testing.T) {
	f := newFixture(t, false, false)
	out := filepath.Join(t.TempDir(), "sid")
	res, err := f.run(helper(t, "sid", out, strconv.Itoa(getpgrp())) + "\n" + okScenario)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("exit %d err %v", res.ExitCode, err)
	}
	b, _ := os.ReadFile(out)
	got := strings.Fields(string(b)) // sid, leader pid, setpgid-EPERM
	if len(got) != 3 || got[2] != "true" || (got[0] != "-1" && got[0] != got[1]) || got[0] == strconv.Itoa(getsid()) {
		t.Fatalf("helper reported %q (test session %d)", b, getsid())
	}
}

// D2-T6: a worker that exits at once is never blind: the tracker's first read
// came before Wait could reap the leader.
func TestFastExitNeverBlind(t *testing.T) {
	for range 20 {
		f := newFixture(t, false, false)
		res, err := f.run(okScenario)
		if err != nil || strings.Contains(strings.Join(res.Warnings, "|"), "survivors_unchecked") {
			t.Fatalf("exit %d warnings %v err %v", res.ExitCode, res.Warnings, err)
		}
	}
}
