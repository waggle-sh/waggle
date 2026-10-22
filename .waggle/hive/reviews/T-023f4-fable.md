<!-- SPDX-License-Identifier: MIT-0 -->
# T-023 phase F follow-up (t-023f-retire-legacy): guard review, fable

verdict: pass
Risk-Tier: 3

Branch `origin/t-023f-retire-legacy` at `164ae93` (two commits, `9811d4c` and `164ae93`) against `origin/main` at `aac0a18`. Six files, 12 insertions, 50 deletions. Control PR for runbook `.waggle/hive/tasks/T-023-trial.md` section 8.2 item 2. Merged as #70 `6fd0d0f`.

## Intent summary

Removes the transitional Stop hook (`handoff-check.sh` deleted, its `Stop` entry dropped from `.claude/settings.json`), drops the `queen.local.md` branch from `session-start.sh` (newest `queen-<n>.md`, else the committed summary), removes the `queen.local.md` line from `.gitignore`, rewrites the Queen summary bullet in WAGGLE.md section 4 to the post-cutover rules, and updates the header and Hooks fact in `queen.md`. The dropped branch-delete sentence is correct (`git ls-remote` shows none of the four branches). Blast radius: the queen's session hooks and two control documents; no product code, leases, signals or git state.

## Checks

1. settings.json parses; its only hook key is `SessionStart`, byte-equal to main's.
2. session-start.sh passes `sh -n`; run read-only against a scratch repo: no handoffs dir prints `queen.md` (or `(missing)`); `queen-1.md` plus `queen-10.md` picks `queen-10.md`; `queen-x.md`, `queen-.md`, `queen-1234567890.md` are skipped; the main checkout prints `queen-1.md`. Exit 0 in every case, including unset `CLAUDE_PROJECT_DIR` outside a repo. Nothing can block a turn.
3. `git grep` for `handoff-check`, `queen.local`, `Stop hook`, `40 lines` finds only merged-brief history, the product fixture in `internal/runtime/launch_test.go`, and N1, N2. Nothing depends on the deleted file at runtime.
4. The WAGGLE.md bullet keeps every true rule (handoff path, gitignored; summary scope and cadence; Keeper by role; no hook checks either file; SessionStart order) and drops only the pre-cutover ones. D-215 and D-220 exist.
5. No em or en dashes, no personal name or AI attribution, noreply author and committer with sign-off. scan-hive: 0 hits, decisions unchanged, exit 2 (expected for a control PR).

## Blocking issues (must-fix)

None.

## Notes

N1. `.waggle/hive/handoff/queen.md:64` still lists this PR as Next work item 1. Suggested: "1. WAGGLE.md section 4 Hive scan bullet: still says "each non-merge commit", and scan-hive prints path-name hits as is (leftovers from runbook 8.2)."

N2. `.waggle/hive/tasks/T-025.md:117` names the deleted file. Not must-fix (unstarted brief). Suggested: "4. `.claude/hooks/session-start.sh`: add the MIT-0 line after the shebang (handoff-check.sh was removed after the T-023 phase F cutover, D-220)."

N3. The references in `T-023.md` and `T-023-trial.md` are merged-brief history and stay.
