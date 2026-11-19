<!-- SPDX-License-Identifier: Apache-2.0 -->
# Waggle Gate

Waggle Gate is a GitHub merge check for repos where AI agents write code. On a repo where it is a required check and PRs are required, it fails a PR whose commits change paths outside the claim of the task each commit names, or whose task's claim does not name the PR's branch.

You give each task a claim: the directories it may change and the branches its PR may come from. The gate reads the claims from git and compares them with the PR's commits.

## What it checks

Each check links the [gate spec](docs/gate.md) section that cites the code behind it.

1. **Each commit names its task, and the task's claim names this PR's branch.** A commit names its task by a `T-<id>: ` subject, a `Waggle-Ticket:` trailer, or a `waggle/T-<id>` or `t-<id>` branch; a commit naming none, or two different tasks, fails. The task is chosen by the commit's author, so the claim's branch list is what ties it to the PR: a commit fails unless its task's claim lists the PR's head branch, and a claim that names no branch fails every commit that names its task whenever the PR's head branch is known ([binding](docs/gate.md#2-binding-a-commit-to-a-task), [branch check](docs/gate.md#3-the-claims-branch-check)).
2. **Each commit stays inside its task's lease.** The lease is the directory list in the task's claim. Every commit is checked, not only the final diff, with exact case and no new symlink or submodule; only a merge from the base branch is accepted ([lease check](docs/gate.md#4-the-per-commit-lease-check)). The gate does not check who wrote a claim or when, so a person or the orchestrator writes it, with its branch list, before the work, never from the agent's credentials.
3. **Protected paths.** Control paths (`.github/workflows/`, `.gitlab-ci.yml`, `.devcontainer/`, `.waggle/`, `go.mod`, `go.sum`, `go.work`, `go.work.sum` and `vendor/`) fail whatever the claim says. An approval does not pass them, except that the Head Keeper (the first entry of `gate.approvers`) can approve a change under `.waggle/hive/`; an admin can bypass the check. Paths your repo lists in `gate.protected_paths` fail too, unless a listed approver approved the PR at its exact head commit (by review, or by a `waggle-approve <sha>` comment) and someone with Write re-ran the failed check ([approvers](docs/gate.md#6-approvers)), or an admin bypasses ([protected paths](docs/gate.md#5-risk-tiers-and-protected-paths)). A forged check can also merge them ([limits](#what-it-does-not-do)).
4. **Intent Packet trailers.** `Waggle-Ticket`, `Waggle-Risk-Tier` and `Waggle-Verdict` in the last paragraph of the PR description are compared with the commits and changed paths, as warnings only. They never fail the gate ([trailers](docs/gate.md#7-trailers)).
5. **Decisions log.** `.waggle/hive/DECISIONS.md` accepts only appended, well-formed entries, judged on the PR's net change. No claim can lease `.waggle/`, so an append also needs the Head Keeper's approval at the PR's exact head commit and a re-run of the failed check, or an admin merge, unless the check is forged. The approval never passes an edit to an existing entry ([limits](#what-it-does-not-do); [decisions log](docs/gate.md#8-the-decisions-log)).

## What it does not do

It does not schedule or run agents, and it is not a sandbox. The limits below are described in the [gate spec](docs/gate.md#10-limits-and-known-bypasses).

- The required check can be forged. A workflow on any branch that declares write permissions can delete the gate's failing run and post a passing check of the same name, and the PR then merges with no admin rights. A token that can both delete Actions runs and write commit statuses can do the same with a commit status and no workflow file. This was shown with test workflows, not with the real gate. Three things help, and none is complete alone. Binding the check to GitHub Actions closes the status route only. Limit agent credentials to Contents and Pull requests. A token with Contents write alone was refused a new workflow file (Pull requests write added was not tested), but it pushed a copy of one already on another branch, and the copy ran, so this holds only while no such workflow exists on any branch. A required review blocks a merge with no approval, but it does not protect the check: a check can be forged after the approval, with no new push (that order was not run). See [L11](docs/gate.md#10-limits-and-known-bypasses).
- It binds only on PRs that must pass `verify-leases-and-tiers` as a required check on a branch that requires PRs. A merge that skips PRs skips the gate.
- Anyone who can push to a branch a claim names can add commits under that claim.
- Whoever can push claim refs can write a lease and its branch list, an agent included if it has your git credentials.
- A PR that edits the gate workflow runs its own copy. Code-owner review on `.github/workflows/**` mitigates this; it does not fix it.
- An admin bypass merges without the gate.
- Once the required check is bound to GitHub Actions, a commit status from another app or person did not count, on a test rule. A check from a workflow in the repository still counts, since it comes from GitHub Actions too.

## Install

1. Build `waggle` from a clean clone of a pushed commit, checked out at `<commit-sha>`: `go build -o <path outside the clone>/waggle ./cmd/waggle`. Run `init` and `upgrade` from that build. Tagged releases carry `waggle_<version>_<os>_<arch>` archives and a `SHA256SUMS` file; check a download against its `SHA256SUMS` line.
2. In your repo, run `waggle init --gate-only`. It writes `.github/workflows/waggle-gate.yml`, `.waggle/control/config.yaml` and `.waggle/hive/DECISIONS.md`, and refuses a forge other than GitHub.
3. Commit those files. The install PR changes the workflow and `.waggle/`, so it fails its own gate. If a rule already requires the check, it needs an admin merge.
4. Require the check `verify-leases-and-tiers`, and PRs, on your default branch.
5. Run `waggle doctor`. It checks that this clone can push claim refs and that a rule requires the check with up-to-date branches. It does not check that PRs are required, required reviews, code-owner review, or which app may post the check, and the required check can still be forged ([L11](docs/gate.md#10-limits-and-known-bypasses)).

The full workflow is in the [gate spec](docs/gate.md#13-the-calling-workflow). A pin is written `<commit-sha>`; for a release tag, read the commit with `git rev-parse vX.Y.Z^{commit}`.

## Writing a claim

```
Usage: waggle claim <task> --dirs <a,b> --gate light|full [--branch <name>]... [--ttl <duration>] [--owner <id>] [--base <rev>] [--repo <dir>] [--json]
Usage: waggle release <task> [--branch <name>] [--unmerged] [--owner <id>] [--repo <dir>] [--json]
```

These are the usage lines `waggle claim -h` and `waggle release -h` print (`cmd/waggle/claim.go:178, 243`).

- `--owner` is required in a repo that has only the gate installed. Its help reads `claim owner, usually your GitHub login; required without .waggle/local.yaml, else defaults to its keeper_id`; the file Waggle reads is `.waggle/control/local.yaml`.
- `--branch` names a PR branch the claim accepts; repeat it, up to 4. With none, a new claim records `t-<nnn>`. Given flags replace the stored list, and a same-owner renewal without flags keeps it. A re-claim after the claim expired starts a new list, so repeat every `--branch`.
- A claim write does not trigger the workflow: after the list changes, re-run the failed check.
- A claim with no branch fails with `<id>: claim names no branch; renew it with waggle claim --branch`. Renew it with `--branch` and re-run the check.
- A `light` claim fails on Tier 2 paths (`CLAUDE.md`, `AGENTS.md`, `.waggle/hive/`); use `full` for those ([gate spec](docs/gate.md#5-risk-tiers-and-protected-paths)). Claims must stay until the PR merges. After a squash or rebase merge, run `waggle release T-<id> --unmerged`.

## Recipes

How to run the gate under the agent you already use, in [docs/recipes](docs/recipes/README.md): [Claude Code agent teams](docs/recipes/claude-code-agent-teams.md) and [GitHub Copilot cloud agent](docs/recipes/copilot-cloud-agent.md).

## Status

Pre-1.0. The [gate spec](docs/gate.md) describes what the code does today, and the recipes have not been run end to end with each tool.

## Documentation

- [Gate spec](docs/gate.md)
- [Recipes](docs/recipes/README.md)
- [Contributing: product layer, repo records and the glossary](CONTRIBUTING.md)

## License
Waggle, the `waggle` CLI, is licensed under the Apache License 2.0 (`LICENSE`, `NOTICE`). The templates `waggle init` writes into your repository (everything from `internal/scaffold/templates/`: your `.waggle/` files and `.github/workflows/waggle-gate.yml`) are licensed under MIT-0 (`LICENSES/MIT-0.txt`): use, change and relicense them with no attribution. Copyright 2026 Level 19 Enterprises, LLC. Contributions are accepted under the Developer Certificate of Origin (see `CONTRIBUTING.md`).
