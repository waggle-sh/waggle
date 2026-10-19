<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: hive-records-8 (PR #48, head 0b184e5)

Verdict: must-fix
Risk-Tier: 2

## Intent Summary

Hive-layer PR, base `main` at `a06d1b7`, three commits, all authored and committed by the D-160 noreply address. `ab13325` appends D-162 to D-168 to `.waggle/hive/DECISIONS.md` (Keeper rulings on Q-T023-13 to Q-T023-19, every queen default approved). `8fbb728` rewrites `.waggle/hive/tasks/T-023.md` after a scout readiness check of phase C: a new phase C0 for the D-137 `default_args` allowlist (stop 130), phase C resized to 700 / 850 / 1000 with guard fix rounds counted toward the stop, a section 10.5 carry list, a much wider 7.2 host-state snapshot (worktree admin dir, `info/`, global git config, pinned git config for post-run calls), a tainted-worktree marker, tests 20 to 27, AC11 to AC13 and section 15.1; it also commits the previous batch's guard review `reviews/hive-records-7.md`. `0b184e5` rewrites `.waggle/hive/handoff/queen.md`. Nothing under `cmd/`, `internal/`, `go.mod` or `.github/`; nothing ships. Blast radius: the brief is what the phase C0 and C builders receive from `origin/main`, and the handoff is what a fresh queen starts from. The decisions and the brief are sound; the handoff rewrite dropped a live open-notes paragraph and carries three facts that are wrong against the repo or against this PR's own decisions, and the brief's "every carry item is listed" claim misses one item from the opus phase C requirements list.

## Blocking Issues (Must-Fix Only)

**B1. The handoff rewrite deleted the live "Open notes carried" paragraph, not just stale sixth-session text.** That paragraph sat in the eighth-session Live state section on `a06d1b7` and nothing replaces it. A fresh queen loses:
- D-143's procedure: every `waggle upgrade --apply` rewrites `waggle-gate.yml` from the stale template until T-008, so revert it right after and run the upgrade in a clean worktree. Phase F moves the pin through `waggle upgrade`; only "T-008 (ends the D-143 revert)" survives, which no longer says what the revert is.
- The pre-public redaction list (media/T-023a, hostname and `/var/folders` probe output, media/T-014b p1 to p6, home paths in several reviews, the email address in reviews/hive-records-4 and commits, history rewrite as its own card). `reviews/hive-records-7.md` N4, committed in this same PR, says "add both to the handoff's pre-public redaction list"; that list no longer exists.
- D-151 symlinked CLAUDE.md follow-up; T-025 phase C before any release (the new text says only "T-025 B then C" off the critical path); T-014b fable F6 and F8 for the next `cmd/waggle/` lease (F8 is the stale `main.go:90` usage line, and phase C leases one `main.go` usage line, so it is relevant now); RFC v0.2 Traditional Equivalent column undecided.
Fix: restore the paragraph under Live state (adding the hostname and temp prefix from hive-records-7 N4 to the redaction list).

**B2. The handoff puts remote claim refs at phase C, against D-119 and this PR's D-168.** Standing facts: "no claim refs on the remote until T-023 phase C, T-014 F10". `forge.remote: origin` lands at phase F (D-119, brief 10.4 step 3), and D-168 moves the D-159 proof to phase F for exactly that reason. Fix: "until T-023 phase F sets `forge.remote` (D-119, D-168)". Otherwise a queen will expect `verify-leases-and-tiers` to start passing after phase C merges.

**B3. The handoff restates the size metric as D-089's superseded reading.** "D-089 sets the metric (physical non-test lines)" is a new gloss added in this rewrite. D-136 replaced that reading with added non-test lines (`git diff --numstat` additions over non-test files), which T-023 section 3 and D-164 ("D-136 metric") use for the C0 stop of 130. Fix: "D-136 sets the metric (added non-test lines)".

**B4. The handoff's in-flight state is false at the commit that carries it.** It says "Running: nothing. No open PRs. No worktrees; `main` is the only local branch" and "Uncommitted in the main checkout for hive-records-8". At `0b184e5` the batch is committed on branch `hive-records-8`, checked out in a scratchpad worktree `hr8`, with PR #48 open. Fix as hive-records-7 did: "In flight: hive-records-8 (PR #48, branch `hive-records-8`, worktree `hr8` in the queen's scratchpad): D-162 to D-168, T-023 brief, this handoff, `reviews/hive-records-7.md`. Then Keeper card, merge with `--admin`."

