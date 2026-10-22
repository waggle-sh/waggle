<!-- SPDX-License-Identifier: MIT-0 -->
Verdict: must-fix
Risk-Tier: 3

# T-023 phase F step 3, fix round 2: confirming guard (opus)

PR #65, branch `t-023f-switch`, head `6b6abac190d113bf8b7737ad0ece0c3d9be384d4`, base main `d87640f`. Round 2 diff is `4be74e6..6b6abac`. Product tree read at `d87640f`; no product file is in the diff.

## Intent Summary

Fix round 2 per D-217 for the step 3 confirming reviews. The trial runbook's `hold.sh` now sends `held.pl`'s output to /dev/null, and `held.pl` sleeps 300 s. The Gate subsection can no longer pass without `held.pid` in run 0a's `survivors_killed` and dead afterwards. A missing or never-listed pid means "sweep not exercised": rerun once, and the cutover card cannot cite the gate. A second child, `held2.pl` (`setpgrp(0, 0)` first, launched by `hold2.sh`), is meant to prove sweep's pid path. Smaller changes: `CAP` covers it, the redaction `sed` covers resolved roots, run 0c names its paths, and 7.2 gains `--repo`. WAGGLE.md section 4 widens the by-role clause. The queen summary says the fixture fix is T-026. The hook change is a comment. `DECISIONS.md` appends D-214 to D-217. Blast radius is the runbook that gates dispatch cutover tomorrow, plus Tier 3 steering text that loads into every queen session.

## Blocking Issues (Must-Fix Only)

1. **`held2.pl` as written does not exercise the pid path. The report and cutover card would cite pid-path evidence that does not exist (D-217, D-203).**

   The tracker marks a process as a group leader only when it first sees it. `observe` in `reap_unix.go` sets `t.leaders[k]` only for a process not yet in `seen`, and only if `pgid == pid` at that moment. Perl runs `setpgrp(0, 0)` as its first statement, a few milliseconds after the fork. The 25 ms poll therefore almost always first sees `held2` after it already leads its own group, so it goes into `leaders`. At sweep time, `targets` names that group: the leader pid is present with its recorded start time, it has a live member (itself), and it is not dispatch's group. `sweep()` kills groups before pids, so `held2` dies on the group path, and `note` records it once.

   I measured this on this Mac with the shipped tracker code at `d87640f`. A throwaway darwin test (deleted after, never committed) built the tree as a setsid `sh` that runs `perl ... >/dev/null 2>&1 & sleep 1`, polled at `pollEvery`, and after the shell exited called `observe` and `targets` once. It called no `sweep` and no `kill`, and the children exited on their own. With `setpgrp(0, 0)` first, the child's pgid was in `groups` in **15 of 15** trials. With a 0.2 s delay before `setpgrp`, the child was in `pids` and not in `groups` in **15 of 15**.

   So the runbook's claims are false for the shipped shape. Line 48 says "It stays tracked by pid only, so it proves sweep's pid path". The D-199 paragraph says "only `held2`, outside every tracked group, proves the pid path". The 7.3 report item says "`held2` for the pid path". The Pid path gate bullet would then record "exercised" on a group kill. Sweep evidence for cutover is still real, because `held2` is killed by the sweep either way. But D-217's added requirement, pid-path evidence, would be reported as met when it was not. That is the misattribution D-217 was opened to remove.

   Fix (runbook text only):
   - Line 48: make `held2.pl` start with `select(undef, undef, undef, 0.2); setpgrp(0, 0);`, then the pid file, then `sleep 300;`. Replace "It stays tracked by pid only, so it proves sweep's pid path" with: "The tracker marks a group leader only when it first lists a process. The 0.2 s pause makes the first listing see `held2.pl` still in the tool shell's group, so its own later group is never a tracked group, and only sweep's pid path can kill it."
   - Leave the `ps-0a-during.txt` check in run 0a step 2 ("`held2.pl` has its own pgid") as it is. It is still right, because the pid file is written after `setpgrp`.
   - No change is needed to the Gate bullets, the D-199 paragraph or 7.3 once line 48 is fixed. They become true.

## Checks

