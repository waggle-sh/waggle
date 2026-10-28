<!-- SPDX-License-Identifier: MIT-0 -->
# Brief review: T-046 (pull_request_target spike, D-276), fable

- **Reviewed head:** `2503b85a8ebd80c8c2b7995a08621da0c7dbe477` on `t-046-brief`, base `origin/main` `648ddc8f98c510e1aa9abdab3276ca51178b9cf6`, PR #177.
- **Diff:** one added file, `.waggle/hive/tasks/T-046.md` (163 lines). No product file.
- **Risk-Tier:** 3 (by path only: `.waggle/hive/`). Blast radius of the brief itself: none; nothing in it runs. The findings it produces steer one Head Keeper ruling and possibly a later Tier 3 template change, which is why the cites and the GitHub Actions claims were checked line by line.
- **Guard model:** fable (claude-fable-5-1). Read only; nothing written in the repo, no refs touched on origin.

## Intent summary

The brief scopes the D-276 spike: whether the gate workflow can run from a copy the PR cannot edit (`pull_request_target`) and still re-evaluate the required check on the PR head when a listed approver reviews, without running PR code in a privileged job. It splits the question into four yes/no parts (head evaluation, review trigger, safety, spoof resistance), assigns phase 1 to a read-only forager subagent (docs plus GET calls on this repo), puts every probe step on a throwaway repo behind one Head Keeper card, and maps the answers to three outcomes (adopt, keep `pull_request` plus code-owner review, hybrid). Output is a findings note under `.waggle/hive/media/T-046/`, committed by the queen in a records batch.

## Cite check at `648ddc8`

Every file:line cite in the brief was opened at the base commit. All hold:

- `.github/workflows/waggle-gate.yml:30, 34-35, 41, 48-51, 55-58, 62-63` and `internal/scaffold/templates/waggle-gate.yml.tmpl:5-9, 16`: triggers, job `if:`, claim fetch, base worktree build, setup-go with `go-version-file`. The two files differ only on line 68 (pin vs `{{GATE_REF}}`), so "identical" is loose but the cited lines match.
- `gate/action.yml:8, 11, 22, 24-35`: payload defaults, `cache: false`, pinned-tree build with `GOPROXY=off` and `-mod=readonly`.
- `cmd/waggle/gate_approve.go:42` (`readPREvent` accepts only `pull_request` and `pull_request_review`), `:96-160` (live review read and `approvedBy`).
- `cmd/waggle/gate_verify.go:137, 156`; `cmd/waggle/gate_commits.go:370` (`claim does not name this pull request's branch`); `cmd/waggle/branches_test.go:152-155` pins exit 1 under `pull_request_target`.
- `internal/scaffold/guard_template_test.go:68`; `internal/scaffold/gate_action_test.go:66, 132`: the `pull_request_target` bans.
- `internal/runtime/launch.go:85-87` (denies `WebFetch`, `WebSearch`, and also every `gh` form); `internal/comb/backend.go:461` (`isWaggleDir` refusal).
- `docs/gate.md:49, 78, 84` (hardcoded control paths, L5, L11); `docs/recipes/README.md:14`; `.github/CODEOWNERS:11`; `.waggle/hive/reviews/hive-control-d269-opus.md:30`; `.waggle/hive/handoff/queen.md:60`.
- Decisions D-140, D-183, D-203, D-205, D-210, D-233, D-235, D-236, D-237, D-256, D-269, D-275, D-276, D-277, D-339, D-341 exist and say what the brief attributes to them. D-341 does order the spike first among the GA items. T-035 sections 2.2, 2.6, 2.7, 5 and T-036 section 3 (Q-T036-7) and 5 carry the ban and the spike as described.

## GitHub Actions claims, checked against published docs (retrieved 2026-10-28)

