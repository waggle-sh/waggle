<!-- SPDX-License-Identifier: Apache-2.0 -->
# Contributing to Waggle

This file is for people building Waggle itself. If you use Waggle in your own repo, the [README](README.md) and the [gate spec](docs/gate.md) are what you need; nothing here applies to your repo.

## Contributing

- **Build and test:** `scripts/check.sh` runs `go build ./...`, `go vet ./...`, the gofmt check and `go test -race ./...` as CI does, and stops at the first failing step. `--quick` skips the race tests; `--cross` also cross-builds `./cmd/waggle` for the release matrix and vets darwin and windows.
- **Sign off every commit** with `git commit -s`. The DCO check fails a PR whose commits lack `Signed-off-by` (section 6).
- New files start with an SPDX header; no em dashes or en dashes.
- **Where work comes from.** Work items normally come from issues, and each task is claimed before work starts. Humans can also open issues: use the bug or feature template, and report vulnerabilities privately as described in [SECURITY.md](SECURITY.md). A maintainer may need to write a claim naming your branch before the repo's gate passes on your PR. To claim an issue yourself with only `gh`, follow the [Level 0 convention](examples/agents-md-claims.md).
- Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

## 1. Two layers in this repo

- **Product:** Waggle itself. `cmd/`, `internal/` (including `internal/scaffold/templates/`, which is what every other hive receives), `go.mod`, `LICENSE`, `README.md`, `docs/` (the gate spec `docs/gate.md`, the recipes in `docs/recipes/`, the design history `docs/RFC-0003.md`), and the product CI workflow. This is what ships.
- **Hive** (layer sense): our own installation of Waggle, used to build the product (own honey, defined in section 4). `.waggle/`, `.github/workflows/waggle-gate.yml`, and the marked import block in `CLAUDE.md`. Nothing in the hive ships.

## 2. Rules

- Our hive runs a pinned, known-good Waggle build recorded as `waggle.pin` in `.waggle/control/config.yaml`, never the code under construction. Changing the pin is a deliberate upgrade logged with `waggle decide` (D-041).
- Template changes reach our hive only through `waggle upgrade`, run from the target build, which becomes the new pin when the hive PR merges. Do not hand-edit hive copies of templates (D-042, D-063).
- Worker leases never include `.waggle/`; hive files change only through waggle commands or the Head Keeper. Waggle enforces this for every hive (D-043).
- A change that touches both layers is split: a product ticket first, then a hive upgrade.
- `DECISIONS.md` records how our hive decided. Anything users rely on also lands in the product docs (D-045).

## 3. Conventions only this repo uses

These are not part of Waggle's schema.

- Every brief carries a `Layer: product | hive` line.
- CODEOWNERS protects `internal/scaffold/templates/` (D-034) and `README.md` (D-065), both via T-015; read `.github/CODEOWNERS` for what it covers now.
- `CONTRIBUTING.md` is not Tier 2 (D-058).

## 4. Words in this repo's records

Our hive's records (briefs, decisions, reviews) use bee words, and [docs/glossary.md](docs/glossary.md) gives each one a short definition. The docs site uses them too, each with a short plain gloss, as [the hive page](https://getwaggle.dev/docs/hive/) does. Where a plain term exists (task, claim, lease, protected path, approver, reviewer, maintainer, merge gate, required check), the gloss uses it. Keeping this section current:

- Any brief that introduces a role or term updates this section in the same PR. The guard checks this: a new capitalized or code-formatted term in a brief with no entry here is a must-fix.
- Each entry is one or two plain sentences. No decision IDs in an entry; sources stay in the brief that adds it.
- The scout owns this section. Changes take a Full gate.
- Retired names stay listed in section 5 with a pointer to the current term, so old commits, decisions, and review records stay readable.
- Product design that users rely on also lands in the product docs (D-045); `DECISIONS.md` records how our hive decided, not how Waggle works.

Naming rule (D-073): bee terms are used only for concepts Waggle invents (Hive, Comb, roles). Things users already know keep their standard industry words (forge, PR, CI, commit, ref), and so do precise mechanisms (Gate, Lease, Claim, Risk Tier, Intent Packet).

No em dashes or en dashes in code, docs, or commits.

### Roles

