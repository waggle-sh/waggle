<!-- SPDX-License-Identifier: MIT-0 -->
<!-- GENERATED FILE: append-only, written by `waggle decide`. Never edit or delete an entry; record a new one that supersedes it. -->
# DECISIONS

## D-001: Claim refs wrap a single lease.json commit

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-002, T-003
- **Supersedes:** none

**Question:** How is a Comb claim stored at refs/waggle/claims/ticket-<n>?

**Options considered:**
1. Bare JSON blob at the ref, as RFC-0003 3.1 says
2. Commit whose tree holds only lease.json, with renewals and takeovers chained

**Hive recommendation:** Not recorded

**Keeper choice:** The ref points to a commit wrapping a single lease.json; renewals and takeovers are child commits of the previous claim; release deletes the ref.

**Rationale:** Gives an audit chain readable with git log while the claim exists. Needs an RFC-0003 section 3.1 amendment (T-003).

## D-002: Claim time rejects Tier 3 overlap

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-002, T-003
- **Supersedes:** none

**Question:** Should AcquireLease refuse leases that touch Tier 3 paths, or leave that to the merge gate?

**Options considered:**
1. Merge gate only
2. Reject at claim time any lease that equals, is inside, or contains a Tier 3 path

**Hive recommendation:** Not recorded

**Keeper choice:** AcquireLease rejects any lease overlapping a Tier 3 path, compared case-insensitively, including the repo root.

**Rationale:** Defense in depth on case-insensitive checkouts; the merge gate stays authoritative.

## D-003: Ticket ID canonical forms

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-002, T-003
- **Supersedes:** none

**Question:** How are ticket IDs accepted, displayed, and mapped to refs?

**Options considered:**
1. Zero-padded refs
2. Display T-%03d with unpadded refs

**Hive recommendation:** Not recorded

**Keeper choice:** Display T-%03d, ref ticket-<n> unpadded, input ^(T-)?[0-9]{1,6}$, value >= 1.

**Rationale:** Leading zeros can never create two refs for one ticket, and strict input blocks ref-name injection.

## D-004: Malformed claim handling

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-002, T-005
- **Supersedes:** none

**Question:** How do claim operations treat malformed claim refs?

**Options considered:**
1. Fail every operation
2. Skip malformed refs on other tickets and report them, refuse a malformed target

**Hive recommendation:** Not recorded

**Keeper choice:** Malformed claims on other tickets are skipped in the overlap check and reported via a typed error; a malformed target ticket returns ErrMalformedClaim.

**Rationale:** One bad ref must not block the hive; the merge gate is authoritative for overlap anyway. Clearing stuck refs is T-005.

## D-005: Queue T-002 follow-up tickets

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-003, T-004, T-005
- **Supersedes:** none

**Question:** Which follow-up tickets are queued after T-002?

**Options considered:**
1. Queue T-003, T-004 and T-005
2. Defer follow-ups

**Hive recommendation:** Not recorded

**Keeper choice:** Queue T-003 (RFC amendment), T-004 (config loader) and T-005 (Controller force-release).

**Rationale:** Each closes a gap T-002 left out of scope.

## D-006: Config parser is a hand-written YAML subset

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-004
- **Supersedes:** none

**Question:** Which parser reads .waggle/config.yaml?

**Options considered:**
1. A: add a YAML library dependency
2. B: hand-written YAML subset parser
3. C: switch config to JSON

**Hive recommendation:** B

**Keeper choice:** B: hand-written YAML subset parser, stdlib only, fuzz tested.

**Rationale:** Config is machine-written by waggle init and small; a zero-dependency control-plane binary is worth more than YAML completeness.

## D-007: Comb remote comes from forge.remote

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-004
- **Supersedes:** none

**Question:** Where does the Comb remote come from?

**Options considered:**
1. forge.remote in shared config
2. remote in per-Keeper local.yaml
3. derive origin from provider and comb mode

**Hive recommendation:** forge.remote in shared config, no CLI override

**Keeper choice:** New forge.remote key in shared config, added by the Controller to the template and .waggle/config.yaml; absent means local-only; no --remote override.

**Rationale:** The remote is hive-wide state; Keepers disagreeing about it would split the Comb.

## D-008: Config loader lives in internal/config

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-004
- **Supersedes:** none

**Question:** Where does the config loader live?

**Options considered:**
1. New internal/config/ package
2. internal/comb/config.go

**Hive recommendation:** New internal/config/ package

**Keeper choice:** New internal/config/ package; the Controller adds it to WAGGLE.md section 2.

**Rationale:** Config also feeds internal/runtime later and should not couple to comb.

## D-009: Controller identity v0.1 is an allow-list

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-005
- **Supersedes:** none

**Question:** How is the Controller identified for force-release in v0.1?

**Options considered:**
1. Allow-list of keeper IDs in a Tier 3 file
2. Signed requests verified against allowed signers
3. Host-only with no identity check

**Hive recommendation:** Allow-list of keeper IDs in a Tier 3 file

**Keeper choice:** Allow-list of keeper IDs in a Tier 3 file, checked against local.yaml keeper_id; advisory against humans with push access, and documented as such.

**Rationale:** A real guard against agents with little work; signed identity can follow.

## D-010: Force-release audit is a durable ref

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-005
- **Supersedes:** none

**Question:** Where is a Controller force-release audited?

**Options considered:**
1. Child commit on the claim ref
2. DECISIONS.md entry
3. Durable append-only audit ref written atomically with the delete

**Hive recommendation:** Durable audit ref; the gate does not fetch it in v0.1

**Keeper choice:** Durable append-only ref refs/waggle/audit/force-releases written atomically with the delete; the gate does not fetch it in v0.1.

**Rationale:** A child commit on a deleted ref is not durable, and DECISIONS.md cannot be written atomically with a ref delete.

## D-011: RFCs get a Tier 2 floor via docs/specs

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-003
- **Supersedes:** none

**Question:** How does the frozen RFC get an enforced Tier 2 floor?

**Options considered:**
1. Move RFCs into docs/specs/
2. Add a docs/RFC- prefix to the Tier 2 list
3. Policy only

**Hive recommendation:** Move RFCs into docs/specs/

**Keeper choice:** A later task moves RFCs into docs/specs/.

**Rationale:** Inherits the existing Tier 2 floor with no code change.

## D-012: T-003 amendment scope widened

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-003
- **Supersedes:** none

**Question:** What does the RFC-0003 Amendment 1 cover, and where is its approval recorded?

