# Guard review: hive control PR #79 (pin 790ade0 to b038b41)

Verdict: pass
Risk-Tier: 3

Subagent guard hand-back, condensed by the queen; local paths replaced by `<scratch>`.

## Intent Summary
`waggle.pin` in `.waggle/control/config.yaml` moves from 790ade0 to b038b41 (main's merge of T-028), and DECISIONS.md appends D-226 and D-227. One commit, 4e42f65, parent b038b41; two files, +40/-1. The hive's dispatches now run T-027 (Bash task line), T-030 (release) and T-028 (scratch TMPDIR exemption); none changes config parsing, role templates, scaffold templates or shim generation.

## Checks
- Scope: only the pin line and a pure append to DECISIONS.md; `.github/`, `.waggle/control/roles/` and `.claude/` equal origin/main.
- Ancestry: b038b41 is origin/main's tip; first-parent history from 790ade0 holds 62f3f46 (#73), 4a65fc4 (#76) and b038b41 (#78).
- Product changes 790ade0..b038b41: no diff in `internal/scaffold/` or `.github/`; `internal/config/` test-only; dispatch and runtime changes as reviewed under T-027, T-028 and T-030.
- Replayed `upgrade --diff` and `--apply` from a stamped b038b41 build (private clone in `<scratch>`) against a snapshot of 4e42f65: every managed file `[same]`, seeded files skipped, the only `[rewrite]` the stale gate workflow template, kept as on main per D-143.
- D-226 and D-227: well formed, Keeper by role only, no dashes, no name-list hits; scan-hive 0 hits, `decisions: ok, appended D-226, D-227`.
- New binary: `status` exits 0 on the main checkout and on the snapshot (`pin b038b41`).

## Blocking Issues
None.

## Notes
- Release T-028's claim after merge rather than wait out the TTL.
- D-227's "all unchanged" holds together with the gate workflow revert clause.
- No refs or worktrees were created in the shared repo; the scratch clone was deleted.

Verdict: pass
