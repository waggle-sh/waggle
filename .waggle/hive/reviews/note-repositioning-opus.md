# Guard review: ruled repositioning note (PR #86)

Verdict: pass
Risk-Tier: 2

Subagent guard hand-back (opus), round 1 at 71c9155, condensed by the queen.

## Intent Summary
PR #86 adds docs/rfc-notes/repositioning.md (Waggle Gate first, `waggle dispatch` second). e622c06 lands the draft; 71c9155 rewrites the Status line and adds a "Ruling: option 1 (D-xxx)." line under Q1 to Q7. Docs only.

## Checks
1. Ruling commit scope: only line 4 and seven ruling lines; mapping Q1 D-229, Q2 D-232, Q3 D-233, Q4 D-234, Q5 D-231, Q6 D-235, Q7 D-236 matches DECISIONS.md.
2. e622c06's blob equals cd7628c's.
3. Public safety: no home paths, emails, machine or personal names, secrets; notes 1 to 3.
4. No em or en dashes.
5. Consistency with the rulings and the T-031 brief: pass with notes.

## Findings
1. note (lines 11, 60): third-party characterizations read as verified fact; attribute them to the review.
2. note (line 66): "the signal is gameable" is unattributed.
3. note (lines 94, 95, 6, 13): "226 decisions", "One maintainer, private repo" and references to an unpublished review will read stale at G5; date-frame them.
4. note (line 6 vs D-229): the review is dated 2026-10-22 in the note and 2026-10-23 in D-229.
5. note (lines 83, 98 vs D-235): G5 ends "then the repo goes public" without D-235's redaction, history-rewrite and flip-card gates.
6. note (line 85): G1 and G2 both lease `internal/comb/`, so "disjoint apart from `cmd/waggle/`" is wrong.
7. note (line 79 vs the brief): the brief says the approver check "moved to G3" though the note's G1 never held it, and moves lease-time protected paths to G2, which the note's G2 does not list.
