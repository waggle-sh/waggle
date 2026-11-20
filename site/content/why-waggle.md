<!-- SPDX-License-Identifier: Apache-2.0 -->
# Why Waggle

Waggle lets many **keepers** (the humans who own work on a repo), each with their own team of AI agents, work on one repository without stepping on each other. One person with one agent can keep the plan in their head. Five people with five agents cannot.

The full vision has three parts. **Claims and leases** say who is working on what. The **gate** is a merge check that holds every pull request to its claim. **Honey** is how contributors earn a say: **Nectar** (reputation from merged work that survives on `main`) and **Quorum** (weighted voting on what gets picked up next, with a Master Keeper veto). The gate is the first piece you can use today. Nectar, Quorum and the multi-keeper parts are on the roadmap. They are not built yet, and this page marks each one as it comes up.

## The problem

Agents are fast and they do not talk to each other. Put a few of them on the same repo and the same things keep going wrong:

- **Overlapping edits.** Two agents change the same files. One overwrites the other, or the two pull requests conflict and someone untangles them by hand.
- **The same work, twice.** Nothing records that a task is taken, so a second contributor's agent starts it too.
- **Pull requests that wander.** An agent asked to fix the parser also "tidies" the CI workflow and a lockfile. The diff is large, and the stray change is easy to miss in review.
- **Nobody knows who is on what.** Branches pile up. To find out which agent is working on which task, a person reads the diffs and asks around.

Waggle answers this with three pieces you can use today. A **claim** says which task is being worked on, which directories it may change (its **lease**), and which branches its pull request may come from. It is written before the work starts by the keeper's **queen** (the coordinating agent a keeper talks to), or another trusted coordinator, never by the **forager** (a worker agent) doing the work. The **gate** is a required GitHub check that fails a pull request whose commits leave the lease. And a keeper on the repo's approver list signs off on the paths you protect. The rest of this page answers the questions teams ask first.

## Meet the hive

Waggle borrows its vocabulary from a beehive. Each term means one plain thing:

- **Hive:** the repo's shared Waggle setup, with the keepers and agents working in it. Its records live under `.waggle/`.
- **Keeper:** a human who owns work on the repo. The **Head Keeper** is the first login in `gate.approvers`.
- **Queen:** the coordinating agent a keeper talks to. She does not write code. She writes claims, and will open issues from the conversation (coming soon).
- **Forager, builder, scout:** worker agents. A forager does triage, repro and lease mapping, a builder implements a task, and a scout writes briefs and specs.
- **Guard:** a reviewer agent that checks a builder's work, on a different model than the builder's.
- **Honey:** Nectar plus Quorum (roadmap).
- **Walt:** our bee mascot.

## How is this different from branch protection or CODEOWNERS?

Branch protection and CODEOWNERS are rules per path. They apply the same way to every pull request: "changes to `billing/` need a review from the billing team." They do not know what a pull request was supposed to do.

A claim is per task. It says "T-123 may change `internal/parser` and `docs`, from branch `t-123`." The gate checks every commit in the pull request against the claim of the task that commit names, not only the final diff. So it catches an agent that drifts into a path nobody thought to protect, which no per-path rule can do.

