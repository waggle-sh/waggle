Verdict: pass
Risk-Tier: 3

**Intent Summary.** T-031 lands step G1 of the repositioning plan in two commits on `t-031`. The gate now binds a commit to a ticket by subject prefix, `Waggle-Ticket:` trailer or the PR branch name, with first match winning and any disagreement failing leases as a ticket conflict. A new `gate.protected_paths` key in the base revision's config adds raise-only Tier 3 prefixes through a `TierPolicy` in `internal/comb`, read via `ls-tree` from `baseOID` only and rejecting symlinks, trees, oversize blobs and parse errors with exit 3. A merge commit is accepted only when `merge-base --all <M> <baseOID>` is exactly its second parent, and it is then diffed from that second parent. The five Go dependency inputs join `HardcodedTier3Prefixes`, which also reaches dispatch's `refuseDirs` and post-run check with no dispatch change. Blast radius: every PR into this repo from the first merge after this one, plus any dispatched build that touches `go.mod`, `go.sum`, `go.work`, `go.work.sum` or `vendor/`.

**Verification run on the head.** `go build ./...`, `go vet ./...` and `gofmt -l .` all produced no output. `go test -race ./...` passed every package, dispatch included.

**Lease and scope (AC2, AC9, AC11).** The nine changed files are all inside the lease rule. The `gate.go` change is usage text only. `internal/dispatch/`, `internal/scaffold/`, `.github/` and `go.mod` are untouched. `CheckChanges` keeps its signature as a wrapper over `CheckChangesPolicy` with an empty policy, and the restructured switch is equivalent for an empty policy. `isTier3Dir` and `matchesPrefix` are unchanged.

**Threat model walk.** Policy comes from `baseOID` through `ls-tree -z --full-tree`, never head, worktree or merge-base, and a head-side config entry is tested not to apply. Mode 120000, tree and commit entries, more than one record, oversize and invalid files each return an error that reaches stderr through `annotData`. The single `merge-base --all` equality test subsumes the brief's `--is-ancestor` condition, since a merge base is by definition an ancestor of `baseOID`; both crafted shapes in 2.3 are tested and refused, and the accepted merge is diffed from its second parent, never its first or skipped. A malformed trailer value fails rather than being treated as absent, and the three bound checks in 2.1 are enforced before any value is parsed. Conflict messages carry canonical IDs only, the branch warning and trailer messages are fixed text, and nothing of a trailer or branch value is echoed. No new message bypasses `gateReport.print`. The hardcoded tier is checked before the repo list, so no entry can lower a path, and `TestTierPolicyIsRaiseOnly` asserts it per path.

**Notes, not must-fix.**
- A protected-paths entry of `./` passes validation because the clean check runs after the trailing slash is stripped. It can never match a diff path, so it is harmless over-acceptance.
- A subject such as `T-000: x` has its prefix rejected by `ParseTicketID` and then binds by trailer or branch instead of failing. An attacker gains nothing over simply omitting the prefix.
- A `Waggle-Ticket:` trailer with an empty value is indistinguishable from no trailer. Equivalent to omitting it.
- AC10 is covered by a new `TestBindHostileText` rather than by extending `TestVerifyEncodesHostileText`. Coverage is equivalent; the branch regex captures digits only, so no part of a branch name can reach output.
- The bad-branch warning is filed under leases, so a `tiers`-only run does not show it. Matches the brief's wording.
- Added non-test lines are roughly 330, under the 670 target, so the planned cut was not needed.

**Blocking Issues (Must-Fix Only):** none.

Verdict: pass