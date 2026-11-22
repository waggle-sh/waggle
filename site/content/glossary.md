<!-- SPDX-License-Identifier: Apache-2.0 -->
# Glossary

Waggle uses bee words on purpose. Each one stands for something plain, and this page says what. The big picture is on [the hive](/docs/hive/). Terms for things that are not in a release yet are marked **on main** (merged, not in a release yet; the latest release is v0.1.1), **coming soon** (a part that already works in a smaller form) or **roadmap** (designed, no code).

## People and agents

**Keeper.** A human working on the repo. Each keeper owns a plan and the agents that carry it out.

**Head Keeper.** The keeper who owns the project's direction. The first login in `gate.approvers`. Only their approval passes a change to the hive's steering files. **On main**, their SSH signature on a PR's head commit also passes Tier 3 paths, and only they can run `waggle force-release` and `waggle quorum veto`.

**Master Keeper.** The project owner in their role as holder of the veto over Quorum results. Not a separate title or setting: today the Head Keeper holds the veto, through `waggle quorum veto` (**on main**).

**Queen.** A coordinating agent a keeper talks to. She does not write feature code. She agrees the task with the keeper, opens the issue, writes the claim and starts the workers.

**Forager.** A worker that brings material back: it reproduces bugs, writes failing tests and maps which directories a task needs.

**Builder.** A worker that implements a task inside its claim.

**Guard.** A reviewer. It reads a change, ideally on a different model than the builder's, and ends with a pass or a must-fix.

**Scout.** A planner. It writes task briefs, the roadmap and the specs.

**Walt.** Our mascot, a round apricot bee. He is not a role.

## Places and records

**Hive.** The shared repository and the rules every agent works under. The `.waggle/hive/` folder holds its steering files, such as the roadmap and the decisions log.

**Claim.** A record, kept in git refs, that a task is being worked on. It names which directories the task may change and which branches its pull request may come from.

**Lease.** The directories a claim lets a task change.

**Gate.** A required GitHub check, `verify-leases-and-tiers`, that fails a pull request whose commits leave their task's lease.

**Tier.** How much review a path needs. Tiers 0 and 1 are small work, Tier 2 is steering files and logic, and Tier 3 is control paths no claim can lease.

**Decisions log.** An append-only file, `.waggle/hive/DECISIONS.md`, that records project decisions with their reasons.

**Dispatch.** `waggle dispatch`: claims a task, runs a worker in its own worktree and checks its changes against the claim.

## Honey

**Honey.** A contributor's standing in the project, built from real merged work. It has two halves, Nectar and Quorum. Both are **on main**.

**Nectar.** Reputation from merged work that survives 30 days on `main` without a revert, credited through the release record of who held the task. **On main**: `waggle nectar` scores it and `waggle nectar explain` shows one task; the release records and `waggle nectar backfill` are in v0.1.1. See [Nectar](/docs/nectar/).

**Quorum.** Quadratic voting on roadmap and issue priority, where each person's weight is the square root of their Nectar. The Head Keeper, in the Master Keeper role, can veto an item or a whole round. **On main**: `waggle vote` and `waggle quorum`. See [Quorum](/docs/quorum/).

See [the hive](/docs/hive/#built-today-and-coming) for what is built and what is coming.
