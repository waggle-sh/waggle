# Guard review: T-038 brief (polish ticket), 4791d78 on t-038-brief (base 1037e77)

Risk-Tier: 3

Reviewer: opus guard, read-only. Checked against the code at 4791d78 (product code identical to main e82da6f; main adds only the T-036 brief, its reviews, D-270 to D-277 and a WAGGLE.md lease line).

## Intent Summary

The brief splits the polish ticket into phase A (`internal/dispatch/`, `internal/runtime/`, `internal/config/`: A1 brief reviews leave `roundMustFix` per D-257, A2 an unmatched exit 4 maps to `refused`, A3 a cancel maps ExitEnv to exit 1 in `ClaimTask` and `ReleaseTask`, A4 the `waggle/T-` release branch held to 100 characters, A5 `topTier` exported, A6 `--second` refuses a record with no model, A7 `./` refused in `protected_paths`) and phase B (`cmd/waggle/`: B1 flags before the task, B2 canonical task in CLI usage errors, B3 usage paragraph moved, B4 trailer bound before split, B5 bad-branch warning on `tiers`). Blast radius: the must-fix count behind builder step-up and guard alternation, the `--second` cross-model path, claim and release exit and JSON codes, the protected-paths schema rule, and the hive gate's usage text at phase B merge. Every item is real and still open at 4791d78; the U+2028 drop is correct. Two brief defects block: the lease rule leaves no legal home for code the items require, and Q-T038-3 argues from a T-036 state that no longer holds.

## Item checks (all verified against the code)

- A1: real. `inputs.go:128` builds the prefix `T-<nnn><suffix>-`, `:136` matches any `.md` under it, capped at `maxReviews` (50, `inputs.go:22`). Brief names on base: `T-029-brief24-opus.md`, `T-031/032/033-brief-*.md`; each matches `^brief[0-9]*(-|\.md$)` after the prefix, and no build review name on base does. Step-up at `policy.go:202`, `confirm_model_unknown` case at `guard.go:189-190`. Skip before the cap, exact case, after the prefix: no undercount path found. Agrees with D-257.
- A2: real but latent. Every `failf(ExitClaim, ...)` on the claim and release paths wraps a table sentinel (`request.go:109`, `task.go:130,143`, `claim.go:87,90`, `release.go:143,176,251`); `guard.go:105` and `run.go:89` are not reached by `ErrorCode`. No existing code changes.
- A3: real. `ResolveBase` (`inputs.go:39-40,51-53`), `blobAt` (`:62-64`) and `lockRepo` (`clean.go:69-71`) wrap git errors as ExitEnv. A pre-cancelled context makes `exec.Cmd.Start` return `ctx.Err()`, so no process starts; no signal, sweep or reap is involved. D-193 does not apply, as the brief says.
- A4: real. `release.go:194` checks `"t-"+rest`, 7 bytes shorter than the branch, so up to 107 passes.
- A5: real. `policy.go:254` and `guard.go:177` are the same predicate; one caller at `policy.go:158`. `guard.go` keeps other `comb` uses, so no import change.
- A6: real in code, see N1 for severity.
- A7: real. `config.go:235` refuses `.`; `"./"` strips to `.` and `path.Clean(".")` is `.` (`:247`). `".//"` is already refused as unclean, so `./` is the only new refusal.
- B1 to B5: real at the cited lines (`claim.go:136-150,177-183,230-232`; `gate.go:69-74`, paragraph between `tiers` and `decisions`; `gate_commits.go:73-74`; `gate_verify.go:140-143`, and `print` at `:65-70` filters by check, so a `tiers` run drops the warning).
- Dropped U+2028 item: confirmed. `LC_ALL=C grep` finds bytes `e2 80 a8` on `cmd/waggle/claim_test.go:195`, `:206` and `:212`.
- Budgets: A 31 doubled to 62, target 81 (80.6), stop 112 (111.6); B 29 doubled to 58, target 75 (75.4), stop 104 (104.4). Arithmetic and D-185 planned cuts are correct; the bottom-up counts are plausible.
- Settled decisions: none reopened. A1 implements D-257, A3 matches the T-032 `interrupted` (exit 1) rule, A7 sits inside D-239.

## Blocking issues

**M1. The lease rule has no legal place for code the items require.** The rule says "only these may change" and gives each file a function scope. Under that scope:

