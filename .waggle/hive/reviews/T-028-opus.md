# T-028 guard review

Verdict: pass

Risk-Tier: 3

## Intent Summary
The worker's push tripwire used to rewrite every local absolute and `file://` push to `file:///dev/null/`. That broke tests in the worker that push to a bare origin under `t.TempDir()`. With this change, `Run` creates a new 0700 scratch directory for each run with `os.MkdirTemp(os.TempDir(), "waggle-<ticket>-<role>-")`. It does this before `Prepare` and before the claim, and passes the path in as `Invocation.ScratchDir`. `workerEnv` sets that path as the only `TMPDIR`. `tripwireEnv` appends keys after the existing 14:
- For the raw path and, when it differs, the symlink-resolved path, it adds a self-rewrite for `<p>/` and for `file://<p>/`.
- For each of those spellings it also adds a longer rewrite of `<p>/../` back to `file:///dev/null/`.

`Prepare` refuses a scratch path that is relative, unclean, equal to `/`, or holds a byte below 0x20, returning `ErrAdapterConfig` (exit 3, before the claim). After the post-run checks and before the record is written, dispatch runs one `os.Remove` that only succeeds on an empty directory; if the directory is not empty it adds `scratch_left: <base>`. A defer does the same on every other return. The `pushBlocked` comment and the `not_enforced` text now name the exemption and both bypasses. The change only affects dispatched workers' push environment. Callers with no scratch directory, including `Verify`, get exactly the same entries as before.

## Checks
- **Build and tests:** `go build ./...` and `go vet ./...` pass. These tests pass under `-race`: `TestTripwireBlocksPush`, `TestPrepareEnv`, `TestTripwireScratchExemption`, `TestTripwireEnvNoScratch`, `TestScratchEnvShape`, `TestScratchPathRefused`, `TestScratchLifecycle` and `TestNewScratchRefusal`. On this macOS host the exemption test also ran the `/private/var` resolved-path cases.
- **AC2 (lease):** only the files the lease rule names changed: `launch.go`, `runtime.go`, `run.go`, `cell.go` (`not_enforced` only), plus two new `scratch_test.go` files. `go.mod` is unchanged.
- **AC3:** pushes into the scratch directory go through by path, by `file://` URL and by the resolved path, and the ref moves. These are still rewritten and move no ref:
  - a directory outside the scratch directory, by path and by `file://`
  - the prefix sibling `<scratch>-x`, by path and by `file://`
  - `<scratch>/../…`, by path and by `file://`
  - http, https, ssh, scp (`git@`), git and a relative path
  
  This is a superset of what `TestTripwireBlocksPush` covers.
- **AC4:** `tripwireEnv(nil)` is checked against a literal copy of the old 14-entry list. `envFixed(nil)` therefore matches the old `envFixed` exactly.
- **AC5:** there is exactly one `TMPDIR` entry (the old value is skipped, not just overridden), one `GIT_CONFIG_COUNT`, and the indexes are contiguous.
- **AC6:** the refusal step comes before `AcquireClaim` (`run.go:265-287`), and the existing `d1_test` confirms that `ErrAdapterConfig` gives `ExitEnv`. The deferred empty-only remove cleans up the directory.
- **AC7 and AC9:** the lifecycle is tested through `newScratch` and `dropScratch` directly. No new test calls `Run` or `sweep()`, sends a signal, or uses `t.Skip`.
- **AC8:** both texts name the exemption, the planted-symlink bypass and the remaining `..` bypass.
- **Threat model:**
  - **No network form is exempted.** Every exemption key begins with `/` or `file:///`. A path that starts with `/` is always local to git, even if it contains a `:` or `::`.
  - **The keys come only from the path dispatch created.** That path is built from the Keeper's `os.TempDir()` and a random suffix, so it is new for each run and the worker cannot choose it.
  - **The trailing slashes are present**, which stops `<scratch>-evil/` from matching.
  - **The only delete is one non-recursive `os.Remove`** of the scratch path itself.
  - **The deny rules and `gitAllowed` are unchanged.**
- **Budget:** about 95 added non-test lines, under the target of 130.

## Notes (not must-fix)
- There is a tiny race window between `MkdirTemp` and `EvalSymlinks` in `Prepare`. A different process running as the same user could swap the new directory for a symlink in that window, and the resolved spelling would then exempt the symlink's target. That process would need to write into the Keeper's `TMPDIR` outside its own tree, which is a separate-escape case under the D-203 bar.
- The commit carries a `Co-Authored-By` trailer. Dispatch's `attribution_trailer` warning asks for it to be dropped before merge.
- I couldn't run `gofmt` here. The alignment I checked by eye looks right, and product-ci will confirm.

## Blocking Issues
None.

No probe files were created.

Verdict: pass