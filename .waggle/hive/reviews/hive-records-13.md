# Review: hive records 13 (PR #60, `hive-records-13` at `5802482`, base main `915d416`)

Verdict: must-fix
Risk-Tier: 3

## Intent Summary

Records batch after T-023 phase D2 (merged `915d416`, #59). It appends D-195 to D-205 to `DECISIONS.md`, adds both D2 guard reviews (`reviews/T-023d2-{opus,fable}.md`) and 135 media files under `media/T-023d2/` (builder runs, guard probes, mutant and fix-removal outputs), refreshes the committed queen summary for D2, and rewrites `tasks/T-023.md`: phases A to D1 trimmed to pointers into `git show c98d130:...`, plus the D2 scout brief (10.7) and its questions (15.3), at 381 lines. No product code. Blast radius: what the next D2b, E and F sessions read as the T-023 spec and the queen's standing facts. Threat bar D-203 applied.

## Blocking Issues (Must-Fix Only)

### M1. `tasks/T-023.md` lands the pre-ruling D2 brief on main as if D2 had not started

The brief in this PR is the D2 scout brief as written before the Keeper ruled. Nothing in the 381 lines mentions D-195 to D-205, `915d416` or #59. On main after this merge it would tell a session:

- line 4 (Status): "Q-T023-28 to Q-T023-31 in section 15.3 await the Keeper" and "Next: Keeper rulings on 15.3, then dispatch phase D2". All four were ruled (D-196, D-197, D-195, D-198) and D2 is merged.
- line 376: "### 15.3 Phase D2 questions (open; ...)".
- line 16: phase E lease lacks `cmd/waggle/dispatch.go`, which D-196 adds.
- line 209 (10.2): `list` "lands in D2 or phase E (Q-T023-28)"; line 211: "D2's proof plan is Q-T023-29".
- line 271 (10.7.3 item 5): `clean` refuses duplicate admin entries and runs `git worktree prune`. D-202 reversed this: clean never runs a repo-wide prune and the duplicate check is gone. A D2b builder told to "give provision's prune the same treatment" (D-202) and reading item 5 as the spec of clean gets the opposite design.
- line 268 (item 2): the macOS clause text, replaced by D-199's sentence.
- line 272 (item 6): `--host-record` reads as D2 scope; it is D2b (D-196 cut, D-198).
- line 231 (10.6): "Open: item 13 (`clean`, phase D2)"; item 13 landed.
- line 60 (section 3): "a fix round under 30 added lines gets one confirming guard (D-184)"; D-205 replaced that rule.
- lines 289 and 310: "Colima is 2 CPU and 2 GB and shared with the Keeper's Supabase stacks". The hive now uses the `colima-waggle` context (2 CPU, 4 GiB, confirmed by `colima list`), as the queen summary says. A D2b builder following the brief would run on the Keeper's default Supabase VM.
- 10.4 step 4 (phase F trial): no D-197 probe 2 shape with `survivors_killed`, no D-199 `cmd >/dev/null 2>&1 &` command or report sentence. The queen summary has them; the phase F spec does not.

That steers future sessions wrongly, which is the must-fix bar for a records PR.

**Fix (scout or queen edit in this PR, stays under 400 lines):**

1. Line 4: replace "Scout brief for phase D2 written 2026-10-20 (section 10.7; Q-T023-28 to Q-T023-31 in section 15.3 await the Keeper)." with "**Phase D2 Done:** merged at `915d416` (#59), 347 added non-test lines against a stop of 350 (D-190), reviews `reviews/T-023d2-*.md`; Q-T023-28 to Q-T023-31 ruled 2026-10-20 as D-196, D-197, D-195 and D-198, with D-199 and D-202 changing items 2 and 5 (section 10.7)." Replace "Next: Keeper rulings on 15.3, then dispatch phase D2." with "Next: phase D2b on the D2 lease (carry list in the `915d416` merge commit), then phase F."
2. After line 15 add "- **Lease (rw), phase D2b:** the D2 lease (10.7.2) plus `worktree.go` (`provision`'s repo-wide prune only, D-202)." Line 16: append "`cmd/waggle/` (`dispatch.go`, `dispatch_test.go`: `list`, D-196)" to the phase E lease.
3. Line 209: "it lands in D2 or phase E (Q-T023-28)" becomes "it lands in phase E (D-196)". Line 211: "D2's proof plan is Q-T023-29" becomes "the remaining proof is the first phase F macOS trial run (D-197)".
4. Insert at the top of 10.7 (after line 235): "**Merged at `915d416` (#59). Changes from the plan below, which stays as the scout wrote it:** item 2's macOS clause is D-199's sentence; item 5's duplicate refusal and repo-wide `git worktree prune` were replaced by D-202 (clean removes only its own ticket's entries and never prunes; D-200 and D-201 are superseded); item 6 and its CLI moved to D2b (D-196 cut, D-198 two-step digest); D2b also removes `Worktree.provision`'s repo-wide prune (D-202) and takes opus N20 and the open notes in both D2 reviews. Containers now run on `docker --context colima-waggle` (2 CPU, 4 GiB), never the default `colima` context; this replaces the Colima sentence in 10.7.5 and section 11."
5. Line 376: "(open; the scout's recommended default first)" becomes "(ruled 2026-10-20; every default approved: Q-T023-28 D-196, Q-T023-29 D-197, Q-T023-30 D-195, Q-T023-31 D-198)".
6. Section 3 table, D2 row: "350 (D-190)" becomes "350 (D-190); merged at 347". Line 60: append "D-205 replaced the 30-line rule: one alternating confirming guard unless the round touches signals, deletes, leases or claims or adds more than 100 non-test lines."
7. Line 231: "Open: item 13 (`clean`, phase D2)" becomes "Item 13 landed in D2 (`915d416`, D-202). Open:".
8. 10.4 step 4: append "The first macOS trial run repeats the D-176 probe 2 shape and must list the tool shell and its background child in `survivors_killed`, or cutover is blocked (D-197); it adds a `cmd >/dev/null 2>&1 &` tool command, and the trial report carries the D-199 sentence."

## Checks

- **DECISIONS.md:** pure append (one hunk `@@ -3720,0 +3721,216 @@`, zero removed lines), D-195 to D-205, every entry has Date, Kind, Tickets, Supersedes, Question, Options, Hive recommendation, Keeper choice and Rationale. `go run ./cmd/waggle gate decisions --base origin/main --head HEAD`: "decisions: ok, appended D-195 ... D-205", exit 0. 205 `## D-` headings, so the summary's "205 entries, next D-206" is right.
- **Keeper choices against what shipped:** D-202 records `Supersedes: D-200, D-201` and says so in its text, so the D-200/D-201 history is handled in the record itself, no note needed. Code on main matches: `clean.go` has no `prune` call and no `CLocale` (D-202), `worktree.go:210` still prunes (D2b, D-202), `cell_unix.go` uses `Setsid: true` (D-197), the `cell.go` NotEnforced macOS clause is D-199's sentence word for word. D-196 (list to phase E, D2b cut), D-198 (carried to D2b) and D-195 match the `915d416` merge message.
- **T-023.md pointers:** every commit the brief names exists (`c98d130`, `037ba4f`, `782e608`, `53941e6`, `2ecd449`, `b2cb0b0`, `12a4372`, `3fff262`, `23bdf75`, `5faf4e2`, `6d7b613`, `afcc9c3`); `git show c98d130:.waggle/hive/tasks/T-023.md` is the 414-line text the trim points to, and its sections 10.1, 10.5, 10.5.1, 10.6, 11, 14, 15 and 15.1 exist. Every `reviews/T-023*.md` named exists, and `media/T-023/d176-probe-2.txt` exists. 381 lines, under 400. Nothing a later phase needs was cut except what M1 lists as missing from the D2 outcome; the phase E and F sections and the 10.6 open items survive.
- **Queen summary:** matches main. D2 `915d416` is #59 (merged, `gh pr view 59`), 347 of 350, D-196 to D-205 summaries are right, the `colima-waggle` profile is 2 CPU and 4 GiB and the global context is `colima` (`docker context ls`, `colima list`), the merge allow rule `Bash(env -u GITHUB_TOKEN -u GH_TOKEN gh pr merge:*)` is in `.claude/settings.local.json`, which is gitignored (`.gitignore:9`). The D-176/D-197 sentence is accurate. The stale D1 open notes C-N3 and C-N4 are rightly dropped (C-N4 is closed by the trim). The scan-hive next-work item matches the script, which also scans merges by remerge diff while WAGGLE.md section 4 says "each non-merge commit".
- **Privacy and dashes:** `scan-hive.sh origin/main HEAD` exit 1 with six email hits, all `x@example.invalid` in `opus-confirm3-clean-probes.txt:9` and `opus-confirm4-clean-probes.txt:9,158` (net diff and commit `a67310c`), as the queen expected; one HOLD for `tasks/T-023.md` (a modified hive file, expected for this batch). Other emails in the added files are `k@example.com` in fable probe scripts and the noreply address. No `/Users/`, `/var/folders/`, the machine hostname or the session scratchpad path in media, reviews, brief or summary; the only temp paths are linux `/tmp/Test...` dirs and literal `/private/var` in prose. No U+2013 or U+2014 in the diff or either commit message. Both commits are authored and committed by the noreply address and carry `Signed-off-by: paulternate <27632860+paulternate@users.noreply.github.com>`. `scan-names.local` was copied in, is ignored, and is not committed.
- **Media:** every `media/T-023d2/` file name referenced in both D2 reviews exists (82 names extracted; the only unmatched ones are Go source file names, not media).

## Notes (non-blocking)

- N1: D-201 to D-205 carry `Date: 2026-10-21` but say "Decided ... on 2026-10-20". That follows the appender's UTC date (`internal/decisions/append.go:88`) against the Keeper's local day; correct as recorded.
- N2: D-203 and D-204 repeat D-202's question and options word for word (one card, three rulings). Accurate, if repetitive.
- N3: `reviews/T-023d2-opus.md:371` reads "Verdict: pass. Risk-Tier: 2 (unchanged)." on one line, so it does not match `^Verdict: (pass|must-fix)$`, and "unchanged" contradicts the Risk-Tier 3 lines of every earlier opus section. The merge commit says opus Tier 2. Leave the review as written; a future must-fix counter (D-177) only counts must-fix lines, so nothing breaks.
- N4: the summary's Colima lock is `scratchpad/colima.lock`, which is per session. A worker from another session does not share it. Name a fixed path if cross-session runs ever overlap.
- N5: the summary's "tenth session" and "phase F on 2026-10-22, about two Keeper hours" are not checkable from the records; no issue if they come from the Keeper.
