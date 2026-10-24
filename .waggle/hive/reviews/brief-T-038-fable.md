# Brief review: T-038 (polish ticket), `.waggle/hive/tasks/T-038.md` at `4791d78` on `t-038-brief`, base `1037e77`

Risk-Tier: 3

Reviewed read-only in the brief's worktree. Main is now `e82da6f` with the T-036 brief merged (D-270 to D-277); the diff between the brief's base and main is hive files only (`DECISIONS.md`, `tasks/T-036.md`, the two T-036 brief reviews), so no code cite in the brief moved.

## Intent summary

T-038 closes twelve review notes from T-031, T-032 and T-034 plus the D-257 product fix, in two dispatched phases: phase A in `internal/dispatch/`, `internal/runtime/` and `internal/config/` (A1 to A7), phase B in `cmd/waggle/` (B1 to B5) after G4 phase A merges. The hive runs the pinned binary for dispatch and builds the gate from the PR base, so phase A changes nothing for the hive until the next pin move and phase B changes only gate usage text and where one warning is filed. Tier 3 by content is right: A1 changes the count that drives the builder step-up and guard alternation (D-177, D-205), A6 touches the `--second` cross-model check (D-188), and A7 tightens a key of a hardcoded Tier 3 file.

## What I verified

Every item is real, still open at `1037e77`, and specified against the code as it stands. Evidence per item:

- A1: `roundMustFix` at `internal/dispatch/inputs.go:127-143` matches `prefix = req.Ticket + <branch suffix> + "-"` (`:128`) and counts any `.md` under it (`:135`), so `T-031-brief-opus.md` and the other old names in `.waggle/hive/reviews/` (`T-031-brief-*`, `T-032-brief-*`, `T-033-brief-*`, `T-029-brief24-opus.md`, all present) count today. D-257 (`DECISIONS.md:4939`) assigns the fix to this ticket. The pattern `^brief[0-9]*(-|\.md$)` applied after the prefix gives the AC3 table exactly: `T-023-briefing-opus.md` and `T-023-Brief-opus.md` count, `T-023-brief.md` and `T-023-brief24-opus.md` do not, and under branch `t-023-brief` the prefix is `T-023-brief-` so `T-023-brief-opus.md` counts. Skipping before `files++` keeps the 50-slot cap (`maxReviews`, `inputs.go:22`) for build files.
- A2: `ErrorCode` (`task.go:150-172`) falls through to `environment` for an `ExitClaim` error that wraps no table sentinel. The two such errors in the package, `refuseDirs` (`run.go:89`) and `guardTarget` (`guard.go:105`), are reached only from `Run` (`run.go:197`, `:333`), never from `ClaimTask` or `ReleaseTask`, so "no such error exists today" holds for the two CLI verbs. Every other `failf(ExitClaim` site wraps a listed sentinel or a `taggedError`.
- A3: `ClaimTask` wraps git failures as `ExitEnv` (`task.go:62`, `ResolveBase`, `blobAt` at `inputs.go:62-64`) while `claimError` maps `context.Canceled` to `ExitFailed` (`claim.go:93-94`). The brief is right that a killed `exec.Cmd` reports `signal: killed` and not `ctx.Err()`, so the `ctx.Err() != nil` gate on return is the only reliable mapping. `Run` stays untouched (lease rule and Q-T038-2 (a)).
- A4: `checkReleaseBranch` (`release.go:189-198`) checks `"t-"+rest`, which is 7 bytes shorter than the `waggle/T-` name, so 107 passes while the message at `:195` says 100.
- A5: `topTier` at `policy.go:252-256`, caller `:158`, restated inline at `guard.go:177`. No test names it.
- A6: `guardModel`'s second branch (`guard.go:166-173`) appends `f.Model` to `AvoidModels` and discards the record name (`f, _, okF` at `:167`); `checkSecond` compares `model == f.Model` (`:257`). With an empty `f.Model`, nothing avoids the first guard's real model. Record names are `T-<nnn>-guard-<n>` (`latestRun`, `guard.go:129`), as AC8 says.
- A7: `validProtectedPath` (`config.go:231-251`) refuses `.` at `:235` but the clean check at `:247` runs on the trailing-slash-stripped entry, and `path.Clean(".")` is `.`, so `./` passes.
- B1: `parseTaskArgs` (`claim.go:136-150`) takes the task only as `args[0]` or `fs.Arg(0)` after one `Parse`, so `--json T-32 --dirs x` leaves three extras. The re-parse is sound: `flag.FlagSet.Parse` may be called again on the same set.
- B2: `o.fail("", ...)` at `claim.go:177-183` and `:230-232`.
- B3: the `--branch` paragraph sits at `gate.go:69-74` between `tiers` and `decisions`.
- B4: `strings.Split` at `gate_commits.go:73` runs before the length check at `:74`; `strings.Count(raw, ",")+1` equals `len(strings.Split(raw, ","))` for a non-empty field, so results are identical. `gate_trailers.go:55` reaches the same function.
- B5: `rep.warn(checkLeases, "branch name ignored")` at `gate_verify.go:140-143`; `check` is in scope (`:144`). `runGate` passes the literal `"verify"` as `check` (`gate.go:46-47`), so "under `tiers` when the check is `tiers`, else `leases`" leaves `verify` output byte for byte unchanged, as the brief says.
- Dropped U+2028 item: confirmed dropped correctly. `cmd/waggle/claim_test.go:195` holds bytes `e2 80 a8` after `\x1b[31m`, and the `ContainsAny` sets at `:206` and `:212` end with the same bytes.

