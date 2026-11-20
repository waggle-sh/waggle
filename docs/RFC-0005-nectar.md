<!-- SPDX-License-Identifier: Apache-2.0 -->
# RFC-0005: Nectar

> **ROADMAP ONLY. NOT IMPLEMENTED.** Nothing in this RFC exists in the code today. No command, ref, field or score described here ships in any Waggle build. It is a design proposal for the Head Keeper to rule on. The decision in "Decision requested" is not recorded.

- **Status:** Proposed. Ticket T-050. Not ruled.
- **Release target:** v0.2 (honey), after the public gate (D-366).
- **Builds on:** RFC-0003 (origin of Nectar and Quorum), `docs/gate.md`, D-232 (lease store stays claim refs), D-234 (commit binds to a task), D-366 (Nectar restored as core).
- **Feeds:** T-051 (scorer), T-052 (status surface), T-053 (Quorum design).

## 1. Summary

VISION.md core item 5 says: reputation from merged commits that survive 30 days on `main` without a revert. This RFC makes that precise.

- **Unit of credit:** one merged task (ticket), not a commit or a line.
- **Who is credited:** the Keeper that owned the claim for the task, by `keeper_id`.
- **When:** 30 days after the task's last commit reached `main`, if no revert has landed and enough of its change is still alive.
- **Where it lives:** nowhere new except a small append-only receipt ref. Nectar is recomputed from `main` history plus receipts. No server.
- **Output:** one non-negative number per `keeper_id`, which Quorum (T-053) turns into a weight with `sqrt`.

One gap blocks this today: a claim ref is deleted on release, so the owner is lost. Section 5 proposes a record change for T-051.

## 2. Goals and non-goals

Goals:

- Reputation that is earned slowly and lost on a revert.
- Deterministic: two clones of the same repo compute the same score.
- Works for agents and humans alike, because credit goes to the Keeper who owns the claim.
- Works for solo Keepers, many Keepers on one repo, and fork contributors.

Non-goals:

- No public dashboard, no hosted service, no token or payment (T-052 is a CLI or derived view).
- No quality judgement by a model. Survival and revert are git facts.
- Not a replacement for review. The gate still decides what merges.

## 3. Definitions

### 3.1 What is a credited merge

A **credited merge** is a change on the first-parent line of the default branch (`main`) that binds to a task (D-234: `T-<id>:` subject, `Waggle-Ticket:` trailer, or ticket branch). The gate already requires this for every PR commit, so every PR merged through the gate has one.

How each merge strategy shows up on `main`:

| Strategy | What `main` gets | How the scorer sees it |
| :--- | :--- | :--- |
| Squash | One commit, subject and trailer from the PR | That commit. Its task comes from subject or trailer. |
| Merge commit | A merge commit plus the branch commits | The merge commit is the unit. Its task is the task all its non-merge commits bind to (the gate rejects mixed tasks). |
| Rebase | The branch commits replayed, no merge commit | The run of commits with one task from one PR is the unit. Group by task id plus PR number when the forge can give it, else by consecutive commits with the same task. |

Rules:

