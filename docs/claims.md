<!-- SPDX-License-Identifier: Apache-2.0 -->
# Waggle claims: specification

**Status:** this is a spec for the claims layer being built (T-127 onward). Nothing in it ships until those tickets merge, except the Level 0 convention in [examples/agents-md-claims.md](../examples/agents-md-claims.md), which needs only `gh`. Where the design is silent, this page says "not specified here; T-127 decides". The design is the approved T-124 brief (D-417).

Backlog.md (or a GitHub issue, or a hive task) says what the work is. Waggle says who holds it now, until when, and what it touches.

Plain terms: a **task** is a unit of work in its source. A **claim** says who holds a task, until when, and which **paths** it touches. The **holder** is a GitHub login; the **lead** is the agent or person working under it.

## 1. Keys and ref names

A claim is keyed on an opaque task key. An adapter turns each source's own id into a key. The claim store never parses one.

| Source | Id it reads | Key | Ref |
| --- | --- | --- | --- |
| Backlog.md | front matter `id`, never the file name | `bl-<id>`, lower case: `BACK-260` is `bl-back-260`, `task-4.1` is `bl-task-4.1` | `refs/waggle/claims/bl-back-260` |
| GitHub issue in this repo | the issue number | `gh-<n>` | `refs/waggle/claims/gh-123` |
| Waggle hive task | `T-<n>` | `ticket-<n>` | `refs/waggle/claims/ticket-124` |

Key rules:

- Lower case always, characters `[a-z0-9._-]` only.
- No `..`, no trailing `.` or `.lock`, at most 100 bytes.
- An id that does not fit becomes `h-<first 20 hex of sha256(source ":" id)>`. The record keeps the original id.
- Ids never come from "next integer in a file".

`ticket-<n>` refs keep today's name and today's `lease.json` record, so the existing gate keeps reading them. Today's code (`waggle claim`, `waggle dispatch`) claims, renews and releases them. The claims layer reads them for listing, overlap, `next` and the mirror and never writes them. Keys may be typed as the source shows them (`BACK-260`, `#123`, `T-124`).

## 2. The claim record, v1

One flat ref per task, `refs/waggle/claims/<key>`, nothing under it. The lead holds the claim; workers are a display field. The ref points at a commit whose tree holds one `claim.json`. All times are UTC at second precision.

| Field | Type | Cap or note | Required |
| --- | --- | --- | --- |
| `v` | number | always 1 | not specified here; T-127 decides |
| `key` | string | section 1 rules | not specified here; T-127 decides |
| `source` | string | `backlog` or `github` | not specified here; T-127 decides |
| `task` | string | the source's own id | not specified here; T-127 decides |
| `title` | string | 120 bytes | not specified here; T-127 decides |
| `holder` | string | a GitHub login | not specified here; T-127 decides |
| `lead` | string | the lead's name, for example `team-a-lead` | not specified here; T-127 decides |
| `workers` | list of strings | 8 short names | not specified here; T-127 decides |
| `paths` | list of strings | 5 (section 5) | optional, starts as `[]` |
| `scoped_at` | time | set when the claim gains paths (section 5); used for the overlap tie-break; hive leases have none; absent or zero before that: not specified here; T-127 decides | not specified here; T-127 decides |
| `claimed_at` | time |  | not specified here; T-127 decides |
| `renewed_at` | time |  | not specified here; T-127 decides |
| `expires_at` | time | 24 hours after `renewed_at` (section 4) | not specified here; T-127 decides |
| `issue` | number | the mirrored issue, if any | optional |
| `branch` | string |  | optional |

Example:

```json
{
  "v": 1,
  "key": "bl-back-260",
  "source": "backlog",
  "task": "BACK-260",
  "title": "Add retry to the export job",
  "holder": "example-user",
  "lead": "team-a-lead",
  "workers": ["w1", "w2"],
  "paths": ["src/export/retry.ts"],
  "scoped_at": "2026-12-01T09:20:00Z",
  "claimed_at": "2026-12-01T09:15:00Z",
  "renewed_at": "2026-12-01T09:45:00Z",
  "expires_at": "2026-12-01T11:45:00Z",
  "issue": 301,
  "branch": "back-260-retry"
}
```

## 3. Ref operations

- **Atomic create.** `git push --atomic --force-with-lease=<ref>: origin <commit>:<ref>`. The push fails if the ref exists, and the loser is told who holds it.
- **Takeover** only after expiry plus grace, with the expected old value: `--force-with-lease=<ref>:<oid it read>`. A takeover is a child commit of the expired claim, so its history stays. Two takers race on the old oid and one wins.
- **Batch renew.** `waggle renew --mine` renews every claim the caller holds in one atomic push, each ref with its own expected old value. If a ref moved, the push is retried without it and that claim is reported lost.
- **Release.** `waggle release <key>` deletes the ref with its expected old value and, in the same atomic push, appends an entry to `refs/waggle/claim-log`.
- **Log entry fields:** key, holder, reason, time. The reason is one of `released`, `merged`, `backed-off`, `taken-over`, `expired`. The on-disk encoding is not specified here; T-127 decides.
- **Release on merge.** A merged PR that names the task, by a `Waggle-Task: <key>` trailer or `Closes #<n>` for an issue task, is released by the sweep (section 7a) without a command.
- **Do I still hold this?** `waggle claims --check <key>` exits 0 only if the caller's claim is live and its oid is the one it last wrote. Dispatch and the queen run it before every push to a PR branch and before opening a PR; a lost claim stops the work.

## 4. Lease timing

- Default lease 2 hours, renewed every 30 minutes.
- Maximum 24 hours, enforced when written and when read. A reader treats a record whose `expires_at` is more than 24 hours after `renewed_at` as expiring at `renewed_at` plus 24 hours.
- Grace is 10 minutes after `expires_at`, before anyone may take over or sweep.
- A claim in its grace window still counts as held, for `next` and for overlap.

