<!-- SPDX-License-Identifier: Apache-2.0 -->
# RFC note: repositioning Waggle as an agent-agnostic merge gate

- **Status:** Ruled by the Head Keeper on 2026-10-23 (D-229, D-231 to D-236); section 5 is the plan of record for gate sequencing. D-366 restores Nectar, Quorum, and multi-Keeper as core vision; section 4 rows for those three items are amended below. Cells (D-231) stay cut unless reversed on their own card.
- **Date:** 2026-10-23
- **Public text:** The prior-art survey, competitor assessments, and the internal sizing and schedule notes are removed. Ruled product decisions stay, including the D-235 eight-week, three-outside-repos kill criterion.
- **Inputs:** `README.md`, `docs/RFC-0003.md`, `.waggle/control/WAGGLE.md`, `.waggle/hive/tasks/`, and the code under `cmd/` and `internal/` at `4a65fc4`.

## 1. Problem

RFC-0003 section 1 leads with a swarm of Keepers. The v0.1 product is narrower. Waggle Gate is a merge gate any agent or orchestrator can run under. The per-task path lease, checked on every commit at merge, is the part that ships first. User-facing docs use plain terms. Bee names stay in this hive's own roles and records (D-236).

Amendment (D-366): coverage of parts of a swarm coordinator does not close Waggle's job. The gate remains the v0.1 wedge. The 1-to-many Keepers story, Nectar, and Quorum stay in the product vision. They are sequenced after the gate, not discarded.

## 2. Proposed positioning

**Product one: Waggle Gate.** A merge gate for PRs written by agents, from any vendor or orchestrator. It enforces five things at merge, as a required GitHub check:

