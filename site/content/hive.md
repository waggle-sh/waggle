<!-- SPDX-License-Identifier: Apache-2.0 -->
# The hive: where Waggle is going

Waggle is for many people, each running their own agent team, working on one repository without stepping on each other. Coding speed is not the hard part any more. Review time, merge coordination and keeping agents inside the paths they were given are. Waggle is the system that handles those, and it speaks in bees on purpose. Every term below gets a short plain gloss, and the [glossary](/docs/glossary/) lists them all in one place.

**The gate is the first piece you can use today, not the whole product.** This page lays out the whole product and marks what exists now and what is still coming.

## The idea in one picture

<p>
  <img class="light-only" src="/images/hive-task-journey-light.webp" width="800" height="494" alt="One ticket, start to merge. Keeper and Queen agree the task, Scout writes the brief, Queen writes the claim, Forager maps dirs and a failing test, Builder implements in the claim, Guard reviews, then the work is merged to main on a pass, or returns to the Builder on a must-fix." />
  <img class="dark-only" src="/images/hive-task-journey-dark.webp" width="800" height="494" alt="One ticket, start to merge. Keeper and Queen agree the task, Scout writes the brief, Queen writes the claim, Forager maps dirs and a failing test, Builder implements in the claim, Guard reviews, then the work is merged to main on a pass, or returns to the Builder on a must-fix." />
</p>

The **hive** (the shared repository and the rules every agent works under) has people and agents in clear roles:

- **Keepers** are the humans. Each keeper owns a plan and the agents that carry it out. One keeper is the **Head Keeper** (the person who owns the product direction and can approve changes to the hive's own steering files).
- **Queens** are the coordinating agents a keeper talks to. A queen does not write feature code. She agrees the task with the keeper, opens the issue, writes the **claim** (the record of who is working on what, and which directories the task may change) and starts the workers.
- **Foragers** are workers that go out and bring things back: they reproduce bugs, write the failing test and map which directories a task needs.
- **Builders** are workers that implement the task inside their claim.
- **Guards** are reviewers. A guard reads the change, ideally on a different model than the builder used, and ends with a pass or a must-fix.
- **Scouts** are planners. They search ahead, write the task brief and keep the roadmap and specs.

Walt, our mascot, is the bee in the corner of the landing page. He is not a role. He is there so the project has a face.

## Many keepers on one repo

The case Waggle is built for is more than one person. Each keeper runs their own agents, on their own plan, for their own work. Nobody donates tokens to anyone else, and nobody needs anyone's agent credentials.

What keeps those keepers from colliding is the same small set of records:

1. Every task has one claim at a time, written before the work starts and naming the directories it may change.
2. Two claims that overlap are refused when the second is written.
3. Every commit is checked against its own task's claim before it can merge.

That works for one person with a few agents, for a company with several teams on one private repo, and for an open source project whose outside contributors bring their own agents. A solo keeper can start alone and add people later without rewriting how the repo works.

## Layer one: the gate

The **gate** is a required GitHub check named `verify-leases-and-tiers`. It reads the claims and the commits and fails a pull request that leaves its task's lease. It also protects the paths that steer the hive, and it keeps an append-only decisions log. It works with any agent that pushes a branch and opens a pull request. Read [the gate](/docs/gate/) for every rule and [claims and leases](/docs/claims-and-leases/) for the records it checks.

We ship the gate first because its records are what the rest is built on. A claim says who did which task. A merge says the work landed. Those are exactly what **honey** (a contributor's standing in the project, built from real merged work) is measured from.

## Honey: Nectar and Quorum

**Honey** has two halves, and both are on `main` but not in a release yet: `waggle nectar` scores Nectar, and `waggle vote` and `waggle quorum` run Quorum rounds. The release records Nectar reads, and `waggle nectar backfill`, are in v0.1.1.

**Nectar** is reputation earned from merged work that lasts. A commit counts only if it survives 30 days on `main` without being reverted. Nectar is attributed through the release record written when the claim is given back, so the record of who held the task is what credits them. If the work is reverted, the credit goes with it. [Nectar](/docs/nectar/) has the rules.

**Quorum** is how the project decides what gets picked up next. Roadmap and issue priority are set by quadratic voting: each person's weight is the square root of their Nectar, so a long record counts for more without letting one big contributor drown out everyone else. It exists so that people who have shipped work that sticks get a say in what the project builds next. Ballots are signed with SSH keys, and anyone with a clone can recompute a closed round's tally. [Quorum](/docs/quorum/) has the commands.

The **Master Keeper veto** sits on top of Quorum. The Master Keeper is a role, not a person: the project owner (the Head Keeper today), holding the veto over Quorum results. `waggle quorum veto` removes one item or rejects a whole round, and must cite a decision in the decisions log. A veto never changes a score. Voting sets priority; it does not remove the owner's last word.

## Built today and coming

| Piece | What it is | Status |
| --- | --- | --- |
| Claims and path leases | A per-task record in git refs of which directories a task may change | Built |
| The merge gate | A required check: commits stay inside the lease, bind to a task, protect control paths | Built |
| Approvers and the Head Keeper | Listed keepers sign off on protected paths; the Head Keeper alone passes changes to the hive's steering files | Built |
| Decisions log | An append-only record of project decisions, checked by the gate, written with `waggle decide` | Built |
| `waggle dispatch` | Claims a task, runs a worker in its own worktree, checks its changes against the claim | Built (runs Claude Code today) |
| Role templates | Queen, scout, forager, builder and guard instructions, written by `waggle init` | Built |
| Release records | `waggle release` records who held each task; `--co` shares the credit; `waggle nectar backfill` fills in older merges | Built |
| Guidance for many keepers on one repo | Identity, contention, stale claims and review between keepers, in the [multi-keeper guide](https://github.com/waggle-sh/waggle/blob/main/docs/multi-keeper.md) | Built |
| `waggle force-release` | The Head Keeper clears a stuck or malformed claim, in two runs, with an audit record | On main |
| Signed Head Keeper approval | The Head Keeper's SSH signature on a PR's head commit passes Tier 3 paths | On main |
| Case-folded claim overlap | Claims whose directories differ only in letter case overlap; `waggle status` warns about older pairs | On main |
| Nectar | Reputation from merged work that survives 30 days on `main`, scored by `waggle nectar` | On main |
| Quorum | Quadratic voting on roadmap and issue priority, weight is the square root of Nectar | On main |
| Master Keeper veto | `waggle quorum veto`: a veto over Quorum results, held by the project owner (the Head Keeper today) | On main |
| Contributors without push access claiming for themselves | Today a maintainer writes the claim for a fork | Coming soon |
| Queens opening issues from keeper conversations | Today a queen can do this with the GitHub tools she already has; Waggle does not do it itself | Roadmap |

"Built" means in the latest release, v0.1.1. "On main" means merged to `main` but not in a release yet; build from source to use it. "Coming soon" means the next steps on a part that already works in a smaller form. "Roadmap" means designed on paper, with a plan and no code. Dates are not promised.

## Where to go next

- New to Waggle? The [quick start](/docs/quick-start/) takes a repo to a required check in about ten minutes.
- Want the reasons behind it? [Why Waggle](/docs/why-waggle/) answers the questions teams ask first.
- Unsure what a term means? The [glossary](/docs/glossary/) has every one.
