# Guard review: ruled repositioning note round 2 (PR #86)

Verdict: pass
Risk-Tier: 2

Subagent guard hand-back (fable confirming guard), round 2 at c6e94e0 on 71c9155, condensed by the queen.

## Intent Summary
c6e94e0 changes nine lines: third-party claims attributed to the review (lines 11, 60, 66), two risk bullets dated as of 2026-10-23 (94, 95), G5 public only after redaction, the history rewrite and the Head Keeper's flip card per D-235 (83, 98), the G1/G2 lease sentence fixed (85), and G2 given the lease-time and post-run protected-path work with `internal/dispatch/` added and size 150 to 200 (80).

## Checks
1. Diff scope: 9 insertions, 9 deletions, each mapped to an opus note or the G2 widening; section 7 untouched.
2. Rulings D-229 and D-231 to D-236 all stated correctly, including D-235's gates and kill criterion.
3. G2 widening consistent: the post-run check is in internal/dispatch/check.go and AcquireLease in internal/comb/, so G2 needs both; about 50 more lines is fair.
4. Public safety: no home paths, emails, machine or personal names, no em or en dashes.

## Findings
1. note (line 6 vs D-229): review dated 2026-10-22 in the note and 2026-10-23 in D-229.
2. note (line 52): section 5 Go sizes now sum to about 1,200 (about 2,800 doubled), not 1,150 and 2,700.
3. note (line 47 vs 80): the internal/dispatch row still says "Nothing for the gate" though G2 now reads gate.protected_paths in the post-run check.
4. note (line 80): "assigned here by T-031" would be more exact than "moved here from G1".
