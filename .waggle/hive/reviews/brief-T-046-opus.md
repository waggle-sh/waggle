<!-- SPDX-License-Identifier: MIT-0 -->
# Brief guard review: T-046 (opus)

- **Reviewed:** `.waggle/hive/tasks/T-046.md` on `t-046-brief`, head `2503b85a8ebd80c8c2b7995a08621da0c7dbe477`, base `origin/main` `648ddc8`, PR #177.
- **Docs retrieved:** 2026-10-28.

Risk-Tier: 3

## Intent Summary

The brief scopes the D-276 spike as a forager subagent session (docs and read-only `gh api` GETs on this repo) plus an optional Head Keeper probe card on a throwaway private repo. It asks one four-part question: (a) head evaluation, (b) review trigger through a base-copy path, (c) safety under `pull_request_target`, and (d) whether a same-named check from the PR's own workflows can override a base-copy run. A selection rule maps the answers to O1 (adopt), O2 (keep `pull_request` plus code-owner review) or O3 (hybrid second check). Nothing outside `.waggle/hive/media/T-046/` changes, and the queen commits it in a D-183 records batch. Blast radius: none in code. The ruling it feeds decides whether a Tier 3 template and `cmd/waggle/` change lands before G5 (D-341 order, D-277 card order).

## Checks

**Cites at 648ddc8.** All verified: workflow lines 5-9, 16, 30, 34-35, 41, 48-51, 55-58, 62-63; `gate/action.yml:8, 11, 22, 24-35`; `gate_approve.go:42`; `gate_verify.go:137, 156`; `gate_commits.go:370`; `branches_test.go:152-155` (exit 1 under `pull_request_target`); `guard_template_test.go:68`; `gate_action_test.go:66, 132`; `launch.go:85-87` (denies `WebFetch`, `WebSearch`); `backend.go:461`; `docs/gate.md:49, 78, 84`; `docs/recipes/README.md:14`; `.github/CODEOWNERS:11`; `queen.md:60`; `hive-control-d269-opus.md:30`. D-140, D-183, D-185, D-203, D-235, D-256, D-269, D-275 to D-277, D-339, D-341 exist and say what the brief uses them for. T-035 sections 2.2, 2.6, 2.7 and 5 and T-036 Q-T036-7 and section 5 exist.

