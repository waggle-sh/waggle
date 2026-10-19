<!-- SPDX-License-Identifier: MIT-0 -->
# Guard confirmation review: T-023 phase C (PR #50), fable, second fix round

Verdict: pass
Risk-Tier: 3

PR #50, branch `t-023c`, head `6d7b613` (second fix commit on top of `22ea50a`), base `23bdf75` (`origin/main`). Second independent guard (fable); no coordination with the other model. Reviewed `git diff 22ea50a 6d7b613` in full (4 files, +48 -6, of which 4 non-test lines changed and 0 added net) and re-ran the full checks against `origin/main`.

## Intent Summary

This round closes the two blockers from the opus re-review at `22ea50a` and the three test gaps both re-reviews noted. R2 (git's detached auto maintenance rewrites `<common>/info/refs` and failed a clean run as `host_state`, tainting the worktree) is fixed by leaving exactly the `info/refs` entry out of the `hostFiles` walk; git never reads that file locally, only dumb-HTTP servers serve it. R1 (a self-ignoring `.gitignore` hides new files in the main checkout) is fixed as wording: the `not_enforced` line now says that any `.gitignore`-covered new or changed file in the main checkout is undetected, including a `.gitignore` the agent writes. The `.tainted` marker now goes through the same `CreateTemp` plus rename path as `result.json`, so a FIFO planted at the marker cannot block `Run` while it holds `dispatch.lock`, and the next run refuses with the taint message instead of blocking. The "writes outside the repo" line now names the system gitconfig (user-writable under Homebrew git) and `~/.ssh/config` as inputs the host push reads. Four tests were added or strengthened and each one kills its mutation. Net size is unchanged at 1223 added non-test lines against `origin/main` (127 under the D-169 stop of 1350). Every changed file is inside the phase C lease, the commit is signed off with the `T-023:` prefix, there is no en or em dash in the diff or the commit messages, and `go build`, `go vet`, `go test -race ./...` and five repeats of the dispatch package pass locally (darwin, git 2.54). Linux `product-ci` is green at `6d7b613`.

Blast radius: `internal/dispatch/` only, one hashed key removed from the host-state set and one marker write made atomic. No config, lease, scaffold, Comb or CLI surface change. The git-call table from my re-review at `22ea50a` is unchanged: this round adds no git call and moves none.

## Blockers from the re-review at `22ea50a`: fixed or not

- **Opus R2, `info/refs` false positive: fixed.** `check.go` line 96 skips the entry whose common-relative slash path equals `info/refs`. `TestFixInfoRefsIsNotHostState` runs `git update-server-info` before and after the ok scenario and expects exit 0 with no violations. My probe P2 drives the real thing inside a session, in local and remote mode: `git gc -q`, `git repack -adq`, `git pack-refs --all`, `git update-server-info`, `git maintenance run --quiet`, then a second in-lease commit and another `gc`. Both modes exit 0 with zero violations and no `claim_` warning, and the remote-mode run pushes. The other files maintenance touches are either unhashed (`objects/pack`, `objects/info/packs`, commit graphs, `gc.log`) or keep the same ref values (`packed-refs`, compared through `for-each-ref`).
- **Opus R1, self-ignoring `.gitignore` wording: fixed.** `cell.go` line 43 now reads as the re-review asked. Probe P5 confirms the behavior the line describes: a worker that writes `*` to a new `.claude/commands/.gitignore` in the main checkout plus a file next to it still gets exit 0, which is the disclosed and accepted ceiling (same as `settings.local.json`). The 11-entry assertion in `run_test.go` still holds.
- **Opus N3 / fable round-one N4 class, tainted marker FIFO: fixed.** `run.go` line 302 writes through `writeFile(dir, filepath.Base(w.Path)+".tainted", ...)`, where `dir` is `worktreesDir(req.RepoDir)` and `w.Path` is `filepath.Join(dir, ticket-role)`, so the target path is identical to before. `TestFixTaintedMarkerFIFO` plants a FIFO at the marker, triggers a host violation and requires `Run` to return within 15 s with exit 1. Probe P6 adds that the marker is a regular file afterwards holding the run id, and that the next `Run` refuses with "host state was changed by run" rather than blocking (`readSmall` lstat-rejects non-regular files, so even a FIFO left there could not block the next run).
- **Opus N4, system gitconfig and `~/.ssh/config`: fixed as wording.** `cell.go` line 41.
- **Opus N1 (a) and fable N3, test gaps: fixed.** See mutation results below. The main-checkout and reuse-path `--ignore-submodules` mutations from my re-review remain uncovered by a test; that was a note, not a blocker, and R-E2 showed the code is right.

## (d) Is the `info/refs` skip exact

Yes. The comparison is string equality on `filepath.ToSlash(rel)` where `rel` comes from `filepath.Rel(common, p)` inside `WalkDir`, which never yields `..` and starts at `<common>/info` and `<common>/hooks`. Probe P1 at the `hostFiles` level and probes P3 and P4 through `Run`:

- `info/refs` as a directory holding `hide`: the directory itself is never hashed (directories never are), and `info/refs/hide` is hashed; through `Run` it is `host_state info/refs/hide`, exit 1. The agent cannot hide a file under that name.
- `info/refs.bak`, `info/refsx`, `info/refs ` and `info/ refs` (leading and trailing space), `info/Refs`, `info/REFS`: all hashed. Through `Run`, a fresh `info/REFS` is `host_state info/REFS`, exit 1.
- Case-insensitive volume (APFS here): once `info/refs` exists, a write through `INFO/REFS` lands in that same file, which the listing still names `refs`, so it is skipped (exit 0). That is the one file git never reads locally, so nothing is hidden; on Linux CI the two names are different files and `INFO/REFS` is hashed.
- `hooks/info/refs` is hashed: the skip is keyed on the full common-relative path, not on the basename.
- Symlinks: `WalkDir` does not follow them, `fileState` returns `other` for any non-regular file, so turning `info/exclude` into a symlink (to `refs` or anywhere) changes its state and is caught. A symlink placed at `info/refs` itself is skipped, and again git does not read it. A hard link from `info/refs` to a hashed file is caught by content. Replacing `<common>/info` itself with a symlink makes every `info/*` key disappear, which is caught.
- What git reads locally under `info/`: `exclude`, `attributes`, `grafts`, `sparse-checkout`; all still hashed. `info/refs` is written by `update-server-info` and read only by the dumb HTTP transport from a URL, never from the local path (local and `file://` fetches use `upload-pack`).

## (a) Mutation tests (each applied alone in a separate copy of `6d7b613`)

| Mutation | Test | Result |
|---|---|---|
| Remove the `info/refs` skip in `hostFiles` | `TestFixInfoRefsIsNotHostState` | killed (`host_state info/refs changed during the run`, plus the two `not inspected` follow-ons) |
| Marker back to `os.WriteFile(w.Path+".tainted", ..., 0o600)` | `TestFixTaintedMarkerFIFO` | killed (blocked 15 s on the FIFO) |
| Drop `c.synced` from the claim-ref exemption | `TestFixSkippedSyncIsStrict` | killed (the mirror change for ticket-24 was no longer reported; only the config change remained) |
| Log range `<base>..refs/heads/<branch>` instead of `<base>..<oid>` | `TestFixPushesCheckedOID` | killed (`outside_lease docs/late.md`: the late commit now carries an out-of-lease file and the branch is moved at the `log` call, before the walk) |

All four killed, none survived. The strengthened push test also still asserts the remote tip equals `head_oid` and differs from the moved commit, so the push-by-ref mutation from the previous round stays covered.

## (b) Test runs

- `go test -race ./...` at `6d7b613`: pass, all packages (dispatch 70 s under the race detector).
- `go test -count=5 -timeout 30m ./internal/dispatch/`: pass, 333 s, no flake; the `info/refs` flake the opus re-review saw in `TestRunBaseAndBranch/remote_tracking_only` did not recur.
- `go build ./...` and `go vet ./...`: clean (implied by the test build; vet run as part of `go test`).

## (c) Size

`git diff origin/main 6d7b613 --numstat` excluding `*_test.go`: 1223 added, 41 deleted, all in `.go` files. This round is +4 -4 non-test (two `not_enforced` strings, one condition, one marker write). 127 under the D-136 and D-169 stop of 1350.

## (e) Git-call table

Unchanged from `.waggle/hive/reviews/T-023c-refable.md`. This round adds no git invocation, removes none, and changes no pinning or gating. The only host-state inputs that moved are one hashed key (`info/refs`, out) and the marker write path (same target, atomic).

## (f) CI

`product-ci` on PR #50: pass at `6d7b613` (ubuntu, 1m35s, run `39287361387`, `headSha` confirmed). `verify-leases-and-tiers` fails with `leases: no claim for T-023; claims must stay until merge` while `tiers: ok` and `decisions: unchanged`; that is the known D-119 and D-168 local-claims gap, not a phase C defect. `relay-claim` skipped.

## Other checks

- Lease: the four changed files are all under `internal/dispatch/`. No `.waggle/control/**`, `go.mod` or `go.sum` change; stdlib only.
- Sign-off: `6d7b613` carries `Signed-off-by` under the Keeper's git identity with the `T-023:` subject prefix, as do the four commits beneath it.
- Dashes: no U+2013 or U+2014 in `git diff origin/main 6d7b613` or in any commit message on the branch, and none in this review.
- `TestFixTaintedMarkerFIFO` calls `f.run` from a goroutine; `f.run` only reaches `t.Fatal` on a scenario write failure, and `newFixture` (with its `t.Setenv`) runs on the test goroutine, so the test is well formed. If the 15 s timeout fires the goroutine stays blocked until the test binary exits, which is the failing case anyway.

## Notes (non-blocking, carried)

- Fable re-review N1 (phase D: record the `hostFiles` map in `result.json` and compare before the next run's first unpinned call), N2 (the `global:<absolute path>` key prints a host path into `result.json` on an XDG violation), N3 (main-checkout and reuse-path `--ignore-submodules` variants still untested), and N4 (round-one carry list) stand unchanged. Opus N2 (Keeper `GIT_CONFIG_KEY_1` and later entries dropped by the Comb env; `GIT_CONFIG_PARAMETERS` precedence), N5 and N6 stand unchanged.
- Opus R2 also noted that an `info/refs` rewrite during a long session stopped every later keepalive renewal through the fix 4 gate. With the key out of the set, that path is closed too; probe P2 saw no `claim_renew_failed` warning, though the fixture session is too short to exercise many renewals.

## Intent Packet (for the Keeper)

- **What:** phase C second fix round at `6d7b613` on PR #50; 4 non-test lines changed, 42 test lines added, 1223 added non-test lines total against `origin/main`.
- **Why:** the opus re-review found that git's own auto maintenance made clean runs fail and taint the worktree (`info/refs`), and that one `not_enforced` line understated the `.gitignore` gap; a planted FIFO at the taint marker could also hang dispatch while holding its lock.
- **Blast radius:** `internal/dispatch/` only; one key leaves the host-state hash (a file git never reads locally), the marker write becomes atomic at the same path, two disclosure strings change. No claim, lease, Comb or CLI semantics change.
- **Evidence:** four mutations killed by the four named tests; real `gc`, `repack`, `pack-refs`, `update-server-info` and `maintenance run` inside a session are clean in both modes; the `info/refs` skip is exact under directory, prefix, suffix, case and symlink variants; race suite and five dispatch repeats pass; linux `product-ci` green at the head.
- **Residual, stated in `not_enforced`:** unchanged from the previous round plus the two sharpened lines (gitignore-covered main-checkout writes including an agent-written `.gitignore`; system gitconfig and `~/.ssh/config` read by the host push).
- **Merge:** pass.
