//go:build unix

package dispatch

import (
	"errors"
	"slices"
	"testing"
)

// These tests feed observe a fabricated table and assert on data only. Nothing
// here signals a process or reads the real table (the incident behind them was
// a tracker rooted at 0 that tracked init and killed everything the user owns).
func fabricated() []proc {
	return []proc{
		{pid: 1, ppid: 0, pgid: 1, start: "1", comm: "launchd"},
		{pid: 2, ppid: 1, pgid: 2, start: "2", comm: "login"},
		{pid: 500, ppid: 1, pgid: 500, start: "5", comm: "root"},
		{pid: 501, ppid: 500, pgid: 500, start: "6", comm: "child"},
		{pid: 502, ppid: 501, pgid: 500, start: "7", comm: "grandchild"},
		{pid: 900, ppid: 1, pgid: 900, start: "9", comm: "stray"},
		{pid: 901, ppid: 900, pgid: 900, start: "10", comm: "stray child"},
	}
}

func pids(ps []proc) (out []int) {
	for _, p := range ps {
		out = append(out, p.pid)
	}
	slices.Sort(out)
	return out
}

func TestObserveTracksOnlyTheRootSubtree(t *testing.T) {
	tr := &tracker{root: 500, self: 4242, seen: map[pkey]string{}}
	tr.observe(fabricated(), nil)
	if got := pids(tr.live); !slices.Equal(got, []int{501, 502}) {
		t.Fatalf("tracked %v, want the root's descendants only", got)
	}
	if len(tr.seen) != 2 || tr.blind {
		t.Fatalf("seen %v blind %v", tr.seen, tr.blind)
	}
}

func TestObserveRefusesInitAndKernelRoots(t *testing.T) {
	for _, root := range []int{-1, 0, 1} {
		tr := &tracker{root: root, self: 4242, seen: map[pkey]string{}}
		tr.observe(fabricated(), nil)
		if len(tr.live) != 0 || len(tr.seen) != 0 || !tr.blind {
			t.Fatalf("root %d tracked %v seen %v blind %v", root, tr.live, tr.seen, tr.blind)
		}
		if tr = newTracker(root); tr.root != 0 || !tr.blind || tr.seen != nil {
			t.Fatalf("newTracker(%d) is not inert: %+v", root, tr)
		}
		tr.observe(fabricated(), nil)
		if len(tr.live) != 0 || len(tr.seen) != 0 {
			t.Fatalf("an inert tracker tracked %v", tr.live)
		}
	}
}

// Entries for init or the kernel in seen, and a table that lists pid 0, never
// pull init or its other children in as tracked parents.
func TestObserveNeverTreatsInitAsTrackedParent(t *testing.T) {
	tr := &tracker{root: 500, self: 4242, seen: map[pkey]string{{1, "1"}: "launchd", {0, "0"}: "kernel"}}
	tab := append(fabricated(), proc{pid: 0, ppid: 0, pgid: 0, start: "0", comm: "kernel_task"})
	tr.observe(tab, nil)
	for _, p := range tr.live {
		if p.pid <= 1 || p.ppid <= 1 || p.pid == 2 || p.pid >= 900 {
			t.Fatalf("tracked %+v", p)
		}
	}
}

// A failed table read must not leave the previous sample for sweep to signal.
func TestObserveErrorClearsLive(t *testing.T) {
	tr := &tracker{root: 500, self: 4242, seen: map[pkey]string{}}
	tr.observe(fabricated(), nil)
	if len(tr.live) == 0 {
		t.Fatal("setup tracked nothing")
	}
	tr.observe(nil, errors.New("procTable failed"))
	if len(tr.live) != 0 || !tr.blind {
		t.Fatalf("live %v blind %v after a failed read", tr.live, tr.blind)
	}
	tr.observe(fabricated(), errors.New("partial table with an error"))
	if len(tr.live) != 0 {
		t.Fatalf("a table returned with an error was used: %v", tr.live)
	}
}
