<!-- waggle-review v1 verdict=pass head=0399938fd3aa29223d83a4d94c4f45c1b48f8980 -->

Risk-Tier: 3

**Intent Summary:** Commit `0399938` (T-035, G3) adds three things to the merge gate. First, a GitHub account listed in the base config's new `gate.approvers` key can pass a change to a repo-listed `gate.protected_paths` path. That needs an approving review at exactly the gated head SHA, submitted strictly after the PR's last base change, with no listed approver's counted state at `CHANGES_REQUESTED`. Hardcoded Tier 3 paths stay bypass-only, `.waggle/hive/` included. When approved, the gate runs the scan again with only `tier3` and `outside_lease` dropped on approvable paths. Any failure on the approver path leaves today's findings and exit code in place and adds at most one `approver check skipped` warning, and only under Actions. Second, the gate now warns, and never fails, about Intent Packet trailers in the PR body. Third, a token-only GraphQL review reader (`internal/forge/reviews.go`) reads `fullDatabaseId` and no review bodies. The workflow template gains `edited`, a `pull_request_review` trigger for `[submitted, dismissed]`, job-level `pull-requests: read`, a concurrency group on PR number plus head SHA, and `GITHUB_TOKEN` in the `Waggle gate` step only. Blast radius: the merge gate every PR passes. In this hive the effect at merge is only `leases: Intent Packet: ...` warnings on PRs without trailers, because the hive config has no `gate.approvers` (brief section 2.7).

**Checks run:**
- **Commands:** `go build ./...` (clean), `go vet ./...` (clean), `gofmt -l .` (no output), `go test -race ./...`. All packages `ok`, including `internal/dispatch` and `internal/comb`, so no push-only test failures on this host.
- **Lease:** the 14 changed files all fit the lease rule. Inside `gate_commits.go` only `commitScan` fields and `scan` changed, and `forge.go` changed only its package doc comment. The `Forge` interface, transports, `comb`, `.waggle/` and `.github/` are untouched. The only changed existing assertion is `internal/config/gate_test.go:50` (`approvers` becomes `owners`), which section 6 requires.
- **Filter:** `gate_commits.go:222` drops only `RuleTier3` or `RuleOutsideLease`, and only on paths `approvablePath` accepts. `approvablePath` (`gate_approve.go:90`) requires `policy.RequiredMinTier == Tier3` and then repeats `matchesPrefix` over `HardcodedTier3Prefixes`, with the same normalization as `TierPolicy.listed`. This covers:
  - `link_mode`, `case_differs`, unbound commits, `Head Keeper:` commits and crafted merges all stay, because those findings come from `classify` or other rules.
  - The rerun builds a fresh `gateReport` seeded only with the pre-scan warnings, so the `check+path` dedup cannot hide a `link_mode` on the same path.
- **Approval rule:** `approvedBy` (`gate_approve.go:111`) sorts by ID and counts only `APPROVED`, `CHANGES_REQUESTED` and `DISMISSED`. Any counted `CHANGES_REQUESTED` blocks. It needs an exact `CommitID == head` and `Submitted.After(LastBaseChange)`, so a zero time never counts.
- **Login matching:** `EqualFold` compares strings that are ASCII-only on both sides (`approverPattern` in config, `ownerPattern` in the reader), so it acts as ASCII folding.
- **Echoed text:** the warning echoes the config entry, not the API login.
- **Reader node rules:** a null author or bad login drops the node. A missing ID, a non-digit ID (checked before `ParseInt`), a duplicate ID or an unknown state fails the whole read. A bad `commit.oid` becomes `""` and a bad `submittedAt` becomes zero. A missing base-change time fails the read. Paging stops at 10 pages and the cursor is bounded. Status errors carry only the status code. Every read error maps to a fixed phrase and none returns exit 2 or 3.
- **Context and environment:** the event file is read only under `GITHUB_ACTIONS=true`, through the injected `getenv`, capped at 2 MiB. The head binding in `finish` nulls the context before both the approver check and the trailer check. The branch default uses `head.ref` without the head binding, as section 2.2 allows. No added line contains `os.Getenv` or an em or en dash.
- **Trailers:** only canonical ticket IDs and digits are echoed, and hostile-value tests pass.
- **Template:** `relay-claim` and the workflow-level `permissions` are unchanged. The token appears once, in the gate step. No PR-controlled field is interpolated into a script.
- **AC coverage:** AC3 to AC10 are covered by `gate_approvers_test.go`, `reviews_test.go`, `gate_approve_test.go` and `gate_approver_template_test.go`. That includes the exact AC7 context phrases and both independent baselines with zero fake calls.

**Notes (not blocking):**
- I could not see the PR body. The queen should confirm it lists the `gate_test.go:50` change, the first `forge` import in `cmd/waggle`, the D-061 deviation, and the GraphQL reference citation (AC2, AC11).
- `approvedBy` gets ASCII-only folding from the reader's `ownerPattern` filter rather than enforcing it itself. That is correct for every real source today.
- Added non-test lines are about 650 including comments and blank lines, under the 720 target.

**Blocking Issues (Must-Fix Only):** None.

Verdict: pass