<!-- SPDX-License-Identifier: Apache-2.0 -->
# RFC note: staged gate modes (open, watched, guarded)

- **Status:** Proposed. Nothing here is ruled. Ticket T-060 (proposed, design). Needs a Head Keeper decision before any code ticket is cut.
- **Date:** 2026-11-18
- **Layer:** `product` (design note). No Go code changes in this PR.
- **Inputs:** `docs/gate.md`, `docs/rfc-notes/repositioning.md`, `.waggle/hive/VISION.md`.

## 1. Problem

Waggle Gate today is all or nothing. `verify-leases-and-tiers` fails a PR that leaves its lease, touches a protected path or breaks the decisions log, and a hive is only protected once someone also makes that check required on the default branch. A solo Keeper or an early project has to adopt the whole regime on day one, or not adopt it. Most will not. The cost of waiting is that the claim and merge history that later features read (Nectar and Quorum, VISION.md core items 5 and 6) never gets recorded.

Goal: adopt Waggle on day one with no friction, record claims from the first task, and later flip one switch to enforce, with no history backfill.

## 2. Proposal

One config line in `.waggle/control/config.yaml`:

```yaml
gate:
  mode: open # open | watch | guard
```

**The check always runs.** Every mode reads the same claims, runs the same lease, tier, binding and decisions checks and produces the same findings. Only the consequences of a finding change.

| Mode | Hive name | Check result | PR output | Branch protection |
| :--- | :--- | :--- | :--- | :--- |
| `open` | open hive | always passes (exit 0) | job summary lists would-be violations | check not required |
| `watch` | watched hive | always passes (exit 0) | job summary plus one PR comment, updated in place, listing each would-be violation | check not required |
| `guard` | guarded hive | fails on a finding (v0.1 behavior, exit 1) | annotations as today | check required on the default branch |

Notes:

- `open`: claims and leases are recorded. `waggle claim` still refuses what it refuses today (`.waggle/` directories, Tier 3 directories). That is a claim-time refusal, not a merge consequence, and it keeps the recorded history clean. Merge-time findings are reported and never block.
- `watch` is the dress rehearsal before a second contributor or an agent team arrives. The comment is the only difference from `open`. It names the commit, the finding and the rule, never file contents.
- `guard` is exactly the v0.1 gate. No v0.1 behavior changes in `guard`.
- Exit codes 2 (usage) and 3 (git or I/O error, broken base config) are not findings and stay unchanged in every mode. See open question 3.

### 2.1 No backfill

Claims are recorded from day one in every mode, as `refs/waggle/claims/ticket-<n>`. Flipping to `guard` changes only what the check does with findings. Old merges are not re-checked, and no history needs a backfill. The first guarded PR is checked against its own claim like any other.

One gap: work merged in `open` with no claim has no record. That is correct, since it never held a lease. `waggle gate status` shows how common that was.

### 2.2 CLI

- `waggle gate guard`: opens a PR that sets `gate.mode: guard`, then applies or updates the default-branch ruleset so it requires `verify-leases-and-tiers`, requires PRs and requires the branch to be up to date (the same three things `waggle doctor` probes). It is idempotent and never merges the PR. It prints the PR URL and the ruleset change before applying it. It needs a token that can administer the repo. Without one it opens the PR and prints the exact ruleset change for a human to apply. See open question 5 for ordering.
- `waggle gate open`: the reverse. It opens a PR that sets `gate.mode: open` (or `watch`, with `--to watch`). It changes no ruleset itself. It requires the Head Keeper's approval at the PR head, so there is no quiet unlocking (section 4). A human relaxes the ruleset after the PR merges, and `waggle doctor` flags a mismatch until then.
- `waggle gate status`: read-only. Over the last 30 days of merged PRs (`--days <n>` to change) it reports what the gate would have blocked: the number of PRs with findings, grouped by finding kind (lease, Tier 3, ticket binding, decisions log), which PRs, and which had no claim. Under `guard` it reports what was blocked. It also prints the current mode, whether the ruleset matches it, and a one-line verdict such as "0 would-be violations in 30 days: safe to guard". It posts nothing.

## 3. Defaults and migration

**New installs.** Candidates:

1. `open` always. Lowest friction, weakest by default. An adopter who never flips it has a recording tool, not a gate, and docs must not call that a guarded hive.
2. `guard` always. Matches today and what the docs promise. High friction for the solo case, and on a one-person repo the one person usually has admin and can bypass the check anyway (L6 in `docs/gate.md`).
3. `open` when the repo has one Keeper, `guard` otherwise. Needs a reliable "one Keeper" signal at init time (open question 1).

**Recommendation: option 3, decided by an explicit answer, not a guess.** Init asks one question, or takes `--mode`: "Will anyone other than you, or an agent team, push here?" No: `open`. Otherwise `guard`. When it cannot ask (non-interactive) and no `--mode` is given, it chooses `guard`: a silent loosening is worse than one extra flag. Init prints the mode it chose and the one command that changes it.

