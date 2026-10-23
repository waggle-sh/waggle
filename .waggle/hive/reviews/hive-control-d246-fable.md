# Guard review: hive control PR #95 (pin b038b41 to 851f707)

Verdict: pass
Risk-Tier: 3

Subagent guard hand-back, condensed by the queen; local paths replaced by `<scratch>`.

## Intent Summary
`waggle.pin` in `.waggle/control/config.yaml` moves from b038b41 to 851f707 (main's merge of T-032, #93), and DECISIONS.md appends D-246. One commit, 49e8750, parent 3e586c5; two files, +20/-1. After merge the hive runs T-031 (gate verify) and T-032 (claim and release verbs, protected paths at lease time and in the post-run check, fully qualified default base). No product change between the pins touches `internal/scaffold/`, `.github/`, role templates or shim generation; `internal/config/` gains an optional `gate.protected_paths` section this hive does not use.

## Checks
- Scope: only the pin line and DECISIONS.md change; `.github/`, `.waggle/control/roles/` and `.claude/` byte-identical to origin/main.
- DECISIONS.md: old file is a byte prefix of the new one; D-246 follows the D-227 shape, numbering continues from D-245, Keeper by role only, no dashes, no name hits; `decisions: ok, appended D-246`.
- Ancestry: 851f707 is entry 2 and b038b41 entry 17 on main's first-parent history.
- Product changes (15 first-parent merges, 55 files): config adds optional `gate: protected_paths`; `internal/dispatch/run.go` adds `basePolicy` (exit 3 on a base config parse error); `inputs.go` and `offBranch` resolve the default base as `refs/remotes/origin/main`, which this hive has; `cmd/waggle/main.go` adds `claim` and `release`. No diff under `internal/scaffold/` or `.github/`.
- Replay from an own clean 851f707 build: on 49e8750 every managed file `[same]`, seeded files skipped, single `[rewrite]` the stale gate workflow template, kept as on main per D-143. From origin/main the plan is `[pin] b038b41 -> 851f707` with the identical file list.
- `upgrade --apply` in a scratch clone: regenerated shims byte-identical to the main checkout's current shims, so post-merge shim regeneration is a no-op.
- `waggle status` from the new build on 49e8750 exits 0; config loads, so dispatch will not hit the new exit 3.
- `go test ./internal/config/ ./internal/scaffold/ ./cmd/waggle/` ok on the new build. scan-hive 0 hits, exit 2 (control file).

## Blocking Issues
None.

## Notes
- After merge, rebuild `bin/waggle` from a clean 851f707 tree (vcs.modified=false) or `upgrade` reports an unpinned build.
- D-246's "all unchanged" alongside the gate workflow revert clause matches D-227 wording and stays consistent.
