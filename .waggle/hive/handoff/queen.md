# Queen handoff

Written by the outgoing queen session on 2026-10-17 (third session of the day). A fresh queen session starts here. Read this file, then `.waggle/hive/tasks/T-006-backfill.md` only when you need a specific decision. The previous version lived at `docs/handoff/queen.md`; this is now the only copy.

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

## Live state (fourth session, main at `c94df60` plus in-flight PRs)

Merged this session: T-021 A `de82ff9` (#25), T-007 `2dcadfc` (#23), T-016 `250f331` (#24), hive-records-2 `e1f2c2e` (#26, D-098 to D-100), T-014 A `c94df60` (#27, Tier 3, Keeper card).

Keeper rulings this session NOT yet recorded (record in hive-records-3, next D numbers after D-100): (1) Cell runtime: CellRuntime interface, OS sandbox default (Seatbelt macOS, bubblewrap Linux) behind a host egress proxy, plus a Docker-compatible container backend; RFC-0003 amendment drafted by architect, Head Keeper approves (strategy). (2) Licensing: Apache-2.0 CLI and daemon, MIT-0 templates, DCO, SPDX headers, copyright Level 19 Enterprises, LLC (policy). (3) GitHub merge queue treated as unavailable on Team; stay on Team, admin-merge loop stays, native_queue opt-in, doctor reports it, revisit when public (policy). (4) Queen commits the generated CLAUDE.md for the Head Keeper (PR #29, approved). (5) T-018 B accepted at 687 lines vs 650 stop after the Keeper-requested N1/N2/N4 hardening (policy, like D-099). (6) T-006 phase 2: a forager adds the missing backfill JSON (D-088 onward) and runs waggle decide for all entries in a hive PR the Keeper approves. (7) Review follow-ups are folded into the brief that next touches the code (T-014 B: lease.json size cap; T-018 C: N3, N5 to N8; scaffold ticket or T-012 B: roster.json escaping and init symlink hole; T-006: Parse bare CR). Remove Cell runtime, Licensing and merge queue from Appendix B when recorded.

In flight:
- PR #29 merged: CLAUDE.md is tracked on main (D-097 step 0 done). T-020 C is unblocked.
- PR #28 T-018 B: re-guard pass at `c85c783`, Keeper approved merge; merge loop running. N9 and N10 join N3, N5 to N8 for the T-018 C brief. Worktree `.claude/worktrees/agent-a433dd662d67a7084`.
- Architect on branch `t-023-brief`: T-023 waggle dispatch brief (Keeper approved).
- Architect on branch `t-024-cells-license`: RFC-0003 Cells amendment draft plus Cells build brief plus licensing brief (Keeper approved both designs).
- Uncommitted hive files: reviews T-014a, T-018b and media dirs.

Next: hive-records-3 (the seven rulings above, reviews, follow-up folding, new briefs), then T-006 phase 2 forager, then T-020 C (two guards per D-100), then T-012 B and T-008, T-014 B.

### Keeper rulings on T-023, T-024, T-025 open questions (2026-10-17, all queen defaults approved on cards; record as D-entries in hive-records-3)

- Merged since: T-018 B `b1e4a3a` (#28, Tier 3, 687 lines accepted), CLAUDE.md `97baeda` (#29). Briefs pushed, not merged: `t-023-brief` `85cf224` (T-023), `t-024-cells-license` `4d54f20` (T-024 Cells, T-025 licensing, RFC-0003 amendment A2 draft in docs/).
- T-023: Q1 dispatch first, host-only with CellRuntime seam (supersedes D-083 wording for T-023). Q2 host runs only with `--no-cell` per invocation, unenforced list printed and recorded. Q3 tool allowlist plus deny list (no git push, gh, curl, wget, ssh, web). Q4 workers use the Keeper's Claude Code login, user settings excluded. Q5 lease from `--dir` and `--gate` flags. Q6 briefs read only from base (briefs merge before first dispatch). Q7 (technical, queen default) shared `comb.CheckChanges`. Q8 dispatch pushes after checks, never force, never opens PRs, `--no-push`. Q9 phase D runs the D-100 second guard. Q10 one trial ticket after phase D, cutover on a card, subagents as rollback. Q11 keeper_id is the GitHub login, placeholder refused with a remote. Q12 phase F sets `forge.remote: origin`.
- T-024: Q1 default is proxy-injected key with zero credentials, and our hive opts into `cell.inference_credential: passthrough` (doctor reports the target unenforced). Q2 `.git` read-only, waggled commits on host. Q3, Q6, Q9 queen technical defaults (Out dir copied by waggled; registries closed, GOPROXY=off with ro module cache; docker/podman directly). Q4 phases A to E after T-023 merges, F after run.sock loop. Q5, Q10, Q11 scope defaults (container Linux/CI only on macOS unless socket check passes; no OS sandbox resource limits; no seccomp in v0.1, follow-up ticket). Q7 path-filtered macOS and Linux CI jobs. Q8 CODEOWNERS `/internal/cell/ @paulternate`. Q12 refuse `enforce_ro_root: false`.
- T-025: Q1 SPDX identifier only, copyright in NOTICE. Q2 one rule: product Apache-2.0, templates and hive MIT-0. Q3 (technical) Go test in `internal/license/`. Q4 `dco` CI job, required in the ruleset. Q5 workers commit under the Keeper's identity with `-s`; Keeper certifies. Q6 to Q11 scope defaults (merge commits signed off; no retroactive sign-off; hive unchecked; JSON exempt; LICENSE, NOTICE, LICENSES/ code-owned; year 2026).
