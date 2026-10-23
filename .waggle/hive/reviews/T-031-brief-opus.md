# Guard review: T-031 brief (PR #87)

Verdict: must-fix
Risk-Tier: 2

Subagent guard hand-back (opus), round 1 at bb43ac6, condensed by the queen.

## Intent Summary
PR #87 adds .waggle/hive/tasks/T-031.md, the brief for gate step G1 (D-229): ticket binding by subject, `Waggle-Ticket:` trailer or branch (D-234); a raise-only `gate.protected_paths` key read from the base config; second-parent checks for merges from base; T-008's five Go dependency entries in `HardcodedTier3Prefixes`. Lease `cmd/waggle/`, `internal/comb/`, `internal/config/`, Tier 3, two guards. Through `refuseDirs` and `CheckChanges` it also changes dispatch lease refusal and the post-run check.

## Checks
1. Lease: pass; no import cycle; `refuseDirs` picks up the new list without a dispatch edit.
2. Gate and tier: pass.
3. ACs: linux-testable or marked review; contradictions in findings 2, 3 and 8.
4. Rule tightness: fail (finding 1); bounds gap (finding 5).
5. Budget: pass (233 bottom-up; 470, 600, 850; clean planned cut).
6. Boilerplate: pass; gofmt condition stale (finding 9).
7. Open questions: real decisions; brief written to option (a).
8. Style: pass.

## Findings
1. must-fix (lines 51, 53, AC8 line 101): the base-merge rule only requires p2 to be an ancestor of base and diffs p2..M. A crafted merge with an older base commit (or a later-merged side-branch commit) as p2 can revert base-side files, Tier 3 included, with no finding. Fix: also require `git merge-base --all <M> <baseOID>` to print exactly p2; add AC8 cases for both shapes.
2. must-fix (line 43, line 80, AC11 line 104 vs AC9 line 102): the brief says `CheckChanges` keeps its behavior and makes any change to its results a must-fix, but the T-008 entries change them for the gate and for dispatch's post-run check (internal/dispatch/check.go line 272). Reword to "unchanged except the five T-008 entries" and add the dispatch effect to the hive effect line.
3. must-fix (line 44 vs AC7 line 100): a `blob` type check cannot detect a symlinked config (mode 120000 is a blob), and `git cat-file` does not cleanly separate a missing path from a git failure. Specify `git ls-tree -z <baseOID> -- .waggle/control/config.yaml`: empty means missing, one blob entry with mode 100644 or 100755 is read after a size check, anything else exits 3.
4. note (line 33): "`t-31` and `T-031` agree" is wrong (regex needs 3 to 6 digits, `ParseTicketID` takes uppercase only). Pass the digit group to an unchanged `ParseTicketID`; `t-000` binds nothing.
5. note (line 80 vs 2.1): give numeric bounds for trailer values and branch length, and say whether values are space-trimmed.
6. note (line 52): a base merge in a two-ticket PR fails on the other ticket's paths; state it as an accepted limitation (rebase instead).
7. note (line 5): after G1 this hive's brief branches bind by branch; records and brief PRs still fail leases, with a different message.
8. note (lines 35, 51): new messages break substrings asserted by TestVerifyNonTicketCommits and TestVerifyMergeCommit; say the change is expected or keep the old substrings.
9. note (line 11, section 6): PR #85 merged; make the gofmt condition unconditional.
10. note (line 44, out of scope): `config.Parse` validates the whole base config, so version skew with newer keys matters for G4's pinned Action.
