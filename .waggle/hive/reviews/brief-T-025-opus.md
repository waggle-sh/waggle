# Brief guard review: T-025 phase C finalize round (ab5e75c)

Risk-Tier: 2 (the diff is a hive brief; the phases it plans include Tier 3 phases B and D)

## Intent Summary

Commit ab5e75c rewrites `.waggle/hive/tasks/T-025.md` sections 2, 4, 5 and 6 against main `f2bbe5f`. Phase C is split into C1, a mechanical SPDX sweep of all 194 Go files under `cmd/` and `internal/` plus `local.yaml.tmpl`, and C2, the `internal/license/` rule table and repo-walk test. The order is sweep first, so no allowlist is needed and main stays green. The rule table becomes ordered first-match, and a new section 4.1 maps every tracked path to a row. Phase B's Head Keeper file list now includes `release.yml`, `scan-hive.sh`, and conditional items for the `gate/` and `CLAUDE.md` questions. Two Keeper questions are opened: Q-T025-12 and Q-T025-13. Blast radius: the brief only. Once built, C2 can turn any PR red, which the brief accounts for with two guards and dispatch checklist 2.1.

## What I verified against the tree at ab5e75c

- **Every path lands on exactly one row.** I mapped all 99 tracked paths outside `.waggle/hive/` (817 hive files are row 2 or 4) to a single row. No path falls through.
- **Counts are correct.** 194 Go files (84 non-test), `cmd/waggle/` 29, `.waggle/control/` 10, templates 13, hive 817.
- **Header states are correct.** Line 1 matches the "State today" column for every path: CODEOWNERS, ci.yml, release.yml, waggle-gate.yml and gate/action.yml are MIT-0; docs, README and CONTRIBUTING are Apache-2.0; both shell scripts start with a shebang and have no header; CLAUDE.md starts with the marker; `local.yaml.tmpl` has no header.
- **Main stays green at each merge (B, C1, C2).**
  - Every file outside the builder leases that C2 would flag is fixed by phase B: items 1 to 3, 5 and 6, plus 7 or 8 depending on the rulings.
  - No test asserts README, NOTICE, CODEOWNERS or the workflows.
  - The only SPDX assertion in tests is `internal/scaffold/gate_action_test.go:47`, and item 7 moves it in the same commit as the `gate/action.yml` change.
  - `local.yaml.tmpl` is seeded (`internal/scaffold/init.go:74`). It already opens with a `#` comment, so the YAML subset parser accepts the header. No test compares its rendered bytes: `internal/config/local_test.go:39` and `upgrade_test.go:241-252` use their own strings.
- **Go headers are buildable.** 38 files start with `//go:build`, and putting a line comment and a blank line above the constraint is valid Go and passes gofmt. The 6 files that start with a package doc comment keep it, because of the blank line. No Go file is empty or lacks a trailing newline. The two tests that read source (`internal/dispatch/misc_test.go:46` through go/parser, `setsid_test.go:45` by string index) are not affected.
- **Leases work with `--dir`.**
  - `cmd/` and `internal/` are neither Tier 3 nor protected (`internal/comb/backend.go:29-39`, `:407`). Two directories is under the D-319 limit of 5.
  - `gate/` is protected (`.waggle/control/config.yaml:53`), so leaving it in phase B is correct.
  - Bash forms in 2.2 match `internal/runtime/launch.go:80`.
- **Cites check out.** `gate_action_test.go:45-49`/`:47`, `init.go:70-76`/`:74` and `render.go:12` are accurate. D-129, D-130, D-133, D-136, D-183, D-185, D-203 to D-205, D-210/D-216, D-230, D-257, D-270, D-272 and D-319 match their cited content.

## Findings

1. **must-fix: Q-T025-13 option (a) rests on a false fact.** At `.waggle/hive/tasks/T-025.md:253` the brief says `.gitignore` "also holds only a waggle block". It does not. The tracked `.gitignore` has `bin/` above the block and four repo-local lines after it (`.claude/settings.local.json`, `.claude/worktrees/`, `.dashboard/`, `.waggle/control/scan-names.local`). Also, `.gitignore` supports `#` comments, so its exemption (row 3, carried over from the approved brief) is not about a missing comment syntax either. The Keeper would rule on a false analogy.
   - **Fix:** restate (a) on its own grounds: `CLAUDE.md` holds only a tool-managed block, and adding a header creates a header duty on a file `init` and `uninstall` edit. Optionally cite `.gitignore` only as "already exempt by row 3".

2. **note: Q-T025-12 option (c) is slightly inaccurate.** `:251` says the file's "license would be stated nowhere". Under (c) the MIT-0 line 1 would stay, so the file would still state MIT-0; it would just be unchecked and absent from NOTICE and the README. Suggested wording: "stated only in an unchecked header that NOTICE and the README contradict by omission". Otherwise the Q-T025-12 options are complete. (a) and (b) correctly list the follow-on edits: the test prefix for (a); NOTICE and the README for (b).