**B5. One item from opus's "Phase C requirements carried from this review" is not in section 10.5, while the status line says every one is.** `reviews/T-023b-opus.md`, last bullet of that list: "Still open from earlier rounds: the OID-level base-equals-branch check, and the env and config residuals of note 4 in the first review." The base check and the config residual are placed (section 4, 7.2). The env residual is not: dispatch's git env keeps `GIT_CONFIG_PARAMETERS`, `GIT_CONFIG_COUNT` / `GIT_CONFIG_KEY_n` / `GIT_CONFIG_VALUE_n` and `GIT_EXEC_PATH` from the Keeper's environment, and 7.2's pinned config (`GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`) does not drop them, so "only the repo config the file step just verified can name a filter" does not strictly hold. Fix, either way is one line in 10.5.2: have post-run git calls also drop those variables, or record it as deliberately not carried (Keeper's own environment, not agent-writable) so the status-line claim is true. Opus's companion suggestion (a test keeping the dispatch and comb scrub lists in sync) can go in the same line.

## Checks

1. Paths. Only `.waggle/hive/DECISIONS.md`, `.waggle/hive/handoff/queen.md`, `.waggle/hive/reviews/hive-records-7.md` and `.waggle/hive/tasks/T-023.md` change (+337, -58). Nothing under `.waggle/control/`, `cmd/`, `internal/`, `go.mod`, `go.sum` or `.github/`. `.waggle/hive/**` is Tier 2 minimum by the hardcoded config comment; no Tier 3 path, hence Risk-Tier 2 (the Keeper card is still the hive practice for record batches).
2. Decisions gate. `waggle` built from this worktree (`a06d1b7`): `gate decisions --base origin/main --head origin/hive-records-8` returns `decisions: ok, appended D-162, D-163, D-164, D-165, D-166, D-167, D-168`, exit 0. Pure append (136 added, 0 removed). 168 `## D-` headings, contiguous 1 to 168, no duplicates. Each new entry has Date, Kind (all valid `decisions.Kind` values), Tickets `T-023`, Supersedes, Question, Options, Hive recommendation, Keeper choice and Rationale, laid out exactly like D-160 and D-161.
3. Decisions against section 15.1, in order Q-T023-13 to 19 = D-162 to D-168:
   - D-162: allowlist `--max-budget-usd=<n>` and `--debug`, each also in the pinned `--help`; `--effort` stays refused (D-134 refuses it, checked). Matches Q-T023-13 default and 10.5.1.
   - D-163: `--no-session-persistence` and `--disable-slash-commands` as fixed argv, with the resume cost stated. Matches Q-T023-14 and fable phase A recommendations 3 and 4.
   - D-164: C0 estimate 60, target 90, stop 130, lease `internal/runtime/launch.go`; C 700 / 850 / 1000; fix rounds count; a crossing round goes to the Keeper before it is written. Matches Q-T023-15, the section 3 table and section 3's text.
   - D-165: unsigned worker commit fails rule 1, `git commit -s` in task lines. Matches Q-T023-16, 5.3 and 7.2 rule 1.
   - D-166: build everywhere via `cell_unix.go` / `cell_other.go`, run on linux and macOS, exit 3 elsewhere. Matches Q-T023-17 and 10.5.2 item 7. See N2.
   - D-167: one run per repo via `dispatch.lock`, parallel builders serialize after phase F. Matches Q-T023-18 and 10.5.2 item 8 (Kind `scope` for a question tagged `process`; decide has no `process` kind).
   - D-168: the D-159 remote proof moves to the phase F trial; phase C proves it locally. Matches Q-T023-19 and the 10.5 closing paragraph. See N1.
   Every entry is worded as a card ruling recorded by the queen, dated 2026-10-18, consistent with the brief's "ruled the same day".
4. Brief against existing decisions. D-100 (two guards, Head Keeper): Risk Tier line unchanged, C0 is Tier 3 in the table. D-123: untouched. D-135: still stated in the status line. D-137: C0 is the precondition, C depends on C0 merged, AC11. D-147: 10.4 step 3 repeats "only after phases A to D merge". D-161: section 3 cites 867 against 580 and the 3.3x overrun (867 / 260 = 3.33). D-136: section 3's metric matches. D-114 / D-139: `comb.CheckChanges` is on main (`internal/comb/changecheck.go:40`, merged at `3063955`), so dropping the comb lease is right. No contradiction found. Code names the brief relies on exist: `refusedArgs`, `helpTokens`, `Verify`, `Prepare`, `ErrRefusedArg` in `internal/runtime/launch.go`; `config.Adapter` really never fills `DefaultArgs` (`internal/config/runtimes.go:19`); the tripwire today rewrites only `https://`, `ssh://` and `git@` (`launch.go:79-81`).
5. Carry coverage. `reviews/T-023b-fable.md` final carry list items 1 to 6, F3, F7 and N7a to N7c; `reviews/T-023b-opus.md` notes 2, 8, the shared-config residual, the N4 residue and its phase C requirements bullets 1 to 3 all map to a 10.5.2 item and a test. Phase B items already closed in phase B (stale-lock race, FIFO lock, relative gitdir F1, `runs` symlink, `EqualFold` placeholder) are correctly absent. T-023a carries (settings copy, `no_verdict`, tripwire gaps, `go test -exec`, `claude --version`, Windows argv limit, exit 1 on `error_max_turns`, non-blocking `stop`) are placed. T-014b fable F5 (a) and (b) are 7.2 rule 2. Missing: B5; non-phase-tagged leftovers in N5 and N6.
6. Status line. Consistent with 15.1 (Q-T023-13 to 19 ruled, D-162 to D-168) and with "Next: dispatch phase C0". Its claim of full carry coverage is the B5 gap.
7. Handoff facts. Pin `6a4886eb16e0de1d005a4ba354e72ecd1dd63756` matches `.waggle/control/config.yaml` line 9. `CLAUDE.md` is tracked (blob in the branch tree). D-084 (Team plan), D-098 (ruleset, `--admin`, auto-merge off), D-131 (`dco` required check), D-142 (brief sweep to new role names), D-147, D-123, D-158, D-160 (noreply address) say what the handoff says. D-089: B3. Hashes for T-022b, T-023b and `a06d1b7` (#47) match the merged records. "161 entries on main, next ID D-169" is right. The three open hive-records-7 notes it lists (N1, N2, the fable header pointer) are still open and accurately described (`tasks/T-014.md` line 153 still says screenshots; `reviews/T-023b-fable.md` line 7 still points at `ab68fb6`). Removed and still needed: B1. Wrong: B2, B3, B4.
8. Privacy. Every added line scanned. No `/Users/`, `/home/`, encoded `-Users-` path, real uid or hostname; the only `/private/tmp/` occurrences are the `claude-<uid>` placeholder and a bare prefix in `reviews/hive-records-7.md`. The only email address is the D-160 noreply (and `keeper_id paulternate`, the public GitHub login, already on main). "email" appears as a bare word only. No tokens. One small residue: N10.
9. Dashes. Zero U+2013 or U+2014 in any added line.

## Findings (non-blocking)

- N1 (D-168 Supersedes): D-168 records `Supersedes: none`, but it moves D-159's "T-023 phase C" placement; D-108 set the precedent of recording a partial supersede (`Supersedes: D-083`). The PR is unmerged, so re-recording is possible; or accept it, since D-168's title names D-159. Related, still open: `tasks/T-014.md` AC7 still says screenshots (hive-records-7 N2).
- N2 (D-166 CI step): "CI adds a Windows cross-compile check" needs a `.github/workflows/**` edit, Tier 3 and outside every builder lease. Test 27 already allows "a test or CI step"; tell the phase C builder to do it as a Go test (or plan a Head Keeper workflow commit), so the decision is not read as in-lease work.
- N3 (15.1 format): section 15 entries carry "**Ruling (D-xxx):**" lines; 15.1 relies on the header's "D-162 to D-168 in order". Correct as written; one ruling clause per bullet would match the rest of section 15.
- N4 (stale sentence): section 3, "If the Keeper wants smaller Tier 3 reviews, the CLI file is the clean cut (Q-T023-15)", reads as open after D-164 chose C0 then C without a CLI split.
- N5 (phase A stream leftovers, not scheduled anywhere): sonnet phase A N4 (one mismatched field drops a whole event), N5 (fall back to the per-message usage sum when `result` has no `usage`; clamp or null a negative `total_cost_usd`, which reaches `cost_usd` in phase C's `result.json` and the phase D ledger; a result line over 4 MiB ends `error`), N10 (env names cannot be verified from `--help`), fable phase A recommendation 7 (`ls` allowed without a rule, one doc line). They were not phase-tagged, so not strictly phase C carry, but they are now in no brief section. `stream.go` is the phase E lease; list them under 10.2 or 10.3.
- N6 (phase B leftovers): opus phase B note 7 (`internal/dispatch/worktree.go:398` still says run records live outside the worktree "so the agent cannot rewrite them", false without a Cell) and fable phase B N6 (assert no claim ref in the refusal loop; add the real `.waggle/worktrees` assertion in `Run`). Phase C already leases `worktree.go` for carry items; both are one-liners.
- N7 (phase F routing, made concrete here): 10.4 step 2, edited in this PR, routes "fable N1 and N2, sonnet note 3" into the queen template edit. hive-records-7 N1 found fable N1 is the `planUpgrade` Go error prefix, which that Head Keeper commit's scope (`templates/roles/queen.md`) does not cover. The handoff keeps N1 open; fix it when the phase F brief is written.
- N8 (tripwire claim): 10.5.1 adds `pushInsteadOf` prefixes `/` and `.`, and says "absolute and relative local path remotes are caught". A bare relative remote (`bare.git`, `sub/bare.git`) matches neither prefix. Either narrow the sentence to `./` and `../` forms or have the builder show such a remote fails too; the test list says only "a relative path bare repo".
- N9 (attribution): "From T-014b fable F5 (and sonnet N2)" means `reviews/T-014b-sonnet.md` N2, while every other "sonnet N2" in the brief means `reviews/T-023a-sonnet.md` N2. Name the file.
- N10 (privacy residue): `reviews/hive-records-7.md` N4 quotes `/var/folders/<tmp>/.../T/`; the first segment is the real first segment of the Keeper's per-user temp directory. Low identifying value; replace with `<tmp>` or add it to the restored redaction list (B1).
- N11 (handoff wording): "sonnet builder, two guards (D-100)" while D-100's text says one guard on sonnet plus a third model; practice since pin `6a4886e` is opus plus fable (D-156), which the handoff's first paragraph explains. One clause ("opus and fable, D-156") would stop a fresh queen re-reading D-100 literally. "Builders blow line budgets by 1.5x to 2.6x" undersells T-023 phase B's 3.3x, which the brief's section 3 now plans around.

## Intent Packet

- What: D-162 to D-168 (T-023 phase C readiness rulings, every queen default); T-023 brief rewritten for phase C0 (D-137 allowlist, two fixed argv flags, tripwire gaps, stop 130) and phase C (target 850, stop 1000, section 10.5 carry list, wider 7.2 host-state snapshot with pinned git config, tainted worktree marker, `dispatch.lock`, sign-off rule, tests 20 to 27, AC11 to AC13, section 15.1); hive-records-7 guard review committed; queen handoff tidied. Hive only.
- Why: lands the brief and rulings on `origin/main` so the phase C0 builder's worktree carries them, and gives a fresh queen a current handoff.
- Risk: Tier 2 (`.waggle/hive/**` only), Keeper card and `--admin` merge by hive practice (leases and tiers fail by design, no claim). No code changes. After B1 to B5, the handoff matches the repo and the decisions, and the brief's carry claim is true.

Verdict: must-fix (B1 to B4: handoff text; B5: one line in T-023 section 10.5.2)

## Re-review at 4007e6e

Verdict: pass
Risk-Tier: 2

Scope: `git diff 0b184e5 origin/hive-records-8` (one commit, `4007e6e`: handoff, T-023 brief, `reviews/hive-records-7.md`, this review file), plus the whole PR diff against `origin/main` (`a06d1b7`) for regressions. Four commits, all authored and committed by the D-160 noreply address. Only `.waggle/hive/DECISIONS.md`, `handoff/queen.md`, `reviews/hive-records-7.md`, `reviews/hive-records-8.md` and `tasks/T-023.md` change; nothing under `.waggle/control/`, `cmd/`, `internal/`, `go.mod` or `.github/`. DECISIONS.md is untouched by the fix commit (168 headings, D-162 to D-168 as reviewed).

### Blocking issues

- B1 fixed. "Open notes carried" is back under Live state: D-143 (upgrade rewrites the gate workflow until T-008, revert right after, clean worktree), the pre-public redaction list now naming the machine hostname, temp-dir prefixes and `/var/folders` paths (hive-records-7 N4 folded in), D-151, T-025 phase C before any release, T-014b fable F6 and F8 with F8 (`main.go:90`) routed to the phase C builder, and the RFC v0.2 column. A new paragraph carries this review's open notes N1 (accepted), N5, N6 and N7, each described correctly. `internal/dispatch/worktree.go:398` still holds the stale "so the agent cannot rewrite them" comment N6 names.
- B2 fixed. Standing facts: "until T-023 phase F sets `forge.remote`, D-119, D-168, T-014 F10". D-119 and D-168 say exactly that.
- B3 fixed. "D-136 sets the metric (added non-test lines)"; range now 1.5x to 3.3x with T-023 phase B as the 3.3x case (867 / 260). Matches D-136's Keeper choice and T-023 section 3.
- B4 fixed. In-flight line names PR #48, branch `hive-records-8`, worktree `hr8` in the queen's scratchpad, the contents, this review's must-fix round, then Keeper card and `--admin`. True at `4007e6e`.
- B5 fixed. 10.5.2 item 10 attributes the residual to opus's phase C requirements bullet, names `GIT_CONFIG_PARAMETERS`, `GIT_CONFIG_COUNT`, `GIT_CONFIG_KEY_<n>`, `GIT_CONFIG_VALUE_<n>` and `GIT_EXEC_PATH`, and has every 7.2 post-run git call and the pre-launch `status` snapshot drop them (case-insensitive, prefix match for the numbered pair). Test 28 covers the canary and the list sync. Identifiers checked: `gitEnv` and `redirectVars` exist in `internal/dispatch/worktree.go` (lines 52 to 66), `scrubbedGitEnv` in `internal/comb/git.go` (line 181, unexported, so the `go/parser` route is right given phase C has no comb lease). Both lists today hold the same 15 names, so the sync assertion passes on main. The claim that the pinned `GIT_CONFIG_GLOBAL` / `GIT_CONFIG_NOSYSTEM` do not drop these variables is correct. The status line's "every phase C carry item is listed" is now true.

No remaining must-fix.

### Notes addressed

- N2: 10.5.2 item 7 and test 27 now make D-166's cross-compile check a Go test in `internal/dispatch`, not a workflow edit.
- N4: section 3's stale CLI-split sentence replaced with "D-164 ruled C0 then C with the CLI inside phase C".
- N8: tripwire claim narrowed to absolute, `.`, `./` and `../` paths; bare relative paths (`bare.git`, `sub/bare.git`) are named as not caught and listed in the doc comment; test list now says `../`. The 7.2 rule 4 backstop holds for a push into the main repo (ref snapshot). A push to some other local bare repo by a bare relative name stays uncaught; the brief says so plainly, and it is local, not egress.
- N9: "`reviews/T-014b-sonnet.md` N2" named.
- N10: `/var/folders/<tmp>/.../T/` in both `reviews/hive-records-7.md` N4 and this file.
- N11: "two guards (opus and fable, D-100 as practiced since D-156)"; D-156 records opus plus fable for T-023 phase B.
- N3 (15.1 format) not addressed; cosmetic, fine to leave.

### New notes (non-blocking)

- R-N1 (test 28 wording vs item 10): item 10 adds five config variables to what post-run calls drop, and test 28 requires dispatch's `redirectVars` to equal comb's `scrubbedGitEnv`, which keeps config variables on purpose (its doc comment: "Credential, config, trace and PATH variables are kept"). So the builder must put the five in a separate list applied only to the 7.2 calls and the pre-launch `status`, not add them to `redirectVars`. The sync test would fail loudly if they did, so this self-corrects; one sentence in the phase C dispatch prompt saves a fix round.
- R-N2: `GIT_EXEC_PATH` is dropped only from 7.2 calls, so the run's other host calls (provision, fetch, claim) still honor it. That matches the opus residual's scope (post-run inspection under verified config) and is fine; mention it if a guard asks.

### Probes

1. Dashes: zero U+2013 or U+2014 in the 413 added lines of the whole PR diff.
2. Privacy: no `/Users/`, `/home/`, encoded `-Users-` segment, real uid, `claude-<uid>` with a real number, the machine hostname (`hostname -s`, 0 hits), or a real `/var/folders/` segment in any added line. The only email address is the D-160 noreply; "email" appears only as a bare word.
3. Handoff facts: main at `a06d1b7` matches `origin/main`; 168 entries once merged, 161 on main, next D-169; pin, D-084, D-098, D-131, D-136, D-142, D-143, D-147, D-151, D-156, D-158, D-160, D-119, D-168 say what the handoff says; T-022b, T-023b and #47 hashes unchanged from the first review's check. No line was lost in the fix commit beyond the five rewritten ones.
4. Regression spot-check of the full PR against `origin/main`: the decisions, the brief edits reviewed at `0b184e5` and the hive-records-7 review are unchanged except the fixes above; no new contradiction with D-100, D-123, D-135, D-137, D-147 or D-161.

### Intent Packet

- What: D-162 to D-168 (T-023 phase C readiness rulings, every queen default); T-023 brief rewritten for phase C0 (D-137 allowlist, two fixed argv flags, tripwire gaps stated honestly, stop 130) and phase C (target 850, stop 1000, section 10.5 carry list now complete including the git config env residual, wider 7.2 host-state snapshot under pinned config, tainted worktree marker, `dispatch.lock`, sign-off rule, tests 20 to 28, AC11 to AC13, section 15.1); hive-records-7 and hive-records-8 guard reviews committed; queen handoff tidied with open notes restored and facts corrected. Hive only, nothing ships.
- Why: lands the brief and rulings on `origin/main` so the phase C0 builder's worktree carries them, and gives a fresh queen a handoff that matches the repo and the decisions.
- Risk: Tier 2 (`.waggle/hive/**` only); Keeper card and `--admin` merge by hive practice (`verify-leases-and-tiers` fails by design, no claim until phase F). No code changes. Opus guard: must-fix at `0b184e5` (B1 to B5), pass at `4007e6e`.

Verdict: pass