- **A3 helper.** A3 says "One unexported helper does it for both," but `task.go` is scoped to "`ClaimTask`'s return path and `ErrorCode`" and `release.go` to "`ReleaseTask`'s return path and `checkReleaseBranch`". A new function is outside both, and the only new-helper allowance in the rule is the A1 helper in `inputs.go`.
- **Stale doc comments.** The `ClaimTask` doc comment (`task.go:38-42`, "ExitEnv for git or config trouble") and the `ReleaseTask` doc comment (`release.go:89`, "a held lock or git trouble is ExitEnv") go stale under A3, and neither comment is in scope.
- **B2 import.** `cmd/waggle/claim.go` imports only `internal/dispatch` (`:3-17`), so `comb.ParseTicketID` needs a new import line. That line is outside "`parseTaskArgs`, and the `o.fail` calls".

A builder following section 0 must stop and report, or a guard must flag the change. Fix: name the A3 helper's file and place (for example, `task.go` next to `ErrorCode`), allow the two doc comments, and allow the `comb` import line in `claim.go`.

**M2. Q-T038-3 and the lease justification argue from a T-036 that is no longer under review.** At e82da6f, T-036 is ruled (D-270 to D-277, status line `T-036.md:4`), and its section 2 says "`dispatch.Release` in `internal/dispatch` stays, unused by the CLI (residue for the polish ticket)" (`T-036.md:121`). The brief still says "G4 (T-036, brief under review)" (lines 11, 64). It recommends (a) because "The T-036 brief is not ruled, so its shape may still change". That premise would reach the Head Keeper on the card. Option (a) also sends to "a later ticket" a residue that the ruled brief hands to this one.

Fix: update the status and justification, and restate Q-T038-3 against the ruled T-036. Option (b) is now coherent: phase B already waits for G4 phase A, and G4 leases nothing in `internal/dispatch/`. If (b) is recommended, adjust phase B's lease, its section 6 entries and its budget. If (a) stays, give a true rationale and say that the ruled T-036 named this ticket.

## Notes (not must-fix)

**N1. A6 severity.** Dispatch never writes a record with an empty model. `RunDir` and `Model: sel.Model` are set only at `run.go:382-384`, after selection, and `guardRun` also requires a verdict. An empty model needs a hand-edited or corrupt record, which D-203 makes a note (run records are trusted). A6 is cheap hardening and is worth keeping. "Worse than the review noted" is true in effect but overstates reachability. It is already first in the planned cut, which is right.

**N2. A3 does not cover an explicit `--base`.** With `--base` set, `ResolveBase` tags a failed `rev-parse` as ExitUsage (`inputs.go:43,53`). A cancel during that call from `waggle claim --base X` therefore stays exit 2 `usage`, and A3's "usage errors keep their codes" keeps it there. AC5 is unaffected because `taskReq` sets no `Base` (`task_test.go:20`). Name the case in section 4.

**N3. "`ErrorCode` then gives `interrupted`" is not universal.** An ExitEnv error that wraps a table sentinel keeps the sentinel's code after mapping. For example, a claim-path `comb.ErrConcurrentUpdate` goes through `claimError` to ExitEnv; under a cancel it would become exit 1 with code `concurrent_update`. This is acceptable, but say so.

**N4. AC5 should require `.waggle/worktrees/` in the release fixture.** `newFixture` creates the directory (`TestReleaseTaskNoWorktreesDir` removes it at `task_test.go:375-377`), so `lockRepo`'s `rev-parse` is the first context call: exit 3 today and exit 1 after A3, a real test. Without the directory, the first context call is Comb `ClaimOID`, whose `gitError` carries `ctxErr` (`comb/git.go:240-246`), and `claimError` already maps that to `interrupted`. That test would pass on today's code.

**N5. AC12's release row does not exercise B2.** `ReleaseTask` sets `res.Task` before `checkReleaseBranch` (`release.go:92-97`), so `release 23 --branch feature/x --json` already returns `task` `T-023`. The only usage error B2 changes in `runTaskRelease` is `task == "" || extra > 0`. Use `release 23 24 --json` (after B1) instead, and state which ID the JSON carries for two tasks.

