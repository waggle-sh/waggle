<!-- SPDX-License-Identifier: Apache-2.0 -->
# Several Keepers on one repo

This page is for teams where more than one person (a Keeper) works in the same repository, each running their own agents on their own plan. It says how claims and the gate keep them apart today, and what is still planned. Statements about behavior name the code that does it; [the gate spec](gate.md) has the line-level cites and the full list of limits (L1 to L12).

Plain terms used here: a **task** (`T-<id>`), a **claim** (the ref `refs/waggle/claims/ticket-<n>`), a **lease** (the directories a claim lets a task change), the **gate** (the required check `verify-leases-and-tiers`), and an **approver** (a login in `gate.approvers`).

## 1. The model in one paragraph

Each Keeper claims a task before work starts. The claim names an owner, the directories, the PR branches and an expiry. Claims are written with a compare-and-swap push, so two people cannot hold the same task at once, and a claim whose directories overlap another active claim is refused. That check at claim time is optimistic. The gate at merge time is authoritative: every commit of a PR must stay inside the lease of the task it names. Nothing else is shared between Keepers. There is no server, no lock service and no shared agent pool; each Keeper's agents run on that Keeper's machine with that Keeper's credentials.

## 2. Identity: who a Keeper is

- **`keeper_id` is the claim owner.** `waggle claim` and `waggle dispatch` read it from `.waggle/control/local.yaml`, a per-clone file that git ignores, or take `--owner <id>` (`internal/config/local.go`, `LoadKeeperID`; `internal/dispatch/task.go`).
- **Use your GitHub login.** The id must match `^[A-Za-z0-9][A-Za-z0-9._@+-]{0,127}$`. The seeded placeholder `local-keeper` is refused as soon as `forge.remote` is set, because two Keepers would collide on it.
- **Approvers are GitHub logins too.** `gate.approvers` in `.waggle/control/config.yaml` holds up to 32 logins, compared ASCII case folded, no teams. The first entry is the Head Keeper. The gate reads this list from the PR's base commit, so a PR cannot add its author (`internal/config/config.go`, [gate spec](gate.md) section 6).
- **The gate does not check who wrote a claim.** It never compares `keeper_id` with the PR author or the pusher (gate spec L3). `keeper_id` is a record of who claimed, not an access control. Keeping it equal to the GitHub login is what makes the record useful later (section 7).

### Adding a second Keeper

1. Give them push access to the repository. Today that is what lets them write `refs/waggle/claims/*`.
2. They clone the repo and set `keeper_id` in `.waggle/control/local.yaml` to their GitHub login. The seeded file says `keeper_id: local-keeper`, which is refused once the repo has a remote.
3. They run `waggle doctor`. Its `claim-refs` finding says whether this clone can push claim refs.
4. If they will review protected paths, the Head Keeper adds their login to `gate.approvers`. That file is under `.waggle/control/`, a Tier 3 path, so the change merges only with an admin bypass or the Head Keeper's SSH signature on the PR head commit ([gate spec](gate.md) section 6, Signed Head Keeper approval). The signature route is on main, not in a release yet.
5. Turn on a required review in the branch ruleset if it is not on already (section 5).

Each Keeper runs their own agents with their own credentials. Keep agent credentials unable to push claim refs, so an agent cannot write or widen its own lease (gate spec L3).

## 3. Contention: two Keepers want the same thing

All of these are decided inside `AcquireLease` (`internal/comb/git.go`), after a fetch of the claim refs from the remote. Nothing is written when a claim is refused, and `waggle claim` exits 4.

| Situation | What happens |
| :--- | :--- |
| Both claim the same task at the same moment | One push wins. The loser's push is rejected by `--force-with-lease`; the CLI re-fetches, sees the ref moved and reports `lost claim race` (`pushClaim`). |
| One claims a task another holds and has not let expire | Refused: `T-<id> held by <keeper_id> until <time>`. |
| Different tasks, overlapping directories | Refused: `T-<a> directory "x" overlaps T-<b> directory "x/y" held by <keeper_id>`. Overlap is on whole path segments: `internal` overlaps `internal/comb`, `internal/comb` does not overlap `internal/combx` (`DirsOverlap`). |
| Different tasks, disjoint directories | Both claims are written. Their PRs merge independently. |
| Same owner claims again | A renewal: a child commit with a new expiry; the stored branch list is kept unless `--branch` is given. |

