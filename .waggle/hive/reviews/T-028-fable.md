Only the host cell exists, so the TMPDIR change needs nothing in the cell spec. The worktree is clean and I created no probe files. Review follows.

# T-028 Review (guard, first review)

Verdict: pass
Risk-Tier: 3

## Intent Summary

One commit on `t-028` narrows the worker's push tripwire by one directory. Dispatch now creates a per-run scratch directory under the host's temp dir, before `Prepare` and before the claim, and passes it through a new `Invocation.ScratchDir`. The worker's `TMPDIR` becomes that directory. The tripwire adds, for the path and its symlink-resolved form, a self-rewrite for `<scratch>/` and `file://<scratch>/` plus a `/dev/null` rewrite for the top-level `..` form of each. After the post-run checks, dispatch removes the directory only when empty and otherwise records a `scratch_left` warning. The `pushBlocked` comment and the `not_enforced` text name the exemption and its two bypasses. Blast radius: every dispatched run's worker environment, and the set of local pushes the tripwire lets through. Network prefixes and every other path remain rewritten as today.

## What I verified

- **Lease and scope (AC2).** Only `cell.go`, `run.go`, `launch.go`, `runtime.go` and two new test files change. `go.mod` untouched. Only a `hostCell` exists, so the cell spec rightly stays unchanged.
- **Build, vet, tests (AC1 partial).** `go build ./...` and `go vet ./...` pass. All nine allowed tests pass with `-race` on this host, including the new scratch tests and the existing tripwire and env tests. I could not run `gofmt`, so formatting rests on product-ci. The diff reads gofmt-clean by eye.
- **Threat bar.** The change is purely additive on the `/dev/null` rules. The only non-dead targets are the self-rewrites, which match solely URLs beginning with `<scratch>/` or `file://<scratch>/`. No network scheme can begin with an absolute path, so no `https`, `ssh`, `git@`, `http`, `git` or `file://` URL outside the scratch directory loses its rewrite. The new test proves it for an outside repo, a prefix sibling, both top-level `..` spellings, and every network form, and checks no ref moved.
- **Exemption key provenance.** The path comes only from `os.MkdirTemp` under the host's `os.TempDir()` with a ticket and role that request validation already constrains. The worker cannot choose it, and the random suffix keeps runs and roles apart.
- **Path validation (AC6).** Absolute, equal to its `Clean`, no byte below 0x20, and an extra refusal of `/` itself. The resolved form is checked too. Refusal happens inside `Prepare`, which `Run` calls before `AcquireClaim`. The deferred empty-only remove covers every early return.
- **Env shape (AC4, AC5).** `tripwireEnv(nil)` is byte-identical to the literal of today's fourteen entries. With a scratch dir there is one `TMPDIR`, one `GIT_CONFIG_COUNT`, and contiguous indexes.
- **Scratch lifecycle (AC7).** A single `os.Remove`, never recursive, never inside the directory. Non-empty leaves a warning with the base name only. Tests cover both paths and the double-remove case.
- **Docs (AC8) and test hygiene (AC9).** Both texts name the exemption, the planted-symlink bypass and the remaining `..` forms. No new test calls `Run`, `sweep()`, sends a signal or skips.
- **Budget.** 97 added non-test lines, under the 100 estimate and 130 target. The resolved-form key pair stayed in.

## Blocking Issues

None.

## Notes (not must-fix)

- **N1.** A Keeper `TMPDIR` that is relative or unclean (for example a doubled slash) makes every dispatch refuse at `Prepare` with exit 3. That matches the brief's rule, but `newScratch` could clean and absolutize its parent first to turn an odd host env into a working run. Follow-up material at most.
- **N2.** The AC6 claim "the empty scratch dir is removed" on a `Prepare` refusal is covered by the deferred remove in `Run`, which the allowed tests cannot exercise. product-ci's full package run is the check.
- **N3.** `TestTripwireScratchExemption` creates and removes a sibling directory next to `t.TempDir()` in the shared temp parent. It cleans up, and the prefix-sibling case requires it, so this is fine.

Verdict: pass