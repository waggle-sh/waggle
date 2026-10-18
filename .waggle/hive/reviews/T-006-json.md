# Review: T-006 phase 2 inputs (backfill JSON D-088 to D-133)

Verdict: pass
Risk-Tier: 1

Reviewed: branch t-006-json, head 91d19d0, one commit on main 9015bc9. Draft PR #33. Gate: full. Evidence in .waggle/hive/media/T-006-json/. The downstream t-006-backfill (dddbf16) was not reviewed.

## Intent Summary

Adds the 46 missing backfill inputs `internal/decisions/testdata/backfill/D-088.json` to `D-133.json`, transcribed from `T-006-backfill.md` Appendix A, so the folder now holds D-001 to D-133 (D-106: forager adds the missing JSON, then runs `waggle decide`). Adds `internal/decisions/backfill_test.go`, which parses Appendix A and checks all 133 files field for field (decode, entry validation, DeepEqual, file count). Regenerates `testdata/backfill.golden` (append-only, 871 lines added) because `TestGoldenBackfill` renders the whole folder. No production code changes; 48 files, test data and one test only. Blast radius: `internal/decisions` tests; the next step (t-006-backfill) consumes these inputs to build DECISIONS.md.

## Blocking issues

None.

## Findings

- Transcription: I wrote a separate checker (media/check-appendix-a.py, not derived from the test's parser) that compares every JSON file to its Appendix A item. 133 items, 0 mismatches, including kind, tickets, supersedes, options split, "Not recorded" literals. Hand spot-check of D-088, D-099, D-100, D-108 (supersedes ["D-083"], kind strategy, 3 tickets, 2 options), D-120 (tickets T-024, T-023 in source order), D-133 against the brief headers: all match. 12 files have `"tickets": []` (the 12 "tickets none" items); that matches the mapping rule.
- Golden: `git diff 9015bc9 91d19d0 -- backfill.golden` shows 549 added lines and 0 removed; the stat reports 871 insertions (blank separators counted). Append-only holds. `TestGoldenBackfill` passes against it.
- Mutation: on a git-archive copy, changing D-099 kind, D-108 supersedes, D-120 choice casing, and deleting D-133.json each fail the new test with a clear message. Baseline copy passes. See mutations.txt.
- Skip risk: the test skips only on `fs.ErrNotExist` for the hive brief. The brief is tracked on main (`git ls-files` confirms), and product CI (`ci.yml`) uses `actions/checkout@v5` with no path filtering or sparse checkout and runs `go test -race ./...` with no `-skip`, so the file is present and the test runs. A skipped run would have shown SKIP in `-v`; here it shows PASS. Residual risk: a vendored or exported copy of the repo without `.waggle/hive/` would skip silently. Non-blocking.
- Gate: go build, go vet, gofmt -l (empty), `go test -race -count=2 ./...` all pass at 91d19d0 in a clean detached worktree. Diff files: 133 JSON (46 new), backfill_test.go (new), backfill.golden (modified). No em or en dashes in added lines. go.mod and go.sum untouched.

## Golden ruling

`internal/decisions/testdata/backfill.golden` is outside the stated lease text but is accepted. It is inside `internal/decisions/`, the module lease from WAGGLE.md section 2, it is not Tier 3, and `TestGoldenBackfill` (append_test.go) renders every file in `testdata/backfill/`, so adding D-088 to D-133 fails that test until the golden is regenerated. It is a necessary, mechanical consequence, regenerated with the existing `-update` flag. Follow-up for the brief text: say "testdata/**" rather than "testdata/backfill/" in the phase 2 lease.

## Non-blocking notes

1. The test and the checker both read the same Appendix A, so a transcription error in the Appendix itself is out of scope; Appendix A is the Keeper's source of truth.
2. Brief section 10 still mentions D-001 to D-085 and `seq -w 1 85`; the D-106 scope is now D-088 to D-133. Update the brief when the downstream branch lands.

## Intent Packet (for the Keeper)

- What: 46 transcribed decision inputs, a test that locks all 133 inputs to Appendix A, and the regenerated golden.
- Why: D-106 backfill; the inputs must exist before `waggle decide` records D-088 onward.
- Risk: Tier 1, test data only, no production code, stdlib only.
- Approve: yes, merge draft PR #33 once marked ready; then t-006-backfill can rebase on it.
