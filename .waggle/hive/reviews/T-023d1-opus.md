<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: T-023 phase D1 (PR #57, `t-023d1` at `d48deaf`, base `2ecd449`), first guard (opus)

Verdict: must-fix
Risk-Tier: 3

## Intent Summary

Phase D1 opens the guard path of `waggle dispatch` and lands the dispatch carry items. `--role guard` reads the builder's claim (exit 4 without one) and writes none, resolves the branch head once to `head_oid`, provisions `T-<nnn>-guard` detached at that OID (resetting and cleaning a reused one), launches with guard task lines (no commits), checks rules 3 and 4 with zero allowed ref moves and a worktree `HEAD` that must still equal `head_oid`, parses the last `Verdict:` line of the final message (none, or a final at `MaxFinal`, is `no_verdict`, exit 5), saves `review.md` and never pushes. Every run now appends one ledger line after `result.json` (failure is a warning). Before the claim, dispatch runs `Verify`, the unix check, the gitignore check for `.waggle/worktrees/` and the ledger, the worker identity resolution and `Prepare` with prompts built in memory, so those refusals write nothing. A new `host-files.json` records the host file hashes after a clean run and the next run compares them before the fetch. Also: path-free global keys, the own-claim OID check in local mode, the stdout-holder cut, the 4 MiB `tap` line, the git-free files step, `waggleCommands` by title, `GIT_CONFIG_PARAMETERS=` in the comb env, `max_budget` in the ledger, the per-phase must-fix count, and the D0 and D0b `not_enforced` and fixture carry. Blast radius: every dispatched session, plus the cross-model review step the hive's Tier 3 merges rely on. 436 added non-test lines (stop 665). One demonstrated defect blocks: the second guard's worktree reset leaves a first guard's planted file in place. The fix is about 3 lines.

## Blocking Issues (Must-Fix Only)

### B1. The second guard's reset leaves a skip-worktree file the first guard planted (10.2) (about 3 lines)

10.2: "Before the second guard runs, dispatch resets the shared worktree to `head_oid` and removes untracked files, so a first guard's leftover probe file cannot reach the second review." `Worktree.reset` (`worktree.go:418`) runs `reset -q --hard <head>` then `clean -q -ffdx`. Neither touches a tracked file whose index entry carries the skip-worktree bit: `reset --hard` keeps the bit and the modified file, and `clean` handles untracked files only.

Probe P2 (`opus-probes_test.go.txt`, `TestOpusSecondGuardReset`): the first guard appends `POISON` to `internal/x/b.go`, runs `git update-index --skip-worktree internal/x/b.go`, and leaves an untracked `probe.txt`. It ends exit 1 (`dirty`, from `probe.txt` only; the poisoned file is hidden from `status`). That is not a host violation, so the worktree is kept and not tainted. The second guard (`--second`) then runs exit 0 with no violations, and `cat internal/x/b.go` shows `ok` followed by `POISON`. The second review reads content that is not `head_oid`. The `--assume-unchanged` variant is cleared by the shipped reset; skip-worktree is not.

Reach: guard settings allow no `git update-index`, but the guard holds `Write` plus the `WAGGLE.md` test commands, and code run by those is not confined (`not_enforced`). A prompt-injected first guard (the builder writes the code it reads) can plant a file that misleads the second guard, which is the scenario the reset exists for.

Fix: in `reset`, remove `<admin>/index` (ignore `ErrNotExist`) before `reset --hard`. Probe P2b (`TestOpusResetFix`) runs the shipped `reset` on a poisoned worktree (content still poisoned, `ls-files -v` shows `S`), then removes the index and runs it again (content back to `head_oid`, flag `H`). Rebuilding the index also drops any other per-entry state a first guard left (assume-unchanged, fsmonitor-valid, untracked cache). Test: `TestGuardSecondResetsWorktree` gains the skip-worktree case.

## Rulings on the builder's deviations

1. **`Validate` drops `--dir` and `--gate` for a guard instead of refusing:** accepted, with a recommendation. The claim is the right source, so nothing is checked wrongly. A queen that passes `--dir` may believe it scoped the guard. While this fix round is open, refusing with exit 2 ("a guard takes its directories and gate from the builder's claim") costs about 3 lines and matches the section 4 synopsis, which lists neither flag for the guard.
2. **`GuardMustFix` across the two guards of a round depends on the queen passing the same `--base`:** not acceptable as an unwritten rule; not a D1 code blocker. Probe P8 shows the failure: `alternates.frontier: fable`, one must-fix review for the phase at base, the builder gets sonnet and the first guard opus. The queen commits the first guard's must-fix review, then dispatches `--second` with the default base. The count is now 2, the re-resolved builder steps up to opus, the re-resolved first guard becomes fable, and the second guard walks down to **sonnet, the model that wrote the code**. Nothing warns. D-100 is broken silently. The comment on `TestMustFixCountPerPhase` says "Both guards of a round read the same base, so they see the same count even after a review is committed". That holds only with an explicit, identical `--base`, and no test exercises guards. Required: correct that comment. Add `must_fix` (the count used) to `result.json`, 1 or 2 lines, so the queen can compare the builder and both guards. The scout writes the rule into 10.2, and the phase F queen template carries it: pin both guards' `--base` to the builder run's `base_oid`, and commit a round's reviews only after both guards ran.
3. **Builder task lines use the requested lease, not the claim read-back:** accepted. `Validate` normalizes with `comb.NormalizeDirectories`, the same normalization the Comb stores, and the checks still use the read-back (`ck.lease = want = claim.Lease`). An equality assertion after `AcquireClaim` would make any future drift loud. Optional.
4. **Guard `claim_verb` is `read`:** accepted. 9.1 lists `claim`, `renew` and `takeover`, so the scout should add `read`. `guardTarget` already calls `ClaimOID` and discards the result. Recording the builder's claim OID in `claim_oid` would tie the review to the claim it read.
5. **`roundMustFix` adds to the old `reviews/T-<nnn>.md` count:** accepted. These are distinct files, so nothing counts twice. P8 counts: `t-023d1` gives 2 (`T-023d1-opus.md` plus `T-023d1-confirm-fable.md` with CRLF), and `T-023d-`, `T-023d10-`, `T-0231-`, a subdirectory file, an indented line and a quoted line give 0. `t-023d10` and `t-023d` each count only their own file. An upper-case suffix (`t-023D1`) counts 0, since names match case-sensitively. A review file over 400 KiB at base makes every dispatch for that phase exit 3 before the claim. That is acceptable.
6. **The guard worktree is removed on exit 0, so the reset runs only after a failed guard:** accepted. Removal gives the next guard a fresh worktree, which is stronger than a reset. The reset runs after exit 1 or 5, or with `--keep-worktree`, which is where B1 bites.
7. **The host record compares keys outside `worktrees/`, and a Keeper's edit makes the next dispatch exit 3 until `host-files.json` is deleted:** accepted for D1, since 10.6 item 1 says any difference is exit 3. It needs a Keeper question before phase F. P6: `git config branch.t-023.remote origin` (what `git push -u` writes), `git remote add` and any global gitconfig edit each make the next dispatch exit 3. This hive's own `.git/config` gained `branch.t-023d1` and `branch.hive-process-1` sections through today's ordinary PR flow. In our hive the record would trip after nearly every records batch and phase PR. The only remedy offered is deleting the record. That trains the queen to delete it on sight, which turns item 1 into noise. Options: ignore `branch.<name>.remote` and `.merge` in the comparison, or have D2's `clean` acknowledge the record and print the diff first.

**The `not_enforced` line for `host-files.json`:** acceptable. Item 1 already accepts that the record is agent-writable. The line is accurate, and P6 confirms both edges. A symlinked record is exit 3, "unreadable". A deleted record is skipped silently, so a planted hook then passes (exit 0).

## D-176 and the exit path today (known open item, not blocking)

`signalGroup` sends SIGTERM or SIGKILL to `-pid` of the worker, the group `Setpgid` created. `stop()` (timeout, cancel, token cap, stream end while running) sends SIGTERM, then SIGKILL after `killGrace` (10 s). The exit goroutine sends SIGKILL to the group right after `cmd.Wait()` returns. The probe shows that Bash tool children lead their own groups, so neither signal reaches them. They die on the SIGTERM path only because Claude Code reaps its own children when it handles SIGTERM. They survive when:

- Claude Code exits on its own while a tool child still runs.
- Claude Code ignores SIGTERM past the grace and gets SIGKILL.
- Claude Code crashes.

`Run`'s promise that "the process group is gone" stays literally true and covers less than it reads. `dispatch.lock` is released while a survivor runs, so the next dispatch can start beside it, and that run's snapshot absorbs whatever the survivor has done so far.

What the post-run check can catch from a survivor:

- Caught: anything written before the files step and the git step, which is every hash, ref, admin entry and status the snapshot covers.
- Caught at the next dispatch, with limits: writes to the hashed host files after the check. The host record catches them only if this run was clean, the survivor wrote after `saveHostRecord` (N2), and it did not rewrite `host-files.json`.
- Caught or erased on reuse: worktree writes. A builder's reuse fails `checkClean` (exit 3). A guard's reuse is reset, except for B1.
- Not caught: ref moves after the check. The push sends the checked OID, but nothing compares refs across runs, so a survivor that moves `refs/heads/main` or a claim ref after the run goes unseen.
- Not caught: main checkout writes after the check. The record holds no status.
- Not caught: writes outside the repo.

On linux, the narrowest fix is `PR_SET_CHILD_SUBREAPER` on dispatch, then killing every remaining descendant after the leader exits. macOS has no subreaper, so there the option is a descendant walk (by `ppid`) while the leader is still alive.

## Carry items (verified)

- 10.6 items 1 to 12, 14, the dispatch half of 19, and 22: present, each with its test. Item 11's third gap, a branch moved at the `log` call, is phase C's `TestFixPushesCheckedOID`, which triggers on the `--reverse` log call. Item 10's comb pin works: with `GIT_CONFIG_PARAMETERS=` removed from `combEnv`, the parent's hook dir fires the canary during a claim write (P9). The test asserts the list first and the behavior second.
- D0 carry: the `cell.go` `not_enforced` wording now names a repo-local `include.path`, not the Keeper's global config (opus N3, fable N3). New lines cover repo-local `core.excludesFile` and `core.attributesFile`, auto maintenance as the tripwire's class (D0b fable N1), `--output=` for guards (fable N6) and `host-files.json`.
- `GuardMustFix` once per round: not done. See ruling 2.
- Guard `HEAD` equals `head_oid`: yes (`check.go:199`). A commit or detach is a `HEAD` violation (PR tests, P1).
- Comb env pin: yes (above). Fixture `shIn`: `-c maintenance.auto=false` added.
- D-176 probe output: `media/T-023/d176-probe.txt` exists in the queen's checkout (FAIL, as briefed). Dispatch never writes `.waggle/hive/`, so the PR does not carry it.

## Checks

- `go build ./...`, `go vet ./...`, gofmt on `cmd` and `internal`, and `go test -race -count=1 ./...` all pass locally (darwin).
- `gh pr checks 57`: `product-ci` passes on `d48deaf` (linux, 1m58s). `verify-leases-and-tiers` fails with "leases: no claim for T-023; claims must stay until merge", which is expected in a local-claims hive (D-119, D-168). `relay-claim` is skipped.
- Lease: all 18 changed files are under `internal/dispatch/` or are `cmd/waggle/dispatch.go` and `cmd/waggle/dispatch_test.go`. `gh pr view 57` lists the same files.
- Size (D-136): non-test additions are 7 (`dispatch.go`) + 6 (`cell.go`) + 21 (`check.go`) + 59 (`guard.go`) + 45 (`inputs.go`) + 27 (`ledger.go`) + 97 (`prerun.go`) + 19 (`request.go`) + 1 (`result.go`) + 123 (`run.go`) + 31 (`worktree.go`) = **436**. That matches the builder (estimate 380, target 530, stop 665). B1 brings it to about 439.
- Commits: both `T-023:` subjects carry `Signed-off-by`. There is no U+2013 or U+2014 in the diff.

## Probes (not committed; source and output in `.waggle/hive/media/T-023d1/opus-probes_test.go.txt` and `opus-probes-output.txt`, run as `internal/dispatch/opus_probe_test.go` and then removed)

- **P1, guard zero change:** caught as `host_state`: stash, notes, tag, `worktree add`, `log --output=` into `hooks/`, `info/`, `refs/heads/` and the main checkout, local config, and a rewritten `.git` file. An untracked file is caught as `dirty`. Commit, detach, branch move and claim move are caught in the PR's own tests. Exit 0 with no violation: commit-then-restore `HEAD` (a dangling object), a skip-worktree or assume-unchanged hidden edit, and `description`. None of these persists past the worktree removal on exit 0 (but see B1 when the guard also fails).
- **P2 and P2b, second guard reset:** B1. A first guard that moves `HEAD` taints the worktree, and the second guard then exits 3 ("host state was changed by run T-023-guard-1").
- **P3, verdict:** `MaxFinal - 1` bytes ending in a verdict parses. Exactly `MaxFinal` gives none. CRLF, a trailing space or indentation gives none (the safe direction). The last matching line wins over an earlier quote.
- **P4, refusals before the claim:** a control-character identity, a guard `Prepare` refusal, a guard without a claim and a host record mismatch each leave no claim ref, worktree, `runs/`, ledger line or new record.
- **P5, identity:** a blank local name falls back to the global one. A local name with a global email mixes the two (N3). A local email planted by an earlier agent before any clean run reaches the next ticket's commits (N1). After a clean run, the same plant makes the next dispatch exit 3.
- **P6, host record:** see ruling 7.
- **P7, ledger:** a symlinked `usage.jsonl` is refused and the target stays untouched, with `ledger_append_failed` as a warning and exit 0. A guard `no_verdict` writes `"role":"guard"` and `"end":"error"`.
- **P8, item 22 and the two-guard round:** see rulings 2 and 5.
- **P9, remote mode:** a remote guard passes, pushes nothing and leaves the origin branch unchanged. A guard that writes another ticket's claim mirror exits 0, under the same post-sync exemption builders have. The comb env mutant is described above.

## Notes (non-blocking)

- **N1. A flagged run's plant reaches other tickets when no clean run preceded it (P5).** The tainted marker blocks one worktree path, and item 1's record exists only after a clean run. A first-ever run that writes `user.email` to `.git/config` fails with `host_state`, but the next dispatch for T-024 commits as `planted@example.test`. Cheap fix: always save the record from the pre-run snapshot, `hostBase(ck.before.files)`, after the checks. On a clean run that equals the current state, and after a host violation it makes the next dispatch of any ticket exit 3. It also closes N2.
- **N2. `saveHostRecord` rehashes after the push and the worktree removal** instead of saving the hashes the check verified. A survivor that writes in that window gets its change recorded as the baseline. The record is agent-writable anyway, so this opens no new hole.
- **N3. Identity keys fall back one at a time.** A local `user.name` with a global `user.email` gives a mixed identity. Taking both values from the scope that sets either would read better.
- **N4. `resolveIdentity`'s global read is unpinned and now precedes `checkHostRecord`.** `git config --global` runs no hook or filter, so this is harmless. The comment "before ResolveBase's fetch, the first unpinned call" (`run.go:210`) is no longer exact.
- **N5. A stdout holder on a normal exit costs the full timeout.** When the leader exits by itself, the cut waits for `tctx` (120 minutes by default) and the run ends `timeout`, exit 5. That matches item 5's text. A short grace after the leader is reaped would end the run sooner.
- **N6. Guard provisioning runs unpinned.** `worktree add --detach` checks out builder-written files under the Keeper's global config, so a global filter such as git-lfs smudge applies to the builder's `.gitattributes`. Phase C's must-fix-round `worktree add <branch>` is in the same class. Pinning both is a small change for a later round.
- **N7. A guard's own skip-worktree edit is invisible to rule 3** (P1). Listing `ls-files -v` entries with `S` or a lower-case tag as `dirty` would close it. After B1 nothing persists, so this is optional.