Section 6: every cited range exists and pins what the brief says it pins (`task_test.go:475-496`, `:170-181`, `:318-370`; `claim_test.go:95-124`, `:136-138`, `:163-170`; `guardmodel_test.go:134-172`, `:212-258`; `review_marker_test.go:131-151`; `guardrun_test.go:142-170`; `fix2_test.go:167-236`; `policy_avoid_test.go:21`, `:68-100`; `config/gate_test.go:10-61`; `cmd/waggle/gate_test.go:216-241`; `gate_bind_test.go:87-145`; `gate_approve_test.go:217-224`, `:326-351`). "Expected to break: none" holds: the usage table at `claim_test.go:95-108` asserts only exit and code, so B2's `task` field breaks nothing; `TestErrorCode` has no `ExitClaim`-without-sentinel row; no dispatch test cancels a context (grep for `WithCancel` and `cancel()` in `internal/dispatch/*_test.go` returns nothing); the only `waggle/T-` names in tests are `T-023`, `T-023-x`, `T-024` and `T-031`; every `guardModel` call in `guardmodel_test.go:140-198` uses records with models set; no test calls `trailerTickets` or `parseTaskArgs` directly. No missing breaking test. No test pins today's `tiers --branch <bad>` silence, so B5 has nothing to break and AC15 is the only pin needed.

Lease and lease rule: phase A shares no directory with either G4 phase; phase B waits for G4 phase A and dodges G4 phase B. `run.go`, `request.go`, `claim.go`, `worktree.go` and `internal/comb/` excluded is right (`ParseTicketID` is read only). See M3 for two gaps in the per-file parts.

Budgets: bottom-up 31 and 29 are plausible for the shapes described; doubled and rounded figures are right (62, 81, 112 and 58, 75, 104 at 1.3x and 1.8x). The cuts keep the items that change results (A1 to A4, A7; B1, B2, B5) and drop the no-behavior-change ones, which is the right order.

Threat model: the guard sentence covers the three must-fix classes the queen named (undercounting, `--second` landing on a used model, exit code drift). D-193 does not apply as written: the tests cancel before the call, so `exec.Cmd.Start` refuses to run git and no process exists to signal.

Settled decisions: none reopened. A1 is what D-257 ordered and narrows D-177 only for names matching the pattern. A2 adds a value under the version 1 additive promise. A3 does not touch D-242. A7 sits under D-239. B1 widens T-032's "task first" without changing any accepted form. B5 departs from the T-031 brief's wording (the fable note at `T-031-fable.md:17` says the leases placement "matches the brief's wording"); that was a brief spec, not a ruling, and the queen's summary lists it as polish.

## Blocking issues (must-fix only)

**M1. Four source cites name the wrong T-031 review, and one more in section 4.** A7 cites `reviews/T-031-opus.md:13` (first note) and B5 cites `T-031-opus.md:17` (fifth note); both notes are in `T-031-fable.md:13` (`./` passes validation) and `:17` (bad-branch warning filed under leases). `T-031-opus.md:13` is the `CheckChanges` paragraph and `:17` is the policy-source check. B3 cites `T-031-fable.md:35` (note 1) and B4 `T-031-fable.md:36` (note 2); the fable review is 22 lines long, and those notes are `T-031-opus.md:35` (usage paragraph) and `:36` (trailer bound order). Section 4 cites `T-031-opus.md:14-15` for the `T-000:` subject and empty-trailer notes; those are `T-031-fable.md:14-15` (the opus review has the empty-trailer note at `:37` and no `T-000:` note). The items themselves are correctly specified; the fix is to swap the file names. It matters because the Read-only context sends the builder to "the source reviews named in section 2".

