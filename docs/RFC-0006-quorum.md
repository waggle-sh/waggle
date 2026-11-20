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

This RFC treats Nectar as an input. It assumes T-050 provides:

- **N1.** A function `Nectar(repo, at) -> map[keeper_id]uint64`, where `at` is a commit on `main`.
- **N2.** Deterministic and recomputable: two clones with the same history and claim records return the same map for the same `at`.
- **N3.** Non-negative integers. Zero for a `keeper_id` with no surviving work.
- **N4.** Keyed by the same `keeper_id` the claim record carries (RFC-0003 section 3.1 grammar).
- **N5.** Derived only from git history and claim records. No forge API call is needed to recompute it.

If RFC-0005 settles a different shape (fractional values, a different key, a decay already applied), the two RFCs are reconciled before T-054 starts (open question 1).

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

The tally is an ordered list of items by weighted score, highest first. Ties break by the lower task or issue number. An item is **ready** when it is in the round, not vetoed, not claimed, and its listed dependencies have merged.

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

Weights use integer math so every platform gets the same bytes: `w_milli(k) = isqrt(Nectar(k) * 1_000_000)`, and scores are `sum w_milli * v` as `uint64`. Display divides by 1000. No floating point appears in a stored tally.

### 5.4 Snapshot timing

The round record pins `nectar_at`, the `main` commit at open. Every weight in that round is `Nectar(repo, nectar_at)`. Work that merges during the round counts in the next round. This blocks a merge rush to farm weight while a vote is live, and makes the tally reproducible long after close.

### 5.5 Zero-Nectar Keepers

A Keeper with zero Nectar (a new contributor) may cast a ballot. Its weight is `sqrt(0) = 0`, so it moves no score. It is still stored, signed and shown in a separate **raw votes** column in `waggle quorum status`, so maintainers see what newcomers want. Their first merged task that survives the Nectar window gives them weight in the next round. There is no weight floor: a floor would pay every fake identity (section 7). Open question 3 covers that choice.

### 5.6 Worked example

Round `C = 100`. Three Keepers, items T-101, T-102, T-103.

| Keeper | Nectar | `w` | Votes (cost) | Credits used |
| :--- | ---: | ---: | :--- | ---: |
| A | 400 | 20 | T-101: 8 (64), T-102: 6 (36) | 100 |
| B | 100 | 10 | T-102: 10 (100) | 100 |
| C | 25 | 5 | T-101: 3 (9), T-103: 9 (81) | 90 |

| Item | Weighted score | Raw votes |
| :--- | :--- | ---: |
| T-102 | 20*6 + 10*10 = **220** | 16 |
| T-101 | 20*8 + 5*3 = **175** | 11 |
| T-103 | 5*9 = **45** | 9 |

Order: T-102, T-101, T-103. A has 16 times B's Nectar but only twice B's weight.

If A instead put all 100 credits on T-101 (10 votes), T-101 would score 20*10 + 5*3 = 215 and T-102 only 100. A can win one item outright, but only by giving up every other item. That is the trade quadratic cost is meant to force.

A fourth Keeper D with Nectar 0 who puts 10 votes on T-103 adds 10 raw votes and 0 to its score.

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

The signature is checked against a **Keeper registry**, `.waggle/control/keepers` in OpenSSH `allowed_signers` format (`<keeper_id> namespaces="waggle-quorum-ballot" <key>`), read from the round's base commit. It lives under `.waggle/control/`, so it is Tier 3: adding a Keeper or rotating a key needs Head Keeper approval, and a PR cannot add a voter to the round it opens in. Round, veto and tally records are signed the same way, with namespaces `waggle-quorum-round` and `waggle-quorum-veto`; only a registered key of a listed approver (`gate.approvers`) may sign them.

### 6.4 Tamper evidence

- **Ballots cannot be forged.** A ballot without a valid signature from the registered key for its `keeper_id` is reported as malformed and never counted.
- **Ballots can be deleted** by anyone who can push `refs/waggle/*`; git refs have no forge-side protection we rely on. So `waggle vote` prints a **receipt** (the ballot OID), and the close record lists every counted ballot OID. `waggle quorum verify --receipt <oid>` tells a voter whether their ballot was counted. A voter whose ballot was dropped re-pushes it from their own clone before close.
- **The close is pinned.** `tally.json` holds the item set, `nectar_at`, the Nectar map used, every counted ballot OID, applied veto records and the scores. Its OID is cited in the records PR that changes `ROADMAP.md`, and that PR goes through the gate.
- **Late ballots** are those not in the ballot set the closer fetched. `cast_at` is author text and proves nothing, so the closer's fetch is the deadline. A disputed close is a Head Keeper call, recorded with `waggle decide`.