**Existing v0.1 users.** A config with no `gate.mode` key means `guard`, so an upgrade changes nothing and nothing loosens. `waggle upgrade` never writes `open` or `watch` into an existing hive. Only a Head Keeper approved PR can set them (section 4). A v0.1 build ignores an unknown `gate.mode` key and keeps enforcing, so an old pin fails safe: `open` takes effect only once the pin moves to a build that reads it.

## 4. Security

The mode is a security setting. `open` turns the gate into a recorder. Nothing may let an agent or a PR loosen it quietly.

- **Read from base.** Like `gate.protected_paths` and `gate.approvers`, `gate.mode` is read from `.waggle/control/config.yaml` at the PR base commit, never the head, so a PR cannot set its own mode (`docs/gate.md`, "Base config").
- **Path tier.** `.waggle/control/config.yaml` is under the hardcoded Tier 3 `.waggle/` prefix: a change there fails the check and only an admin bypass merges it, approval does not pass it. This RFC does not lower that for any mode.
- **Loosening needs the Head Keeper.** Lowering the mode (`guard` to `watch` or `open`, or `watch` to `open`) needs the Head Keeper's approval at exactly the head commit, which `waggle gate open` requests, plus the admin merge Tier 3 already requires. Raising the mode takes the same Tier 3 route today. Open question 2 asks whether raising should be easier.
- **Who may run the commands.** `waggle gate guard` and `open` only open PRs and, for `guard`, edit the ruleset. A ruleset edit needs repo admin, which agents should not hold (L8 and L11 in `docs/gate.md`). Without an admin token the command prints the change and does not apply it.
- **Mode and ruleset must agree.** `waggle doctor` gains a finding: mode `guard` with no required check, or `open` or `watch` with the check required. It is reported, never auto-fixed.
- **Honest limits.** `open` and `watch` do not touch L11 and L6; there is simply nothing to forge or bypass. Docs must say plainly that an open or watched hive records and reports and does not protect.
- **Comments leak nothing.** The `watch` comment names findings and paths the PR already touches, never file contents.
- **Mode is never per PR.** No label, trailer or commit message selects a mode.
- **Status is not an audit log.** It is a convenience read and must not be cited as evidence.

## 5. Naming

Plain language: **open hive**, **watched hive**, **guarded hive**. Config values are `open`, `watch`, `guard`. This lines up with the guard role and with Walt: "guarded" describes the gate's enforcement and the guard role still reviews PRs under every mode. `gate.mode` does not change the Full gate review process of a hive. No other names: no extra levels, no new mascots or bee terms.

CONTRIBUTING.md section 4 gets entries for these terms in the build PR, not this one. The scout owns that section and a change takes a Full gate.

## 6. What stays the same

- The five things of `repositioning.md` section 2 are what `guard` enforces.
- Claims, leases, binding, tiers and the decisions log work identically in all modes; only consequences differ.
- Hardcoded Tier 3 paths stay Tier 3.
- `verify-leases-and-tiers` stays the one check name, so existing rulesets and `waggle doctor` keep working.
- Exit codes 2 and 3 are the same in every mode.

## 7. Open questions

1. **New-install signal.** Explicit prompt (proposed), count of `gate.approvers`, or repo collaborators? Collaborator counts need extra API scope and mislead on open source repos.
2. **Raising the mode.** Should `open` to `guard` need less than Tier 3, so a solo Keeper can enforce without an admin merge? That needs a carve-out for one config line in the hardcoded rules, and its own Head Keeper card.
3. **Faults in `open`.** Does a broken base config (exit 3) still fail in `open`? Proposal: yes, a gate that cannot run is not a pass. Cost: a config typo blocks every PR on an open repo. The alternative is exit 0 with a loud warning.
4. **`watch` permissions.** The comment needs `pull-requests: write` for the gate job, `read` today. That is a workflow permission change, itself Tier 3. Is a job summary alone enough?
5. **Order in `waggle gate guard`.** Ruleset first, or merge of the config PR first? Each leaves a window. The proposal narrows it and does not remove it.
6. **Fork PRs.** The comment and `status` need write or read tokens that fork PRs lack. Should `status` also run from CI?
7. **Admin bypass visibility.** Can `status` see admin merges under `guard`? Depends on what the forge exposes. If not, drop it.
8. **Plans without rulesets** (L2). Fall back to printed instructions. Is that enough?
9. **Sequencing.** Before the public gate (G5) or after? It lowers the adoption wall, which argues for before, and the D-235 kill criterion counts outside repos. The Head Keeper decides.

## 8. Plan

T-060 is this design. If approved, the scout cuts code tickets, each its own PR:

1. `gate.mode` key, parsing, `open` and `guard` behavior in `verify`. Tier 3 by content, so two guards on different models.
2. `watch` summary and comment.
3. `waggle gate status`.
4. `waggle gate guard` and `waggle gate open`, with the `doctor` mismatch finding.
5. Scaffold default and prompt, docs, CONTRIBUTING terms.
