# Review: T-012 phase C, guard 1 of 2 (sonnet) (branch t-012c, head b3f731f)

Risk-Tier: 3

Reviewed: branch t-012c, head b3f731f (two commits on main 6a2401b), draft PR #40. Gate: full. Did not read the other guard's review.

Checks: `go build ./...`, `go vet ./...`, `go test -race ./...` all pass, `gofmt -l .` is empty. `waggle gate verify --base origin/main --head HEAD` prints `leases: FAIL: no claim for T-012; claims must stay until merge`, `tiers: ok`, `decisions: unchanged`. The leases failure is the same pre-merge condition as phase B (no claim ref), not a lease violation. The diff has no em or en dashes. No `go.mod` or `go.sum` change. AC3 grep over the two touched non-test files finds no model name (only the adapter name `claude-code`).

## Intent Summary

`waggle upgrade --runtime claude-code` now builds the model policy at plan time, from the hive's current `config.yaml` bytes and the roster the plan is about to write (the rendered `want`, never the file on disk), validates it and resolves every role, and refuses with `ErrRefused` wrapping the config, roster or policy error before anything is written. The resolved policy rides on `upgradePlan` and is passed to `Materialize` via `WithPolicy`, so regenerated shims keep their `model:` lines. To support this, `internal/config/roster.go` gains `ParseRoster` and `ParsePolicy` (bytes forms; the path forms delegate), a `MaxFileSize` cap on both inputs, a `levels` fix hint on levels errors (reaches `init` too), and a fix for the `(<nil>)` message on an empty `roles` array. The `checkSymlinks` message now says "waggle writes" so it is true for `init`. Blast radius: `upgrade --runtime claude-code` only; `--runtime none` is untouched. It now also refuses any config `Parse` rejects (see finding 4).

## Blocking Issues

None.

## Checks against the brief

- **Lease:** four files changed: `internal/scaffold/upgrade.go`, new `internal/scaffold/upgrade_models_test.go` (matches `upgrade_*_test.go`), `internal/config/roster.go` and `roster_test.go` (listed in the phase C lease line). `init.go`, `internal/runtime/`, T-004 files, templates and `cmd/` are untouched. The existing `upgrade_test.go` needed no fixture change because `hive()` seeds from the phase B templates. Within lease.
- **Refusal before any write:** `resolvePolicy` runs inside `planUpgrade` (step 6, before `planShimPrune`), which the code documents as writing nothing; `Upgrade` returns the error before the `Dry run` line and before `execute`. `Resolve(role, nil, nil)` is run for every role in `Minimums`, so `ErrNoCrossModel` and `ErrLevelUnmapped` also surface at plan time. Both calls are identical for dry run and `--apply`. `refusedUpgrade` runs both and asserts `ErrRefused` plus the sentinel, an unchanged file map (including `.claude/agents`), unchanged repo state, empty `git status --porcelain`, and the old pin.
- **Shims keep `model:` after upgrade:** confirmed in the test (stale model-less shims replaced, guard differs from builder, scout is opus) and live (below).
- **Tests 16 to 20 exist and fail without the fix.** I rebuilt a scratch copy with `upgrade.go` reverted to main: tests 16, 17 and 18 FAIL, as intended. Test 19 passes on both sides, as it should (regression guard for `--runtime none`). Test 20 (`TestParseFormsMatchPathForms`) cannot compile without the new functions. It compares bytes and path forms for four roster inputs (valid, leftover `mode`, bad level, empty roles) and for the policy, plus the oversize roster, oversize config and missing-levels cases.
- **Deviation 1 (levels hint by matching `.levels.` in the error text):** acceptable and the smallest option inside the lease. It is stringly coupled to `runtimes.go` key paths (`...levels.<level>`). Gap: a `levels:` that is present but not a mapping, or an empty `levels:` header, would produce a key path ending in `.levels` with no trailing dot and so no hint. The error still names the key. Non-blocking (finding 3).
- **Deviation 2 (`ParsePolicy` caps config bytes at `MaxFileSize`):** sound. The path form already stopped at the cap through its file reader, so this keeps both forms equal; tested.
- **"For the guards" 1, refused dry run has printed plan lines:** confirmed. Output is the full plan (pin, rewrite, same, skip lines), then `waggle upgrade refused: model policy for claude-code shims: ...`, with no `Dry run: nothing changed` line. No write happened. The output reads a little odd but the refusal is the last line and exit is nonzero. Acceptable.
- **"For the guards" 2, roster-vs-role-file mismatch still fails in Materialize after writes:** I tried the realistic case, an extra role file in `.waggle/control/roles/` with no roster entry. It is a managed directory, so the plan lists `[remove]` for it and `--apply` succeeds; no failure. The residual case needs a build that ships a role file with no roster entry, which is a build defect, not a hive state. Acceptable, not a Keeper concern.
- **"For the guards" 3, `upgrade --runtime claude-code` now refuses configs `Parse` rejects:** true, and intended (the policy needs a valid config). Escape hatch: `--runtime none`. See finding 4.
- **Optional folds:** N3 (symlink message) done, tests match only `is a symlink`, all pass. N4 done through the shared hint. N5 done and tested. Symlink refusal for `CLAUDE.md` is not loosened.
- **Size:** 70 added non-test lines (37 in `roster.go`, 33 in `upgrade.go`, from `git diff --numstat`), target 40, stop 80. Under the stop with 10 to spare, over the 40 target; the PR states this.

