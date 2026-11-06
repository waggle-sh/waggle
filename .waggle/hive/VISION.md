# Waggle vision

Owned by the Head Keeper. This file is the bar for product direction. Agents and leads may sequence work; they may not quietly shrink what is below.

## What Waggle is for

Waggle is an open source, git-native tool you drop into any greenfield or brownfield repo, public or private, so many humans, each running their own agent teams, can work the same codebase without stepping on each other. Coding speed is not the hard part anymore. Review bandwidth, merge coordination, and keeping agents inside the paths they were given are.

I have this problem at work too: many teams and many agents on one repo. Waggle is how I want that to work for us and for open source maintainers who face the same mess.

## Who it is for

- Multi-Keeper teams: more than one human, each with agents, sharing one repo through claims and checks.
- Companies with many teams on one private repo (my work problem).
- Open source maintainers who want outside contributors (and their agents) to earn a say by shipping work that sticks.
- Solo Keepers who start at one person and grow without rewriting the model.
- Level 19's own repos ("own honey"): we build Waggle with Waggle and measure before/after on every adopt.

The old RFC headline of 1 solo Keeper up to 500+ distributed Keepers is the scale story we still mean. We ship the gate first because the claim records are what later reputation measures. We do not redefine the product as "single-owner merge gate only."

## Core features (do not cut without my typed reason)

These stay in scope. A proposal that drops, freezes, or narrows any of them is a CUT and needs my typed reason (not a tap on a recommended option).

1. **Claims and path leases.** Per-task holds in git refs (`refs/waggle/claims/ticket-<n>`) naming which directories a task may change. Optimistic at claim time, authoritative at merge.
2. **Merge gate.** Required check that each commit stays inside its task's lease, binds the commit to a task, protects control-plane paths, and keeps an append-only decisions log.
3. **Multi-Keeper.** The product is for many humans on one repo, not only a solo owner. Contention, identity, and (later) how untrusted contributors claim without push access stay on the path.
4. **Open source drop-in.** Waggle itself is open source; it drops into any repo, public or private, greenfield or brownfield. Small footprint. Licenses stay Apache-2.0 for the CLI and MIT-0 for templates.
5. **Nectar.** Reputation from merged commits that survive 30 days on `main` without a revert. That is the durable half of "honey."
6. **Quorum.** Roadmap and issue priority via quadratic voting with weight `sqrt(Nectar)`, plus Head Keeper veto. That is how higher honey means more say over what gets picked up next.

Honey = Nectar + Quorum. Gate ships public first; Nectar and Quorum follow in v0.2 (or a named follow-on release) because the gate's claim and merge records are what Nectar scores.

## Explicitly out of scope (for now)

- Competing with every orchestrator on scheduling, prompts, and role swarms. We stay agent-agnostic at the gate; `waggle dispatch` is our reference client, not the only way in. [inferred from Oct 23 gate-first sequencing the Head Keeper kept]
- Rebuilding a full Cell sandbox stack when a vendor sandbox already covers the same targets. Sandbox settings derived from a lease are fine; a giant Cell rewrite is not required to keep the vision. [inferred: D-231 Cell cut can stay as sequencing unless I reverse it by name]
- Inventing unspecified v0.2 bee roles (weaver, nurse, studio, assembly) before Nectar and Quorum have a design. Deliberation for Quorum may need a role later; that is a design card, not a silent cut of honey itself.

## Vision guard

1. Before any strategy or scope card that cuts, freezes, or narrows a core item above, the lead must mark the option as **CUT** and quote the core item.
2. A CUT does not land unless I type a reason in my own words (or an explicit "I cut X because ..."). Tapping a recommended option, or approving a bundle that hides a cut inside a longer list, does not count.
3. Gate-first sequencing is allowed and preferred. "Ship later" is not the same as "cut." Deferred work stays on the roadmap with a ticket and a release.
4. User-facing docs may keep plain terms (task, claim, lease, gate). Hive records may keep bee names. Plain vocabulary is not a cut of Nectar, Quorum, or multi-Keeper.
5. If this file and a decision or RFC note disagree, this file wins until I update it.

## Sources

- RFC-0003 core thesis and Nectar/Quorum (docs/RFC-0003.md).
- The Head Keeper's framing of honey, greenfield/brownfield drop-in, and many humans' agent teams on one repo (2026-11-05 vision notes).
- Oct 23 repositioning (docs/rfc-notes/repositioning.md) and D-229, D-231..D-236: gate-first sequencing kept; Nectar, Quorum, and multi-Keeper cuts reversed by the restore decision.
