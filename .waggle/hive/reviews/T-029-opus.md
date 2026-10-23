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