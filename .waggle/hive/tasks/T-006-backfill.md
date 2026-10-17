<!-- SPDX-License-Identifier: MIT-0 -->
# T-006 backfill list and open decisions

Moved out of `docs/tasks/T-006.md` (Appendices A and B) to keep that brief under 400 lines. T-006 phase 1 transcribes Appendix A into `internal/decisions/testdata/backfill/D-0NN.json`; phase 2 records them with `waggle decide`. This file is the source of truth for that transcription.

## Appendix A: Keeper-approved decisions to backfill (verbatim input, in order)

Each item maps one to one to `testdata/backfill/D-0NN.json`. `Options` items are separated by ` || ` here and become separate array entries. "Not recorded" is literal text. An item with no `Supersedes` line has `"supersedes": []`; `Supersedes: D-0NN` becomes `"supersedes": ["D-0NN"]`.

**D-001** | kind `technical` | tickets `T-002, T-003`
- Title: Claim refs wrap a single lease.json commit
- Question: How is a Comb claim stored at refs/waggle/claims/ticket-<n>?
- Options: Bare JSON blob at the ref, as RFC-0003 3.1 says || Commit whose tree holds only lease.json, with renewals and takeovers chained
- Recommendation: Not recorded
- Choice: The ref points to a commit wrapping a single lease.json; renewals and takeovers are child commits of the previous claim; release deletes the ref.
- Rationale: Gives an audit chain readable with git log while the claim exists. Needs an RFC-0003 section 3.1 amendment (T-003).

**D-002** | kind `policy` | tickets `T-002, T-003`
- Title: Claim time rejects Tier 3 overlap
- Question: Should AcquireLease refuse leases that touch Tier 3 paths, or leave that to the merge gate?
- Options: Merge gate only || Reject at claim time any lease that equals, is inside, or contains a Tier 3 path
- Recommendation: Not recorded
- Choice: AcquireLease rejects any lease overlapping a Tier 3 path, compared case-insensitively, including the repo root.
- Rationale: Defense in depth on case-insensitive checkouts; the merge gate stays authoritative.

**D-003** | kind `technical` | tickets `T-002, T-003`
- Title: Ticket ID canonical forms
- Question: How are ticket IDs accepted, displayed, and mapped to refs?
- Options: Zero-padded refs || Display T-%03d with unpadded refs
- Recommendation: Not recorded
- Choice: Display T-%03d, ref ticket-<n> unpadded, input ^(T-)?[0-9]{1,6}$, value >= 1.
- Rationale: Leading zeros can never create two refs for one ticket, and strict input blocks ref-name injection.

**D-004** | kind `technical` | tickets `T-002, T-005`
- Title: Malformed claim handling
- Question: How do claim operations treat malformed claim refs?
- Options: Fail every operation || Skip malformed refs on other tickets and report them, refuse a malformed target
- Recommendation: Not recorded
- Choice: Malformed claims on other tickets are skipped in the overlap check and reported via a typed error; a malformed target ticket returns ErrMalformedClaim.
- Rationale: One bad ref must not block the hive; the merge gate is authoritative for overlap anyway. Clearing stuck refs is T-005.

**D-005** | kind `scope` | tickets `T-003, T-004, T-005`
- Title: Queue T-002 follow-up tickets
- Question: Which follow-up tickets are queued after T-002?
- Options: Queue T-003, T-004 and T-005 || Defer follow-ups
- Recommendation: Not recorded
- Choice: Queue T-003 (RFC amendment), T-004 (config loader) and T-005 (Controller force-release).
- Rationale: Each closes a gap T-002 left out of scope.

**D-006** | kind `technical` | tickets `T-004`
- Title: Config parser is a hand-written YAML subset
- Question: Which parser reads .waggle/config.yaml?
- Options: A: add a YAML library dependency || B: hand-written YAML subset parser || C: switch config to JSON
- Recommendation: B
- Choice: B: hand-written YAML subset parser, stdlib only, fuzz tested.
- Rationale: Config is machine-written by waggle init and small; a zero-dependency control-plane binary is worth more than YAML completeness.

**D-007** | kind `technical` | tickets `T-004`
- Title: Comb remote comes from forge.remote
- Question: Where does the Comb remote come from?
- Options: forge.remote in shared config || remote in per-Keeper local.yaml || derive origin from provider and comb mode
- Recommendation: forge.remote in shared config, no CLI override
- Choice: New forge.remote key in shared config, added by the Controller to the template and .waggle/config.yaml; absent means local-only; no --remote override.
- Rationale: The remote is hive-wide state; Keepers disagreeing about it would split the Comb.

**D-008** | kind `technical` | tickets `T-004`
- Title: Config loader lives in internal/config
- Question: Where does the config loader live?
- Options: New internal/config/ package || internal/comb/config.go
- Recommendation: New internal/config/ package
- Choice: New internal/config/ package; the Controller adds it to WAGGLE.md section 2.
- Rationale: Config also feeds internal/runtime later and should not couple to comb.

**D-009** | kind `policy` | tickets `T-005`
- Title: Controller identity v0.1 is an allow-list
- Question: How is the Controller identified for force-release in v0.1?
- Options: Allow-list of keeper IDs in a Tier 3 file || Signed requests verified against allowed signers || Host-only with no identity check
- Recommendation: Allow-list of keeper IDs in a Tier 3 file
- Choice: Allow-list of keeper IDs in a Tier 3 file, checked against local.yaml keeper_id; advisory against humans with push access, and documented as such.
- Rationale: A real guard against agents with little work; signed identity can follow.

**D-010** | kind `technical` | tickets `T-005`
- Title: Force-release audit is a durable ref
- Question: Where is a Controller force-release audited?
- Options: Child commit on the claim ref || DECISIONS.md entry || Durable append-only audit ref written atomically with the delete
- Recommendation: Durable audit ref; the gate does not fetch it in v0.1
- Choice: Durable append-only ref refs/waggle/audit/force-releases written atomically with the delete; the gate does not fetch it in v0.1.
- Rationale: A child commit on a deleted ref is not durable, and DECISIONS.md cannot be written atomically with a ref delete.

**D-011** | kind `policy` | tickets `T-003`
- Title: RFCs get a Tier 2 floor via docs/specs
- Question: How does the frozen RFC get an enforced Tier 2 floor?
- Options: Move RFCs into docs/specs/ || Add a docs/RFC- prefix to the Tier 2 list || Policy only
- Recommendation: Move RFCs into docs/specs/
- Choice: A later task moves RFCs into docs/specs/.
- Rationale: Inherits the existing Tier 2 floor with no code change.

