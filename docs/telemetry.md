<!-- SPDX-License-Identifier: Apache-2.0 -->
# Waggle telemetry: payload schema v1

This page lists every field of the weekly telemetry report and the data that is never sent. The machine-readable form is [`telemetry.schema.json`](telemetry.schema.json) (JSON Schema 2020-12). It sets `additionalProperties: false` on the root and on every nested object, so any key not listed here is rejected. Design: `docs/rfc-notes/telemetry-insights-held-hive.md`, section 3.2.

Telemetry is off unless a hive opts in. Nothing is sent before the collector ships. This page holds the field table, the never-sent list, the install id rules, how to see a payload, the rule for publishing aggregates, the threat model and the privacy statement.

## Fields

"Required" means always sent. The four optional fields are omitted, never guessed, when their source is missing.

| Name | Type | Required | Meaning |
| :--- | :--- | :--- | :--- |
| `claim_rate` | percent, rounded to 10 | yes | Percent of merged PRs with a bound claim (receipts vs merged PRs). |
| `claim_to_merge_hours` | p50 and p90 | yes | Receipt `claimed_at` to `merged_at`, rounded to 1, 4, 12, 24, 72, 168 or 336+ (`336` in the payload means 336 or more). |
| `contributors` | bucket `1`, `2-5`, `6-20`, `21+` | yes | Distinct commit authors on the default branch, 90 days. |
| `failed` | counts per check | no | Counts per check (`leases`, `tiers`, `binding`, `decisions`), bucketed (`0`, `1-5`, `6-20`, `21-50`, `51+`). Expected only under `guard` and `hold`. From the gate counts artifact. |
| `fix_cycles_per_pr` | p50 and p90, integer, capped at 10 | yes | Pushes after first review. |
| `forge_capability` | `required_check` or `none` | yes | Result of the doctor probe. |
| `gate_mode` | `open`, `guard` or `hold` | yes | Gate mode from the base config. |
| `install_id` | string | yes | Install id; rules in the install id section below. |
| `keepers` | bucket, same edges as `contributors` | yes | Count of `gate.approvers`. |
| `prs_merged` | bucket `0`, `1-5`, `6-20`, `21-50`, `51+` | yes | PRs merged, from receipts and the forge. |
| `rebase_cycles_per_pr` | p50 and p90, integer, capped at 10 | yes | Head force-updates and base merges. |
| `receipt_rate` | percent, rounded to 10 | yes | Percent of claims released with a receipt. |
| `releases_90d` | integer, capped at 12 | yes | Tags matching `v*` in 90 days. |
| `schema` | constant `1` | yes | Payload schema version. |
| `stage_dwell_hours` | p50 and p90 per stage, rounded to 1, 4, 12, 24, 72, 168 or 336+ | no | Time in each stage, from git history of the stage field. |
| `ticket_to_merge_hours` | p50 and p90, same rounding | yes | Task file creation to receipt `merged_at`. |
| `tickets_by_stage` | counts per `backlog`, `ready`, `in-progress`, `done`, each bucketed (`0`, `1-5`, `6-20`, `21-50`, `51+`) | no | Tickets per stage, from the structured stage field. |
| `waggle_version` | release tag, minor only (`0.2`) | yes | Build version. |
| `week` | ISO week, e.g. `2026-W48` | yes | Report week, from the clock. |
| `would_fail` | counts per check | no | Counts per check (`leases`, `tiers`, `binding`, `decisions`), bucketed (`0`, `1-5`, `6-20`, `21-50`, `51+`), of findings that would fail under a stricter mode. From the gate counts artifact. |

The table is sorted by name; the schema defines the same 20 properties.

## Never sent

Logins, names, emails, repo or org names, URLs, branch names, ticket IDs or titles, paths, commit SHAs, commit or PR text, decision text, model names, token counts or costs (the usage ledger stays local), IP addresses (the collector drops them).

## Install id

The install id is a random 128 bit value, written as 32 lowercase hex characters, kept in a git-ignored local file. It is never derived from repo data.

- It is replaced every 90 days, so one id is linkable across at most 13 weekly reports.
- It is tied to no one: it names no person, repo or org.
- A stable id is not used, because over time it would become a repo identifier.

## Gate counts artifact

The gate can write `waggle-gate-counts.json` as a job artifact under any mode. It holds counts per check, with no paths and no messages. The weekly job reads the last 7 days of these artifacts. Without the artifact, `would_fail` and `failed` are omitted, never guessed.

## Seeing the payload before anything is sent

- `waggle telemetry show` computes this week's report and prints the exact JSON that would be sent, byte for byte. It needs no network and works with telemetry off.
- `waggle telemetry send` prints the same JSON, then sends it. With telemetry off it refuses.
- The weekly workflow writes the payload into its job summary, so every sent report is visible in the repo's own Actions log.
- Adding a field is a schema version bump and a Head Keeper decision.

## Publishing aggregates

- A published cell needs at least k = 10 distinct reports from at least k = 10 distinct install ids in the window. Below that, the cell is merged into a wider bucket or suppressed.
- Percentiles are published only from cells that meet k.
- Raw rows are never published. They are kept 13 months, then only rollups remain.

## Threat model: what can still leak

- **Tiny repos.** A repo with one contributor and three PRs a week has a distinctive shape. Buckets and rounding blur it, and the k rule hides it from published aggregates. The operator of the collector can still see the row.
- **Timing.** Someone who knows a repo merged on a given day and watches the published aggregates cannot isolate it below k. The operator could match send time to public activity on a public repo. The weekly job sends at a random minute in a 24 hour window to reduce this.
- **Network metadata.** The collector sees the source IP when a request arrives (a CI runner IP, or the user's own IP for a hand `send`). The collector drops it and does not store it. Cloudflare still processes it.
- **Version and mode.** A rare version combined with `hold` and a `21+` contributor bucket is a small set. Version is sent at minor granularity only.
- **Supply chain.** A malicious PR could add fields to the sender. This page and the schema are listed in `gate.protected_paths` (D-402), so a change to either needs a listed approver at the head, and the collector rejects unknown keys.
- **Forks.** A fork of Waggle can send anything. The guarantees hold only for official builds.

## Privacy statement

Telemetry is off unless a hive opts in, and nothing is sent before the collector ships. When on, the report is aggregate, no personal data by design.

- The never-sent list above is complete. Nothing outside the field table is sent.
- The install id is linkable across 13 weekly reports, then it rotates.
- Raw rows are kept 13 months.
- The operator can see rows. Cloudflare processes the source IP even though the collector drops it.
- Forks are not covered.
- There is no per-install deletion, because the id rotates and is tied to no one.