- **`pull_request_target` context.** Events reference: "This event runs in the context of the default branch of the base repository, rather than in the context of the merge commit"; GITHUB_SHA "Last commit on default branch", GITHUB_REF "Default branch". Changelog "Actions: pull_request_target and environment branch protections changes": "The workflow file and checkout commit will always be taken from the repository's default branch", "GITHUB_SHA will point to the latest commit on that branch", "These changes will take effect on 12/8/2025". The brief's C3 asks the forager to confirm exactly this, so the spike will get it right; but the brief's own text in section 1 (a) and section 2 item 4 still says "a base branch commit". See note N1.
- **No review activity type on `pull_request_target`.** Its activity types include `labeled`, `review_requested` and `review_request_removed` but not `submitted` or `dismissed`. Brief (b) is right.
- **`pull_request_review`** runs with GITHUB_SHA "Last merge commit on the GITHUB_REF branch", GITHUB_REF `refs/pull/<n>/merge`: the PR's own copy, as the brief says. It also has an `edited` type the template does not list; irrelevant to the spike.
- **`workflow_run`**: "This event will only trigger a workflow run if the workflow file exists on the default branch"; "The workflow started by the workflow_run event is able to access secrets and write tokens, even if the previous workflow was not." P9 is consistent; the chained check runner is a privileged default-branch job, which is what the brief wants and what makes P9's own spoof question (d) necessary.
- **Re-runs**: "The workflow will also use the same GITHUB_SHA (commit SHA) and GITHUB_REF (git ref) of the original event"; "Re-runs use the privileges of the actor who initially triggered the workflow"; re-run allowed up to 30 days. P5's question (is the original payload reused, does the head check update) is the right open point; the docs do not say the payload is reused.
- **`GITHUB_HEAD_REF`**: "only set when the event that triggers a workflow run is either pull_request or pull_request_target". Brief section 2 item 2 is right.
- **Required checks and same-name runs.** The rulesets page says the rule can bind an app as "the expected source of status updates" and "If the status is set by any other person or integration, merging won't be allowed", but neither it nor the status checks page says how two check runs of one name on one SHA are combined. The brief's Q-T046-1 statement that the docs do not answer (d) is correct, and (d) belongs in the probe.
- **Secure-use guide**: workflows on `pull_request_target` and `workflow_run` "must not explicitly check out untrusted code, including from pull request forks", "may have repository write access and access to referenced secrets", and "share the same cache of the main branch with other privileged workflow triggers". C4 (cache scope) and C6 (read head objects only, never check them out) follow from this.
- **`actions/setup-go`** README: "caching is enabled by default". C4's premise holds.

## Threat model (D-203)

