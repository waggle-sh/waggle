# Guard review: T-032 brief (PR #91)

Verdict: must-fix
Risk-Tier: 2

Subagent guard hand-back (opus), round 1 at 12df5c2, read against main 3567764. `scan-hive.sh 3567764 12df5c2` exits 2 (no hits; one added file outside the records set, as expected for a brief). The queen reproduced M2 in a scratch repo on 2026-10-23: after a plain fetch, a pushed tag `origin/main` on a branch commit makes `git rev-parse --verify "origin/main^{commit}"` warn "refname is ambiguous" and resolve to the tag's commit.

## Intent Summary
PR #91 adds .waggle/hive/tasks/T-032.md, the brief for gate step G2 (D-229). It adds top-level `waggle claim` and `waggle release` verbs with JSON output. Both are thin wrappers over the new `dispatch.ClaimTask` and `dispatch.ReleaseTask`, which reuse `AcquireClaim`, `AcquireLease`, `ReleaseLease` and the T-030 merged check. It also applies `gate.protected_paths`, read from the base revision, in three places: in `prepareLease` through a new `comb.Options.Protected`, in a new `Run` step after `ResolveBase`, and in dispatch's post-run check through `CheckChangesPolicy`. Lease is `cmd/waggle/`, `internal/comb/`, `internal/dispatch/`, Tier 3, gate full, two guards. Blast radius: every claim write (dispatch included), dispatch's post-run verdicts, and the `waggle dispatch release` path, which is kept as a byte-identical wrapper.

## Checks
1. Named code: pass, with one exception. `AcquireLease`, `prepareLease`, `comb.Options`, `NewGitBackend`, `CheckChanges`/`CheckChangesPolicy`/`TierPolicy`, `refuseDirs`, `Release`/`branchMerged`/`releaseError`, `blobAt`, `ResolveBase`, `printable`, `config.GateConfig`, `MinTTLMinutes`/`MaxTTLMinutes`/`MaxFileSize`, `LoadKeeperID` and the test fixtures all exist and behave as described. `blobAt` (inputs.go:60-88) refuses a symlink, tree, submodule and oversized blob, matching `baseTierPolicy` (gate_verify.go:199-233). The exception is line 31: `Request.Validate` does not do the hardcoded Tier 3 refusal (N2).
2. Lease rule: fails for the `locked` sentinel (M1), and the M2 fix needs inputs.go. The `internal/config/` claim holds: `CombOptions` leaves the new `Protected` field zero, `ErrNoKeeperID` is already exported, and comb cannot import config, so `basePolicy` correctly lives in dispatch.
3. Raise-only and base read: the matching is sound. `CoversDir` with `touchesPrefixes` semantics covers every lease directory under which a file could match an entry. Q-T032-4's reasoning holds (git.go:560 skips malformed claims in the overlap check). The post-run check reads the run's `baseOID`. Weak point: the default base revision itself (M2).
4. JSON, exits, stdout and stderr: mostly complete. The error-code mapping is ambiguous (N1) and the push tripwire list is mislabeled (N4). Tests that need a push to a local origin: AC8, plus the existing `TestReleaseClaimRemote` (release_test.go:202) under AC9. Everything else runs locally.
5. Budget: pass. Bottom-up 255, doubled to 510, target 660, stop 920; the ratios match T-031. The cut (release widenings move to a follow-up) is a real split. `claim.go` at 95 lines looks light (N11).
6. D-193: no kill signals, but the sentence is loose (N6). D-236: AC11's list is incomplete, and a new sentinel would carry a `comb: ` prefix (N7).
7. Questions: Q1 to Q5 recommendations are sound; no question is missing that blocks the build. N9 should be stated.
8. Style: no em or en dashes, emails or home paths. "alice" and "bob" are placeholders. There is one raw U+2028 (N8).

## Must-fix
M1 (brief line 79, lease rule line 10, AC7 line 135): the `locked` code cannot be built inside the lease.
- Line 79 asks for unexported sentinels "wrapped with %w where those errors are made today". The held-lock errors are made in `lockRepo` (clean.go:77-79) and `takeLock` (worktree.go:493, 496, 504).
- The lease rule forbids clean.go and does not list worktree.go. `lockRepo`'s other errors (rev-parse, `worktreesDir`) share the same return path, so `ReleaseTask` cannot tell them apart, yet AC7 requires exit 3 `locked`.
- Fix, either way: allow worktree.go (`takeLock` only) to wrap an `errLocked`, or drop the `locked` code (a held lock becomes `environment`, exit 3) and amend AC7.

