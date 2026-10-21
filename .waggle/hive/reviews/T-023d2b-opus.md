# T-023 phase D2b review (opus guard)

Branch `t-023d2b` at `9244e78`, base main `be3d121`, PR #61. First review, Tier 3.

Verdict: pass

Risk-Tier: 3

## Intent Summary

D2b adds `waggle dispatch clean --host-record [--accept <digest>]` (new `hostrecord.go`). It lists the host-file keys that differ from `host-files.json` with values cut to 12 characters and keys passed through `printable`. It accepts the current state only when the 16-hex digest, taken over the stored record's file state plus the current key and value pairs (D-209), still matches under `dispatch.lock`. `checkHostRecord`'s two messages now name that command. `provision` no longer runs `git worktree prune` (D-202, D-207). Git's own refusal for a stale entry comes back as exit 3 with a `waggle dispatch clean T-<id> --role <role>` hint, and all four `worktree add` forms run through `g.pinned()`. `clean` now prints a line on every exit 0 and refuses an existing directory that no `worktree list` line names (exit 3, directory, `.tainted` and takeover guard kept). `stopSignals` adds SIGHUP, and `TestWorkerLeadsOwnSession` accepts `getsid() == -1`. Blast radius: the host record gates every later run, and provision's stale-entry handling and dispatch's stop signals change. No sweep, reap, run, cell, guard or ledger code changes. 184 added non-test lines, under the 190 target, so items 4a and 4b stay in (D-206).

## Blocking Issues (Must-Fix Only)

None.

## Checks

Evidence is in `.waggle/hive/media/T-023d2b/`: `opus-checks.txt`, `opus-probes.txt`, `opus-fixremoval.txt` and `opus-probe-src.txt`. The probe file was a temporary uncommitted `_test.go` and has been removed. Every probe and fix-removal check is git and file only: no `Run`, no signal, no `sweep()`, no container.

1. **`--host-record` and `--accept`.**
   - A record rewritten after the listing is refused with exit 3 and its bytes unchanged. This holds for valid to valid, bad JSON to different bad JSON, and oversize to bad JSON.
   - A further config edit is refused the same way. So is a well-formed digest that does not match.
   - A held `dispatch.lock` is exit 3 with nothing written.
   - A malformed or empty digest is exit 2, decided before any directory is created.
   - Over a symlinked record, the rename replaces the link and the target bytes stay as they were.
   - A forged record and planted `hooks/` and `info/` names carrying OSC 8, CSI, BEL, U+0085, U+200B, U+202E, U+2066 and U+2067 print stripped, with keys capped at 100 bytes. Values are hashes or fixed states, so neither config values nor host paths reach the output.
   - Every exit 0 prints a line.
2. **Provision.**
   - No non-test file in `internal/dispatch/` or `cmd/` runs a prune; the only hits are comments.
   - A GIT_TRACE probe covered provision, a stale own entry, `Clean` and a guard `--detach` provision. Its 17 built-in git calls contain no `prune`, `gc` or `maintenance`, and the trace does contain `worktree add`, so the check is not vacuous.
   - T-024's stale entry survives the whole sequence.
   - The D-207 hint fires only after a `worktree add` attempt while the path is absent. It does not fire for an existing file or directory. The error is exit 3 through `Provision`'s `failf(ExitEnv)`.
3. **`clean`.**
   - A plain directory (builder and guard role, and the all-roles form) is exit 3 with the directory, `.tainted` and `.lock.takeover` kept and the role lock released.
   - A mis-cased `--repo` on APFS (upper-cased parent component) over a real registered worktree is exit 3 with all three kept. The correctly cased clean then removes it.
   - I found no ordinary input that removes a tainted marker while its directory remains. The only branch that removes markers without a removal runs when `Lstat(wt)` failed, and an `Lstat` failure other than not-exist cannot follow a successful `takeLock` in the same directory.
