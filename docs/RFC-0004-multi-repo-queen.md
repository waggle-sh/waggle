<!-- SPDX-License-Identifier: Apache-2.0 -->
# RFC-0004: Multi-repo queen

> **ROADMAP ONLY. NOT IMPLEMENTED.** Nothing in this RFC exists in the code today. No command, config key, ref or role described here ships in any Waggle build. It is a proposal for discussion, and the proposed decision in Appendix A is not recorded.

- **Status:** Proposed, roadmap (ticket T-062). Nothing is frozen.
- **Date:** 2026-11-19
- **Builds on:** `docs/RFC-0003.md` (hive taxonomy, claims, leases), `docs/gate.md` (what the gate checks today), `CONTRIBUTING.md` section 4 (words in our records).
- **Vocabulary:** bee words for the roles and places this RFC introduces, standard words for mechanisms (D-073): Keeper, queen, hive, comb, claim, lease, gate.

## 1. Problem and motivation

Today one queen belongs to one hive, that is, one repository. A Keeper who works across several repositories (a CLI, a docs site, a service, a shared library) runs one queen session per hive and carries the context between them by hand: which change in the library the service is waiting on, which hive has an open decision card, which claim is about to expire.

The shape that works for a person managing several projects is one conversation with one coordinator that knows all of their repos. The Keeper says "what is blocked across everything?" or "plan the library change and the service follow-up", and one queen answers. The workers stay per repo.

What goes wrong without it:

- **Context loss.** The Keeper is the only link between hives, and the link is memory.
- **Decision cards scatter.** Cards from several queens arrive in several sessions, with no single "what needs me" list.
- **Cross-repo work has no home.** A change that spans a library and its caller is two unrelated tasks in two hives, and nobody holds the ordering between them.

What must not change: claims, leases, protected paths and the gate stay per repo and keep their meaning (RFC-0003 section 3, `docs/gate.md`). This RFC adds a way to talk to many hives, not a way to loosen any one of them.

## 2. Goals and non-goals

**Goals**

1. One Keeper talks to one queen about all of that Keeper's hives.
2. Each hive's claims, leases, risk tiers and gate remain authoritative for that hive alone.
3. The queen holds, for each hive, no more access than the Keeper chose to give it.
4. Each phase is useful on its own and can be stopped without leaving the hives in a changed state.

**Non-goals**

- A Keeper-to-Keeper network. Multi-Keeper (D-366) is a separate axis: many Keepers in one hive. This RFC is one Keeper across many hives. They compose, but neither depends on the other.
- A cross-repo lock or transaction. There is no atomic multi-repo merge.
- Changing the gate, the claim format or `lease.json`.
- Running workers. Workers stay one per task, in one hive.

## 3. Terms

- **Home hive.** A hive in the Keeper's registry (section 5.3). A repo is a home hive because the Keeper listed it, not because of anything its contents say.
- **Hub queen.** The single queen the Keeper talks to in the multi-repo setup. It is the same role as today's queen, with a registry.
- **Hive view.** What the hub queen holds about one home hive: its claims, open cards, plan and status, read through that hive's own interfaces.
- **Cross-repo plan.** An ordered list of per-hive tasks with the dependencies between them. It is a document, not a lock (section 6.4).

## 4. Architecture options

### Option A: Hub queen with per-repo workers

One queen, running on the Keeper's machine, holds a registry of home hives. For any action inside a hive it uses that hive's own commands (`waggle claim`, `waggle status`, `waggle dispatch`) with that hive's credentials. Workers run exactly as today, one per task, in one hive.

- **For:** One conversation, one list of cards. Smallest change to the existing model: the queen role gains a registry and nothing else. Workers and the gate are untouched.
- **Against:** The hub is a single point of confusion. If it mixes up two hives it can act in the wrong one. One session carries context for many repos, so its context fills faster. Its credentials are the union of what it was granted, so a compromised hub session reaches every home hive it was granted.
- **Mitigations:** Every hive action names its hive explicitly and is checked against the registry (section 7). Credentials are per hive and are not shared in a session (section 7.2).

