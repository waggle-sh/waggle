<!-- waggle-review v1 verdict=pass head=bdc729e4f86efe0677445ed5fbf5d5baa85495a3 -->

Risk-Tier: 3

**Intent Summary:** T-044 fixes the bug behind the PR #155 gate failure. When `waggle dispatch` renews its own live claim on a branch the stored list doesn't name, the renewal now adds that branch to the list. Before, it kept the old list, so the gate failed the PR with `claim does not name this pull request's branch`.

- **Flag:** `Request` gains one unexported field, `addBranch` (`internal/dispatch/request.go:73-74`). `Run` sets it only on a local copy at the claim step (`run.go:360-362`). `ClaimTask`, `Validate` and `cmd/waggle/` can't set it, so `waggle claim` keeps the D-292 behavior.
- **Renewal rule:** in `AcquireClaim` (`claim.go:67-72`), a dispatch renewal keeps the stored list if it already names `req.Branch`. Otherwise it builds a new list with `slices.Concat` and puts the branch at the end. The stored slice is never changed in place.
- **Full list:** if the list already has 4 entries and lacks the branch, the call refuses before any write. It returns `ExitClaim`, the message names no branch, and it wraps `comb.ErrInvalidLease` (`:68-69`).
- **Unchanged:** a fresh claim, a takeover (expiry is still checked before keeper, `:60`), a renewal with no stored list, a request with `ClaimBranches`, the guard refusal, `claimError` and the keepalive closure.
- **Blast radius:** only `internal/dispatch/`. It reaches the hive only after the Head Keeper moves the pin. The diff is 4 files: 3 code files with about 13 added non-test lines, well under the target of 39, plus the new test file `renew_branch_test.go` (198 lines). The planned cut wasn't needed. No existing test, assertion or fixture changed.

**Checks run at head `bdc729e`:**
- `go build ./...`, `go vet ./...`: clean. `gofmt -l .`: no output.
- `go test -race ./...`: every package ok (`internal/dispatch` 228.8s).
- The six new tests run and pass when selected with `-run ... -v`; none is skipped. They cover:
  - AC1 and AC2: same branch, append, re-listing, and a fork entry that isn't the plain branch name.
  - AC3: the full-list refusal leaves the claim ref at the same OID, and the message contains no list entry and not `t-023c`.
  - AC4: goes through `Run` with `d.Now` pinned to the `claimEnv` clock. The verb is `renew`, there's one note, and every recorded lease is `[t-023 t-023b]`. It uses no `Timeout`, cancel or signal, so D-193 and D-195 are met.
  - AC5: `ClaimTask` keeps `[t-023b]`.
  - AC6: a takeover after expiry, and a renewal of a claim with no stored list.
  - AC7: the guard is refused with or without the flag, and nothing is written.
- `renew_branch_test.go` has the SPDX header and `//go:build unix`.
- `git diff -G "[–—]"` finds no added em or en dash.
- Lease rule: only `claim.go` (the renewal case and the `AcquireClaim` doc comment), the `Request` struct, the `run.go` claim step and one new `_test.go` file changed. `AcquireClaim`'s exported signature, `ClaimTask`, `Validate`, `checkBranch`, `claimError`, the keepalive `renew` closure, `task.go`, `release.go` and `internal/comb/` are untouched.

**Blocking Issues (Must-Fix Only):** None.

**Non-blocking notes:**
- `claim.go:36`: the new doc sentence was put in front of the existing text without re-wrapping, so the line is very long. It's cosmetic; gofmt accepts it.
- The `Claim` doc comment (`claim.go:23-25`) still says a renewal note shows "role, gate or directories" changes. It could mention branches too, but the lease allowed this edit without requiring it.

Verdict: pass