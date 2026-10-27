# Guard review: T-025 phase D, `dco` job in product CI (second guard, fable)

Risk-Tier: 3

- Branch: `t-025d`, head `36fbe13`, base `origin/main` `bae5b28`, PR #157
- Diff: `.github/workflows/ci.yml`, 25 added lines, nothing else
- Spec: `.waggle/hive/tasks/T-025.md` section 7 and acceptance 7; D-131, D-098, D-132; threat bar D-203

## Intent summary

The commit appends a `dco` job to the product CI workflow. On `pull_request` events it checks out the merge ref with full history and no persisted credentials, then walks `base.sha..head.sha` and fails if any commit (merge commits included) lacks a `Signed-off-by` trailer whose email matches the commit author's email, case-insensitively. Event values reach the script through `env`, never inline in `run`. The job is skipped on push to `main`. The existing `product-ci` job is untouched. Blast radius: once the Head Keeper adds `dco` to the `main` ruleset (the spec's next step, not in this diff), every PR into `main` needs the trailer on every commit, including the queen's records batches and Head Keeper commits. The job holds only `contents: read` and writes nothing.

## What I checked

- The added block is byte for byte the draft in T-025 section 7 (`diff` of the extracted draft against the extracted diff: identical).
- YAML parses; `jobs` is `[product-ci, dco]`; the `if` expression is a bare string, which Actions evaluates as an expression.
- No `${{ }}` inside `run`. The two event values are passed via `env` as `BASE` and `HEAD`. Workflow-level `permissions: contents: read` is the only grant; `persist-credentials: false` on checkout.
- Dry-ran the exact loop under `bash -e` (the default `run` shell) against real commits: `79ae6ba` (signed) passes, `950329e` (unsigned) fails with the expected `::error::` line, the PR's own range `bae5b28..36fbe13` passes.
- Scratch repo cases, all with the expected result: uppercase email in the trailer passes (`grep -i`); lowercase `signed-off-by:` key passes (git matches trailer keys case-insensitively); several trailers with the author's last passes; a commit whose committer differs from the author but carries the author's trailer passes (the rebase-by-maintainer case); a commit with only the committer's trailer fails; an unsigned merge commit inside the range fails and `git merge --signoff` passes (spec: merge commits are checked like any other); a subject containing `::warning::`, `$(...)` and backticks is echoed inertly because the line already starts with `::error::` and `%s` is single-line.
- Running with `bash -eo pipefail` (what an explicit `shell: bash` would give) produces the same results, so a later `shell:` addition would not change behavior.
- Push to `main`: skipped by the `if`. Fork PRs: with `fetch-depth: 0` the merge commit is fetched by sha and its second parent is `head.sha`, so the range resolves; the fork token is read-only anyway. `base.sha` is on `refs/heads/main` history unless `main` was force-pushed (see note 1).
- CONTRIBUTING section 6 states the trailer must match the commit author, so binding to `%ae` rather than committer is the documented rule, not a drift.

## Findings

### Must-fix

None.

### Notes

1. **A bad revision range passes vacuously.** `for c in $(git rev-list "$BASE..$HEAD")` is a word-list expansion, so `set -e` does not trip when `rev-list` fails; the loop runs zero times and the step exits 0 with only a `fatal: Invalid revision range` on stderr (reproduced locally: rc=0). Reachable only if `base.sha` or `head.sha` is absent from the clone, which needs a force-push of `main` (the ruleset blocks it) or a stale run that the concurrency group already cancels. Within the D-203 bar, so a note. One-line hardening if the Keeper wants it: `commits=$(git rev-list "$BASE..$HEAD")` on its own line (an assignment does trip `-e`), then `for c in $commits`.

2. **Existing history is mostly unsigned; records batches will go red.** Of 397 non-merge commits reachable from `origin/main` since 2026-10-17, 162 lack a matching trailer, including recent hive records commits (for example `950329e`) and several Head Keeper commits. Nothing already on `main` is re-checked (only PR ranges are), but from the first PR after the ruleset change the queen's records batches and the Keeper's own commits need `-s` or `git rebase --signoff`. The `gh pr merge --merge --admin` merge commit stays outside the range, as the spec says.

3. **Admin merge bypasses a red `dco` just as it bypasses `product-ci`.** Under D-098 every merge is a logged admin bypass, and `--admin` merges over failing required checks. So the check is a visible signal, not a hard stop, until merges stop bypassing. This is the standing D-098 posture, not something this diff can change; worth stating in the D-098 record update the spec asks the queen to log.

4. **Author-email binding rejects GitHub-generated commits.** Commits made in the web UI (applied suggestions, "Update branch" merges), bot commits, and `git commit --author=<someone else> -s` (where `-s` writes the committer's identity) all fail. There is no `.github/dependabot.yml`, so no bot traffic today. Intended per CONTRIBUTING section 6 and Q-T025-6; noted so nobody reads a future failure as a bug.

5. **Trailer must be in the final paragraph.** Git's trailer parser only sees the last block of the message; a `Signed-off-by` followed by a prose paragraph is not a trailer and fails (reproduced). This is the same rule `git commit -s` and `git interpret-trailers` follow, so it only bites hand-written messages.

6. **`actions/checkout@v5` is tag-pinned**, matching the existing `product-ci` step. Consistent with the file; sha-pinning would be a separate Tier 3 change for both steps.

7. **Not in this diff, still open for acceptance 7:** adding `dco` to the `main` ruleset's required checks and reading it back, the `waggle decide` entry updating D-098, and the evidence pair (a PR with an unsigned commit fails `dco`; after `git rebase --signoff` it passes). This PR's own commit is signed, so it will not produce the failing half of that evidence; a throwaway PR is needed.

## Spec and decision check

- T-025 section 7: job name, `if`, runner, timeout, checkout options, `env` passing, loop body: all match the draft exactly.
- Acceptance 7: the code half is satisfied (an unsigned commit fails `dco`, shown locally); the ruleset half is the Keeper's follow-up.
- Acceptance 8: no U+2013 or U+2014 in the diff.
- D-131: enforcement is a product CI job, no third-party app. D-132: a Keeper-identity `-s` commit passes, as shown by the PR's own range. D-203: findings 1 and 3 need capabilities that already let the holder merge anything, so they stay notes.

Verdict: pass
