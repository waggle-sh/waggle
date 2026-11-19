<!-- SPDX-License-Identifier: Apache-2.0 -->
# Recipe: GitHub Copilot cloud agent

Read [what every recipe assumes](README.md) first. Not run end to end by Waggle. The L-numbers are the limits in [the gate spec](../gate.md#10-limits-and-known-bypasses). GitHub's current name for the product is Copilot cloud agent (formerly Copilot coding agent).

## What the tool does

- "Copilot can only work on one branch at a time and can open one pull request per task" ([about the agent](https://docs.github.com/en/copilot/concepts/agents/coding-agent/about-coding-agent)).
- It names its branch itself, for example `copilot/add-theme-switcher` ([changelog](https://github.blog/changelog/2025-10-16-copilot-coding-agent-uses-better-branch-names-and-pull-request-titles)). That is an example, not a stated rule.
- "By default, GitHub Actions workflows will not run automatically when Copilot pushes changes to a pull request"; a person uses "Approve and run workflows" ([review Copilot PRs](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/review-copilot-prs)).
- It reads repository instructions: "Repository-wide instructions (using the `.github/copilot-instructions.md` file)", "Path-specific instructions (using `.github/instructions/**/*.instructions.md` files)" and "Agent instructions (using `AGENTS.md`, `CLAUDE.md` or `GEMINI.md` files)" ([custom instructions support](https://docs.github.com/en/copilot/reference/custom-instructions-support)).

## How commits bind

Copilot's branch names do not bind: the gate binds only `waggle/T-<id>` and `t-<id>` branches (`cmd/waggle/gate_commits.go:36`). So binding depends on commit subjects or trailers, which you ask for in the repository instructions. That depends on Copilot following an instruction. A commit that names no task fails closed (`cmd/waggle/gate_commits.go:304-307`).

## Who claims, and when

The queen, or whoever assigns the task for the keeper, writes the claim first, then renews it once the PR opens and its branch name is known:

```
waggle claim T-<id> --dirs <a,b> --gate full --owner <your GitHub login> --branch copilot/<slug>
```

Add `--branch t-<nnn>` too if work also lands there. Until the claim names the PR's branch the gate fails the PR, which fails closed (`cmd/waggle/gate_commits.go:359-368`). A claim write does not trigger the workflow, so after the renewal re-run the failed check, or let the next push run it. A review alone does not clear a failed run ([approvers](../gate.md#6-approvers)). Workflows on Copilot's pushes wait for "Approve and run workflows", so the check stays pending until a person approves the run.

A re-claim after the claim expired drops `copilot/<slug>`: repeat every `--branch` (`internal/dispatch/claim.go:58-60`). Copilot PRs are often squash-merged; release with `waggle release T-<id> --unmerged`.

## What to put in the instructions

In `.github/copilot-instructions.md`, text such as:

```
Start every commit subject with the id of your task and a colon and a space, for example "T-<id>: fix the parser".
Change only files under the directories named in the task: <a,b>.
Do not edit .github/workflows/ or .waggle/.
```

## When Copilot strays

The gate fails the PR with a line such as `T-123: path is outside the claim directories` against the stray path (`internal/comb/changecheck.go:85`, `cmd/waggle/gate_commits.go:233`).

## Limit

Binding depends on Copilot following the instruction; the gate only fails closed when it does not. The check is a required check that waits for a person's approval of the run, so a PR can sit pending. The claim is written by the queen or another trusted coordinator, never by Copilot (L3). The required check can be forged by a workflow or a wider credential, and a person's approval of the run is not shown to stop that (L11).

## Sources

- https://docs.github.com/en/copilot/concepts/agents/coding-agent/about-coding-agent
- https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/review-copilot-prs
- https://github.blog/changelog/2025-10-16-copilot-coding-agent-uses-better-branch-names-and-pull-request-titles
- https://docs.github.com/en/copilot/reference/custom-instructions-support
