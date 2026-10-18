# T-020 phase C review (guard, fable), PR #38, branch `t-020c`

**Verdict: PASS**
**Risk-Tier: 3**

Reviewed at `8209a53` (nine Head Keeper commits, `ed08260` to `8209a53`, base `3063955`) against `.waggle/hive/tasks/T-020.md` sections 2, 4, 7 and 8, `T-016.md` sections 2 and 3, and D-138 to D-145 as ruled by the Head Keeper. Reviewed independently of the sonnet guard. The diff touches 38 files, all under `.waggle/`; no product file, no workflow, no `go.mod`.

Evidence in `.waggle/hive/media/T-020c/`: `fable-upgrade-dryrun.log` (AC7 run from a clean stamped build of `3063955`, `vcs.modified=false`) and `fable-sweep-repro.log` (section 2 reproduced over the `origin/main` briefs and compared byte for byte with the two mechanical commits). `go build ./...`, `go vet ./...`, `go test -race ./...` pass on the head.

## Blocking findings

None. The control plane is exactly the target build's render (every managed file `[same]`, pin `(unchanged)`, no `[remove]`, no `[note]`), the sweep is reproducible by script, the decisions are append-only and match the rulings, and the records batch is byte-identical to the main checkout.

## Fixes and notes (non-blocking, numbered)

**F1. The kept-back workflow is a standing manual step, not a latent corruption.** `waggle upgrade` keeps no manifest and no per-file hash: `planUpgrade` renders each template and compares bytes with the file on disk every run (`internal/scaffold/upgrade.go`, step 2). So the moved pin records nothing that disagrees with `.github/workflows/waggle-gate.yml`, a later run cannot mark it "user-modified", and `waggle status` only reads the pin line. The one real hazard is `--apply`: it executes the plan, so every `--apply` in our hive rewrites the workflow until T-008 lands, and the Keeper must `git checkout -- .github/workflows/waggle-gate.yml` before committing. My dry run on the head prints exactly one non-`[same]` managed line, the workflow `[rewrite]`. Suggest the Keeper treat "exactly one `[rewrite]`, and it is the workflow" as the expected shape of every dry run until T-008 merges, and anything else as a stop. I confirmed D-143's premise by diffing the template against the hive copy: the template has `contents: write`, `pull-requests: write`, `issues: write` at workflow level, `|| true` on the claim fetch, and a placeholder loop in place of the `Waggle gate` step. CODEOWNERS on `.github/workflows/` (D-140) is the backstop if a rendered copy ever reaches a PR.

**F2. `handoff/queen.md` still steers with the old meanings.** It is in this batch unedited, as the records rule requires, but it is also what `.claude/hooks/session-start.sh` feeds the next queen session. Line 35 says "build in an isolated worktree (forager; architect for docs)" and line 55 says "architect subagent"; after this PR `forager` names the triage role and `architect` names nothing. Line 38 already carries the new names. The Stop hook forces a handoff refresh once `main` moves, so the queen should rewrite those two lines in the post-merge refresh rather than in this PR.

