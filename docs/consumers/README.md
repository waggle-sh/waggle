# Consumer hives

`hives.txt` lists the consumer hives the post-release job upgrades. When a
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
