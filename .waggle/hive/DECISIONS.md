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

## D-178: T-023 item 23: a forager triages the linux TestRunHostState/new_admin_entry failure now, beside D0

- **Date:** 2026-10-19
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** TestRunHostState/new_admin_entry failed in 2 of the last 4 linux product-ci runs (plain, then symlinked), each time reporting a planted worktree admin entry as clean (exit 0, no violations): the post-run tamper check missing tampering. It is 10.6 item 23, scheduled for phase D. When?

**Options considered:**
1. Forager now, beside D0; a real check bug gets a small Tier 3 fix PR on internal/dispatch/ before phase D, a fixture artifact stays item 23
2. Leave it in phase D and rerun red CI until then
3. A blocking fix PR before D0 dispatches

**Hive recommendation:** Forager now, beside D0; a real check bug gets a small Tier 3 fix PR on internal/dispatch/ before phase D, a fixture artifact stays item 23

**Keeper choice:** A forager reproduces and triages item 23 on linux in parallel with the D0 build. If the production check can miss a planted admin entry, the fix lands as its own Tier 3 PR on internal/dispatch/ before phase D; if it is a test-fixture artifact, it stays phase D item 23. D0 is not held. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** A tamper check that passes a tampered host is a security gap, and the failure rate is too high to rerun past for long; triage costs one forager run and does not delay D0.

## D-179: A second guard with no unused model at or above its level takes the highest unused model below it

- **Date:** 2026-10-19
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Opus N7 on T-023 phase D0: a frontier builder (any step-up) plus a frontier first guard fill both frontier slots, so --second always fails, even with alternates.frontier set. This hits exactly the Tier 3 tasks that needed a step-up. How should the second guard resolve?

**Options considered:**
1. Allow a level down: the highest unused model below the guard's level, still cross-model
2. alternates.<level> becomes a list (config schema change)
3. Keep the rule; the queen runs the second guard by hand

**Hive recommendation:** Allow a level down: the highest unused model below the guard's level, still cross-model

**Keeper choice:** When the second guard (TaskContext.SecondGuard) finds no model at or above its level that differs from the builder's and the first guard's, it takes the highest-level unused model below that level (primary before alternate at each level); ErrNoCrossModel only when no unused model exists at any level. The first guard and every other role are unchanged. Lands in the T-023 phase D0 fix round, since policy.go is in the D0 lease and not phase D's. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Keeps the D-116 cross-model rule and makes --second work on stepped-up Tier 3 tasks with about 10 lines and no config schema change.

## D-180: The D-179 second-guard downward walk stops at the roster guard minimum

- **Date:** 2026-10-19
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** D-179

**Question:** Opus confirmation at ebce792: D-179 lets the second guard walk below the roster guard minimum with no bound, so on this hive's config (opus/sonnet/haiku, guard minimum standard, no alternates) an unstepped Tier 3 task gets sonnet builder, opus first guard and haiku second guard; before D-179 that case refused with a config hint. Bound the walk?

**Options considered:**
1. Bound the walk at the roster guard minimum
2. Keep D-179 unbounded

**Hive recommendation:** Bound the walk at the roster guard minimum

**Keeper choice:** The second guard's downward walk (D-179) goes no lower than the roster guard minimum; below it, ErrNoCrossModel with its config hint stands. D-179's stepped-up case still resolves (frontier builder, first guard on alternates.frontier, second guard at standard). Lands in a second T-023 phase D0 fix round with tests for primary-before-alternate, the avoid list inside the walk, and the walk's own exhaustion path (opus confirmation C3). Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Keeps every Tier 3 review at or above the guard floor the roster sets; the queen's D-179 card did not surface that the walk would also send unstepped Tier 3 second guards to the fast model.

## D-181: T-023 item 23 is git's detached auto maintenance pruning the planted entry; fix lands as phase D0b before phase D

- **Date:** 2026-10-19
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Forager round 3 found item 23's mechanism: the fixture's git commit starts git maintenance run --auto --detach (geometric strategy, default since git 2.54, includes worktree-prune); when it runs inside the window between the agent's mkdir and its gitdir write it deletes the planted admin entry, so the check correctly sees nothing. Deterministic repro: the proving test fails 16 of 16 without the fix and passes -race -count=50 on linux with it. Fix is about 3 lines: maintenance.auto=false and gc.auto=0 in the worker's GIT_CONFIG_* keys (internal/runtime/launch.go) and -c maintenance.auto=false on host-side git (internal/dispatch/worktree.go), optionally the comb ExtraEnv. Where?

**Options considered:**
1. A small Tier 3 PR now (phase D0b), two guards, before phase D
2. Fold into phase D as item 23 with a lease widening to launch.go

**Hive recommendation:** A small Tier 3 PR now (phase D0b), two guards, before phase D

**Keeper choice:** Phase D0b (t-023d0b) leases internal/runtime/launch.go, internal/dispatch/worktree.go and run.go (comb ExtraEnv only) and their tests, lands the round 3 fix and the proving test TestRunPlantDuringAutoMaintenance, Tier 3 with opus and fable guards, and merges before phase D dispatches. 10.6 item 23 is closed by D0b. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Not a check bug, but it reddens every PR's product-ci including phase D's, and a worker session should not spawn a detached git process that edits the common dir; three lines with a deterministic test.

## D-182: The queen handoff stays local and short; a summary is committed only at phase boundaries

- **Date:** 2026-10-19
- **Kind:** policy
- **Tickets:** none
- **Supersedes:** none

**Question:** The committed queen handoff is a running log reviewed through hive-records PRs; its in-flight paragraph was a must-fix in batches 8, 9 and 10 and it is rewritten after every merge for the Stop hook. Change it?

**Options considered:**
1. Keep it local and uncommitted, Stop hook still checks freshness, facts only, 40-line cap; commit a short summary at phase boundaries
2. Keep committing it in every records batch

**Hive recommendation:** Keep it local and uncommitted, Stop hook still checks freshness, facts only, 40-line cap; commit a short summary at phase boundaries

**Keeper choice:** The working queen handoff moves to an uncommitted local file (gitignored), still printed by the session-start hook and checked for freshness by the Stop hook; it is facts only with a 40-line cap. A short committed summary at .waggle/hive/handoff/queen.md is refreshed only at phase boundaries (a phase merge) and in records batches. The hook, gitignore and WAGGLE.md section 4 changes land in a hive control PR (Tier 3). Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Removes the recurring stale in-flight must-fix and the per-merge rewrite cost while keeping a fresh-session entry point.

## D-183: A scripted hive scan replaces the guard and card for hive-records batches that touch only .waggle/hive/**

- **Date:** 2026-10-19
- **Kind:** policy
- **Tickets:** none
- **Supersedes:** none

**Question:** Hive-records batches get a full opus guard and a Keeper card; most of the guard's value is a mechanical privacy scan. Change it?

