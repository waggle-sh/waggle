<!-- SPDX-License-Identifier: MIT-0 -->
# ROADMAP

Owned by the scout; Full gate. Vision bar: VISION.md (core items need a typed Head Keeper reason to cut).

## Releases

### v0.1 Public Waggle Gate (in flight)

Ship the agent-agnostic merge gate (claims, per-commit lease check, protected paths, trailers, decisions log) and `waggle dispatch` as the reference client. Plan of record: docs/rfc-notes/repositioning.md section 5 (G1..G5), as amended by later tickets. Marketing may lead with the gate (D-287). The long product story stays multi-Keeper and honey (D-366).

### v0.2 Honey (Nectar + Quorum)

After the gate is public and claim/merge history exists in real repos:

- Nectar: reputation from merged commits that survive 30 days on main without revert, attributed through claim owners / keeper_id.
- Quorum: quadratic votes on roadmap and issue priority, weight = sqrt(Nectar), Head Keeper veto.
- Multi-Keeper hardening: contention docs, identity rules, and (when designed) relay or equivalent so contributors without push access can still claim.

Gate-first is sequencing, not a cut.

## Backlog (honey and multi-Keeper)

T-050 · Nectar design brief (scoring rules, revert window, attribution from claims) · S · proposed · v0.2 · design
T-051 · Nectar scorer (git history + claim owners, 30-day survival, revert clawback) · M · proposed · v0.2 · product
T-052 · Nectar status surfaces (CLI / derived view; no public dashboard required) · M · proposed · v0.2 · product
T-053 · Quorum design brief (ballot store, sqrt weight, Head Keeper veto, Sybil notes) · S · proposed · v0.2 · design
T-054 · Quorum voting MVP (roadmap/issue priority ballots; four phases, nothing cut) · L · proposed · v0.2 · product
T-055 · Multi-Keeper story restore (RFC + README/CONTRIBUTING cut-list unwind; relay design spike) · S · proposed · v0.2 · docs+design
T-069 · Quorum RFC phase 3 (area rounds, alias merge, collusion flags, commit-reveal) · TBD · proposed · follow-on release (named by the Head Keeper, not v0.2) · product
T-070 · Quorum RFC phase 2 (fork contributor export and import, relay transport with T-055) · TBD · proposed · v0.2 · product

## Still deferred (not honey)

- v0.2 roles weaver / nurse / studio / assembly: unspecified; open only when a measured job needs them.
- Lanes (`int-<release>-<n>`): frozen until a user needs shared Light-gate review.
- `waggled` / run.sock: frozen while dispatch + gate cover the path.
- Full Cell runtime (T-024): remains cut per D-231 unless a typed reverse card lands; lease-to-sandbox-settings stays the substitute for dispatch.
