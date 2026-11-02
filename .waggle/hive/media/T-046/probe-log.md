<!-- SPDX-License-Identifier: MIT-0 -->
# T-046 probe log (phase 2, T-046b)

Probe repo `waggle-sh/gate-probe`, private, created 2026-10-28 with a README only. The queen ran the steps on a Head Keeper card (2026-10-28, "Queen drives"), which replaces the brief section 4.3 line that the queen does none of them; approvals come from the Head Keeper's second account; every dismissal was made by the main account through the API. Default workflow token permission at creation: `read`, `can_approve_pull_request_reviews: false`.

Setup: `main` at `8f78273` holds `verdict` = `fail`, `p1-target.yml`, `p3-review.yml`, `p7-clone.yml` (copies of `media/T-046/probe/`). Ruleset 26047427 on the default branch: pull request with 1 approval and stale reviews dismissed on push; required status check `probe-gate`, not bound to an app, not strict.

Isolation method: the 1-approval rule masks the check result, so for each check reading the queen set approvals to 0, read `mergeStateStatus` after 15 seconds, and set approvals back to 1. Control (same PRs, approvals 0, required check removed): `UNSTABLE` on #1 and #2, so `BLOCKED` with the check present comes from the required check.

Rows P1 to P4 ran on 2026-10-28 UTC; P3, E1, E2, P9 and E3 ran on 2026-11-02 UTC. Rows E1, E2 and E3 were added by the queen beyond the brief. The live ruleset also carries `require_extra_approval_for_unattributed_changes: true`.

Rule and main commit per row (the queen changed the rule between rows; `pull_request` rule stays 1 approval unless stated):

| Row | Required check | Probe main | Workflow lives on |
|---|---|---|---|
| P1, P7, P3 | `probe-gate` | `8f78273` | main |
| P4a, P4b | `probe-gate` | `8f78273` | PR branch (P4 files) |
| E1 | `e1-gate` | `8f78273` | PR branch |
| E2 | `e2-gate`, 0 approvals | `8f78273` | PR branch |
| P9 | `probe-gate` | `61411ea` (job renamed `probe-review`, `p9-chained.yml` added) | main |
| E3 | `e3-gate` | `e28e41c` (E3 files added) | main |