### 6.5 Reading and tallying

`waggle quorum close` and `verify` fetch `refs/waggle/quorum/*`, read the round record, load each ballot ref, verify signatures against the registry at the base commit, re-check `sum v^2 <= C` and the item set, read Nectar at `nectar_at` through the T-050 interface, apply vetoes (section 8), and compute scores. Malformed ballots are listed by ref and reason, as malformed claims are, and never echoed as free text. `verify` exits 0 only if its recomputed `tally.json` matches the stored one byte for byte.

## 7. Sybil resistance and collusion

These are design controls, not reasons to narrow Quorum.

**The core risk.** `sqrt` is concave, so splitting Nectar across identities raises total weight: one identity with Nectar 400 has weight 20; four with 100 each have weight 40. The controls make splitting expensive and visible:

1. **Nectar-gated weight.** Weight comes only from Nectar, which comes only from merged work that survives on `main` (VISION.md item 5) under claims a maintainer wrote (`docs/gate.md` L3). A fake identity has zero weight until it ships real, reviewed, surviving work under its own claims.
2. **Nectar does not transfer.** It is earned per `keeper_id` from claim records. There is no command to move it, so splitting means doing the work under several identities, each through review.
3. **Identity binding.** A ballot counts only with a key in the Tier 3 registry, added with Head Keeper approval. The registry rule is one human, one `keeper_id`. In a company repo the registry follows SSO accounts; in open source it follows forge accounts with a signing key.
4. **Alias merge.** The Head Keeper may record, with `waggle decide`, that several `keeper_id`s are one person. The registry then marks them as one voter: their Nectar is summed before `sqrt`, and only their latest ballot counts.
5. **Credit cap per round.** Every voter has the same `C`, credits do not roll over, and cost is quadratic, so no one can pile votes on one item cheaply.
6. **Optional weight cap.** `quorum.max_weight` (unset by default) caps `w(k)`. A repo with one dominant contributor can use it to keep Quorum meaningful for others.
7. **Snapshot and clawback.** Nectar is read at `nectar_at`, and T-050's revert clawback lowers it on the next round's recompute. Gamed work that is later reverted loses weight.
8. **Decay.** A Nectar decay, so that old work counts less, belongs in T-050. If RFC-0005 has none, Quorum may apply a window at read time (open question 4).
9. **Collusion signals.** `waggle quorum status` flags, without penalty, voters whose ballots are identical across a round, and new registry entries whose first ballot arrives in the same round. The Head Keeper decides what to do; the tool reports.
10. **Head Keeper veto** is the backstop for a captured result (section 8).

Public ballots make vote buying verifiable for the buyer. Commit-reveal (hash during the round, reveal at close) would make that harder and is open question 6.

## 8. Head Keeper veto

- **Who.** The Head Keeper as the gate defines it: the first login in `gate.approvers` of the base config (`docs/gate.md` section 6).
- **How it is recorded.** Two parts, both required. First a decision entry through `waggle decide` (kind `scope`) that names the round, the item and a typed reason. Then `waggle quorum veto --round <r> --item <id> --decision D-<n>` writes a signed `veto.json` onto the round chain. The tally applies a veto only if the cited `D-<n>` exists in `DECISIONS.md` at the close commit. A veto with no entry is reported and ignored.
- **Scope.** A veto removes one item from the round's ready list, or (with `--item all`) rejects the whole round so the previous order stands. It cannot add votes, change weights, reorder other items or edit a ballot. It ends when the round closes; a veto in the next round needs a new entry.
- **Transparency.** `waggle quorum status` shows every vetoed item with its score and the decision ID. The decisions log is append-only under the gate (`docs/gate.md` section 8), so a veto cannot be quietly removed.
- **Override.** In this design the veto is final for its round, as the Head Keeper holds Quorum veto authority in RFC-0003 section 2.2. The Head Keeper can lift a veto with a new decision that supersedes the old one. A vote to override a veto (a supermajority of weight) is open question 5.

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

Config keys, read from the base config: `quorum.credits`, `quorum.round_days`, `quorum.max_weight`. The new module `internal/quorum/` needs its line in `WAGGLE.md` section 2 before T-054 claims it, and `.waggle/control/keepers` is a Head Keeper commit.

