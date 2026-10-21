<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: T-023 phase D2 (PR #59), fable

Verdict: must-fix
Risk-Tier: 3

PR #59, branch `t-023d2`, head `2f1352d`, base `c98d130` (three commits). Second independent Tier 3 guard (fable, D-184); no coordination with the opus guard. Every probe that can signal ran in the Colima linux container under the shared lock (D-193, D-195); on this Mac I ran only the committed suite and one data-only measurement that calls no `sweep()` and sends no signal.

## Intent Summary

D2 changes which processes dispatch signals and adds host git writes. The sweep no longer sends `kill(-pgid)` for every tracked process's group; `observe` records the identity of each tracked process it first sees leading its own group, and a pure `targets(tab)` names the pids to kill (tracked, live, above 1, not dispatch) and the groups (a recorded leader's pid when it is above 1, dispatch's own group is known and differs, the table lists a live member, and the leader pid is absent or still carries the recorded start time; nothing from a table that does not list dispatch). `sweep` is the loop only (read through a seam, retry a failed read 3 times, groups then pids, record survivors once, 40 rounds). The worker starts with `Setsid`, so no descendant can `setpgid` into a group of the Keeper's terminal session. The tracker's first read happens right after `cmd.Start` and before the `Wait` goroutine, `rootStart` is set once, and an unlisted root sets `blind`. `waggle dispatch clean <ticket>` removes a run's worktree, registration, tainted marker, takeover guard and run lock under `dispatch.lock` and the role lock, refuses symlinks and duplicate admin entries, and runs git pinned. The host-record acknowledge is split to D2b (D-185, D-196). 314 added non-test lines by `git diff --numstat c98d130 2f1352d` over files not ending in `_test.go` (target 200, review threshold 280, stop 350): the planned cut applied. Every file is under `internal/dispatch/` or `cmd/waggle/`; the diff holds no U+2013 or U+2014. `go build`, `go vet`, gofmt and `go test -race ./...` pass on this Mac (`internal/dispatch` 182 s); `product-ci` is green; `verify-leases-and-tiers` fails on the expected "no claim for T-023".

Blast radius: the sweep's reach is now bounded structurally. A group is signalled only when its id is the pid of a process the tracker saw as a tracked group leader, and every member of such a group is in a session created inside the worker's own `Setsid` session, hence a worker descendant; the only way a foreign process can sit in that group id is pid reuse after the group emptied, which the start-time guard excludes whenever the reused pid is listed. The one remaining theoretical path (a reused pid that creates group G, forks, and exits before the sweep, so G has members and no listed leader) is the "leader absent" case D-194 itself mandates; I note it (N2) with a one-line narrowing for a later lease. What the branch gets wrong is the one honesty sentence D-194 ruled on: the new macOS clause claims a background child that stays in its tool shell's group is killed by the group sweep, and the PR's own data (0 of 20) and mine (0 of 20 at two shell lifetimes) show that holds only when the shell was listed alive by a read before it exited. That is B1, a one-line string change that adds no lines.

## Blocking Issues

**B1. The macOS `NotEnforced` clause overstates the group sweep and drops the shape D-194 ordered named** [1 changed line, 0 added; `cell.go` line 50]. D-194's Keeper choice: the text "says plainly that on macOS a background child whose parent exits before the next 25 ms read, which is the usual cmd & from a tool shell, escapes until T-024". The committed clause reads "a background child that calls setsid or daemonizes before the next read escapes until T-024 (one that stays in its tool shell's group is killed by the group sweep)". The parenthesis is unconditional and false for the shape D-194 names. `targets` lists a group only for a leader in `leaders`, and `observe` adds a leader only when a read lists it alive (zombies are skipped in the walk); a tool shell that starts, forks and exits between two 25 ms reads is never a leader, and its child, though it stays in that group, is never signalled. The builder measured exactly this: `builder-darwin-catch.txt` is 0 of 20 when the leader exits at once, `builder-darwin-catch-100ms.txt` is 20 of 20 when it lives 100 ms. My data-only run (`media/T-023d2/fable-darwin-catch.txt`, no `sweep()`, no signal, 25 ms polls as `runSession` uses) separates the two conditions: with the tool-shell stand-in living 0 or 5 ms before its fork, the shell is seen alive 0 of 20 and the group is listed 0 of 20; living 20 or 50 ms before the fork, the shell is seen 20 of 20 and the group is listed 20 of 20 while the child itself is tracked by pid 0 of 20. So the rule is "the shell was seen alive by any read before it exited", and what the shell does after the fork no longer matters (which is why the old D-194 sentence is also out of date: the parent's exit before the next read no longer decides). The mandated shape (the usual `cmd &`) is caught or missed by how long the tool shell lives before the fork, which for `sh -c` is a few milliseconds and for a real Claude Code `zsh` tool shell is unmeasured in D2 (no real run is allowed on the Mac, D-193; the phase F trial is the first measurement, D-197). Wording I would put on the Keeper's card, replacing the clause from "on macOS" to "until T-024":

> on macOS (sysctl read every 25 ms) a background child escapes until T-024 when its tool shell started and exited between two reads, which is the usual cmd & from a short-lived shell (0 of 20 caught with a shell gone within 5 ms, 20 of 20 with one alive 20 ms), or when it calls setsid or daemonizes before the next read; a child that stays in the group of a tool shell the tracker listed alive is killed by the group sweep

The rest of the line (host record names hook and config writes; ref and main-checkout writes are not named) stays as it is. The phase F trial report carries the same sentence (D-194).

## Rulings on the queen's eight questions

**1. Group-leader sweep: every D-194 guard is present and pinned; no path signals outside the worker's tree except the mandated "leader absent" case under pid reuse (N2).**

