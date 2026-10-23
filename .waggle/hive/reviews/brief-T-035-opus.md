# Brief review: T-035 (G3, named approver and Intent Packet trailers), opus

Risk-Tier: 3

Reviewed `.waggle/hive/tasks/T-035.md` at a4c5710 on `t-035-brief` (base origin/main caa5c92; the commit adds only that file, 173 lines). Checked against `docs/rfc-notes/repositioning.md` sections 5 and 6, DECISIONS D-017, D-203, D-229 to D-260, T-031, T-032, T-033, T-034, and the code in `cmd/waggle/gate*.go`, `internal/comb/changecheck.go`, `internal/config/config.go`, `internal/forge/`, `internal/scaffold/templates/waggle-gate.yml.tmpl`, `.github/workflows/waggle-gate.yml` (byte-identical to the template today) and the tests section 6 names.

## Intent Summary

T-035 adds `gate.approvers` (comma-separated GitHub logins, base config only) and makes a protected-path change pass when a listed approver's latest decisive review is APPROVED at exactly the gated head OID, with no listed change request outstanding. It does this by running the commit scan a second time with a filter that drops only `tier3` and `outside_lease` on listed, non-hardcoded paths. Reviews come from a new `internal/forge/reviews.go` over the existing token transport. PR number, head, ref and body come from `GITHUB_EVENT_PATH`. It also adds warning-only Intent Packet trailer checks on the PR body, and changes the scaffold gate workflow (review trigger, `pull-requests: read`, token in the gate step, a concurrency group). Blast radius: the merge gate every PR passes, in this repo from the first PR after merge (the gate is built from base source), and the workflow that `waggle init` writes. The core approval design is sound. The rule is pure and ordered by review ID, the comparison is a full OID, the list comes from base only, the filter is narrow, and every failure degrades to today's exit 1. The section 6 test map is accurate. Three problems in the brief need fixing before dispatch: contradictory output invariants, a wrong hive-effect statement, and an unrerun retarget that passes listed paths with no approval.

## Blocking Issues (Must-Fix Only)

**M1. The brief contradicts itself about when output may change, so the builder and the guards cannot both satisfy it.**
- 2.4 (T-035.md:59) runs the trailer check "when the PR context exists", with no condition on approvers. So every `pull_request` run with a body that lacks trailers gains `Intent Packet:` warnings.
- Section 5 (T-035.md:123) makes "any output or exit change when `gate.approvers` is absent or empty" a must-fix.
- AC7 (T-035.md:163) requires "With approvers absent: ... the output is byte for byte the output with no event file". That is false whenever the AC's event file has a body without valid trailers.
- 2.2 (T-035.md:40) says that outside `pull_request`/`pull_request_review` "the gate behaves as today, with no warning". AC7 expects an `approver check skipped` warning for event name `push` and for a missing `GITHUB_EVENT_PATH`.

Fix: scope the section 5 invariant and AC7 to the approver path (no forge call and no approver warning when approvers are absent or no listed path is hit). State that trailer warnings are the one intended output change on every PR with context. Make AC7's event file carry three valid matching trailers, or compare against the same event with approvers set to none. Say explicitly that "no warning" in 2.2 applies when approvers are absent or no listed path is hit, and that with approvers and a listed hit, a missing context under `GITHUB_ACTIONS=true` warns `approver check skipped: no pull request context ...`. As written, a guard can flag a correct build against section 5.

**M2. "Hive effect: none until the hive pin moves" (T-035.md:5) is wrong.** The hive's gate step builds `./cmd/waggle` from the PR base (`.github/workflows/waggle-gate.yml:98-100`, D-143, D-249), not from the pin. The first hive PR opened after T-035 merges is gated by T-035 code, under today's workflow:
- `GITHUB_EVENT_NAME=pull_request` and `GITHUB_EVENT_PATH` are set by the runner.
- So the PR context exists and the trailer check runs.
- Every queen PR, records batch and dispatched build PR then gets `::warning::leases: Intent Packet: ...` annotations on the live required check, before 2.7 item 4 changes any PR body.

