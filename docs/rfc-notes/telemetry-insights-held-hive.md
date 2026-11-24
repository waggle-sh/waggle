<!-- SPDX-License-Identifier: Apache-2.0 -->
# RFC note: opt-in telemetry, the insights loop, and a held hive for GitHub Free

- **Status:** Approved under D-389 on 2026-11-23 at 10:35 PM Pacific Time: decision option 1, with every open question in section 6 ruled as recommended. Ticket T-103 (design). Code tickets T-104 to T-118 are opened as stubs; the scout writes full briefs before any dispatch.
- **Date:** 2026-11-23
- **Layer:** `product` (design note). No Go code changes in this PR.
- **Read against:** `waggle-sh/waggle` main at `7b6254d` (merge of #305, 2026-11-23 09:50 PT).
- **Inputs:** `docs/rfc-notes/staged-gate-modes.md` (T-060, D-382), `docs/gate.md` (sections 6, 10, 11), `internal/config/config.go` (T-076), `cmd/waggle/gate_verify.go`, `cmd/waggle/doctor.go`, `internal/forge/probe_github.go`, `internal/forge/github.go`, `internal/comb/receipt.go`, `internal/comb/force.go`, `internal/usage/`, `.waggle/hive/VISION.md`, `.waggle/hive/tasks/T-015.md`, `T-047.md`.
- **Vision items touched:** core item 2 (merge gate) for part 3; core item 4 (small footprint, open source drop-in) for part 1; core item 5 (Nectar) through reverts in part 3. No option below is marked CUT except where noted.

## 1. Problem

Three gaps, one note, because they share data and the same pilot repos.

1. **We cannot see how Waggle is used.** The only usage data today is the per-machine token and cost ledger (`internal/usage/record.go`, T-021 section 8), which never leaves the machine. We do not know which gate check causes friction, how long tickets sit in each stage, or whether small teams skip claims.
2. **Nothing turns that evidence into framework changes.** The Head Keeper wants Waggle "always evaluating the numbers and suggesting Waggle framework improvements which I may or may not approve" (Head Keeper, 2026-11-23).
3. **`guard` needs a plan most small orgs do not have.** `guard` protects only when `verify-leases-and-tiers` is a required check (`docs/gate.md` L2). Rulesets and required checks on private repos need GitHub Team or higher (`T-015.md:55`, `T-037.md` row G6). On GitHub Free a private repo can only run the gate in `open` (`gate_verify.go:189`: only `verify` with the exact value `open` loosens), and nothing stops a Merge click. The Head Keeper's own product org is on GitHub Free and has chosen to stay there.

## 2. What exists today (code facts)

| Fact | Where |
| :--- | :--- |
| `gate.mode` accepts only `guard` (default) and `open`; any other value fails `validate` | `internal/config/config.go:41-44, 185, 432-435` |
| `open` prints each finding as `would fail (mode open)` and keeps exit 0; a too-large range still fails | `cmd/waggle/gate_verify.go:66-119, 186-197` |
| Mode, approvers and protected paths are read from the base commit, never the head | `cmd/waggle/gate_verify.go:314-351` |
| `watch`, `gate status`, `gate guard`, `gate open` from T-060 are not built | `cmd/waggle/gate.go` dispatches only `decisions`, `verify`, `leases`, `tiers` |
| An unknown top-level config section is ignored by older builds; an unknown key or value inside `gate` exits 3 | `docs/gate.md` L10 |
| `doctor` probes claim-ref push, required check, up-to-date branches and force-push blocking; a 403/404 on protection reads as "needs repo admin" | `cmd/waggle/doctor.go:27-89`, `internal/forge/probe_github.go:135-244` |
| The github forge already has a head-pinned merge (`sha` = expected head, 409 on head moved) | `internal/forge/github.go:107-140` |
| Release receipts carry `claimed_at`, `merged_at`, `pr_number`, `approvers`, `merge_proof` on `refs/waggle/receipts/main` | `internal/comb/receipt.go:25, 69-90` |
| An append-only audit chain pattern exists for force releases | `internal/comb/force.go:28` (`refs/waggle/audit/force-releases`) |
| Approver checks run only under Actions on PR events | `docs/gate.md` section 6, L8 |
| Ticket stage is not machine-readable: hive task files carry a prose `- **Status:**` line, and no code parses backlog / ready / in-progress / done | `.waggle/hive/tasks/*.md`; no match in `internal/` |

## 3. Part 1: opt-in telemetry

### 3.1 Shape

One new top-level section in `.waggle/control/config.yaml`:

```yaml
telemetry:
  enabled: false   # default; true sends one weekly aggregate report
```

- **Off by default.** No key, or `enabled: false`, means nothing is computed for sending and no network call is made.
- **Top-level on purpose.** An older pinned gate ignores unknown top-level sections (L10), so adding `telemetry:` cannot break an old pin the way a new `gate.*` key would.
- **Tier 3 by path.** The file is under `.waggle/`, so turning telemetry on needs the Head Keeper (admin merge or signed approval). An agent cannot switch it on quietly.
- **Weekly, not per command.** No CLI call ever phones home. A scheduled workflow (or `waggle telemetry send` run by hand) computes one report per ISO week from git refs and sends it. One report per repo per week is the whole stream.

### 3.2 What is sent (allowlist schema v1)

Everything is computed locally. Only these fields exist; the sender refuses any other key.

| Field | Form | Source |
| :--- | :--- | :--- |
| `schema` | `1` | constant |
| `week` | ISO week, e.g. `2026-W48` | clock |
| `waggle_version` | release tag, minor only (`0.2`) | build |
| `gate_mode` | `open` / `guard` / `hold` | base config |
| `forge_capability` | `required_check` / `none` | doctor probe |
| `contributors` | bucket `1`, `2-5`, `6-20`, `21+` | distinct commit authors on default branch, 90 days |
| `keepers` | bucket, same edges | `gate.approvers` count |
| `prs_merged` | bucket `0`, `1-5`, `6-20`, `21-50`, `51+` | receipts / forge |
| `tickets_by_stage` | counts per `backlog`/`ready`/`in-progress`/`done`, each bucketed | structured stage field (new, ticket 2b) |
| `stage_dwell_hours` | p50 and p90 per stage, rounded to 1, 4, 12, 24, 72, 168, 336+ | git history of the stage field |
| `ticket_to_merge_hours` | p50, p90, same rounding | task file creation to receipt `merged_at` |
| `claim_to_merge_hours` | p50, p90, same rounding | receipt `claimed_at` to `merged_at` |
| `would_fail` | counts per check: `leases`, `tiers`, `binding`, `decisions`, bucketed | gate counts artifact (3.4) |
| `failed` | same, for `guard`/`hold` | gate counts artifact |
| `fix_cycles_per_pr` | p50, p90, integer, capped at 10 | pushes after first review |
| `rebase_cycles_per_pr` | p50, p90, capped at 10 | head force-updates / base merges |
| `claim_rate` | percent of merged PRs with a bound claim, rounded to 10 | receipts vs merged PRs |
| `receipt_rate` | percent of claims released with a receipt, rounded to 10 | receipts |
| `releases_90d` | count, capped at 12 | tags matching `v*` |
| `install_id` | see 3.3 | |

Never sent: logins, names, emails, repo or org names, URLs, branch names, ticket IDs or titles, paths, commit SHAs, commit or PR text, decision text, model names, token counts or costs (the usage ledger stays local), IP addresses (the collector drops them, 3.6).

### 3.3 Install id

Options:
1. **None.** Reports cannot be linked week to week. Trends come only from population totals. Most private.
2. **Rotating.** A random 128-bit id kept in a git-ignored local file, replaced every 90 days. Allows quarter-long trends per install (did friction drop after an upgrade?). Linkable for 13 weekly reports.
3. Stable id. Not proposed: it becomes a repo identifier over time.

**Recommendation:** Option 2, rotating every 90 days, never derived from repo data.

### 3.4 Gate counts artifact

`verify` already knows each finding's check. Under any mode it can write `waggle-gate-counts.json` (counts per check, no paths, no messages) as a job artifact. The weekly job reads the last 7 days of artifacts through the Actions API. Without the artifact, `would_fail` and `failed` are omitted, never guessed.

### 3.5 Seeing the payload

- `waggle telemetry show` computes this week's report and prints the exact JSON that would be sent, byte for byte. It needs no network and works with telemetry off.
- `waggle telemetry send` prints the same JSON, then sends it. With `enabled: false` it refuses.
- The weekly workflow writes the payload into its job summary, so every sent report is visible in the repo's own Actions log.
- The schema lives at `docs/telemetry.md` with a JSON Schema file; adding a field is a schema version bump and a Head Keeper decision.

### 3.6 Collector options

| | A: Cloudflare Worker + D1 | B: Worker + R2 (JSON lines) | C: GitHub issue/artifact drop | D: Plausible-style event counter |
| :--- | :--- | :--- | :--- | :--- |
| Ingest | POST, schema-validated in the Worker | POST, appended per day | Workflow opens an issue in a public repo | Custom events |
| Query | SQL, easy weekly rollups | Batch scripts | Painful | Only counts, no percentiles |
| Drops IP | Yes, never stored (do not read `CF-Connecting-IP`) | Yes | No: issue author is a GitHub login | Vendor-side hashing |
| Cost at pilot scale (under 100 repos) | $0 on free tiers | $0 | $0 | $9+/month hosted |
| Cost at 10,000 repos weekly (about 43,000 writes/month) | Workers Paid $5/month covers it; D1 and R2 stay inside included quotas | about $5/month | $0 but unusable | $19+/month |

Costs are estimates from Cloudflare's published free and paid tiers; re-check before building.

**Recommendation:** A, a small Worker in a public repo (`waggle-sh/insights`), D1 for rows, schema checked on ingest, no IP or headers stored, rate limit per source address held only in memory.

### 3.7 Publishing aggregates: k-anonymity

- A published cell (for example "would-fail rate for tiers, contributors 1, guard") needs at least **k = 10** distinct reports from at least **k = 10** distinct install ids in the window. Below that the cell is merged into a wider bucket or suppressed.
- Percentiles are published only from cells meeting k.
- Raw rows are never published. Retention: raw rows 13 months, then only rollups.

### 3.8 Threat model: what can still leak

- **Tiny repos.** A one-contributor repo with three PRs a week has a distinctive shape. Buckets and rounding blur it; k-anonymity hides it from the public dashboard. The collector operator (us) can still see the row. Mitigation: no id option, and data minimisation; it is not zero.
- **Timing correlation.** Someone who knows a repo merged on a given day and watches the dashboard cannot isolate it below k. The operator could correlate send time with public activity on a public repo. Mitigation: the weekly job sends at a random minute in a 24 hour window.
- **Network metadata.** The Worker sees the source IP at request time (GitHub Actions runner IPs, or the user's IP for a hand `send`). We drop it; Cloudflare still processes it. Documented.
- **Version and mode fingerprint.** A rare version plus `hold` plus `21+` is a small set. Version is sent at minor granularity only.
- **Supply chain.** A malicious PR could add fields to the sender. Mitigation: the allowlist and schema live under Tier 3 paths and the Worker rejects unknown keys.
- **What it does not cover.** A fork of Waggle can send anything; the guarantees hold only for official builds.

## 4. Part 2: the insights loop (self-improving Waggle)

### 4.1 Shape

An **analyst** role runs on a schedule, reads the aggregates, and opens proposals. It never changes framework code, config or hive files itself.

```
reports -> collector -> weekly rollup -> analyst -> proposal (decision/RFC ticket) -> Head Keeper yes/no
   yes -> scout cuts a Ready ticket -> normal claim, build, gate, merge
   no  -> rejection recorded with reason -> analyst will not re-propose unless evidence changes materially
```

### 4.2 Where it runs

| | A: `waggle-sh/insights` repo | B: scheduled job in the Head Keeper's own cloud account |
| :--- | :--- | :--- |
| Transparency | Public: schema, Worker, rollups, analyst prompts and proposals visible | Private by default |
| Access to raw rows | Worker binding only | AWS credentials, a new trust path |
| Cost | Actions minutes on a public repo are free | One more container task a week, small |
| Uses Waggle itself | Yes, a hive with its own `.waggle/` and gate | Indirectly |
| Fits "Waggle itself is evaluating" | Yes, Waggle's own repo family | Feels like a private tool |

**Recommendation:** A. The analyst job lives in `waggle-sh/insights`, runs weekly by Actions schedule, and opens proposals against `waggle-sh/waggle` through the normal decision path. B stays available for dogfooding before the collector has outside data.

### 4.3 Detection

Weekly, comparing the last 4 weeks to the prior 8, per cell meeting k:

- **Friction share:** one check causes more than 40 percent of would-fail or failed findings in a cell.
- **Drift:** a metric moves more than 2 standard deviations, or p90 dwell in a stage doubles.
- **Adoption gaps:** claim rate under 50 percent in a cell, or receipt rate under 70 percent.
- **Mode stickiness:** share of installs that stay in `open` for more than 8 weeks.
- **Upgrade effect:** before and after a release, per rotating install id.

Each signal must persist two consecutive weeks before a proposal (one noisy week does nothing).

### 4.4 Proposal format

Opened as a Waggle decision card in the hive's existing form (Question, Options, Hive recommendation, Keeper choice), filed as an RFC ticket proposal. Each carries:

- **Signal:** the metric, the cell, the window, k for the cell.
- **Evidence:** a small table and a link to the public rollup.
- **Hypothesis:** why the framework, not the users, is the cause.
- **Options:** at least two, one of them "do nothing", each with cost and risk tier. CUT markers per VISION rule 1 where a core item would narrow.
- **Success metric:** what number should move, by how much, by when.
- **Fingerprint:** a hash of (signal type, check, cell) used for the repeat filter.

### 4.5 Approval, rejection, memory

- The Head Keeper approves or rejects. Only an approval leads to work; the scout turns it into a Ready ticket with acceptance criteria, lease and tier, and it goes through claim and gate like any ticket.
- A rejection is recorded with its fingerprint and reason in an append-only `rejections.md` in the insights repo (proposals live there, not in `.waggle/hive`, which stays Head Keeper only). The analyst skips a fingerprint for 6 months unless the signal grows by half again or the Head Keeper reopens it.
- After an approved change ships, the analyst reports the success metric at 4 and 12 weeks: did it work.
- Rate limit: at most 2 open proposals at a time, at most 1 new one a week.

### 4.6 Dogfooding first

Pilots are three of the Head Keeper's own private repos in `open` (audit-only, never blocking), pinned to the next release after T-102. A fourth follows after its current release deadline. Phase 0 runs the analyst over these three repos' own refs (no collector, no network) for 4 to 6 weeks. Three repos is below k, so nothing is published; the goal is to tune detectors and the proposal format on real data the Head Keeper already knows.

## 5. Part 3: a held hive for GitHub Free

### 5.1 Name

New value **`gate.mode: hold`**, plain name **held hive**: findings fail the check exactly as in `guard`, but enforcement lives in Waggle's merge path and a post-merge audit instead of a required check, because the forge cannot hold the merge.

| | `open` | `guard` | `hold` |
| :--- | :--- | :--- | :--- |
| Check result on a finding | pass, `would fail` | fail | fail |
| Where it blocks | nowhere | forge required check | `waggle merge` refuses |
| Needs | nothing | rulesets or branch protection (Team+, or public repo) | a merge identity, a push-triggered audit workflow |
| Manual Merge click | allowed, recorded | blocked (unless admin, L6) | allowed, detected, breach recorded, revert proposed |
| Direct push to main | allowed | blocked by "require PR" | allowed, detected as "no PR" breach |
| Forged check (L11) | n/a | can merge | does not help: `waggle merge` re-runs the checks itself |

### 5.2 The merge path: `waggle merge`

`waggle merge <pr>`:

1. Reads `gate.mode`, approvers and protected paths from the base commit (as `verify` does).
2. Fetches the PR head and base, re-runs `leases`, `tiers` and `decisions` locally at the exact head SHA.
3. Reads approvals through the forge reviews API (`internal/forge/reviews.go`) under the section 6 rules, since the approver check cannot run off Actions today (L8). Signed Head Keeper approvals verify against `allowed_signers` at base.
4. Merges with the existing head-pinned `forge.Merge` (`internal/forge/github.go:107-140`): a push after the check gets a 409 and nothing merges.
5. Writes a gate pass record (head SHA, base SHA, merge SHA) to `refs/waggle/audit/merges`, modeled on the force-release chain.

The merge identity: a GitHub App (preferred; separate identity, revocable) or a bot account. Agents call `waggle merge`; they hold no token that can merge directly.

### 5.3 Post-merge audit

A workflow on `push` to the default branch (and a daily sweep) checks each new first-parent commit:

- Merged by the merge identity and present in `refs/waggle/audit/merges` with a matching head: ok.
- Otherwise: a breach. Append a record to `refs/waggle/audit/breaches` (commit, PR number if any, merger login, findings from a re-run of `verify` at that head).
- Then, by config: `on_breach: issue` (default) opens a "decision needed" issue for the Head Keeper; `on_breach: revert` also opens a revert PR (`git revert -m 1`), which itself goes through `waggle merge`. The audit never merges the revert itself.
- A breach with zero findings (someone clicked Merge on a clean PR) is recorded as `ungated, clean` and only reported in the weekly summary.

Nectar note: a revert inside 30 days removes Nectar for the reverted commits (core item 5). A revert PR proposed by the audit should name that, and the Head Keeper decides.

### 5.4 Write-access restriction pattern (documented, not code)

On GitHub Free orgs: set the org base permission to Read; give Write only to the merge identity and the Head Keeper; contributors and agents work from forks or from branches pushed by a pusher identity with no merge right. Claims for fork contributors follow the relay note (`relay.md`, maintainer pre-claims now, comment relay in v0.2). This is the strongest prevention GitHub Free offers; it is a convention the audit checks, not something Waggle enforces.

### 5.5 `waggle doctor` picks the strongest mode

`doctor` probes capability, not the plan name (org plan is readable only by org owners):

- Rulesets list returns 403 with an upgrade message, or classic protection is unavailable, on a private repo: capability `none` (this is the 403 noted in `T-047.md:335`).
- Capability `required_check`: recommend `guard`; if the hive is in `hold`, report that a stronger mode is available.
- Capability `none`: recommend `hold` and check its parts: merge identity configured, audit workflow present and pinned, `refs/waggle/audit/*` pushable, who holds Write (count only), last breach.
- Never auto-changes the mode (mode changes stay Tier 3, D-382).

### 5.6 What `hold` does not stop (honest limits)

- Anyone with Write can click Merge or push to main. `hold` detects and records; it does not prevent.
- A writer can edit or delete the audit workflow on main, or delete `refs/waggle/audit/*` (L3 applies to every `refs/waggle/` ref). Mitigation: the weekly report and `doctor` flag a missing workflow or a rewound chain; the Head Keeper's local clone keeps the chain.
- Force-push to main rewrites history on Free private repos (no protection). The audit sees a non-fast-forward and records it, if it still runs.
- A stolen merge identity token merges anything. Keep it in Actions secrets of the merge job only, never on agent machines.
- Windows: a breach between the merge and the audit run is live on main for minutes. Deploys triggered on push run before any revert.
- Old pins: `hold` is a new value in a known key, so a gate pinned before the parser exits 3 on every PR (L10). The pin must move first, as with `gate.mode`.

### 5.7 VISION check

Core item 2 says "Required check". `hold` is offered only where the forge cannot provide a required check; it does not replace `guard` where `guard` is possible, so nothing is narrowed. Offering `hold` as a choice where `guard` is available would narrow core item 2 for that hive: **CUT? needs Head Keeper typed reason** (open question 6).

## 6. Open questions

All eleven were ruled as recommended under D-389 on 2026-11-23.

1. **Telemetry install id.** Recommend rotating every 90 days. Pro: per-install trends, upgrade effects. Con: links 13 reports; "none" is simpler to explain.
2. **Telemetry default for the pilots.** Recommend on for the three pilots, Phase 0 local only (no send) for 4 to 6 weeks. Pro: real data, no outside exposure. Con: below k, so learnings are anecdotal.
3. **Collector home.** Recommend public `waggle-sh/insights` with Cloudflare Worker + D1. Pro: auditable, near-free. Con: a public endpoint to maintain and abuse-protect; a new Cloudflare account surface (money, needs Head Keeper yes).
4. **Analyst autonomy.** Recommend proposals only, max 2 open, 1 new per week. Pro: Head Keeper stays the strategist without noise. Con: slower loop; good ideas wait.
5. **Rejection memory window.** Recommend 6 months or 1.5x signal. Pro: no nagging. Con: a rejected idea in a changed world waits.
6. **`hold` only where `guard` is impossible?** Recommend yes. Pro: no core item narrowed. Con: a Team-plan user who wants Waggle-owned merges cannot pick `hold` without a CUT reason.
7. **Mode value or separate key.** Recommend one mode value `hold`. Pro: one switch, one doctor finding. Con: mixes "what a finding does" with "where enforcement lives"; the alternative is `gate.enforce: forge | waggle` beside `mode: guard`.
8. **Breach default.** Recommend `on_breach: issue`, revert opt-in. Pro: no surprise reverts, Nectar untouched until a human decides. Con: an ungated change stays on main until someone acts.
9. **Merge identity.** Recommend a GitHub App. Pro: separate, revocable identity. Con: per-repo setup and a private key to guard (secrets, Head Keeper yes).
10. **Ticket stage field.** Stage dwell needs a structured stage in task files, which Waggle does not have. Recommend a one-line `stage:` field that the scout may edit, so tickets move between stages without a Head Keeper card for each move. Pro: real dwell data, and agents can move tickets along. Con: a hive format change, and it widens what the scout may edit in `.waggle/hive`.
11. **Sequencing.** Recommend: telemetry schema and `show` first (no network), `hold` second (the pilots need it now), collector and analyst third. Pro: value for the pilots early, privacy design reviewable before any send. Con: the self-improving loop arrives last.

## 7. Tickets

| ID | Ticket | Tier | Depends on |
| :--- | :--- | :--- | :--- |
| T-104 | `docs/telemetry.md` schema v1 and JSON Schema; privacy statement | Tier 2 (steering docs), Head Keeper review | T-103 |
| T-105 | `telemetry:` config section parse; `waggle telemetry show` (local, no network) | Tier 2 | T-104 |
| T-106 | Machine-readable ticket stage (`stage:` front-matter line in task files), editable by the scout; until it lands, stage fields are omitted from the payload | Tier 2 (hive format), Head Keeper decision | T-104 |
| T-107 | Gate counts artifact from `verify` (counts only) | Tier 3 by content (gate) | T-105 |
| T-108 | Weekly telemetry workflow template, `waggle telemetry send`, random send minute | Tier 3 (workflow scaffold) | T-105, T-107, T-110 |
| T-109 | Analyst Phase 0: local rollups over the pilot repos' refs, detector tuning, no network | Tier 2 (separate repo) | T-105 |
| T-110 | `waggle-sh/insights` repo, Worker + D1 ingest, schema validation, no IP storage | Tier 3 (new infra, money, secrets): explicit yes | T-104 |
| T-111 | Public rollup with k = 10 suppression | Tier 2 | T-110 |
| T-112 | Analyst weekly job, proposal format, rejection memory | Tier 2 | T-109, T-111 |
| T-113 | `gate.mode: hold` parse and verify behavior (fails like guard) | Tier 3 by content, two guards on different models | release pin move |
| T-114 | `waggle merge`: base config, local re-run, API approvals, head-pinned merge, merge record ref | Tier 3 by content | T-113 |
| T-115 | Post-merge audit workflow, breach ref, `on_breach: issue / revert` | Tier 3 (workflow, audit refs) | T-114 |
| T-116 | `doctor` capability probe and `hold` readiness findings | Tier 2 | T-113 |
| T-117 | Docs: held hive, write-access pattern, honest limits (`docs/gate.md` L13), CONTRIBUTING terms | Tier 2 (steering docs), scout owns CONTRIBUTING section 4 | T-113 to T-116 |
| T-118 | Pilot rollout: pilots move `open` to `hold` after 2 clean weeks | Tier 3 (`.waggle/control` per repo): Head Keeper signature | T-115, T-116 |

## 8. Decision requested

**Question:** Approve the direction for opt-in telemetry, the insights loop, and the held hive, with the recommendations in section 6?

**Options:**
1. Approve all three directions with section 6 recommendations; sequence as question 11.
2. Approve `hold` (part 3) now; hold telemetry and the insights loop for a second draft.
3. Approve telemetry and insights (parts 1 and 2) now; keep the pilots in `open` with scripted merges and defer `hold`.
4. Offer `hold` everywhere, Team plans included. **CUT? needs Head Keeper typed reason** (narrows core item 2).

**Hive recommendation:** Option 1.

**Keeper choice:** Option 1, approved by the Head Keeper, Paul Cleary, on 2026-11-23 at 10:35 PM Pacific Time (relayed in writing by the orchestrator), in the words "All the recs look good!". Open questions 1 to 11 follow the section 6 recommendations. Option 4 is not adopted and stays marked CUT. Code tickets T-104 to T-118 are opened as stubs (D-389).
