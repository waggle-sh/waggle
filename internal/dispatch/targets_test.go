// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"slices"
	"testing"
)

// D2-T1: targets reads fabricated tables only. No table read, no signal, no sweep().

func pr(pid, ppid, pgid int, start string) proc {
	return proc{pid: pid, ppid: ppid, pgid: pgid, start: start, comm: "c" + start}
}

// targetTable: dispatch (4242, group 4000), the root 500, a tracked leader 501
// with member 502, a tracked leader 510 with child 511, and the Keeper's shell
// group 7000, which no tracked process ever led.
func targetTable() []proc {
	return []proc{pr(1, 0, 1, "1"), pr(4242, 1, 4000, "4242"), pr(500, 4242, 500, "500"), pr(501, 500, 501, "501"), pr(502, 501, 501, "502"),
		pr(510, 500, 510, "510"), pr(511, 510, 510, "511"), pr(7000, 1, 7000, "7000"), pr(7001, 7000, 7000, "7001")}
}

func without(tab []proc, pids ...int) (out []proc) {
	for _, p := range tab {
		if !slices.Contains(pids, p.pid) {
			out = append(out, p)
		}
	}
	return out
}

func TestTargetsOnFabricatedTables(t *testing.T) {
	zombie := func(p proc) proc { p.zombie = true; return p }
	cases := []struct {
		name                 string
		tab                  []proc
		group                int
		wantPids, wantGroups []int
	}{
		{"all tracked", targetTable(), 4000, []int{501, 502, 510, 511}, []int{501, 510}},
		{"leader gone, orphan stays in its group", append(without(targetTable(), 510, 511), pr(512, 1, 510, "512")), 4000, []int{501, 502}, []int{501, 510}},
		{"leader pid reused", append(without(targetTable(), 510), pr(510, 1, 510, "999")), 4000, []int{501, 502, 511}, []int{501}},
		{"leader group is dispatch's own", targetTable(), 501, []int{501, 502, 510, 511}, []int{510}},
		{"own group unknown", targetTable(), 0, []int{501, 502, 510, 511}, nil},
		{"own group 1", targetTable(), 1, []int{501, 502, 510, 511}, nil},
		{"group with no listed member", without(targetTable(), 501, 502), 4000, []int{510, 511}, []int{510}},
		{"only zombie members", append(without(targetTable(), 501, 502), zombie(pr(502, 1, 501, "502")), zombie(pr(501, 1, 501, "501"))), 4000, []int{510, 511}, []int{510}},
		{"nil table", nil, 4000, nil, nil},
		{"empty table", []proc{}, 4000, nil, nil},
		{"table without dispatch", without(targetTable(), 4242), 4000, nil, nil},
	}
	for _, c := range cases {
		tr := &tracker{root: 500, self: 4242, group: 4000, seen: map[pkey]string{}}
		tr.observe(targetTable(), nil) // learn the tree from the full table
		tr.observe(c.tab, nil)
		tr.group = c.group // after observe, which reads it from the table
		pids, groups := tr.targets(c.tab)
		slices.Sort(pids)
		if !slices.Equal(pids, c.wantPids) || !slices.Equal(groups, c.wantGroups) {
			t.Errorf("%s: pids %v groups %v, want %v %v", c.name, pids, groups, c.wantPids, c.wantGroups)
		}
	}
}

func TestTargetsRefuseInitSelfAndKernel(t *testing.T) {
	tab := append(targetTable(), pr(0, 0, 0, "0"))
	tr := &tracker{root: 500, self: 4242, group: 4000, seen: map[pkey]string{}}
	tr.observe(targetTable(), nil)
	tr.live = append(tr.live, pr(0, 0, 0, "0"), pr(1, 0, 1, "1"), pr(4242, 1, 4000, "4242"))
	tr.leaders[pkey{0, "0"}], tr.leaders[pkey{1, "1"}] = true, true
	pids, groups := tr.targets(tab)
	slices.Sort(pids)
	if !slices.Equal(pids, []int{501, 502, 510, 511}) || !slices.Equal(groups, []int{501, 510}) {
		t.Fatalf("pids %v groups %v", pids, groups)
	}
}

// leaders holds a process only when it was tracked and leads its own group.
func TestObserveRecordsOnlyTrackedLeaders(t *testing.T) {
	tr := &tracker{root: 500, self: 4242, group: 4000, seen: map[pkey]string{}}
	tr.observe(targetTable(), nil)
	if len(tr.leaders) != 2 || !tr.leaders[pkey{501, "501"}] || !tr.leaders[pkey{510, "510"}] {
		t.Fatalf("leaders %v", tr.leaders)
	}
}

// D2-T2: a first table without the root is blind, and a later table listing the
// root pid with another start (a reused pid) does not move rootStart.
func TestObserveRootStartSetOnce(t *testing.T) {
	tr := &tracker{root: 500, self: 4242, seen: map[pkey]string{}}
	tr.observe(without(targetTable(), 500), nil)
	if !tr.blind || tr.rootStart != "" {
		t.Fatalf("blind %v rootStart %q", tr.blind, tr.rootStart)
	}
	tr = &tracker{root: 500, self: 4242, seen: map[pkey]string{}}
	tr.observe(targetTable(), nil)
	if tr.blind || tr.rootStart != "500" {
		t.Fatalf("blind %v rootStart %q", tr.blind, tr.rootStart)
	}
	tr.observe(append(without(targetTable(), 500), pr(500, 1, 500, "9999")), nil)
	if tr.rootStart != "500" {
		t.Fatalf("a reused root pid moved rootStart to %q", tr.rootStart)
	}
}

// A group a read listed empty is gone for good: its id returning as a foreign
// group (a new leader that exited, a member left) must not be signalled.
func TestTargetsDropGroupSeenEmpty(t *testing.T) {
	tr := &tracker{root: 500, self: 4242, group: 4000, seen: map[pkey]string{}}
	tr.observe(targetTable(), nil)
	gone := without(targetTable(), 510, 511)
	tr.observe(gone, nil)
	foreign := append(gone, pr(9000, 1, 510, "f9000"))
	tr.observe(foreign, nil)
	if _, groups := tr.targets(foreign); !slices.Equal(groups, []int{501}) {
		t.Errorf("groups %v, want [501]", groups)
	}
}