The approver path stays inert only because the hive config has no `gate.approvers`. The pin governs dispatch's config reader (D-246), not the gate. This is exactly the "changes today's output on a live required check" risk. The Head Keeper should know about it when ruling Q-T035-3. Fix: correct the Layer line and 2.7. Say what lands at merge (trailer warnings on every hive PR, and the branch default, which is inert under `pull_request`) and what lands at the pin move (dispatch can read `gate.approvers`, plus the workflow `[rewrite]`). Either order 2.7 item 4 (queen PR body trailers) before or with the merge, or record the noise as accepted.

**M3. A base retarget does not rerun the gate, so a listed-path change can pass with no approval at all. The brief files the retarget case as a harmless note.**
- `on.pull_request.types` is `[opened, synchronize, reopened, labeled]` (template line 7), and 2.6 (T-035.md:85) keeps it.
- Changing a PR's base fires only `pull_request` `edited`. The green `verify-leases-and-tiers` check on head H was computed against the old base: its merge-base, its base config (`baseTierPolicy`, gate_verify.go:181) and its protected paths. That check stays attached to H.
- Attack inside the brief's own threat model, which lists retargeting (T-035.md:119). The author pushes a branch `x` (from an old main without `protected_paths`, or one that already holds the `deploy/` commit). They open the PR against `x`, where the gate passes, then retarget to `main` and merge. No approver is involved.
- The section 5 note (T-035.md:125, "an approval survives a base retarget at the same head (the approver approved that head tree)") misreads this. The whole verdict is stale, not only the approval.
- The hole predates G3, but G3 rewrites this exact `on:` block and makes named approval the product guarantee of a public launch (D-258).

Fix: add `edited` to the `pull_request` types in 2.6 and AC10. The job `if:` already admits `pull_request`. This also reruns the trailer check on body edits, which removes the "PR body edit does not rerun" note. If the scout prefers not to, route it to the Head Keeper as a question with the attack stated. Do not leave it as a note.

## Notes (not blocking)

**N1. The concurrency claim is wrong** (T-035.md:88): "Queued, not cancelled, so no required check ends `cancelled`." With `cancel-in-progress: false`, GitHub allows one running and one pending job per group, and a newer queued job cancels the older pending one. The newest event still runs last, so the final verdict follows the newest event. But intermediate runs do end `cancelled`, and keying the group on the PR number alone lets a run for one head cancel a pending run for another. Suggest `waggle-gate-${{ github.event.pull_request.number }}-${{ github.event.pull_request.head.sha }}` (both GitHub-written fields; the head ref stays forbidden by `guard_template_test.go:69`), and correct the sentence.

**N2. The review list returns review bodies.** `maxBody` is 1 MiB per response (`internal/forge/github_transport.go:21`), and 100 reviews per page with bodies of up to 65536 characters each overflow it. Any account that can post a review can force `GitHub reviews unavailable` on a PR. Today that means the author's own accounts; after G5, anyone on a public repo. It fails closed (admin bypass as today), so this is a DoS on approval, not a pass. Consider a GraphQL query for `databaseId`, `state`, `commit.oid` and `author.login` only (`tokenTransport` already does POST /graphql), or record the limit in 2.5.

**N3. Second-scan details the builder will otherwise guess** (2.3, T-035.md:53):
- The second report must carry the pre-scan `branch name ignored` warning (gate_verify.go:131-134) and must run `caseCollisions` again.
- Say what a git error in the second scan does. As written it returns through `scanRange` as exit 3, which section 5 lists as a must-fix for "any approver ... failure". Either keep the first report and add a skip warning, or rule that a git error stays exit 3.
- The branch default needs the resolved head OID before `branchTicket` (today the order is gate_verify.go:127-135, with head resolved inside `scanRange`), so `runGateRange` must reorder. That fits the lease rule. Mention it so it is not read as scope creep.

**N4. Version skew (2.1, T-035.md:34; 2.7 item 2).** Inside this repo the base-built gate cannot see an unknown `gate.approvers`, because the base config and the base source are one revision and the key can land only after T-035 is on main. The real skew is dispatch's pinned binary (D-246) and any installed CLI, which 2.7 item 2 handles correctly. After G4, a released Action pinned by tag reads the repo's config at any version, which is where this rule bites outside users. Carry it to G4 and the G5 recipes. The 2.1 sentence about the gate is harmless but overstated.

**N5. AC tightening.**
- AC5's "at H then DISMISSED": GitHub dismisses a review in place (same ID, state becomes `DISMISSED`). The table should model the approval itself as `DISMISSED`, plus a case where a dismissed change request is followed by an approval at H, which should pass.
- AC6 runs only `verify`. Add one `gate leases` and one `gate tiers` case, since 2.3 places the warning differently for `gate leases`.