### Option B: Federated queens with a coordinating queen

Each hive keeps its own queen exactly as today. A coordinating queen is the Keeper's single entry point, and it talks to the per-hive queens rather than to the hives.

- **For:** Strong isolation. Each hive's queen keeps its own context, credentials and habits, and the coordinator never holds a hive credential. A hive can leave the federation by being unlisted.
- **Against:** Two layers of queens means two layers of relayed decision cards and a protocol between them that does not exist yet (today the queen's only interface is the Keeper). Latency and lost-in-translation errors: a card relayed twice is easy to answer wrongly. It is the most to build.
- **Fit:** Good end state if hubs get large. Heavy first step.

### Option C: Single queen with a multi-repo registry (read-mostly)

The same registry as Option A, but the queen only reads across hives and acts in one hive at a time, in a session that is scoped to that hive for any write. Switching hive for a write is an explicit step.

- **For:** Keeps the one-hive-per-write-session property of today. Cross-repo awareness without cross-repo authority.
- **Against:** Cross-repo planning and coordinated work still need a place to live. Option C alone does not provide it.

### Recommendation

Build **Option C first, grow it into Option A**, and keep Option B as the escape hatch:

- Phases 0 to 1 are Option C: a registry and a read-only view.
- Phase 2 adds planning in the hub, which still writes nothing to any hive.
- Phase 3 is where Option A or Option B is chosen, on evidence from phases 1 and 2. The phases are ordered so that no early phase forecloses either.

| | A: hub queen | B: federated | C: registry, read-mostly |
| :--- | :--- | :--- | :--- |
| One conversation | yes | yes | yes |
| Hub holds hive credentials | yes, scoped per hive | no | read only |
| New protocol needed | no | yes | no |
| Blast radius of a compromised hub session | all granted hives | one coordinator, no hive credentials | read access to the registry |
| Build cost | medium | high | low |

## 5. Design sketch

All names here are placeholders for discussion.

### 5.1 What stays per hive

Everything in RFC-0003 section 3 and `docs/gate.md` is unchanged and still enforced by the home hive alone:

- **Claims** live in that hive's refs, `refs/waggle/claims/ticket-<n>`, written by `CombBackend` against that hive's remote. There is no cross-hive claim and no ref that names another hive.
- **Leases** name directories inside one repo. A lease cannot name a path in another repo.
- **The gate** runs in that repo's CI from that repo's pinned tree and reads only that repo's claims. It does not know the hub exists.
- **Decisions** are appended to that hive's `DECISIONS.md`, through that hive's own approval path.

If the hub vanished tomorrow, every hive would work as it does today.

### 5.2 What the hub adds

- A registry the Keeper owns (section 5.3).
- A hive view per home hive, built from that hive's own commands and refs.
- Cross-repo plans (phase 2), kept by the hub.
- One merged list of open decision cards, each tagged with its hive.

### 5.3 The registry

A Keeper-owned file outside any repo, for example in the Keeper's user config directory (not inside a hive, and not writable by a worker). Each entry has a name, a local path or remote, the Keeper identity to use in that hive, and the access level (`view`, `plan`, `work`; section 7.1). The registry is never read from a repo's contents, so a repo cannot add itself, and a pull request cannot widen the hub's reach.

## 6. How it fits claims, leases and the gate

### 6.1 Claims stay per repo

A hive's claim is written only by that hive's tooling, in that hive's checkout, under the Keeper identity registered for it, through that hive's `waggle claim`. The hub never writes a ref into a hive on its own: it asks that hive's commands to do it, as the Keeper would. **No cross-repo claim writing**: no operation exists, or is proposed, that writes a claim in hive X as a side effect of work in hive Y.

### 6.2 Leases do not cross

A task that needs a change in two repos is two tasks with two claims and two leases. Neither lease widens because the other task exists.

### 6.3 The gate does not change

The gate checks that each commit's task claim names the PR's branch and that the commit stays inside the lease. It has no concept of a cross-repo plan, and this RFC gives it none. A cross-repo plan can be wrong without the gate being weakened, because the plan is advisory and the gate is authoritative.

### 6.4 Ordering between repos

A cross-repo plan says "the library change merges before the service change". The hub reports the ordering and flags when it is violated. It does not enforce it. If an enforcing check is ever wanted, it must be a per-repo gate rule that reads that repo's own state, and it is out of scope here.

## 7. Trust and credential model

### 7.1 Access levels per hive

Each registry entry carries one level, set by the Keeper, and each is a ceiling:

- **`view`:** read the hive's refs, `DECISIONS.md`, tasks and status. No writes.
- **`plan`:** `view`, plus the hub keeps a cross-repo plan about the hive. Still no write to the hive.
- **`work`:** `plan`, plus the hub may run the hive's own `waggle claim` and `waggle dispatch` for the Keeper.

A hive is `view` until the Keeper raises it, one hive at a time.

### 7.2 Least privilege per repo

- Credentials are per hive: a token or key that reaches hive X does not reach hive Y. The hub never holds one broad credential for all repos.
- Read access for `view` and `plan` uses a read-only credential. Only `work` is given write access, and only for what `waggle claim` and `waggle dispatch` need there.
- The hub passes a hive's credential only to that hive's commands, never to workers. Workers keep today's rule: they get no credential that can write claim refs (the README's limit on whoever can push claim refs).
- A worker's lease and the hub's access level are independent. Raising the hub's level never widens a lease.