**Options considered:**
1. Three approved items only
2. Also all T-002 claim-time rules and docs/tasks/** in the RFC 3.3 Tier 2 list

**Hive recommendation:** Widen the scope; record approval in the PR plus RFC Appendix A

**Keeper choice:** Record all T-002 claim-time rules and add docs/tasks/** to the RFC 3.3 Tier 2 list; approval is recorded in the PR plus RFC Appendix A.

**Rationale:** Agents planning leases need the rules, and code and CI already enforce docs/tasks/** at Tier 2.

## D-013: Humans review intent; 500-line task cap

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-002
- **Supersedes:** none

**Question:** What do humans review, and how large may a task be?

**Options considered:**
1. Humans review intent, never diffs, with a task size cap
2. Alternatives not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** Humans review intent, never diffs. Every decision reaching the Keeper is concise, self-contained, and carries the hive's recommendation, cost and reversibility. Tasks are capped at about 500 lines of non-test code.

**Rationale:** The cap keeps guard review accurate.

## D-014: waggled build order

- **Date:** 2026-10-17
- **Kind:** strategy
- **Tickets:** T-006
- **Supersedes:** none

**Question:** In what order is waggled built?

**Options considered:**
1. decide, dispatch, run.sock loop, Cells
2. Alternatives not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** waggle decide, then waggle dispatch (worktree plus headless claude -p), then the run.sock daemon loop, then Cells.

**Rationale:** Smallest deterministic slice first; each step is usable on its own.

## D-015: GitHub App is the sole claim writer in team mode

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-002, T-005
- **Supersedes:** none

**Question:** How is claim integrity protected in team mode?

**Options considered:**
1. Waggle GitHub App as sole claim writer with gate signature checks
2. Alternatives not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** In team mode a Waggle GitHub App is the sole claim writer via the GitHub API (GitHub-verified commits); the gate rejects claim chains containing commits not signed by the App; waggled raises an alert on rewrites.

**Rationale:** Git refs have no per-ref ACL. Needs a GitHub App registration (a credential decision) later.

## D-016: Keeper identity is the GitHub login

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-005
- **Supersedes:** none

**Question:** What identifies a Keeper?

**Options considered:**
1. GitHub login now, commit signing later
2. Alternatives not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** GitHub login first; commit signing enabled later and possibly enforced.

**Rationale:** Uses an identity the forge already verifies.

## D-017: PR merge flow with Intent Packet merge commits

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** none
- **Supersedes:** none

**Question:** How does work merge, solo or team?

**Options considered:**
1. PR flow with real merge commits
2. Alternatives not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** PR flow always, solo or team: t-<id> branch, PR, waggle gate (same Go code locally and in CI), real merge commit (no squash) whose message carries the Intent Packet with trailers for ticket, risk tier and guard verdict; auto-merge on pass.

**Rationale:** One path for every hive size, with the review record kept in history.

## D-018: Guard runs on a different model than the forager

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** none
- **Supersedes:** none

**Question:** Which model runs the guard?

**Options considered:**
1. A different Claude model than the forager, configurable per hive
2. Alternatives not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** Guard runs on a different Claude model than the forager, configurable per hive; cross-provider later via config.

**Rationale:** Reduces correlated blind spots between author and reviewer.

## D-019: T-002 reviewed split by file instead of re-cut

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-002
- **Supersedes:** none

**Question:** T-002 has 1,160 non-test lines, over the D-013 cap. Re-cut the branch or review it as one PR?

**Options considered:**
1. Re-cut T-002 into tasks under the cap
2. Keep one PR and have the guard review it split by file in 5 units

**Hive recommendation:** Keep one PR and review it split by file

**Keeper choice:** T-002 is reviewed split by file by the guard in 5 units rather than re-cut to meet the ~500 line cap.

**Rationale:** Re-cutting costs a Forager rerun for no correctness gain, and each review unit stays near the cap.

## D-020: WAGGLE.md section 2 adds internal/config and internal/decisions

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-004, T-006
- **Supersedes:** none

**Question:** How do the new internal/config/ and internal/decisions/ packages become leasable modules?

**Options considered:**
1. One Controller commit adding both to WAGGLE.md section 2
2. Alternatives not recorded

**Hive recommendation:** One Controller commit adding both

**Keeper choice:** WAGGLE.md section 2 gains internal/config/ (T-004) and internal/decisions/ (T-006), committed on branch controller/waggle-md-modules (0056ad9) with Master Keeper approval.

**Rationale:** WAGGLE.md is Tier 3, so module boundaries change only through a Controller commit the Master Keeper approves.

## D-021: waggle gate rejects non-append DECISIONS.md changes

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-007
- **Supersedes:** none

**Question:** Should waggle gate reject any DECISIONS.md change that is not a pure append?

**Options considered:**
1. Yes, a gate check using VerifyAppendOnly
2. No, rely on review and the integrity parser

**Hive recommendation:** Yes, as a follow-up task once the gate exists

**Keeper choice:** waggle gate rejects any DECISIONS.md change that is not a pure append (T-007).

**Rationale:** The integrity parser cannot catch a hand edit that keeps the format valid; only a diff against the previous version can.

## D-022: go.mod and go.sum become hardcoded Tier 3

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-008
- **Supersedes:** none

**Question:** What tier protects go.mod and go.sum?

**Options considered:**
1. Hardcoded Tier 3
2. Alternatives not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** go.mod and go.sum become hardcoded Tier 3 (T-008).

**Rationale:** Not recorded

## D-023: Decision entries carry no approver field; backfill uses recording date

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-006
- **Supersedes:** none

**Question:** Should waggle decide entries record who approved them, and how are backfilled dates and missing history handled?

**Options considered:**
1. No approver field; PR approval ties each decision to a GitHub login
2. Self-asserted keeper field on each entry
3. A flag to set the original decision date

**Hive recommendation:** No approver field; backfill with the recording date and "Not recorded" where history is missing

**Keeper choice:** In v0.1 waggle decide entries carry no approver field; PR approval ties decisions to a GitHub login. Backfilled entries use the recording date and "Not recorded" where history is missing. Defaults applied by the Queen, reversible.

**Rationale:** A self-asserted field would look like identity without being one, and a date override would let anyone backdate an entry.

## D-024: GitHub home is the waggle-sh organization

- **Date:** 2026-10-17
- **Kind:** strategy
- **Tickets:** T-010
- **Supersedes:** none

**Question:** Where does the project live on GitHub, and what is its Go module path?

**Options considered:**
1. waggle-dev (taken by an unrelated user account)
2. New organization waggle-sh

**Hive recommendation:** Not recorded

**Keeper choice:** New organization waggle-sh; repo waggle-sh/waggle; the Go module path becomes github.com/waggle-sh/waggle via a rename ticket (T-010) after T-002 merges.

**Rationale:** waggle-dev is taken by an unrelated user account, and the module path must match the repo for go install to work.

## D-025: Repo starts private and goes public at v0.1 launch

- **Date:** 2026-10-17
- **Kind:** publish
- **Tickets:** none
- **Supersedes:** none

**Question:** When does the repository become public?

**Options considered:**
1. Private until the v0.1 launch
2. Alternatives not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** The repo starts private and goes public at the v0.1 launch.

**Rationale:** Not recorded

## D-026: T-002 approved; merges as the first PR

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-002
- **Supersedes:** none

**Question:** Is T-002 approved, and how does it merge?

**Options considered:**
1. Approve and merge as one PR
2. Hold for fixes before merge

**Hive recommendation:** Approve and merge as one PR; queue one follow-up ticket for the guard findings

**Keeper choice:** The Keeper approved T-002 after a guard pass; it merges as the first PR once the repo exists, using a merge commit carrying the Intent Packet.

**Rationale:** The guard verdict was pass with no blocking findings; the merge commit follows D-017.

## D-027: T-002 guard findings become T-009, ahead of any WORKPLAN.md write

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-002, T-009
- **Supersedes:** none

**Question:** How are the non-blocking T-002 guard findings handled?

**Options considered:**
1. Fold the fixes into T-002 before merge
2. One follow-up ticket ordered before any WORKPLAN.md write

**Hive recommendation:** One small follow-up ticket for F1 to F4 and F6

**Keeper choice:** Guard findings on T-002 become T-009, which must land before anything writes WORKPLAN.md to disk.

**Rationale:** Finding F2 is an injection path into WORKPLAN.md, a steering file agents read.

## D-028: Roles declare minimum capability levels, never model names

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** none
- **Supersedes:** none

**Question:** How does a hive choose which model runs each role?

**Options considered:**
1. Roles name concrete models
2. Roles declare a minimum capability level (frontier, standard, fast) and hive config maps levels to models per runtime

**Hive recommendation:** Not recorded

**Keeper choice:** Roles declare minimum capability levels, never model names. Levels are frontier, standard and fast; hive config maps each level to a concrete model per runtime. Minimums: architect frontier, queen frontier, guard at or above the forager's level and on a different model, forager standard, scout fast; waggled uses no model. The guard minimum cannot be lowered on Tier 2 and above; other overrides are allowed and logged as decisions. A forager steps up one level after two must-fix verdicts on a task. In v0.1 the Keeper assigns models to levels by hand; later, Waggle eval data (guard pass rates) does.

**Rationale:** Not recorded

## D-029: Glossary at docs/GLOSSARY.md, Tier 2 minimum, owned by the architect

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-013
- **Supersedes:** none

**Question:** Where do Waggle's terms get defined, and who keeps them current?

**Options considered:**
1. Terms defined only in the RFC
2. A glossary at docs/GLOSSARY.md, Tier 2 minimum, owned by the architect

**Hive recommendation:** Not recorded

**Keeper choice:** docs/GLOSSARY.md is Tier 2 minimum, owned by the architect, linked from the README, and updated in the same PR as any brief that introduces a term.

**Rationale:** Not recorded

## D-030: Single-folder footprint; never gitignore project-owned files

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-003, T-011
- **Supersedes:** none

**Question:** Which files may Waggle create or change in a host repo?

**Options considered:**
1. Keep the current root files (WAGGLE.md, .devcontainer/) and the .gitignore block that ignores project files
2. Everything under .waggle/, plus a thin gate workflow and one marked import line

**Hive recommendation:** Not recorded

**Keeper choice:** Waggle owns .waggle/control/ (Tier 3: config.yaml, roster.json, roles/, WAGGLE.md, devcontainer.json, gitignored local.yaml and run.sock) and .waggle/hive/ (Tier 2 minimum: tasks/, reviews/, media/, DECISIONS.md, WORKPLAN.md). Outside .waggle/ only .github/workflows/waggle-gate.yml and one marked import line in an existing CLAUDE.md or AGENTS.md, or a one-line file created if none exists. Waggle never gitignores project-owned files, only its own runtime state. waggle uninstall removes .waggle/, the workflow and the marked lines. The Cell dev container starts with devcontainer --config .waggle/control/devcontainer.json; no .devcontainer/ directory. Needs an RFC amendment (T-003).

**Rationale:** Not recorded

## D-031: "Head Keeper" replaces "Master Keeper" and "Controller"

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-011, T-013
- **Supersedes:** none

**Question:** What is the name for the Keeper with final authority over a hive?

**Options considered:**
1. Master Keeper
2. Controller
3. Head Keeper

**Hive recommendation:** Not recorded

**Keeper choice:** Head Keeper replaces Master Keeper and Controller. The retired aliases are recorded in the glossary.

**Rationale:** Avoids the master connotation and matches real beekeeping usage.

## D-032: Queue a full waggle gate ticket after T-002

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-007, T-008
- **Supersedes:** none

**Question:** Which ticket builds the general merge gate?

**Options considered:**
1. Fold gate checks into T-007
2. Queue a separate waggle gate ticket after T-002

**Hive recommendation:** Queue a gate ticket after T-002 merges (Q-T007-2)

**Keeper choice:** A full waggle gate ticket is queued after T-002: per-commit lease checks, failing Tier floors instead of warnings, and case-insensitive path matching, including the GO.MOD and waggle.md gaps.

**Rationale:** Not recorded

## D-033: T-008 also protects go.work, go.work.sum and vendor/

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-008
- **Supersedes:** none

**Question:** Which Go dependency paths besides go.mod and go.sum are hardcoded Tier 3?

**Options considered:**
1. go.mod and go.sum only
2. Also go.work, go.work.sum and vendor/

**Hive recommendation:** Not recorded

**Keeper choice:** T-008 also makes go.work, go.work.sum and vendor/ hardcoded Tier 3.

**Rationale:** Not recorded

## D-034: Protect internal/scaffold/templates/ with CODEOWNERS

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** none
- **Supersedes:** none

**Question:** How is the template source that every new hive receives protected in this repo?

**Options considered:**
1. Hardcoded tier entry
2. CODEOWNERS in this repo, as a separate ticket

**Hive recommendation:** Not recorded

**Keeper choice:** internal/scaffold/templates/ is protected via CODEOWNERS in this repo, in a separate ticket.

**Rationale:** Not recorded

## D-035: A corrupt DECISIONS.md on main blocks PRs until a Head Keeper repair

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-007
- **Supersedes:** none

**Question:** What happens when DECISIONS.md on main fails the integrity check?

**Options considered:**
1. A skip flag on the gate check
2. Fail every PR touching it until a Head Keeper admin-merges a repair

**Hive recommendation:** Not recorded

**Keeper choice:** A corrupt DECISIONS.md on main fails every PR touching it until a Head Keeper admin-merges a repair. There is no skip flag.

**Rationale:** Not recorded

## D-036: Windows device names rejected in lease paths on every OS

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-009
- **Supersedes:** none

**Question:** Should lease paths named like Windows reserved devices (CON, PRN, AUX, NUL, COM1, LPT1 and so on) be rejected on every OS?

**Options considered:**
1. Reject on every OS
2. Reject only on Windows

**Hive recommendation:** Reject on every OS (Q-T009-1)

**Keeper choice:** Windows device names are rejected in lease paths on every OS.

**Rationale:** Not recorded

## D-037: v0.1 lease directories are ASCII safe-set only

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-009
- **Supersedes:** none

**Question:** May leased directory names contain spaces or non-ASCII characters?

**Options considered:**
1. ASCII safe set only, no spaces
2. Allow spaces and non-ASCII

**Hive recommendation:** ASCII safe set for v0.1 (Q-T009-2)

**Keeper choice:** v0.1 lease directories use the ASCII safe set only, with no spaces.

**Rationale:** Not recorded

## D-038: Case-insensitive directory overlap deferred

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-009
- **Supersedes:** none

**Question:** Does T-009 also make lease overlap checks case-insensitive?

**Options considered:**
1. Fold into T-009
2. Defer to the next Comb ticket

**Hive recommendation:** Keep T-009 to the guard findings (Q-T009-3)

**Keeper choice:** Case-insensitive directory overlap is deferred to the next Comb ticket.

**Rationale:** Not recorded

## D-039: Template omits the gate decisions step; non-Go manifests and nested modules deferred

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-007, T-008
- **Supersedes:** none

**Question:** Does the scaffold gate template get the decisions step now, and do non-Go manifests and nested Go modules become Tier 3 now?

**Options considered:**
1. Add both now
2. Defer both

**Hive recommendation:** Defer both (Q-T007-1, Q-T008-2)

**Keeper choice:** The scaffold template omits the gate decisions step until a release binary exists; non-Go manifests and nested modules are deferred to a later ticket. Defaults applied by the Queen, reversible.

**Rationale:** Not recorded

## D-040: Ticket order T-002, T-010, T-011, then everything else

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-002, T-010, T-011
- **Supersedes:** none

**Question:** In what order do the queued tickets merge?

**Options considered:**
1. Merge in ticket number order
2. T-002, then T-010, then T-011, with all other tickets rebasing after T-011

**Hive recommendation:** Not recorded

**Keeper choice:** T-002 merges first, then T-010, then T-011; all other tickets rebase after T-011.

**Rationale:** Not recorded

## D-041: The hive runs a pinned, known-good Waggle build

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-007, T-011
- **Supersedes:** none

**Question:** Which Waggle build does our own hive run while Waggle is under construction?

**Options considered:**
1. The current working tree or main
2. A pinned known-good build recorded in hive config

**Hive recommendation:** Not recorded

**Keeper choice:** The hive never runs on code under construction. It runs a pinned, known-good Waggle build recorded in hive config (commit or tag). Upgrades are deliberate and logged with waggle decide. Extends T-007's rule of building the checker from the base.

**Rationale:** Not recorded

## D-042: Product template changes reach the hive only through an upgrade command

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-011
- **Supersedes:** none

**Question:** How do changes to product templates reach our hive's installed copies?

**Options considered:**
1. Hand edits to hive copies
2. An upgrade command run from the pinned build, in a hive PR

**Hive recommendation:** Not recorded

**Keeper choice:** Product template changes reach our hive only through an upgrade command run from the pinned build, in a hive PR. Hand edits to hive copies of templates are not allowed. A waggle upgrade ticket is queued.

**Rationale:** Not recorded

## D-043: Worker leases never include .waggle/

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-011, T-013
- **Supersedes:** none

**Question:** How are hive files protected from workers, and how does this repo keep product and hive changes apart?

**Options considered:**
1. A configurable rule
2. A universal rule built into Waggle with no config and no new user-facing terms

**Hive recommendation:** Not recorded

**Keeper choice:** Universal rule built into Waggle with no config and no new user-facing terms: a worker lease can never include .waggle/; hive files change only through waggle commands or the Head Keeper. Enforced at claim time (internal/comb) and at the gate. Waggle-repo-only conventions, not in Waggle's schema: a "Layer: product | hive" line in our own briefs, CODEOWNERS on templates, and a CONTRIBUTING.md section explaining product versus hive. Mixed changes split into a product ticket, then a hive upgrade.

**Rationale:** Not recorded

## D-044: "Own honey" adopted

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-013
- **Supersedes:** none

**Question:** What do we call using Waggle to build Waggle?

**Options considered:**
1. Dogfooding
2. Own honey

**Hive recommendation:** Not recorded

**Keeper choice:** "Own honey" is the term for using Waggle to build Waggle.

**Rationale:** Not recorded

## D-045: Product design that users rely on also lands in the RFC or product docs

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** none
- **Supersedes:** none

**Question:** Is DECISIONS.md enough to document product design?

**Options considered:**
1. DECISIONS.md alone
2. DECISIONS.md plus the RFC or product docs

**Hive recommendation:** Not recorded

**Keeper choice:** DECISIONS.md records how the hive decided; any product design users rely on must also land in the RFC or product docs.

**Rationale:** Not recorded

## D-046: GitHub home is the waggle-sh organization owned by Level 19 Enterprises, LLC

- **Date:** 2026-10-17
- **Kind:** strategy
- **Tickets:** none
- **Supersedes:** none

**Question:** Who owns the GitHub organization and holds the copyright for Waggle?

**Options considered:**
1. Personal account
2. Organization waggle-sh owned by Level 19 Enterprises, LLC under the GitHub Corporate Terms

**Hive recommendation:** Not recorded

**Keeper choice:** GitHub home is the organization waggle-sh, owned by the business Level 19 Enterprises, LLC (GitHub Corporate Terms). Copyright holder for NOTICE and copyright lines will be Level 19 Enterprises, LLC (implementation pending, ties into the open licensing item). The Keeper also owns getwaggle.dev and getwaggle.sh (no use decided yet).

**Rationale:** Not recorded

## D-047: The waggle-sh org runs GitHub Team with one seat

- **Date:** 2026-10-17
- **Kind:** spend
- **Tickets:** T-015
- **Supersedes:** none

**Question:** Which GitHub plan does the waggle-sh organization use while the repo is private?

**Options considered:**
1. GitHub Free with advisory-only CODEOWNERS
2. Make the repo public earlier
3. GitHub Team, one seat

**Hive recommendation:** GitHub Team, one seat

**Keeper choice:** The org runs GitHub Team (1 seat, about $4 per month, Keeper-approved spend) so branch rulesets, required checks and code-owner review are enforced on the private repo before launch.

**Rationale:** Not recorded

## D-048: The Waggle GitHub App authors agent PRs; until it exists, PRs use admin bypass

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-005, T-014, T-015
- **Supersedes:** none

**Question:** Which account opens agent PRs and pushes branches, given GitHub never lets a PR author approve their own PR?

**Options considered:**
1. The Head Keeper's own account with bypass merges
2. A separate machine account
3. The Waggle GitHub App from D-015

**Hive recommendation:** A separate machine account or GitHub App

**Keeper choice:** The Waggle GitHub App (from D-015) also authors agent PRs and pushes branches; the Head Keeper reviews. Until the App exists, PRs come from the Keeper's account and protected merges use admin bypass.

**Rationale:** Not recorded

## D-049: Tier 3 or Head Keeper PRs always fail the gate and merge by Head Keeper bypass

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-014
- **Supersedes:** none

**Question:** How does the gate treat a PR that touches Tier 3 or carries a Head Keeper commit?

**Options considered:**
1. Fail the gate and merge only by Head Keeper bypass
2. Pass when the PR has an approval from a listed login, read through the GitHub API

**Hive recommendation:** Fail the gate and merge only by Head Keeper bypass

**Keeper choice:** Any PR touching Tier 3 or containing a Head Keeper commit fails the automatic gate and merges only via Head Keeper bypass.

**Rationale:** Commit text can be forged.

## D-050: Architect brief work runs under planning tickets

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-014
- **Supersedes:** none

**Question:** Brief-writing commits have no ticket of their own; how do they pass a gate that requires one?

**Options considered:**
1. Planning tickets with a full-gate claim on the hive tasks directory
2. Merge by bypass

**Hive recommendation:** Planning tickets

**Keeper choice:** Architect brief work runs under planning tickets with a full-gate claim on the hive tasks directory.

**Rationale:** Not recorded

## D-051: CI gating requires forge.remote; local-only hives gate locally

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-014
- **Supersedes:** none

**Question:** How does a hive with no forge.remote, whose claims CI cannot see, run the gate?

**Options considered:**
1. Relax the CI gate for local hives
2. Keep the CI gate strict and require forge.remote; local-only hives run waggle gate locally

**Hive recommendation:** Keep the gate strict and require forge.remote for CI

**Keeper choice:** CI gating requires forge.remote; local-only hives run waggle gate locally.

**Rationale:** Not recorded

## D-052: Guard appends one verdict line per review round; T-012 may add two config files

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-004, T-012
- **Supersedes:** none

**Question:** Where does the must-fix count come from, and may T-012 add files to internal/config/?

**Options considered:**
1. Count appended Verdict lines in the review file
2. A counter in the claim
3. A separate config ticket before T-012 phase B

**Hive recommendation:** Count appended Verdict lines; allow the two new config files

**Keeper choice:** The guard appends one verdict line per review round (guard role template edit, Tier 3); T-012 may add two new files in internal/config/, T-004 files stay read-only.

**Rationale:** Not recorded

## D-053: Queen-applied reversible defaults for model levels, gate checks, branches and CODEOWNERS

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-012, T-014, T-015
- **Supersedes:** none

**Question:** Which reversible defaults from T-012, T-014 and T-015 stand until the Keeper changes them?

**Options considered:**
1. Apply the briefs' defaults
2. Hold each until answered

**Hive recommendation:** Apply the briefs' defaults

**Keeper choice:** Defaults applied by the Queen (reversible): different model means a different model string; forager steps up once; model overrides go in a brief header line; templates ship opus/sonnet/haiku aliases; expired claims pass the gate with a warning; PR branches must be rebased with no merge commits inside the branch, while the final merge into main remains a real merge commit carrying the Intent Packet (D-017 unchanged); CODEOWNERS initial owner @paulternate.

**Rationale:** Not recorded

## D-054: Hardcoded tier lists drop root steering files Waggle no longer reads

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-011
- **Supersedes:** none

**Question:** Should root WAGGLE.md, DECISIONS.md, ROADMAP.md, WORKPLAN.md, docs/specs/ and docs/tasks/ stay in the hardcoded tier lists?

**Options considered:**
1. Keep them
2. Drop them

**Hive recommendation:** Drop them

**Keeper choice:** Hardcoded tier lists drop root WAGGLE.md, DECISIONS.md, ROADMAP.md, WORKPLAN.md, docs/specs/ and docs/tasks/.

**Rationale:** Waggle no longer reads them post T-011.

## D-055: Root CLAUDE.md and AGENTS.md are Tier 2 minimum

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-011
- **Supersedes:** none

**Question:** Should root CLAUDE.md and AGENTS.md be hardcoded Tier 2 minimum?

**Options considered:**
1. Yes
2. No

**Hive recommendation:** Yes

**Keeper choice:** Root CLAUDE.md and AGENTS.md are Tier 2 minimum.

**Rationale:** They carry the import into the hive instructions.

## D-056: Hive records are committed under .waggle/hive/

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-011
- **Supersedes:** none

**Question:** Are our hive's tasks, reviews and media committed?

**Options considered:**
1. Commit them
2. Keep them untracked

**Hive recommendation:** Commit them

**Keeper choice:** Hive records (tasks, reviews, media) are committed under .waggle/hive/.

**Rationale:** Not recorded

## D-057: The Queen commits architect and guard output until waggled exists

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-011
- **Supersedes:** none

**Question:** How do architect briefs and guard reviews reach .waggle/hive/ when worker leases never include .waggle/?

**Options considered:**
1. The Queen commits their output as Head Keeper-approved hive commits
2. Workers lease the hive directory

**Hive recommendation:** The Queen commits their output

**Keeper choice:** Until waggled exists, the Queen commits architect and guard output into .waggle/hive/ as Head Keeper-approved hive commits.

**Rationale:** Not recorded

## D-058: Queen-applied reversible defaults for T-011 and T-013 open questions

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-011, T-013
- **Supersedes:** none

**Question:** Which reversible defaults from T-011 (Q-T011-3, -4, -6, -7) and T-013 (Q-T013-1, -2) stand until the Keeper changes them?

**Options considered:**
1. Apply the briefs' defaults
2. Hold each until answered

**Hive recommendation:** Apply the defaults

**Keeper choice:** Defaults applied by the Queen (reversible): .devcontainer/ stays Tier 3 where present; ROADMAP and specs live in the hive; agent tool config such as .claude/settings.json hooks gets its own classification ticket; Tier3ControlAuth renamed Tier3ControlPlane; docs/GLOSSARY.md is protected in this repo by CODEOWNERS rather than a hardcoded Tier 2 entry (a hardcoded entry would hit every host project's own GLOSSARY, against D-043's no-new-burden rule); CONTRIBUTING.md is not Tier 2.

**Rationale:** Not recorded

## D-059: waggle-sh deploy access uses a fine-grained token owned by waggle-sh

- **Date:** 2026-10-17
- **Kind:** credentials
- **Tickets:** none
- **Supersedes:** none

**Question:** Which credential does deploy work use for GitHub access to waggle-sh?

**Options considered:**
1. The Keeper's personal token
2. A new fine-grained token owned by waggle-sh

**Hive recommendation:** Not recorded

**Keeper choice:** Waggle-sh GitHub access for deploy work uses a new fine-grained token owned by waggle-sh, stored in the L19 Claude Deploy 1Password vault.

**Rationale:** Not recorded

## D-060: waggle-sh GitHub access on the Keeper's machine uses the gh CLI keyring login

- **Date:** 2026-10-17
- **Kind:** credentials
- **Tickets:** none
- **Supersedes:** D-059

**Question:** Which credential does the Keeper's machine use for GitHub access to waggle-sh?

**Options considered:**
1. A new fine-grained token owned by waggle-sh in 1Password (D-059)
2. The existing gh CLI keyring login

**Hive recommendation:** Not recorded

**Keeper choice:** Waggle-sh GitHub access on the Keeper's machine uses the existing gh CLI keyring login, the path a typical user takes, so the hive exercises it; no new 1Password token. Repo waggle-sh/waggle created private; main and t-002 pushed; T-002 opened as PR #1 (https://github.com/waggle-sh/waggle/pull/1) with its Intent Packet.

**Rationale:** The Keeper's machine is not representative of users ("not everyone will have 1password ... or want to set up a PAT").

## D-061: Product credential policy: git push access plus the agent CLI, no secret managers

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** none
- **Supersedes:** none

**Question:** What credentials does Waggle require of Keepers, contributors, and cells?

**Options considered:**
1. Require PATs or a secret manager
2. Minimum of git push access plus the agent CLI, with an optional per-hive GitHub App for team mode

**Hive recommendation:** Not recorded

**Keeper choice:** Waggle's minimum requirement is git with push access plus the agent CLI. Claims use plain git push with whatever git auth the Keeper already has. PRs and checks use gh if logged in, else a GITHUB_TOKEN env var. No secret-manager integrations (users wrap commands themselves, e.g. op run). Team mode adds a per-hive GitHub App created in one click via the App manifest flow, with its private key stored only in the repo's Actions secrets; Keepers never hold it and no Waggle-hosted service is needed. Relay-mode contributors need only a GitHub account. Cells hold zero credentials. PATs and 1Password are never required.

**Rationale:** The Keeper's machine is not representative of users ("not everyone will have 1password ... or want to set up a PAT").

## D-062: Release checklist includes a clean-environment test

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** none
- **Supersedes:** none

**Question:** How is a public release checked against environments unlike the Keeper's machine?

**Options considered:**
1. Test only on the Keeper's machine
2. Clean-environment test before any public release

**Hive recommendation:** Not recorded

**Keeper choice:** The release checklist includes a clean-environment test (fresh macOS user account or Linux container without 1Password, Orca, or the Keeper's global Claude config) before any public release.

**Rationale:** The Keeper's machine is not representative of users ("not everyone will have 1password ... or want to set up a PAT").

## D-063: The target build runs waggle upgrade and becomes the pin

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-016
- **Supersedes:** none

**Question:** D-042 says the upgrade command runs "from the pinned build". Which build runs it: the old pin, or the new known-good version?

**Options considered:**
1. The old pinned build
2. The target build (the new known-good version)

**Hive recommendation:** The target build

**Keeper choice:** Clarifies D-042: the target build (the new known-good version) runs waggle upgrade, renders its templates into a hive PR, and becomes the pin when that PR merges. The old pinned build never renders templates; it is recorded only as the "from" value.

**Rationale:** The old build cannot render templates it does not contain.

## D-064: Default merge strategy is an up-to-date check; native merge queue is opt-in

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-004, T-011, T-015
- **Supersedes:** none

**Question:** How does a hive serialize merges to main by default?

**Options considered:**
1. GitHub native merge queue (merge_strategy: native_queue)
2. Up-to-date check: branches must be up to date with main and pass waggle gate, with the gate re-run after each rebase

**Hive recommendation:** Not recorded

**Keeper choice:** The default merge strategy is "up-to-date check": a branch must be up to date with main and pass waggle gate before merge, and waggle re-runs the gate after each rebase. Native merge queue becomes opt-in where the forge plan supports it. The config template default changes from merge_strategy: native_queue to merge_strategy: up_to_date in T-011 phase A, which already edits config.yaml.tmpl; T-004 accepts both values and treats an absent key as up_to_date; T-015's ruleset requires branches to be up to date.

**Rationale:** Not recorded

## D-065: Roles and terms live in README sections, not docs/GLOSSARY.md

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-013, T-015
- **Supersedes:** D-029

**Question:** Where are Waggle's roles and terms defined, and how is that kept current and protected?

**Options considered:**
1. docs/GLOSSARY.md, Tier 2 minimum (D-029)
2. README sections Roles and Terms, protected by CODEOWNERS

**Hive recommendation:** Not recorded

**Keeper choice:** Roles and terms live in two README sections, Roles and Terms, updated in the same PR as any brief that introduces a term or role. Product versus hive layer terms go in CONTRIBUTING.md. README.md is protected in this repo by CODEOWNERS (T-015), not by a hardcoded tier. If the Terms list grows past about 60 entries it may split into docs/GLOSSARY.md linked from the README.

**Rationale:** Not recorded

## D-066: Pre-existing gofmt trailing-newline issues are fixed in a separate chore ticket

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-010, T-017
- **Supersedes:** none

**Question:** Should T-010 also fix the four Go files that fail gofmt -l because they lack a trailing newline?

**Options considered:**
1. Fix them inside T-010
2. Separate chore ticket T-017, keeping T-010 a pure rename

**Hive recommendation:** Separate chore ticket T-017

**Keeper choice:** Default applied by the Queen (reversible): the missing trailing newlines in cmd/waggle/main.go, internal/runtime/claude.go, internal/runtime/runtime.go and internal/scaffold/init.go are fixed in chore ticket T-017 after T-010 merges, keeping T-010 a pure rename.

**Rationale:** T-010's acceptance criteria require every changed line to be module-path text only.

## D-067: Plain-git core with a small Forge interface and a setup check that states what is not enforced

- **Date:** 2026-10-17
- **Kind:** strategy
- **Tickets:** T-018
- **Supersedes:** none

**Question:** How does Waggle depend on a code host without building that host's assumptions into the core?

**Options considered:**
1. Build GitHub behavior directly into the core
2. Keep the core plain git, put host operations behind a small Forge interface, and probe each repo's capabilities at setup

**Hive recommendation:** Not recorded

**Keeper choice:** Waggle's core stays plain git: claims, leases, gate logic, decisions, WORKPLAN and Cells. A small Forge interface covers opening, commenting on and merging changes (PRs or MRs), the CI shim file, bot identity, and relay. A setup check (in waggle init and a new waggle doctor command) probes what each repo can enforce (custom ref pushes, required checks, rulesets, merge queue, App or bot support), picks the best mode, and states plainly what is not enforced; it never silently degrades. If a host rejects refs/waggle/* pushes, claims fall back to a dedicated branch. Personal and organization GitHub accounts are handled as capability differences, not account types.

**Rationale:** Not recorded

## D-068: The hive bot SSH-signs claim and audit commits; the gate verifies against an allowed-signers file

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-005, T-014
- **Supersedes:** D-015

**Question:** How is D-015's rule that the gate rejects claim chains not written by the hive's bot implemented?

**Options considered:**
1. GitHub's verified-commit badge on commits written through the GitHub API
2. Git's built-in SSH commit signing by the bot, verified by the gate against an allowed-signers file in the Tier 3 control folder

**Hive recommendation:** Not recorded

**Keeper choice:** Implements D-015: the hive's bot signs claim and audit commits with its own SSH key using git's built-in commit signing, and the gate verifies them against an allowed-signers file in the Tier 3 control folder. GitHub's verified-commit badge is not used. The bot's key lives where the bot runs (Actions secrets on GitHub). Compatible with D-016's later Keeper commit signing.

**Rationale:** Not recorded

## D-069: No second forge before v0.1 ships, and no GitHub assumption in the core

- **Date:** 2026-10-17
- **Kind:** strategy
- **Tickets:** T-018
- **Supersedes:** none

**Question:** Which forge comes after GitHub, and when?

**Options considered:**
1. Pick the next forge now
2. No forge order until after v0.1 ships

**Hive recommendation:** Not recorded

**Keeper choice:** No forge order after GitHub until after v0.1 ships. Nothing forge-specific beyond GitHub is built now, but no GitHub assumption may enter the core.

**Rationale:** Not recorded

## D-070: Drop the roster mode field

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-012, T-013
- **Supersedes:** none

**Question:** What is the roster's "mode": "swarm" field (Q-T013-3)?

**Options considered:**
1. Drop the field
2. Define modes solo and swarm
3. Reserve the field for v0.2 operating modes

**Hive recommendation:** Drop the field

**Keeper choice:** Drop the roster "mode" field. The roster loader ignores a leftover mode key for one release. "Swarm" stays an informal README term. T-012 phase B's template commit deletes the line.

**Rationale:** The field is undefined and nothing reads it.

## D-071: The default merge_strategy value is named up_to_date; native_queue is opt-in

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-004, T-011
- **Supersedes:** none

**Question:** What config value names D-064's default merge strategy?

**Options considered:**
1. Keep native_queue as the default value
2. up_to_date as the default value, with native_queue opt-in

**Hive recommendation:** Not recorded

**Keeper choice:** The default merge setting value is named up_to_date; native_queue is opt-in.

**Rationale:** Not recorded

## D-072: Product CI runs as its own workflow, required on main, before T-011 merges

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-011, T-019
- **Supersedes:** none

**Question:** Where do the product's build, vet, format and test checks run?

**Options considered:**
1. Inside the hive gate workflow
2. A separate product CI workflow required on main

**Hive recommendation:** Not recorded

**Keeper choice:** Product CI (go build, go vet, gofmt on tracked files, go test -race) runs as its own workflow, separate from the hive gate, required on main, and must exist before T-011 merges.

**Rationale:** Not recorded

## D-073: Bee names only for concepts Waggle invents; standard words for everything users already know

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-013, T-020
- **Supersedes:** none

**Question:** When does Waggle use a bee-themed name, and when a standard industry word?

**Options considered:**
1. Bee-themed names throughout
2. Bee terms only for concepts Waggle invents, standard words for known things and precise mechanisms

**Hive recommendation:** Not recorded

**Keeper choice:** Bee terms are used only for concepts Waggle invents (Hive, Comb, Cell, roles). Things users already know keep their standard industry words (forge, PR, CI, commit, ref), and so do precise mechanisms (Gate, Lease, Claim, Risk Tier, Intent Packet). The rule is recorded in CONTRIBUTING.md. The v0.2 role name "drone" is retired because real drones do no work; studio and assembly get names when they are designed.

**Rationale:** Not recorded

## D-074: Roles are renamed to recognizable bee jobs: Scout plans, Builder codes, Forager triages

- **Date:** 2026-10-17
- **Kind:** strategy
- **Tickets:** T-020
- **Supersedes:** none

**Question:** Should every role name match a real bee job that fits what the role does?

**Options considered:**
1. Keep architect, scout and forager
2. Rename so every role is a recognizable bee job

**Hive recommendation:** Not recorded

**Keeper choice:** All roles become recognizable bee jobs. architect becomes Scout (plans tasks and writes briefs; scouts search ahead and dance directions to the hive). forager becomes Builder (writes code; builder bees make comb). scout becomes Forager (triage: reproduces bugs, writes failing tests, maps leases; foragers bring material back from outside). Queen and guard are unchanged. The rename is done in one rename ticket after T-011 merges.

**Rationale:** Not recorded

## D-075: Always-on token savings and one context.profile setting that never weakens the Tier 2+ guard

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-021
- **Supersedes:** none

**Question:** How does Waggle control token cost without weakening review?

**Options considered:**
1. No cost controls
2. Always-on savings plus a single profile setting with per-role overrides

**Hive recommendation:** Not recorded

**Keeper choice:** Always-on savings: a fresh session per task, lease-scoped context, a cache-friendly prompt order (role prompt, then WAGGLE.md, then the brief), trimmed tool output, and handoff instead of compaction. One Keeper setting, context.profile, takes lean, balanced or thorough, with per-role overrides. No profile may skip or weaken the guard on Tier 2 and above. waggle status reports tokens and dollars per ticket.

**Rationale:** Not recorded

## D-076: T-011 phase A approved for merge after the guard pass, once product CI is on main

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-011, T-019
- **Supersedes:** none

**Question:** Does the Head Keeper approve T-011 phase A (branch t-011a) for merge?

**Options considered:**
1. Approve after the guard pass
2. Hold for changes

**Hive recommendation:** Not recorded

**Keeper choice:** The Head Keeper approved T-011 phase A (branch t-011a) for merge after the guard pass, with merge_strategy up_to_date added to the config template. It merges once product CI (T-019) is on main.

**Rationale:** Not recorded

## D-077: D-068 supersedes D-015 wording; provider none stays; T-012 roster change via waggle upgrade

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-004, T-005, T-012, T-014
- **Supersedes:** none

**Question:** How are the D-015 and D-068 overlap, forge.provider none in T-004, and T-012's hive roster edit resolved?

**Options considered:**
1. Leave as written
2. Apply the Queen's defaults

**Hive recommendation:** Apply the Queen's defaults

**Keeper choice:** D-068 supersedes D-015's implementation wording, and D-068 carries a Supersedes: D-015 line. T-004 keeps accepting forge.provider: none. T-012's roster change reaches our hive through waggle upgrade, not a hand edit (D-042). Defaults applied by the Queen.

**Rationale:** Not recorded

## D-078: The queen session hands off through a written file and a fresh session

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** none
- **Supersedes:** none

**Question:** What happens when the queen session's context grows large?

**Options considered:**
1. Let the runtime compact the session
2. Write a handoff file and start a fresh session

**Hive recommendation:** Not recorded

**Keeper choice:** When its context grows large, the queen session writes a handoff file and continues in a fresh session. Long term, waggled makes the queen stateless.

**Rationale:** Not recorded

## D-079: Waggle never requires a paid forge plan; the gate is enforced by Waggle itself and server-side where the plan allows

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-004, T-018, T-019
- **Supersedes:** none

**Question:** Does Waggle depend on a paid forge plan (rulesets, required checks) to enforce the gate, and why does our own repo pay for GitHub Team?

**Options considered:**
1. Require a plan that supports rulesets and required checks
2. Never require a paid plan: Waggle-driven merges run waggle gate first and refuse on failure, server-side enforcement is added wherever the plan allows, and waggle doctor reports when the gate is advisory

**Hive recommendation:** Never require a paid plan

**Keeper choice:** Waggle never requires a paid forge plan. Waggle-driven merges always run waggle gate first and refuse on failure, whatever the plan. Server-side enforcement (rulesets, required checks) is used wherever the plan allows, and waggle doctor says plainly when the gate is advisory. Our own repo uses GitHub Team only because it is private before launch; Team can be dropped after the repo goes public.

**Rationale:** Not recorded

## D-080: Claims written after the role rename are lease version 2; version 1 roles are mapped on read

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-020
- **Supersedes:** none

**Question:** How does an upgraded binary read a stored claim whose role name was written before the rename, when forager and scout both exist before and after with different meanings (Q-T020-1)?

**Options considered:**
1. Drain claims only, no code
2. New claims are lease version 2 with the new role names; version 1 claims have their role names mapped old to new on read

**Hive recommendation:** Lease version 2 with the version 1 mapping

**Keeper choice:** Claims written after the role rename are lease version 2 and carry the new role names. Version 1 claims have their role names mapped on read (scout to forager, forager to builder, guard unchanged).

**Rationale:** Without the mapping an upgraded binary silently reads an old implementation claim as a triage claim.

## D-081: T-020 phases A and B run right after T-011, before T-012 and T-016

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-011, T-012, T-016, T-020
- **Supersedes:** none

**Question:** When do the product phases of the role rename run (Q-T020-2)?

**Options considered:**
1. Right after T-011, before T-012, T-016 and other tickets write more code with the old names
2. After T-012 merges, sweeping its role table and tests afterwards

**Hive recommendation:** Right after T-011

**Keeper choice:** T-020 (role rename) phases A and B run right after T-011, before T-012, T-016 and other tickets write more code with the old names.

**Rationale:** Code written after the rename is born with the new names, so nothing has to be swept twice. "Right after T-011" means after T-011 phase A only: the order is merge T-011 phase A, then T-020 phases A and B, then T-011 phases B and C.

## D-082: context.profile is hive-wide in config.yaml with default balanced; waggle status shows the runtime's own cost estimate as est. $

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-021, T-022
- **Supersedes:** none

**Question:** Where does context.profile live, what is its default, and how does waggle status report dollars (Q-T021-1, Q-T021-2, Q-T021-3)?

**Options considered:**
1. Allow a per-Keeper value in local.yaml, default lean, and a Keeper-maintained price table
2. One hive-wide value in config.yaml, default balanced, and the runtime's own estimate labeled est. $ with no price table

**Hive recommendation:** Hive-wide in config.yaml, default balanced, runtime estimate labeled est. $

**Keeper choice:** context.profile is hive-wide in config.yaml, default balanced. waggle status shows the runtime's own cost estimate labeled est. $ with no price table in Waggle.

**Rationale:** Not recorded

## D-083: Approved follow-ups from T-020 and T-021: T-016 deletes dropped managed files, T-013 seed renamed, T-022 now and T-023 after the Cell runtime decision, T-006 module path fixed

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-006, T-013, T-016, T-022, T-023
- **Supersedes:** none

**Question:** Which follow-ups raised by T-020 (Q-T020-3, Q-T020-4) and T-021 (Q-T021-5) are approved, and is the stale module path in T-006 fixed?

**Options considered:**
1. Leave the briefs as written
2. Apply the follow-ups

**Hive recommendation:** Apply the follow-ups

**Keeper choice:** Approved follow-ups: T-016 must delete managed files the new templates no longer ship. The T-013 README seed is updated for the new role names, drone is dropped, and the D-073 rule is added. T-022 (waggle status with token and cost reporting) is written now and T-023 (worker session launcher) after the Cell runtime decision. The old module path on T-006 line 18 is fixed.

**Rationale:** Not recorded

## D-084: The waggle-sh organization is on GitHub Team, one seat, paid through a personal PayPal account with the invoice in the LLC's name

- **Date:** 2026-10-17
- **Kind:** spend
- **Tickets:** T-015, T-019
- **Supersedes:** none

**Question:** How was the GitHub Team upgrade approved in D-047 paid for, given the LLC card was declined?

**Options considered:**
1. Wait for the LLC card to clear
2. Pay through a personal PayPal account with the invoice in the LLC's name

**Hive recommendation:** Not recorded

**Keeper choice:** The Keeper upgraded the waggle-sh organization to GitHub Team (1 seat, about $4 per month) on 2026-10-17, paid through a personal PayPal account with the invoice in the LLC's name, after the LLC card was declined by the processor.

**Rationale:** Team is needed only for rulesets and required checks on the private repo before launch (D-047, D-079); it can be dropped after the repo goes public.

## D-085: Zero Actions runs were caused by the org's billing state; product CI is green and required on main through one ruleset that T-015 extends

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-015, T-018, T-019
- **Supersedes:** none

**Question:** Why did waggle-sh/waggle create no Actions runs, and how is product CI now enforced on main?

**Options considered:**
1. Fix the org Actions policy, repo setting or workflow registration
2. Clear the org's billing hold, then require product-ci in a single main ruleset

**Hive recommendation:** Not recorded

**Keeper choice:** Cause of the zero Actions runs in waggle-sh/waggle: the org Actions policy, repo setting and workflow registration were all correct; runs were not being created because of the org's billing state (declined card on a new org). The first runs appeared on PR #4 within a minute, after the Team upgrade cleared the billing hold. Product CI (ci.yml, T-019) merged at c367ab5 and is green on main. The main ruleset exists (id 25310890): product-ci required, branches up to date, bypass for repository admins only (the Head Keeper). Verified with scratch PR #5, which was reported BLOCKED with product-ci failing, then closed. T-015 adds verify-leases-and-tiers to this same ruleset rather than creating a second one, and waggle doctor (T-018) reports the rule as present. Evidence: docs/media/T-019/.

**Rationale:** Not recorded

## D-086: T-011 phase C is drafted by a worker in an isolated worktree, reviewed cross-model, and merged on Head Keeper approval; C-5 stays with the Head Keeper

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-011, T-016
- **Supersedes:** none

**Question:** Who runs T-011 phase C (the hive PR), given that hive files are never leasable (D-043)?

**Options considered:**
1. The Head Keeper runs the section 10 commands himself
2. Wait for T-016 (`waggle upgrade`) and let it move the control plane
3. A worker (architect subagent, isolated worktree) drafts the phase C hive PR verbatim from T-011 section 10 (C-1 to C-4), guard reviews cross-model, the Head Keeper approves the merge

**Hive recommendation:** Worker drafts C-1 to C-4 in an isolated worktree, cross-model guard review, Head Keeper approves the merge

**Keeper choice:** A worker (architect subagent, isolated worktree) drafts the phase C hive PR verbatim from T-011 section 10 (C-1 to C-4). Commit subjects end "(Head Keeper)" as the brief writes them. Guard reviews cross-model. The Head Keeper approves the merge. C-5 stays with the Head Keeper in the main checkout after merge. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected the Keeper running the commands himself and waiting for T-016. The brief already spells out every command, so a worker can execute it verbatim without touching the main checkout, and the Head Keeper keeps the merge approval and the post-merge step that only makes sense in the main checkout.

## D-087: T-011 phase C pins `b4f2221` (the phase A merge commit); the hive keeps the old role names until T-020 phase C moves the pin through `waggle upgrade`

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-011, T-016, T-020
- **Supersedes:** none

**Question:** Which commit does T-011 phase C pin, and when does our hive pick up the renamed roles?

**Options considered:**
1. Pin current main `924a59f`
2. Pin `b4f2221` (the phase A merge commit)

**Hive recommendation:** Pin `b4f2221`

**Keeper choice:** T-011 phase C pins `b4f2221` (the phase A merge commit). Our hive keeps the old role names (queen, architect, scout, forager, guard) until T-020 phase C moves the pin through `waggle upgrade`, which makes that the first real test of managed-file removal (D-083). Confirms the queen default recorded in T-011 section 10 and T-020 section 9. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected pinning current main `924a59f`: it would need `git rm .waggle/control/roles/architect.md` in C-3 and would pull the T-020 section 7.3 brief sweep into this PR.

## D-088: T-006 phase 1 merges at 708 non-test lines (D-013 budget waived for this ticket) with a 200-byte title limit so the backfill titles transcribe verbatim

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-006
- **Supersedes:** none

**Question:** Does T-006 phase 1 merge as built (708 non-test lines against the D-013 450-line budget), and does the decision title limit rise from 100 to 200 bytes?

**Options considered:**
1. Split `internal/decisions` into two packages or two PRs to fit the budget
2. Waive the budget for this ticket and merge phase 1 as built
3. Keep the 100-byte title limit and shorten the seven backfill titles that exceed it
4. Raise the title limit to 200 bytes so every backfill title transcribes verbatim

**Hive recommendation:** Waive the budget for phase 1; raise the title limit to 200 bytes

**Keeper choice:** The D-013 450-line budget is waived for T-006 phase 1, which merges as built at 708 non-test lines (PR #14). The title limit in T-006 section 4.2 is 200 bytes (was 100), so phase 2 transcribes all 87 backfill titles verbatim. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected splitting the package: parser, validator, renderer and append-only verifier are one round-trip contract and a split would add an import seam with no second consumer. Rejected shortening seven titles: the backfill is a transcription, and edited titles would make the record disagree with its source. The budget stays in force for every other ticket; this is a waiver, not a new limit.

## D-089: Size figures in briefs are physical non-test lines (`wc -l` of non-test Go files); T-018 phase A accepted at 527 against a 350 cap, and the architect re-estimates phases B and C

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-006, T-007, T-012, T-014, T-016, T-018, T-021, T-022
- **Supersedes:** none

**Question:** What does a brief's "non-test lines" figure count, and does T-018 phase A merge at 527 lines against its 350 cap?

**Options considered:**
1. Count code lines only (blanks and comments excluded)
2. Count physical lines, `wc -l` of non-test Go files touched
3. Trim phase A to fit the 350 cap
4. Accept phase A as built and re-estimate phases B and C with the same metric

**Hive recommendation:** Physical non-test lines; accept phase A; architect re-estimates B and C

**Keeper choice:** Every size figure in a brief (estimate, target, cap, stop value) counts physical non-test lines: `wc -l` over the non-test Go files the phase touches, blanks and comments included. T-018 phase A merges as built at 527 lines (PR #15). The architect re-estimates T-018 phases B and C in this metric before phase B is dispatched. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected code-only counting: it needs a tool and an argument at every review, while `wc -l` is one command whose answer nobody disputes. Rejected trimming required behavior to fit a cap: the cap is a signal that the estimate was wrong, not a reason to drop the merge CAS or the report lines the brief requires. The fix is to estimate honestly against the metric the guard uses.

## D-090: `waggle upgrade` reports seeded-file schema gaps with `[note]` lines; the Head Keeper edits seeded files by hand in the same PR

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-016
- **Supersedes:** none

**Question:** How does `waggle upgrade` handle a seeded file (`config.yaml`, `WAGGLE.md`, `local.yaml`) whose schema has changed between the old and the target build? (Q-T016-2)

**Options considered:**
1. Report-only: one `[note]` line per top-level key the build's `config.yaml.tmpl` has and the hive file lacks
2. In-place migration now, re-serializing the file through a comment-preserving parser

**Hive recommendation:** Report-only

**Keeper choice:** Report-only. `upgrade` prints one `[note]` line per top-level key present in the build's `config.yaml.tmpl` and absent from the hive's `config.yaml`; the Head Keeper edits seeded files by hand in the same hive PR. Versioned migration functions keyed by a `schema` field come later, when a release first changes the schema. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected in-place migration: it re-serializes a Head Keeper-owned file and needs a parser that preserves comments, which nothing on `main` has. No release has changed the schema yet, so the migration machinery would be built against an imagined change.

## D-091: `waggle upgrade` refuses a release-tag downgrade unless `--allow-downgrade`; commit pins are unordered

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-016
- **Supersedes:** none

**Question:** Does `waggle upgrade` check the direction of a pin move? (Q-T016-3)

**Options considered:**
1. Refuse when both pins are release tags and the target is lower, unless `--allow-downgrade`; print from and to for commit pins with no ordering check
2. No downgrade check at all

**Hive recommendation:** Refuse tag downgrades unless `--allow-downgrade`

**Keeper choice:** When both pins are release tags and the target is lower, `upgrade` refuses unless `--allow-downgrade`. Commit pins print from and to with no ordering check (a host repo does not contain Waggle's history). The semver compare is a hand-written numeric compare; a prerelease sorts below the same release. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected no check at all: the Keeper picks the target on purpose, but a stale binary on `PATH` is the common way to run the wrong build, and one flag is cheaper than a reverted hive PR.

## D-092: `waggle upgrade` prints the `waggle decide` input; a person runs `decide` in the same hive PR

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-006, T-016
- **Supersedes:** none

**Question:** Who writes the upgrade decision into `DECISIONS.md`? (Q-T016-4)

**Options considered:**
1. `upgrade` prints the `waggle decide` input and the Head Keeper, or the queen after an explicit Keeper answer, runs `waggle decide` in the same PR
2. `upgrade --apply` calls `decisions.Append` itself

**Hive recommendation:** Print the input; a person runs `decide`

**Keeper choice:** `upgrade` prints the `waggle decide` input (T-016 section 3.7) in both modes. The Head Keeper, or the queen after an explicit Keeper answer, runs `waggle decide` in the same hive PR (T-006 section 8). `upgrade` never writes `DECISIONS.md`. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected `--apply` calling `decisions.Append`: it turns "a command ran" into "a decision was made", adds `internal/decisions` as an import, and makes T-006 a code dependency of T-016 instead of a procedural one.

## D-093: The pin moves through a one-line edit of `config.yaml` that requires exactly one `pin:` under `waggle:` and preserves every other byte

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-016
- **Supersedes:** none

**Question:** How does `waggle upgrade` change the pin inside the seeded `config.yaml`? (Q-T016-5)

**Options considered:**
1. Narrow line edit: replace the value on the one `pin:` line under `waggle:`, refuse when there are zero or two, never re-serialize the file
2. Parse and re-render through `internal/config`

**Hive recommendation:** Narrow line edit

**Keeper choice:** `scaffold.WritePin` edits exactly the one `pin:` line under `waggle:`: the value is replaced; the key, indentation and trailing comment stay byte for byte; everything else in the file is untouched. Zero or two `pin:` lines are refused before any write. The file is never re-serialized through `config.Parse`. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected re-rendering through `internal/config`: it drops every comment in a Head Keeper-owned file, and the comments are the documentation of that file.

## D-094: No hive-only changes to managed files: a step goes into the template or not at all

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-007, T-016, T-018
- **Supersedes:** none

**Question:** May a hive keep a local change to a managed file across `waggle upgrade`? (Q-T016-6)

**Options considered:**
1. No per-hive exceptions and no exception list in config
2. A recorded exception mechanism (an `upgrade.keep:` list in `config.yaml`)

**Hive recommendation:** No exceptions

**Keeper choice:** No per-hive exceptions and no exception list in config. A step a hive wants in the gate workflow goes into the template or not at all. T-007's `gate decisions` step therefore targets the template or waits for T-018 phase C's CI shim. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected an `upgrade.keep:` list: it reopens the hand-edit door D-042 closed, and every kept file is one the next upgrade cannot reason about.

## D-095: `waggle upgrade` regenerates and prunes the running Keeper's shims only; other Keepers rerun `waggle init --force`

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-016, T-020
- **Supersedes:** none

**Question:** What does `waggle upgrade` do with the `.claude/agents/waggle-*.md` shims? (Q-T016-7)

**Options considered:**
1. Regenerate and prune this Keeper's shims through `--runtime`; others rerun `init --force` or a future `waggle shims`
2. Leave shims alone and tell every Keeper to rerun init

**Hive recommendation:** Regenerate and prune this Keeper's shims

**Keeper choice:** `upgrade` removes each shim carrying `# waggle:generated` whose role file is gone after the removals, and regenerates `.claude/agents/waggle-*.md` once when `--runtime=claude-code`. A shim without `# waggle:generated` is never touched. Other Keepers rerun `waggle init --force` or a future `waggle shims`. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected leaving shims alone: it makes T-020 section 7.2's "shims regenerated with `waggle-architect.md` removed" a manual step, which is the kind of step that gets skipped.

## D-096: Pins are full 40-hex commits or release tags, validated on both ends; `scaffold.ReadPin` is the shared reader and `internal/config/` stays read-only

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-004, T-016, T-022
- **Supersedes:** none

**Question:** What is a valid pin, and which package owns the reader? (Q-T016-8)

**Options considered:**
1. Full 40-hex or release tag, validated on both ends, `scaffold.ReadPin` shared, `internal/config/` read-only
2. Widen the T-016 lease to `internal/config/` and add `config.Config.Pin`

**Hive recommendation:** `scaffold.ReadPin`, `internal/config/` read-only

**Keeper choice:** A pin is a full 40-hex commit, a release tag (`vMAJOR.MINOR.PATCH` with optional prerelease), or the literal `unpinned`; never a short hash. `upgrade` validates both the "from" and the "to" value. `scaffold.ReadPin` in `internal/scaffold/upgrade.go` is the shared reader; `waggle status` switches to it in T-022 phase B. `internal/config/` stays read-only for T-016. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected widening the lease to `internal/config/`: `config.Config.Pin` is the cleaner long-term home, but a lease change on a package three tickets consume costs more now than a one-line switch in `status` later. Moving the reader into `internal/config` stays listed as a follow-up.

## D-097: The generated `CLAUDE.md` is tracked like every other init output; the Head Keeper commits ours before T-020 phase C's first upgrade

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-011, T-016, T-020
- **Supersedes:** none

**Question:** What happens to the untracked generated `CLAUDE.md` in our main checkout, which trips every clean-tree check? (Q-T016-9)

**Options considered:**
1. Track it like all init output, no `git add` inside init or upgrade, Head Keeper commits ours as step 0 of T-020 phase C
2. Ignore `CLAUDE.md` in the template `.gitignore` block again
3. Let `upgrade` tolerate one untracked import file

**Hive recommendation:** Track it

**Keeper choice:** `CLAUDE.md` (or `AGENTS.md`) is tracked like every other file init writes; the markers separate generated from hand-written content. Neither `init` nor `upgrade` runs `git add`; init's closing hint gains `commit CLAUDE.md (or AGENTS.md) and .gitignore with the rest`. In our hive the Head Keeper commits `CLAUDE.md` as step 0 of T-020 phase C section 7.2, before the first `upgrade` run. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected ignoring it again: project content outside the markers becomes unignorable. Rejected a tolerance in the clean-tree rule: one special case in a precondition that exists to be absolute.

## D-098: The `main` ruleset requires product-ci and verify-leases-and-tiers plus one code owner review; every merge uses a logged admin bypass

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-015
- **Supersedes:** none

**Question:** What does the `main` ruleset enforce after T-015, and how do PRs merge when GitHub never counts the author's own approval?

**Options considered:**
1. Not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** Ruleset `main` (id 25310890) requires `product-ci` and `verify-leases-and-tiers`, up-to-date branches, blocks force-push, restricts deletion, and requires one code owner review with stale reviews dismissed; admin bypass is set to always. Because GitHub never counts the PR author's own approval, every merge uses `gh pr merge <n> --merge --admin` (a logged bypass). Auto-merge is disabled at the repo level. Applied and read back by the Head Keeper on 2026-10-17.

**Rationale:** Not recorded

## D-099: T-016 accepted at 737 physical non-test lines against the 700 stop after the guard must-fix round

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-016
- **Supersedes:** none

**Question:** Does T-016 merge at 737 physical non-test lines against the brief's 700 stop, after the guard must-fix round (pin with trailing text refused; symlinked write paths refused)?

**Options considered:**
1. Accept as built
2. Split the `git.go` lift into its own PR
3. Trim to 700

**Hive recommendation:** Accept

**Keeper choice:** Accept. T-016 merges at 737 physical non-test lines. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** The 54 added lines are the two security fixes and their checks. Splitting costs a forager plus guard pass and delays T-020 phase C. Trimming trades readability on a Tier 3 command for a number.

## D-100: From T-020 phase C on, every Tier 3 PR gets two guard reviews, one on sonnet and one on a third model; Tier 2 keeps one sonnet guard

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-020
- **Supersedes:** none

**Question:** Should Tier 3 PRs get a second guard review on a different model, and from when?

**Options considered:**
1. Yes, from T-020 phase C
2. Sonnet only
3. A second guard on all PRs

**Hive recommendation:** Yes, from T-020 phase C

**Keeper choice:** From T-020 phase C on, every Tier 3 PR (T-020 and any later Tier 3 ticket) gets two guard reviews: one on sonnet and one on a third model different from both sonnet and the builder's model (Fable while foragers run on Opus). Tier 2 keeps a single sonnet guard. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Tier 3 code rewrites the control plane, so a missed defect costs more than about 100k extra tokens per Tier 3 PR. Doubling Tier 2 review spend is not justified by today's record (the sonnet guard found the T-016 symlink escape).

## D-101: Cell runtime: a `CellRuntime` interface with an OS sandbox default (Seatbelt on macOS, bubblewrap on Linux) behind a host egress proxy, plus a Docker-compatible container backend

- **Date:** 2026-10-17
- **Kind:** strategy
- **Tickets:** T-023, T-024
- **Supersedes:** none

**Question:** What runs a worker's Cell, so that RFC-0003's targets (zero credentials, read-only repository with read-write lease, egress lockdown) are enforced?

**Options considered:**
1. Not recorded

**Hive recommendation:** A `CellRuntime` interface with an OS sandbox default (Seatbelt on macOS, bubblewrap on Linux) behind a host egress proxy, plus a Docker-compatible container backend

**Keeper choice:** A `CellRuntime` interface with an OS sandbox default (Seatbelt on macOS, bubblewrap on Linux) behind a per-Cell host egress proxy, plus a Docker-compatible container backend. T-024 builds it. RFC-0003 gains amendment A2, which the Head Keeper approves in its own product PR. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Not recorded

## D-102: Licensing: Apache-2.0 for the CLI and daemon, MIT-0 for templates, DCO sign-off, SPDX headers, copyright Level 19 Enterprises, LLC

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-025
- **Supersedes:** none

**Question:** Which licenses, contributor rule and file headers does Waggle ship with?

**Options considered:**
1. Not recorded

**Hive recommendation:** Apache-2.0 for the CLI and daemon, MIT-0 for templates, plus DCO and SPDX headers

**Keeper choice:** The CLI and daemon are Apache-2.0 and the templates are MIT-0. Contributions carry a DCO sign-off, files carry SPDX headers, and the copyright holder is Level 19 Enterprises, LLC (D-046), stated in NOTICE. T-025 builds it. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Not recorded

## D-103: GitHub merge queue is treated as unavailable on the Team plan; the org stays on Team, the admin-merge loop stays, and `native_queue` stays opt-in

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-015, T-018
- **Supersedes:** none

**Question:** Is GitHub's native merge queue available to waggle-sh/waggle (an organization repo, D-024) on the Team plan (D-047), and what follows if it is not?

**Options considered:**
1. Not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** Treat the native merge queue as unavailable on GitHub Team and stay on Team. Merges keep the admin-merge polling loop (D-098). `merge_strategy: native_queue` stays opt-in (D-064, D-071), and `waggle doctor` reports queue availability per repo (T-018 phase C). Revisit when the repo goes public. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Not recorded

## D-104: The queen commits our generated CLAUDE.md for the Head Keeper as T-020 phase C step 0 (PR #29)

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-016, T-020
- **Supersedes:** none

**Question:** Who makes D-097's Head Keeper commit of our hive's generated `CLAUDE.md`?

**Options considered:**
1. Not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** The queen made the commit on the Head Keeper's behalf (`0a4bed4`, PR #29, merged at `97baeda`) and the Head Keeper approved the PR. This completes T-020 phase C section 7.2 step 0 (D-097). Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Not recorded

## D-105: T-018 phase B accepted at 687 physical non-test lines against the 650 stop after the Keeper-requested N1, N2 and N4 hardening

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-018
- **Supersedes:** none

**Question:** Does T-018 phase B merge at 687 physical non-test lines (D-089 metric) against its 650 stop, after hardening commit `c85c783` closed guard notes N1, N2 and N4?

**Options considered:**
1. Not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** Accept. T-018 phase B merges at 687 physical non-test lines (PR #28, merged at `b1e4a3a`). Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** The lines over the stop are the hardening the Keeper asked for (gh stdout and stderr caps, a per-call gh deadline with `WaitDelay`, token redirects pinned to the same https origin), as with D-099.

## D-106: T-006 phase 2: a forager adds the missing backfill JSON from D-088 onward and runs `waggle decide` for every entry in a hive PR the Head Keeper approves

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-006
- **Supersedes:** none

**Question:** Who runs the T-006 phase 2 backfill, and how does it land?

**Options considered:**
1. Not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** A forager adds the missing `internal/decisions/testdata/backfill/` JSON files (D-088 onward) transcribed from Appendix A, then runs `waggle decide` once per entry, in order, for every Appendix A entry, in a hive PR the Head Keeper approves. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Not recorded

## D-107: Non-blocking review follow-ups are folded into the brief that next touches the code

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-006, T-012, T-014, T-018
- **Supersedes:** none

**Question:** Where do the non-blocking guard follow-ups from T-014 phase A, T-016, T-006 and T-018 phase B go?

**Options considered:**
1. Not recorded

**Hive recommendation:** Not recorded

**Keeper choice:** Each follow-up becomes one line in the brief that next touches the code: T-014 phase B caps `lease.json` size in `loadClaims`; T-018 phase C takes notes N3 and N5 to N10; T-012 phase B (or the next brief leasing `internal/scaffold/`) takes `roster.json` value escaping and the `waggle init` symlink hole; T-006 takes `Parse` accepting a bare CR in a title. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Not recorded

## D-108: T-023 `waggle dispatch` is built now, host-only with a `CellRuntime` seam, ahead of Cells (D-014 order); supersedes D-083's sequencing for T-023 only

- **Date:** 2026-10-17
- **Kind:** strategy
- **Tickets:** T-021, T-023, T-024
- **Supersedes:** D-083

**Question:** Does T-023 wait for the Cell runtime, or follow D-014 and ship host-only with a Cell seam? (Q-T023-1)

**Options considered:**
1. Follow D-014: approve T-023 now, host-only with the section 7.3 seam, superseding D-083's "after the Cell runtime decision" for T-023 only
2. Hold T-023 until the Cell runtime is decided

**Hive recommendation:** Follow D-014, host-only with the seam

**Keeper choice:** T-023 is approved now and runs workers on the host behind the `CellRuntime` seam of its section 7.3. This supersedes D-083's "T-023 after the Cell runtime decision" for T-023 only. Cells (T-024) stay the slice after the `run.sock` loop. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected holding T-023: it delays own honey (D-044) by at least one RFC amendment.

## D-109: Until Cells exist, dispatch runs a worker on the host only with `--no-cell` on each invocation, printing and recording what is not enforced

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** May dispatch run a worker outside a Cell, and how is that allowed? (Q-T023-2)

**Options considered:**
1. Allowed only with `--no-cell` on every invocation, the not-enforced list printed and recorded, and `cell: host` in the result
2. A config key that allows host runs hive-wide

**Hive recommendation:** `--no-cell` on every invocation

**Keeper choice:** Host runs need `--no-cell` on every invocation; without it dispatch exits 2. Dispatch prints the not-enforced list of T-023 section 8, records it in the run record, and writes `cell: host` in `result.json`. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected a hive-wide config key: it is a Tier 3 template change, and it is easy to forget it is on.

## D-110: Headless workers get an explicit tool allowlist and a deny list (no git push, gh, curl, wget, ssh, web fetch or web search)

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Which tools may a headless dispatch worker use? (Q-T023-3)

**Options considered:**
1. An explicit allowlist in the generated settings file plus a deny list
2. Skip permission prompts entirely

**Hive recommendation:** Allowlist plus deny list

**Keeper choice:** The generated settings file allows the read, edit and write tools, and Bash only for the build, test, vet and format commands in `WAGGLE.md` section 3 plus `git add`, `git commit`, `git status`, `git diff`, `git log` and `git show`. It denies `git push`, `gh`, `curl`, `wget`, `ssh`, web fetch and web search. Dispatch never passes `--dangerously-skip-permissions` by default. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Accepted that some runs end on a denied tool and need a rerun. Rejected skipping permission prompts: the post-run check would become the only guard.

## D-111: Dispatch workers use the Keeper's existing Claude Code login with user settings excluded; no new credential

- **Date:** 2026-10-17
- **Kind:** credentials
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Which model login does a dispatch worker use? (Q-T023-4)

**Options considered:**
1. The Keeper's existing Claude Code login, no new credential, user settings excluded, exposure stated
2. An isolated Claude config directory per run with an API key in the env

**Hive recommendation:** The Keeper's existing login

**Keeper choice:** Workers use the Keeper's existing Claude Code login and no new credential is created. A worker loads none of the Keeper's user settings, plugins, connectors or global `CLAUDE.md`, and none of the repo's `.claude/settings.json` hooks (T-023 section 8). Every run states the exposure. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected a per-run API key: cleaner isolation, but it moves spend to API billing, which is a separate spend decision.

## D-112: A dispatch lease comes from `--dir` and `--gate` flags the queen copies from the brief, never from parsing the brief

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Where does `waggle dispatch` get the lease for a run? (Q-T023-5)

**Options considered:**
1. `--dir` and `--gate` flags the queen copies from the brief
2. Parse a machine-readable lease block in the brief

**Hive recommendation:** Flags

**Keeper choice:** The queen copies the phase's Lease line into `--dir` and `--gate`. Comb validation applies unchanged. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected a lease block in the brief: a Tier 2 steering file would then grant write scope.

## D-113: Dispatch reads briefs and review files only from base, so a brief merges in a hive PR before its first dispatch

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Where does dispatch read a ticket's brief and review file? (Q-T023-6)

**Options considered:**
1. Only from base, so a brief merges before its first dispatch
2. A `--brief-file` flag reading the main checkout

**Hive recommendation:** Only from base

**Keeper choice:** Dispatch reads the brief and the review file only from the base revision. A brief merges (hive PR) before its first dispatch. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected `--brief-file`: it keeps today's habit of uncommitted briefs, and lets unreviewed steering text reach workers.

## D-114: One lease and tier check: a pure `comb.CheckChanges` in `internal/comb`, written by whichever of T-014 phase B and T-023 phase C lands first and called by both

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-014, T-023
- **Supersedes:** none

**Question:** Where does the per-commit lease and tier check live, so that `waggle gate verify` and `waggle dispatch` share one implementation? (Q-T023-7)

**Options considered:**
1. A pure `comb.CheckChanges(lease, changes) []Violation` in `internal/comb`, written by whichever of T-014 phase B and T-023 phase C lands first and called by both

**Hive recommendation:** `comb.CheckChanges` in `internal/comb`

**Keeper choice:** A pure `comb.CheckChanges(lease, changes) []Violation` in `internal/comb`, written by whichever of T-014 phase B and T-023 phase C lands first and called by both; T-014 section 4.3 says so. Queen default, within authority; approved with the other T-023 defaults on the Head Keeper's queen card on 2026-10-17.

**Rationale:** D-017 wants the same check locally and in CI, and `internal/dispatch` cannot import a check that lives in `cmd/waggle` (package `main`).

## D-115: Dispatch pushes the worker branch from the host after every check passes; it never force-pushes and never opens a PR, and `--no-push` skips the push

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-018, T-023
- **Supersedes:** none

**Question:** Who pushes a dispatched worker's branch? (Q-T023-8)

**Options considered:**
1. Dispatch pushes from the host with the Keeper's git auth after every check passes; never force; never opens a PR; `--no-push` skips it

**Hive recommendation:** Dispatch pushes from the host after the checks

**Keeper choice:** Dispatch pushes the branch from the host with the Keeper's git auth, only after every post-run check passes. It never force-pushes and never opens a PR; `--no-push` skips the push. The worker never pushes. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Not recorded

## D-116: T-023 phase D adds D-100 second-guard model resolution to `internal/runtime/policy.go`, so Tier 3 reviews also run through dispatch

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-012, T-023
- **Supersedes:** none

**Question:** How does dispatch run the D-100 second guard on Tier 3 PRs? (Q-T023-9)

**Options considered:**
1. Phase D adds second-guard resolution to `internal/runtime/policy.go`
2. The second guard stays a subagent until a T-012 follow-up

**Hive recommendation:** Phase D adds it

**Keeper choice:** Phase D adds second-guard resolution to `internal/runtime/policy.go`: a model different from both the builder's and the first guard's, failing with a config hint when none exists. `waggle dispatch --role guard --second` uses it. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Not recorded

## D-117: Dispatch cutover: one trial ticket after T-023 phase D, cutover on a Head Keeper card, Claude Code subagents kept as the rollback path

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** How does our hive switch from Claude Code subagents to `waggle dispatch`? (Q-T023-10)

**Options considered:**
1. One trial ticket through dispatch after phase D, cutover on a Keeper card, subagents kept as the rollback path, phase E not a precondition
2. Run the trial ticket both ways for comparison

**Hive recommendation:** One trial ticket, cutover on a card

**Keeper choice:** One trial ticket runs through dispatch after phase D. The cutover is a Head Keeper card, and the subagents stay as the rollback path. Phase E is not a precondition. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected running the trial both ways: about double the tokens for that ticket.

## D-118: `keeper_id` in `local.yaml` is the Keeper's GitHub login; the `local-keeper` placeholder is refused when the Comb has a remote

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** How does dispatch identify the Keeper in claims? (Q-T023-11)

**Options considered:**
1. `keeper_id` from `local.yaml`, set to the GitHub login (D-016), with the `local-keeper` placeholder refused when the Comb has a remote

**Hive recommendation:** `keeper_id` from `local.yaml`

**Keeper choice:** `keeper_id` comes from `local.yaml` and is the Keeper's GitHub login (D-016). The `local-keeper` placeholder is refused when the Comb has a remote and allowed for local-only hives. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** The placeholder would collide across Keepers sharing a remote.

## D-119: T-023 phase F sets `forge.remote: origin` in our hive's `config.yaml`, so claims reach the remote and the CI gate can verify them

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Does our hive set `forge.remote`? (Q-T023-12)

**Options considered:**
1. Phase F sets `forge.remote: origin` in our `config.yaml`

**Hive recommendation:** Set it in phase F

**Keeper choice:** Phase F sets `forge.remote: origin` in our hive's `config.yaml`, so claims are pushed and the CI gate can verify them (D-051). Every claim write then needs network access and the Keeper's git push auth. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Not recorded

## D-120: Cell inference credential: the egress proxy injects it and the Cell holds none by default; our hive opts into `cell.inference_credential: passthrough`, reported as not enforced

- **Date:** 2026-10-17
- **Kind:** credentials
- **Tickets:** T-023, T-024
- **Supersedes:** none

**Question:** How does an agent inside a Cell authenticate to the inference endpoint? (Q-T024-1)

**Options considered:**
1. The egress proxy injects the credential and the Cell holds none; a subscription login the proxy cannot inject is unsupported in Cells in v0.1
2. An opt-in `cell.inference_credential: passthrough` that places the agent's own login token in the Cell and reports the credentials target as not enforced

**Hive recommendation:** Proxy injection

**Keeper choice:** The product default is proxy injection: the agent talks to the in-Cell forwarder, the host proxy adds the credential over TLS, and the Cell holds zero credentials. `cell.inference_credential: passthrough` is also built as an opt-in, and our hive opts into it; `waggle doctor` then reports the zero-credentials target as not enforced. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Not recorded

## D-121: Cells never write `.git`; `waggled` commits on the host after checking the diff against the lease

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-024, T-025
- **Supersedes:** none

**Question:** Who commits a Cell worker's changes? (Q-T024-2)

**Options considered:**
1. Cells never write `.git`; `waggled` commits on the host after checking the diff against the lease, with the `T-<id>:` subject
2. Give the Cell write access to `.git`

**Hive recommendation:** `waggled` commits on the host

**Keeper choice:** `.git` is read-only inside every Cell. `waggled` commits on the host after checking the diff against the lease, with the `T-<id>:` subject. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected `.git` write access: a worker could write any ref, including `refs/heads/main` and `refs/waggle/claims/*`, from inside the sandbox.

## D-122: T-024 technical defaults: `waggled` copies a per-Cell `Out` dir into hive paths, registries stay closed with `GOPROXY=off`, and the container backend calls docker or podman directly

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-024
- **Supersedes:** none

**Question:** How does forager and guard output leave a Cell (Q-T024-3), are package registries reachable under `strict` (Q-T024-6), and does the container backend use the devcontainer CLI (Q-T024-9)?

**Options considered:**
1. An `Out` dir copied by `waggled`; registries closed with `GOPROXY=off` and a read-only module cache; `docker` or `podman` called directly
2. Files over `run.sock`; `proxy.golang.org` and `sum.golang.org` allowed by default; `devcontainer up` required

**Hive recommendation:** The three defaults

**Keeper choice:** Each Cell gets an `Out` dir outside the repo that `waggled` validates and copies into `.waggle/hive/reviews/` and `.waggle/hive/media/`. `strict` keeps registries closed: Go builds use `GOPROXY=off` with the host module cache mounted read-only, and other hosts go in `cell.egress_allow` by exact name. The container backend calls `docker` or `podman` and reads only `image` and `runArgs` from `devcontainer.json`. Queen default, within authority; approved on the Head Keeper's queen card on 2026-10-17.

**Rationale:** Rejected `devcontainer up`: a Node dependency, and it runs lifecycle hook commands Waggle cannot vet.

## D-123: T-024 phases A to E dispatch after T-023 merges; phase F waits for the `run.sock` loop

- **Date:** 2026-10-17
- **Kind:** strategy
- **Tickets:** T-023, T-024
- **Supersedes:** none

**Question:** D-014 puts Cells after dispatch and the `run.sock` loop. May T-024 start before the loop? (Q-T024-4)

**Options considered:**
1. Phases A to E after T-023 merges, phase F after the `run.sock` loop
2. Hold every phase until the loop merges

**Hive recommendation:** Phases A to E after T-023

**Keeper choice:** T-024 phases A to E dispatch after T-023 merges; they are libraries with no caller. Phase F, the wiring, waits for the `run.sock` loop. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Not recorded

## D-124: T-024 v0.1 scope: container backend on Linux hosts and CI only, macOS only if the socket check passes; no resource limits on OS sandboxes; no seccomp filter

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-024
- **Supersedes:** none

**Question:** v0.1 scope for the container backend on macOS (Q-T024-5), resource limits on the OS sandbox backends (Q-T024-10) and a seccomp filter for bubblewrap (Q-T024-11)

**Options considered:**
1. Container backend on Linux and CI only, macOS unless the socket check passes; no OS sandbox resource limits; no seccomp in v0.1
2. A TCP transport for `run.sock` and the proxy; cgroups through `systemd-run --user --scope` on Linux; a seccomp filter in phase C

**Hive recommendation:** The three defaults

**Keeper choice:** In v0.1 the container backend is supported on Linux hosts and CI; on macOS the probe reports it unsupported unless the T-024 section 7 socket check passes. The Seatbelt and bubblewrap backends set no resource limits and `waggle doctor` says so. bubblewrap ships without a seccomp filter; a follow-up ticket adds a denylist filter. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected a TCP transport: it needs a per-Cell secret and weakens the zero-credentials target. Rejected seccomp in phase C: about 100 more lines and a syscall table per architecture.

## D-125: Cell sandbox tests run in two CI jobs path-filtered to `internal/cell/**`, macOS for Seatbelt and Linux for bubblewrap, added by Head Keeper commit

- **Date:** 2026-10-17
- **Kind:** spend
- **Tickets:** T-024
- **Supersedes:** none

**Question:** Do the sandbox conformance tests run in CI? (Q-T024-7)

**Options considered:**
1. Two jobs in `ci.yml` path-filtered to `internal/cell/**`, one on `macos-latest` and one on `ubuntu-latest` with the AppArmor setting changed
2. Local evidence only

**Hive recommendation:** Two path-filtered jobs

**Keeper choice:** `ci.yml` gains two jobs path-filtered to `internal/cell/**`: one on `macos-latest` for Seatbelt, one on `ubuntu-latest` that sets `kernel.apparmor_restrict_unprivileged_userns=0` for bubblewrap. `ci.yml` is Tier 3, so the Head Keeper commits it. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** macOS minutes bill at 10 times Linux on private repos: about 40 included minutes per macOS run out of GitHub Team's 3,000 per month.

## D-126: CODEOWNERS gains `/internal/cell/ @paulternate` by Head Keeper commit; `internal/cell/` is not a hardcoded Tier 3 path

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-015, T-024
- **Supersedes:** none

**Question:** How is `internal/cell/` protected? (Q-T024-8)

**Options considered:**
1. Add `/internal/cell/ @paulternate` to CODEOWNERS, not a hardcoded Tier 3 entry
2. Rely on guard Tier classification alone

**Hive recommendation:** CODEOWNERS entry

**Keeper choice:** The Head Keeper adds `/internal/cell/ @paulternate` to CODEOWNERS before T-024 phase A, as in T-015. It is not a hardcoded Tier 3 entry. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** A hardcoded Tier 3 path would apply to every host repo's own `internal/cell/` (the D-043 reasoning used for `docs/GLOSSARY.md`).

## D-127: `cell.enforce_ro_root: false` is refused at config load; the key stays only so existing files parse

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-004, T-024
- **Supersedes:** none

**Question:** What does `cell.enforce_ro_root: false` do once Cells exist? (Q-T024-12)

**Options considered:**
1. Refused at config load
2. Honored as an opt-out

**Hive recommendation:** Refused

**Keeper choice:** `enforce_ro_root: false` is refused at config load. The key stays only so existing files parse. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected honoring it: a Tier 3 config edit would turn off the lease boundary.

## D-128: Files carry only the SPDX identifier; the copyright line lives in NOTICE and `LICENSES/MIT-0.txt`

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-025
- **Supersedes:** none

**Question:** Does each file carry a copyright line? (Q-T025-1)

**Options considered:**
1. No: SPDX identifier only, copyright in `NOTICE` and `LICENSES/MIT-0.txt`
2. `SPDX-FileCopyrightText: 2026 Level 19 Enterprises, LLC` on product files only

**Hive recommendation:** SPDX identifier only

**Keeper choice:** Files carry only the SPDX identifier. The copyright line lives in `NOTICE` and `LICENSES/MIT-0.txt`. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** A per-file line adds year churn, and in MIT-0 templates it would stamp our copyright into every host repo's own files.

## D-129: One license rule: product files are Apache-2.0, templates and hive files are MIT-0; CONTRIBUTING.md, ci.yml and CODEOWNERS switch to Apache-2.0

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-025
- **Supersedes:** none

**Question:** Which license covers product docs and repo config? (Q-T025-2)

**Options considered:**
1. One rule: product is Apache-2.0, templates and hive files are MIT-0
2. Leave existing MIT-0 headers on docs and config as they are

**Hive recommendation:** One rule

**Keeper choice:** Product files are Apache-2.0; templates and hive files are MIT-0. `CONTRIBUTING.md`, `ci.yml` and `CODEOWNERS` switch from MIT-0 headers to Apache-2.0. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Level 19 Enterprises, LLC holds all copyright today, so the switch needs nobody else's consent.

## D-130: The SPDX header check is a Go test in `internal/license/` that product-ci already runs

- **Date:** 2026-10-17
- **Kind:** technical
- **Tickets:** T-025
- **Supersedes:** none

**Question:** Where does the header check run? (Q-T025-3)

**Options considered:**
1. A Go test in `internal/license/` run by `product-ci`
2. A shell step in `ci.yml`
3. A `waggle gate` step

**Hive recommendation:** Go test in `internal/license/`

**Keeper choice:** A Go test in `internal/license/`, run by the existing `product-ci` job. Queen default, within authority; approved on the Head Keeper's queen card on 2026-10-17.

**Rationale:** Rejected a shell step: a Tier 3 edit that does not run locally. Rejected `waggle gate`: it would impose our license map on every hive (D-043).

## D-131: DCO is enforced by a `dco` job in product CI, required in the `main` ruleset; no third-party app

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-015, T-025
- **Supersedes:** none

**Question:** How is DCO sign-off enforced? (Q-T025-4)

**Options considered:**
1. A `dco` job in product CI plus a required check in the ruleset
2. The DCO GitHub App
3. A `waggle gate` step

**Hive recommendation:** `dco` job plus required check

**Keeper choice:** A `dco` job in product CI, added as a required check in the `main` ruleset. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** No third-party app gets access to a private repo, and the rule lives in a file the hive reviews. Rejected a `waggle gate` step: it would impose DCO on every hive.

## D-132: Worker commits use the Keeper's git identity with `-s`, and that Keeper is the person certifying the DCO

- **Date:** 2026-10-17
- **Kind:** policy
- **Tickets:** T-023, T-024, T-025
- **Supersedes:** none

**Question:** DCO is a statement by a person. Who signs off worker commits? (Q-T025-5)

**Options considered:**
1. Workers commit under the Keeper's git identity with `-s`, and that Keeper certifies
2. A person adds sign-off by hand before review

**Hive recommendation:** Keeper identity with `-s`

**Keeper choice:** Workers commit under the Keeper's git identity with `-s`, and that Keeper is the certifying person; CONTRIBUTING says so. When `waggled` commits on the host (D-121), it adds the trailer from the Keeper's identity. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected sign-off by hand: it means rewriting every worker branch.

## D-133: T-025 scope defaults: merge commits signed off, no retroactive sign-off, hive records and JSON unchecked, legal files code-owned, copyright year 2026

- **Date:** 2026-10-17
- **Kind:** scope
- **Tickets:** T-025
- **Supersedes:** none

**Question:** Merge commits, history, hive records, JSON files, code owners for legal files and the copyright year (Q-T025-6 to Q-T025-11)

**Options considered:**
1. The six defaults
2. Exempt merge commits; require MIT-0 headers in `.waggle/hive/**`; REUSE-style `.license` sidecars for JSON; guard review only for legal files; a yearly copyright range

**Hive recommendation:** The six defaults

**Keeper choice:** Merge commits are checked like any other (`git merge --signoff`). No retroactive sign-off: enforcement starts when `dco` is required, and open PRs then run `git rebase --signoff`. `.waggle/hive/**` is not checked. JSON files are exempt and follow the path rule. `/LICENSE`, `/NOTICE` and `/LICENSES/` join CODEOWNERS for the Head Keeper. The year is `2026`, never bumped. Decided by the Head Keeper via queen card on 2026-10-17.

**Rationale:** Rejected exempting merge commits: conflict resolutions would go uncertified. Rejected hive headers: a header duty on every guard review and a Head Keeper backfill commit.

## D-134: T-023 phase A gets a short fix pass before merge: `--restricted`, Write and Edit scoped to `./**`, MalformedLines fix, more `default_args` refusals

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Does T-023 phase A merge as reviewed, or after a short fix pass on the guard findings?

**Options considered:**
1. Short fix pass before merge
2. Merge as reviewed and carry every finding to phase C

**Hive recommendation:** Short fix pass before merge

**Keeper choice:** Phase A gets one short fix pass before merge: add `--restricted`, scope Write and Edit to `./**`, fix MalformedLines, and refuse `--effort`, `--fallback-model`, `--remote-control`, `--environment`, `--chrome`, `--ide` and a bare positional word in `default_args`. Other findings carry to later phases. Decided by the Head Keeper on 2026-10-17.

**Rationale:** The fixes are small and close the widest gaps the sonnet and fable guards found while the adapter has no caller. Rejected carrying them: phase C would launch workers on an adapter known to be wider than D-110.

## D-135: Auto-compaction stays off in workers; long sessions end at max_tokens until T-023 phase E

- **Date:** 2026-10-18
- **Kind:** technical
- **Tickets:** T-021, T-023
- **Supersedes:** none

**Question:** With `DISABLE_AUTO_COMPACT=1` set by the T-023 phase A adapter and no handoff loop yet, what happens to long worker sessions?

**Options considered:**
1. Keep compaction off; long sessions end at max_tokens and are rerun until phase E
2. Allow compaction until phase E ships the handoff loop

**Hive recommendation:** Keep compaction off; long sessions end at max_tokens and are rerun until phase E

**Keeper choice:** Auto-compaction stays off in workers (`DISABLE_AUTO_COMPACT=1`). Until T-023 phase E adds the handoff loop, a long session ends at `max_tokens` (or with an API error when the model window is smaller) and the ticket is rerun. Closes the T-021 compaction carry. Decided by the Head Keeper on 2026-10-17.

**Rationale:** A compacted session loses context silently, which T-021 S5 rules out. Rejected allowing compaction until phase E: it trades a loud, rerunnable stop for a quiet loss of state.

## D-136: Clarifies D-089: the size metric is added non-test lines, not the whole length of every touched file

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-016, T-018, T-021, T-023
- **Supersedes:** none

**Question:** D-089 says "`wc -l` of the non-test Go files the phase touches". When a phase edits existing files, does the figure count whole files or only the lines the phase adds?

**Options considered:**
1. Added non-test lines (numstat additions over non-test files)
2. Whole touched files (`wc -l` of every non-test file touched)

**Hive recommendation:** Added non-test lines (numstat additions over non-test files)

**Keeper choice:** Every size figure in a brief counts added non-test lines: the additions `git diff --numstat` reports over non-test files, blanks and comments included. Whole touched files are not counted. For phases that only add files this equals D-089's `wc -l`. T-023 phase A is 469 under this reading. Decided by the Head Keeper on 2026-10-17.

**Rationale:** Counting whole touched files penalizes a one-line edit to a long file and made T-023 phase A read as 934 against a 520 stop. Added lines is the metric the guards already used for phases that added files, and it stays one undisputed command.

## D-137: A `default_args` allowlist replaces the denylist, as a precondition for T-023 phase C

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Does `default_args` keep a denylist of refused Claude Code flags, or move to an allowlist before phase C launches workers?

**Options considered:**
1. Allowlist before phase C; anything else refused
2. Keep the denylist and extend it each Claude Code release

**Hive recommendation:** Allowlist before phase C; anything else refused

**Keeper choice:** `default_args` moves to an allowlist of known flags, and any other flag or bare word is refused. This is a precondition for T-023 phase C; phase A merged with the denylist. Decided by the Head Keeper on 2026-10-17.

**Rationale:** A denylist against a third-party CLI whose flag set changes every release is structurally unsound: in 2.1.302 several unrefused flags widen the worker or break policy (fable and sonnet guard reviews of T-023 phase A). Rejected extending the denylist: every release reopens the gap.

## D-138: T-020 phase C is drafted by a worker in an isolated worktree, reviewed by two guards, merged with Head Keeper approval

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-020
- **Supersedes:** none

**Question:** Who drafts T-020 phase C (the hive PR that renames our roles), and when?

**Options considered:**
1. A worker drafts it (the D-086 pattern) after hive-records-4 merges and nothing else is running
2. The Head Keeper runs the upgrade and edits the hive files by hand

**Hive recommendation:** A worker drafts it (the D-086 pattern) after hive-records-4 merges and nothing else is running

**Keeper choice:** A worker drafts T-020 phase C in an isolated worktree, following the D-086 pattern, once hive-records-4 has merged and nothing else is running. Two guards review it (sonnet and fable) and the Head Keeper approves the merge. The Keeper renames the model_overrides keys in his own gitignored local.yaml himself. Decided by the Head Keeper on 2026-10-18; recorded by the queen.

**Rationale:** The rename is mechanical but touches Tier 3 control files and many briefs; a worker draft plus two independent guard reviews gives the Head Keeper a checked diff instead of a hand edit. local.yaml is gitignored and per Keeper, so no PR can change it.

## D-139: T-014 phase B accepted at 550 added non-test lines against the 450 stop

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-014
- **Supersedes:** none

**Question:** T-014 phase B came in at 550 added non-test lines against a 450 stop value. Accept it or split it?

**Options considered:**
1. Accept at 550
2. Split the tiers check into its own phase

**Hive recommendation:** Accept at 550

**Keeper choice:** T-014 phase B is accepted at 550 added non-test lines against the 450 stop. The overrun is the scope D-107 and D-114 added to the phase (the lease.json size cap and the shared comb.CheckChanges) plus the B1 fix, counted per D-089 and D-136. Decided by the Head Keeper on 2026-10-18; recorded by the queen.

**Rationale:** The extra lines are scope added by rulings after the estimate, not drift. Splitting a reviewed, passing phase would cost another review round for no safety gain.

## D-140: CODEOWNERS covers .github/workflows/ and .github/actions/; workflow edits need code owner review

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-014, T-015
- **Supersedes:** none

**Question:** pull_request workflows run from the PR's merge ref, so a PR can edit the gate workflow and run its own edit (T-014b fable F4). How is that closed?

**Options considered:**
1. CODEOWNERS on .github/workflows/ and .github/actions/ with required code owner review
2. Rely on the Head Keeper reading the diff before a bypass merge

**Hive recommendation:** CODEOWNERS on .github/workflows/ and .github/actions/ with required code owner review

**Keeper choice:** CODEOWNERS covers .github/workflows/ and .github/actions/ for the Head Keeper, so any workflow or action edit needs code owner review (T-014b fable F4: pull_request workflows run from the merge ref). Decided by the Head Keeper on 2026-10-18; recorded by the queen.

**Rationale:** Building the checker from BASE_SHA closes the Go side, but the YAML itself runs from the merge ref, so a PR could replace the gate step with exit 0. Code owner review is the platform control that covers the YAML.

## D-141: comb keeps rejecting claims under .waggle/; hive-record commits merge by Head Keeper bypass

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-014
- **Supersedes:** none

**Question:** Every hive-record commit fails the leases check because comb rejects claims under .waggle/ (T-014b fable F10). Allow claims there, or keep bypass merges?

**Options considered:**
1. Keep rejecting claims under .waggle/; hive records merge by Head Keeper bypass
2. Allow leases on .waggle/hive/ for hive-record commits

**Hive recommendation:** Keep rejecting claims under .waggle/; hive records merge by Head Keeper bypass

**Keeper choice:** comb keeps rejecting claims under .waggle/. Every hive-record commit fails the leases check and merges by Head Keeper bypass (T-014b fable F10). Revisit only if bypass volume becomes a problem. Decided by the Head Keeper on 2026-10-18; recorded by the queen.

**Rationale:** Keeping hive records out of any lease keeps workers from writing them. The cost is a bypass merge per hive PR, which is acceptable at current volume.

## D-142: The T-020 phase C brief sweep covers every brief with a phase still to dispatch

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-020
- **Supersedes:** none

**Question:** T-020 section 7.3 sweeps open briefs, defined as briefs with no Merge commit on main. Do multi-phase briefs with a phase still to dispatch get the sweep too?

**Options considered:**
1. Sweep every brief with a phase still to dispatch
2. Sweep only briefs with no Merge commit on main

**Hive recommendation:** Sweep every brief with a phase still to dispatch

**Keeper choice:** The phase C brief sweep covers every brief with a phase still to dispatch, not only briefs with no Merge commit: T-008, T-012, T-014, T-018, T-021, T-022, T-023 and T-025. Each gets the section 2 sweep (quoted decision text stays as quoted) and the Role names line. Decided by the Head Keeper on 2026-10-18; recorded by the queen.

**Rationale:** Remaining phases are dispatched from these briefs; leaving them on the old names would hand workers briefs whose role words mean the other role once the pin moves.

## D-143: waggle upgrade rewrote waggle-gate.yml from a stale template; the hive keeps main's copy and T-008 updates the template

- **Date:** 2026-10-18
- **Kind:** technical
- **Tickets:** T-008, T-016, T-020
- **Supersedes:** none

**Question:** waggle upgrade planned a rewrite of .github/workflows/waggle-gate.yml from a template that lacks the T-014 gate step and widens permissions. What happens to the hive copy and the template?

**Options considered:**
1. Keep main's copy in the hive; T-008 brings the template in line; manual check on every upgrade until then
2. Drop the waggle:managed line so upgrade skips the file
3. Accept the template render

**Hive recommendation:** Keep main's copy in the hive; T-008 brings the template in line; manual check on every upgrade until then

**Keeper choice:** The hive keeps main's waggle-gate.yml. T-008 brings the gate workflow template in internal/scaffold/templates in line with it: the waggle gate verify step built from BASE_SHA, contents: read at workflow level, write permissions only on relay-claim. Until T-008 merges, every waggle upgrade needs a manual revert check of that file. Decided by the Head Keeper on 2026-10-18; recorded by the queen.

**Rationale:** Applying the template would have removed the gate and given the verify job write permissions. Fixing the template keeps the file managed, so future upgrades converge instead of skipping it forever.

## D-144: hive-records-5 rides in the T-020 phase C PR

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-020
- **Supersedes:** none

**Question:** Does hive-records-5 (reviews, media, handoff and decisions D-138 to D-145) get its own hive PR or ride in the T-020 phase C PR?

**Options considered:**
1. Ride in the T-020 phase C PR
2. Separate hive PR after phase C merges

**Hive recommendation:** Ride in the T-020 phase C PR

**Keeper choice:** hive-records-5 rides in the T-020 phase C PR: the batch 5 records and decisions D-138 to D-145 merge with the rename, under the same guard reviews and Head Keeper bypass merge. Decided by the Head Keeper on 2026-10-18; recorded by the queen.

**Rationale:** Both are hive-only and merge by bypass; one PR saves a review and merge cycle, and the decisions describe this PR.

## D-145: Upgrade hive pin to 30639558288f93abdb378bf278a8b860e84a915e

- **Date:** 2026-10-18
- **Kind:** technical
- **Tickets:** T-016, T-020
- **Supersedes:** none

**Question:** Which Waggle build does this hive run?

**Options considered:**
1. Stay on b4f222166f4a824e34414e205544fb17397ccfa1
2. Move to 30639558288f93abdb378bf278a8b860e84a915e

**Hive recommendation:** Move to 30639558288f93abdb378bf278a8b860e84a915e

**Keeper choice:** Move the pin from b4f222166f4a824e34414e205544fb17397ccfa1 to 30639558288f93abdb378bf278a8b860e84a915e; managed files re-rendered by waggle upgrade from the target build, except .github/workflows/waggle-gate.yml, which keeps main's copy (D-143). Decided by the Head Keeper on 2026-10-18; recorded by the queen.

**Rationale:** 30639558288f93abdb378bf278a8b860e84a915e is the known-good build chosen by the Head Keeper (D-041).

## D-146: T-012 section 7.1 template commit written by the builder

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-012
- **Supersedes:** none

**Question:** Who writes the T-012 section 7.1 Head Keeper template commit on t-012b before the builder's phase B commits?

**Options considered:**
1. Builder, as a separate first commit
2. Head Keeper writes and pushes it first
3. Queen writes it in a scratch worktree

**Hive recommendation:** Builder, as a separate first commit

**Keeper choice:** The builder writes the section 7.1 commit as the first, separate commit on t-012b, titled 'Head Keeper: T-012 model levels in roster and config templates', touching only roster.json.tmpl and config.yaml.tmpl; the guards check it against section 7.1. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Zero Keeper time and no wait; the commit stays isolated and reviewable, and the queen keeps out of product code.

## D-147: Own honey switch stays after T-023 phase D

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** When does our hive switch from Claude Code subagents to waggle dispatch (own honey, D-044)?

**Options considered:**
1. After T-023 phase C, guards stay subagents until D
2. After T-023 phase D, as briefed
3. After T-023 phase E

**Hive recommendation:** After T-023 phase C, guards stay subagents until D

**Keeper choice:** The switch stays as T-023 section 10.4 briefs it: after phases A to D merge, builder and guard both through dispatch. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Rejected the early switch after phase C: it would run builders through dispatch without guard dispatch, the second-guard pick and the usage ledger.

## D-148: internal/dispatch/ added to WAGGLE.md before T-023 phase B

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Add internal/dispatch/ to WAGGLE.md section 2 now, or when T-023 phase B dispatches?

**Options considered:**
1. Add it now in hive-records-6
2. Add it with T-023 phase B

**Hive recommendation:** Add it now in hive-records-6

**Keeper choice:** Head Keeper commit adds internal/dispatch/ to the WAGGLE.md module list now, riding in hive-records-6. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Removes a T-023 phase B prerequisite from the critical path; one reversible line.

## D-149: T-022 phase B runs in parallel with T-023 phase B

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-022, T-023
- **Supersedes:** none

**Question:** Run T-022 phase B alongside T-023 phase B once T-021 phase B merges?

**Options considered:**
1. Run them together
2. Keep them serial

**Hive recommendation:** Run them together

**Keeper choice:** T-022 phase B and T-023 phase B dispatch together after T-021 phase B merges; their leases are disjoint. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Takes T-022 phase B off the T-023 phase D critical path for one extra concurrent builder and guard run.

## D-150: T-012 phase B merged; upgrade gap fixed by a new phase C

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-012, T-021
- **Supersedes:** none

**Question:** Merge T-012 phase B (#39) with waggle upgrade still materializing shims without a policy, and how to close that gap?

**Options considered:**
1. Merge #39; follow-up phase C (t-012c) makes upgrade policy-aware
2. Workaround: waggle init --force after upgrade
3. Fold the fix into T-021 phase B

**Hive recommendation:** Merge #39; follow-up phase C (t-012c) makes upgrade policy-aware

**Keeper choice:** #39 merged as is at 6a2401b. New T-012 phase C resolves the policy at upgrade plan time from the hive config plus the rendered roster, refuses before any write when levels is missing, and passes the policy to Materialize; section 7.2 reordered so the levels edit is committed before upgrade --apply. Phase C serialized before T-021 phase B. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Both guards passed phase B and found the same gap (T-012b-sonnet finding 1, T-012b-fable F12); a fix in #39 was outside the lease and past the 300-line stop.

## D-151: Symlinked CLAUDE.md stays refused by init and upgrade

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-012, T-016
- **Supersedes:** none

**Question:** Should waggle init and waggle upgrade refuse a symlinked CLAUDE.md, such as CLAUDE.md pointing to AGENTS.md?

**Options considered:**
1. Refuse for now; allow in-repo targets in a later ticket
2. Allow symlinks resolving to a regular file inside the repo now

**Hive recommendation:** Refuse for now; allow in-repo targets in a later ticket

**Keeper choice:** Both commands keep refusing symlinked destinations including CLAUDE.md (D-107 default). Open follow-up, recorded in T-012 section 9: allow a symlink that resolves to a regular file inside the repo, for both commands. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Safe default with no new review surface now; the common CLAUDE.md to AGENTS.md layout is blocked until the follow-up.

## D-152: T-012 phase C merged; queen runs the 7.2 hive follow-up

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-012
- **Supersedes:** none

**Question:** Merge T-012 phase C (#40), and who runs the T-012 section 7.2 hive follow-up?

**Options considered:**
1. Queen runs it in hive-records-6
2. Keeper runs it
3. Defer until T-021 phase B

**Hive recommendation:** Queen runs it in hive-records-6

**Keeper choice:** #40 merged as is at 6a4886e. The queen commits the levels block, builds waggle from 6a4886e outside the tree, runs upgrade then --apply on the hive-records-6 branch, restores main's waggle-gate.yml (D-143), and opens one hive PR for Head Keeper merge. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Both guards passed phase C and rehearsed 7.2 against our real hive files; one hive PR carries the pin move and the batch records.

## D-153: Upgrade hive pin to 6a4886eb16e0de1d005a4ba354e72ecd1dd63756

- **Date:** 2026-10-18
- **Kind:** technical
- **Tickets:** T-012, T-016
- **Supersedes:** none

**Question:** Which Waggle build does this hive run?

**Options considered:**
1. Stay on 30639558288f93abdb378bf278a8b860e84a915e
2. Move to 6a4886eb16e0de1d005a4ba354e72ecd1dd63756

**Hive recommendation:** Move to 6a4886eb16e0de1d005a4ba354e72ecd1dd63756

**Keeper choice:** Move the pin from 30639558288f93abdb378bf278a8b860e84a915e to 6a4886eb16e0de1d005a4ba354e72ecd1dd63756; roster.json re-rendered (min_level per role, mode removed) and shims regenerated with model lines (queen, scout, guard opus; builder sonnet; forager haiku) by waggle upgrade from the target build, except .github/workflows/waggle-gate.yml, which keeps main's copy (D-143). Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** 6a4886eb16e0de1d005a4ba354e72ecd1dd63756 is the known-good build chosen by the Head Keeper (D-041) and the first that writes model lines through upgrade (T-012 phase C).

## D-154: T-021 section 7.3 template commit written by the builder

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-021
- **Supersedes:** none

**Question:** Who writes the T-021 phase B Head Keeper commit editing config.yaml.tmpl and roles/queen.md?

**Options considered:**
1. Builder, as a separate first commit
2. Head Keeper writes and pushes it first

**Hive recommendation:** Builder, as a separate first commit

**Keeper choice:** The builder writes the section 7.3 commit as the first, separate commit on t-021b, titled 'Head Keeper: T-021 context profile template and queen handoff', touching only those two templates; the guards check it against section 7.3 and the Head Keeper approves the PR at merge. Same pattern as D-146. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** No wait on the Keeper; the commit stays isolated and reviewable, and the queen keeps out of product code.

## D-155: T-021 phase B dispatched before hive-records-6 merges

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-021
- **Supersedes:** none

**Question:** Dispatch T-021 phase B now, before the hive-records-6 PR (#41) merges and the session restarts on the new shims?

**Options considered:**
1. Dispatch now
2. After #41 and a session restart

**Hive recommendation:** Dispatch now

**Keeper choice:** T-021 phase B dispatches now; product and hive changes do not overlap. The builder runs on the session model (opus) because this session's shims predate the model lines. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Keeps the own honey critical path moving; the cost is one opus builder run instead of sonnet.

## D-156: T-023 phase B and T-022 phase B dispatched before hive-records-7

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-022, T-023
- **Supersedes:** none

**Question:** Dispatch T-023 phase B and T-022 phase B now, as approved at the end of the seventh session, or land hive-records-7 first?

**Options considered:**
1. Dispatch both now
2. Hive records 7 first
3. Both in parallel
4. Hold

**Hive recommendation:** Dispatch both now

**Keeper choice:** Both builders dispatch now on sonnet in isolated worktrees; T-023 phase B gets an opus guard plus a fable guard and Head Keeper merge (D-100), T-022 phase B one opus guard (Tier 2). Hive-records-7 follows. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Keeps the own honey critical path moving; the product leases do not overlap the hive records.

## D-157: T-021 phase B merged as is; two lease overreaches ratified; handoff-format and error-prefix notes go to T-023 phase F

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-021, T-023
- **Supersedes:** none

**Question:** PR #42 (T-021 phase B) added `internal/scaffold/context_ignore_test.go` and one constant in `init_test.go`, outside its lease. Merge as is, and where do the guards' handoff-format and error-prefix notes (fable N1, N2; sonnet 3) go?

**Options considered:**
1. Merge as is, ratify both overreaches, notes to T-023 phase F's queen template edit
2. Revert the overreaches in a fix pass before merge

**Hive recommendation:** Merge as is, ratify both overreaches, notes to T-023 phase F's queen template edit

**Keeper choice:** #42 merged as is at `8650124`; the new `context_ignore_test.go` and the one `init_test.go` constant are ratified. Fable N1 and N2 and sonnet note 3 go to T-023 phase F's queen template edit. Ruled by the Head Keeper at the end of the seventh session and confirmed via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Both overreaches are test-only and small; a fix pass would cost a full Tier 3 review cycle for no behavior change.

## D-158: Our hive keeps the committed queen handoff and the Stop hook until T-023 phase F

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-021, T-023
- **Supersedes:** none

**Question:** T-021 phase B's queen template writes handoffs to `.waggle/control/handoffs/`. Does our hive switch now or keep `.waggle/hive/handoff/queen.md` and the Stop hook?

**Options considered:**
1. Keep the committed handoff and Stop hook until T-023 phase F; a WAGGLE.md line tells the queen to ignore the template path
2. Switch to the template path at the next pin move

**Hive recommendation:** Keep the committed handoff and Stop hook until T-023 phase F; a WAGGLE.md line tells the queen to ignore the template path

**Keeper choice:** Our hive keeps `.waggle/hive/handoff/queen.md` and the Stop hook until T-023 phase F. A Head Keeper commit adds a Hive Overrides section to our `WAGGLE.md` telling the queen to ignore the template's `.waggle/control/handoffs/` path; it lands before the next pin move. Ruled by the Head Keeper at the end of the seventh session and confirmed via queen card on 2026-10-18; recorded by the queen.

**Rationale:** The handoff path and hook switch together with dispatch in phase F; switching earlier would break the Stop hook check.

## D-159: T-014 AC7 met by check-run API evidence instead of screenshots

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-014
- **Supersedes:** none

**Question:** Both scratch PRs (#44 out-of-lease edit, #45 `GO.MOD` edit) fail the Waggle gate with annotations. Is the saved check-run API output enough for AC7, or are screenshots required?

**Options considered:**
1. API evidence is enough
2. Keeper takes screenshots

**Hive recommendation:** API evidence is enough

**Keeper choice:** AC7 is met by the check-run and annotation JSON plus the summary in `.waggle/hive/media/T-014/AC7.md`. #44 proves the gate refuses an edit no lease covers; an edit outside an existing claim's paths is proven once claims exist on the remote (T-023 phase C). #44 and #45 closed unmerged and their branches deleted. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** The API output is the same data the Checks tab renders, is machine-checkable, and carries no personal data; screenshots add Keeper time and nothing else.

## D-160: The Head Keeper's DCO sign-off email is the GitHub noreply address

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-025
- **Supersedes:** none

**Question:** Which email signs off the Head Keeper's commits once DCO is enforced (T-025)? The author email must match, and it becomes public when the repo opens.

**Options considered:**
1. GitHub noreply (27632860+paulternate@users.noreply.github.com)
2. Company address
3. Keep the personal address

**Hive recommendation:** GitHub noreply (27632860+paulternate@users.noreply.github.com)

**Keeper choice:** The Head Keeper authors and signs off commits as `27632860+paulternate@users.noreply.github.com`, set in git config and backed by GitHub's email privacy setting, before T-025 phase D enforces DCO. Past commits are not rewritten by this decision; history redaction stays a separate decision before the repo goes public. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Keeps a personal address out of every future commit in a repo that will go public.

## D-161: T-023 phase B accepted at 867 added non-test lines against a 580 stop

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** T-023 phase B grew from 667 to 867 added non-test lines through three guard fix passes, against the brief's 580 stop. Accept, or cut?

**Options considered:**
1. Accept at 867 and record it
2. Leave the ruling in the reviews only

**Hive recommendation:** Accept at 867 and record it

**Keeper choice:** Phase B is accepted at 867 added non-test lines (D-089 metric), including the third fix pass for Windows build tags, relative gitdir resolution and the adminDir hardening. Every line past 667 is a guard-requested security fix (worktree reuse pinned to its registered gitdir, bounded lock reads, atomic stale-lock takeover); both guards found no cut worth the risk. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** The overrun is larger than earlier precedents (D-136), so it is recorded rather than left in the reviews; the phase C and D estimates should assume similar safety overhead.

## D-162: Q-T023-13: the D-137 default_args allowlist permits only --max-budget-usd and --debug

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Which default_args flags does the D-137 allowlist permit?

**Options considered:**
1. --max-budget-usd=<n> and --debug, each also present in the pinned --help
2. --max-budget-usd only
3. Empty allowlist

**Hive recommendation:** --max-budget-usd=<n> and --debug, each also present in the pinned --help

**Keeper choice:** The C0 allowlist permits `--max-budget-usd=<n>` and `--debug` only, and each must also appear in the pinned `--help`. `--effort` stays refused (D-134). Today config.yaml cannot express sequences, so the allowlist guards programmatic callers only. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Smallest set with a known use; anything else can be added by a later decision.

## D-163: Q-T023-14: every worker run adds --no-session-persistence and --disable-slash-commands

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Add --no-session-persistence and --disable-slash-commands to every worker argv?

**Options considered:**
1. Both
2. Only --disable-slash-commands
3. Neither

**Hive recommendation:** Both

**Keeper choice:** Phase C0 adds `--no-session-persistence` and `--disable-slash-commands` as fixed argv flags on every worker run. A failed session cannot be resumed from the Keeper's Claude Code history; `transcript.jsonl` keeps the full run. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Keeps worker sessions out of the Keeper's history and removes an unreviewed command surface inside the cell.

## D-164: Q-T023-15: T-023 phase C splits into C0 (allowlist, stop 130) then C (target 850, stop 1000)

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** How should phase C be split and sized?

**Options considered:**
1. C0 then C
2. C0, C, and a separate CLI phase
3. One phase C

**Hive recommendation:** C0 then C

**Keeper choice:** Phase C0 (`t-023c0`, lease `internal/runtime/launch.go`) lands the D-137 allowlist first: estimate 60, target 90, stop 130 added non-test lines. Phase C follows after C0 merges: estimate 700, target 850, stop 1000 (D-136 metric). Guard fix rounds count toward the stop, and a fix round that would cross the stop comes to the Keeper before it is written. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** D-161 showed guard fix rounds add about 30 percent; this sets realistic stops and makes C0 a reviewable precondition.

## D-165: Q-T023-16: a worker commit without a Signed-off-by trailer is a dispatch violation

- **Date:** 2026-10-18
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Is an unsigned worker commit (D-132) a violation or a warning?

**Options considered:**
1. Violation
2. Warning

**Hive recommendation:** Violation

**Keeper choice:** The phase C post-run check (rule 1) fails a run whose worker commits lack a `Signed-off-by` trailer, and the branch is not offered for review. Worker task lines say `git commit -s`. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Workers are told to sign; a missing trailer is a real miss that DCO enforcement (T-025) would reject later anyway.

## D-166: Q-T023-17: waggle dispatch builds on every platform and runs only on unix hosts

- **Date:** 2026-10-18
- **Kind:** technical
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Which platforms does waggle dispatch support?

**Options considered:**
1. Build everywhere, run on unix only
2. Unix only, no Windows build

**Hive recommendation:** Build everywhere, run on unix only

**Keeper choice:** Dispatch builds on every platform via `cell_unix.go` and `cell_other.go` build tags, and runs only on linux and macOS; elsewhere it exits 3 with a clear message. CI adds a Windows cross-compile check. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Keeps go build ./... green for every contributor without building a Windows cell.

## D-167: Q-T023-18: one dispatch run per repo, enforced by dispatch.lock

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Can two dispatch runs share one repo at the same time?

**Options considered:**
1. One run per repo via dispatch.lock
2. Design for concurrent runs now

**Hive recommendation:** One run per repo via dispatch.lock

**Keeper choice:** Phase C allows one dispatch run per repo at a time, enforced by `dispatch.lock`. Parallel builders serialize after phase F until a later decision relaxes this. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Concurrent runs fail each other's host-state check; per-run scoping would grow phase C well past its stop.

## D-168: Q-T023-19: the D-159 remote gate proof lands at the phase F trial, not phase C

- **Date:** 2026-10-18
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** When does the D-159 remote gate proof land?

**Options considered:**
1. At the phase F trial
2. In phase C with throwaway remote claims

**Hive recommendation:** At the phase F trial

**Keeper choice:** The remaining D-159 proof of `verify-leases-and-tiers` against real remote claims lands at the T-023 phase F trial, where our hive first has claims on the remote. Phase C proves the gate locally only. Decided by the Head Keeper via queen card on 2026-10-18; recorded by the queen.

**Rationale:** Avoids scratch claim refs on the remote; phase F produces the real evidence with no extra setup.

## D-169: T-023 phase C reviewed at 1186 added non-test lines; the stop moves from 1000 to 1350

- **Date:** 2026-10-19
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** T-023 phase C was built at 1186 added non-test lines against the 1000 stop (target 850); `run.go` came in at 490 against a 200 estimate. Guard fix rounds count toward the stop (D-164). Accept and review, cut first, or split?

**Options considered:**
1. Accept, new stop 1350
2. Cut to about 1100 (defer progress events and the settings.json and runtime_version copy to phase D), stop 1250
3. Split into C and C2 (CLI and keepalive in a second PR)

**Hive recommendation:** Accept, new stop 1350

**Keeper choice:** Phase C goes to the opus and fable guards at 1186 added non-test lines (D-136 metric). The stop for phase C, guard fix rounds included, is 1350; a fix round that would cross 1350 comes back to the Keeper before it is written. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Every line traces to a brief item, the offered cuts total about 83 lines and still miss 1000, and two of them are carry items. Splitting interdependent run, check and CLI code adds a full Tier 3 review cycle and delays the own honey switch. Phase D and E estimates should assume a similar overrun.

## D-170: Q-T023-20: phase D's runtime carry lands in a phase D0 PR leasing internal/runtime/ before phase D

- **Date:** 2026-10-19
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Phase D's runtime carry (10.6 items 16 to 21: second guard, guard settings, worker git config, Config() clone, help match, comments) needs internal/runtime/ beyond policy.go. Where does it land?

**Options considered:**
1. A phase D0 PR leasing internal/runtime/ (launch.go, claude.go, runtime.go, policy.go, tests), stop 130, merged before D
2. Widen phase D's lease to those files
3. A separate T-021 or T-012 follow-up ticket

**Hive recommendation:** A phase D0 PR leasing internal/runtime/ (launch.go, claude.go, runtime.go, policy.go, tests), stop 130, merged before D

**Keeper choice:** Phase D0 (`t-023d0`) leases `internal/runtime/` (`launch.go`, `claude.go`, `runtime.go`, `policy.go` and their tests) for 10.6 items 16 to 21, estimate 60, target 90, stop 130, Tier 3 with two guards, and merges before phase D dispatches. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Same pattern as C0 before C: a small, separately reviewable Tier 3 diff. A separate ticket would leave the second guard and the worker git config out of the phase F trial.

## D-171: Q-T023-21: workers run with GIT_CONFIG_GLOBAL=/dev/null and GIT_CONFIG_NOSYSTEM=1, identity passed as GIT_AUTHOR_* and GIT_COMMITTER_*

- **Date:** 2026-10-19
- **Kind:** credentials
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Should workers keep the Keeper's global and system git config, or run isolated from it with identity passed explicitly?

**Options considered:**
1. Isolate: GIT_CONFIG_GLOBAL=/dev/null, GIT_CONFIG_NOSYSTEM=1, identity from the repo's user.name and user.email, falling back to the Keeper's global
2. Keep the global config and list the bypass in not_enforced

**Hive recommendation:** Isolate: GIT_CONFIG_GLOBAL=/dev/null, GIT_CONFIG_NOSYSTEM=1, identity from the repo's user.name and user.email, falling back to the Keeper's global

**Keeper choice:** `workerEnv` sets `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1` and passes `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL`, `GIT_COMMITTER_NAME` and `GIT_COMMITTER_EMAIL` from the repo's `user.name` and `user.email` (D-160 noreply), else the Keeper's global, else exit 3 (10.6 item 19). Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Closes the C0 fable probe where a dotfile pushInsteadOf beat the tripwire and hides global credential helpers from workers. Workers lose the Keeper's global aliases, ignore file and signing config; accepted.

## D-172: Q-T023-22: end max_budget maps to itself in the usage ledger

- **Date:** 2026-10-19
- **Kind:** technical
- **Tickets:** T-023
- **Supersedes:** none

**Question:** A budget stop ends a run as `end: max_budget`. How does section 9.1 and the T-022 ledger record it?

**Options considered:**
1. Map it to itself, a limit stop like max_turns
2. Map it to error

**Hive recommendation:** Map it to itself, a limit stop like max_turns

**Keeper choice:** `max_budget` maps to itself in section 9.1 and in `usage.Append` records. No `internal/usage/` code change; the `record.go` comment is fixed in that package's next lease. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** T-022's reader neither validates nor counts End, and mapping to error would hide budget stops from waggle status.

## D-173: Q-T023-23: phase D re-estimated at 550, target 770, stop 960, built as one PR after D0

- **Date:** 2026-10-19
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Phase D's first estimate (230, stop 520) predates the phase C reviews. Bottom up it is about 550. Size and split?

**Options considered:**
1. D0 then D at 550 / 770 / 960, fix rounds counted
2. Three PRs: D0, D1 (about 380), D2 (about 170)
3. Move list to a follow-up ticket

**Hive recommendation:** D0 then D at 550 / 770 / 960, fix rounds counted

**Keeper choice:** Phase D is one PR after D0, estimate 550, target 770, stop 960 added non-test lines (D-136), guard fix rounds counted (D-164). If the build passes 770 before review, it comes back to the Keeper with the D1 and D2 cut of section 3; a fix round that would cross 960 comes back before it is written. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Built bottom up with phase C's 1.75x overrun on its estimate in mind; a third PR adds a Tier 3 cycle before phase F.

## D-174: Q-T023-24: a dispatched guard keeps read, Edit and Write in its worktree and the WAGGLE.md commands, but not git add or git commit

- **Date:** 2026-10-19
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** A dispatched guard today gets the builder's settings, including git add and git commit. What may it do?

**Options considered:**
1. Probes allowed, no git add or commit; a leftover probe fails rule 3 with the verdict recorded
2. Read-only guard
3. Builder settings unchanged

**Hive recommendation:** Probes allowed, no git add or commit; a leftover probe fails rule 3 with the verdict recorded

**Keeper choice:** `workerSettings` drops `git add` and `git commit` for the guard role; the guard keeps read tools, `Edit` and `Write` inside its worktree and the `WAGGLE.md` commands. A probe file left behind fails rule 3, exit 1, with the verdict still recorded (10.6 item 20). Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Hostile probes found phase C's worst defects, so a read-only guard would weaken reviews; removing commit rights keeps the guard from changing the branch.

## D-175: Q-T023-25: dispatch exits 3 before the claim when .waggle/worktrees/ or the usage ledger is not gitignored

- **Date:** 2026-10-19
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Our hive's .gitignore lacks the ledger and handoffs lines the scaffold already ships. What does dispatch do when the worktrees or ledger path is not ignored?

**Options considered:**
1. Exit 3 before the claim; our hive gets the lines from phase F's waggle upgrade
2. Warn and append anyway

**Hive recommendation:** Exit 3 before the claim; our hive gets the lines from phase F's waggle upgrade

**Keeper choice:** Before the claim, a pinned `git check-ignore -q` on `.waggle/worktrees/` and `layout.Usage`; either missing is exit 3 naming the `.gitignore` line, with nothing written (10.6 item 6). Our hive gets the lines through phase F's `waggle upgrade`. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Dispatch never leaves an untracked ledger in the main checkout that a hive records batch could commit.

## D-176: Q-T023-26: the phase D builder probes whether Bash tool children stay in the worker's process group on the pinned Claude Code

- **Date:** 2026-10-19
- **Kind:** technical
- **Tickets:** T-023
- **Supersedes:** none

**Question:** When do we verify that real Claude Code keeps Bash tool children in the worker's process group, so the timeout kill reaches them (opus N9)?

**Options considered:**
1. Phase D builder runs an AC7-style manual probe and saves the output; a failure blocks phase F, not phase D
2. Leave it to the phase F trial

**Hive recommendation:** Phase D builder runs an AC7-style manual probe and saves the output; a failure blocks phase F, not phase D

**Keeper choice:** The phase D builder runs a manual probe on the pinned Claude Code (a Bash tool sleep child, timeout kill, ESRCH) and saves the output under `media/T-023/`. A failure blocks phase F, not phase D's merge. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Finds a Cell-seam problem before the trial instead of costing the trial.

## D-177: Q-T023-27: dispatch counts must-fix verdicts across the phase's review files for the T-012 builder step-up

- **Date:** 2026-10-19
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** The T-012 step-up counts must-fix verdicts in reviews/T-<nnn>.md, which our hive never writes, so it never fires. Fix?

**Options considered:**
1. Count Verdict: must-fix lines across reviews/T-<nnn><phase>-*.md at base
2. Queen template appends every verdict to reviews/T-<nnn>.md
3. Leave it

**Hive recommendation:** Count Verdict: must-fix lines across reviews/T-<nnn><phase>-*.md at base

**Keeper choice:** Dispatch counts `Verdict: must-fix` lines across every `reviews/T-<nnn><suffix>-*.md` at base, where `<suffix>` is the branch suffix, so the step-up counts this phase's rounds; two Tier 3 guards both must-fix in one round count 2 (10.6 item 22). Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Makes the T-012 step-up work with our hive's review naming in about 10 lines; a process habit would be easy to skip.
