<!-- SPDX-License-Identifier: Apache-2.0 -->
# Priority rounds

`waggle quorum` and `waggle vote` order work by a vote that weighs each voter by their [Nectar](nectar.md). It orders the backlog; it does not assign, claim or merge anything. The design is in [RFC-0006](RFC-0006-quorum.md). It needs OpenSSH `ssh-keygen` with `-Y` support on every machine that votes, closes or verifies.

## Rounds and credits

A round lists tasks (`T-101`) and issues (`issue-7`). Each voter gets `C` credits (`quorum.credits`, default 100). Casting `v` votes on one item costs `v` squared credits, so 10 votes cost 100 and 3 votes cost 9. Spreading votes is cheap and piling them on one item is not. Votes are whole numbers, never negative.

The round is open for `quorum.round_days` days (default 14) unless `--closes` says otherwise. Add the block to `.waggle/control/config.yaml` only to change the defaults:

```
quorum:
  credits: 100
  round_days: 14
```

Casting again replaces your earlier ballot; the last one counts.

## Weight

On the tally, each voter's votes are multiplied by the square root of their Nectar, read once when the round opens. Nectar earned during the round changes nothing. With the example in RFC-0006 section 5.6, A (Nectar 400, weight 20) puts 8 votes on T-101 and 6 on T-102, B (Nectar 100, weight 10) puts 10 on T-102, and C (Nectar 25, weight 5) puts 3 on T-101 and 9 on T-103. T-102 scores 20*6 + 10*10 = 220, T-101 scores 20*8 + 5*3 = 175 and T-103 scores 5*9 = 45. A has 16 times B's Nectar but only twice B's weight. A voter with no Nectar has no weight. Scores are whole thousandths; no fractions are stored.

## Commands

- `waggle quorum open --items T-101,issue-7` or `--from-roadmap` (the `T-` lines under the Backlog heading of `ROADMAP.md` on main) starts a round. Only a registered key of a `gate.approvers` login can open one.
- `waggle vote --round 1 T-101=3 T-102=2` casts a ballot and prints its receipt. `--dry-run` checks the votes and prints the cost; it writes nothing.
- `waggle quorum status` shows the latest round: scores so far, raw votes, credits left, vetoes and ballots that were not counted. `--round <n>` picks a round. On a closed round, `--next` lists the items that can be picked up now: not vetoed, no active claim, and the tasks they depend on merged.
- `waggle quorum close --round 1` ends the round after its close time and records the tally.
- `waggle quorum verify --round 1` recomputes the tally from the refs and fails unless it matches byte for byte. `--receipt <oid>` also says whether your ballot was counted, replaced by a later one, or is missing.

Every command takes `--json` where it prints a result, and `--repo <dir>`. Exit codes: 0 done, 1 a check or validation failed, 2 usage, 3 git, signing tool or file error.

## Keys and the registry

Ballots and records are signed with an SSH key. `--key <path>` names the private key; without it, `git config user.signingkey` is used when `gpg.format` is `ssh`. Your name in the round is `keeper_id` from `.waggle/local.yaml`.

The public keys live in `.waggle/control/keepers` on main, in OpenSSH `allowed_signers` format, one line per key:

```
<keeper_id> namespaces="waggle-quorum-ballot,waggle-quorum-round,waggle-quorum-veto" <key-type> <public-key>
```

A line without `namespaces=` limited to those three names is ignored. To add a voter, open a pull request that adds the line; the file is a protected path, so it needs the Head Keeper's approval. A round reads the registry as it was on main when the round opened, so a later change does not alter an open round. One person, one `keeper_id`.

## Tally, receipts and verification

Ballots live under `refs/waggle/quorum/`. Closing writes a signed `tally.json` that lists the Nectar used, every counted ballot, the vetoes applied or ignored, and the scores. Anyone with a clone can run `verify` and get the same bytes. Keep the receipt `vote` prints: if a ballot ref is deleted on the remote, `verify --receipt` reports it as missing.

New order goes into `ROADMAP.md` only through a normal pull request that cites the tally.

## Veto

The Head Keeper (the first login in `gate.approvers`) can remove one item from a round, or reject the whole round so the earlier order stands, with `waggle quorum veto --round 1 --item T-102 --decision D-001`. The veto must cite a decision entry that is in `.waggle/hive/DECISIONS.md` on main when the round closes; a veto without one is ignored and listed in the tally. A veto never changes a score or another item's place.

## Limits

- Ballots are public to anyone who can read the repository, so a vote can be bought and checked.
- Anyone who can push `refs/waggle/*` can delete a ballot or rewind a round. Signatures stop forged ballots, not deletions. Receipts, the counted list in the tally and the decision entry make a deletion visible. Protect the `refs/waggle/quorum/` namespace on the forge where it allows.
- Splitting one person's Nectar across several identities raises their total weight, because the square root is concave. The one person, one `keeper_id` rule in the registry is the control until alias merge exists.
- The closer's fetch is the deadline: a ballot not yet pushed when the round closes is not counted.
