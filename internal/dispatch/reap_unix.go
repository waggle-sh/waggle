// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dispatch

import (
	"slices"
	"syscall"
	"time"
)

type proc struct {
	pid, ppid   int
	pgid        int
	start, comm string
	zombie      bool
}

type pkey struct {
	pid   int
	start string // with the pid, the identity: a reused pid has another start time
}

// tracker follows the worker's descendants by sampling the process table (D-187).
// A process that left the group is still a descendant by ppid until its parent
// exits; one whose parent exits before the next sample is never seen (not_enforced).
type tracker struct {
	root, self int
	group      int                    // dispatch's own process group from the table, 0 when unknown
	rootStart  string                 // set once, from the first table that lists the leader
	reaped     bool                   // the leader is gone: on linux its orphans reparent to dispatch
	seen       map[pkey]string        // tracked processes and their command names
	leaders    map[pkey]bool          // tracked processes that lead their own group
	live       []proc                 // tracked and alive at the last poll
	blind      bool                   // a poll failed or the root was unusable: survivors may have been missed
	read       func() ([]proc, error) // the sweep's table read and kill: seams that only tests replace
	kill       func(int, syscall.Signal) error
}

func sysKill(p int, s syscall.Signal) error { return syscall.Kill(p, s) }

// newTracker takes the first table read itself. Callers start it right after
// cmd.Start and before cmd.Wait can run, so that read lists the leader (alive,
// or a zombie with its start time) and the sweep knows the reaping threshold.
func newTracker(root int) *tracker {
	if root <= 1 { // inert: pid 0 and init would make every process of the user a descendant
		return &tracker{blind: true, read: procTable, kill: sysKill} // seams set: its sweep reads nothing it may signal
	}
	setSubreaper()
	t := &tracker{root: root, self: syscall.Getpid(), seen: map[pkey]string{}, read: procTable,
		kill: sysKill}
	t.poll()
	return t
}

func (t *tracker) poll() { t.observe(procTable()) }

// observe selects the tracked descendants from one table. On a failed read the
// table is dropped, so live empties (its pids may be reused) and blind says so.
func (t *tracker) observe(tab []proc, err error) {
	if err != nil || t.root <= 1 {
		tab, t.blind = nil, true
	}
	kids := map[int]bool{t.root: !t.reaped, t.self: t.reaped && subreaper}
	for _, p := range tab {
		if p.pid == t.self { // from the table, because solaris and illumos lack syscall.Getpgid and Getpgrp
			t.group = p.pgid
		}
		if _, ok := t.seen[pkey{p.pid, p.start}]; ok && !p.zombie {
			kids[p.pid] = true
		} else if p.pid == t.root && t.rootStart == "" { // a reused root pid after the reap must not move the threshold
			t.rootStart = p.start
		}
	}
	for _, p := range tab { // reparented to dispatch and dead: reap it, or it stays a zombie of ours
		if p.zombie && p.ppid == t.self && t.reaped && subreaper && startedAfter(p.start, t.rootStart) {
			_, _ = syscall.Wait4(p.pid, nil, syscall.WNOHANG, nil)
		}
	}
	for grew := true; grew; {
		grew = false
		for _, p := range tab {
			k := pkey{p.pid, p.start}
			if _, known := t.seen[k]; known || p.zombie || p.pid <= 1 || p.ppid <= 1 || p.pid == t.root || p.pid == t.self || !kids[p.ppid] ||
				(p.ppid == t.self && !startedAfter(p.start, t.rootStart)) { // an adopted orphan must be younger than the leader
				continue
			}
			t.seen[k], kids[p.pid], grew = p.comm, true, true
			if p.pgid == p.pid {
				if t.leaders == nil {
					t.leaders = map[pkey]bool{}
				}
				t.leaders[k] = true
			}
		}
	}
	if slices.ContainsFunc(tab, func(p proc) bool { return p.pid == t.self }) {
		for k := range t.leaders { // a group a read lists empty is gone for good: its id may return as a foreign group
			if !slices.ContainsFunc(tab, func(p proc) bool { return p.pgid == k.pid }) {
				delete(t.leaders, k)
			}
		}
	}
	if t.rootStart == "" { // the leader was never listed: its start time is unknown
		t.blind = true
	}
	t.live = t.live[:0]
	for _, p := range tab {
		if _, ok := t.seen[pkey{p.pid, p.start}]; ok && !p.zombie && p.pid > 1 {
			t.live = append(t.live, p)
		}
	}
}

// targets is what the sweep signals, from one table and the tracker's own
// state only. Pids: tracked live processes above 1, never dispatch. Groups
// (D-194): the id of a group the tracker saw a tracked process lead, when it is
// above 1, not dispatch's own, has a live member and its leader pid is absent
// or has the recorded start time. A table that does not list dispatch (empty,
// nil) yields nothing: it would make every leader look absent.
func (t *tracker) targets(tab []proc) (pids, groups []int) {
	if !slices.ContainsFunc(tab, func(p proc) bool { return p.pid == t.self }) {
		return nil, nil
	}
	for _, p := range t.live {
		if p.pid > 1 && p.pid != t.self {
			pids = append(pids, p.pid)
		}
	}
	for k := range t.leaders {
		member, same := false, true
		for _, p := range tab {
			member = member || p.pgid == k.pid && !p.zombie
			same = same && (p.pid != k.pid || p.start == k.start)
		}
		if k.pid > 1 && t.group > 1 && k.pid != t.group && member && same {
			groups = append(groups, k.pid)
		}
	}
	slices.Sort(groups)
	return pids, groups
}

// sweep runs after the leader is reaped. Each round reads the table (a failed
// read signals nothing and is retried up to 3 times), kills the groups and then
// the pids that targets names, and repeats until none remain or 40 rounds pass.
// It returns each process it had to kill, once.
func (t *tracker) sweep() (killed []Survivor) {
	t.reaped = true
	done, fails := map[pkey]bool{}, 0
	note := func(p proc) {
		if k := (pkey{p.pid, p.start}); !done[k] && p.pid > 1 && p.pid != t.self && !p.zombie {
			done[k] = true
			killed = append(killed, Survivor{p.pid, p.comm})
		}
	}
	for range 40 {
		tab, err := t.read()
		if t.observe(tab, err); err != nil {
			if fails++; fails > 3 {
				break
			}
			time.Sleep(25 * time.Millisecond)
			continue
		}
		pids, groups := t.targets(tab)
		if len(pids)+len(groups) == 0 {
			break
		}
		for _, g := range groups {
			if t.kill(-g, syscall.SIGKILL) == nil {
				for _, p := range tab {
					if p.pgid == g {
						note(p)
					}
				}
			}
		}
		for _, p := range t.live {
			if slices.Contains(pids, p.pid) && t.kill(p.pid, syscall.SIGKILL) == nil {
				note(p)
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	return killed
}
