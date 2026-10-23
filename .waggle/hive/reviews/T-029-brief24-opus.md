# Guard review: PR #82 (T-029 brief fix 24, cf2e431 on 14e9da5)

Verdict: pass
Risk-Tier: 3

Subagent guard hand-back, condensed by the queen.

## Intent Summary
Brief and evidence only, no code: T-029 sections 2.4 (proving test recipe), 5, AC6 and 8, plus round 1 Colima output and `fix-removal.diff` under `media/T-029/`. The recipe forces auto gc in the fixture's local config and seeds 40 loose blobs with ids starting `17` between the plant's `mkdir` and the second worker commit, so only the worker-side `GIT_CONFIG_*` keys stop a detached gc whose `worktree prune` removes the planted entry. Lease, threat model and assertion unchanged.

## Experiment (host git 2.54.0)
- Without keys: `objects/17` 0 then 40 files; after the second commit the planted dir was gone and the `printf` failed (expected fail).
- With `maintenance.auto=false` and `gc.auto=0` via `GIT_CONFIG_COUNT`: entry present (expected pass).
- `maintenance.auto=false` alone: kept. `gc.auto=0` alone: pruned on 2.54. Without keys and without seeds: also pruned on 2.54.
- Not run on 2.39 (the container's git); the brief matches a source reading. A gc slower than `sleep 3` would show as a pass in both runs, which AC6 makes a must-fix.

## Checks
First commit stays under the threshold; the local config change is in the pre-run snapshot; seeded objects are outside `hostFiles`; assertion unchanged; lease rule sufficient; AC6 testable; no dashes; Keeper by role only; `fix-removal.diff` matches f5135e8.

## Blocking Issues
None.

## Notes
1. "Either key alone stops it" holds on 2.39 by source reading, not on 2.54.
2. The `+ ...` header lines in the round 1 evidence files are queen summaries, not the literal command (no `--context colima-waggle`, no mounts). Label or log the literal command in round 2.
3. Confirm the round 2 without-keys failure is the `Fatalf` with no `worktrees/planted` violation, not only a `t.TempDir` cleanup error.
4. The Status line still says "Not dispatched".

Verdict: pass
