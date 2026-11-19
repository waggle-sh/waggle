<!-- SPDX-License-Identifier: Apache-2.0 -->
# Recipe: Claude Code agent teams

Read [what every recipe assumes](README.md) first. Not run end to end by Waggle. The L-numbers are the limits in [the gate spec](../gate.md#10-limits-and-known-bypasses).

## What the tool does

- Agent teams are experimental and disabled by default; they are enabled by setting `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` ([agent teams](https://code.claude.com/docs/en/agent-teams)).
- Teammates are "Separate Claude Code instances". They load "CLAUDE.md, MCP servers, and skills" and the spawn prompt, and "The lead's conversation history does not carry over" (same page).
- "Two teammates editing the same file leads to overwrites. Break the work so each teammate owns a different set of files." (same page).
- A teammate launches on a named Agent call "unless the call is a fork or passes `isolation` on the call itself" (same page). So an Agent call with `isolation` is not a teammate.

## What this recipe infers

The pages above do not say where teammates work. This recipe infers that teammates usually share one working tree and so one PR branch. It therefore binds commits by subject prefix or trailer, not by branch name, and has one claim per task, every one listing that PR branch.

## Who claims, and when

The queen, or whoever coordinates the team for the keeper (never a teammate doing the work), writes one claim per task before the teammates start, each with the PR branch:

```
waggle claim T-<id> --dirs <a,b> --gate full --owner <your GitHub login> --branch <the PR branch>
```

One PR may carry commits of several tasks; each commit is checked against its own task's claim (`cmd/waggle/gate_commits.go:260-310, 315-338`). The PR branch name must match neither `waggle/T-<id>` nor `t-<id>`, or commits of any other task fail with `ticket conflict` (`cmd/waggle/gate_commits.go:279-298`). Give teammates disjoint directories, which also matches the page's advice about overwrites.

## What to put in the instructions

Put this in the project's CLAUDE.md or in each teammate's spawn prompt, which teammates load:

```
Start every commit subject with the id of your task and a colon and a space, for example "T-<id>: fix the parser".
Change only files under the directories of your task: <a,b>.
Never run waggle claim or waggle release, and never push to refs/waggle/.
```

Binding then depends on the teammate following the instruction. A commit that names no task fails closed.

## When a teammate strays

The gate fails the PR with a line such as `T-123: path is outside the claim directories` against the stray path (`internal/comb/changecheck.go:85`, `cmd/waggle/gate_commits.go:233`).

## Limit: permission rules are not a boundary

Deny rules for `waggle claim`, `waggle release` and pushes to `refs/waggle/` reduce accidents. They are not a security boundary: a Bash rule "doesn't match the same program invoked in a different form, so a deny or ask rule covers the invocation Claude usually produces and isn't a security boundary around the program" ([permissions](https://code.claude.com/docs/en/permissions#bash-rule-limits)). While the session holds your git credentials, it can still write claims (L3), and with credentials that can push workflow files it can forge the required check (L11). Teammates start with the lead's permission mode, except `dontAsk` mode ([agent teams](https://code.claude.com/docs/en/agent-teams)).

## Optional: a local check in a hook

The agent teams page lists the hooks `TeammateIdle`, `TaskCreated` and `TaskCompleted`, and says to "Exit with code 2" to block and send feedback. A `TaskCompleted` hook can run the gate locally. Preconditions:

1. Fetch claim refs first: `git fetch origin '+refs/waggle/claims/*:refs/waggle/claims/*'`. The gate reads claims from the local repo (`cmd/waggle/gate_verify.go:230`).
2. Fetch a current base: `git fetch origin <default-branch>`.
3. Run `waggle gate verify --base origin/<default-branch> --head HEAD --branch <the PR branch>`. Without `--branch` the branch check is skipped (`cmd/waggle/gate_commits.go:349-352`).
4. Map the gate's exit 1 to the hook's blocking exit 2.

A local run skips the approver and trailer checks (`cmd/waggle/gate_verify.go:130-132`, `cmd/waggle/gate_approve.go:175-187`). It does not replace the required check.

## Sources

- https://code.claude.com/docs/en/agent-teams
- https://code.claude.com/docs/en/permissions