- **Keeper.** A human who runs Waggle on their own machine, donating compute and tokens. A Keeper talks only to the queen and answers decision cards.
- **Head Keeper.** The Keeper with final authority over a hive: approves every Tier 3 change, can force-release a stuck claim, and does the admin merge that repairs a corrupt decision log. Identified in v0.1 by an allow-list in a Tier 3 file; formerly called Master Keeper or Controller.
- **queen.** The entry session and the only agent a Keeper talks to. It asks decision cards and never waits on workers or writes code or specs.
- **scout.** Planning role: breaks goals into task briefs under 400 lines, sets each task's lease and gate, and owns the roadmap, specs, and this section, but never writes feature code.
- **forager.** Triage role: reproduces bugs, writes failing tests, maps the lease a ticket needs, and estimates size; it never writes implementation code.
- **builder.** Implementation role: reads only its brief and leased files, runs tests and a self-review, then reports done, and never touches paths outside its lease.
- **guard.** The review bee, run with `waggle dispatch --role guard`. It is never the merge check, which is called the gate. Review role: checks every acceptance criterion, saves evidence, and returns a verdict (`pass` or `must-fix`), a Risk Tier, and an Intent Packet. Runs on a different model at or above the builder's capability level and never fixes what it finds.

### Terms

- **Hive.** A repository with Waggle installed. Waggle keeps its files in `.waggle/` (control plane in `.waggle/control/`, plans and records in `.waggle/hive/`), plus the gate workflow and one marked import line.
- **Comb.** The hive's shared coordination state, stored as git refs under `refs/waggle/`.
- **Swarm.** All the agents working in one hive at a time. An informal word, not a setting.
- **Claim.** A hold on one ticket, stored as the ref `refs/waggle/claims/ticket-<n>`. It records the owner, expiry, gate level, leased directories and the PR branches it accepts (a list, up to 4). Renewals and takeovers add commits to the claim; releasing it deletes the ref.
- **Lease.** The directories a claim may change, checked when claimed and again, authoritatively, at merge. A lease can never include `.waggle/` or any Tier 3 path.
- **Gate.** The required merge check `verify-leases-and-tiers`, which verifies leases and risk tiers on every PR. It is always called the gate, never the guard (the guard is the review bee, see Roles). Also the review a task passes before it merges. A **Light gate** covers small, isolated Tier 0 or 1 work; a **Full gate** is its own guard review, required for Tier 2 and above and for anything over about 150 lines or touching schemas, auth, shared modules, dependencies, or steering files.
- **Risk Tier.** How much review a change needs, set by what it touches: Tier 0 comments and typos, Tier 1 small isolated changes with tests, Tier 2 normal logic and every steering file, Tier 3 control plane, CI, sandbox, and dependencies (Full gate plus Head Keeper approval). Hardcoded floors can be raised by config, never lowered.
- **Intent Packet.** The guard's short account of what changed, why, and the blast radius. Its trailers for ticket, risk tier and verdict are read from the PR description, not the merge commit (`cmd/waggle/gate_trailers.go:27`).
- **Decision card.** A structured question the queen puts to a Keeper: at most four options, recommendation first, cost and reversibility stated. Approved answers go into the hive's `DECISIONS.md`.
- **Capability level.** `frontier`, `standard`, or `fast`. Roles declare a minimum level, never a model name, and hive config maps levels to concrete models.
- **Own honey.** Building Waggle with Waggle: the Waggle repo is itself a hive that runs a pinned, known-good Waggle build.

### Coming soon and roadmap

- **Nectar.** Coming soon (restored, D-366): reputation from merged commits that survive 30 days on main without revert. `waggle nectar backfill` is on main; the scorer is not merged yet.
- **Quorum.** Roadmap (restored, D-366): hive voting weighted by sqrt(Nectar), with a veto for the project owner (the Head Keeper today). Design only, no code.

### Cut or frozen

- **weaver, nurse, studio, assembly.** Cut: the unspecified v0.2 roles.
- **Lane.** Frozen: an integration branch (`int-<release>-<n>`) where several Light-gate tasks share one guard pass. Not used.
- **relay.** The stub job of the old gate workflow, removed from the template.
- **Cell.** Cut: the worker sandbox with its own dev container. Sandboxing is left to the agent tool; the design history is in `docs/RFC-0003.md`.
- **waggled.** Frozen: the host daemon. The gate needs no daemon and none ships.

## 5. Retired names

| Retired | Use instead | Retired by |
| :--- | :--- | :--- |
| Master Keeper | Head Keeper | D-031 |
| Controller | Head Keeper | D-031 |
| architect (before T-020) | scout | D-074 |
| scout (before T-020) | forager | D-074 |
| forager (before T-020) | builder | D-074 |
| drone | none; retired | D-073 |

In records written before T-020, read architect as Scout, scout as Forager, and forager as Builder.

## 6. Sign-off (DCO)

- Every commit carries a `Signed-off-by: Name <email>` trailer matching the commit author, certifying the Developer Certificate of Origin 1.1 (`https://developercertificate.org/`). Use `git commit -s`; for merges, `git merge --signoff`; to fix a branch, `git rebase --signoff <base>`.
- Sign-off certifies that you have the right to submit the change under the file's license. It is a personal statement by a person. The trailer for work a worker commits under your identity is yours.
- Commits carry no AI attribution trailers.
- New files start with the SPDX header for their path. Reviewers require it; an automatic check is planned and does not run yet.
