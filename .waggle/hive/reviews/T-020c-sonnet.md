<!-- SPDX-License-Identifier: MIT-0 -->
PASS WITH FIXES

# Guard review: T-020 phase C + hive-records-5 (PR #38, `t-020c`, 8209a53)

- Guard: sonnet (second guard, fable, reviews independently)
- Risk-Tier: 3
- Build: no Go file changed, so build and tests were not rerun for the diff. `waggle` was built from `origin/main` (3063955) in a scratch worktree, outside the tree, and run against this branch.
- Evidence: commands run in the guard worktree, detached at `origin/t-020c`. Scratch scripts live in the session scratchpad, not in the hive. Branch not edited.

The rename is correct and mechanical. The control plane matches the upgrade output exactly, the pin is right, no merged brief was swept, no quoted decision text was renamed, and D-138 to D-145 match the rulings. One brief with phases still to dispatch (T-024) was left out of the sweep and the Role names line, and D-142's list omits it. That is a small, non-blocking fix.

## Blocking findings

None.

## Fixes (numbered, none blocking)

1. **T-024 has phases A to F still to dispatch and no `Role names` line.** Its status says "no phase dispatched", and the `Merge T-024` commit on main is only the RFC-0003 A2 PR (#31), not the Go work. T-024 was written with the new names (`builder` and `forager` as roles, "Builder hazards"), so it must not get the section 2 sweep: it would turn the new `forager` into `builder`, a double rename. Add only the line `- **Role names:** updated by T-020 (D-074).` under its header, as was done for T-023 and T-025, in this PR or the next hive-records batch. D-142 names eight briefs and not T-024, and DECISIONS.md is append-only, so note the gap in the next decision or the hive-records-6 body instead of editing D-142. AC8 as written is satisfied (every brief on the list has the line), but the list is one short of D-142's own rule ("every brief with a phase still to dispatch").
2. **T-008 section 3.4 item 5 versus "byte-identical".** Item 5 removes the template comment "Not in the scaffold template until waggle ships a release binary (T-007, Q-T007-1)", but the hive copy of `waggle-gate.yml` carries that comment (first line of the `Waggle gate` run block). AC8 wants the render and the hive file byte-identical. As written the builder cannot satisfy both unless the Head Keeper commit also deletes that comment from the hive copy, and AC2 permits only the `case` line and a step-order change there. Add the comment removal to AC2 and to item 5 ("the hive copy drops the same comment in the section 6 commit"). I diffed the template against the hive file: the only differences are the permissions (item 1), `|| true` (item 2), the setup-go and `Waggle gate` step replacing the placeholder loop (item 3), and that comment (item 5). Nothing else differs.
3. **`handoff/queen.md` (added to the PR) still says `architect` in 4 places** (lines 5, 35, 38, 55) and lists D-138 to D-145 under "NOT yet recorded", which this PR makes false at merge. It is not on section 4's excluded list, so AC3's literal grep hits it. Lines 38 and 55 are intentional history; lines 5 and 35 are live guidance ("Brief first (architect)"). The next queen session should refresh it. The file is byte-identical to the main checkout copy, as the PR states.
4. **T-008 3.4 item 4 (skip cleanly where there is no `go.mod`)** is implementable in lease, but "our hive" is the Waggle repo itself, which has `go.mod`. The condition only matters for scaffolded hives (the template). Say "scaffolded hive" so a builder does not edit the hive copy needlessly. Wording only.

## Rulings (each recorded decision checked against the brief)

- **D-138 (policy):** matches. Worker draft, two guards (sonnet, fable), Head Keeper approval, Keeper renames his own `local.yaml` keys. The body adds "once hive-records-4 has merged and nothing else is running", which is the D-086 pattern timing, not a new ruling. Accept.
- **D-139 (policy):** matches. 550 added non-test lines against the 450 stop; the body names D-107, D-114 and B1, counted per D-089 and D-136. The T-014b fix verification reports 550. Accept.
- **D-140 (policy):** matches, F4. The CODEOWNERS change landed on main in `81f9b89`; this PR touches no CODEOWNERS. Accept.
- **D-141 (policy):** matches, F10. Accept.
- **D-142 (scope):** matches the Keeper ruling and the eight-brief list, but see fix 1 (T-024).
- **D-143 (technical):** matches. The hive keeps main's `waggle-gate.yml` (the PR does not touch it); T-008 carries the template fix; the manual revert check is recorded. Accept.
- **D-144 (scope):** kind `scope` is right (`planning` does not exist as a kind). Accept.
- **D-145 (technical):** matches T-016 section 2 step 4. The pin is the full 40-hex hash, the old pin is named in full, and the body records the D-143 exception. Accept.
- **Append-only:** `gate decisions --base origin/main --head HEAD` prints `decisions: ok, appended D-138, D-139, D-140, D-141, D-142, D-143, D-144, D-145`. 153 added lines and 0 removed in DECISIONS.md, so earlier entries are byte-unchanged. No invented rulings; the last earlier ID is D-137.

## Hostile probe results

1. **No product change: pass.** `git diff --stat origin/main...origin/t-020c` lists 38 files, all under `.waggle/`. A name filter for `cmd/`, `internal/`, `docs/`, `go.mod`, `go.sum`, `README`, `CONTRIBUTING`, `LICENSE*`, `NOTICE` and `.github/` prints nothing. `waggle-gate.yml` is untouched.
2. **AC7: pass.** `waggle upgrade` (dry run, main build 3063955) on this branch: `[pin] ... (unchanged)`, `[same]` for `roster.json`, `devcontainer.json`, all five role files, `.gitignore`, `CLAUDE.md`; `[skip]` for the three seeded files; the only `[rewrite]` is `.github/workflows/waggle-gate.yml`, the documented D-143 case. `.waggle/control/roles/` holds `builder, forager, guard, queen, scout`, no `architect.md`. The shims are gitignored and absent from a fresh worktree, so I rebuilt a scratch copy of the tree, seeded the five old shims (`architect, forager, guard, queen, scout`) and ran `upgrade` and `upgrade --apply`: the plan shows `[remove] .claude/agents/waggle-architect.md`; after apply `.claude/agents/` holds exactly `waggle-{builder,forager,guard,queen,scout}.md`; the only modified tracked file is the gate workflow (the manual-revert case). `roster.json` ids are `queen, scout, forager, builder, guard`.
3. **Pin: pass.** `config.yaml` pin is `30639558288f93abdb378bf278a8b860e84a915e`, 40 hex, equal to `git rev-parse 3063955`. D-145 and `waggle status` ("pin 3063955...") agree.
4. **AC3 and AC8: pass, with fix 1 and fix 3.** `git grep -n -i -w -E 'architects?'` outside reviews and DECISIONS.md hits: merged briefs (T-002, T-003, T-006-backfill, T-007, T-009, T-010, T-011, T-013, T-016), `T-020.md`, evidence folders under `media/`, `CONTRIBUTING.md` (retired-names table), `internal/decisions/testdata/` (verbatim history), and `handoff/queen.md` (fix 3). Nothing in `.waggle/control/`, `CLAUDE.md`, or any of the eight swept briefs. All eight swept briefs (T-008, T-012, T-014, T-018, T-021, T-022, T-023, T-025) carry the `Role names` line, and no other brief gained it. Merged briefs swept: none (the diff touches only those eight). I read every changed line in the eight briefs: the mappings are correct (`architect` to `scout`, `scout` to `forager`, `forager` to `builder`), including the T-012 level table (scout and queen `frontier`, builder `standard`, forager `fast`), tests 2 to 7 and the T-018 owner column. No double rename, no quoted `T-006-backfill.md` text altered, no sentinel left (`zzwr` grep empty). T-021 and T-022 now state one role set. The T-022 AC4 grep list `architect|scout|forager|builder` is unchanged and harmless. Section 4's excluded paths are untouched. Missed brief: T-024 (fix 1). Fully merged briefs wrongly swept: none.
5. **DECISIONS.md: pass.** See rulings. IDs are consecutive D-138 to D-145; kinds are `policy` x4 (D-138 to D-141), `scope` x2 (D-142, D-144), `technical` x2 (D-143, D-145); all dated 2026-10-18, all `Supersedes: none`.
6. **Records batch: pass.** `git diff-tree` on `6da7e62` lists 21 files (handoff, 4 reviews, T-014b and T-025a media). All 21 are `cmp`-identical to the same paths in `/Users/paul/code/waggle`. Sensitive-content scan (home paths, emails, tokens, key patterns, private keys): no tokens or keys. Hits: the Keeper's protonmail address in `handoff/queen.md` line 59 and `reviews/hive-records-4.md` (known); the scratchpad path `/private/tmp/claude-501/-Users-paul-code-waggle/<session>/scratchpad` in `media/T-014b/p1.sh` to `p6.sh` (known; it encodes the local username and uid); `t@e.com` in `media/T-014b/lib.sh` (throwaway git identity, benign). One mention to track: the handoff names `~/Repos/znap` and `media/T-023a/fable-run2-tools.jsonl` as holding the Keeper's email and that path, to redact before the repo goes public. That jsonl is already on main, not in this PR. No other finding.
7. **T-008 additions: pass, with fixes 2 and 4.** Section 3.4 matches the actual template-to-hive diff (permissions, `|| true`, setup-go plus `Waggle gate` step, comment). The work stays inside T-008's lease (`internal/scaffold/`, the template) plus the existing Head Keeper commit for the workflow. Section 3.5 (F3) is accurate: test 8 in `cmd/waggle/gate_verify_test.go` is keyed on `comb.RequiredMinTier`, so AC9 "unchanged and passes" is right and needs no `cmd/waggle/` lease.
8. **WAGGLE.md and config: pass.** `WAGGLE.md` line 12 reads "modules Builders may lease"; no other role word in it. `config.yaml` has no role-keyed settings and no role words; only the pin changed. `go run ./cmd/waggle status` and the main build both load the hive cleanly (`Hive: ... pin 3063955...`, `Claims (0 active)`) against the branch's config and roster. `local.yaml` is gitignored and Keeper-owned (D-138).
9. **Commits: pass.** 9 commits over `origin/main`, all subjects begin `Head Keeper: `, all carry `Signed-off-by: paulternate <cleary.paul@protonmail.com>`, one author address. No em or en dash in any added line under `.waggle/control`, `tasks/` or DECISIONS.md.

## Acceptance criteria (T-020 section 8, phase C items)

- AC3 (grep): pass. Sentinels absent; `architect` only in excluded paths and the handoff (fix 3).
- AC7: pass (probe 2).
- AC8: pass for the eight listed briefs; T-024 is a miss against D-142's rule (fix 1).
- AC1, AC2, AC4, AC5, AC6: phase A and B items, not in scope for this PR.
- T-016 section 2: upgrade logged (D-145), `architect.md` and its shim removed by `upgrade` and not by hand.

## Intent Packet

- What: moves our hive to the new role names (queen, scout, forager, builder, guard) by running `waggle upgrade` from main at 3063955. It rewrites `roster.json` and the role prompts, adds `builder.md`, removes `architect.md`, moves `waggle.pin`, and hand-edits the seeded `WAGGLE.md` (Foragers to Builders). It sweeps eight open briefs, adds the gate-template catch-up and the F3 confirmation to T-008, adds hive-records-5 (reviews, media, handoff) and records D-138 to D-145.
- Guarantees: no product file changes. A dry-run `upgrade` is `[same]` everywhere except the gate workflow. The hive's `waggle-gate.yml` stays as on main, so the T-014 gate keeps running with `contents: read` at workflow level. No merged brief, review, decision or evidence file is rewritten. DECISIONS.md is append-only (`gate decisions` ok).
- Does not guarantee: that T-024 carries the role-names line (fix 1); that `upgrade` in our hive is safe to apply blind (it still plans a rewrite of `waggle-gate.yml` until T-008 merges, D-143); that the handoff is current (fix 3). The PR fails the `leases` check by design (D-141) and merges by Head Keeper bypass.
- Blast radius: every future worker prompt and brief for the eight open tickets; the hive's role files and roster; `.waggle/hive/` records. No build, test or CI-file effect.

**Final verdict: PASS WITH FIXES** (no blockers; fixes 1 and 2 are best applied before the T-024 and T-008 dispatches, not before this merge)