### Seeing who holds what

`waggle status` fetches the claim refs and lists every active claim with its task, role, owner, gate, directories and time left:

```
Claims (2 active):
  T-101  builder  ana  full  internal/checkout/  expires in 1h12m
  T-102  builder  ben  full  internal/pricing/ docs/pricing/  expires in 47m
```

Raw git works too: `git ls-remote origin 'refs/waggle/claims/*'` lists the refs, and `git log refs/waggle/claims/ticket-<n>` shows a claim's chain of claim, renewal and takeover commits until it is released.

**How to resolve an overlap.** The refusal names the other task and its owner. The options are human ones: wait for it to merge and be released, agree to narrow one lease and re-claim, or fold the work into one task. Waggle has no queue or arbitration step.

**What the claim check cannot see.** Overlap is compared with ASCII letter case folded, so `Internal/Comb` overlaps `internal/comb`, because macOS and Windows checkouts treat them as one directory. A same-owner renewal keeps a directory it already holds byte for byte, so two claims that collided before this rule existed keep renewing but never grow or change hands; `waggle status` prints a warning for each such pair. An older binary can still write a colliding claim, and the gate's `case_differs` and head tree checks still catch the commits at merge. Two claims written in the same instant against a stale view can both pass, because the compare-and-swap is per task ref, not per directory (RFC-0003 section 3.1). A claim also does not reserve files outside its directories. In each case the gate still holds each PR to its own lease, and git's own merge conflicts catch two PRs that edit the same lines. The gate does not compare one task's lease with another's.

## 4. Stale claims and expiry

- **TTL.** A claim expires at `claimed_at` plus `--ttl`, or `leases.default_ttl_minutes` (120 by default; 1 minute to 7 days) (`internal/config/config.go`).
- **Renew.** The owner runs `waggle claim` again before expiry. `waggle dispatch` renews its own claim every half TTL while the agent runs (`internal/dispatch/run.go`).
- **Expired claims block nothing.** An expired claim is skipped by the overlap check, and anyone may claim its task. That is a takeover: a child commit with the new owner, and a note `taking over T-<id> from <old owner> (expired <time>)` (`internal/dispatch/claim.go`).
- **The gate still reads expired claims.** It accepts them with a warning and still runs the branch check (gate spec L4). After a takeover the claim carries the new owner's branch list, so the old owner's open PR fails `claim does not name this pull request's branch` until the claim names it again.
- **Release.** Only the owner can release (`ReleaseLease` returns "held by another owner" otherwise). `waggle release T-<id>` refuses until the task's branch is merged into main, and `--unmerged` skips that check. Release deletes the ref, so the claim's history goes with it.
- **Malformed claims.** A malformed claim on a task refuses both claim and release. `waggle status` and `waggle doctor` name it and print the fix: `waggle force-release --ref <ref>`. Only the first login in `gate.approvers`, read from main on the remote and never from local files, can clear it. It takes two runs. The first, without `--oid`, only looks: it prints the ref, the full object id, who may clear it and the exact second command. The second, with `--oid` and `--reason`, deletes the ref only if it is still at that object id, and records who cleared it and why in `refs/waggle/audit/force-releases`; read it with `git log --first-parent refs/waggle/audit/force-releases`. The check is advisory: anyone who can push to the remote can still delete the ref with raw git and leave no record.
- **A stuck claim that has not expired.** The owner is gone and the TTL is days away, so the claim still blocks its directories. The first login in `gate.approvers` runs the same two steps with the task, `waggle force-release T-<id>`, then the line it prints. The owner's open PR then fails the gate with `no claim for T-<id>` until the task is claimed again with that branch.

**Habit for teams:** set a TTL that matches how long your agents run, renew when a run is extended, and release after merge. A claim left to expire is not lost work, but whoever takes it over writes a new branch list.

## 5. Review between Keepers

There are two separate things, and teams need both.

