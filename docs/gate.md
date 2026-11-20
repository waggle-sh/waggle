<!-- SPDX-License-Identifier: Apache-2.0 -->
# Waggle Gate: specification

This page says what `waggle gate verify` checks, what it does not check, and how to install it. Every statement of what the gate enforces cites the code or test that does it, as `file:line` at the commit this page was written against. A line number can drift; the function named by the cite is the authority.

Plain terms used here:

- A **task** is a unit of work with an id of the form `T-<id>` (for example `T-123`).
- A **claim** is a record that a task is being worked on. It is stored as a git ref `refs/waggle/claims/ticket-<n>` and holds the task, an owner, an expiry, a gate level, a list of directories (the **lease**) and a list of branch names.
- A **lease** is the list of directories a task's commits may change.
- A **protected path** is a path whose change fails the gate's check, unless an admin bypasses the check or, for paths your repo lists, a listed approver's approval and a re-run pass it (section 6). L11 is the limit on this.
- An **approver** is a GitHub login listed in `gate.approvers`.
- The **Head Keeper** is the first login in `gate.approvers` of the base config. Only the Head Keeper's approval passes a change under `.waggle/hive/` (section 6).
- A **required check** is a status that GitHub requires before a PR can merge. Here it is `verify-leases-and-tiers`.
- A **risk tier** is how much review a path needs: Tier 0 and 1 for small work, Tier 2 for steering files and logic, Tier 3 for control paths.
- A **light** or **full** claim is the gate level recorded in the claim. Its only effect on the gate is that a Tier 2 path fails under a `light` claim (`internal/comb/changecheck.go:97-100`). Separately, `waggle claim` refuses a `light` claim on a Tier 2 directory (`internal/comb/backend.go:468-474`); that is a refusal when the claim is written, not a gate check.

## 1. Inputs

- **Claim refs.** The gate reads `refs/waggle/claims/ticket-<n>` from the local clone (`cmd/waggle/gate_verify.go:234`, `cmd/waggle/gate_commits.go:330`). The workflow fetches them from `origin` before the gate runs (`internal/scaffold/templates/waggle-gate.yml.tmpl:29-30`, `gate/action.yml:37-41`). A lease and its branch list are part of the claim (`internal/comb/backend.go:88, 121-123`).
- **Base config.** `gate.protected_paths` and `gate.approvers` are read from `.waggle/control/config.yaml` at the PR base commit, never from the head. A broken base config is exit 3 (`cmd/waggle/gate_verify.go:265-303`; test `TestBaseConfigFaultsExit3`, `cmd/waggle/gate_bind_test.go:255`).
- **Range.** Every check compares `merge-base(base, head)` to head (`cmd/waggle/gate.go:51-56`). The clone needs full history (`fetch-depth: 0`); on a shallow clone with no merge-base the gate exits 3 (`cmd/waggle/gate_verify.go:229-233`).

## 2. Binding a commit to a task

A commit names its task in one of three ways: a subject that starts `T-<id>: `, a `Waggle-Ticket:` trailer, or a PR branch named `waggle/T-<id>` or `t-<id>` with an optional `-suffix` (`cmd/waggle/gate_commits.go:33-36, 51-64, 69-90`). The first source present wins. Two sources that name different tasks fail with `ticket conflict`, including a branch task that differs from a commit's subject or trailer. A commit that names none fails with `commit has no T-<id>: prefix, Waggle-Ticket: trailer or ticket branch` (`cmd/waggle/gate_commits.go:270-320`; tests `cmd/waggle/gate_bind_test.go`). A merge from the base branch with nothing to bind takes its first parent's task (`cmd/waggle/gate_commits.go:310-313`).

A commit whose subject starts `Head Keeper: ` is a maintainer commit: it always fails the lease check, so it needs an admin bypass, or a forged check (L11), to merge (`cmd/waggle/gate_commits.go:275-278`). The message is `commit <12 hex> "<subject>": maintainer commit (subject starts "Head Keeper: "): merge with an admin bypass`.

## 3. The claim's branch check

The task a commit names is author-chosen, so the claim's branch list is what ties the task to the PR.

