<!-- SPDX-License-Identifier: Apache-2.0 -->
# Recipe: gate-run receipts after a merge

Read [what every recipe assumes](README.md) first. This one is about Nectar, not the merge check: the check never reads receipts. A **receipt** is the record of who owned a task when it merged. `waggle release` writes one. A fork contributor, or a claim that expired, rarely runs `release`, so their merges would have no receipt. `waggle nectar gate-receipt` writes one after the merge instead.

## The command

```
waggle nectar gate-receipt --before <sha> --after <sha> [--actor <id>] [--repo <dir>] [--json]
```

`--before` and `--after` are main before and after a push (the `before` and `after` of a GitHub `push` event; an all-zero `before` looks at `after` only). `--actor` is who ran the push and is recorded as `released_by`; it defaults to `$GITHUB_ACTOR`, and must be a valid keeper id, so an actor such as `name[bot]` is refused with exit 2.

For each commit on the first-parent line of `before..after` that binds to a task (a `T-<id>: ` subject or a `Waggle-Ticket:` trailer), it writes one receipt with source `gate_run` and merge proof `binding` when:

- the task still has a live claim, and
- no receipt names that commit, and the task has no receipt yet.

Commits of one task that land one after the other (a rebase merge) count as one merge, named by the last commit. A task with no live claim gets no receipt and stays unattributed. The command never releases or changes a claim.

The forge is asked once per merge, for the pull request whose merge commit is that commit, and its number, author, head branch and approvers are recorded. With no `gh` or `GITHUB_TOKEN`, or on an API error (one warning), those fields stay null and the receipt is marked `incomplete`; the merge itself is still recorded.

Running it again over the same range writes nothing. If a `waggle release` lands first, that receipt stands. All receipts of one run go in one atomic append.

Exit codes: 0 done, including nothing to write; 1 interrupted; 2 usage; 3 environment.

## Permissions

A job that runs this needs **push rights to `refs/waggle/receipts/*`**. On GitHub Actions that is `contents: write` for the job's token, plus `pull-requests: read` for the forge read. Check out the pushed `main` only, never pull request code. If a ruleset protects `refs/waggle/receipts/**`, it must let that job append (no deletes, no rewinds, but a fast-forward from the job's token). Anyone who can push `refs/waggle/*` can also add, rewind or delete receipts ([L3](../gate.md#10-limits-and-known-bypasses)), so a reader treats receipts as a record, not proof.

The shipped Action input and workflow job that run this on every push to `main` are a separate change: `gate/` and the workflow template are Head Keeper paths. Until they land, run the command from your own job or by hand.