1. **GitHub's required review.** A ruleset can require one or more approving reviews before merge. GitHub does not let anyone approve their own pull request, so this is where a second Keeper comes in. It is a GitHub setting, not a Waggle check, and it is also the main defense against a forged check (gate spec L11).
2. **The gate's approver rule (D-233).** A change to a path in `gate.protected_paths` fails the gate unless a listed approver approved the PR at its exact head commit, after the last base change, with no listed approver at changes-requested; then someone with Write re-runs the failed check. Only the Head Keeper's approval passes `.waggle/hive/` paths. An approval never clears a path outside the lease on an ordinary path, and never clears a Tier 3 control path.

A listed approver who opened the PR cannot approve it on GitHub, so the gate also counts a comment with the line `waggle-approve <40-hex head SHA>` from that approver (gate spec section 6). That passes the gate's approver rule only. It does not satisfy GitHub's required review, which still needs another person.

## 6. Worked example: two Keepers, four agents

Ana and Ben share `example/shop`. Both logins are in `gate.approvers`; Ana is first, so she is the Head Keeper. Each has `keeper_id` set to their login.

1. Ana claims the checkout work: `waggle claim T-101 --dirs internal/checkout --gate full --branch t-101`. Her builder agent runs through `waggle dispatch` on her laptop.
2. Ben claims the pricing work: `waggle claim T-102 --dirs internal/pricing,docs/pricing --gate full`. The branch defaults to `t-102`.
3. Ben's second agent needs a shared helper and he tries a wide lease. It is refused, because `internal` overlaps Ana's `internal/checkout` (and his own `internal/pricing`). He claims T-103 as `internal/money` instead:

   ```
   $ waggle claim T-103 --dirs internal --gate full
   waggle claim: T-103 directory "internal" overlaps T-101 directory "internal/checkout" held by ana: comb: directories overlap an active lease
   $ waggle claim T-103 --dirs internal/money --gate full
   ```
4. Ana's agent edits `internal/pricing/rates.go` by mistake. Her PR fails the gate: `T-101: path is outside the claim directories`. She drops the commit, or asks Ben to take the change in T-102.
5. Ben's PR is clean. Ana reviews and approves it on GitHub; the ruleset's one required review is met, the gate is green, and it merges. Ben runs `waggle release T-102`.
6. Ana's T-101 runs long and its claim expires overnight. Nobody else wanted it, so in the morning she runs `waggle claim T-101 --dirs internal/checkout --gate full --branch t-101` again. Because the old claim had expired, this is a takeover of her own claim, not a renewal, so she names the branch again. Had Ben taken it over first, her PR would fail the branch check until the claim named `t-101` again.

## 7. Nectar and Quorum across Keepers

Both are on main, not in a release yet (the latest release is v0.1.1).

- **Nectar** credits each merged task to the Keeper in its release record: `keeper_id`, plus up to 4 `--co` keepers who share the credit. `waggle release` writes the record, and so do `waggle nectar backfill` and the gate's optional post-merge job, so a release no longer loses who held the task. `waggle nectar` scores every Keeper from `main` and those records, and anyone with a clone gets the same number. A self merge in a repo with several Keepers earns less review credit. See [Nectar](nectar.md).
- **Quorum** orders the backlog by quadratic votes weighted by `sqrt(Nectar)`. Each voter signs ballots with an SSH key listed under their `keeper_id` in `.waggle/control/keepers`, a protected path. The rule is one person, one `keeper_id`. The Head Keeper can veto an item or a whole round, citing a decision. See [Quorum](quorum.md).

Keeping `keeper_id` equal to the GitHub login (section 2) is what lets both line up a Keeper's claims, records, approvals and ballots.

## 8. What is not built yet

- **Claims for contributors without push access.** Today a maintainer writes the claim for a fork PR, with an `<owner>/<repo>:<branch>` branch entry (gate spec L3). `forge.comb_mode: relay` is accepted by config but every claim operation then refuses with "relay comb mode not implemented" (`internal/comb/git.go`). The design spike is [docs/rfc-notes/relay.md](rfc-notes/relay.md). Head Keeper decision, Paul Cleary, 2026-11-19: maintainer claims on a contributor's behalf now; v0.2 builds the `/waggle claim` comment relay with maintainer approval on by default.
- **Proof of who wrote a claim.** The gate trusts any claim ref it can read (gate spec L3). A signature check on claims is planned and not designed.
