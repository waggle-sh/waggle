<!-- SPDX-License-Identifier: MIT-0 -->
# Brief review: T-047, round 1 (opus)

Risk-Tier: 3

Reviewed oid: `8f0c03f6d1230269ff01a4938db0e91e173fc323` on `origin/t-047-brief`, file `.waggle/hive/tasks/T-047.md`, base `339c823`. The branch adds that one file.

## Intent Summary

The brief specifies `docs/setup.md`, a pinned instructions file a user's coding agent follows to install Waggle Gate, plus one README subsection, one Status sentence and a doc-drift test in `cmd/waggle`. No product code changes. Blast radius in the hive is one more test. Blast radius for users is what an agent does with their credentials, so the doc is Tier 3 by content. The four Head Keeper rulings are covered and the threat statement is plain and correct. Six steps or criteria fail as written. All six are small text fixes. Most of the code citations hold.

## How this was checked

I built the binary from a scratch clone of `339c823`, with an annotated scratch tag `v0.1.0` and the `release.yml` flags (go1.27.2, `CGO_ENABLED=0`, `-trimpath`, `-ldflags "-s -w -X main.version=0.1.0"`). It stamped `mod v0.1.0`, `vcs.modified=false`. I then ran Steps 4, 6 (local half), 9 and 10 against a local bare `origin`, with default branch `main` and again with `master`. GitHub reads were GET only. Doctor's GitHub findings and the Actions run were read from code, not run. The only write to the repo itself was a `git fetch` of `origin`.

## Blocking Issues (Must-Fix Only)

**M1. Section 4.2 Step 9: `waggle claim` fails on any repo whose default branch is not `main`.** `--base` defaults to `refs/remotes/origin/main` (`cmd/waggle/claim.go:185`, help: "default main on the remote"). The doc handles `<default>` everywhere else. Run on a `master` default:

```
waggle claim: base "refs/remotes/origin/main" does not resolve to a commit: git rev-parse: exit status 1:
exit 3
```

With `--base origin/master` the claim succeeds. Fix: add `--base origin/<default>` to the Step 9 line. Section 12 lists Step 9 as unverified, but the brief makes it a fenced command that AC2 holds byte for byte.

**M2. Section 4.2 Step 9: `waggle release T-900 --unmerged` fails in every gate-only repo.** It lacks `--owner`. README line 48 and the help text both say `--owner` is required without a local config. Run:

```
waggle release: config: no usable keeper_id in .waggle/control/local.yaml: ...; or pass --owner <id>
exit 2
```

With `--owner` it exits 0 and the claim ref is gone. The result is a leftover claim on T-900 in the user's repo. T1 cannot catch this, since it only parses flags. Fix: `waggle release T-900 --unmerged --owner <your GitHub login>`.

**M3. Section 4.2 Steps 9 and 10: the person's `waggle` is not on `PATH` when Step 9 needs it.** The only copy is under `$HOME/.cache/waggle/v0.1.0/...`. Step 10, which comes after, is where the doc says the person may put it on `PATH`. On a fresh machine both Step 9 text commands give command not found. Fix: write them as `"<waggle>" ...` with the agent printing the absolute path (T1 already accepts that form), or move the `PATH` sentence before Step 9.

**M4. Section 6 T4 and AC4: T4 as written fails on the doc's own Step 3.** T4 forbids `| sh` in ` ```sh ` fences. Step 3's required line holds `| shasum -a 256 -c -`, which contains that substring, and AC2 keeps it byte for byte. AC4 requires T4 to pass. Fix: say `| sh` followed by a space or end of line, or give T4 a word-boundary pattern and a passing row for `| shasum`.

**M5. Section 9 item 1 and AC9: the dry run fails at Step 4 as written.** It specifies "no network" and "an `origin` URL set and never pushed to", then runs Steps 1 and 4. Step 4's `git fetch origin` needs a reachable remote, and `git switch -c waggle-gate-install origin/<default>` needs a remote-tracking branch that a never-pushed `origin` does not have. Step 1's `gh auth status` and `gh repo view` need network and a real repo. AC9 depends on this run. Fix: use a local bare `origin` that has been pushed to (this works, see below), drop the `gh` lines and the URL-form check from the dry run, and keep the separate non-GitHub case. Note `--base main` also assumes the scratch default is `main`.

