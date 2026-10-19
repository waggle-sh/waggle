<!-- SPDX-License-Identifier: MIT-0 -->
# Guard confirmation: T-023 phase C (PR #50, `t-023c` at `6d7b613`, base `23bdf75`), first guard (opus)

Verdict: pass
Risk-Tier: 3

## Intent Summary

The second fix round (`6d7b613`, 4 files, 2 non-test lines changed) closes both re-review blockers and the two test gaps. `hostFiles` now skips exactly `info/refs`, so git's own detached auto maintenance no longer fails a clean run as `host_state`. The gitignored-writes `not_enforced` entry now says that a `.gitignore` the agent writes in the main checkout is also undetected. The `.tainted` marker goes through `writeFile` (temp plus rename) into the worktrees dir. The outside-the-repo `not_enforced` entry names the system gitconfig and `~/.ssh/config`. `TestFixPushesCheckedOID` now moves the branch at the `log` call to an out-of-lease commit. The new `TestFixSkippedSyncIsStrict` covers the claim-ref exemption when the sync is skipped. Blast radius is unchanged: every future agent launch through `waggle dispatch`, with the Keeper's environment and push auth on the host side. Size: 1223 added non-test lines against origin/main, under the D-169 stop of 1350.

## Re-review blockers

- **R1 (self-ignoring `.gitignore` understated): fixed.** The probe still exits 0 with `pushed: true` and no violations, as expected. The new entry reads "new or changed files in the main checkout that a .gitignore covers are not detected, including a .gitignore the agent writes there (.claude/settings.local.json, .waggle/control/local.yaml, a self-ignoring directory)". That matches what the probe shows. The count stays 11.
- **R2 (auto maintenance writes `info/refs`): fixed.** Probe R-AM (1200 files committed inside the lease, then a 3 s sleep) now gives exit 0 with no violations; it gave `host_state info/refs` before. A worker that runs `git update-server-info` also gives exit 0. `TestFixInfoRefsIsNotHostState` fails when the skip is removed (`host_state info/refs changed during the run`).

## The `info/refs` skip (check e)

The compare is `filepath.ToSlash(rel) != "info/refs"` on the path relative to the common dir, so it matches one exact path. Probes, each a fresh fixture with the write before `okScenario`:

- `info/Refs` (case variant, created fresh): exit 1, `host_state info/Refs`.
- `info/refs` replaced by a directory holding `x`: exit 1, `host_state info/refs/x`. The walk still descends into it, and only the exact file entry is skipped.
- `info/refs.lock`: exit 1. `info/refs ` (trailing space): exit 1.
- `info/exclude` appended: exit 1, `host_state info/exclude`.
- `info/refs` with planted content: exit 0. `info/refs` as a symlink: exit 0. Both are expected (see N1).

On a case-insensitive volume, a write to `info/Refs` when `info/refs` already exists lands in the same file and keeps the stored name, so it is skipped. That is the same file, so it adds nothing.

Nothing git runs locally reads `info/refs` for behavior. Only `git http-backend` and dumb-HTTP clients use it, and those read the serving side's copy. `update-server-info` reads the old file only to compare, and it writes through a lock file and rename, so a planted symlink is replaced, not followed. The neighbours git does read locally (`info/exclude`, `info/attributes`, `info/sparse-checkout`, `info/grafts`) are all still hashed. `objects/info/alternates` lives outside the walked `info/` dir and is unaffected by this change.

## Mutations (check a)

Each was applied alone in a scratch copy of `6d7b613` and then reverted.

- **`.tainted` back to `os.WriteFile(w.Path+".tainted", ...)`: killed** by `TestFixTaintedMarkerFIFO` (the Run blocks, and the test fails at its 15 s limit).
- **`c.synced` dropped from the claim-ref exemption: killed** by `TestFixSkippedSyncIsStrict` (the violations lack `refs/waggle/claims/ticket-24`).
- **Log range back to `<base>..refs/heads/<branch>`: killed** by `TestFixPushesCheckedOID` (`outside_lease docs/late.md`).
- Sanity: **`info/refs` skip removed: killed** by `TestFixInfoRefsIsNotHostState`.

So both N1 survivors from the re-review are now killed.

## Other fixes checked

- **Tainted marker.** `dir` is the worktrees dir from `worktreesDir`, and `w.Path` is `filepath.Join(dir, <ticket>-<role>)` in `Provision`. So `writeFile(dir, base+".tainted")` writes the same path that `taintedErr` reads. A FIFO left at the marker with no host violation makes the next dispatch fail closed: `readUpTo` lstats it and returns "not a regular file", which surfaces as `ExitEnv`.
- **`not_enforced` wording.** The system gitconfig and `~/.ssh/config` entry is honest, and the count is unchanged.

