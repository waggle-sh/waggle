<!-- waggle-review v1 verdict=pass head=273d5e0601a24ba7e9e94ead25cb7709bbd346e7 -->

Risk-Tier: 3

**Intent Summary:** T-034 changes how dispatch picks a guard's model, in `internal/runtime/policy.go`, `internal/dispatch/guard.go` and `internal/dispatch/run.go`, plus tests. It makes three changes:
- **Alternation:** a plain guard on a Tier 3 or unknown-tier task avoids the previous round's guard model, read from run records, and avoids the builder record's model on every tier. If no other model exists it repeats the model and adds `confirm_model_repeated`. It adds `confirm_model_unknown` when a must-fix round exists but no record does.
- **`--second`:** it takes the first guard's `base_oid` as its default base. `checkSecond` now needs both records on the head under review, hex-checks their bases, and requires the second guard's base to equal the first guard's. The builder's base may be an ancestor, checked with `git merge-base --is-ancestor`, and a git error always refuses.
- **Hint:** the second guard's `ErrNoCrossModel` now names the level whose alternate would resolve.

For a guard run only, `run.go` moves the model steps after `guardTarget`. Builder and forager step order is unchanged. The blast radius is the model choice of every guard run. The cross-model protection (guard model never equals the builder's or this round's first guard's) is kept and extended by the builder record.

**Checks run:**
- `go build ./...`, `go vet ./...` and `gofmt -l .` are clean (the last prints nothing).
- `go test -race ./...` passes in every package, including `internal/dispatch` and `internal/runtime`.
- The diff touches only the lease paths: `internal/dispatch/{guard,run}.go`, `internal/runtime/policy.go` and `*_test.go` files in the two directories. `cmd/waggle/`, `.waggle/` and `go.mod` are untouched.
- No em or en dashes appear anywhere under `internal/` (AC9).
- The changed existing assertions match the brief's sections:
  - `TestSecondGuardHiveShapeRefuses`: `alternates.frontier`, 2.3.
  - `TestSecondGuardWalk` `floor` subtest: `alternates.standard`, 2.3.
  - `TestSecondGuardRefusals`: "base moved" split into the two Q-T034-4 (a) checks, "review committed" flipped to a run on the first guard's base, and "model repeats" now expects `builder "<model>"`.
  - `guardrun_test.go`: only the stale comment above `TestMustFixCountPerPhase`.
  - I did not see a PR body in this review, so I could not check the list of changed assertions the brief asks the builder to put there.

**Review notes (not blocking):**
- **Run records:** the lookup in `latestRun` filters on ticket, branch, `second` and a non-empty verdict, with no `end` filter. It compares `<seq>` as a number and skips non-directories and malformed or oversize records. It satisfies the threat model's record-counting rules, including the `T-0340` prefix, which cannot match `T-034-guard-`.
- **Model avoidance:** the builder record's model is always in `AvoidModels`, so the stepped-up case (record sonnet, policy builder opus) cannot land on sonnet. `checkBuilderModel` and `checkSecond` rule 5 stay as defense in depth.
- **Round 1 plain guard:** a Tier 3 plain guard whose builder record differs from the policy's builder model can now walk down where it used to return `ErrNoCrossModel`. It never lands on an avoided model, so this only turns an error into a result. It is a small extension of Q-T034-5, which the Head Keeper has ruled.

**Blocking Issues (Must-Fix Only):** None.

Verdict: pass