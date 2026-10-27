<!-- waggle-review v1 verdict=pass head=bdc729e4f86efe0677445ed5fbf5d5baa85495a3 -->

Risk-Tier: 3

**Intent Summary:** One commit on `t-044` makes a `waggle dispatch` renewal append its run's branch to the claim's stored branch list when the list does not already name it, so a later-phase run's pull request passes the gate's branch check without the queen's manual `waggle claim --branch` step. `Request` gains an unexported `addBranch` flag, set only on a local copy inside `Run`'s claim step; `AcquireClaim` honors it in the same-keeper renewal case, refusing with `ExitClaim` wrapping `ErrInvalidLease` before any write when the stored list already has four entries. `waggle claim`, takeover, fresh claim, the guard, `claimError`, `Validate` and the keepalive closure are untouched. Blast radius is the claim ref content for dispatch renewals only. The hive sees no change until the pin moves past this merge.

**Verification**

| Check | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | no output |
| `go test -race ./...` | all packages ok, including `internal/license` (SPDX) |
| Non-test added lines (numstat) | 13, under estimate 30 and target 39 |
| Files changed | `claim.go`, `request.go`, `run.go`, new `renew_branch_test.go` only |

**AC walk-through**

- **AC1, AC2:** `TestDispatchRenewalBranches` passes. Same branch keeps `[t-023]` with no note; gate change gives one note with `[t-023]` twice; `t-023b` appends with a note naming both lists; the fork-entry case records `[copilot/x owner/repo:t-023 t-023]`.
- **AC3:** `TestDispatchRenewalFullList` passes. Refusal is `ExitClaim`, `errors.Is ErrInvalidLease`, contains `already names 4 branches`, names none of the four entries nor `t-023c` (the message prints `T-023` uppercase from `Validate`, so no lowercase entry is a substring), and the claim OID is unchanged. The refusal returns at `claim.go:69` before `AcquireLease` at `:80`.
- **AC4:** `TestRunRenewalAddsBranch` passes through `Run` with `d.Now` pinned to the `claimEnv` clock, `KeepaliveEvery` set, no `Timeout`, no cancel, no signal. Verb is `renew`, one note, at least two leases all carrying `[t-023 t-023b]`. The expiry-before-keeper order at `claim.go:60-63` is unchanged.
- **AC5:** `TestClaimTaskRenewalKeepsList` passes; `ClaimTask` never sets the flag (grep shows the flag set only at `run.go:361` and in the new test file).
- **AC6:** `TestDispatchClaimResetsList` passes for both the takeover and the no-stored-list renewal.
- **AC7:** `TestAcquireClaimGuardRefused` passes with the flag off and on; `ExitUsage`, no ref written. `Run`'s guard path at `run.go:359` still skips the claim step.
- **AC8:** `AcquireClaim`'s exported signature, `ClaimTask`, `Validate`, `checkBranch`, `claimError` and the keepalive closure at `run.go:406-418` are byte-for-byte unchanged in the diff. The field is unexported and `Validate` does not touch it.
- **AC9:** new file has `// SPDX-License-Identifier: Apache-2.0` on line 1 and `//go:build unix`.
- **AC10:** a grep for em and en dashes over the four changed files finds none.

**Threat model (section 5) check**

- Append uses `slices.Concat`, a new slice; stored order is kept and only `req.Branch` is added. `slices.Contains` is an exact match, the same as the gate's `e == h.name` at `gate_commits.go:364`.
- Duplicate and over-4 lists cannot be written by this path; `ValidateBranches` at `backend.go:494-500` still backstops.
- No existing test file or assertion changed (diff touches no `*_test.go` other than the new file), consistent with the T-034 rule and section 6.

**Notes, not must-fix**

- The `Claim` doc comment at `claim.go:23-25` still says a renewal note covers "role, gate or directories"; branches have been in the note since T-039, so this was already stale at base. Optional one-word fix in a later round.
- The `AcquireClaim` doc comment edit at `claim.go:36` leaves one long line; style only.

**Blocking Issues (Must-Fix Only):** none.

Verdict: pass