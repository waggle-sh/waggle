# Queen handoff

Written by the outgoing queen session on 2026-10-17 (third session of the day). A fresh queen session starts here. Read this file, then `.waggle/hive/tasks/T-006-backfill.md` only when you need a specific decision. The previous version lived at `docs/handoff/queen.md`; this is now the only copy.

You are the queen (`.waggle/control/roles/queen.md`): you talk to the Keeper (Paul, the Head Keeper), you surface decisions only through AskUserQuestion cards, you never write feature code or specs, and you never wait on workers. Until `waggled` exists, you dispatch workers as Claude Code subagents. Our hive's shims are `.claude/agents/waggle-{queen,architect,scout,forager,guard}.md` (old role names, by design, until T-020 phase C); the Agent tool types that worked this session were `architect`, `forager`, `guard`. The guard always runs on a different model than the builder (`model: sonnet`). Workers get `isolation: worktree`; branches are created inside the worktree from `origin/main`.

## Framing the Keeper insists on

Two layers. Product is Waggle itself (cmd/, internal/ including internal/scaffold/templates/, go.mod, README, CONTRIBUTING, docs/). Hive is our own installation used to build it: `.waggle/control/` (config, roster, roles, WAGGLE.md, devcontainer), `.waggle/hive/` (tasks, reviews, media, handoff, DECISIONS.md once T-006 phase 2 runs) and the gate workflow. Nothing in the hive ships. Keep the two apart in every brief and every answer.

## State of the repo (main at `fedf68e` when this was written)

