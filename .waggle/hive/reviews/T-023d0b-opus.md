<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: T-023 phase D0b (PR #55, `t-023d0b` at `1599c2b`, base `53941e6`), opus guard

Verdict: pass
Risk-Tier: 3

## Intent Summary

Phase D0b closes 10.6 item 23 (D-181). The fixture's worker `git commit` started `git maintenance run --auto --detach` (geometric strategy, default since git 2.54). Its `worktree-prune` task deleted a planted `worktrees/<id>` while that entry had no `gitdir` file yet, so the post-run check found nothing and product-ci went red. The fix turns off auto maintenance and auto gc for every git process dispatch starts or hands an env to. The worker's single `GIT_CONFIG_*` emission (`tripwireEnv`) gains keys 12 and 13 (`maintenance.auto=false`, `gc.auto=0`, COUNT 14). Host-side `Git.RunIn` adds `-c maintenance.auto=false -c gc.auto=0`. The comb's `ExtraEnv` grows from COUNT=1 to COUNT=3 with the same two keys. A new proving test plants the entry after the result line, then makes a second worker commit. A second test pins the `RunIn` flags. Blast radius: the git config of every dispatched worker and every dispatch-side git call. Git will no longer auto-repack or auto-gc the Keeper's repository as a side effect of a dispatch run. The Keeper's own git commands are unaffected. The PR adds 8 non-test lines (stop 20), all inside the D-181 lease.

## Blocking Issues (Must-Fix Only)

None.

## Checks

- **Lease (D-181):** the PR changes `internal/runtime/launch.go`, `internal/runtime/launch_test.go`, `internal/dispatch/worktree.go`, `internal/dispatch/run.go` and the new `internal/dispatch/maintenance_test.go`. `gh pr view 55` lists the same five files. The `run.go` hunk touches only the comb `opts.ExtraEnv` line, as the lease requires.
- **Size (D-136):** `git diff --numstat` over the non-test files gives 2 (`run.go`) + 1 (`worktree.go`) + 5 (`launch.go`: 3 comment lines, 2 code lines) = 8 added lines. That matches the builder's claim (stop 20).
- **Commit:** a single commit, `1599c2b`, with a `T-023: phase D0b:` subject and a `Signed-off-by: paulternate <27632860+paulternate@users.noreply.github.com>` trailer. Author and committer are the same noreply identity. The commit is unsigned (`%G?` = N), like every commit on main. No U+2013 or U+2014 in the changed files.
- **Build, vet, tests:** `go build ./...`, `go vet ./...` and `go test -race ./...` all pass locally (go 1.27.2, Apple git 2.54.0). `go test -race -count=20 -timeout 30m -run 'TestRunHostState|TestRunPlantDuringAutoMaintenance' ./internal/dispatch/` passes (`ok`, 384s). `gh pr checks 55`: `product-ci` passes on linux, and its runner logs `git version 2.55.0`. `verify-leases-and-tiers` fails with "leases: no claim for T-023; claims must stay until merge", as expected.
- **Worker env (probe `opus-env_test.go.txt`):** the parent env sets `GIT_CONFIG_COUNT=99` and `GIT_CONFIG_KEY_12=maintenance.auto` / `GIT_CONFIG_VALUE_12=true`. `workerEnv` emits `GIT_CONFIG_COUNT` exactly once (`=14`), and keys 12 and 13 hold `maintenance.auto=false` and `gc.auto=0`. No parent value survives, because `workerEnv` still never copies `os.Environ()`. The two new keys come after `core.excludesFile` (10) and `core.attributesFile` (11), and the tripwire's push rewrites keep keys 0 to 9. Under the dumped env, in a repo whose local config sets `maintenance.auto=true` and `gc.auto=1`, `git config --get` returns `false` and `0`. `git remote get-url --push` still rewrites `https://`, `git@`, `ssh://`, `/` and `./` URLs to `file:///dev/null/...` (`opus-tripwire.txt`).
- **Fix removal (`opus-mutants.sh`, output in `opus-mutants.txt`):**
  - Removing worker keys 12 and 13 makes `TestRunPlantDuringAutoMaintenance` fail 10 of 10 subtests (5 runs, plain and symlinked).
  - Removing only `maintenance.auto` also fails 10 of 10.
  - Removing only `gc.auto` passes. `maintenance.auto` is the load-bearing key, and `gc.auto=0` is belt and braces. That is correct: in 2.54 the `worktree-prune` task has its own auto condition and does not read `gc.auto`.
  - Removing the worker keys also fails `TestPrepareEnv`.
  - Removing the `RunIn` flags fails `TestRunInDisablesAutoMaintenance` (so that test is meaningful), but the proving test still passes 5 of 5.
  - Setting the comb back to COUNT=1 passes both dispatch tests 5 of 5.

  The sources were restored byte for byte, and `git status` shows only the media directory. The host and comb halves are therefore not pinned by the proving test. See N1.
- **The plant still follows `result`:** `okScenario` ends with `emit(assistant(100), okResult)`. The proving scenario appends the `mkdir`, the second `commitFile`, `sleep 1` and the `gitdir` write after it.
- **Real git, traced (`opus-trace.sh`, output in `opus-trace.txt`):** each variant builds a repo with a bare origin and a linked worktree, then plants `worktrees/planted` with no `gitdir`. It makes three worker commits under the dumped worker env. It also runs the dispatch host calls with `RunIn`'s flags: `fetch`, `worktree prune`, `worktree add`, `status` and `push`. Everything runs under `GIT_TRACE2_EVENT`.
  - PR env and flags: no `maintenance` or `gc` child from any worker or host process in the repo, and the plant survives.
  - Pre-PR env (COUNT=12) and pre-PR flags: 5 `git maintenance run --auto --detach` children plus a `git worktree prune --expire 3.months.ago`, and the plant is PRUNED.
  - PR env and flags, with the repo's local config hostile (`maintenance.auto=true`, `maintenance.strategy=geometric`, `gc.auto=1`, `gc.autoDetach=true`, `fetch.writeCommitGraph=true`, `fetch.prune=true`, `maintenance.worktree-prune.auto=1`, `gc.worktreePruneExpire=now`): the plant survives with no maintenance child. Command-line and env scope beat the local scope.
  - The same hostile config on the pre-PR env prunes the plant, so the hostile config was live.

  The one maintenance child left in the PR variants has the trace2 hierarchy `push/receive-pack/maintenance`. It runs in the bare origin, because git clears `GIT_CONFIG_*` for a local `receive-pack`, and it runs after the post-run check (N3).
- **Comb (`opus-comb-fetch.sh`):** the comb's sync, `git fetch --prune --no-tags --quiet origin +refs/waggle/claims/*:...`, starts one maintenance child under the pre-PR `ExtraEnv` and none under the PR's. On the env merge, `childEnv` appends `ExtraEnv` after the parent env, and `os/exec` keeps the last value of a duplicate key. So COUNT=3 and keys 0 to 2 override any Keeper `GIT_CONFIG_COUNT/KEY_0..2`. A Keeper's keys at 3 or above are ignored, because COUNT=3. The pre-PR COUNT=1 already discarded the Keeper's env config, so this is not a new clobber. The comb adds no `GIT_CONFIG_*` of its own elsewhere, and its per-call env sets only `GIT_AUTHOR_*`/`GIT_COMMITTER_*`. A parent `GIT_CONFIG_PARAMETERS` is not scrubbed by the comb and would still apply. Only the Keeper's own environment sets it (N2).
- **Other git auto paths:** `fetch.writeCommitGraph` writes `objects/info/commit-graph*` only and never touches `worktrees/`. `fetch.prune` deletes remote-tracking refs, never admin entries. `gc.autoDetach` and `maintenance.strategy` do nothing when `maintenance.auto=false`. The hostile trace variant sets all of them and starts no maintenance child. `receive.autogc` acts only in the receiving repo (N3). `git am`, `merge`, `rebase`, `commit` and `fetch` all go through `run_auto_maintenance`, which the one key gates. Git 2.56 is not available here. The checks used Apple git 2.54 locally and linux git 2.55 in CI.

### Every dispatch-time git call site

| Call site | Path | Auto-maintains without fix? | Covered |
|---|---|---|---|
| Worker git (agent shell, fake `claude` commits) | `runtime.workerEnv` → `tripwireEnv` keys 12, 13 | yes (`commit`, `fetch`, `merge`, `rebase`, `am`) | yes, unless the worker shell resets `GIT_CONFIG_*` (existing residual class) |
| Input fetch before the run | `inputs.go:35` `g.Run("fetch")` → `RunIn` | yes | yes |
| Provision: `worktree prune`, `worktree add` x3, `rev-parse`, `merge-base`, `worktree list`, reuse `status` | `worktree.go:201..426` → `RunIn` | no (prune is explicit and runs before the snapshot) | yes |
| Claim checks, `merge-base`, `rev-parse --git-common-dir` | `claim.go:75`, `run.go:112,116,243` → `RunIn` | no | yes |
| Snapshot and post-run check: `for-each-ref`, `status`, `rev-parse`, `log`, `diff-tree` | `check.go:54..264` → `RunIn` | no | yes |
| Push after the check | `run.go:310` `g.Run("push")` → `RunIn` | local: no. Remote receive-pack: yes, in the remote (N3) | local yes, remote no |
| Comb claim, keepalive `AcquireLease`, post-run `ClaimOID` sync: `fetch --prune`, `push`, `update-ref`, `hash-object`, `mktree`, `commit-tree`, `cat-file`, `for-each-ref` | `comb/git.go:226` `b.git` with `ExtraEnv` | yes (`fetch`) | yes |
| `git --version` probe | `run.go:363` direct `exec` | no | n/a |
| `cmd/waggle/gate.go`, `status.go`, `internal/forge`, `internal/scaffold` | direct `exec` | n/a (not inside a dispatch run) | out of scope |

Every host-side git call in `internal/dispatch/` goes through `Git.RunIn` except the `--version` probe. Every comb call goes through `b.git`.

- **`prune_dups` (`opus-prunedups.sh`, `opus-prunedups.txt`, `opus-prunedups-inrun.txt`):** with real git, a planted `worktrees/a` whose `gitdir` names the run's worktree causes `git worktree prune` to delete the run's own `worktrees/t-023` ("duplicate entry"). git keeps the entry with the lower id. Afterwards `git status` in the run's worktree fails. A plant with id `zz` is the one deleted instead. In dispatch the run's id is `T-023-builder`, and uppercase sorts first, so the agent needs an id such as `0` or `A`. In-run probe (`TestOpusPruneDups`): the agent plants `worktrees/0` and runs `git worktree prune` itself after `result`. The run exits 1 with `host_state` violations for both `worktrees/0` and `worktrees/T-023-builder`, plus "worktree admin dir pin lost", "worktree HEAD no longer names the branch" and "not inspected: host config changed". This is loud. With the default spelling `a`, git deletes the plant itself, the run's entry is untouched, and nothing changed. The only host-side `worktree prune` (Provision) runs before the snapshot. After D0b, no host or comb git process prunes during the snapshot-to-check window. A plant left by an earlier run is already a `host_state` violation that taints that worktree. Its `gitdir` can only name an existing worktree path, and the next Provision prune would hit that tainted worktree, not a fresh one. Not in scope for D0b. See N4 for the phase D brief.

## Notes (non-blocking)

- **N1. The host and comb halves have no test that fails without them.** `TestRunInDisablesAutoMaintenance` pins the `RunIn` argv, but nothing pins the comb `ExtraEnv` keys. Setting the comb back to COUNT=1 keeps every test green. Both halves still matter. The post-run `ClaimOID` sync and the keepalive renew run `git fetch` between the snapshot and the check in remote mode. The pre-run input fetch can leave a detached maintenance running into the session. My probe `TestOpusPlantNoGitdir` leaves a plant with no `gitdir` and exits. It passed 10 of 10 under every mutant (comb, host, worker, host plus comb) (`opus-mutants2.txt`), because detached maintenance on the fixture repo reaches `worktree-prune` after the check has finished. That race is real but not reproducible in the fixture. A cheap pin for a later phase that already leases `run.go` or `comb` tests is a unit assertion that the comb options carry `maintenance.auto=false`. Lifting the `ExtraEnv` literal into a named var would make that one line.
- **N2. `GIT_CONFIG_PARAMETERS` from the Keeper's env reaches the comb.** `RunIn` scrubs it (`gitEnv`), but comb `childEnv` keeps it, and git applies it after the `GIT_CONFIG_COUNT` entries. A Keeper who runs `waggle dispatch` from inside `git -c maintenance.auto=true <alias>` would turn comb auto maintenance back on. Only the Keeper sets that env, so this is the Keeper's own choice. Recorded so phase D does not assume the comb is sealed.
- **N3. A local push starts maintenance in the remote.** `push` (`run.go:310`, after the check) runs `receive-pack` in the origin under the origin's own config. Git clears `GIT_CONFIG_*` and `GIT_CONFIG_PARAMETERS` for local transports, so `-c` cannot reach it. The trace shows `push/receive-pack/maintenance` in the bare origin. It runs after the check and touches the remote repository, never the host common dir, so item 23 is unaffected. A Keeper whose `origin` is another local non-bare clone would see that clone auto-maintained. This is ordinary git behavior.
- **N4. For the phase D brief.** `waggle dispatch clean` should restore or refuse when a stale `worktrees/<id>` duplicates a live run's `gitdir`. Otherwise the next `git worktree prune` (Provision's or the Keeper's) silently drops whichever registration has the higher id. The run's own `T-023-builder` sorts after `0` to `9` and `A` to `S`. Today that state only arises after a run that was already flagged.
- **N5. A Keeper's scheduled maintenance is not covered.** `git maintenance start` (launchd, cron or systemd) runs outside any env dispatch controls and can run during a session. I did not verify which tasks the scheduled runs include on 2.54+. If a scheduled run reaches `worktree-prune`, it can remove an agent's incomplete plant, the same miss as item 23, but only by timing the Keeper controls and the agent cannot trigger. Worth one line in the operator docs ("unregister scheduled maintenance for a hive repo, or accept the window"). No D0b action.
- **N6. git 2.56 was not exercised.** `maintenance.auto` has gated `run_auto_maintenance` since 2.29, and the mechanism did not change in 2.55 (CI green). If a later git adds an auto path that does not read `maintenance.auto`, the proving test fails loudly on the CI runner. It cannot pass silently.

## Probe files

- `media/T-023d0b/opus-mutants.sh`, `opus-mutants.txt`: the fix-removal check.
- `media/T-023d0b/opus-mutants2.sh`, `opus-mutants2.txt`: host and comb mutants against `TestOpusPlantNoGitdir`.
- `media/T-023d0b/opus-env_test.go.txt`: the worker env dump, run as `internal/runtime/opus_probe_test.go` and then removed.
- `media/T-023d0b/opus-prunedups-inrun_test.go.txt`, `opus-prunedups-inrun.txt`: `TestOpusPruneDups` and `TestOpusPlantNoGitdir`, run as `internal/dispatch/opus_probe_test.go` and then removed.
- `media/T-023d0b/opus-trace.sh`, `opus-trace.txt`, `opus-comb-fetch.sh`: real git under trace2.
- `media/T-023d0b/opus-prunedups.sh`, `opus-prunedups.txt`: real git `prune_dups`.
- `media/T-023d0b/opus-tripwire.sh`, `opus-tripwire.txt`: the push tripwire and the effective keys under the 14-key env.
