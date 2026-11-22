<!-- SPDX-License-Identifier: Apache-2.0 -->
# RFC note: staged gate modes (open, watched, guarded)

- **Status:** Proposed. Nothing here is ruled. Ticket T-060 (proposed, design). Needs a Head Keeper decision before any code ticket is cut.
- **Date:** 2026-11-18
- **Layer:** `product` (design note). No Go code changes in this PR.
- **Inputs:** `docs/gate.md`, `docs/rfc-notes/repositioning.md`, `.waggle/hive/VISION.md`.

## 1. Problem

Waggle Gate today is all or nothing. `verify-leases-and-tiers` fails a PR that leaves its lease, touches a protected path or breaks the decisions log, and a hive is only protected once someone also makes that check required on the default branch (`docs/gate.md:133`). A solo Keeper or an early project has to adopt the whole regime on day one, or not adopt it. Most will not. The cost of waiting is that the claim and merge history that later features read (Nectar and Quorum, VISION.md core items 5 and 6) never gets recorded.

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

- `open`: claims and leases are recorded. `waggle claim` still refuses what it refuses today (`.waggle/` directories, Tier 3 directories; `internal/comb/backend.go:458-462`). That is a claim-time refusal, not a merge consequence, and it keeps the recorded history clean. Merge-time findings are reported and never block.
- `watch` is the dress rehearsal before a second contributor or an agent team arrives. The comment is the only difference from `open`. It names the commit, the finding and the rule, never file contents.
- `guard` is exactly the v0.1 gate. No v0.1 behavior changes in `guard`.
- Exit codes 2 (usage) and 3 (git or I/O error, broken base config) are not findings and stay unchanged in every mode (`cmd/waggle/gate.go:25-30`). Exit 3 is also what an old gate build returns on a base config that carries `gate.mode` (section 3), so a config typo and an old pin both block every PR. See open question 3.

### 2.1 No backfill

Claims are recorded from day one in every mode, as `refs/waggle/claims/ticket-<n>`. Flipping to `guard` changes only what the check does with findings. Old merges are not re-checked, and no history needs a backfill. The first guarded PR is checked against its own claim like any other.

One gap: work merged in `open` with no claim has no record. That is correct, since it never held a lease. `waggle gate status` shows how common that was. It tells "never claimed" apart from "claim deleted after merge" by the merged commits: a commit that carries a `Waggle-Ticket` trailer for a ticket whose claim ref is gone was claimed and released, and a commit with no ticket binding was never claimed. This is a heuristic, which is one more reason status is not evidence (section 4).

### 2.2 CLI

- `waggle gate guard`: opens a PR that sets `gate.mode: guard`, then applies or updates the default-branch ruleset so it requires `verify-leases-and-tiers` and requires the branch to be up to date (two of the three things `waggle doctor` probes: `required-gate` and `up-to-date`; the third is `claim-refs`; `docs/gate.md:133`, `cmd/waggle/doctor.go:74-89`). The same ruleset change also requires PRs. `waggle doctor` does not check that PRs are required (`docs/gate.md:133`), so this RFC does not claim it does; whether `waggle gate guard` should probe for it is part of the build tickets. It is idempotent and never merges the PR. It prints the PR URL and the ruleset change before applying it. It needs a token that can administer the repo. Without one it opens the PR and prints the exact ruleset change for a human to apply. See open question 5 for ordering.
- `waggle gate open`: the reverse. It opens a PR that sets `gate.mode: open` (or `watch`, with `--to watch`). It changes no ruleset itself. It requires the Head Keeper's approval at the PR head, so there is no quiet unlocking (section 4). A human relaxes the ruleset after the PR merges, and `waggle doctor` flags a mismatch until then.
- `waggle gate status`: read-only. Over the last 30 days of merged PRs (`--days <n>` to change) it reports what the gate would have blocked: the number of PRs with findings, grouped by finding kind (lease, Tier 3, ticket binding, decisions log), which PRs, and which had no claim. Under `guard` it reports what was blocked. It also prints the current mode, whether the ruleset matches it, and a one-line verdict such as "0 would-be violations in 30 days: safe to guard". It posts nothing.