## Blocking Issues

None.

## Notes (non-blocking)

- **N1.** The agent can plant `info/refs` content or a symlink there, and it is no longer detected. Nothing local reads it. If a Keeper served this repo over dumb HTTP, clients would see the planted ref list, but they verify object hashes, and the next maintenance run rewrites the file. No change needed.
- **N2.** Still open from the re-review, and none blocks: a Keeper's own `GIT_CONFIG_PARAMETERS` beats `ExtraEnv`, and the Keeper's `GIT_CONFIG_KEY_1` and later entries are dropped (re-review N2). `TimedOut` is conservative near the deadline (N5). A failed remote sync taints the worktree on a flaky network (N6). The `-race` suite does not cover the failed-sync path for `c.synced`, but it uses the same flag as the skipped path, which is now tested.
- **N3.** When `TestFixTaintedMarkerFIFO` fails, it leaves a blocked goroutine behind. That only happens when it is already failing, so it is fine.

## Checks

- `go build ./...`, `go vet ./...`, `gofmt -l ./cmd ./internal`: clean.
- `go test -race -count=1 ./...`: pass (darwin, git 2.54; dispatch 71.7 s). `go test -race -count=3 ./internal/dispatch/`: pass (213.9 s). No `info/refs` flake was seen.
- `product-ci` at `6d7b613` (run 39287361387, head SHA confirmed): pass, 1m35s. `verify-leases-and-tiers` fails with "leases: no claim for T-023; claims must stay until merge", the expected local-claims gap (D-119, D-168), not a phase C defect. `relay-claim` was skipped.
- Size (D-136): 1223 added non-test lines against origin/main (`23bdf75`, which is the merge base), the same as the builder's figure. The non-test files are `cmd/waggle/dispatch.go` 97, `cmd/waggle/main.go` 5, `internal/config/local.go` 8, `cell.go` 66, `cell_other.go` 10, `cell_unix.go` 24, `check.go` 282, `claim.go` 17, `inputs.go` 1, `result.go` 94, `run.go` 497 and `worktree.go` 122. That leaves 127 under the 1350 stop.
- Lease: all 21 changed files are under `cmd/waggle/`, `internal/config/` (`local.go`, `local_test.go`) and `internal/dispatch/`. `go.mod` is unchanged.
- Sign-off: all five commits on the branch (`e6e914b`, `0c6ca29`, `05343b4`, `22ea50a`, `6d7b613`) carry `Signed-off-by` matching the author and the `T-023:` prefix.
- Dashes: no U+2013 or U+2014 in the PR diff or the commit messages.
- Probes ran in a scratch copy and were never committed.

## Intent Packet (merge commit, all of phase C)

T-023 phase C adds `waggle dispatch`'s run path: claim, worktree, a headless agent session in a host Cell, post-run checks, push, and `result.json`.

- **Session.** It provisions or reuses the ticket's worktree on its branch. It launches the agent runtime with fixed argv and the allowlisted `default_args`. It keeps the claim alive with renewals that are gated on the host files and run with hooks off. On the deadline it kills the worker's process group.
- **Checks.** After the session it runs the checks in a fixed order. The host files are hashed first: repo and XDG git config, hooks, `info/` (except `info/refs`) and refs. Then come the final claim sync (remote mode, and only when the host files are unchanged), main checkout state, worktree cleanliness, and the commit range from the check base to the branch OID resolved once (500 commit limit, sign-off and ticket prefix, paths inside the leased dirs). If none of these fail, it pushes that exact OID as `<oid>:refs/heads/<branch>`, without a force.
- **Failures.** A host-state violation turns off the later inspection and writes a `.tainted` marker, which blocks further runs until `waggle dispatch clean`.
- **Outputs.** `result.json` and `final.md` are written through a temp file and rename. `result.json` lists eleven `not_enforced` limits, including undetected gitignored writes in the main checkout, writes outside the repo (system gitconfig, `~/.ssh/config`), processes that escape the process group, and the best-effort push tripwire.
- **Scope.** Blast radius: every future agent launch, with the Keeper's environment and push credentials on the host side. 1223 added non-test lines in `cmd/waggle/`, `internal/config/` and `internal/dispatch/`, stdlib only.
- **Review.** Reviewed in three rounds by two guards on different models; this confirmation passes at `6d7b613`.
