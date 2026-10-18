# Queen handoff

Written by the queen session on 2026-10-18 (sixth session). A fresh queen session starts here. Read this file, then `.waggle/hive/tasks/T-006-backfill.md` only when you need a specific decision. The previous version lived at `docs/handoff/queen.md`; this is now the only copy.

You are the queen (`.waggle/control/roles/queen.md`): you talk to the Keeper (Paul, the Head Keeper), you surface decisions only through AskUserQuestion cards, you never write feature code or specs, and you never wait on workers. Until `waggled` exists, you dispatch workers as Claude Code subagents. Our hive's shims are `.claude/agents/waggle-{queen,architect,scout,forager,guard}.md` (old role names, by design, until T-020 phase C); the Agent tool types that worked this session were `architect`, `forager`, `guard`. The guard always runs on a different model than the builder (`model: sonnet`). Workers get `isolation: worktree`; branches are created inside the worktree from `origin/main`.

## Framing the Keeper insists on

Two layers. Product is Waggle itself (cmd/, internal/ including internal/scaffold/templates/, go.mod, README, CONTRIBUTING, docs/). Hive is our own installation used to build it: `.waggle/control/` (config, roster, roles, WAGGLE.md, devcontainer), `.waggle/hive/` (tasks, reviews, media, handoff, DECISIONS.md once T-006 phase 2 runs) and the gate workflow. Nothing in the hive ships. Keep the two apart in every brief and every answer.

## State of the repo (main at `6b84500` when this was written)

