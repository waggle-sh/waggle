<!-- SPDX-License-Identifier: Apache-2.0 -->
# Troubleshooting: failure messages and their fixes

Search this page for the line the gate, `waggle doctor` or `waggle claim` printed. Each entry gives the message as the code prints it, the exit code, the cause and the fix. The [gate spec](gate.md) has the full rules behind each check.

A `<name>` in a message stands for a value that changes, such as a task id (`T-97`), a path or a branch. The gate prints a failure as `leases: FAIL: <message>` or `tiers: FAIL: <message>`, or `leases: FAIL: <path>: <message>` when the finding is about one path. Under GitHub Actions it is an `::error::` annotation with the same text. `waggle doctor` folds long lines, so a message can wrap onto the next line.

Exit codes: `waggle gate verify` exits 0 on a pass, 1 when a check fails, 2 on a usage error and 3 on a git or I/O error. `waggle claim` exits 0 on success, 2 on a usage error, 3 on a git, config or I/O error and 4 when the claim is refused. `waggle doctor` exits 0 when every required setting is ok, 1 when one is missing or unknown, 2 on a usage error and 3 when it cannot read the config, forge or default branch.

A claim write does not start the workflow. After you renew a claim, re-run the failed check on the PR.

## Gate: the claim

### `claim names no branch; renew it with waggle claim --branch`

Exit 1. The claim for the task has no branch list, so the gate cannot tie the PR to it. Renew the claim with the PR branch: `waggle claim T-<id> --dirs <dirs> --gate <light|full> --branch <pr-branch> --owner <your GitHub login>`. Repeat every `--branch` you want kept, because given flags replace the stored list.

### `claim does not name this pull request's branch`

Exit 1. The claim lists branches, but not the head branch of this PR. Renew the claim with `--branch <pr-branch>` (same command as above), or rename the branch to one the claim lists.

### `no claim for <id>; claims must stay until merge`

Exit 1. A commit names a task that has no claim ref. The claim was never written, was released too early or was deleted. Write it again with `waggle claim T-<id> --dirs <dirs> --gate <light|full> --branch <pr-branch>`. After a squash or rebase merge, release with `waggle release T-<id> --unmerged`.

### `pull request branch unknown`

Exit 1. The run is a GitHub event that carries no pull request (for example `push` or `pull_request_target`), so the gate cannot read the head branch. Run the gate on the `pull_request` event, as the workflow from `waggle init --gate-only` does.

## Gate: commits and paths

### `commit has no T-<id>: prefix, Waggle-Ticket: trailer or ticket branch`

Exit 1. A commit does not say which task it belongs to. Start the subject with `T-<id>: `, add a `Waggle-Ticket: T-<id>` trailer, or use a branch named `t-<id>`. Fix an existing commit with `git commit --amend` or `git rebase -i`, then push again.

### `path is outside the claim directories`

Exit 1. A commit changes a file that is not under any directory in the task's claim. Either move the change out of this PR, or widen the claim with `waggle claim T-<id> --dirs <old dirs and the new one> --gate <light|full> --branch <pr-branch>`. Directories that other active claims hold are refused.

### `case differs from lease`

Exit 1. The path matches a claim directory only when letter case is ignored, for example `Src/` against `src/`. Rename the path to the exact case in the claim, or renew the claim with the case you use.

### `symlink or submodule mode <mode> is not allowed`

Exit 1. A commit adds a symlink (mode 120000) or a submodule (mode 160000). The gate refuses both, whatever the claim says. Replace the link with a regular file, or leave it out of the PR.

### `Tier 2 path under a light claim`

Exit 1. The claim's gate level is `light`, but the PR changes a steering file such as `CLAUDE.md`, `AGENTS.md` or `.waggle/hive/`, which needs a `full` gate. Renew the claim with `--gate full`.

### `Tier 3 path; only an admin bypass merges it, approval does not`

Exit 1. The PR changes a control path: `.github/workflows/`, `.waggle/`, `go.mod`, `go.sum` or `vendor/`. No claim covers these. Split the change into its own PR and have an admin merge it with a bypass.

### `PR too large for the gate; split it`

Exit 1. The PR has more than 500 commits, or one commit changes more than 5000 paths. Split it into smaller PRs.

### `no merge-base; is the clone shallow? use fetch-depth: 0`

Exit 3. The clone has too little history for the gate to find where the PR branched. In the workflow, set `fetch-depth: 0` on the checkout step. Locally, run `git fetch --unshallow`.

## Any command with `--repo`

### `waggle <cmd>: <dir> is not a git repository (run it inside your repo or pass --repo <dir>)`

Exit 3. `claim`, `release`, `status`, `doctor`, `pace`, `nectar`, `quorum` and `vote` work on a git repository, and the directory they looked at is not inside one. Run the command from inside your repo, or pass `--repo <dir>` with the path to it.

## `waggle claim`

### `pass --owner <your GitHub login> or create .waggle/control/local.yaml with keeper_id: <your GitHub login>`

Exit 2. Waggle does not know who holds the claim. Pass `--owner <your GitHub login>`, or put `keeper_id: <your GitHub login>` in `.waggle/control/local.yaml`.

### `comb: directories overlap an active lease`

Exit 4. Another active claim already holds a directory that contains, or sits inside, one of yours. The full message names both tasks and the owner. Pick directories that do not overlap, wait for the other claim to be released or expire, or ask its owner to release it.

### `comb: ticket already claimed`

Exit 4. Another owner holds an active claim for this task, or won a race to write it. The full message says who and until when. Wait for the claim to expire, or ask the owner to release it. A stuck claim can be cleared by the first login in `gate.approvers` with `waggle force-release`.

### `comb: light gate cannot lease a Tier 2 steering path`

Exit 4. You asked for `--gate light` with a directory that holds steering files. Use `--gate full`.

## `waggle doctor`

### `This remote does not accept pushes to refs/waggle/*.`

Exit 1. Claims are git refs under `refs/waggle/`, and the remote refused a push to them. Allow your user to create and delete refs under `refs/waggle/` on the remote, then run `waggle doctor` again.

### `No rule on <branch> requires the <check> check.`

Exit 1. Nothing stops a PR from merging without the gate. In GitHub, add a branch ruleset on the default branch that requires the check `verify-leases-and-tiers`, then run `waggle doctor` again.