1. **A lease record per task.** Someone other than the agent (a human or the orchestrator) writes which directories a task may change, before the work starts. Stored as today in `refs/waggle/claims/ticket-<n>`.
2. **A commit-by-commit lease check.** Every commit in the PR is checked against its task's lease, so a lane or a squashed history cannot hide a stray path.
3. **Protected paths that need a named human.** Hardcoded control-plane paths (workflows, CI, devcontainer, Waggle's own config, dependency inputs) plus repo-defined ones, raise-only. A change there passes only with a listed approver's sign-off.
4. **Intent Packet trailers.** Ticket, risk tier and verdict as machine-readable trailers, so `git log` answers "which task and which review let this in".
5. **A decisions log.** An append-only `DECISIONS.md` the gate refuses to let anyone rewrite.

It runs under Copilot coding agent, Claude Code agent teams, Agent Orchestrator, Codex or a human with a branch. It does not schedule, prompt or run agents.

The sharp difference from CODEOWNERS and push rulesets ([rulesets docs](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets)): those are static per path for every PR. A lease is per task, so it catches an agent that wanders outside its assignment into a path nobody thought to protect.

**Product two: `waggle dispatch`.** A lease-scoped headless runner: claim, worktree, headless agent run, post-run lease and dirty checks, push tripwire, token caps, cost ledger. It is the gate's reference client. It ships after the gate, and only grows a second runtime when a second user asks for one.

**One-sentence pitch:** "Waggle Gate lets you run parallel coding agents from any vendor and know that each PR touched only what its task was given, and that nothing in your control plane merged without a named human."

**First user (v0.1 gate):** a solo developer or a two to five person team already running two or more agents in parallel on one GitHub repo (Claude Code agent teams or Copilot coding agent), who has had an agent edit a file outside its task. This repository already runs the gate on every pull request.

**North-star users (unchanged by gate-first):** multi-Keeper teams on public or private repos (including companies with many teams on one private repo) and open source maintainers who need claims, a merge gate, and later honey (Nectar + Quorum) so durable contributors gain priority on roadmap and issues (D-366).

## 3. What exists and what is missing

At the date of this note, claim storage, lease validation, the per-commit check, and safe annotations were already built. Section 5 is what was still missing for a public gate: ticket binding, claim verbs anyone can run, a named approver, and distribution. The internal package inventory and line-count sheet are omitted.

## 4. Cut, freeze or keep

| Item | Call | Reason |
| :--- | :--- | :--- |
| T-028, T-029 | Keep | Already dispatched under D-226; they fix dispatch, which survives every option. |
| T-023 phase E (handoff loop, `MaxSessions`, target 350) | Freeze | Long-session handoff serves the queen-swarm model; lease-scoped headless runs should be short, so revisit only if real runs hit the cap. |
| T-024 Cells (six phases, about 2,500 target lines) | Cut | Replace with a small settings generator from the lease. A vendor sandbox already covers the isolation primitives Cells would have rebuilt. |
| T-025 phases B to D (CODEOWNERS lines, SPDX sweep, DCO check) | Keep | Cheap, and a public OSS release needs them; run phase C in a window with no other claims under `internal/` or `cmd/`. |
| T-018 phase C (probes, doctor, target 650) | Keep, cut to about 300 | "Is the gate a required check" and "can this clone push claim refs" are the setup questions every new user hits; the nine-finding ruleset walk can wait. |
| T-021 phase C | Cut | Blocked on `waggled` and a session launcher that this positioning no longer needs. |
| T-008 (dependency inputs as Tier 3, about 50 lines) | Keep | Protecting `go.mod` and friends from agent PRs is a direct selling point, and it is the first protected-path rule users will want for their own lockfiles. |
| `waggled` and `run.sock` | Freeze | The gate needs no daemon and dispatch runs without one today. |
| Nectar | Restore (v0.2) | D-366. Core vision. Ship after the public gate; score merged commits that survive 30 days on main without revert, using claim records the gate already writes. Gameability and overlap with other reputation systems are design risks for T-050/T-051, not grounds to delete the feature. |
| Quorum | Restore (v0.2) | D-366. Core vision. Quadratic voting with weight sqrt(Nectar) and Head Keeper veto. Sybil resistance is a design problem for T-053/T-054, not a cut. "One Keeper today" is a temporary hive fact, not the product ceiling. |
| Relay mode | Cut from v0.1; revisit for multi-Keeper | Still out of the public gate slice. Needed later so contributors without push access can claim; track under T-055. |
| v0.2 roles (weaver, nurse, studio, assembly) | Cut | Unspecified. Roles are orchestrator territory, which is not the v0.1 product. |
| Multi-Keeper (contention, coordination) | Restore in full (docs now; build after gate) | D-366. Core vision (1 solo Keeper to many Keepers on one repo, public or private). Each Keeper runs their own agents on their own plan. CAS claims stay the base. Contention docs and the claim path for contributors without push access ride T-055. |
| Lanes (`int-<release>-<n>`) | Freeze | The per-commit check already makes lanes safe; no user needs them yet. |
| README vocabulary | Replace in user-facing docs | Lead with task, claim, lease, protected path, approver, reviewer; keep bee names for this hive's own roles and records. |
| D-205 second guard | No change here | D-226 revisits it after T-028 and T-029; the gate product does not depend on it. |

## 5. Sequence to a public v0.1 of the gate

Each step is one ticket, Tier 3, built through `waggle dispatch` in this hive.

1. **G1: ticket binding and protected paths.** `gate verify` binds a commit to a ticket by its `T-<id>:` subject, a `Waggle-Ticket:` trailer, or a `waggle/T-<id>` branch name, and fails on a conflict. Repo-defined protected paths, raise-only, from a Tier 3 config key; fold in T-008. Allow a PR's merge commits from the base branch only. Lease: `cmd/waggle/`, `internal/comb/`, `internal/config/`.
2. **G2: claim verbs for anyone.** `waggle claim T-<id> --dirs a,b --gate full` and `waggle release T-<id>` outside dispatch, reusing `AcquireLease` and the T-030 merged check, with JSON output for orchestrators. Repo-defined protected paths (`gate.protected_paths`) also apply at lease time and in dispatch's post-run check (moved here from G1 by T-031). Lease: `cmd/waggle/`, `internal/comb/`, `internal/dispatch/`.
3. **G3: named approver and trailers.** A protected-path change passes when a listed approver approved the PR at its head SHA (GitHub reviews API, read-only token); otherwise it fails as today and the admin bypass remains. Check Intent Packet trailers in the PR body. Lease: `internal/forge/`, `cmd/waggle/`.
4. **G4: distribution.** Tagged release binaries with checksums, a composite Action pinned by tag and SHA, `waggle init --gate-only`, the scaffold template runs the gate, the relay stub removed, T-018 phase C cut to the two doctor checks. Lease: `internal/scaffold/`, `internal/forge/`, `cmd/waggle/`, plus Head Keeper commits for workflows.
5. **G5: public release.** README rewritten in plain terms, a gate spec, two recipes (Claude Code agent teams, Copilot coding agent), T-025 phases B to D, a demo repo where an agent PR is refused for a path outside its lease. The repo goes public only after redaction, the history rewrite and the Head Keeper's flip card, each on its own card (D-235). Mostly prose and Head Keeper commits.

G1 and G2 both lease `cmd/waggle/` and `internal/comb/`, so they run in sequence. G3 depends on G1. G4 depends on G1 and G3.

## 6. Risks and the kill criterion

- **Someone has to write the lease.** The gate's value is a lease set by someone other than the agent. If orchestrators will not call `waggle claim` and humans will not either, we fall back to the agent declaring its own lease, which is weak. G2's JSON verbs and the recipes exist to test this early.
- **Merges that skip PRs.** The gate binds only when merges go through a PR with required checks. An orchestrator that merges locally and pushes to `main` needs a ruleset requiring PRs first, or the gate is advisory.
- **Custom refs friction.** GitHub's UI does not show `refs/waggle/`, contributors without push access cannot write them, and forks complicate it.
- **Gate strictness.** No merge commits, `fetch-depth: 0`, a 500-commit cap and a required ticket prefix will frustrate a Copilot user on day one unless G1 relaxes the binding.

**Kill criterion.** Eight weeks after the repo goes public (at G5, after redaction, the history rewrite and the Head Keeper's flip card, D-235), if fewer than three repos outside l19 run the gate as a required check (counted by public workflow references to the Action, or by users telling us) and no outside issue or PR has arrived, stop community work and keep Waggle as l19's internal tool, per the review's fallback. An earlier technical kill: if in G5's demo the gate cannot bind tickets for a Copilot or Claude Code agent-teams PR without changes to that agent's behavior, the "agent-agnostic" claim is false and the pitch needs rework before going public.

## 7. Questions for the Head Keeper

**Q1 (strategy). Approve the repositioning?**
1. Gate first, dispatch second, cut list as in section 4. **Recommended:** reuses the strongest built code.
2. Gate only; dispatch stays internal and gets no public docs.
3. Stay the course on RFC-0003.

Ruling: option 1 (D-229).

**Q2 (design). Where does the lease record live?**
1. Keep `refs/waggle/claims/ticket-<n>`. **Recommended:** built, tested, CAS-safe, and readable by the gate from the base side.
2. A PR label or check-run that a maintainer sets.
3. A lease file committed under a protected path.

Ruling: option 1 (D-232).

**Q3 (design). How does a protected-path change get its named human?**
1. Approving review from a listed approver at the head SHA, admin bypass kept as the fallback. **Recommended:** visible in the PR, no admin rights needed.
2. Admin bypass only, as today.
3. Delegate to CODEOWNERS plus "Require review from Code Owners".

Ruling: option 1 (D-233).

**Q4 (design). How does a commit name its ticket?**
1. Subject prefix, `Waggle-Ticket:` trailer or branch name, first match wins, a conflict fails. **Recommended:** works for agents that do not control commit subjects.
2. Subject prefix only, as today.

Ruling: option 1 (D-234).

**Q5 (scope). T-024 Cells?**
1. Cut; replace with a lease-to-sandbox-settings generator for product two. **Recommended.**
2. Freeze the brief unchanged.
3. Build as designed.

Ruling: option 1 (D-231).

**Q6 (publish). When does the repo go public, and is the kill criterion acceptable?**
1. Public at G5, with the eight-week, three-outside-repos criterion. **Recommended:** a working demo first, then a dated test.
2. Public now, before G1 to G4.
3. Stay private until a second user is found privately.

Ruling: option 1 (D-235).

**Q7 (policy). User-facing vocabulary?**
1. Plain terms in README and the gate spec; bee names stay for this hive's roles and records. **Recommended:** no audience yet to carry jargon.
2. Keep the current vocabulary everywhere.

Ruling: option 1 (D-236).
