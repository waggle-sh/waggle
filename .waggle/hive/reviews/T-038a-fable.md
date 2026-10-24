<!-- waggle-review v1 verdict=pass head=6b916cf65b5d5b3e534f213fe12b4a78bddc4497 -->

Verification is complete: build, vet and gofmt are clean, the full race suite passes on the host, and I have traced each item against the brief. Writing the review.

Risk-Tier: 3

**Intent Summary:** Phase A of T-038 lands all eight polish items (A1 to A8) in one commit on `t-038a`, touching six product files and two test files, all inside the phase A lease rule. Dispatch's `roundMustFix` now skips brief reviews named `T-<nnn><suffix>-brief[0-9]*(-|.md)` before they use a `maxReviews` slot; `ErrorCode` returns `refused` for a sentinel-less exit 4; `ClaimTask` and `ReleaseTask` map an `ExitEnv` error to exit 1 `interrupted: ...` only when the context is done, via a deferred helper next to `ErrorCode`; the `waggle/T-` release-branch form is capped at 100 characters; `topTier` is exported as `TopTier` and `guardModel` calls it; `--second` refuses a builder or first-guard record with an empty model; `./` is refused as a protected path; and `ClaimTask` names the canonical task before `Validate`. Blast radius is dispatch's must-fix count and model choice, the `claim` and `release` CLI exit and JSON codes, and base-config validation. The hive sees none of it until the next pin move (D-269). Added non-test lines: 42, under the target of 91. Head Keeper approval is needed at merge (Tier 3).

**Verification run (host, this worktree):**

```
go build ./...        clean
go vet ./...          clean
gofmt -l .            no output
go test -race ./...   ok for every package (dispatch 256s, cmd/waggle 48s, comb 31s)
git status            clean, no probe files written
```

**Lease and rule checks:**
- Changed files: `config.go`, `gate_test.go`, `guard.go`, `inputs.go`, `polish38_test.go` (new), `release.go`, `task.go`, `policy.go`. Nothing in `run.go`, `claim.go`, `request.go`, `worktree.go`, `internal/comb/`, `cmd/waggle/`, `.waggle/`, `.github/` or `go.mod`. Within each file only the named functions change. The A1 helper is a package-level `regexp` var beside `roundMustFix` plus the stdlib import it needs, which I read as the brief's "new unexported helper next to it".
- Existing assertions: none changed. `gate_test.go` gains one refused row (`./`). No `t.Skip` added. AC2 holds.
- D-193: the cancel test cancels the context before every call. With an already-cancelled context `exec.CommandContext` refuses to start git, so no process is spawned or killed. No signals are sent.
- AC16: no em or en dash in any added line (searched all `.go` files) or the commit message.
- No existing `ErrorCode` row changes. The only new result is `ExitClaim` without a sentinel, which A2 specifies.

**Item-by-item:**
- **A1:** the skip test runs after `HasPrefix` and before `files < maxReviews`, so a skipped name uses no slot. `briefing` and `Brief` still count (anchored, case-sensitive). On branch `t-023-brief`, `T-023-brief-opus.md` counts because the skip applies after the prefix. The per-ticket `T-<nnn>.md` path is untouched. The builder substituted `T-023-Brief-sonnet.md` for AC3's `T-023-Brief-opus.md` to avoid a case-insensitive filesystem collision with `T-023-brief-opus.md`; the assertion is equivalent. Both AC3 tests fail on today's code by construction (50 sorted brief files would fill the slots before `T-023-zzz.md`).
- **A2:** the trailing `if` became a `switch`; `ExitFailed` still gives `interrupted`, `ExitUsage` without a sentinel still falls through to `environment` as before.
- **A3:** `interruptedEnv` fires only when `err != nil`, `ctx.Err() != nil` and `ExitCode(err) == ExitEnv`. The wrap uses `%w` through `*Error.Unwrap`, so table sentinels keep their code as the brief accepts. The fixture has `.waggle/worktrees/` (created by `scaffold/init.go:96`), so the first context-aware call in `ReleaseTask` is `lockRepo`'s `rev-parse` (`ExitEnv`), and in `ClaimTask` it is `ResolveBase` with the default base (`ExitEnv`). Both return exit 3 today, so AC5 exercises the new mapping rather than `claimError`. The release case also checks the claim ref survives. Bad-ticket cases stay exit 2 under cancel. `Run` is unchanged.
- **A4:** `len(branch) > 100` is added only on the `waggle/T-` path; the `t-` path still returns `checkBranch` directly. AC6 table covers 100, 101 and 107.
- **A5:** `TopTier` has the identical body; the only two callers are `policy.go:158` and `guard.go:184`. `guard.go` still uses `comb` elsewhere, so no import churn.
- **A6:** the empty-model check sits after both records are confirmed found and before `AvoidModels` is appended, with the required message and `ExitEnv`. Plain guards are untouched. `TestGuardAlternates`, `TestSecondGuardRefusals` and `TestSecondGuardHint` pass unchanged.
- **A7:** `e == "./"` is exact. `./` is the only entry whose single-trailing-slash-stripped form is `.`; `.//` already fails the clean check.
- **A8:** `res.Task` is set from `ParseTicketID` before `Validate` and reassigned to the same canonical value after; an unparseable ticket leaves it empty. AC9b covers the no-gate, bad-ticket and empty-repo cases.

**Notes (not must-fix):**
- `TestClaimReleaseCancelled` asserts `code(err) != ExitFailed || ... || err == nil` ordering is fine, but `TestSecondRefusesEmptyModel` checks `code(err) != ExitEnv || err == nil` where the `err == nil` clause is redundant (a nil error has exit 0). Harmless.
- Builder's final report should name the push-mode tests for the queen as the brief asks; on this host the remote fixtures (`TestClaimReleaseRemote` and the comb remote tests) ran and passed.

**Blocking Issues (Must-Fix Only):** none.

Verdict: pass