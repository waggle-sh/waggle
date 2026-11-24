<!-- SPDX-License-Identifier: Apache-2.0 -->
# Consumer hives

`hives.yml` lists the consumer hives the post-release job upgrades. When a
release is published, the `upgrade-consumers` job in
`.github/workflows/release.yml` installs the published CLI, clones each listed
repo and runs `waggle upgrade --apply --pr` (the `--pr` flag comes from T-101;
the first release published after T-101 merges is the first live run).

Before `waggle upgrade --apply --pr`, the job sets `user.name` and `user.email` in each consumer clone, because `--pr` takes its author and committer from git config and refuses when neither is set.

Adding a repo here is a rollout decision for the Head Keeper. Do not list a
repo that has not opted in.

## Secret

The default `github.token` of this repo cannot push to a consumer. The job
reads the repository secret `WAGGLE_UPGRADE_TOKEN`: a fine-grained token (or
app token) with `contents: write` and `pull-requests: write` on exactly the
listed consumer repos. The Head Keeper creates it in the repo settings; it is
never committed. If it is missing, the job logs a warning and skips. The
token is passed only through the `GH_TOKEN` environment and is never echoed.

## Variables

The job also reads two repository variables (not secrets), set by the Head
Keeper in the repo settings:

- `WAGGLE_UPGRADE_GIT_NAME`: the git identity name for the bump commits.
- `WAGGLE_UPGRADE_GIT_EMAIL`: the git identity email for the bump commits.

The job sets them as `user.name` and `user.email` in each consumer clone. If
either is empty, the job logs a warning and skips, the same way it does when
`WAGGLE_UPGRADE_TOKEN` is missing.
