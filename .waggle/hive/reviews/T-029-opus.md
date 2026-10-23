I found one blocking issue: `internal/dispatch/run.go:254` isn't gofmt-clean, so product-ci will fail and AC1 can't be met. Everything else in the change is correct.

**Commands run:** `go build ./...` passed, `go vet ./...` passed, and `go test -race ./... -run 'TestWaggleCommandsAndOutcome|TestInputsComeFromBase|TestProvisionExistingBranch|TestCutOfNamesTheCut|TestBranchPointLine|TestOffBranchReturnsBranchPoint'` passed (`internal/dispatch` ok). Following section 5, I ran no `TestRun*`, `TestGuard*`, `TestClean*` or `reap_*` tests. I couldn't check product-ci from here, and I wrote no probe files.

Risk-Tier: 3

**Intent Summary:** Commit `10f6fd8` changes four files, all allowed by the lease (`run.go`, `result.go`, `run_test.go`, `maintenance_test.go`); `go.mod` is unchanged.
- **Branch point:** `offBranch` now also returns the branch point (the merge-base for an existing branch, `baseOID` for a new one). A new pure helper, `withBranchPoint`, inserts the `Branch point:` line right after the exact `Base: <baseOID>` line. It is applied in `Run` to the lines from both `TaskLines` and `guardTaskLines`, so `inputs.go` and `guard.go` don't need to change, and the lines stay byte-identical when the two OIDs match.
- **Warnings:** `branch_moved` is added when the branch point differs from `Provision`'s `CheckBase`, and `branch_behind_base` when `CheckBase` differs from the base. Both carry only short OIDs that dispatch resolved itself. `Provision` stays the authority for `check_base_oid` and the checked range.
- **Progress line:** a `cutOf` helper now holds the cut detection that was in `outcome`. Both `outcome` and the `ended` emit use it, so a timeout prints `ended: timeout, …`. `end` in `result.json` and `outcome`'s results are unchanged.
- **Field docs:** `result.go` documents `base_oid`, `check_base_oid` and `cost_usd` as sections 2.1 and 2.3 ask, with no schema change.
- **Plant test:** it now unsets the repo-local `maintenance.auto` and `gc.auto` in `f.real`. The keys are set there (the fixture-quiet test asserts this), so the `--unset` calls succeed.

Blast radius is limited to the worker prompt text, run-record warnings and the stdout progress line. There are no new git calls, and the branch-point lookup reuses the existing `merge-base` call in `offBranch`. Nothing touches the push, the claim, the checked range or the revision inputs are read from. About 45 non-test lines were added, under the target of 85.

**Blocking Issues (Must-Fix Only):**
1. **gofmt (AC1):** in the `var (...)` block of `Run`, the line `in             Inputs` has 13 spaces before `Inputs`. Every other line in the block aligns the type at the column of `baseOID, runDir  string` (15 spaces after `in`). `gofmt -l` will list `run.go`, so product-ci will fail. Fix: realign that line to match its neighbours.

**Not a must-fix for the builder:** AC6 is unverified because `.waggle/hive/media/T-029/` doesn't exist at base `7e5e3ac`. The queen's Colima run of `TestRunPlantDuringAutoMaintenance` and its fix-removal check still need to be done, on the fixed head, before this can pass.

Verdict: must-fix
I couldn't check one thing: I have no `gh` access here, so whether product-ci is green on PR #80 still needs a look before merge.

# Review T-029 (guard), head `fb9501f`, base `60a9d57`

Verdict: pass
Risk-Tier: 3

