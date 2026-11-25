<!-- SPDX-License-Identifier: Apache-2.0 -->
# Waggle

Waggle is a system for many people, each running their own agent team, to work on one repository without colliding. Coding speed is rarely the hard part any more. Review time, merge coordination and keeping agents inside the paths they were given are. Waggle handles those, and it speaks in bees on purpose.

The **hive** is the shared repository and the rules every agent works under. It has people and agents in clear roles:

- **Keepers** are the humans. Each keeper owns a plan and the agents that carry it out. The **Head Keeper** is the keeper who owns the product direction and can approve changes to the hive's own steering files.
- **Queens** are the coordinating agents a keeper talks to. A queen agrees the task with the keeper, writes the **claim** (the record of who is working on what, and which directories the task may change) and starts the workers.
- **Foragers** reproduce bugs, write the failing test and map which directories a task needs.
- **Builders** implement the task inside their claim.
- **Guards** review the change, ideally on a different model than the builder used, and end with a pass or a must-fix.
- **Scouts** plan ahead, write the task brief and keep the roadmap and specs.

Every task has one claim at a time, written before the work starts. Two claims that overlap are refused when the second is written, and every commit is checked against its own task's claim before it can merge. That holds for one keeper with a few agents, for a company with several teams on one private repo, and for an open source project whose outside contributors bring their own agents. Each keeper runs their own agents on their own plan, so nobody donates tokens or shares agent credentials.

On top of those records sits **honey**, a contributor's standing in the project, built from real merged work. It has two halves. **Nectar** is reputation earned from work that survives 30 days on `main` without being reverted, credited through the claim. **Quorum** is how the project picks what to build next, by quadratic voting where each person's weight is the square root of their Nectar. The **Master Keeper** is the name for the owner's veto over a Quorum result. Today the Head Keeper holds it, through `waggle quorum veto`.

The **gate** is a required GitHub check named `verify-leases-and-tiers`, and it is the first piece of Waggle you can use today. On a repo where it is a required check and PRs are required, it fails a PR whose commits change paths outside the claim of the task each commit names, or whose task's claim does not name the PR's branch. It works with any agent that pushes a branch and opens a PR. The gate's records, who held which task and what merged, are what Nectar is measured from.

## Status

"Built" means in the latest release, v0.1.1. "On main" means merged to `main` but not in a release yet: the install script does not get it, so build from source to use it. "Roadmap" means designed on paper, with a plan and no code. "Coming soon" means the next steps on a part that already works in a smaller form. Dates are not promised. Waggle is pre-1.0.

| Piece | Status |
| --- | --- |
| Claims and path leases, the merge gate, approvers and the Head Keeper, the decisions log (`waggle decide`), role templates | Built |
| `waggle dispatch` (claims a task, runs a worker in its own worktree, checks its changes against the claim) | Built, runs Claude Code today |
| Release records (`waggle release` records who held each task, and `--co` shares the credit) and `waggle nectar backfill` | Built |
| Guidance for many keepers on one repo | Built: the [multi-keeper guide](docs/multi-keeper.md) covers identity, contention, stale claims and review between keepers |
| `waggle force-release` (the Head Keeper clears a stuck or malformed claim, with an audit record) | On main |
| Signed Head Keeper approval (an SSH signature on the PR head commit passes Tier 3 paths) | On main |
| Claim overlap compared with letter case folded, and a `waggle status` warning for older claims that differ only in case | On main |
| Nectar scoring: [`waggle nectar`](docs/nectar.md), `nectar explain`, and receipts written after a merge by `nectar gate-receipt` | On main |
| Quorum: [`waggle vote` and `waggle quorum`](docs/quorum.md) | On main |
| Master Keeper veto | On main, as `waggle quorum veto`, held by the Head Keeper |
| Contributors without push access claiming for themselves | Coming soon. Today a maintainer writes the claim for a fork |
| Queens opening issues from keeper conversations | Roadmap. Today a queen can use the GitHub tools she already has |