**M6. Section 4.2 Step 1 and the "No `gh`" row of 4.3: the no-`gh` path has no way through.** The row says tell the user and "Use the Step 3 and Step 6 fallbacks". Step 1 has no fallback, and it is the only source of `<default>`, which Steps 4, 6 and 8 need, of `viewerPermission`, which Step 7 needs, and of the `merge_queue` stop. Step 8's `gh api` also has none, so "a rule requires PRs" cannot be checked. An agent cannot satisfy Step 1's success line and is told to continue anyway. The Step 3 fallback also does not exist before the flip. Fix, either: (a) make `gh` a stop, the simplest and safest, or (b) name `git ls-remote --symref origin HEAD` for the default branch, state that merge queue and PRs required are then reported under "Not checked", and say Step 7 prints the admin sentence unconditionally.

## Rulings and threat statement

All four rulings are covered and none is contradicted.

1. Gate only: Step 4 runs `init --gate-only`, and section 16 puts a full install out of scope.
2. GA scope, no card moved: section 11 matches the D-341 and D-277 order, including D-277's "before any other PR merges" for the pin move. Part 1 lands before redaction, part 2 inside "demo repo pins", part 3 after the flip.
3. Tag and `SHA256SUMS`: `refs/tags/` URL, Step 3, T2, T4, AC7. Section 8.3 is honest that the sums share the release's trust.
4. Agent opens the PR, human merges and creates the rule, agent runs doctor and reports: Steps 6 to 8, MUST NOT list.

