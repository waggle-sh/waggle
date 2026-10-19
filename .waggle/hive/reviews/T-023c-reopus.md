<!-- SPDX-License-Identifier: MIT-0 -->
# Guard re-review: T-023 phase C (PR #50, `t-023c` at `22ea50a`, base `23bdf75`), first guard (opus)

Verdict: must-fix
Risk-Tier: 3

## Intent Summary

The fix round (`22ea50a`) closes the eleven merged first-round items. The push now sends the checked OID (`<oid>:refs/heads/<branch>`), and that one OID is also the log range and `head_oid`. The remote-mode final claim sync is gated on the host-file comparison, and the claim-mirror exemption applies only after a sync that ran. The dispatch-built Comb backend runs with `core.hooksPath=/dev/null` through `ExtraEnv`. Keepalive renewals are gated on the host files before any git call. The XDG `git/ignore` and `git/attributes` files are hashed. `RunIn` pins `log.showSignature=false`, the three status calls pass `--ignore-submodules=dirty`, `result.json` and `final.md` are written through `CreateTemp` plus rename, `TimedOut` keys on the deadline alone, the test dash literals are escapes, and four `not_enforced` lines were added. Every first-round probe I re-ran no longer fires, and 54 timed or toggling push races pushed nothing unchecked. Two new blockers remain, both small. A worker session that writes about a thousand objects makes git's own detached auto maintenance repack the shared object store and write `info/refs`. Dispatch then fails a clean run as `host_state` and taints the worktree. The second blocker: one new self-ignoring `.gitignore` hides any new file in the main checkout, and the new `not_enforced` line understates that. Blast radius is unchanged from round 1: every future agent launch, with the Keeper's environment and push auth on the host side. Size: 1223 added non-test lines; the fixes below add about 1.

## First-round blockers

Opus round 1:

- **B1 (push sends the branch by name): fixed.** `check.go` resolves `refs/heads/<branch>` once with `rev-parse --verify` and uses the OID for the log range. `run.go` sets `HeadOID` from it and pushes `ck.head+":"+ref`, with no `+`. Probe R-P6, the timed move after dispatch's worktree status, ran 48 times over delays from 0 to 100 ms. The branch moved in every run, and every run pushed the checked OID (0 of 48 bad; it was 2 of 6 before). The toggling probe R-P5 gave 0 of 6 bad (all 6 caught, exit 1).
- **B2a (final sync runs planted config): fixed.** The files comparison runs before `ClaimOID`. On a change, the sync is skipped with `claim_sync_skipped`, and `synced` stays false, so claim refs compare strictly. After the session, no git call runs before the files step. The order is `writeFile`, then `hostFiles`, then a `ClaimOID` gated on unchanged files, then `ck.run`, whose first step is `hostFiles`. R-P2 (planted `remote.origin.uploadpack`): the canary did not fire.
- **B2b (local keepalive runs a planted hook): fixed.** The hook is off through `ExtraEnv`, and the renewal is gated before `AcquireLease`. In R-P1, the planted `reference-transaction` hook fired only from the worker's own commit, with no `GITHUB_TOKEN` in its env. It never fired from a host renewal (the canary saw no secret; in round 1 it did).
- **B2c (remote keepalive under planted config): fixed.** Renewals are skipped once the host files differ, with a `not_enforced` line for the written-and-reverted residual. R-P1b (planted uploadpack, 200 ms renewals): the canary did not fire.
- **B3 (XDG ignore hides writes): fixed.** Both XDG files are hashed, with the `~/.config` fallback. R-P7 now exits 1 with `host_state global:.../git/ignore`. The two `not_enforced` lines are present. See R1 for the gitignore wording.
- **B4 (literal dashes): fixed.** `cmd/waggle/dispatch_test.go` uses the Go escapes for U+2013 and U+2014. The PR diff and the commit messages have no U+2013 or U+2014.

Fable round 1 (merged into the same eleven items):

- **B1 (push re-resolves the ref): fixed**, as above.
- **B2 (pre-check claim sync): fixed**, as above.
- **B3 (`log.showSignature` runs `gpg.program`): fixed.** It is pinned in `RunIn`. `TestFixShowSignature` fails when the flag is removed.
- **B4 (status enters a staged nested repo): fixed** on all three status calls. `TestFixNestedRepoFilter` fails when the flag is removed from the worktree status call.

The non-blocking items folded into the round are also done: atomic result writes (opus N2), `TimedOut` (opus N7), `include.path` (opus N11) and the renewal residual (fable N6).

## Blocking Issues

### R1. A self-ignoring `.gitignore` hides any new file in the main checkout, and the new `not_enforced` line understates it (rule 3) (0 added lines)

