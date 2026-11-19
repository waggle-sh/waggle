<!-- SPDX-License-Identifier: Apache-2.0 -->
# Claims and leases

A **claim** is the record that a task is being worked on. A **lease** is the list of directories in it. The gate compares every commit in a PR with the claim of the task that commit names.

## What a claim holds

A claim is a git ref, `refs/waggle/claims/ticket-<n>`, in the repo's remote. It holds:

- the task id (`T-<id>`),
- an owner, usually a GitHub login,
- an expiry,
- a gate level, `light` or `full`,
- the lease: up to `leases.max_directories_per_claim` directories (5 by default),
- the branch list: up to 4 PR branches the claim accepts.

The gate does not check who wrote a claim or when. A person or the orchestrator writes it, with its branch list, before the work, and never from the agent's credentials: anyone who can push `refs/waggle/claims/*` can write a lease.

## Writing and giving back a claim

```
waggle claim <task> --dirs <a,b> --gate light|full [--branch <name>]... [--ttl <duration>] [--owner <id>] [--base <rev>] [--repo <dir>] [--json]
waggle release <task> [--branch <name>] [--unmerged] [--owner <id>] [--repo <dir>] [--json]
```

- `--owner` is required in a repo that has only the gate installed. With a full install it defaults to `keeper_id` in `.waggle/control/local.yaml`.
- `--branch` names a PR branch the claim accepts; repeat it, up to 4. With none, a new claim records `t-<nnn>`. Given flags replace the stored list; a same-owner renewal without flags keeps it. A re-claim after the claim expired starts a new list, so repeat every `--branch`.
- `--ttl` sets the lifetime in whole minutes. The default is `leases.default_ttl_minutes` (120). An expired claim is accepted by the gate with a warning, not a failure.
- A claim write does not trigger the workflow. After the branch list changes, re-run the failed check.
- Claims must stay until the PR merges. After a squash or rebase merge, run `waggle release T-<id> --unmerged`.

## How a commit names its task

A commit binds to a task in one of three ways. The first one present wins, and two that name different tasks fail with `ticket conflict`.

1. A subject that starts `T-<id>: `.
2. A `Waggle-Ticket: T-<id>` trailer.
3. A PR branch named `waggle/T-<id>` or `t-<id>`, with an optional `-suffix`.

A commit that names no task fails. The task is chosen by the commit's author, which is why the claim's branch list matters.

## What the gate checks against a claim

- **The branch.** The claim of the task a commit names must list the PR's head branch exactly. A claim with no branch fails every commit that names its task, with `claim names no branch; renew it with waggle claim --branch`. Renew it with `--branch` and re-run the check.
- **The lease.** Every commit is checked, not only the final diff. A path outside the lease fails, with exact case, and so does a new symlink or submodule. The only merge commit accepted is an update from the base branch.
- **Size.** A PR over 500 commits, or a commit that changes over 5000 paths, fails with `PR too large for the gate; split it`.
- **A missing claim** fails with `no claim for <id>; claims must stay until merge`.

## What a claim cannot lease

`waggle claim` refuses directories the gate would fail anyway:

- anything under `.waggle/`,
- Tier 3 control paths (`.github/workflows/`, `.gitlab-ci.yml`, `.devcontainer/`, `go.mod`, `go.sum`, `go.work`, `go.work.sum`, `vendor/`),
- paths listed in [`gate.protected_paths`](/docs/config/#gate),
- Tier 2 paths (`CLAUDE.md`, `AGENTS.md`, `.waggle/hive/`) under a `light` claim: use `--gate full` for those.

Protected paths and approvals are covered in the gate spec: [risk tiers and protected paths](/docs/gate/#5-risk-tiers-and-protected-paths) and [approvers](/docs/gate/#6-approvers).

## Who writes the claim

Never the agent that does the task. The [recipes](/docs/recipes/#who-writes-the-claim) say how to keep `waggle claim` and approver credentials away from agents.