| Row | PR, head | Run IDs | Result |
|---|---|---|---|
| P1 | #1, `ae366bb` | 40704121855 (`pull_request_target`, probe-target, failure) | Job saw `github.sha` = `8f78273` (main), `github.ref` = `refs/heads/main`, payload base `8f78273`, head `ae366bb`. Its `probe-gate` check run is on head `ae366bb`, app `github-actions`. Merge state `BLOCKED` with approvals 0. **(a4) yes: the check run attaches to the PR head and the required check is evaluated there.** |
| P7 | #1, `ae366bb` | 40704121544 (`pull_request_target`, probe-clone, success) | `fetch-depth: 0` clone: base present, head present (same-repo branch). With `persist-credentials: false`, `git fetch origin refs/pull/1/head` failed (no stored credentials); head still present from the branch fetch. With `persist-credentials: true`, the same fetch succeeded (`pull head fetch: ok`): this is the C5 functional answer. C1 confirm: label POST with the job token returned HTTP 403 "Resource not accessible by integration". Fork rows not run (no fork account). |
| P4a | #2, `48eaa88` | 40704261179 (probe-target, failure), 40704261276 (`pull_request`, probe-spoof-job, success) | Two `probe-gate` check runs on the head, both app `github-actions`: failure (14:16:18Z) and success (14:17:44Z, later). Merge state `BLOCKED` with approvals 0; control `UNSTABLE`. **A later passing same-named run from the PR's own workflow does not satisfy the required check while the protected run is failing.** |
| P4b run 1 | #3, `35c902e` | 40704810541 (probe-spoof-api) | Token default `read`, no `permissions:` in the PR workflow. Create check run: HTTP 403. Patch the failing run: HTTP 403. Merge state `BLOCKED`. |
| P4b run 2 | not run | none | Setting the repo default to `write` returned HTTP 409 "Write permissions for workflows are disabled by the organization". The org policy pins the default to read-only, so run 2 cannot occur in this org. |
| P4b run 3 | #4, `4028254` | 40705245905 (probe-spoof-api) | Token default `read`, PR workflow declares `permissions: checks: write`. Create check run: succeeded, a passing `probe-gate` run (app `github-actions`) now sits beside the failing one. Patch the failing run: HTTP 403 "Check run status and conclusions can only be updated internally by GitHub Actions" (changelog 2025-02-12). Merge state `BLOCKED` with approvals 0. **(d) holds for the vectors tested, create and patch: a created passing run does not override, and the protected run cannot be edited. Deleting or cancelling the protected run with a PR token that has `actions: write` was not tested.** |
| P3 | #1, `ae366bb` | 41377133301 (submitted, approved), 41377175219 (dismissed); 41376521188 (a comment-only review by the PR author) | Each run: `pull_request_review`, `github.sha` = merge commit `caa3cb6`, `github.ref` = `refs/pull/1/merge`, so it ran the merge ref's copy of the workflow and `verdict`. Each posted `probe-gate` on head `ae366bb`. Fires on submit and dismiss with no manual step, and on a comment. Merge state `BLOCKED` throughout. The queen's main account dismissed through the API. |
| E1 (added row, the gate's shape) | #5, `04b5f4c` | 41377253765 (`pull_request`, failure), 41377663483 (review approved, success), 41377821904 (review dismissed, failure) | One workflow, one job `e1-gate`, required alone. With the approval standing and a passing review run on the head: `APPROVED BLOCKED`. Control, same state, check not required: `APPROVED UNSTABLE`. **A passing review run does not clear the earlier failing `pull_request` run, so an approval alone cannot turn this shape green.** |
| E2 (added row) | #6, `048e2e7` | 41377876604 | One job that fails on attempt 1 and passes on attempt 2, approvals 0. Attempt 1 `BLOCKED`; after `gh run rerun --failed`, `UNSTABLE`. **A re-run of the failed run clears it for the required check.** Not run on the real gate binary. |
| P9 | #7, `2dab4dd` | 41378080696 (probe-target, failure), 41378895064 (review), 41378909869 (`workflow_run`, probe-chained, success) | Chained run started from the review run with `workflow_run.head_sha` = head and PR 7, read 1 approval and posted a passing `probe-gate` on the head with its own token. State `APPROVED BLOCKED`. **The chained posted check does not clear the failing protected run, so this is not a full review path.** Dismissal leg: review run 41379067993, chain 41379085595, posted failure check run 125065728736. The PR that deletes `p3-review.yml` was not run: the path already fails on submit. |
| E3 (added row, chained re-run) | #8, `f85a2b0` | 41379147949 (`pull_request_target`, e3-target, attempts 1 to 3), 41379243903 and 41379417671 (review), 41379256462 and 41379433329 (`workflow_run`, e3-chain, `actions: write`) | Required check `e3-gate` alone. The job reads reviews live. Attempt 1 failure, `BLOCKED`. After the second account approved, the review run fired the chain, which re-ran the protected run: attempt 2 success, `APPROVED UNSTABLE` with no manual step (`UNSTABLE` is a non-required failing check, `probe-review`). After dismissal: attempt 3 failure, `REVIEW_REQUIRED BLOCKED`. **Both submit and dismiss re-evaluate the head check with no manual step: a full review path.** A re-run replaces the failing result; a posted check (P9) does not. Same-repo PR only; test workflow, not the real gate binary. |

Reading across rows, a hypothesis and not a documented rule: for a required name, the latest check run in each check suite must pass. So a failing protected run blocks a same-named passing run from another suite or the API (P4a, P4b run 3, E1, P9), while a re-run replaces its own suite's result (E2, E3).

Outcome under D-346: O1, chosen by the Head Keeper on a card on 2026-11-02, for same-repo PRs. Part (a) row a3 stays unknown for fork PRs, since no fork row ran. T-048 must run the fork rows on the real gate before G5; a failure there returns to the Head Keeper, because failing closed on fork PRs regresses today's fork support.

## T-048 attack rows, 2026-11-02 UTC (test workflows, not the real gate; approvals 0 to isolate the check)

Probe main `00ff6f0` held only `s-target.yml` (`pull_request_target`, job `s-gate`, always fails), so each victim head had one protected suite, the gate-only shape. Required check `s-gate`.

| Row | Victim | Attacker | Result |
|---|---|---|---|
| S10, post only | #9, `3b01f9b` | `on: push` workflow on a plain branch, `checks: write` | POST 201. Passing run landed in the protected suite 112151856031, beside the failure; both show under `filter=latest` and `filter=all`. `BLOCKED`. **A post alone does not clear the failure.** |
| S1, delete | #10, `2f35d23` | same, `actions: write` | DELETE 204 on the protected run. Its check run and suite left the commit. `BLOCKED` (required check missing). |
| S1, delete then post | #10 | second plain branch posts a pass | `CLEAN`. |
| S1, other PR | #11, `26ad367` | workflow in PR #12: delete then post | `CLEAN`. **Merged with no admin flag.** |
| S1, own PR | #13, `504a1e7` | the PR's own `pull_request` job named `s-gate` passes and deletes the protected run | `CLEAN`. |
| S1, today's shape | #14, `4f4dc5f` (main `f29a183`, `today.yml`, required `t-gate`) | plain branch: delete then post | `UNSTABLE`, mergeable (the non-required `s-gate` still fails). **Today's template has the same hole.** |

Part (d) is `no`: a credential that can push a workflow file can remove the protected result and supply a passing one. The D-346 rule selects O2. S2 and S3 were not run; they cannot change this answer. S12, whether a credential without the workflow permission is stopped, is not yet run.

S12, run by the Head Keeper on 2026-11-02 with a fine-grained token (Contents write, no Workflows). Control `s12-c` (a plain file): pushed. `s12-a` (a new workflow file): refused, "refusing to allow a Personal AccessToken to create or update workflow `.github/workflows/s12.yml` without `workflow` scope". `s12-b` (a file identical to one on another branch): pushed. **A credential without the workflow permission cannot plant a new workflow, but it can copy a workflow file that already exists on another branch.** The copied `atk-4.yml` then ran on `s12-b` (run 41385839102) with its own write token: POST of a passing check 201 (its delete got 404, since that run was already gone). So the limit holds only while no attack workflow exists on any branch of the repo. Whether a copy must match the path and bytes exactly was not tested beyond this one case.