**N6. B2 leaves some usage errors without a task.** `ClaimTask` sets `res.Task` only after `Validate` (`task.go:45-49`), so `T-34 --dirs docs --json` (no gate, an existing "no gate" row) keeps `task` `""` while "no dirs" gains `T-034`. Optional fix in phase A (`ClaimTask` is leased): set `res.Task` from `ParseTicketID` before `Validate`. Otherwise, list the case in section 4.

**N7. Section 6 is missing some pins (it is a map, so this is a note).**

- A1: `dispatch_test.go:468` and `fix2_test.go:242` pin the per-ticket `T-023.md` count.
- A3: `TestReleaseTaskNoWorktreesDir` (`task_test.go:372`).
- A6: these reach `guardModel(second=true)` with real records that carry models: `guardrun_test.go:103` (`TestGuardSecondResetsWorktree`), `second_base_test.go:16-40` (`TestSecondDefaultBase`) and `fix2_test.go:184-236`.

All should pass unchanged. No missing breaking test was found: no existing test expects `environment` for an exit 4, a 101 to 107 character `waggle/T-` branch, or `./` accepted.

**N8. Fix-round guard count.** "Model choice" is not in D-205's list. Extending it is conservative and harmless. However, D-205 gives two guards to a round that touches claims or deletes git state, and an A4-only round changes `checkReleaseBranch`, which decides whether `ReleaseTask` deletes a claim ref. The brief's "any other round gets one" would give that round one guard. Widen the rule to any round touching `task.go` or `release.go`.

**N9. Citation.** The Risk Tier line cites D-242 for the claim and release exit codes. D-242 is the squash-merge release ruling; the code list is the T-032 brief section 2 (`T-032.md:80`).

**N10. B1 second parse.** The second `fs.Parse` can return `flag.ErrHelp` (`--dirs a T-32 -h`). Say that it exits 0 as today, and add the case to AC11.

Verdict: must-fix

## Round 2 (6cce568)

Risk-Tier: 3

Reviewer: opus confirming guard, read-only. Diff `b82ea99..6cce568` (one file, `.waggle/hive/tasks/T-038.md`), checked against the code at 6cce568 (product code identical to main e82da6f).

### Intent Summary

Round 2 closes both opus must-fixes and all three fable must-fixes: the lease rule now places the A3 helper in `task.go` next to `ErrorCode`, allows the `ClaimTask` and `ReleaseTask` doc comments and the `internal/comb` import in `cmd/waggle/claim.go`; Q-T038-3 is restated on the ruled T-036 and recommends (b), adding B6 (delete `dispatch.Release` and `legacyOwner` in phase B) with a scoped phase B lease on `internal/dispatch/release.go` and `release_test.go`; the T-031 cites are swapped to the right files. It adds A8 (`ClaimTask` sets `res.Task` before `Validate`), folds the round 1 notes in, extends section 6, and resizes the budgets. Blast radius grows by B6 only: one exported function and one struct field leave `internal/dispatch`, after G4 phase A removes their only product caller. One new contradiction blocks: the AC12 release row cannot pass under B1 as specified.

### Round 1 must-fix checks

- Opus M1, fable M3 (lease homes): closed. `task.go` scope now names `ClaimTask`'s first lines (A8), return path, doc comment at `:38-42` (verified: lines 38 to 42 are that comment), `ErrorCode` (`:152`) and the A3 helper next to it. `release.go` scope names the doc comment at `:82-90` (verified). Phase B allows one import line for `internal/comb` in `cmd/waggle/claim.go` (verified: `:3-17` imports only `internal/dispatch` today). The "no new package edge" claim holds: `gate_commits.go` already imports `comb`.
- Opus M2, fable M2 (Q-T038-3): closed. Status and justification now say T-036 is ruled (D-270 to D-277). Cites verified: `tasks/T-036.md:12` lists `internal/dispatch/` as a must-fix file "(its `Release` stays, now unused by the CLI: residue for the polish ticket)" and `:121` repeats it. T-036 leases nothing in `internal/dispatch/` (also `:167`, `:206`), so "phase A shares no directory with either G4 phase" holds. The reasoning for keeping B6 out of phase A is right: `cmd/waggle/dispatch.go:141` still calls `dispatch.Release` on base.
- Fable M1 (T-031 cites): closed. `T-031-fable.md:13` is the `./` note, `:17` the bad-branch warning, `:14-15` the `T-000:` and empty-trailer notes; `T-031-opus.md:35` is the usage paragraph, `:36` the trailer bound order, `:37` the empty trailer. All match.

