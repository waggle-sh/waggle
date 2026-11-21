<!-- SPDX-License-Identifier: Apache-2.0 -->
# Reputation score

`waggle nectar` prints a reputation score for each keeper, computed from the merge history of `main` and the release records. Anyone with a clone can run it. It calls no hosted service and no forge API, and it changes no file and no ref except the release record ref that a fetch updates. The design is in [RFC-0005](RFC-0005-nectar.md).

Two commands read the score:

- `waggle nectar` prints the table for every keeper, every count of a rule that gave less than full credit, and the exact inputs it was scored at.
- `waggle nectar explain <task>` shows why one task earned what it did.

## The number

Each merged task is one unit. A unit's weight is `tier x size x review`:

- `tier` comes from the risk tier of the paths the task changed.
- `size` grows with the changed lines. Code lines count in full and docs lines count at half. A task under the configured minimum counts as a minimal unit, never zero.
- `review` is 1, or lower when a rule gave less than full review credit (a self merge in a repo with several keepers).

A unit's weight counts as `pending` until it matures, then as `matured`. The score is `matured` minus `clawed_back`, never below zero. Pending weight is never part of it.

## The window

A unit matures 30 days after it merged to `main`. For a direct push (no pull request recorded) the 30 days start when the release record was first seen, if that is later than the merge. Work that is still alive on `main` at the end of the window earns its weight. If too few of its added lines survive, the unit is `dropped` and earns nothing.

A revert before the end of the window drops the unit. A revert after it claws back the weight, with no time limit: a revert a year later still takes the weight back. A partial revert claws back its share. If a revert is itself reverted, the work is re-landed, the window starts again at the re-landing and the clawback is undone.

## Who is credited

A unit is credited to the keeper in its release record. Records come from three sources:

- `waggle release`, run by the keeper after the merge.
- `waggle nectar backfill`, for tasks that merged before records existed.
- The gate's post-merge job, which writes a `gate_run` record for a merged task that still has a live claim.

A merged unit with no valid record is `unattributed`. A record naming a merge that left `main` marks the unit `orphaned`, with `rewritten` when its base is still on `main` and `missing` when it is not.

## Counts, flags and inputs

Nothing is excluded silently. The `counts:` line prints every count, zeros included:

| Count | Meaning |
| :--- | :--- |
| `unbound` | commits on `main` that name no task |
| `unattributed` | merged tasks with no valid release record |
| `orphaned` | recorded merges that left `main` |
| `unmerged` | tasks whose records name no merge |
| `incomplete` | records written offline, so forge facts are missing |
| `third_party` | records released by someone other than the keeper |
| `fork_mismatch` | the pull request author differs from the credited keeper |
| `direct_push` | units with no pull request recorded |
| `self_merge` | units the keeper merged themselves in a repo with several keepers |
| `carried` | units whose weight carries past the daily cap |
| `unconfirmed_revert` | commits that look like reverts but name no reverted commit |

The `excluded:` line lists keepers dropped by `nectar.exclude_keepers` with their weight, and `reciprocal pairs:` lists keeper pairs that approve each other's work with their share. Both print `none` when empty.

The `scored:` line names the inputs: `main` and `receipts` object ids in full (`receipts none` when there are no records), `rules_version`, the time it was scored at, the repo, and `local only` when this clone was scored without a fetch or a fetch failed, or `receipts rewound` when the remote record chain was rewound. The same inputs always give the same score.

## waggle nectar

```
waggle nectar [--repo <dir>] [--keeper <id>] [--now <rfc3339>] [--main <oid>] [--receipts <oid>] [--no-fetch] [--recompute] [--json]
```

| Flag | Meaning |
| :--- | :--- |
| `--repo <dir>` | repository directory; default is the current work tree root |
| `--keeper <id>` | show only this keeper's table row; the counts stay whole; not allowed with `--json` |
| `--now <rfc3339>` | score as of this time; default is the current time |
| `--main <oid>` | score at this `main` commit, a full hex id |
| `--receipts <oid>` | score with the release records up to this commit, or all zeros for none |
| `--no-fetch` | do not fetch; score what this clone holds |
| `--recompute` | ignore any cached result |
| `--json` | print the summary object and nothing else |

