<!-- SPDX-License-Identifier: MIT-0 -->
# T-023 phase F step 3 (PR #65, `t-023f-switch` at `de8baa4`, rebased on main `91b9e2f`): opus confirming guard review 4

Verdict: pass
Risk-Tier: 3

## Intent Summary

Narrow confirm of what changed since `ec50cac` (fable confirm3, pass). The branch was rebased from `790ade0` onto main `91b9e2f`, which now holds hive records 15 and PR #66 (the `TestRepoConfig` fix, D-219). `git range-diff` shows commits 1 to 13 identical (`=`) to the old 4 to 16, the three hive records 15 commits dropped as now upstream, and two new commits: `78aa70f` adds `reviews/T-023f3-fable-confirm3.md` and `reviews/T-004-remote-test-opus.md`, and `de8baa4` changes four lines of `tasks/T-023-trial.md`. The whole tree diff `ec50cac..de8baa4` is those three files plus #66's `internal/config/config_test.go` change, nothing else. The four runbook hunks are exactly the approved set: N1 (Pass bullet ends "and no other listable leftover pid is alive in its after capture (Blocks bullet)"), N2 (the run 0p qualifier "when the SIGKILL path was taken" removed), and the two N5 lines (Status line names fix round 3 and D-218; the 2.2 lead sentence excepts `held2.pl`). Blast radius: the runbook text the queen drives for the trial and the cutover card; no product code.

## Blocking issues

None.

## Check 1: only the approved content moved

- Range-diff `790ade0..ec50cac` against `91b9e2f..origin/t-023f-switch`: 13 of 13 carried commits are `=`; the only additions are `78aa70f` and `de8baa4`.
- `git diff ec50cac de8baa4`: the two added review files, 4 changed lines in the runbook, and `config_test.go` from #66. The `.gitignore` hunk vs main is the same two lines as before the rebase.

## Check 2: the Gate subsection against D-214, D-217 and D-218

- **Not vacuous.** Pass still needs `held.pid` written, that pid in run 0a's `survivors_killed` and dead in `ps-0a-after.txt` (D-217). A missing or unlisted `held` is "sweep not exercised", one rerun, and a card that cannot cite the gate.
- **Live leftovers block.** The new Pass clause defers to the Blocks bullet, so a live `held2`, or a live `probe` or its shell in run 0p, fails Pass and blocks cutover (D-214, "any live pid blocks"). With N2's qualifier gone, a `probe` alive after a SIGTERM-only path is now a listable leftover and blocks. That closes the gap fable confirm3 N2 described.
- **SIGTERM cleanup does not block.** A `probe` and shell that Claude Code ends on SIGTERM are dead in `ps-0p-after.txt` with `survivors_killed` empty. They are not alive, so the new Pass clause holds and the first Blocks clause does not fire. The "Not evidence, not a block" bullet names the case. The old Pass wording would have failed it by the letter, since it required every other listable leftover in `survivors_killed`; N1 removes that.
- **`held2` miss does not block.** A dead `held2` missing from `survivors_killed` passes the new clause. The Pid path and rerun bullets record it as "pid path not proven", which matches D-218 and opus confirm2 N1 and N2. The literal clash fable confirm3 N1 found is gone.
- **D-214's letter.** D-214 wrote "every listable leftover pid appears in survivors_killed". The runbook now requires that only of `held` and makes it the pid-path test for `held2`. The same decision also says a SIGTERM self-cleanup is not a block, so for `probe` its letter cannot hold. For `held2`, D-218's pid-path framing decides. The runbook follows the narrower and newer rulings, and the Keeper approved this fold, so I see no conflict that needs a decision.

## Check 3: no regression on main

- `git diff --stat origin/main...origin/t-023f-switch`: 20 files, all under `.waggle/`, `.claude/hooks/` or `.gitignore`. No file under `internal/`, `cmd/`, `go.mod` or `go.sum`, and the Go tree is byte-identical to main.
- `DECISIONS.md`: 118 added lines, 0 removed. A `waggle` built from this tree reports `decisions: ok, appended D-214, D-215, D-216, D-217, D-218, D-219` for `--base origin/main --head origin/t-023f-switch`, exit 0.
- `go test -race -count=1 ./internal/config/` on a detached checkout of `de8baa4` (whose `config.yaml` sets `forge.remote: origin`): ok. That checkout was a temporary git worktree under the session scratchpad, removed afterward.

## Check 4: scan, dashes, attribution

- `scan-hive.sh origin/main origin/t-023f-switch`, run in that same temporary checkout with the local names file copied in and deleted afterward: 15 commits scanned, 0 hits, exit 2. The 10 held changes are the control, hook, `.gitignore` and task paths this PR is expected to carry, and the decisions gate is ok.
- No em or en dash in any added line of the PR or in any commit message.
- No attribution line or trailer. Every author and committer is the public noreply identity. The only "claude" match in commit text is the env var `CLAUDE_PROJECT_DIR`.
- D-193: this review ran only git, `go build`, the config package tests, the decisions gate and the scan. No tracker, sweep or probe code ran.

## Notes (not blocking)

- **N1, the Blocks bullet's second clause.** "or listed alive with no `survivors_killed` entry" does not say which capture. Read as the after capture, it repeats the first clause. Read as the during capture, it would block a SIGTERM-cleaned `probe` and an unlisted dead `held2`, which contradicts the "Not evidence" and Pid path bullets. This text predates this round and would only over-block, which shows up on a Keeper card, so it is wording. Exact fix: delete the clause, leaving "any listable leftover pid alive in an after capture."
- **N2, the Status line.** It says confirm3 "N1 and N2 folded in", but this round also folded N5 (the Status line itself and the 2.2 lead). Fix: "N1, N2 and N5 folded in".
- **N3, carried over.** Fable confirm3 N3 (the pid-path claim assumes the tracker listed `held2.pl` within its 0.2 s pause) and N4 (which run's `held` the card cites after a rerun) were not in the approved fold and remain open as notes.
