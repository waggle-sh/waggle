<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: hive-records-7 (PR #47, head 73c16f6)

Verdict: must-fix
Risk-Tier: 3

## Intent Summary

Hive-layer PR, base `main` at `3fff262`, seven commits. One Head Keeper commit (`8b706bd`) adds section 4 Hive Overrides to `.waggle/control/WAGGLE.md`: until T-023 phase F our queen keeps writing the committed `.waggle/hive/handoff/queen.md` (checked by the Stop hook) and ignores the template's `.waggle/control/handoffs/` path (D-158). The rest are records: D-156 to D-161, T-014 AC7 evidence in `media/T-014/` (check-run and annotation JSON for scratch PRs #44 and #45) with AC7 ticked, T-022 and T-023 status lines for phase B, the T-021b (x2), T-022b and T-023b (x2) guard reviews, `media/T-023b/` probe drivers and output, and a handoff refresh for the eighth session. 32 files, 2598 insertions, 20 deletions, all under `.waggle/hive/**` except the three WAGGLE.md lines. Blast radius: the queen's handoff behavior, and (once `waggle dispatch` runs) every worker prompt, since `ReadInputs` injects WAGGLE.md whole. Nothing ships.

## Blocking Issues (Must-Fix Only)

**B1. Username and home path in the handoff (privacy, repo goes public).** `.waggle/hive/handoff/queen.md`, "In flight" line, names the worktree by its full scratchpad path under `/private/tmp/claude-redactedredacted<uid>/`, whose next segment is the Keeper's home directory and username in encoded form (`-Users-<name>-code-waggle`), plus a local uid. Replace with a neutral form, as hive-records-6 did: "worktree `hr7` in the queen's scratchpad".

**B2. The handoff misstates the opus T-023b verdict.** The handoff says "Opus `reviews/T-023b-opus.md` PASS at `ab68fb6` (the later commits are test-only)". The review's header and its last section ("Fix pass 3 re-check (ab68fb6)") both say `Verdict: must-fix` at `ab68fb6` (F1: `TestReuseRefusesExtraRegistration` red on linux, `product-ci` failing). There is no opus re-check of `78e201a` or `ca56131`; the fable review's "Final confirmation (ca56131)" is the only pass on the merged head. Suggested text: "Opus `reviews/T-023b-opus.md` must-fix at `ab68fb6` on one linux test assertion only (F1, code sound), fixed test-only by `78e201a` and `ca56131`; fable `reviews/T-023b-fable.md` PASS at `ca56131`." The Keeper merged on the "small fix then merge" card, so the merge itself stands; the record should say what happened.

Both are one-line text edits to the handoff. Nothing else needs re-review; a guard can confirm the two lines.

## Checks

1. Paths. Only `.waggle/control/WAGGLE.md` and `.waggle/hive/**` change. Nothing under `cmd/`, `internal/`, `go.mod`, `go.sum` or `.github/`. The WAGGLE.md diff is three added lines (blank line, `## 4. Hive Overrides`, one bullet); sections 1 to 3 are byte-unchanged. That commit is the only one touching `.waggle/control/**` and its subject starts `Head Keeper: `.
2. Gate. `waggle` built from `origin/main` (`3fff262`): `gate decisions --base origin/main --head HEAD` returns `decisions: ok, appended D-156, D-157, D-158, D-159, D-160, D-161`, exit 0. `gate verify` exits 1 on the expected lines only: leases "Head Keeper commit: merge with a Head Keeper bypass" for `8b706bd`, "commit has no T-<id>: prefix" for the six `Hive records 7:` commits, and tiers "WAGGLE.md: Tier 3 path; needs Head Keeper commit and bypass". All clear through the Head Keeper `--admin` merge (D-141).
3. DECISIONS.md. Pure append (zero removed lines), 161 `## D-` headings, D-156 to D-161 contiguous after D-155 and in commit order (D-156, then D-157 and D-158, then D-159 and D-160, then D-161). Each entry carries Date, Kind, Tickets, Supersedes, Question, Options, Hive recommendation, Keeper choice and Rationale.
4. Decision content against the evidence.
   - D-156: matches the reviews: T-023b has an opus and a fable guard, T-022b one guard, Tier 2.
   - D-157: `8650124` is the T-021 phase B merge. Both overreaches (`context_ignore_test.go`, the `init_test.go` constant) are named in `reviews/T-021b-sonnet.md` "Rulings needed". See N1 on the routing of the notes.
   - D-158: matches the WAGGLE.md line (check 5).
   - D-159: matches `media/T-014/AC7.md` and the JSON (check 6). See N2.
   - D-160: the noreply address is the intended one; it matches the author of `589724e` and `73c16f6` and the handoff's repo-local `user.email`.
   - D-161: 667 to 867 matches both T-023b reviews' size rulings (667, 740, 819, 867), and both accept. See N3.
   No decision claims a Keeper choice beyond what the reviews and handoff support; all are worded as card rulings recorded by the queen.
5. WAGGLE.md line. Accurate against D-158 and against `internal/scaffold/templates/roles/queen.md` line 14 (`.waggle/control/handoffs/queen-<n>.md`). Our hive's current `.waggle/control/roles/queen.md` has no handoff clause yet (pin predates `63b1bbf`), so the line is pre-emptive, which is what D-158 asks for ("lands before the next pin move"). `.claude/hooks/handoff-check.sh` checks `.waggle/hive/handoff/queen.md`, as the line says. No code parses WAGGLE.md by section: `layout.WaggleMD` is used only by `init` (seed), `upgrade` (seeded, skipped) and `dispatch.ReadInputs` / `runtime` prompt assembly (whole-file prompt text). `runtime.Invocation.Commands` is filled by its caller, not parsed from section 3. A new section 4 breaks nothing.
6. T-014 AC7. `AC7.md` matches the JSON: heads `969b25d` and `4f5d1af`, check `verify-leases-and-tiers` conclusion failure on both, annotations `leases: no claim for T-014; claims must stay until merge` on both and `tiers: path collides with GO.MOD on case-insensitive checkouts` on `go.mod` for #45. The Limit paragraph states the missing-claim gap. The AC7 tick is consistent with D-159.
7. Status lines. T-022: `f816050` is the phase B merge, #43, 326 lines (matches `reviews/T-022b.md`). T-023: `3fff262`, #46, 867 lines (D-161), "three guard fix rounds" plus two test-only commits, consistent with the reviews. Handoff hashes `4441171`, `8fa7b20`, `3958676`, `ab68fb6`, `78e201a`, `ca56131` all exist; next ID D-162 is right.
8. Privacy. No email address other than the D-160 noreply (4 occurrences). No tokens (`ghp_`, `gho_`, `github_pat_`, `sk-`, AKIA, JWT, `sbp_`). No `/Users/` or `/home/` path except `/home/runner/work/waggle/waggle` in the GitHub runner annotations (not personal). One username path, B1. See N4 for the hostname and macOS temp paths in `media/T-023b/`.
9. Dashes. No U+2013 or U+2014 in any added line.

## Findings (non-blocking)

- N1 (D-157 routing): fable N1 is the `planUpgrade` error prefix ("model policy for claude-code shims:"), a Go change in the upgrade path, not a queen template edit. Phase F's lease must include that file or the note will be lost. "Sonnet note 3" does not map to a numbered sonnet finding (its check 3 is the lease check); it reads as the third "Rulings needed" bullet (tighten the queen Handoff clause). Clarify in the T-023 phase F brief, since DECISIONS.md cannot be edited.
- N2 (AC7 evidence strength): every PR without a claim fails with "no claim for T-014", so #44 shows the gate fails an unclaimed PR; it does not exercise path-versus-lease checking. `AC7.md` and D-159 say "#44 proves the gate refuses an edit no lease covers", true but vacuous. The stated limit is honest about the missing case, and #45's tiers annotation is a real path-specific refusal. Also: `product-ci` passing on both PRs is asserted in `AC7.md` but not in the saved JSON, and the AC7 text still says "screenshots"; a parenthetical "(API evidence, D-159)" on the AC line would help.
- N3 (D-161 wording): "Every line past 667 is a guard-requested security fix" slightly overstates; the third pass includes the Windows and Solaris build-tag split (`nofollow_*.go`, 15 lines), a portability fix, and opus calls the round 2 lines "the fixes the Head Keeper asked for". Substance holds.
- N4 (redaction list): `media/T-023b/` probe output carries the Keeper's machine hostname (`*.local`, 5 lines, in lock "held by pid" messages) and a macOS per-user temp prefix `/var/folders/<tmp>/.../T/`. Not a username, but machine-identifying; add both to the handoff's pre-public redaction list.
- N5 (commit authors): `8b706bd` through `a258d38` (five new commits) are authored with the Keeper's personal email address; only the last two use the D-160 noreply. They are unmerged, so re-authoring them now is cheap and avoids adding five more to the history-redaction job. Keeper call; D-160 only rules out rewriting past commits.
- N6 (stale, pre-existing): `tasks/T-014.md` Status still says "Phase B is next", but phase B merged at `3063955`. `reviews/T-023b-fable.md` line 7 still says the `ab68fb6` section carries the current verdict; the `ca56131` section now does.
- N7 (follow-up): section 4 rides in every worker prompt once dispatch runs. Remove it in the phase F Head Keeper commit when the handoff path switches.

## Intent Packet

- What: Head Keeper WAGGLE.md section 4 Hive Overrides (queen keeps `.waggle/hive/handoff/queen.md` and the Stop hook until T-023 phase F, D-158); D-156 to D-161; T-014 AC7 met by check-run API evidence (D-159); T-022 and T-023 phase B status lines; T-021b, T-022b, T-023b guard reviews and `media/T-023b/` probes; eighth-session handoff. Hive only.
- Why: records the session's Keeper rulings and phase B outcomes, and stops a future pin move from switching the queen to a handoff path the Stop hook does not check.
- Risk: Tier 3 (`.waggle/control/WAGGLE.md`), Head Keeper merge with `--admin` (leases and tiers fail by design, D-141). No code reads WAGGLE.md by section. After B1 and B2 are fixed, nothing in the added files carries personal data beyond the intended noreply address, with the N4 hostname noted for the redaction pass.

Verdict: must-fix (B1, B2: two handoff lines)

## Re-check (11e562a)

Verdict: pass
Risk-Tier: 3

Branch rebased onto `origin/main` (`3fff262`) with every commit re-authored, then one fix commit `11e562a`. Eight commits: `e88a4b7` (Head Keeper, was `8b706bd`), `c4d2726`, `09ef52b`, `1c39135`, `be95ecd`, `5b35eb2`, `7cfed30`, `11e562a`. Checked in a detached checkout of `origin/hive-records-7`.

1. B1 fixed. The "In flight" line now reads "worktree `hr7` in the queen's scratchpad". No `/private/tmp/` path, uid or encoded home directory remains in any added line.
2. B2 fixed, and accurate. The handoff now says opus PASS at `3958676`, then must-fix at `ab68fb6` on one linux test failure only (code ruled sound), no opus re-check of `78e201a` or `ca56131`, fable PASS at `ca56131`, fable the only guard on the final head. This matches `reviews/T-023b-opus.md` ("Fix pass 2 re-check (3958676)" `Verdict: pass`, "Fix pass 3 re-check (ab68fb6)" `Verdict: must-fix`) and the fable review's final confirmation. The line also records the queen's first-draft misstatement, which is fine.
3. Authors. `git log --format='%ae %ce' origin/main..HEAD` gives `27632860+paulternate@users.noreply.github.com` as author and committer on all eight commits (N5 closed). The Head Keeper commit is still the only one touching `.waggle/control/**`, and its subject still starts `Head Keeper: `.
4. Tree unchanged apart from the fix. `git diff 73c16f6 HEAD` (old reviewed head against new) touches exactly the seven files in `11e562a`: the handoff, `tasks/T-014.md`, and five `media/T-023b/` output files. Against `origin/main`: the same 32 paths, nothing under `cmd/`, `internal/`, `go.mod`, `go.sum` or `.github/`; WAGGLE.md still +3/-0; DECISIONS.md still +117/-0. `waggle` built from this tree (Go code identical to `origin/main`): `gate decisions --base origin/main --head HEAD` returns `decisions: ok, appended D-156, D-157, D-158, D-159, D-160, D-161`, exit 0.
5. Privacy scan of every added line (2599). Zero hits for the machine hostname (`hostname -s`), `/Users/`, `-Users-`, the real `/var/folders/` prefix, or `/home/` outside the GitHub runner paths. The only email address is the D-160 noreply (4 occurrences). "email" appears twice as a bare word in handoff redaction notes, with no address. No tokens. 60 lines carry `<host>` or `<tmp>` placeholders (five `<host>.local`).
6. Redaction keeps meaning. Substitutions are line-for-line (62 removed, 62 added). The `/var/folders/<tmp>` versus `/private/var/folders/<tmp>` distinction is preserved, which matters because the T-023b fix pass 3 cause analysis turns on the macOS temp directory symlink. Lock-holder messages still show pid and timestamp. No review text quotes the redacted values, so no review lost meaning.
7. N6. `tasks/T-014.md` Status now says Phase B Done at `3063955` (that commit is the T-014 phase B merge). The second half of N6 is not done: `reviews/T-023b-fable.md` line 7 still points at the `ab68fb6` section as current. That file is unchanged since `73c16f6`; it needed no redaction (scan clean), so the brief's mention of redacting it does not apply. Non-blocking.
8. Dashes. No U+2013 or U+2014 in any added line.

Still open, non-blocking: N1 (D-157 routing, clarify in the phase F brief), N2 (AC7 evidence wording), N3 (D-161 wording), N6 second half (fable header pointer), N7 (remove section 4 in phase F). Commits already on main still carry the personal address; D-160 leaves that to the history-rewrite card.

## Intent Packet (11e562a)

- What: Head Keeper WAGGLE.md section 4 Hive Overrides (queen keeps `.waggle/hive/handoff/queen.md` and the Stop hook until T-023 phase F, D-158); D-156 to D-161; T-014 AC7 met by check-run API evidence (D-159) and T-014 status corrected to phase B done; T-022 and T-023 phase B status lines; T-021b, T-022b, T-023b guard reviews and `media/T-023b/` probes with hostname and temp paths redacted; eighth-session handoff with the T-023b guard record stated correctly. Hive only; all eight commits authored and committed by the D-160 noreply address.
- Why: records the session's Keeper rulings and phase B outcomes, and stops a future pin move from switching the queen to a handoff path the Stop hook does not check.
- Risk: Tier 3 (`.waggle/control/WAGGLE.md`), Head Keeper merge with `--admin` (leases and tiers fail by design, D-141). No code reads WAGGLE.md by section. No personal data in added files.

Verdict: pass
