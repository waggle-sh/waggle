<!-- waggle-review v1 verdict=pass head=103b8d142c8c2cd6007fa8e39a5f996d67894198 -->

Risk-Tier: 3

**Intent Summary:** T-045 adds a guard-only `--keeper-commit` flag to `waggle dispatch`. With it, the queen can run both D-205 guards on a branch the Head Keeper committed, as in the T-025 phase B case. Before this, the second guard there had to run as a Claude Code subagent with no run record.

Under the flag, the rule that a builder record must exist on the branch becomes a rule that it must not. Plain and `--second` runs refuse with exit 3 if `builderRun` finds a record on the branch; this check is `keeperBuilder` in `guardModel` and `checkSecond`.

For `--keeper-commit --second`, dispatch needs only this round's first guard record. That record must be on the same head, be a plain run (`second: false`) and have a verdict. Dispatch avoids that guard's model through `AvoidModels`, and `RecordedBuilder` stays empty. `checkSecond` gets a keeper branch that skips `merge-base` and runs the five checks from brief section 3.4, including the re-check against the policy's builder model.

`guardTarget` accepts a missing claim under the flag (verb `none`, empty claim OID, `TicketID` set). It refuses a claim whose branch list names the branch, and the guard prompt shows `none (Head Keeper commit)` and `full (Head Keeper commit)`. `Validate` refuses the flag on any non-guard role (exit 2). `Result` gains `keeper_commit`.

Blast radius: `internal/dispatch/` (`guard.go`, `run.go`, `request.go`, `result.go`) and `cmd/waggle/dispatch.go`, all inside the lease rule. Nothing in `internal/runtime/`, `.waggle/` or `gate/` changes.

**Verification**
- **Without the flag, nothing changes:**
  - In `guardModel`, `!okF || (!okB && !keeper)` is the same as the old `!okB || !okF` when the flag is off, and `secondMissing(..., false)` keeps the old wording exactly.
  - `checkSecond` runs today's path unchanged.
  - `guardTarget` still exits 4 with the old text.
  - `guardTaskLines` is unchanged byte for byte (`TestKeeperTaskLines`).
  - Existing tests changed only at call sites, each passing `false` or `""`.
- **First-guard lookup:** `latestRun` matches the `<ticket>-guard-` prefix with a strict `Atoi` round trip, and checks `c.Ticket == ticket`, `!Second`, branch, verdict and head. So a `T-0250` record, a record with `second: true`, a record with no verdict, or a record on another head or branch never counts as this round's first guard.
- **Hex check before git:** under the flag, `f.BaseOID` is checked as hex before anything else uses it, and the keeper path runs no git command.
- **Post-run check:** a guard still has every ref change flagged. The claim OID check is skipped only when `claimOID` is empty. Nothing in the flag's path writes, moves or deletes a claim ref.
- **New call in `run.go`:** the `checkSecond` step now calls `l.policy.Resolve("builder", &pb, nil)` on every `--second` run. It works on a copy, `Resolve` does not change its task, and the result is used only on the keeper path. So it adds no behavior or message change for runs without the flag. It matches how `guardLevel` computes the builder model, step-up included.
- **Commands:**
  - `go build ./...`, `go vet ./...`: clean.
  - `gofmt -l .`: empty.
  - `go test -race ./...`: all packages `ok`, including `internal/dispatch` (230s). The new tests passed: `TestKeeperSecondModel`, `TestKeeperSecondNoAlternate`, `TestKeeperRefusesBuilderRecord`, `TestKeeperCheckSecond`, `TestKeeperGuardTarget`, `TestKeeperTaskLines`, `TestKeeperValidate`, `TestKeeperCommitGuards` (AC7, local mode, no push) and `TestDispatchHelpListsKeeperCommit`.
- **Acceptance criteria:** AC1 to AC8 are each covered by those tests.
- **AC9, AC10:**
  - The new files carry the SPDX header, and `keeper_run_test.go` has `//go:build unix` under it.
  - No en or em dash appears in `internal/dispatch` or `cmd/waggle` Go files.
  - Non-test additions are about 75 lines, under the 120 target, so no cut was needed.

**Notes (not must-fix)**
- On runs without the flag, the builder-model `Resolve` call is computed and thrown away. It costs little, but it could move inside a `req.KeeperCommit` guard.
- The keeper path checks a missing first guard before the builder-model rule; the brief lists 3.2 first. A builder record still refuses before any other check: `guardModel` runs first and calls `keeperBuilder` before anything else.

**Blocking Issues (Must-Fix Only):** None.

Verdict: pass