## Intent Summary
The change makes dispatch say which base a reused branch is really on. `offBranch` now returns the branch point, which is `merge-base(branch, base)`, or the base OID for a new branch. A new helper, `withBranchPoint`, adds a `Branch point: <oid> (...)` line after `Base: <oid>` in both the builder and guard task lines, but only when the two OIDs differ. After `Provision`, `Run` adds a `branch_behind_base` warning when `CheckBase != baseOID`. It also adds a `branch_moved` warning when the branch point seen at prompt time differs from `Provision`'s value, which stays the one recorded. The cut detection in `outcome` is moved into `cutOf`, and the `ended` progress line now uses it, so a timeout prints `timeout` instead of `interrupted`. `result.json`'s `end` field does not change. `result.go` now documents `base_oid`, `check_base_oid` and `cost_usd`, and the schema is unchanged.

`TestRunPlantDuringAutoMaintenance` now forces auto gc through the work repo's local config. It seeds 40 loose blobs whose ids start with `0x17` after the plant's `mkdir` and before the second commit. The sanity check fails the test if a seed id does not start with `17`, and the assertion is unchanged.

Blast radius: the worker prompt changes only for reused branches that fork behind the base, plus two new warnings and the wording of the `ended` line. The revision inputs are read from, the checked range, the push and the claim are all unchanged. No new git call is added; the merge-base was already computed in `offBranch`.

## Checks
- **Lease (AC2):** the diff touches only `run.go`, `result.go`, `maintenance_test.go` and `run_test.go`. `go.mod` is unchanged.
- **AC1:**
  - `go build ./...` passed.
  - `go vet ./...` passed.
  - The `var` block alignment and the map literal in the new test both look gofmt-clean.
  - Product-ci: not checked (no `gh` in this environment).
- **AC3:**
  - `withBranchPoint` returns the lines unchanged when the branch point equals the base, so a new branch, or one forked at the base, gets byte-identical task lines.
  - Both `inputs.go:154` and `guard.go:39` write `"Base: "+baseOID` as its own line, so the insert finds it in both.
  - The warnings carry only OIDs that dispatch resolved itself.
  - `branchRev` and the `merge-base` call are the same in `offBranch` and `Provision`, so no false `branch_moved` warning comes from a mismatch.
- **AC4:** `outcome` gives the same results as before; `TestWaggleCommandsAndOutcome` passes. `TestCutOfNamesTheCut` covers all four cuts and the no-cut case.
- **AC5:** the doc comments match 2.1 and 2.3, and no field changed.
- **AC6:** the recipe is followed exactly.
  - Colima evidence on `fb9501f` (in `60a9d57:.waggle/hive/media/T-029/`), git 2.39.5, `--cpus 1`:
    - **With the worker-side keys:** PASS for both repo spellings.
    - **With `fix-removal-r2.diff` applied:** FAIL for both. The planted entry was pruned and the run reported no violations.
  - So the fix-removal check now does what it should.
- **AC7:** the new tests (`TestCutOfNamesTheCut`, `TestBranchPointLine`, `TestOffBranchReturnsBranchPoint`) do not call `Run` or `sweep()` and send no signals. There is no `t.Skip`.
- **Tests run:** `go test -race ./... -run 'TestCutOfNamesTheCut|TestBranchPointLine|TestOffBranchReturnsBranchPoint|TestWaggleCommandsAndOutcome|TestInputsComeFromBase|TestProvisionExistingBranch'` passed (`internal/dispatch` ok). No `TestRun*`, `TestGuard*`, `TestClean*` or `reap_*` tests were run.
- **Existing warning assertions:** they use `Contains` or `HasPrefix`. The new warnings are added after `lockWarns`, `w.Warnings` and `claim.Warnings`, and `TestRunClaimReadBackWarning`'s fixture builds a new branch, so none of these tests change.
- **Budget:** about 65 added non-test lines, under the target of 85.
- **Threat model (D-203):** there is no change to the input revision, checked range, push or claim. No unpinned or early git call is added. The task lines and warnings carry only OIDs.

## Blocking Issues
None.

## Notes (not blocking)
- Product-ci status on PR #80 must be confirmed before merge (AC1).
- Guard runs on reused branches will now also get the `Branch point` line, which is correct and expected.

No probe files were created.

Verdict: pass