The [gate spec](docs/gate.md) describes what the gate code does today. The recipes have not been run end to end with each tool.

## Documentation

Start at [getwaggle.dev](https://getwaggle.dev) and its [docs](https://getwaggle.dev/docs). The [vision page](https://getwaggle.dev/docs/hive/) lays out the whole hive. In this repo:

- [Gate spec](docs/gate.md)
- [Several keepers on one repo](docs/multi-keeper.md)
- [Nectar: the reputation score](docs/nectar.md) and [Quorum: priority rounds](docs/quorum.md) (on main, not in a release yet)
- [Recipes](docs/recipes/README.md)
- [Gate walkthrough: a runnable pass and fail, no GitHub needed](examples/gate-walkthrough/README.md)
- [Contributing: product layer, repo records and the glossary](CONTRIBUTING.md)

## Quick start

Install the `waggle` CLI (macOS or Linux, amd64 or arm64):

```sh
curl -fsSL https://raw.githubusercontent.com/waggle-sh/waggle/main/install.sh | sh
waggle version
```

The script downloads the latest release archive for your OS and CPU, checks it against the release's `SHA256SUMS`, and installs `waggle` to `~/.local/bin` (or `/usr/local/bin` when that is not writable). Set `WAGGLE_VERSION=0.1.0` to pin a release or `WAGGLE_INSTALL_DIR` to choose the directory. With Go installed, `go install github.com/waggle-sh/waggle/cmd/waggle@latest` works too; that build prints a `-dev` version, since only release builds stamp it. On Windows, download the `.zip` from the [releases page](https://github.com/waggle-sh/waggle/releases).

Then gate your own GitHub repo:

```sh
cd your-repo
waggle init --gate-only     # writes the gate workflow, a small config and DECISIONS.md
git add .github .waggle && git commit -m "Add Waggle gate" && git push
# on GitHub: require the check verify-leases-and-tiers, and PRs, on your default branch
waggle doctor               # checks claim refs can be pushed and the check is required
waggle claim T-001 --dirs src --gate light --branch t-001 --owner <your-github-login>
```

Now a PR from branch `t-001` whose commits start with `T-001: ` passes while they change only `src/`, and fails once one touches anything else. The install commit changes `.github/workflows/` and `.waggle/`, which the gate always protects, so if you push it as a PR it fails its own gate and needs an admin merge. [Install](#install) has the details, and [Writing a claim](#writing-a-claim) covers the flags.

## What the gate checks

Each check links the [gate spec](docs/gate.md) section that cites the code behind it.

1. **Each commit names its task, and the task's claim names this PR's branch.** A commit names its task by a `T-<id>: ` subject, a `Waggle-Ticket:` trailer, or a `waggle/T-<id>` or `t-<id>` branch; a commit naming none, or two different tasks, fails. The task is chosen by the commit's author, so the claim's branch list is what ties it to the PR: a commit fails unless its task's claim lists the PR's head branch, and a claim that names no branch fails every commit that names its task whenever the PR's head branch is known ([binding](docs/gate.md#2-binding-a-commit-to-a-task), [branch check](docs/gate.md#3-the-claims-branch-check)).
2. **Each commit stays inside its task's lease.** The lease is the directory list in the task's claim. Every commit is checked, not only the final diff, with exact case and no new symlink or submodule; only a merge from the base branch is accepted ([lease check](docs/gate.md#4-the-per-commit-lease-check)). The gate does not check who wrote a claim or when, so a person or the orchestrator writes it, with its branch list, before the work, never from the agent's credentials.
3. **Protected paths.** Control paths (`.github/workflows/`, `.gitlab-ci.yml`, `.devcontainer/`, `.waggle/`, `go.mod`, `go.sum`, `go.work`, `go.work.sum` and `vendor/`) fail whatever the claim says. An approval does not pass them, except that the Head Keeper (the first entry of `gate.approvers`) can approve a change under `.waggle/hive/`; an admin can bypass the check. On main, not in a release yet, the Head Keeper's SSH signature on the PR head commit also passes every Tier 3 path, once `.waggle/control/allowed_signers` on the base branch holds the Head Keeper's key ([signed approval](docs/gate.md#signed-head-keeper-approval)). Paths your repo lists in `gate.protected_paths` fail too, unless a listed approver approved the PR at its exact head commit (by review, or by a `waggle-approve <sha>` comment) and someone with Write re-ran the failed check ([approvers](docs/gate.md#6-approvers)), or an admin bypasses ([protected paths](docs/gate.md#5-risk-tiers-and-protected-paths)). A forged check can also merge them ([limits](#what-the-gate-does-not-do)).
4. **Intent Packet trailers.** `Waggle-Ticket`, `Waggle-Risk-Tier` and `Waggle-Verdict` in the last paragraph of the PR description are compared with the commits and changed paths, as warnings only. They never fail the gate ([trailers](docs/gate.md#7-trailers)).
5. **Decisions log.** `.waggle/hive/DECISIONS.md` accepts only appended, well-formed entries, judged on the PR's net change. No claim can lease `.waggle/`, so an append also needs the Head Keeper's approval at the PR's exact head commit and a re-run of the failed check, or an admin merge, unless the check is forged. The approval never passes an edit to an existing entry ([limits](#what-the-gate-does-not-do); [decisions log](docs/gate.md#8-the-decisions-log)).

## What the gate does not do

It does not schedule or run agents, and it is not a sandbox. The limits below are described in the [gate spec](docs/gate.md#10-limits-and-known-bypasses).

- The required check can be forged. A workflow on any branch that declares write permissions can delete the gate's failing run and post a passing check of the same name, and the PR then merges with no admin rights. A token that can both delete Actions runs and write commit statuses can do the same with a commit status and no workflow file. This was shown with test workflows, not with the real gate. Three things help, and none is complete alone. Binding the check to GitHub Actions closes the status route only. Limit agent credentials to Contents and Pull requests. A token with Contents write alone was refused a new workflow file (Pull requests write added was not tested), but it pushed a copy of one already on another branch, and the copy ran, so this holds only while no such workflow exists on any branch. A required review blocks a merge with no approval, but it does not protect the check: a check can be forged after the approval, with no new push (that order was not run). See [L11](docs/gate.md#10-limits-and-known-bypasses).
- It binds only on PRs that must pass `verify-leases-and-tiers` as a required check on a branch that requires PRs. A merge that skips PRs skips the gate.
- Anyone who can push to a branch a claim names can add commits under that claim.
- Whoever can push claim refs can write a lease and its branch list, an agent included if it has your git credentials.
- A PR that edits the gate workflow runs its own copy. Code-owner review on `.github/workflows/**` mitigates this; it does not fix it.
- An admin bypass merges without the gate.
- Once the required check is bound to GitHub Actions, a commit status from another app or person did not count, on a test rule. A check from a workflow in the repository still counts, since it comes from GitHub Actions too.

## Install

1. Get `waggle`. The [Quick start](#quick-start) script installs a release: tagged releases carry `waggle_<version>_<os>_<arch>` archives and a `SHA256SUMS` file, and the script checks the download against its `SHA256SUMS` line. To build from source instead, use a clean clone of a pushed commit, checked out at `<commit-sha>`: `go build -o <path outside the clone>/waggle ./cmd/waggle`. Run `init` and `upgrade` from that build. `waggle version` prints the version you have.
2. In your repo, run `waggle init --gate-only`. It writes `.github/workflows/waggle-gate.yml`, `.waggle/control/config.yaml` and `.waggle/hive/DECISIONS.md`, and refuses a forge other than GitHub.
3. Commit those files. The install PR changes the workflow and `.waggle/`, so it fails its own gate. If a rule already requires the check, it needs an admin merge.
4. Require the check `verify-leases-and-tiers`, and PRs, on your default branch.
5. Run `waggle doctor`. It checks that this clone can push claim refs and that a rule requires the check with up-to-date branches. It does not check that PRs are required, required reviews, code-owner review, or which app may post the check, and the required check can still be forged ([L11](docs/gate.md#10-limits-and-known-bypasses)).

The full workflow is in the [gate spec](docs/gate.md#13-the-calling-workflow). A pin is written `<commit-sha>`; for a release tag, read the commit with `git rev-parse vX.Y.Z^{commit}`.

## Writing a claim

```
Usage: waggle claim <task> --dirs <a,b> --gate light|full [--branch <name>]... [--ttl <duration>] [--owner <id>] [--base <rev>] [--repo <dir>] [--json]
Usage: waggle release <task> [--branch <name>] [--unmerged] [--co <id>]... [--owner <id>] [--repo <dir>] [--json]
Usage: waggle force-release <task> | --ref <ref> [--oid <id> --reason <text>] [--owner <id>] [--repo <dir>] [--json]
```

These are the usage lines `waggle claim -h`, `waggle release -h` and `waggle force-release -h` print (`cmd/waggle/claim.go:181, 246`, `cmd/waggle/force.go:40`).

- `--owner` is required in a repo that has only the gate installed. Its help reads `claim owner, usually your GitHub login; required without .waggle/local.yaml, else defaults to its keeper_id`; the file Waggle reads is `.waggle/control/local.yaml`.
- `--branch` names a PR branch the claim accepts; repeat it, up to 4. With none, a new claim records `t-<nnn>`. Given flags replace the stored list, and a same-owner renewal without flags keeps it. A re-claim after the claim expired starts a new list, so repeat every `--branch`.
- A claim write does not trigger the workflow: after the list changes, re-run the failed check.
- A claim with no branch fails with `<id>: claim names no branch; renew it with waggle claim --branch`. Renew it with `--branch` and re-run the check.
- A claim is refused when its directories overlap another active claim. On main, not in a release yet, that comparison folds ASCII letter case, so `Internal/Comb` overlaps `internal/comb`, as it does on a macOS or Windows checkout, and `waggle status` warns about two older claims that differ only in case. `waggle status` syncs claims once per run, and the claims table and the case warning read that one sync.
- `waggle release` deletes the claim and records who held the task, which Nectar reads. `--co <id>` names another keeper who shares the credit, up to 4.
- `waggle force-release` (on main, not in a release yet) clears a stuck or malformed claim. Only the first login in `gate.approvers`, read from main on the remote, can run it. The first run only prints what it would delete and the exact second command; the second, with `--oid` and `--reason`, deletes the ref if it has not moved and records who cleared it and why in `refs/waggle/audit/force-releases` ([multi-keeper guide](docs/multi-keeper.md#4-stale-claims-and-expiry)).
- A `light` claim fails on Tier 2 paths (`CLAUDE.md`, `AGENTS.md`, `.waggle/hive/`); use `full` for those ([gate spec](docs/gate.md#5-risk-tiers-and-protected-paths)). Claims must stay until the PR merges. After a squash or rebase merge, run `waggle release T-<id> --unmerged`.

## Recipes

How to run the gate under the agent you already use, in [docs/recipes](docs/recipes/README.md): [Claude Code agent teams](docs/recipes/claude-code-agent-teams.md), [GitHub Copilot cloud agent](docs/recipes/copilot-cloud-agent.md), and [gate-run receipts after a merge](docs/recipes/gate-run-receipts.md) (on main, not in a release yet).

## License
Waggle, the `waggle` CLI, is licensed under the Apache License 2.0 (`LICENSE`, `NOTICE`). The templates `waggle init` writes into your repository (everything from `internal/scaffold/templates/`: your `.waggle/` files and `.github/workflows/waggle-gate.yml`) are licensed under MIT-0 (`LICENSES/MIT-0.txt`): use, change and relicense them with no attribution. Copyright 2026 Level 19 Enterprises, LLC. Contributions are accepted under the Developer Certificate of Origin (see `CONTRIBUTING.md`).