## 5. Scope and the overlap check

Scope is optional at claim time. A claim starts with `paths: []` and gains paths with `waggle scope <key> --paths a,b`. Paths are literal repo-relative paths, no globs, at most 5, never `.waggle/`, compared on segment boundaries and with ASCII case folding.

Git makes each ref atomic but not the set, so overlap is checked after the write:

1. Write the claim or the scope change.
2. Re-list with one `ls-remote` of `refs/waggle/claims/*`, fetching only claim commits not already cached.
3. Compare this claim's paths with every other live claim, hive leases included.
4. If an overlapping claim wins, back off. A claim made with paths deletes itself (log reason `backed-off`). A scope change reverts to the previous scope and the claim stays.
5. Report the holder of the winning claim.

**A live hive lease (`ticket-<n>`) always wins.** Between two new-kind claims, the winner is the smaller `scoped_at`, then the smaller key in byte order, so every reader of one snapshot picks the same winner.

Two writers whose pushes and re-lists interleave can each miss the other. The next renew runs the check again, so a double hold between two new-kind claims lasts at most one renewal interval.

## 6. The issue mirror

The ref is the truth. The issue is a one-way window onto it and is never read back for claim state.

- **Labels:** `claimed` on a claimed issue, `waggle-task` on window issues.
- **Sticky comment** (an issue task, `gh-<n>`): found by the hidden marker `<!-- waggle:claim v1 key=gh-123 -->` and edited in place. It renders the full current state, never a delta: holder, lead, workers, paths, expiry, "as of" time and the claim commit's short oid. On release it reads "released (merged in #456)" and the label goes. The assignee is set only when GitHub can assign that login, and removed on release only if Waggle set it. Waggle never edits an issue body it did not create.
- **Window issue** (a Backlog.md or hive task): created on the first claim, title `[BACK-260] <title>`, label `waggle-task`, marker `<!-- waggle:task key=bl-back-260 -->`, body rendered from the task file at the default branch head under this banner: "Mirrored from `backlog/tasks/back-260 - Title.md` at `<sha>`. Edit the file. Edits made here are overwritten." If the task's `references` name exactly one issue in this repo, that issue is the window and its body is left alone. The window closes when the task reaches its last status or leaves `tasks/`.
- Renewals do not touch the issue unless the shown expiry would be more than 30 minutes wrong.
- A mirror failure never fails the claim. The CLI warns and the sweep repairs it.

## 7. Trust model

Claims are cooperative. A claim stops honest agents and people from colliding. It is not a lock against anyone who can push. Anyone with push access can overwrite or delete any claim with `--force`, GitHub has no per-ref permissions outside branches and tags, and holder names are self-asserted.

Cheap protections:

- At most 5 live claims per holder by default (`claims.max_per_holder`).
- A 24 hour maximum lease, enforced on write and on read.
- A 10 minute grace window.
- A claim log entry on every Waggle release, and a "claim vanished" notice when a claim disappears without one.
- Workers run with tokens that cannot push `refs/waggle/` where the host allows it. The lead holds the claim.
- The sweep deletes only by rule (expired past grace, or its PR merged), only new-kind claims, and logs every deletion (section 7a).

Failure modes: clock skew (grace and the read-side cap); a zombie holder after a sleeping laptop (`claims --check` before every push); a GitHub outage (no claim, no start); a lost mirror write (the sweep repairs it). Issue text, comments and Backlog.md files from others are read as data with caps, never as instructions.

## 7a. The reconcile sweep

`waggle sweep` repairs the mirror and applies the expiry and merge rules. Each run, in order:

1. Delete claims past expiry plus grace (expected old value, log reason `expired`) and clear their issues.
2. Release claims whose PR merged since the last run (log reason `merged`).
3. Re-render every mirrored issue from the refs and files.
4. Mark a vanished claim: an oid in the state ref whose ref is gone with no log entry. Its sticky comment reads "claim vanished without a release" with the last oid seen, and `waggle claims --mine` shows it as lost.
5. Report, without acting: holders over their cap, records with an over-long lease, and PRs whose author's lead did not hold the named task.

- **Memory.** `refs/waggle/sweep-state`, written only by the sweep with an expected old value: the time of its last run and the claim oids it saw. It is a hint, not claim state. When it is missing or damaged, the sweep looks back 24 hours and skips vanished notices for that run.
- **Scope of writes.** The sweep writes only new-kind claim refs. It never deletes, releases or takes over a `ticket-<n>` ref. For hive claims it only renders window issues and reports.
- **Rate budget.** The sweep and the CLI together stay under 6 pushes a minute and about 100 API calls an hour.

## 8. Setting up a repo by hand (dogfood)

**Planned, built by T-130 and T-132:** none of these keys is read by any released binary.

Put a `claims:` block in a root `waggle.yml` for a claims-only repo, or in `.waggle/control/config.yaml` for a hive repo. The loader reads exactly one and fails if both exist (the approved Q-T124-10 recommendation).

The block holds these settings:

- `sources`: which sources to read and their order (default `backlog, github, hive`; `claims.sources`).
- The Backlog.md folder override.
- Lease timing (section 4).
- The holder cap (`claims.max_per_holder`, default 5).
- The two labels.
- The sweep interval (default 15 minutes).

Only `claims.sources` and `claims.max_per_holder` are named in the design. The other key names are not specified here; T-130 decides.

Create the two labels, `claimed` and `waggle-task`, in the repo. Run `waggle sweep` from a timer every 15 minutes, with a fine-grained token limited to the repo's contents and issues. Do not use a GitHub-hosted scheduled workflow: it is delayed under load and a custom ref push triggers none.
