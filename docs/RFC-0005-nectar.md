<!-- SPDX-License-Identifier: Apache-2.0 -->
# RFC-0005: Nectar

> **ROADMAP ONLY. NOT IMPLEMENTED.** Nothing in this RFC exists in the code today. No command, ref, field or score described here ships in any Waggle build. It is a design proposal for the Head Keeper to rule on. The decision in "Decision requested" is not recorded.

- **Status:** Proposed. Ticket T-050. Not ruled.
- **Release target:** v0.2 (honey), after the public gate (D-366, D-287).
- **Builds on:** RFC-0003 (origin of Nectar and Quorum), `docs/gate.md`, D-232 (lease store stays claim refs), D-233 (listed approvers), D-234 (commit binds to a task), D-366 (Nectar restored as core).
- **Vocabulary (D-236):** "Nectar" is a hive and RFC term. User-facing docs written later say "reputation score".
- **Feeds:** T-051 (scorer), T-052 (status surface), T-053 (Quorum design).

## 1. Summary

VISION.md core item 5 says: reputation from merged commits that survive 30 days on `main` without a revert. This RFC makes that precise.

- **Unit of credit:** one merged task (ticket), not a commit or a line.
- **Who is credited:** the Keeper that owned the claim for the task, by `keeper_id`.
- **When:** 30 days after the task's last commit reached `main`, if no revert has landed and enough of its change is still alive.
- **Where it lives:** one small append-only receipt ref. Nectar is recomputed from `main` history plus receipts. No server.
- **Output:** one non-negative number per `keeper_id`, which Quorum (T-053) turns into a weight with `sqrt`.

Two gaps block this today: a claim ref is deleted on release, so the owner is lost, and nothing durable records the forge facts (PR, approvers) the controls need. Section 6 proposes one record change for T-051. Nothing is excluded silently: every rule that gives a unit less than full credit, or none, is reported by the scorer and listed in 8 or 11.3.

## 2. Goals and non-goals

Goals: reputation earned slowly and lost on a revert; deterministic (two clones compute the same score); works for agents and humans alike (credit goes to the claim owner); works for solo Keepers, many Keepers and fork contributors.

Non-goals: no public dashboard, hosted service, token or payment (T-052 is a CLI or derived view); no model judging quality (survival and revert are git facts); not a replacement for review (the gate still decides what merges).

## 3. Definitions

### 3.1 What is a credited merge

A **credited merge** is a change on the first-parent line of `main` that binds to a task (D-234: `T-<id>:` subject, `Waggle-Ticket:` trailer, or ticket branch). The gate requires this for every PR commit.

| Strategy | What `main` gets | How the scorer sees it |
| :--- | :--- | :--- |
| Squash | One commit, subject and trailer from the PR | That commit. |
| Merge commit | A merge commit plus the branch commits | The merge commit is the unit. Its task is the one all its non-merge commits bind to (the gate rejects mixed tasks). |
| Rebase | The branch commits replayed, no merge commit | The run of commits with one task and one `pr_number` (from the receipt) is the unit. |

Rules:

- **No task binding.** A commit on `main` with no task (an admin direct push, a bot commit) has no claim and so no owner. It is reported as `unbound`, never silently dropped.
- **Bot commits.** Credit follows the claim, not the author. A bot working under a Keeper's claim credits that Keeper. A bot with its own claim is a Keeper like any other; a repo may list it in `nectar.exclude_keepers`, and the report shows the excluded ids and their weight.
- **Hive-only merges** (records batches touching only `.waggle/hive/`) are credited when bound to a task, at the docs factor (3.3). Whether to credit them at all is open question 10.
- **Merge from base** into a branch is not a credited unit.

### 3.2 Unit of credit (proposed: per merged task)

**One unit per merged task.** A task that merges in several PRs or commits is one unit, completed when its last credited merge reaches `main`. A task that never finishes earns nothing.

Why: the claim is per task, so the claim is the natural key; it is the hardest unit to farm by splitting; it matches how the hive plans work.

| Alternative | For | Against |
| :--- | :--- | :--- |
| Per commit | Simple. | Rewards splitting. Squash vs rebase changes the score for the same work. |
| Per PR | Close to a task. | PR count depends on workflow. |
| Per changed line | Tracks size. | Rewards churn and generated code. Unstable under formatting. |