### New content checks

- B6 grep: callers of `Release(` are `cmd/waggle/dispatch.go:141` (removed by G4 phase A, and B6 says stop if it remains) and `release_test.go:56` (`doRelease`). `doRelease` callers are exactly `release_test.go:73, 134, 147, 155, 164, 168, 193, 208, 214`, matching section 6. `legacyOwner` appears only at `release.go:28, 58, 128`. `ReleaseLine` callers `cmd/waggle/claim.go:244` and `task_test.go:386` are kept and listed. `io` is used in `release.go` only by `Release` (`:8`, `:57`), so the "unused import removed" clause is needed and allowed. No test reaches the `legacyOwner` fork: `newFixture` always writes `keeper_id: alice` (`fixture_test.go:104`) and `release_test.go` never removes it, so the `doRelease` rewrite keeps every assertion. Nothing still calls the code B6 deletes once G4 phase A has merged.
- A8: sound. `Validate` canonicalizes with the same `comb.ParseTicketID` (`request.go:81-85`), so the early `res.Task` equals the later `req.Ticket`. `TestClaimTaskRefusals` (`task_test.go:89-135`) checks only exit and code, and `claim_test.go:110` (`x`, `task` `""`) stays true. No existing assertion breaks.
- Phase ordering: line 17 says phase B dispatches only after G4 phase A and T-038 phase A have merged, and only when no claim covers `cmd/waggle/` or `internal/dispatch/`. Stated correctly.
- Budgets (D-185): A 6+3+10+2+2+3+5+2+2 = 35, doubled 70, target 91, stop 126. B 8+7+1+7+3+4+3 = 33, doubled 66, target 85.8 rounded to 86, stop 118.8 rounded to 119. The (a) variant (30, 60, 78, 108) is also right. Planned cuts are named for both phases, and B6 stays above the cut.
- Fix-round guard rule now covers any round touching `task.go` or `release.go` (round 1 N8). The `roundMustFix` skip is unchanged and still fail-safe: no undercount path. A6 now states its note-level severity and stays first in the cut; `--second` cannot land on the first guard's model through any path this brief opens.
- No em or en dash, home path or email in the added lines; the Keeper is named by role.

### Blocking issues

**M1. AC12's release row cannot pass under B1 as written.** AC12 expects `release 23 24 --json` to give exit 2, code `usage` and `task` `T-023` as JSON. Today, and under B1, `parseTaskArgs` (`cmd/waggle/claim.go:136-149`) takes `23` as the task because it is the first argument, then runs `fs.Parse(["24", "--json"])`, which stops at `24`. `--json` is never parsed, `asJSON` stays false, and `o.fail` (`:100-106`) prints text only, with no JSON on stdout. B1's second parse runs only "when ... no task was taken yet", so it does not apply here. A builder can meet AC12 only by also re-parsing after a second positional argument, which B1 does not specify and which changes what `T-34 T-35 --json` prints. My round 1 N5 suggested this exact row, so the error started with me. Fix: use `release --json 23 24` (the first `Parse` reads `--json` and stops at `23`, B1 takes `23` and re-parses `["24"]`, which leaves extra 1). The B2 sentence "With two task arguments the JSON carries the first one's canonical ID" should say it holds when `--json` comes before the second task. The same applies to any B1 or B2 row where a flag follows two positional arguments.

### Notes (not must-fix)

- N1. A8 also sets `res.Task` when `RepoDir` is empty, because `Validate` checks `RepoDir` before the ticket (`request.go:78-80`). This is harmless and consistent with "a valid task", but AC9b could add that row.
- N2. B6 says to update "the `ReleaseRequest` and `ReleaseTask` comments that name `Release`". Only the `legacyOwner` field comment (`release.go:28`, deleted anyway) and the `Release` doc comment (`:56`) name it. The `ReleaseTask` doc comment does not, and the A3 sentence "`Release` ... inherits it" goes stale at phase B, which is harmless. `cmd/waggle/claim.go:215` ("the same release as waggle dispatch release") is the comment G4 phase A edits.

Verdict: must-fix