**F3. Main checkout `local.yaml` has stale keys.** `model_overrides` reads `architect: inherit`, `forager: sonnet`, no `builder`. Nothing reads it today (`internal/scaffold/init.go` only writes it; T-012 phase B's loader is not merged), so nothing fails, but after merge `forager: sonnet` would mean the triage role. D-138 assigns the rename to the Keeper; do it in the same sitting as the `--apply` and the workflow revert.

**F4. `.github/CODEOWNERS` line 6 says "not a Forager task".** Old meaning. Product file, Head Keeper commit, outside this PR's scope. One-word fix the next time the file is touched.

**F5. T-024 has no `Role names` line.** Like T-023 it was written with the new names, has no phase dispatched, and the sweep is correctly not applied (it would double-rename). D-142 lists eight briefs and T-024 is not among them, so this is consistent with the ruling. Cosmetic; add the line when T-024 phase A dispatches. T-023's line reads "updated by T-020" although nothing in it changed; harmless.

**F6. Stale status lines.** T-012 ("Phase A merging via PR #18"), T-014 ("Phase B is next"), T-025 ("no phase dispatched") are behind `main`. Correctly left alone here; the scout fixes them on the next touch.

## Semantic check of the sweep (focus c)

The mapping in section 1 is old `architect` to `scout`, old `scout` to `forager`, old `forager` to `builder`. I read every changed line for meaning, not only for the word:

- T-012 section 1 table: planner row is now `scout` at `frontier`, implementer `builder` at `standard`, triage `forager` at `fast`; the guard floor is "at or above the builder's effective level"; test 2 reads "scout and queen get opus, builder sonnet, forager haiku"; test 15 expects `waggle-scout.md` with the frontier model. All consistent with `internal/runtime/policy.go`, which already names `roleBuilder`.
- T-014 Q-T014-4 "Scout and docs commits need a ticket" and AC3 "the scout splits `tiers`" are the planner, correct. T-018's "Scout, once phase A merges" rows are the planner, correct.
- T-021 and T-022 role lines: the sweep produced "`builder` (implementation; `builder` after T-020)"; the hand fix collapsed them to "`builder` (implementation)". T-022 AC4's grep list was restored to `architect|scout|forager|builder`, which is the right list for a check that no role name is hardcoded.
- T-022 sample output rows now show `builder` claims; correct for implementation claims.
- T-008 "no Builder can claim it" and "a Builder can add a module" are the implementer, correct.
- No swept line quotes `T-006-backfill.md` or `DECISIONS.md` text. No sentinel remains anywhere (`git grep -i 'zzwr[123]zz'` outside T-020.md is empty).
- T-023 untouched apart from the line; its `--role builder|forager`, "The scout owns this brief" and the exit code table already use the new names. T-025 has only `builder`.

Reproduction: copying the seven briefs from `origin/main`, running section 2 steps 1 and 2 unmodified, and comparing with `a871143` (T-008) and `c3d6c00` (the rest) gives `MATCH` for all seven. Hand edits are confined to `e5df6a5`, `42282f8` and `9a946df`, and I read each of them in full.

## Decisions (focus d)

`DECISIONS.md` on `main` (114952 bytes) is a byte prefix of the head file; the diff is pure addition after D-137; the file ends at D-145. D-138 to D-144 match the Head Keeper's rulings as given to the guards: worker draft, two guards, Keeper renames his own `local.yaml` (D-138); 550 lines (D-139); CODEOWNERS on workflows and actions (D-140); no `.waggle/` claims, bypass merges (D-141); sweep covers every brief with a phase to dispatch, naming the eight (D-142); hive keeps `main`'s workflow, T-008 fixes the template, manual check until then (D-143); records-5 rides here (D-144). D-145 is the T-016 3.7 template with the full 40-hex pin, plus the stated two edits (workflow exception, T-020 in tickets). D-144's kind `scope` instead of "planning" follows the D-081 precedent and the `waggle decide` validator. The gate's own `decisions` check on the PR run printed `ok, appended D-138 ... D-145`. No em or en dash in the new entries.

## Records batch (focus e)

All 21 paths `cmp` equal to the main checkout files. Scan of the batch: the Keeper's email address appears once in `handoff/queen.md` (line 59) and once in `reviews/hive-records-4.md` (line 24), both already in `main`'s history; `media/T-014b/p1.sh` to `p6.sh` line 1 hardcode `/private/tmp/claude-redactedredactedredacted/waggle-scratch/<session>/scratchpad`; `lib.sh` uses `t@e.com`. No token, key, `op://`, bearer or AKIA pattern. Nothing new beyond what the PR body lists.

## Gate (focus f)

`verify-leases-and-tiers` fails on `leases` (nine Head Keeper commits with no claim, each "merge with a Head Keeper bypass") and `tiers` (Tier 3 paths under `.waggle/control/`); `decisions` is ok; `product-ci` passes; `relay-claim` is skipped. Nothing fails for an unexpected reason.

## Rulings

**Process: no scout commits in a hive PR that section 7.3 assigns to the architect.** All nine commits are Head Keeper commits by a worker draft, per D-138. Accepted.

**T-014 swept with no open phase.** Both phases are merged (`3063955`), so by the brief's own rule it is a merged brief and should be untouched. The Keeper listed it in D-142, and the swept text is internally consistent, so I accept it; the stale "Phase B is next" status is the only cost (F6).

**Shim removal path not exercised on the head I reviewed.** My worktree has no `.claude/agents/`, so the dry run shows no `[remove] .claude/agents/waggle-architect.md`. The PR body reports the seeded-shim run, and `planShimPrune` in `upgrade.go` removes any `# waggle:generated` shim whose role file is not shipped, which covers `waggle-architect.md` in the main checkout. Accepted on code reading; the Keeper's post-merge `--apply` is the live test.

## Acceptance criteria (phase C items)

- AC1 pass (build, vet, race tests; product CI green on the PR).
- AC3 pass: no sentinel outside T-020.md; `architect` hits only in DECISIONS.md, reviews, media, merged briefs, T-020.md, T-006-backfill.md, T-013's retired-names rows, T-016's ordering facts, `internal/comb/backend.go`'s v1 mapping comment, and CODEOWNERS (F4).
- AC4 pass: no section 4 path changed; DECISIONS.md is append-only; no review, evidence or merged brief content altered (the batch adds new files only).
- AC7 pass: dry run on the head prints `[same]` for `roster.json` and all five role files, pin unchanged, no `[remove]`; `.waggle/control/roles/` holds exactly `builder.md forager.md guard.md queen.md scout.md`. The shim half holds in the drafting worktree per the PR body and will hold in the main checkout after the Keeper's `--apply`.
- AC8 pass: the eight listed briefs each carry the `Role names` line; T-023 and T-025 were not swept and did not need it; T-024 lacks the line (F5, cosmetic).

## Intent Packet

Phase C moves our hive from `b4f2221` to `3063955` through the first real `waggle upgrade`: `roster.json` ids become `scout`, `forager`, `builder`; `roles/architect.md` is removed and `roles/builder.md` created; `forager.md` and `scout.md` swap to their new prompts; `guard.md` routes must-fix to the Builder; the seeded `WAGGLE.md` says "Builders may lease". The gate workflow keeps `main`'s copy on purpose (D-143), so every upgrade until T-008 lands plans one `[rewrite]` for it and needs a manual revert after `--apply`. Eight briefs with phases still to dispatch are swept by the section 2 sentinel method (reproducible by script) plus hand fixes in three commits; T-023 is left alone because it already uses the new names. Hive records batch 5 and D-138 to D-145 ride along.

Blast radius: after merge the words `forager` and `scout` mean the other role everywhere live in this hive. Agent types for the queen's dispatch after the Keeper runs the pinned build's `upgrade --apply` in the main checkout: `scout` (briefs, roadmap, specs), `forager` (triage, repro, lease mapping), `builder` (implementation), `guard` (review), `queen`. `architect` no longer exists. Until that `--apply` runs, the main checkout's gitignored shims still present `architect`, `forager` (old implementer body) and `scout` (old triage body); do not dispatch from the main checkout in that window. The Keeper's same sitting should also rename `local.yaml`'s `model_overrides` keys and revert the workflow rewrite.