### 3.3 Weight of a unit

```
weight(task) = tier_factor * size_factor * review_factor
```

- `tier_factor`: highest Risk Tier of any path the task changed, using the gate's tier rules (`internal/comb`). Proposed: Tier 0 = 1, Tier 1 = 2, Tier 2 = 3, Tier 3 = 4.
- `size_factor`: `1 + log2(1 + min(changed_lines, 400) / 25)`, 3 decimals. Range 1 to 5.17. The cap stops size farming. `changed_lines = code_lines + docs_factor * docs_lines`. **Docs count**: `docs_factor` is proposed at 0.5 and is never 0. A docs-only task (this RFC is one) earns credit. Generated files and lockfiles are not authored work and are left out of the line count; the report lists them per unit, and open question 10 asks whether that is right.
- `review_factor`: 1, or `nectar.self_merge_factor` (0.5) for a self-merge in a repo with more than one Keeper (section 8). Never 0.
- A `light` and a `full` claim weigh the same. The gate level is a safety setting, not merit.

Constants live in one versioned table (`rules_version`, section 7). Changing a constant changes the version, never the past silently.

Nectar of a Keeper is the sum of `weight` over their units that have **matured** (section 4), minus **clawed back** weight (section 5). It is a non-negative real.

## 4. The 30-day survival window

### 4.1 Clock

The window starts at the **committer timestamp of the credited merge on `main`** (`merged_at`): the squash commit, the merge commit, or the last commit of a rebase run. It matures at `merged_at + 30 x 24 hours`.

Commit time is in git history, so any clone can read it. Committer time can be set by the committer, but for squash and merge commits on `main` the forge sets it. For a direct push, the scorer uses the later of `merged_at` and the **first-seen time**: the committer time of the receipt commit that first names the unit's `merged_sha` (6.2). That clock is the receipt writer's, so this is a weak control, not a proof; section 8 says so.

The scorer takes an explicit `--now` and never reads the wall clock (section 7).

### 4.2 What "survives" means

A unit survives if, at maturity:

1. **No revert** of any of its commits has landed on `main` (section 5).
2. **Enough of it is alive.** Of the lines the task added, at least `survive_ratio` (proposed 50%) are still on `main`, by `git blame -w -M -C`, honoring `.git-blame-ignore-revs`.

**Ordinary later work is not a revert.** Later edits lower the alive ratio, but a unit is dropped only if more than half its lines were deleted or rewritten within 30 days. A rename or move keeps its blame.

- **Planned supersede.** A `Waggle-Supersedes: T-<id>` trailer (proposed) exempts the earlier unit from rule 2. Rule 1 is unaffected.
- **Pure deletion tasks** (no added lines) use rule 1 only and weigh as a minimal unit.
- **Docs and hive lines** count in rule 2 like any added line.

### 4.3 Partial reverts

A revert that undoes only some hunks of a unit (checked by diff intersection) claws back that unit in proportion to the share of its weighted lines reverted, 3 decimals. A full revert is a full clawback.

### 4.4 Reverted and reapplied

**One rule: restored.** The revert of a revert (re-landing) restores the original unit at its **original weight**, with `merged_at` set to the **re-landing commit's committer time**. It matures 30 days after that. Until then it is pending. A **new task** that re-adds the same lines is a new unit for its own owner; the original stays clawed back unless its revert is itself reverted. A Keeper cannot revert and reapply to dodge a loss. They only restart the clock.

### 4.5 Force-pushes to main

A repo that allows force-push to `main` can rewrite history and the score.

- The scorer reads `main` by first-parent line and checks each receipt's `merged_sha` against it. `base_sha` (6.2) lets it tell a rewritten history from a missing one.
- If `merged_sha` is no longer reachable from `main`, the unit is `orphaned` and **suspended**: not paid, not clawed back, until a new receipt or a maintainer entry resolves it. Orphans are listed.
- T-051 adds a `waggle doctor` finding that warns when `main` allows force-push.

## 5. Revert detection and clawback

### 5.1 Signals, in order of trust

