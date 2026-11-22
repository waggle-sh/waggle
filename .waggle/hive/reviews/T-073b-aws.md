<!-- waggle-review v1 verdict=pass head=0ed00dc24febd4c99a3166b8c53a52e9df658bfe -->

Risk-Tier: 2

**Intent Summary:** T-073 phase B. `waggle status` now calls the phase A `CaseCollisions` reader and prints one `warning:` line per case-only collision after the claims table. A failed scan prints one stderr warning and exit stays 0. `docs/multi-keeper.md` section 3 gets the folded rule, the same-owner renewal rule and the older-binary note. Section 7 drops the case-folded bullet. `docs/RFC-0003.md` section 3.1 is amended, with an Appendix A3 entry and a Status line update. Blast radius is `waggle status` output and docs only.

**Checked at 0ed00dc (all clean):**
- Warning text matches section 2.2. It names `waggle force-release`, the first login in `gate.approvers`, and uses no dashes and no `Keeper`. It prints after the claims table and the malformed line. `checkPlain` covers it, and the golden test with no collisions is unchanged.
- A failed scan goes through `collisionsWarning`, one stderr line, and `runStatus` still returns 0. `loadStatus` only fails on the existing ListActiveLeases and usage errors.
- Sync: `noFetch` sets `opts.Remote = ""` before the backend is built, so `CaseCollisions` does no sync there. `waggle hives status` (`readHive`, `loadStatus(root, true)`) still never syncs. It ignores collisions, as the brief allows. Plain `status` syncs as before. `status` has no `--no-fetch` flag in this tree.
- Docs agree with `internal/comb`. Overlap is ASCII folded (`caseOnlyOverlap` is `dirsOverlapFold` and not `DirsOverlap`). A same-owner renewal keeps held directories byte for byte (`git.go:569,611`). The gate's `case_differs` and head tree checks stay as the backstop. A3 and the Status line are consistent with section 3.1.
- Lease: only `cmd/waggle/status.go`, two status tests, `docs/multi-keeper.md` and `docs/RFC-0003.md` changed. `internal/comb`, `doctor.go` and the gate files are untouched.
- Tests: `TestRenderStatusCollisions` (B1 and B3), `TestStatusCaseCollision` (end to end with forged claims: one warning, exit 0, empty stderr) and `TestCollisionsWarning` (B2).
- `go build ./...`, `go vet ./...` and `gofmt -l .` are clean. `go test ./cmd/waggle/` passes. `-race` could not run because this image has no gcc (`CGO_ENABLED=1` fails), so I ran without it. The change adds no concurrency.

**Blocking Issues (Must-Fix Only):** None.

Non-blocking notes:
- The failed scan path is tested only through the `collisionsWarning` helper. No test injects a failing `CaseCollisions` through `runStatus`.
- In remote mode `CaseCollisions` syncs a second time after `ListActiveLeases`. That is one extra fetch per status run.

Verdict: pass

