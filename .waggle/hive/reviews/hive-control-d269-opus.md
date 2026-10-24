# Guard review: hive control PR #109 (pin 1475d74 to 97127fc, gate workflow render, D-269)

Risk-Tier: 3

Reviewer: opus guard. Branch hive-pin-97127fc at 32aefd9 against origin/main c9cb754. Read-only.

## Intent Summary

PR #109 is one signed-off commit touching three files. `.waggle/control/config.yaml` moves `waggle.pin` from 1475d74 to 97127fc04e4d8eb803d4bd7e5c59e218cccc6e6f, the merge of T-035 (#107). `.github/workflows/waggle-gate.yml` takes the T-035 template render: `edited` on `pull_request`, a `pull_request_review: [submitted, dismissed]` trigger, the job `if:` widened to both events, job-level `pull-requests: read`, a per PR and head concurrency group with `cancel-in-progress: false`, and `GITHUB_TOKEN: ${{ github.token }}` in the `Waggle gate` step env. `.waggle/hive/DECISIONS.md` appends D-269. Blast radius: the merge gate on every hive PR. With `gate.approvers` still absent from the hive config, the only output change is `Intent Packet:` warnings. The approver path stays dormant, and `.waggle/**` and `.github/workflows/**` stay Head Keeper commit plus bypass.

## Checks

- **Pin.** `97127fc` is `Merge pull request #107 from waggle-sh/t-035` (parents 167d5f7, 0399938) and sits on the first-parent history of origin/main. On the first-parent line, 1475d74..97127fc holds #104 (records), #105 (previous pin), #106 (T-035 brief) and #107. Outside `.waggle/hive`, every product change in that range is T-035: gate_approve.go, gate_trailers.go, gate_verify.go, gate_commits.go, config, forge/reviews.go and the template.
- **Binary.** `go version -m` on `<scratch>/waggle-97127fc` reports `vcs.revision=97127fc04e4d8eb803d4bd7e5c59e218cccc6e6f` and `vcs.modified=false`.
- **Render.** Running `waggle-97127fc upgrade --diff` from the worktree reports `[pin] ... (unchanged)` and `[same]` for `.github/workflows/waggle-gate.yml` and for every other managed file. `git show 97127fc:internal/scaffold/templates/waggle-gate.yml.tmpl | cmp - .github/workflows/waggle-gate.yml` shows no difference, so the file matches byte for byte. A replay on an export of origin/main reports `[rewrite]` only for the workflow and `[same]` for everything else, which supports D-269's "every other managed file is [same]".
- **D-269.** `waggle-97127fc gate decisions --base origin/main --head HEAD` prints `decisions: ok, appended D-269`. The diff has 19 added lines and 0 removed. The entry has the D-250 shape: technical kind, tickets T-016 and T-035, options, the Keeper by role, both pins in full, no dashes and no home paths. It is accurate. Its parenthetical lists the triggers, the permission and the token but leaves out the job `if:` and the concurrency group (N1).
- **This PR's own CI** ran the new workflow on `pull_request` at 32aefd9. The token block showed `Contents: read, Metadata: read, PullRequests: read`. The gate was built from base c9cb754. It failed as expected: one leases error (the commit binds no ticket), two `tiers` Tier 3 path errors (config.yaml and the workflow), `decisions: ok, appended D-269`, and two `Intent Packet` warnings (N2). No `approver check skipped` line appeared, which is right with no approvers listed.

## Security review of the workflow

