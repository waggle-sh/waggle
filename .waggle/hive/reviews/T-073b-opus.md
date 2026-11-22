<!-- waggle-review v1 verdict=pass head=0ed00dc24febd4c99a3166b8c53a52e9df658bfe -->

Risk-Tier: 2

## Intent Summary

Phase B wires the phase A reader `comb.GitBackend.CaseCollisions` into `waggle status`. `loadStatus` calls it after `ListActiveLeases` and stores the result and any error in `statusData`. `renderStatus` prints one indented `warning:` line per collision directly after the claims table (after the malformed line, before the blank line and the usage block). A failed scan prints one stderr line through `collisionsWarning` and status still exits 0. Docs: `docs/multi-keeper.md` section 3 replaces "compared exactly, not case folded" with the folded rule, the same-owner renewal rule, the status warning and the older-binary residual; section 7 drops the T-073 bullet. `docs/RFC-0003.md` section 3.1 gets the new sentence with `(Amended: A3)`, the Status line names Amendment 3, and Appendix A gains an A3 entry. Blast radius: `waggle status` output (stdout gains lines only when a collision exists) and docs. Nothing writes a ref.

## Blocking Issues (Must-Fix Only)

None.

## Checks

- **Warning line (2.2 item 1).** Text: `  warning: <TA> <DA> and <TB> <DB> differ only in letter case; they are one directory on macOS and Windows. Release one, or ask the first login in gate.approvers to clear it with waggle force-release.` Matches the brief's sentence; T-072 phase A is on main (the malformed line already names `waggle force-release`), so naming it is the correct branch of Q-T073-6. Placed after the claims table (`cmd/waggle/status.go:241-244`). No dashes, no "Keeper"; `checkPlain` runs on both the render and the end-to-end line. The two-space indent matches the existing malformed line.
- **Terminal safety (threat model).** Only `TicketA/B` and `DirA/B` reach the line, all from active, well-formed claims (`c.reason == ""` filter in `internal/comb/git.go:981-985`), so ticket IDs are canonical and directories are in the ASCII safe set. Owners are not printed.
- **Failed scan.** `CollisionsErr` never becomes a `statusFault`; `runStatus` prints exactly one stderr line (`status.go:80-82`) and falls through to `return 0`. With an error, `Collisions` is nil, so no stdout lines.
- **No sync under --no-fetch / hives status.** `loadStatus` clears `opts.Remote` when `noFetch` (`status.go:143-145`) before constructing the backend, and `CaseCollisions` syncs only when `b.remote != ""`. `waggle hives status` calls `loadStatus(root, true)` (`hives.go:161`), so it still never fetches. Plain status keeps its sync.
- **Docs accuracy against phase A.** Folded overlap after exact (`checkOverlap`, `git.go:613-638`), same-owner renewal skips only the folded check for byte-identical held directories, write-time self check in `prepareLease` with `ErrInvalidLease` (`git.go:648-654`), `NormalizeDirectories`/`decodeLease` untouched so nothing turns malformed. The RFC sentence and A3 summary match. D-038 and D-380 exist in `DECISIONS.md`. A3 follows the A2 format.
- **Lease.** `git diff main...HEAD --name-only`: `cmd/waggle/status.go`, `cmd/waggle/status_collision_test.go`, `cmd/waggle/status_test.go`, `docs/RFC-0003.md`, `docs/multi-keeper.md`. `internal/comb`, `cmd/waggle/doctor.go`, `cmd/waggle/gate*.go`, `gate/`, `.waggle/`, `.github/`, `go.mod`, `go.sum` unchanged. Size: about 25 non-test lines plus docs, well under the 120 target.
- **Tests vs phase B plan.** B1: `TestRenderStatusCollisions` asserts no-collision output has no warning and the one-collision output equals the no-collision output with the line inserted before the usage block; the existing golden (`status_test.go:278`) passes `nil` and stays byte-identical. B1/B3 end to end: `TestStatusCaseCollision` forges two colliding claims, expects exit 0, empty stderr, one warning line. B2: `TestCollisionsWarning` (see note 2). B3: `checkPlain` on all new output. B4 is a doc check, verified by reading: no "not case folded", no section 7 bullet, no "deferred" in RFC 3.1, A3 present.
- **Commands.** `go build ./...` ok; `go vet ./...` ok; `gofmt -l .` empty; `go test -race ./cmd/waggle/` `ok github.com/waggle-sh/waggle/cmd/waggle 242.395s`. Added lines contain no en or em dashes.

## Non-blocking notes

1. **Plain `waggle status` now fetches twice.** `ListActiveLeases` and `CaseCollisions` each call `sync` in remote mode, so status pays a second network round trip, and the collision lines can come from a slightly newer snapshot than the claims table above them. A flaky second fetch surfaces as the scan warning, which is the designed behavior. Fixing it needs a comb reader that reuses one scan, which is outside this phase's lease (internal/comb frozen in phase B); worth a follow-up.
2. **B2 is covered only at the helper level.** `TestCollisionsWarning` checks the line's shape; no test drives `runStatus` with a failing scan to assert exit 0 and a single stderr line. The code path is short and correct by inspection, and forcing the second sync to fail while the first succeeds would need a test hook in `internal/comb`, which is out of lease.
3. **Wording nit, `docs/multi-keeper.md:55`.** "keep renewing but never grow or change hands" reads as if the claim can never add directories; phase A lets a grandfathered renewal add any directory that does not collide. "the collision never grows" would be exact.
4. `waggle hives status` drops `Collisions` and `CollisionsErr` without showing them. Not required by the brief; noted for the doctor/hives follow-up (Q-T073-5).

Verdict: pass