They work together. The gate needs branch protection (or a ruleset) to require pull requests and the `verify-leases-and-tiers` check, or it binds nothing. Code-owner review on `.github/workflows/**` is one of the settings the [recipes](/docs/recipes/#what-every-recipe-assumes) ask for, so a pull request cannot quietly edit the gate's own workflow.

## Who writes the claims?

The keeper's queen, or another trusted coordinator. Never the forager or builder doing the work.

In the full Waggle setup, each keeper talks with their own queen. The keeper and the queen agree on the task. Today the queen writes the claim and starts a worker with `waggle dispatch`, which records the claim before the worker runs. **Coming soon:** the queen opening GitHub issues from the keeper's conversation. That is not built yet, and Waggle does not open issues today. The worker never writes its own claim.

People can also skip the queen entirely. Open an issue by hand, write the claim yourself with `waggle claim`, and point any agent at it:

```
waggle claim T-123 --dirs internal/parser,docs --gate light --owner <your GitHub login> --branch t-123
```

One honest caveat: the gate does not check who wrote a claim. Anyone who can push `refs/waggle/claims/*` can write one. So keep `waggle claim` and claim-writing credentials away from your worker agents. The [recipes](/docs/recipes/#who-writes-the-claim) show how for each tool.

## Can two agents end up on the same work?

Not quietly. A task has one claim at a time. If someone else holds an unexpired claim on T-123, `waggle claim T-123` is refused and tells you who holds it and until when. A claim whose directories overlap another active claim is refused too, so two tasks cannot both lease `internal/parser`.

The overlap check runs when the claim is written. Two claims written at the same instant can both slip through. The gate is the backstop: every commit still has to stay inside its own task's lease to merge.

And because claims are git refs in your repo, everyone on the team reads the same list of who is on what. In a full install, `waggle status` prints the active claims.

## What happens when an agent edits outside its lane?

The pull request fails the required check. The gate posts a line against the stray path, such as:

```
T-123: path is outside the claim directories
```

It checks each commit, so a later commit that reverts the stray edit does not hide it. A pull request from a branch the claim does not list fails too, with `claim does not name this pull request's branch`, and a commit that names no task fails closed.

Some paths fail no matter what the claim says. Workflows under `.github/workflows/`, `.gitlab-ci.yml`, `.devcontainer/`, Waggle's own `.waggle/` config, and Go's `go.mod`, `go.sum` and `vendor/` are hardcoded control paths. No claim can lease them and no approval passes them; only an admin merge does.

If you run workers through `waggle dispatch`, it also checks the worker's changes against the claim when the run ends, so you hear about a stray edit early. Then you choose: widen the claim if the task needs it, or have the agent back the change out.

## What does a keeper approve?

The paths you decide matter. You list them in `gate.protected_paths` in `.waggle/control/config.yaml`, and you list your approvers' GitHub logins in `gate.approvers`. A change to a protected path fails the gate until a listed approver approves the pull request at its exact head commit and someone with write access re-runs the check. A push after the approval makes it stale.

GitHub does not let you approve your own pull request, so an approver who opened it posts a `waggle-approve <head commit SHA>` comment instead.

The first login in `gate.approvers` is the Head Keeper, the keeper who leads the hive. Only the Head Keeper's approval passes a change under `.waggle/hive/`, the shared files that steer every later agent. The gate reads both lists from the base branch, so a pull request cannot add its author as an approver.

An approval never passes the hardcoded control paths above. Those need an admin merge, on purpose. The full rules are in [Approvers](/docs/gate/#6-approvers).

## Does it work with Claude Code and other agents?

Yes. The gate does not run, prompt or schedule agents. It reads commits and claims, so it works with any agent that pushes a branch and opens a pull request. An agent only has to name its task in each commit, with a `T-123: ` subject prefix, a `Waggle-Ticket: T-123` trailer, or a branch named `t-123`.

There are recipes for [Claude Code agent teams](/docs/recipes/claude-code-agent-teams/) and the [GitHub Copilot cloud agent](/docs/recipes/copilot-cloud-agent/). We have not yet run each recipe end to end with each tool, and the recipes say so.

`waggle dispatch`, the runner the queen uses to start foragers and builders, runs Claude Code today.

A few limits to know: Waggle supports GitHub only, the gate does not run in GitHub's merge queue, and a merge that skips pull requests skips the gate.

## What about contributors who can't push to the repo?

Outside contributors, including anyone opening a pull request from a fork, cannot write claim refs to your repo. Today a maintainer writes the claim for them, naming the fork's branch as `<owner>/<repo>:<branch>`. The gate always reads claims from your repo, never from the fork.

**Coming soon:** a way for contributors without push access to request and hold their own claims, and guidance for many keepers sharing one repo. These multi-keeper parts are on the roadmap and not built yet.

## What are Nectar and Quorum?

**Coming soon.** Nectar and Quorum are designed but not built. They are the other half of the vision, and they follow the gate.

- **Nectar** is reputation earned from merged work that survives on `main`. The claim and merge records the gate keeps are what Nectar will score.
- **Quorum** is weighted voting on what gets picked up next. A contributor's weight grows with their Nectar, and the Master Keeper holds a veto.

Together they are **honey**: the way a contributor, human or agent team, earns a say by shipping work that sticks. It matters most for open-source maintainers and for companies with many teams on one repo.

## Is it open source?

The `waggle` CLI and the gate are licensed Apache-2.0, and the templates `waggle init` writes into your repo are MIT-0, so you can use and change them freely.

These docs describe what the code does today, and the [gate spec](/docs/gate/) cites the code behind every rule.

## What Waggle does not do

It is not a sandbox and it does not stop an agent from writing a file. It stops the change from merging. The required check can be forged by a workflow or token with the right permissions, and an admin bypass merges without it. Every known limit is written down, with the tests that show it, in [the gate spec](/docs/gate/#10-limits-and-known-bypasses).

Ready to try it? The [quick start](/docs/quick-start/) takes a repo to a required check in about ten minutes.