- GitHub `waggle-sh/waggle`, private, Team plan (D-084). Ruleset `main` (id 25310890), edited at the end of this session for T-015: `product-ci` and `verify-leases-and-tiers` required, branches up to date, force-push blocked, deletion restricted, pull request rule with code owner review required (count 1, stale reviews dismissed), admin bypass always. Because GitHub never counts the PR author's own approval, every merge now needs `gh pr merge <n> --merge --admin` (logged bypass). Auto-merge is disabled at the repo level, so merge with a polling loop. Record this as D-098 (policy, T-015) in the backfill in the next batch; not yet recorded. Auth: `env -u GITHUB_TOKEN -u GH_TOKEN gh ...` from the main checkout; inside a worker worktree push with `git -c credential.helper= -c 'credential.helper=!gh auth git-credential' push ...`.
- Hive layout is post-T-011: pinned to `b4f222166f4a824e34414e205544fb17397ccfa1` (full hash, D-087, D-096). Generated `CLAUDE.md` is untracked in every checkout until D-097 is implemented by T-016 (`?? CLAUDE.md` is expected). `docs/handoff/` is gone; this file replaced it.
- Merged today, in order, each merge commit carrying the guard's Intent Packet: T-020 phase A `924a59f` (#9), T-011 phase B `86f5f5b` (#10), T-011 phase C `06da7bc` (#11), T-011 C-5 `a99cfc2` (#12), T-009 `aaf9fa5` (#13), T-004 `f690987` (#16), T-022 phase A `4a036b5` (#19), T-006 phase 1 `8ade7c2` (#14), T-018 phase A `f6c0fbf` (#15), T-012 phase A `5bc92f0` (#18), T-005 `9765a57` (#20), T-003 `fedf68e` (#17).
- First wave dispatched at the end of the session as forager subagents in isolated worktrees: T-007 (branch `t-007`), T-016 (branch `t-016`), T-021 phase A (branch `t-021a`). If you are a fresh session, their reports are lost: check `git branch -r` and `gh pr list`; a pushed branch means the build finished, so open a draft PR and dispatch the guard; a missing branch means re-dispatch from the brief.
- T-015 is Done: CODEOWNERS merged at `6b84500` (#21) and the ruleset edit applied and read back. Update `T-015.md`'s status line in the next batch.
- Hooks: `.claude/settings.json` (committed with the hive) runs `.claude/hooks/session-start.sh` on startup, resume, clear and compact (prints this file and the live repo state) and `.claude/hooks/handoff-check.sh` on Stop (refuses to end a turn while this file is older than main's last commit, so refresh it after every merge batch; `touch` is not a refresh, rewrite the facts).
- Hive records batch 1 merged at `72d4207` (#22). From now on every guard review, brief edit and handoff refresh is an uncommitted tracked change under `.waggle/hive/` until the next batch PR; open `hive-records-2` the same way when a batch of merges is done.

## Decisions

D-001 to D-097 are in `.waggle/hive/tasks/T-006-backfill.md`. Today's: D-086 (worker drafts T-011 phase C), D-087 (pin b4f2221), D-088 (T-006 budget waiver, 200-byte titles), D-089 (size metric is physical non-test lines; T-018 B and C re-estimated to 500 and 650), D-090 to D-097 (T-016 design points, all queen defaults approved). `waggle decide` now exists on main; T-006 phase 2 (the backfill, 87 entries, loops 1 to 87) is the next hive-layer task and the Keeper approves that PR.

## Next work, in order (all briefs in `.waggle/hive/tasks/`, each lists its lease)

1. Collect the first wave (T-007, T-016, T-021 A): draft PR, guard on sonnet, merge with `--admin` on green (Tier 2 on guard pass; Tier 3 on a card).
2. Record D-098 and update T-015's status, then hive records batch 2.
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

## Live state (sixth session, 2026-10-18, main at `3063955`)

Merged this session: T-014 phase B `3063955` (#36, Tier 3, Keeper card, --admin). It carries a Head Keeper commit `81f9b89` adding `/.github/workflows/` and `/.github/actions/` to CODEOWNERS (fable F4). Fix verification PASS appended to `reviews/T-014b-sonnet.md` (550 added non-test lines, B1, B2/F1, F2, F3 fixed, sign-offs present). The new gate (`waggle gate verify`) now runs on every PR; hive-record and Head Keeper commits fail `leases` by design and merge by bypass.

DECISIONS.md: 137 entries. Next ID D-138. New decisions go through `waggle decide --file <json>`.

Keeper rulings NOT yet recorded (hive-records-5):
- D-138 (policy, T-020): T-020 phase C is drafted by a worker (D-086 pattern) after hive-records-4 merges and nothing else is running; two guards (sonnet, fable); Head Keeper approval at merge; the Keeper renames his own local.yaml keys.
- D-139 (policy, T-014): T-014 phase B accepted at 550 added non-test lines against the 450 stop; overrun is D-107 and D-114 scope plus the B1 fix (D-089, D-136).
- D-140 (policy, T-014, T-015): CODEOWNERS covers `.github/workflows/` and `.github/actions/`, so workflow edits need code owner review (fable F4: pull_request workflows run from the merge ref).
- D-141 (policy, T-014): comb keeps rejecting claims under `.waggle/`; every hive-record commit fails `leases` and merges by Head Keeper bypass (fable F10). Revisit only if bypass volume becomes a problem.

Running: T-020 phase C draft (architect subagent, isolated worktree, branch `t-020c`). If its report is lost, check `git branch -r` for `t-020c`; pushed means open a draft PR and dispatch sonnet and fable guards; missing means re-dispatch from `T-020.md` section 7.

Uncommitted hive files for hive-records-5: `reviews/T-025a.md`, `media/T-025a/hashes.txt`, `reviews/hive-records-4.md`, `reviews/T-014b-sonnet.md`, `reviews/T-014b-fable.md`, `media/T-014b/`, this handoff.

Open notes: `media/T-023a/fable-run2-tools.jsonl` holds the Keeper's git email and a `~/Repos/znap` path; redact before the repo goes public. T-025 A sign-off used cleary.paul@protonmail.com; Keeper to decide before phase D (DCO enforcement). T-025 phase C must merge before any release. T-014 AC7 is deferred to post-merge: the Keeper opens two scratch PRs (out-of-lease edit, `GO.MOD` edit) and drops screenshots in `media/T-014/` (fable F9). T-008 brief must fix or lease `cmd/waggle/gate_verify_test.go` test 8, which asserts `GO.MOD` is not Tier 3 (fable F3). Fable F5 (CheckChanges canonical-path precondition) goes into the T-023 brief that calls it; F6 and F8 are small follow-ups for the next `cmd/waggle/` lease.

Next work, in order:
1. T-020 C: collect draft, PR, sonnet and fable guards, Keeper card (Head Keeper approval). Hive-records-5 can follow it or ride with it.
2. Critical path after T-020 C: T-012 B (with init symlink and roster.json fold), then T-021 B, then T-023 B. T-008 now unblocked on T-014 B, still after T-012 B. T-018 C now unblocked. T-025 B (Head Keeper commit) then C.
3. T-024 phases A to E after all of T-023 merges (D-123); phase F after the run.sock loop.

Habits from these sessions: merge with a backgrounded poll script (`gh pr ready`, `gh pr update-branch`, poll `gh pr checks`, `gh pr merge --merge --admin` with the Intent Packet in `--body-file`); in zsh use `unset GITHUB_TOKEN GH_TOKEN`; after a merge that adds files you hold untracked locally, diff against `git archive origin/main` and delete identical local copies before pulling; never switch the main checkout's branch to commit, use a worktree (scratchpad is fine); scan builder evidence for secrets before copying into media; Tier 3 gets sonnet plus fable guards; guards write reviews straight into the main checkout; agents can't be resumed after /clear, so re-dispatch fresh on the same worktree.