1. **Trailer.** `git revert` writes `This reverts commit <sha>.`
2. **Diff match.** A commit with no trailer whose diff inverts a credited commit's diff (patch-id of the reversed patch) is a manual revert.
3. **Revert of a squash** names the one commit. `git revert -m 1` names a merge commit, which is the unit. For a rebase run, each reverted commit claws back its share by line.
4. **Subject only.** `Revert "..."` without a SHA is not trusted alone. It is reported as `unconfirmed_revert` (open question 4).

A revert commit earns nothing itself: `kind: revert`, weight 0.

### 5.2 Clawback timing

- **Within the window:** the unit never matures; nothing is paid, nothing to claw back.
- **After the window:** paid credit is clawed back in full, or in proportion for a partial revert. **No time limit.** A revert at day 400 still removes the credit.
- **Floor and cap:** Nectar never goes below 0, and one unit's clawback cannot exceed its own weight.
- **Not for the reverter.** Reverting bad work is good work; the reverter is neither credited nor penalised.
- **Re-landing** follows 4.4: the unit is restored at original weight, `merged_at` = re-landing commit time, and is pending until it matures again.

## 6. Attribution and receipts

### 6.1 Where the owner comes from

`refs/waggle/claims/ticket-<n>` holds `lease.json` (`internal/comb/backend.go`, `Lease`): `ticket_id`, `keeper_id`, `role`, `gate`, `directories`, `claimed_at`, `expires_at`, `branches`. `keeper_id` is the owner, a GitHub login by convention (D-016; `internal/config/local.go`). Credit goes to the `keeper_id` of the claim whose `branches` names the merged PR's head branch and whose ticket matches the task, the same match the gate makes.

### 6.2 The gaps and the receipt ref

Today `waggle release` deletes the claim ref (`ReleaseLease`, `internal/comb/git.go`) and the commit chain is left to garbage collection. The gate records nothing about who owned a task when a PR merged, and the force-release audit ref (`refs/waggle/audit/force-releases`) holds only forced releases. The git author is not enough: a squash author can be the forge or a bot, and a maintainer can merge for a fork contributor. Forge facts (PR number, approvers) are also not in `main`.

**Proposed record change for T-051a:** an append-only **receipt ref**, `refs/waggle/receipts/main`, like the audit chain. One commit per receipt, first parent is the previous receipt head, tree holds one `receipt.json`:

```json
{
  "version": 1,
  "source": "release",
  "ticket_id": "T-050",
  "keeper_id": "alice",
  "co_keepers": [],
  "role": "builder",
  "gate": "full",
  "claimed_at": "2026-11-20T09:00:00Z",
  "branches": ["t-050-nectar-brief"],
  "base_sha": "<40 hex>",
  "merged_sha": "<40 hex>",
  "merged_at": "2026-11-20T11:30:00Z",
  "pr_number": 234,
  "pr_author": "alice",
  "head_branch": "t-050-nectar-brief",
  "approvers": ["bob"],
  "released_by": "alice"
}
```

- **`base_sha`:** the first parent on `main` of the unit's first commit. **`merged_sha`:** the credited merge (3.1; the last commit of a rebase run). **First-seen** is the receipt commit's committer time (4.1).
- **Forge fields** `pr_number`, `pr_author`, `head_branch`, `approvers` are read from the forge **once, when the receipt is written**, and never again. `approvers` is every login whose latest review is APPROVED at the merged head and not dismissed. **This is a record change:** the gate reads only *listed* approvers (`docs/gate.md`, section 6, D-233), so it does not hold this list today. The receipt writer reads all reviews through the forge adapter (`internal/forge/reviews.go`). If the forge is unreachable, the fields are left empty, the receipt is marked `incomplete`, and the scorer reports it.
- The claim's own commit chain is a second parent of the receipt, as in the audit chain, so renewals and takeovers are kept.
- Pushed with the same CAS rules as claims. The gate and `waggle doctor` already probe that a clone can push `refs/waggle/*`.

**How `merged_sha` is found.** `branchMerged` (`internal/dispatch/release.go`) detects only a merged branch tip, and this repo squash-merges, so a squash or rebase merge today needs `waggle release --unmerged`. T-051a therefore resolves `merged_sha` itself, in order:

1. **Forge:** the merge commit the forge reports for the merged PR whose head branch is in `branches` (squash, rebase and merge commit alike).
2. **Git only:** the first first-parent commit on `main` after `claimed_at` that binds to the ticket (D-234). For a rebase run, take the run to its last commit; if two candidates differ in PR, take none.
3. **Merged tip:** `branchMerged`'s tip, as today, for a plain merge.
4. **None found:** the receipt is written with `merged_sha: null`, `source: "release"`, `status: "unmerged"`. It credits nothing now.