The brief says plainly that nothing stops an agent with admin credentials skipping the human steps (section 2, section 8.1 "this file only tells it not to", 8.3 closing line, Step 7's "binds nothing until then"). I found no wording that implies otherwise. No attack finding clears D-203: every tamper path needs tag, release or maintainer access, or an agent already disobeying the doc.

## Claims confirmed

- Asset names (`release.yml:112, 149`), `version` output, annotated-tag rule behind `^{}`, draft release (`:207-223`).
- `init --gate-only`: exactly three `[create]` lines on a tagged build; rerun gives `[skip]` and exit 0 with `Gate initialized`; succeeds outside a repo (no remote read). A tagged build pins `gate@<sha>` with `waggle.pin v0.1.0`, so Step 5 compares the right field and `grep` finds exactly one line.
- `gate verify --base main --head HEAD --branch waggle-gate-install` exits 1.
- T-900 scenario: `leases: FAIL: other/b: T-900: path is outside the claim directories`, exit 1, matching Step 9's success text.
- `doctor`: findings, exit codes, `report.go:107`, `doctor.go:54`, `probe_github.go:150`, `github_transport.go:53-65`, `remote.go:14`; non-GitHub `origin` exits 3 (run).
- `-h` exits 0 for `init`, `doctor`, `claim`, `release`, `uninstall`. All flags in the doc exist.
- `uninstall`: dry run by default, `--apply` executes, `refs/waggle/*` kept, block in `CLAUDE.md` removed with the rest of the file intact (run, in a gate-only repo; section 12 lists this as unverified).
- License row and header form (`rules.go:63, 112-113`, `rules_test.go:34`), lease shape (T-037 precedent), `launch.go:85-87`, `CODEOWNERS` `/README.md`, tier claims, `layout` and `forge` constants, `checkPlain` signature, no `TestMain`.
- Live reads: repo `PRIVATE`, raw URL 404, `cli/cli` tag URL 200, rules list exactly as quoted, one `main` ruleset and no tag ruleset, Actions `access_level: none`, no releases yet.
- Budget (section 15): 187 doubled to 374; target, stop and planned cut present; D-136 counts docs lines; D-185 and D-210 wording correct.

## Notes (not blocking)

**N1. Section 4.2 Step 6, section 9 item 1: a third failure is not explained.** The install PR also prints `decisions: FAIL: decisions: change is not a pure append: head: entries removed or rewritten (base has 0 entries, head has 0)`. The brief names only the commit's task and the control paths. Step 6's success rule still holds, but the doc's explanation and the PR body should name it, since the wording reads like damage. An empty log added to a base with none reporting "rewritten" may deserve its own line beside N1 to N3 in section 12.

**N2. Section 4.2 Step 8 report: "Not checked" omits bypass actors.** A ruleset can let the installing agent's admin credentials bypass the rule (README line 26, L6). The rules list does not show bypass. Ruling 4 asks for an honest report, so this line belongs there.

**N3. Section 4.2 Step 7, 4.3: no row for a plan without rulesets.** `docs/gate.md` L2 says rulesets are documented for Team and Enterprise. On a private repo under a plan without them, Step 7 cannot be done and doctor exit 1 sends the agent back to Step 7 in a loop.

**N4. Section 4.2 Step 3: `cd` persists in some agent shells.** 4.1 assumes shell calls may start fresh. In a persistent shell the `cd` in Step 3 leaves the agent in the cache directory, so Step 4's `git fetch` fails there, and if an agent carries on, `init --gate-only` writes into the cache directory with exit 0 (run). Wrapping in `( cd ... && ... )` costs nothing.

**N5. Section 4.2 Step 6: a token without the `workflow` scope, or a fine-grained token without Workflows write, cannot push this branch over HTTPS.** Likely a common first failure. "For anything else, report and stop" handles it safely; a row would save a round trip. From my knowledge of GitHub's push rules, not run.

**N6. Section 4.2 Step 5: `git ls-remote https://github.com/waggle-sh/waggle` needs credentials while the repo is private.** During the rehearsal it may prompt or fail for an SSH-only setup. Section 9 item 2 should expect this. `GIT_TERMINAL_PROMPT=0` avoids a hang.

**N7. Section 14 AC2 and 8.5: "every step has a success criterion".** Steps 2, 7 and 10 have none stated, while 8.5 makes a missing one a must-fix. State them or scope the rule to Steps 1, 3 to 6 and 8. Likewise 4.1's "Pause only at Step 7" conflicts with Step 9 and with table rows that ask the user; AC3 has the better wording.

**N8. Section 6 T1: help prints single dashes.** `init -h` shows `-gate-only`, not `--gate-only`, so a literal `--flag` match fails. The builder must compare on `-name`, as `TestHelpPlainTerms` does with `-dirs`.

**N9. Section 15 planned cut: `T-047b` is a phase name, not a ticket id.** `waggle claim T-047b` exits 2 (`invalid ticket id`) and `ticketSubject` and `ticketBranch` do not bind `T-047b: ` or `t-047b`. Under the `t-025b` precedent the branch works only if the claim is `T-047 --branch t-047b` and subjects stay `T-047: `. Worth one sentence.

**N10. Section 15: the estimate is 370 against a doubled 374,** and the cut saves about 40 lines. Section 4 caps the doc at 200 lines, so 480 is reachable only after AC1's ceiling has already gone. Harmless, but the cut is unlikely to be what fires.

**N11. Section 4.2 Step 10: `uninstall --apply` leaves an uncommitted change, and merging it needs an admin bypass** (the command says so; `.waggle/control/` is Tier 3). The back-out sentence should say a person does that merge, in line with the MUST NOT list.

**N12. Section 7: if the T-041 site goes live before the flip, it shows a prompt whose URL returns 404.** Follows from the home page rendering the README. Not a card move, but the queen may want it on the site's card.

**N13. Section 9 item 2 assumes the "demo repo" card leaves no `.waggle/` and no gate workflow** until "demo repo pins"; otherwise Step 1 stops with "Already installed". This fits D-277 (no outside pin before the rewrite). No demo repo brief exists to confirm it.

**N14. README Install step 3 ("needs an admin merge"):** section 2 is right that this holds only once a rule blocks the merge. Flagged as the brief invited; out of this ticket's lease.

**N15. AC4 for T5:** `checkPlain` takes `*testing.T`, and the lease forbids editing it. `new(testing.T)` works for a planted row on go1.27.2 (run: `Errorf` sets `Failed()` with no panic), so the criterion is testable. Say so, or the builder may reach for the forbidden file.

## Open questions

I do not rule on Q-T047-1 to Q-T047-7. The cited facts behind each recommendation check out, with one overstatement: Q-T047-3 says "Reversible: `waggle uninstall` strips it". True, but uninstall removes the whole gate with it, and that merge needs an admin bypass. Removing only the block is a hand edit.

Verdict: must-fix

# Brief review: T-047, round 2 (opus, confirming)

Risk-Tier: 3

Reviewed oid: `cb065e32c97bfda74276040f33c9a274b261576a` on `origin/t-047-brief`, file `.waggle/hive/tasks/T-047.md`. Scope: the diff from `8f0c03f`, plus rulings on N7 and N8.

## Intent Summary

Round 2 fixes M1 to M6 and folds in guard notes. No scope or ruling changes, and the blast radius is still one doc, one README subsection and one test. M1 to M4 and M6's removal of the fallbacks hold under a run. Four must-fix items remain, each about one line. Two are the rulings asked for on N7 and N8. One is the second half of the M5 fix. One is a failure of the rules-list call that the M6 fix relies on, which I missed in round 1.

## How this was checked

I built from a clone of `339c823` with an annotated scratch tag and the `release.yml` flags (the diff touches only the brief, so the product code is the same). I ran section 9 item 1 as written against local bare remotes, and the full Steps 4, 6 and 9 on a `master` default. I ran every flag in the doc's `"<waggle>"` lines against `-h`. GitHub reads were GET only. The only write to the repo was a `git fetch`. The Actions run was read from `gate/action.yml`, not run.

## M1 to M6

- **M1, fixed.** With `--base origin/master`, `claimed T-900`, exit 0.
- **M2, fixed.** `released T-900 (claim ..., unmerged)`, exit 0, claim ref gone, run after the branch was deleted as the step orders. `release.go:112-115` is the right citation.
- **M3, fixed.** Steps 9 and 10 use `"<waggle>"`, which T1 and T4 both match.
- **M4, fixed.** The pattern passes `| shasum` and catches `| sh`, `| bash -s` and `|sh` (run with `grep -E`).
- **M5, half fixed.** The first half runs as written: exactly three `[create]` lines, `gate verify` exit 1 with the commit, tier and decisions lines. The second half fails: see M9 below.
- **M6, fixed as a design, one gap.** No fallback is left anywhere in the brief (`curl`, the compare URL and "no `gh`" searched). Steps 1, 3, 6 and 8 and rows "No `gh`" and "`gh` not signed in" agree. Section 9's dry run drops the `gh` lines, and its smoke item no longer implies a path without `gh`. The README's Install steps do not need `gh`, since doctor also takes `GITHUB_TOKEN`, so the row's pointer is true. The gap is M10 below.

Other new claims confirmed: `internal/decisions/gate.go:50-52`, `gate_commits.go:35, 38`, `release.yml:52-56`, `docs/gate.md` L2, T5's `archive` trap (`plain_terms_test.go:33`). The Action runs `gate verify` (`gate/action.yml:51`), so Step 6's decisions line appears in the log; the line as quoted is a substring of what prints, `decisions: FAIL: decisions: change is not a pure append: ...`.

## Blocking Issues (Must-Fix Only)

**M7. Ruling on N7: must-fix. Section 14 AC2, 8.5, section 4.2 Steps 2, 7 and 10.** AC2 says "every step has a success criterion" and 8.5 makes "a step without a success criterion" a must-fix. Section 4.2 is what the doc says, and a builder who writes it faithfully fails both at Step 10, which has none. Steps 2 and 7 have a stop rule and a wait, not the stated form. Fix: one `Success:` clause on each (Step 2: both `uname` values map; Step 7: the person says it is done; Step 10: the message gives the binary path, the back-out lines and, if section 5 ran, the new-session prompt). Scoping AC2 and 8.5 to the other steps also works.

**M8. Ruling on N8: must-fix. Section 6 T1, AC4.** T1 asserts "the help text lists each `--flag` on the line", and AC4 requires T1 to pass. Run: `init -h` has no `--gate-only`.

```
init --gate-only   literal matches 0, flag line "-gate-only" 1
```

Every other flag passes literally only because those commands' `Usage:` lines happen to use double dashes. So the test as written fails on the one row T1 calls out by name, the inline `init` set. Fix: compare on `-<name>` at the start of a help flag line, as `TestHelpPlainTerms` does with `-dirs`.

**M9. Section 9 item 1 and AC9: "Step 9's claim and release against a second bare repo" fails as written.** The item sets up nothing in that repo, and both commands read the config:

```
waggle claim: config: open .../.waggle/control/config.yaml: no such file or directory
claim=3
waggle release: config: open .../.waggle/control/config.yaml: no such file or directory
release=3
```

After `init --gate-only` is committed and merged to `master` and pushed, both exit 0. Fix: say so in the item, and give the push of `T-900: test the gate` plus `gate verify --base origin/master --head HEAD --branch t-900` (exit 1 with `path is outside the claim directories`, run) if the item is meant to cover the refusal.

**M10. Section 4.2 Step 1 and 4.3: the rules-list call returns 403 on a private repo whose plan has no rulesets, and no row handles it.** Run on a private repo under a free personal account:

```
gh: Upgrade to GitHub Pro or make this repository public to enable this feature. (HTTP 403)
exit 1
```

Step 1 now fences this call and M6 makes it mandatory, so such a repo ends at "for anything else, report the full output and stop", before anything is written. That outcome is safe, but the agent has no words for it, the failure is not in section 12, and the round 2 row "The person says the plan does not offer the rule" at Step 7 never fires for this case, so its plain sentence about the gate binding nothing does not reach the user. The same plan is likely a common first install. Fix: add a row for this 403 in Step 1: stop, and say that GitHub does not offer the rule on this private repo's plan, so the gate would bind nothing here. The stop is the right result, since the install could not be made to bind. One run on one account: other plans and an organization on a free plan should go in section 12 as not verified. The message names Pro where `docs/gate.md` L2 names Team and Enterprise, so the row should not name plans.

## Notes (not blocking)

**N16. Section 12: "Step 9's claim, refused push and release".** Nothing refused a push. In my runs the push succeeds and `gate verify` fails. Say "the gate's refusal".

**N17. Status line against the Risk Tier bullet.** The Status says round 2 needs one confirming guard (D-205). The Risk Tier bullet says a fix round that changes a fenced command, a stop rule or the human step gets two, treated like D-205's lease and claim case. That bullet is written about the doc's build review, but this diff does all three, and it changes the claim and release lines. The queen should choose knowingly.

**N18. 4.1 "Pause only at Step 7".** Still at odds with AC3 ("and, when asked, Step 9"), and now with the "ask, then resume at Step 1" rows. It is safe, because a stop is not a pause. AC3's wording would serve 4.1 too.

**N19. "`gh` not signed in ... resume at Step 1" fits a failure at Step 1 only.** A sign-in lost after Step 6 would resume at a Step 1 that then says "Already installed". 4.1's "resume at the failed step" already covers it if the row said so. Rare.

## Rulings

None contradicted by the diff. No attack finding clears D-203.

Verdict: must-fix

# Brief review: T-047, O2 round (opus, confirming)

Risk-Tier: 3

Reviewed oid: `43384ddf38d3644c597987f96a43a75f4c83517d` on `origin/t-047-brief`, file `.waggle/hive/tasks/T-047.md`. Scope: the diff from `e7c9014`, against `T-048.md` section 4 at `e387340` on `origin/t-048-brief`, `media/T-046/probe-log.md` and the code at `origin/main` `1344b41`.

## Intent Summary

This diff records D-350 to D-355, applies T-048 section 4 (warning sentences, Step 7 additions, Step 8 report, Step 10 credentials sentence, the rules block line), drops the O1 wait and moves the rehearsal to throwaway private repos after the first tag. There is still no scope or lease change, and the blast radius is one doc, one README subsection and one test. The T-048 text is applied faithfully, the O1 material is gone, and the warning holds against the probe log. Three one-line fixes remain: one wrong fact that T-048 section 4 itself carries, one overclaim of protection in Step 7 item 3, and one path that the D-352 edit to section 5 opened in Steps 6 and 10.

## How this was checked

I read the diff, T-048 sections 1 to 4, D-350 to D-358 and the final probe log. I built the binary from `origin/main` into a scratch directory and ran `doctor --no-write` on this repo, whose checks D-357 bound to GitHub Actions. Live reads were GET only. Nothing in this repo or its branches changed; the only git writes were fetches.

## Blocking Issues (Must-Fix Only)

**M11. Section 4.2 Steps 6 and 10 against section 5: the default path fails on any repo with neither `CLAUDE.md` nor `AGENTS.md`.** The diff changes section 5 to "If neither exists, the agent adds nothing and shows the block in its Step 10 message", which D-352 requires. Steps 6 and 10 did not follow. Step 6 still says "If `agent_rules` is `yes`, apply section 5 first and run the second `git add` line", `agent_rules` defaults to `yes`, and that line then has no file to name. Run:

```
git add CLAUDE.md
fatal: pathspec 'CLAUDE.md' did not match any files
exit 128
```

Under 4.1 that failure is "report the full output and stop", so an agent reading the step literally stops before the PR. Step 10 says "If section 5 ran: start a new session so the agent reads the rules", which is false when nothing was written, and its success clause does not require the block that section 5 sends there. Fix: Step 6 runs the second line only if section 5 wrote a file. Step 10 gives the new-session prompt only if a file was written, otherwise shows the block, and its success clause says so.

**M12. Section 4.2 Step 8 report line and section 12: "Doctor's `Enforcement: enforced` covers only the three findings above" is a wrong fact.** It covers two. `Report.Enforcement()` (`internal/forge/report.go`, the function above `Render`) reads `required-gate` and the merge strategy's rule, here `up-to-date`, and never `claim-refs`. Run on this repo:

```
Enforcement: enforced

  skipped   claim-refs      Not tested; run waggle doctor without --no-write.
  ok        required-gate   A rule on main requires the verify-leases-and-tiers check.
  ok        up-to-date      Pull requests into main must be up to date before they merge.
```

The old closing line did read all three, through `NotEnforced()`, so the swap to the label changed what the sentence is about. The text comes from T-048 section 4 item 7, which is not one of the card-ruled sentences. Fix: "Doctor's `Enforcement: enforced` covers only required-gate and up-to-date, and the required check can still be forged." The T-048 brief should follow.

**M13. Section 4.2 Step 7 item 3: "A PR that edits the gate workflow runs its own copy without it" overclaims.** It reads as though the CODEOWNERS line stops the PR running its own copy. It does not: the copy runs either way, and the line only requires an owner's review before that PR merges. `docs/gate.md` L5 says "Mitigation, not a fix", README's limits say "it does not fix it", and T-048 adds that code-owner review "does not cover a workflow on another branch". The probe log's own-PR row shows a PR's own job can forge the check. T-048 section 4 item 6 says this wording "stays true", and it does not hold beside that row. Fix: "A PR that edits the gate workflow runs its own copy either way. This makes a code owner review that change before it merges. It does not cover a workflow on another branch."

## Checked and holding

- **Warning, against the log.** Workflow push forges (S1 rows, `CLEAN`, `UNSTABLE`). Delete plus status forges only unbound, and the sentence on binding follows at once (token rows, `BLOCKED` when bound). Binding does not stop a workflow (bound workflow row). A required approval blocks the merge and leaves a forged green (required review row). "Limited to Contents and Pull requests" is advice with no claim of closure, and "None of these is complete alone" sits beside it, so the S12 exception is not contradicted. Text matches `T-048.md` section 4 item 5 at `e387340` byte for byte.
- **Step 7.** Item 2 and "Without this, a forged check can merge with no person" match the other-PR row (merged, no admin flag) and D-356's "stating the risk without one". The success rule is unchanged.
- **Step 8.** A check bound to GitHub Actions still reads `required-gate ok` (run above), and `internal/forge` never reads `integration_id` (git grep), so "which app may post" under "Not checked" is true. The rules list does carry `required_approving_review_count` and `dismiss_stale_reviews_on_push` (read live). The list prints the four types as quoted.
- **Step 10.** "could push workflow files, which the install needed" is true by Step 6. A classic `repo` token does hold more.
- **With T-048 merged first.** Doctor's exit codes and findings do not change, and Step 8 reads only those, so the new closing lines break nothing. No T1 to T5 assertion reads doctor's output. T5's bare words and `hive` check pass on the new warning and Step 7 text. README step 3 under T-048 agrees with Step 7 item 1 and section 2. Section 7's paragraph anchor is at the current README and T-048 does not touch it. No product file differs from `339c823` through `1344b41` (`git diff --stat` empty), as the Status says.
- **Rehearsal in the G5 order.** D-341's order has the first tag, then demo repo pins, then the flip, so the slot exists and moves no card. D-353 is followed. The org plan is `team`, so a private throwaway passes Step 1's rules call with no 403. The `gh api` raw form works (run on `README.md`). `git ls-remote` over HTTPS on the private repo worked here with prompts off, on this machine's credential helper. `access_level` is still `none`, as the item says. Item 7's new-tag path still fits.
- **O1 remnants.** None outside the dated Status history. T-048 items 1 to 4 and 7 are applied: Depends on, section 2, section 11 part 1, section 17, the old line 165, N3 and N4 renumbered, section 16 reads N1 to N3.
- **Human steps.** Every new sentence is in Step 7's printed text, which opens "Only a person should do this part", or follows Step 10's "Tell the person". Nothing tells the agent to merge, set a rule, create a token or change a setting. Section 8.5 gains the right must-fix line.
- **Rulings.** D-351 to D-355 are marked and match their entries. The Head Keeper card closures in the Status have no DECISIONS entry of their own, so I took them as stated.

## Notes (not blocking)

**N20. Section 4 against 8.1.** Section 4 says "no L-number without its link", and AC1 holds the warning verbatim with "(gate spec, L11)" and no link. A builder cannot satisfy both, and the PR body in a user's repo has no gate spec to point at. One clause settles it: name a link beside the warning in the doc and README, and let the verbatim text stand.

**N21. Warning and Step 10: the recommended token can still write claims.** Contents write reaches `refs/waggle/*`, so "write claims yourself, from credentials the agent does not hold" still applies after the person narrows the token. Nothing claims otherwise, but the two sentences sit close together. Not run. Step 10 could add one clause.

**N22. "the status route"** is not defined anywhere the setup reader sees it, in the warning or Step 7. "Writing a commit status" would read plainly. The text is card-ruled in the warning, so this is the Head Keeper's choice.

**N23. Step 7's "can be forged after the approval with no new push"** is flat in the doc, and section 12 records that order as not run, where T-048's L11 says so to the reader. It errs toward less protection, so it is not an overclaim. The flat form is what T-048 section 4 asks for.

**N24. Section 9 item 2: the setup is not stated.** D-353's reason for Codex is an agent that reads `AGENTS.md`, and the block goes only into a file that exists, so a README-only throwaway never exercises the block. Seed `AGENTS.md` for the Codex run and `CLAUDE.md` for the Claude Code run, or run the M11 path on one on purpose. The repo also needs a first commit, since Step 4 needs `origin/<default>`. The pass rule could also name Step 10's credentials sentence.

**N25. Step 8 "whatever doctor printed".** After a second doctor exit 1, the report's last line names a label doctor may have printed as `advisory`. "If doctor printed" would cover it.

**N26. MUST NOT list.** Step 7 now names an Actions setting and a CODEOWNERS line. Step 7's heading covers them, but the list says only "a ruleset or branch protection". "Or any repository setting" costs three words.

**N27. Status cites `T-048.md` section 4 "at `8d264bb`".** Items 5 and 6 changed by `e387340`. The next sentence and the text are right, so this affects only the cite.

**N28. AC6 does not carry the card ruling that public docs hold no hive path** (T-048 Status). T5 lets `.waggle/hive/...` through by design, so only a guard catches it. Harmless while the doc copies nothing from `media/`.

**N29. For T-048, not this brief.** Its section 8 says the probe repo "is deleted (D-358)". A GET still returned `waggle-sh/gate-probe` at this review.

Verdict: must-fix