- GitHub `waggle-sh/waggle`, private, Team plan (D-084). Ruleset `main` (id 25310890): `product-ci` required, branches up to date, admin bypass. Auto-merge is disabled at the repo level, so merge with a polling loop. Auth: `env -u GITHUB_TOKEN -u GH_TOKEN gh ...` from the main checkout; inside a worker worktree push with `git -c credential.helper= -c 'credential.helper=!gh auth git-credential' push ...`.
- Hive layout is post-T-011: pinned to `b4f222166f4a824e34414e205544fb17397ccfa1` (full hash, D-087, D-096). Generated `CLAUDE.md` is untracked in every checkout until D-097 is implemented by T-016 (`?? CLAUDE.md` is expected). `docs/handoff/` is gone; this file replaced it.
- Merged today, in order, each merge commit carrying the guard's Intent Packet: T-020 phase A `924a59f` (#9), T-011 phase B `86f5f5b` (#10), T-011 phase C `06da7bc` (#11), T-011 C-5 `a99cfc2` (#12), T-009 `aaf9fa5` (#13), T-004 `f690987` (#16), T-022 phase A `4a036b5` (#19), T-006 phase 1 `8ade7c2` (#14), T-018 phase A `f6c0fbf` (#15), T-012 phase A `5bc92f0` (#18), T-005 `9765a57` (#20), T-003 `fedf68e` (#17).
- First wave dispatched at the end of the session as forager subagents in isolated worktrees: T-007 (branch `t-007`), T-016 (branch `t-016`), T-021 phase A (branch `t-021a`). If you are a fresh session, their reports are lost: check `git branch -r` and `gh pr list`; a pushed branch means the build finished, so open a draft PR and dispatch the guard; a missing branch means re-dispatch from the brief.
- Open draft PR #21 `t-015` (CODEOWNERS, guard pass in `.waggle/hive/reviews/T-015.md`). Needs a Keeper card: merge the file, and apply the ruleset edit (require code owner review count 1, keep admin bypass, add non-fast-forward and deletion rules, require `verify-leases-and-tiers` as well as `product-ci`). Gotcha: GitHub never counts the PR author's own approval, so with one seat every merge afterwards needs `gh pr merge --admin` (logged bypass). The Keeper has not yet been told the exact call; the draft is in the T-015 architect's report, reproduce it from the brief section 4 and the live ruleset JSON (`gh api repos/waggle-sh/waggle/rulesets/25310890`).
- Hooks: `.claude/settings.json` (committed with the hive) runs `.claude/hooks/session-start.sh` on startup, resume, clear and compact (prints this file and the live repo state) and `.claude/hooks/handoff-check.sh` on Stop (refuses to end a turn while this file is older than main's last commit, so refresh it after every merge batch; `touch` is not a refresh, rewrite the facts).
- Uncommitted hive records in the main checkout (16 modified tracked files plus untracked reviews and media for T-003, T-004, T-005, T-006, T-009, T-012a, T-015, T-018a, T-022a, this file): the Keeper chose "one hive PR per batch, Head Keeper approves". Next step: branch `hive-records-1` from main, `git add .waggle/hive`, commit `T-011: hive records batch 1 (Head Keeper)`, PR, merge on green. Guard reviews need no second review.

## Decisions

D-001 to D-097 are in `.waggle/hive/tasks/T-006-backfill.md`. Today's: D-086 (worker drafts T-011 phase C), D-087 (pin b4f2221), D-088 (T-006 budget waiver, 200-byte titles), D-089 (size metric is physical non-test lines; T-018 B and C re-estimated to 500 and 650), D-090 to D-097 (T-016 design points, all queen defaults approved). `waggle decide` now exists on main; T-006 phase 2 (the backfill, 87 entries, loops 1 to 87) is the next hive-layer task and the Keeper approves that PR.

## Next work, in order (all briefs in `.waggle/hive/tasks/`, each lists its lease)

1. Hive records PR: opened as `hive-records-1` at the end of this session (check `gh pr list`); merge on green, Head Keeper approved on the card.
2. T-015 card (merge plus ruleset edit), then switch merges to `--admin`.
3. Already dispatched: T-007, T-016, T-021 phase A (see above). Next, disjoint leases: T-012 phase B (`internal/runtime/`, `internal/config/` new files, `internal/scaffold/init.go` Materialize call only; serialize with T-016 on `internal/scaffold/`, so run T-016 first or T-012 B first, not both), T-008 (`internal/comb/` backend.go and `internal/scaffold/` gate template; needs T-005 merged; serialize with T-014 A and with T-016 on scaffold), T-006 phase 2 backfill (hive PR, Head Keeper approves; the Keeper runs or a worker runs `waggle decide` 87 times from `internal/decisions/testdata/backfill/D-*.json`). Suggested first wave: T-007, T-016, T-021 A. Second wave: T-012 B, T-008, T-014 A, T-018 B (expect about 500 lines).
4. T-020 phase C after T-016 merges (hive rename through `waggle upgrade`, brief sweep, D-097 CLAUDE.md commit as step 0).
5. Open with no decision yet: Cell runtime (needs an RFC amendment; queen recommends a CellRuntime interface with an OS-sandbox default, Seatbelt on macOS and bubblewrap on Linux, host egress proxy, plus a container backend), licensing details (Apache-2.0 for CLI and daemon, MIT-0 for templates, copyright Level 19 Enterprises, LLC per D-046; DCO and SPDX headers to confirm), forge order after GitHub (D-069, after v0.1), T-023 launcher brief, T-021 phase C.

## Habits that worked

- Brief first (architect), build in an isolated worktree (forager; architect for docs), review on sonnet (guard) with explicit hostile probes in the prompt, Intent Packet in the merge commit, remove the worktree after merge (`git worktree remove --force`, then `git branch -D`).
- Open the draft PR yourself right after the build report so CI runs during review; mark ready and merge on green with a backgrounded poll loop. When main moved, `gh pr update-branch <n>` first (ruleset requires up to date), then wait for CI again.
- Tier 2: merge on guard pass without a card when the Keeper has said to keep building. Tier 3 and anything touching policy, budgets, schema limits or spend: card, recommended option first, cost and reversibility stated. Batch rulings on one card.
- Tell foragers which role names are current (queen, scout, forager, builder, guard) and that briefs predate the rename; two drafts reintroduced `architect` and were sent back.
- Foragers blow line budgets by 1.5x to 2.6x; D-089 sets the metric. Expect it and let the guard rule.
- Keep Keeper messages short, lead with the answer, no em or en dashes anywhere.
- Hand off again before context passes about 200k, and refresh this file after every merge batch.
