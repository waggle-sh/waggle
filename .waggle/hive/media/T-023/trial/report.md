<!-- SPDX-License-Identifier: MIT-0 -->
# T-023 phase F trial report (2026-10-22)

Run by the queen with the Keeper present, per `tasks/T-023-trial.md`. Binary built from the pin `790ade0` (`vcs.modified=false`); Claude Code 2.1.308, not the 2.1.304 of probe 2 (`versions.txt`).

## Runs

| Run | Ticket | Role | Model | end | Exit | Tokens | Cost (USD) | Wall |
|---|---|---|---|---|---|---|---|---|
| 0a | T-901 | builder | sonnet | no_change | 1 | 40,318 | 0.021 | 14 s |
| 0p try 1 | T-903 | builder | none | claim refused | 4 | 0 | 0 | 0 s |
| 0p | T-903 | builder | sonnet | timeout | 5 | 7,641 | null | 90 s |
| 0c try 1 | T-902 | builder | sonnet | no_change | 1 | 17,907 | 0.020 | 308 s |
| 0c | T-902 | builder | sonnet | interrupted | 1 | 7,646 | null | 13 s |
| 1 try 1 | T-026 | builder | sonnet | lease_violation (dirty) | 1 | 151,842 | 0.164 | 35 s |
| 1 | T-026 | builder | sonnet | done, pushed | 0 | 439,087 | 0.252 | 99 s |
| 2 | T-026 | guard | opus | done, verdict pass | 0 | 282,760 | 0.392 | 83 s |

`waggle status` after the trial: T-026 builder 2 sessions, guard 1 session, est. $0.81 (`status.txt`). Probe runs are counted in the scratch hives' own ledgers, which were deleted on the 7.2 card.

## D-214 and D-217 gate: pass

- Run 0a wrote `held.pid` (39799). It is in `survivors_killed` and dead in `ps-0a-after.txt`. No other listable leftover was alive. The gate passed on the first run, with no rerun.
- Pid path proven: `held2` (39888, its own pgid 39888 in `ps-0a-during.txt`) is in `survivors_killed` and dead.
- During capture: `held.pl` sat in pgid 39797 (the tool shell's group, distinct from claude's 39711), so the tool shell led its own group on 2.1.308 as on 2.1.304 (opus T-023f3-confirm N4). The shell had already exited when the capture ran, so this is read from `held.pl`'s pgid.
- No `caffeinate` entry appeared in any `survivors_killed`.
- Run 0p did not reach the SIGKILL path: `survivors_killed` is empty and both the tool shell `zsh` (46857) and `perl` (46858) are dead. An empty `survivors_killed` after a SIGTERM self-cleanup is Claude Code cleaning up, not a sweep result.

## D-199 measurement

on macOS (sysctl read every 25 ms) a background child escapes until T-024 when its tool shell started and exited between two reads, which is the usual cmd & from a short-lived shell (0 of 20 caught with a shell gone within 5 ms, 20 of 20 with one alive 20 ms), or when it calls setsid or daemonizes before the next read; a child that stays in the group of a tool shell the tracker listed alive is killed by the group sweep

- `bg1`: refused by the permission rules (the transcript shows the denial), so the D-199 measurement rests on `bg2` alone.
- `bg2` (40044): caught (in `survivors_killed`, dead after).
- Which path killed what (inferred from `ps-0a-during.txt`, since `result.json` does not record it): `held` and `bg2` by the group path (their tool shell's group was tracked); `held2` by the pid path (its own group is never a tracked group, D-218). Run 0p's `zsh` and `perl` were not swept at all.
- D2 opus N8 (a darwin child still in `SIDL` when its parent is reaped can be missed by one read): no child in this trial was missed, but a sample this small cannot rule N8 out.

## D-213: session closed mid-run

- Try 1 was not exercised: the run ended on its own (`no_change`) before the Keeper typed `/exit`. The worker's Bash tool moved `probe.pl` to the background at 120 s and the worker waited out its 300 s sleep, so the 120 s assumption in the runbook does not hold on 2.1.308.
- Try 2: the Keeper closed the session with `/exit` about 10 s after the start and chose "stop running tasks" at Claude Code's prompt. The hangup reached dispatch: `end: interrupted`, exit 1, `result.json` written, `dispatch.lock` released, the T-902 worktree kept, `survivors_killed` empty, and dispatch, worker, tool shell and `probe.pl` all dead.

## Run 1 and run 2 (T-026, first remote-mode run)

- Claim ref `refs/waggle/claims/ticket-26` reached origin. Draft PR #68.
- Try 1 failed: the builder made two compound Bash calls (`cd ... && python3 ...` and `gofmt ...; go build ./... && ...`), both denied by the dontAsk allowlist. It took that to mean Bash was off and stopped uncommitted. The Keeper approved clean and a brief fix (PR #67: a Bash forms bullet; opus must-fix M1 and M2, fable confirm pass with notes folded, opus confirm pass). The rerun built and pushed cleanly.
- Gate run (`gate-run.txt`): CI fetched the claim ref; `waggle gate verify` printed `leases: ok` and `tiers: ok`; product-ci passed. This is the T-014 AC7 proof (D-159, D-168).
- Guard: opus by policy, `verdict: pass`, `head_oid` equal to the PR head at review time. The branch was then rebased onto main on a Keeper card. The patch is identical (`run2-review-head-equivalence.txt`), and the AI co-author trailer the builder added was dropped. CI passed again and #68 merged as `18961b5` on that card.

## Host record

`host-record-first.txt`: `no host record; the next run writes one`. No stale lock warning, and no dispatch exited 3, so no `--accept` card.

## Clean

`clean.txt`: `nothing to clean for T-026`. The guard run had already removed its own worktree, and the builder worktree from try 1 was cleaned on its own card.

## Not covered by the runbook (follow-ups)

1. A finished run keeps its claim for the full lease TTL (120 min), and no verb releases it, so a second dispatch on the same `--dir` in one hive is refused (exit 4). On a Keeper card the trial used a fresh, identical scratch hive per run (hive-0p, hive-0c, hive-0c2).
2. The builder role and prompt do not tell the worker that Bash takes one allowlisted command per call in the exact WAGGLE.md section 3 forms. Fix this in the role template or the dispatch prompt, not per brief.
3. The worker's push tripwire (`url.file:///dev/null/.pushInsteadOf`) also rewrites local pushes, so workers cannot run any test that pushes to a local bare origin (seen by both the builder and the guard). Those tests ran only on the host and in product-ci.
4. A reused branch is not rebased: run 1's rerun built on the `t-026` branch from try 1 (parent `5df0422`), while `result.json` recorded `base_oid` `8deba1c`. The recorded base does not match the real parent.
5. The stdout progress line on a timeout says `ended: interrupted` while `end` is `timeout`. `cost_usd` is null on timeout and interrupt, because no result event arrives.
6. The `CAP` pattern also matches the claude worker, because its appended system prompt carries the probe commands.
7. The builder committed with a `Co-Authored-By: Claude` trailer. Dispatch does not set the worker's attribution setting.
8. T-026 builder note for review: `TestRunPlantDuringAutoMaintenance` may now pass without the worker-side setting.