**`--unmerged` releases.** The flag skips the merged check, not the receipt. Steps 1 and 2 still run, so a squash merge released with `--unmerged` still gets its `merged_sha`. If nothing is found (the task made no PR, or was abandoned) the receipt is `unmerged` and a later receipt for the same ticket may supersede it, newest by receipt order.

**Claims that expire or are never released.**

- **Post-merge gate run.** The gate action, on a push to `main`, writes a receipt with `source: "gate_run"` for each credited merge that has no receipt, reading the live claim ref if it still exists (`keeper_id`, `branches`). This also covers direct merges by admins. It needs the gate workflow to run on `push` to `main` and to be able to push `refs/waggle/receipts/*`.
- **Backfill.** `waggle nectar backfill` fills older tasks from claim refs that still exist, reachable claim history and `DECISIONS.md` references, marked `source: "backfill"`.

**Named remaining gaps.** (a) A claim that expired, was pruned or deleted before any release, gate run or backfill, with no `DECISIONS.md` reference, stays `unattributed` and earns nothing; the report counts it. (b) A repo where the gate never runs on `main` pushes depends on release and backfill alone. (c) Forge data missing at write time is `incomplete` and cannot be recovered offline later. (d) A history before receipts exist is covered only as far as backfill can reach.

Alternative: keep the claim ref and mark it released instead of deleting. Rejected as the proposal because it changes the claim contract the gate reads (D-232) and keeps a ref per task forever.

**Trust note.** Receipts are writable by anyone who can push `refs/waggle/*` (`docs/gate.md`, L3). The scorer treats a receipt as valid only if its `merged_sha` is on `main` and its `branches` or `head_branch` match that commit's PR, and reports a receipt written by someone other than `keeper_id` or a listed maintainer as `third_party` (shown, not rejected, since maintainers write claims for fork contributors).

### 6.3 Several Keepers on one task

- **Default:** one owner, 100%.
- **Takeover:** a takeover chains a new claim commit with a new owner. Credit follows the owner when the PR **merged**. Earlier partial work is not credited unless listed.
- **Shared credit:** `co_keepers` (written by `waggle release --co <id>`) splits the weight equally with the owner, at most 4. A co-keeper must be named by the owner. Weighted split is open question 3.
- **Guards and reviewers** earn no Nectar for review in v0.2 (open question 5).

### 6.4 Agents and humans

Credit goes to the Keeper who owns the claim, whoever typed. An agent runs for a Keeper on that Keeper's plan (VISION item 3) and has no `keeper_id` of its own.

## 7. Storage and derivation

Nectar is **derived**:

```
nectar(main history, receipts, rules_version, now) -> scores
```

- **Inputs, and only these:** first-parent history of `main`; the receipt ref, including its forge fields `pr_number`, `pr_author`, `head_branch`, `approvers`, `base_sha` and `merged_at`; `.git-blame-ignore-revs`; the repo's tier and Nectar config; `now`. **Scoring is offline.** The scorer makes no network call. Forge data enters once, when a receipt is written (6.2).
- **No central server.** A clone with `refs/waggle/receipts/*` fetched computes it alone.
- **Deterministic:** no wall clock in the scorer (`--now` is explicit; the CLI shell passes the wall clock), no map iteration order, sort by `(merged_at, sha)`. Same inputs give byte-identical JSON.
- **Cache:** `.git/waggle/nectar/<rules_version>.json`, local, never committed, never authoritative. A stale or corrupt cache is ignored. `waggle nectar --recompute` ignores it.
- **Rules versioning:** `rules_version` is in the output. A rules change is a recorded decision. The scorer can compute under an older version.
- **Nothing is committed to `main` for scores.** A published snapshot is an optional export signed by a Keeper, never a scorer input.

## 8. Gameability and prior art

Prior art (repositioning.md section 4 and the Oct 23 review): **Wasteland stamps** give reputation as signed stamps from other participants. **Gittensor** pays for merged contributions with a token on a public chain. Waggle differs: the evidence is the repo's own `main` plus gate records, there is no token or market, and credit is withdrawn on revert. Each risk has a control, and no control zeroes a unit.

