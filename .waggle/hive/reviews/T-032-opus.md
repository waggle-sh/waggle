# Guard review: T-032 (head `f5f86c3`, base `f05cd81`)

**Verdict: pass. Risk-Tier: 3.** I found no blocking issues and nothing to send back to the Builder.

**Commands I ran:** `go build ./...`, `go vet ./...` and `gofmt -l .` all printed nothing. `go test -race ./...` passed in every package. That includes the push tests against a local bare origin (`TestClaimReleaseRemote`, `TestReleaseClaimRemote`), so none failed only because the origin sits outside the scratch directory.

**Review file:** I did not write `.waggle/hive/reviews/T-032.md`. It sits outside this run's lease and would show up as a lease violation, so the review is here for the queen to file. I left no probe files.

## Intent Summary

T-032 adds two commands. `waggle claim` writes a claim for any person or orchestrator. `waggle release` gives one back. Both can print JSON.

The work lives in `internal/dispatch`:
- **`ClaimTask`** (`task.go`) reuses `Request.Validate` and refuses hardcoded Tier 3 directories before any fetch. It reads `gate.protected_paths` from the base revision only and refuses a covered directory, also when a test injects a backend. It then writes through the existing `AcquireClaim`.
- **`ReleaseTask`** (`release.go`) is the old `Release` body returning a result. `Release` stays as a wrapper with the same text output.
  - Release also accepts the `waggle/T-<nnn>` branch form and an `--owner` flag.
  - With no `--branch`, only the first branch that exists is checked against the second-parent merge rule.
  - When `.waggle/worktrees/` is absent (per `os.Lstat`), release takes no lock and creates nothing.

Other changes:
- The Comb gains `Options.Protected` and `ErrProtectedPath`. The protected-path check runs in `prepareLease` only, when a lease is written, so stored claims are never re-checked and can't turn malformed.
- `TierPolicy.CoversDir` reuses `touchesPrefixes`.
- `ResolveBase`'s default base and the base name in `offBranch` are now fully qualified (`refs/remotes/<remote>/main` or `refs/heads/main`). A tag named `origin/main` or `main` can no longer pick the base for `Run`, `ReadInputs`, `ClaimTask` or `branchMerged`.
- After a run, dispatch's check now calls `CheckChangesPolicy` with the base policy. With an empty policy the results are the same as before; `TestCheckerPolicy` compares them.

**Blast radius:**
- `waggle dispatch` now refuses, and flags after the run, any directory listed in the base config's `gate.protected_paths`. This hive has no `gate:` section, so nothing changes here until one is added.
- `waggle dispatch release` also gets the wider default branch choice and the no-worktrees path.

## Checks against the brief

- **AC2, lease rule:** all 14 changed files are named in the lease rule. Each edit stays inside its allowed part (in `run.go`: the policy step, the backend options, the checker field and the `offBranch` name). `go.mod`, `internal/config/`, `gate*.go`, `clean.go`, `request.go` and `worktree.go` are unchanged. Every ref write still goes through `AcquireLease` and every delete through `ReleaseLease`.
- **AC9, compatibility:** no existing test file changed apart from added test functions, and I found no `t.Skip`. The `waggle dispatch release` lines, including `warning: ` lines, match the old output.
- **Threat model:**
  - Protected paths are read only at the base OID, through `blobAt` with the `MaxFileSize` cap. A symlink, directory, oversized or invalid config exits 3.
  - The policy only adds refusals.
  - The merged check can't be satisfied by pushing a `waggle/T-<nnn>` branch while an unmerged `t-<nnn>` exists; a test covers that case.
  - With `--json`, stdout carries one `encoding/json` object, and `error.exit` comes from `dispatch.ExitCode`, the same value the process exits with.
  - In text mode, values from refs and config pass through `clean`, which drops Cc, Cf, Zl and Zp characters.
- **AC11:** `ErrProtectedPath` text matches the brief exactly. The new user-facing strings contain none of the banned words. No new test calls `Run` or `sweep()` or sends a signal; `offBranch` is tested on its own.
- **AC12:** `TestDefaultBaseIgnoresTags` first confirms the tag would win an unqualified lookup, then shows that `ResolveBase`, `ClaimTask` (the `docs` claim is not refused), `ReleaseTask` and `offBranch` all ignore it, in both local and remote mode.
- **Budget:** about 560 added non-test lines, under the target of 800.

## Blocking Issues (Must-Fix Only)

None.

## Notes (not must-fix)

1. **AC10 test gap:** the hostile error text in the test lacks U+2028. The code handles it (`clean` drops Zl and `encoding/json` escapes it); only the test misses that one case.
2. **AC12 setup:** in remote mode the test pushes the tag's commit to the origin as `refs/heads/evil` rather than making it there directly. The test still proves the point.
3. **Branch length:** `checkReleaseBranch` applies the 100-character limit to the `t-` form, so a `waggle/T-…` name can be up to 107 characters.
4. **Error code mapping:** an exit-4 error that wraps no known sentinel would get the code `environment`. No such error exists today.
5. **Argument order:** `waggle claim --json T-32 --dirs …` is a usage error, because Go's flag parsing stops at the first non-flag argument. The task ID goes first, as in the usage line.

Verdict: pass