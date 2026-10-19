<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: hive-records-10 (PR #52, head 185ac8d)

Verdict: must-fix
Risk-Tier: 2

## Intent Summary

Hive-layer PR, base `main` at `236de39` (hive-records-9 merged), four commits, all authored and committed by the D-160 noreply address. `b664370` appends D-170 to D-177 to `.waggle/hive/DECISIONS.md` (Keeper rulings on Q-T023-20 to Q-T023-27, every default approved): a phase D0 runtime PR before phase D, isolated worker git config, `max_budget` in the ledger map, phase D resized to 550 / 770 / 960, guard permissions without `git add` and `git commit`, an ignore check before the claim, a manual process-group probe by the phase D builder, and a per-phase must-fix count for the T-012 step-up. `0803b61` rewrites the phase D part of `tasks/T-023.md` (section 3 D0 and D rows and estimate paragraph, 10.2 guard details, new 10.6 with 22 carry items, 9.1, 9.2, AC8, section 11 numbering, 15.2). `5486680` and `185ac8d` rewrite the queen handoff. Nothing under `cmd/`, `internal/`, `go.mod`, `.github/` or `.waggle/control/`; nothing ships. Blast radius: 10.6 and the D0 and D leases are what the next two Tier 3 builders read. The decisions, the carry mapping, the handoff and the privacy scan are sound; one carry item (19) as written cannot be built inside the D0 lease without breaking every dispatch run test, and product-ci on the PR is red from a product flake this PR did not cause.

## Blocking Issues (Must-Fix Only)

**B1. Item 19's runtime half breaks `internal/dispatch/` at D0's head, and D0 cannot edit `internal/dispatch/`.** 10.6 item 19 (tagged `[R plus D]`) has D0 add `Invocation.GitIdentity` and says "`Prepare` refuses an empty value or one holding a control character". `Prepare` already follows that pattern for `Selection` ("Prepare refuses a zero value", `internal/runtime/runtime.go`), so a D0 builder will make an empty identity a refusal. But phase C's `internal/dispatch/run.go` builds the `runtime.Invocation` (around line 237) without any identity, and when `Deps.Runtime` is nil (every `f.run` in `run_test.go`, `host_test.go` and the `cmd/waggle/dispatch_test.go` path) it uses the real `runtime.NewClaudeAdapter` (run.go line 181). At D0's head every successful dispatch run would exit 3 at `Prepare`, so `go test ./internal/dispatch/ ./cmd/waggle/` goes red, and the one-line fix lives in `run.go`, outside the D0 lease (D-170: `launch.go`, `claude.go`, `runtime.go`, `policy.go` and tests only). The builder must either break the lease or ship red CI, which is a D-169-style escalation built into the spec. Fix in this batch, one of:
- (recommended) Item 19 runtime half: `Prepare` refuses a partly set identity or a control character, and with a zero `GitIdentity` sets the two `GIT_CONFIG_*` values but no `GIT_AUTHOR_*`/`GIT_COMMITTER_*`; the dispatch half (phase D) refuses an empty resolved identity with exit 3, before the claim (same place as item 4). The fixture agent already commits with `-c user.name=... -c user.email=...`, so phase C tests stay green under the isolated config.
- Or widen the D0 lease (section 3 row, the D0 lease line, D-170's scope note in 15.2) to the `runtime.Invocation{...}` literal in `internal/dispatch/run.go` and its test, which needs a Keeper note because D-170 names the files.

## Checks

1. Paths and authorship. 3 files, +224 / -24 (`DECISIONS.md` +156 / -0, `handoff/queen.md` +12 / -13, `tasks/T-023.md` +56 / -11), all under `.waggle/hive/`. Risk-Tier 2 by path; Keeper card by hive practice. All four commits: author and committer are the D-160 noreply, `Signed-off-by` the same.
2. Privacy. Every added line in all three files plus all four commit messages and author fields scanned. The only addresses are the D-160 noreply (commit metadata only) and one SSH remote fixture of the form `git@<forge host>:` in item 19's test. No home path, encoded home segment, `/var/folders` segment, harness temp prefix, hostname, email address, the Keeper's full name, or token shape. The handoff's lines naming the Keeper's first name and the email to-do are unchanged context from main, not added lines. The new hive-records-9 bullet describes the earlier redaction without quoting anything. Clean.
3. Decisions gate. `waggle` built from the branch tree (Go sources identical to `origin/main`, since the diff touches only `.waggle/hive/`): `waggle gate decisions --base origin/main --head origin/hive-records-10` returns `decisions: ok, appended D-170, D-171, D-172, D-173, D-174, D-175, D-176, D-177`, exit 0. Pure append (0 lines removed), D-170 follows D-169 directly, 177 `## D-` headings on the branch and 169 on `origin/main`. Field layout (Date, Kind, Tickets, Supersedes, Question, Options considered, Hive recommendation, Keeper choice, Rationale) matches D-169 in all eight; the file ends with a single newline.
4. D-170 to D-177 against the brief.
   - D-170: section 3 D0 row (`t-023d0`, items 16 to 21, `internal/runtime/`, 60 / 90 / 130, Tier 3), the D0 lease line (`launch.go`, `claude.go`, `runtime.go`, `policy.go`, tests), Depends-on ("Phase D: D0 merged") and Q-T023-20 all agree. `policy.go` left the phase D lease and the D row, consistent.
   - D-171: item 19 and Q-T023-21 match (two `GIT_CONFIG_*` values, four identity values, repo config then the Keeper's global then exit 3). See B1 and N2.
   - D-172: 9.1 adds `max_budget` to the self-mapped list; item 14 points there; Q-T023-22 matches, including no `internal/usage/` change.
   - D-173: D row 550 / 770 / 960, the estimate paragraph (190 + 35 + 80 + 60 + 70 + 115 = 550; 1.4x and 1.75x of 550 give 770 and 962, rounded to 960; phase C 1223 / 850 = 1.44 and 1223 / 700 = 1.75; 1223 - 1186 = 37 lines of fix rounds), fix rounds counted, D1 and D2 cut. See N3.
   - D-174: item 20, the 10.2 guard task lines ("do not commit; delete any probe files"), Q-T023-24. See N4.
   - D-175: item 6 and Q-T023-25. Premise checked: in this checkout `git check-ignore` matches `.waggle/worktrees/` but not `.waggle/control/usage.jsonl`. AC8 now excepts `layout.Usage` (`.waggle/control/usage.jsonl`), consistent.
   - D-176: Q-T023-26 and the 10.6 trailer route the probe to the phase D builder, failure blocks phase F. See N5.
   - D-177: item 22 and Q-T023-27 agree on `reviews/T-<nnn><suffix>-*.md` at base and counting both guards.
   - 15.2 header says D-170 to D-177 in order; the Q numbers map one to one.
5. Phase tags against the leases. Each symbol checked on main: `Config()` in `internal/runtime/claude.go` (16); `Verify` (17), `envFixed` and `allowedDefaultArgs` (18), `workerEnv` and `Prepare` (19), `workerSettings` (20) in `launch.go`; `Invocation` in `runtime.go` (19); `ErrNoCrossModel` and the alternates key in `policy.go` (21). All `[R]` items sit inside the D0 lease. `[D]` items: `hostFiles` and the `global:` keys (`check.go`), `runSession`, `tap` and its 64 KiB cap, `waggleCommands`, the comb `ExtraEnv` (set by dispatch in `run.go`, not in `internal/comb/`), `ck.common`, the claim calls, all in `internal/dispatch/`; `runtime.MaxFinal` and `runtime.MaxStreamLine` (4 MiB) exist, so items 7 and 12 need no `stream.go` lease; `layout.Worktrees` exists for item 1. The one disagreement is B1. The 10.6 intro defines `[R]` as "beyond `policy.go`", but item 21 is `policy.go` only (N6).
6. 10.6 items without a test. 18 says "No test" (comments only); 15 is a documentation move to phase E ("Zero lines"); 11 is itself three tests. Acceptable as written, but the 10.6 heading and the handoff both say every item has a test (N6). Every other item names a test.
7. Carry completeness against the handoff at `236de39`. Phase C reviews: hostFiles across runs (1), `global:` keys (2), own claim OID (3), Prepare and unix check before the claim (4), stdout holder (5), `check-ignore` (6), `tap` cap (7), git-free admin step (8), `max_budget` (14), per-turn usage (15), `waggleCommands` heading (9), `GIT_CONFIG_PARAMETERS` and `GIT_CONFIG_KEY_1+` (10), `--ignore-submodules` tests (11). C0 reviews: `Config()` clone (16), word-boundary help (17), `envFixed` and `--debug` comments (18), worker git config (19). Process-group probe: routed to D-176. Remote-mode claim-sync taint: routed to the phase F trial in the 10.6 trailer and the handoff. Phase B `.takeover` and taint marker: 13. Phase A truncated verdict: 12. hive-records-8 N5: routed to phase E. Second guard from the old phase D scope: 21. Nothing dropped.
8. Handoff facts at `185ac8d`. PR #52 is OPEN (draft) with head `hive-records-10` at `185ac8d`. `git worktree list` shows `hr10` in the queen's scratchpad on `hive-records-10`, the main checkout, and this guard's worktree only; none of the four phase C guard worktrees the base handoff named remain, and no `worktree-agent-*` branch other than this guard's exists (N7 on the count). DECISIONS line "177 entries once hive-records-10 merges (169 on main). Next ID D-178" is true. "Main at `236de39`" is true. The hive-records-9 bullet matches the merged state.
9. Next work. Item 1 (this batch before D0, so the D0 builder's worktree carries 10.6) is correct. Items 2 and 3 match D-170 and D-173 (leases, items, sizes, the 770 Keeper return, the D-176 probe). Item 4 places phase F after D, matching D-147 ("after phases A to D merge") and D-117 (phase E not a precondition). Item 5 keeps T-024 phases A to E after T-023 (D-123).
10. CI on PR #52 (linux). `verify-leases-and-tiers` fails on "commit has no T-<id>: prefix" for all four commits: the expected hive-records failure. `product-ci` FAILS in `go test -race ./...`: `TestRunHostState/new_admin_entry/plain` (`internal/dispatch/host_test.go:68`) reported "exit 0 err <nil> violations []". This PR changes no Go file, and the identical tree passed Product CI on `main` at `236de39` (push run) and on hive-records-9; 15 local repetitions on macOS pass. So it is a flake in phase C product code, not caused by this PR, but `product-ci` is a required check: rerun it and get SUCCESS before the Keeper card. See N1.
11. Dashes. Zero U+2013 or U+2014 in any added line, in the commit messages, or in this review.

## Notes (non-blocking)

- **N1 (product flake, route to phase D).** The failed case is the host-state check missing an agent-planted `worktrees/planted` admin entry on linux: one failure against six passing linux Product CI runs of the same Go code since phase C's last fix (`6d7b613`, `5faf4e2`, three hive-records-9 heads, `236de39`). A silent miss there is a security check passing a tampered host, so even if it is a test-timing artifact (for example git's detached auto maintenance pruning or touching admin entries between the agent's commit and the post-run snapshot), it should not be waved off as noise. Recommend adding a 10.6 item 23 `[D]`: reproduce `TestRunHostState/new_admin_entry` on linux with `-count` and find whether the entry or the snapshot changes; fix the cause or the fixture. Save the CI log line in `media/T-023/`.
- **N2 (item 19 resolution wording).** "Dispatch resolves it with pinned `git config user.name` ... else the Keeper's global": the pinned env sets `GIT_CONFIG_GLOBAL=/dev/null` (`worktree.go`), so a pinned call can never see the global fallback, and an unscoped `git config user.name` without the pin reads every scope. Say "pinned `git config --local`, else an explicit `--global` read" so the order D-171 sets is what gets built.
- **N3 (D1 and D2 cut).** The section 3 paragraph puts items 1 to 12, 19 and 22 in D1 and `clean` and `list` in D2, which leaves item 14 (`max_budget`, tested by test 18's ledger mapping) in D2 though the ledger is D1. Put 14 in D1. The paragraph also gives the cut without saying the build returns to the Keeper at 770; D-173 and the handoff say so, and one clause in section 3 would make the brief self-contained.
- **N4 (item 20 test).** D-174 drops both `git add` and `git commit`; the test asserts only no `Bash(git commit` rule. Assert both. Separately, 10.2 has both guards share `T-<nnn>-guard`; with Edit and Write kept, say whether the worktree is reset to `head_oid` before the second guard, so a first guard's leftover file (exit 1 under rule 3) cannot reach the second guard's review.
- **N5 (D-176 visibility to the builder).** The probe appears in 15.2 and in a 10.6 trailer that begins "Not phase D code", which a builder could read as not their job. A line under 10.2 or a phase D acceptance item ("the builder saves the Q-T023-26 probe output under `media/T-023/`") would make it a deliverable the guards check.
- **N6 (10.6 wording).** The intro defines `[R]` as needing `internal/runtime/` "beyond `policy.go`"; item 21 is `policy.go` only and is correctly in D0 under D-170. Reword to "needs `internal/runtime/` (phase D0, D-170)". The heading "(each with ... its test)" and the handoff's "22 carry items each with a test" overstate: 15 and 18 have none, by design.
- **N7 (handoff count).** "The two guard worktrees from phase C were removed": the base handoff named four (two detached and locked, two with `worktree-agent-*` branches). All four and their branches are gone, so the state is right; "the four" would match the base text.
- **N8 (small brief gaps).** 9.1's `result.json` field list lacks the `second` field that 10.2 now records. Depends-on serializes "phases C0 and E with any open `internal/runtime/` lease" but not D0. The D0 lease line still says "(Q-T023-20 default)" where "(D-170)" is now accurate.

## Confirmation at f22258f

Verdict: must-fix
Risk-Tier: 2

Scope: `git diff 185ac8d f22258f` (3 files, +64 / -14): `578c317` commits this review and a handoff worktree count; `f22258f` applies B1 and N1 to N8 to `tasks/T-023.md` and the handoff. Hive layer only, no Tier 3 path.

### B1 at 185ac8d: fixed

Item 19 now splits cleanly. Runtime half (D0): `Prepare` refuses a partly set `GitIdentity` or a control character, and a zero `GitIdentity` still sets `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1` with no `GIT_AUTHOR_*`/`GIT_COMMITTER_*`. Dispatch half (D): pinned `git config --local`, else an explicit `--global` read, else exit 3, with an empty resolved identity exit 3 before the claim (with item 4). The D0 tests and the D test "a repo with no identity at either scope exits 3 with no claim ref" are named. Checked against `origin/main`:

- `internal/dispatch/run.go` builds `runtime.Invocation{Role, TaskID, WorktreeDir, Selection, Commands}` with no identity and calls the real `ClaudeAdapter.Prepare` (`internal/runtime/launch.go`). A zero `GitIdentity` that is not refused leaves that call working, so D0 needs no edit outside its lease.
- `workerEnv` on main adds `envAllowed`, `envFixed` and the settings values. Adding the two `GIT_CONFIG_*` values changes nothing the phase C tests rely on: `newRepo` and the fixtures already `t.Setenv` `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1` (`dispatch_test.go`, `fixture_test.go`, `guard_test.go`, `cmd/waggle/dispatch_test.go`), and every fixture agent commit passes `-c user.name=a -c user.email=a@e.co` (`fixture_test.go` `commitFile`, `run_test.go` plumbing and `sign`, `fix_test.go`, `host_test.go`). The test helper `sh` sets the identity through env. No test in `internal/dispatch/` or `cmd/waggle/` asserts the worker env.
- So `go test -race ./internal/dispatch/ ./cmd/waggle/` stays green at D0's head, and the refusal lands in phase D before the claim, inside the D lease. N2's wording (`--local`, then an explicit `--global` read) is in.

### N1 to N8 as applied

- N1: item 23 `[D]` added (host-state flake, test under `-race -count=50` on linux), and it appears in the D row, the estimate paragraph ("items 22 and 23 about 115"), the D1 cut and the 10.6 intro. Fixed.
- N3: D1 now lists items 1 to 12, 14, 19, 22 and 23; the paragraph says a build past 770 before review "returns to the Keeper with this cut (D-173)". Fixed.
- N4: item 20's test asserts no `Bash(git add` and no `Bash(git commit` rule. 10.2 resets the shared guard worktree to `head_oid` and removes untracked files before the second guard. Fixed.
- N5: 10.2 makes the D-176 probe a phase D builder deliverable under `media/T-023/` that the guards check; the 10.6 trailer now opens "Other phase D work outside the carry list" and calls it a deliverable. Matches D-176 (builder runs it, failure blocks phase F). Fixed.
- N6: `[R]` reads "needs `internal/runtime/` (phase D0, D-170)"; the 10.6 heading says "its test where one applies" and the intro says 15 and 18 carry no test by design. Fixed.
- N7: the handoff says "the four guard worktrees the previous handoff named, and their branches". Fixed.
- N8: 9.1 lists `verdict` and `second` (guard); Depends-on serializes "phases C0, D0 and E" with any open `internal/runtime/` lease; the D0 lease line cites D-170. Fixed.
- Ranges: the section 3 D0 row (16 to 21) and D row (1 to 15, 19 in part, 22, 23), the estimate paragraph, the 10.6 intro, 15.2 Q-T023-20 (16 to 21) and Q-T023-27 (item 22), and the handoff's "Phase D carry" paragraph (1 to 15, 22 and 23 phase D) all agree with 23 items. One handoff line does not (C-B1).

### Handoff at f22258f

- In-flight paragraph: PR #52 is OPEN (draft) on `hive-records-10` at `f22258f`; `hr10` in the queen's scratchpad is on `hive-records-10` at `f22258f`. None of the four phase C guard worktrees or their branches remain. The `agent-*` worktrees present are this batch's first guard (detached at `185ac8d`) and this confirmation's, each with a `worktree-agent-*` branch, so "any `agent-*` worktree left ... is this batch's guard" is true. Counts read 23 in the in-flight and "Phase D carry" paragraphs.
- Stale: "Next work" item 3 (C-B1).

### Blocking

- **C-B1 (handoff next-work range).** `handoff/queen.md` "Next work" item 3 still reads "10.6 items 1 to 15, 19 in part, 22" for phase D, dropping item 23. The section 3 D row, the estimate paragraph, the D1 cut and the handoff's own "Phase D carry" paragraph all include 23, and that line is what the next queen reads when she writes the phase D task. Item 23 is a security check that passed a tampered host on linux, so it should not fall out of the dispatch list. Fix: "10.6 items 1 to 15, 19 in part, 22 and 23". One line, no other change needed.

### Non-blocking

- **C-N1 (in-flight list punctuation).** "new 10.6 with 23 carry items, each with a test where one applies; item 23 is the linux ... flake from the hive-records-10 review, 9.1 `max_budget`, AC8 ledger exception, 15.2" puts the 9.1, AC8 and 15.2 entries after the semicolon, so they read as part of the item 23 clause. Move the item 23 clause to the end of the parenthesis or into its own sentence.
- **C-N2 (guard cleanup wording).** "remove it with `git worktree remove -f -f` after merge": there are now two guard worktrees, each with a `worktree-agent-*` branch. "remove them and delete their branches" matches the state.
- **C-N3 (phase D test file name).** The D lease names a new `guard.go` with tests, but `internal/dispatch/guard_test.go` already exists on main (phase B safety tests such as `TestPlantedRepoNotReused`), as do `guard2_test.go` and `guard3_test.go`. Say whether the builder adds to it or picks another name, so the guards do not read an edit to an existing test file as outside the lease's "new ... tests". Pre-existing at `185ac8d`, not introduced here.
- **C-N4 (brief length).** `tasks/T-023.md` is 414 lines against the 400-line brief limit (413 at `185ac8d`). Carry, do not block: the overage is history of merged phases A to C (10.1, 10.5, 10.5.1, 10.5.2, the section 15 rulings), and trimming it now would churn a brief two Tier 3 builders are about to read. Trim when phase D closes, moving phase A to C detail into pointers to the phase reviews.

### Privacy and dashes

Every added line in the three files and both commit messages and author fields scanned. The only addresses are the D-160 noreply in commit metadata and the SSH remote fixture already in item 19's test. No home path, encoded home segment, temp prefix, hostname, personal address, full name or token shape. The words "email" and "full name" in the committed first review are descriptions, not values. Zero U+2013 or U+2014 in the diff, the commit messages, or this section.

### CI on PR #52

- Product CI run 39300338293 (head `185ac8d`), attempt 2: SUCCESS. The first attempt's `TestRunHostState/new_admin_entry/plain` failure did not repeat, consistent with a flake now tracked as item 23.
- Product CI run 39301474400 (head `f22258f`): SUCCESS (`product-ci` pass, 1m53s).
- `verify-leases-and-tiers` (run 39301474259) fails on "commit has no T-<id>: prefix" for all six commits: the expected hive-records failure. `relay-claim` skipped.

### Re-check after fix

Read "Next work" item 3 in `handoff/queen.md` at the new head and confirm it names items 22 and 23. Nothing else needs re-review.