3. **note: C1 bottom-up is 169, not 168.** At `:41`, the 1-line header on `local.yaml.tmpl` is a non-test addition under D-136, which AC4 confirms with `1 0`. Doubled, that is 338, so estimate 340, target 360 and stop 400 still hold. One-word fix.

4. **note: AC4 conflicts with the C1 planned cut.** AC4 (`:227`) requires every Go file at the C1 base to be in "C1's diff". If the cut fires (`:41`), C1 ships part of the tree and C1b ships the rest. Suggest "C1's diff (with C1b if cut)", and make C2's dependency (`:13`) read C1 and C1b.

5. **note: `**` must match zero segments.** Row 2's `internal/scaffold/templates/**/*.json.tmpl` (`:115`) catches `devcontainer.json.tmpl` and `roster.json.tmpl` only if `**/` can match zero segments. If a builder implements one-or-more, those files fall to row 6 and fail as an unknown extension. Section 5 (`:165`) says only "`**` across segments". The per-row representative test in `rules_test.go` would catch it, but stating "`**/` matches zero or more segments" in 5 saves a fix round.

6. **note: Check should read only the header line.** `internal/decisions/render.go:12` holds an MIT-0 SPDX string. `gate_action_test.go:47` holds one under rulings (b) and (c). `rules_test.go` will hold wrong-identifier fixtures. The brief implies line 1 or 2 only (`:106`) but never says Check ignores later occurrences. Add "Check reads only line 1 (line 2 after a shebang)" to section 5.

7. **note: CRLF and empty-file behavior is unspecified.** `:43` and `:167` list CRLF and empty files as cases but give no expected outcome. Say whether a CRLF header line is rejected or accepted after stripping `\r`, and that an empty checked file is an error.

8. **note: one tracked file has mixed line endings.** "No CRLF files today" (`:159`) holds for every checked path, but `git ls-files --eol` shows one mixed-EOL file at `.waggle/hive/media/T-023a/fable-go-test-push-result.txt` (row 4, not checked). Harmless; qualify the sentence as "no checked file".

9. **note: repo-walk skip condition.** `:166` skips the test when `git` or `.git` is absent. A module copy sitting inside some other git checkout would make `git ls-files` list that outer repo. Under D-203 this is a note only; checking that `git rev-parse --show-toplevel` equals the module root closes it.

10. **note: Q-T025-13 option (b) side effect.** Under (b), `waggle uninstall` in this repo (`internal/scaffold/uninstall.go:110`, `RemoveBlock`) would leave a `CLAUDE.md` holding only the header line. Worth one clause in (b).

11. **note: phase B item 7 overlaps C1's lease while C1 runs.** It edits `internal/scaffold/gate_action_test.go`, inside C1's `internal/` lease. The hunks are far apart (line 1 vs line 47), so the merge is clean as `:173` says, and the Head Keeper commit merges with an admin bypass. Checklist 2.1 item 2 speaks only of claims, so it may be worth a line saying B can land during C1.

Verdict: must-fix

# Confirming brief guard: T-025 round 2 (0108799 on `t-025c-brief`)

Risk-Tier: 2 (hive brief only; it plans Tier 3 phases B and D and Tier 2 builder phases C1 and C2)

## Intent Summary

Commit 0108799 changes one file, `<repo>/.waggle/hive/tasks/T-025.md` (+28/-32). It applies the two rulings: D-333 makes `gate/` Apache-2.0, with `gate/action.yml` and the `gate_action_test.go:47` assertion switched in the phase B commit, and D-334 exempts `CLAUDE.md`. Row 5 is folded into row 3, phase B item 7 becomes unconditional and item 8 is dropped. It also restates Q-T025-13 so it no longer relies on the false `.gitignore` analogy, and folds in the round 1 guard notes. Blast radius is the brief only. No code, control file or other record changes in this commit. The D-333 and D-334 entries are uncommitted appends in `DECISIONS.md` and match how the brief applies them.

## What I checked

- **Opus finding 1 (must-fix) is fixed.** Q-T025-13 (`T-025.md:250`) now rests on its own grounds and no longer mentions `.gitignore`. Every factual claim in it holds against the tree:
  - `CLAUDE.md` is three lines: start marker, `@.waggle/control/WAGGLE.md`, end marker.
  - It is a hardcoded Tier 2 path (`internal/comb/backend.go:51`).
  - `waggle init` writes it through `UpsertBlock` (`internal/scaffold/init.go:197`) and `waggle uninstall` removes it through `RemoveBlock` (`internal/scaffold/uninstall.go:255`).
  - "Uninstall would leave a file holding only the header" is correct. `planUnblock` removes the file only when what is left is whitespace (`uninstall.go:263`).
  - No false claim is left anywhere in the file.