Probe: the worker runs `mkdir -p <main>/.claude/commands`, writes `*` to `<main>/.claude/commands/.gitignore`, and writes `<main>/.claude/commands/evil.md`. The result is exit 0, `pushed: true`, zero violations. The `.gitignore` ignores itself and every file next to it, so the main checkout status never changes. This is the same class as round-1 B3: one agent write hides every main-checkout write. A reader of the new line, "writes to gitignored paths in the main checkout (.claude/settings.local.json, .waggle/control/local.yaml) are not detected", would take it to mean paths that are already ignored. The damage ceiling equals what is already disclosed (`settings.local.json` gives the queen's next session hooks), so this needs honest wording, not new code.

Fix: change that `not_enforced` entry to say what holds. For example: "new or changed files in the main checkout that a .gitignore covers are not detected, including a .gitignore the agent writes there (.claude/settings.local.json, .waggle/control/local.yaml, a self-ignoring directory)". The `len(res.NotEnforced) != 11` assertion is unchanged. Detecting it is optional and costs more: `status --ignored=matching` would catch it, but it would also flag ignored files that the Keeper's own tools write during a run.

### R2. Git's own auto maintenance writes `info/refs` during a clean run, which fails as `host_state` and taints the worktree (about 1 line plus a test)

Every `git commit` and `git fetch` starts `git maintenance run --auto --detach`. On git 2.54 (the Keeper's local git), a few hundred new files are enough to trigger a repack. The repack runs `update-server-info`, which writes `<common>/info/refs`. `hostFiles` walks all of `info/`, so the run gets `host_state info/refs changed during the run`. That key starts with `info/`, so `inspect` is turned off, and the result is exit 1 and a tainted worktree. Evidence:

- Probe R-AM: a worker that commits 1200 files inside its lease and then sleeps 3 s gives exit 1 with `host_state info/refs`. A worker that runs `git update-server-info` gives the same result.
- A standalone repo with 450 new files gets `info/refs` and 4 pack files within 1.5 s of the commit, from the detached maintenance.
- The flake already appears in the suite. One of about ten full `./internal/dispatch` runs failed `TestRunBaseAndBranch/remote_tracking_only` with exactly `{host_state info/refs changed during the run}`. A detached maintenance started by the fixture's setup commits finished inside the session window. `product-ci` can fail the same way.

The same maintenance also starts after dispatch's own claim fetches, and once `info/refs` changes, the fix 4 gate stops every later renewal. So a long session can also lose its claim.

Fix: leave `info/refs` out of the walk. Git never reads that file locally; only dumb-HTTP servers serve it. In `hostFiles`:

```go
if rel, rerr := filepath.Rel(common, p); err == nil && rerr == nil && !e.IsDir() && filepath.ToSlash(rel) != "info/refs" {
```

I applied this locally (not committed). Both R-AM cases then exit 0 with no violations, and `go test ./internal/dispatch/` passes. Regression test: a scenario that runs `git update-server-info` before `okScenario` must give exit 0 (about 6 test lines). Everything else maintenance touches is either unhashed (`objects/pack`, `objects/info/packs`, `commit-graph`, `gc.log`) or keeps the same ref values (`pack-refs`).

## Probes of the fixes themselves

- **(8) Temp name and symlinked run dir.** The agent cannot pre-create the temp name: `os.CreateTemp` uses a random suffix with `O_EXCL` and retries on a collision. A planted symlink or FIFO at `result.json` or `final.md` is replaced by the rename. If the agent replaces the run dir itself with a symlink, `result.json` and `final.md` land in the target directory (probe: both found there, exit 0). The worker can already write anywhere in HOME ("writes outside the repo"), so this adds no capability. Note only (N3).
- **(2) Git before the files step:** none (see B2a above). One window remains: an escaped process can still change config between the gate's `hostFiles` and the sync's `fetch`. The setsid `not_enforced` line covers it.
- **(3) Coverage and env merge.** Every Comb call runs through the one `GitBackend.git` exec site, and `childEnv` appends `ExtraEnv` to it. That covers fetch, push, update-ref, mktree, hash-object and cat-file. `AcquireClaim`'s own calls go through `RunIn`, which already sets `-c core.hooksPath=/dev/null`. A Keeper `GIT_CONFIG_COUNT` merges safely: `os/exec` keeps the last duplicate key, so `COUNT=1` and `KEY_0` win. The Keeper's own `KEY_1` and later entries are silently dropped, which is fail-safe for hooks but could drop a Keeper-set credential or `insteadOf` entry in remote mode. A Keeper `GIT_CONFIG_PARAMETERS` naming `core.hooksPath` overrides `ExtraEnv`, because git reads it after `GIT_CONFIG_COUNT` (probe: the Keeper-path hook ran). That env is the Keeper's own and not reachable by the agent. Note only (N2).
- **(4) Gate placement.** The gate is the first statement of `renew`, before `backend.AcquireLease`, so no git call runs on changed files. A renewal that has already passed the gate can race a config write. That is the disclosed residual.

## Notes (non-blocking)

- **N1. Test gaps found by mutation.** Every fix's regression test kills its own mutation, with two exceptions. (a) Reverting the log range to `..refs/heads/<branch>` (keeping the OID push) survives, because `TestFixPushesCheckedOID` moves the branch at `diff-tree`, after `log`, and to a clean commit. Moving it at the `log` call to an out-of-lease commit would kill that mutation. (b) Dropping `c.synced` from the claim-ref exemption survives: no test covers a remote run whose sync failed or was skipped while another ticket's claim mirror changed. Both are worth about 10 test lines when convenient.
- **N2.** The comb env drops the Keeper's `GIT_CONFIG_KEY_1` and later entries, and the Keeper's `GIT_CONFIG_PARAMETERS` can re-enable hooks (see (3)). A comment on the `ExtraEnv` line would do. A strict fix would scrub `GIT_CONFIG_PARAMETERS` and append to the Keeper's count rather than replace it.
- **N3.** The `.tainted` marker still uses `os.WriteFile`. If the agent plants a FIFO at `<worktree>.tainted` and then triggers any host violation, `Run` blocks in `open` indefinitely and holds `dispatch.lock` (probe: still blocked after 15 s, and it returned only when a reader opened the FIFO). Liveness only, the same class as round-1 N4. Using `writeFile(filepath.Dir(w.Path), filepath.Base(w.Path)+".tainted", ...)` fixes it with a one-line change. A symlinked run dir (see (8)) is the same class.
- **N4.** `hostFiles` does not hash the system gitconfig. The Comb backend and the final push (`g.Run`, unpinned) read it. On Linux and with Apple git it is root-owned. With a Homebrew git, `$(brew --prefix)/etc/gitconfig` is user-writable, and a planted `core.sshCommand` there would run at the push with the Keeper's env. `~/.ssh/config` (`ProxyCommand`, `Match exec`) is the same reach. Both are covered in spirit by "writes outside the repo (not detected)", but a `not_enforced` clause naming the system gitconfig and `~/.ssh/config` would be more honest.
- **N5.** `TimedOut` keyed on the deadline alone also marks a session that ended on its own just before the deadline, if `Collect` returned after it. That case reads as a timeout (exit 5, no push). It is conservative and rare; no change needed.
- **N6.** In remote mode, when the final sync fails (network), claim refs compare strictly. Claim mirrors that the session's own renewal fetches updated for other tickets then read as `host_state` and taint the worktree. That fails closed and is acceptable, but `claim_sync_failed` plus a taint can surprise a Keeper on a flaky network.

## Checks

- `go build ./...`, `go vet ./...`, gofmt on `cmd` and `internal`, and `go test -race ./...`: pass locally (darwin, git 2.54). `go test -count=5 ./internal/dispatch/`: pass. One earlier full run hit the R2 flake.
- `product-ci` on PR #50 at `22ea50a`: pass (ubuntu, 1m37s). `verify-leases-and-tiers` fails with the expected local-claims gap (D-119, D-168), not a phase C defect.
- Lease: all 21 changed files are under `cmd/waggle/`, `internal/config/local*.go` and `internal/dispatch/`. `go.mod` is unchanged, and only the stdlib is used.
- Size: 1223 added non-test lines against origin/main, matching the claim. About 127 remain under the D-169 stop, and R1 plus R2 need about 1.
- Sign-off: all four commits carry `Signed-off-by` matching the author and the `T-023:` prefix.
- Dashes: none in the PR diff or the commit messages.
- Mutation tests (each applied alone, then reverted). Killed, by the named test:
  - push by ref: `TestFixPushesCheckedOID`
  - ungated sync: `TestFixClaimSyncSkipped`
  - no `ExtraEnv`: `TestFixCombRunsNoHooks`
  - ungated renew: `TestFixKeepaliveGate`
  - no XDG files: `TestFixGlobalIgnore`
  - no `showSignature`: `TestFixShowSignature`
  - no `--ignore-submodules` on the worktree status: `TestFixNestedRepoFilter`
  - `os.WriteFile` for `result.json` and for `final.md`: `TestFixResultSymlink`
  - `TimedOut` with `EndInterrupted`: `TestFixTimeoutWithErrorResult`

  Survived: the log range by ref and the dropped `c.synced` (N1), and the missing flag on the main-checkout and reuse status calls. The last is expected, since no fixture has a nested repo in the main checkout or in a reused worktree.
- Probes (adapted from the round-1 media files, never committed). Source is in `.waggle/hive/media/T-023c/reopus-probes_test.go.txt` and output in `reopus-probes-output.txt`, both in this worktree.
  - R-P1 (local keepalive, planted hook, never fires from the host): fixed.
  - R-P1b (remote keepalive, planted uploadpack): fixed.
  - R-P2 (final sync, planted uploadpack): fixed.
  - R-P5 (toggle race): 0 of 6 bad.
  - R-P6 (timed race): 0 of 48 bad.
  - R-P7 (XDG ignore): fixed.
  - Self-ignoring `.gitignore`: hidden (R1).
  - R-AM (auto maintenance): false positive (R2).
  - Tainted FIFO: blocks (N3).
  - Symlinked run dir: redirected (N3).
  - `GIT_CONFIG_PARAMETERS` precedence: the Keeper env wins (N2).
