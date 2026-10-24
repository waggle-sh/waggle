<!-- waggle-review v1 verdict=pass head=6b916cf65b5d5b3e534f213fe12b4a78bddc4497 -->

Risk-Tier: 3

**Intent Summary:** Phase A of T-038 (branch `t-038a`, head `6b916cf`, one signed commit on base `2bcc79a`) closes the review notes that phase A lists. **A1:** `roundMustFix` no longer counts brief reviews. It skips a name whose part after the phase prefix matches `^brief[0-9]*(-|\.md$)`, and does so before the file uses one of the `maxReviews` slots (`inputs.go:124-140`). **A2:** `ErrorCode` returns `refused` for an `ExitClaim` error that wraps no listed sentinel. **A3:** a new `interruptedEnv` helper next to `ErrorCode` in `task.go` is applied through a defer in both `ClaimTask` and `ReleaseTask`. When `ctx.Err() != nil` it turns an `ExitEnv` error into `ExitFailed` with the message `interrupted: ...`. Usage and refusal errors keep their codes, and both doc comments mention the cancel case. **A4:** a `waggle/T-` release branch longer than 100 characters is refused with the existing message. **A5:** `topTier` is renamed `TopTier`, its caller in `Resolve` is updated, and `guardModel` uses it. **A6:** `--second` refuses with `ExitEnv` when the builder or first guard record has no model, naming the record. **A7:** a `./` entry in `protected_paths` is refused with the existing message. **A8:** `ClaimTask` sets `res.Task` before `Validate` when the ticket parses. Blast radius: the hive is unaffected until the pin moves. For outside users, claim and release exit codes change only under a cancel, the JSON `refused` value is new, and the must-fix count no longer includes brief reviews. `Run` is untouched.

**Checks:**
- **Lease:** the changed files are `internal/config/config.go`, `internal/config/gate_test.go`, `internal/dispatch/{guard,inputs,release,task}.go`, the new `internal/dispatch/polish38_test.go` and `internal/runtime/policy.go`. Every change is in a function the phase A lease rule allows: `validProtectedPath`, `guardModel` (lines 167-184), `roundMustFix` with the new helper next to it, `ReleaseTask`'s return path and doc comment, `checkReleaseBranch`, `ClaimTask`'s first lines, return path and doc comment, `ErrorCode` with the new helper after it, and `topTier` with its one caller. Nothing changed in `run.go`, `claim.go`, `request.go`, `worktree.go`, `.waggle/`, `.github/` or `go.mod`.
- **Existing assertions:** none changed. The only edit to an existing test file adds the `"dot slash"` row to `TestGateProtectedPaths`. No `t.Skip` was added.
- **Threat model (section 5):**
  - The skip is anchored, case-sensitive and applied after the prefix. `T-023-ci.md`, `briefing` and `Brief` names still count, and the per-ticket `T-023.md` path is untouched.
  - `TopTier` has the same body as `topTier`.
  - The `ctx.Err() != nil` and `ExitEnv` conditions are both required. `ExitCode` takes the outermost `*Error`, so a wrapped sentinel keeps its own code, which the brief accepts.
  - The `t-` branch rule is unchanged.
  - `./` is the only `protected_paths` entry whose result changes. `.//` was already refused by the clean check.
  - `--second` with both models set still runs the same `Resolve` call.
  - No new test sends a signal or cancels a running git call: the contexts are cancelled before the call.
  - A3's tests are not passing for the wrong reason. In `ClaimTask` the first call that uses the context is the git `ResolveBase`, and in `ReleaseTask` it is `lockRepo` (the fixture's worktrees directory exists). Without the change both would give `environment`, not `interrupted`. `defer unlock()` runs before the mapping defer, which is harmless.
- **Acceptance criteria:** AC3 to AC9b are covered by `polish38_test.go`, with the existing pins (`task_test.go:179`, `TestGuardAlternates`) passing unchanged. Small differences from the brief:
  - AC3 uses `T-023-Brief-sonnet.md` instead of `T-023-Brief-opus.md`, with a comment about case-insensitive disks.
  - AC3 checks `ci`, `briefing` and `Brief` together for a count of 4 rather than checking `ci` alone for 2. That is equivalent.
- **Budget:** about 41 non-test lines were added, under the target of 91.
- **Dashes:** no em or en dash in the changed `internal/` files.
- **Commands:** all four were run on the head.
  - `go build ./...`: clean.
  - `go vet ./...`: clean.
  - `gofmt -l .`: printed nothing.
  - `go test -race ./...`: every package ok (`internal/dispatch` took 281s). No test failed because it pushes.

**Notes (not blocking):**
- When the PR is opened, its body should say that no existing assertion changed (AC2).
- Under a cancel, a failure that is not from git, such as a bad config, is also reported as `interrupted`. The brief specifies exactly that ("any error whose `ExitCode` is `ExitEnv`").

**Blocking Issues (Must-Fix Only):** None.

Verdict: pass