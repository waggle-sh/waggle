<!-- SPDX-License-Identifier: Apache-2.0 -->
# Waggle telemetry: payload schema v1

This page lists every field of the weekly telemetry report and the data that is never sent. The machine-readable form is [`telemetry.schema.json`](telemetry.schema.json) (JSON Schema 2020-12). It sets `additionalProperties: false` on the root and on every nested object, so any key not listed here is rejected. Design: `docs/rfc-notes/telemetry-insights-held-hive.md`, section 3.2.

The install id rules, the k rule for published aggregates, the threat model and the privacy statement follow in a second part, after the Head Keeper's ruling. Until then this page holds only the field table and the never-sent list.

## Fields

"Required" means always sent. The four optional fields are omitted, never guessed, when their source is missing.

| Name | Type | Required | Meaning |
| :--- | :--- | :--- | :--- |
| `claim_rate` | percent, rounded to 10 | yes | Percent of merged PRs with a bound claim (receipts vs merged PRs). |
| `claim_to_merge_hours` | p50 and p90 | yes | Receipt `claimed_at` to `merged_at`, rounded to 1, 4, 12, 24, 72, 168 or 336+. |
| `contributors` | bucket `1`, `2-5`, `6-20`, `21+` | yes | Distinct commit authors on the default branch, 90 days. |
| `failed` | counts per check | no | Counts per check (`leases`, `tiers`, `binding`, `decisions`), bucketed, for `guard` and `hold`. From the gate counts artifact. |
| `fix_cycles_per_pr` | p50 and p90, integer, capped at 10 | yes | Pushes after first review. |
| `forge_capability` | `required_check` or `none` | yes | Result of the doctor probe. |
| `gate_mode` | `open`, `guard` or `hold` | yes | Gate mode from the base config. |
| `install_id` | string | yes | Install id; rules follow in the second part. |
| `keepers` | bucket, same edges as `contributors` | yes | Count of `gate.approvers`. |
| `prs_merged` | bucket `0`, `1-5`, `6-20`, `21-50`, `51+` | yes | PRs merged, from receipts and the forge. |
| `rebase_cycles_per_pr` | p50 and p90, integer, capped at 10 | yes | Head force-updates and base merges. |
| `receipt_rate` | percent, rounded to 10 | yes | Percent of claims released with a receipt. |
| `releases_90d` | integer, capped at 12 | yes | Tags matching `v*` in 90 days. |
| `schema` | constant `1` | yes | Payload schema version. |
| `stage_dwell_hours` | p50 and p90 per stage, rounded to 1, 4, 12, 24, 72, 168 or 336+ | no | Time in each stage, from git history of the stage field. |
| `ticket_to_merge_hours` | p50 and p90, same rounding | yes | Task file creation to receipt `merged_at`. |
| `tickets_by_stage` | counts per `backlog`, `ready`, `in-progress`, `done`, each bucketed | no | Tickets per stage, from the structured stage field. |
| `waggle_version` | release tag, minor only (`0.2`) | yes | Build version. |
| `week` | ISO week, e.g. `2026-W48` | yes | Report week, from the clock. |
| `would_fail` | counts per check | no | Counts per check (`leases`, `tiers`, `binding`, `decisions`), bucketed, of findings that would fail under a stricter mode. From the gate counts artifact. |

The table is sorted by name; the schema defines the same 20 properties.

## Never sent

Logins, names, emails, repo or org names, URLs, branch names, ticket IDs or titles, paths, commit SHAs, commit or PR text, decision text, model names, token counts or costs (the usage ledger stays local), IP addresses (the collector drops them).
