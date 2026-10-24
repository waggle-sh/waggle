<!-- waggle-review v1 verdict=pass head=0399938fd3aa29223d83a4d94c4f45c1b48f8980 -->

Risk-Tier: 3

**Intent Summary:** T-035 adds two gate features as product code in the four leased directories, in one commit (`0399938`) on top of base `167d5f7`. The approver path: `gate.approvers` in `internal/config` (login pattern, 32-entry cap, line-numbered errors), a token-only GraphQL `ReviewReader` in `internal/forge/reviews.go` (no bodies, `fullDatabaseId` as a string-decoded int64, 10-page cap, node rules that fail the read on a bad ID or state from a live login, base-change time from `BaseRefChangedEvent`), PR context from the event file through the injected `getenv` with the head-SHA binding, a pure `approvedBy` rule (ID order, counted state, exact head, strict `After` on the base change, any listed `CHANGES_REQUESTED` blocks), and a full filtered rescan whose output replaces the first report only on a qualifying approval, with every failure degrading to today's exit 1 plus one fixed-phrase warning under Actions. The trailer path: warnings only, under `leases`, in `verify` only, last-paragraph parse, values never echoed. The template gains `edited`, `pull_request_review`, `pull-requests: read` on the verify job, a concurrency group and `GITHUB_TOKEN` in the gate step only. Blast radius: every repo's merge gate; in this hive the trailer warnings appear on the first PR after merge (accepted, section 2.7 item 1) and the approver path stays inert until the pin move and a `gate.approvers` entry.

**Verification run (all from the worktree, one command per call):**

| Command | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | no output |
| `go test -race ./...` | all packages ok (`cmd/waggle` 42s, `dispatch` 205s, `forge` 11s, `scaffold` 10s) |

**Lease and assertion rule:** every changed file is in the lease rule's list. `gate_commits.go` changes only `commitScan` fields and `scan`. `gate.go` adds four usage lines after the first line. `forge.go` changes the doc comment only. The template changes only `on:` and the verify job; `relay-claim` and workflow-level `permissions` are byte-identical. The one changed existing assertion is `internal/config/gate_test.go:50` (`approvers: x` to `owners: x`), which section 6 requires. No `t.Skip` or `t.Parallel` added. Added non-test lines: 453, under the 554 estimate and the 720 target, so no planned cut applies.

**Threat model walk (D-203 bar), all held:**
- Approver list and protected paths read from the base config only (`baseTierPolicy` at `baseOID`); head-config approver case tested and fails.
- Approvable filter is `TierPolicy.RequiredMinTier == Tier3` minus a copy of comb's equal-or-prefix match over `HardcodedTier3Prefixes`, normalized as `TierPolicy.listed` does. `go.sum` and a listed `.waggle/hive/tasks/` path still fail after approval (tested).
- Filter drops only `tier3` and `outside_lease` on approvable paths; `outside_lease` is only emitted for `cl.ok` commits, so unbound and `Head Keeper:` commits still fail in `classify`; `link_mode`, `case_differs`, merges and case collisions are untouched; the rescan uses a fresh `gateReport`, so the dedup trap does not apply; pre-scan warnings are cloned into the second report (tested with a control-byte `--branch`).
- Event head is compared to the resolved `--head` OID before any forge call; the fake is never called on mismatch (tested). Branch default from `head.ref` runs before that binding by design (2.2) and goes through `branchTicket`.
- Logins: config side `^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`, API side `ownerPattern`, both ASCII, so `EqualFold` is ASCII folding; prefix and suffix cases tested.
- Token: only in the Authorization header via the existing `tokenTransport`; `StatusError` carries the status only; non-status errors collapse to `GitHub reviews unavailable`; no `os.Getenv` in `cmd/waggle` outside `main.go`.
- Nothing in the approver or trailer path can return exit 2 or 3: a rescan error prints the first report with `rescan failed`; degrade table and the two byte-for-byte baselines (`approvers: ""` vs absent; event file vs no event) pass.
- Template: no `pull_request_target`, no write permission, no PR-controlled field interpolated into `run:`; token appears once, inside the `Waggle gate` step; `TestGateTemplateBaseBuilt` and `TestRenderedTemplatesClean` pass unchanged.
- GraphQL field names (`fullDatabaseId`, `state`, `submittedAt`, `commit { oid }`, `author { login }`, `timelineItems(last: 1, itemTypes: [BASE_REF_CHANGED_EVENT])`, `BaseRefChangedEvent.createdAt`) match GitHub's reference; `databaseId` and `body` are absent from the query (tested with the letter-boundary regexp).

**Blocking Issues (Must-Fix Only):** none.

**Notes (not blocking):**
- N1: an unparseable or null `BaseRefChangedEvent.createdAt` is a read error in `reviews.go:149-155` (`GitHub reviews unavailable`), where section 2.3 says such an approval simply does not count. Both fail closed; the read-error form is slightly louder. Fine as is.
- N2: items the brief puts in the PR body (first `forge` importer named, D-061 deviation stated, GraphQL reference cited, changed-assertion list) are not verifiable from this worktree; the commit body carries only the sign-off. The queen should check the PR body before merge.
- N3: `readPREvent` runs on every Actions run, including `gate leases` and `gate tiers` with no approvers. It produces no output in that case, so section 8's "no change" rules hold; it is only an extra file read.
- N4: a listed path that also matches a hardcoded Tier 2 prefix under a light claim gets `tier3` from `CheckChangesPolicy` and never `tier2_light`, so after an approval no Tier 2 finding remains for it. That is comb's existing case order, the approver saw the exact head, and the lease cannot cover a listed path anyway. Carry as a note for a later polish ticket if `tier2_light` should survive an approval.
- N5: the hive effect at merge is as the brief states: the next hive PR under `pull_request` will show `leases: Intent Packet: ...` warnings until the queen template carries the trailers (2.7 item 1).

Verdict: pass