## 3. Defaults and migration

**New installs.** Candidates:

1. `open` always. **CUT? needs Head Keeper typed reason.** Lowest friction, weakest by default. An adopter who never flips it has a recording tool, not a gate, and docs must not call that a guarded hive.
2. `guard` always, with `open` and `watch` as opt-in choices. Nothing is cut. Matches today and what the docs promise. High friction for the solo case, and on a one-person repo the one person usually has admin and can bypass the check anyway (L6 in `docs/gate.md`).
3. `open` when the repo has one Keeper, `guard` otherwise. **CUT? needs Head Keeper typed reason.** Needs a reliable "one Keeper" signal at init time (open question 1).

**Recommendation: see section 10.** Options 1 and 3 make `open` the default for some or all new installs and narrow VISION core items 1 and 2, so they need a Head Keeper typed reason (section 5). Without that reason the default is `guard`, and `open` and `watch` are opt-in, chosen by an explicit answer, not a guess. Init takes `--mode open|watch|guard` and, when asked to, one question: "Will anyone other than you, or an agent team, push here?" With `guard` as the default, init chooses `guard` unless the Keeper passes `--mode` or answers that the repo is solo and picks a looser mode. A silent loosening is worse than one extra flag. Init prints the mode it chose and the one command that changes it.

**Existing v0.1 users.** A config with no `gate.mode` key means `guard`, so an upgrade changes nothing and nothing loosens. `waggle upgrade` never writes `open` or `watch` into an existing hive. Only a Head Keeper approved PR can set them (section 4). A v0.1 build does not ignore an unknown `gate.mode` key. Any unknown key under `gate` is `unknown key` (`internal/config/config.go:217-218`), and a broken base config exits 3 on every PR (`cmd/waggle/gate_verify.go:265-303`, `TestBaseConfigFaultsExit3` at `cmd/waggle/gate_bind_test.go:255-263`). An old pin therefore fails closed by blocking every PR, not by enforcing. Rollout order: the gate pin (`gate/`, the workflow's base-tree build, `docs/gate.md:106`) must move to a build that parses `gate.mode` before any hive writes `gate.mode`. `waggle upgrade` moves the pin first and the first code ticket ships the parser before any command or scaffold writes the key. A hive on an old pin must not hand-edit `gate.mode` in.

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

## 5. VISION core items

`.waggle/hive/VISION.md` lists core item 1, "Claims and path leases ... Optimistic at claim time, authoritative at merge", and core item 2, "Merge gate. Required check that each commit stays inside its task's lease ...". Its rule 1 says an option that narrows a core item must be marked **CUT** with the item quoted, and rule 2 says a CUT lands only on a typed Head Keeper reason.

- `open` and `watch` make the check always pass and not required (section 2 table). That is not authoritative at merge and not a required check.
- As an opt-in mode with `guard` as the default, nothing is narrowed: every hive that does not choose otherwise keeps both items. This is the recommended shape.
- As the default for new installs (section 3, options 1 and 3), both items are narrowed for new hives. Those options carry the CUT marker.

## 6. Naming

Plain language: **open hive**, **watched hive**, **guarded hive**. Config values are `open`, `watch`, `guard`. This lines up with the guard role: "guarded" describes the gate's enforcement and the guard role still reviews PRs under every mode. `gate.mode` does not change the Full gate review process of a hive. No other names: no extra levels, no new mascots or bee terms.

CONTRIBUTING.md section 4 gets entries for these terms in the build PR, not this one. The scout owns that section and a change takes a Full gate.

## 7. What stays the same

- The five things of `repositioning.md` section 2 are what `guard` enforces.
- Claims, leases, binding, tiers and the decisions log work identically in all modes; only consequences differ.
- Hardcoded Tier 3 paths stay Tier 3.
- `verify-leases-and-tiers` stays the one check name, so existing rulesets and `waggle doctor` keep working.
- Exit codes 2 and 3 are the same in every mode.

## 8. Open questions

1. **New-install signal.** Explicit prompt (proposed), count of `gate.approvers`, or repo collaborators? Collaborator counts need extra API scope and mislead on open source repos.
2. **Raising the mode.** Should `open` to `guard` need less than Tier 3, so a solo Keeper can enforce without an admin merge? That needs a carve-out for one config line in the hardcoded rules, and its own Head Keeper card.
3. **Faults in `open`.** Does a broken base config (exit 3) still fail in `open`? Proposal: yes, a gate that cannot run is not a pass. Cost: a config typo, or an old pin that meets `gate.mode` (section 3), blocks every PR on an open repo. The alternative is exit 0 with a loud warning.
4. **`watch` permissions.** The comment needs `pull-requests: write` for the gate job, `read` today. That is a workflow permission change, itself Tier 3. Is a job summary alone enough?
5. **Order in `waggle gate guard`.** Ruleset first, or merge of the config PR first? Each leaves a window. The proposal narrows it and does not remove it.
6. **Fork PRs.** The comment and `status` need write or read tokens that fork PRs lack. Should `status` also run from CI?
7. **Admin bypass visibility.** Can `status` see admin merges under `guard`? Depends on what the forge exposes. If not, drop it.
8. **Plans without rulesets** (L2). Fall back to printed instructions. Is that enough?
9. **Sequencing.** Before the public gate (G5) or after? It lowers the adoption wall, which argues for before, and the D-235 kill criterion counts outside repos. The Head Keeper decides.

## 9. Plan

T-060 is this design. If approved, the scout cuts code tickets, each its own PR:

1. `gate.mode` key, parsing, `open` and `guard` behavior in `verify`. Tier 3 by content, so two guards on different models.
2. `watch` summary and comment.
3. `waggle gate status`.
4. `waggle gate guard` and `waggle gate open`, with the `doctor` mismatch finding.
5. Scaffold default and prompt, docs, CONTRIBUTING terms.

## 10. Decision requested

**Question:** What mode should a new install start in, how may the mode be raised, what does `open` do on a gate fault, and does this ship before the public gate (G5)? These are open questions 1, 2, 3 and 9.

**Options:**
1. Default `guard` for every new install. `open` and `watch` are opt-in through `--mode` or the init question, and the signal is that explicit answer (open question 1). Raising the mode stays Tier 3 and needs no carve-out (open question 2). Exit 3 on a broken base config stays a failure in every mode (open question 3). Build before G5, because the opt-in lowers the adoption wall (open question 9). Nothing is cut.
2. Default `open` for every new install. **CUT? needs Head Keeper typed reason.** It narrows VISION core item 1 ("authoritative at merge") and core item 2 ("Required check") for every new hive.
3. Default `open` for a one-Keeper repo and `guard` otherwise, from an explicit prompt. **CUT? needs Head Keeper typed reason.** It narrows the same two core items for solo hives.
4. As option 1, but a carve-out lets a solo Keeper raise `open` to `guard` without an admin merge (open question 2). Needs its own Head Keeper card, since it changes the hardcoded Tier 3 rules.
5. As option 1, but exit 3 in `open` becomes exit 0 with a loud warning (open question 3). Avoids blocking every PR on a typo or an old pin, and weakens "a gate that cannot run is not a pass".
6. Sequencing, any option above: build after G5 instead of before (open question 9). Later is not a cut (VISION rule 3).

**Hive recommendation:** Option 1, with `guard` as the default and `open` and `watch` opt-in, so nothing is cut. Raising stays Tier 3 for now, exit 3 stays a failure in every mode, and the build goes before G5.

**Keeper choice:** Option 1, approved by the Head Keeper, Paul Cleary, on 2026-11-21 at 9:12 PM Pacific Time, in the words "Approve T-060 direction with recommended defaults" (D-382). Open questions 1, 2, 3 and 9 follow the RFC recommendations. Questions 4, 5, 6, 7 and 8 are not decided.