4. **Pinned `worktree add`.** All four forms use `pg := g.pinned()`.
   - Probe with a global `core.hooksPath` post-checkout hook, a repo `hooks/post-checkout`, a global smudge and process filter, `GIT_CONFIG_COUNT`/`KEY`/`VALUE` and `GIT_CONFIG_PARAMETERS` filters, and a `* filter=canary` branch attribute: no canary was written.
   - With `pinned()` removed, the four builder subtests fail and my probe fails too (the bad env `process` filter aborts the checkout).
5. **Lease.**
   - Every changed file is in 10.8.3: `clean.go`, new `hostrecord.go`, `prerun.go` (the two `checkHostRecord` messages only), `worktree.go` (`provision` only), the listed tests and `cmd/waggle/dispatch.go` and its test.
   - `setsid_test.go` holds item 4c per guard note N9.
   - There is no diff to `reap_*.go`, `run.go`, `cell*.go`, `guard.go`, `ledger.go`, `internal/runtime/`, `go.mod`, `.github/` or `.waggle/`, and no added line names sweep, targets or observe.
6. **Size and hygiene.**
   - 184 added non-test lines against `be3d121`.
   - No U+2013 or U+2014 in the diff or the commit message, and the commit carries Signed-off-by.
   - `product-ci` passes on linux at `9244e78`. `verify-leases-and-tiers` fails only on "no claim for T-023" (tiers ok, decisions unchanged), which the PR body expects until phase F.
   - `GOOS=windows`, `solaris` and `illumos` (amd64) vet pass.
7. **Fix removal, rerun by me.** Each named test fails with the guard it covers removed:
   - The digest compare.
   - The record-state term of the digest. My unreadable-record probe also fails.
   - `printable` in the listing.
   - The unregistered-directory refusal. Replaced by `_ = listed`, it fails the builder test plus my mis-cased and takeover probes.
   - The prune removal. With the prune restored, the builder T-024 test and my trace probe fail.
   - `pinned()` on `worktree add`.
   - SIGHUP in `stopSignals`.
8. **Host suite.** `go test -race ./internal/dispatch/... ./cmd/...` passes (dispatch 202.5s, cmd 16.4s), and so do `go build ./...` and `go vet ./...`.

## Notes

- **N1 (`hostrecord.go`, `printable`).** U+2028 and U+2029 (categories Zl and Zp) pass, because the brief names only `IsControl` and Cf. Only a planted name or a forged record supplies them (note level under 10.8.1). Fix if wanted: also drop `unicode.Zl` and `unicode.Zp`, 1 line.
- **N2 (`prerun.go:128`).** `checkHostRecord` joins the differing keys raw into the run's error. A planted `hooks/` name with escapes then reaches dispatch's stderr unfiltered, while the listing filters it. Planted names are note level. Fix for a later lease: map `printable` over `keys`, 2 lines.
- **N3 (`worktree.go:231`, the D-207 hint).** The hint is appended to git's own "use 'add -f' to override, or 'prune' or 'remove' to clear". So the Keeper is shown git's advice to run exactly the repo-wide prune D-202 removed. The hint also fires for "'t-023' is already used by worktree at <other path>", where cleaning T-023 does not help; its "if a stale entry ... remains" wording keeps it accurate. Suggest appending "(not git worktree prune)", 1 line changed.
- **N4 (`worktree.go:271`).** An existing plain directory at the worktree path gets "run waggle dispatch clean or remove it" from provision. `clean` now refuses that case by design, so only "remove it" works. Word it as "remove it by hand" in a later lease, 1 line changed.
- **N5 (`hostrecord.go`, digest).** Every unreadable record state of the same kind hashes to the same term (`other` for a symlink, directory or oversize file). A symlink retargeted between listing and accept is still accepted. The listing is byte-identical in that case, and the accept writes only `cur` and replaces the link, so the Keeper approves exactly what was shown. No change needed.
- **N6 (`result.go`, `writeFile`).** When the record is a directory, the rename fails and leaves a `.host-files.json.tmp*` file behind. This needs a planted directory in `.waggle/worktrees/`, so it is note level.