- Guards in `targets`: `k.pid > 1`, `t.group > 1`, `k.pid != t.group`, `member` (a non-zombie process with `pgid == k.pid` in `tab`), `same` (no `tab` row with pid `k.pid` and another start), and the early `return nil, nil` when `tab` does not list `self`. Pids: `t.live` filtered to `> 1` and `!= self`. `observe` adds to `leaders` only inside the descendant walk (`t.seen[k]` and `p.pgid == p.pid`), so an untracked leader (the Keeper's shell 7000 in the fixture table) is never recorded. The root itself is never in `seen`, so the worker's group is never a sweep target; `signalGroup`'s `kill(-pid)` after `Wait` covers it, as in D1.
- Why the set is safe: the worker is a session leader (`Setsid`). A session can be joined only by fork from a member, and a group belongs to one session, so a group led by a tracked descendant contains only descendants of the worker. The Keeper's terminal session and the queen's tool-shell session cannot be reached by `setpgid` from inside (EPERM across sessions; `TestWorkerLeadsOwnSession` proves the EPERM on darwin and linux).
- Pgid reuse: the group id is the leader's pid. While any member lives the group exists and pid G cannot be allocated to a new process... except on linux, where a reused pid G can call `setpgid(0,0)` and join a still-populated orphan group of another session; and on both systems once the group is empty a new pid G can lead a new group G. In both shapes the new pid G is listed with another start, `same` is false, and the group is skipped (`TestTargetsOnFabricatedTables/leader pid reused`). The residue is N2.
- Own group from the table row for `self` instead of `Getpgid(0)`: justified. `GOOS=solaris go doc syscall.Getpgid` and `Getpgrp` both report "no symbol", and the same for illumos, so the brief's N19 spelling does not build there either. The row is refreshed on every read; a read that lacks `self` cannot signal anyway (`targets` returns nothing), and a failed first read leaves `group` 0, which yields no groups.
- Zombie-only groups not targeted: a safe deviation (a kill of zombies does nothing; `member` requires a live process), pinned by `TestTargetsOnFabricatedTables/only zombie members`.
- Leader that exits and its pid is reused as a new leader: `same` is false, skipped. Leader that exits leaving an orphan: `same` true (absent), `member` true, group signalled: the intended catch (`leader gone, orphan stays in its group`). A tracked process that calls `setsid` into a new session after it was first seen: its group is `pid == pgid` only after the call, and `leaders` is written at first sight only, so the group is not targeted (N3: conservative; its own pid is still killed by the pid path while it lives).
- Table row for `self` missing: `targets` returns nothing for that round; stale: `t.group` is rewritten on every read and dispatch never changes its group. Seams: `newTracker` sets `read: procTable` and `kill: syscall.Kill`; the inert tracker (`root <= 1`) leaves both nil, and `sweep()` on it would panic on `t.read()`; unreachable from `runSession` (the root is `cmd.Process.Pid` after a successful `Start`), noted as N4.
- Container evidence (`media/T-023d2/fable-ctr-unmutated.txt`): `TestSweep*`, `TestTargets*`, `TestObserve*`, `TestToolShellChildKilled`, `TestWorkerLeadsOwnSession`, `TestFastExitNeverBlind` and `TestRunLimitsKillTheGroup` pass in the container at head. My fix-removal set (`fable-fix-removal-table.txt`, chosen independently of the builder's 22): see the table below.

**2. Setsid: correct, and the effects the brief lists are the real ones.** `kill(-pid)` is unchanged because `setsid` makes the caller lead a group whose id is its pid. The worker's stdin is `os.DevNull` (`runSession` never sets `cmd.Stdin`), so losing the controlling terminal changes nothing there; `open("/dev/tty")` now fails with ENXIO where a background read on the Keeper's terminal used to stop on SIGTTIN. Claude Code headless already runs with a pipe on stdout and no TTY features, and its tool shells `setsid` themselves (D1 N1), so the ppid walk and the D-176 path are unchanged. Ctrl-C: SIGINT goes to the terminal's foreground group, which holds dispatch, not the worker, exactly as with `Setpgid` (the worker's group was never the foreground group); dispatch's context stops the worker. SIGHUP on a closed terminal: the kernel signals the foreground group and the shell signals its jobs; the worker was reachable by neither before and is not now. `TestWorkerLeadsOwnSession` proves it on both systems: the helper is a child of the worker, writes `getsid(0)` and `getppid()`, and they are equal (the helper's session id is the worker's pid, so the worker leads a session) and differ from the test's session, and `setpgid(0, <test's group>)` fails with EPERM (a cross-session join, since the helper is not itself a session leader). It passed in the host suite (darwin) and in the container (linux); mutant F5 (`Setsid` back to `Setpgid`) makes it fail in the container.

**3. Leader start before `Wait`: correct.** `tr := newTracker(cmd.Process.Pid)` runs after `cmd.Start` and before `go func() { cmd.Wait() ... }`, so the first read lists the leader alive or as a zombie that still carries its start time (darwin `kern.proc.all` lists `SZOMB` entries, linux `/proc/<pid>/stat` state `Z`); `observe`'s first loop sets `rootStart` without a zombie check, so a zombie leader is enough. `blind` is set when the root was never listed (`t.rootStart == ""` after the walk) or a read failed. `rootStart` is written only while empty, so a reused root pid after the reap cannot move the adoption threshold (`TestObserveRootStartSetOnce`; mutant F7). Read retry: `fails > 3` breaks, so a failure is followed by at most 3 more reads inside the 40-round bound, nothing is signalled on a failed read (`observe` empties `live`, `targets` sees a nil table), and `blind` stays set (`TestSweepReadRetry`; mutant F8).

**4. `dispatch clean`: as specified, with two cosmetic notes.** Lock order: `dispatch.lock` (`takeLock`; a live holder is exit 3 "a dispatch run is in progress in this repo"; `Run` holds that lock for its whole life, so a prune cannot land inside another run's snapshot-to-check window), then the role's `<wt>.lock` (live pid exit 3 with the lock untouched, because the `defer os.Remove` is registered only after a successful take; dead pid taken over with the warning). Refusals before any removal: `requireRealDir(wt)` joined with `EvalSymlinks` (symlink or non-directory exit 3), `noDuplicates` over every `<common>/worktrees/<id>/gitdir` read with `readSmall` (bounded, final link not followed, `e.IsDir()` from `Lstat` so a symlinked admin entry is ignored), exit 3 naming both ids and the relative path with nothing removed. Git calls: `(&Git{...}).pinned()` for `rev-parse`, `worktree list --porcelain`, `worktree remove --force <path from git's own list>` and `worktree prune`; `TestCleanGitCallsArePinned` plants a `trace2.eventTarget` in the global config and sees no event. Never deletes a branch (`worktree remove` does not), never touches a claim ref, never touches `host-files.json` (`TestCleanRemovesWorktreeAndMarkers` checks all three under both repo spellings). `--runs`: `requireRealDir(runs)` first (a symlinked `runs` is exit 3), then `os.RemoveAll` on entries matching `^T-023-(builder|forager|guard)-[0-9]+$` (RemoveAll does not follow a symlink entry). Exit codes: bad ticket or role 2, environment and git failures 3 naming the step, usage combos 2 in the CLI (`TestDispatchCleanCLI`). TOCTOU: between the checks and `git worktree remove` the only writer that could race a Keeper command is an escaped worker process (nothing else runs: dispatch is excluded by `dispatch.lock`); it would have to turn the real worktree directory into a symlink whose target passes git's `validate_worktree` back-reference, which is not the case for the Keeper's other repositories; a note, not a gap the brief asked to close. Cosmetic: "removed worktree" is printed before the `remove` result is checked, so a failed remove prints it and then the error (N5); `worktreesDir` creates `.waggle/worktrees` even in a directory that is not a repository before the `rev-parse` fails (N6).

**5. NotEnforced wording: not honest as committed; B1, with the sentence for the card.** The builder followed the brief's 10.7.3 item 2 text, so this is a scout drafting slip that the builder's own measurement then contradicted; the fix is one changed line.

**6. Tests: the D-195 move is complete; the probabilistic tests are sound; test 27 and the tag change are fine.** No file built for darwin calls `sweep()`: the two direct callers moved unchanged to `sweep_linux_test.go` (`//go:build linux`), and `grep -n 'sweep()'` over `internal/dispatch/*_test.go` finds callers only there. No committed test kills a pid read from a file: `TestSurvivorKilledAndRecorded`, `TestBlankNameChildKilled` and `TestStdoutHolder` lost their `syscall.Kill(pid, ...)` cleanups; their helpers wait for a release file and exit on their own within 60 s; `pidFrom` refuses `pid <= 1`. `TestFastExitNeverBlind` (20 runs, worker gone within milliseconds) is deterministic in the passing direction, since the first read precedes the `Wait` goroutine; mutant F6 (read after the goroutine) is caught. `TestNoRenewalStartsAfterSessionEnds`: the stub records the decision time as the renewal context's deadline minus `renewBound`, the worker marks the SIGTERM moment, and a decision later than that plus 20 ms fails; with the check in place every decision precedes the deadline that triggers the SIGTERM, so a false failure needs the clock to run backwards, and the 12 iterations exist only to catch the removal (the builder's mutant 16, caught 3 of 3). It adds about 12 fixtures to the host suite (the suite's `internal/dispatch` time stayed at 182 s against 187 s in the builder's run). Test 27 now vets `windows`, `solaris` and `illumos` on the host (in the suite; `builder-cross-build-vet.txt` agrees). `fifo_unix_test.go` gained `&& !solaris` because `syscall.Mkfifo` does not exist for solaris or illumos (`go doc` confirms); the file is not in the lease's enumerated edits but is inside the `internal/dispatch/` lease and the change is the one token test 27 needs (N7). The lease's test list also did not foresee `getsid_test.go`, `getsid_other_test.go` and `renewal_test.go` (new, test only). `TestToolShellChildKilled` on linux is satisfied by the subreaper path as much as by the group sweep, so it is a D2-T4 regression test rather than proof of the group path; the group path's proof is the data tests (N8).

**7. Size: 314 confirmed.** `git diff --numstat c98d130 2f1352d` over non-test files: 34 + 2 + 3 + 179 + 91 + 5 = 314 (D-136; moved code counted as added). Past the 280 review threshold and under the 350 stop; the planned cut (item 6 and its CLI to a D2b PR on the same lease) is what D-185 and D-196 prescribe for a build past 280, so the split is in order. `checkHostRecord`'s message change belongs to item 6 and is correctly absent.

**8. `gh pr checks 59`:** `product-ci` pass (3m0s), `verify-leases-and-tiers` fail ("leases: no claim for T-023; claims must stay until merge", the expected D-168 shape), `relay-claim` skipping.

## Fix removal in the container (`media/T-023d2/fable-fix-removal-table.txt`, one container at a time, linux binary cross-compiled on the host and never run there)

Each mutant is one substitution on a scratch copy of the branch (diff in `fable-mutant-<id>-diff.txt`, output in `fable-mutant-<id>.txt`, runner `fable-fctr2.sh.txt`, driver `fable-mutants.sh.txt`). All nine are caught; the builder's 22 (`builder-fix-removal-table.txt`) overlap F1, F5, F6, F7, F8, F10 and F11 and I did not rerun the rest.

| Mutant | Removed | Named test | Result |
| :--- | :--- | :--- | :--- |
| F1 | `targets`: a group needs a listed live member (`member &&` dropped) | `TestTargetsOnFabricatedTables` | FAILS (caught): group 501 listed with no member |
| F3 | `observe`: `leaders` holds only tracked processes (every table leader recorded) | `TestObserveRecordsOnlyTrackedLeaders`, `TestTargetsOnFabricatedTables` | FAILS (caught): the Keeper's shell group 7000 becomes a target |
| F5 | `cell_unix.go`: `Setsid` back to `Setpgid` | `TestWorkerLeadsOwnSession` | FAILS (caught) |
| F6 | `run.go`: tracker's first read moved after the `Wait` goroutine | `TestFastExitNeverBlind` | FAILS (caught) at 29 s, inside the 20 runs |
| F7 | `observe`: `rootStart` set once | `TestObserveRootStartSetOnce` | FAILS (caught) |
| F8 | `sweep`: read retry bound of 3 (made 1000) | `TestSweepReadRetry` | FAILS (caught): 40 reads on a persistent failure |
| F9 | `targets`: `member` computed from `live` instead of the table (an unseen orphan no longer counts) | `TestTargetsOnFabricatedTables` | FAILS (caught): `leader gone, orphan stays in its group` |
| F10 | `targets`: `same` always true (a leader pid listed with another start no longer excludes) | `TestTargetsOnFabricatedTables` | FAILS (caught): `leader pid reused` |
| F11 | `targets`: the "table lists self" early return removed | `TestTargetsOnFabricatedTables` | FAILS (caught): nil, empty and self-less tables |

## Checks

- Host (darwin, committed suite only): gofmt clean, `go build ./...`, `go vet ./...`, `go test -race ./...` all ok (`media/T-023d2/fable-host-race.txt`).
- Container at head: the sweep, targets, observe, session and group-kill tests pass (`fable-ctr-unmutated.txt`).
- Darwin data-only measurement (`fable-darwin-catch.txt`; source `fable-darwin-catch_test.go.txt`): no `sweep()`, no signal, every process exits on its own; removed from the scratch copy after the run; nothing of mine is in the branch.
- Lease: every changed file is under `internal/dispatch/` or `cmd/waggle/`; `main.go`, `go.mod`, `internal/runtime/`, `.github/` and `.waggle/` untouched; production edits limited to the files and regions 10.7.2 names.
- Dashes: none in the diff (`grep` for U+2013 and U+2014 over the saved diff: 0).
- Acceptance: D2-AC1 (targets guards and sweep equals targets) by `targets_test.go` and `sweep_linux_test.go`; AC2 by `TestWorkerLeadsOwnSession` and test 13; AC3 by `TestObserveRootStartSetOnce` and `TestFastExitNeverBlind`; AC4 by `clean_test.go` (item 6 deferred to D2b per D-196); AC5 holds (no darwin file calls `sweep()`, every mutant and container command is named in the builder's media and mine); AC6: the darwin measurement and the line count are in `media/T-023d2/`, the three vets pass, size 314.

## Notes

- **N1 (Keeper, the card).** B1's sentence. If the Keeper prefers to keep D-194's original sentence verbatim, it is still true in the direction that matters (the child escapes) but no longer names the right cause; the sentence in B1 names it.
- **N2 (later lease, 1 line, scout).** The "leader absent" case admits one pid-reuse residue: after a tracked leader G exits and its group empties, an unrelated process gets pid G, leads group G (or on linux joins the orphan group with `setpgid(0,0)`), forks, and exits before the sweep; the sweep then sees members and no leader and signals G. It needs a full pid-space wrap inside one run (about 100k forks on macOS, more on linux) and a leader that dies leaving children, so it is remote; `observe` could drop a leader from `leaders` the first time a read lists neither its pid nor any member of its group, since a group that emptied can only be recreated by a reused pid. One line in the walk's first loop.
- **N3 (zero lines).** `leaders` is written at first sight only. A tracked process seen before it calls `setsid` or `setpgid(0,0)` (the fixture's `session` helper is this shape: a Go binary that calls `Setsid` a few milliseconds after start) is never a leader, so a child it later leaves in its group escapes the group sweep when the process itself is gone before the sweep. Claude Code's tool shells are session leaders from birth (`setsid` before `exec`), so the product shape is covered. Refreshing `leaders` on every read for a live tracked process with `pgid == pid` would close it at the same safety argument (the group is still inside the worker's session); 3 lines, not for D2.
- **N4 (zero lines).** The inert tracker (`newTracker(root <= 1)`) has nil `read` and `kill`; `sweep()` on it would panic. `runSession` never builds one. Giving the inert tracker no-op seams, or an early `return` in `sweep` when `t.read == nil`, would make the incident probe's shape (`newTracker(0)` then `sweep()`) a no-op instead of a crash.
- **N5 (builder, cosmetic, D2b or later).** `clean` prints "removed worktree" before checking the `worktree remove` result.
- **N6 (zero lines).** `Clean` on a directory that is not a repository creates `.waggle/worktrees/` there before `rev-parse` fails (shared with `Run`).
- **N7 (queen, lease record).** `fifo_unix_test.go` (one build-tag token) and the new `getsid_test.go`, `getsid_other_test.go`, `renewal_test.go` are inside `internal/dispatch/` but outside 10.7.2's enumerated test files; all test only, all needed by D2-T5, D2-T10 and test 27.
- **N8 (zero lines).** On linux `TestToolShellChildKilled` passes through the subreaper as well as the group sweep, so it does not isolate the group path; `TestSweepKillsTrackedGroup` (leader seeded into `leaders`) and the data tests do. On darwin the group path is exercised only by my measurement and, in phase F, by the trial run.
- **N9 (zero lines).** `getsid_other_test.go` returns -1 and the comment says the session test then relies on EPERM alone, but `TestWorkerLeadsOwnSession` also requires `got[0] != strconv.Itoa(getsid())`, which is "-1" != "-1" there, so on openbsd, netbsd and dragonfly the test would fail. No such host runs the suite; the comment and the check disagree.
- **N10 (phase F, Keeper should know).** D-197 makes the first macOS trial run the proof; its probe 2 shape (a foreground tool child held past the stop) exercises the pid path, not the group path. A second trial command of the `cmd >/dev/null 2>&1 &` shape would show whether a real Claude Code `zsh` tool shell lives the 20 ms before its fork that makes the group sweep catch the child, which is the open number behind B1's sentence.


## Confirmation at 9edaf84

Verdict: pass
Risk-Tier: 3

Fix round `2f1352d..9edaf84` (commits b9611f2 and 9edaf84; 93 insertions, 20 deletions over 6 files), confirmed independently of the opus guard (D-184). On this Mac only the committed suite, host git probes that signal nothing, and two data-only `observe`/`targets` runs on extracted copies of the trees ran; the one probe that calls `sweep()` ran in the Colima container under the shared lock (D-193, D-195). Nothing of mine is in the branch.

**Size, CI, suite.** `git diff --numstat c98d130 9edaf84` over files not ending in `_test.go`: 34 + 2 + 3 + 186 + 101 + 5 = **331** added non-test lines (D-136; stop 350). Every changed file is under `internal/dispatch/` or `cmd/waggle/`. No U+2013 or U+2014 in the fix range or the full range. `go build ./...`, `go vet ./...`, gofmt clean; `go test -race -count=1 ./...` on the host ok, `internal/dispatch` 187 s (`media/T-023d2/fable-confirm-host-race.txt`). `gh pr checks 59` at `9edaf84`: product-ci pass (2m18s), verify-leases-and-tiers fail with "leases: no claim for T-023; claims must stay until merge" (expected), relay-claim skipping.

**1. Opus B1 fix (my N2): closed; accepted.** `observe` now ends with: when the table lists `self`, every `leaders` entry whose pid is nobody's `pgid` in that table is deleted. Zombies count as members because the check reads `p.pgid` without a zombie test, and a leader first recorded in the same call is listed alive with `pgid == pid`, so it is never dropped at first sight. A failed read passes a nil table (no `self`, no drop) and a table that lacks `self` cannot drop either. Data run on both trees (`fable-confirm-leader-drop.txt`, source `fable-confirm-data_test.go.txt`, no table read, no signal, no `sweep()`):

| Case (group 510 is a tracked leader with child 511) | 2f1352d groups | 9edaf84 groups |
| :--- | :--- | :--- |
| P1: a read lists the group empty, then a foreign member 9000 (ppid 1, pgid 510) | [501 510] | [501] |
| Orphan catch: leader gone, child 511 listed on every read | [501 510] | [501 510] |
| Zombie leader holds the group, then the orphan alone | [501 510] | [501 510] |
| Transient omission: one successful read lists neither 510 nor 511, then the orphan | [501 510] | [501] |
| Failed read between: not a drop | [501 510] | [501 510] |
| Table without self between: not a drop | [501 510] | [501 510] |

The first row is P1 and N2 closed: once a group is listed empty, its id is never signalled again, and a pid wrap inside one 25 ms poll is the only residue. The fourth row is the one new path: a group whose only live member is absent from one successful read. On linux it does not exist: `/proc` lists every live task and every zombie, and a child orphaned inside the window is adopted by dispatch (subreaper) and tracked by pid. On macOS `kern.proc.all` collects the pid list under the process list lock and then looks each pid up again; a process that goes from alive to zombie between the two steps is absent from that one snapshot, and a child it forked after the collection is absent too. So the escape needs the shell's child C to fork D and exit inside the read's own iteration (well under a millisecond, inside one 25 ms poll), with D staying in group 510 and never calling setsid: a double fork without setsid, which the macOS clause already names as "daemonizes before the next read". 2f1352d would have caught D on the next round; 9edaf84 trades that corner for never signalling a foreign group, which is the direction D-193 and D-194 order. Accepted. N11 names a two-line narrowing for later. The container run at head passes every `TestSweep*`, `TestTargets*`, `TestObserve*`, `TestTargetsDropGroupSeenEmpty` and `TestTrackerStartsBeforeWait` (`fable-confirm-container.txt`); the builder's B1 fix-removal (`builder-fix1-container-mutant-b1.txt`) fails `TestTargetsDropGroupSeenEmpty` with groups [501 510], the P1 shape.

**2. Opus B2 fix: closed; no false refusal of a healthy repo.** `noDuplicates()` now walks every `<common>/worktrees/<id>`, refuses an entry that is not a directory or whose `gitdir` is not a readable regular file within the bound (`readSmall`: `Lstat`, no link followed, `maxSmall`), strips `/.git`, resolves the path with `EvalSymlinks` when it exists, and refuses the first path seen twice, before any git call that removes. The committed `TestCleanRefusesForeignDuplicateAndLinkedGitdir` covers both opus probes, and the builder's two fix-removal runs fail it in the right places (`builder-fix1-b2-link-fix-removal-host.txt`, `builder-fix1-b2-n7-fix-removal-host.txt`). My host probes (`fable-confirm-b2-probe.txt`, script `fable-confirm-b2-probe.sh.txt`, git 2.54 with the global and system config disabled, nothing signalled):

- A: T-023 and T-024 worktrees plus a prunable entry for a deleted third worktree: exit 0, T-023 removed, T-024 intact, the prunable entry pruned by clean's own `git worktree prune` (repository-wide, as the brief specifies).
- B: a worktree deleted with `rm -rf` and re-added at the same path: git 2.54 refuses the re-add ("missing but already registered worktree"), so a modern git never leaves two entries for one path by itself; clean on the prunable entry: exit 0 as A.
- C: a locked T-024 entry (`worktrees/<id>/locked`): exit 0, T-024 kept and still locked.
- D: a stray regular file `worktrees/.DS_Store`: exit 3 "worktrees/.DS_Store has no readable gitdir; nothing was removed"; `git worktree prune -v` removes it ("not a valid directory"). N12.
- E: an entry directory with no `gitdir` (an interrupted add): exit 3 as D; git prunes it ("gitdir file does not exist"). N12.
- F: the repo reached through a symlinked parent, T-023 added by the link path and T-024 by the real path: git stores real paths in both `gitdir` files, exit 0, T-023 removed, T-024 intact.

So the refusals beyond the two opus shapes are the states git itself would prune (D, E); each exits 3 naming the entry with nothing removed, and the Keeper's `git worktree prune` clears it. Conservative and inside the item 5 rule ("refuse anything git would follow or misread"); not a must-fix.

**3. My N4: the inert tracker's seams; ruled, nil was not safer.** `newTracker(root <= 1)` returns `{blind: true, read: procTable, kill: sysKill}` with `self` 0 and `seen` nil. Proof that `sweep()` on it signals nothing: `observe` begins with `if err != nil || t.root <= 1 { tab, t.blind = nil, true }`, so the table the sweep read is discarded before any walk; `targets(tab)` then gets that same real table from `sweep`, but `targets` names groups only from `t.leaders` (nil: nothing was ever walked) and pids only from `t.live` (emptied by `observe` from the nil table), so it returns nothing; `sweep` breaks at `len(pids)+len(groups) == 0` after one read and returns nil without reaching `t.kill`. Container evidence (`fable-confirm-container.txt`, source `fable-confirm-linux_test.go.txt`, linux build tag): for roots 0, 1 and -7, with a recording kill seam the sweep made one real table read, zero kills, zero survivors, `blind` true, `seen`, `live` and `leaders` empty; then with the real seams the sweep returned zero survivors. Nil was not safer: a nil seam turns the same call into a panic, which inside the `swept` goroutine of `runSession` would take dispatch down after the worker's run with no record written, while a no-op leaves a `blind` warning. Both are unreachable from `runSession` (the root is `cmd.Process.Pid` after a successful `Start`); the no-op is the better failure. N14 notes the missing pin.

**4. `TestTrackerStartsBeforeWait`: acceptable.** The property is program order (the goroutine that calls `cmd.Wait` must be started after `newTracker`), and source order is that property for straight-line code in one function; a runtime test cannot show it reliably (opus N1, my question 3 of the first review). The test reads `run.go` from the package directory `go test` guarantees as cwd, finds `newTracker(cmd.Process.Pid)` and the first `cmd.Wait()`, and fails when either is missing or the order is reversed; the builder's host mutant (`builder-fix1-host-mutant-n1.txt`) fails it at the D1 position, which is the regression opus found undetected. Every way the check can be wrong fails loudly (a renamed call or file, a `cmd.Wait()` placed earlier in a comment or helper), never silently. It costs zero non-test lines. Accepted.

**5. The macOS clause.** `cell.go` line 50 from "on macOS" to the colon equals the D-199 quotation byte for byte (compared programmatically from the uncommitted `DECISIONS.md` against the source). A repo-wide grep for the old parenthesis ("stays in its tool shell's group is killed") finds nothing outside the two guard reviews and the builder media. The only other "group sweep" mention outside the hive records is the D-194 rationale in `DECISIONS.md` ("closes most of the cmd & case"), a historical decision record that D-199 amends, not product text.

**6. Numbers.** 331 added non-test lines (table above); CI as above; host race suite ok once.

### Cosmetic notes from the fix round (zero lines now)

- **N11 (later lease, 2 lines).** Dropping a leader after one empty read opens the macOS corner in row 4 of the table. Requiring two consecutive successful reads that list the group empty (50 ms) keeps the pid-wrap protection (no wrap completes in 50 ms) and closes the corner. Scout item for the next sweep lease.
- **N12 (Keeper should know).** `clean` exits 3 on a stray regular file or a gitdir-less directory in `<common>/worktrees` (a `.DS_Store` from browsing `.git` in Finder, an interrupted add), where `git worktree prune` removes them. The message names the entry; the recovery is `git worktree prune`.
- **N13 (builder, cosmetic).** In `noDuplicates`, `if rp, e := filepath.EvalSymlinks(p); e == nil` shadows the loop's `e` (the `DirEntry`) inside the `if`; it compiles and vets, and the shadowed name is not used there, but the next reader will stumble.
- **N14 (zero lines, optional).** No committed test pins that `newTracker(0).sweep()` signals nothing; a 10-line case in `sweep_linux_test.go` (the recorder shape of my container probe) would, at zero non-test lines.
- The `noDuplicates` refusal now prints the absolute worktree path instead of the ticket-relative one; it goes to the Keeper's terminal only, not to a record.

### Evidence files (`media/T-023d2/`)

`fable-confirm-host-race.txt`, `fable-confirm-container.txt` (runner `fable-confirm-fctr3.sh.txt`), `fable-confirm-leader-drop.txt` (source `fable-confirm-data_test.go.txt`), `fable-confirm-linux_test.go.txt`, `fable-confirm-b2-probe.txt` (script `fable-confirm-b2-probe.sh.txt`), `fable-confirm-checks.txt`.


## Confirmation at 2d1a1b3

Verdict: pass
Risk-Tier: 3

Fix round 2, `9edaf84..2d1a1b3` (one commit; 44 insertions, 5 deletions over `clean.go`, `clean_test.go`, `sweep_linux_test.go`), confirmed independently of the opus guard (D-184). This round touches git-only code and adds one linux-only test, so on this Mac only the committed suite and git-only host probes ran; no signal was sent and no container run was needed (D-193, D-195). Nothing of mine is in the branch.

**Size, CI, suite.** `git diff --numstat c98d130..2d1a1b3` over files not ending in `_test.go`: 34 + 2 + 3 + 189 + 101 + 5 = **334** added non-test lines (D-136; stop 350); the round adds 3. Every changed file is under `internal/dispatch/` or `cmd/waggle/`. No U+2013 or U+2014 in the full-range diff or the commit message. The commit is signed off by `paulternate <27632860+paulternate@users.noreply.github.com>`. `go build ./...`, `go vet`, gofmt clean; `GOOS=linux go vet` and `GOOS=linux go test -c` of `internal/dispatch` compile the new linux-only test. `go test -race -count=1 ./internal/dispatch/... ./cmd/...` on the host ok once, `internal/dispatch` 185 s, `cmd/waggle` 17 s (`media/T-023d2/fable-confirm2-host-race.txt`). `gh pr checks 59` at head `2d1a1b3`: product-ci pass (2m26s), verify-leases-and-tiers fail on the expected "no claim for T-023", relay-claim skipping.

**1. Opus B4 (case-folded duplicate compare): closed.** `noDuplicates` keys `byPath` on `strings.ToLower(p)` at both the lookup (`clean.go:160`) and the store (`clean.go:163`); the message still prints the un-folded path. Host probes, git 2.54 (Apple), `core.ignorecase` true (`media/T-023d2/fable-confirm2-probe.txt`, script `fable-confirm2-probe.sh.txt`):

- P1: planted `worktrees/0/gitdir` naming `t-024-builder/.git` (the opus P-c spelling): exit 3 "worktrees/0 and worktrees/T-024-builder both name ...; nothing was removed", every admin entry and both worktrees intact, T-024's `git status` clean.
- P1b: the same against the cleaned ticket's own worktree (`t-023-builder`): exit 3, nothing removed.
- P2 (false refusal check): a healthy repo with `T-023-builder`, `T-024-builder`, `T-024-guard`, `T-024-forager` and a user worktree `p2-Mine` outside `.waggle`: exit 0, "removed worktree .waggle/worktrees/T-023-builder", the other four listed and intact. Waggle's own paths cannot differ only in case because `comb.ParseTicketID` canonicalizes the id and the roles are fixed lower-case strings, so the only path to a case-only refusal is two user worktrees that differ only in case on a case-sensitive filesystem, which exits 3 with nothing removed (the safe direction opus named in B4). `ToLower` folds more than git's `strcasecmp` (non-ASCII case), again only toward refusal.

**2. Opus N9 / my N12 (skip of stray entries): closed; the skip cannot steer git's duplicate pass.** The new line skips a regular-file entry and a directory entry whose `gitdir` read fails with `ErrNotExist` only (Go parses `a || b && c` as `a || (b && c)`). In git's `should_prune_worktree`, a non-directory entry is "not a valid directory" (`delete_git_dir` falls back to `unlink` on ENOTDIR) and a directory without `gitdir` is "gitdir file does not exist"; neither returns a path, so neither enters the `kept` list that `prune_dups` compares, and a `locked` gitdir-less directory is kept by git, again without a path. So a skipped entry can neither be chosen by nor change the choice of `prune_dups`, and a prune cannot reach a registered worktree through it. Probes (same media):

- P3: `worktrees/.DS_Store` file, `worktrees/half` dir with no gitdir, `worktrees/lk` dir with `locked` and no gitdir: exit 0, T-023 removed, T-024 intact, git pruned `.DS_Store` and `half` and kept `lk`. P5 shows a plain `git worktree prune -v` on the same strays does the same two removals.
- Every hostile shape still refuses with exit 3, both worktrees intact: P4a a symlinked admin entry (`ReadDir` types it by `lstat`, so it is neither regular nor a directory, and the read through the link succeeds, which lands in the `!e.IsDir()` refusal); P4b a `gitdir` that is a symlink to T-024's; P4c a dangling `gitdir` symlink (`readUpTo` begins with `Lstat`, so the error is "not a regular file", not ENOENT); P4d `gitdir` mode 000; P4e `gitdir` of 4097 bytes; P4f `gitdir` that is a directory; P4h a dangling symlink as the entry itself; P4i a FIFO as the entry. P4g (a case-variant entry directory) is the same directory on APFS, so `mkdir` fails and clean behaves as on a healthy repo. P4j (an empty `gitdir`) proceeds and git prunes the entry as "invalid gitdir file", harmless since it carries no path.

**3. N13 and N14: closed.** The `EvalSymlinks` result is now `serr`, so the loop's `e` is no longer shadowed. `TestInertTrackerSweepSignalsNothing` (`sweep_linux_test.go`, linux build tag) is the recorder shape of my container probe: for roots 0 and 1 it leaves `read` on the real table, replaces `kill` with a counter and asserts zero calls, zero survivors and `blind` set. It cannot run on this Mac (D-195); it compiles under `GOOS=linux`, and product-ci (ubuntu) is green at this head, which runs it. The new `case` mode of `TestCleanRefusesForeignDuplicateAndLinkedGitdir` and `TestCleanSkipsStrayAdminEntries` ran here in the race suite.

**4. Numbers.** 334 added non-test lines; CI as above; host race suite ok once.

### Notes (non-blocking)

- The skip's comment says "git prunes a stray file or a gitdir-less entry"; a `locked` gitdir-less directory is kept by git instead (P3's `lk`). It makes no difference to safety, since git keeps it without a path, and the Keeper's `git worktree prune` message will not mention it. Zero lines.

### Evidence files (`media/T-023d2/`)

`fable-confirm2-probe.txt` (scripts `fable-confirm2-probe.sh.txt`, `fable-confirm2-probe2.sh.txt`), `fable-confirm2-host-race.txt`.

## Confirmation at ddccd51

Verdict: must-fix
Risk-Tier: 3

Fix round 3, `2d1a1b3..ddccd51` (one commit; `clean.go` +16/-8, `clean_test.go` +19/-3), confirmed independently of the opus guard (D-184). Git-only host probes on this Mac (git 2.54.0, Apple, `core.ignorecase` true), the committed suite on the host, no signal sent, no container (D-193, D-195). Nothing of mine is in the branch; the one patch I tried was reverted with `git checkout` and never committed.

Opus B5 is closed: every spelling of the `.git` suffix now refuses. But the same hunt the queen asked for (a path git treats as equal that `SameFile` does not) found one more, in the relative gitdir branch, and it is the same silent prune of another ticket's registration. That is B6.

### Blocking

**B6. A relative gitdir is joined lexically, but git resolves it with realpath, so a symlink followed by `..` names one worktree for git and another for clean (`clean.go:159-161`; about 4 lines plus a test mode).** Git 2.54 `should_prune_worktree` (worktree.c) keeps an absolute gitdir as written, but a relative one is appended to the realpath of the admin entry and passed through `strbuf_realpath_forgiving`, which follows links component by component before applying `..`. Clean does `filepath.Join(admin, g)`, and `Join` cleans lexically, so `X/..` cancels before `X` is ever looked at. Plant `worktrees/0/gitdir` = `../../../Z/X/../T-024-builder/.git` with `Z/X` a symlink to `.waggle/worktrees/T-024-builder` (or to any directory under `.waggle/worktrees`): git resolves it to `<repo>/.waggle/worktrees/T-024-builder/.git`, byte-equal to T-024's real entry, and `prune_cmp` orders id `0` before `T-024-builder`, so `prune_dups` drops T-024's admin entry. Clean resolves it to `<repo>/Z/T-024-builder/.git`, stats a different directory (or none), finds no duplicate, and runs the prune. Probes P11 and P12 (`media/T-023d2/fable-confirm3-probe.txt`): `git worktree prune --dry-run -v` prints "Removing worktrees/T-024-builder: duplicate entry"; `waggle dispatch clean 23` exits 0 with "removed worktree .waggle/worktrees/T-023-builder", the admin dir is left with `0` only, and T-024's worktree answers `fatal: not a git repository: (null)`. P12 puts the link inside a third ticket's worktree (`T-025-builder/X`), which `clean 23` does not remove, so a hostile builder needs nothing outside `.git/worktrees/0` and its own tree. P9 shows why the link must survive the cleaned worktree: with `X` inside T-023's tree, `worktree remove --force` deletes it first and git then prunes `0` as non-existent, so T-024 lives by ordering, not by the check. P13 (the same relative path with no link in it) and P8 (a plain relative path) refuse, which isolates the cause to link-then-dotdot. This hole predates round 3: at 2d1a1b3 `EvalSymlinks` ran on the already-cleaned `Join` result, so I missed it then too.

Fix: resolve a relative gitdir the way git does, before any cleaning, and refuse when it does not resolve, since git would then prune that entry itself or follow a path clean cannot see:

```go
if !filepath.IsAbs(g) { // git resolves a relative gitdir with realpath: links before any ..
	rp, rerr := filepath.EvalSymlinks(admin + string(filepath.Separator) + g)
	if rerr != nil {
		return failf(ExitEnv, "worktrees/%s has a relative gitdir that does not resolve; nothing was removed", e.Name())
	}
	g = rp
}
```

Go's `EvalSymlinks` walks components in order and applies `..` to the resolved prefix, so it matches `strbuf_realpath` here. I tried exactly this patch in my worktree: P11, P12, P13 exit 3 with every entry kept; P14 (a healthy repo with `worktree.useRelativePaths=true`, where git itself writes `../../../.waggle/worktrees/T-024-builder/.git`) still exits 0 and removes only T-023; probe set 1 is unchanged (P6 exit 0, everything else exit 3); `go test -race -run TestClean ./internal/dispatch/` ok. Line budget: 342 + 4 = 346, under 350. Test: an eighth mode in `TestCleanRefusesForeignDuplicateAndLinkedGitdir`, say `relative`, that writes `../../../Z/X/../T-024-builder/.git` after linking `Z/X` to T-024's worktree; it runs on linux too, since nothing in it depends on case folding. Refusing every relative gitdir outright (3 lines) also closes it but falsely refuses a `useRelativePaths` repo, so I would not take that route.

### 1. The seven asked spellings

All exit 3, every admin entry and both worktrees kept, T-024 `git status` clean: `/.GIT`, `/.Git`, `/.gIt` (P1), the whole path upper-cased (P2), a symlinked parent `ln -> .waggle/worktrees` (P3), a copied gitdir (P4) and a `gitdir` that is a symlink to T-024's (P5, which lands in the "no readable gitdir" refusal rather than the duplicate one, as before).

### 2. Git's equality versus `SameFile`

From the 2.54 source: `prune_worktrees` keeps, per entry, the raw gitdir string (absolute) or its realpath (relative), appends the common dir, sorts with `prune_cmp` (`fspathcmp`, then main first, then id), and prunes the later of any two `fspathcmp`-equal strings. `fspathcmp` is `strcasecmp` under `core.ignorecase`, else `strcmp`; no trailing slash, `/./` or `..` handling, no Unicode normalization. An entry only enters the kept list when `file_exists` (lstat) of the string succeeds; `expire` is `TIME_MAX`, so a missing path is always pruned on its own and never compared.

Spellings git keeps apart but clean folds, all exit 3 (P7a to P7e): trailing slash, `/./`, a lexical `..`, the worktree directory without `.git`, and an NFD spelling of an NFC worktree name (`café`; the real entry's gitdir is NFC, APFS resolves both to one inode). Each is a refusal under a planted entry only, the safe direction. The one-sided missing case cannot cross: `os.Stat(p)` failing implies git's lstat of `p/.git` fails, so git prunes that entry itself and it is never in the kept list; the lowercased fallback only ever compares two entries git would both prune. Case: on this case-insensitive disk every `strcasecmp`-equal pair resolves to one inode, so `SameFile` covers git's fold. The residual is `core.ignorecase=true` with the worktrees on a case-sensitive volume, where two distinct directories differing only in case can both exist and git merges them while `SameFile` does not; round 2's always-on lowercased key caught that and round 3 narrowed it to the both-missing case. It needs a misconfigured or cross-volume repo plus a planted entry; I note it below rather than block on it. The relative branch is the one real divergence, B6 above.

### 3. No false refusal

P6 (T-023-builder, T-024-builder, T-024-guard, T-024-forager, a user worktree `p6-Mine` outside `.waggle`) exits 0, removes only T-023, leaves five registered. P14 with `worktree.useRelativePaths=true` exits 0 on the committed binary too.

### 4. Tests on a case-sensitive filesystem

The skip is `os.Stat(filepath.Dir(g))`. On linux `case` and `upper` skip (the lowercased directory does not exist), but `GIT` and `Git` run, because `Dir` of `.../T-024-builder/.GIT` is the real `T-024-builder`, and `copy`, `link` and `parent` never depended on case. So linux CI exercises five of seven modes, including the `EqualFold` suffix strip and the `SameFile` fold through a link. The skips are silent `continue`s; a `t.Logf` would make the ubuntu log show which ran, zero risk either way.

### 5. Numbers

342 added non-test lines over `c98d130..ddccd51` (D-136; stop 350), every file under `internal/dispatch/`. No U+2013 or U+2014 in the range diff or the commit message. Signed off by `paulternate <27632860+paulternate@users.noreply.github.com>`, author the same. `gh pr checks 59` at head `ddccd51`: product-ci pass (2m23s), verify-leases-and-tiers fail on the expected "no claim", relay-claim skipping.

### 6. Host suite

`go test -race ./internal/dispatch/... ./cmd/...` on the committed tree: `internal/dispatch` ok 186.2 s, `cmd/waggle` ok 17.3 s (`media/T-023d2/fable-confirm3-host-race.txt`).

### Notes (non-blocking)

- Round 3 dropped the lowercased compare for two existing paths, so `core.ignorecase=true` over a case-sensitive worktree volume (two real directories differing only in case, one planted entry) is no longer refused while git would merge them. Exotic; adding `|| s.key == strings.ToLower(p)` unconditionally would close it at the price of refusing two legitimately case-distinct user worktrees on linux. Zero lines required this round.
- The test's `continue` skips leave no trace in the CI log.

### Evidence files (`media/T-023d2/`)

`fable-confirm3-probe.txt` (scripts `fable-confirm3-probe.sh.txt`, `fable-confirm3-probe2.sh.txt`), `fable-confirm3-host-race.txt`.

## Confirmation at 5ecb9c0

Verdict: must-fix
Risk-Tier: 3

Fix round 4, `ddccd51..5ecb9c0` (one commit; `clean.go` +22/-26, `worktree.go` +9, `clean_test.go` +55/-4), confirmed independently of the opus guard (D-184). Git-only host probes on this Mac (git 2.54.0 Apple, `core.ignorecase` true), the committed suite on the host, no signal sent, no container (D-193, D-195). Nothing of mine is in the branch; no patch was tried this round.

D-200 is implemented as ruled: the dry run runs under the pinned env plus C locale before anything is removed, every earlier spelling is now decided by git, and waggle's own comparison is gone. One probe still drops another ticket's registration, and it is a new shape rather than a leftover: the dry run and the real prune look at different filesystems, because `worktree remove --force` runs between them. That is B7.

### Blocking

**B7. The real prune runs after `worktree remove --force` deleted the cleaned tree, so a relative gitdir that resolves harmlessly through a symlink inside that tree resolves lexically onto another ticket once the link is gone (`clean.go:105-118`; about 3 lines).** Git's `should_prune_worktree` resolves a relative gitdir with `strbuf_realpath_forgiving` (`REALPATH_MANY_MISSING`): an existing symlink is followed and `..` strips the link's target, but a missing component is kept as spelled and `..` then strips it lexically. Plant `worktrees/0/gitdir` = `../../../.waggle/worktrees/T-023-builder/X/../../T-024-builder/.git` with `X` a link inside T-023's own tree to `D/E` (two levels down). Before the remove, git follows `X` to `T-023-builder/D/E`, the two `..` climb to `T-023-builder`, and the entry names `T-023-builder/T-024-builder/.git`: absent, so the dry run prints "gitdir file points to non-existent location", no duplicate, and clean proceeds (P15; in P15b the builder creates that directory, the entry is kept and the dry run prints nothing). Clean then removes T-023's worktree, which deletes `X`, and runs the real prune: `T-023-builder` and `X` are now missing, the two `..` strip `X` and `T-023-builder` lexically, the entry resolves to `.waggle/worktrees/T-024-builder/.git`, byte-equal to T-024's, and `prune_cmp` drops T-024 (id `0` sorts first). Result (`media/T-023d2/fable-confirm4-probe.txt`): exit 0, "removed worktree .waggle/worktrees/T-023-builder", admin dir left with `0` only, T-024's worktree answers `fatal: not a git repository: (null)`. P15c runs the same two dry runs by hand around an `rm -rf` of T-023's tree and shows git itself switching from "0: non-existent" to "T-024-builder: duplicate entry". A hostile builder needs only its own tree and `.git/worktrees/0`, as in B6, and round 3's P9 was this shape one `..` short.

Fix: ask git again at the moment that matters, right before the real prune, since the remove is the only step between the two that changes the filesystem and nothing clean does after the second dry run changes it again:

```go
if err == nil {
	if err = c.noDuplicates(ctx); err != nil {
		return err
	}
	_, err = c.pg.Run(ctx, "worktree", "prune")
}
```

That is 3 added lines, 347 to 350, at the D-190 stop and not past it; D-200 names D2b as the fallback if the queen reads the stop as exclusive. The first call stays, so the "nothing was removed" wording holds for every refusal the Keeper is likely to see; a refusal from the second call comes after T-023's own tree is gone, which is the thing clean was asked to remove, and the message then overstates by that one tree. A fourth line can make it exact if wanted. The alternative, dropping clean's own `worktree prune` (2 lines fewer; `worktree remove --force` already deletes the admin entry of a missing tree), ends the class outright but changes what clean does for stale entries, so it is the queen's call, not mine. Test: a `toctou` mode in `TestCleanRefusesForeignDuplicateAndLinkedGitdir` cannot reuse the table's dry-run gate (the dry run before the remove says no duplicate); a separate test that plants `X -> D/E` inside T-023's tree and the gitdir above, then expects exit 3 and T-024's admin entry present after clean, runs on linux too.

### 1. Earlier bypasses and B6 probes

`copy`, `case`, `.GIT`, `.Git`, `upper`, P11 and P12 exit 3 with "git sees duplicate worktree entries (worktrees/T-024-builder)", every admin entry and both worktrees kept, T-024 `git status` clean. `link` exits 3 on the kept "no readable gitdir" refusal. `parent` (an absolute gitdir through a symlinked parent directory) is the other ruled outcome: git keeps an absolute gitdir as spelled and sees no duplicate, so clean exits 0, removes only T-023, and T-024's entry and tree are untouched; the planted entry stays registered as a second spelling of T-024's path, which a later `clean 24` prunes as non-existent after removing the real one. P14 (`worktree.useRelativePaths=true`) and P6 (five worktrees, one outside `.waggle`) exit 0 and remove only T-023.

### 2. Parse robustness and git versions

The matched line is `fprintf_ln(stderr, _("Removing %s/%s: %s"), "worktrees", id, reason)` from `prune_worktree`, with `reason` the untranslated-under-C `"duplicate entry"` from `prune_dups`; both strings and the stderr target are unchanged from the 2.23 series that added duplicate pruning through 2.55 (the source read last round), so 2.39, 2.48 and 2.55 all print it. Before 2.23 git neither detects nor prunes duplicates, so an older git fails open on the check and has nothing to act on. `de.po` and `fr.po` translate the line ("Entferne %s/%s: %s"), which is why `CLocale` is load-bearing; `LC_ALL=C` and `LANGUAGE=C` are appended last and Go's exec keeps the last value of a duplicated key. P20 (German `LC_ALL`, `LANG`, `LANGUAGE` in the parent env) and P19 (`GIT_TRACE=1`, which writes trace lines to the merged stream) both refuse the duplicate and clean the healthy repo. I could not test a translated git here (Apple's git ships no `.mo`).

Ids: a git-made id passes `sanitize_refname_component`, which rewrites control characters, space and colon, so the victim's line never splits or carries a fake suffix. A planted id can carry both: with a newline it splits only its own line, and git prints the planted id only when the planted entry sorts after the victim and is the one git drops (P17b: exit 0, T-024 intact); when it sorts first the victim's clean id is printed and clean refuses (P17a). An id ending in ": duplicate entry" is printed with git's reason appended, so the suffix check fails and the entry is a normal prune (P18a). The wrong-direction risk is a false refusal, not a miss. On fail-closed: I would not add a format check. The only version that prints nothing is one that has no duplicate pruning, and git has no format to recognize when there are no duplicates; a dry run that exits non-zero already refuses (`TestCleanRefusesFailingDryRun`).

### 3. Dry run versus real prune

Same function, same `expire` (`TIME_MAX`, no `--expire` on either call), same kept list; `show_only` only skips `delete_git_dir`. A locked entry leaves `should_prune_worktree` before `wtpath` is set, so it is never compared on either run: a locked planted duplicate (P16) is skipped by both, T-024 stays, and the planted entry stays registered as a second list line for T-024's path. A locked planted duplicate of the cleaned ticket itself (P16b) makes `worktree list` name T-023's path twice; the first `remove --force` takes the tree and one entry, the second fails on the lock, clean exits 3 after "removed worktree", and T-024 is untouched. An unlocked one (P16c) is refused by the dry run as a duplicate of T-023's own entry. A planted entry naming the main worktree (P21) and two planted entries naming T-024 (P22) refuse. The one divergence between the two runs is state changed in between, which is B7.

### 4. CLocale scope

`noDuplicates` copies `*c.pg` into `dry` and sets `CLocale` on the copy; `c.pg` keeps `CLocale` false for `list`, `remove` and the real prune, and nothing else in the tree sets the field. On a failed `CLocale` run the error's stderr suffix is empty because stderr went to `out`; the dry run error still reaches the Keeper through `failf`, so cosmetic only.

### 5. Numbers

347 added non-test lines over `c98d130..5ecb9c0` (D-136; stop 350), every file under `internal/dispatch/`. No U+2013 or U+2014 in the range diff or the commit message. Signed off by `paulternate <27632860+paulternate@users.noreply.github.com>`, author and committer the same. `gh pr checks 59` at head `5ecb9c0`: product-ci pass (2m46s), verify-leases-and-tiers fail on the expected "no claim", relay-claim skipping.

### 6. Host suite

`go test -race ./internal/dispatch/... ./cmd/...` on the committed tree: `internal/dispatch` ok 184.0 s, `cmd/waggle` ok 16.9 s (`media/T-023d2/fable-confirm4-host-race.txt`).

### Notes (non-blocking)

- `Worktree.provision` (`worktree.go:219`) runs an unguarded `worktree prune` before provisioning, outside D2's lease; the same `prune_dups` applies there. Worth a D2b line, not a block here.
- The `parent` table mode now passes through the "no duplicate" branch on every filesystem, so its planted entry is never refused; the `t.Logf` makes that visible in CI.

### Evidence files (`media/T-023d2/`)

`fable-confirm4-probe.txt` (script `fable-confirm4-probe.sh.txt`), `fable-confirm4-host-race.txt`.

## Confirmation at f519bf6

Verdict: must-fix
Risk-Tier: 3

Fix round 5, `5ecb9c0..f519bf6` (two commits; `clean.go` 91 lines changed, `worktree.go` -9, `clean_test.go` 158 lines changed; 343 added non-test lines over `c98d130..f519bf6`, stop 350), confirmed independently of the opus guard (D-184) under the D-203 threat bar. Git-only host probes on this Mac (git 2.54.0 Apple), the committed suite on the host, no signal sent, no container, scratch repos inside my worktree and removed after. Nothing of mine is in the branch.

D-202 is implemented as ruled: clean's git calls are `rev-parse --git-common-dir`, `worktree list --porcelain` and `worktree remove --force <path>`; there is no `worktree prune` in `clean.go` and no `--dry-run`, `noDuplicates` or `Git.CLocale` anywhere in the tree (the only prune left is `Worktree.provision`, `worktree.go:210`, which D-202 assigns to D2b). The symlinked-entry, linked, unreadable and oversized gitdir refusals and the two-entries refusal are kept. One ordinary-state gap remains in the new missing-worktree path, and it is the P14 shape the brief asked about.

### Blocking

**B8. `entryFor` does not resolve a relative gitdir, so under `worktree.useRelativePaths=true` a missing worktree's own entry is never removed and clean exits 0 saying nothing (`clean.go:168`; 3 lines).** Git 2.48 and later write `../../../.waggle/worktrees/T-023-builder/.git` into `worktrees/<id>/gitdir` under that config (P-A shows the file), and resolve it against the gitdir file's own directory. `entryFor` cleans the string, strips `/.git` and compares `../../../.waggle/worktrees/T-023-builder` against the absolute `wants`, so nothing matches. With the directory gone the early return fires (`e == ""`, no marker files) and clean prints nothing and exits 0; the entry stays (`media/T-023d2/fable-confirm5-probe.txt`, P-A and P-C: `entries: T-023-builder` after clean). The Keeper's next `git worktree add` at that path then refuses with "is a missing but already registered worktree" (P-A). `waggle dispatch` itself still recovers today because `provision` prunes first; once D2b removes that prune as D-202 directs, a cleaned-then-redispatched ticket under this config fails to provision. The present-tree path is fine (P-A2: `worktree list` prints the resolved absolute path, which `wants` holds). `adminAt` (`worktree.go:332`) already handles exactly this case with the comment `// worktree.useRelativePaths`, so the two readers of the same file disagree. Under D-203 this is ordinary repo state (a repo-local config the pinned env does not hide), not a planted entry, and the effect is a silent miss of what D-202 says clean removes, so must-fix; it deletes nothing outside the ticket. Fix, mirroring `adminAt`:

```go
g := filepath.Clean(strings.TrimSpace(string(b)))
if !filepath.IsAbs(g) {
	g = filepath.Join(root, e.Name(), g)
}
if g, ok := strings.CutSuffix(g, string(filepath.Separator)+".git"); ok && slices.Contains(wants, g) {
```

That is +3 lines, 346 of 350. Test: `TestCleanKeepsOtherTicketsPrunableEntry` already sets `useRelativePaths`; a second pass that removes `wt` before `doClean` and asserts `worktrees/T-023-builder` is gone catches it, and `TestCleanLeavesPlantedEntriesAlone`'s `relative` mode still passes because a planted relative entry that resolves to T-024 only ever names T-024 and is left alone. If the Keeper prefers to carry this to D2b with the provision change (D-204 context: this is the second must-fix on `entryFor`'s matching, though a different cause from rounds 3 and 4), the D2b brief must carry it, since that is where the leak starts to break dispatch.

### 1. No repo-wide action (brief item 1)

Confirmed by reading: every git call in `clean.go` is listed above and each `remove --force` takes one path from `worktree list` that is in `wants`. P-E (T-023 gone, T-024 live, T-025 stale, absolute gitdirs): exit 0, only `T-023-builder` removed, `git worktree list` afterwards still shows T-025 as prunable and T-024 usable.

### 2. `wants` against git 2.54's spellings (brief item 2)

Git writes the realpath of the worktree at `add` time (absolute unless `useRelativePaths`). A repo reached through a symlinked parent matches through `EvalSymlinks(Dir(wt))/Base(wt)` whether the worktree was added through the link or the real path (P-B, P-B2: entry removed, exit 0); `/var` vs `/private/var` is the same mechanism and the suite's `eachRepoSpelling` covers it on both OSes. A relative gitdir never matches (B8). P-D (`--repo` spelled in a different case on this case-insensitive disk, worktree present): `wants` holds the Keeper's spelling, `worktree list` prints git's, nothing matches, and clean exits 0 silently with tree and entry untouched. The CLI defaults `--repo` to `.`, so this needs a hand-typed path; note only.

### 3. `os.RemoveAll(entry)` (brief item 3)

`entry` is `common/worktrees/<name>` from `ReadDir`, a real directory (a symlinked entry hits the `!e.IsDir()` refusal after its gitdir is read, read-only), whose gitdir names `wt` exactly, with two such entries refused before anything is removed (`TestCleanRefusesAmbiguousEntries`, both present and gone). From ordinary inputs the only directory that can satisfy that is the registration of `wt`, under any admin id git chose for it. Planted shapes that could steer it need git-dir write access: notes under D-203, none new.

### 4. Existing worktree (brief item 4)

Only `worktree remove --force <p>` for list lines whose cleaned path is in `wants`; a `remove` failure returns exit 3 after any "removed worktree" line. A planted second entry naming the same path (one relative, so `entryFor` sees one) would make the loop call `remove` twice and fail on the second: note, git-dir write needed.

### 5. Behavior change (brief item 5)

`provision` still prunes before `worktree add` (`worktree.go:210`), so a stale entry clean now leaves (another ticket's, or B8's) does not block the next dispatch. `check.go:102` records every entry in the host-files baseline at run start, so entries clean leaves are not violations. `adminAt` matches on the resolved real path, so other tickets' stale entries cannot be mistaken for the new worktree's registration. Nothing else in Run reads `worktrees/`.

### 6. Tests and the fix-removal claim (brief item 6)

(a) `TestCleanRefusesAmbiguousEntries`: exit 3, no "removed", both entries kept, tree present or gone as planted. (b) `TestCleanLeavesPlantedEntriesAlone`: six planted spellings of T-024's entry (copy, case, .GIT, upper, symlinked parent, relative through a link), clean exits 0, T-023 gone, planted entry and T-024's entry and tree kept, T-024 `git status` works; the earlier case-sensitive-fs skip is rightly gone since clean no longer compares. (c) `TestCleanMissingWorktreeRemovesOwnEntry` under `eachRepoSpelling`: own entry gone, T-024 (live) and T-025 (prunable) entries kept. (d) `TestCleanKeepsOtherTicketsPrunableEntry`: `useRelativePaths`, present tree, T-025's prunable entry kept; it does not exercise the gone case (B8). (e) `TestCleanRefusesLinkedGitdir`: exit 3, tree kept. `TestCleanRefusesDuplicateEntries`, `TestCleanRefusesForeignDuplicateAndLinkedGitdir`, `TestCleanHealthyRepoWithRelativeAndPrunableEntries` and `TestCleanRefusesFailingDryRun` are removed with the code they tested; `os/exec` import dropped. Fix removal confirmed: no `noDuplicates`, `CLocale`, dry run or C-locale env in `internal/` or `cmd/` outside `waggle decide --dry-run`, which is unrelated.

### 7. Numbers (brief item 7)

343 added non-test lines over `c98d130..f519bf6` (stop 350), all under `internal/dispatch/`. No U+2013 or U+2014 in the range diff or either commit body. Both commits signed off by `paulternate <27632860+paulternate@users.noreply.github.com>`. `gh pr checks 59` at `f519bf6`: product-ci pass (2m30s), verify-leases-and-tiers fail on the expected "no claim for T-023; claims must stay until merge", relay-claim skipping.

### 8. Host suite (brief item 8)

`go test -race ./internal/dispatch/... ./cmd/...` on the committed tree at `f519bf6`: `internal/dispatch` ok 205.7 s, `cmd/waggle` ok 18.9 s, exit 0 (`media/T-023d2/fable-confirm5-host-race.txt`).

### Notes (non-blocking)

- P-D above: a present worktree that no list line matches ends in a silent exit 0 with the tree left; a one-line "no registered worktree matches <rel>" would make a mis-spelled `--repo` visible. Keeper input only.
- Planted-entry shapes (B2 to B7 lineage) are closed by design now, not by matching: every one leaves the planted entry in place and T-024 usable (test (b)). None re-raised.
- `Worktree.provision`'s prune (`worktree.go:210`) remains the one repo-wide prune; D-202 assigns it to D2b, and B8's fix is a precondition for removing it.

### Evidence files (`media/T-023d2/`)

`fable-confirm5-probe.txt` (script `fable-confirm5-probe.sh.txt`), `fable-confirm5-host-race.txt`.

## Confirmation at bfdb82f

Verdict: pass
Risk-Tier: 3

Fix round 6, `f519bf6..bfdb82f` (one commit; `clean.go` +5/-1, `clean_test.go` +17; 347 added non-test lines over `c98d130..bfdb82f`, stop 350), confirmed independently of the opus guard (D-184) under the D-203 threat bar. Git-only host probes on this Mac (git 2.54.0 Apple), the committed suite on the host, no signal sent, no container, scratch repos inside my worktree. Nothing of mine is in the branch.

### 1. B8 closed (P-A, P-C)

`entryFor` now joins a relative gitdir to its own admin dir before cleaning and matching, the same shape as `adminAt` (`worktree.go:332`). Reran the B8 probes with T-024 live and T-025 prunable alongside, all three with relative gitdirs: P-A (plain parent), P-C (symlinked parent, added and cleaned via the link), P-C2 (added via the real path, cleaned via the link) and P-C3 (added via the link, cleaned via the real path) each print `removed worktree .waggle/worktrees/T-023-builder`, exit 0, leave `entries: T-024-builder T-025-builder`, and a plain `git worktree add` at the same path afterwards exits 0 without a prune; T-024 `git status` works and `worktree list` still shows T-025 as prunable. P-G (tree present under `useRelativePaths`) still goes through the `worktree list` path and removes tree and entry.

### 2. Ordinary state cannot match a different entry

`c.common` comes from `rev-parse --path-format=absolute --git-common-dir`, which returns the real path even when `.git` is itself a symlink to a store outside the repo (P-H: common dir printed as the store, gitdir file `../../../H/.waggle/worktrees/T-023-builder/.git`, clean removed only T-023's entry, re-add exit 0). Git computes the relative gitdir from the real admin dir to the real worktree, so the lexical `Join` lands on the same base git used and `Clean` only folds the `..` segments git wrote; another entry's relative gitdir resolves to that entry's own worktree (P-F: T-023 never added, clean 23 exits 0 and both other entries stay). Lexical join and git's realpath diverge only when a symlink sits inside the `.waggle/worktrees/<name>` path or inside the joined `..` chain, which under D-203 is a planted entry or a planted worktrees dir, not ordinary state: note only, and the planted-entry suite (`TestCleanLeavesPlantedEntriesAlone`, `relative` mode) still passes.

### 3. Tests and fix removal

`TestCleanDropsMissingWorktreeEntryWithRelativeGitdir` passes at `bfdb82f` and fails with the three `IsAbs`/`Join` lines removed (`clean_test.go:264: out "" err <nil>`, the exact B8 symptom), while `TestCleanKeepsOtherTicketsPrunableEntry` and `TestCleanLeavesPlantedEntriesAlone` still pass without the fix, so the new test is the one that pins it. All 13 `TestClean*` tests pass on the host (`fable-confirm6-clean-tests.txt`).

### 4. Numbers

347 added non-test lines over `c98d130..bfdb82f` (stop 350). No U+2013 or U+2014 in the diff or commit body. Signed-off-by `paulternate <27632860+paulternate@users.noreply.github.com>`. `gh pr checks 59` at head `bfdb82f`: product-ci pass (2m33s, run head sha `bfdb82f`), verify-leases-and-tiers fail on the expected "no claim for T-023; claims must stay until merge", relay-claim skipping.

### 5. Host suite

`go test -race ./internal/dispatch/... ./cmd/...` at `bfdb82f`: `internal/dispatch` ok 171.3 s, `cmd/waggle` ok 17.0 s, exit 0.

### Notes (non-blocking)

- Symlink inside the worktree path (item 2 above): lexical `Join` vs git's realpath, planted only.
- P-D from the f519bf6 round (mis-cased `--repo`, silent exit 0) is unchanged and remains Keeper input only.
- `Worktree.provision`'s prune (`worktree.go:210`) is still the one repo-wide prune; with B8 closed, D2b can remove it as D-202 directs.

### Evidence files (`media/T-023d2/`)

`fable-confirm6-probe.txt` (script `fable-confirm6-probe.sh.txt`), `fable-confirm6-clean-tests.txt`, `fable-confirm6-fix-removal.txt`, `fable-confirm6-host-race.txt`.
