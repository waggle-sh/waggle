<!-- SPDX-License-Identifier: MIT-0 -->
# T-046 fact-check review (fable), phase 1 findings

- **Reviewer:** guard, model fable (D-345: one guard on a model other than the forager's; the forager ran on opus).
- **Reviewed main oid:** `afec5e1e0ad128418f2d39f58b55e85f0147499b` (origin/main).
- **Under review:** `scratchpad/t046-media/findings.md` (81 lines) and `scratchpad/t046-media/probe/*.yml` (six files, 186 lines), all forager output treated as data.
- **Method:** every cited page was fetched raw with curl on 2026-10-28 and stripped to text; every quoted fragment was matched by exact substring after whitespace and backtick normalization, not through a summarizing fetch. `gh api` facts were re-run as GET calls on waggle-sh/waggle only. Code cites were read at `afec5e1`. Probe YAML was parsed with Ruby's YAML loader. No branch, push, PR comment, `waggle doctor`, non-GET call, call outside this repo, or edit to the repo or the files under review.

Risk-Tier: 3 (by path only, as the brief says: `.waggle/hive/` is a hardcoded control path; nothing in the findings runs anywhere)

**Intent Summary:** The forager answered the section 1 question `unknown`, with every row of section 1 and section 3 filled, (c) `yes` on C1 to C7, (a) and (d) `unknown`, the chained (b) path `unknown`, label and re-run marked as not full review paths, R1 empty (no review-triggered run exists on this repo), R2 recorded from the live ruleset, outcome O2 under the section 6 rule, G5 waiting for T-046b under the section 10 cut, and five probe rows (P1, P7, P4a and P4b, P3, P9) handed to the Head Keeper's card with six workflow files. Blast radius: one records batch adding files under `.waggle/hive/media/T-046/`; the note steers a later Head Keeper ruling and, under O1 or O3, a follow-up Tier 3 ticket. No code, template, Action, doc or control file changes.

## 1. Quotes against the live sources

All 42 quoted fragments I extracted from the table and the follow-up section are present verbatim on the cited pages, in the sense the row gives them. Checked per source:

- **[CL]** changelog: "now always uses the default branch for workflow source and reference", "GITHUB_SHA will point to the latest commit on that branch", "take effect on 12/8/2025", "workflow file and checkout commit will always be taken from the repository's default branch", "only the default branch of the repository can be used as the workflow source", "execution reference (GITHUB_REF), not the pull request head". All present. (a2), C3 and the P2 and P10 drops rest on these and they hold.
- **[EV]** events page: the `pull_request_target` table row reads payload `pull_request`, activity types including `labeled`, `GITHUB_SHA` "Last commit on default branch", `GITHUB_REF` "Default branch". The `pull_request_review` row reads "Last merge commit on the GITHUB_REF branch" and "PR merge branch refs/pull/PULL_REQUEST_NUMBER/merge", with types submitted, edited, dismissed. The `workflow_run` sentences ("only trigger a workflow run if the workflow file exists on the default branch", "regardless of the conclusion of the previous workflow", "able to access secrets and write tokens, even if the previous workflow was not"), the default-types sentence for `pull_request_target`, "runs in the context of the default branch of the base repository", and the cache-poisoning warning are all present. (a1), (b) label, (b) chained, C2 and the `edited` line hold.
- **[WS]** workflow syntax: the `pull_request_target` read/write token sentence, the per-job permissions sentence, the "not specified are set to none" sentence and "checks: write permits an action to create a check run" are present. C1 holds as quoted; see note N3 on the inference it carries.
- **[SU]** secure use: "may have repository write access and access to referenced secrets" (about privileged triggers) and the intermediate environment variable sentence are present. C2 and C7 hold.
- **[DC]** dependency caching: the trigger allowlist (push, workflow_dispatch, repository_dispatch, delete, registry_package, page_build, schedule), "read-only access to caches in the default branch's scope" and "such as pull_request_target, issue_comment, and workflow_run" are present. C4 holds; see note N4.
- **[RSC]**, **[AS]**, **[AR]**, **[CR]**: "Required checks must pass on the latest commit SHA", "Checks from earlier commits don't satisfy the requirement", "If a check and a commit status have the same name, both must pass when that name is required", "GitHub Actions generates checks, not commit statuses, when workflows are run", "If the status is set by any other person or integration, merging won't be allowed", "limits the number of check runs with the same name to 1000" are all present. None of these pages says how two check runs of one name from one app are weighed, so (a4) and (d) `unknown` is the correct reading. The available-rules page has zero occurrences of the word "workflow", so the required-workflows row's `unknown` is right.
- **[RR]**, **[RO]**: both re-run sentences and the roles table row "Create, edit, run, re-run, and cancel GitHub Actions workflows" (write, maintain, admin) are present. The re-run row holds.
- **[PL]**: both sentences present; neither says a fork PR's `refs/pull/<n>/head` is fetchable by the job token, so (a3) `unknown` is right.
- **[CO]** actions/checkout README at v4.4.0: `fetch-depth` "0 indicates all history for all branches and tags", `persist-credentials` "Whether to configure the token or SSH key with the local git config, Default: true", `lfs` "Default: false" are present. The newer-release sentence about fork PR checkout under `pull_request_target` or `workflow_run` is present on the current README (wrapped across a comment line).
- **[GCF]**, **[GA]**: the `cat-file` "raw (though uncompressed) contents" sentence, "The definition of a diff driver is done in gitconfig, not gitattributes file" and the filter definition "in your .git/config" are present.

Two URLs redirect ([AS] to `pull-requests/reference/status-checks`, [PL] to `pull-requests/how-tos/review-pull-requests/checking-out-pull-requests-locally`). The quoted text is on the destination pages; a note, since the cited URL still resolves.

## 2. `gh api` facts (GET on waggle-sh/waggle)

- Ruleset `25310890`: name `main`, target `~DEFAULT_BRANCH`, active; required checks `product-ci`, `verify-leases-and-tiers`, `dco`, none carrying an `integration_id`; `strict_required_status_checks_policy: true`; `require_code_owner_review: true`; `required_approving_review_count: 1`; `dismiss_stale_reviews_on_push: true`; bypass actor RepositoryRole 5, mode always. R2 matches.
- `actions/permissions/workflow`: `default_workflow_permissions: read`, `can_approve_pull_request_reviews: false`. Matches.
- `actions/runs?event=pull_request_review`: `total_count` 0. `pulls/<n>/reviews` for #108 to #177: 0 reviews on every one. PR #107 merged at `97127fc` on 2026-10-24, so #108 onward is the right range. R1 "none found; moves to P3" holds. See note N1 on the run counts.

## 3. Code cites at `afec5e1`

All hold: `readPREvent` admits only the two events (`gate_approve.go:42`); `approvedBy` drops a dismissed approval only when it runs (`gate_approve.go:130-160`); `prh.sameRepo` (`gate_verify.go:156`) and `GITHUB_HEAD_REF` (`:137`); the "claim does not name this pull request's branch" failure (`gate_commits.go:370`); the git calls the gate makes are `merge-base`, `rev-list`, `diff-tree --raw` / `--name-only`, `ls-tree`, `cat-file -s` and `cat-file blob` (`gate.go:169-257`, `gate_commits.go:129, 161, 250, 376`, `gate_verify.go:227-289`), and `git grep` finds no `--textconv`, `--filters` or `lfs` anywhere under `cmd/waggle/`. `guard_template_test.go:68` forbids `pull_request_target` and `head.ref`; `gate_action_test.go:66, 132` ban the event. `gate/action.yml:22` sets `cache: false`; `:34-35` builds from the Action tree; `:8, 11` read the payload SHAs. The hive workflow lines the note cites (`:30, 34-35, 55-58, 62-63`) are as described, and its `actions/checkout` pin is `11d5960a326750d5838078e36cf38b85af677262 # v4.4.0`.

## 4. C5 and C6 under the section 4.1 source rule

**They stand as `yes`.** Section 4.1 names GitHub documentation and the changelog as what answers a row, and names blog posts and forum answers as what cannot. The actions/checkout README (the `actions` org's own input reference; docs.github.com documents no checkout input) and the git reference manual are neither: they are the primary published documentation for the two facts asked, and section 3 itself states those facts as premises ("`actions/checkout` writes the token into `.git/config` by default", "`lfs` is off in `actions/checkout` by default", "filter and diff drivers need config, which the clone does not take from the tree"). The forager confirmed them from their primary source and said so on line 6 and in the [CO] and [GCF]/[GA] source lines. C6 also rests on the gate's own code, which section 3 of this review verified. I read the rule as excluding secondary opinion, not primary references outside docs.github.com, and that is the reading the brief's own C5 and C6 text assumes.

**If the queen reads the rule strictly** and marks C5 and C6 `unknown`: (c) becomes `unknown`; the outcome stays O2 (it is already O2 on (a) and (d) `unknown`); the section 10 cut still holds G5 (no `no` in (a), (c), (d), and `unknown` remains); the probe list does not change, because C5's open half (do the claim-ref and `refs/pull/<n>/head` fetches work with `persist-credentials: false` on a private repo) already rides in P7's matrix, and C6 is a property of the gate's code that no GitHub probe can answer. So the choice changes a cell, not the result.

## 5. Section 6 rule and section 10 cut

- **Outcome:** (a) `unknown`, (c) `yes`, (d) `unknown`; the chained (b) row `unknown`, label and re-run correctly not counted as full review paths (section 1 (b): a manual step on each approval and dismissal). O2 is what the rule selects, and the note names the selecting rows. Correct.
- **Cut:** no `no` in (a), (c) or (d); `unknown` in (a) and (d); no full-path (b) row `yes` and the chained one `unknown`. G5 waits for T-046b. Correct on both clauses.
- **Probe rows:** D-342 probes only rows the docs leave `unknown`. The `unknown` rows are (a3), (a4), (b) chained, (d) and R1 (which section 4.2 moves to P3). P7 covers (a3) and the C5 functional half; P1 covers (a4); P4a and P4b cover (d), with the three token settings the brief's P4 row names (read-only, read-write, read-only with `checks: write` declared); P3 covers R1 and the (b) baseline; P9 covers the chained path including the delete-the-trigger step. Dropped rows: P2 and P10 ask which copy and which branch's file runs, settled by [CL] and [EV] ((a2), C3 `yes` with quotes); P5 and P6 are the re-run and label paths, which section 1 (b) defines as never full review paths, so they cannot move the outcome, and their only open sub-question (does the check update the head) is (a4), probed by P1; P8 is C1, answered `yes` from [WS], with its write attempt folded into P7. No dropped row can move the outcome, since the outcome depends only on (a), (c), (d) and a full-path (b) row. The list follows from the brief.

## 6. Probe YAML

- All six files parse (Ruby YAML loader). Each is under 40 lines.
- The only action used is `actions/checkout@11d5960a326750d5838078e36cf38b85af677262`, in P1, P3 and P7; the SHA equals the hive workflow's pin. P4a, P4b and P9 use no action.
- Every `${{ }}` sits in an `env:` or `with:` block; none is in `run:` or `name:`. P7's `with:` carries `${{ matrix.persist }}`, not a payload field. Payload fields used: `github.event.pull_request.base.sha`, `.head.sha`, `.number`, `github.event.action`, `github.event.review.state`, `github.event.workflow_run.head_sha`, `.event`, `.pull_requests[0].number`, all GitHub-written.
- No `secrets.*` reference; the only token is `github.token`, in P4b, P7's C1 step and P9.
- Nothing runs PR head code under `pull_request_target` or `workflow_run`: P1 and P7 check out the default branch (post 2025-12-08) and P7 only `git fetch`es `refs/pull/<n>/head` into the object store, with no checkout; P9 has no checkout and only reads reviews and the PR head through the API. P3 runs under `pull_request_review` and checks out the merge ref, which is the baseline the brief's P3 row asks for and the same exposure today's gate has. P4a and P4b run under `pull_request` on the probe author's own branch in a throwaway repo. Nothing stores a credential beyond the `persist-credentials: true` matrix leg in P7, which is the thing P7 measures.
- Safe to run on the throwaway repo as written. No U+2013 or U+2014 in any file; the Head Keeper is named by role only.

## 7. Blocking Issues (Must-Fix Only)

None.

## 8. Notes (not blocking)

- **N1, run counts.** Findings line 46 says "Runs listed by event: 70 `pull_request`, 30 `push`". The API's `total_count` today is 573 `pull_request` and 167 `push`; the forager's numbers are page sizes, not totals. The row's answer (zero `pull_request_review` runs, zero reviews on #108 to #177) is unaffected, but the sentence should say "on the first page" or drop the numbers.
- **N2, C3 recommendation.** The brief asks the findings to say whether the follow-up should build from `GITHUB_SHA` or refuse non-default bases; the note offers both. One sentence picking a default (refusing non-default bases is the smaller change and keeps the build at the payload base) would save the follow-up scout a question.
- **N3, C1 inference.** "No event is excepted" is an argument from silence in [WS], not a quoted sentence; the quoted sentences cover the per-job narrowing and the none-by-default rule. The P7 confirm step makes this a sound `yes` pending the probe; the row could say so in those words.
- **N4, C4 opt-out.** [DC] continues: "A repository can opt out of this restriction for a specific workflow or job by explicitly declaring a write-capable cache-mode." The template declares none, so the row holds, but the follow-up requirement (`cache: false`) is what keeps it true, and the proposed requirement list already carries it.
- **N5, P9 fork PRs.** `github.event.workflow_run.pull_requests` is empty for a fork PR, so P9's `PR` env would be blank; the forager scopes P9 to same-repo PRs, which is what the card runs, so this is a card note only.
- **N6, redirects.** [AS] and [PL] redirect to new paths (section 1 of this review); the cited URLs still resolve. The note could cite the destination paths so the links outlive the redirects.
- **N7, P4b PATCH step.** The Checks API accepts `conclusion` without `status` on update (it sets `completed`), and the step is `continue-on-error`, so a 403 or 422 is recorded, not fatal. Fine as a probe.

Verdict: pass
