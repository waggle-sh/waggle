<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: hive-records-6 (PR #41, head d61d021)

Risk-Tier: 3

## Intent Summary

Hive-layer PR, base `main` at `6a4886e`. It runs the T-012 section 7.2 hive follow-up: commits the `levels` block in our seeded `config.yaml` first (`ee61be7`), then `waggle upgrade --apply` from a build of `6a4886e` (`dbc3938`) moves the pin to `6a4886eb16e0de1d005a4ba354e72ecd1dd63756`, re-renders `roster.json` (adds `min_level` on all five roles, drops `mode`) and keeps main's `waggle-gate.yml` per D-143. It also adds `internal/dispatch/` to the WAGGLE.md module list (D-148), appends D-146 to D-153, adds the T-012 B and C and T-020c guard reviews and media, the `media/T-012/` upgrade plan and apply output, the T-012 brief phase C text, and refreshes the queen handoff. Only `.waggle/**` changes (17 files, 915 insertions, 39 deletions). Blast radius: every future worker launch in our hive, because the regenerated (gitignored) shims now carry `model:` lines: queen, scout and guard opus, builder sonnet, forager haiku. Nothing ships.

## Checks

1. Upgrade correctness. Built `waggle` from `6a4886e` in a separate worktree. `waggle upgrade` (dry run) on the branch checkout: `[pin] ... (unchanged)`, `[same]` for `roster.json`, `devcontainer.json`, all five role files, the `.gitignore` and `CLAUDE.md` blocks; `[skip]` for the seeded files; `[rewrite]` only for `.github/workflows/waggle-gate.yml`, as expected under D-143. `waggle-gate.yml` on the branch is byte-identical to `origin/main`'s (`git diff --quiet`). Pin is `6a4886eb16e0de1d005a4ba354e72ecd1dd63756`. The `levels` block is `frontier: opus`, `standard: sonnet`, `fast: haiku` under `runtimes.adapters.claude-code`, matching section 3.2, no `alternates`. `--apply` in a throwaway worktree exited 0, changed only the gate workflow in git, and wrote shims with `model:` lines: builder `sonnet`, guard `opus` (different), queen and scout `opus`, forager `haiku`. The queen's `hr6` worktree holds the same five `model:` lines and a clean tree.
2. Step order matches section 7.2 as reordered by D-150: the levels commit `ee61be7` precedes the upgrade commit `dbc3938`, which touches only `config.yaml` (pin line) and `roster.json`. `merge_strategy` is already `up_to_date`.
3. Paths and subjects: every changed path is under `.waggle/`; no product files, no `.github/`. All five commit subjects start `Head Keeper: `.
4. DECISIONS.md: pure append after D-145, no removed lines. `waggle gate decisions --base origin/main --head HEAD` (pin build) returns `ok, appended D-146 ... D-153`. 153 `## D-` headings, matching the handoff's count and next ID D-154. Each entry has Date, Kind, Tickets, Supersedes, Question, Options, recommendation, Keeper choice and Rationale. Content matches the PR body and handoff: D-146 (builder wrote the 7.1 commit), D-147 (own honey after T-023 D), D-148 (WAGGLE.md now, matches `1942d57`), D-149 (T-022 B parallel with T-023 B), D-150 (phase C and 7.2 reorder), D-151 (symlinked CLAUDE.md refused; follow-up present in T-012 section 9), D-152 (queen runs 7.2), D-153 (pin move, with model assignments matching the shims observed in check 1).
5. Records: all six reviews and the three `media/T-020c/` files are byte-identical (`cmp`) to the main checkout. `media/T-012/upgrade-plan.txt` and `upgrade-apply.txt` exist only on the branch (generated there); the plan shows `30639558... -> 6a4886eb...` with `[rewrite]` on roster and gate, consistent with the post-apply `[same]` I observed.
6. Secrets: no `ghp_`, `github_pat_`, `sk-`, AKIA, `re_`, JWT or private key material in added lines. One match on the word "secrets" is handoff prose.
7. Personal data and home paths in added lines: `/Users/` path and `cleary.paul@protonmail.com` in `reviews/T-020c-sonnet.md`; scratchpad `/private/tmp/claude-501/...` paths in both T-020c reviews and `media/T-020c/pr-body.md`; `t@e.com` test fixture address in the same three files. All are covered by the handoff's Open notes (T-020c reviews, `pr-body.md`, protonmail in the T-020c reviews). Nothing new beyond that list.
8. Dashes: no U+2013 or U+2014 in any added line.
9. Handoff: commit hashes, PR numbers (#39, #40), merge commits `6a2401b` and `6a4886e`, branch and `hr6` worktree (present, at `d61d021`), entry count and next ID all match git state.

## Findings (non-blocking)

- N1 (handoff staleness): the edited "Habits that worked" bullet still reads "review on sonnet (guard)", while the new Live state says the guard is opus by default and must not be set to sonnet. The older "State of the repo (main at `6b84500`)" and its "Next work" list are also stale, though untouched by this PR. Fix in the next handoff refresh; not worth a respin.
- N2 (brief wording): T-012 section 7.2 says the guard records the command output in `media/T-012/`; here the queen recorded it, and this review verified it independently. No effect on correctness.
- N3 (tickets): the upgrade's generated decide input lists only T-016; D-153 lists T-012 and T-016. A reasonable editorial widening, noted for traceability.

No finding needs a Keeper ruling.

## Intent Packet

- What: T-012 section 7.2 hive follow-up (levels block, pin to `6a4886e`, roster `min_level`, shims with `model:` lines, gate workflow kept at main's copy), WAGGLE.md `internal/dispatch/`, D-146 to D-153, batch 6 reviews and media, handoff refresh. Hive only.
- Why: makes our hive run the build that enforces model levels and the guard's different-model rule; records the session's Keeper rulings.
- Risk: Tier 3 (`.waggle/control/**`), Head Keeper merge with `--admin` (leases and tiers fail by design, D-141). Dry run is clean apart from the D-143 gate rewrite; after merge, copy the shims into the main checkout and restart the session.

Verdict: PASS