- **Fork PRs and the token.** `pull_request_review` from a fork behaves like `pull_request`: a read-only `GITHUB_TOKEN` and no secrets. The job asks only for `contents: read` and `pull-requests: read`. There is no `pull_request_target` and no write scope on this job. `relay-claim` keeps its own `if:` (`issue_comment` plus the `/waggle claim` prefix) and its write permissions. It is untouched and cannot run on the new events.
- **Checkout and gate source.** On `pull_request_review`, GITHUB_REF is `refs/pull/<n>/merge`, the same as on `pull_request`. `actions/checkout@v4` with `fetch-depth: 0` gets the merge commit, so the PR head and the base history are reachable. If the payload head was force-pushed away before the run, `HEAD_SHA` does not resolve and the gate fails closed. The checker is still built in a detached worktree at `BASE_SHA`, never from PR code.
- **Token in the gate step.** The token is in the env of the base-source `go build` and of the base-built binary only. That adds no new exposure: checkout's default `persist-credentials` already puts the same token in `.git/config` for every step.
- **Concurrency.** The group is `waggle-gate-${{ github.event.pull_request.number }}-${{ github.event.pull_request.head.sha }}`. Both fields are GitHub-written (an integer and hex), and the expression sits in `concurrency:`, not in `run:`. The job `if:` admits only the two events whose payloads carry `pull_request`, so the empty form `waggle-gate--` can only arise on a skipped job, and no other job shares the group.
- **Injection.** The change adds no `${{ }}` inside any `run:`. In the gate job, PR fields reach shell only through env. The branch default on review runs comes from `pull_request.head.ref`, which Go reads from the event file under `branchTicket`'s checks and never interpolates in YAML. The only `run:` interpolation left is in the relay job (login and issue number, both GitHub-written), and it is unchanged.
- **Can a trigger turn a red required check green without an approval?**
  - With no approvers listed, `prRun.approve` returns at `len(out.approvers) == 0` before any forge call. A review-triggered run therefore computes the same findings as a `pull_request` run at the same base and head. Anyone who can post a review gets a rerun and nothing more. The author could already force a rerun by pushing or reopening.
  - With approvers listed later, `approvablePath` excludes every `comb.HardcodedTier3Prefixes` entry, so hive control and workflow paths can never be approved through.
  - `edited` closes a hole that was open on main. Before this PR, an author could retarget to a branch whose base lacks `cmd/waggle/gate.go` (the `present=false` skip goes green), and nothing reran when they retargeted back. Now each retarget reruns the job against the new base, and the required check stays pending until the rerun finishes.
  - One route stays open, the YAML being run from the merge ref so a PR could rewrite the gate. It predates this change, D-140 (CODEOWNERS on `.github/workflows/`) covers it, and the new review trigger runs the same merge-ref YAML, so it is unchanged.
- **Gate behavior with no `gate.approvers`.** The hive config has no `gate:` block. `scanRange` passes `approvable` into `commitScan`, but with `filter` false no finding is dropped; it only sets `hit`. `maxTier` and `bound` are bookkeeping. `finish` adds trailer warnings under `leases` when the event context exists, and only for `verify`. Exit codes do not change. The new branch default (`head.ref` when `GITHUB_HEAD_REF` is empty) only applies on review runs, which did not exist before, and it reproduces what `GITHUB_HEAD_REF` gives a `pull_request` run.

## Comparison with D-250 (a8eff90)

The pattern is the same: one Head Keeper signed-off commit, a pin line, the managed workflow taken from the render, a pure-append decision, and an upgrade replay reporting `[same]` afterwards. D-250 also re-rendered guard.md and roster.json. Here only the workflow differs. D-250 N5 (setup-go reads `go.mod` from the merge commit) still applies unchanged.

## Blocking Issues

None.

## Notes

- N1: D-269's Keeper choice parenthetical leaves out the job `if:` and the concurrency group. The pin and "every other managed file is [same]" are both correct. Fix it in a later entry only if precision matters. The decisions gate forbids editing an entry in place.
- N2: this PR's body ends `Waggle-Verdict: pending`, which the gate reports as `Waggle-Verdict trailer malformed`. It also gives `Waggle-Ticket: T-035` while the commit binds no ticket, which gives `names T-035; commits bind no ticket`. Both are warnings only, as T-035 2.7 item 1 expects until the queen's template lands. If the queen wants these PRs quiet, the template could use `pass` or `must-fix` once the guards report, or drop the verdict line on control PRs.
- N3: T-035 2.7 item 2 checks are still owed on the first PR after merge. Confirm a review reruns `verify-leases-and-tiers`, that the review-run check attaches to the PR head SHA and satisfies the required check, that both SHAs are reachable, and that a base retarget reruns the job. This PR showed only the `pull_request` path.
- N4: any account that can post a review can now queue reruns. The concurrency group caps this at one running and one pending run per head, so the cost is runner minutes and a briefly pending check. No result changes.
- N5: `gate.approvers` may now be added to the hive config (2.7 item 3). Under Q-T035-1 (a) it gains nothing until the config lists `gate.protected_paths` outside the hardcoded prefixes.
- N6: wait for product-ci (pending at review time) before the bypass merge.

Verdict: pass