**Options considered:**
1. A committed scan script (emails, home paths, hostnames, dashes, decisions gate); a clean batch touching only .waggle/hive/** merges without guard or card, with a one-line note to the Keeper
2. Keep the opus guard and card

**Hive recommendation:** A committed scan script (emails, home paths, hostnames, dashes, decisions gate); a clean batch touching only .waggle/hive/** merges without guard or card, with a one-line note to the Keeper

**Keeper choice:** A committed scan script checks every added line of a records batch for email addresses other than the D-160 noreply and known fixtures, the Keeper's name, home and temp paths, hostnames, token shapes and en or em dashes, and runs waggle gate decisions. A batch that touches only .waggle/hive/** and scans clean merges with --admin without a guard or card; the queen tells the Keeper in one line. Anything touching .waggle/control/ or failing the scan keeps the guard and card. The script lands in the hive control PR. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** The privacy scan is mechanical and a script does it more reliably; saves about an hour and a card per batch.

## D-184: Review depth scales with diff size: one confirming guard for small fix rounds

- **Date:** 2026-10-19
- **Kind:** policy
- **Tickets:** none
- **Supersedes:** none

**Question:** Every Tier 3 fix round gets two full confirmations even for a few lines. Scale it?

**Options considered:**
1. Two guards on every first review; a fix round under about 30 added lines gets one confirming guard (alternating models), and a second only if the first finds anything
2. Keep two confirmations per round

**Hive recommendation:** Two guards on every first review; a fix round under about 30 added lines gets one confirming guard (alternating models), and a second only if the first finds anything

**Keeper choice:** Every Tier 3 first review keeps two guards on different models. A fix round under 30 added lines (D-136 metric) gets one confirming guard, alternating between the two models round to round; if it finds anything, the other model confirms too. Larger fix rounds keep two confirmations. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** First reviews find the defects; small-round confirmations mostly pass, so one guard keeps the check at about half the cost.

## D-185: Scout estimates are doubled and passing the target splits along the planned cut without a card

- **Date:** 2026-10-19
- **Kind:** policy
- **Tickets:** none
- **Supersedes:** none

**Question:** Every phase ran 1.5x to 3.3x over its estimate, so stop cards are predictable rather than informative. Recalibrate?

**Options considered:**
1. Double scout bottom-up estimates; past the target, split along the planned cut without a card; only the stop comes to the Keeper
2. Keep current budgets and cards

**Hive recommendation:** Double scout bottom-up estimates; past the target, split along the planned cut without a card; only the stop comes to the Keeper

**Keeper choice:** From now on the scout doubles its bottom-up estimate when setting estimate, target and stop. A build that passes its target splits along the brief's planned cut without a card; only crossing the stop comes to the Keeper. For T-023 phase D the D-173 numbers stand and the D1/D2 cut of section 3 applies without a card if the build passes 770. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Turns a predictable overrun card into a planned split and keeps the stop as the real control.

## D-186: T-023 phase D fix rounds run without a Keeper card

- **Date:** 2026-10-19
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** May Tier 3 fix rounds on T-023 phase D run without a card when the guards confirm and the round stays under the stop?

**Options considered:**
1. Yes, no card per fix round; merges still come to the Keeper
2. No, a card per round

**Hive recommendation:** Yes, no card per fix round; merges still come to the Keeper

**Keeper choice:** Fix rounds on T-023 phase D (D1 and D2) run without a Keeper card while the guards confirm and the build stays under the stop; every merge still comes to the Keeper on a card. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Saves one wait per fix round, typically two or three in phase D.

## D-187: D-176 mitigation: dispatch tracks the worker's descendants live and kills them on stop and exit

- **Date:** 2026-10-20
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** The D-176 probe failed: Claude Code 2.1.304's Bash tool starts each command as a session leader, so neither a group kill nor a session sweep reaches it, and after the worker exits its children reparent to pid 1. A survivor can move refs, write the main checkout or plant hooks after the check while dispatch.lock is released. What is built before phase F?

**Options considered:**
1. Track descendants live: poll the tree about every 250 ms, kill every tracked (pid, start time) on stop and exit, hold dispatch.lock until the sweep, record survivors, PR_SET_CHILD_SUBREAPER on linux
2. Walk the tree only before the kill on the stop path
3. Defer to T-024 as NotEnforced and override D-176

**Hive recommendation:** Track descendants live: poll the tree about every 250 ms, kill every tracked (pid, start time) on stop and exit, hold dispatch.lock until the sweep, record survivors, PR_SET_CHILD_SUBREAPER on linux

**Keeper choice:** Dispatch polls the worker's descendant tree about every 250 ms while it runs and records each (pid, start time). On stop and after the leader exits it kills every tracked process still alive whose start time matches, repeats until none remain, and only then runs the post-run check and releases dispatch.lock; any process it had to kill is recorded in result.json. On linux dispatch also sets PR_SET_CHILD_SUBREAPER. A process that daemonizes between two polls stays a documented NotEnforced gap until T-024. Built in the T-023 phase D1 fix round inside internal/dispatch/; the D-176 probe is rerun with the pinned Claude Code as proof, and phase F stays blocked until it passes. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Closes the common leak (a background tool child outliving the worker) on both operating systems without waiting for the Cell runtime, for about 60 to 90 lines.

## D-188: --second checks the recorded builder and first-guard models and fails closed

- **Date:** 2026-10-20
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Re-resolving the second guard from policy can hand it the builder's own model when the must-fix count changes between guard runs. How is --second protected?

**Options considered:**
1. Read the builder's and first guard's result.json for the round; exit 3 if missing, if base_oid differs, or if the resolved model equals either; add must_fix to result.json
2. A queen rule only

**Hive recommendation:** Read the builder's and first guard's result.json for the round; exit 3 if missing, if base_oid differs, or if the resolved model equals either; add must_fix to result.json

**Keeper choice:** A --second guard run reads the latest builder run and first guard run records of the task, and exits 3 before launch when either is missing, when their base_oid differs from the second guard's base, or when the resolved second-guard model equals the model either recorded. result.json gains must_fix, the count used. Built in the T-023 phase D1 fix round; no internal/runtime change. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Keeps the D-100 and D-116 cross-model guarantee true by construction instead of by queen discipline, for about 20 lines.

## D-189: The cross-run host record ignores branch keys and is re-baselined by an acknowledge step

- **Date:** 2026-10-20
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Item 1's cross-run host record trips on routine edits: every git push -u adds a branch section to .git/config. The only recovery is deleting host-files.json. Change it?

**Options considered:**
1. Compare config by parsed keys ignoring branch.<name>.*; D2's dispatch clean --host-record shows the diff and re-baselines on confirm; store the verified pre-check snapshot
2. Keep strict

**Hive recommendation:** Compare config by parsed keys ignoring branch.<name>.*; D2's dispatch clean --host-record shows the diff and re-baselines on confirm; store the verified pre-check snapshot

**Keeper choice:** The host record compares the common config as parsed keys and ignores branch.<name>.* entries; every other difference is still exit 3. The record stores the snapshot the checks verified (fable B1), not a fresh hash after push and removal. Phase D2's dispatch clean --host-record prints the differences and re-baselines only after the Keeper confirms. The parse and snapshot land in the D1 fix round, the acknowledge step in D2. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Keeps item 1 meaningful without training the queen to delete the record after every PR.

## D-190: T-023 phase D1 accepted at 734 lines with a stop of 800; D2 gets its own budget

- **Date:** 2026-10-20
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** D-173

**Question:** T-023 phase D1 reached 734 added non-test lines against its 665 stop after the D1 fix round built D-187 (about 190 lines against a 60 to 90 estimate), D-188 and D-189; the builder crossed the stop instead of pausing. The D-176 probe now passes. Accept, or split D-187 into its own PR?

**Options considered:**
1. Accept 734, D1 stop 800 including confirmation fix rounds; D2 own budget estimate 200, target 280, stop 350
2. Split D-187 into a separate PR

**Hive recommendation:** Accept 734, D1 stop 800 including confirmation fix rounds; D2 own budget estimate 200, target 280, stop 350

**Keeper choice:** T-023 phase D1 is accepted at 734 added non-test lines (D-136) and its stop moves to 800 including the confirmation fix rounds. Phase D2 (clean, list, item 13, the D-189 host-record acknowledge, stale duplicate worktree entries) gets its own budget: estimate 200, target 280, stop 350. D-173's combined phase D stop of 960 no longer applies. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** The overage came from Keeper-ruled safety work (live descendant tracking) that has to land before phase F; splitting it would add a review cycle without reducing risk.

## D-191: On macOS the D-187 tracker polls by sysctl every 25 ms; a detached background child stays a NotEnforced gap until T-024

- **Date:** 2026-10-20
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** D-187

**Question:** Fable showed that on macOS a tool command that forks a detached background child escapes D-187's 250 ms ps polling almost every time (0 of 15; the real Claude Code run left the child alive), and its ref and main-checkout writes after the check are never named. Linux's subreaper catches every case. The queen's D-187 card had said this closes the common leak. How to proceed toward phase F?

**Options considered:**
1. Read the process table by sysctl (about 1 ms) and poll every 25 ms, record the macOS gap as NotEnforced, trial phase F on macOS, T-024 closes it
2. Run phase F workers in a Linux VM or container
3. Accept the 250 ms ps poll and document the gap

**Hive recommendation:** Read the process table by sysctl (about 1 ms) and poll every 25 ms, record the macOS gap as NotEnforced, trial phase F on macOS, T-024 closes it

**Keeper choice:** On darwin the descendant tracker reads the process table through sysctl (kern.proc) instead of /bin/ps and polls every 25 ms; linux keeps the subreaper. A process that daemonizes between two polls on macOS is a documented NotEnforced gap (an escaped child's hook and config writes are still named by the next dispatch's host record; its ref and main-checkout writes are not) until T-024's Cell closes it. The phase F trial runs on macOS with this gap stated in its report. Lands in the T-023 phase D1 second fix round. Amends D-187. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Narrows the macOS window about tenfold for about 30 lines without delaying phase F; only code a worker runs (go test and similar) can reach the gap, and the Cell runtime is the real fix.

## D-192: The host record ignores only routine branch keys; pushRemote, path or URL values and global branch settings stay strict

- **Date:** 2026-10-20
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** D-189

**Question:** D-189 ignores every branch.* key. A planted branch.<name>.pushRemote, or a branch remote that names a path, silently redirects the Keeper's own argument-free git push or pull in the main checkout, unflagged. Tighten?

**Options considered:**
1. Ignore only branch.<name>.merge and branch.<name>.remote values that name a configured remote; keep pushRemote, URL or path values and global branch.* settings strict
2. Keep D-189 and name the gap in not_enforced

**Hive recommendation:** Ignore only branch.<name>.merge and branch.<name>.remote values that name a configured remote; keep pushRemote, URL or path values and global branch.* settings strict

**Keeper choice:** The cross-run host record ignores branch.<name>.merge and branch.<name>.remote when the value names a remote configured in the same file; every other branch key (pushRemote, a remote given as a URL or path, rebase or other per-branch settings) and every global branch.* setting is compared strictly. Lands in the T-023 phase D1 second fix round. Amends D-189. Decided by the Head Keeper via queen card on 2026-10-19; recorded by the queen.

**Rationale:** Keeps git push -u from tripping the record while still catching a redirect of the Keeper's own push or pull.

## D-193: Tests and probes that send real kill signals run only in the linux container

- **Date:** 2026-10-20
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** An uncommitted guard probe (newTracker(0), poll, a forced procTable failure, sweep) SIGKILLed every process the Keeper owns on the Mac twice on 2026-10-20. Should a standing hive rule keep kill-sending tests off the Keeper's host?

**Options considered:**
1. Linux container only: any test or probe that calls sweep() or signals a process it did not spawn runs only in the Colima linux container; prefer pure data assertions; recorded and put in builder and guard briefs
2. Allow on the host again after the D1 hardening lands and both guards pass it
3. Put the rule in this round's briefs only, no standing decision

**Hive recommendation:** Linux container only: any test or probe that calls sweep() or signals a process it did not spawn runs only in the Colima linux container; prefer pure data assertions; recorded and put in builder and guard briefs

**Keeper choice:** Any test, probe or fix-removal check that calls the tracker's sweep(), or sends a signal to a process the test did not itself spawn, runs only inside the Colima linux container, never on the Keeper's darwin host. Builders and guards assert tracker and sweep behavior on data (fabricated process tables, returned target lists) wherever they can. Every builder and guard brief that touches internal/dispatch process handling states this rule. The darwin catch-rate measurements owed by D-191 are taken without sweep(). Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** A probe with a bad root took down the Keeper's whole session twice; a container's pid namespace bounds the damage of the next such bug at small cost in speed.

## D-194: macOS tracker stays at 25 ms with honest NotEnforced wording in D1; group-leader sweep lands in D2

- **Date:** 2026-10-20
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Builder and both guards measured the darwin sysctl tracker: a background child whose parent exits within milliseconds (the usual cmd & from a tool shell) is caught 0 of 65 (builder) and 7 of 20 (fable) at 25 ms; shorter intervals cost CPU and close little, since the parent's lifetime after the fork decides the catch. What follows D-191?

**Options considered:**
1. Reword NotEnforced in D1, keep 25 ms; add a group-leader sweep in D2
2. Reword in D1, defer the fix to T-024
3. Drop the darwin tracker

**Hive recommendation:** Reword NotEnforced in D1, keep 25 ms; add a group-leader sweep in D2

**Keeper choice:** The darwin tracker keeps its 25 ms sysctl poll. In the T-023 D1 fix round the NotEnforced text says plainly that on macOS a background child whose parent exits before the next 25 ms read, which is the usual cmd & from a tool shell, escapes until T-024; the phase F trial report carries the same sentence. T-023 phase D2 adds a group-leader sweep: at cleanup, also kill the process group of every group leader the tracker saw, guarded so the group id is above 1, is not dispatch's own group, and its leader pid is absent or still has the recorded start time; tested on data per D-193. Amends D-191. Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Honest wording costs no lines and must not wait; the group sweep closes most of the cmd & case for about 6 lines without waiting for the Cell runtime.

## D-195: D-193 applies to ad hoc probes and fix-removal checks; committed tests that reach sweep() only through their own spawned run stay on unix

- **Date:** 2026-10-20
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Q-T023-30: D-193 bans any test that calls sweep() from the Mac, but the committed suite already reaches sweep() on darwin (two direct callers, and every Run test through runSession). How far does D-193 reach into the committed suite?

**Options considered:**
1. Narrow: direct sweep() callers become linux-only; Run tests stay on unix; no test kills a pid read from a file; probes and fix-removal checks stay container-only
2. Strict: every sweep-reaching test becomes linux-only

**Hive recommendation:** Narrow: direct sweep() callers become linux-only; Run tests stay on unix; no test kills a pid read from a file; probes and fix-removal checks stay container-only

**Keeper choice:** In T-023 phase D2 the committed tests that call sweep() directly (TestSweepKillsTrackedGroup, TestSweepSkipsReusedPid and any new one) move to linux-only files, and no committed test kills a pid it read from a file. Run tests that reach sweep() only through a run they spawned stay on unix, as the only darwin coverage of the sysctl tracker, so `go test ./...` on the Keeper's Mac is allowed. Every ad hoc probe, uncommitted test and fix-removal check that calls sweep() or signals a process it did not spawn stays container-only as D-193 says. Amends D-193 (Q-T023-30). Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Keeps the only darwin tracker coverage while removing the direct sweep() calls that resemble the incident probe.

## D-196: waggle dispatch list moves from T-023 phase D2 to phase E

- **Date:** 2026-10-20
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Q-T023-28: D2 is estimated at 446 doubled lines with list and 336 without, against a 350 stop (D-190). Keep list in D2?

**Options considered:**
1. Move list to phase E and add cmd/waggle/dispatch.go to phase E's lease
2. Keep list in D2 and raise the stop to 450

**Hive recommendation:** Move list to phase E and add cmd/waggle/dispatch.go to phase E's lease

**Keeper choice:** `waggle dispatch list` leaves T-023 phase D2 and lands in phase E, whose lease gains cmd/waggle/dispatch.go. D2 keeps the D-190 budget of 200 / 280 / 350; if the build passes 280 before review, the host-record acknowledge splits into a D2b PR on the same lease under D-185 (Q-T023-28). Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Phase F does not need list, and the cut keeps D2 inside its stop without raising a budget.

## D-197: Workers start with Setsid; the D-176 proof moves to the first phase F macOS trial run

- **Date:** 2026-10-20
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Q-T023-29: D2 starts workers with Setsid instead of Setpgid. D-193 rules out rerunning the real Claude Code process-group probe on the Mac. How is the D-176 behavior proved?

**Options considered:**
1. Setsid plus narrowed group targets; the first phase F macOS trial run repeats probe 2's shape and must list the tool shell and child in survivors_killed or cutover is blocked
2. Keep Setpgid
3. Rerun the real probe in the linux container with an API key

**Hive recommendation:** Setsid plus narrowed group targets; the first phase F macOS trial run repeats probe 2's shape and must list the tool shell and child in survivors_killed or cutover is blocked

**Keeper choice:** In T-023 phase D2 the worker starts in its own session (Setsid) and the sweep signals only groups the tracker saw led by a tracked process (D-194 guards). There is no D2 rerun of the real Claude Code probe. The first phase F macOS trial run repeats the D-176 probe 2 shape and must list the tool shell and its background child in survivors_killed; if it does not, the phase F cutover is blocked (Q-T023-29). Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Closes the shared-session group gap (D1 guard N15) at no new credential or cost, and the trial is the first real run anyway.

## D-198: dispatch clean --host-record uses a two-step digest; the queen accepts only after a Keeper card

- **Date:** 2026-10-20
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Q-T023-31: who accepts a changed host record with dispatch clean --host-record?

**Options considered:**
1. Two-step digest: --host-record lists differing keys and a digest; --accept <digest> re-baselines only if nothing changed; the queen runs --accept only after a Keeper card
2. Interactive y/N prompt

**Hive recommendation:** Two-step digest: --host-record lists differing keys and a digest; --accept <digest> re-baselines only if nothing changed; the queen runs --accept only after a Keeper card

**Keeper choice:** `waggle dispatch clean --host-record` prints the differing keys and a digest of the new record and changes nothing; `--accept <digest>` re-baselines only when the current record still hashes to that digest. The queen runs --accept only after the Keeper approves the listed keys on a card (Q-T023-31). Implements the acknowledge step of D-189 and D-192. Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Works from the queen's non-interactive tool shell and cannot accept a record that changed after the Keeper saw it.

## D-199: The macOS NotEnforced clause states the measured group-sweep limit; the phase F trial measures a real tool shell

- **Date:** 2026-10-20
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** With the D2 group-leader sweep, a child left in its tool shell's group is killed only if some 25 ms read listed the shell alive (fable: 0 of 20 with a shell gone within 5 ms, 20 of 20 with one alive 20 ms). The builder's text claimed the group sweep always catches it; D-194's sentence names an outdated cause. Which sentence ships?

**Options considered:**
1. Fable's measured text
2. Opus's shorter text
3. Keep the D-194 sentence

**Hive recommendation:** Fable's measured text

**Keeper choice:** The macOS clause of NotEnforced in internal/dispatch/cell.go reads: "on macOS (sysctl read every 25 ms) a background child escapes until T-024 when its tool shell started and exited between two reads, which is the usual cmd & from a short-lived shell (0 of 20 caught with a shell gone within 5 ms, 20 of 20 with one alive 20 ms), or when it calls setsid or daemonizes before the next read; a child that stays in the group of a tool shell the tracker listed alive is killed by the group sweep"; the rest of the line is unchanged. The phase F trial report carries the same sentence, and the phase F macOS trial adds a `cmd >/dev/null 2>&1 &` tool command next to the D-197 probe 2 shape to measure whether a real Claude Code tool shell lives long enough to be listed. Lands in the T-023 phase D2 fix round. Amends D-194. Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Both D2 guards found the builder's sentence overstated the catch; the measured text is honest and gives the trial a concrete number to check.

## D-200: dispatch clean asks git's prune dry run for duplicate worktree entries instead of reimplementing git's rules

- **Date:** 2026-10-20
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Four D2 guard rounds each found a planted admin entry that waggle's own duplicate comparison missed while git's prune still treated it as a duplicate and silently dropped another ticket's registration (symlinked gitdir, case, a .GIT suffix, a relative gitdir through a symlink). How should clean detect duplicates?

**Options considered:**
1. Run git worktree prune --dry-run -v first and refuse on any duplicate entry
2. Patch the relative-gitdir case and keep waggle's comparison
3. Move the precise check to D2b and refuse any foreign admin entry for now

**Hive recommendation:** Run git worktree prune --dry-run -v first and refuse on any duplicate entry

**Keeper choice:** Before it removes anything, `waggle dispatch clean` runs the pinned `git worktree prune --dry-run -v` and exits 3 with nothing removed if git reports any duplicate entry, naming the entries. Waggle's own path comparison for duplicates is removed; the refusals of a symlinked admin entry and of a symlinked, unreadable or oversized gitdir stay. If the change would take T-023 phase D2 past its 350-line stop (D-190), the duplicate check moves to D2b instead. Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Git's own resolution matches git's own prune by construction, which ends the series of spelling bypasses and should cost fewer lines than the comparison it replaces.

## D-201: dispatch clean re-runs git's duplicate dry run immediately before its real prune; guarding provision's prune moves to D2b

- **Date:** 2026-10-21
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Fable B7: clean's dry run happens before `worktree remove --force` deletes the cleaned tree, and a relative gitdir through a symlink in that tree resolves differently afterwards, so the real prune drops another ticket's entry. Provision runs an unguarded repo-wide prune too. How does D2 close B7?

**Options considered:**
1. Re-run the dry-run check right before the real prune; D2b guards provision's prune
2. Drop clean's repo-wide prune
3. Move clean to D2b

**Hive recommendation:** Re-run the dry-run check right before the real prune; D2b guards provision's prune

**Keeper choice:** `waggle dispatch clean` keeps its first duplicate dry run and runs the same check again immediately before its `git worktree prune`, so git judges the state it is about to act on; a refusal at the second check says that the ticket's own worktree was already removed and nothing else was pruned. The fix must stay within the D-190 stop of 350 added non-test lines for T-023 phase D2, or the builder stops. Guarding the repo-wide prune in Worktree.provision with the same check is a T-023 phase D2b item. Amends D-200. Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Checks the exact state git will prune at the cost of 3 lines and keeps clean in D2 for phase F.

## D-202: dispatch clean removes only its own ticket's worktree entries and never runs a repo-wide prune

- **Date:** 2026-10-21
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** D-200, D-201

**Question:** Five D2 guard rounds each found a planted admin entry that steers clean's repo-wide git worktree prune into dropping another ticket's registration (B2, B4, B5, B6, B7). Planting one needs write access to the git dir, which already lets the attacker break any worktree directly. Change course?

**Options considered:**
1. Stop round 5; clean removes only its own ticket's entries and never runs a repo-wide prune; add a threat bar to every brief; adopt a two-round stop rule
2. Finish the D-201 re-check first
3. Keep patching

**Hive recommendation:** Stop round 5; clean removes only its own ticket's entries and never runs a repo-wide prune; add a threat bar to every brief; adopt a two-round stop rule

**Keeper choice:** `waggle dispatch clean` removes only the entries of the ticket and role it was given: `git worktree remove --force` for a worktree that exists, and that entry's own admin directory under the common git dir when its worktree is already gone. It never runs `git worktree prune` or any other repo-wide prune, so other stale or planted entries are left for the Keeper or git. The duplicate dry run and Git.CLocale added for D-200 are removed. Keeps the refusals of a symlinked admin entry and of a symlinked, unreadable or oversized gitdir for the entries it acts on. In T-023 phase D2b the repo-wide prune in Worktree.provision gets the same treatment. Supersedes D-200 and D-201. Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Removes the whole class of prune steering instead of patching each spelling, and frees about 25 lines of D2's budget.

## D-203: Every brief carries a threat bar: an attack that needs a capability already able to do the same damage is a note, not a must-fix

- **Date:** 2026-10-21
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Five D2 guard rounds each found a planted admin entry that steers clean's repo-wide git worktree prune into dropping another ticket's registration (B2, B4, B5, B6, B7). Planting one needs write access to the git dir, which already lets the attacker break any worktree directly. Change course?

**Options considered:**
1. Stop round 5; clean removes only its own ticket's entries and never runs a repo-wide prune; add a threat bar to every brief; adopt a two-round stop rule
2. Finish the D-201 re-check first
3. Keep patching

**Hive recommendation:** Stop round 5; clean removes only its own ticket's entries and never runs a repo-wide prune; add a threat bar to every brief; adopt a two-round stop rule

**Keeper choice:** Every scout brief states its threat model, and every guard prompt repeats it: a finding whose attacker needs a capability that already lets them cause the same damage directly (for example write access to the git dir or to another ticket's worktree, or an escaped worker process) is recorded as a note, not a must-fix, unless the Keeper raises it. Must-fix stays for anything a worker inside its lease, a reviewer, or ordinary repo state can trigger, and for any path that signals or deletes outside the worker's own tree from ordinary inputs. Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Without a bar, guards rank every silent failure as blocking, and fix rounds chase attacks that add no capability.

## D-204: After two must-fix rounds on the same function, the queen brings a design card instead of a third patch round

- **Date:** 2026-10-21
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Five D2 guard rounds each found a planted admin entry that steers clean's repo-wide git worktree prune into dropping another ticket's registration (B2, B4, B5, B6, B7). Planting one needs write access to the git dir, which already lets the attacker break any worktree directly. Change course?

**Options considered:**
1. Stop round 5; clean removes only its own ticket's entries and never runs a repo-wide prune; add a threat bar to every brief; adopt a two-round stop rule
2. Finish the D-201 re-check first
3. Keep patching

**Hive recommendation:** Stop round 5; clean removes only its own ticket's entries and never runs a repo-wide prune; add a threat bar to every brief; adopt a two-round stop rule

**Keeper choice:** When guards return must-fix findings on the same function or check in two consecutive fix rounds, the queen stops patching and brings the Keeper a design card (keep patching, change the design, or move the item to a later PR) before any third round. Amends D-186 for that case. Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Caps the cost of a review loop that keeps finding variants of one weakness.

## D-205: Fix rounds get two confirming guards only when they touch signals, deletes, leases or claims, or exceed 100 added non-test lines

- **Date:** 2026-10-21
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Review turnaround is the bottleneck: small fix rounds, some almost all test lines, have been confirmed by both guards. Narrow D-184?

**Options considered:**
1. Two confirming guards only for fix rounds that touch signals, deletes, leases or claims or exceed 100 added non-test lines; one alternating guard otherwise
2. Keep D-184 as is

**Hive recommendation:** Two confirming guards only for fix rounds that touch signals, deletes, leases or claims or exceed 100 added non-test lines; one alternating guard otherwise

**Keeper choice:** Every Tier 3 first review keeps two guards on different models. A fix round gets one confirming guard, alternating between the two models round to round, unless its diff changes code that sends signals, deletes files or git state, or touches leases or claims, or it adds more than 100 non-test lines (D-136 metric); those get two confirmations. If a single confirming guard finds a must-fix, the other model confirms the next round. Amends D-184. Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Keeps two models on the dangerous paths and saves a guard pass on test-only and wording rounds.

## D-206: T-023 phase D2b budget is 140 / 190 / 240 added non-test lines

- **Date:** 2026-10-21
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Q-T023-32: what line budget does T-023 phase D2b get? The raw estimate is 69 lines; D2 ran 2.07x its estimate and the worst observed overrun is 3.3x (about 228).

**Options considered:**
1. Estimate 140, target 190, stop 240
2. 100 / 140 / 180
3. 180 / 250 / 320

**Hive recommendation:** Estimate 140, target 190, stop 240

**Keeper choice:** T-023 phase D2b has an estimate of 140, a target of 190 and a stop of 240 added non-test lines (D-136 metric), fix rounds counted. If the build passes 190 before review, items 4a (SIGHUP) and 4b (pinned worktree add) move to phase E (section 10.8.2) (Q-T023-32). Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Covers the worst observed overrun on the 69-line raw estimate without leaving slack for fix-round surface.

## D-207: Nothing replaces provision's repo-wide prune; git's worktree add refusal surfaces with a clean hint

- **Date:** 2026-10-21
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Q-T023-33: D-202 removes the repo-wide git worktree prune in Worktree.provision. What replaces it?

**Options considered:**
1. Nothing: git's worktree add refusal becomes exit 3 with a waggle dispatch clean hint
2. Provision pre-checks with entryFor and refuses with the hint
3. Provision drops its own stale entry before worktree add

**Hive recommendation:** Nothing: git's worktree add refusal becomes exit 3 with a waggle dispatch clean hint

**Keeper choice:** In T-023 phase D2b, Worktree.provision loses its repo-wide git worktree prune and gains no replacement: when git worktree add refuses a stale entry, dispatch exits 3 with a hint to run waggle dispatch clean. A Keeper who deletes a worktree directory by hand runs clean once before the next dispatch (Q-T023-33). Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** About 3 lines and no new delete path in every dispatch; one bad entry elsewhere does not block dispatch.

## D-208: The remaining T-023 D1 notes carry to phase E

- **Date:** 2026-10-21
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Q-T023-34: where do D1 opus N3 (identity scope, prerun.go), opus N7 (skip-worktree edits invisible to rule 3, check.go) and fable D1 N4 (verdict on the last non-blank line, guard.go) land? About 20 doubled lines; none blocks phase F.

**Options considered:**
1. Phase E carry, with phase E's lease widened to those three functions
2. Add to D2b and raise its numbers by 20
3. A follow-up ticket after phase F

**Hive recommendation:** Phase E carry, with phase E's lease widened to those three functions

**Keeper choice:** D1 opus N3 and N7 and fable D1 N4 carry to T-023 phase E, whose lease widens to the three named functions in prerun.go, check.go and guard.go (Q-T023-34). Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Low risk and keeps D2b small ahead of phase F.

## D-209: T-023 phase D2b design defaults: stricter host-record digest, pinned provision worktree add, host-only tests

- **Date:** 2026-10-21
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** The D2b scout brief makes three design choices without a Keeper question: the host-record digest also covers the stored record's file state; provision's git worktree add is pinned; every D2b test and fix-removal check is git and file only. Accept them?

**Options considered:**
1. Accept all three
2. Discuss before the build

**Hive recommendation:** Accept all three

**Keeper choice:** T-023 phase D2b follows section 10.8 as written: the --accept digest covers the stored host record's file state as well as the current map, so a record rewritten after the listing is refused (stricter than D-198); provision's git worktree add calls run through the pinned git, so a git-lfs repo's worker worktree holds pointer files; every D2b test and fix-removal check is git and file only and avoids Run, so it runs on the host under D-193 and D-195, and anything needing Run, a signal or the tracker runs only on docker --context colima-waggle. Decided by the Head Keeper via queen card on 2026-10-20; recorded by the queen.

**Rationale:** Each choice is the safer reading and can change in a later lease.

## D-210: T-023 D2b: the 190 cut applies only before review; post-review rounds count toward the 240 stop

- **Date:** 2026-10-21
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** D2b passed both guards at 184 added non-test lines against a 190 target. A polish round for opus N1, N3 and N4 may cross 190. Do items 4a and 4b stay in?

**Options considered:**
1. Stay in: the D-206 cut applies only past 190 before review; post-review rounds count toward the 240 stop
2. Cut at 190 at any time

**Hive recommendation:** Stay in: the D-206 cut applies only past 190 before review; post-review rounds count toward the 240 stop

**Keeper choice:** The D-206 cut of items 4a and 4b applies only when the build passes 190 before review. Fix and polish rounds after review count toward the 240 stop and do not move 4a or 4b. T-023 phase D2b gets one polish round before merge for opus N1 (printable also drops U+2028 and U+2029), N3 (the D-207 hint says not to run git worktree prune) and N4 (provision tells the Keeper to remove a plain directory by hand), confirmed by one guard (D-205). Decided by the Head Keeper via queen card on 2026-10-21; recorded by the queen.

**Rationale:** Keeps the reviewed scope intact and fixes guidance phase F would show the Keeper.

## D-211: Template keeps --no-cell with no first-dispatch card for v0.1

- **Date:** 2026-10-21
- **Kind:** scope
- **Tickets:** T-023, T-024
- **Supersedes:** none

**Question:** Should the queen template require a Keeper card before a hive's first dispatch, given every worker runs with --no-cell until T-024?

**Options considered:**
1. No change for v0.1
2. Add a first-dispatch card

**Hive recommendation:** No change for v0.1

**Keeper choice:** No change for v0.1; revisit when T-024 ships the cell.

**Rationale:** --no-cell is mandatory until T-024 and the refusal text already lists what is not enforced; a card per hive adds an ask without adding enforcement.

## D-212: Rename the upgrade error prefix in a later product follow-up

- **Date:** 2026-10-21
- **Kind:** scope
- **Tickets:** T-012, T-021, T-023
- **Supersedes:** none

**Question:** Should the 'model policy for claude-code shims:' error prefix be renamed in code (fable N1, D-157), so the queen template's escalation rule 4 can drop its second sentence?

**Options considered:**
1. Yes, later follow-up
2. No, keep the template note

**Hive recommendation:** Yes, later follow-up

**Keeper choice:** Yes: a small T-012 or T-021 product follow-up renames the prefix and removes the template sentence in the same commit; not part of phase F.

**Rationale:** The template note goes stale once code changes; fixing the wording at the source is a few lines and keeps phase F small.

## D-213: Phase F trial records dispatch behavior when the session closes

- **Date:** 2026-10-21
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Should the phase F trial verify what a background waggle dispatch does when the Claude Code session closes mid-run?

**Options considered:**
1. Trial records it
2. Not needed

**Hive recommendation:** Trial records it

**Keeper choice:** Yes: the trial closes a session during one run and saves what result.json shows in .waggle/hive/media/T-023/trial/; the template wording stays.

**Rationale:** Nobody has confirmed which signal the host sends background shells on exit; about ten minutes of trial time settles it before cutover.

## D-214: Phase F trial proves the D-197 sweep with an outliving child

- **Date:** 2026-10-22
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** With the real 10 s killGrace, Claude Code kills its own tool shell and child on SIGTERM, so the probe 2 shape (which listed survivors only with a forced 1 ns grace) likely ends with survivors_killed empty. How should the phase F trial prove the sweep before cutover?

**Options considered:**
1. Outliving-child shape
2. Keep D-197 literal (needs a product change to shorten killGrace)
3. Both shapes, either passes

**Hive recommendation:** Outliving-child shape

**Keeper choice:** Outliving-child shape: the trial adds a probe child that outlives the worker's normal exit so the sweep fires on the product path. Cutover gate: every listable leftover pid appears in survivors_killed and is dead in the after capture; any live pid blocks cutover. A SIGTERM self-cleanup by Claude Code is recorded as not evidence and not a block; whether the SIGKILL path was reached is recorded, not required.

**Rationale:** Clarifies the D-197 trial gate only; the rest of D-197 stands. The literal D-197 shape cannot reach the sweep on the real timeout path without a product change; the outliving-child shape tests the same guarantee (no worker leftovers survive dispatch) through shipped code. Raised by the T-023 phase F step 3 fable guard (must-fix 1).

## D-215: Phase F step 3 design defaults accepted

- **Date:** 2026-10-22
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Accept the scout's and guards' defaults for the T-023 phase F step 3 hive PR (#65)?

**Options considered:**
1. Accept all
2. Accept, but discuss one

**Hive recommendation:** Accept all

**Keeper choice:** Accept all: rules the upgrade erased from roles/queen.md and roles/scout.md live in WAGGLE.md section 4; queen.local.md and its Stop check stay until the cutover card, then retire; the trial probes run in a local-only scratch hive and run 1 is the first remote-mode run; the trial guard is opus by policy; the pin-move decide entry goes in the next records batch.

**Rationale:** waggle upgrade rewrites role files from templates, so hive rules there do not survive; keeping the local handoff until cutover keeps the queen's session working through the trial.

## D-216: D-210 becomes a standing hive rule

- **Date:** 2026-10-22
- **Kind:** policy
- **Tickets:** T-023
- **Supersedes:** none

**Question:** D-210 ruled only on T-023 D2b. Should it be a standing hive rule stated in WAGGLE.md section 4?

**Options considered:**
1. Standing rule
2. D2b only

**Hive recommendation:** Standing rule

**Keeper choice:** Standing rule: for every budgeted brief, a target cut applies only before review, and fix and polish rounds after review count toward the stop.

**Rationale:** Matches how D-205 and D-206 are applied and keeps budgets predictable across tickets.

## D-217: D-214 trial gate cannot pass without sweep evidence; pid path proven too

- **Date:** 2026-10-22
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Two consecutive must-fix rounds on the D-214 trial gate (D-204 design card): the gate could pass over zero listable pids, and hold.sh left held.pl on the tool shell's stdout pipe so it died before the sweep. How should round 2 fix it?

**Options considered:**
1. Converged fix plus pid path
2. Converged fix only
3. Fix, skip confirmation

**Hive recommendation:** Converged fix plus pid path

**Keeper choice:** Converged fix plus pid path: hold.sh redirects held.pl's output so the tool call returns; the gate passes only if run 0a wrote held.pid, that pid is in survivors_killed and it is dead after; a missing or never-listed pid means sweep not exercised: rerun run 0a once, and if still not exercised the cutover card says so and cannot cite the gate as passed. A second probe child (held2.pl, own process group via setpgrp) proves the pid path and backs up held.pl. Opus and fable confirm the round.

**Rationale:** Closes the vacuous pass both confirming guards found and the pipe hold that would misattribute the result, and adds the pid-path evidence fable N1 showed was missing, all as runbook text.

## D-218: held2.pl pauses 0.2 s before setpgrp so the trial proves the pid path

- **Date:** 2026-10-22
- **Kind:** scope
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Third must-fix round on the trial gate (D-204 design card): held2.pl calls setpgrp before the tracker's first 25 ms poll, so the tracker can record it as a group leader and sweep kills it on the group path, making the D-217 pid-path claim a coin flip. How do we finish the gate?

**Options considered:**
1. Apply the 0.2 s fix
2. Drop the pid-path claim
3. Fix and skip the guard

**Hive recommendation:** Apply the 0.2 s fix

**Keeper choice:** Apply the 0.2 s fix: held2.pl starts with select(undef, undef, undef, 0.2); setpgrp(0, 0); and runbook 2.2 is reworded to explain first-sight leader tracking. One confirming guard.

**Rationale:** Both round 2 confirming guards found the race independently and gave the same fix; the opus guard measured the pid path at 15 of 15 with the pause.

## D-219: TestRepoConfig accepts forge.remote empty or origin, in a product PR before the hive switch

- **Date:** 2026-10-22
- **Kind:** technical
- **Tickets:** T-004, T-023
- **Supersedes:** none

**Question:** product-ci fails on PR #65 because TestRepoConfig (internal/config/config_test.go) loads the hive config.yaml and asserts forge.remote is empty, and phase F sets forge.remote: origin. How do we fix it?

**Options considered:**
1. Product PR first
2. Fix it inside #65
3. Drop the hive coupling

**Hive recommendation:** Product PR first

**Keeper choice:** Product PR first: a small internal/config test change accepts an empty remote or origin, gets one guard and merges; then #65 rebases.

**Rationale:** Keeps product and hive changes in separate PRs while letting the hive config move to remote mode.

## D-220: Cut over from subagent workers to waggle dispatch after the phase F trial

- **Date:** 2026-10-22
- **Kind:** strategy
- **Tickets:** T-023, T-026
- **Supersedes:** none

**Question:** The T-023 phase F trial (tasks/T-023-trial.md, media/T-023/trial/report.md) passed the D-214 gate on the first run 0a with no rerun, proved the pid path with held2, exercised D-213, and built, guarded and merged T-026 (#68) through dispatch in remote mode with verify-leases-and-tiers seeing the claim. Do we cut over?

**Options considered:**
1. Approve cutover
2. Records first, then cutover
3. Hold cutover

**Hive recommendation:** Approve cutover

**Keeper choice:** Approve cutover: builders, foragers and guards run through waggle dispatch from now on; the queen's handoff moves to .waggle/control/handoffs/queen-<n>.md and queen.local.md is deleted, which turns off the transitional Stop hook check. The subagent shims stay for the queen and scout sessions as the rollback path.

**Rationale:** Gate passed without a rerun, pid path proven, session close handled with a record, and the first remote-mode ticket merged green at about 0.85 USD. The eight follow-ups in the report (allowed Bash forms in the builder role, push tripwire blocking local-push tests, claim TTL with no release, reused-branch base_oid) do not block cutover and go to the follow-up control PR or new tickets.

## D-221: Hive pin moved from 6a4886e to 790ade0 in phase F step 3

- **Date:** 2026-10-22
- **Kind:** technical
- **Tickets:** T-023
- **Supersedes:** none

**Question:** Which Waggle build does this hive run after T-023 phase F step 3 (#65)?

**Options considered:**
1. Stay on 6a4886eb16e0de1d005a4ba354e72ecd1dd63756
2. Move to 790ade0b64b8cf5a6d6f6c1522c47cf5fdcd3548

**Hive recommendation:** Move to 790ade0b64b8cf5a6d6f6c1522c47cf5fdcd3548

**Keeper choice:** Move the pin from 6a4886eb16e0de1d005a4ba354e72ecd1dd63756 to 790ade0b64b8cf5a6d6f6c1522c47cf5fdcd3548; managed files re-rendered by waggle upgrade from the target build, gate workflow kept (D-143). Merged by the Head Keeper in #65; recorded in this records batch per D-215.

**Rationale:** 790ade0 carries T-023 phases A to D2b and the phase F queen template, which the phase F trial ran on (D-220).

## D-222: T-030 release counts a branch as merged only when its head is a second or later merge parent on main

- **Date:** 2026-10-22
- **Kind:** technical
- **Tickets:** T-030
- **Supersedes:** none

**Question:** T-030's merged check failed two guard rounds in a row (D-204): an ancestor test, then any-parent-of-a-merge, both pass a branch with no commits of its own once a later merge lands. How should release decide a ticket's branch has merged?

**Options considered:**
1. Second-or-later parent
2. Require own commits too
3. Drop the merged check

**Hive recommendation:** Second-or-later parent

**Keeper choice:** Second-or-later parent: the branch head must be the second or a later parent of a merge commit reachable from <remote>/main (git rev-list --parents --merges <head>..<remote>/main, pinned git, fields 3 onward), never only a first parent. Squash, rebase and no-commit heads count as unmerged and need --unmerged on a Keeper card.

**Rationale:** Every queen merge uses --merge, so real tickets always qualify; a no-commit branch tip is only ever a first parent. Verified with plain git by the queen and two guards. Decided on a D-204 design card. Scope of the brief's must-fix bar (round 3 opus confirm N1): it means a no-commit branch at a main tip, whose head is only ever a first parent; a branch that points at a commit already merged as a second parent passes by design and is not a must-fix.

## D-223: T-027 to T-030 briefs merged with the recommended answers to their open questions

- **Date:** 2026-10-22
- **Kind:** scope
- **Tickets:** T-025, T-027, T-028, T-029, T-030
- **Supersedes:** none

**Question:** Merge the trial follow-up briefs T-027 to T-030 (and the T-025 phase B fix), and how to settle their nine open questions?

**Options considered:**
1. Merge, take recommendations
2. Merge, rule questions later
3. Hold

**Hive recommendation:** Merge, take recommendations

**Keeper choice:** Merge, take recommendations. Q-T027-1: no role-template copy; the generated task line is the only statement. Q-T027-2: unit test on the settings JSON plus a check of the first dispatched builder commit; a trailer means a follow-up. Q-T027-3: a trailer is a warning. Q-T028-1: per-run scratch TMPDIR with a longest-match self-rewrite. Q-T028-2: remove the scratch dir only when empty, else a scratch_left warning. Q-T029-1: keep a reused branch, tell the worker the branch point, document fields, warn branch_behind_base. Q-T029-2: cost_usd stays null on timeout and interrupt, documented. Q-T030-1: an owner-only release verb that refuses until merged, --unmerged for tickets with no PR. Q-T030-2: the queen runs release without a card on a ticket whose PR she merged; --unmerged always on a card; until the records batch that updates the role template and handoff/queen.md, release only on a card. Dispatch order: T-027, T-030, then T-028 and T-029.

**Rationale:** Briefs passed two guard rounds plus confirms (opus and fable) with all must-fixes folded; scan-hive clean. Each ticket stays Tier 3 with two guards at build time.

## D-224: fable as the frontier alternate so a Tier 3 second guard can resolve

- **Date:** 2026-10-22
- **Kind:** policy
- **Tickets:** T-027
- **Supersedes:** none

**Question:** The second T-027 guard was refused (exit 3): with only opus, sonnet and haiku mapped and the guard minimum at standard, a sonnet builder plus an opus first guard leaves no model for --second. How should the second guard get a third model?

**Options considered:**
1. Add alternates.frontier: fable to config.yaml
2. Run this second guard once as a fable guard subagent
3. Hold for a product ticket with a second-guard-only alternate

**Hive recommendation:** Add alternates.frontier: fable to config.yaml

**Keeper choice:** Add alternates.frontier: fable under runtimes.adapters.claude-code in .waggle/control/config.yaml through a control PR, then rerun the T-027 second guard through dispatch. Effect: an unstepped standard builder keeps an opus first guard and gets a fable second guard; after a builder step-up to frontier, the guard becomes fable and the second guard walks down to sonnet (D-179, bounded by D-180), where both refused before. Builder, forager, scout and queen models do not change.

**Rationale:** The first proposal, alternates.standard: fable, failed review: pick() tries a level's alternate before moving up, so it would have made fable the first guard on every ticket and the T-027 second guard would still have refused. The frontier alternate keeps the first guard on opus and keeps every second guard inside waggle dispatch with a run record and a head_oid check. Reverting it is a one-line PR. Product follow-up: the ErrNoCrossModel hint names alternates.<level> at the guard level, which is the wrong level for this case. Decided by the Head Keeper via queen card on 2026-10-22; recorded by the queen.

## D-225: T-030 one-off waiver of the D-185 planned cut at 184 against a 180 target

- **Date:** 2026-10-23
- **Kind:** policy
- **Tickets:** T-030
- **Supersedes:** none

**Question:** The T-030 build is 184 added non-test lines against target 180 and stop 240. D-185 splits it along the planned cut (--unmerged to a follow-up), which here means a hive PR with the cut instruction, a builder rerun and two more rounds for 4 lines, and drops a feature ruled for in D-223. How should T-030 proceed?

**Options considered:**
1. Waive the cut for T-030 and review now
2. Apply the cut
3. Hold

**Hive recommendation:** Waive the cut for T-030 and review now

**Keeper choice:** Waive the D-185 cut for T-030 only: --unmerged stays in and the build goes to the opus guard, then the fable second guard. D-185 stands for every other build; post-review rounds count toward the 240 stop (D-210).

**Rationale:** 4 lines over target and 56 under the stop; executing the cut costs a hive PR, a builder rerun and two more review rounds, about $1.50, and removes a verb option the Keeper ruled for in D-223. Process gap for a later records batch: applying a planned cut has no defined path, since the queen writes no code and a builder rerun reads instructions only from base. Decided by the Head Keeper via queen card on 2026-10-22; recorded by the queen.

## D-226: Keep T-028 and T-029; the scout drafts a repositioning note; the second guard stays

- **Date:** 2026-10-23
- **Kind:** strategy
- **Tickets:** T-028, T-029
- **Supersedes:** none

**Question:** A prior-art review found Gas Town, Beads and Bernstein cover most of the swarm vision, and that merge-time lease and tier enforcement under any orchestrator is the strongest piece. How should work continue, and should the Tier 3 second guard become optional?

**Options considered:**
1. Keep T-028 and T-029 and have the scout draft a pivot note
2. Pause builds until the pivot is decided
3. Stay the course

**Hive recommendation:** Keep T-028 and T-029 and have the scout draft a pivot note

**Keeper choice:** Dispatch T-028 then T-029. In parallel the scout drafts an RFC note on repositioning Waggle as an agent-agnostic merge gate plus a lease-scoped headless runner, with a cut list; the pivot itself comes back as a strategy card. The D-205 second guard stays as is and is revisited after T-028 and T-029.

**Rationale:** T-028 and T-029 fix dispatch, which survives every positioning option. The repositioning is high impact and needs its own written case before a ruling. Two runs are too little data to change D-205. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-227: Upgrade hive pin to b038b414c077292e061ad7e6d83f99c8b344d766

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-027, T-028, T-030
- **Supersedes:** none

**Question:** Which Waggle build does this hive run?

**Options considered:**
1. Stay on 790ade0b64b8cf5a6d6f6c1522c47cf5fdcd3548
2. Move to b038b414c077292e061ad7e6d83f99c8b344d766

**Hive recommendation:** Move to b038b414c077292e061ad7e6d83f99c8b344d766

**Keeper choice:** Move the pin from 790ade0b64b8cf5a6d6f6c1522c47cf5fdcd3548 to b038b414c077292e061ad7e6d83f99c8b344d766; managed files re-rendered by waggle upgrade from the target build (all unchanged), gate workflow rewrite reverted (D-143).

**Rationale:** b038b41 carries T-027 (Bash task line, attribution off, trailer warning), T-030 (dispatch release) and T-028 (scratch TMPDIR through the push tripwire); the release verb unblocks T-029 without waiting out T-028's claim TTL. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-228: T-029 merges on the opus confirming guard; D-205 alternation not met

- **Date:** 2026-10-23
- **Kind:** policy
- **Tickets:** T-029
- **Supersedes:** none

**Question:** T-029's confirming guard ran on opus, the round 1 model, because dispatch does not alternate confirm models; the fable --second then refused because the builder (217e81e) and first guard (60a9d57) bases differ, and the builder's base lacks the round 2 Colima evidence. How should T-029 finish?

**Options considered:**
1. Merge on the opus confirm
2. Re-baseline the builder and rerun both guards
3. Run fable at the builder's base

**Hive recommendation:** Merge on the opus confirm

**Keeper choice:** Merge PR #80 on the opus confirming guard, with product-ci and verify-leases-and-tiers green and the Colima AC6 proof on main (records 19). D-205 alternation is not met for this ticket only. Product follow-up: dispatch should pick the other model for a confirming guard after a must-fix round, and --second should accept the first guard's base when the builder's base is an ancestor of it.

**Rationale:** The fix rounds were a one-line gofmt change and a test recipe now proven in Colima; the opus guard read that evidence and passed. Re-baselining costs about $2.50 for no new information. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-229: Reposition Waggle as an agent-agnostic merge gate; dispatch ships second

- **Date:** 2026-10-23
- **Kind:** strategy
- **Tickets:** none
- **Supersedes:** none

**Question:** The prior-art review (2026-10-23) found the RFC-0003 swarm vision largely covered by Gas Town, Beads, Bernstein and Claude Code agent teams, and rated the per-task, commit-by-commit lease check at merge as the defensible piece. Should Waggle reposition (repositioning note Q1, branch scout-repositioning)?

**Options considered:**
1. Gate first, dispatch second, cut list as in note section 4
2. Gate only; dispatch stays internal
3. Stay the course on RFC-0003

**Hive recommendation:** Gate first, dispatch second, cut list as in note section 4

**Keeper choice:** Gate first, dispatch second. Product one is Waggle Gate (lease record per task, commit-by-commit lease check, protected paths with a named human, Intent Packet trailers, append-only decisions log) as a required check under any orchestrator; product two is waggle dispatch as its reference client. Q2 to Q7 of the note follow on their own cards; the note lands in a hive PR.

**Rationale:** Reuses the strongest built code and complements the incumbents instead of competing with them. Reversible until tickets are cut or the README is rewritten. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-230: Add gofmt -l . to the hive's verification commands

- **Date:** 2026-10-23
- **Kind:** policy
- **Tickets:** none
- **Supersedes:** none

**Question:** Three fix rounds on 2026-10-23 were gofmt-only (T-030 r1, T-029 r1). Should WAGGLE.md section 3 list `gofmt -l .` so workers catch formatting before review?

**Options considered:**
1. Add it, two guards under D-205
2. Add it, one opus guard with a D-205 waiver
3. Skip

**Hive recommendation:** Add it, two guards under D-205

**Keeper choice:** Add `gofmt -l .` to WAGGLE.md section 3 in a Tier 3 control PR reviewed by two guards on different models (D-205).

**Rationale:** Moves a recurring review-round cost into the worker's own checks; the revert is one line. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-231: Cut T-024 Cells; replace with a lease-to-sandbox-settings generator

- **Date:** 2026-10-23
- **Kind:** scope
- **Tickets:** T-024
- **Supersedes:** none

**Question:** Repositioning note Q5 (after D-229): what happens to T-024 Cells (about 2,500 target lines, not started)?

**Options considered:**
1. Cut; replace with a lease-to-sandbox-settings generator for dispatch
2. Freeze the brief unchanged
3. Build as designed

**Hive recommendation:** Cut; replace with a lease-to-sandbox-settings generator for dispatch

**Keeper choice:** T-024 is cut. Dispatch gets a lease-to-sandbox-settings generator (about 150 lines) on Claude Code's own sandbox instead; the T-024 brief stays in history.

**Rationale:** Avoids rebuilding a vendor-maintained primitive. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-232: Waggle Gate reads leases from claim refs

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** none
- **Supersedes:** none

**Question:** Repositioning note Q2 (after D-229): where does the gate read a task's lease from?

**Options considered:**
1. Keep refs/waggle/claims/ticket-<n>
2. A PR label or check-run set by a maintainer
3. A lease file under a protected path

**Hive recommendation:** Keep refs/waggle/claims/ticket-<n>

**Keeper choice:** Keep refs/waggle/claims/ticket-<n> as the lease record the gate reads from the base side.

**Rationale:** Built, tested and CAS-safe; known costs are UI invisibility and no write access for outside contributors, carried as gate risks. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-233: Protected-path changes need an approving review from a listed approver

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** none
- **Supersedes:** none

**Question:** Repositioning note Q3 (after D-229): how does a protected-path change get its named human?

**Options considered:**
1. Approving review from a listed approver at the head SHA, admin bypass as fallback
2. Admin bypass only, as today
3. CODEOWNERS with required code-owner review

**Hive recommendation:** Approving review from a listed approver at the head SHA, admin bypass as fallback

**Keeper choice:** An approving review from a listed approver at the PR head SHA passes a protected-path change; admin bypass stays as the fallback.

**Rationale:** Visible in the PR and needs no admin rights. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-234: Commits bind to a ticket by subject prefix, trailer or branch name

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** none
- **Supersedes:** none

**Question:** Repositioning note Q4 (after D-229): how does a commit name its ticket?

**Options considered:**
1. Subject prefix, Waggle-Ticket: trailer or branch name, first match wins, a conflict fails
2. Subject prefix only, as today

**Hive recommendation:** Subject prefix, Waggle-Ticket: trailer or branch name, first match wins, a conflict fails

**Keeper choice:** Subject prefix, a Waggle-Ticket: trailer or the branch name binds a commit to its ticket; first match wins and a conflict between them fails the gate.

**Rationale:** Works for agents that do not control commit subjects. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-235: Repo goes public at G5 with the eight-week kill criterion

- **Date:** 2026-10-23
- **Kind:** publish
- **Tickets:** none
- **Supersedes:** none

**Question:** Repositioning note Q6 (after D-229): when does the repo go public, and is the kill criterion acceptable?

**Options considered:**
1. Public at G5, with the eight-week, three-outside-repos criterion
2. Public now, before G1 to G4
3. Stay private until a second user is found privately

**Hive recommendation:** Public at G5, with the eight-week, three-outside-repos criterion

**Keeper choice:** The repo goes public at G5, after a working demo repo where an agent PR is refused for a path outside its lease, and after redaction and the history rewrite on their own cards. Kill criterion: eight weeks after going public, fewer than three repos outside l19 running the gate as a required check and no outside issue or PR means community work stops and Waggle stays l19's internal tool. The flip itself comes back on its own card.

**Rationale:** A working demo first, then a dated test. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-236: Plain terms in user-facing docs; bee names stay in the hive

- **Date:** 2026-10-23
- **Kind:** policy
- **Tickets:** none
- **Supersedes:** none

**Question:** Repositioning note Q7 (after D-229): what vocabulary do users see?

**Options considered:**
1. Plain terms in README and the gate spec; bee names stay for this hive's roles and records
2. Keep the current vocabulary everywhere

**Hive recommendation:** Plain terms in README and the gate spec; bee names stay for this hive's roles and records

**Keeper choice:** README, the gate spec and recipes use plain terms; bee names (Keeper, Hive, Comb, Cell, Nectar, Quorum) stay in this hive's roles and records.

**Rationale:** No audience yet to carry jargon. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-237: Gate binds waggle/T-<id> and t-<id> branch names

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-031
- **Supersedes:** none

**Question:** Q-T031-1: which branch names bind a commit to a ticket in the gate (D-234 branch source)?

**Options considered:**
1. waggle/T-<id> and t-<id>, each with an optional -suffix
2. waggle/T-<id> exactly
3. Any branch containing T-<id>

**Hive recommendation:** waggle/T-<id> and t-<id>, each with an optional -suffix

**Keeper choice:** waggle/T-<id> and dispatch's t-<id> bind, each optionally followed by -<anything>; other branch names bind nothing.

**Rationale:** Dispatch already names branches t-<nnn>, so no new habit; any-branch matching binds by accident. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-238: gate.protected_paths is one comma-separated scalar

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-031
- **Supersedes:** none

**Question:** Q-T031-2: what shape does the gate.protected_paths config key take?

**Options considered:**
1. One comma-separated scalar
2. Add block sequences to the YAML subset

**Hive recommendation:** One comma-separated scalar

**Keeper choice:** gate.protected_paths is a single comma-separated scalar; the YAML subset parser (D-006) is not widened.

**Rationale:** Widening the parser that reads a Tier 3 file is new attack surface for a cosmetic gain. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-239: Gate fails closed on an invalid base config

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-031
- **Supersedes:** none

**Question:** Q-T031-3: what does the gate do when the base revision's .waggle/control/config.yaml is present but invalid?

**Options considered:**
1. Error, exit 3
2. Warn and run with no repo entries

**Hive recommendation:** Error, exit 3

**Keeper choice:** A present but invalid, non-regular or oversized base config is a gate error, exit 3.

**Rationale:** A broken config must never silently drop protection. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-240: waggle claim takes --owner, falling back to keeper_id

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-032
- **Supersedes:** none

**Question:** Q-T032-1: who owns a claim written by waggle claim?

**Options considered:**
1. --owner flag, falling back to keeper_id in .waggle/local.yaml
2. keeper_id only
3. Detect from git config or the forge login

**Hive recommendation:** --owner flag, falling back to keeper_id in .waggle/local.yaml

**Keeper choice:** An --owner <id> flag with keeper_id as the fallback.

**Rationale:** Orchestrators need a stable owner per run without a local file; the owner is a label, not authentication. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-241: waggle dispatch release stays as an alias until G4

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-032
- **Supersedes:** none

**Question:** Q-T032-2: what happens to waggle dispatch release?

**Options considered:**
1. Keep it as an alias through the same function, drop it in G4
2. Remove it now
3. Keep two implementations

**Hive recommendation:** Keep it as an alias through the same function, drop it in G4

**Keeper choice:** waggle dispatch release stays as an alias of waggle release through the same code with today's text output, and is dropped in G4.

**Rationale:** Keeps the queen's routine and T-030's tests working with no duplicated code. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-242: release counts squash and rebase merges as unmerged

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-032
- **Supersedes:** none

**Question:** Q-T032-3: does release count a squash or rebase merge as merged?

**Options considered:**
1. No, as D-222; such users pass --unmerged; revisit with forge PR state in G3 or G4
2. Accept a branch whose patches are all on main
3. Read PR merged state through internal/forge now

**Hive recommendation:** No, as D-222; such users pass --unmerged; revisit with forge PR state in G3 or G4

**Keeper choice:** No: squash and rebase merges count as unmerged and need --unmerged; revisit with forge PR state in G3 or G4.

**Rationale:** Patch matching has false positives on a delete path, and forge reads are G3's lease. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-243: Repo protected paths apply when a lease is written, not to stored claims

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-032
- **Supersedes:** none

**Question:** Q-T032-4: when the base config gains a protected path, what happens to claims that already cover it?

**Options considered:**
1. Nothing at read time; the rule applies when a lease is written, and the post-run check and gate catch commits
2. Re-validate stored claims on every read

**Hive recommendation:** Nothing at read time; the rule applies when a lease is written, and the post-run check and gate catch commits

**Keeper choice:** The rule applies at claim, renewal and takeover only; stored claims are not re-validated on read.

**Rationale:** Re-validation would mark live claims malformed, and the overlap check skips malformed claims, which loosens protection. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-244: release in a repo with no .waggle/worktrees/ takes no lock and creates nothing

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-032
- **Supersedes:** none

**Question:** Q-T032-5: what does release do in a repo with no .waggle/worktrees/?

**Options considered:**
1. Take no lock, check no worktree, create nothing
2. Create the directory and take dispatch.lock as today

**Hive recommendation:** Take no lock, check no worktree, create nothing

**Keeper choice:** No lock, no worktree check, nothing created; any other state goes through lockRepo as today.

**Rationale:** A gate-only repo has no dispatch runs to wait for; the claim compare-and-swap still refuses a racing loser. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-245: T-032 also qualifies offBranch's default base name

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-032
- **Supersedes:** none

**Question:** Fable guard N2 on the T-032 brief: should T-032 qualify the unqualified origin/main name in offBranch (run.go), which a pushed tag can shadow when the operator passes --base?

**Options considered:**
1. Fix it in T-032 by allowing the offBranch name assignment in the run.go lease rule
2. Record it as residue for a later ticket

**Hive recommendation:** Fix it in T-032 by allowing the offBranch name assignment in the run.go lease rule

**Keeper choice:** T-032 resolves offBranch's default name fully qualified, like ResolveBase's default, and AC12 covers it.

**Rationale:** Closes the last unqualified base lookup the guards found for about two lines. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-246: Upgrade hive pin to 851f7071a93eacdd1d91a44e4e268b841eb4ac53

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-031, T-032
- **Supersedes:** none

**Question:** Which Waggle build does this hive run?

**Options considered:**
1. Stay on b038b414c077292e061ad7e6d83f99c8b344d766
2. Move to 851f7071a93eacdd1d91a44e4e268b841eb4ac53

**Hive recommendation:** Move to 851f7071a93eacdd1d91a44e4e268b841eb4ac53

**Keeper choice:** Move the pin from b038b414c077292e061ad7e6d83f99c8b344d766 to 851f7071a93eacdd1d91a44e4e268b841eb4ac53; managed files re-rendered by waggle upgrade from the target build (all unchanged), gate workflow rewrite reverted (D-143).

**Rationale:** 851f707 carries T-031 (gate verify) and T-032 (claim and release verbs, protected paths at lease time and in the post-run check, qualified default base); G2 of the D-229 plan is merged and the hive should run it. After the pin every dispatch run exits 3 when the base config is invalid. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-247: A dispatched guard keeps evidence in its final message and writes no file in the repo

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-033
- **Supersedes:** none

**Question:** Q-T033-1: where does a dispatched guard put evidence it would have written under .waggle/hive/media/?

**Options considered:**
1. Inside the final message; the guard writes nothing in the repo and the queen copies what is worth keeping to media/
2. A dispatch scratch directory outside the worktree, kept in the run record
3. Widen the guard lease to .waggle/hive/reviews/ and .waggle/hive/media/

**Hive recommendation:** Inside the final message; the guard writes nothing in the repo and the queen copies what is worth keeping to media/

**Keeper choice:** Evidence goes in the guard's final message; the guard writes no file in the repo.

**Rationale:** Costs only template text and ends the lease_violation exit 1 seen on the T-032 fable second guard; a read-only reviewer gets no write access to hive records. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-248: A guard round's verdict is a dispatch-written marker at the top of review.md

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-033
- **Supersedes:** none

**Question:** Q-T033-2: which line is a guard round's verdict for the builder step-up count?

**Options considered:**
1. A versioned marker dispatch writes at the top of review.md with the parsed verdict, quoted markers escaped; legacy text before the first marker keeps today's count
2. One file per round, counting each file's last Verdict line
3. The first Verdict line after each blank-line round separator

**Hive recommendation:** A versioned marker dispatch writes at the top of review.md with the parsed verdict, quoted markers escaped; legacy text before the first marker keeps today's count

**Keeper choice:** A versioned dispatch-written marker (v1) at the top of review.md; legacy files counted as today.

**Rationale:** No new queen habit and no existing count moves; a quoted line cannot count. Blank-line separation fails because rounds contain blank lines. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.

## D-249: The scaffold gate workflow template ships the hive workflow with three stated differences

- **Date:** 2026-10-23
- **Kind:** technical
- **Tickets:** T-033
- **Supersedes:** none

**Question:** Q-T033-3: what does the scaffold gate workflow template ship?

**Options considered:**
1. The hive workflow with a base-source presence step gating setup-go and the gate step, no gate decisions fallback and a reworded comment; the hive takes the render at its next pin move
2. The hive workflow byte for byte
3. Only the permissions and claim fetch fix, keeping the D-143 revert until G4

**Hive recommendation:** The hive workflow with a base-source presence step gating setup-go and the gate step, no gate decisions fallback and a reworded comment; the hive takes the render at its next pin move

**Keeper choice:** The hive workflow with the three differences in T-033 section 2.3, applied in the hive at the next pin move on a Head Keeper card.

**Rationale:** Makes D-143's ruling safe for repos without waggle's source and ends the manual revert at every pin move. Byte for byte fails setup-go without go.mod and trips the init test. Decided by the Head Keeper via queen card on 2026-10-23; recorded by the queen.
