<!-- SPDX-License-Identifier: Apache-2.0 -->
# Quick start

Waggle Gate is a GitHub merge check for repos where AI agents write code. This page takes a repo from nothing to a required check in about ten minutes. Waggle is pre-1.0: the [gate spec](/docs/gate/) describes what the code does today, and the [recipes](/docs/recipes/) have not been run end to end with each agent tool.

## What you get

On a repo where the check is required and PRs are required, the gate fails a PR whose commits change paths outside the claim of the task each commit names, or whose task's claim does not name the PR's branch. A keeper talks with their queen. The queen gives each task a [claim](/docs/claims-and-leases/): the directories it may change and the branches its PR may come from. The worker does not write that claim. The gate reads the claims from git and compares them with the PR's commits.

## Before you start

- A GitHub repo you administer. v0.1 supports GitHub only (`waggle init` refuses any other forge).
- Go, to build `waggle` from source, or a tagged release archive (`waggle_<version>_<os>_<arch>`) if one exists for the version you want.
- A clean clone of a **pushed** commit of Waggle, so the Action ref the installer writes exists on GitHub.

## Install

1. Build `waggle` from a clean clone of a pushed commit, checked out at `<commit-sha>`: `go build -o <path outside the clone>/waggle ./cmd/waggle`. Build outside the clone: an untracked binary inside it makes the next build stamp a modified tree, and `init --gate-only` then refuses. Tagged releases carry `waggle_<version>_<os>_<arch>` archives and a `SHA256SUMS` file; check a download against its `SHA256SUMS` line.
2. In your repo, run `waggle init --gate-only`. It writes `.github/workflows/waggle-gate.yml`, `.waggle/control/config.yaml` and `.waggle/hive/DECISIONS.md`.
3. Commit those files. The install PR changes the workflow and `.waggle/`, so it fails its own gate. If a rule already requires the check, it needs an admin merge.
4. Require the check `verify-leases-and-tiers`, and PRs, on your default branch. Without both, the gate binds nothing.
5. Run `waggle doctor`. It checks that this clone can push claim refs and that a rule requires the check with up-to-date branches. It does not check that PRs are required, required reviews, code-owner review, or which app may post the check.

The full workflow is in the [gate spec](/docs/gate/#13-the-calling-workflow). A pin is written `<commit-sha>`; for a release tag, read the commit with `git rev-parse vX.Y.Z^{commit}`.

## The queen writes the first claim

A keeper talks with their queen, and the queen writes the claim before a worker starts. Opening the GitHub issue from that conversation is not automatic yet. A keeper can still open the issue directly. The command the queen runs is:

```
waggle claim T-123 --dirs internal/parser,docs --gate light --owner <your GitHub login> --branch t-123
```

Then the worker starts on branch `t-123` and names the task in every commit subject, `T-123: fix the tokenizer`. The queen opens the PR. The gate runs `verify-leases-and-tiers`:

- a commit that touches `cmd/main.go` fails with `path is outside the claim directories`, because `cmd` is not in the lease;
- a PR from branch `fix-parser` fails with `claim does not name this pull request's branch`, because the claim lists only `t-123`.

When the PR merges, the queen gives the claim back: `waggle release T-123` (after a squash or rebase merge, `waggle release T-123 --unmerged`). See [Claims and leases](/docs/claims-and-leases/) for every rule.

## What it does not do

The gate does not schedule or run agents and is not a sandbox. Its limits are written down in [the gate spec](/docs/gate/#10-limits-and-known-bypasses). The two to know on day one:

- **The required check can be forged.** A workflow or a token with the right permissions can post a passing check of the same name. Bind the check to GitHub Actions, give agents credentials limited to Contents and Pull requests, and require a person's approval. None of these is complete alone.
- **It binds only on PRs.** A merge that skips PRs skips the gate, and an admin bypass merges without it.

## Next

- [The gate](/docs/gate/): what each check does, with the code that does it
- [Claims and leases](/docs/claims-and-leases/)
- [Configuration](/docs/config/) and [CLI reference](/docs/cli/)
- [Recipes](/docs/recipes/) for Claude Code agent teams and the Copilot cloud agent
