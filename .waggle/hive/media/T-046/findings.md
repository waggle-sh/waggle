<!-- SPDX-License-Identifier: MIT-0 -->
# T-046 findings, phase 1 (docs research)

**Answer to the section 1 question: unknown.** Missing: (a) where a `pull_request_target` check run attaches (PR head or the default branch commit in `GITHUB_SHA`) and whether it satisfies the required check there; (d) how a required check treats two check runs of one name from the GitHub Actions app; and, for O1 only, whether the chained (b) path re-evaluates the head check. No row in (a), (c) or (d) is `no`.

Phase 1 by the forager subagent on 2026-10-28 against main `afec5e1`. Sources: published GitHub docs and changelog, quoted as retrieved (long sentences are quoted as exact fragments), plus `gh api` GET calls on this repo. Every retrieval date is 2026-10-28. Two rows (C5, C6) also quote `actions/checkout`'s README and the git reference manual, marked where used.

## Sources

- [EV] https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows
- [CL] https://github.blog/changelog/2025-11-07-actions-pull_request_target-and-environment-branch-protections-changes/
- [WS] https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax
- [SU] https://docs.github.com/en/actions/reference/security/secure-use
- [DC] https://docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching
- [RSC] https://docs.github.com/en/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks
- [AR] https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets
- [AS] https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/collaborating-on-repositories-with-code-quality-features/about-status-checks
- [CR] https://docs.github.com/en/rest/checks/runs
- [RR] https://docs.github.com/en/actions/how-tos/manage-workflow-runs/re-run-workflows-and-jobs
- [RO] https://docs.github.com/en/organizations/managing-user-access-to-your-organizations-repositories/managing-repository-roles/repository-roles-for-an-organization
- [PL] https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/checking-out-pull-requests-locally
- [CO] https://github.com/actions/checkout (README, published by GitHub's `actions` org)
- [GCF] https://git-scm.com/docs/git-cat-file and [GA] https://git-scm.com/docs/gitattributes (git reference manual)

## Answers