| Risk | Control |
| :--- | :--- |
| **Trivial-commit farming** | Unit is a task; log size factor with a 400-line cap; a per-Keeper **daily cap** on weight that **matures** per UTC day (`nectar.daily_cap`, default 12). Excess is **carried forward**, not dropped: it matures on the next days with spare room, in `(merged_at, sha)` order. Total credit is unchanged, only delayed. `nectar.min_changed_lines` defaults to 0 (off). |
| **Self-merge** | The `approvers` list (6.2) has no login equal to `keeper_id` and the repo has more than one Keeper: weight is multiplied by `nectar.self_merge_factor` (0.5), reported. **A single-Keeper repo is the default solo case:** self-merge counts at full weight with no opt-in. "Single-Keeper" means the configured approver list has at most one login. |
| **Collusion** | Reciprocal approvals are tracked from `approvers`. When more than 60% of a pair's units are approved by each other, the pair's units weigh 50% (`nectar.reciprocity_discount`). Pairs are shown in the report, not banned. |
| **Churn** | Survival rule 2 (4.2) and unlimited late clawback (5.2). Moves and formatting do not count. |
| **Revert-and-reapply** | 4.4: restored at original weight, clock restarts at the re-landing commit; a new task re-adding the lines does not undo the clawback. |
| **Sybil** | Per `keeper_id`; a new id starts at 0. Quorum weight is `sqrt(Nectar)` (T-053), so splitting loses weight. Claims need push rights or a maintainer. |
| **Forged receipts** | 6.2 trust note: must match `main` and the PR. `third_party` shown. |
| **Time gaming** | 4.1: later of committer time and first-seen (receipt commit time). Weak, since the receipt writer sets its own clock; the main control is that `main` is first-parent and the forge sets squash time. |
| **Fork mismatch** | `explain` shows `pr_author` beside `keeper_id`; a mismatch is reported, not blocked. |
| **Keeper leaves** | Nectar stays on the id. No decay in v0.2 (open question 2). |

## 9. Interfaces

### 9.1 CLI for T-052

```
waggle nectar [--repo <dir>] [--keeper <id>] [--now <rfc3339>] [--recompute] [--json]
waggle nectar explain <task> [--json]
waggle nectar backfill
```

Plain output is a table: `keeper_id`, `nectar`, `matured`, `pending`, `clawed_back`. `explain` shows one task's weight, factors, dates, and why it was or was not credited. Exit 0 success, 1 scoring finding (orphans, unattributed), 2 usage, 3 unreadable repo. `--json > nectar.json` is enough for a static site.

### 9.2 Output shape for Quorum (T-053)

Stable under `schema_version`:

```json
{
  "schema_version": 1,
  "rules_version": 1,
  "repo": "waggle-sh/waggle",
  "main_sha": "<40 hex>",
  "computed_at": "2026-11-20T12:00:00Z",
  "keepers": [
    {"keeper_id": "alice", "nectar": 14.25, "matured": 17.5, "pending": 3.0, "clawed_back": 3.25, "units": 6}
  ],
  "unattributed": 2,
  "orphaned": 0
}
```

- `nectar = max(0, matured - clawed_back)`, a non-negative finite number, 3 decimals.
- `pending` is **never** part of `nectar`. Carried-forward daily-cap excess counts as pending.
- Quorum weight is `sqrt(nectar)`, computed by Quorum.
- Keepers are sorted by `keeper_id`; absent means 0.
- The snapshot is for one `main_sha`, which Quorum records so a ballot can be replayed.

## 10. Multi-Keeper and fork contributors

- **Many Keepers with push access:** each writes their own claims through CAS (VISION item 3); receipts name the owner.
- **Fork contributors:** they cannot write claims (`docs/gate.md`, L3). A maintainer writes the claim with a `<owner>/<repo>:<branch>` entry, with `keeper_id` the **contributor's login** and `released_by` the maintainer. Credit goes to `keeper_id`; `third_party` flags the maintainer write.
- **Relay (T-055 spike):** if relay lets a contributor claim without push access, it must record the authenticated login as `keeper_id` in the claim and receipt. This RFC does not depend on relay.
- A maintainer cannot credit work the contributor did not do: the receipt must match the PR, and `pr_author` shows beside `keeper_id`.