1. **Round 1 must-fix (vacuous pass): closed.** The Pass bullet requires `held.pid` written, its pid in run 0a's `survivors_killed`, and dead in `ps-0a-after.txt`. I checked each way the first `held` clause can fail:
   - Pid file missing: "sweep not exercised".
   - In `survivors_killed` but alive: blocks cutover.
   - Alive and not listed: blocks cutover.
   - Dead and not listed: not exercised.

   The not-exercised case requires one rerun after the Keeper sees the transcript, then a cutover card that says so and cannot cite the gate. Line 123 (8.1) adds "a gate the sweep never exercised is not a pass". No path through the subsection passes with zero listed pids. This matches D-217's Keeper choice word for word.
2. **`held2`, apart from must-fix 1.** The tracker does record `held2`'s pid even though it leaves the group at once. Tracking is by parent: `hold2.sh`'s `sh` is a tracked parent for about 1 s, about 40 polls. Once seen, the pair of pid and start time stays in `live` after it reparents to launchd (no subreaper on darwin). Perl's two-argument `setpgrp` is `setpgid(0, 0)`. It makes `held2` a group leader in the tool shell's session, not a session leader. It succeeds because `held2` is not a session leader. The tracker never reads sessions, so leader versus session leader does not matter to it. What matters is first-sight leadership (must-fix 1). The Pid path bullet is not vacuous: a missing pid file or an unlisted pid is "not exercised", the same rerun-once rule applies, and a live `held2` falls under the Blocks bullet as a listable leftover.
3. **Redirect.** In my own Bash tool session, a script of the form `perl -e 'sleep 6' >/dev/null 2>&1 & sleep 1` returned at about 1 s. A `ps` in the next call showed the perl alive with ppid 1, still in the departed tool shell's pgid, and it exited on its own. The non-interactive `sh` gives the background job /dev/null as stdin, so no tool pipe is held. `held.pl` and `held2.pl` therefore outlive the call, and the worker's normal exit, as run 0a needs.
4. **D-193.** Round 2 adds no signal. The new commands are `ps` only, plus the probe children's own sleeps (at most 300 s). Run 0c changed only in its run-record and lock paths and in `no_change` in the "kept running" list. It still says never signal an orphan.
5. **DECISIONS.md.** A binary built from the head printed `decisions: ok, appended D-214, D-215, D-216, D-217` for `waggle gate decisions --base origin/main --head HEAD`, exit 0, with `origin/main` at `d87640f`. The diff is one hunk at line 4089 with no removed lines. The redaction at `T-023f3-fable-confirm.md` line 34 replaces the per-user temp folder path with the words "the macOS per-user temp folder". The finding still says the resolved temp root form needs its own `sed` term, which the runbook now has.
6. **Scan.** I copied the names file in, ran `scan-hive.sh origin/main origin/t-023f-switch` and deleted the file afterwards. Result: 0 hits, decisions gate clean, exit 2 (10 held control and task paths, expected for this PR). Neither the added lines nor any commit message has an em or en dash. Every author and committer is the public noreply identity. The only trailers are `Signed-off-by`, with no attribution.
7. **Other round 2 text.** The hook diff is one comment line, and `sh -n` passes. WAGGLE.md section 4 and the summary line are as the round 1 reviews asked. In the redaction `sed`, the resolved scratch and temp terms come before the home term, and `<main>` is resolved without depending on the cwd.

## Notes (not blocking)

- N1: Two Gate bullets point different ways. The Pid path bullet says a pid-path failure is "recorded as not exercised". The rerun bullet says "the same rerun-once rule applies to `held2`", and by reference that includes "cannot cite the gate as passed". So a second pid-path miss would block citing the gate even when `held` passed. That is stricter than D-217 strictly requires, and safe. One clause would settle it: "if the pid path is still not exercised after the rerun, the cutover card says so; the `held` gate result stands on its own".
- N2: D-217 says `held2` "backs up `held.pl`", but the Pass bullet still requires `held`. If Claude Code cleans up the tool shell's group at exit, both runs of 0a come back "not exercised" even when `held2` was swept. That matches the Keeper's text. The cutover card should still show the `held2` result in that case.
- N3: With the fix, the `comm` recorded at first sight may be `sh`, from before the fork execs perl. `survivors_killed` takes `comm` from the sweep-time table, so it reads `perl`. No action.