## Confirmation at c211ea0 (opus)

Verdict: pass
Risk-Tier: 2

**Intent Summary.** c211ea0 is the D-210 polish on top of 9244e78, which both guards passed. It changes 5 lines in 4 files under `internal/dispatch/`. `printable` now also drops U+2028 and U+2029 (N1). The provision hint for a refused `worktree add` on a missing path ends "(not git worktree prune)" and keeps the conditional "if a stale entry for ... remains" wording (N3). `checkReusable`'s plain-directory message says "remove it by hand" in place of pointing at clean, which refuses that case (N4). The blast radius is three message strings and one sanitizer rune set. No control flow changed.

**Checks**

1. Scope. `git diff 9244e78 c211ea0` touches only hostrecord.go, hostrecord_test.go, provision_test.go and worktree.go, and contains only N1 (the rune clause plus a test input with U+2028 and U+2029), N3 (the suffix plus the stronger test assertion) and N4 (the message). Nothing else.
2. N1. Dropping the clause, which exactly restores the 9244e78 blob fb63f0f, fails `TestHostDigestAndPrintable` with `printable = "abcdef g h"`. Restored, it passes. The same input still covers C0 (`\x00`, `\x1b`), C1 (`\u0085`) and Cf (U+202E, U+200B), and the 100-byte cap case is unchanged. Evidence: `media/T-023d2b/opus-confirm1-n1-mutation.txt`.
3. N3. The suffix is reached only when `worktree add` fails and the path is absent. On a stale entry, git 2.54 itself says "use 'add -f' to override, or 'prune' or 'remove' to clear". The suffix corrects that, and `clean T-<id> --role <r>` drops exactly that admin entry (the `entry != ""` branch). If the branch is checked out elsewhere (fable's case), git names the other worktree. The hint is conditional and claims no cause, and clean for this path prints "no worktree entry for ..." and changes nothing. So the hint is accurate on every path that reaches it.
   N4. `bad` is reached when the branch is missing locally, when EvalSymlinks fails, when the path is not registered on the expected branch, and (wrapped) when `adminDir` fails. Removing the directory by hand is correct in every case. Where the directory is still a registered worktree, clean could also do it. Removing it by hand instead leaves an entry, and the next provision then gets the N3 hint, which leads to clean. So the two steps meet up. The other messages that point at clean are `taintedErr` (worktree.go:191, a registered run worktree that clean removes) and the two `--host-record` messages in prerun.go. None of them sends the Keeper to clean for a case clean refuses. The non-directory case (worktree.go:216) says "remove it", not clean.
4. Size: 185 added non-test lines against be3d121 (stop 240). No U+2013 or U+2014 in the diff or the commit messages. c211ea0 is signed off by paulternate.
5. Host suite `go test -race -count=1 ./internal/dispatch/... ./cmd/...`: both ok. Evidence: `media/T-023d2b/opus-confirm1-checks.txt`.

**CI.** `product-ci` at c211ea0 failed, not passed. The cause is `TestRunBaseAndBranch/remote_tracking_only`: `TempDir RemoveAll cleanup: ... origin.git/objects: directory not empty`. That test and its fixture are unchanged since be3d121. `product-ci` passed at 9244e78, and this commit changes only message strings and the sanitizer. This is the detached `git maintenance --auto` flake already logged in T-023c-reopus.md, this time hitting tempdir cleanup and not `info/refs`. It is not a must-fix for this round. A rerun of run 39604347542 needs to go green before merge. `verify-leases-and-tiers` fails as expected (missing T-023 claim).

**Blocking Issues (Must-Fix Only).** None.

**Notes (non-blocking).**
- The `printable` doc comment still says "(C0, C1, U+202E)". It could name U+2028 and U+2029 in a later round.
- The fixture flake now has two symptoms (`info/refs` and tempdir cleanup). Setting `maintenance.auto=false` and `gc.auto=0` in the test fixture, origin included, would close both. That is a separate ticket.
