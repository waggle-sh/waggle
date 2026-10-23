# Guard review: hive control PR #95 (pin b038b41 to 851f707)

Verdict: pass
Risk-Tier: 3

Subagent guard hand-back, condensed by the queen; local paths replaced by `<scratch>`.

## Intent Summary
`waggle.pin` in `.waggle/control/config.yaml` moves from b038b41 to 851f707 (merge of T-032, #93), and DECISIONS.md appends D-246. The new build carries T-031 (gate verify binding, `gate.protected_paths` config) and T-032 (claim and release verbs, protected paths at lease time and in the post-run check, fully qualified default base). Blast radius is every later dispatch, claim and gate run in this hive; dispatch now parses the base config through `basePolicy` and exits 3 when it is invalid. This hive's config parses cleanly under the new build at the PR head and at main.

## Checks
- Scope: `git diff 3e586c5 49e8750` touches 2 files, the pin line (+1/-1) and a pure append to DECISIONS.md after D-245 (+19). `.github/`, `.waggle/control/roles/` and `.claude/` unchanged against origin/main. One commit, author and committer paulternate, noreply email, Signed-off-by only, no attribution, no dashes.
- Ancestry: 851f707 is entry 2 and b038b41 entry 17 on main's first-parent history; the move is forward.
- Product changes b038b41..851f707: `internal/config/config.go` adds an optional `gate:` section (only key `protected_paths`); this hive has none and parses with an empty list; no existing key changed meaning. Nothing changed under `internal/scaffold/`, no role template source changed, shim generation untouched.
- Upgrade replay from a clean 851f707 build (vcs.modified=false) on a checkout of 49e8750: pin unchanged, roster, devcontainer, all 5 roles, .gitignore and CLAUDE.md blocks `[same]`, seeded files skipped, shims regenerated, one `[rewrite]` of `.github/workflows/waggle-gate.yml` back to the stale template (drops the base-built gate verify step, widens permissions to write, adds `|| true` on the claim fetch). Reverting it per D-143 is correct; the branch keeps main's workflow byte for byte.
- New build on the branch: `waggle status` and `waggle doctor` exit 0. Dispatch's `load()` (config, policy, context, Keeper ID, runtimes, comb options) succeeds in a scratch-only test. `basePolicy` succeeds at 49e8750 and 3e586c5; `ExitEnv` is 3, matching D-246.
- D-246: `waggle gate decisions` reports ok, appended D-246; all fields present; Keeper by role only; no em or en dashes.

## Blocking Issues
None.

## Notes
- `verify-leases-and-tiers` fails as expected for a hive control PR (no T-<id> prefix, Tier 3 config path needing Head Keeper bypass), same as #79.
- The scaffold gate workflow template is still stale against main's workflow, so every `upgrade --apply` proposes the same unsafe rewrite and needs the manual D-143 revert. Worth a ticket so a pin move cannot quietly regress the gate.
- The queen replaced the default decide JSON's T-016 ticket and D-041 rationale with T-031, T-032 and a specific rationale.