**N6. Exact-head approval (D-233, settled) means every base update merge needs a fresh approval** under an up-to-date merge rule. Worth one sentence for the G5 recipes, not a design change.

**N7. Section 6 checked against the tests.** I grepped for `approvers`, `GITHUB_EVENT*`, `GITHUB_TOKEN`, `GITHUB_REPOSITORY`, `pull_request`, `concurrency`, `github.token`, `event_name`, `gate:` configs, `GateWorkflow`, `commitScan`, `scanRange` and `baseTierPolicy` across `cmd/` and `internal/` tests.
- The only assertion that breaks is `internal/config/gate_test.go:50` (`approvers: x` asserted invalid), as the brief says.
- Every pinned line I checked matches the code: `gate_bind_test.go:147-192`, `:194-237` (`x: y` at :199), `:37-85` and `:249-271`; `gate_verify_test.go:168-180`, `:274-295`, `:298-306` and `:309-322`; `gate_test.go:216-241`; `guard_template_test.go:40`, `:46` (`pull-requests: write` count, which `pull-requests: read` does not match), `:50`, `:57`, `:60`, `:66` and `:69`; `init_test.go:381-410`; `upgrade_test.go:138`; `github_test.go:171-176`; `config_test.go:88-97`.
- No test constructs `commitScan` or `gateReport` directly.
- Existing GHA-env tests set no `GITHUB_EVENT_NAME` (`ghaEnv`, gate_verify_test.go:66), so they stay context-free.
- I found no missing breaking or pinning test.
- No package imports `internal/forge` today, so `cmd/waggle` becomes its first importer and no import-boundary test exists to break.

**N8. Fallback and trust.** The `GITHUB_HEAD_REF` fallback is correct: GitHub sets that variable only for `pull_request` and `pull_request_target`. `pull_request.head.ref` carries the same author-controlled trust as `GITHUB_HEAD_REF` and goes through `branchTicket` unchanged. Reading the event file through the injected `getenv` with a cap is right. The runner writes the file and no earlier step in the job writes `GITHUB_ENV`. The head binding always holds in CI (both sides come from the same payload) but is a correct guard for manual runs.

**N9. YAML, lease, budget and questions.**
- The YAML belongs in G3. D-258's rationale counts it as G3 work, and the G4 lease still covers the composite Action.
- The lease is minimal: `internal/config/` is forced by gate_test.go:50, and `internal/scaffold/` by the template. The file-level rule is tight.
- The budget is plausible: 250 bottom-up lines doubled to 500, target 650, stop 900. Scan integration at 40 lines may be light, and the trailer cut stands alone.
- Q-T035-1 to Q-T035-6 and their recommendations hold. None reopens a settled decision: Q5 defers merged state to G4 as D-242 allows, Q4 matches D-017 and D-234, and 2.1 follows D-238 and D-239.
- One caution for the Q1 card: under (a), every `go.mod` bump from an outside agent still needs an admin merge, which is the friction D-258 cites.

Verdict: must-fix

## Round 2 (db951d7)

Risk-Tier: 3

Reviewed `git diff a4c5710 db951d7` (only `.waggle/hive/tasks/T-035.md`, +58/-54) and the whole file at db951d7, against the round 1 findings, `internal/comb/backend.go`, `internal/scaffold/guard_template_test.go`, the template, `.github/workflows/waggle-gate.yml` and GitHub's GraphQL reference for pull requests (docs.github.com/en/graphql/reference/pulls, fetched during this review).

### Round 1 items

