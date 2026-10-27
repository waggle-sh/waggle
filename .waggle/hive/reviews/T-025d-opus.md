<!-- waggle-review v1 verdict=pass head=36fbe1328ee3cdb0e4432fb84445b7949dc62ee4 -->

Risk-Tier: 3

**Intent Summary:** This is the phase D Head Keeper commit `36fbe13`. Its parent is `bae5b28` and it has a `Signed-off-by` trailer. It adds a `dco` job to `.github/workflows/ci.yml` (25 lines, nothing deleted). The job copies the brief's section 7 draft line for line. It runs only on `pull_request`. The checkout uses `fetch-depth: 0` and `persist-credentials: false`. The base and head SHAs reach the script through `env`, not inline `${{ }}`. For every commit in `BASE..HEAD`, including merge commits per Q-T025-6, it fails if no `Signed-off-by` trailer contains `<author-email>`. The match ignores case. The `product-ci` job, triggers, workflow `permissions: contents: read` and concurrency are all unchanged. Line 1 still reads `# SPDX-License-Identifier: Apache-2.0`, which is what the C2 repo check expects. Blast radius: one extra PR-only job. It blocks nothing until the Head Keeper adds `dco` to the `main` ruleset's required checks. The commit message says that change comes on a separate card. AC7's evidence (an unsigned commit fails, then passes after `git rebase --signoff`, plus the ruleset read-back) can only exist after merge. The ruleset change also needs a `waggle decide` entry updating D-098.

Evidence:
- `git diff bae5b28 36fbe13`: only `.github/workflows/ci.yml` changes, with lines 51 to 74 added.
- `go test -race ./... -count=1` passes in every package, including `internal/license` and `internal/scaffold`. `go vet ./...` and `gofmt -l .` print nothing.
- No U+2013 or U+2014 in `ci.yml` or the commit message (AC8).

**Notes (not blocking; the brief's threat model says this check catches mistakes, not attackers, D-203):**
- **Substring match:** the trailer check matches a substring, so a sign-off in a different name with the same email passes. That is acceptable under the DCO, where the email identifies the person.
- **Stale base SHA:** `pull_request.base.sha` is the base tip when the event fired. Since `fetch-depth: 0` fetches every branch, `BASE` can always be found. A stale base only widens the range, so the check gets stricter, never weaker.
- **Fork commit text:** a fork can choose its commit subjects and emails, and both are printed into an `::error::` line. A single line can't start a new workflow command, and the job has only read access and no stored credentials, so this does not matter.
- **Push to `main`:** the job is skipped on push. That is expected, because the merge commit `--admin` creates is outside the PR range, as the brief states.
- **Unpinned action:** `actions/checkout@v5` uses a version tag, not a pinned SHA, the same as the existing job.

**Blocking Issues (Must-Fix Only):** None.

Verdict: pass