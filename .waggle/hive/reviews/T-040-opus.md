<!-- waggle-review v1 verdict=must-fix head=5c27ffbcbbc451bab11f4d2693c427594dcea99c -->

Risk-Tier: 3

**Intent Summary:** T-040 changes wording only, so a gate-only user no longer reads bee names in what Waggle prints or installs. One commit (`5c27ffb`) on top of base `4f8f558` changes 27 files, all inside the five-directory lease. It rewords 35 strings from section 3: rows 1 to 28 and 30 to 36, with row 29 left out as round 4 requires. It also:
- adds `fs.PrintDefaults()` to the `waggle claim` and `waggle release` usage closures (Q-T040-7 (a)), so `-h` now lists their flags;
- splits `printDecision` with a new `upgradePlan.gateOnly` field, taken from the `isGateOnly` value `planUpgrade` already computes. A gate-only repo gets the row 21 to 25 decision input with an empty `tickets` and no T-/D- IDs. A full hive prints exactly today's block;
- renames one gate workflow step and two gate-config comments.

The blast radius is everything outside users see: gate findings, claim and release errors, help text, the `waggle status` header, the upgrade and uninstall last lines, and every installed gate workflow at the next pin move.

**What I checked:**
- **Commands:** `go build ./...`, `go vet ./...` and `gofmt -l .` all print nothing. `go test -race ./...` passes in every package, including `internal/dispatch` (220s) and `internal/forge`. No push-fixture failures.
- **Exact rows:** every changed literal matches its section 3 "exact" text, with placeholders and their order kept.
- **Unchanged behaviour:** the `HasPrefix("Head Keeper: ")` test, the sentinel identities (`ErrWaggleDir`, `ErrNotOwner`), exit codes and `not_merged` (`task_test.go:334`, `claim_test.go:160`) are unchanged. Row 1 stays distinct from the listed-path text (`gate_bind_test.go:239`).
- **Templates:** `waggle-gate.yml.tmpl` changes only the step name, with the `run:` line identical. `gate-config.yaml.tmpl` changes only lines 2 and 14.
- **Lease:** `claim.go` changes only the two `--owner` strings and the two `PrintDefaults` lines. `status.go` changes only lines 77, 164, 166 and 168. Nothing changes in `internal/forge`, `internal/config`, `.waggle`, `.github` or `go.mod`.
- **Changed assertions:** every edited existing assertion maps to its row: row 1 (`policy_test.go`, `gate_bind_test.go`), row 2 (`gate_bind_test.go`, `gate_verify_test.go`), row 6 (`dispatch_test.go`, `release_test.go`), row 9 (`git_test.go`), row 18 (`upgrade_test.go`), rows 26 and 28 (`decide_test.go`) and rows 33 to 36 (`status_test.go`). No `t.Skip` was added.
- **Bee words left in changed files (section 7 Grep):** each one is kept for a stated reason:
  - `uninstall.go:88` (`stop waggled first`) is a `waggled` message, hive only.
  - `main.go:68` and `:74` are the full `init` lines.
  - `backend.go:43` and `:50` quote the `.waggle/hive/` path.
  - `gate_commits.go:265-266` keeps the `Head Keeper: ` prefix literal (Q-T040-1).
  - The full-hive branch of `printDecision` keeps today's text by design.
- **Dashes:** no en or em dash appears anywhere under `cmd/` or `internal/`.
- **New tests:** they cover AC3 (through the changed assertions), AC6 (gate-only decode, `decisions.Next`, no `T-0`/`D-0`, plus a byte-for-byte full-hive golden), AC7, AC8, AC9, AC10 (including `claim -h` and `release -h` listing flags) and AC10b (a gate-only `waggle status` with a local bare `origin`, a fetch and not a push).

**Blocking Issues (Must-Fix Only):**
1. **AC4 is not tested (rows 4, 7, 8).** Section 8 says every criterion not marked "review" is a test. AC4 asks for `VerifyTransition` on a corrupt base to return an error containing `repair needs an admin merge`, and for the `ErrWaggleDir` and `ErrNotOwner` texts to be the row 7 and 8 texts. No test asserts any of these. Grep over `*_test.go` for `repair needs` and `fails integrity` finds nothing, and the sentinel tests only use `errors.Is`. Only the "bad owner names `owner`" part is covered (`git_test.go:967`). The fix is a test change in `internal/decisions` and `internal/comb`.
2. **AC5 is not tested for row 5.** No test checks that `ReleaseTask` on an unmerged branch carries the new tail (`if the task merged by squash or rebase, or makes no PR, release it with --unmerged`). Grep for `squash or rebase` and `is not merged into main` in tests finds nothing; the existing tests check only `ExitClaim` and `not_merged`. Row 6 is covered by the changed assertions in `release_test.go:174` and `dispatch_test.go:235`. The fix is a test-only addition in `internal/dispatch`.

**Notes (not blocking):**
- AC8's test checks that `- name: Fetch claim refs` is followed by a `run: ` line, but not that this is the claim-fetch line. Asserting the full `git fetch origin 'refs/waggle/claims/*:refs/waggle/claims/*'` line would match the AC exactly.
- AC10b's first-line check uses a prefix and a suffix around the pin rather than an exact string; that is acceptable.
- AC12 (PR body trailers and the changed-assertion list) can't be checked from the worktree; the queen should confirm it on the PR.
- The fix round is test-only and touches none of the files listed in the brief's Risk Tier, so the brief and D-205 call for one confirming guard on the other model.

Verdict: must-fix