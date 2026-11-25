<!-- SPDX-License-Identifier: Apache-2.0 -->
# Waggle telemetry: payload schema v1

Telemetry is **off by default**. A hive that does not set `telemetry.enabled: true` computes nothing for sending and makes no network call. This page and its schema only describe the report. Nothing in the change that added them sends or writes data: there is no sender, no config key and no CLI command yet (T-105 and T-108 add them).

This page lists every field of the weekly report and the data that is never sent. The machine-readable form is [`telemetry/schema-v1.json`](telemetry/schema-v1.json) (JSON Schema draft 2020-12). It sets `additionalProperties: false` on the root and on every nested object, so any key not listed here is rejected. Design: `docs/rfc-notes/telemetry-insights-held-hive.md`, section 3.2.

## Report window

One report covers the last complete ISO week, Monday 00:00 to Sunday 23:59:59 UTC. The `week` field names it, for example `2026-W48`. A week still in progress is never reported.

## What is collected

Everything is computed locally from git refs, receipts and the gate. A field is either required (always present) or optional (omitted, never guessed, when its source is missing). Optional fields are the ones the first `waggle telemetry show` (T-105) cannot yet fill, so a report that leaves them out still validates.

Forms used below:

- **Bucket:** a string naming a range. People edges: `1`, `2-5`, `6-20`, `21+`. Count edges: `0`, `1-5`, `6-20`, `21-50`, `51+`.
- **Hours:** an integer from the set 1, 4, 12, 24, 72, 168, 336. A duration is rounded up to the smallest edge that is at least as large; 336 means 336 hours or more. A pair is an object `{"p50": h, "p90": h}`.
- **Cycles:** an integer from 0 to 10; larger values are reported as 10. A pair is `{"p50": n, "p90": n}`.
- **Percent:** an integer rounded to the nearest 10, from 0 to 100.
- **Check counts:** an object with the optional keys `leases`, `tiers`, `binding` and `decisions`, each an integer of 0 or more (a plain count, not bucketed).

| Name | Type and form | Enum values | Bucket edges and rounding | Required | Why it exists | Filled by |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `schema` | integer | `1` | none | required | Lets the collector pick the right validator. | T-105 |
| `week` | string, ISO week `YYYY-Www` | none | week 01 to 53, Monday to Sunday UTC | required | Names the report window; one report per repo per week. | T-105 |
| `waggle_version` | string, `major.minor` | none | patch dropped (`0.2`) | required | Compares friction across releases without a rare patch fingerprint. | T-105 |
| `gate_mode` | string | `open`, `guard`, `hold` | none | required | Splits every other number by enforcement mode. | T-105 |
| `forge_capability` | string | `required_check`, `none` | none | optional | Shows where `hold` is needed; comes from the doctor probe. | T-116 |
| `contributors` | people bucket | `1`, `2-5`, `6-20`, `21+` | people edges; distinct commit authors on the default branch over 90 days | required | Team size context. | T-105 |
| `keepers` | people bucket | `1`, `2-5`, `6-20`, `21+` | people edges; count of `gate.approvers` | required | Approval load context. | T-105 |
| `prs_merged` | count bucket | `0`, `1-5`, `6-20`, `21-50`, `51+` | count edges; PRs merged in the window | required | Throughput context. | T-105 |
| `tickets_by_stage` | object, keys `backlog`, `ready`, `in-progress`, `done`, each a count bucket | per key as `prs_merged` | count edges; each key optional | optional | Shows where tickets pile up. Needs the structured stage field. | T-106 |
| `stage_dwell_hours` | object, same keys, each an hours pair | none | hours edges, round up | optional | Shows where tickets wait. Needs the stage field history. | T-106 |
| `ticket_to_merge_hours` | hours pair | none | hours edges, round up; task file creation to receipt `merged_at` | required | End to end lead time. | T-105 |
| `claim_to_merge_hours` | hours pair | none | hours edges, round up; receipt `claimed_at` to `merged_at` | required | Build time once work starts. | T-105 |
| `would_fail` | check counts | keys `leases`, `tiers`, `binding`, `decisions` | integer per check key, no bucketing | optional | Findings that would fail under a stricter mode; shows friction before it bites. Needs the gate counts artifact. | T-107 |
| `failed` | check counts | same keys | integer per check key, no bucketing | optional | Findings that failed under `guard` or `hold`. Needs the gate counts artifact. | T-107 |
| `fix_cycles_per_pr` | cycles pair | none | integer 0 to 10, capped at 10; pushes after first review | optional | Review churn. | T-105 |
| `rebase_cycles_per_pr` | cycles pair | none | integer 0 to 10, capped at 10; head force-updates and base merges | optional | Rebase churn. | T-105 |
| `claim_rate` | percent | none | rounded to 10; merged PRs with a bound claim | required | Shows how much work follows the claim flow. | T-105 |
| `receipt_rate` | percent | none | rounded to 10; claims released with a receipt | required | Shows how much work closes cleanly. | T-105 |
| `releases_90d` | integer | none | 0 to 12, capped at 12; tags matching `v*` in 90 days | required | Release cadence. | T-105 |
| `install_id` | string | none | none | optional | Links reports from one install across a quarter. Its rules belong to cut B and are not decided here. | T-105 |

The schema defines these 20 properties and no others. Required: `schema`, `week`, `waggle_version`, `gate_mode`, `contributors`, `keepers`, `prs_merged`, `ticket_to_merge_hours`, `claim_to_merge_hours`, `claim_rate`, `receipt_rate`, `releases_90d`. Every other field is optional.

Bucket edges for `tickets_by_stage` are not spelled out in the RFC (section 3.2 says only "each bucketed"). This page reuses the `prs_merged` count edges.

## Never sent

Logins, names, emails, repo or org names, URLs, branch names, ticket IDs or titles, paths, commit SHAs, commit or PR text, decision text, model names, token counts or costs (the usage ledger stays local), IP addresses (the collector drops them).

Adding a field, or changing a form or an edge, is a schema version bump and a Head Keeper decision.

## Not yet decided

These parts of the telemetry design are not written here. They wait on two open Head Keeper questions (the privacy statement and the protected paths):

- The install id rules (form, rotation, storage).
- The k = 10 publishing rule for aggregates.
- The threat model.
- The privacy statement.

Until the Head Keeper rules, treat this page as the field list only.