- **M1 closed.** 2.2 "What prints when" (line 43) limits the missing-context warning to approvers listed plus a listed hit under `GITHUB_ACTIONS=true`. 2.4 (line 61) names trailer warnings as the one intended output change and scopes the "no change" rules to the approver path. Section 5 (line 127) now reads "from the approver path when `gate.approvers` is absent or empty, or when no approvable path is hit". The AC preamble (line 159) gives every event file three valid trailers. AC7's `push` and missing `GITHUB_EVENT_PATH` cases expect the warning, and 2.2 now allows it. The four places agree.
- **M2 closed.** The Layer line (line 5) says the effect lands at merge because the hive gate builds from base source. I checked this against `.github/workflows/waggle-gate.yml:98-100`. It also says trailer warnings appear on every hive PR with no exit change, and that the approver path and branch default stay inert. Pin-move effects are listed separately. 2.7 item 1 orders the queen template change before or with the merge, or else records the noise as accepted. Q-T035-3 states the cost.
- **M3 closed.** `edited` appears in 2.6 (line 87), AC10 and the section 5 must-fix list ("a missing `edited` type"). The cost is stated (about one runner minute per title or body edit, check pending meanwhile). The hive's own hole stays open until the pin move, and section 5 notes it. Q-T035-7 is framed fairly. (b) is a real option, and its attack is concrete and correct: with the rerun, an approval of the small diff against `x` at exactly H still matches H after a retarget to `main`. The cost of (a) is stated, and the dependence on GitHub's stale-dismissal setting is rejected for a stated reason.
- **Fable N2 filter holds.** In `comb.RequiredMinTier` (backend.go:70-83), Tier 3 means "matches `HardcodedTier3Prefixes` and is not in the exact-case `.waggle/hive/` carve-out". The 2.3 filter excludes every path matching that list, carve-out included, so its excluded set is a strict superset of the comb Tier 3 set. A listed path can therefore become approvable only when no hardcoded prefix matches it, and a listing stays raise-only. Prefix matching (`go.mod` also matches `go.modx`) can only over-exclude, which fails closed. Gate paths come from git and are already clean. AC6 covers `go.sum` and `.waggle/hive/tasks/`, and section 5 names the prefix case as must-fix.
- **Second-scan fallback holds.** Any rerun error prints the first report, which keeps its exit 1 fails, plus `rescan failed`. It never reaches exit 3 and never prints a pass. The rerun is the only route to 0, and it runs only after a qualifying approval. Rerun drift can only add findings (for example, a claim that expires between the two scans), so it fails closed. Pre-scan warnings carry over.
- **Section 6 holds after the diff.** I grepped tests again for the newly touched terms (`edited`, `synchronize`, `graphql`, `timelineItems`, `head.sha`, `GITHUB_EVENT_PATH`, `GITHUB_EVENT_NAME`, `GITHUB_HEAD_REF`). No test pins the `pull_request` types line. The concurrency key `github.event.pull_request.head.sha` does not contain the forbidden `github.event.pull_request.head.ref` (guard_template_test.go:69). `github_transport_test.go:176` and `github_test.go:147-160` exercise `POST /graphql` through `do` and `autoMerge`, and neither changes under the lease. No missing breaking or pinning test.
- **Budget holds.** 20+65+35+40+45+55+3+14 = 277, doubled to 554, stated as 550. Target 720 and stop 990 are the 1.3x and 1.8x ratios used before. A planned cut is named, the D-210 rule is repeated, and only crossing the stop goes to the Head Keeper (D-185).

### Blocking Issues (Must-Fix Only)

**M1. The query orders reviews by `databaseId`, which GitHub has deprecated for `PullRequestReview` and scheduled for removal.** The GraphQL reference for pulls says: "`databaseId` (Int): Deprecated ... will be removed because it does not support 64-bit signed integer identifiers", with removal on 2024-07-01, and says to use `fullDatabaseId` (BigInt). REST review IDs passed 2^31 in 2024. 2.5 (line 75) selects `databaseId`, 2.3 (line 53) orders by review ID, and AC4 decodes "in ID order". Possible outcomes:
- If the field is gone or errors, every query returns `errors`, so the gate always skips and the approval route is dead on arrival. This fails closed but breaks the feature.
- If it returns null or a wrapped value, the brief does not say what a missing ID does. A builder who copies the null-login rule and drops the node, or who sorts zeros first, can reorder or lose a listed approver's later `CHANGES_REQUESTED`. Under Q-T035-2 (a), the earlier approval then counts. That is a pass without the blocking rule the brief specifies.

The line "the builder confirms the field and enum names" does not cover this, because the brief names the wrong field. Fix in 2.5, 2.3 and AC4:
- Select `fullDatabaseId`. A BigInt arrives as a JSON string, so decode it as a decimal string into `int64`.
- Order by it.
- State that a node from a listed login with a missing or unparseable ID, or an unknown `state`, makes the whole read an error (`GitHub reviews unavailable`, exit 1), never a dropped node.
- Add AC4 cases: a string ID above 2^31 decodes and orders correctly, and a null ID on a listed login's node is an error.