- **No task binding.** A commit on `main` with no task (a direct push by an admin, a bot commit) earns nothing. It is not an error. Counted in the report as `unbound`.
- **Bot commits.** Commits authored by a bot account earn nothing by author. Credit follows the claim, so a bot that works under a Keeper's claim credits that Keeper. A bot with its own claim (for example a dependency updater) is a Keeper like any other and may be listed in `nectar.exclude_keepers` (section 8).
- **Hive-only merges.** A merge that touches only `.waggle/hive/` (records batches) earns nothing. These are bookkeeping, not product work.
- **Merge from base.** A merge of `main` into a branch is not a credited unit (the gate already takes the first parent's task for it).

### 3.2 Unit of credit (proposed: per merged task)

**Proposal: one unit per merged task.** A task that merges in several PRs or several commits is one unit, completed when its last credited merge reaches `main`. A task that never finishes earns nothing.

Why:

- The claim is per task, and attribution comes from the claim. The claim is the natural key.
- It is the hardest unit to farm by splitting. Ten tiny commits for one task are still one unit.
- It matches how the hive already plans work.

Alternatives, rejected as the base unit:

| Alternative | For | Against |
| :--- | :--- | :--- |
| Per commit | Simple. | Rewards splitting. Squash vs rebase changes the score for the same work. |
| Per PR | Close to a task. | PR count depends on workflow. Several PRs per task would be several units. |
| Per changed line | Tracks size. | Rewards churn and generated code. Unstable under formatting. |

Size still counts, but with a cap (3.3).

### 3.3 Weight of a unit

```
weight(task) = tier_factor * size_factor
```

- `tier_factor`: from the highest Risk Tier of any path the task changed, using the gate's own tier rules (`internal/comb`). Proposed: Tier 0 = 1, Tier 1 = 2, Tier 2 = 3, Tier 3 = 4. Hard paths earn more because they carry more review.
- `size_factor`: `1 + log2(1 + min(changed_lines, 400) / 25)`, rounded to 3 decimals, with `changed_lines` counted over non-generated, non-lockfile files. Range 1 to 5.17. The cap stops size farming.
- A `light` claim and a `full` claim weigh the same. The gate level is a safety setting, not merit.

All constants live in one table in the scorer and are versioned (`rules_version`, section 9). Changing a constant changes the version, never the past silently.

Nectar of a Keeper is the sum of `weight` over their units that have **matured** (section 4) and were not **clawed back** (section 5). It is a non-negative real number.

## 4. The 30-day survival window

### 4.1 Clock

The window starts at the **committer timestamp of the credited merge on `main`**: for a squash, the squash commit; for a merge commit, the merge commit; for a rebase run, the last commit of the run. The window length is 30 x 24 hours. Matures at `merged_at + 30d`.

Why the commit time, not the push or PR time: it is in git history, so any clone can read it, and it needs no forge call. Committer time can be set by the committer, but on `main` it is set by the forge for squash and merge commits. For a direct push by a Keeper with push rights, the scorer uses the later of committer time and the time the commit was first seen in the receipt ref (4.5).

The scorer takes an explicit `--now`. It never reads the wall clock inside the scoring function (section 7).

### 4.2 What "survives" means

A unit survives if both hold at maturity:

1. **No revert** of any of its commits has landed on `main` (section 5).
2. **Enough of it is alive.** Over the lines the task added, at least `survive_ratio` (proposed 50%) are still present on `main` at maturity, measured with `git blame` ignoring whitespace (`-w`) and moved lines (`-M -C`), and ignoring revs listed in `.git-blame-ignore-revs`.

Rule 2 is the answer to "is refactoring a revert?". **Ordinary later work is not a revert.** Edits by later tasks lower the alive ratio but do not claw back. The alive ratio is only a floor: a unit is dropped only if more than half of its lines were deleted or rewritten within 30 days. That is a signal that the work did not last. A file renamed or moved keeps its blame, so a move does not count against it.

Special cases:

- **Deleted by a planned task.** If a later task's body or the claim says it supersedes this one (`Waggle-Supersedes: T-<id>` trailer, proposed), the earlier unit is exempt from rule 2. This is how a deliberate rewrite avoids punishing the first author. Rule 1 is not affected.
- **Pure deletion tasks** (no added lines) use rule 1 only, and always weigh as a minimal unit.
- **Docs-only and generated files** are excluded from the line count (3.3) and from rule 2.

### 4.3 Partial reverts

A revert commit that undoes only some hunks of a unit (checked by diff intersection with the original) claws back that unit **in proportion** to the share of the unit's weighted lines reverted, rounded to 3 decimals. Revert of 100% of lines is a full clawback. This keeps a small fix-up revert from erasing a large unit.

### 4.4 Reverted and reapplied

A revert followed by a reapply (a revert of the revert, or a new task that re-adds the same change):

- The **revert of the revert** restores the original unit and its original `merged_at`. The window restarts from the re-landing commit. Credit is paid at the new maturity.
- A **new task** that re-adds the same lines is a new unit for its own owner. The original stays clawed back.

So a Keeper cannot revert themselves and reapply to reset or dodge anything. They only restart the clock.

### 4.5 Force-pushes to main

A repo that allows force-push to `main` can rewrite history and the score. Design controls:

- The scorer reads `main` by the first-parent line from the **receipt ref** (section 5.2), which stores the merged SHA and the base SHA at gate time.
- If a receipt's merged SHA is no longer reachable from `main`, the unit is marked `orphaned` and is **suspended**, not paid, not clawed back, until a new receipt or a maintainer entry resolves it. The report lists orphans.
- `waggle doctor` already checks for a required gate. T-051 adds a finding that warns when `main` allows force-push, because that makes Nectar unreliable.

## 5. Revert detection and clawback

### 5.1 Signals, in order of trust

1. **Trailer.** `git revert` writes `This reverts commit <sha>.` in the body. This is the primary signal.
2. **Diff match.** A commit that has no trailer but whose diff exactly inverts a credited commit's diff (patch-id of the reversed patch matches) is a manual revert.
3. **Revert of a squash.** A squash is one commit, so the trailer names it directly. When the original was a merge commit, `git revert -m 1` names the merge commit, which is the unit. When the original was a rebase run, each commit reverted claws back its share by line.
4. **Subject only.** `Revert "..."` without a SHA is not trusted alone. It is reported as `unconfirmed_revert` for a Keeper to confirm in a records batch (section 11, question 4).

A revert commit itself earns nothing. It binds to a task (the gate requires it) but is marked `kind: revert` and has weight 0.

### 5.2 Clawback timing

- **Within the window:** the unit never matures. No credit is paid. Nothing to claw back.
- **After the window (late revert):** credit that was paid is clawed back in full, or in proportion for a partial revert. **Clawback has no time limit.** A revert at day 400 still removes the credit. Reputation from code that later turns out wrong should not stay.
- **Floor:** Nectar never goes below 0.
- **Cap on late loss:** the clawback of one unit cannot exceed that unit's own weight. A revert cannot punish a Keeper beyond what the unit earned.
- **Not for the reverter.** The Keeper who reverts is not credited and not penalised for the revert itself. Reverting bad work is good work.

A revert of a clawed-back unit's revert (re-landing) restores the unit after a new window (4.4).

## 6. Attribution

### 6.1 Where the owner comes from

The claim for `ticket-<n>` at `refs/waggle/claims/ticket-<n>` holds `lease.json` (`internal/comb/backend.go`, `Lease`): `ticket_id`, `keeper_id`, `role`, `gate`, `directories`, `claimed_at`, `expires_at`, `branches`. `keeper_id` is the owner, a GitHub login by convention (D-016; pattern `^[A-Za-z0-9][A-Za-z0-9._@+-]{0,127}$`, `internal/config/local.go`).

Credit goes to `keeper_id` of the claim whose `branches` list names the merged PR's head branch and whose ticket matches the task. That is the same match the gate already makes (`docs/gate.md`, section on claims and branches).

### 6.2 The gap: the owner is lost on release

Today `waggle release` deletes the claim ref (`ReleaseLease`, `internal/comb/git.go`). The claim commit chain is left unreferenced and git will garbage-collect it. The gate writes no record of who owned the claim when a PR merged. The decisions log records decisions, not merges. The force-release audit ref (`refs/waggle/audit/force-releases`) records only forced releases.

So after a normal release, **nothing durable says who owned task T-n.** The git author of the merged commit is not enough: the squash author can be the forge or a bot, an agent can run under a human's git identity, and a maintainer can merge for a fork contributor.

**Needed record change for T-051 (proposed):** an append-only **receipt ref**, `refs/waggle/receipts/main`, in the style of the force-release audit chain.

- Written by `waggle release` (which already checks "merged") before it deletes the claim. One commit per release, first parent is the previous receipt head.
- Tree holds one file, `receipt.json`:

```json
{
  "version": 1,
  "ticket_id": "T-050",
  "keeper_id": "alice",
  "co_keepers": [],
  "role": "builder",
  "gate": "full",
  "claimed_at": "2026-11-20T09:00:00Z",
  "branches": ["t-050-nectar-brief"],
  "merged_sha": "<40 hex>",
  "merged_at": "2026-11-20T11:30:00Z",
  "released_by": "alice"
}
```

- Pushed with the same CAS rules as claims. The gate and `waggle doctor` already probe that a clone can push `refs/waggle/*`.
- The claim's own commit chain is reachable from the receipt as a second parent, as the force-release audit does, so the full claim history (renewals, takeovers) is kept.
- A **backfill** command reads reachable claim history and `DECISIONS.md` references to fill older tasks, marked `source: backfill`. Unresolvable tasks stay `unattributed` and earn nothing.

Alternative: keep the claim ref and mark it released instead of deleting. Rejected as the proposal because it changes the claim contract the gate reads (D-232 keeps the store as is) and keeps a ref per task forever. A separate receipt ref leaves claims untouched.

Trust note: like claims, receipts are writable by anyone who can push `refs/waggle/*` (`docs/gate.md`, L3). The scorer therefore treats a receipt as valid only if its `merged_sha` is on `main` and its `branches` match that commit's PR head, and it reports receipts written by a Keeper other than `keeper_id` or a listed maintainer as `third_party` (shown, not rejected, because maintainers write claims for fork contributors).

### 6.3 Several Keepers on one task

- **Default:** the claim has one owner. That owner gets 100%.
- **Takeover:** a takeover chains a new claim commit with a new owner. Credit follows the owner at the time the PR **merged**, not at the start. Partial work by the first owner is not credited unless it is listed.
- **Shared credit:** `co_keepers` (proposed, written by `waggle release --co <id>`) splits the weight equally between `keeper_id` and listed co-keepers, at most 4. A co-keeper must have been named by the owner, so nobody can add themselves. Equal split is a deliberate simple rule; a weighted split is open question 3.
- **Guards and reviewers** earn no Nectar for review in v0.2. See open question 5.

### 6.4 Agents and humans

Credit goes to the Keeper who owns the claim, whether a human or an agent did the typing. An agent runs for a Keeper, on that Keeper's plan (VISION item 3). It has no `keeper_id` of its own. A Keeper's Nectar is the Keeper's reputation for the work they put their name to and answer for.

## 7. Storage and derivation

Nectar is **derived**. It is a function:

```
nectar(main history, receipts, rules_version, now) -> scores
```

- **Inputs:** the first-parent history of `main`, the receipt ref (6.2), `.git-blame-ignore-revs`, the repo's tier config, and `now`.
- **No central server.** A clone with `refs/waggle/receipts/*` fetched computes it alone.
- **Deterministic:** no wall clock in the scorer (`--now` defaults to the newest commit time on `main` in tests and the wall clock only in the CLI shell), no network, no map iteration order, sorting by `(merged_at, sha)`. Same inputs give byte-identical JSON.
- **Cache:** `.git/waggle/nectar/<rules_version>.json` (local, never committed, never authoritative). It stores the last scored `main` SHA and per-task state so a rescore reads only new history. Delete it and the result is the same. A stale or corrupt cache is ignored.
- **Recompute:** `waggle nectar --recompute` ignores the cache.
- **Rules versioning:** `rules_version` is part of the output. A rules change is a recorded decision. The scorer can compute under an older version to explain a past number.
- **Nothing is committed to `main` for scores.** A published snapshot (for Quorum or a README badge) is an explicit optional export, signed by a Keeper, never an input to the scorer.

## 8. Gameability and prior art

Prior art (repositioning.md section 4 and the Oct 23 review): **Wasteland stamps** give reputation as signed stamps from other participants. **Gittensor** pays for merged contributions with a token on a public chain. Both overlap with Nectar in the idea of reputation from accepted work. Waggle differs in three ways: the evidence is the repo's own `main` plus gate records, there is no token or market, and credit is withdrawn on revert. That overlap is a reason to be careful, not a reason to drop it. Each risk below has a control.

| Risk | Control |
| :--- | :--- |
| **Trivial-commit farming** (many tiny tasks) | Unit is a task, weight has a log size factor with a 400-line cap. Tasks under `nectar.min_changed_lines` (default 5) weigh 0. A per-Keeper **daily cap** on weight (`nectar.daily_cap`, default 12) limits burst farming. |
| **Self-merge** (a Keeper approves and merges their own work) | A unit counts only if the merge had an approval from a login that is not `keeper_id` (the gate already reads approvals and approvers). Solo repos with one Keeper may opt in with `nectar.allow_self_merge: true`, and then weights are halved. That is stated in the report. |
| **Collusion** (two Keepers approve each other's trivial work) | Reciprocal approvals are tracked. When more than 60% of a Keeper pair's units are approved by each other, the units from the pair weigh 50% (`nectar.reciprocity_discount`). The Head Keeper can inspect pairs in the report. A pair is not banned. |
| **Churn** (add code, rewrite it, repeat) | Survival rule 2 (4.2) drops units whose lines do not last, and a late revert claws back with no time limit (5.2). Moves and formatting do not count. |
| **Revert-and-reapply** to reset a clock or hide a loss | 4.4: the clock restarts and the original stays clawed back. |
| **Sybil** (many `keeper_id`s) | Nectar is per `keeper_id`, and a new id starts at 0. Weight for Quorum is `sqrt(Nectar)` (T-053), so splitting Nectar across ids loses weight. Claims need a person with push rights (or a maintainer, for forks), so ids are not free. |
| **Self-written receipts** (forging history) | 6.2 trust note: receipt must match `main` and the PR head. `third_party` shown. |
| **Time gaming** (backdated commit time) | 4.1: scorer uses the later of committer time and first-seen time for direct pushes. |
| **Keeper leaves, repo lives on** | Nectar stays attached to the id. It does not decay in v0.2. See open question 2. |

## 9. Interfaces

### 9.1 CLI for T-052

```
waggle nectar [--repo <dir>] [--keeper <id>] [--now <rfc3339>] [--recompute] [--json]
waggle nectar explain <task> [--json]
```

Plain output is a table: `keeper_id`, `nectar`, `matured`, `pending`, `clawed_back`. `explain` shows one task's weight, dates and why it was or was not credited. Exit 0 on success, 1 on a scoring finding (orphans), 2 usage, 3 unreadable repo. No hosted view is required. A static export (`--json > nectar.json`) is enough for a site.

### 9.2 Output shape for Quorum (T-053)

T-053 may rely on this shape, stable under `schema_version`:

```json
{
  "schema_version": 1,
  "rules_version": 1,
  "repo": "waggle-sh/waggle",
  "main_sha": "<40 hex>",
  "computed_at": "2026-11-20T12:00:00Z",
  "keepers": [
    {
      "keeper_id": "alice",
      "nectar": 14.25,
      "matured": 17.5,
      "pending": 3.0,
      "clawed_back": 3.25,
      "units": 6
    }
  ],
  "unattributed": 2,
  "orphaned": 0
}
```

Guarantees:

- `nectar` is a non-negative finite number, `nectar = max(0, matured - clawed_back)`, rounded to 3 decimals.
- `pending` (inside the window) is **never** part of `nectar`.
- Quorum weight is `sqrt(nectar)`. Quorum computes it. Nectar does not.
- Keepers are sorted by `keeper_id`. A Keeper with no matured unit may be absent, and absent means 0.
- The snapshot is for one `main_sha`. Quorum must record the `main_sha` it used so a ballot can be replayed.

## 10. Multi-Keeper and fork contributors

- **Many Keepers with push access:** each writes their own claims through CAS (VISION item 3). Receipts name the owner. Nothing else is needed.
- **Fork contributors without push access:** they cannot write claims (`docs/gate.md`, L3). Today a maintainer writes the claim with a `<owner>/<repo>:<branch>` entry. The claim's `keeper_id` is then the **fork contributor's login**, written by the maintainer, and `released_by` is the maintainer. Credit goes to `keeper_id`, so the contributor earns Nectar, and `third_party` flags that a maintainer wrote it.
- **Relay mode (T-055 spike):** if relay lets a contributor claim without push access, the relay must record the authenticated contributor login as `keeper_id` in the claim and receipt. This RFC asks T-055 to keep that field and to write receipts. It does not depend on relay to ship, because the maintainer path works now.
- A maintainer cannot give a contributor credit for work the contributor did not do: the receipt must match the PR head, and the PR author (from the forge) is shown beside `keeper_id` in `explain`. A mismatch is reported, not blocked.

## 11. Phasing, tests and open questions

### 11.1 Phasing

- **T-051a:** receipt ref write in `waggle release` plus `backfill`. Smallest slice, and the one that stops the loss of owners. Can land before v0.2 so data accrues from the public gate on.
- **T-051b:** scorer: units, weights, window, revert detection, clawback, cache.
- **T-052:** `waggle nectar` and `explain`, JSON in 9.2.
- Planned cut for T-051: 051a and 051b split at the receipt ref, as the budget rule asks.

### 11.2 Test plan

Fixture repos built with git in a temp dir, fixed `--now`:

- Squash, merge and rebase merges give one unit each.
- No trailer, bot-only, and hive-only merges give zero credit and count as `unbound`.
- Day 29 vs day 30 boundary, and a clock that moves back.
- Full, partial, manual, merge-commit and late (day 400) reverts.
- Revert then re-apply; force-push orphan.
- Rewrite over 50% vs a move or format change; `Waggle-Supersedes` exemption.
- Takeover owner, `co_keepers` split, fork contributor claim.
- Receipt that does not match `main` is rejected; `third_party` flagged.
- Determinism: shuffle input order and cache state, compare JSON byte for byte.
- Quorum shape: non-negative, `pending` excluded, schema fields present.
- Gameability fixtures: farming under the daily cap, self-merge, reciprocal pair.

### 11.3 Open questions

1. Are the proposed constants right (50% survive, tier factors, size cap, daily cap 12, reciprocity 60% and 50%)? They are starting values, to be tuned on this repo's own history before v0.2.
2. Should Nectar decay when a Keeper is inactive? Proposed: no in v0.2. **CUT? needs Head Keeper typed reason** applies to any later move to cap or expire Nectar, since that narrows item 5.
3. Weighted `co_keepers` split instead of equal.
4. Who confirms an `unconfirmed_revert`: the owner, a maintainer, or both?
5. Should review work (guards, approvers) earn Nectar? This RFC says no for v0.2.
6. Is the receipt ref acceptable, or should the claim be tombstoned instead (rejected above, but it touches D-232)?
7. Self-merge on solo repos: halved weight, or full weight when the repo has one Keeper?
8. Backfill: how far back, and is `DECISIONS.md` an acceptable source?
9. Should a published, signed snapshot be required for Quorum ballots, or is the live derived value enough?

## 12. Decision requested

The Head Keeper is asked to rule on the design of Nectar.

**Options:**

1. **Adopt this design.** Unit is a merged task; 30-day window from the merge commit; revert clawback with no time limit; attribution through a new append-only receipt ref written at release; derived, cached under `.git`, no server. T-051 is split at the receipt ref.
2. **Adopt with changes.** Take the design, and set the open questions in 11.3 (constants, co-keeper split, self-merge rule, receipt ref vs tombstone) in the ruling.
3. **Per-commit unit.** Credit each merged commit. Simpler, but rewards splitting work and differs by merge strategy.
4. **Need more evidence.** Ask T-051a to land first and tune constants on real data before the rest is ruled. This defers no core item: it keeps T-051 and T-052 on the roadmap.

**Hive recommendation:** Option 1, with the receipt ref (T-051a) built first so owners are recorded from the public gate onward. The open questions are tuned on this repo's history before v0.2 ships.

**Keeper choice:** Not decided.
