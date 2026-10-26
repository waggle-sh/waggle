<!-- SPDX-License-Identifier: Apache-2.0 -->
# Waggle Gate: specification

This page says what `waggle gate verify` checks, what it does not check, and how to install it. Every statement of what the gate enforces cites the code or test that does it, as `file:line` at the commit this page was written against. A line number can drift; the function named by the cite is the authority.

Plain terms used here:

- A **task** is a unit of work with an id of the form `T-<id>` (for example `T-123`).
- A **claim** is a record that a task is being worked on. It is stored as a git ref `refs/waggle/claims/ticket-<n>` and holds the task, an owner, an expiry, a gate level, a list of directories (the **lease**) and a list of branch names.
- A **lease** is the list of directories a task's commits may change.
- A **protected path** is a path that changes only with an admin bypass, or, for paths your repo lists, a listed approver's approval.
- An **approver** is a GitHub login listed in `gate.approvers`.
- A **required check** is a status that GitHub requires before a PR can merge. Here it is `verify-leases-and-tiers`.
- A **risk tier** is how much review a path needs: Tier 0 and 1 for small work, Tier 2 for steering files and logic, Tier 3 for control paths.
- A **light** or **full** claim is the gate level recorded in the claim. Its only effect on the gate is that a Tier 2 path fails under a `light` claim (`internal/comb/changecheck.go:97-100`). Separately, `waggle claim` refuses a `light` claim on a Tier 2 directory (`internal/comb/backend.go:468-474`); that is a refusal when the claim is written, not a gate check.

## 1. Inputs

- **Claim refs.** The gate reads `refs/waggle/claims/ticket-<n>` from the local clone (`cmd/waggle/gate_verify.go:230`, `cmd/waggle/gate_commits.go:320`). The workflow fetches them from `origin` before the gate runs (`internal/scaffold/templates/waggle-gate.yml.tmpl:29-30`, `gate/action.yml:37-41`). A lease and its branch list are part of the claim (`internal/comb/backend.go:88, 121-123`).
- **Base config.** `gate.protected_paths` and `gate.approvers` are read from `.waggle/control/config.yaml` at the PR base commit, never from the head. A broken base config is exit 3 (`cmd/waggle/gate_verify.go:258-296`; test `TestBaseConfigFaultsExit3`, `cmd/waggle/gate_bind_test.go:255`).
- **Range.** Every check compares `merge-base(base, head)` to head (`cmd/waggle/gate.go:51-56`). The clone needs full history (`fetch-depth: 0`); on a shallow clone with no merge-base the gate exits 3 (`cmd/waggle/gate_verify.go:225-229`).

## 2. Binding a commit to a task

A commit names its task in one of three ways: a subject that starts `T-<id>: `, a `Waggle-Ticket:` trailer, or a PR branch named `waggle/T-<id>` or `t-<id>` with an optional `-suffix` (`cmd/waggle/gate_commits.go:33-36, 51-64, 69-90`). The first source present wins. Two sources that name different tasks fail with `ticket conflict`, including a branch task that differs from a commit's subject or trailer. A commit that names none fails with `commit has no T-<id>: prefix, Waggle-Ticket: trailer or ticket branch` (`cmd/waggle/gate_commits.go:260-310`; tests `cmd/waggle/gate_bind_test.go`). A merge from the base branch with nothing to bind takes its first parent's task (`cmd/waggle/gate_commits.go:300-303`).

A commit whose subject starts `Head Keeper: ` is a maintainer commit: it always fails the lease check and merges only with an admin bypass (`cmd/waggle/gate_commits.go:265-268`). The message is `commit <12 hex> "<subject>": maintainer commit (subject starts "Head Keeper: "): merge with an admin bypass`.

## 3. The claim's branch check

The task a commit names is author-chosen, so the claim's branch list is what ties the task to the PR.

