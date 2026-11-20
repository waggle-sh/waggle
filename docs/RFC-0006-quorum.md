<!-- SPDX-License-Identifier: Apache-2.0 -->
# RFC-0006: Quorum

> **ROADMAP ONLY. NOT IMPLEMENTED.** Nothing in this RFC exists in the code today. No command, config key, ref or file described here ships in any Waggle build. It is a design proposal for the Head Keeper to rule on, and the decision in the last section is not recorded.

- **Status:** Proposed, roadmap (ticket T-053, release v0.2). Nothing is frozen.
- **Date:** 2026-11-19
- **Builds on:** `.waggle/hive/VISION.md` core item 6, `docs/RFC-0003.md` (claims, CAS refs, Tier 3), `docs/gate.md` (approvers, Head Keeper, decisions log), D-366 (Quorum restored as core vision).
- **Depends on:** T-050 (Nectar design, `docs/RFC-0005-nectar.md`, drafted in parallel). Section 3 states the interface this RFC needs from it.
- **Feeds:** T-054 (voting MVP), T-055 (relay spike).

## 1. Problem

VISION.md core item 6: "Roadmap and issue priority via quadratic voting with weight `sqrt(Nectar)`, plus Head Keeper veto. That is how higher honey means more say over what gets picked up next."

Today the roadmap order is whatever the scout and the Head Keeper write in `ROADMAP.md`. That works for one Keeper. It does not work for a repo shared by many Keepers, a company with many teams on one private repo, or an open source project whose outside contributors should earn a say by shipping work that sticks. "One Keeper today" is a temporary fact about this hive, not the ceiling of the design.

Quorum gives every Keeper a ballot, weighs it by the Nectar they earned, makes strong preferences cost more than mild ones, keeps every ballot in git, and leaves the Head Keeper a recorded, visible veto.

## 2. Goals and non-goals

**Goals**

1. Keepers vote on roadmap tasks and issue priority, and the result says what is picked up next.
2. Cost is quadratic in votes; weight is `sqrt(Nectar)`, exactly as VISION.md states.
3. Ballots live in git. No central server. Anyone with a clone can recompute the tally and get the same bytes.
4. Every ballot is bound to a Keeper identity by a signature, not by who pushed it.
5. The Head Keeper veto is recorded in the append-only decisions log and shown next to the tally.
6. Sybil and collusion risks are answered with design controls (section 7).

**Non-goals**

- Changing how Nectar is scored (T-050, T-051).
- Changing the gate, claims or `lease.json`. Quorum never writes a claim and never widens a lease.
- Secret ballots in v0.2. Ballots are public to repo readers (open question 6).
- New bee roles. If deliberation needs a worker later, that is its own design card (VISION.md out-of-scope list).

## 3. Interface needed from T-050 (assumption)