## 11. Phasing

Each phase ships and is evaluated before the next. All three stay on the roadmap; none is a cut.

- **Phase 0: this RFC.** The Head Keeper rules on the decision below. No code.
- **Phase 1: T-054 voting MVP.** Registry, round, ballot, veto and tally records; direct writes for Keepers with push access; `vote`, `quorum open|status|veto|close|verify`; Nectar through the section 3 interface, with a fixed-map stub until T-051 lands. **Exit:** one real round in this hive, closed and recomputed byte for byte from a second clone.
- **Phase 2: fork contributors.** `vote --export|import`, then the T-055 relay transport. Needs its own ticket.
- **Phase 3: scale controls.** Area-scoped rounds, alias merge, collusion flags, optional commit-reveal. Needs its own ticket.

## 12. Test plan for T-054

- **Tally math.** The section 5.6 example as a golden test, including the zero-Nectar voter and the integer `w_milli` values.
- **Ballot validation.** Over budget, negative or non-integer votes, unknown item, duplicate item, wrong round, bad `keeper_id`: each refused before any git write.
- **Signatures.** Valid; wrong key; key not in the registry at the base commit; wrong namespace; edited `ballot.json` after signing; registry changed after open does not change the round.
- **CAS.** Two writers on one ballot ref race (the `hookBeforeWrite` pattern in `internal/comb`); recast chains to the previous ballot; last ballot wins.
- **Ref safety.** `keeper_id` values with `.`, `@`, `+` and the 128-character maximum map to valid hashed refs; a ballot whose ref hash does not match its `keeper_id` is malformed; a non-canonical ref under the namespace is reported, never parsed.
- **Determinism.** Close in one clone, `verify` in another: same `tally.json` bytes. Nectar changed after `nectar_at` does not change the tally.
- **Censorship.** A deleted ballot shows as missing for its receipt.
- **Veto.** Applied with a matching decision entry; ignored and reported without one; `all` keeps the prior order; veto signed by a non-Head-Keeper key is refused.
- **Sybil.** Split identities score higher until alias-merged; after merge the score matches one identity.

## 13. Open questions

1. **T-050 reconciliation.** Does RFC-0005 provide N1 to N5? In particular: integer or fractional Nectar, keyed by `keeper_id`, and recomputable once a claim ref is released and its chain collected (RFC-0003 section 3.1 says claim history is not durable past release).
2. **Continuous voting.** Should a later phase add standing ballots, re-read at each pickup, with a staleness rule?
3. **Weight floor for newcomers.** Zero weight is proposed. A small floor would give newcomers a say before their first surviving merge, at the cost of paying every fake identity. This RFC keeps newcomer ballots at zero weight; dropping their ballots altogether would narrow core item 6 and is not proposed.
4. **Decay.** In Nectar (T-050) or at Quorum read time, and over what window?
5. **Veto override.** Should a supermajority of weight be able to override a veto in the next round?
6. **Secret ballots.** Is commit-reveal worth its cost in phase 3?
7. **Area rounds.** Repo-wide or area-earned Nectar for weight?
8. **Delegated veto.** Should a company repo be able to delegate an area veto to a team lead, given the gate knows one Head Keeper?
9. **Negative votes.** Allow `v < 0` (against) at the same `v^2` cost? Proposed no for phase 1; it adds a griefing path.
10. **Urgent work.** How a security fix is picked up ahead of the order: a round veto plus a decision entry, or a named lane?

## Decision requested

This is the text proposed for the hive's decision log. It is **not** in `.waggle/hive/DECISIONS.md`, and this ticket does not edit `.waggle/`. The ID is left for the queen to assign.

**Question:** How should Quorum (VISION.md core item 6) be built for T-054?

**Options:**

1. Accept this RFC: rounds with one Nectar snapshot at open; fixed credits with `v^2` cost; weight `sqrt(Nectar)` on the tally; ballots in `refs/waggle/quorum/*` with SSH-signed payloads checked against a Tier 3 Keeper registry; veto as a `waggle decide` entry plus a signed veto record; T-054 is phase 1, and phases 2 and 3 get their own tickets.
2. As option 1, but weight on credits (each Keeper gets `Nectar` credits) instead of on the tally.
3. As option 1, but continuous voting instead of rounds.

**Hive recommendation:** Option 1. Ballots stay valid without Nectar, the tally is reproducible from git, and identity rests on signatures, so the same design works for one Keeper, many teams and fork contributors.

**Keeper choice:** Not decided.
