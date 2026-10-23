<!-- SPDX-License-Identifier: Apache-2.0 -->
# RFC note: repositioning Waggle as an agent-agnostic merge gate

- **Status:** Ruled by the Head Keeper on 2026-10-23 (D-229, D-231 to D-236); section 5 is the plan of record.
- **Date:** 2026-10-23
- **Inputs:** the prior-art review of 2026-10-22 (research output; its claims are treated as data and its source links are kept below), `README.md`, `docs/RFC-0003.md`, `.waggle/control/WAGGLE.md`, `.waggle/hive/tasks/`, and the code under `cmd/` and `internal/` at `4a65fc4`.
- **Sizes:** added non-test lines, bottom-up. Under D-185 each ticket brief doubles these before setting estimate, target and stop.

## 1. Problem

The review found that the swarm vision in RFC-0003 section 1 ("1 solo Keeper to 500+ distributed Keepers donating local compute and tokens") is already shipped by others and is mostly redundant for us. Gas Town with Beads and the Wasteland covers the coordinator, worktree workers, git-backed claims, a merge queue, ten-plus runtimes and a federated reputation board ([gastown](https://github.com/gastownhall/gastown), [beads](https://github.com/gastownhall/beads), [WASTELAND.md](https://github.com/gastownhall/gastown/blob/main/docs/WASTELAND.md)). Bernstein covers the deterministic, policy-gated orchestrator ([bernstein](https://github.com/sipyourdrink-ltd/bernstein)). Claude Code ships agent teams and the OS sandbox our Cells were going to rebuild ([agent teams](https://code.claude.com/docs/en/agent-teams), [sandboxing](https://code.claude.com/docs/en/sandboxing)), and OpenAI ships cross-model review as a Claude Code plugin ([codex-plugin-cc](https://github.com/openai/codex-plugin-cc)). Claims in git refs are common ([aco](https://github.com/overnightworks/aco), [grite](https://github.com/neul-labs/grite)). The one piece the review rated strongest is per-task path leases checked commit by commit at merge, under any orchestrator, because each vendor wants its own agent in charge and is unlikely to build a cross-vendor gate. The current pitch leads with the parts that lost and buries the part that might win. It also leads with vocabulary (Keeper, Hive, Comb, Cell, Nectar, Quorum) a project with no audience cannot afford, and "queen-led swarm" is already Ruflo's framing ([ruflo](https://github.com/ruvnet/ruflo)).

One correction to the review's data: it counts about 56,600 lines of Go. `wc -l` over `cmd/` and `internal/` gives 12,760 non-test and 15,770 test lines at `4a65fc4`. The conclusion does not change.

## 2. Proposed positioning

**Product one: Waggle Gate.** A merge gate for PRs written by agents, from any vendor or orchestrator. It enforces five things at merge, as a required GitHub check:

1. **A lease record per task.** Someone other than the agent (a human or the orchestrator) writes which directories a task may change, before the work starts. Stored as today in `refs/waggle/claims/ticket-<n>`.
2. **A commit-by-commit lease check.** Every commit in the PR is checked against its task's lease, so a lane or a squashed history cannot hide a stray path.
3. **Protected paths that need a named human.** Hardcoded control-plane paths (workflows, CI, devcontainer, Waggle's own config, dependency inputs) plus repo-defined ones, raise-only. A change there passes only with a listed approver's sign-off.
4. **Intent Packet trailers.** Ticket, risk tier and verdict as machine-readable trailers, so `git log` answers "which task and which review let this in".
5. **A decisions log.** An append-only `DECISIONS.md` the gate refuses to let anyone rewrite.

It runs under Gas Town, Copilot coding agent, Claude Code agent teams, Agent Orchestrator, Codex or a human with a branch. It does not schedule, prompt or run agents.

The sharp difference from CODEOWNERS and push rulesets ([rulesets docs](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets)): those are static per path for every PR. A lease is per task, so it catches an agent that wanders outside its assignment into a path nobody thought to protect.

**Product two: `waggle dispatch`.** A lease-scoped headless runner: claim, worktree, headless agent run, post-run lease and dirty checks, push tripwire, token caps, cost ledger. It is the gate's reference client and l19's own tool. It ships after the gate, and only grows a second runtime when someone outside l19 asks for one.

**One-sentence pitch:** "Waggle Gate lets you run parallel coding agents from any vendor and know that each PR touched only what its task was given, and that nothing in your control plane merged without a named human."

**First user:** a solo developer or a two to five person team already running two or more agents in parallel on one GitHub repo (Claude Code agent teams, Copilot coding agent, or Gas Town polecats), who has had an agent edit a file outside its task. l19 is user zero: this repo already runs the gate on every PR.

## 3. What exists and what is missing

| Package | Exists today | Missing for the gate pitch | Rough size |
| :--- | :--- | :--- | :--- |
| `internal/comb` (2,171) | Claim refs with CAS, lease validation, `CheckChanges`, `RequiredMinTier`, hardcoded Tier 2 and 3 lists, overlap check, `WORKPLAN.md` renderer | Repo-defined protected paths (raise-only, read from a Tier 3 config file); T-008 dependency inputs | 120 |
| `cmd/waggle` gate (642) | `gate verify` runs leases, tiers, case collisions, decisions; GitHub annotations; built from base, never head | Ticket binding beyond the `T-<id>:` subject (trailer or branch name); a Tier 3 approver check; a trailer check; tolerate a merge commit from GitHub's "Update branch" button, which fails the gate today | 300 |
| `cmd/waggle` claim verbs | None. Claims are written only inside `waggle dispatch`; `dispatch release` (T-030) frees them | `waggle claim` and `waggle release` usable by any human or orchestrator, with `--dirs`, `--gate`, `--ttl` and JSON output | 150 |
| `internal/forge` (1,039) | `Forge` interface, `none` and GitHub adapters (open, comment, merge with Intent Packet body), CI annotator | Read PR reviews at the head SHA for the approver check; T-018 phase C doctor, cut down to "is the gate a required check, can this clone push claim refs" | 250 |
| Gate workflow (`.github/workflows/waggle-gate.yml`, template) | Runs `waggle gate verify` in this repo only, built from source; the scaffold template omits that step (T-007 Q-T007-1); the relay job is an `echo` stub | Release binaries with checksums; a composite Action pinned by tag and SHA; the template runs the gate; the stub job removed | 100 Go, 200 YAML |
| `internal/decisions` (704) | Append-only writer, renderer, `gate decisions` | Make it optional per repo | 20 |
| `internal/scaffold` (1,506) | `init`, `upgrade`, `uninstall`, embedded roles and roster | `waggle init --gate-only`: workflow, config and `DECISIONS.md` only, no roles, roster or queen | 120 |
| `internal/config` (947) | Typed config, local config, roster, runtimes | `protected_paths` and `approvers` keys | 80 |
| `internal/dispatch` (3,231) | Claim, worktree, headless run, post-run check, guard dispatch, ledger, clean, release | Nothing for the gate. For product two: read the brief from a path or stdin instead of `.waggle/hive/tasks/` | 60 |
| `internal/runtime` (1,438) | Claude Code adapter: argv allowlist, settings isolation, env allowlist, stream parser, caps | For product two only: lease-derived sandbox settings for Claude Code's sandbox; a Codex CLI adapter later | 150, then 300 |
| `internal/usage` (313) | Ledger reader behind `waggle status` | Nothing | 0 |
| Docs | RFC-0003, README in hive vocabulary | README in plain terms; three integration recipes; a gate spec separate from the hive spec | 0 Go |

Gate total, bottom-up: about 1,150 Go and 200 YAML lines, about 2,700 after the D-185 doubling, across four tickets. Almost all of the hard part (claim storage, validation, the per-commit check, safe annotations) is built and reviewed.

## 4. Cut, freeze or keep

| Item | Call | Reason |
| :--- | :--- | :--- |
| T-028, T-029 | Keep | Already dispatched under D-226; they fix dispatch, which survives every option. |
| T-023 phase E (handoff loop, `MaxSessions`, target 350) | Freeze | Long-session handoff serves the queen-swarm model; lease-scoped headless runs should be short, so revisit only if real runs hit the cap. |
| T-024 Cells (six phases, about 2,500 target lines) | Cut | Claude Code's sandbox-runtime already does Seatbelt, bubblewrap, write paths and an egress allowlist; replace with a small settings generator from the lease ([sandbox-runtime](https://github.com/anthropics/sandbox-runtime)). |
| T-025 phases B to D (CODEOWNERS lines, SPDX sweep, DCO check) | Keep | Cheap, and a public OSS release needs them; run phase C in a window with no other claims under `internal/` or `cmd/`. |
| T-018 phase C (probes, doctor, target 650) | Keep, cut to about 300 | "Is the gate a required check" and "can this clone push claim refs" are the setup questions every new user hits; the nine-finding ruleset walk can wait. |
| T-021 phase C | Cut | Blocked on `waggled` and a session launcher that this positioning no longer needs. |
| T-008 (dependency inputs as Tier 3, about 50 lines) | Keep | Protecting `go.mod` and friends from agent PRs is a direct selling point, and it is the first protected-path rule users will want for their own lockfiles. |
| `waggled` and `run.sock` | Freeze | The gate needs no daemon and dispatch runs without one today. |
| Nectar | Cut | Wasteland stamps and Gittensor exist, the signal is gameable, and it matters only with many contributors ([Gittensor](https://subnetalpha.ai/subnet/gittensor/)). |
| Quorum | Cut | Quadratic voting needs Sybil resistance we have not designed, and there is one Keeper. |
| Relay mode | Cut from v0.1 | Unimplemented; the workflow ships an `echo` stub that implies a feature. Remove the job; keep the RFC text as a deferred design. |
| v0.2 roles (weaver, nurse, studio, assembly) | Cut | Unspecified, and roles are orchestrator territory we are leaving. |
| Multi-Keeper (contention, token donation) | Freeze | CAS claims already serve several writers with push access; the donation terms risk is unverified ([VentureBeat](https://venturebeat.com/ai/anthropic-cracks-down-on-unauthorized-claude-usage-by-third-party-harnesses/)). |
| Lanes (`int-<release>-<n>`) | Freeze | The per-commit check already makes lanes safe; no user needs them yet. |
| README vocabulary | Replace in user-facing docs | Lead with task, claim, lease, protected path, approver, reviewer; keep bee names for this hive's own roles and records. |
| D-205 second guard | No change here | D-226 revisits it after T-028 and T-029; the gate product does not depend on it. |

## 5. Sequence to a public v0.1 of the gate

Each step is one ticket, Tier 3, built through `waggle dispatch` in this hive. Sizes are bottom-up; the briefs double them.

1. **G1: ticket binding and protected paths.** `gate verify` binds a commit to a ticket by its `T-<id>:` subject, a `Waggle-Ticket:` trailer, or a `waggle/T-<id>` branch name, and fails on a conflict. Repo-defined protected paths, raise-only, from a Tier 3 config key; fold in T-008. Allow a PR's merge commits from the base branch only. Lease: `cmd/waggle/`, `internal/comb/`, `internal/config/`. About 350.
2. **G2: claim verbs for anyone.** `waggle claim T-<id> --dirs a,b --gate full` and `waggle release T-<id>` outside dispatch, reusing `AcquireLease` and the T-030 merged check, with JSON output for orchestrators. Lease: `cmd/waggle/`, `internal/comb/` (read only if possible). About 150.
3. **G3: named approver and trailers.** A protected-path change passes when a listed approver approved the PR at its head SHA (GitHub reviews API, read-only token); otherwise it fails as today and the admin bypass remains. Check Intent Packet trailers in the PR body. Lease: `internal/forge/`, `cmd/waggle/`. About 300.
4. **G4: distribution.** Tagged release binaries with checksums, a composite Action pinned by tag and SHA, `waggle init --gate-only`, the scaffold template runs the gate, the relay stub removed, T-018 phase C cut to the two doctor checks. Lease: `internal/scaffold/`, `internal/forge/`, `cmd/waggle/`, plus Head Keeper commits for workflows. About 350 Go and 200 YAML.
5. **G5: public release.** README rewritten in plain terms, a gate spec, three recipes (Claude Code agent teams, Copilot coding agent, Gas Town), T-025 phases B to D, a demo repo where an agent PR is refused for a path outside its lease, then the repo goes public. Mostly prose and Head Keeper commits.

G1 and G2 have disjoint leases apart from `cmd/waggle/` and can run in sequence in one week. G3 depends on G1. G4 depends on G1 and G3. At recent rates (about one Tier 3 ticket every one to two days with two guards) this is two to three weeks.

## 6. Risks and the kill criterion

- **Someone has to write the lease.** The gate's value is a lease set by someone other than the agent. If orchestrators will not call `waggle claim` and humans will not either, we fall back to the agent declaring its own lease, which is weak. G2's JSON verbs and the recipes exist to test this early.
- **Merges that skip PRs.** The gate binds only when merges go through a PR with required checks. An orchestrator that merges locally and pushes to `main` (Gas Town's Refinery may do this; not verified) needs a ruleset requiring PRs first, or the gate is advisory.
- **Custom refs friction.** GitHub's UI does not show `refs/waggle/`, contributors without push access cannot write them, and forks complicate it (review, differentiator 1).
- **Platform copy.** GitHub could add per-PR path scopes to rulesets or to Agent HQ; review estimate: about a week for an incumbent ([GitHub blog](https://github.blog/news-insights/product-news/welcome-home-agents/)).
- **Gate strictness.** No merge commits, `fetch-depth: 0`, a 500-commit cap and a required ticket prefix will frustrate a Copilot user on day one unless G1 relaxes the binding.
- **Process weight leaks in.** This hive runs 226 decisions and two-guard Tier 3 reviews; a gate user must get none of that by default.
- **No audience.** One maintainer, private repo, crowded category ([awesome-agent-orchestrators](https://github.com/andyrewlee/awesome-agent-orchestrators)). A gate is easier to adopt than an orchestrator, but distribution is still the bottleneck.
- **Sunk cost on dispatch.** About 4,700 non-test lines in `internal/dispatch` and `internal/runtime` serve product two; if the gate wins and dispatch does not, most of that stays internal.

**Kill criterion.** Eight weeks after the repo goes public, if fewer than three repos outside l19 run the gate as a required check (counted by public workflow references to the Action, or by users telling us) and no outside issue or PR has arrived, stop community work and keep Waggle as l19's internal tool, per the review's fallback. An earlier technical kill: if in G5's demo the gate cannot bind tickets for a Copilot or Claude Code agent-teams PR without changes to that agent's behavior, the "agent-agnostic" claim is false and the pitch needs rework before going public.

## 7. Questions for the Head Keeper

**Q1 (strategy). Approve the repositioning?**
1. Gate first, dispatch second, cut list as in section 4. **Recommended:** reuses the strongest built code, complements the incumbents instead of fighting them.
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
1. Cut; replace with a lease-to-sandbox-settings generator for product two. **Recommended:** about 150 lines against about 2,500 target lines, on a vendor-maintained primitive.
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