### 7.3 Keeper identity

The Keeper is the same human across hives, but identity is recorded per hive: the claim's `keeper_id` is the one registered for that hive. The hub does not invent an identity and does not write a claim under one it was not given. Head Keeper powers (Tier 3 approval, force-release, admin merge) are per hive and are never exercised by the hub: the hub can surface "this needs the Head Keeper", and the Keeper does it in that hive.

### 7.4 Trust boundaries

- **Hive contents are untrusted input to the hub.** A hive's `DECISIONS.md`, briefs and handoffs are read into the hub's context. They can contain text meant to steer the queen. The hub treats them as data and renders them as RFC-0003 `WORKPLAN.md` does: validated fields only, no free text relayed as an instruction.
- **The hub does not follow instructions found in one hive to act in another.** A line in hive X that says "also update hive Y" is a note for the Keeper, not an action.
- **The registry is the only authority** on which hives exist and at what level.

## 8. Failure modes

| Failure | Effect | Handling |
| :--- | :--- | :--- |
| Hub acts in the wrong hive | Wrong claim, wrong branch | Every hive action names its hive and is checked against the registry; a write needs `work`. The gate still refuses lease-violating commits in that hive. |
| Hub context overflows with many hives | Stale or mixed-up view | Hive views are summaries rebuilt from refs, not accumulated chat; a hive is pulled in on demand. |
| A hive is unreachable (offline, no remote) | Partial view | The view says "stale since T" for that hive and does not guess. |
| Cross-repo plan drifts from reality | Ordering advice is wrong | Plans are rebuilt from per-hive state on each read; a plan is never the source of truth for a claim. |
| Credential leak from the hub host | Access to every `work` hive | Per-hive credentials, `view` as the default, and the README's existing advice to keep agent credentials narrow. The hub host is as sensitive as the Keeper's own machine. |
| A hive's text tries to steer the hub | Cross-hive action | Hive text is data (section 7.4); writes are by explicit Keeper request and registry level. |
| Two Keepers' hubs act in one hive | Racing claims | Per-ticket claim CAS applies unchanged (RFC-0003 section 3.1). The hub adds no new race. |
| Hub is removed | None to the hives | The hives never depended on it (section 5.1). |

