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

## Live state (end of fourth session, main at `12a4372`, nothing in flight)

Merged this session, in order: T-021 A (#25), T-007 (#23), T-016 (#24), hive-records-2 (#26), T-014 A (#27), CLAUDE.md (#29), T-018 B (#28), hive-records-3 (#30), RFC-0003 Amendment 2 Cells (#31), T-006 JSON inputs (#33), DECISIONS.md backfill D-001 to D-133 (#34), T-023 A (#32). No open PRs, no worktrees, no agents running.

DECISIONS.md now exists (`.waggle/hive/DECISIONS.md`, 133 entries, append-only, gate-checked). New decisions go through `waggle decide --file <json>` (build waggle from main; input shape is `internal/decisions/testdata/backfill/D-*.json`), not into T-006-backfill Appendix A. Next ID D-134.

Keeper rulings NOT yet recorded with waggle decide (do this in hive-records-4):
- D-134 (policy, T-023): T-023 A short fix pass before merge (`--restricted`, Write/Edit scoped to `./**`, MalformedLines fix, refuse `--effort`, `--fallback-model`, `--remote-control`, `--environment`, `--chrome`, `--ide`, bare positional).
- D-135 (technical, T-023, T-021): auto-compaction stays off in workers; long sessions end at max_tokens until phase E.
- D-136 (policy, T-018, T-016, T-021, T-023): D-089 clarification, the size metric is added non-test lines, not whole touched files.
- D-137 (policy, T-023): a `default_args` allowlist replaces the denylist as a precondition for T-023 phase C.

Uncommitted hive files for hive-records-4: reviews RFC-0003-A2-sonnet.md, RFC-0003-A2-fable.md, T-006-json.md, T-023a-sonnet.md, T-023a-fable.md and media dirs RFC-0003-A2/, T-006-json/, T-023a/, plus this handoff.

Brief edits for hive-records-4 (status lines and one-line folds only):
- T-023: phase A Done (`12a4372`, #32, 469 lines); phase C precondition: default_args allowlist (D-137); carry: settings.json copied into run record (phase C), truncated guard verdict as no_verdict (phase D, sonnet N3), push tripwire gaps for http://, git://, local paths, pushInsteadOf and pushurl (sonnet N9, fable), `go test -exec` widening (N2), record `claude --version` in result.json (N10), Windows argv limit note (N11).
- T-024: fable's builder hazards from `RFC-0003-A2-fable.md` (104-byte sun_path so short runtime dir, Seatbelt loopback allow for the forwarder, allowlist before DNS, bwrap keeps /run out and consider --disable-userns, apiKeyHelper seam, macOS keychain token materialized into the Cell home for passthrough); A2 merged (`44a5e51`); four bullets lack Amended markers (fix next RFC edit).
- T-006: Done (#33, #34); section 10 loop fixed in practice with printf (update the text); lease text should read `testdata/**`.
- RFC-0003 A1 "Approved by" still placeholder on main.

Next work, in order:
1. hive-records-4 (above), Keeper approves.
2. T-020 phase C (hive rename through waggle upgrade; CLAUDE.md already tracked), Tier 3, two guards (sonnet plus fable, D-100).
3. Parallel, disjoint leases: T-023 phase B (`internal/dispatch/`, `internal/config/local.go`), T-014 phase B (`cmd/waggle/` gate verify plus the loadClaims cap), T-021 phase B, T-025 phase A (license files, Tier 2). Serialize on `internal/scaffold/`: T-020 C, then T-012 B (with the init symlink and roster.json fold), then T-008 (also after T-014 B on internal/comb). T-018 C after T-014 B (main.go) and carries T-007 section 8 and N3, N5 to N10.
4. T-024 phases A to E dispatch after all of T-023 merges (D-123); phase F after the run.sock loop.

Habits from this session: merge with a backgrounded poll script (`gh pr ready`, `gh pr update-branch`, poll `gh pr checks`, `gh pr merge --merge --admin` with the Intent Packet); after a merge that adds files you hold untracked locally, delete identical local copies before pulling; never switch the main checkout's branch to commit (it deletes untracked-then-tracked files like CLAUDE.md), use a worktree; scan builder evidence for secrets before copying into media; Tier 3 gets sonnet plus fable guards; agents can't be resumed after /clear, so re-dispatch fresh on the same worktree.