I verified the other names against the same reference: `BaseRefChangedEvent.createdAt` (DateTime!), the `BASE_REF_CHANGED_EVENT` value of `PullRequestTimelineItemsItemType`, `timelineItems(last:, itemTypes:)`, `reviews(first:, after:)`, and `state`, nullable `submittedAt`, nullable `commit { oid }` and `author { login }` on `PullRequestReview`. They are real. The builder check that remains is the outer `repository(owner:, name:)` wrapper, which 2.5 leaves implied.

### Notes (not blocking)

**N1. AC7's approvers-absent comparison is vacuous** (line 167). "Approvers absent" compared with "the same input minus the key" compares a run with itself. Make it two comparisons. First, `approvers: ""` against no `gate:` approvers line. Second, approvers absent with the valid-trailer event file against the same run with no event file, which is the comparison that pins section 5. Both must be byte for byte equal, and the fake must never be called.

**N2. Reason phrases for the non-file cases are undefined.** AC7 expects `no pull request context (<reason>)` for event name `push` and for a missing `GITHUB_EVENT_PATH`. 2.2 enumerates reasons only for a file that was read. Name two fixed phrases, for example `event is not a pull request event` and `no GITHUB_EVENT_PATH`. The event name is runner-written, but the phrase should stay fixed rather than echo it.

**N3. Same-second ties.** `submittedAt` and `createdAt` have one-second resolution. "After" in 2.3 is strict, so an approval in the same second as a retarget does not count. That fails closed. One clause in 2.3 would stop a builder from using `!Before`.

**N4. The estimate is stated as 550 against a doubled 554.** This is cosmetic, and the ratios still hold.

No new contradiction found elsewhere. The Status line, Layer line, 2.1 skew paragraph (its item 3 reference is correct), 2.7, section 4, section 5 notes, Q-T035-1 to Q-T035-7 and section 9 agree with one another. No dashes, home paths or emails were added. The Keeper is named by role.

Verdict: must-fix

## Round 3 (dde2eb1)

Risk-Tier: 3

Confirming guard on the round 3 fix. Reviewed `git diff db951d7 dde2eb1` (only `.waggle/hive/tasks/T-035.md`, +13/-11) and the surrounding 2.2, 2.4 and section 9 text at dde2eb1, against my round 2 findings and the fable round 2 nits. Field names checked against GitHub's public GraphQL schema (octokit/graphql-schema `schema.graphql`, fetched during this review).

### Round 2 M1 closed

- **Field and type verified.** The schema has, on `type PullRequestReview`, `databaseId: Int @deprecated(reason: "... does not support 64-bit signed integer identifiers. Use fullDatabaseId instead. Removal on 2024-07-01 UTC.")` and `fullDatabaseId: BigInt` (nullable). `scalar BigInt` is described as "encoded as a string". `Query.repository(owner: String!, name: String!, followRenames: Boolean = true): Repository` exists. So the 2.5 query (`repository(owner: $owner, name: $name) { pullRequest(number: $n) ... }`) and the string decode are correct. The wrapper is now explicit, and a missing `repository` is an error. The remaining builder check is only a citation.
- **Decode.** 2.5 "Review IDs" says the value is decoded as a decimal string of 1 to 19 digits into a positive `int64` with `strconv.ParseInt`, base 10, never through a float. A 19-digit value above `MaxInt64` fails `ParseInt`, and `0` and `-1` fail "positive", so every out-of-range value is an error. AC4 pins the JSON-string form (an unquoted number is an error), exact decode above 2^31 (`"2147483648"`, `"9000000000"`), and ordering above `"2147483647"`.
- **Ordering.** 2.3 orders by `fullDatabaseId` ascending, not by response order or `submittedAt`. It states there are no ties because duplicates are a read error. AC5 adds the 2^31 + 1 versus 2^31 - 1 case, which catches an int32 or lexicographic sort.
- **Failure semantics.** 2.5 "Node rules" covers every non-dropped node: a missing, null or unparseable ID, a duplicate ID, or a state outside the five enum values (the five match `PullRequestReviewState`) fails the whole read as `GitHub reviews unavailable`, "gate exit 1 as for every read error, never exit 3, never a pass". 2.3 "Degrade" lists the node rules under that phrase. Section 5 adds the matching must-fix entries (deprecated `databaseId` requested, a bad node dropped or ordered instead of failing, a float decode). AC4 has a case for each: null, missing, unquoted, `"-1"`, `"x"`, duplicate, `UNKNOWN`. AC7 adds the `runGate` exit 1 case with the phrase. The rule applies to nodes from any valid login, not only listed ones, which is stricter than I asked for and avoids a builder needing the approver list inside `forge`.
- **Null author.** Dropped first, and AC4 has a case for it.

