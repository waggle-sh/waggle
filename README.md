# Waggle

**Waggle** is an open-source, git-native coordination, execution, and review framework for AI agent swarms. It scales from **1 solo Keeper** on a laptop to **500+ distributed Keepers** donating local compute and tokens.

## Quickstart (v0.1 Bootstrap)

```bash
# Build the waggle CLI
go build -o bin/waggle ./cmd/waggle

# Initialize a Waggle Hive in the current repository
./bin/waggle init --profile=cli --delivery=installed --comb-mode=direct

# Check Comb status
./bin/waggle status
```

## Roles

- **Keeper.** A human who runs Waggle on their own machine, donating compute and tokens. A Keeper talks only to the queen and answers decision cards.
- **Head Keeper.** The Keeper with final authority over a hive: approves every Tier 3 change, can force-release a stuck claim, and does the admin merge that repairs a corrupt decision log. Identified in v0.1 by an allow-list in a Tier 3 file; formerly called Master Keeper or Controller.
- **queen.** The entry session and the only agent a Keeper talks to. It asks decision cards and relays answers to `waggled`; it never waits on workers or writes code or specs.
- **scout.** Planning role: breaks goals into task briefs under 400 lines, sets each task's lease and gate, and owns the roadmap, specs, and these README sections, but never writes feature code. Scouts search ahead and dance directions to the hive.
- **forager.** Triage role: reproduces bugs, writes failing tests, maps the lease a ticket needs, and estimates size; it never writes implementation code and is safe to run unattended. Foragers bring material back from outside.
- **builder.** Implementation role: reads only its brief and leased files, works inside a Cell, runs tests and a self-review, then reports done, and never touches paths outside its lease. Builder bees make comb.
- **guard.** Review role: checks every acceptance criterion, saves evidence, and returns a verdict (`pass` or `must-fix`), a Risk Tier, and an Intent Packet. Runs on a different model at or above the builder's capability level and never fixes what it finds.
- **waggled.** The host daemon, a deterministic Go loop with no model. It starts Cells, routes worker messages over `.waggle/control/run.sock`, writes the decision log, enforces token caps, and renders derived views.

Records written before the role rename use older names for the scout, forager, and builder roles; the retired-names table in [CONTRIBUTING.md](CONTRIBUTING.md) maps them.

Planned for v0.2 and not yet specified. RFC-0003 groups them under UX, visuals, hygiene, and deliberation.

- **weaver.** Planned (v0.2).
- **nurse.** Planned (v0.2).
- **studio.** Planned (v0.2).
- **assembly.** Planned (v0.2).

## Terms

Any brief that adds a role or term updates these two sections in the same PR.

- **Hive.** A repository with Waggle installed. Waggle keeps its files in `.waggle/` (control plane in `.waggle/control/`, plans and records in `.waggle/hive/`), plus the gate workflow and one marked import line.
- **Comb.** The hive's shared coordination state, stored as git refs under `refs/waggle/`. The same code path serves one Keeper on a local repo and hundreds on a shared remote.
- **Cell.** The sandbox a worker runs in: its own git worktree and dev container, the repo read-only except leased directories, no host credentials, network egress blocked.
- **Swarm.** All the agents working in one hive at a time, across every Keeper. An informal word, not a setting.
- **Claim.** A Keeper's hold on one ticket, stored as the ref `refs/waggle/claims/ticket-<n>`. Renewals and takeovers add commits to the claim; releasing it deletes the ref.
- **Lease.** The directories a claim may change, checked when claimed and again, authoritatively, at merge. A lease can never include `.waggle/` or any Tier 3 path.
- **Gate.** The review a task passes before it merges. A **Light gate** covers small, isolated Tier 0 or 1 work and may ride a lane; a **Full gate** is its own guard review, required for Tier 2 and above and for anything over about 150 lines or touching schemas, auth, shared modules, dependencies, or steering files.
- **Risk Tier.** How much review a change needs, set by what it touches: Tier 0 comments and typos, Tier 1 small isolated changes with tests, Tier 2 normal logic and every steering file, Tier 3 control plane, CI, sandbox, and dependencies (Full gate plus Head Keeper approval). Hardcoded floors can be raised by config, never lowered.
- **Intent Packet.** The guard's short account of what changed, why, and the blast radius. It goes in the merge commit message with trailers for ticket, risk tier, and verdict.
- **Decision card.** A structured question the queen puts to a Keeper: at most four options, recommendation first, cost and reversibility stated. Approved answers go into the hive's `DECISIONS.md`.
- **Lane.** An integration branch (`int-<release>-<n>`) where several Light-gate tasks share one guard pass. Leases are still checked commit by commit.
- **Nectar.** Planned (v0.2): reputation a Keeper earns from merged commits that survive 30 days on `main` without a revert.
- **Quorum.** Planned (v0.2): hive voting weighted by the square root of Nectar, with a Head Keeper veto.
- **Capability level.** `frontier`, `standard`, or `fast`. Roles declare a minimum level, never a model name, and hive config maps levels to concrete models.
- **Own honey.** Building Waggle with Waggle: the Waggle repo is itself a hive that runs a pinned, known-good Waggle build.

## Documentation
- [RFC-0003: Waggle Architecture & v0.1 Bootstrap Specification](docs/RFC-0003.md)
- [Contributing: product and hive](CONTRIBUTING.md)

## License
Waggle, the `waggle` CLI and the `waggled` daemon, is licensed under the Apache License 2.0 (`LICENSE`, `NOTICE`). The templates `waggle init` writes into your repository (everything from `internal/scaffold/templates/`: your `.waggle/` files and `.github/workflows/waggle-gate.yml`) are licensed under MIT-0 (`LICENSES/MIT-0.txt`): use, change and relicense them with no attribution. Copyright 2026 Level 19 Enterprises, LLC. Contributions are accepted under the Developer Certificate of Origin (see `CONTRIBUTING.md`).