- The gate reads the claim of whichever task the commit names. That claim must list the PR head branch, byte for byte. A fork branch matches only an entry written `<owner>/<repo>:<branch>` whose repository equals the event's head repository, ignoring case. Otherwise there is one `leases` failure with no path, `<id>: claim does not name this pull request's branch`, which no approval clears (`cmd/waggle/gate_commits.go:325-348, 353-379`; tests `cmd/waggle/branches_test.go`).
- A claim with no branch list fails every bound commit on every known head with `<id>: claim names no branch; renew it with waggle claim --branch`, which no approval clears. Renew the claim with `--branch` and re-run the check (`cmd/waggle/gate_commits.go:365-368`; tests `cmd/waggle/claim_branches_test.go:28`, `cmd/waggle/branches_test.go:202`).
- Under GitHub Actions the head is `--branch`, else `GITHUB_HEAD_REF`, else the event's head ref. A run with no readable pull request event (for example `pull_request_target`, `push` or `merge_group`) that passes no `--branch`, as the workflow and the Action pass none, treats the head as unknown and fails every bound commit (`<id>: pull request branch unknown`, or the does-not-name failure). A local run without `--branch` skips the branch check with one warning, `no pull request branch; claim branches not checked (pass --branch)` (`cmd/waggle/gate_verify.go:130-155`, `cmd/waggle/gate_approve.go:39-42`, `cmd/waggle/gate_commits.go:353-379`; tests `cmd/waggle/branches_test.go:215, 237-242`).
- A branch name over 255 bytes or with a control byte binds no task and is ignored with a warning (`cmd/waggle/gate_commits.go:51-54`, `cmd/waggle/gate_verify.go:140-147`).

## 4. The per-commit lease check

Every commit of the PR is checked, not only the final diff. Every path a commit changes must sit inside its task's claim directories, with exact case; a path that differs only in case fails, and so does a new symlink or submodule (`internal/comb/changecheck.go:79-91`). The finding for a stray path reads `<id>: path is outside the claim directories` (`internal/comb/changecheck.go:85`, `cmd/waggle/gate_commits.go:243`).

- A missing claim fails with `no claim for <id>; claims must stay until merge`; a malformed claim fails. An expired claim is accepted with a warning (`cmd/waggle/gate_commits.go:325-348`, warning at `:340-343`).
- The only merge commit accepted is an update from the base branch (`cmd/waggle/gate_commits.go:203-211, 254-263`).
- A PR over 500 commits, or a single commit changing over 5000 paths, fails with `PR too large for the gate; split it` (`cmd/waggle/gate_commits.go:16-23, 177-178, 198-199`).
- Head tree paths that are equal under case folding, or over 4096 bytes, fail (`cmd/waggle/gate_commits.go:19, 383-405`).

## 5. Risk tiers and protected paths

- **Hardcoded control paths** fail whatever the claim says, with `Tier 3 path; only an admin bypass merges it, approval does not` (limit: L11). They are `.waggle/` (except the `.waggle/hive/` carve-out below), `.github/workflows/`, `.gitlab-ci.yml`, `.devcontainer/`, and the Go dependency inputs `go.mod`, `go.sum`, `go.work`, `go.work.sum` and `vendor/` (`internal/comb/backend.go:29-39`, `internal/comb/changecheck.go:93-94`). No approval passes them.
- **Paths your repo lists** in `gate.protected_paths` fail with `protected path (listed in .waggle/control/config.yaml); needs an admin bypass to merge` (limit: L11), or pass when a listed approver's approval at the head commit applies and someone with Write re-runs the failed check (section 6) (`internal/comb/changecheck.go:95-96`).
- **Tier 2 paths** (`.waggle/hive/`, `CLAUDE.md`, `AGENTS.md`) fail under a `light` claim (`internal/comb/backend.go:49-53`, `internal/comb/changecheck.go:97-100`). `.waggle/hive/` is the one Tier 2 carve-out inside `.waggle/`. No claim can lease it (`internal/comb/backend.go:43, 458-462`), so every change there fails the lease check with `path is outside the claim directories`, unless the Head Keeper's approval at the head commit and a re-run pass it (section 6). The carve-out matches exact case: a path under `.waggle/HIVE/` is Tier 3, and no approval passes it (`cmd/waggle/gate_approve.go:126-131`).
- **`waggle claim` refuses** `.waggle/` directories, Tier 3 directories, Tier 2 directories under `light`, repo-listed paths, and a bad branch list: more than 4 entries, a duplicate, an entry over 255 bytes, an invalid fork repository in an `<owner>/<repo>:<branch>` entry, or a name that is not a valid printable-ASCII branch name (`internal/comb/backend.go:458-474, 487-544`; listed paths `internal/comb/git.go:601-605`; tests `cmd/waggle/claim_test.go`, `cmd/waggle/claim_branches_test.go`, `internal/comb/branches_test.go`).

These describe the gate's own check. A forged check (L11) can also merge such a change. A required review blocks that merge until a person approves.

## 6. Approvers

An approver is a listed GitHub login (ASCII case folded, up to 32 entries, no teams) whose approval counts: a latest counted review of APPROVED, or an approve comment (below), at exactly the head commit and made strictly after the last change to the base, with no listed approver at CHANGES_REQUESTED. Such an approval passes only repo-listed paths outside the hardcoded Tier 3 prefixes, plus hive paths when the approver is the Head Keeper (below): only `tier3` and `outside_lease` findings on those paths are dropped (`cmd/waggle/gate_approve.go:108-192, 222-310`, `cmd/waggle/gate_commits.go:229-245`, `internal/config/config.go:76-83, 207-212, 311`, `internal/forge/reviews.go`, `internal/forge/comments.go`; tests `cmd/waggle/gate_approve_test.go`, `cmd/waggle/gate_comment_test.go`).

GitHub does not let you approve your own pull request, so a listed approver who opened the PR approves it with a comment instead. Post a comment on the PR's conversation with this line on its own:

```
waggle-approve <the full 40-character head commit SHA>
```

The comment counts as that login's approval of that commit under the same rules as a review. The SHA must be the PR head exactly, in lowercase, so a push makes the comment stale and you post a new one for the new head. The comment's creation time must be after the last change to the base. Editing an older comment does not move that time, so a base change needs a new comment. A counted CHANGES_REQUESTED from any listed approver still blocks, and no comment clears it, including one from the approver who requested the changes. A comment last edited by another account does not count. Other text in the comment is fine, but a line that quotes, prefixes or follows the SHA does not match. If the gate cannot read the comments, it decides from reviews alone and warns `PR comments could not be read; only reviews were checked`. It never prints a comment's text. A comment does not run the gate, so after posting it someone with Write re-runs the failed check (below).

The Head Keeper is the first entry of `gate.approvers` in the base config. When the approval that passes the PR is the Head Keeper's, by review or by comment, under the same rules (exactly the head commit, after the last base change, no listed approver at CHANGES_REQUESTED), the gate also drops `outside_lease` and `tier3` findings on paths under the exact-case `.waggle/hive/` (`cmd/waggle/gate_approve.go:126-139, 289-309`, `cmd/waggle/gate_commits.go:229-234`; tests `TestHeadKeeperHivePaths`, `cmd/waggle/gate_keeper_test.go:44`, and `TestApproveCommentHivePaths`, `cmd/waggle/gate_comment_test.go:18`). It then prints the warning `hive paths approved by the Head Keeper <login> at <12 hex>`. Another listed approver's approval leaves those findings failing, with the warning `hive paths need an approving review or waggle-approve comment from the Head Keeper (the first entry of gate.approvers) at <12 hex>`. The rest of the gate still applies to a hive change:

- each commit must bind a task whose claim names the PR branch (sections 2 and 3);
- a `light` claim still fails a hive path;
- the decisions check (section 8) runs on its own, and no approval clears it;
- a push after the approval moves the head, so the approval or comment no longer counts;
- the Tier 3 control paths of section 5 still fail.

With no `gate.approvers` there is no Head Keeper, and every hive change fails as before (test `TestHeadKeeperInertWithoutApprovers`). A PR cannot change who the Head Keeper is, because the config is read from base.

An approval alone does not turn the check green. The review runs the gate, and that run passes, but the earlier failing `pull_request` run stays on the head and keeps blocking. Someone with Write then re-runs the failed run; a re-run reads reviews again and replaces its own result. This was shown with test workflows, not with the real gate. If reviews added other failed runs (a comment-only review also runs the gate), each should need its own re-run; that was not run. A later review run that fails blocks a mergeable PR with no re-run, shown with a comment-only review on a test workflow. A dismissal or a request for changes runs the gate again and should do the same; neither was read after a green check.

The approver check runs only under GitHub Actions on a `pull_request` or `pull_request_review` event, with `GITHUB_TOKEN` and `pull-requests: read`. Any failure leaves the change failing, with a warning under Actions and silently elsewhere (`cmd/waggle/gate_verify.go:130-132`, `cmd/waggle/gate_approve.go:39-92, 222-275`).

## 7. Trailers

The `Waggle-Ticket`, `Waggle-Risk-Tier` and `Waggle-Verdict` trailers in the last paragraph of the PR description are compared with the tasks the commits bind and the tier the changed paths need. A mismatch is a warning only and never changes the exit code (`cmd/waggle/gate_trailers.go:27-104`, `cmd/waggle/gate_approve.go:206-218`). The trailers reach `git log` only if your merge message includes the PR description.

## 8. The decisions log

`.waggle/hive/DECISIONS.md` accepts only pure appends of well-formed entries, judged on the net change from the merge-base to head. Intermediate commits are not inspected (`cmd/waggle/gate.go:81-129, 147-158`). No claim can lease `.waggle/hive/`, so an append passes the lease check only with the Head Keeper's approval at the head commit and a re-run (section 6), or else needs an admin merge. No approval clears this check: a change that is not a pure append of well-formed entries fails whoever approved (L9, L11).

## 9. Output and exit codes

Exit 0 pass, 1 check failed, 2 usage, 3 git or I/O error. No override flag exists; an override is an admin merge that bypasses the required check (`cmd/waggle/gate.go:22-28, 51-56`). Under GitHub Actions each finding is an `::error::` or `::warning::` annotation, otherwise a `<check>: FAIL: <message>` or `<check>: warning: <message>` line; a passing check prints `<check>: ok` (`cmd/waggle/gate_verify.go:65-94`). `verify` runs leases, tiers and decisions and exits with the worst code (`cmd/waggle/gate_verify.go:172-181`).

## 10. Limits and known bypasses

- **L1, who can use a claim.** A commit's task is whatever its subject, trailer or branch names, and the claim's branch list is the only tie to the PR. So anyone who can push to a branch a claim names can add commits under that claim (same-repo branches are writable by every repo writer unless protected). A claim with no list fails every bound commit with `<id>: claim names no branch; renew it with waggle claim --branch` until it is renewed and the check re-run (`cmd/waggle/gate_commits.go:365-368`). Only printable ASCII branch names can be listed (`internal/comb/backend.go:524-544`), so a PR head with a non-ASCII name never matches and always fails. A re-claim after expiry resets the list (`internal/dispatch/claim.go:58-60`). The PR-body `Waggle-Ticket` trailer only warns on a mismatch. Mitigation: protect claimed branches or keep one writer per branch, and have reviewers check that the named task is the one assigned.
- **L2, required check.** The gate binds only on PRs that must pass `verify-leases-and-tiers` on a branch that requires PRs. A merge that skips PRs skips the gate. GitHub documents rulesets for GitHub Team and GitHub Enterprise plans; check your plan's page for your repository ([about rulesets](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets)). The rules are "Require a pull request before merging" and "Require status checks to pass before merging" ([available rules](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets)).
- **L3, who writes leases.** Anyone who can push `refs/waggle/claims/*` can write a lease and its branch list, an agent running with your git credentials included; an agent that can run `waggle claim` can name its own branch. The gate does not check who wrote a claim or when (`cmd/waggle/gate_commits.go:325-348`). So a person or the orchestrator writes the claim, with its branch list, before the work starts and never from the agent's credentials. Contributors without push access to the base repository, fork PRs included, cannot write claims; a maintainer writes the claim for them with an `<owner>/<repo>:<branch>` entry, and the gate reads the base repository's claim refs for every PR (`internal/scaffold/templates/waggle-gate.yml.tmpl:29-30`, `gate/action.yml:37-41`).
- **L4, expired claims pass** with a warning (`cmd/waggle/gate_commits.go:340-343`) and keep their branch list until renewed or released, since the branch check still runs on them (`:344`). An expired claim with no list fails closed on a known head with the renew hint (`:365-368`).
- **L5, workflow self-edit.** A PR that edits the gate workflow runs its own copy under `pull_request`. Mitigation, not a fix: code-owner review on `.github/workflows/**`, which binds only with the ruleset's "Require review from Code Owners" on, or an organization ruleset that requires the workflow. The PR's copy runs either way, and it can forge the check (L11). Code-owner review makes a code owner review the change before that PR merges. It does not cover a workflow on another branch (L11). Since 8 December 2025, `pull_request_target` always runs the default branch's workflow file, so a PR cannot change the copy that runs under that event. The workflow does not use it, because the gate would then also need a separate path for reviews, and moving would not close L11. Under `pull_request_target` the gate fails every bound commit (`cmd/waggle/gate_approve.go:39-42`, `cmd/waggle/gate_verify.go:148-155`, `cmd/waggle/gate_commits.go:353-379`), and the workflow's job condition does not run it there at all (`internal/scaffold/templates/waggle-gate.yml.tmpl:16`).
- **L6, admin bypass** merges skip the gate: the gate has no override flag, and an override is an admin merge that bypasses the required check (`cmd/waggle/gate.go:51-56`).
- **L7, trailers** are author text and prove nothing; the gate only warns on them (`cmd/waggle/gate_trailers.go:23-27`).
- **L8, approvers.** An agent holding a listed approver's token can approve. One holding the Head Keeper's token can also pass hive changes, which steer every later agent. A renamed and re-registered login counts. Off Actions the approver check never runs (`cmd/waggle/gate_verify.go:130-132`, `cmd/waggle/gate_approve.go:236-238`).
- **L9, decisions log** is judged on net change only (`cmd/waggle/gate.go:147-158`). No claim can lease `.waggle/` (`internal/comb/backend.go:458-462`), so an append merges only with the Head Keeper's approval at the head commit and a re-run (section 6), with an admin merge, or with a forged check (L11). The approval never passes a change that is not a pure append.
- **L10, version skew.** A hand-written workflow pinned to an older Action that meets a newer key inside the `forge`, `leases` or `gate` section of the base config exits 3, because the parser rejects an unknown key in those sections with `unknown key` and the gate reports any config error as exit 3 (`internal/config/config.go:164, 185, 218`, `cmd/waggle/gate_verify.go:240-242, 160-169`). A newer value in a known key exits 3 the same way, through `validate` (`internal/config/config.go:294-298`, for example a future `forge.merge_strategy` value). An unknown top-level section is ignored, so a newer section does not break an older Action (`internal/config/config.go:138-224`, `internal/config/config_test.go:255-260`). A newer CLI writes claims with a branch list that an older Action ignores (an optional field of the claim, `internal/comb/backend.go:88, 121-123`). An older CLI that renews such a claim drops the list, as does any tool that writes a claim ref without `branches`, so every bound commit then fails with `<id>: claim names no branch; renew it with waggle claim --branch` until a newer build renews it and the check is re-run (`cmd/waggle/gate_commits.go:365-368`). A gate built before the Head Keeper rule (section 6) fails every hive change whoever approved, so a repo on the pinned Action gets the rule only after its pin moves past it. Run `init` and `upgrade` from a pushed or released build: an unpushed commit renders an Action ref that fails at job setup, and a module build pins by tag and warns (`internal/scaffold/init.go:159, 426-440`).
- **L11, the required check can be forged.** A workflow on any branch can declare `actions: write` and `checks: write`, delete the gate's failing run and post a passing check of the same name. Deleting removes the failing check run from the commit; a post alone does not clear a failure, and a workflow token cannot edit one. The PR then merges with no admin rights. That covers hive changes too: where the Head Keeper's approval and a re-run would pass one, a forged check passes it with neither. The PR's own job, another open PR's workflow and a workflow on a plain branch each reached a mergeable state against a test workflow using `pull_request_target`, and one such PR was merged. Against a test workflow with today's `pull_request` job, the plain-branch form ran and GitHub reported the PR mergeable; with several failing runs each would need deleting, which was not run. A token that could both delete Actions runs and write commit statuses, using no workflow file, deleted the run and posted a commit status of the required name, and the PR was mergeable on a rule not bound to an app. This was shown with test workflows, not with the real gate.

  Three things help, and none is complete alone. **Bind the check to GitHub Actions.** On a test rule this kept the status route blocked. It did not stop a workflow, whose check comes from GitHub Actions too. **Limit agent credentials** to Contents and Pull requests, with no Workflows, Actions or Commit statuses permission; a classic token with `repo` already reaches Actions and commit statuses. A token with Contents write alone was refused a new workflow file, but it pushed a copy of one already on another branch, and the copy ran with its own write token, so this holds only while no attack workflow exists on any branch. No test ran a token with Pull requests write added. **Require a review.** With one approval required, a forged pass left the PR blocked and a merge with no admin rights was refused. That keeps a person in the merge; it does not protect the check. An approver may see a forged green check, and a check can be forged after the approval, with no new push, so stale dismissal does not fire and an approval must not rely on the check still failing. That order was not run. Keep "Allow GitHub Actions to create and approve pull requests" off: GitHub documents that setting for this, and no test ran it on.

  `waggle doctor` does not check that the check is bound to an app: its required-check probe matches the check by name only (`internal/forge/probe_github.go:97-118, 140-144`).
- **L12, merge queues.** The workflow has no `merge_group` trigger (`internal/scaffold/templates/waggle-gate.yml.tmpl:5-9, 16`), and a gate run outside a PR event fails every bound commit (`cmd/waggle/gate_verify.go:130-155`, `cmd/waggle/gate_commits.go:353-379`). A repo that merges through GitHub's merge queue is unsupported and untested. `waggle doctor` does not detect a merge queue.

## 11. Config reference

`.waggle/control/config.yaml`, as `waggle init --gate-only` seeds it (`internal/scaffold/templates/gate-config.yaml.tmpl:9-17`):

- `waggle.pin`: the Waggle build the repo runs, a release tag or a 40-hex commit. Change it only through `waggle upgrade`.
- `forge.provider`: `github`. `forge.remote`: `origin`; claims are pushed to this remote and the gate fetches them from it.
- `gate.protected_paths`: comma separated repo-relative paths whose change fails the check unless a listed approver's approval and a re-run pass it (section 6), or an admin bypasses (L11).
- `gate.approvers`: comma separated GitHub logins, without `@`.

The gate reads `gate.protected_paths` and `gate.approvers` from the config at the PR base, never from the head tree (`cmd/waggle/gate_verify.go:265-303`). A key the running build does not know inside the `forge`, `leases` or `gate` section makes the parse fail, and the gate exits 3 (`internal/config/config.go:164, 185, 218`; L10). Tests cover an unknown key under `leases` and under `forge` (`internal/config/config_test.go:180-181`); no test covers an unknown key under `gate`. A top-level section the build does not know is ignored (`internal/config/config_test.go:255-260`). `forge.merge_strategy` also exists (`up_to_date` by default); the value `native_queue` only makes `waggle doctor` skip the up-to-date check and print an informational `merge-queue` finding marked unknown (`internal/forge/probe_github.go:66-73`, `internal/config/config.go:31, 127`). It does not make the gate run in a merge queue (L12).

## 12. Install and `waggle doctor`

`waggle init --gate-only` writes `.github/workflows/waggle-gate.yml`, `.waggle/control/config.yaml` and `.waggle/hive/DECISIONS.md` (`internal/scaffold/init.go:391-397`). It refuses any forge but `github` (`:402-404`) and a build that has neither a clean commit stamp (a `vcs.revision` with `vcs.modified` false) nor a release tag, with `this build has no commit or release tag to pin the gate Action to` (`:105-156, 162, 426-429`); it does not check that the commit was pushed (L10). It prints `Gate initialized. Next: commit .github/workflows/waggle-gate.yml, .waggle/control/config.yaml and .waggle/hive/DECISIONS.md, then require the check verify-leases-and-tiers on your default branch.` (`cmd/waggle/main.go:58-65`).

Build `waggle` from a clean clone of a pushed commit, checked out at `<commit-sha>`, with `go build -o <path outside the clone>/waggle ./cmd/waggle`. Do not build into the clone root: the untracked binary makes the next build stamp a modified tree, and `init --gate-only` then refuses (`internal/scaffold/init.go:426-429`).

The install PR changes `.github/workflows/` and `.waggle/`, so it fails its own gate. If a rule already requires the check, it needs an admin merge (section 5).

`waggle doctor [--repo <dir>] [--no-write]` (`cmd/waggle/doctor.go:32-34`) reports three required findings: `claim-refs` (can this clone push claim refs; it pushes two temporary refs under `refs/waggle/probe/` in one atomic push and deletes them, unless `--no-write`; `internal/forge/probe.go:38-42`), `required-gate` (a rule on the default branch requires `verify-leases-and-tiers`; `internal/forge/probe_github.go:155-172`) and `up-to-date` (branches must be up to date; `:174-191`). Exit 0 when every required finding is ok or skipped, 1 when one is missing or unknown, 2 usage, 3 when the config, forge or default branch is unreadable (`cmd/waggle/doctor.go:27-30, 84-89`). With no `forge.remote` it prints one line and exits 0 (`:64-67`). With no finding missing it closes with `No required setting above was found missing.` and a line that the required check can still be forged (L11). `Enforcement: enforced` means only that `required-gate` and the merge strategy's rule (`up-to-date` by default) are ok. It does not read `claim-refs`. The command does not check that PRs are required, required reviews, code-owner review, or which app may post the check (L11).

## 13. The calling workflow

This is the merged template with the two pin values replaced. `<commit-sha>` is the full commit of the Waggle build you installed from (for a release tag, `git rev-parse vX.Y.Z^{commit}`, since release tags are annotated; or `git ls-remote <repo> 'refs/tags/vX.Y.Z^{}'`). Prefer `waggle init --gate-only`, which writes this file for you. Source: `internal/scaffold/templates/waggle-gate.yml.tmpl`.

```yaml
# SPDX-License-Identifier: MIT-0
# waggle:managed: installed by waggle init; removed by waggle uninstall. Do not edit by hand.
name: Waggle Gatekeeper

on:
  pull_request:
    types: [opened, synchronize, reopened, labeled, edited]
  pull_request_review:
    types: [submitted, dismissed]

permissions:
  contents: read

jobs:
  verify-leases-and-tiers:
    if: github.event_name == 'pull_request' || github.event_name == 'pull_request_review'
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: read
    concurrency:
      group: waggle-gate-${{ github.event.pull_request.number }}-${{ github.event.pull_request.head.sha }}
      cancel-in-progress: false
    steps:
      - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262 # v4.4.0
        with:
          fetch-depth: 0

      - name: Fetch claim refs
        run: git fetch origin 'refs/waggle/claims/*:refs/waggle/claims/*'

      - name: Find gate source on base
        id: src
        env:
          BASE_SHA: ${{ github.event.pull_request.base.sha }}
        run: |
          if ! git cat-file -e "$BASE_SHA^{commit}" 2>/dev/null; then
            echo "::error::base $BASE_SHA is not in the clone"
            exit 1
          fi
          if git cat-file -e "$BASE_SHA:cmd/waggle/gate.go" 2>/dev/null; then
            echo "present=true" >> "$GITHUB_OUTPUT"
          else
            echo "present=false" >> "$GITHUB_OUTPUT"
            echo "::notice::waggle gate source not on base; running the pinned gate Action"
          fi

      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5.6.0
        if: steps.src.outputs.present == 'true'
        with:
          go-version-file: go.mod

      - name: Waggle gate
        if: steps.src.outputs.present == 'true'
        env:
          BASE_SHA: ${{ github.event.pull_request.base.sha }}
          HEAD_SHA: ${{ github.event.pull_request.head.sha }}
          GITHUB_TOKEN: ${{ github.token }}
        run: |
          # The checker is built from the base revision, never from the PR head.
          # Repos without the gate source on base run the pinned gate Action instead.
          git worktree add --detach "$RUNNER_TEMP/gate-src" "$BASE_SHA"
          (cd "$RUNNER_TEMP/gate-src" && go build -o "$RUNNER_TEMP/waggle" ./cmd/waggle)
          "$RUNNER_TEMP/waggle" gate verify --base "$BASE_SHA" --head "$HEAD_SHA"

      - name: Waggle gate (Action)
        if: steps.src.outputs.present == 'false'
        uses: waggle-sh/waggle/gate@<commit-sha> # waggle.pin <commit-sha>
        with:
          base-sha: ${{ github.event.pull_request.base.sha }}
          head-sha: ${{ github.event.pull_request.head.sha }}
          token: ${{ github.token }}
```

The Action takes the inputs `base-sha`, `head-sha` and `token`, defaulting to the event's base SHA, head SHA and `github.token`. It builds the gate from its own pinned tree with the module proxy off, fetches claim refs, and runs `gate verify --base --head` with no `--branch` (`gate/action.yml:5-51`).