Section 8 names the attacker (same-repo and fork PR author controlling the whole head tree and any workflow its event's token allows), the trusted set, and a must-fix list that is about evidence quality and card discipline rather than hypothetical GitHub behavior. The guard prompt sentence is present and verbatim-repeatable. It follows D-203.

## Head Keeper actions

Every write outside this repo is a card item in section 4.3: repo creation, ruleset, pushing the base workflows, second-account approvals, running P1 to P9, deletion. Forager Bash forms are read-only GET calls on this repo; R1 and R2 are GETs. No builder or forager step writes anywhere. Satisfied.

## Budget (D-185)

Bottom-up 295, doubled to 590, estimate 590, target 650, stop 800, planned cut at the phase line (T-046 phase 1, T-046b probe). Fix rounds count toward the stop only (D-210). Follows D-185. See note N5 on the 200-line cap in section 5.

## Questions Q-T046-1 to Q-T046-5

Each has a recommended option first with a cost and a reversibility statement. Q-T046-3 (b) is correctly marked not possible with the cites that make it so. Satisfied.

## Blocking issues (must-fix)

**M1. The section 6 selection rule is not total, so AC3 and the section 8 must-fix "a recommended outcome that does not follow the section 6 rule" cannot be met for likely answer sets.** The rule reads: O1 only if (a), (b) on a path, (c) and (d) are all yes; O2 if (d) is no or (b) has no path; O3 if (d) is yes and (b) has a path only for some events. Two gaps:

1. (a) = no or (c) = no, with (d) = yes and (b) = yes on a full path, selects nothing. (c) = no is a realistic result: C2 asks whether adopting the event exposes repository secrets to fork PRs in adopters' repos, and the events reference and secure-use guide both say the privileged job has secrets and write tokens, so the forager may well answer (c) no for the template as adopters run it.
2. `unknown` after the last phase that runs is unmapped in section 6. Q-T046-1 (b) says an unknown row "defaults to O2" only under the docs-only option; section 10's planned cut assumes O1 or O3 can be recommended with an unknown in (a), (b) or (d), which contradicts the O1 condition "all yes".

Also, "(b) has a path only for some events" does not name a testable condition; O3's actual premise is that a base-copy run exists for the self-edit check but no base-copy path re-evaluates on a review.

Fix is a few sentences in section 6, for example: O2 if any of (a), (c) or (d) is no, or if (b) has no path, or if any of (a) to (d) is still unknown after the last phase that runs; O3 if (a), (c) and (d) are yes and a base-copy run exists but no base-copy path re-evaluates on a review (so the review path stays on `pull_request_review`); O1 only if all four are yes with (b) on at least one base-copy path that re-evaluates on a review. Then align section 10's planned-cut sentence ("G5 waits for T-046b only if ...") with the rule.

## Notes (not blocking)

- **N1. Base branch vs default branch.** Section 1 (a) and section 2 item 4 state that under `pull_request_target` GITHUB_SHA and the workspace checkout are a base branch commit. Per the events reference and the 2025-12-08 changelog quoted above they are the default branch. C3 already sends the forager to confirm, so the spike will not record a wrong answer, but the brief should say "default branch (since 2025-12-08)" in (a) and 2.4 so the probe design accounts for it: a probe repo with only `main` as base cannot distinguish base from default, so if C3 matters to an outcome, add a row with a PR into a non-default branch, or state that the demo and adopter recipes gate the default branch only.
- **N2. P4 token permission.** A PR-added workflow can create a check run only if its GITHUB_TOKEN has `checks: write`; the repo's default workflow permissions setting (read-only vs read-write) decides that and differs across adopters. The probe log should record the probe repo's setting, and P4 should run both ways or state which one it tests.
- **N3. P5 re-run actor.** Docs: re-runs use the privileges of the original actor, and anyone starting the re-run needs write access. The cost line for the re-run path should say an approver needs write access and that the re-run is a manual step after every approval.
- **N4. `gh` is also denied to dispatched workers** (`internal/runtime/launch.go:85`), not only `WebFetch` and `WebSearch`; worth adding to the lease justification and Q-T046-3 (b), since R1 and R2 need `gh api`.
- **N5. Two caps on the note.** Section 5 caps `findings.md` at 200 lines while section 10's doubled phase 1 estimate is 270; the 200 cap governs and the budget is upper-bound slack. Harmless, but one sentence saying the 200 cap is the real limit avoids a false "passed target" split.
- **N6. "identical at `648ddc8`"** in section 2 item 1: the template and the hive workflow differ on line 68 (`{{GATE_REF}}`/`{{PIN}}` vs the rendered pin). Say "identical except the pin line".
- **N7. Read-only context** lists `docs/gate.md` sections 2 and 10; section 13 (the calling workflow) is the one that matters alongside 10. Section 2 (binding a commit to a task) is not needed.
- **N8. `pull_request_review` `edited` type** exists and is not in the template; the forager may note whether a review edit should re-run anything. Not part of the D-276 question.
- **N9. Status line review depth.** The brief asks for one brief guard because it changes no product file. D-205 governs code reviews; D-256 keeps brief reviews as subagents. The queen's call; recorded only so the choice is visible.

Verdict: must-fix

## Round 2

- **Reviewed head:** `bb3cd2a38b954680240cffe1d8f4cb22c85bbba2` on `t-046-brief` (round 1 head `2503b85`, base `origin/main` `648ddc8`, PR #177). Diff between heads: one file, `.waggle/hive/tasks/T-046.md`, 69 insertions, 55 deletions; no product file.
- **Role:** confirming brief guard after two must-fix reviews (opus M1, fable M1) on the section 6 selection rule. Read only; nothing written in the repo, no refs touched on origin.
- **Docs retrieved:** 2026-10-28.

Risk-Tier: 3

### Intent summary

Round 2 replaces the section 6 selection rule with a total one keyed on (a), (c), (d) and a per-path "full review path" flag for (b), defines that flag in section 1 (b) as both submitted and dismissed re-evaluating the head check with no manual step, and realigns AC3, the section 8 lists, the guard sentence, section 10's planned cut and Q-T046-1 and Q-T046-5 to it. It folds in the round 1 notes: default-branch sourcing since 2025-12-08 as a fact in (a), 2.4 and C3 with a new probe row P10; the chained path's trigger file being the PR's own copy in (b) and P9; P4 under both default token settings; the re-run path's write-access actor and manual step; a new Q-T046-6 pricing the second account; the 200-line cap as the binding limit on the note with the budget restated to 640/700/850. Blast radius unchanged: nothing in it runs; it steers one Head Keeper ruling.

### Section 6 rule

Total and exclusive. (a), (c), (d) each take `yes`, `no` or `unknown`; each (b) path carries a full-path flag. O1 needs (a), (c), (d) all `yes` and at least one full-path flag `yes`. O3 needs (a), (c), (d) all `yes` and no full-path flag `yes` (every one `no` or `unknown`). O2 is everything else, which is exactly "some `no` or `unknown` in (a), (c) or (d)". Every combination lands in one outcome and no combination in two. Testable: the forager marks each cell and the selection follows from the table, as AC3 and section 8 require.

The full review path definition closes the dismissal gap: label and re-run are named as not full, with the reason (`approvedBy` drops a dismissed approval only when the job runs, `cmd/waggle/gate_approve.go:130-160`, verified at `648ddc8`: the function is at 130-160 and the DISMISSED handling is inside it). The chained path gets its own condition (the check must start red on every new head and only the chained run turns it green) because the triggering review workflow is the PR's copy. The section 8 must-fix list and the guard sentence both make "a label or re-run path counted as a full review path" and "a manual path counted as a full review path" blocking, matching section 1 (b).

### Consistency across sections

- AC3 ("the one the section 6 rule selects from the table, with the selecting rows named") matches section 6 and section 5 item 3.
- AC2 and section 5 item 2 both list the (b) rows with their full-path flag.
- Q-T046-1 (b) ("any unknown in (a), (c) or (d) selects O2") matches O2.
- Q-T046-5 (a) restates the rule exactly.
- Q-T046-6 (c) ("O2 or O3, never O1" without a second account) follows: the review rows that need an approver (P3, P5, P6, P9) stay `unknown`, so no full-path flag is `yes`.
- Section 10's cut sentence follows the rule for the O2 cases (a `no` in (a), (c) or (d) is final; an `unknown` there can still move to O1 or O3). See N2 for one edge.

### Round 2 additions against published docs

- **Default-branch sourcing.** Changelog 2025-11-07: "The workflow file and checkout commit will always be taken from the repository's default branch", "GITHUB_SHA will point to the latest commit on that branch", "These changes will take effect on 12/8/2025". Events reference: `pull_request_target` GITHUB_SHA "Last commit on default branch", GITHUB_REF "Default branch". Section 1 (a), section 2 item 4 and C3 now state this correctly, and P10 (PR into `release` where `verdict` differs from `main`) is the right probe: it separates the default branch workflow and checkout from the payload `base.sha`.
- **No review activity type on `pull_request_target`.** The events reference lists `assigned` through `auto_merge_disabled`; no `submitted` or `dismissed`. Section 1 (b) holds.
- **`workflow_run`.** "This event will only trigger a workflow run if the workflow file exists on the default branch"; GITHUB_SHA "Last commit on default branch". P9's design (chained run posts a check on the head SHA, then a PR deletes the trigger file) is correct and the row now asks the trigger-deletion question.
- **Re-run.** Re-run page: "Re-runs use the privileges of the actor who initially triggered the workflow"; the run uses "the same `GITHUB_SHA` (commit SHA) and `GITHUB_REF` (git ref)"; allowed "up to 30 days after its initial run". The write-access claim is not on that page; it is in the repository roles table: "Create, edit, run, re-run, and cancel GitHub Actions workflows" is Write, Maintain and Admin only. The brief's sentence is correct; see N5 for the source the forager should cite.
- **P4 under both token settings.** The repository settings page: the two options are "read and write access for all permissions (the permissive setting)" and "just read access for the contents and packages permissions (the restricted setting)", and "Anyone with write access to a repository can modify the permissions granted to the GITHUB_TOKEN" "by editing the permissions key in the workflow file". See N1: the setting flip is not a boundary for a same-repo PR.
- **Budget.** Bottom-up 140 + 150 + 30 = 320, doubled 640 (D-185: "the scout doubles its bottom-up estimate when setting estimate, target and stop"), target 700, stop 850, planned cut at the phase line, fix rounds toward the stop only (D-210, standing per D-216). The 200-line cap is stated as the binding limit on the note (section 5, section 10, AC1). Conforms.

### Cites changed in round 2, checked at `648ddc8`

`cmd/waggle/gate_approve.go:130-160` (`approvedBy`), `internal/runtime/launch.go:85-87` (denies `Bash(gh)`, `Bash(gh *)`, `WebFetch`, `WebSearch`), `internal/scaffold/guard_template_test.go:68` (forbids `pull_request_target` and `github.event.pull_request.head.ref`), `docs/gate.md` sections 5 (Risk tiers and protected paths), 10 (Limits and known bypasses) and 13 (The calling workflow), `.github/workflows/waggle-gate.yml:30, 35, 48-51, 56-57`. All hold. Section 2 item 1 now says "identical except the Action pin line", which is exact.

### Cards

Unchanged in substance: repo creation, ruleset (now also on `release`), base commits, approvals and dismissals from the second account, flipping the token setting, running the rows, deletion are all Head Keeper card items (4.3 items 1 to 6). Forager Bash forms are GET only on this repo. Nothing unsafe runs without a card.

### Blocking issues (must-fix)

None.

### Notes (not blocking)

- **N1. P4 and the `permissions` key.** Flipping the repository default between restricted and permissive does not bound a same-repo PR: the docs say anyone with write access can change the token's permissions "by editing the permissions key in the workflow file", so a PR-added workflow can declare `permissions: checks: write` under either setting. P4 should include that variant, or the "does it need `checks: write`" sub-answer will read as a boundary that is not one. The primary spoof (a job named `probe-gate`) needs no token permission at all, so the (d) outcome is not at risk; only the sub-answer is.
- **N2. Section 10 cut sentence, one edge.** "an `unknown` remains in (a), (c), (d) or every full-path (b) row" misses the case where (a), (c), (d) are `yes`, some (b) rows are `no` and one is `unknown`: the outcome is O3 and the probe could still move it to O1, yet the sentence as written says G5 does not wait for T-046b. Under O3 G5 waits for the follow-up ticket anyway, so the practical effect is only whether that follow-up starts before T-046b resolves. "or no full-path (b) row is `yes` and at least one is `unknown`" would match the rule.
- **N3. Where the phase 1 probe workflows are committed.** The lease rule admits `probe/*.yml` only as "the probe workflows as they ran, if a probe runs", while section 10's planned cut has T-046 commit "the probe workflows as text" in phase 1 (about 150 lines bottom-up). They do not fit in `findings.md` (200 cap, 140 estimated for the rest). Say in the lease rule that `probe/*.yml` holds the workflows as written in phase 1, replaced by the copies that ran in phase 2.
- **N4. Q-T046-6 (c)** says the review rows "stay `unknown`" without a second account. The chained row could in principle be answered from docs alone; the sentence holds for the probe rows, which is what the question is about.
- **N5. Re-run write access source.** The re-run page does not name an access level. The forager should cite the repository roles table ("Create, edit, run, re-run, and cancel GitHub Actions workflows": Write, Maintain, Admin) for section 1 (b)'s re-run row, so the `yes` or `no` on that row carries a quoted sentence as AC2 requires.

Verdict: pass
