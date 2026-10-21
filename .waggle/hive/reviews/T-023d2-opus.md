<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: T-023 phase D2 (PR #59, `t-023d2` at `2f1352d`, base `c98d130`), opus guard

Verdict: must-fix
Risk-Tier: 3

## Intent Summary

Phase D2 changes which processes dispatch signals and adds a host git write path. The sweep now kills, besides each tracked live pid, the process group of every tracked process the tracker saw lead its own group (`leaders`), through a pure `targets(tab)` with the D-194 guards, and `sweep` is a loop over two seams (`read`, `kill`) with a 3-retry bound on failed reads. The worker starts with `Setsid` instead of `Setpgid`. `newTracker` takes its first table read itself, right after `cmd.Start` and before the `Wait` goroutine, sets `rootStart` once and marks the run blind when the root was never listed. New `waggle dispatch clean <ticket> [--role] [--runs] [--repo]` removes a failed or tainted run's worktree, registration, markers and run lock under `dispatch.lock`, refusing duplicate admin entries. The macOS `NotEnforced` clause is reworded. Tests: data tests for `targets` and `observe`, the direct `sweep()` callers moved to `sweep_linux_test.go` (D-195), helpers that exit on their own after 60 s, a renewal-after-end test, solaris and illumos vet. Host-record acknowledge (item 6) is split to D2b per D-196. Blast radius: every dispatched session's cleanup on the Keeper's Mac, which is the code path behind the 2026-10-20 incident, plus a new command that runs `git worktree prune` on the main repository. 314 added non-test lines (stop 350). Three items block: a stale leader record lets the sweep SIGKILL a foreign process group after pid reuse, `clean`'s duplicate refusal misses the cases where `git worktree prune` silently chooses, and the new macOS sentence overstates what the group sweep catches.

## Blocking Issues (Must-Fix Only)

### B1. A recorded leader whose group died can name a foreign group later (item 1; about 5 lines)

`leaders` keeps every leader identity for the whole run (up to 120 minutes). `targets` accepts group G when the leader pid G is absent from the table and some live process has pgid G. The kernel never reuses pid G while group G exists, so "leader absent, member present" proves identity only while the group has never been empty. Once group G empties (its leader and children exit, the usual end of a tool shell), pid G is free. If a foreign process later gets pid G, starts its own group (setsid, or an interactive job) and exits while a member stays (a double-fork daemon; `cat f | less` at the Keeper's prompt), the next sweep sees "leader absent, member present" and SIGKILLs `-G`, a group of the Keeper's that the worker never touched. This is the "leader exits and its pid is reused as a new group leader" path the brief asks about, and it is new in D2: D1 signalled only the group of a live tracked process.

Evidence (`media/T-023d2/opus-container-probes-head.txt`, data only, container): P1 learns `targetTable`, observes a table where group 510 has no member, then a table with foreign member 9000 (ppid 1, pgid 510). `targets` returns groups `[501 510]`. Odds per run are small (needs a pid wrap during the run onto one of the run's recorded leader pids), but the outcome is a SIGKILL outside the worker's tree, which is what D-193 and D-194 exist to prevent.

Fix: in `observe`, after a successful read that lists `self`, delete from `leaders` every identity whose pid is no process's pgid in that table (zombies count as members, since they hold the id). A group seen empty is gone for good; any later group with that id is foreign. The window left is a full die, reuse and re-form inside one 25 ms poll. Candidate (`opus-container-probes-fix.txt`):

```go
for k := range t.leaders { // a group a read lists empty is gone: its id may return as a foreign group
	if !slices.ContainsFunc(tab, func(p proc) bool { return p.pgid == k.pid }) {
		delete(t.leaders, k)
	}
}
```

placed where `tab` is known good and lists `self` (the scratch copy used `tab != nil`; use the self check). With it, P1 targets `[501]` only, and every `targets`, `observe` and `sweep` test plus `TestToolShellChildKilled` and `TestSurvivorKilledAndRecorded` pass in the container. The `cmd &` catch is unaffected: the orphan keeps group G non-empty. Test: P1 as a case in `targets_test.go`.

### B2. `clean`'s duplicate refusal misses two cases where `git worktree prune` chooses silently (item 5; about 6 lines)

Item 5: "read every `<common>/worktrees/<id>/gitdir` ... and if two entries name the same path ... exit 3 ... with nothing removed", and the scout default: `clean` "never chooses which one to delete, since the pinned admin dir of a tainted worktree is itself suspect". `noDuplicates` (`clean.go:139`) counts only entries naming the worktree being cleaned, and skips an entry whose `gitdir` `readSmall` refuses. Git does neither: `worktree prune` is repository-wide, and git follows a symlinked `gitdir`.

- **Another worktree's duplicate.** Probe (`opus-clean-duplicate-other-ticket.txt`, host, no signals): T-023 and T-024 worktrees, a planted `worktrees/0` copied from `worktrees/T-024-builder`. `waggle dispatch clean 23` exits 0; afterwards only `worktrees/0` remains and T-024's worktree is broken (its `.git` names the pruned entry). Prune kept the planted entry and dropped the real one.
- **A symlinked `gitdir`.** Probe (`opus-clean-duplicate-symlinked-gitdir.txt`): `worktrees/0/gitdir` is a symlink to `worktrees/T-023-builder/gitdir`. The refusal does not fire, `worktree list` shows the path twice, the first `remove --force` deletes the worktree and the real `T-023-builder` entry, the second fails, exit 3. The planted entry survives and something was removed. With the symlink pointing at T-024's `gitdir`, exit 0 and T-024 is broken as above.

Fix: build a map from cleaned path to ids over every entry, and refuse (exit 3, nothing removed) on any path with two ids, and on any entry that is a directory whose `gitdir` is not a readable regular file within the bound (git would follow or misread it). Optionally also refuse when `worktree list` names a wanted path twice. Tests: both probes as `clean_test.go` cases.

### B3. The macOS `NotEnforced` sentence says more than the data shows (item 2; 1 line, Keeper card)

The new text says a background child escapes only if it "calls setsid or daemonizes before the next read", and that "one that stays in its tool shell's group is killed by the group sweep". The group sweep can kill group G only if a read listed the tool shell alive, since that is when G enters `leaders`. The builder's own darwin measurement shows the gap: 0 of 20 when the shell exits at once, 20 of 20 when it lives 100 ms. A `cmd &` from a tool shell that exits within one 25 ms read escapes exactly as before, and Claude Code's real tool-shell lifetime is not measured until the phase F trial (D-197). The sentence is honest about the setsid case and not about the short-shell case. It also departs from the D-194 sentence, which the Keeper ruled, so the change needs a card either way.

Wording I would put on the Keeper's card (replacing the builder's macOS clause, the rest of the item unchanged):

> on macOS (sysctl read every 25 ms) a background child escapes until T-024 when no read lists its tool shell alive (a shell that exits within about 25 ms of starting), or when the child calls setsid or daemonizes before a read lists it; a child left in the group of a tool shell that a read listed is killed by the group sweep

with the card stating the evidence: 0/20 with an immediate exit, 20/20 with a 100 ms shell (builder, data only, darwin), and the D-197 trial as the first measurement on real Claude Code. The phase F trial report carries the same sentence (D-194).

## Rulings on the builder's deviations

1. **Own group from the table row for `self`, not `Getpgid(0)`:** accepted. `go doc` with `GOOS=solaris` and `GOOS=illumos` finds neither `syscall.Getpgid` nor `syscall.Getpgrp`. The value is refreshed from each table, `targets` and `observe` use the same table in `sweep`, and a table that does not list `self` yields nothing. On linux and darwin a process's own group does not change during the run, so per-table is at least as good as a single read.
2. **Zombie-only groups not targeted:** accepted. The spec's "lists at least one process whose pgid is G" counts a zombie; excluding zombies only narrows what is signalled, and a group of zombies has nothing to kill. For B1, zombies must still count as members when deciding a group is gone.
3. **`TestSweepKillsTrackedGroup` "moved unchanged":** it gained `tr.leaders = ...`, needed because groups are now signalled only from `leaders`. Accepted.
4. **`fifo_unix_test.go` build tag (`unix && !solaris`) outside the brief's file list:** accepted. It is inside the `internal/dispatch/` directory lease, test-only, and needed because solaris and illumos (which matches the `solaris` tag) lack `syscall.Mkfifo`, so test 27's new vet would fail. The queen should note it in the merge record.
5. **NotEnforced macOS clause:** see B3. The builder followed the 10.7.3 item 2 text; the text itself is what is wrong.

## Review questions

1. **Group sweep and seams.** Every D-194 guard is present and pinned by a data case (`TestTargetsOnFabricatedTables`, `TestTargetsRefuseInitSelfAndKernel`) and by the builder's container fix-removal runs 00 to 09. Paths checked for a signal outside the tree: pgid reuse while the group lives is impossible (the kernel holds the id); a leader pid reused while the leader is listed fails the start check; a missing `self` row yields nothing; a stale own group cannot occur within one table; `setsid` by a tracked process cannot reach the Keeper's session (Setsid, below). The open path is B1. The seams default to `procTable` and `syscall.Kill` in `newTracker`; an inert tracker (root 0 or 1) has nil seams, so `sweep` on it would panic rather than signal. `runSession` never builds one (the root is `cmd.Process.Pid`), and a panic is the safe failure here; optional: give the inert tracker the defaults or an early return in `sweep`.
2. **Setsid.** `kill(-pid)` is unchanged (the worker leads a group whose id is its pid). Stdin is a pipe from `bytes.NewReader`, never a tty, so headless Claude Code reads its prompt as before; a tool opening `/dev/tty` now fails at once instead of stopping on SIGTTIN. Ctrl-C reaches only the terminal's foreground group, dispatch, which cancels the context, and `stop` sends SIGTERM then SIGKILL to the worker's group: the same as with Setpgid, since the worker's group was never the foreground group. `TestWorkerLeadsOwnSession` asserts `getsid` of a worker child equals the worker's pid and differs from the test's session, and that `setpgid` into the test's group is EPERM. With Setpgid the child's session would be the test's and the test fails (builder mutant 10, container). It runs on darwin in the host suite and passed; `SYS_GETSID` exists there. It proves the property on both.
3. **Item 3.** `newTracker` reads before the `Wait` goroutine, `rootStart` is set once (`TestObserveRootStartSetOnce`), an unknown root sets `blind`, and the read retry stops after 4 failed reads with no signal (`TestSweepReadRetry`). AC3 holds structurally: a run either listed the root or warns. But `TestFastExitNeverBlind` does not detect the realistic regression: my container mutant puts `newTracker` back at its D1 position (after the `Wait` and renewal goroutines) and the test passes 20 of 20 (`opus-mutant-item3-d1-position.txt`). The builder's table says mutant 11 is caught; it must have moved the read further. Note N1.
4. **`dispatch clean`.** Lock order matches `Run` (`dispatch.lock`, then the role lock), and `Run` holds `dispatch.lock` for its whole life, so `clean` cannot overlap a live run in the same repo; a held lock is exit 3, a dead pid is taken over with a warning. Symlinked or non-directory worktree and a symlinked `runs` are exit 3. Every git call is pinned (`TestCleanGitCallsArePinned`, trace2 log empty). No `branch -D`, `update-ref` or `host-files.json` write exists in `clean.go`, and the tests check branch and claim unchanged. Exit codes: usage 2, environment 3, verified in `TestDispatchCleanCLI` and by reading. TOCTOU: a process that escaped the sweep could swap the worktree for a symlink between `requireRealDir` and `worktree remove`; git validates the `.git` back-pointer, and a same-user escapee is already named in `NotEnforced`. Accepted. The duplicate refusal is B2.
5. **NotEnforced wording.** B3.
6. **Tests.** No non-linux test file calls `sweep()` (`grep` finds it only in `sweep_linux_test.go`). No committed test sends a real signal to a pid read from a file: the remaining file-read pids are only probed with `kill(pid, 0)` (`gone`, `TestRunLimitsKillTheGroup`), and the helpers exit on their own within 60 s; `pidFrom` refuses pid 1 or less. Probabilistic tests: `TestFastExitNeverBlind` is weak (N1); `TestNoRenewalStartsAfterSessionEnds` caught the removal twice and passed unmutated 3 times in the builder's container runs, and passed in my host run; N6 names its flake mode. Test 27 vets windows, solaris and illumos; the builder's cross-build file shows all six targets.
7. **Size.** `git diff --numstat c98d130..2f1352d` over non-test files: 34 + 2 + 3 + 179 + 91 + 5 = **314** (target 200, review threshold 280, stop 350). Past 280, so the item 6 split to D2b follows D-185 and D-196, and `prerun.go` is untouched. B1 and B2 add about 11 lines (about 325).
8. **CI.** `gh pr checks 59` at `2f1352d`: product-ci pass (3m0s); verify-leases-and-tiers fails with "leases: no claim for T-023; claims must stay until merge", expected for this round; relay-claim skipped.

## Checks

- `go build ./...`, `go vet ./...`, `gofmt -l cmd internal`: clean (host).
- `go test -race -count=1 ./...` on the host (committed suite, allowed by D-195): every package ok, `internal/dispatch` 180 s (`opus-host-test-race.txt`).
- Container runs (Colima, `--cpus 1 --memory 700m -e GOGC=50`, `-test.run` only, the shared lock taken before and released after each run, three runs total): probes at head, probes and tracker tests on the fix copy, the item 3 mutant.
- `clean` probes ran on the host: they start no process that is signalled and call no tracker code.

## Notes (non-blocking)

- **N1.** `TestFastExitNeverBlind` cannot tell the D2 order from the D1 order. Since AC3 holds through `blind`, this costs only spurious `survivors_unchecked` warnings if the order regresses. A deterministic guard would need a seam between `Start` and the `Wait` goroutine; not worth lines now. The fix-removal table should describe mutant 11 exactly.
- **N2.** A tracked process first seen inside another group that later calls `setsid` is never recorded as a leader, so an orphan left in its new group is not targeted (P2 in `opus-container-probes-head.txt`; darwin only, linux adopts it). This is the `setsid` case the `NotEnforced` text names. Optional fix: record a leader for any live tracked process with pgid equal to pid on every poll, not only when first seen.
- **N3.** SIGHUP is not in `signal.NotifyContext` (`cmd/waggle/dispatch.go:81`). Closing the Keeper's terminal kills dispatch with no stop, no sweep and no checks, and the worker (own session) runs on. Unchanged by Setsid (with Setpgid the worker was not a shell job either). One line for a later round.
- **N4.** `clean` prints "removed worktree" before the `remove` result, so a failed remove reads as removed then an error (visible in builder mutant 17 and in my symlink probe).
- **N5.** The tainted-marker print strips `unicode.IsControl` (Cc) only; bidi format characters (Cf, U+202E) pass. Low.
- **N6.** `TestNoRenewalStartsAfterSessionEnds` infers the decision time from the renewal deadline. A goroutine preempted for more than 20 ms between the `tctx.Err()` check and `WithTimeout` reads as a renewal after the end. Rare; watch CI.
- **N7.** `clean --repo <not a repo>` creates `.waggle/worktrees/` there before failing exit 3 (`worktreesDir` runs `MkdirAll`).

## Confirmation at 9edaf84

Range `2f1352d..9edaf84` (b9611f2, 9edaf84), checked out as `origin/t-023d2`.

Verdict: must-fix
Risk-Tier: 3

B1 and B3 are closed, N1, N4 and N7 are fixed, and the inert-tracker seams are safe. B2 is closed for the two probes it named, but the same silent prune survives one spelling: a planted entry that names another ticket's worktree in a different letter case. On macOS (case-insensitive APFS, `core.ignorecase=true` by default) git counts that as a duplicate and `waggle dispatch clean 23` exits 0 with T-024's worktree broken. That is B4, one line.

### Blocking

**B4. The duplicate refusal compares paths case-sensitively; git prunes them case-insensitively on macOS (item 2; 1 line plus a test case).** `noDuplicates` keys `byPath` on the cleaned, `EvalSymlinks` path. `EvalSymlinks` does not canonicalize letter case, so `.../worktrees/t-024-builder` and `.../worktrees/T-024-builder` are two keys. Git's `prune_dups` compares with `fspathcmp`, which is `strcasecmp` under `core.ignorecase`, and keeps the lowest id. Probe P-c (`media/T-023d2/opus-confirm-clean-probes.txt`, host, git only): `worktrees/0` with `gitdir` naming `<repo>/.waggle/worktrees/t-024-builder/.git`. `clean 23` prints "removed worktree .waggle/worktrees/T-023-builder", exits 0, only `worktrees/0` is left and T-024's worktree is broken. This is the B2 outcome exactly. Fix: key the map on `strings.ToLower(p)`. On a case-sensitive filesystem the only cost is a refusal (exit 3, nothing removed) for two worktrees whose paths differ only in case, which is the safe direction. Keying on `ToLower` only when `git config --bool core.ignorecase` is true matches git exactly, but costs a git call. Test: P-c as a third case in `TestCleanRefusesForeignDuplicateAndLinkedGitdir` (with the `ToLower` key it refuses on linux as well, so it runs on both).

### 1. B1: leaders dropped once a read lists their group empty

**Closed.** The drop runs after the walk, only when the table lists `self`. A leader recorded from this table has `pgid == pid`, so it is never dropped in the same pass. Zombies count as members, because the check is `p.pgid == k.pid` with no zombie filter. `TestTargetsDropGroupSeenEmpty` is my P1, and the builder's container mutant without the drop fails it (`builder-fix1-container-mutant-b1.txt`). I also removed only the self check (my mutant M1, `opus-confirm-container.txt`): the committed `TestSweepReadRetry` fails ("a read that recovers: kills [501 502 510 511]"), and so does my probe `TestOpusFailedReadKeepsLeaders`. So the guard that keeps a failed or empty read from emptying `leaders` is pinned.

This also closes fable's N2. One side note on fable's N2 text: on linux a reused pid cannot join a populated group G with `setpgid(0,0)`. The kernel frees `struct pid` G only when no task holds G as a PID, PGID or SID, so pid G cannot be handed out while group G exists. Both systems therefore reach a foreign group G only through "group emptied, pid G reused". The drop shuts that path except when the group dies, its pid is reused and the group re-forms all inside one 25 ms poll.

New paths I checked:
- **A read that lists self but is partial.** This is the only way the drop can lose a live worker group.
  - **Linux.** `procTable` reads the whole `/proc` directory and then each `stat`, so a child C forked after the `ReadDir` is missed, and its parent P can be reaped before P's `stat` is read. That table lists self and no member of G, so G is dropped. The cost is nil on linux: C reparents to dispatch (the subreaper), and at the sweep `kids[self]` holds, C is younger than the root, and C is killed by pid.
  - **Darwin.** `kern.proc.all` snapshots both the live and zombie lists under the proc list lock, and a process that turns into a zombie during the walk is still reported. My read-only probe (`opus-confirm-darwin-read.txt`: no sweep, no signal, only its own `sh` children, the uncommitted file deleted afterwards) ran 600 trials of a group leader that forks its last child and exits at once, with reads every 25 ms and back to back. Zero reads that listed self showed the group empty before the child appeared. The remaining window is a child still in `SIDL` at snapshot time whose parent is reaped before its own row is copied. That is a sub-millisecond window on a process that was already listed, and I did not hit it once. Non-blocking (N8).
- **A dropped leader whose group comes back legitimately.** This needs a new process with pid G (reuse). A tracked one that leads its group when first seen is recorded again under its new start time. One that calls `setpgid` later is fable's N3 shape, which 2f1352d also missed: there the old record failed `same` while the reused pid was listed.
- **Does the drop let a real group escape that D1 or 2f1352d caught?** Only through a partial read, as above. Linux loses nothing because of the subreaper. On darwin the escape needs the SIDL window. D1 never signalled such a group anyway.

### 2. B2: repository-wide duplicates, EvalSymlinks, non-regular gitdir

Re-ran both of my probes at 9edaf84 on the host (git only, `opus-confirm-clean-probes.txt`):
- **P-a (another ticket's duplicate).** Exit 3, "worktrees/0 and worktrees/T-024-builder both name ...", nothing removed, T-024 usable.
- **P-b1 and P-b2 (symlinked `gitdir` to T-023's or T-024's).** Exit 3, "has no readable gitdir", nothing removed.
- **P-d (new: T-024's path through a symlinked parent).** Exit 3. `EvalSymlinks` works.
- **P-c (new: letter-case variant).** Exit 0, T-024 broken. This is B4.

Healthy repos:
- **H-1, prunable entry for a worktree deleted by hand.** Exit 0. The entry is pruned, as git would.
- **H-2, `worktree add -f` over a missing registered path.** Git replaces the old entry, so no duplicate, and exit 0.
- **H-5, a locked T-024 worktree.** Exit 0. T-024 is untouched.
- **Entries of other tickets.** Distinct paths, never refused.

Two refusals of repos that git would prune quietly, both safe in direction (N9):
- **H-3, a stray `.git/worktrees/.DS_Store` file.** Exit 3.
- **H-4, an admin dir with no `gitdir` (an interrupted add).** Exit 3.

### 3. Inert tracker seams

**Accepted, and better than nil.** For `root <= 1` the tracker has `self 0`, `group 0`, nil `seen` and nil `leaders`. In `sweep()`, `observe` sees `t.root <= 1`, replaces the table with nil and sets `blind`. The walk, the reap loop and the drop then iterate nothing, and `live` is empty. `targets` gets the sweep's own real table, not observe's nil.

One correction to the brief's reasoning: on darwin that table includes pid 0 (`kernel_task`), so the self guard does pass for `self 0` there. Nothing is signalled anyway, because `live` is empty, `leaders` is nil, and `t.group > 1` would reject every group regardless. The round breaks on empty targets. Container probe `TestOpusInertSweepSignalsNothing` (kill is a recorder): roots 0, 1 and -5 give 0 kill calls, nothing killed, blind true.

A nil seam would panic in the sweep goroutine. That crashes dispatch with no checks, no ledger and the locks left behind, which is a worse failure than a no-op flagged with `survivors_unchecked`.

### 4. N1 pin

**Acceptable, non-blocking.** `TestTrackerStartsBeforeWait` fails on the realistic regression (builder's host mutant: newTracker at 19346, first `cmd.Wait` at 19285). `run.go` has one `newTracker(cmd.Process.Pid)` and one `cmd.Wait()`. Both ways it can go wrong are acceptable:
- A false failure, such as a comment that mentions `cmd.Wait()` earlier in the file, is the safe direction.
- A false pass needs a deliberate rewrite of the call shape, and review would see that.

AC3 still holds through `blind` either way.

### 5. N4 and N7

**Both fixed.**
- **N4.** "removed worktree" now prints only when `git worktree remove` succeeds.
- **N7.** `rev-parse --git-common-dir` runs before `worktreesDir`, and `TestCleanOutsideRepoCreatesNothing` pins it. The builder's fix-removal run fails both new tests.

A `--repo` that is a non-repo directory inside some other repository still gains `.waggle/worktrees`, because `rev-parse` finds the parent repo. That case is the same in `Run` and not worth lines.

### 6. D-199 text

**Exact.** The clause quoted in D-199's Keeper choice (422 characters) appears verbatim on `cell.go:50`, between the unchanged head ("... only the worker's own group is killed, ") and the unchanged tail (": its hook and config writes ... are not"). Checked by substring match (`opus-confirm-checks.txt`).

No other file outside `.git`, reviews and media makes the old claim. The remaining hits are D-194's entry in `DECISIONS.md`, which is append-only history.

### 7. Size and CI

- **Size.** `git diff --numstat c98d130..9edaf84` over non-test files outside `.waggle`: 34 + 2 + 3 + 186 + 101 + 5 = **331** (stop 350). B4 adds 0 lines (one expression changes).
- **CI.** `gh pr checks 59` at 9edaf84: product-ci pass (2m18s). verify-leases-and-tiers fails with "leases: no claim for T-023; claims must stay until merge", which is expected. relay-claim skipped.

### 8. Host suite

`go build ./...`, `go vet ./...` and `gofmt -l cmd internal` are clean. `go test -race -count=1 ./...` passes every package on the host, with `internal/dispatch` at 186.7 s (`opus-confirm-host-test-race.txt`).

Container (one run under the shared lock, `-test.run` only): the head binary passes every `TestTargets*`, `TestObserve*` and `TestSweep*` test plus `TestToolShellChildKilled` and `TestSweepWaitsForRenewal`. `TestSurvivorKilledAndRecorded` failed only because I named the binary `head.test` and the test wants "dispatch" in the survivor's command name. The pid matched. This is an artifact of my run.

### New notes (non-blocking)

- **N8.** The B1 drop trusts a read that lists `self`. A darwin `SIDL` child whose parent is reaped before its row is copied could be missed by one read. 0 of 600 in my probe. Name it alongside the D-199 trial if the Keeper wants it recorded.
- **N9.** `noDuplicates` refuses a non-directory entry (`.DS_Store`) and an entry with no `gitdir` (ENOENT). Git prunes both as "not a valid directory" and "gitdir file does not exist", and neither can name a path, so neither can be a duplicate. Skipping both (and still refusing a `gitdir` that is a link or unreadable for any other reason) would end two false refusals in about 2 lines. Safe as it stands, because the message names the entry.


## Confirmation at 2d1a1b3

Range `9edaf84..2d1a1b3` (3 files, 44 insertions, 5 deletions), checked out as `t-023d2-opus3`. Host only, git-only probes, no process signalled, no container run (D-193/D-195).

Verdict: must-fix
Risk-Tier: 3

N9 is fixed cleanly and the skip cannot be abused. N13 is fine. B4 is closed for the letter case of the directories, but the same silent prune survives one more spelling: the `.git` suffix in another case. `noDuplicates` trims `/.git` before it lowers the key, so a gitdir ending in `/.GIT` keeps its suffix and never matches. That is B5, one line.

### Blocking

**B5. The `/.git` suffix is trimmed case-sensitively, so a planted gitdir ending in `/.GIT` or `/.Git` escapes the B4 fold (item 1; 1 line plus a test case).** `p := strings.TrimSuffix(filepath.Clean(g), string(filepath.Separator)+".git")` runs before `strings.ToLower`. For `.../worktrees/T-024-builder/.GIT` the trim does nothing, `EvalSymlinks` keeps the spelling as given, and the key is `.../t-024-builder/.git`, while the real entry's key is `.../t-024-builder`. Git compares the raw gitdir strings (suffix included) with `fspathcmp`, which is `strcasecmp` under `core.ignorecase`, and `file_exists` on APFS accepts `.GIT`. So git sees a duplicate and prunes it. Probes (`media/T-023d2/opus-confirm2-clean-probes.txt`): P-c3 (`/.GIT` only), P-c5 (`/.Git`) and P-c2 (whole path upper-cased) all print "removed worktree .waggle/worktrees/T-023-builder", exit 0, leave only `worktrees/0`, and T-024's worktree is broken. P-c4 (directories upper-cased, `.git` kept) refuses, which isolates the cause to the suffix. Fix: trim the suffix case-insensitively, for example
`p := filepath.Clean(g); if strings.EqualFold(filepath.Base(p), ".git") { p = filepath.Dir(p) }`.
On a case-sensitive filesystem this can only add a refusal, the safe direction. Test: a fourth mode in `TestCleanRefusesForeignDuplicateAndLinkedGitdir` that rewrites the suffix to `/.GIT`; with the fold it refuses on linux too.

### 1. B4: case fold

The map is keyed on `strings.ToLower(p)` at both the lookup and the store. My P-c rerun exits 3 with "worktrees/0 and worktrees/T-024-builder both name ...", all three admin entries kept, both worktrees intact. The new `case` mode in the committed test pins it. `ToLower` is a Unicode superset of git's ASCII `strcasecmp`, so any extra match only refuses. Still open through B5.

### 2. N9: skipped stray entries

The skip is `e.Type().IsRegular() || e.IsDir() && errors.Is(err, fs.ErrNotExist)`. Both type checks come from `ReadDir`, which uses the entry's own type and never follows a link. `readSmall` runs `Lstat` on `gitdir` first, so ENOENT means that no `gitdir` name exists at all in a real directory. A link (dangling or not), a directory, an unreadable file, an oversized file, or an unsearchable entry dir each give a different error and still refuse. I found no way to use the skip. A skipped entry names no path, so git cannot count it as a duplicate of anything. Git removes it ("not a valid directory" or "gitdir file does not exist"), or keeps it when it holds `locked`. Neither outcome touches another entry. H-4b (a gitdir-less entry carrying T-024's `HEAD` and `commondir`) shows this: exit 0, the stray is pruned, T-024 is intact. The remaining hole is a race where the entry is swapped for a link between `ReadDir` and `Lstat`. It needs a writer racing clean inside `.git`, and even then the skip only fires on ENOENT, so nothing names a path.

Probes: H-3 (`.DS_Store`), H-4 and H-4b exit 0, only T-023 removed. S-1, S-1b and S-1c (symlinked entry to the real entry, to a file, dangling), S-2 (dangling `gitdir` link), S-3 (`gitdir` mode 000), S-4 (`gitdir` over 4096 bytes), S-5 (entry dir mode 000), S-6 (`gitdir` is a directory) and S-7 (a FIFO entry) all exit 3 with nothing removed. The committed `TestCleanSkipsStrayAdminEntries` pins the two skips and the symlinked entry. Fable's dangling-link and unreadable cases cover the rest.

### 3. N13: `TestInertTrackerSweepSignalsNothing`

Fine. It is in the linux-only file. For roots 0 and 1, `newTracker` returns the inert tracker with the real `procTable` read and a recording kill seam, and the test asserts zero kill calls, no survivors and `blind`. The name now matches what the test checks. `GOOS=linux go vet ./internal/dispatch/` is clean. I did not run it here, because the test is linux-only and no container run was asked for. The builder's `builder-fix2-container.txt` in the main checkout lists it passing.

### 4. Checks

- **Size.** `git diff --shortstat c98d130..2d1a1b3 -- . ':!*_test.go'` gives 334 insertions (stop 350). The B5 fix adds 0 or 1 lines.
- **Dashes.** No em or en dash in the added lines of `c98d130..2d1a1b3`.
- **Sign-off.** `Signed-off-by: paulternate <27632860+paulternate@users.noreply.github.com>`, and the author is the same address.
- **CI.** `gh pr checks 59`: product-ci pass (2m26s). verify-leases-and-tiers fails with "no claim", which is expected until phase F. relay-claim skipped.

### 5. Host suite

`go test -race ./internal/dispatch/... ./cmd/...` passes: `internal/dispatch` 185.5 s, `cmd/waggle` 17.1 s. Run alone, the two changed clean tests pass.

### Notes (non-blocking)

- **N10.** A probe that shares the scratchpad with the fable guard overwrote my run-1 script file mid-review. The output had already been captured, and the media file records the cases instead of the script text. Only the process is affected, not the product.

## Confirmation at ddccd51

Range `2d1a1b3..ddccd51` (`clean.go` +16/-8, `clean_test.go` +19/-3), checked out as `t-023d2-opus4`. Host only, git-only probes, no process signalled, no container run.

Verdict: pass
Risk-Tier: 3

B5 is closed. `noDuplicates` now strips a final component that EqualFolds `.git`, stats the worktree directory and compares entries by `os.SameFile`, with a lowercased-path fallback when both sides are missing. Every planted spelling that git itself treats as a duplicate now exits 3 with nothing removed. The extra refusals the new design adds (spellings git would not prune as duplicates) are all the safe direction and only fire on planted entries. One narrow regression against 2d1a1b3 remains, recorded as N14, non-blocking.

### 1. Reruns

`media/T-023d2/opus-confirm3-clean-probes.txt` runs each case through `git worktree prune -n -v` first (a dry run showing what git would prune) and then `waggle dispatch clean 23`. P-a (copy), P-b2 (gitdir link), P-c (leaf case), P-c2 (all upper-cased), P-c3 (`/.GIT`), P-c4 (directories upper-cased), P-c5 (`/.Git`), P-d (symlinked parent) and N-7 (relative gitdir) all exit 3 with all three admin entries kept and T-024 usable. Git's dry run says "Removing worktrees/T-024-builder: duplicate entry" for every one of them except P-d, so each was a live silent prune that is now refused.

### 2. Hunting the new design

Git's duplicate test (`prune_dups`) compares the raw gitdir strings, `.git` suffix included, with `fspathcmp`, after dropping any entry whose path fails `file_exists`. So git sees a duplicate only when the two strings are equal up to ASCII case (relative gitdirs are resolved first, see N-7). Waggle can miss one only where those strings name different files, or where one stat fails and the other does not.

- **Trailing slash, `/./`, `..`, the worktree directory instead of `.git`, `.git/`, `.git/.`, NFD for an NFC directory** (N-1 to N-6, N-11): all exit 3. Git's dry run treats none of them as duplicates (N-1, N-5 and N-6 are pruned as "non-existent location", the rest are kept), so these are over-refusals, the safe direction.
- **Missing path** (N-8, planted gitdir naming T-099): one side has `fi == nil` and the other does not, so the loop compares nothing and clean proceeds. Git prunes only the planted entry ("non-existent location"); T-024 stays usable. That is correct: a string equal to an existing path up to case is itself an existing path on a case-insensitive filesystem, so on macOS a missing gitdir can never be git-equal to an existing one.
- **Both missing** (N-9 differs in case, N-10 same string): both refuse through the fallback. Git would prune both as non-existent, not as duplicates, so this is conservative. The 2d1a1b3 design refused these too.

### 3. Precedence

`fi != nil && s.fi != nil && os.SameFile(fi, s.fi) || fi == nil && s.fi == nil && s.key == strings.ToLower(p)`. In Go `&&` binds tighter than `||`, so this parses as the two intended conjunctions. `SameFile` is never reached with a nil side.

### 4. Tests on case-sensitive filesystems

The skip is `os.Stat(filepath.Dir(g))`. On linux `case` and `upper` skip. `GIT` and `Git` do not skip, because `filepath.Dir` of `.../T-024-builder/.GIT` is the real worktree directory, so linux CI still runs `copy`, `link`, `GIT`, `Git` and `parent`. That covers the suffix fold, the stat compare and the link-through-parent path. The letter-case folding itself is pinned only on macOS (host runs). That is acceptable: on a case-sensitive filesystem without `core.ignorecase`, git does not fold case either.

### 5. Size, dashes, sign-off, CI

`git diff --shortstat c98d130..ddccd51 -- . ':!*_test.go'` gives 342 insertions (stop 350). No en or em dash in the range diff or the commit message. `ddccd51` author and `Signed-off-by` are the noreply address. `gh pr checks 59` at `ddccd51`: product-ci pass (2m23s). verify-leases-and-tiers fails with "leases: no claim for T-023; claims must stay until merge", expected as before.

### 6. Host suite

`go test -race -count=1 ./internal/dispatch/... ./cmd/...` passes: `internal/dispatch` 186.3 s, `cmd/waggle` 16.9 s. `go vet ./internal/dispatch/` is clean.

### Notes (non-blocking)

- **N14. Case folding now depends on the filesystem, not on `core.ignorecase`.** The 2d1a1b3 design folded case in the key itself. The new one folds only through `SameFile`, which folds case only when the filesystem does. If a repo has `core.ignorecase=true` on a case-sensitive filesystem (for example a `.git` copied from a macOS volume onto linux or a case-sensitive APFS volume), a planted directory `.../t-024-builder/` holding a `.git` entry and a gitdir naming it pass `file_exists` and match T-024's string under git's `strcasecmp`, so git would prune T-024's entry. Waggle sees two different inodes and does not refuse. This needs a manual misconfiguration plus a planted directory, and I reasoned it from git's `prune_dups` rather than probing it (no case-sensitive volume on this host). The fix is one line and simpler than the current guard: compare the keys whenever they are equal, `s.key == key || fi != nil && s.fi != nil && os.SameFile(fi, s.fi)`. It only adds refusals. Optional for this PR.

## Confirmation at 5ecb9c0

Verdict: pass
Risk-Tier: 3

Fix round 4 (`ddccd51..5ecb9c0`, `clean.go`, `worktree.go`, `clean_test.go`) implements D-200. `noDuplicates` keeps the symlinked-entry and unreadable-gitdir refusals and the N9 skips, drops waggle's own path comparison, and runs `git worktree prune --dry-run -v` through a copy of the pinned runner with the new `CLocale` flag, refusing with exit 3 on any line of the form `Removing <id>: duplicate entry` and on any dry-run failure. `CLocale` appends `LC_ALL=C` and `LANGUAGE=C` and sends stderr into the output buffer. No item blocks. Evidence: `media/T-023d2/opus-confirm4-clean-probes.txt`, `media/T-023d2/opus-confirm4-host-test-race.txt`.

### 1. Earlier bypass probes

I reran my confirm-3 probe set and fable's confirm-3 probe set 2 unchanged, against a binary built from 5ecb9c0.

- **Every case where git's dry run reports a duplicate exits 3 with nothing removed, and T-024 stays usable with its entry kept.** That covers P-a copy, P-b2 gitdir link (still refused earlier, by the gitdir check), P-c leaf case, P-c2 and P-c4 upper case, P-c3 `.GIT`, P-c5 `.Git`, N-7 relative, P11 (fable's B6), P12 and P13.
- **Every case where git reports no duplicate exits 0 and leaves T-024 alone.** These are P-d link parent, N-1 to N-6, N-8, N-11 and P14. Git either keeps the planted entry or prunes only `worktrees/0` as "non-existent location". In N-9 and N-10, T-024's worktree directory was deleted by the probe setup, so git prunes T-024's entry as non-existent. That is git's ordinary prune of a missing worktree, which clean already did at N-8 and which the new healthy-repo test asserts for T-025.

### 2. Parse robustness

- **Hiding a duplicate does not work.** `prune_dups` sorts the entries and reports every entry after the first one with the same path. The line git prints names the entry it drops, not the one an attacker planted. An attacker needs T-024's entry to be the one dropped, so the planted id has to sort below it, and then the reported line is `Removing worktrees/T-024-builder: duplicate entry`, with a name waggle wrote. I-1 plants a duplicate named `0<newline>X` and gets exit 3. In I-2 the planted id `zz<newline>foo` sorts above T-024. Git splits its own line, waggle does not see a duplicate, clean proceeds, and git drops only the planted entry. That is harmless.
- **Forging a duplicate only adds refusals.** I-3 names an entry `a: duplicate entry`. Git prints `...: duplicate entry: gitdir file points to non-existent location`, which does not match. I-4 puts a newline in the name and forges a matching first line, which gives exit 3, the safe direction. I-5 embeds a newline and a forged line in the gitdir content. Git reads the gitdir as one path, prunes it as non-existent and prints nothing forged.
- **Merged stderr.** Git 2.54 prints the `Removing` lines on stderr only (stdout is 0 bytes). Hostile repo config (`color.ui always`, `core.quotePath`, an unknown key, `gc.worktreePruneExpire never`, I-7) and a hostile Keeper environment (French `LC_ALL`/`LANG`/`LANGUAGE`, `GIT_CONFIG_PARAMETERS` color, I-6) neither hide the line nor change the result. Both exit 3. A warning can only add a line. It forges a match only if the attacker controls its text, which again only refuses, and a warning that makes git exit non-zero also refuses.
- **CLocale does not leak.** `dry := *c.pg` is a value copy, so `c.pg.CLocale` stays false for `worktree list`, `worktree remove` and the real `prune`. Using the same `*bytes.Buffer` for Stdout and Stderr is safe: `os/exec` serializes writes when the two are the same comparable writer.
- **Locale.** `exec.Cmd` keeps the last value of a duplicated key, so the appended `LC_ALL=C` beats any inherited value. GNU gettext ignores `LANGUAGE` when the message locale is `C`, so `LC_ALL=C` alone already forces the msgid, and `LANGUAGE=C` is a harmless extra. I could not test a gettext-enabled git: the host has only Apple git, which printed English under I-6. This is reasoned, not measured.
- **Locked entries.** I-8 plants a locked copy of T-024's entry. Git does not prune locked entries and leaves them out of the duplicate list, so T-024 is not dropped and clean exits 0.

### 3 and 4. TOCTOU and the real prune

Inside `role()`, the dry run is followed by `worktree list --porcelain`, `worktree remove --force` on paths waggle chose, and then `worktree prune`. All three use `c.pg`, the same pinned runner without CLocale. None of them adds an admin entry. Removing our own worktree turns any entry pointing at it into a "non-existent location" prune, not a duplicate. Clean holds `dispatch.lock` and the role's lock, so no dispatch run can write `.git/worktrees` in that window.

Only a process escaped from an earlier run, or the Keeper, can write the admin dir between the dry run and the real prune. An escaped process is the existing NotEnforced setsid/daemonize clause, together with the host-files.json clause. Such a process can also rewrite T-024's own gitdir, refs or config at any time, so the window adds no capability. This is the same window as the gitdir checks had at ddccd51.

### 5. Refusal changes vs ddccd51

- **Fewer refusals, all correct.** P-d, N-1 to N-6 and N-11 now exit 0 where they exited 3. Git does not drop T-024 in any of them, so these were over-refusals. Fable's P11 and P12 now exit 3 where ddccd51 let git drop T-024, which was an under-refusal. N-9 and N-10 now prune two entries whose worktrees are both missing, which matches plain `git worktree prune`.
- **My earlier N14 is resolved.** That was case folding on a case-sensitive filesystem with `core.ignorecase=true`. Git now decides the folding itself.
- **The "relative" test mode and the `dup` oracle** use the same dry run as the code under test. They still pin the safety property: when git reports no duplicate, the test requires T-024's entry and worktree to be intact.

### 6 and 7. Size, dashes, sign-off, CI, host suite

- **Size:** `git diff --numstat c98d130..5ecb9c0`, excluding tests, gives 347 added lines (stop 350).
- **Dashes:** no en or em dash in the range diff or the commit message.
- **Sign-off:** the author and `Signed-off-by` are `27632860+paulternate@users.noreply.github.com`.
- **CI:** `gh pr checks 59` shows product-ci pass (2m46s). verify-leases-and-tiers fails with "leases: no claim for T-023; claims must stay until merge", expected as in earlier rounds. relay-claim is skipped.
- **Host suite:** `go test -race ./internal/dispatch/... ./cmd/...` passes: `internal/dispatch` 182.5 s, `cmd/waggle` 16.9 s.

### Notes (non-blocking)

- **N15. A failed dry run loses git's message.** With CLocale, stderr goes into `out`, and the error path formats `errb`, which is now empty. A dry run that fails therefore reports `git worktree: exit status N: ; nothing was removed` without git's reason. It still refuses, so this is diagnostics only. A one-line fix is to append `out` to the error when CLocale is set.
- **N16. The check depends on git's English reason string.** If a future git rewords "duplicate entry" or stops printing it under `-n -v`, the check passes silently, which fails open. Git versions before `prune_dups` existed do not drop duplicates at all, so older git is safe. `TestCleanRefusesDuplicateEntries` (copy duplicate, expects exit 3) runs in CI, so a rewording would surface as a failure there on that git. This is acceptable for now.
- **N17. Older gits that print `Removing` lines on stdout.** On such a git, stdout is block-buffered and stderr is not, so a stderr write in the middle of the prune loop could in principle split a line at a buffer boundary. Git's prune loop writes nothing else to stderr per entry, so I see no way to trigger it. Theoretical only.

## Confirmation at f519bf6

Verdict: pass
Risk-Tier: 3

**Intent Summary:** Fix round 5 (`5ecb9c0..f519bf6`, clean.go, worktree.go, clean_test.go) applies D-202. `waggle dispatch clean` no longer runs `git worktree prune` or its dry run. For a worktree that exists it runs `git worktree remove --force` on the listed path that matches the ticket's own path; for a worktree that is gone it removes only the one admin directory whose gitdir names that path. `entryFor` still refuses a symlinked entry, a gitdir that is not a small regular file, and two entries naming the same worktree. `Git.CLocale` is gone. Blast radius: clean can now leave other stale entries, which `Worktree.provision` still prunes at the next dispatch (D2b item). Reviewed under the D-203 threat bar.

**Blocking Issues:** none.

### 1. No repo-wide git call

clean.go has three git calls: `rev-parse --path-format=absolute --git-common-dir`, `worktree list --porcelain` (read only) and `worktree remove --force <p>`, where `p` must equal one of `wants`. The only `worktree prune` left in non-test code is `Worktree.provision` (worktree.go:210), which is the D2b item. The one filesystem delete is `os.RemoveAll(entry)` on an entry `entryFor` selected.

### 2. entryFor and ordinary path spellings

Probes in `opus-confirm5-clean-probes.txt` (binary from f519bf6, git 2.54.0, repo reached through a symlinked parent or by its real path, tickets T-001 and T-002, clean T-001 builder):

- **Absolute gitdirs (git's default).** Git writes the realpath of the worktree. `wants` carries `EvalSymlinks(Dir(wt))/Base`, so the entry is found both when the worktree exists (A) and when it is gone (B through the link, C through the real path). Only `worktrees/T-001-builder` goes, and T-002 stays. The macOS `/var` vs `/private/var` case is the same shape and is covered by `TestCleanMissingWorktreeRemovesOwnEntry` through `eachRepoSpelling` under `t.TempDir()`.
- **Relative gitdirs (`worktree.useRelativePaths=true` in repo config; the pinned env drops only global and system config).** Git writes `../../../.waggle/worktrees/T-001-builder/.git`. With the worktree present (D), clean works, because removal goes through `worktree list`, which git prints absolute. With the worktree gone (E, F), `entryFor` compares the raw relative string with absolute `wants`, finds nothing, removes nothing, prints nothing and exits 0. The entry stays as `prunable`. That is a leak in the safe direction (N18). The ambiguity check cannot misfire on relative entries: they never match, so they cannot add a false second hit.

### 3. Missing-worktree path

`os.RemoveAll(entry)` runs only after the role lock, `requireRealDir` (skipped since `wt` is gone), and an `entryFor` pass with no refusal and exactly one hit. `entry` is `<common>/worktrees/<name>` from `os.ReadDir`, and a symlinked entry has no `IsDir` type, so it is refused, not followed. The hit needs a gitdir equal to `<wt>/.git` under one of the two spellings of this ticket's own path. Ordinary git state cannot point another ticket's entry at that path: `git worktree add` refuses a registered missing path without `-f`, and provision prunes first. Only git-dir write access can plant such an entry, which is a note under D-203, and even then the only entry deleted names our own worktree.

### 4. Existing-worktree path

Only `git worktree remove --force <p>` for a `worktree list` line whose path is in `wants` (which now also has `EvalSymlinks(wt)`). `requireRealDir(wt)` still refuses a symlinked worktree first.

### 5. Behavior change

Nothing in `Run` relied on clean's prune. `hostFiles` snapshots other admin entries only inside a run under `dispatch.lock`, which clean also takes, so clean cannot change them mid-run. A ticket's stale entry that clean leaves (N18 or any other) is pruned by `provision` before `worktree add`, so redispatch still works. Once D2b guards or narrows that prune, N18 matters more: a leaked relative entry would then make `git worktree add` fail with "missing but already registered".

### 6. Tests and fix removal

New tests: (a) `TestCleanRefusesAmbiguousEntries` (worktree present and gone), (b) `TestCleanLeavesPlantedEntriesAlone` (copy, case, `.GIT`, upper, parent link, relative through a link; T-024 stays usable via `git status`), (c) `TestCleanMissingWorktreeRemovesOwnEntry` (both repo spellings; T-024 usable and T-025 prunable both stay), (d) `TestCleanKeepsOtherTicketsPrunableEntry`, (e) `TestCleanRefusesLinkedGitdir`. I ran four mutants, each against `go test ./internal/dispatch -run TestClean`, restoring the file after each one:

- Re-adding `worktree prune` after removal fails (b), (c) and (d).
- Disabling the `len(found) > 1` refusal fails (a).
- Disabling the gitdir refusal fails (e).
- Skipping `os.RemoveAll(entry)` fails (c).

No test covers a missing worktree whose own entry is relative (N18).

### 7 and 8. Size, dashes, sign-off, CI, host suite

- **Size:** `git diff --numstat c98d130..f519bf6` without tests and hive files gives 343 added lines (stop 350). clean.go accounts for 198.
- **Dashes:** no en or em dash in the added lines of the range or in the commit messages.
- **Sign-off:** both commits have the author and `Signed-off-by` `27632860+paulternate@users.noreply.github.com`.
- **CI:** `gh pr checks 59` shows product-ci pass (2m30s). verify-leases-and-tiers fails with "leases: no claim for T-023; claims must stay until merge", which is expected. relay-claim is skipped.
- **Host suite:** `go test -race ./internal/dispatch/... ./cmd/...` passes: `internal/dispatch` 190.0 s, `cmd/waggle` 17.8 s (`opus-confirm5-host-test-race.txt`). It ran from a clean tree, and the mutants ran after its build.

### Notes (non-blocking)

- **N18. A missing worktree with a relative gitdir is not cleaned.** `entryFor` does not resolve a relative gitdir against its admin dir, while `adminAt` (worktree.go) does. Clean exits 0 silently and leaves the entry, and provision's prune recovers it today. A two-line fix, `if !filepath.IsAbs(g) { g = filepath.Join(admin, g) }` before the `Clean`, would match `adminAt`. It should land before or with D2b, plus a test that sets `useRelativePaths` before creating T-023's worktree.
- **N19. One bad entry anywhere blocks clean for every role.** `entryFor` runs even on the early-return probe, so an unreadable or symlinked gitdir in any other entry refuses clean with exit 3. Only git-dir write access can create that, so this is a note under D-203. It fails closed and nothing is removed.
- **N20. Silent no-op messages.** When nothing matches (N18, or an existing worktree that `worktree list` spells differently), clean prints no "removed" line yet still drops the tainted marker and the takeover guard. The Keeper sees only exit 0. Diagnostics only.
- N15 to N17 are void now that the dry run and CLocale are gone.

## Confirmation at bfdb82f

Verdict: pass. Risk-Tier: 2 (unchanged).

Fix round 6 (`f519bf6..bfdb82f`) changes `entryFor` in `internal/dispatch/clean.go` to join a relative gitdir to its own admin dir before the `Clean` and suffix match, the same way `adminAt` does, and adds `TestCleanDropsMissingWorktreeEntryWithRelativeGitdir`.

1. **N18 and fable B8 closed.** The new test sets `worktree.useRelativePaths=true`, adds T-023 and T-024, removes T-023's directory and checks that clean drops only T-023's entry. I added a scratch probe (not committed) with the symlinked repo parent (`newFixture(t, false, true)`, worktrees added through the link) and a third worktree `T-0230-builder` as a prefix neighbor. git recorded `../../../.waggle/worktrees/T-023-builder/.git`; clean printed `removed worktree .waggle/worktrees/T-023-builder` and left `T-024-builder` and `T-0230-builder` (`opus-confirm6-with.txt`).
2. **No wrong match from ordinary state.** `c.common` comes from `rev-parse --path-format=absolute --git-common-dir`, and git writes the relative path between the real admin dir and the real worktree. For worktrees under `.waggle/worktrees/` the `..` steps stay inside `.git/worktrees/<name>`, so a lexical Join gives the same path as git's realpath resolution. The two can differ only when a symlink is planted inside `.git/worktrees` or along the path, which needs git-dir write (note under D-203). Exact equality against `wants` and the `len(found) > 1` refusal still hold.
3. **Tests and fix removal.** All 13 `TestClean*` tests pass with the fix. Disabling the relative branch fails both the new test (`out "" err <nil>`) and my linked probe, which leaves the T-023 entry behind (`opus-confirm6-without.txt`). The file was restored and the probe removed afterwards.
4. **Size, dashes, sign-off, CI.** 347 added non-test lines vs `c98d130` (stop 350). No en or em dash in the added lines or the commit messages. All 11 commits carry `Signed-off-by` `27632860+paulternate@users.noreply.github.com`. `gh pr checks 59`: product-ci pass (2m33s). verify-leases-and-tiers fails with "leases: no claim for T-023; claims must stay until merge", as in earlier rounds. relay-claim is skipped.
5. **Host suite.** `go test -race ./internal/dispatch/... ./cmd/...` passes: `internal/dispatch` 170.0 s, `cmd/waggle` 16.8 s (`opus-confirm6-host.txt`).

N18 is closed. N19 and N20 stand as notes.
