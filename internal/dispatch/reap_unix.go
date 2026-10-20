//go:build unix

package dispatch

import (
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
// exits; one that daemonizes between two samples is never seen (not_enforced).
type tracker struct {
	root, self int
	rootStart  string
	reaped     bool            // the leader is gone: on linux its orphans reparent to dispatch
	seen       map[pkey]string // tracked processes and their command names
	live       []proc          // tracked and alive at the last poll
	blind      bool            // a poll failed or the root was unusable: survivors may have been missed
}

func newTracker(root int) *tracker {
	if root <= 1 { // inert: pid 0 and init would make every process of the user a descendant
		return &tracker{blind: true}
	}
	setSubreaper()
	return &tracker{root: root, self: syscall.Getpid(), seen: map[pkey]string{}}
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
		if _, ok := t.seen[pkey{p.pid, p.start}]; ok && !p.zombie {
			kids[p.pid] = true
		} else if p.pid == t.root {
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
		}
	}
	t.live = t.live[:0]
	for _, p := range tab {
		if _, ok := t.seen[pkey{p.pid, p.start}]; ok && !p.zombie && p.pid > 1 {
			t.live = append(t.live, p)
		}
	}
}

// sweep runs after the leader is reaped. It kills every tracked process still
// alive, polls again for the children of what it killed and repeats until none
// remain or the bound is hit. It returns each process it had to kill.
func (t *tracker) sweep() (killed []Survivor) {
	t.reaped = true
	done := map[pkey]bool{}
	for range 40 {
		if t.poll(); len(t.live) == 0 {
			break
		}
		for _, p := range t.live {
			if p.pgid > 1 && p.pgid != syscall.Getpgrp() {
				_ = syscall.Kill(-p.pgid, syscall.SIGKILL) // an unseen child left in a tracked process's group
			}
			if k := (pkey{p.pid, p.start}); p.pid > 1 && p.pid != t.self && syscall.Kill(p.pid, syscall.SIGKILL) == nil && !done[k] {
				done[k] = true
				killed = append(killed, Survivor{p.pid, p.comm})
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	return killed
}