By default it fetches release records first and `main` second, so a record is never read without the merge it names. A failed fetch warns once on stderr and scores local state. Git does not fetch `refs/waggle/*` on its own, so a fresh clone that never ran this command has no records and shows tasks as `unattributed`.

Exit codes:

- `0`: scored, no finding.
- `1`: a finding: any `orphaned` or `unattributed` unit, or `receipts rewound`. The output is complete.
- `2`: usage: an unknown flag, an extra argument, a bad `--now`, an invalid `--keeper`, a malformed `--main` or `--receipts`, or `--keeper` with `--json`.
- `3`: not a Waggle repo, invalid config, relay mode, a pin that does not resolve, or any error after the fetches.

## waggle nectar explain

```
waggle nectar explain <task> [--repo <dir>] [--now <rfc3339>] [--main <oid>] [--receipts <oid>] [--no-fetch] [--json]
```

It reads the same inputs as `waggle nectar` with the same flags (there is no `--keeper` and no `--recompute`). `<task>` is `T-052`, `052` or `52`. It prints one labeled line each for:

- `task`, `kind` and `state`: `pending`, `matured`, `clawed_back`, `dropped`, `orphaned` (with `rewritten` or `missing`) or `unattributed`.
- `commits`, `merged_sha`, `base_sha`, `merged_at`, `first_seen` and `matures_at`.
- `weight` as `tier x size x review`, then `changed_lines`, `code_lines`, `docs_lines` and each excluded file.
- `keeper_id` with `pr_author` beside it, each `co_keepers` share, `alive_ratio`, `clawed_back` and `carried`.
- The chosen release record: `oid`, `source`, `status`, `merge_proof`, `incomplete`, `released_by`, `pr_number`, `head_branch` and `approvers`.
- Each flag that is set: `third_party` (with `released by a maintainer` when the releaser is a configured approver), `relay`, `fork_mismatch`, `direct_push` and `self_merge`.
- Each reason code with one sentence. A code with no sentence prints bare.
- Each revert entry that names the task: its commit, `merged_at` and `clawed_back`.

A value that is not set prints `none`. `pr_author` is `none` for a record written offline or for a direct push; `git show -s --format=%an <merged_sha>` gives the commit author.

A task with no merged unit on `main`, including one whose only commits name no task, prints `no merged unit for T-<id>` and exits 0.

With `--json` it prints the detail object `{"detail_version": 1, "counts": {...}, "units": [...]}`, with `units` narrowed to the task and `counts` whole.

Exit codes:

- `0`: explained, or no unit for the task.
- `1`: this task is `orphaned` or `unattributed`, or `receipts rewound`. Another task's finding does not change it.
- `2`: usage: an unknown flag, a missing or extra argument, a malformed task id, a bad `--now`, or a malformed pin.
- `3`: the same environment failures as `waggle nectar`.

## JSON

`waggle nectar --json` prints the summary object of RFC-0005 section 9.2 and one newline, and nothing else on stdout. Warnings go to stderr. Pin `--now`, `--main` and `--receipts` to replay a score: the same three values give the same bytes in any clone that holds those commits. The `repo` field is `owner/repo` for a GitHub remote and empty otherwise, so two clones may differ there only.

`waggle nectar --json` and `explain --json` differ from `waggle status` and `waggle nectar backfill`, whose failures with `--json` print an object with `ok` set to false. With these two commands a failure with `--json` prints nothing on stdout, so `waggle nectar --json > score.json` never holds anything but the summary. Read the exit code to tell a finding (`1`) from a refusal (`2` or `3`).

## Limits

- Anyone who can push `refs/waggle/*` can write a release record. The score trusts the records the way it trusts the history of `main`.
- `third_party`, `relay`, `pr_author` and `approvers` are asserted by whoever wrote the record. The output shows them as written.
- Forge facts in a record are as of the time it was written.
- A record written offline is `incomplete`: the score does not check its review.
- A force push to `main` can rewrite the score, because the score is derived from `main`. A rewritten unit shows as `orphaned`.
- A clone that did not fetch, or ran with `--no-fetch`, shows fewer attributed units. The `scored:` line says `local only`.
- The rules for gaming (the daily cap and reciprocal approval) are described in RFC-0005 as accepted. They apply from the release that ships all of the scoring work.