## Section 7.2 rehearsal against our hive's real files

Setup: a scratch clone of main at 6a2401b (our hive's real `config.yaml`, `roster.json`, roles, pin `3063955`), a binary built from b3f731f (stamped, `vcs.modified=false`) run as `waggle upgrade --runtime claude-code`.

1. Without the `levels` edit: refused at plan time. Plan lines print (`[pin] 3063955 -> b3f731f`, `[rewrite] roster.json`, `[rewrite] waggle-gate.yml`, others `[same]` or `[skip]`), then `waggle upgrade refused: model policy for claude-code shims: scaffold: refused, nothing written: config: invalid .waggle/control/config.yaml: line 27: runtimes.adapters.claude-code.levels.fast: missing; ... add or fix the levels block from T-012 section 3.2 in .waggle/control/config.yaml (a seeded file: edit it by hand, commit, rerun)`.
2. After adding the section 3.2 `levels` block (frontier opus, standard sonnet, fast haiku, no alternates) under `runtimes.adapters.claude-code` and committing: the dry run plans `[pin]`, `[rewrite] .waggle/control/roster.json`, `[rewrite] .github/workflows/waggle-gate.yml`, `[shims] regenerate`, then the decision JSON, and does not refuse.
3. `--apply` succeeds. Shims: queen opus, scout opus, guard opus, builder sonnet, forager haiku. Roster gains 5 `min_level` and loses `mode` (0 occurrences). Guard differs from builder. `git status` shows only the gate workflow, config and roster modified (shims are ignored).
4. After committing, a second `upgrade` shows `[pin] ... (unchanged)` and `[same]` for the roster and every managed file, which satisfies the AC2 re-run check.

So the 7.2 sequence works against the real files. Note for the hive PR: the plan also rewrites `.github/workflows/waggle-gate.yml` (pin change), expected and Tier 3.

## Findings

**1. (Info) Test 17 asserts the key path `runtimes.adapters.claude-code.levels` as a substring.** The real message is `...levels.fast: missing`, so the test and the brief's wording are both met. No action.

**2. (Info) The hint text is appended to every `.levels.` error, including value errors.** For example `levels.frontier: -x` also gets "add or fix the levels block". Harmless, and the wording says "add or fix".

**3. (Low, optional) The hint misses a non-mapping or empty `levels` header** because the match needs `.levels.` with a trailing dot. The error still names the key and line. Fix later by matching on the key-path prefix `levels` rather than `.levels.` if the helper is touched again; not worth the lines now.

**4. (Keeper note, no ruling needed) `upgrade --runtime claude-code` now fails on any config that `config.Parse` rejects.** Before, a hive with an unknown `forge` or `leases` key could still upgrade shims. This is consistent with `init` and with the brief (the policy must load), and the refusal is plan-time with a clear message and a `--runtime none` escape. Mention it in the release notes if there are any.

## Evidence

Reproduced locally on origin/t-012c: build, vet, race tests (all packages ok), gofmt and gate verify as above. Fix-removal check done in a scratch copy under the session scratchpad (main's `upgrade.go`): 16, 17, 18 fail. The 7.2 rehearsal ran in a scratch clone of main, so the main checkout and the worktree were not modified. Not run: a live `claude` session reading the `model:` frontmatter. The Keeper's own recorded command output for `.waggle/hive/media/T-012/` is the rehearsal above, to be repeated in the real hive PR.

Verdict: PASS
