<!-- SPDX-License-Identifier: Apache-2.0 -->
# Recipes: running Waggle Gate under the agent you already use

Waggle is for many **keepers** (the humans who own work on a repo) and their agent teams on one repo. The gate is the piece you can use today. Each recipe says how one agent tool's output meets the gate. The gate itself is specified in [the gate spec](../gate.md). Every claim about another tool links the public page that says it; where a page does not say something, the recipe says it is an inference or leaves it out. Waggle has not tested these recipes end to end with each tool.

- [Claude Code agent teams](claude-code-agent-teams.md)
- [GitHub Copilot cloud agent](copilot-cloud-agent.md)
- [Gate-run receipts after a merge](gate-run-receipts.md)

## What every recipe assumes

- The gate is installed as [the README](../../README.md#install) describes, and `waggle doctor` passes.
- `verify-leases-and-tiers` is a required check and PRs are required on the default branch. Without both, the gate binds nothing ([L2](../gate.md#10-limits-and-known-bypasses)).
- Changes to `.github/workflows/**` need code-owner review, with the ruleset's "Require review from Code Owners" on, or an organization ruleset that requires the workflow. A PR that edits the gate workflow runs its own copy otherwise (L5).
- Bind the required check to GitHub Actions. Limit an agent's credentials to Contents and Pull requests, with no Workflows, Actions or Commit statuses permission, and keep any workflow you would not want an agent to copy off every branch. Require a person's approval, with stale approvals dismissed, if a person must be in the merge. Without an approval, a copied workflow or a wider credential can merge a forged check with no person. None of these is complete alone ([L11](../gate.md#10-limits-and-known-bypasses)). Keep "Allow GitHub Actions to create and approve pull requests" off.
- The install PR changes the workflow and `.waggle/`, so it fails its own gate. If a rule already requires the check, it needs an admin merge.
- Run `waggle init` and `waggle upgrade` from a build made at a pushed commit or from a release, so the pinned Action ref exists. A build from an unpushed commit renders a ref that fails at job setup (L10).

## Who writes the claim

The keeper's **queen** (the coordinating agent a keeper talks to), or another trusted coordinator, writes the claim and its branch list, with the keeper in control; a person writing it is the fallback when there is no queen. Never the worker (a **forager** or **builder**) that does the task, and never from credentials the agent holds: anyone who can push `refs/waggle/claims/*` can write a lease and a branch list, so an agent with your git credentials can write its own (L3). Keep approver credentials and `waggle claim` away from agents, and credentials that hold the Workflows, Actions or Commit statuses permission, or a classic `repo` token (L11). A narrower token can still push claim refs (L3).

Write the claim before the work and before the PR's first gate run:

```
waggle claim T-<id> --dirs <a,b> --gate light|full --owner <your GitHub login> --branch <the PR branch>
```

- `--owner` is required in a repo that has only the gate installed.
- `--branch` is repeatable, up to 4. Every claim must name the PR branch its commits arrive on. Given flags replace the stored list; a same-owner renewal without flags keeps it. A re-claim after the claim expired starts a new list, so repeat every `--branch` (`internal/dispatch/claim.go:58-65`).
- A claim write does not trigger the workflow (`internal/scaffold/templates/waggle-gate.yml.tmpl:5-9`). After you change a list, re-run the failed check, or push. A review alone does not clear a failed run, and someone with Write does the re-run ([approvers](../gate.md#6-approvers)).
- After a squash or rebase merge, release with `waggle release T-<id> --unmerged`. Claims must stay until the PR merges.

## How commits bind

A commit binds by a `T-<id>: ` subject prefix, a `Waggle-Ticket:` trailer, or a PR branch named `waggle/T-<id>` or `t-<id>` (`cmd/waggle/gate_commits.go:33-36, 260-310`). When one PR carries commits of several tasks, each task's claim must list that PR's branch, and the branch name must match neither `waggle/T-<id>` nor `t-<id>`, or every commit naming another task fails with `ticket conflict` (`cmd/waggle/gate_commits.go:279-298`).

## Troubleshooting

- `<id>: claim names no branch; renew it with waggle claim --branch`: the claim has no branch list. Renew it with `--branch <the PR branch>`, then re-run the check (`cmd/waggle/gate_commits.go:355-358`). An older build that renews a claim drops the list (L10), so renew with a build that records branches. The L-numbers in these recipes are the limits in [the gate spec](../gate.md#10-limits-and-known-bypasses).
- `<id>: claim does not name this pull request's branch`: add the PR's head branch to the claim with `--branch` and re-run the check (`cmd/waggle/gate_commits.go:368`).
- `commit <12 hex> "<subject>": commit has no T-<id>: prefix, Waggle-Ticket: trailer or ticket branch`: that commit names no task (`cmd/waggle/gate_commits.go:305`).
