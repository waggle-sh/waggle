# Guard review: T-027 (first review, fable)

The change does what the brief asks and stays inside its lease. Build, vet and every test the brief allows on this host pass with race. Nothing widens what the worker's settings allow.

Verdict: pass
Risk-Tier: 3

## Intent Summary

One commit, `a353947`, on `t-027` over base `5347024`. It moves the Bash allowlist validation out of `workerSettings` into a new exported `runtime.BashCommands(role, commands)` (`launch.go:308`), which returns the hive commands followed by `gitAllowed`, minus `git add` and `git commit` for a guard, after the same checks as before (plain words, no wildcard, quote, control character, shell operator or rule syntax, no denied program, no `git push`). `workerSettings` now builds its `Bash(<cmd>)` and `Bash(<cmd> *)` allow rules from that list and adds `"attribution":{"commit":"","pr":"","sessionUrl":false}` next to `permissions`. In `Run` (`run.go:263`) the same list is computed before the task lines, in the step before `AcquireClaim`, and feeds a new `Bash:` task line (`bashLine`, `inputs.go:163`) in both `TaskLines` and `guardTaskLines`, after `Gate:`. `Invocation.Commands` stays the raw WAGGLE.md list, so both consumers validate and neither filters twice. The post-run checker counts commits in `CheckBase..head` with a `Co-authored-by:` line (any case) through the pinned git (`check.go:278`) and `Run` adds one `attribution_trailer: <n> commit(s) ...` warning with the count only. Blast radius: every dispatched worker's settings JSON and variable prompt, plus one new optional warning in `result.json`. Exit code, push and the stable prompt are unchanged.

Files: `launch.go`, `run.go`, `inputs.go`, `guard.go`, `check.go` and five `*_test.go` files in the two lease directories; all are in the lease rule's list. `go.mod` unchanged. About 91 added non-test lines (284 insertions, 193 of them in tests), under the target of 120, so the planned cut was not needed and 2.4 is in.

## Checks against the brief

- **AC1:** `go build ./...` and `go vet ./...` pass. product-ci is the only full run.
- **AC2:** only lease-rule files change.
- **AC3:** `TestBashCommandsMatchSettings` shows the allow rules after the five fixed ones equal `Bash(c)`, `Bash(c *)` per `BashCommands` entry in order; `TestBashLineNamesAllowedCommands` shows the backticked names in the `Bash:` line equal the same list, for builder and guard.
- **AC4:** `TestBashCommandsRefusals` covers wildcard, quote, shell operator, control character, denied program and `git push` (plus double space, empty, rule syntax, `gh`), through both `BashCommands` and `workerSettings`. The refusal in `Run` is in the step at `run.go:263`, which precedes `AcquireClaim` at `run.go:280`. The error is `ErrAdapterConfig`, not a dispatch `*Error`, so `ExitCode` still gives 3.
- **AC5:** guard list and line carry no `git add` or `git commit`; `TestGuardSettingsDropGitWrites` still passes.
- **AC6:** `TestPrepareArgv` asserts the exact attribution substring and that the settings hold exactly two top-level keys. The allow and deny slices and `defaultMode` are built by the same code as before, so `permissions` is byte-identical.
- **AC7:** `TestTrailerCountWarnsWithoutText` counts 2 of 3 commits (one `CO-AUTHORED-BY`, one lowercase, one `Signed-off-by` only), 1 over a shorter range, 0 over an empty one. The warning format carries only the integer. Warnings do not feed `end`, `exit_code` or `pushed`.
- **AC8:** `TestBashLineStablePromptUnchanged` and `TestPrepareStableAcrossTasks` pass.
- **AC9:** no new test calls `Run`, `sweep()` or sends a signal; no `t.Skip`. The trailer test uses `newRepo` and `sh`, git only.

**Threat model (D-203):** no new allow rule, no dropped deny rule, validation unchanged and on every path; the task line holds only the validated list plus fixed text, nothing from the brief, reviews or run; the guard keeps no git writes; the refusal stays before `AcquireClaim`; the scan runs on `pg` (pinned) and never keeps message text. Guard runs return from `checker.run` before the commit walk, so `trailers` stays 0 for them, which is right since guards do not commit.

Tests run on this host, all passing with `-race`: the six new tests plus `TestPrepare[A-Z]`, `TestTripwireBlocksPush`, `TestGuardSettingsDropGitWrites`, `TestInputsComeFromBase`, `TestWaggleCommandsByTitle`, `TestWaggleCommandsAndOutcome`.

## Blocking Issues

None.

## Notes (not must-fix)

- **N1, the commit itself carries a trailer:** `a353947` has a `Co-Authored-By: Claude Sonnet 5.5` line. The builder ran under the dispatch binary from before this change, so the attribution setting was not yet in effect. This is the merge-time cleanup the queen already does and is exactly what 2.4 reports from now on.
- **N2, gofmt:** denied in this session, so I checked the added lines by eye: tab indentation, no trailing whitespace (grep over the nine files), and the aligned blocks (the `perms`/`attribution` struct fields, the `checker.trailers` comment, the refusals map literal) match gofmt's column rules.
- **N3, Run-level behavior read, not tested:** the refusal ordering and the single-warning shape are verified by reading `run.go`; section 4 forbids `TestRun*` on this host, so product-ci is the only place they run end to end.
- **N4, test name versus assertion:** `TestTrailerCountWarnsWithoutText` asserts counts only; the "without text" property rests on the format string in `run.go:360`, which has one `%d` and no message field. Fine as is.
- **N5, Q-T027-2 still open:** whether Claude Code applies `attribution` under `--setting-sources=` is not provable in a dispatched build. The first post-merge builder commit confirms it, and a trailer now surfaces as a warning.
- **N6, pre-existing shape:** when the branch cannot be resolved, `rng` is `<CheckBase>..` for both the existing commit listing and the new scan. A failed scan counts as 0. Not new.

Verdict: pass
