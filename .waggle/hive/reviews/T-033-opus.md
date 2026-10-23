**Risk-Tier: 3**

## Intent Summary

T-033 fixes three things the T-032 cycle surfaced, in one commit (`5d6a4d2`, 8 files, all inside the lease rule).

1. **The guard writes no file in the repo.** The guard role template no longer names `.waggle/hive/reviews/` or `.waggle/hive/media/`. It says the review is the final message, puts evidence there too, and asks for `Risk-Tier:` near the top with the `Verdict:` line last and only once. The roster guard entry now has `"owns": []` and a matching `purpose`. The `Rules:` task line gains `write no file in the repo; your final message is the review` and keeps the `do not commit`, probe-file and last-line verdict clauses word for word.
2. **One verdict per review round.** `reviewFile` writes `review.md` as `<!-- waggle-review v1 verdict=… head=<req.Head> -->`, a blank line, then the final message. The marker is written only when a verdict was parsed and the head is 40 or 64 lowercase hex. On every write, any line of the message that starts at column 0 with `<!-- waggle-review` gets one leading space. `final.md` is unchanged. `countReviewMustFix` counts text before the first marker with `runtime.CountMustFix` (unchanged); after the first marker, only exact marker lines count, after trimming trailing spaces and `\r`. `ReadInputs` and `roundMustFix` now call it, with the same caps and the same file selection.
3. **The gate workflow template catches up with the hive.**
   - Workflow permissions are `contents: read`, and the verify job also gets `contents: read`. Write permissions appear only on `relay-claim`.
   - `|| true` is gone from the claim fetch, and the echo-only commit loop is removed.
   - A new `src` step checks the base revision for `cmd/waggle/gate.go`. Its result gates setup-go and a gate step that builds from the base and runs `gate verify`.

**Blast radius:**
- Every guard prompt and rendered `guard.md`/`roster.json`.
- The builder step-up count, for review rounds written by a binary past this merge.
- The `waggle-gate.yml` that `waggle init` ships.

The hive sees no effect until the next pin move.

## Verification run

| Command | Result |
|---|---|
| `go build ./...` | ok, no output |
| `go vet ./...` | ok, no output |
| `gofmt -l .` | empty |
| `go test -race ./...` | all packages ok (dispatch 213.9s, scaffold 10.9s, runtime 5.8s, cmd/waggle 34.2s) |

## AC checks

- **AC2:** The diff touches only `guard.go`, `inputs.go` (the two call swaps), `run.go` (the `review.md` line inside `if guard`), `roles/guard.md`, `roster.json.tmpl` (guard `purpose`/`owns` only), `waggle-gate.yml.tmpl`, and two new `_test.go` files. Nothing changed in `internal/runtime/`, `cmd/waggle/`, `.github/`, `.waggle/`, `go.mod` or any existing test. No `t.Skip` was added.
- **AC3 and AC4:** `TestGuardTemplateWritesNothing` and `TestGuardTaskLinesRules` cover these. I also read the rendered text myself.
- **AC5:** `TestGuardReviewMarker` runs through `guardFixture`. It covers a verdict run, a quoted marker on a pass run, a quoted marker on a `no_verdict` run, and a cut final. It also checks that `final.md` is byte for byte the final.
- **AC6:** `TestCountReviewMustFix` covers:
  - the legacy table compared against `CountMustFix`
  - three rounds counting 2
  - a legacy part plus a marker counting 2
  - leading space, uppercase hex, 39-hex, `v2`, `verdict=fail` and trailing text all rejected
  - `\r` and trailing spaces accepted, and a 64-hex head accepted
  - an escaped marker read back not counting
- **AC7:** `TestInputsMarkerRounds`: `roundMustFix` returns 2 (the old rule gives 4). `ReadInputs` returns 4, which is the review file's 2 plus the phase files' 2, since `ReadInputs` adds both. That confirms the review file alone counts 2, as AC7 requires.
- **AC8:** `TestGateTemplateBaseBuilt` asserts every listed string, the permission placement, and exactly two `present == 'true'` guards. The existing init and upgrade tests pass unchanged.
- **AC9:** I compared the template with the hive's `.github/workflows/waggle-gate.yml`. Lines 1 to 67 are identical: header, `on:`, permissions, relay job, verify job permissions, fetch without `|| true`, and the tier `case` step. The only differences are the three from section 2.3:
  1. The base-commit and `gate.go` presence checks moved into the `src` step, which has its own `BASE_SHA` env and writes `present=` to the step output. Both setup-go and the gate step are gated on it.
  2. There is no `gate_verify.go` fallback to `gate decisions`.
  3. The `T-007, Q-T007-1` comment line is gone. The gate step now has a short comment saying the checker is built from the base revision and a release binary replaces the step later (G4).
  
  Every check reads `$BASE_SHA` and nothing reads the PR head. Actions' default `bash -e` makes a failed worktree add or build fail the step.
- **AC10:** No em or en dash appears in the changed files. The new template text uses plain terms.
- **Section 5 threat-model must-fix list:** I checked each item and none applies. The escape runs at column 0 on every write. Markers are written only when a verdict was parsed. No-marker files count exactly as `CountMustFix`. No read goes past the existing caps.

**Budget:** about 100 added non-test lines, under the target of 290.

## Blocking Issues

None.

Notes (not must-fix):
- AC7's fixture checks the review file's count through the `ReadInputs` total, not on its own. The arithmetic still pins it to 2.
- The `|| true` check in the template test covers the whole file, so it also guards the gate step. That is stricter than the AC asks.

I wrote no file in the repo.

Verdict: pass