M2 (lines 45, 83, 52; internal/dispatch/inputs.go:46-50): the default base can be shadowed by a pushed tag.
- `ResolveBase` runs `git fetch <remote>` (default tag auto-follow; no `--no-tags` in `Git.RunIn`, worktree.go:52), then `rev-parse --verify "origin/main^{commit}"` (or `main`) with an unqualified name.
- By gitrevisions order, `refs/tags/<name>` beats both `refs/heads/<name>` and `refs/remotes/<name>`. A contributor who pushes a tag named `origin/main` (or `main`) on a commit of a pushed branch gets it fetched, and the tag then wins.
- Effect 1: `basePolicy` would read protected paths from the attacker's commit, which the brief's own must-fix list rules out (line 111: "protected paths read from anywhere but the base revision").
- Effect 2: `branchMerged` (release.go:125-147) would test merges against the attacker's history, so a crafted merge commit passes the merged check.
- This is pre-existing (T-023 `ReadInputs`, T-030 release), but T-032 makes it the anchor for lease-time policy and the new release verb.
- Fix: resolve the default as `refs/remotes/<remote>/main` or `refs/heads/main`, fully qualified. Add `internal/dispatch/inputs.go` (`ResolveBase` only) to the lease rule; this also protects dispatch's inputs. Add an AC: a fetched tag `origin/main` and a local tag `main` change neither the policy base nor the merged check.

## Notes
N1 (line 79): `usage`, `invalid_task`, `interrupted` and `environment` are not sentinels.
- If `usage` means `ExitCode == 2`, it comes first and makes `invalid_task` unreachable, since `ErrInvalidTicketID` is wrapped in `ExitUsage` (request.go:85-88).
- No line says which sentinel yields `protected_path`. AC4 (line 132) expects it for `vendor`, which only wraps `ErrTier3Path` (backend.go:459).
- State it: `protected_path` means `ErrTier3Path` (which covers `ErrProtectedPath` and `ErrWaggleDir`), `invalid_task` comes before `usage`, and `usage` and `environment` fall back on exit code.

N2 (line 31): `Validate` refuses only the repo root (request.go:101-106 via backend.go:352). `ClaimTask` refuses `vendor` only at `AcquireLease`, after a fetch and the base read. The outcome is correct; either reword, or call `refuseDirs` from `task.go`, which leaves `run.go` unchanged.

N3 (lines 31, AC3 line 131): AC3's fake clock and any injected backend need `Now` and `Backend` fields on `ClaimTaskRequest`. The brief names no fields for it; `ReleaseRequest` already has `Backend` (release.go:23).

N4 (line 123): "AC6 and AC8 remote cases" is wrong, since AC6 has no remote case. The push-dependent tests are AC8 and the existing `TestReleaseClaimRemote` under AC9.

N5 (line 52): with no `--branch`, either head passing counts. A contributor can push `waggle/T-<nnn>` at another PR's merged head even when `t-<nnn>` exists and is unmerged. Today the same works only when `t-<nnn>` is absent, so this is a note.
- Suggestion: when `t-<nnn>` exists, check only that head.
- The release `--branch` suffix set (`[A-Za-z0-9._-]`) is also stricter than the gate's `(?:-.*)?` (gate_commits.go:36). Those users need `--unmerged`; say so.

N6 (line 117): the release path reaches `takeLock`, then `processAlive`, which sends `Signal(0)` (worktree.go:138, 495). It is harmless, but "nothing sends a signal" is loose. Require AC7's lock test to use `os.Getpid()`, as release_test.go:122 does.

N7 (AC11 line 139): the list omits Nectar and Quorum (D-236). A new `ErrProtectedPath` written in the existing `comb: ...` style (backend.go:144-172) would trip AC11. Give its text in the brief.

N8 (AC10 line 138): the fourth character is a raw U+2028 between backticks. Write it as `U+2028`.

N9 (lines 31, 50): both verbs still need the working-tree `.waggle/control/config.yaml` (`config.Load`, release.go:49) and exit 3 without it. Say so, and note that G4 `init --gate-only` writes it.
- `LoadKeeperID` also returns `ErrNoKeeperID` for the placeholder `keeper_id` when a remote is set (internal/config/local.go). The "no owner" exit 2 message would mislead there.

N10 (line 10): name the file for `basePolicy` (`task.go`). Allow the `AcquireLease` doc comment (git.go:509-511) so it can say `Protected` applies at write time only; it is outside the git.go allowance as written.

N11 (line 143): `cmd/waggle/claim.go` at 95 lines is light for two flag sets, the JSON structs, the code mapping and both printers. 120 to 130 is likelier. The doubled estimate still clears the target.

N12 (out of scope): M2's tag shadowing also lets a pushed tag pick the base for dispatch's `ReadInputs` (briefs and role prompts). Worth a queen card even if T-032 fixes it only for its own reads.