## 11. Phasing, tests and open questions

### 11.1 Phasing

- **T-051a:** receipt ref write in `waggle release`, the `merged_sha` resolver, the post-merge gate-run receipt, and `backfill`. It stops the loss of owners and can land before v0.2.
- **T-051b:** scorer: units, weights, window, revert detection, clawback, cache.
- **T-052:** `waggle nectar` and `explain`, JSON in 9.2.
- **Planned cut for T-051:** 051a and 051b split at the receipt ref.

### 11.2 Test plan

Fixture repos built with git in a temp dir, fixed `--now`:

- Squash, merge and rebase merges give one unit each, including `--unmerged` release of a squash.
- Expired claim with a gate-run receipt; never-released claim with backfill; unattributed gap counted.
- No trailer or bot-only gives `unbound`; docs-only and hive-only units earn credit at the docs factor.
- Day 29 vs day 30, and a clock that moves back.
- Full, partial, manual, merge-commit and late (day 400) reverts; revert then re-land (restored, new `merged_at`, original weight); force-push orphan.
- Rewrite over 50% vs a move or format change; `Waggle-Supersedes`.
- Takeover, `co_keepers`, fork contributor claim.
- Receipt not matching `main` is rejected; `third_party`; `incomplete` forge fields.
- Determinism: shuffle input and cache state, compare JSON byte for byte; scorer runs with the network disabled.
- Quorum shape: non-negative, `pending` excluded.
- Gameability: daily cap with carry-forward, self-merge (solo full, multi-Keeper halved), reciprocal pair.

### 11.3 Open questions

1. Are the constants right (50% survive, tier factors, size cap, `docs_factor` 0.5, daily cap 12, self-merge 0.5, reciprocity 60% and 50%)? Starting values, to be tuned on this repo's history before v0.2.
2. Decay and caps. Inactivity decay is not proposed. **CUT? needs Head Keeper typed reason** for any move to decay, expire or cap total Nectar. The daily cap here is a rate limit with carry-forward, so total credit is unchanged and it is not a CUT. A version that discards the excess would narrow item 5 and needs a typed reason.
3. Weighted `co_keepers` split instead of equal.
4. Who confirms an `unconfirmed_revert`: owner, maintainer, or both?
5. Should review work (guards, approvers) earn Nectar? This RFC says no for v0.2.
6. Is the receipt ref acceptable, or should the claim be tombstoned (rejected above, but it touches D-232)? A new `refs/waggle/receipts/*` namespace also needs a mention in the gate and doctor docs.
7. Self-merge: a single-Keeper repo is the default solo case at full weight, and a multi-Keeper self-merge is halved. Is the definition (at most one listed approver) right, and is 0.5 the right factor?
8. Backfill: how far back, and is `DECISIONS.md` an acceptable source?
9. Must a signed snapshot back Quorum ballots, or is the live derived value enough?
10. Docs and hive-only work counts at `docs_factor` 0.5, and generated files and lockfiles are left out of line counts. Is that right? **CUT? needs Head Keeper typed reason** if any of these were instead excluded from credit.

## 12. Decision requested

The Head Keeper is asked to rule on the design of Nectar.

**Options:**

1. **Adopt this design.** Unit is a merged task; 30-day window from the merge commit; revert clawback with no time limit; revert-of-revert restores at original weight from the re-landing time; attribution and forge facts through a new append-only receipt ref; docs and hive work count; derived offline, cached under `.git`, no server. T-051 is split at the receipt ref.
2. **Adopt with changes.** Take the design and set the open questions in 11.3 (constants, docs factor, co-keeper split, self-merge rule, receipt ref vs tombstone) in the ruling.
3. **Per-commit unit.** Simpler, but rewards splitting and differs by merge strategy.
4. **Need more evidence.** Land T-051a first and tune constants on real data before the rest is ruled. This defers no core item: T-051 and T-052 stay on the roadmap.

**Hive recommendation:** Option 1, with the receipt ref (T-051a) built first so owners are recorded from the public gate onward. The open questions are tuned on this repo's history before v0.2 ships.

**Keeper choice:** Accepted pending fix; starting values to be tuned on this repo's history before v0.2. Head Keeper decision, Paul Cleary, 2026-11-19.
