Risk-Tier: 3

**Intent Summary:** PR #139 (head 52fa51d, base 0d102ca) moves the hive pin in `.waggle/control/config.yaml` from 97127fc to 0d102ca, applies the `.github/workflows/waggle-gate.yml` that `waggle upgrade --apply` renders from a 0d102ca build, and appends D-320. This is T-042 section 9 Step 1, and it has the same shape as D-269.

The workflow changes all come from T-036 (f3bff0b) and T-040 (5c27ffb):
- the `issue_comment` trigger and the `relay-claim` stub job are gone
- the inline tier-annotation step is gone
- checkout and setup-go are pinned by SHA
- one step is renamed
- a `waggle-sh/waggle/gate@0d102ca` fallback step is added, which runs only when the base lacks `cmd/waggle/gate.go`

Blast radius is the required `verify-leases-and-tiers` check on every PR to main. For this hive, the verify path is unchanged: build from base, then `gate verify`.

**Evidence**
- **Render reproduced byte for byte.** I built `cmd/waggle` in a detached worktree at 0d102ca (`vcs.revision=0d102ca...`, `vcs.modified=false`). I ran `waggle upgrade`, then `waggle upgrade --apply`, on a git copy of the origin/main tree.
  - The dry run reports `[rewrite]` for the workflow and `[same]` for every other managed file.
  - After `--apply`, `diff` shows the workflow and config.yaml identical to the PR branch.
  - Only those two tracked files changed, plus the gitignored `.claude/agents` shims.
- **Triggers.** `pull_request` (opened, synchronize, reopened, labeled, edited) and `pull_request_review` (submitted, dismissed) are unchanged; only `issue_comment` is removed. The top-level `permissions: contents: read` is unchanged. The job keeps its `if` on pull_request or pull_request_review, its job-level `contents: read` and `pull-requests: read`, and its concurrency group (PR number plus head SHA, no cancel).
- **Required checks.** The `main` ruleset (active) requires `product-ci` and `verify-leases-and-tiers`. The job id is unchanged. `relay-claim` was never a required check, so removing it cannot strand the ruleset.
- **SHA pins.** The GitHub API resolves the version comments to the pinned SHAs: `actions/checkout` tag v4.4.0 is 11d5960a326750d5838078e36cf38b85af677262, and `actions/setup-go` tag v5.6.0 is 40f1582b2485089dde7abd97c1529aa768e1baff.
- **Gate Action.** `gate/action.yml` exists at 0d102ca. Its inputs are exactly `base-sha`, `head-sha` and `token`, which matches the `with:` block. It builds from `$GITHUB_ACTION_PATH/..` (its own pinned tree) with GOPROXY off and GOTOOLCHAIN local, using Go 1.23, which matches `go 1.23` in go.mod at 0d102ca. The step runs only on `steps.src.outputs.present == 'false'`. Our base always carries `cmd/waggle/gate.go`, so the Action never runs here.
- **Live run on this PR.** The PR's own gate run confirms it:
  - "Find gate source on base" took the `present=true` path.
  - "Waggle gate" built from base 0d102ca and ran `gate verify`.
  - The run ended with `tiers: Tier 3 path; only an admin bypass merges it` and `decisions: ok, appended D-320`.

  The FAILURE status is the expected Tier 3 result: only an admin bypass merges it.
- **Other checks.**
  - config.yaml: the only change is the pin line.
  - DECISIONS.md: pure append, ends with a newline, and the gate's decisions check passes.
  - No em or en dashes in the added lines.
  - `scan-hive.sh origin/main HEAD` exits 2, meaning no privacy or dash hit; 2 is expected because the PR changes files outside `.waggle/hive/`.

**Blocking Issues (Must-Fix Only)**

None.

**Notes**
1. **Removing the tier-notice step loses nothing that was enforced.** `.github/workflows/waggle-gate.yml` (removed lines at origin/main around L43-73) only printed `::notice` and `::warning` annotations and never failed the job. `gate verify` still enforces tiers, as this run's `tiers:` errors show. What is lost is the per-file annotation in the PR UI, and that is cosmetic.
2. **Removing `relay-claim` drops nothing functional and shrinks the attack surface.** It was an echo-only stub with a commented-out invocation, but it ran on any `/waggle claim` comment with `contents`, `pull-requests` and `issues` write permissions. T-036 removed the relay feature itself.
3. **D-320 is accurate but incomplete on where the workflow changes come from.** At `.waggle/hive/DECISIONS.md` (D-320, Keeper choice) it says 0d102ca "carries T-039 #128 and T-040 #135". It does, but the workflow render comes from T-036 (relay removal, gate Action) plus the T-040 wording. The pin also brings in T-036 through T-038b and other merges since 97127fc. The list of changes in D-320 is correct. This needs no fix.
4. **The commit message fails the gate's lease check.** The gate's leases check errors on commit 52fa51d because it has no `T-<id>:` prefix, `Waggle-Ticket` trailer or ticket branch, and warns that the `Waggle-Verdict` trailer is missing. Since the Tier 3 admin bypass is the merge path anyway, this does not block. It is the same class of result a D-269-style Head Keeper pin commit would get.
5. **`T-042.md` predicted `[same]` but this pin move rewrites the workflow.** Section 9 Step 1 of `.waggle/hive/tasks/T-042.md` says the workflow "usually reports `[same]`". The step explicitly does not promise either outcome, so the rewrite is consistent with the ticket.
6. **The fallback Action fetches claim refs differently.** The composite Action fetches with `--prune '+refs/...'`, while the inline workflow step uses a non-forced fetch on a fresh clone. The inline step's behavior is unchanged from main, so this is not a regression.

Relevant files: `.github/workflows/waggle-gate.yml`, `.waggle/control/config.yaml`, `.waggle/hive/DECISIONS.md`, and `gate/action.yml` and `internal/scaffold/templates/waggle-gate.yml.tmpl` at 0d102ca.

Verdict: pass
