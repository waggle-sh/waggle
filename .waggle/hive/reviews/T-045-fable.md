<!-- waggle-review v1 verdict=pass head=103b8d142c8c2cd6007fa8e39a5f996d67894198 -->

Build, vet and gofmt are clean, and the full race suite passes in every package, including the remote-mode fixtures in `internal/dispatch`. I have what I need for the review.

Risk-Tier: 3

**Intent Summary:** One commit on `t-045` adds a guard-only `--keeper-commit` flag to `waggle dispatch` so both D-205 guards can run through dispatch on a Head Keeper branch that no builder worked on. With the flag, a builder record on the branch refuses (plain or `--second`), `--second` needs only this round's first guard record on the head and avoids its model plus the policy's builder model with no ancestry rule, a missing claim ref is accepted with verb `none`, a claim whose branch list names the branch refuses, and the guard prompt shows `none (Head Keeper commit)` and `full (Head Keeper commit)` for directories and gate. Without the flag every path is byte-for-byte unchanged, including `secondMissing`'s existing text and the non-keeper `checkSecond` body. Blast radius is `internal/dispatch/` and one flag in `cmd/waggle/`; no runtime, comb, control or hive file changes. Added non-test lines total 77 by numstat, under the 120 target, so no planned cut was needed and the claim acceptance landed in this ticket.

**Lease and scope check**

- Changed files are exactly `dispatch.go`, `guard.go`, `request.go`, `result.go`, `run.go` and test files. In `run.go` only the two model steps, the `guardTarget` step and the `Result` literal changed. `checkBranch`, `latestRun`, `builderRun`, `guardRun`, `secondBase`, `checkBuilderModel`, `AcquireClaim` and `policy.go` are untouched.
- The three new files carry the Apache-2.0 header on line 1. The end-to-end test has `//go:build unix` after it. No U+2013 or U+2014 in any changed file.
- No code sends a signal, reaps, or deletes files or refs. A guard still never acquires a claim (`run.go:365` gates on `!guard`).

**Design conformance, 3.1 to 3.5**

- 3.1: `Validate` refuses the flag on builder and forager with exit 2 and the brief's text, next to the `--second` case. Help text and usage line match the brief verbatim. `Result.KeeperCommit` is set from the request.
- 3.2: `keeperBuilder` runs first in both `guardModel` and `checkSecond`, before any record is used, with the brief's exact text. Any head and any end refuse; another branch of the same ticket does not (`TestKeeperRefusesBuilderRecord`).
- 3.3: the keeper `--second` path sets `AvoidModels` to the first guard's model only and leaves `RecordedBuilder` empty, so `Resolve` takes `firstGuardModel` from the record and avoids the policy builder as well. `guardRun` plus `latestRun` exclude `second: true`, verdict-less, other-branch and other-ticket records, and the `T-0450` prefix cannot match the `T-045-guard-` cut. The empty-model refusal keeps today's text. `hiveShape(false)` yields `ErrNoCrossModel` with exit 3 (`TestKeeperSecondNoAlternate`).
- 3.4: the five rows are in the brief's order and texts. Nothing in the keeper branch reaches git, and the test passes a `&Git{}` with fake OIDs so any `merge-base` call would fail with another text. The policy builder model passed from `run.go` is derived by `Resolve("builder")` on a copy of the task with the same `GuardMustFix`, which is the same `effectiveLevel` and `pick` that `guardLevel` uses, so the re-check agrees with `Resolve`'s own avoid list. Its error is dropped, which only disables the redundant row 5 check; the guard resolution would already have failed on the same policy error.
- 3.5: `ErrNotFound` under the flag yields an empty lease with `TicketID` set, empty OID and verb `none`; a claim listing the branch refuses with the brief's text and exit 3; a claim listing another branch is read as today; any other error still goes through `claimError` with the same code and text as without the flag (all in `TestKeeperGuardTarget`). The prompt lines come from `req.KeeperCommit` inside `guardTaskLines` with its signature unchanged, and every other line is identical (`TestKeeperTaskLines`).

**Verification run here**

```
go build ./...      clean
go vet ./...        clean
gofmt -l .          no output
go test -race ./... ok in every package (internal/dispatch 227.9s, cmd/waggle 56.4s)
```

AC7 (`TestKeeperCommitGuards`) runs a first and a `--second` guard on `t-023k` with no builder and no claim, exits 0 for both, with `keeper_commit: true`, different models, verb `none`, empty claim OID and equal bases. `TestSecondGuardRefusals` and the T-034 model tests pass with only the zero-value argument added at call sites.

**Blocking Issues (Must-Fix Only):** none.

**Notes, not must-fix**

- `claim.Verb` becomes `none` when `req.KeeperCommit && claim.OID == ""` rather than on the `ErrNotFound` branch itself. If `ClaimOID` reported not-found while `ReadClaim` found a lease in the same instant, the record would say `none` after reading a claim. That window needs a concurrent ref write by the queen and changes a label only.
- `run.go` now calls `Resolve("builder")` on every `--second` run, keeper or not, and ignores the value on the non-keeper path. No observable behavior changes without the flag, but a follow-up could gate the call on `req.KeeperCommit`.
- AC1's "exact text" check for the no-flag refusal matches the prefix up to the runs path rather than the whole string. The string comes from the unchanged `secondMissing` branch, so the assertion still pins today's wording.

Verdict: pass