| Row | Answer | Source and quoted passage |
|---|---|---|
| (a) head evaluation | **unknown** | See (a1) to (a4). The check run's attachment is not documented. |
| (a1) payload names base and head | yes | [EV] `pull_request_target` row, "Webhook event payload" cell: `pull_request` (the same payload the template reads at `.github/workflows/waggle-gate.yml:35, 56-57`). |
| (a2) run source is the default branch | yes (fact) | [CL] "The `pull_request_target` event now always uses the default branch for workflow source and reference:"; "`GITHUB_SHA` will point to the latest commit on that branch."; "These changes will take effect on 12/8/2025." [EV] table: `GITHUB_SHA` "Last commit on default branch". |
| (a3) base, head and claim refs in the clone | unknown (fork head) | [CO] fetch-depth: "0 indicates all history for all branches and tags." A same-repo head is a branch, so it is in the clone; a fork head is not. [PL] "Commits in a pull request are available in a repository even before the pull request is merged." and "Fetch the reference to the pull request based on its ID number." No GitHub page says `refs/pull/<n>/head` is fetchable by the job's token for a fork PR. Probe P7. Claim refs are fetched by an explicit refspec (`.github/workflows/waggle-gate.yml:30`), independent of the event. |
| (a4) check run attaches to the PR head and satisfies the rule | unknown | Not stated in [EV], [RSC], [AS] or [CR]. [RSC] says only "Required checks must pass on the latest commit SHA." and "Checks from earlier commits don't satisfy the requirement." Since `GITHUB_SHA` is a default branch commit (a2), attachment to the head is not implied. Probe P1. |
| (b) chained `workflow_run` | **unknown**; full review path if it works: yes | [EV] `pull_request_review`: "Runs your workflow when a pull request review is submitted, edited, or dismissed."; `GITHUB_REF` "PR merge branch `refs/pull/PULL_REQUEST_NUMBER/merge`" (the PR's own copy). `workflow_run`: "This event will only trigger a workflow run if the workflow file exists on the default branch."; "A workflow run is triggered regardless of the conclusion of the previous workflow."; "The workflow started by the `workflow_run` event is able to access secrets and write tokens," "even if the previous workflow was not." [WS] "`checks: write` permits an action to create a check run." Not documented: whether a later `probe-gate` check run on the head replaces the earlier failing one for the required check (the same question as (d)), and what `workflow_run.head_sha` is for a review run. Probes P3, P9. No manual step on submit or dismiss. |
| (b) label | fires: yes; full review path: **no** (manual) | [EV] activity types for `pull_request_target` include `labeled`; "By default, a workflow only runs when a `pull_request_target` event's activity type is `opened`, `synchronize`, or `reopened`." so `labeled` must be listed. Fires on the label, not on submit or dismiss: a dismissal with no push leaves the last result until someone labels again. Whether its check updates the head is (a4). |
| (b) re-run | works with a manual step; full review path: **no** | [RR] "Re-runs use the privileges of the actor who initially triggered the workflow"; "The workflow will also use the same `GITHUB_SHA` (commit SHA) and `GITHUB_REF` (git ref)". [RO] row "Create, edit, run, re-run, and cancel GitHub Actions workflows": Write, Maintain and Admin only. Cost: a writer re-runs after every approval and every dismissal. Whether the re-run's check updates the head is (a4). |
| (c) safety | **yes** | All of C1 to C7 below are `yes`; C3 and C5 carry notes for the follow-up. |
| C1 token scope | yes | [WS] "When a workflow is triggered by the `pull_request_target` event, the `GITHUB_TOKEN` is granted" "read/write repository permission, even when it is triggered from a public fork." That is the default; the same `permissions` section: "you can configure a different set of permissions for the `GITHUB_TOKEN` for each job" and "If you specify the access for any of these permissions, all of those that are not specified are set to `none`." No event is excepted, so the template's job block (`contents: read`, `pull-requests: read`) bounds the token. A confirming step rides in P7 at no extra run. |
| C2 secrets | yes | [SU] these privileged triggers "may have repository write access and access to referenced secrets". [EV] warning: "These vulnerabilities include cache poisoning and granting unintended access to write privileges or secrets." [CL] environment rules "evaluate against the execution reference", the default branch for this event. The template references no secret and no `environment:`, so no secret is exposed; an adopter who adds `secrets.*` or an environment to the gate job exposes it to fork PRs. |
| C3 workflow copy and source | yes, with a note | [CL] "The workflow file and checkout commit will always be taken from the repository's default branch,"; "only the default branch of the repository can be used as the workflow source that is executed for these events". [EV] "This event runs in the context of the default branch of the base repository". Consequence: a PR into a non-default branch runs the default branch's workflow, while `present=true` builds from the payload `$BASE_SHA` (that branch). Note (D-203, needs push rights): the follow-up should build from `GITHUB_SHA` or refuse non-default bases. P10 is not needed: the changelog settles it. |
| C4 cache | yes | [DC] "only these workflow triggers can create or overwrite caches in the default branch's scope:" `push`, `workflow_dispatch`, `repository_dispatch`, `delete`, `registry_package`, `page_build`, `schedule`; other events that resolve to the default branch "are given read-only access to caches in the default branch's scope", "such as `pull_request_target`, `issue_comment`, and `workflow_run`." The job cannot write the shared cache. The follow-up still sets `cache: false`, as `gate/action.yml:22` does. |
| C5 persisted credentials | yes, with a note | [CO] persist-credentials: "Whether to configure the token or SSH key with the local git config", "Default: true". No step runs head content, so the stored token is not reachable by PR code. The follow-up sets `persist-credentials: false`; whether the claim-ref and `refs/pull/<n>/head` fetches then still work on a private repo is not documented (functional, not safety). P7 runs both settings. |
| C6 reading head objects | yes | The gate reads the head only through plumbing: `rev-list`, `merge-base`, `diff-tree --raw`/`--name-only`, `ls-tree`, `cat-file blob` (`cmd/waggle/gate.go:169-257`, `gate_commits.go:129, 161, 250, 376`, `gate_verify.go:227-289`). [GCF] "If <type> is specified, the raw (though uncompressed) contents of the <object> will be returned."; `--filters` and `--textconv` are opt-in options the gate does not pass. [GA] "The definition of a diff driver is done in `gitconfig`, not `gitattributes` file"; filter commands are defined "in your .git/config". [CO] lfs "Default: false". The workspace under this event is the default branch, so no head `.gitattributes` is in the work tree. |
| C7 expressions | yes | [SU] "For inline scripts, the preferred approach to handling untrusted input is to set the value of the expression" (into an environment variable). The template puts only GitHub-written fields in `env:` (`.github/workflows/waggle-gate.yml:34-35, 55-58`); the event change does not alter that, and `guard_template_test.go:68` still forbids `head.ref`. |
| (d) spoof resistance | **unknown** | [RSC] "If a check and a commit status have the same name, both must pass when that name is required." (check plus status only). [AS] "GitHub Actions generates checks, not commit statuses, when workflows are run." so a PR's `pull_request` job of the same name comes from the same app. [AR] app binding: "If the status is set by any other person or integration, merging won't be allowed." which cannot separate two Actions check runs. [CR] "In a check suite, GitHub limits the number of check runs with the same name to 1000." No page says whether the latest, any failing, or every run of one name decides, or whether a PR token with `checks: write` may edit another run's check run. Probe P4. |
| R1 review-run baseline | none found; moves to P3 | `gh api repos/waggle-sh/waggle/actions/runs?event=pull_request_review`: `total_count` 0. `gh api repos/waggle-sh/waggle/pulls/<n>/reviews` for every PR #108 to #177 (all after pin `97127fc`, merged in #107): 0 reviews each. Runs listed by event: 70 `pull_request`, 30 `push`. |
| R2 ruleset | recorded | `gh api repos/waggle-sh/waggle/rulesets/25310890`: name `main`, target `~DEFAULT_BRANCH`, active; required checks `product-ci`, `verify-leases-and-tiers`, `dco`, **none with an `integration_id`**; strict policy on; `require_code_owner_review: true`; 1 approval; stale reviews dismissed on push; bypass: repository role 5, always. `gh api repos/waggle-sh/waggle/actions/permissions/workflow`: `default_workflow_permissions: read`, `can_approve_pull_request_reviews: false`. No repo endpoint returns the plan name. |
| Required workflows (out of scope, one row) | unknown | [AR] has no rule for requiring workflows; the organization ruleset page links there without plan or source-ref details. Not settled; the spike designs nothing for it. |
| `pull_request_review` `edited` (one line) | note | [EV] lists `edited` as a review activity type; the template does not use it and it is out of scope. |

## Selected outcome (section 6 rule, D-346)

**O2** on the phase 1 answers: (a) and (d) are `unknown`, and the rule gives O2 for any `unknown` in (a), (c) or (d). (c) is `yes`.

**Section 10 cut: G5 waits for T-046b.** No row in (a), (c) or (d) is `no`, and `unknown` remains in (a) and (d), so the probe can still move the outcome to O3 or O1. Separately, no full-path (b) row is `yes` and the chained row is `unknown`, which also holds G5.

Probe rows phase 2 must run (D-342), all Head Keeper card items:

- **P1** (a4): `probe/p1-target.yml`.
- **P7** (a3, C5 functional, C1 confirm): `probe/p7-clone.yml`, same-repo PR and, if the optional fork account is used, a fork PR.
- **P4** (d): `probe/p4-spoof-job.yml` and `probe/p4-spoof-api.yml`, each in its own same-repo PR, the API one under read-only, read-write, and read-only with `checks: write` declared.
- **P3** (b baseline, R1): `probe/p3-review.yml`, approve then dismiss.
- **P9** (b chained): `probe/p9-chained.yml`, approve, dismiss, then a PR that deletes `p3-review.yml`.

Not needed: P2 and P10 (settled by [CL]), P5 and P6 (manual paths, never full review paths, cannot move the outcome), P8 (C1 settled; the confirm rides in P7). About 12 runs.

Read across P4 and P9: the chained path turns the head green only if a later `probe-gate` check run supersedes the earlier failing one. If GitHub does that, a PR's own `pull_request` job of that name can do it too, so (d) would be `no` while `pull_request` workflows may post that name. If GitHub blocks on any failing run, the chained path needs to edit the protected run's check run instead (P4b's second step tests whether a PR token can do the same). So O1 needs P4 and P9 to come out in a combination the docs do not predict; O3 is the likelier best case.

## Follow-up under O1 or O3

Not selected now. If T-046b selects O1 or O3, the lease, files and size are section 6's (O1: `internal/scaffold/`, `cmd/waggle/`, `docs/`; 30 to 60 non-test lines), plus three requirements from this note: build from `GITHUB_SHA` or refuse non-default bases (C3), `cache: false` (C4), `persist-credentials: false` with whatever P7 shows the fetches need (C5). Newer `actions/checkout` releases add an input "Required to check out fork pull request code from a workflow triggered by `pull_request_target` or `workflow_run`." [CO]; the gate never checks out the head, so a pin move does not need it.

## Proposed sentences for `docs/gate.md` section 10 (not edited here)

- **L5:** "Since 8 December 2025, `pull_request_target` always runs the default branch's workflow file, so a PR cannot change the copy that runs under that event, but the gate does not use it (see T-046)."
- **L11:** "Bind `verify-leases-and-tiers` to the GitHub Actions app in the ruleset. That rejects a status from any other app or person, but not a check run of the same name from another workflow in the repository, which also comes from GitHub Actions." R2 shows this repo's ruleset binds no app on any required check.
- **L12:** no change found.

## Probe card notes for the Head Keeper (phase 2)

Order: commit `verdict` (`fail`), `p1-target.yml`, `p7-clone.yml` and `p3-review.yml` to `main`; run P1 and P7 with a same-repo PR; P3 with the second account; P4a and P4b in separate PRs (flip the token setting between P4b runs and record it); rename the P3 job to `probe-review` and commit `p9-chained.yml`; run P9. Every probe workflow pins `actions/checkout` at `11d5960a326750d5838078e36cf38b85af677262` (v4.4.0, the SHA this repo already pins), uses no other action (API calls go through the runner's `gh`), puts payload fields only in `env:`, and uses no secret beyond `github.token`.

## Phase 2 result

Phase 1 selected O2 pending the probe. See `probe-log.md` for phase 2 and DECISIONS.md for the Head Keeper's ruling: O1 for same-repo PRs, with fork rows and the real-gate test in T-048 before G5.