### Can a dropped null-author node hide a listed approver's change request?

No, not in a way that lets an approval count under the D-203 model. `author` resolves to null only once the account no longer exists. A listed approver's change request turns null-author only if that approver deletes their own account. That action is theirs alone, and it removes their approvals from the same login at the same moment. The PR author cannot null another account's authorship, and reviews from other accounts the author controls are unlisted and never count. The residual case is a deleted listed login that gets re-registered and then approves while the old account's change request is gone. That is the Q-T035-6 note ("a renamed then re-registered login counts") already in section 5, so it needs no new text. Ghost reviews: if GraphQL returns the `ghost` actor for a deleted account instead of null, the node passes `ownerPattern` and is kept. It must then carry a valid ID and state, or the read fails closed. `ghost` is a reserved GitHub login, so no listed approver can be it, and the node can only add a blocking state if a Keeper listed `ghost`, never remove one. Ghost reviews do not matter for safety.

### Round 2 notes

- **N1 closed.** AC7 now has two comparisons against independent baselines: `approvers: ""` against no `approvers` line; and approvers absent with the valid-trailer event file against a `GITHUB_ACTIONS=true` run with no event name or path. The fake is never called in any of the runs. The second baseline yields no output from the approver path, because 2.2 "What prints when" prints nothing for a missing context with approvers absent, and 2.4 is silent without a context. That is the comparison that pins section 5.
- **N2 closed.** 2.3 "Degrade" enumerates fixed context phrases, including `event is not a pull request event` (any name but the two, not echoed) and `no GITHUB_EVENT_PATH`, and AC7 pins both exact strings.
- **N3 closed.** 2.3 requires `Submitted.After(LastBaseChange)`, never `!Before`. AC5 adds the same-second case, and section 5 adds the matching must-fix.
- **N4 closed.** Section 9 reads estimate 554, target 720 (1.3 times 554 is 720.2), stop 997 (1.8 times 554 is 997.2). D-185 holds: the line estimate is doubled, all three numbers are set, the planned cut is unchanged, and only crossing the stop goes to the Head Keeper. Folding the ID rules into the existing 65 for `reviews.go` is plausible at about 15 lines.

### Fable round 2 nits

- **Degrade wording off Actions.** 2.3 now adds the warning only under `GITHUB_ACTIONS=true` and nothing when it is unset, consistent with 2.2 and AC7.
- **matchesPrefix.** 2.3 now says six lines and gives the normalization `filepath.ToSlash(filepath.Clean(p))` then lowercase, matching `TierPolicy.listed`.
- **Nil first scan.** 2.3 "When it runs" now requires the first `scanRange` to return nil, with `errTooLarge` and git errors named as never triggering a lookup or rerun.

### Blocking Issues (Must-Fix Only)

None.

### Notes (not blocking)

**R3-N1.** AC4's "no `databaseId` field in the query text" must be a case-sensitive or token check, since `fullDatabaseId` contains `DatabaseId`. A case-insensitive `Contains` would always fail the test, so a builder would notice. Fails loud, not open.

**R3-N2.** "1 to 19 digits" excludes a sign, but `strconv.ParseInt` accepts `"+5"`. The builder should check digits before calling `ParseInt`, as the wording implies. A leading-zero form such as `"007"` decodes to the same value as `"7"` and is caught by the duplicate rule. Neither can come from GitHub, and either way the result fails closed or is harmless.

No new contradiction found. The Status line names round 3 and its contents. 2.3, 2.5, section 5, AC4, AC5 and AC7 agree on the ID, ordering, node and time rules. The 2.3 phrase list agrees with the 2.2 event-file reasons. No stale `databaseId`, `550` or `990` remains. The file has no em or en dashes, home paths or emails, and the Keeper is named by role.

Verdict: pass
