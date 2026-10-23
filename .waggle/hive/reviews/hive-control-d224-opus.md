# Guard review: PR #74 (hive-fable-alternate, c79b742 on 5347024)

Verdict: must-fix
Risk-Tier: 3

Subagent guard hand-back, saved by the queen with local paths replaced by `<repo>` and `<scratch>`.

## Intent Summary
The PR adds `alternates: {standard: fable}` under `runtimes.adapters.claude-code` in `.waggle/control/config.yaml` and appends D-224, so that `waggle dispatch T-027 --role guard --second` can resolve a model after a sonnet builder and an opus first guard. Blast radius: guard model resolution on every ticket. The YAML parses and the bookkeeping is clean, but `pick()` tries each level's alternate before moving up a level, so fable displaces opus as the first guard and the second guard comes out as opus.

## Blocking Issues
- B1: the T-027 second guard resolves to opus and dispatch refuses it again. `Resolve` with `SecondGuard` re-derives the first guard from policy (`internal/runtime/policy.go` line 134); `pick` walks standard primary, standard alternate, then frontier, so policy thinks the first guard is fable, and the second guard avoids sonnet and fable and picks opus. The real first-guard record (`<repo>/.waggle/worktrees/runs/T-027-guard-1/result.json`) says opus, so `checkSecond` (`internal/dispatch/guard.go:93`) fails.
- B2: every later ticket's first guard silently changes from opus to fable for a standard-level builder at must-fix count below 2, Tier 2 and Tier 3 alike; neither D-224 nor the Keeper card records that.
- Probe (`config.LoadPolicy` plus `Policy.Resolve`, Tier 3, T-027): base gives first guard opus and second guard ErrNoCrossModel; this PR gives first guard fable (standard) and second guard opus.
- Suggested fix: `alternates: {frontier: fable}`. First guard stays opus, second guard fable; builder, forager, scout and queen unchanged; after a builder step-up the guard becomes fable at frontier and the second guard walks down to sonnet (D-179), where today both refuse. Record that in the decision.

## Checks that passed
- Parse and pinned binary: `standard` is a valid level, `fable` matches `modelPattern` and differs from the primary; pinned `bin/waggle status` exits 0; no non-test diff from the pin in `internal/runtime`, `internal/config` or `internal/dispatch`.
- Guard minimum: the walk-down is bounded by `max(LevelFast, Minimums[guard])`; builder, forager and step-up use `pick` with no avoid list and never read alternates.
- D-224 is a pure append; scan-hive `decisions: ok`; no dash hits; the Keeper is named by role only. `scan-names.local` was missing in the worktree, so the surname check did not run there.

## Process note
A `cp -R` probe copy shared the worktree's git index and staged the old config there; the guard reset that index entry at once, the working file was never modified, and the copy was deleted.

Verdict: must-fix
