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

### v0.3 Quorum phase 3

RFC-0006 phase 3 on top of the v0.2 Quorum rounds. Sequencing, not a cut: every RFC phase 3 item stays in T-069. Phase A waits for T-054 phases A to D and one real round replayed (D-377).

T-069 · Quorum RFC phase 3 (area rounds, alias merge, collusion flags, commit-reveal) · L (four phases) · proposed · v0.3 (named by the Head Keeper, D-378) · product

## Backlog (honey and multi-Keeper)

T-050 · Nectar design brief (scoring rules, revert window, attribution from claims) · S · merged (#234) · v0.2 · design
T-051 · Nectar scorer (git history + claim owners, 30-day survival, revert clawback) · M · merged (phases A to D2, #241 to #254) · v0.2 · product
T-052 · Nectar status surfaces (CLI / derived view; no public dashboard required) · M · merged (phases A and B, #258, #260) · v0.2 · product
T-053 · Quorum design brief (ballot store, sqrt weight, Head Keeper veto, Sybil notes) · S · merged (#235) · v0.2 · design
T-054 · Quorum voting MVP (roadmap/issue priority ballots; four phases, nothing cut) · L · code merged through phase D (#279); pin move is PR #283, open · v0.2 · product
T-071 · Signed Head Keeper approval for protected paths (T-054 Quorum family; verify-leases-and-tiers accepts it so gate/, .github/workflows and control path PRs merge without an admin bypass) · M · merged (#273) · v0.1 (public gate; a gate change, sequenced after T-054 phase A) · product
T-055 · Multi-Keeper story restore (RFC + README/CONTRIBUTING cut-list unwind; relay design spike) · S · merged (#236) · v0.2 · docs+design
T-070 · Quorum RFC phase 2 (fork contributor export and import, relay transport with T-055) · TBD · proposed · v0.2 · product
T-072 · waggle force-release (the Head Keeper clears a stuck or malformed claim, with an audit trail) · M · merged (phases A, B and the README line: #267, #271, #272) · v0.1 · product
T-073 · Case-folded claim overlap checks (carries out D-038; opened by T-072 Q-T072-5) · S to M · merged (phases A and B: #266, #274) · v0.1 · product
T-074 · Gate head tree collision check: Unicode normalization variants (T-073 Q-T073-8) · S to M · proposed (scout brief merged, #277; no build yet) · v0.1 · product
T-075 · Doctor claims finding: collision count (T-073 Q-T073-5; after T-072 phase B and T-073 phase A) · S · proposed (scout brief merged, #278; no build yet) · v0.1 · product
T-081 · One claim sync per waggle status run (T-073 phase B guard note) · S · proposed · v0.1 · product
T-082 · Status test for a failed collision scan (T-073 phase B guard note) · S · proposed · v0.1 · product
T-083 · multi-keeper.md wording: the collision never grows (T-073 phase B guard note) · S · proposed · v0.1 · docs
T-084 · waggle hives status shows case collisions (T-073 phase B guard note) · S · proposed · v0.1 · product

## Backlog (public source release polish)

Ported on 2026-11-23 from dev-mgr's `docs/BACKLOG.md` (PR #294, superseded). Every ticket is decision-free and touches Tier 1 or Tier 2 paths only. Each is a stub: the scout writes the full brief (lease, target, stop and planned cut under D-185) before any dispatch.

T-087 · Top-level help and version flags, usage on the right stream · S · proposed · v0.1 · product
T-088 · waggle init testable and strict about its arguments · S · proposed · v0.1 · product
T-089 · Docs drift test: usage lines and command list · S · proposed · v0.1 · product
T-090 · Claim and release error messages name the flag and the fix · S · proposed · v0.1 · product
T-091 · Clear message when run outside a git repo or without a remote · S · proposed · v0.1 · product
T-092 · Gate tests: submodule end to end and rename out of a Tier 3 path · S · proposed · v0.1 · product
T-093 · Gate tests: awkward path names and claim directory spelling · S · proposed · v0.1 · product
T-094 · Test the install script offline · S · proposed · v0.1 · product
T-095 · Docs cite source by line number: audit and guard · S · proposed · v0.1 · docs
T-096 · Runnable gate walkthrough in examples/ · S · proposed · v0.1 · docs
T-097 · Troubleshooting page built from real gate messages · S · proposed · v0.1 · docs
T-098 · Empty states for status and doctor · S · proposed · v0.1 · product
T-099 · Golden tests for --json output · S · proposed · v0.1 · product
T-100 · One local check script that matches CI and the release matrix · S · proposed · v0.1 · product

## Still deferred (not honey)

- v0.2 roles weaver / nurse / studio / assembly: unspecified; open only when a measured job needs them.
- Lanes (`int-<release>-<n>`): frozen until a user needs shared Light-gate review.
- `waggled` / run.sock: frozen while dispatch + gate cover the path.
- Full Cell runtime (T-024): remains cut per D-231 unless a typed reverse card lands; lease-to-sandbox-settings stays the substitute for dispatch.
