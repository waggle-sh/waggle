# Guard review (opus): hive control pin move to 1475d74 and D-259 (PR #105, head 655d867)

Risk-Tier: 3

## Intent Summary

Branch hive-pin-1475d74 (one commit, 655d867, on origin/main d422800) moves `.waggle/control/config.yaml` `waggle.pin` from 7bef53b400910d55bfdcf397134b227168fa0bde to 1475d74c6ee8ed9e50de8dffa6984edfd454ddbb, the merge of T-034 (PR #103), and appends D-259 to `.waggle/hive/DECISIONS.md`. No managed file is re-rendered, unlike the D-250 move (a8eff90), which also applied the gate workflow, guard role and roster renders. Blast radius: every later `waggle dispatch` in this hive runs T-034 code, so guard model choice now reads dispatch run records (builder model always avoided; Tier 3 and unknown-tier plain guards also avoid the previous round's guard model, with the D-179 downward walk) and `--second` defaults its base to the first guard record's base and accepts an ancestor builder base.

## Checks run

- Pin commit: `git cat-file -t 1475d74c...` is `commit`; `git merge-base --is-ancestor 1475d74c... origin/main` succeeds; its parents are 7ed603d and 273d5e0, subject "Merge pull request #103 from waggle-sh/t-034". It is the T-034 merge.
- Diff `origin/main..HEAD`: 2 files, 20 insertions, 1 deletion. config.yaml changes only the pin value (comment kept verbatim). DECISIONS.md is a pure append after D-258, no earlier line touched.
- `waggle-1475d74 gate decisions --base origin/main --head HEAD`: `decisions: ok, appended D-259`.
- `scan-hive.sh origin/main HEAD`: 0 hits, decisions gate exit 0, config.yaml held as outside `.waggle/hive/` (exit 2 class, as expected for a control change: guard and card). The worktree has no `scan-names.local`, so the surname check did not run here; I read the 20 added lines by eye and they hold no names, emails, home paths or dashes, and refer to the Keeper by role.
- `go version -m waggle-1475d74`: `vcs.revision=1475d74c6ee8ed9e50de8dffa6984edfd454ddbb`, `vcs.modified=false`, go1.27.2, path `github.com/waggle-sh/waggle/cmd/waggle`.
- `waggle-1475d74 upgrade` (dry run, from the worktree): `[pin] 1475d74... (unchanged)`; roster.json, devcontainer.json, waggle-gate.yml and all five roles `[same]`; config.yaml, WAGGLE.md, local.yaml `[skip] (seeded)`; .gitignore and CLAUDE.md blocks `[same]`; `[shims] regenerate .claude/agents/waggle-*.md` (those are gitignored by `.gitignore:9`, so no tracked change); `Dry run: nothing changed`. `git status` clean afterwards. Consistent with `git diff 7bef53b 1475d74` touching only config.yaml among control files apart from the renders a8eff90 already applied.
- Run record compatibility: the T-034 lookups filter on `ticket`, `branch`, `base_oid`, `head_oid`, `verdict`, `second` and `model`. All exist in the 7bef53b `Result` struct (internal/dispatch/result.go:39), and a real record (runs/T-034-guard-1/result.json) carries them, so records written under the old pin are readable by the new one.

## Blocking Issues (Must-Fix Only)

None.

## Notes

N1. Expected model path for our next Tier 3 builds. With `levels.frontier: opus`, `alternates.frontier: fable`, roster guard `min_level: standard`, and a frontier opus builder: round 1 plain guard resolves fable; the confirming guard on a fix round avoids opus (builder) and fable (previous guard), finds nothing at frontier, and walks down to sonnet; the round after that goes back to fable. A `--second` on round 1 also lands on sonnet, as before. So Tier 3 fix rounds dispatched through `waggle dispatch` will alternate fable and sonnet, never opus. This is what D-255 accepted ("a standard-level model may confirm a stepped-up Tier 3 build outside --second"), so not a hazard, but the queen should expect sonnet on even fix rounds and pair it per D-205 when the round's diff qualifies for two guards.

N2. Fail-closed changes worth knowing before the next `--second`: it now needs a first guard record whose `head_oid` equals the head under review (previously any latest guard record on the same base), and the base must equal the first guard record's base exactly. A `--second` after a fix commit without a fresh first guard on the new head now exits with `--second needs a builder run and a first guard run of <ticket> on head <oid>`. This refuses rather than misroutes, so no hazard.

N3. Plain Tier 3 guard with a must-fix count but no prior guard record on the branch (for example a review done outside dispatch, such as this one) gets `confirm_model_unknown` and no alternation. Our hive control and records reviews are Claude Code subagents chosen by the queen (D-256), so the queen still picks models by hand there; T-034 does not cover them.

N4. D-259 text is accurate. The rationale summarizes `--second` as taking "the first guard record's base", which is the no-`--base` default (D-254); it does not mention the ancestor-builder-base acceptance. Tickets T-016 and T-034 follow the upgrade template's T-016 plus the carried ticket; D-250 listed only T-033, a harmless inconsistency between the two pin records.

Verdict: pass