- **Folded notes are handled correctly.**
  - Opus 3: C1 bottom-up is now 169, doubled to 338 (`:41`).
  - Opus 4: AC4 and C2's dependency now cover C1b (`:228`, `:13`, `:59`), and AC6 now lists C1b (`:230`).
  - Opus 5 and fable 3: section 5 now says `**/` matches zero or more segments, with the `roster.json.tmpl` example, and there is a test case for it.
  - Opus 6: `Check` reads only line 1, or line 2 after `#!`.
  - Opus 7: a CRLF header line is rejected and an empty checked file is an error.
  - Opus 8 and fable 2: the EOL sentence is now limited to checked files (`:158`). `git ls-files --eol` confirms the only mixed file is `.waggle/hive/media/T-023a/fable-go-test-push-result.txt`.
  - Opus 9: the repo walk skips the test when `git rev-parse --show-toplevel` is not the module root.
  - Opus 11: phase B may land during C1, stated at 2.1 item 2, Depends and section 6.
  - Opus 2 (the wording of the old option (c)) is moot: that option text was replaced in the ruled form of the question. The Q-T025-12 alternative at `:249` uses the suggested "contradict by omission" wording.
- **The rulings are applied the same way in every section.**
  - Status (`:4`), Depends (`:13`), read-only context (D-333 and D-334 added) and section 1 (`:27`) all agree.
  - In the 2 Leases paragraph, `gate/` goes to phase B under D-333.
  - In the row table, row 3 now holds `CLAUDE.md` and row 11 is `Apache-2.0 | product code (D-333)`.
  - In inventory 4.1, `CLAUDE.md` maps to row 3 (exempt) and `gate/action.yml` to "phase B (D-333)".
  - Phase B item 7 now gives the exact new prefix, and `CLAUDE.md` gets no change.
  - Checklist 2.1 item 1, AC4 and AC6 agree, and section 9 says no question is open.
  - No stale "open", "per Q-T025-1x", "(a)/(b)/(c)" or "item 8" reference is left.
- **Main stays green at each merge in the order B, C1, (C1b), C2.**
  - **Phase B** switches `gate/action.yml` line 1 and `gate_action_test.go:47` together. That line is the only SPDX assertion in Go (`git grep SPDX -- '*.go'` finds only that line and the `render.go:12` const).
  - **Phase B during C1.** Phase B is a Head Keeper commit, not a claim. Its only file inside C1's lease is `gate_action_test.go`. C1 inserts two lines above line 1 (`package scaffold`), and phase B edits line 47. The hunks share no context, so they merge cleanly, and both sides stay green on their own.
  - **C1 and C1b.** No check exists before C2, so these merges cannot turn main red.
  - **C2** is dispatched from a base that already has B, C1 and C1b, so every offender outside the builder leases is already fixed by B items 1 to 3 and 5 to 7. `CLAUDE.md` and `.gitignore` are exempt under row 3.
- **Row numbering.** Rows run 1 to 4, then 6 to 13. The note at `:110` explains the gap. No reference anywhere in the file points at row 5 or at a renumbered row: row 12's "caught by row 2" and every row number in 4.1 are still correct. The file has no em or en dashes and no home-directory paths.

## Findings

1. **note: Q-T025-12 no longer records the MIT-0 option the Keeper was shown.** `T-025.md:249` now lists "Default: Apache-2.0" and "Alternative: exempt". The question was asked with three options, and D-333 records MIT-0 (named in NOTICE and the README) as option 2. The brief's account of the question should match the decision log. Add "Alternative: MIT-0, with NOTICE and the README naming `gate/`" next to the exempt alternative.

2. **note: the split arithmetic mixes raw and doubled figures and still says 168.** `:45` reads "about 168 + 270 = 438 doubled". 168 (now 169) is C1's raw bottom-up and 270 is C2's doubled estimate. Raw that is 169 + 135 = 304, and doubled 338 + 270 = 608. The conclusion (over a 400 stop) holds either way, so only the sum needs fixing. This line was carried over from round 1.

3. **note: the top-level skip can hide failures in a checkout reached through a symlink.** The new skip condition at section 5 compares `git rev-parse --show-toplevel`, which prints a path with symlinks resolved, against the module root. On macOS a checkout under `/tmp` resolves to `/private/tmp`, so a plain string comparison skips the test there, silently. CI on Linux is unaffected. Under D-203 this is a note. Telling the builder to compare after `filepath.EvalSymlinks` closes it.

4. **note: AC6 omits C2b.** AC6 (`:230`) now covers C1b but not C2b, the planned C2 cut at `:43`. Add "C2b (if cut)" so both cuts are treated the same.

No must-fix found.

Verdict: pass