## 9. Phasing

Each phase ships and is evaluated before the next starts. None is scheduled.

### Phase 0: Docs

This RFC, plus the glossary entries it needs (hub queen, home hive, registry) in `CONTRIBUTING.md` section 4 once the proposal is accepted. No code. **Exit:** the Keeper accepts, amends or rejects the proposal, and the decision is recorded in the hive's log by the queen.

### Phase 1: Read-only cross-repo view

A registry file, and a queen that can answer "what is open, claimed, blocked or waiting on me" across home hives from each hive's refs and records, at `view`. No writes to any hive. One merged decision-card list. **Exit:** for a Keeper with at least three hives, the view matches what each hive's own `waggle status` says, and no hive state changes.

### Phase 2: Cross-repo planning

At `plan`, the hub keeps cross-repo plans: ordered per-hive tasks, with dependencies, held outside the hives. It may draft per-hive briefs for the Keeper to land in each hive through that hive's normal path. It still writes nothing into a hive on its own. **Exit:** one real change spanning two repos is planned in the hub and landed through two ordinary per-hive PRs, each through its own gate.

### Phase 3: Coordinated work

At `work`, the hub runs a hive's own `waggle claim` and `waggle dispatch` on the Keeper's behalf, hive by hive, with that hive's credential, in the order the plan gives. Decide here between Option A and Option B (section 4) on the evidence from phases 1 and 2. **Exit:** a two-repo change is claimed, run and merged with every claim written in its own hive under the right `keeper_id`, and the gates unchanged.

## 10. Open questions

1. **Registry location and format.** A user config file is proposed; a per-machine daemon registry is the alternative.
2. **Hub placement.** Does the hub live in one of the hives, or in none? One of the hives would let it read that hive's records for free but would tie the hub to it. None is proposed.
3. **How the hub reads a hive.** By the hive's local checkout, or from the remote. A checkout is faster and may be stale; the remote is authoritative for claims (RFC-0003 section 3.1).
4. **Cards across hives.** Whether one merged card list can carry a hive's decision card without the hub answering it for the Keeper (it must not).
5. **Interaction with multi-Keeper (D-366).** A hive may have several Keepers each with their own hub. The design assumes the hub adds no shared state, so this should hold; phase 1 should test it.

## Appendix A: Proposed decision entry (not recorded)

This is the text proposed for the hive's decision log. It is **not** in `.waggle/hive/DECISIONS.md`: that file is append-only through the queen and the Head Keeper (the `queen` row of the roster table in RFC-0003 section 2.2, and `docs/gate.md` section 8), and this ticket does not edit `.waggle/hive/`. The ID is left open for the queen to assign.

```
## D-<next>: T-062: multi-repo queen is roadmap; phase 0 is the RFC

- **Date:** 2026-11-19
- **Kind:** product
- **Status:** proposed (roadmap, not implemented)
- **Tickets:** T-062

**Question:** Should Waggle add a multi-repo queen, one queen that a Keeper talks to across all of that Keeper's hives, and in what order?

**Options considered:**
1. Accept docs/RFC-0004-multi-repo-queen.md as the roadmap: phase 0 docs, phase 1 read-only cross-repo view, phase 2 cross-repo planning, phase 3 coordinated work, choosing hub queen or federated queens at phase 3. Claims, leases and the gate stay per repo.
2. Accept the direction but only phases 0 and 1, and decide the rest later
3. Reject; one queen per hive stays

**Hive recommendation:** Option 2, the cheapest way to learn whether the read-only view is worth having before any write path exists.

**Keeper choice:** Not decided.

**Rationale:** A Keeper with several repos is the link between their hives today. A registry and a read-only view add that awareness without any cross-repo authority. No claim is written across repos in any phase. Nothing is implemented by this decision.
```
