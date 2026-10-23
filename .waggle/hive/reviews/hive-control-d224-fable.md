# Guard review: hive control PR #75 (hive-fable-alternate-2, d55c574 on 5347024), confirming round

Verdict: pass
Risk-Tier: 3

Subagent guard hand-back, saved by the queen with local paths replaced by `<repo>`.

Intent Summary: `.waggle/control/config.yaml` adds `alternates: {frontier: fable}` under `runtimes.adapters.claude-code` (2 lines) and DECISIONS.md appends D-224. No Go code changes. Blast radius: guard model selection only, and only where base refused with ErrNoCrossModel.

Evidence (throwaway test in a separate worktree, `config.LoadPolicy` on the branch against `config.ParsePolicy` on base, plus the real `checkSecond`; removed afterwards):
- Tier 3 T-027, must-fix 0: builder sonnet/standard, first guard opus/standard, second guard fable/standard; base refused the second guard. `checkSecond` with builder sonnet and guard opus at base 5347024 accepts fable and rejects opus. The real records `<repo>/.waggle/worktrees/runs/T-027-builder-1` and `T-027-guard-1` are sonnet and opus at 5347024.
- Unchanged versus base for must-fix 0 to 3 and tier 3, 2 and nil: builder, forager haiku, scout opus, queen opus, Tier 2 single guard opus/standard.
- Stepped (must-fix >= 2, builder opus/frontier): guard fable/frontier, second guard sonnet/standard; base refused both. Matches D-224. The walk-down is bounded at standard and no guard resolution went below it. Alternates are read only when `avoid` is non-empty.
- D-224 matches the behavior; base DECISIONS.md is a byte prefix of the branch file; Keeper by role only; no dashes.
- Pinned `bin/waggle status` exits 0; `go vet` and `go test -race` on internal/config, internal/runtime, internal/dispatch pass.

Blocking Issues: none.

Notes: the config.yaml alternates comment is long but accurate. scan-hive exits 2 by design for a control change.

Verdict: pass
