<!-- SPDX-License-Identifier: MIT-0 -->
# T-023 phase F follow-up control PR: guard review (opus)

verdict: pass

Risk-Tier: 3

Branch `t-023f-retire-legacy` at `164ae93` (two commits: `9811d4c`, `164ae93`) against `origin/main` at `aac0a18`. Merged as #70 `6fd0d0f`.

## Intent Summary

Retires the legacy queen handoff path after the cutover (D-220): deletes the transitional Stop hook `.claude/hooks/handoff-check.sh` and its `Stop` entry in `.claude/settings.json`; the SessionStart hook now picks the newest `.waggle/control/handoffs/queen-<n>.md`, else the committed `.waggle/hive/handoff/queen.md`; drops the `queen.local.md` line from `.gitignore`; trims the Queen summary bullet in WAGGLE.md section 4 to the rules that still hold; updates the matching summary sentences. Also removes the stale "Remote branches for the Keeper to delete" sentence (`git ls-remote origin` shows all four gone). Blast radius: only the queen's interactive session hooks; no product code, leases, claims, signals or deletes.

## Checks

1. settings.json parses; the SessionStart block is byte-identical to main; only `Stop` is removed.
2. session-start.sh, branch copy run read-only with `sh` (and `dash` once), stdin `/dev/null`: every run exits 0 and the hook has no exit 2 path. No handoffs dir prints the summary; `queen-1.md`, `queen-2.md`, `queen-10.md` prints `queen-10.md` (numeric compare); `queen-x.md`, `queen-.md`, `queen--1.md`, `queen-1234567890.md` are ignored; summary missing prints `(missing)`; unset `CLAUDE_PROJECT_DIR` outside a repo exits 0 silently; the main checkout prints `queen-1.md`.
3. Remaining references are history (`session-start.sh:7`, `queen.md:3`, `T-023-trial.md:123-124`, `T-023.md:186`), a self-contained product fixture (`internal/runtime/launch_test.go:238-244`), or stale notes (N1, N2). No settings file references `handoff-check`.
4. The WAGGLE.md bullet keeps every rule still true and drops only the transitional ones; "No hook checks either file." is now accurate.
5. No em or en dashes, Keeper by role only, nothing new is private, no AI attribution trailer, DCO sign-off on both commits.

## Blocking Issues (Must-Fix Only)

None.

## Notes

N1. `.waggle/hive/handoff/queen.md:64`: Next work item 1 describes this PR and goes stale on merge; keep its two scan-hive leftovers. Suggested: "1. WAGGLE.md section 4 still says "each non-merge commit" and scan-hive prints path-name hits as is (control PR, Tier 3)."

N2. `.waggle/hive/tasks/T-025.md:117`: phase B still names the deleted hook. Not must-fix (a brief, nothing runs it). Suggested: "4. `.claude/hooks/session-start.sh`: add the MIT-0 line after the shebang (`handoff-check.sh` was removed at the T-023 phase F cutover, D-220)."

N3. `.claude/hooks/session-start.sh:7`: the retirement line is accurate history; it can go at the next edit of the hook.