**GitHub Actions claims, against published docs:**
- `pull_request_review` runs the merge ref. Events page: GITHUB_SHA "Last merge commit on the `GITHUB_REF` branch", GITHUB_REF "PR merge branch `refs/pull/PULL_REQUEST_NUMBER/merge`" (https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows). Correct.
- `pull_request_target` has no review activity type: its list carries only pull request types. Correct.
- `workflow_run`: "This event will only trigger a workflow run if the workflow file exists on the default branch" and "The workflow started by the `workflow_run` event is able to access secrets and write tokens". GITHUB_SHA is "Last commit on default branch", so a chained run's own check does not land on the PR head and P9's explicit Checks API post is the right probe. Correct.
- Re-runs: "Re-runs use the privileges of the actor who initially triggered the workflow" and "The workflow will also use the same `GITHUB_SHA` (commit SHA) and `GITHUB_REF` (git ref)" (https://docs.github.com/en/actions/how-tos/manage-workflow-runs/re-run-workflows-and-jobs). The page does not say whether the payload is reused, so P5 is the right place to answer it.
- Required-check evaluation: "If a check and a commit status have the same name, both must pass when that name is required." "Required checks must pass on the latest commit SHA." The same page says the merge commit must pass if it has a status, otherwise the head commit (https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/collaborating-on-repositories-with-code-quality-features/troubleshooting-required-status-checks). It says nothing about two check runs with the same name from the same app. That supports the brief's expectation that (d) goes to the probe (P4).
- Check-run identity: L11's point holds. A PR's own workflow and a GITHUB_TOKEN Checks API post both come from the GitHub Actions app, so binding by integration does not separate them.

**Threat model.** Follows D-203: attacker, trusted set, must-fix list, note list and the verbatim guard sentence are present.

**Cards.** Repo creation, ruleset, base commits, PR pushes, second-account approvals, the optional fork PR and deletion are all Head Keeper card items (4.3 items 1 to 6). The forager's Bash forms are GET-only on this repo. Nothing unsafe runs without a card.

**Budget.** Bottom-up 295, doubled to 590 (D-185), target 650, stop 800, with a planned cut at the phase line and fix rounds counted toward the stop only (D-210). Conforms.

**Questions.** Q-T046-1 to Q-T046-5 each put the recommended option first, with cost and reversibility.

## Blocking Issues (Must-Fix Only)

**M1. The section 6 selection rule can pick the wrong outcome and leaves cases unassigned.** AC3 and the section 8 must-fix list make this rule the arbiter, so the defect would steer the ruling.
1. O1 needs only "(b) on at least one path". Section 1 (b) reads "submits or dismisses". The label path (P6) re-evaluates only when someone adds a label. The re-run path (P5) needs someone to click re-run. Neither fires on a dismissal. Under O1, a dismissed approval with no new push leaves the base-copy check green, because `approvedBy` treats a DISMISSED review as not approving (`gate_approve.go:144-157`) only when the job runs. Today `pull_request_review: [submitted, dismissed]` re-runs on dismissal, so O1 on a submit-only path would regress the approver check while reporting (b) yes. Fix: a (b) path counts as yes only if both submission and dismissal re-evaluate the check on the head SHA without a manual step (or the brief states that a manual-only path fails closed and why).
2. O3 keeps the approver path on `pull_request` and `pull_request_review`, so it does not need (b) at all. Yet the rule sends "(b) has no path" to O2 and makes O3 depend on "(b) has a path only for some events". The case O3 was designed for, (b) failing, therefore never selects it. O3 does need (a) and (c) for its second check, and the rule omits both.
3. No outcome is assigned when (a) or (c) is no or unknown while (d) is yes. No rule says what an `unknown` row selects. Q-T046-1 (b) says unknowns default to O2, but section 6 does not.

Suggested rule: O1 if (a), (c) and (d) are yes and some (b) path is yes for both submitted and dismissed. O3 if (a), (c) and (d) are yes and O1's (b) condition fails. O2 otherwise, including any `unknown` in (a), (c) or (d).

## Notes (not blocking)

- **N1.** Section 1 (a) says "`GITHUB_SHA` under `pull_request_target` is a base branch commit", and C3 asks whether default-branch sourcing is in effect. It is. The changelog says "The workflow file and checkout commit will always be taken from the repository's default branch" and "These changes will take effect on 12/8/2025" (https://github.blog/changelog/2025-11-07-actions-pull_request_target-and-environment-branch-protections-changes/). The events page now gives GITHUB_SHA as "Last commit on default branch". The brief can state this as a fact and keep C3 for the consequences.
- **N2.** Under `pull_request_target`, present=true builds the gate from the payload `BASE_SHA`. A PR into a non-default branch that a writer controls would build that branch's code in a job that holds fork-reachable secrets. Under D-203 this is a note, because it needs push rights, and a same-repo `pull_request` run already has that exposure. C2 and C3 should still record it for adopters.
- **N3.** The P9 chain is triggered by a `pull_request_review` workflow that runs the PR's own copy, so a PR can delete or rename the trigger. That fails closed only if the base-copy check starts red. The forager should state this in the (b) chained row.
- **N4.** `findings.md` is capped at 200 lines (section 5), while phase 1 is estimated at 135 lines, 270 doubled, against a 650 target. Reconcile the cap with the budget, or say the cap is on the note and the budget covers the probe files.
- **N5.** Card item 4 needs a second account with write access. Q-T046-2 does not price it. Without one, P3, P5, P6 and P9 cannot run, and those rows default per the rule.
- **N6.** Section 2 item 1 says the template and the hive workflow are "identical". They differ in the Action pin line (`:68`, placeholder against a rendered SHA). "Identical in render" would be exact.
- **N7.** The hive config lists no approvers (`hive-control-d269-opus.md:31`), so R1 can show where a review-run check attaches, but not an approval's effect on it.
- **N8.** Per the troubleshooting page, the merge commit must pass if it has a status. P1 should record whether a `pull_request_target` check counts at the head when the merge commit carries a `pull_request` check of the same name, which overlaps with P4.

Verdict: must-fix