- The gate reads the claim of whichever task the commit names. That claim must list the PR head branch, byte for byte. A fork branch matches only an entry written `<owner>/<repo>:<branch>` whose repository equals the event's head repository, ignoring case. Otherwise there is one `leases` failure with no path, `<id>: claim does not name this pull request's branch`, which no approval clears (`cmd/waggle/gate_commits.go:315-338, 343-369`; tests `cmd/waggle/branches_test.go`).
- A claim with no branch list fails every bound commit on every known head with `<id>: claim names no branch; renew it with waggle claim --branch`, which no approval clears. Renew the claim with `--branch` and re-run the check (`cmd/waggle/gate_commits.go:355-358`; tests `cmd/waggle/claim_branches_test.go:28`, `cmd/waggle/branches_test.go:202`).
- Under GitHub Actions the head is `--branch`, else `GITHUB_HEAD_REF`, else the event's head ref. A run with no readable pull request event (for example `pull_request_target`, `push` or `merge_group`) that passes no `--branch`, as the workflow and the Action pass none, treats the head as unknown and fails every bound commit (`<id>: pull request branch unknown`, or the does-not-name failure). A local run without `--branch` skips the branch check with one warning, `no pull request branch; claim branches not checked (pass --branch)` (`cmd/waggle/gate_verify.go:130-155`, `cmd/waggle/gate_approve.go:39-42`, `cmd/waggle/gate_commits.go:343-369`; tests `cmd/waggle/branches_test.go:215, 237-242`).
- A branch name over 255 bytes or with a control byte binds no task and is ignored with a warning (`cmd/waggle/gate_commits.go:51-54`, `cmd/waggle/gate_verify.go:140-147`).

## 4. The per-commit lease check

Every commit of the PR is checked, not only the final diff. Every path a commit changes must sit inside its task's claim directories, with exact case; a path that differs only in case fails, and so does a new symlink or submodule (`internal/comb/changecheck.go:79-91`). The finding for a stray path reads `<id>: path is outside the claim directories` (`internal/comb/changecheck.go:85`, `cmd/waggle/gate_commits.go:233`).

- A missing claim fails with `no claim for <id>; claims must stay until merge`; a malformed claim fails. An expired claim is accepted with a warning (`cmd/waggle/gate_commits.go:315-338`, warning at `:330-333`).
- The only merge commit accepted is an update from the base branch (`cmd/waggle/gate_commits.go:199-207, 244-253`).
- A PR over 500 commits, or a single commit changing over 5000 paths, fails with `PR too large for the gate; split it` (`cmd/waggle/gate_commits.go:16-23, 173-174, 194-195`).
- Head tree paths that are equal under case folding, or over 4096 bytes, fail (`cmd/waggle/gate_commits.go:19, 373-395`).

## 5. Risk tiers and protected paths

- **Hardcoded control paths** fail whatever the claim says, with `Tier 3 path; only an admin bypass merges it, approval does not`. They are `.waggle/` (except the `.waggle/hive/` carve-out below), `.github/workflows/`, `.gitlab-ci.yml`, `.devcontainer/`, and the Go dependency inputs `go.mod`, `go.sum`, `go.work`, `go.work.sum` and `vendor/` (`internal/comb/backend.go:29-39`, `internal/comb/changecheck.go:93-94`). No approval passes them.
- **Paths your repo lists** in `gate.protected_paths` fail with `protected path (listed in .waggle/control/config.yaml); needs an admin bypass to merge`, or pass when a listed approver's approval at the head commit applies (section 6) (`internal/comb/changecheck.go:95-96`).
- **Tier 2 paths** (`.waggle/hive/`, `CLAUDE.md`, `AGENTS.md`) fail under a `light` claim (`internal/comb/backend.go:49-53`, `internal/comb/changecheck.go:97-100`). `.waggle/hive/` is the one Tier 2 carve-out inside `.waggle/`, yet it is never leasable (`internal/comb/backend.go:43, 458-462`) and never approvable (`cmd/waggle/gate_approve.go:107-121`).
- **`waggle claim` refuses** `.waggle/` directories, Tier 3 directories, Tier 2 directories under `light`, repo-listed paths, and a bad branch list: more than 4 entries, a duplicate, an entry over 255 bytes, an invalid fork repository in an `<owner>/<repo>:<branch>` entry, or a name that is not a valid printable-ASCII branch name (`internal/comb/backend.go:458-474, 487-544`; listed paths `internal/comb/git.go:601-605`; tests `cmd/waggle/claim_test.go`, `cmd/waggle/claim_branches_test.go`, `internal/comb/branches_test.go`).

## 6. Approvers

An approver is a listed GitHub login (ASCII case folded, up to 32 entries, no teams) whose latest counted review is APPROVED at exactly the head commit and submitted strictly after the last change to the base, with no listed approver at CHANGES_REQUESTED. Such an approval passes only repo-listed paths outside the hardcoded Tier 3 prefixes: only `tier3` and `outside_lease` findings on those paths are dropped (`cmd/waggle/gate_approve.go:107-158, 189-242`, `cmd/waggle/gate_commits.go:222-228`, `internal/config/config.go:76-83, 207-212, 311`, `internal/forge/reviews.go`; tests `cmd/waggle/gate_approve_test.go`).

The approver check runs only under GitHub Actions on a `pull_request` or `pull_request_review` event, with `GITHUB_TOKEN` and `pull-requests: read`. Any failure leaves the change failing, with a warning under Actions and silently elsewhere (`cmd/waggle/gate_verify.go:130-132`, `cmd/waggle/gate_approve.go:39-92, 189-226`).

## 7. Trailers

The `Waggle-Ticket`, `Waggle-Risk-Tier` and `Waggle-Verdict` trailers in the last paragraph of the PR description are compared with the tasks the commits bind and the tier the changed paths need. A mismatch is a warning only and never changes the exit code (`cmd/waggle/gate_trailers.go:27-104`, `cmd/waggle/gate_approve.go:175-187`). The trailers reach `git log` only if your merge message includes the PR description.

## 8. The decisions log

`.waggle/hive/DECISIONS.md` accepts only pure appends of well-formed entries, judged on the net change from the merge-base to head. Intermediate commits are not inspected (`cmd/waggle/gate.go:81-129, 147-158`). Because `.waggle/` can never be leased, every append also needs an admin merge (L9).

## 9. Output and exit codes

Exit 0 pass, 1 check failed, 2 usage, 3 git or I/O error. No override flag exists; an override is an admin merge that bypasses the required check (`cmd/waggle/gate.go:22-28, 51-56`). Under GitHub Actions each finding is an `::error::` or `::warning::` annotation, otherwise a `<check>: FAIL: <message>` or `<check>: warning: <message>` line; a passing check prints `<check>: ok` (`cmd/waggle/gate_verify.go:65-94`). `verify` runs leases, tiers and decisions and exits with the worst code (`cmd/waggle/gate_verify.go:172-181`).

## 10. Limits and known bypasses

- **L1, who can use a claim.** A commit's task is whatever its subject, trailer or branch names, and the claim's branch list is the only tie to the PR. So anyone who can push to a branch a claim names can add commits under that claim (same-repo branches are writable by every repo writer unless protected). A claim with no list fails every bound commit with `<id>: claim names no branch; renew it with waggle claim --branch` until it is renewed and the check re-run (`cmd/waggle/gate_commits.go:355-358`). Only printable ASCII branch names can be listed (`internal/comb/backend.go:524-544`), so a PR head with a non-ASCII name never matches and always fails. A re-claim after expiry resets the list (`internal/dispatch/claim.go:58-60`). The PR-body `Waggle-Ticket` trailer only warns on a mismatch. Mitigation: protect claimed branches or keep one writer per branch, and have reviewers check that the named task is the one assigned.
- **L2, required check.** The gate binds only on PRs that must pass `verify-leases-and-tiers` on a branch that requires PRs. A merge that skips PRs skips the gate. GitHub documents rulesets for GitHub Team and GitHub Enterprise plans; check your plan's page for your repository ([about rulesets](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets)). The rules are "Require a pull request before merging" and "Require status checks to pass before merging" ([available rules](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets)).
- **L3, who writes leases.** Anyone who can push `refs/waggle/claims/*` can write a lease and its branch list, an agent running with your git credentials included; an agent that can run `waggle claim` can name its own branch. The gate does not check who wrote a claim or when (`cmd/waggle/gate_commits.go:315-338`). So a person or the orchestrator writes the claim, with its branch list, before the work starts and never from the agent's credentials. Contributors without push access to the base repository, fork PRs included, cannot write claims; a maintainer writes the claim for them with an `<owner>/<repo>:<branch>` entry, and the gate reads the base repository's claim refs for every PR (`internal/scaffold/templates/waggle-gate.yml.tmpl:29-30`, `gate/action.yml:37-41`).
- **L4, expired claims pass** with a warning (`cmd/waggle/gate_commits.go:330-333`) and keep their branch list until renewed or released, since the branch check still runs on them (`:334`). An expired claim with no list fails closed on a known head with the renew hint (`:355-358`).
- **L5, workflow self-edit.** A PR that edits the gate workflow runs its own copy under `pull_request`. Mitigation, not a fix: code-owner review on `.github/workflows/**`, which binds only with the ruleset's "Require review from Code Owners" on, or an organization ruleset that requires the workflow. Under `pull_request_target` the gate fails every bound commit (`cmd/waggle/gate_approve.go:39-42`, `cmd/waggle/gate_verify.go:148-155`, `cmd/waggle/gate_commits.go:343-369`), and the workflow's job condition does not run it there at all (`internal/scaffold/templates/waggle-gate.yml.tmpl:16`).
- **L6, admin bypass** merges skip the gate: the gate has no override flag, and an override is an admin merge that bypasses the required check (`cmd/waggle/gate.go:51-56`).
- **L7, trailers** are author text and prove nothing; the gate only warns on them (`cmd/waggle/gate_trailers.go:23-27`).
- **L8, approvers.** An agent holding a listed approver's token can approve. A renamed and re-registered login counts. Off Actions the approver check never runs (`cmd/waggle/gate_verify.go:130-132`, `cmd/waggle/gate_approve.go:203-205`).
- **L9, decisions log** is judged on net change only, and every append needs an admin merge because `.waggle/` cannot be leased (`cmd/waggle/gate.go:147-158`, `internal/comb/backend.go:458-462`).
- **L10, version skew.** A hand-written workflow pinned to an older Action that meets a newer config key exits 3. A newer CLI writes claims with a branch list that an older Action ignores (an optional field of the claim, `internal/comb/backend.go:88, 121-123`). An older CLI that renews such a claim drops the list, as does any tool that writes a claim ref without `branches`, so every bound commit then fails with `<id>: claim names no branch; renew it with waggle claim --branch` until a newer build renews it and the check is re-run (`cmd/waggle/gate_commits.go:355-358`). Run `init` and `upgrade` from a pushed or released build: an unpushed commit renders an Action ref that fails at job setup, and a module build pins by tag and warns (`internal/scaffold/init.go:159, 426-440`).
- **L11, required-context spoofing.** Any app or status with write access can post a status named like the required check unless the rule binds the check to an integration. `waggle doctor` does not check that binding: its required-check probe matches the check by name only (`internal/forge/probe_github.go:99-115, 138-143`).
- **L12, merge queues.** The workflow has no `merge_group` trigger (`internal/scaffold/templates/waggle-gate.yml.tmpl:5-9, 16`), and a gate run outside a PR event fails every bound commit (`cmd/waggle/gate_verify.go:130-155`, `cmd/waggle/gate_commits.go:343-369`). A repo that merges through GitHub's merge queue is unsupported and untested. `waggle doctor` does not detect a merge queue.

## 11. Config reference

`.waggle/control/config.yaml`, as `waggle init --gate-only` seeds it (`internal/scaffold/templates/gate-config.yaml.tmpl:9-17`):

- `waggle.pin`: the Waggle build the repo runs, a release tag or a 40-hex commit. Change it only through `waggle upgrade`.
- `forge.provider`: `github`. `forge.remote`: `origin`; claims are pushed to this remote and the gate fetches them from it.
- `gate.protected_paths`: comma separated repo-relative paths that change only with a listed approver's approval or an admin bypass.
- `gate.approvers`: comma separated GitHub logins, without `@`.

A config key the running build does not know exits 3 (L10). `forge.merge_strategy` also exists (`up_to_date` by default); the value `native_queue` only makes `waggle doctor` skip the up-to-date check and print an informational `merge-queue` finding marked unknown (`internal/forge/probe_github.go:66-73`, `internal/config/config.go:31, 127`). It does not make the gate run in a merge queue (L12).

## 12. Install and `waggle doctor`

`waggle init --gate-only` writes `.github/workflows/waggle-gate.yml`, `.waggle/control/config.yaml` and `.waggle/hive/DECISIONS.md` (`internal/scaffold/init.go:391-397`). It refuses any forge but `github` (`:402-404`) and a build that has neither a clean commit stamp (a `vcs.revision` with `vcs.modified` false) nor a release tag, with `this build has no commit or release tag to pin the gate Action to` (`:105-156, 162, 426-429`); it does not check that the commit was pushed (L10). It prints `Gate initialized. Next: commit .github/workflows/waggle-gate.yml, .waggle/control/config.yaml and .waggle/hive/DECISIONS.md, then require the check verify-leases-and-tiers on your default branch.` (`cmd/waggle/main.go:58-65`).

Build `waggle` from a clean clone of a pushed commit, checked out at `<commit-sha>`, with `go build -o <path outside the clone>/waggle ./cmd/waggle`. Do not build into the clone root: the untracked binary makes the next build stamp a modified tree, and `init --gate-only` then refuses (`internal/scaffold/init.go:426-429`).

The install PR changes `.github/workflows/` and `.waggle/`, so it fails its own gate and needs an admin merge (section 5).

`waggle doctor [--repo <dir>] [--no-write]` (`cmd/waggle/doctor.go:32-34`) reports three required findings: `claim-refs` (can this clone push claim refs; it pushes two temporary refs under `refs/waggle/probe/` in one atomic push and deletes them, unless `--no-write`; `internal/forge/probe.go:36-41`), `required-gate` (a rule on the default branch requires `verify-leases-and-tiers`; `internal/forge/probe_github.go:153-170`) and `up-to-date` (branches must be up to date; `:172-189`). Exit 0 when every required finding is ok or skipped, 1 when one is missing or unknown, 2 usage, 3 when the config, forge or default branch is unreadable (`cmd/waggle/doctor.go:25-28, 76-81`). With no `forge.remote` it prints one line and exits 0 (`:56-59`). It does not check that PRs are required, code-owner review, or which app may post the check (L11).

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