**M2. Q-T038-3 rests on a premise that is no longer true and contradicts the merged T-036 brief.** The option (a) rationale says "The T-036 brief is not ruled, so its shape may still change". T-036 is ruled (D-270 to D-277, merged at `e82da6f`), and its lease rule and section 2.8 both name `dispatch.Release` as "residue for the polish ticket", that is, this one (`tasks/T-036.md:12`, `:121` on main). Option (a) hands it to "a later ticket that leases `internal/dispatch/`", so as written the two briefs each leave the residue to the other. The question stays open for the Head Keeper, but the card must show the current state: drop the "not ruled" sentence, say that (a) declines the hand-off T-036 made and name where the residue is then tracked (the queen summary's Next work list), or switch the recommendation to (b). Facts for the card: `Release` has one product caller, `runRelease` at `cmd/waggle/dispatch.go:141`, which G4 phase A deletes; tests reach it only through `doRelease` at `internal/dispatch/release_test.go:50-58`; `legacyOwner` (`release.go:28`, `:58`, `:128`) is a live exit-code fork (`ExitEnv` against `ExitUsage`) that nothing will reach once `runRelease` is gone. Either option is defensible on cost; my lean is (b), because phase B already waits for G4 phase A and the deletion is about 17 product lines plus one test helper swap, but that is the Head Keeper's call.

**M3. The lease rule leaves two necessary edits outside its named parts.** (i) A3 says "One unexported helper does it for both" but names no file for it. `task.go` is allowed for "`ClaimTask`'s return path and `ErrorCode`" and `release.go` for "`ReleaseTask`'s return path and `checkReleaseBranch`"; the natural home, `request.go` next to `failf` and `ExitCode`, is a must-fix file. Name the file (`task.go` beside `ErrorCode` fits). (ii) B2 calls `comb.ParseTicketID`, and `cmd/waggle/claim.go` does not import `internal/comb` today (`claim.go:3-16` lists `internal/dispatch` as its only internal import), so the import block changes; the phase B rule allows only `parseTaskArgs` and the two `o.fail` call sites in that file. Say the import is allowed. Both are one-clause fixes; without them a guard applying the rule as written ("only these may change") has grounds to fail an otherwise correct round, and the builder's own rule says to stop and report.

## Notes (not must-fix)

**N1. A6 severity.** The consequence is stated correctly (with an empty first guard model, `checkSecond` compares against `""` and the second guard can land on the first guard's real model), but the brief should not read as a live hole. Dispatch writes `result.json` only after a successful `Resolve`, with `Model: sel.Model` (`run.go:384`), so an empty `model` needs a hand-edited or pre-schema record in the host-local runs directory. Under D-203 that capability already lets the holder rewrite both records, so this is hardening, which is still worth the five lines. The parallel plain-guard case (builder record with empty model: `checkBuilderModel` compares against `""` and `Resolve` avoids only the policy's builder model) has the same prerequisite and the same three-line shape; section 4 leaves it out deliberately, which is fine, but the asymmetry is worth one clause so a guard does not raise it as a gap.

**N2. A2 wording.** "No such error exists today" is true for `ClaimTask` and `ReleaseTask` only; the package has two (`run.go:89`, `guard.go:105`), both reachable only from `Run`. Saying "reachable from `ClaimTask` or `ReleaseTask`" closes the question before a guard asks it.

**N3. Q-T038-1 (a) and Q-T038-2 (a).** Agree with both. (a) for Q-T038-1 is the fail-safe direction: an unknown name still counts, so neither the D-177 step-up nor the D-205 alternation can be undercounted by a new naming habit; (b) would drop every pre-T-033 review and (c) depends on the policy's model names. One edge the brief could state: a brief review carrying a phase suffix (`T-036a-brief-opus.md` on `t-036a`) is also skipped, since the skip applies after the phase prefix. (a) for Q-T038-2 keeps the change out of `run.go`, whose cancel path reaps processes (D-193); the cost (dispatch exits 3 on a pre-session cancel) is what T-032's fable note already described as today's behavior.

**N4. A1 source line.** The parenthetical "the T-034 pair before the rename" next to `brief-T-034-opus.md` and `brief-T-034-fable.md` reads as if those are the old names; they are the renamed pair (both present under the new names on `main`). Harmless; one word ("now renamed") fixes it.

**N5. Section 6 B5.** The brief could add that no test pins today's silence on `tiers --branch <bad>`, so nothing breaks and AC15 is the only pin; it says this implicitly through "verify and leases output is byte for byte unchanged".

**N6. Risk-tier paragraph.** "Fix rounds: a phase A round that touches `roundMustFix`, `guardModel` or the cancellation mapping gets two confirming guards (D-205: leases, claims and model choice)" is stricter than D-205, which lists signals, deletes, leases, claims and the 100-line metric and not model choice. Stricter is fine; attributing "model choice" to D-205 is loose. Say "D-205 plus model choice for this ticket".

**N7. Hive effect paragraphs.** Both are right: dispatch is pin `97127fc` (D-269), the gate builds from the PR base, and the only phase B change a hive `verify` run shows is usage text. Phase A's three-directory lease is justified by one line each in `internal/runtime/` and `internal/config/`.

Verdict: must-fix

## Round 2 (6cce568)

Risk-Tier: 3

Confirming guard on the fix round, `.waggle/hive/tasks/T-038.md` at `6cce568` on `t-038-brief`, rebased onto `e82da6f` (round 1 is `b82ea99`). Read-only; the diff is one hive file, 43 added and 34 removed lines, no product code. Product code at `6cce568` is identical to `e82da6f`.

### Intent summary

Round 2 closes the three round 1 must-fixes (lease rule homes, Q-T038-3 against the ruled T-036, T-031 cites) and folds in the notes: new item A8 (`ClaimTask` sets `res.Task` before `Validate`), new item B6 (`dispatch.Release` and `legacyOwner` deleted in phase B under Q-T038-3 (b)), phase B's lease widened to `internal/dispatch/release.go` and `release_test.go`, phase B now waits on T-038 phase A, section 4 and 6 additions, and budgets A 70/91/126 and B 66/86/119. Blast radius unchanged from round 1: Tier 3 by content, hive effect nil at phase A merge (pinned dispatch, D-269) and usage text only at phase B merge.

### Must-fixes closed, checked against the code

- **M1 (T-031 cites).** All swapped correctly. `T-031-fable.md` is 21 lines: `:13` is the `./` note (A7), `:14-15` are the `T-000:` and empty-trailer notes (section 4), `:17` is the bad-branch warning under leases (B5). `T-031-opus.md` is 40 lines: `:35` is the usage paragraph (B3), `:36` the trailer bound order (B4), `:37` the repeated empty-trailer note, cited as such in section 4.
- **M2 (Q-T038-3).** Rewritten against the ruled T-036: the "not ruled" sentence is gone; the lease justification and the question both cite `tasks/T-036.md:12` (phase A lease rule) and `:121` (section 2.8), and both lines on `e82da6f` carry "residue for the polish ticket". The question now recommends (b), with (a) stated as leaving the residue to the queen summary's Next work list. B6's facts hold: `Release` has exactly two callers, `cmd/waggle/dispatch.go:141` (`runRelease`, deleted by G4 phase A) and `release_test.go:56` (inside `doRelease`); `legacyOwner` appears at `release.go:28`, `:58`, `:128` and nowhere else; `ReleaseLine` is called from `cmd/waggle/claim.go:244`, `release.go:66` and `task_test.go:386`, so keeping it is right; the only import `Release` alone uses is `io` (`io.Writer` at `:57`; `fmt` stays used at `:79`, `:143`, `:176`, `:195`), matching "an import the deletion leaves unused removed"; `grep keeper_id|local.yaml` in `release_test.go` returns nothing, so no test pins the `legacyOwner` fork; `ReleaseTask` sets `res.Task` at `:97` before `checkReleaseBranch` at `:99`, so the `release 23 --branch feature/x` row is correctly dropped from AC12. The nine `doRelease` callers the brief lists (`:73`, `:134`, `:147`, `:155`, `:164`, `:168`, `:193`, `:208`, `:214`) are exactly the grep result. B6 cannot delete code something still calls: phase B waits for G4 phase A, and the brief adds "if phase B finds `runRelease` or any other caller of `Release` still on base, it stops and reports", with "`Release` deleted while a caller remains on base" in the must-fix list.
- **M3 (lease rule homes).** (i) The A3 helper now has a named home: `task.go`, a new unexported function next to `ErrorCode` (`task.go:152`), with the reason `request.go` (home of `failf` at `:37` and `ExitCode` at `:43`) is excluded. (ii) `cmd/waggle/claim.go` may gain one import line for `internal/comb`; today its only internal import is `internal/dispatch` (`claim.go:16`), and `cmd/waggle` already imports `internal/comb` in `gate_commits.go:11` and six other files, so "no new package edge" is true. The two doc comments are now in scope at the right lines: `ClaimTask`'s at `task.go:38-42` (function at `:44`) and `ReleaseTask`'s at `release.go:82-90` (function at `:91`).

### New content

- **A8.** Real and correctly placed. `ClaimTask` sets `res.Task = req.Ticket` at `task.go:49`, after `Validate` at `:45-48`. `Validate` canonicalizes through `comb.ParseTicketID` (`request.go:81`), and `task.go` already imports `internal/comb` (`:10`), so setting `res.Task` from `ParseTicketID` before `Validate` gives the same ID with no import change. The lease rule names "`ClaimTask`'s first lines for A8". AC9b pins it; `TestClaimTaskRefusals` (`task_test.go:89`) and `claim_test.go:110` are the right unchanged pins.
- **A2, A3 wording.** "Reachable from `ClaimTask` or `ReleaseTask`", the explicit `--base` case (`ResolveBase` sets `code = ExitUsage` when `explicit`, `inputs.go:43`, and uses it at `:53`) and the sentinel-under-cancel note match the code.
- **Section 6.** Every new cite resolves: `dispatch_test.go:468` and `fix2_test.go:242` write `reviews/T-023.md`; `task_test.go:372` is `TestReleaseTaskNoWorktreesDir`; `guardrun_test.go:103` is a `Second = true` run; `second_base_test.go:16` is `TestSecondDefaultBase`; `claim_test.go:149` is `TestReleaseCLI`. The "expected to break" paragraph now names the one compile break (`doRelease`) and says no assertion changes, which the helper's described body supports.
- **Depends on.** Phase B waiting on T-038 phase A is stated, with the reason (both phases touch `release.go`), and the claim check now covers `internal/dispatch/` as well as `cmd/waggle/`.
- **Budgets.** Bottom-up A 35 (31 plus 2 doc comments plus 2 for A8), B 33 (29 plus 1 import plus 3 for B6). Doubled per D-185: A 70, 1.3x is 91, 1.8x is 126; B 66, 1.3x is 85.8 rounded 86, 1.8x is 118.8 rounded 119. The (a) alternative (B 30, 60, 78, 108) is also right. Planned cuts keep A8 and B6 and still drop the no-result-change items first.
- **Fix-round guard count.** Any round touching `task.go` or `release.go` gets two confirming guards; `roundMustFix` and `guardModel` rounds get two under "D-205 plus model choice, a rule of this ticket". That is stricter than D-205 and no longer misattributes model choice to it. A B6 fix round touches `release.go`, so it gets two.
- **Risk Tier cite.** The exit-code list is now cited to `tasks/T-032.md` section 2, `:80`, which is the `error.code` order list; D-242 is gone from that clause.

### Contradictions

None found. Section 4 keeps `dispatch.Release` out of scope only under Q-T038-3 (a), consistent with B6 under (b). Status line, layer line, lease lines, Q-T038-3, section 6, AC12b and section 9 all agree on (b) as the recommendation and (a) as the alternative. Phase A still shares no directory with either G4 phase. Hygiene: no em or en dash, no home path, no email, Keeper by role.

### Notes (not must-fix)

**N1. Phase B heading.** Section 2's `### Phase B (`cmd/waggle/`)` at line 46 still names one directory while B6 lives in `internal/dispatch/release.go`. The layer, lease and lease rule lines all say so, so nothing is misleading on a card. One parenthetical would align the heading.

**N2. `release_test.go` imports under B6.** The lease rule allows "the `doRelease` helper at `:50-58` only". `bytes` is imported for that helper alone (`bytes.Buffer` at `:55`); `context` is used elsewhere (`:35`, `:80`). If the builder rewrites the helper to collect output in a `strings.Builder` or a slice, `bytes` goes unused and the import block changes outside the allowance. The described body (print each warning, then `ReleaseLine`) keeps a buffer, so a builder who follows it is fine; one clause ("the helper keeps `bytes.Buffer`, or the import block may drop `bytes`") closes the gap a guard might otherwise raise.

**N3. "Comments that name `Release`".** In `release.go` the only comment naming `Release` outside its own doc comment is the `legacyOwner` field comment at `:28`, which goes with the field. The allowance is harmless; the builder should expect to find nothing else to update.

**N4. "Any other round gets one".** D-205's remaining clauses (signals, file or git-state deletes, more than 100 added non-test lines) still apply hive-wide through WAGGLE.md. The brief's rule is additive and does not say otherwise, but "any other round gets one" reads as a complete rule. Appending "(subject to D-205)" would avoid a reading that the brief narrows it.

Verdict: pass

## Round 3 (a79d9fe)

Risk-Tier: 3

Confirming guard on the round 3 fix, `.waggle/hive/tasks/T-038.md` at `a79d9fe` on `t-038-brief`, diff `6cce568..a79d9fe`. Read-only; the diff is one hive file, nine changed lines, no product code. Product code at `a79d9fe` is unchanged from `e82da6f`.

### Intent summary

Round 3 closes the opus round 2 must-fix (the AC12 release row could not produce JSON under B1) and folds the round 2 notes from both guards: the AC9b empty `RepoDir` row, B6's comment wording, the unused `io` import named, A3's "until B6 removes it", the phase B heading, the `bytes` import allowance in `release_test.go`, and "subject to D-205" on the fix-round guard count. Blast radius unchanged: Tier 3 by content, no hive effect at phase A merge, usage text only at phase B merge.

### Opus M1 (AC12 release row), traced against `cmd/waggle/claim.go:136-149` under B1

- `release --json 23 24`: `args[0]` is a flag, so no task is taken; the first `Parse` sets `--json` and stops at `23` with two arguments left. B1's second parse applies (arguments left, no task taken): task `23`, re-parse `["24"]` stops at once, extra 1. `task == "" || extra > 0` fires, `o.fail` gets `ParseTicketID("23")` = `T-023`, `asJSON` is true: exit 2, code `usage`, task `T-023`, JSON. The row now passes.
- `release 23 24 --json`: `23` is taken first, `Parse(["24", "--json"])` stops at `24`, task already taken so no second parse, `--json` never read, text output. The AC12 explanation is right.
- B2's sentence: `--json T-34 T-35` and `T-34 --json T-35` both give JSON with `T-034` (the first parse, or the only parse, reads `--json` before stopping at the second positional); `T-34 T-35 --json` prints text, today and after B1 (today: task `T-34`, `Parse` stops at `T-35`, extra 2, `asJSON` false). "When `--json` comes before the second task" is the correct condition, and "as today" holds. AC11's "`T-34 T-35` in any position is exit 2" agrees.

### Folded notes, checked

- AC9b empty `RepoDir`: `Validate` checks `RepoDir` at `request.go:78-80` before `ParseTicketID` at `:81`, and `ClaimTask` calls `Validate` as its first statement (`task.go:45`), so with ticket `34` and empty `RepoDir` A8 sets `res.Task` `T-034` and the error is exit 2. Row is correct.
- B6 comment wording: in `release.go` the word `Release` appears outside its own doc comment (`:56`) only in the `legacyOwner` field comment (`:28`); no comment says `waggle dispatch release` except `:56`. "No other comment in `release.go` names `Release`; the `ReleaseTask` doc comment does not" is true. B6's change sentence and the lease rule agree ("no other comment needs an update").
- `io` import: `io.Writer` at `:57` is the only use; `io/fs` is a separate import and stays. Correct.
- A3 "until B6 removes it": consistent with B6 under (b); harmless under (a).
- Phase B heading now names `internal/dispatch/release.go` under Q-T038-3 (b), matching the layer, lease and lease rule lines.
- `bytes` allowance: `bytes.Buffer` at `release_test.go:55` is the only `bytes` use; the clause lets the import block drop it only if the helper rewrite stops using it. Correct and narrow.
- Guard-count sentence: "subject to D-205 (signals, file or git-state deletes, more than 100 added non-test lines)" matches the D-205 clauses in WAGGLE.md section 4 and no longer reads as a narrowing.

### Contradictions

None found. Status, layer, lease, lease rule, B2, B6, section 6 B1 cite (`claim_test.go:136-138`, two tasks stays exit 2) and AC11, AC12, AC12b agree. Hygiene: no em or en dash, no home path, no email, Keeper by role.

### Notes (not must-fix)

- N1. The Status line still ends at "Brief round 2"; round 3 is recorded only in the commit subject. Cosmetic.
- N2. The `ClaimTask` doc comment is at `task.go:39-43` (the function is at `:44`); the brief and both earlier reviews say `:38-42`, which starts on the blank line and misses `// fails.`. A builder finds the comment by the function; no effect on the build.

Verdict: pass
