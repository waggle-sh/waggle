# T-020 phase C: rename our hive's roles, plus hive-records-5 (hive PR)

Hive PR for T-020 section 7, with hive-records-5 folded in (D-144). It moves our hive from the old role names (queen, architect, scout, forager, guard) to the new ones (queen, scout, forager, builder, guard). It does this by running `waggle upgrade` from `main` at `3063955`, sweeping the eight briefs with a phase still to dispatch (D-142), and recording D-138 to D-145.

No product code changes: every path in the diff is under `.waggle/`, except one comment line in `.github/CODEOWNERS` (commit 12, a Head Keeper file). This merges by Head Keeper bypass (hive files are never leasable), so the gate's leases check failing is expected. `waggle gate decisions --base origin/main --head HEAD` passes: `decisions: ok, appended D-138, D-139, D-140, D-141, D-142, D-143, D-144, D-145`. Process follows D-086 and D-138: worker draft, two guards (sonnet and fable), Head Keeper approval.

## Commits

1. `ed08260` Head Keeper: T-020 run waggle upgrade to 30639558288f93abdb378bf278a8b860e84a915e
2. `98ceca5` Head Keeper: T-020 rename Foragers to Builders in seeded WAGGLE.md
3. `a871143` Head Keeper: T-020 sweep role names in open brief T-008 (mechanical)
4. `e5df6a5` Head Keeper: T-020 add role names line to T-008 and fix article after sweep
5. `c3d6c00` Head Keeper: T-020 sweep role names in briefs with open phases (mechanical)
6. `42282f8` Head Keeper: T-020 role names lines and post-sweep fixes in briefs with open phases
7. `9a946df` Head Keeper: T-020 add gate template catch-up and fable F3 to T-008 (D-143)
8. `6da7e62` Head Keeper: T-011 hive records batch 5
9. `8209a53` Head Keeper: T-020 record D-138 to D-145 (hive-records-5)

Fix pass after both guards passed (T-020c sonnet and fable reviews):

10. `5439c79` Head Keeper: T-020 T-008 gate comment and scaffolded hive wording (guard fixes): section 3.4 item 4 names scaffolded hives; item 5, section 6 and AC2 let the Head Keeper commit drop the Q-T007-1 comment from the hive workflow, so it can stay byte-identical to the template.
11. `0fde868` Head Keeper: T-020 add role names line to T-024 (no sweep: it already uses the new names).
12. `5b966f8` Head Keeper: T-020 CODEOWNERS comment names the Builder role (`.github/CODEOWNERS`: "not a Forager task" becomes "not a Builder task").

All signed off. Branch head `5b966f8beefc772b597d5a153489cf137ca53b2c`. Commits 3 and 5 are pure section 2 sentinel sweeps, so a guard can reproduce them by script (AC2 style). Hand edits are only in commits 4, 6 and 7.

## Control plane (commits 1 and 2)