**D-012** | kind `scope` | tickets `T-003`
- Title: T-003 amendment scope widened
- Question: What does the RFC-0003 Amendment 1 cover, and where is its approval recorded?
- Options: Three approved items only || Also all T-002 claim-time rules and docs/tasks/** in the RFC 3.3 Tier 2 list
- Recommendation: Widen the scope; record approval in the PR plus RFC Appendix A
- Choice: Record all T-002 claim-time rules and add docs/tasks/** to the RFC 3.3 Tier 2 list; approval is recorded in the PR plus RFC Appendix A.
- Rationale: Agents planning leases need the rules, and code and CI already enforce docs/tasks/** at Tier 2.

**D-013** | kind `policy` | tickets `T-002`
- Title: Humans review intent; 500-line task cap
- Question: What do humans review, and how large may a task be?
- Options: Humans review intent, never diffs, with a task size cap || Alternatives not recorded
- Recommendation: Not recorded
- Choice: Humans review intent, never diffs. Every decision reaching the Keeper is concise, self-contained, and carries the hive's recommendation, cost and reversibility. Tasks are capped at about 500 lines of non-test code.
- Rationale: The cap keeps guard review accurate.

**D-014** | kind `strategy` | tickets `T-006`
- Title: waggled build order
- Question: In what order is waggled built?
- Options: decide, dispatch, run.sock loop, Cells || Alternatives not recorded
- Recommendation: Not recorded
- Choice: waggle decide, then waggle dispatch (worktree plus headless claude -p), then the run.sock daemon loop, then Cells.
- Rationale: Smallest deterministic slice first; each step is usable on its own.

**D-015** | kind `policy` | tickets `T-002, T-005`
- Title: GitHub App is the sole claim writer in team mode
- Question: How is claim integrity protected in team mode?
- Options: Waggle GitHub App as sole claim writer with gate signature checks || Alternatives not recorded
- Recommendation: Not recorded
- Choice: In team mode a Waggle GitHub App is the sole claim writer via the GitHub API (GitHub-verified commits); the gate rejects claim chains containing commits not signed by the App; waggled raises an alert on rewrites.
- Rationale: Git refs have no per-ref ACL. Needs a GitHub App registration (a credential decision) later.

**D-016** | kind `policy` | tickets `T-005`
- Title: Keeper identity is the GitHub login
- Question: What identifies a Keeper?
- Options: GitHub login now, commit signing later || Alternatives not recorded
- Recommendation: Not recorded
- Choice: GitHub login first; commit signing enabled later and possibly enforced.
- Rationale: Uses an identity the forge already verifies.

**D-017** | kind `policy` | tickets none
- Title: PR merge flow with Intent Packet merge commits
- Question: How does work merge, solo or team?
- Options: PR flow with real merge commits || Alternatives not recorded
- Recommendation: Not recorded
- Choice: PR flow always, solo or team: t-<id> branch, PR, waggle gate (same Go code locally and in CI), real merge commit (no squash) whose message carries the Intent Packet with trailers for ticket, risk tier and guard verdict; auto-merge on pass.
- Rationale: One path for every hive size, with the review record kept in history.

**D-018** | kind `policy` | tickets none
- Title: Guard runs on a different model than the forager
- Question: Which model runs the guard?
- Options: A different Claude model than the forager, configurable per hive || Alternatives not recorded
- Recommendation: Not recorded
- Choice: Guard runs on a different Claude model than the forager, configurable per hive; cross-provider later via config.
- Rationale: Reduces correlated blind spots between author and reviewer.

**D-019** | kind `policy` | tickets `T-002`
- Title: T-002 reviewed split by file instead of re-cut
- Question: T-002 has 1,160 non-test lines, over the D-013 cap. Re-cut the branch or review it as one PR?
- Options: Re-cut T-002 into tasks under the cap || Keep one PR and have the guard review it split by file in 5 units
- Recommendation: Keep one PR and review it split by file
- Choice: T-002 is reviewed split by file by the guard in 5 units rather than re-cut to meet the ~500 line cap.
- Rationale: Re-cutting costs a Forager rerun for no correctness gain, and each review unit stays near the cap.

**D-020** | kind `technical` | tickets `T-004, T-006`
- Title: WAGGLE.md section 2 adds internal/config and internal/decisions
- Question: How do the new internal/config/ and internal/decisions/ packages become leasable modules?
- Options: One Controller commit adding both to WAGGLE.md section 2 || Alternatives not recorded
- Recommendation: One Controller commit adding both
- Choice: WAGGLE.md section 2 gains internal/config/ (T-004) and internal/decisions/ (T-006), committed on branch controller/waggle-md-modules (0056ad9) with Master Keeper approval.
- Rationale: WAGGLE.md is Tier 3, so module boundaries change only through a Controller commit the Master Keeper approves.

**D-021** | kind `policy` | tickets `T-007`
- Title: waggle gate rejects non-append DECISIONS.md changes
- Question: Should waggle gate reject any DECISIONS.md change that is not a pure append?
- Options: Yes, a gate check using VerifyAppendOnly || No, rely on review and the integrity parser
- Recommendation: Yes, as a follow-up task once the gate exists
- Choice: waggle gate rejects any DECISIONS.md change that is not a pure append (T-007).
- Rationale: The integrity parser cannot catch a hand edit that keeps the format valid; only a diff against the previous version can.

**D-022** | kind `policy` | tickets `T-008`
- Title: go.mod and go.sum become hardcoded Tier 3
- Question: What tier protects go.mod and go.sum?
- Options: Hardcoded Tier 3 || Alternatives not recorded
- Recommendation: Not recorded
- Choice: go.mod and go.sum become hardcoded Tier 3 (T-008).
- Rationale: Not recorded

**D-023** | kind `technical` | tickets `T-006`
- Title: Decision entries carry no approver field; backfill uses recording date
- Question: Should waggle decide entries record who approved them, and how are backfilled dates and missing history handled?
- Options: No approver field; PR approval ties each decision to a GitHub login || Self-asserted keeper field on each entry || A flag to set the original decision date
- Recommendation: No approver field; backfill with the recording date and "Not recorded" where history is missing
- Choice: In v0.1 waggle decide entries carry no approver field; PR approval ties decisions to a GitHub login. Backfilled entries use the recording date and "Not recorded" where history is missing. Defaults applied by the Queen, reversible.
- Rationale: A self-asserted field would look like identity without being one, and a date override would let anyone backdate an entry.

**D-024** | kind `strategy` | tickets `T-010`
- Title: GitHub home is the waggle-sh organization
- Question: Where does the project live on GitHub, and what is its Go module path?
- Options: waggle-dev (taken by an unrelated user account) || New organization waggle-sh
- Recommendation: Not recorded
- Choice: New organization waggle-sh; repo waggle-sh/waggle; the Go module path becomes github.com/waggle-sh/waggle via a rename ticket (T-010) after T-002 merges.
- Rationale: waggle-dev is taken by an unrelated user account, and the module path must match the repo for go install to work.

**D-025** | kind `publish` | tickets none
- Title: Repo starts private and goes public at v0.1 launch
- Question: When does the repository become public?
- Options: Private until the v0.1 launch || Alternatives not recorded
- Recommendation: Not recorded
- Choice: The repo starts private and goes public at the v0.1 launch.
- Rationale: Not recorded

**D-026** | kind `policy` | tickets `T-002`
- Title: T-002 approved; merges as the first PR
- Question: Is T-002 approved, and how does it merge?
- Options: Approve and merge as one PR || Hold for fixes before merge
- Recommendation: Approve and merge as one PR; queue one follow-up ticket for the guard findings
- Choice: The Keeper approved T-002 after a guard pass; it merges as the first PR once the repo exists, using a merge commit carrying the Intent Packet.
- Rationale: The guard verdict was pass with no blocking findings; the merge commit follows D-017.

**D-027** | kind `scope` | tickets `T-002, T-009`
- Title: T-002 guard findings become T-009, ahead of any WORKPLAN.md write
- Question: How are the non-blocking T-002 guard findings handled?
- Options: Fold the fixes into T-002 before merge || One follow-up ticket ordered before any WORKPLAN.md write
- Recommendation: One small follow-up ticket for F1 to F4 and F6
- Choice: Guard findings on T-002 become T-009, which must land before anything writes WORKPLAN.md to disk.
- Rationale: Finding F2 is an injection path into WORKPLAN.md, a steering file agents read.

**D-028** | kind `policy` | tickets none
- Title: Roles declare minimum capability levels, never model names
- Question: How does a hive choose which model runs each role?
- Options: Roles name concrete models || Roles declare a minimum capability level (frontier, standard, fast) and hive config maps levels to models per runtime
- Recommendation: Not recorded
- Choice: Roles declare minimum capability levels, never model names. Levels are frontier, standard and fast; hive config maps each level to a concrete model per runtime. Minimums: architect frontier, queen frontier, guard at or above the forager's level and on a different model, forager standard, scout fast; waggled uses no model. The guard minimum cannot be lowered on Tier 2 and above; other overrides are allowed and logged as decisions. A forager steps up one level after two must-fix verdicts on a task. In v0.1 the Keeper assigns models to levels by hand; later, Waggle eval data (guard pass rates) does.
- Rationale: Not recorded

**D-029** | kind `policy` | tickets `T-013`
- Title: Glossary at docs/GLOSSARY.md, Tier 2 minimum, owned by the architect
- Question: Where do Waggle's terms get defined, and who keeps them current?
- Options: Terms defined only in the RFC || A glossary at docs/GLOSSARY.md, Tier 2 minimum, owned by the architect
- Recommendation: Not recorded
- Choice: docs/GLOSSARY.md is Tier 2 minimum, owned by the architect, linked from the README, and updated in the same PR as any brief that introduces a term.
- Rationale: Not recorded

**D-030** | kind `technical` | tickets `T-011, T-003`
- Title: Single-folder footprint; never gitignore project-owned files
- Question: Which files may Waggle create or change in a host repo?
- Options: Keep the current root files (WAGGLE.md, .devcontainer/) and the .gitignore block that ignores project files || Everything under .waggle/, plus a thin gate workflow and one marked import line
- Recommendation: Not recorded
- Choice: Waggle owns .waggle/control/ (Tier 3: config.yaml, roster.json, roles/, WAGGLE.md, devcontainer.json, gitignored local.yaml and run.sock) and .waggle/hive/ (Tier 2 minimum: tasks/, reviews/, media/, DECISIONS.md, WORKPLAN.md). Outside .waggle/ only .github/workflows/waggle-gate.yml and one marked import line in an existing CLAUDE.md or AGENTS.md, or a one-line file created if none exists. Waggle never gitignores project-owned files, only its own runtime state. waggle uninstall removes .waggle/, the workflow and the marked lines. The Cell dev container starts with devcontainer --config .waggle/control/devcontainer.json; no .devcontainer/ directory. Needs an RFC amendment (T-003).
- Rationale: Not recorded

**D-031** | kind `policy` | tickets `T-013, T-011`
- Title: "Head Keeper" replaces "Master Keeper" and "Controller"
- Question: What is the name for the Keeper with final authority over a hive?
- Options: Master Keeper || Controller || Head Keeper
- Recommendation: Not recorded
- Choice: Head Keeper replaces Master Keeper and Controller. The retired aliases are recorded in the glossary.
- Rationale: Avoids the master connotation and matches real beekeeping usage.

**D-032** | kind `scope` | tickets `T-007, T-008`
- Title: Queue a full waggle gate ticket after T-002
- Question: Which ticket builds the general merge gate?
- Options: Fold gate checks into T-007 || Queue a separate waggle gate ticket after T-002
- Recommendation: Queue a gate ticket after T-002 merges (Q-T007-2)
- Choice: A full waggle gate ticket is queued after T-002: per-commit lease checks, failing Tier floors instead of warnings, and case-insensitive path matching, including the GO.MOD and waggle.md gaps.
- Rationale: Not recorded

**D-033** | kind `policy` | tickets `T-008`
- Title: T-008 also protects go.work, go.work.sum and vendor/
- Question: Which Go dependency paths besides go.mod and go.sum are hardcoded Tier 3?
- Options: go.mod and go.sum only || Also go.work, go.work.sum and vendor/
- Recommendation: Not recorded
- Choice: T-008 also makes go.work, go.work.sum and vendor/ hardcoded Tier 3.
- Rationale: Not recorded

**D-034** | kind `scope` | tickets none
- Title: Protect internal/scaffold/templates/ with CODEOWNERS
- Question: How is the template source that every new hive receives protected in this repo?
- Options: Hardcoded tier entry || CODEOWNERS in this repo, as a separate ticket
- Recommendation: Not recorded
- Choice: internal/scaffold/templates/ is protected via CODEOWNERS in this repo, in a separate ticket.
- Rationale: Not recorded

**D-035** | kind `policy` | tickets `T-007`
- Title: A corrupt DECISIONS.md on main blocks PRs until a Head Keeper repair
- Question: What happens when DECISIONS.md on main fails the integrity check?
- Options: A skip flag on the gate check || Fail every PR touching it until a Head Keeper admin-merges a repair
- Recommendation: Not recorded
- Choice: A corrupt DECISIONS.md on main fails every PR touching it until a Head Keeper admin-merges a repair. There is no skip flag.
- Rationale: Not recorded

**D-036** | kind `policy` | tickets `T-009`
- Title: Windows device names rejected in lease paths on every OS
- Question: Should lease paths named like Windows reserved devices (CON, PRN, AUX, NUL, COM1, LPT1 and so on) be rejected on every OS?
- Options: Reject on every OS || Reject only on Windows
- Recommendation: Reject on every OS (Q-T009-1)
- Choice: Windows device names are rejected in lease paths on every OS.
- Rationale: Not recorded

**D-037** | kind `policy` | tickets `T-009`
- Title: v0.1 lease directories are ASCII safe-set only
- Question: May leased directory names contain spaces or non-ASCII characters?
- Options: ASCII safe set only, no spaces || Allow spaces and non-ASCII
- Recommendation: ASCII safe set for v0.1 (Q-T009-2)
- Choice: v0.1 lease directories use the ASCII safe set only, with no spaces.
- Rationale: Not recorded

**D-038** | kind `scope` | tickets `T-009`
- Title: Case-insensitive directory overlap deferred
- Question: Does T-009 also make lease overlap checks case-insensitive?
- Options: Fold into T-009 || Defer to the next Comb ticket
- Recommendation: Keep T-009 to the guard findings (Q-T009-3)
- Choice: Case-insensitive directory overlap is deferred to the next Comb ticket.
- Rationale: Not recorded

**D-039** | kind `scope` | tickets `T-007, T-008`
- Title: Template omits the gate decisions step; non-Go manifests and nested modules deferred
- Question: Does the scaffold gate template get the decisions step now, and do non-Go manifests and nested Go modules become Tier 3 now?
- Options: Add both now || Defer both
- Recommendation: Defer both (Q-T007-1, Q-T008-2)
- Choice: The scaffold template omits the gate decisions step until a release binary exists; non-Go manifests and nested modules are deferred to a later ticket. Defaults applied by the Queen, reversible.
- Rationale: Not recorded

**D-040** | kind `scope` | tickets `T-002, T-010, T-011`
- Title: Ticket order T-002, T-010, T-011, then everything else
- Question: In what order do the queued tickets merge?
- Options: Merge in ticket number order || T-002, then T-010, then T-011, with all other tickets rebasing after T-011
- Recommendation: Not recorded
- Choice: T-002 merges first, then T-010, then T-011; all other tickets rebase after T-011.
- Rationale: Not recorded

**D-041** | kind `policy` | tickets `T-011, T-007`
- Title: The hive runs a pinned, known-good Waggle build
- Question: Which Waggle build does our own hive run while Waggle is under construction?
- Options: The current working tree or main || A pinned known-good build recorded in hive config
- Recommendation: Not recorded
- Choice: The hive never runs on code under construction. It runs a pinned, known-good Waggle build recorded in hive config (commit or tag). Upgrades are deliberate and logged with waggle decide. Extends T-007's rule of building the checker from the base.
- Rationale: Not recorded

**D-042** | kind `policy` | tickets `T-011`
- Title: Product template changes reach the hive only through an upgrade command
- Question: How do changes to product templates reach our hive's installed copies?
- Options: Hand edits to hive copies || An upgrade command run from the pinned build, in a hive PR
- Recommendation: Not recorded
- Choice: Product template changes reach our hive only through an upgrade command run from the pinned build, in a hive PR. Hand edits to hive copies of templates are not allowed. A waggle upgrade ticket is queued.
- Rationale: Not recorded

**D-043** | kind `policy` | tickets `T-011, T-013`
- Title: Worker leases never include .waggle/
- Question: How are hive files protected from workers, and how does this repo keep product and hive changes apart?
- Options: A configurable rule || A universal rule built into Waggle with no config and no new user-facing terms
- Recommendation: Not recorded
- Choice: Universal rule built into Waggle with no config and no new user-facing terms: a worker lease can never include .waggle/; hive files change only through waggle commands or the Head Keeper. Enforced at claim time (internal/comb) and at the gate. Waggle-repo-only conventions, not in Waggle's schema: a "Layer: product | hive" line in our own briefs, CODEOWNERS on templates, and a CONTRIBUTING.md section explaining product versus hive. Mixed changes split into a product ticket, then a hive upgrade.
- Rationale: Not recorded

**D-044** | kind `policy` | tickets `T-013`
- Title: "Own honey" adopted
- Question: What do we call using Waggle to build Waggle?
- Options: Dogfooding || Own honey
- Recommendation: Not recorded
- Choice: "Own honey" is the term for using Waggle to build Waggle.
- Rationale: Not recorded

**D-045** | kind `policy` | tickets none
- Title: Product design that users rely on also lands in the RFC or product docs
- Question: Is DECISIONS.md enough to document product design?
- Options: DECISIONS.md alone || DECISIONS.md plus the RFC or product docs
- Recommendation: Not recorded
- Choice: DECISIONS.md records how the hive decided; any product design users rely on must also land in the RFC or product docs.
- Rationale: Not recorded

**D-046** | kind `strategy` | tickets none
- Title: GitHub home is the waggle-sh organization owned by Level 19 Enterprises, LLC
- Question: Who owns the GitHub organization and holds the copyright for Waggle?
- Options: Personal account || Organization waggle-sh owned by Level 19 Enterprises, LLC under the GitHub Corporate Terms
- Recommendation: Not recorded
- Choice: GitHub home is the organization waggle-sh, owned by the business Level 19 Enterprises, LLC (GitHub Corporate Terms). Copyright holder for NOTICE and copyright lines will be Level 19 Enterprises, LLC (implementation pending, ties into the open licensing item). The Keeper also owns getwaggle.dev and getwaggle.sh (no use decided yet).
- Rationale: Not recorded

**D-047** | kind `spend` | tickets `T-015`
- Title: The waggle-sh org runs GitHub Team with one seat
- Question: Which GitHub plan does the waggle-sh organization use while the repo is private?
- Options: GitHub Free with advisory-only CODEOWNERS || Make the repo public earlier || GitHub Team, one seat
- Recommendation: GitHub Team, one seat
- Choice: The org runs GitHub Team (1 seat, about $4 per month, Keeper-approved spend) so branch rulesets, required checks and code-owner review are enforced on the private repo before launch.
- Rationale: Not recorded

**D-048** | kind `policy` | tickets `T-005, T-014, T-015`
- Title: The Waggle GitHub App authors agent PRs; until it exists, PRs use admin bypass
- Question: Which account opens agent PRs and pushes branches, given GitHub never lets a PR author approve their own PR?
- Options: The Head Keeper's own account with bypass merges || A separate machine account || The Waggle GitHub App from D-015
- Recommendation: A separate machine account or GitHub App
- Choice: The Waggle GitHub App (from D-015) also authors agent PRs and pushes branches; the Head Keeper reviews. Until the App exists, PRs come from the Keeper's account and protected merges use admin bypass.
- Rationale: Not recorded

**D-049** | kind `policy` | tickets `T-014`
- Title: Tier 3 or Head Keeper PRs always fail the gate and merge by Head Keeper bypass
- Question: How does the gate treat a PR that touches Tier 3 or carries a Head Keeper commit?
- Options: Fail the gate and merge only by Head Keeper bypass || Pass when the PR has an approval from a listed login, read through the GitHub API
- Recommendation: Fail the gate and merge only by Head Keeper bypass
- Choice: Any PR touching Tier 3 or containing a Head Keeper commit fails the automatic gate and merges only via Head Keeper bypass.
- Rationale: Commit text can be forged.

**D-050** | kind `policy` | tickets `T-014`
- Title: Architect brief work runs under planning tickets
- Question: Brief-writing commits have no ticket of their own; how do they pass a gate that requires one?
- Options: Planning tickets with a full-gate claim on the hive tasks directory || Merge by bypass
- Recommendation: Planning tickets
- Choice: Architect brief work runs under planning tickets with a full-gate claim on the hive tasks directory.
- Rationale: Not recorded

**D-051** | kind `policy` | tickets `T-014`
- Title: CI gating requires forge.remote; local-only hives gate locally
- Question: How does a hive with no forge.remote, whose claims CI cannot see, run the gate?
- Options: Relax the CI gate for local hives || Keep the CI gate strict and require forge.remote; local-only hives run waggle gate locally
- Recommendation: Keep the gate strict and require forge.remote for CI
- Choice: CI gating requires forge.remote; local-only hives run waggle gate locally.
- Rationale: Not recorded

**D-052** | kind `policy` | tickets `T-004, T-012`
- Title: Guard appends one verdict line per review round; T-012 may add two config files
- Question: Where does the must-fix count come from, and may T-012 add files to internal/config/?
- Options: Count appended Verdict lines in the review file || A counter in the claim || A separate config ticket before T-012 phase B
- Recommendation: Count appended Verdict lines; allow the two new config files
- Choice: The guard appends one verdict line per review round (guard role template edit, Tier 3); T-012 may add two new files in internal/config/, T-004 files stay read-only.
- Rationale: Not recorded

**D-053** | kind `policy` | tickets `T-012, T-014, T-015`
- Title: Queen-applied reversible defaults for model levels, gate checks, branches and CODEOWNERS
- Question: Which reversible defaults from T-012, T-014 and T-015 stand until the Keeper changes them?
- Options: Apply the briefs' defaults || Hold each until answered
- Recommendation: Apply the briefs' defaults
- Choice: Defaults applied by the Queen (reversible): different model means a different model string; forager steps up once; model overrides go in a brief header line; templates ship opus/sonnet/haiku aliases; expired claims pass the gate with a warning; PR branches must be rebased with no merge commits inside the branch, while the final merge into main remains a real merge commit carrying the Intent Packet (D-017 unchanged); CODEOWNERS initial owner @paulternate.
- Rationale: Not recorded

**D-054** | kind `policy` | tickets `T-011`
- Title: Hardcoded tier lists drop root steering files Waggle no longer reads
- Question: Should root WAGGLE.md, DECISIONS.md, ROADMAP.md, WORKPLAN.md, docs/specs/ and docs/tasks/ stay in the hardcoded tier lists?
- Options: Keep them || Drop them
- Recommendation: Drop them
- Choice: Hardcoded tier lists drop root WAGGLE.md, DECISIONS.md, ROADMAP.md, WORKPLAN.md, docs/specs/ and docs/tasks/.
- Rationale: Waggle no longer reads them post T-011.

**D-055** | kind `policy` | tickets `T-011`
- Title: Root CLAUDE.md and AGENTS.md are Tier 2 minimum
- Question: Should root CLAUDE.md and AGENTS.md be hardcoded Tier 2 minimum?
- Options: Yes || No
- Recommendation: Yes
- Choice: Root CLAUDE.md and AGENTS.md are Tier 2 minimum.
- Rationale: They carry the import into the hive instructions.

**D-056** | kind `scope` | tickets `T-011`
- Title: Hive records are committed under .waggle/hive/
- Question: Are our hive's tasks, reviews and media committed?
- Options: Commit them || Keep them untracked
- Recommendation: Commit them
- Choice: Hive records (tasks, reviews, media) are committed under .waggle/hive/.
- Rationale: Not recorded

**D-057** | kind `policy` | tickets `T-011`
- Title: The Queen commits architect and guard output until waggled exists
- Question: How do architect briefs and guard reviews reach .waggle/hive/ when worker leases never include .waggle/?
- Options: The Queen commits their output as Head Keeper-approved hive commits || Workers lease the hive directory
- Recommendation: The Queen commits their output
- Choice: Until waggled exists, the Queen commits architect and guard output into .waggle/hive/ as Head Keeper-approved hive commits.
- Rationale: Not recorded

**D-058** | kind `policy` | tickets `T-011, T-013`
- Title: Queen-applied reversible defaults for T-011 and T-013 open questions
- Question: Which reversible defaults from T-011 (Q-T011-3, -4, -6, -7) and T-013 (Q-T013-1, -2) stand until the Keeper changes them?
- Options: Apply the briefs' defaults || Hold each until answered
- Recommendation: Apply the defaults
- Choice: Defaults applied by the Queen (reversible): .devcontainer/ stays Tier 3 where present; ROADMAP and specs live in the hive; agent tool config such as .claude/settings.json hooks gets its own classification ticket; Tier3ControlAuth renamed Tier3ControlPlane; docs/GLOSSARY.md is protected in this repo by CODEOWNERS rather than a hardcoded Tier 2 entry (a hardcoded entry would hit every host project's own GLOSSARY, against D-043's no-new-burden rule); CONTRIBUTING.md is not Tier 2.
- Rationale: Not recorded

**D-059** | kind `credentials` | tickets none
- Title: waggle-sh deploy access uses a fine-grained token owned by waggle-sh
- Question: Which credential does deploy work use for GitHub access to waggle-sh?
- Options: The Keeper's personal token || A new fine-grained token owned by waggle-sh
- Recommendation: Not recorded
- Choice: Waggle-sh GitHub access for deploy work uses a new fine-grained token owned by waggle-sh, stored in the L19 Claude Deploy 1Password vault.
- Rationale: Not recorded

**D-060** | kind `credentials` | tickets none
- Title: waggle-sh GitHub access on the Keeper's machine uses the gh CLI keyring login
- Question: Which credential does the Keeper's machine use for GitHub access to waggle-sh?
- Options: A new fine-grained token owned by waggle-sh in 1Password (D-059) || The existing gh CLI keyring login
- Recommendation: Not recorded
- Choice: Waggle-sh GitHub access on the Keeper's machine uses the existing gh CLI keyring login, the path a typical user takes, so the hive exercises it; no new 1Password token. Repo waggle-sh/waggle created private; main and t-002 pushed; T-002 opened as PR #1 (https://github.com/waggle-sh/waggle/pull/1) with its Intent Packet.
- Rationale: The Keeper's machine is not representative of users ("not everyone will have 1password ... or want to set up a PAT").
- Supersedes: D-059

**D-061** | kind `policy` | tickets none
- Title: Product credential policy: git push access plus the agent CLI, no secret managers
- Question: What credentials does Waggle require of Keepers, contributors, and cells?
- Options: Require PATs or a secret manager || Minimum of git push access plus the agent CLI, with an optional per-hive GitHub App for team mode
- Recommendation: Not recorded
- Choice: Waggle's minimum requirement is git with push access plus the agent CLI. Claims use plain git push with whatever git auth the Keeper already has. PRs and checks use gh if logged in, else a GITHUB_TOKEN env var. No secret-manager integrations (users wrap commands themselves, e.g. op run). Team mode adds a per-hive GitHub App created in one click via the App manifest flow, with its private key stored only in the repo's Actions secrets; Keepers never hold it and no Waggle-hosted service is needed. Relay-mode contributors need only a GitHub account. Cells hold zero credentials. PATs and 1Password are never required.
- Rationale: The Keeper's machine is not representative of users ("not everyone will have 1password ... or want to set up a PAT").

**D-062** | kind `scope` | tickets none
- Title: Release checklist includes a clean-environment test
- Question: How is a public release checked against environments unlike the Keeper's machine?
- Options: Test only on the Keeper's machine || Clean-environment test before any public release
- Recommendation: Not recorded
- Choice: The release checklist includes a clean-environment test (fresh macOS user account or Linux container without 1Password, Orca, or the Keeper's global Claude config) before any public release.
- Rationale: The Keeper's machine is not representative of users ("not everyone will have 1password ... or want to set up a PAT").

**D-063** | kind `policy` | tickets `T-016`
- Title: The target build runs waggle upgrade and becomes the pin
- Question: D-042 says the upgrade command runs "from the pinned build". Which build runs it: the old pin, or the new known-good version?
- Options: The old pinned build || The target build (the new known-good version)
- Recommendation: The target build
- Choice: Clarifies D-042: the target build (the new known-good version) runs waggle upgrade, renders its templates into a hive PR, and becomes the pin when that PR merges. The old pinned build never renders templates; it is recorded only as the "from" value.
- Rationale: The old build cannot render templates it does not contain.

**D-064** | kind `policy` | tickets `T-004, T-011, T-015`
- Title: Default merge strategy is an up-to-date check; native merge queue is opt-in
- Question: How does a hive serialize merges to main by default?
- Options: GitHub native merge queue (merge_strategy: native_queue) || Up-to-date check: branches must be up to date with main and pass waggle gate, with the gate re-run after each rebase
- Recommendation: Not recorded
- Choice: The default merge strategy is "up-to-date check": a branch must be up to date with main and pass waggle gate before merge, and waggle re-runs the gate after each rebase. Native merge queue becomes opt-in where the forge plan supports it. The config template default changes from merge_strategy: native_queue to merge_strategy: up_to_date in T-011 phase A, which already edits config.yaml.tmpl; T-004 accepts both values and treats an absent key as up_to_date; T-015's ruleset requires branches to be up to date.
- Rationale: Not recorded

**D-065** | kind `scope` | tickets `T-013, T-015`
- Title: Roles and terms live in README sections, not docs/GLOSSARY.md
- Question: Where are Waggle's roles and terms defined, and how is that kept current and protected?
- Options: docs/GLOSSARY.md, Tier 2 minimum (D-029) || README sections Roles and Terms, protected by CODEOWNERS
- Recommendation: Not recorded
- Choice: Roles and terms live in two README sections, Roles and Terms, updated in the same PR as any brief that introduces a term or role. Product versus hive layer terms go in CONTRIBUTING.md. README.md is protected in this repo by CODEOWNERS (T-015), not by a hardcoded tier. If the Terms list grows past about 60 entries it may split into docs/GLOSSARY.md linked from the README.
- Rationale: Not recorded
- Supersedes: D-029

**D-066** | kind `technical` | tickets `T-010, T-017`
- Title: Pre-existing gofmt trailing-newline issues are fixed in a separate chore ticket
- Question: Should T-010 also fix the four Go files that fail gofmt -l because they lack a trailing newline?
- Options: Fix them inside T-010 || Separate chore ticket T-017, keeping T-010 a pure rename
- Recommendation: Separate chore ticket T-017
- Choice: Default applied by the Queen (reversible): the missing trailing newlines in cmd/waggle/main.go, internal/runtime/claude.go, internal/runtime/runtime.go and internal/scaffold/init.go are fixed in chore ticket T-017 after T-010 merges, keeping T-010 a pure rename.
- Rationale: T-010's acceptance criteria require every changed line to be module-path text only.

**D-067** | kind `strategy` | tickets `T-018`
- Title: Plain-git core with a small Forge interface and a setup check that states what is not enforced
- Question: How does Waggle depend on a code host without building that host's assumptions into the core?
- Options: Build GitHub behavior directly into the core || Keep the core plain git, put host operations behind a small Forge interface, and probe each repo's capabilities at setup
- Recommendation: Not recorded
- Choice: Waggle's core stays plain git: claims, leases, gate logic, decisions, WORKPLAN and Cells. A small Forge interface covers opening, commenting on and merging changes (PRs or MRs), the CI shim file, bot identity, and relay. A setup check (in waggle init and a new waggle doctor command) probes what each repo can enforce (custom ref pushes, required checks, rulesets, merge queue, App or bot support), picks the best mode, and states plainly what is not enforced; it never silently degrades. If a host rejects refs/waggle/* pushes, claims fall back to a dedicated branch. Personal and organization GitHub accounts are handled as capability differences, not account types.
- Rationale: Not recorded

**D-068** | kind `policy` | tickets `T-005, T-014`
- Title: The hive bot SSH-signs claim and audit commits; the gate verifies against an allowed-signers file
- Question: How is D-015's rule that the gate rejects claim chains not written by the hive's bot implemented?
- Options: GitHub's verified-commit badge on commits written through the GitHub API || Git's built-in SSH commit signing by the bot, verified by the gate against an allowed-signers file in the Tier 3 control folder
- Recommendation: Not recorded
- Choice: Implements D-015: the hive's bot signs claim and audit commits with its own SSH key using git's built-in commit signing, and the gate verifies them against an allowed-signers file in the Tier 3 control folder. GitHub's verified-commit badge is not used. The bot's key lives where the bot runs (Actions secrets on GitHub). Compatible with D-016's later Keeper commit signing.
- Rationale: Not recorded
- Supersedes: D-015

**D-069** | kind `strategy` | tickets `T-018`
- Title: No second forge before v0.1 ships, and no GitHub assumption in the core
- Question: Which forge comes after GitHub, and when?
- Options: Pick the next forge now || No forge order until after v0.1 ships
- Recommendation: Not recorded
- Choice: No forge order after GitHub until after v0.1 ships. Nothing forge-specific beyond GitHub is built now, but no GitHub assumption may enter the core.
- Rationale: Not recorded

**D-070** | kind `technical` | tickets `T-012, T-013`
- Title: Drop the roster mode field
- Question: What is the roster's "mode": "swarm" field (Q-T013-3)?
- Options: Drop the field || Define modes solo and swarm || Reserve the field for v0.2 operating modes
- Recommendation: Drop the field
- Choice: Drop the roster "mode" field. The roster loader ignores a leftover mode key for one release. "Swarm" stays an informal README term. T-012 phase B's template commit deletes the line.
- Rationale: The field is undefined and nothing reads it.

**D-071** | kind `technical` | tickets `T-004, T-011`
- Title: The default merge_strategy value is named up_to_date; native_queue is opt-in
- Question: What config value names D-064's default merge strategy?
- Options: Keep native_queue as the default value || up_to_date as the default value, with native_queue opt-in
- Recommendation: Not recorded
- Choice: The default merge setting value is named up_to_date; native_queue is opt-in.
- Rationale: Not recorded

**D-072** | kind `scope` | tickets `T-011, T-019`
- Title: Product CI runs as its own workflow, required on main, before T-011 merges
- Question: Where do the product's build, vet, format and test checks run?
- Options: Inside the hive gate workflow || A separate product CI workflow required on main
- Recommendation: Not recorded
- Choice: Product CI (go build, go vet, gofmt on tracked files, go test -race) runs as its own workflow, separate from the hive gate, required on main, and must exist before T-011 merges.
- Rationale: Not recorded

**D-073** | kind `policy` | tickets `T-013, T-020`
- Title: Bee names only for concepts Waggle invents; standard words for everything users already know
- Question: When does Waggle use a bee-themed name, and when a standard industry word?
- Options: Bee-themed names throughout || Bee terms only for concepts Waggle invents, standard words for known things and precise mechanisms
- Recommendation: Not recorded
- Choice: Bee terms are used only for concepts Waggle invents (Hive, Comb, Cell, roles). Things users already know keep their standard industry words (forge, PR, CI, commit, ref), and so do precise mechanisms (Gate, Lease, Claim, Risk Tier, Intent Packet). The rule is recorded in CONTRIBUTING.md. The v0.2 role name "drone" is retired because real drones do no work; studio and assembly get names when they are designed.
- Rationale: Not recorded

**D-074** | kind `strategy` | tickets `T-020`
- Title: Roles are renamed to recognizable bee jobs: Scout plans, Builder codes, Forager triages
- Question: Should every role name match a real bee job that fits what the role does?
- Options: Keep architect, scout and forager || Rename so every role is a recognizable bee job
- Recommendation: Not recorded
- Choice: All roles become recognizable bee jobs. architect becomes Scout (plans tasks and writes briefs; scouts search ahead and dance directions to the hive). forager becomes Builder (writes code; builder bees make comb). scout becomes Forager (triage: reproduces bugs, writes failing tests, maps leases; foragers bring material back from outside). Queen and guard are unchanged. The rename is done in one rename ticket after T-011 merges.
- Rationale: Not recorded

**D-075** | kind `policy` | tickets `T-021`
- Title: Always-on token savings and one context.profile setting that never weakens the Tier 2+ guard
- Question: How does Waggle control token cost without weakening review?
- Options: No cost controls || Always-on savings plus a single profile setting with per-role overrides
- Recommendation: Not recorded
- Choice: Always-on savings: a fresh session per task, lease-scoped context, a cache-friendly prompt order (role prompt, then WAGGLE.md, then the brief), trimmed tool output, and handoff instead of compaction. One Keeper setting, context.profile, takes lean, balanced or thorough, with per-role overrides. No profile may skip or weaken the guard on Tier 2 and above. waggle status reports tokens and dollars per ticket.
- Rationale: Not recorded

**D-076** | kind `policy` | tickets `T-011, T-019`
- Title: T-011 phase A approved for merge after the guard pass, once product CI is on main
- Question: Does the Head Keeper approve T-011 phase A (branch t-011a) for merge?
- Options: Approve after the guard pass || Hold for changes
- Recommendation: Not recorded
- Choice: The Head Keeper approved T-011 phase A (branch t-011a) for merge after the guard pass, with merge_strategy up_to_date added to the config template. It merges once product CI (T-019) is on main.
- Rationale: Not recorded

**D-077** | kind `technical` | tickets `T-004, T-005, T-012, T-014`
- Title: D-068 supersedes D-015 wording; provider none stays; T-012 roster change via waggle upgrade
- Question: How are the D-015 and D-068 overlap, forge.provider none in T-004, and T-012's hive roster edit resolved?
- Options: Leave as written || Apply the Queen's defaults
- Recommendation: Apply the Queen's defaults
- Choice: D-068 supersedes D-015's implementation wording, and D-068 carries a Supersedes: D-015 line. T-004 keeps accepting forge.provider: none. T-012's roster change reaches our hive through waggle upgrade, not a hand edit (D-042). Defaults applied by the Queen.
- Rationale: Not recorded

**D-078** | kind `scope` | tickets none
- Title: The queen session hands off through a written file and a fresh session
- Question: What happens when the queen session's context grows large?
- Options: Let the runtime compact the session || Write a handoff file and start a fresh session
- Recommendation: Not recorded
- Choice: When its context grows large, the queen session writes a handoff file and continues in a fresh session. Long term, waggled makes the queen stateless.
- Rationale: Not recorded

**D-079** | kind `policy` | tickets `T-004, T-018, T-019`
- Title: Waggle never requires a paid forge plan; the gate is enforced by Waggle itself and server-side where the plan allows
- Question: Does Waggle depend on a paid forge plan (rulesets, required checks) to enforce the gate, and why does our own repo pay for GitHub Team?
- Options: Require a plan that supports rulesets and required checks || Never require a paid plan: Waggle-driven merges run waggle gate first and refuse on failure, server-side enforcement is added wherever the plan allows, and waggle doctor reports when the gate is advisory
- Recommendation: Never require a paid plan
- Choice: Waggle never requires a paid forge plan. Waggle-driven merges always run waggle gate first and refuse on failure, whatever the plan. Server-side enforcement (rulesets, required checks) is used wherever the plan allows, and waggle doctor says plainly when the gate is advisory. Our own repo uses GitHub Team only because it is private before launch; Team can be dropped after the repo goes public.
- Rationale: Not recorded

**D-080** | kind `technical` | tickets `T-020`
- Title: Claims written after the role rename are lease version 2; version 1 roles are mapped on read
- Question: How does an upgraded binary read a stored claim whose role name was written before the rename, when forager and scout both exist before and after with different meanings (Q-T020-1)?
- Options: Drain claims only, no code || New claims are lease version 2 with the new role names; version 1 claims have their role names mapped old to new on read
- Recommendation: Lease version 2 with the version 1 mapping
- Choice: Claims written after the role rename are lease version 2 and carry the new role names. Version 1 claims have their role names mapped on read (scout to forager, forager to builder, guard unchanged).
- Rationale: Without the mapping an upgraded binary silently reads an old implementation claim as a triage claim.

**D-081** | kind `scope` | tickets `T-020, T-011, T-012, T-016`
- Title: T-020 phases A and B run right after T-011, before T-012 and T-016
- Question: When do the product phases of the role rename run (Q-T020-2)?
- Options: Right after T-011, before T-012, T-016 and other tickets write more code with the old names || After T-012 merges, sweeping its role table and tests afterwards
- Recommendation: Right after T-011
- Choice: T-020 (role rename) phases A and B run right after T-011, before T-012, T-016 and other tickets write more code with the old names.
- Rationale: Code written after the rename is born with the new names, so nothing has to be swept twice. "Right after T-011" means after T-011 phase A only: the order is merge T-011 phase A, then T-020 phases A and B, then T-011 phases B and C.

**D-082** | kind `policy` | tickets `T-021, T-022`
- Title: context.profile is hive-wide in config.yaml with default balanced; waggle status shows the runtime's own cost estimate as est. $
- Question: Where does context.profile live, what is its default, and how does waggle status report dollars (Q-T021-1, Q-T021-2, Q-T021-3)?
- Options: Allow a per-Keeper value in local.yaml, default lean, and a Keeper-maintained price table || One hive-wide value in config.yaml, default balanced, and the runtime's own estimate labeled est. $ with no price table
- Recommendation: Hive-wide in config.yaml, default balanced, runtime estimate labeled est. $
- Choice: context.profile is hive-wide in config.yaml, default balanced. waggle status shows the runtime's own cost estimate labeled est. $ with no price table in Waggle.
- Rationale: Not recorded

**D-083** | kind `scope` | tickets `T-016, T-013, T-022, T-023, T-006`
- Title: Approved follow-ups from T-020 and T-021: T-016 deletes dropped managed files, T-013 seed renamed, T-022 now and T-023 after the Cell runtime decision, T-006 module path fixed
- Question: Which follow-ups raised by T-020 (Q-T020-3, Q-T020-4) and T-021 (Q-T021-5) are approved, and is the stale module path in T-006 fixed?
- Options: Leave the briefs as written || Apply the follow-ups
- Recommendation: Apply the follow-ups
- Choice: Approved follow-ups: T-016 must delete managed files the new templates no longer ship. The T-013 README seed is updated for the new role names, drone is dropped, and the D-073 rule is added. T-022 (waggle status with token and cost reporting) is written now and T-023 (worker session launcher) after the Cell runtime decision. The old module path on T-006 line 18 is fixed.
- Rationale: Not recorded

**D-084** | kind `spend` | tickets `T-019, T-015`
- Title: The waggle-sh organization is on GitHub Team, one seat, paid through a personal PayPal account with the invoice in the LLC's name
- Question: How was the GitHub Team upgrade approved in D-047 paid for, given the LLC card was declined?
- Options: Wait for the LLC card to clear || Pay through a personal PayPal account with the invoice in the LLC's name
- Recommendation: Not recorded
- Choice: The Keeper upgraded the waggle-sh organization to GitHub Team (1 seat, about $4 per month) on 2026-10-17, paid through a personal PayPal account with the invoice in the LLC's name, after the LLC card was declined by the processor.
- Rationale: Team is needed only for rulesets and required checks on the private repo before launch (D-047, D-079); it can be dropped after the repo goes public.

**D-085** | kind `policy` | tickets `T-019, T-015, T-018`
- Title: Zero Actions runs were caused by the org's billing state; product CI is green and required on main through one ruleset that T-015 extends
- Question: Why did waggle-sh/waggle create no Actions runs, and how is product CI now enforced on main?
- Options: Fix the org Actions policy, repo setting or workflow registration || Clear the org's billing hold, then require product-ci in a single main ruleset
- Recommendation: Not recorded
- Choice: Cause of the zero Actions runs in waggle-sh/waggle: the org Actions policy, repo setting and workflow registration were all correct; runs were not being created because of the org's billing state (declined card on a new org). The first runs appeared on PR #4 within a minute, after the Team upgrade cleared the billing hold. Product CI (ci.yml, T-019) merged at c367ab5 and is green on main. The main ruleset exists (id 25310890): product-ci required, branches up to date, bypass for repository admins only (the Head Keeper). Verified with scratch PR #5, which was reported BLOCKED with product-ci failing, then closed. T-015 adds verify-leases-and-tiers to this same ruleset rather than creating a second one, and waggle doctor (T-018) reports the rule as present. Evidence: docs/media/T-019/.
- Rationale: Not recorded

**D-086** | kind `policy` | tickets `T-011, T-016`
- Title: T-011 phase C is drafted by a worker in an isolated worktree, reviewed cross-model, and merged on Head Keeper approval; C-5 stays with the Head Keeper
- Question: Who runs T-011 phase C (the hive PR), given that hive files are never leasable (D-043)?
- Options: The Head Keeper runs the section 10 commands himself || Wait for T-016 (`waggle upgrade`) and let it move the control plane || A worker (architect subagent, isolated worktree) drafts the phase C hive PR verbatim from T-011 section 10 (C-1 to C-4), guard reviews cross-model, the Head Keeper approves the merge
- Recommendation: Worker drafts C-1 to C-4 in an isolated worktree, cross-model guard review, Head Keeper approves the merge
- Choice: A worker (architect subagent, isolated worktree) drafts the phase C hive PR verbatim from T-011 section 10 (C-1 to C-4). Commit subjects end "(Head Keeper)" as the brief writes them. Guard reviews cross-model. The Head Keeper approves the merge. C-5 stays with the Head Keeper in the main checkout after merge. Decided by the Head Keeper via queen card on 2026-10-17.
- Rationale: Rejected the Keeper running the commands himself and waiting for T-016. The brief already spells out every command, so a worker can execute it verbatim without touching the main checkout, and the Head Keeper keeps the merge approval and the post-merge step that only makes sense in the main checkout.

**D-087** | kind `technical` | tickets `T-011, T-020, T-016`
- Title: T-011 phase C pins `b4f2221` (the phase A merge commit); the hive keeps the old role names until T-020 phase C moves the pin through `waggle upgrade`
- Question: Which commit does T-011 phase C pin, and when does our hive pick up the renamed roles?
- Options: Pin current main `924a59f` || Pin `b4f2221` (the phase A merge commit)
- Recommendation: Pin `b4f2221`
- Choice: T-011 phase C pins `b4f2221` (the phase A merge commit). Our hive keeps the old role names (queen, architect, scout, forager, guard) until T-020 phase C moves the pin through `waggle upgrade`, which makes that the first real test of managed-file removal (D-083). Confirms the queen default recorded in T-011 section 10 and T-020 section 9. Decided by the Head Keeper via queen card on 2026-10-17.
- Rationale: Rejected pinning current main `924a59f`: it would need `git rm .waggle/control/roles/architect.md` in C-3 and would pull the T-020 section 7.3 brief sweep into this PR.

**D-088** | kind `policy` | tickets `T-006`
- Title: T-006 phase 1 merges at 708 non-test lines (D-013 budget waived for this ticket) with a 200-byte title limit so the backfill titles transcribe verbatim
- Question: Does T-006 phase 1 merge as built (708 non-test lines against the D-013 450-line budget), and does the decision title limit rise from 100 to 200 bytes?
- Options: Split `internal/decisions` into two packages or two PRs to fit the budget || Waive the budget for this ticket and merge phase 1 as built || Keep the 100-byte title limit and shorten the seven backfill titles that exceed it || Raise the title limit to 200 bytes so every backfill title transcribes verbatim
- Recommendation: Waive the budget for phase 1; raise the title limit to 200 bytes
- Choice: The D-013 450-line budget is waived for T-006 phase 1, which merges as built at 708 non-test lines (PR #14). The title limit in T-006 section 4.2 is 200 bytes (was 100), so phase 2 transcribes all 87 backfill titles verbatim. Decided by the Head Keeper via queen card on 2026-10-17.
- Rationale: Rejected splitting the package: parser, validator, renderer and append-only verifier are one round-trip contract and a split would add an import seam with no second consumer. Rejected shortening seven titles: the backfill is a transcription, and edited titles would make the record disagree with its source. The budget stays in force for every other ticket; this is a waiver, not a new limit.

**D-089** | kind `policy` | tickets `T-018, T-006, T-016, T-012, T-014, T-007, T-021, T-022`
- Title: Size figures in briefs are physical non-test lines (`wc -l` of non-test Go files); T-018 phase A accepted at 527 against a 350 cap, and the architect re-estimates phases B and C
- Question: What does a brief's "non-test lines" figure count, and does T-018 phase A merge at 527 lines against its 350 cap?
- Options: Count code lines only (blanks and comments excluded) || Count physical lines, `wc -l` of non-test Go files touched || Trim phase A to fit the 350 cap || Accept phase A as built and re-estimate phases B and C with the same metric
- Recommendation: Physical non-test lines; accept phase A; architect re-estimates B and C
- Choice: Every size figure in a brief (estimate, target, cap, stop value) counts physical non-test lines: `wc -l` over the non-test Go files the phase touches, blanks and comments included. T-018 phase A merges as built at 527 lines (PR #15). The architect re-estimates T-018 phases B and C in this metric before phase B is dispatched. Decided by the Head Keeper via queen card on 2026-10-17.
- Rationale: Rejected code-only counting: it needs a tool and an argument at every review, while `wc -l` is one command whose answer nobody disputes. Rejected trimming required behavior to fit a cap: the cap is a signal that the estimate was wrong, not a reason to drop the merge CAS or the report lines the brief requires. The fix is to estimate honestly against the metric the guard uses.

**D-090** | kind `technical` | tickets `T-016`
- Title: `waggle upgrade` reports seeded-file schema gaps with `[note]` lines; the Head Keeper edits seeded files by hand in the same PR
- Question: How does `waggle upgrade` handle a seeded file (`config.yaml`, `WAGGLE.md`, `local.yaml`) whose schema has changed between the old and the target build? (Q-T016-2)
- Options: Report-only: one `[note]` line per top-level key the build's `config.yaml.tmpl` has and the hive file lacks || In-place migration now, re-serializing the file through a comment-preserving parser
- Recommendation: Report-only
- Choice: Report-only. `upgrade` prints one `[note]` line per top-level key present in the build's `config.yaml.tmpl` and absent from the hive's `config.yaml`; the Head Keeper edits seeded files by hand in the same hive PR. Versioned migration functions keyed by a `schema` field come later, when a release first changes the schema. Decided by the Head Keeper via queen card on 2026-10-17.
- Rationale: Rejected in-place migration: it re-serializes a Head Keeper-owned file and needs a parser that preserves comments, which nothing on `main` has. No release has changed the schema yet, so the migration machinery would be built against an imagined change.

**D-091** | kind `technical` | tickets `T-016`
- Title: `waggle upgrade` refuses a release-tag downgrade unless `--allow-downgrade`; commit pins are unordered
- Question: Does `waggle upgrade` check the direction of a pin move? (Q-T016-3)
- Options: Refuse when both pins are release tags and the target is lower, unless `--allow-downgrade`; print from and to for commit pins with no ordering check || No downgrade check at all
- Recommendation: Refuse tag downgrades unless `--allow-downgrade`
- Choice: When both pins are release tags and the target is lower, `upgrade` refuses unless `--allow-downgrade`. Commit pins print from and to with no ordering check (a host repo does not contain Waggle's history). The semver compare is a hand-written numeric compare; a prerelease sorts below the same release. Decided by the Head Keeper via queen card on 2026-10-17.
- Rationale: Rejected no check at all: the Keeper picks the target on purpose, but a stale binary on `PATH` is the common way to run the wrong build, and one flag is cheaper than a reverted hive PR.

**D-092** | kind `policy` | tickets `T-016, T-006`
- Title: `waggle upgrade` prints the `waggle decide` input; a person runs `decide` in the same hive PR
- Question: Who writes the upgrade decision into `DECISIONS.md`? (Q-T016-4)
- Options: `upgrade` prints the `waggle decide` input and the Head Keeper, or the queen after an explicit Keeper answer, runs `waggle decide` in the same PR || `upgrade --apply` calls `decisions.Append` itself
- Recommendation: Print the input; a person runs `decide`
- Choice: `upgrade` prints the `waggle decide` input (T-016 section 3.7) in both modes. The Head Keeper, or the queen after an explicit Keeper answer, runs `waggle decide` in the same hive PR (T-006 section 8). `upgrade` never writes `DECISIONS.md`. Decided by the Head Keeper via queen card on 2026-10-17.
- Rationale: Rejected `--apply` calling `decisions.Append`: it turns "a command ran" into "a decision was made", adds `internal/decisions` as an import, and makes T-006 a code dependency of T-016 instead of a procedural one.

**D-093** | kind `technical` | tickets `T-016`
- Title: The pin moves through a one-line edit of `config.yaml` that requires exactly one `pin:` under `waggle:` and preserves every other byte
- Question: How does `waggle upgrade` change the pin inside the seeded `config.yaml`? (Q-T016-5)
- Options: Narrow line edit: replace the value on the one `pin:` line under `waggle:`, refuse when there are zero or two, never re-serialize the file || Parse and re-render through `internal/config`
- Recommendation: Narrow line edit
- Choice: `scaffold.WritePin` edits exactly the one `pin:` line under `waggle:`: the value is replaced; the key, indentation and trailing comment stay byte for byte; everything else in the file is untouched. Zero or two `pin:` lines are refused before any write. The file is never re-serialized through `config.Parse`. Decided by the Head Keeper via queen card on 2026-10-17.
- Rationale: Rejected re-rendering through `internal/config`: it drops every comment in a Head Keeper-owned file, and the comments are the documentation of that file.

**D-094** | kind `policy` | tickets `T-016, T-007, T-018`
- Title: No hive-only changes to managed files: a step goes into the template or not at all
- Question: May a hive keep a local change to a managed file across `waggle upgrade`? (Q-T016-6)
- Options: No per-hive exceptions and no exception list in config || A recorded exception mechanism (an `upgrade.keep:` list in `config.yaml`)
- Recommendation: No exceptions
- Choice: No per-hive exceptions and no exception list in config. A step a hive wants in the gate workflow goes into the template or not at all. T-007's `gate decisions` step therefore targets the template or waits for T-018 phase C's CI shim. Decided by the Head Keeper via queen card on 2026-10-17.
- Rationale: Rejected an `upgrade.keep:` list: it reopens the hand-edit door D-042 closed, and every kept file is one the next upgrade cannot reason about.

**D-095** | kind `technical` | tickets `T-016, T-020`
- Title: `waggle upgrade` regenerates and prunes the running Keeper's shims only; other Keepers rerun `waggle init --force`
- Question: What does `waggle upgrade` do with the `.claude/agents/waggle-*.md` shims? (Q-T016-7)
- Options: Regenerate and prune this Keeper's shims through `--runtime`; others rerun `init --force` or a future `waggle shims` || Leave shims alone and tell every Keeper to rerun init
- Recommendation: Regenerate and prune this Keeper's shims
- Choice: `upgrade` removes each shim carrying `# waggle:generated` whose role file is gone after the removals, and regenerates `.claude/agents/waggle-*.md` once when `--runtime=claude-code`. A shim without `# waggle:generated` is never touched. Other Keepers rerun `waggle init --force` or a future `waggle shims`. Decided by the Head Keeper via queen card on 2026-10-17.
- Rationale: Rejected leaving shims alone: it makes T-020 section 7.2's "shims regenerated with `waggle-architect.md` removed" a manual step, which is the kind of step that gets skipped.

**D-096** | kind `technical` | tickets `T-016, T-022, T-004`
- Title: Pins are full 40-hex commits or release tags, validated on both ends; `scaffold.ReadPin` is the shared reader and `internal/config/` stays read-only
- Question: What is a valid pin, and which package owns the reader? (Q-T016-8)
- Options: Full 40-hex or release tag, validated on both ends, `scaffold.ReadPin` shared, `internal/config/` read-only || Widen the T-016 lease to `internal/config/` and add `config.Config.Pin`
- Recommendation: `scaffold.ReadPin`, `internal/config/` read-only
- Choice: A pin is a full 40-hex commit, a release tag (`vMAJOR.MINOR.PATCH` with optional prerelease), or the literal `unpinned`; never a short hash. `upgrade` validates both the "from" and the "to" value. `scaffold.ReadPin` in `internal/scaffold/upgrade.go` is the shared reader; `waggle status` switches to it in T-022 phase B. `internal/config/` stays read-only for T-016. Decided by the Head Keeper via queen card on 2026-10-17.
- Rationale: Rejected widening the lease to `internal/config/`: `config.Config.Pin` is the cleaner long-term home, but a lease change on a package three tickets consume costs more now than a one-line switch in `status` later. Moving the reader into `internal/config` stays listed as a follow-up.

**D-097** | kind `policy` | tickets `T-016, T-020, T-011`
- Title: The generated `CLAUDE.md` is tracked like every other init output; the Head Keeper commits ours before T-020 phase C's first upgrade
- Question: What happens to the untracked generated `CLAUDE.md` in our main checkout, which trips every clean-tree check? (Q-T016-9)
- Options: Track it like all init output, no `git add` inside init or upgrade, Head Keeper commits ours as step 0 of T-020 phase C || Ignore `CLAUDE.md` in the template `.gitignore` block again || Let `upgrade` tolerate one untracked import file
- Recommendation: Track it
- Choice: `CLAUDE.md` (or `AGENTS.md`) is tracked like every other file init writes; the markers separate generated from hand-written content. Neither `init` nor `upgrade` runs `git add`; init's closing hint gains `commit CLAUDE.md (or AGENTS.md) and .gitignore with the rest`. In our hive the Head Keeper commits `CLAUDE.md` as step 0 of T-020 phase C section 7.2, before the first `upgrade` run. Decided by the Head Keeper via queen card on 2026-10-17.
- Rationale: Rejected ignoring it again: project content outside the markers becomes unignorable. Rejected a tolerance in the clean-tree rule: one special case in a precondition that exists to be absolute.

## Appendix B: Still open (do not record until the Keeper answers)

- **Cell runtime.** Queen recommends a `CellRuntime` interface with an OS-sandbox default (Seatbelt on macOS, bubblewrap on Linux) behind a host egress proxy, plus a Docker-compatible container backend. Requires an RFC amendment.
- **Licensing.** Queen recommends keeping Apache-2.0 for the CLI and daemon and MIT-0 for templates, plus DCO and SPDX headers. The copyright holder is settled as Level 19 Enterprises, LLC (D-046); the NOTICE and copyright lines land with this item.
- **GitHub merge queue availability:** confirm for an organization repo (waggle-sh/waggle, D-024) on the GitHub Team plan (D-047). Unverified; affects D-017 auto-merge and `merge_strategy: native_queue`. The repo starts private (D-025), and GitHub has documented merge queue for private organization repos only on Enterprise Cloud, so Team may not be enough. D-064 and D-071 make native queue opt-in, so this now matters only for hives that opt in; `waggle doctor` (T-018) reports it per repo.