This RFC treats Nectar as an input. It relies on the output shape in RFC-0005 section 9.2 (PR #234):

- **N1.** A snapshot `Nectar(repo, main_sha, now) -> map[keeper_id]decimal`, the `keepers` list of `waggle nectar --json --now <now>` computed at one `main_sha`. `now` is an input because a unit matures 30 days after merge, so the same `main_sha` gives a different map at a different `now`.
- **N2.** Deterministic: the same inputs (first-parent history of `main` to `main_sha`, the receipt ref, `rules_version`, `now`) give byte-identical JSON in any clone. Quorum records every input it used (section 5.4) so a tally can be replayed.
- **N3.** Each `nectar` is a non-negative finite decimal rounded to 3 places, `max(0, matured - clawed_back)`. `pending` (units still inside the 30-day window) is never part of it. A `keeper_id` absent from the list has Nectar 0.
- **N4.** Keyed by the `keeper_id` the claim and receipt carry (RFC-0003 section 3.1 grammar). Quorum computes `sqrt(nectar)`; Nectar does not.
- **N5.** Derived from git only: the first-parent history of `main`, the receipt ref `refs/waggle/receipts/main` (RFC-0005 section 6.2), `.git-blame-ignore-revs` and the tier config. No forge API call or central server is needed to recompute it.

Any mismatch still open between this section and RFC-0005 is listed in open question 1 and is reconciled before T-054 starts.

## 4. What is voted on

### 4.1 Items

An **item** is one of:

- A roadmap task, written `T-<id>` in canonical display form (RFC-0003 section 3.1). The set comes from the `## Backlog` lines of `.waggle/hive/ROADMAP.md` at the round's base commit.
- A forge issue, written `issue-<n>`. The round opener lists them explicitly.

Item grammar: `^(T-[0-9]{3,6}|issue-[0-9]{1,9})$`. Anything else is rejected before any git call, as ticket IDs are today.

### 4.2 Rounds, not continuous voting

Quorum runs in **rounds**. A round has a fixed item set, a base commit, a credit budget, an open time and a close time. Keepers may recast their ballot any number of times while it is open; the last ballot wins.

Why rounds over continuous voting:

- **One Nectar snapshot per round.** Weights cannot shift under a ballot mid-count (section 5.4).
- **A tally is a single object** that anyone can recompute and cite in a records PR.
- **Credits reset per round,** so nobody hoards influence for a later fight.
- **Continuous voting** needs a rule for when a standing ballot goes stale and when Nectar is re-read. That is open question 2, not the MVP.

Default cadence is set by `quorum.round_days` (14 by default, read from the base config). A round may also be opened per release.

### 4.3 From tally to "what gets picked up next"

The tally is an ordered list of items by weighted score, highest first. Ties break by the lower task or issue number. An item is **ready** when it is in the round, not vetoed, has no active claim (an expired claim does not hold a task), and its listed dependencies have merged.

- `waggle quorum status --next` prints the top ready item.
- The scout and queen (this hive's roles), or whoever plans in another repo, pick the top ready item unless the Head Keeper vetoed the order (section 6).
- The new order lands in `ROADMAP.md` through a normal records PR that cites the tally commit OID. The gate checks that PR as it checks any hive change.

Quorum orders work. It does not assign it, claim it or merge it.

## 5. Quadratic voting mechanics

### 5.1 Credits and cost

Each Keeper gets the same `C` voice credits per round (`quorum.credits`, 100 by default). A ballot gives each item a whole number of votes `v >= 0`. The cost of `v` votes on one item is `v^2`. A ballot is valid only if the sum of `v^2` over its items is at most `C`. Unspent credits are lost at close.

### 5.2 Where `sqrt(Nectar)` applies: the tally, not the credits

The tally for item `i` is:

```
score(i) = sum over Keepers k of  w(k) * v(k, i),   where  w(k) = sqrt(Nectar(k))
```

The alternative is to give each Keeper `Nectar(k)` credits. Both scale a Keeper's maximum push on one item by `sqrt(Nectar)`. This RFC picks the tally because:

1. **A ballot is valid without Nectar.** `sum v^2 <= C` is checked at cast time with no Nectar lookup. The ballot store and the Nectar scorer stay separate, and a Nectar fix (a revert clawback, a T-051 bug fix) changes the tally on recompute without making any stored ballot invalid.
2. **Intensity and standing stay separate.** Quadratic cost measures how much a Keeper cares about one item relative to their other items. Nectar measures how much say they earned. Mixing them in one budget hides which is which.
3. **It is the literal reading** of "weight `sqrt(Nectar)`" in VISION.md: a per-Keeper multiplier on votes.
4. **Every Keeper has the same ballot form,** so a new contributor learns the tool before earning weight.

### 5.3 Deterministic arithmetic

Weights use integer math so every platform gets the same bytes. Nectar arrives as a decimal with at most 3 places (N3). The reader parses each `nectar` value from its JSON text as a decimal, never through a float, into integer milli-Nectar `m(k) = Nectar(k) * 1000` (so 14.25 becomes 14250). A value that is negative, non-finite or has more than 3 decimal places is malformed and stops the close. Then:

```
w_milli(k) = isqrt(m(k) * 1000)        // = floor(1000 * sqrt(Nectar(k)))
score(i)   = sum over k of w_milli(k) * v(k, i)    // uint64
```

`sqrt(m / 1000) * 1000 = sqrt(m * 1000)`, so `w_milli` is the weight in thousandths, floored. For Nectar 400, `m = 400000` and `w_milli = isqrt(400000000) = 20000`. For Nectar 14.25, `m = 14250` and `w_milli = isqrt(14250000) = 3774` (3774^2 = 14243076, 3775^2 = 14250625). Display divides by 1000. No floating point appears in a stored tally.

### 5.4 Snapshot timing

The round record pins the Nectar snapshot as `nectar_at`, which holds every scorer input: `main_sha` (the `main` commit at open), `now` (the RFC 3339 open time, passed as `--now`), `receipts_sha` (the head of `refs/waggle/receipts/main` at open) and `rules_version`. Every weight in that round is `Nectar(repo, main_sha, now)` read from that receipt head. Work that merges or matures during the round counts in the next round. This blocks a merge rush to farm weight while a vote is live, and makes the tally reproducible long after close.

### 5.5 Zero-Nectar Keepers

A Keeper with zero Nectar (a new contributor) may cast a ballot. Its weight is `sqrt(0) = 0`, so it moves no score. It is still stored, signed and shown in a separate **raw votes** column in `waggle quorum status`, so maintainers see what newcomers want. Their first merged task that survives the Nectar window gives them weight in the next round. There is no weight floor: a floor would pay every fake identity (section 7). Open question 3 covers that choice.

### 5.6 Worked example

Round `C = 100`. Three Keepers, items T-101, T-102, T-103.

| Keeper | Nectar | `m` | `w_milli` | `w` | Votes (cost) | Credits used |
| :--- | ---: | ---: | ---: | ---: | :--- | ---: |
| A | 400.000 | 400000 | 20000 | 20 | T-101: 8 (64), T-102: 6 (36) | 100 |
| B | 100.000 | 100000 | 10000 | 10 | T-102: 10 (100) | 100 |
| C | 25.000 | 25000 | 5000 | 5 | T-101: 3 (9), T-103: 9 (81) | 90 |

| Item | Weighted score (`w`) | Stored (`w_milli`) | Raw votes |
| :--- | :--- | ---: | ---: |
| T-102 | 20*6 + 10*10 = **220** | 220000 | 16 |
| T-101 | 20*8 + 5*3 = **175** | 175000 | 11 |
| T-103 | 5*9 = **45** | 45000 | 9 |

Order: T-102, T-101, T-103. A has 16 times B's Nectar but only twice B's weight.

If A instead put all 100 credits on T-101 (10 votes), T-101 would score 20*10 + 5*3 = 215 and T-102 only 100. A can win one item outright, but only by giving up every other item. That is the trade quadratic cost is meant to force.

A fourth Keeper D with Nectar 0 who puts 10 votes on T-103 adds 10 raw votes and 0 to its score. A fifth Keeper E with Nectar 14.250 (`w_milli` 3774) who puts 5 votes on T-103 would add 18870 to its stored score, displayed as 18.870.

## 6. Ballot store

### 6.1 Refs

All Quorum state lives under `refs/waggle/quorum/`, beside the claim namespace and fetched the same way:

- `refs/waggle/quorum/round-<r>/record` points to the **round commit chain**: the open record (`round.json`), then any veto records (`veto.json`), then the close record (`tally.json`). `<r>` is an unpadded integer, as `ticket-<n>` is.
- `refs/waggle/quorum/round-<r>/k-<hash>` points to one Keeper's **ballot chain**, where `<hash>` is the lowercase SHA-256 hex of the `keeper_id` bytes. A hash keeps every legal `keeper_id` (which may hold `.`, `@` or `+`, and run to 128 characters) a safe ref component of fixed length, and blocks ref-name injection. The reader checks that the hash matches the `keeper_id` inside the ballot.
- Both sit under `round-<r>/`, so no ref is also a directory of another ref.

One ref per Keeper per round means two Keepers never contend for the same ref. A recast writes a child commit whose parent is that Keeper's previous ballot, so `git log` of the ref is the ballot's history.

### 6.2 Writes reuse the claim CAS

Writes reuse the `CombBackend` pattern in `internal/comb/git.go`: build the blob with `hash-object`, wrap it with `mktree` and `commit-tree` (fixed identity and dates, like claim commits), then write with `update-ref <ref> <new> <expected>` locally, or with `fetch` then `push --atomic --force-with-lease=<ref>:<expected>` to the remote. A lost race on one's own ballot (two machines) retries on the new parent. T-054 should share the commit and CAS helpers rather than copy them.

### 6.3 Signing and identity

The claim commit seam (`commitTree`) is unsigned by design, and the gate does not check who wrote a claim (`docs/gate.md` L3). A ballot cannot rely on that, because the push identity is not the voter: a relay, a maintainer or an attacker with push access could push it.

So each ballot commit's tree holds two blobs:

- `ballot.json`: `version`, `round`, `keeper_id`, `votes` (sorted list of `{item, votes}`), `credits_used`, `prev` (the previous ballot OID or empty).
- `ballot.sig`: an SSH signature over the exact `ballot.json` bytes, made with `ssh-keygen -Y sign -n waggle-quorum-ballot`.

The signature is checked against a **Keeper registry**, `.waggle/control/keepers` in OpenSSH `allowed_signers` format (`<keeper_id> namespaces="waggle-quorum-ballot" <key>`), read from the round's base commit. It lives under `.waggle/control/`, so it is Tier 3: adding a Keeper or rotating a key needs Head Keeper approval, and a PR cannot add a voter to the round it opens in. Round and tally records are signed the same way with namespace `waggle-quorum-round`; only a registered key of a listed approver (`gate.approvers`) may sign them. Veto records use their own namespace, `waggle-quorum-veto`, and only the Head Keeper's registered key may sign them (section 8); a veto signed by any other key, approver or not, is refused.

### 6.4 Tamper evidence

- **Ballots cannot be forged.** A ballot without a valid signature from the registered key for its `keeper_id` is reported as malformed and never counted.
- **Ballots can be deleted** by anyone who can push `refs/waggle/*`; git refs have no forge-side protection we rely on. So `waggle vote` prints a **receipt** (the ballot OID), and the close record lists every counted ballot OID. `waggle quorum verify --receipt <oid>` tells a voter whether their ballot was counted. A voter whose ballot was dropped re-pushes it from their own clone before close.
- **The close is pinned.** `tally.json` holds the item set, `nectar_at` (section 5.4), the Nectar map used, every counted ballot OID, applied veto records and the scores. Its OID is cited in the records PR that changes `ROADMAP.md`, and that PR goes through the gate.
- **Late ballots** are those not in the ballot set the closer fetched. `cast_at` is author text and proves nothing, so the closer's fetch is the deadline. A disputed close is a Head Keeper call, recorded with `waggle decide`.

### 6.5 Reading and tallying

`waggle quorum close` and `verify` fetch `refs/waggle/quorum/*`, read the round record, load each ballot ref, verify signatures against the registry at the base commit, re-check `sum v^2 <= C` and the item set, read Nectar for the pinned `nectar_at` inputs through the section 3 interface, apply vetoes (section 8), and compute scores. Malformed ballots are listed by ref and reason, as malformed claims are, and never echoed as free text. `verify` exits 0 only if its recomputed `tally.json` matches the stored one byte for byte.

## 7. Sybil resistance and collusion

These are design controls, not reasons to narrow Quorum.

**The core risk.** `sqrt` is concave, so splitting Nectar across identities raises total weight: one identity with Nectar 400 has weight 20; four with 100 each have weight 40. The controls make splitting expensive and visible:

1. **Nectar-gated weight.** Weight comes only from Nectar, which comes only from merged work that survives on `main` (VISION.md item 5) under claims a maintainer wrote (`docs/gate.md` L3). A fake identity has zero weight until it ships real, reviewed, surviving work under its own claims.
2. **Nectar does not transfer.** It is earned per `keeper_id` from claim records. There is no command to move it, so splitting means doing the work under several identities, each through review.
3. **Identity binding.** A ballot counts only with a key in the Tier 3 registry, added with Head Keeper approval. The registry rule is one human, one `keeper_id`. In a company repo the registry follows SSO accounts; in open source it follows forge accounts with a signing key.
4. **Alias merge.** The Head Keeper may record, with `waggle decide`, that several `keeper_id`s are one person. The registry then marks them as one voter. Alias merge does not change the weight formula: it only sums the Nectar of those `keeper_id`s into one per-person value before `sqrt`, so the voter's weight is still `sqrt(Nectar)`, and only their latest ballot counts.
5. **Credit cap per round.** Every voter has the same `C`, credits do not roll over, and cost is quadratic, so no one can pile votes on one item cheaply. This caps credits, not weight.
6. **Snapshot and clawback.** Nectar is read at the pinned `nectar_at` inputs, and T-050's revert clawback lowers it on the next round's recompute. Gamed work that is later reverted loses weight.
7. **Collusion signals.** `waggle quorum status` flags, without penalty, voters whose ballots are identical across a round, and new registry entries whose first ballot arrives in the same round. The Head Keeper decides what to do; the tool reports.
8. **Head Keeper veto** is the backstop for a captured result (section 8).

Not part of this proposal: a cap on weight and any decay or window on Nectar. A weight cap departs from weight `sqrt(Nectar)`, and decay discounts Nectar a Keeper earned; either would narrow VISION.md core items 5 and 6. They are open questions 4 and 11, each marked CUT? needs Head Keeper typed reason.

Public ballots make vote buying verifiable for the buyer. Commit-reveal (hash during the round, reveal at close) would make that harder and is open question 6.

## 8. Head Keeper veto

- **Who.** The Head Keeper as the gate defines it: the first login in `gate.approvers` of the base config (`docs/gate.md` section 6).
- **How it is recorded.** Two parts, both required. First a decision entry through `waggle decide` (kind `scope`) that names the round, the item and a typed reason. Then `waggle quorum veto --round <r> --item <id> --decision D-<n>` writes a signed `veto.json` onto the round chain. The tally applies a veto only if the cited `D-<n>` exists in `DECISIONS.md` at the close commit. A veto with no entry is reported and ignored.
- **Scope.** A veto removes one item from the round's ready list, or (with `--item all`) rejects the whole round so the previous order stands. It cannot add votes, change weights, reorder other items or edit a ballot. It ends when the round closes; a veto in the next round needs a new entry.
- **Transparency.** `waggle quorum status` shows every vetoed item with its score and the decision ID. The decisions log is append-only under the gate (`docs/gate.md` section 8), so a veto cannot be quietly removed.
- **Override.** In this design the veto is final for its round, as RFC-0003 section 2.2 (the roster table, where the role is still called Master Keeper) gives the Head Keeper Quorum veto authority. The Head Keeper can lift a veto with a new decision that supersedes the old one. A vote to override a veto (a supermajority of weight) is open question 5.

## 9. Multi-Keeper, company repos and open source

**Many Keepers with push access** (VISION.md item 3) write their own ballot refs directly. Ballot refs never contend across Keepers, so the only race is one Keeper on two machines, which CAS handles.

**Company repos with many teams.** Ballots are readable only by repo readers, since refs live in the private repo. The registry is filled from the company's SSO accounts. Two needs come up: rounds scoped to an area, and team leads with their own veto. For area rounds, the round record may carry `scope` (a list of directories), and only tasks whose lease sits under it are items. Whether weight in an area round uses repo-wide Nectar or Nectar earned in that area is open question 7. A delegated area veto is open question 8; the gate today knows one Head Keeper.

**Open source with fork contributors.** Contributors without push access cannot write `refs/waggle/*`, just as they cannot write claims. Because authenticity comes from the signature, not the pusher, transport can be untrusted:

- `waggle vote --export <file>` writes the signed ballot commit as a small bundle.
- A maintainer imports it with `waggle vote import <file>`, which verifies before writing.
- The relay that T-055 designs can carry the same bundle from a PR or issue comment and write the ref with the repo token. Quorum needs nothing from the relay except "write this signed object to this ref".

A fork contributor earns weight the same way: a maintainer writes their claim with an `<owner>/<repo>:<branch>` entry, the work merges and survives, and Nectar follows the claim's `keeper_id`.

## 10. CLI surface for T-054

Placeholders for discussion. Exit codes follow the house rule: 0 ok, 1 check or validation failed, 2 usage, 3 git or I/O error.

| Command | Who | What it does |
| :--- | :--- | :--- |
| `waggle quorum open [--items T-101,issue-7] [--from-roadmap] [--closes <RFC3339>]` | approver | Writes a signed round record with the item set, `C`, `nectar_at` and close time. |
| `waggle vote --round <r> T-101=8 T-102=6 [--dry-run]` | any Keeper | Checks cost, signs and writes the ballot; prints the receipt OID. |
| `waggle vote --export <file>` / `waggle vote import <file>` | contributor / maintainer | Moves a signed ballot without push access. |
| `waggle quorum status [--round <r>] [--next] [--json]` | anyone | Items, provisional scores, raw votes, credits left, vetoes, malformed ballots, collusion flags. |
| `waggle quorum veto --round <r> --item <id or all> --decision D-<n>` | Head Keeper | Writes a signed veto record that cites a decision entry. |
| `waggle quorum close --round <r>` | approver | Fetches ballots, computes and writes `tally.json`. |
| `waggle quorum verify --round <r> [--receipt <oid>]` | anyone | Recomputes the tally from refs, registry, Nectar and decisions; checks a receipt. |

Config keys, read from the base config: `quorum.credits`, `quorum.round_days`. The new module `internal/quorum/` needs its line in `WAGGLE.md` section 2 before T-054 claims it, and `.waggle/control/keepers` is a Head Keeper commit.

## 11. Phasing

Each phase ships and is evaluated before the next. All three stay on the roadmap; none is a cut.

- **Phase 0: this RFC.** The Head Keeper rules on the decision below. No code.
- **Phase 1: T-054 voting MVP.** Registry, round, ballot, veto and tally records; direct writes for Keepers with push access; `vote`, `quorum open|status|veto|close|verify`; Nectar through the section 3 interface, with a fixed-map stub until T-051 lands. **Exit:** one real round in this hive, closed and recomputed byte for byte from a second clone.
- **Phase 2: fork contributors.** `vote --export|import`, then the T-055 relay transport. Needs its own ticket.
- **Phase 3: scale controls.** Area-scoped rounds, alias merge, collusion flags, optional commit-reveal. Needs its own ticket.

## 12. Test plan for T-054

- **Tally math.** The section 5.6 example as a golden test, including the zero-Nectar voter, the fractional Keeper E and the integer `w_milli` values.
- **Nectar parsing.** Values with 0 to 3 decimals parse exactly; a negative, non-finite or 4-decimal value is malformed; an absent `keeper_id` is 0; `pending` is never read.
- **Ballot validation.** Over budget, negative or non-integer votes, unknown item, duplicate item, wrong round, bad `keeper_id`: each refused before any git write.
- **Signatures.** Valid; wrong key; key not in the registry at the base commit; wrong namespace; edited `ballot.json` after signing; registry changed after open does not change the round.
- **CAS.** Two writers on one ballot ref race (the `hookBeforeWrite` pattern in `internal/comb`); recast chains to the previous ballot; last ballot wins.
- **Ref safety.** `keeper_id` values with `.`, `@`, `+` and the 128-character maximum map to valid hashed refs; a ballot whose ref hash does not match its `keeper_id` is malformed; a non-canonical ref under the namespace is reported, never parsed.
- **Determinism.** Close in one clone, `verify` in another: same `tally.json` bytes. A `main` advance, a later `now` or receipts appended after the pinned `nectar_at` do not change the tally.
- **Censorship.** A deleted ballot shows as missing for its receipt.
- **Veto.** Applied with a matching decision entry; ignored and reported without one; `all` keeps the prior order; veto signed by a non-Head-Keeper key is refused.
- **Sybil.** Split identities score higher until alias-merged; after merge the score matches one identity.

## 13. Open questions

1. **T-050 reconciliation.** Section 3 follows RFC-0005 section 9.2. Durable ownership after a claim is released rests on the receipt ref RFC-0005 section 6.2 proposes (T-051a); until it lands, phase 1 uses the fixed-map stub. Remaining mismatches to settle with RFC-0005 before T-054 starts:
   - **Pinning.** 9.2 says the snapshot is for one `main_sha` and Quorum must record it. A replay also needs `now` (maturity is time-dependent) and the receipt head, since receipts are appended after open. Quorum records both in `nectar_at` (section 5.4); 9.2 does not yet output `now` or a receipt head, and its `computed_at` may be the wall clock.
   - **Decimal encoding.** 9.2 gives `nectar` as a JSON number rounded to 3 decimals. Section 5.3 parses its text as an exact decimal into milli-Nectar. RFC-0005 should guarantee the serialization (no exponent form, at most 3 decimals) so no reader goes through a float.
   - **Signed snapshot.** RFC-0005 open question 9 asks whether Quorum should require a published, signed snapshot. This RFC recomputes from the pinned inputs and does not require one.
2. **Continuous voting.** Should a later phase add standing ballots, re-read at each pickup, with a staleness rule?
3. **Weight floor for newcomers.** Zero weight is proposed. A small floor would give newcomers a say before their first surviving merge, at the cost of paying every fake identity. This RFC keeps newcomer ballots at zero weight; dropping their ballots altogether would narrow core item 6 and is not proposed.
4. **Decay. CUT? needs Head Keeper typed reason.** Not part of this proposal. Any decay or window on Nectar, in T-050 or at Quorum read time, would discount earned Nectar and narrow VISION.md core items 5 and 6. RFC-0005 open question 2 proposes no decay in v0.2 and applies the same CUT? rule.
5. **Veto override.** Should a supermajority of weight be able to override a veto in the next round?
6. **Secret ballots.** Is commit-reveal worth its cost in phase 3?
7. **Area rounds.** Repo-wide or area-earned Nectar for weight?
8. **Delegated veto.** Should a company repo be able to delegate an area veto to a team lead, given the gate knows one Head Keeper?
9. **Negative votes.** Allow `v < 0` (against) at the same `v^2` cost? Proposed no for phase 1; it adds a griefing path.
10. **Urgent work.** How a security fix is picked up ahead of the order: a round veto plus a decision entry, or a named lane?
11. **Weight cap. CUT? needs Head Keeper typed reason.** Not part of this proposal, and no config key exists for it. A cap on `w(k)` (for a repo with one dominant contributor) would depart from weight `sqrt(Nectar)` in VISION.md core item 6.

## Decision requested

This is the text proposed for the hive's decision log. It is **not** in `.waggle/hive/DECISIONS.md`, and this ticket does not edit `.waggle/`. The ID is left for the queen to assign.

**Question:** How should Quorum (VISION.md core item 6) be built for T-054?

**Options:**

1. Accept this RFC: rounds with one Nectar snapshot at open; fixed credits with `v^2` cost; weight `sqrt(Nectar)` on the tally; ballots in `refs/waggle/quorum/*` with SSH-signed payloads checked against a Tier 3 Keeper registry; veto as a `waggle decide` entry plus a signed veto record; T-054 is phase 1, and phases 2 and 3 get their own tickets.
2. As option 1, but weight on credits (each Keeper gets `Nectar` credits) instead of on the tally.
3. As option 1, but continuous voting instead of rounds.

**Hive recommendation:** Option 1. Ballots stay valid without Nectar, the tally is reproducible from git, and identity rests on signatures, so the same design works for one Keeper, many teams and fork contributors.

**Keeper choice:** Not decided.