- `roster.json`: ids `architect`, `scout`, `forager` become `scout`, `forager`, `builder`. Nothing else in the roster changes.
- `roles/`: `architect.md` removed, `builder.md` created, `forager.md` and `scout.md` rewritten to the new prompts (with the bee-job line), `guard.md` rewritten ("route must-fix back to the Builder"). `queen.md` unchanged.
- `config.yaml`: only the pin line, `b4f222166f4a824e34414e205544fb17397ccfa1` to `30639558288f93abdb378bf278a8b860e84a915e` (full hash of the origin/main merge commit). `config.yaml` has no role-keyed settings, so no key renames.
- `WAGGLE.md` (seeded, hand edit, 7.2 step 3): "modules Foragers may lease" becomes "modules Builders may lease".
- `.claude/agents/waggle-*.md` (gitignored, not in the diff): upgrade removed `waggle-architect.md` and regenerated the five new shims in the drafting worktree.
- Step 0 (D-097, commit the generated `CLAUDE.md`) was already done on main at `97baeda` (#29, D-104), so there is no step 0 commit. The plan shows `[same] CLAUDE.md (waggle block)`.

### Upgrade dry run

Target built from origin/main `3063955` (clean, stamped build: `vcs.modified=false`), outside the work tree.

```
  [pin]     b4f222166f4a824e34414e205544fb17397ccfa1 -> 30639558288f93abdb378bf278a8b860e84a915e
  [rewrite] .waggle/control/roster.json
  [same]    .waggle/control/devcontainer.json
  [rewrite] .github/workflows/waggle-gate.yml
  [create]  .waggle/control/roles/builder.md
  [rewrite] .waggle/control/roles/forager.md
  [rewrite] .waggle/control/roles/guard.md
  [same]    .waggle/control/roles/queen.md
  [rewrite] .waggle/control/roles/scout.md
  [remove]  .waggle/control/roles/architect.md
  [skip]    .waggle/control/config.yaml (seeded)
  [skip]    .waggle/control/WAGGLE.md (seeded)
  [skip]    .waggle/control/local.yaml (seeded)
  [same]    .gitignore (waggle block)
  [same]    CLAUDE.md (waggle block)
  [remove]  .claude/agents/waggle-architect.md
  [shims]   regenerate .claude/agents/waggle-*.md
  [keep]    refs/waggle/* and .waggle/hive/**
```

The `--apply` run printed the same plan, then `[shim]` lines for `waggle-builder.md`, `waggle-forager.md`, `waggle-guard.md`, `waggle-queen.md`, `waggle-scout.md`. To exercise the shim removal path, the drafting worktree was seeded with copies of the main checkout's five old shims before the run.

## Gate workflow drift (D-143)

The upgrade also planned and wrote `[rewrite] .github/workflows/waggle-gate.yml`. I restored `main`'s copy before committing, so this PR does not touch the workflow. The template render would have moved the write permissions to workflow level (so `verify-leases-and-tiers` would run with write access), removed the T-014 `Waggle gate` step (`waggle gate verify` built from `BASE_SHA`), and added `|| true` to the claim refs fetch.

Per D-143, T-008 now carries the fix (commit 7, new section 3.4 and AC8): bring the template in line with the hive copy, with `contents: read` at workflow level and write permissions only on `relay-claim`. Section 3.4 also asks that the gate step skip cleanly in a hive with no `cmd/waggle/` and no `go.mod`.

**Until T-008 merges, every `waggle upgrade` in our hive needs a manual revert check on `.github/workflows/waggle-gate.yml`.** The upgrade plans the same rewrite every time (see the AC7 run below).

Fable F3 (T-014b) is in T-008 as section 3.5 and AC9. The brief picks the `comb.RequiredMinTier` option. That is already the state on `main`: test 8 in `cmd/waggle/gate_verify_test.go` compares the "Tier 3 path" output with `comb.RequiredMinTier(p) == comb.Tier3ControlPlane`, so no lease on `cmd/waggle/` is needed. The builder only confirms that it passes.

## Briefs touched (AC8, extended by D-142; T-024 added in the fix pass)

| Brief | Phases still open | Change |
| :--- | :--- | :--- |
| `T-008.md` | whole ticket (no `Merge T-008` on `main`) | sweep, `Role names` line, article fix, D-143 section 3.4, fable F3 section 3.5, AC8, AC9 |
| `T-012.md` | B (A merged) | sweep, `Role names` line, article fix ("an scout") |
| `T-014.md` | none found on `main`: A and B merged (`Merge T-014 phase B` is `3063955`). Swept because the Keeper listed it; its status line is stale ("Phase B is next") | sweep, `Role names` line |
| `T-018.md` | C (A, B merged) | sweep, `Role names` line |
| `T-021.md` | B, C (A merged) | sweep, `Role names` line, role line fix |
| `T-022.md` | B (A merged) | sweep, `Role names` line, role line fix, AC4 grep list fix |
| `T-023.md` | B to F (A merged) | `Role names` line only; **not swept** (see below) |
| `T-025.md` | B, C, D (A merged) | sweep (no-op, no old role words), `Role names` line |
| `T-024.md` | open phases remain | `Role names` line only (fix pass, commit 11); not swept, it already uses the new names |

Hand fixes after the sweep (commit 6):

- T-021 and T-022 had role lines naming both sets ("`forager` (implementation; `builder` after T-020)"). The sweep left "`builder` (implementation; `builder` after T-020)"; both now read "`- **Role:** `builder` (implementation)`".
- T-022 AC4 greps for role names in its code. The sweep turned `'architect|scout|forager|builder'` into `'scout|forager|builder|builder'`; restored to `'architect|scout|forager|builder'`.
- T-012 and T-008: "an scout" becomes "a scout".

**T-023 is not swept.** It was written after phase A with the new names already ("Role: `builder`", "The scout owns this brief", `--role builder|forager`). Running section 2 on it would rename them a second time (forager to builder, scout to forager) and break it. It only gets the `Role names` line. T-025 has only `builder`, which the sweep does not touch.

Quoted decision text: none of the swept lines quote `T-006-backfill.md` or DECISIONS.md. Sample output lines in T-021 and T-022 that already said `builder` are unchanged.

Not touched: `T-020.md` (section 4 excludes it), `T-006-backfill.md`, every other merged brief, reviews, decisions text, `ROADMAP.md`, `WORKPLAN.md`.

Stale status lines seen (not edited, outside this PR's rulings): T-012 says "Phase A merging via PR #18", T-014 says "Phase B is next", T-025 says "no phase dispatched", but `main` has `Merge T-012 phase A`, `Merge T-014 phase B` and `Merge T-025 phase A`. T-015's status already reads Done (merged #21 at `6b84500`, ruleset applied), so it was not edited.

## Hive records batch 5 (commit 8)

Byte-identical copies from the main checkout (checked with `cmp` and `diff -r`): `reviews/T-025a.md`, `reviews/hive-records-4.md`, `reviews/T-014b-sonnet.md`, `reviews/T-014b-fable.md`, `media/T-025a/` (1 file), `media/T-014b/` (16 files), `handoff/queen.md` (an update to the tracked file). `media/T-020c/` is not included.

Scan results (copied unedited, as instructed; for the Head Keeper to decide):

- `handoff/queen.md` line 59 and `reviews/hive-records-4.md` line 24 name the Keeper's personal git email (the email address). The same address is already the author on 96 commits on `main` and in the tracked `media/T-023a/fable-run2-tools.jsonl`, so this adds no new exposure, but it is two more places to redact before the repo goes public.
- `media/T-014b/p1.sh` to `p6.sh` line 1 hardcode a local scratch path that encodes the home directory and username (`/private/tmp/claude-redactedredactedredacted/-Users-<user>-code-waggle/...`). The T-014b sonnet review already notes these scripts hardcode their scratch dir.
- No tokens, keys or `op://` references. `media/T-014b/lib.sh` uses the dummy identity `t@e.com`.

## Decisions (commit 9)

Recorded with `waggle decide --file` from the target build, one at a time, stopping on any ID mismatch. Every assigned ID matched the Keeper's list:

| ID | Kind | Tickets | Subject |
| :--- | :--- | :--- | :--- |
| D-138 | policy | T-020 | phase C drafted by a worker, two guards, Head Keeper merge; Keeper renames his own local.yaml keys |
| D-139 | policy | T-014 | phase B accepted at 550 added non-test lines against the 450 stop |
| D-140 | policy | T-014, T-015 | CODEOWNERS covers `.github/workflows/` and `.github/actions/` (fable F4) |
| D-141 | policy | T-014 | comb keeps rejecting claims under `.waggle/`; hive records merge by bypass (fable F10) |
| D-142 | scope | T-020 | brief sweep covers every brief with a phase still to dispatch |
| D-143 | technical | T-008, T-016, T-020 | stale gate template; hive keeps main's copy; T-008 fixes the template |
| D-144 | scope | T-020 | hive-records-5 rides in this PR |
| D-145 | technical | T-016, T-020 | upgrade log entry: pin to `30639558288f93abdb378bf278a8b860e84a915e` (T-016 section 2 step 4) |

Each choice ends "Decided by the Head Keeper on 2026-10-18; recorded by the queen." The date field is assigned by `waggle decide` (2026-10-18).

Two deviations from the ruling text:

- **D-144 kind is `scope`, not `planning`.** `waggle decide` accepts only publish, spend, delete, credentials, strategy, policy, technical and scope. The earlier planning ruling D-081 was recorded as `scope` too.
- **D-145 is the T-016 3.7 template input, with two edits:** the choice adds "except .github/workflows/waggle-gate.yml, which keeps main's copy (D-143)", and `tickets` adds T-020 to T-016.

## AC7 evidence

Rerun of `waggle upgrade` (dry run) on the final head:

```
  [pin]     30639558288f93abdb378bf278a8b860e84a915e (unchanged)
  [same]    .waggle/control/roster.json
  [same]    .waggle/control/devcontainer.json
  [rewrite] .github/workflows/waggle-gate.yml
  [same]    .waggle/control/roles/builder.md
  [same]    .waggle/control/roles/forager.md
  [same]    .waggle/control/roles/guard.md
  [same]    .waggle/control/roles/queen.md
  [same]    .waggle/control/roles/scout.md
  [skip]    .waggle/control/config.yaml (seeded)
  [skip]    .waggle/control/WAGGLE.md (seeded)
  [skip]    .waggle/control/local.yaml (seeded)
  [same]    .gitignore (waggle block)
  [same]    CLAUDE.md (waggle block)
  [shims]   regenerate .claude/agents/waggle-*.md
  [keep]    refs/waggle/* and .waggle/hive/**
```

- `roster.json` and all five role files: `[same]`. Pin unchanged. No `[remove]` lines.
- `ls .waggle/control/roles`: `builder.md forager.md guard.md queen.md scout.md`. No `architect.md`.
- `ls .claude/agents`: `waggle-builder.md waggle-forager.md waggle-guard.md waggle-queen.md waggle-scout.md`.
- The only managed line that is not `[same]` is the workflow `[rewrite]` (D-143).
- Sanity: `go build ./...`, `go vet ./...`, `go test -race ./...` all pass. `git diff --name-only origin/main..HEAD` lists only paths under `.waggle/` plus `.github/CODEOWNERS` (fix pass).

## After merge (not doable in this PR)

- **Main checkout shims.** `.claude/agents/` is gitignored, so the merge does not update the main checkout's shims. After merge, the Head Keeper runs the pinned build's `waggle upgrade --apply` in the main checkout (then reverts `waggle-gate.yml` per D-143) to remove `waggle-architect.md` and write `waggle-builder.md`. Until then, `--agent forager` and `--agent scout` in the main checkout start the old roles.
- **`local.yaml`** `model_overrides` keys: the Keeper renames his own (D-138, 7.2 step 5). Untouched here. Keeper scripts or habits that run `claude --agent forager` or `--agent scout` must change too: after this PR those names start the other role.

## Open note (T-020 section 11)

v0.2 Traditional Equivalent column in `docs/RFC-0003.md`: phase B left "UX, Visuals, Hygiene, Deliberation" against four remaining roles. Still undecided whether "Visuals" was drone's slot and should go. Not touched here: the RFC is product, so any change is a product follow-up once the Keeper decides.
