<!-- SPDX-License-Identifier: MIT-0 -->
# Brief review: T-037 (G5 prose), opus, round 1

Risk-Tier: 2

Reviewed `.waggle/hive/tasks/T-037.md` at `af7f863` on `t-037-brief` (base `1037e77`), against main at `e82da6f` (T-036 merged with rulings D-270 to D-277), `docs/rfc-notes/repositioning.md`, DECISIONS D-229 to D-277, the gate code at the base (`cmd/waggle/gate*.go`, `internal/comb/backend.go`, `changecheck.go`, `internal/config/config.go`, `internal/forge/reviews.go`) and the public pages behind section 4, fetched 2026-10-24. No gate code changed between `1037e77` and `e82da6f` (`git diff --stat` touches only `.waggle/`).

## Intent summary

T-037 is the prose half of G5. It rewrites `README.md` in plain terms (D-236), adds a cited gate spec at `docs/gate.md` and three recipes under `docs/recipes/`, moves the hive glossary from the README into CONTRIBUTING, corrects two CONTRIBUTING claims, and adds Apache-2.0 SPDX lines to the docs it touches (D-129, T-025 section 4). No code, `.waggle/` or `.github/` file changes. The blast radius is what users will believe the gate enforces once the repo flips, so a misleading security claim is the main risk (D-203). The structure is sound: claim map with citations, limits list, reader-facing threat bar, placeholders for G4 values, `<commit-sha>` pins. Four things need fixing before dispatch. One instruction contradicts the bee-name ban. The "five checks" are left to a source note that overstates two of them. Obligations the ruled T-036 assigns to these recipes are missing. Two section 4 sources are wrong or out of date.

## What checks out

- **Lease and lease rule.** `leaseMatch` accepts an exact file path (`internal/comb/changecheck.go:112-124`, `path == d`). `normalizeDir` allows `.` in entries (`backend.go:296-299`), and none of `README.md`, `CONTRIBUTING.md` or `docs` is Tier 2 or 3 (`backend.go:29-53`). T-025 phase A already leased `README.md` and `CONTRIBUTING.md` as file paths. The rule lists files exactly, and its stop-and-report clause is right.
- **Tier.** Calling it Tier 2 by content holds up. CODEOWNERS gates `/README.md`, and `RequiredMinTier` gives Tier 0 for these paths. Q-T037-4 (a), two guards, is a fair call under D-205's reasoning, and it does not change D-205.
- **Budget.** 90 + 200 + 40 + 180 + 80 + 5 = 595. Doubled, that is 1,190. 1.3 times gives 1,547, rounded to 1,550. 1.8 times gives 2,142, rounded to 2,140. That matches T-036 section 9's convention and D-185 and D-210. The planned cut is concrete.
- **Expired claim.** `gate_commits.go:328-331` warns `claim for T-x expired at ...; accepted`, and the gate passes. A missing claim fails at `:320-321` and a malformed one at `:322-323`. The claim is correct.
- **Claim map rows.** I re-read every row at the base. Binding order, conflict and unbound failures: `gate_commits.go:259-309`. The `Head Keeper: ` prefix: `:264-267`. The base-merge rule: `:198-208, 243-252`. Caps: `:15-20, 193`. Tree collisions and the 4096-byte limit: `:339-361`. Branch default under Actions only: `gate_verify.go:133-139`. Base-only config, with a bad base config as exit 3: `gate_verify.go:239-273`. Tier 2 under a light claim: `changecheck.go:97-100`. Claim refusals: `backend.go:455-471`. Approver: `gate_approve.go:90-141, 172-226` (counted states, exact head, after `LastBaseChange`, any listed CHANGES_REQUESTED blocks, only `tier3` and `outside_lease` dropped on approvable paths). Approvers are capped at 32 (`config.go:76-83`). Skips warn only under Actions (`gate_approve.go:180-185`). Trailers only ever warn (`gate_approve.go:158-170`, `gate_trailers.go:27-84`). The decisions check runs inside `verify` (`gate_verify.go:167-169`). Exit codes and the absence of an override flag: `gate.go:22-28, 38-57`. Every row matches the code.
- **Redaction list (section 5).** `docs/RFC-0003.md:113` holds `Approved by: Head Keeper (paulternate)`. The note's l19 strategy text is at lines 29, 33 and 98. CODEOWNERS and the gate output strings are quoted correctly. I found nothing in `docs/` the list misses.
- **CONTRIBUTING corrections.** Section 6's "CI rejects a missing or wrong one" is false today: there is no `internal/license/` and no `dco` job. The opening line points at README sections that are moving. Both corrections are needed.
- **ACs.** AC1, AC4, AC6, AC7, AC10 and AC11 can be checked mechanically. AC2, AC8 and AC9 can be checked with the claim map. AC3 and AC5 have the gaps noted below.
- **Settled decisions.** Recommended options (a) for Q-T037-1 to Q-T037-5 reopen nothing. Q-T037-1 (b) is labeled as amending D-234 and D-237 (see N9 for D-263).

## Blocking issues (must-fix)

**M1. Section 3.1 says to keep the License section "word for word, it is correct", but it names `waggled`.** Line 63 of `README.md` reads "Waggle, the `waggle` CLI and the `waggled` daemon, is licensed under...". Section 3 bans `waggled` from the README, and the AC3 regex lists `waggled`. It is also a claim about a component that does not exist: dispatch ships second (D-229), and no daemon is built. The builder cannot meet both 3.1 and AC3. Fix: allow exactly one edit, dropping "and the `waggled` daemon". Everything else stays word for word. Under D-129 the copyright holder is unchanged, so the edit needs nobody else's consent. The scaffold MIT-0 sentence stays true for `--gate-only`, which also renders from `internal/scaffold/templates/`.

**M2. The README's "five things the gate checks" are never listed, and the context the builder is pointed to overstates two of them.** Section 3.1 says "the five checks" and section 15 points to the note's section 2. That section says item 3 "passes only with a listed approver's sign-off". D-261 contradicts this: hardcoded Tier 3 paths are never approvable, and only an admin bypass passes them (`gate_approve.go:90-104`). Item 4 says trailers mean "`git log` answers which task and which review let this in". In the code they are warnings only, read from the last paragraph of the PR body (D-263, `gate_trailers.go:27-104`). They reach `git log` only if the merge message carries the PR body. Item 5, "refuses to let anyone rewrite", checks net change only: intermediate commits are not inspected (`gate.go:155-156`). In a user repo `.waggle/` cannot be leased (`backend.go:455-458`), so every append also fails leases and needs an admin merge. Item 1 is an input the gate reads, not a check. Section 3.1 also says the pitch "may be shortened, not strengthened", but the pitch ("know that each PR touched only what its task was given") already holds only under the required-check precondition. Fix: write the five items into 3.1 in corrected form, each tied to its claim-map row. State trailers as warnings. State protected paths as approver for repo-listed paths and admin bypass for hardcoded ones. State the decisions log as net-change and admin-merged. Make the pitch conditional on the required check, or say in 3.1 that the first "What it does not do" line carries that condition.

**M3. Obligations the ruled T-036 assigns to the G5 recipes are not carried into 3.2 or 3.3.** T-036 2.11 item 8 and section 5 (merged at `e82da6f`) list them. Missing from T-037:
- `--owner` is required in a gate-only repo. There is no `local.yaml` (T-036 2.5, D-240), yet 3.1 shows `[--owner <id>]` as optional.
- Run `upgrade` and `init` from a pushed or released build. A build from an unpushed commit renders an Action ref that fails at job setup (T-036 2.4 rule 1). A module build pins by tag and warns (rule 2).
- The remaining version skews. A hand-written workflow pinned to an older Action that meets a newer config key exits 3 (D-239). A newer CLI can write claims that an older Action reads (`backend.go:113`).
- A known bypass: any app or status with write access can satisfy a required context that has no integration binding (T-036 section 5 note). It belongs in the 3.2 limits list and in AC9.
- D-276's mitigation has two forms: code-owner review on `.github/workflows/**`, or an organization ruleset that requires the workflow. The `pull_request_target` spike is still pending, so the docs must call this a mitigation, not a fix. Code-owner review binds only with "Require review from Code Owners" on, which the repo's own CODEOWNERS header says.

Fix: add these to 3.3's recipes index list and the 3.2 limits, and extend AC9 to cover the required-context bypass.

**M4. Section 4 rows G5 and G4 need source changes, and the Copilot product name has changed.** I fetched these pages on 2026-10-24.
- **G5.** The `about-rulesets` page does not say rulesets can require a pull request or required status checks; the fetch found no such sentence. Those rules are on `https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets`: "Require a pull request before merging", "Require status checks to pass before merging", and the up-to-date option. As written, AC5's allowlist (section 4 sources plus pages cited in the note) would forbid the correct source. Add that URL to G5 and AC5. The queen should also check whether rulesets on a private repo need a paid plan, since `about-rulesets` scopes some rulesets to Team and Enterprise. That would be a reader precondition.
- **G4.** Cloud agent support is stated, on `https://docs.github.com/en/copilot/reference/custom-instructions-support`. Under GitHub.com, Copilot cloud agent lists repository-wide instructions (`.github/copilot-instructions.md`), path-specific instructions, agent instructions (`AGENTS.md`, `CLAUDE.md` or `GEMINI.md`) and organization instructions. Use that page as G4's source and mark the row verified, or leave it for the queen with that URL.
- **Name.** The G1 and G2 pages now say "Copilot cloud agent", not "coding agent". The recipe's title, file name (`copilot-coding-agent.md`) and 3.3 heading should use GitHub's current name, or say "Copilot cloud agent (formerly coding agent)". A doc that names a product wrongly reads as stale or unofficial.

## Notes (not blocking)

- **N1. Section 2 pins.** Section 2 says to read a pin from "`git rev-parse` of the tag". Release tags are annotated (T-036 2.10), so `git rev-parse vX.Y.Z` prints the tag object, not the commit. `uses:` needs the commit. Write `git rev-parse vX.Y.Z^{commit}` (or `git ls-remote <repo> 'refs/tags/vX.Y.Z^{}'`). A wrong pin fails at job setup, so it fails closed, but the builder has no network to catch it.
- **N2. AC3 regex over citations.** `\bcomb\b` matches every `internal/comb/...` citation. `\bhive\b` matches the Tier 2 path `.waggle/hive/` in the protected-path list, not only the `DECISIONS.md` path. `keeper` matches the config key `keeper_id`. `\bT-0[0-9]{2}\b` matches example tasks such as `T-042`. Add file paths and config keys in code spans to the exemption, and tell the builder to write examples as `T-123` or `T-<id>`.
- **N3. Limits list (3.2).** Unlike the claim map, the list has no sources. "Fork PRs cannot read claim refs they cannot push" has no source, and it looks wrong: the job fetches claim refs from the base repo's `origin`, which a fork PR run can read. The real limit is the second half, that outside contributors cannot write claims (D-232). Give each limit a source (code, decision or doc), and drop or fix the fork sentence. Two more limits belong in the list: the decisions check sees net change only (`gate.go:155-156`), and decisions-log appends always need an admin merge because `.waggle/` cannot be leased.
- **N4. Claude Code recipe.** A local `TaskCompleted` hook needs the claim refs fetched first (`git fetch origin '+refs/waggle/claims/*:refs/waggle/claims/*'`), or every commit fails with "no claim for T-x". It also needs a current local base. Row C3's "can overwrite each other's files in one working tree" is an inference: the page says same-file edits overwrite, and that an Agent call with `isolation` is not a teammate. It does not say "one working tree". Word it as an inference. Row C5 leaves out the `dontAsk` exception.
- **N5. Release step in recipes.** The gate requires claims to stay until merge (`gate_commits.go:321`). `waggle release` treats squash and rebase merges as unmerged and needs `--unmerged` (D-242, D-265). Copilot and agent-team PRs are often squash-merged, so each recipe's release step should say so.
- **N6. CONTRIBUTING after the move.** Section 4's "CODEOWNERS requires Head Keeper review" will be false for a glossary in CONTRIBUTING: CODEOWNERS lists `/README.md`, not `/CONTRIBUTING.md`. Either add `/CONTRIBUTING.md` to follow-up 12.2 or reword the rule. The "60 entries, `docs/GLOSSARY.md` linked from the README" line goes stale. So does the CODEOWNERS comment "README.md defines every Waggle term" (Head Keeper follow-up). Moved entries that are stale: Intent Packet "goes in the merge commit message" (the gate reads the PR body), `waggled`, and Gate "may ride a lane". Mark them or trim them along with the v0.2 cuts.
- **N7. Light and full claims.** `--gate light|full` appears in the README with no plain-terms definition, and its only gate effect is the Tier 2 rule (`changecheck.go:97-100`, `backend.go:465-471`). Have `docs/gate.md` define it once.
- **N8. AC1 wording.** "`docs/RFC-0003.md` differs in line 1 only" means one inserted line above the heading. Say "one added line, line 1", so a guard does not read it as a replaced heading. The same applies to the README.
- **N9. Q-T037-1 (b).** Binding `copilot/` branches by the PR body's `Waggle-Ticket` also reopens D-263, under which PR-body trailers are author text and warnings only. Name it next to D-234 and D-237. Recommendation (a) stands.
- **N10. Q-T037-5.** `gastown.dev` did not resolve for me either, so T2 is still unverified. T3 rests only on PR #2609 (`gastownhall/gastown`, merged 2026-03-12): `merge_strategy` defaults to `direct`, `pr` runs `gh pr create` on a rebased branch, it lives in a formula variable, and the page does not say who merges. That is PR text, not docs. Expect the (b) fallback, and if the queen keeps T3, cite the PR as a PR. Row C6 checks out: the permissions page says a Bash deny rule "isn't a security boundary around the program" and shows `Bash(git push *)` missing `git -C . push origin main`.
- **N11. README size.** 90 lines is tight if the calling workflow is inlined. The base template is 107 lines before phase A's removals. Put the full YAML in `docs/gate.md` or the recipes index and link to it from the README.
- **N12. Tier 3 list.** The hardcoded Tier 3 row should name the `.waggle/hive/` carve-out (`backend.go:41-43`), so readers do not take `.waggle/hive/` for Tier 3 in the tier check. It is still never approvable (D-261) and never leasable.

Verdict: must-fix

## Round 2 (cd3bf5a)

Risk-Tier: 2

Reviewed `.waggle/hive/tasks/T-037.md` at `cd3bf5a` (diff from `faf50d1`, rebased on `e82da6f`) against the gate code at `e82da6f`, `tasks/T-036.md` and D-270 to D-277.

### Round 1 must-fixes

- **opus M1 (License): closed.** 3.1 allows exactly one edit, dropping "and the `waggled` daemon"; AC10 checks it.
- **opus M2 (five checks): closed.** 3.1 now writes the five items. Item 3 matches D-261 (hardcoded paths pass only by admin bypass, `gate_approve.go:90-104`). Item 4 states trailers as warnings and the `git log` condition (D-263). Item 5 states net change and the admin merge (`gate.go:150-157`: "intermediate commits ... are not inspected"; `backend.go:455-458`). The pitch carries the required-check condition and L1. See the new finding R2-M1 on item 2.
- **opus M3 (T-036 recipe items): closed.** `--owner` is required in a gate-only repo (3.1, 3.3). A pushed or released build is required, with module-build tag pinning (L10, T-036 2.4 rules 1 and 2). Version skews: D-239, and the lease `version` field at `backend.go:113`. L11 matches the T-036 section 5 note, and AC9 covers it. L5 frames D-276 as a mitigation with a pending spike, cites the CODEOWNERS header for the ruleset setting, and keeps the organization-ruleset alternative, which is in D-276's Keeper choice.
- **opus M4 (sources): closed.** G5 cites `available-rules-for-rulesets`, and G6 holds the plan question as a queen check. G4 cites `custom-instructions-support` and quotes the cloud agent rows. Copilot cloud agent is the name in the title, the file name, 3.3 and AC8.
- **fable M1 (L1): closed.** I confirmed `classify` and `claimFor` (`gate_commits.go:259-334`): the gate reads whichever task the commit names, with no tie to the PR or its author. The claim map row, L1, pitch, item 1, the README line and AC9 all carry it. The fix is described as a mitigation, and nothing in the docs text promises a product fix.
- **fable M2 (fork): closed.** `waggle-gate.yml.tmpl:47-48` is `git fetch origin 'refs/waggle/claims/*:refs/waggle/claims/*'`, and L3 now says what the code does.

Q-T037-5 (b) is framed fairly. Option (a) is stated with its requirements. The merge-queue link is hedged ("appears"), with the release note as its source. L12 says "untested", not "broken". Option (b) is reversible. Budget: 590 bottom-up is under round 1's 595, and the estimate, target and stop (1,190 / 1,550 / 2,140) stand. The planned cut is concrete. I also re-checked the new citations: `claim.go:159, 162, 220`, `gate_verify.go:125-142, 207`, `git.go:602-603`, `gate_commits.go:16-23, 172-173, 193-194` and `backend.go:29-53`. All hold.

### Blocking issues (must-fix)

**R2-M1. README item 2 states a claim property the gate does not check, and it contradicts L3.** 3.1 item 2 reads "The lease is the directory list in the task's claim, written before the work by someone other than the agent". `claimFor` (`gate_commits.go:313-336`) reads the claim as it exists at verify time. It checks neither when the claim was written nor who wrote it. A claim pushed after the commits, by the agent itself, passes. L3 says exactly that. AC2 locks the README checks to 3.1 "item by item", so the builder must put this sentence in the checks list, where a reader takes it as enforced. That is the misleading security claim the threat model names. Fix: state it as a requirement on the user, not a check, and tie it to L3. For example: "The lease is the directory list in the task's claim. The gate does not check who wrote the claim or when, so a human or the orchestrator should write it before the work (L3)."

### Notes (not blocking)

- **N1. L1 against today's ruling.** The L1 parenthetical and follow-up 5 still say the question was "taken to" the Keeper role. It has now been ruled: a product fix before G5, in a separate ticket. Update both. If that fix merges before T-037's dispatch base, item 1 ("The task is author-chosen"), the claim map row at `gate_commits.go:259-334`, L1 and AC9 must be rewritten from the merged code, not copied from the brief. Add one sentence telling the builder this, so AC9 does not force a limit that is no longer true. The docs must still not mention a planned fix.
- **N2.** Item 3's "Go dependency files" also has to cover `vendor/` (`backend.go:38`). Name it, or the builder may list only `go.mod` and `go.sum`.

Verdict: must-fix

## Round 3 (09ec17a)

Risk-Tier: 2

Reviewed the diff `cd3bf5a..09ec17a` of `.waggle/hive/tasks/T-037.md` (8 lines changed), against `claimFor` (`cmd/waggle/gate_commits.go:313-336`) and `HardcodedTier3Prefixes` (`internal/comb/backend.go:29-39`) in the worktree.

### Round 2 must-fix

- **R2-M1 (check 2 claim provenance): closed.** 3.1 item 2 now states only what the code checks: the lease is the claim's directory list, every commit is checked, exact case, no new symlink or submodule, only a base merge accepted. The provenance sentence is set apart "as a requirement on the user and not as a check", says the gate does not check who wrote the claim or when, cites `gate_commits.go:313-336`, and ties it to L3. I confirmed `claimFor` reads the claim ref as it stands at verify time and checks only presence, well-formedness and expiry (warn only). AC2 now requires each README check to state only what the code checks and forbids presenting author or time as part of a check.

### Round 2 notes

- **N1 (L1 ruling): folded in.** The L1 parenthetical and follow-up 5 carry the Head Keeper's 2026-10-24 ruling (product fix before G5, separate ticket). Both are brief text; the parenthetical tells the builder "no promise of a fix" and "no mention of a planned fix", so nothing in user docs promises it. The builder rule covers both orders: fix merged at the dispatch base means item 1, the claim-map row, L1 and AC9's L1 item are written from merged code; otherwise today's behavior. The Depends line now holds dispatch until the L1 ticket is settled, and follow-up 5 has the queen reopen L1, README item 1 and the claim-map row if the fix lands after T-037. Consistent.
- **N2 (vendor/): closed.** Item 3 lists `.github/workflows/`, `.gitlab-ci.yml`, `.devcontainer/`, `.waggle/`, `go.mod`, `go.sum`, `go.work`, `go.work.sum` and `vendor/`, which is exactly the nine entries at `backend.go:29-39`.
- **L3 marker: done.** The `gate/action.yml` claim-ref fetch cite is now `<<G4: gate/action.yml claim-ref fetch line, T-036 2.2 step 3>>`, so AC7 forces the builder to replace it from merged code or drop it.

### Blocking issues (must-fix)

None.

### Notes (not blocking)

- **R3-N1.** The new L3 marker is not in section 2's "Known markers" list. AC7 still catches it, since it bans any remaining `<<G4`, so this is cosmetic; add it to the list in a later touch.
- **R3-N2.** AC9 text still says the README names "L1 author-chosen task". The L1 builder rule overrides it by naming AC9's L1 item, which is enough for a builder, but a guard reading AC9 alone could flag a correct post-fix README. If the fix merges first, the queen should restate AC9 in the dispatch note.
- **R3-N3.** Round 1 N12 (name the `.waggle/hive/` carve-out, `backend.go:41-43`) is still open. Not blocking: item 3's "fail whatever the claim says" is true for hive paths too, since `.waggle/` can never be leased.

Verdict: pass

# Brief review: T-037 refresh round (58e8c32, parent 32adb18), opus

Verdict: must-fix

Risk-Tier: 2

**Intent summary.** The refresh rebases the merged T-037 brief (G5 prose: README, `docs/gate.md`, recipes, CONTRIBUTING glossary, docs SPDX) on main `32adb18`. It resolves every round 3 `<<G4: ...>>` placeholder from merged code (section 2), re-cites `gate_commits.go` and `gate_verify.go` after T-038 A and B, makes T-037 wait for T-039 and T-040, writes T-039's claim `--branch` behavior and T-040's final strings into the docs plan, re-fetches section 4 rows C6, G4, G5, G6, T2 and T3, raises the budget to 643 bottom-up, and adds Q-T037-6 on the D-291 sunset. One file changes (`.waggle/hive/tasks/T-037.md`, +93 -89). Blast radius is the docs plan only; nothing in code, control or CI moves.

What I checked and found sound:

- **G4 values against main.** `gate/action.yml:5-14` (inputs and defaults), `:24-35` (build with `GOPROXY: "off"`), `:37-41` (`git fetch --prune origin '+refs/waggle/claims/*:refs/waggle/claims/*'`), `:43-51` (no `--branch`): all exact. Template `waggle-gate.yml.tmpl` `:5-9`, `:11-20`, `:25-27`, `:29-30`, `:32-46`, `:48-64`, `:66-72`: all exact. `init.go:391-397`, `:402-404`, `:426-429`, text `:162`, warning `:159`; the `[warn]` print is at `:438-440`, not `:437-439` (off by one, note). `main.go:58-65` strings exact. `gate-config.yaml.tmpl:9-17` exact. `doctor.go:25-28`, `:32-34`, `:56-59`, `:76-81`; `probe.go:36-41`; `probe_github.go:66-73`, `:153-170`, `:172-189`: all exact. No `release.yml` at `32adb18`, so "build from source at a pinned, pushed commit" is the only install path named, which matches main.
- **gate_commits.go and gate_verify.go re-cites.** `:16-23`, `:33`, `:51`, `:69`, `:171-172`, `:192-193`, `:197-207`, `:242-251`, `:258-309`, `:263-266`, `:277-296`, `:312-335`, `:326-330`, `:338-360`; `gate_verify.go:125-146`, `:211`, `:243-277`: all land on the stated code. `backend.go:29-39` holds the nine hardcoded prefixes the README check 3 lists; `changecheck.go:79-100` matches its rows.
- **T-039 and T-040 behavior quoted.** README checks 1 and 2, "Writing a claim", claim-map rows 79 to 81, L1, L3, L4 and L10 match T-039 2.1 to 2.5 and D-291 to D-294 as ruled. The T-040 rows the header names (1 to 3, 10, 30) are the strings the docs quote, and section 5's kept-literal list matches T-040 section 11 and its "Reached but kept" line. Order T-039, then T-040, then T-037 holds: T-040's own Depends on waits for T-039 (both lease `cmd/waggle/`, `internal/comb/`, `internal/dispatch/`), and T-037 leases nothing either touches.
- **Section 4 rows.** I fetched all six cited pages on 2026-10-24. C6: the sentence and the `Bash(git push *)` table row are verbatim on the permissions page. G4: all three instruction types listed for Copilot cloud agent, verbatim. G5: both rules and the up-to-date checkbox as a setting of the status checks rule, as stated. G6: "customers on GitHub Team and GitHub Enterprise plans", no Free or visibility split; partial status and fallback wording are honest. T2: all four quotes present (the second is a sentence fragment, which the row presents as one). T3: PR #2609 merged 2026-03-12 with the two quoted bullets, no merge-queue mention; v1.0.0 release notes say "Native GitHub merge queue integration via merge_strategy=pr". The T3 fallback wording says no more than that.
- **Budget.** 105 + 240 + 82 + 130 + 82 + 4 = 643; doubled 1,286, rounded 1,290; target 1.3 times 1,677, 1,680; stop 1.8 times 2,322, 2,320. Matches D-185 (double, split along the planned cut past the target, only the stop to the Head Keeper) and D-210 (post-review rounds toward the stop only). The planned cut (worked examples and the local hook section) removes no precondition or limit, so cutting it cannot make the docs oversell.
- **No overselling of support.** Merge queues are named unsupported and untested (L12), the T3 fallback says so, and install names only a source build at a pinned commit; release assets are conditional on a merged `release.yml`.

## Must-fix

### M1. Q-T037-6 is put to the Head Keeper on a false premise and contradicts T-042's open questions

**Evidence.** Section 13, Q-T037-6: "No ticket for that follow-up exists yet." The T-042 brief (commit `7a02279`, parent `32adb18`, not merged) is that follow-up. Its Q-T042-4 recommends (a) "Merge T-042 before T-037's L1 is written", and its Q-T042-2 (a) says "T-037's docs describe only the fail-closed behavior: L1 ... drops T-039 2.5's fallback bullet, and the troubleshooting text quotes the renew hint and says to re-run the check". Q-T037-6 recommends the opposite (do not wait, describe the fallback). Ruled at their recommendations, the two cards contradict each other, and the T-037 builder would be told both to write and not to write the fallback.

The main reason Q-T037-6 gives for (a), that waiting "puts that chain in front of G5's prose", is weak. D-291 already puts the sunset (pin move, renewal of every hive claim, T-042) before the G5 flip, so that chain is in front of G5 either way. And per Q-T042-2, the first public release carries T-039 and T-042 together, so under (a) the fallback text describes behavior that no public user ever runs. It is guaranteed to be rewritten before the flip, at the cost of another two-guard docs round (D-282).

**Fix.** Rewrite Q-T037-6 with T-042 in view, and make its recommendation agree with Q-T042-4. I recommend this text: "(b) T-037 dispatches after T-042 merges. Expected order: T-039, T-040, the pin move and claim renewals (T-042 section 9), T-042, T-037. Fallback, only on a queen card: if the pin move or the renewals slip past T-040's merge, T-037 dispatches under (a) and the queen notes the touch-up on the card." Then carry the ruling through the brief:

- the header Depends on adds T-042 and the fallback;
- under (b), L1, claim-map row 80, L4 ("or fallback") and L10 ("puts it back on the fallback") state the fail-closed result instead, quoting the T-039 renew hint `<id>: claim names no branch; renew it with waggle claim --branch` and saying to re-run the check, per Q-T042-2 (a);
- section 12 item 5 ("T-037's own claim passes on `t-037` through the D-291 fallback") and item 10 get updated. Under (b), T-037's claim must carry a branch list written by a T-039 build, since the pin will have moved;
- section 6's "The D-291 sunset itself (Q-T037-6)" names T-042.

If the queen prefers to keep (a), Q-T037-6 still has to name T-042, state that it conflicts with Q-T042-2 (a) and Q-T042-4 (a), and send both cards to the Head Keeper together.

### M2. L12 says `waggle doctor` reports `merge-queue` as unknown, but the code does that only under a config value a gate-only install does not set

**Evidence.** L12: "`waggle doctor` reports `merge-queue` as unknown (section 2)". `probe_github.go:66-73` adds the `merge-queue` finding only when `opt.MergeStrategy == "native_queue"`. The finding is not `Required`, so it never changes the exit code (`doctor.go:76-81`). The default is `up_to_date` (`internal/config/config.go:31`, `:127`), and `gate-config.yaml.tmpl` seeds no `forge.merge_strategy`. On a default gate-only repo, doctor prints no merge-queue line at all. A reader who trusts L12 would read a clean doctor run as "no merge queue in play", which the code never checks.

There is a second gap. `forge.merge_strategy: native_queue` is a valid config value (`config.go:292-295`), and the brief's `docs/gate.md` plan has a "config reference" section. Listing that value there without a caveat reads as merge-queue support, and setting it makes doctor skip the required `up-to-date` finding. Both breach the section 7 bar: a statement the cited code does not do, and overselling merge-queue support.

**Fix.** Reword L12's last clause to: "`waggle doctor` does not detect a merge queue; only with `forge.merge_strategy: native_queue` set does it skip the up-to-date check and print an informational `merge-queue` finding marked unknown, and that setting does not make the gate run in a merge queue (`internal/forge/probe_github.go:66-73`)." Section 2's doctor bullet already conditions it correctly; add one builder rule to 3.2: the config reference either omits `forge.merge_strategy` or documents `native_queue` with that same sentence. Add both to AC9's L12 check.

## Notes (not must-fix)

1. `init.go` tag warning is printed at `:438-440`, not `:437-439`. The builder re-reads at the dispatch base anyway.
2. T-042 cites `tasks/T-037.md:98` (L1) and `:218` (section 12 item 5). At `58e8c32` they are `:99` and `:220`. T-042's finalize round should fix these, and T-039's brief carries the same stale cites.
3. The C6 row's section name ("what a Bash rule doesn't match") matches the page's anchor text `#bash-rule-limits`. The builder should link the anchor, not the prose name, so AC5 can check it.
4. T2's second quote is a fragment of a longer sentence ("It instructs polecats, after their final build/pre-verify passes, to push ..."). It is fine as quoted, but the recipe should not present it as a full sentence.
5. Row G1 is still marked verified from the first fetch and was not re-fetched in this round. It is a low-risk quote, but the queen may want it re-fetched with the others before dispatch, since the page was renamed to Copilot cloud agent.
6. `changecheck.go:96` (listed-path finding) says "needs an admin bypass to merge", which is untrue when a listed approver approves. T-040 does not reword it, and the docs that quote it need the plain gloss "or a listed approver's approval at the head commit". The builder should not quote it bare.
7. Under (b) in M1, the expected order puts the pin move after T-040. That lets one Head Keeper pin commit also carry T-040 row 30 (T-040 section 10 item 2), which saves a second pin card.

## Recommended ruling for Q-T037-6

(b), with a queen-card fallback to (a): T-037 dispatches after T-042 merges, in the order T-039, T-040, pin move and renewals, T-042, T-037. This agrees with T-042's Q-T042-2 (a) and Q-T042-4 (a). It costs about a day, mostly queen and Head Keeper work that D-291 requires before the flip anyway. It saves a guaranteed second two-guard docs round, and it keeps out of the public docs a fallback that no public release ever runs.

<!-- SPDX-License-Identifier: MIT-0 -->
# Brief guard review: T-037 refresh fix round 2 (dad99b3, parent 3ce85e1), opus, confirming

Verdict: pass

- **Reviewed:** `.waggle/hive/tasks/T-037.md` at `dad99b3` (one file, 8 insertions, 8 deletions against `3ce85e1`). Checked against `cmd/waggle/gate_commits.go` at `8978c62` (T-039) and `32adb18` (main), `gate_approve.go:88-104` at `32adb18`, `tasks/T-042.md` at `67d04fd`, and the prior confirming review `brief-T-037-fable-r4.md`. Read-only: `git show`, `git diff`, `git log` and file reads only.
- **Risk-Tier:** 2 (a hive record; the docs ticket stays two guards per round under D-282).

## Intent summary

The round answers fable r4 M1 (under Q-T037-6 (a) the README claim section, L4 and L10 stated only the fail-closed result) with one scoping sentence at the end of Q-T037-6 (a), widens the (a) reopen list from four spots to six in both Q-T037-6 (a) and section 12 item 10, and folds in fable r4 notes N2 (T-042 cite to `67d04fd`), N9 (pitch commentary), N10 (L10 names other claim writers), N11 (one dry-run build name) and N12 (the T-040-stall fallback names its option). Blast radius: the brief text only. No code, control or CI file moves.

## Must-fix

None.

## Confirmations

1. **M1 closed.** The scoping sentence appears once (line 233, end of Q-T037-6 (a)), names the three spots fable r4 listed (README "Writing a claim" fail sentence, L4's last clause, L10's fail clause), points them at T-039 2.4 item 3 (b) and 2.5, and ends "Under (b) none of this applies." Its warning text `claim for <id> names no branch; accepted on ticket branch name` matches `gate_commits.go:357` at `8978c62` byte for byte after the ID (`"claim for "+id+" names no branch; accepted on ticket branch name"`), and the fail text matches `:360`. The behavior it states (accepted on the ticket's own same-repo branch names, any other head fails) matches `checkHead` at `:355-361`. For L4, the expired path at `:330-334` warns and then calls `checkHead`, so the fallback applies to an expired no-list claim too, as the sentence implies.
2. **Six-spot list consistent.** Q-T037-6 (a): fallback in L1, the claim-map row, L4, L10 and the README claim section, plus "the one hedge clause" in README check 1 and the pitch. Section 12 item 10: "the fallback sentences in L1, the claim-map row, L4, L10 and the README claim section, and the hedge clause in README check 1 and the pitch". Same members in both; the count of six treats the shared hedge clause as one spot, the same convention the round 1 "four" used (three fallback spots plus the hedge). The Status line says six as well.
3. **Pitch approver exception accurate.** At `32adb18`, `gate_commits.go:221` is `if s.approvable != nil && s.approvable(v.Path) && (v.Rule == comb.RuleTier3 || v.Rule == comb.RuleOutsideLease)`, dropping the finding when `s.filter` is set (the approved rescan). `approvablePath` (`gate_approve.go:90-104`) is true only for a path at Tier 3 by policy and under no hardcoded prefix, so "a repo-listed path" is the right scope, and check 3 (line 59) already states the listed-approver rule at the exact head commit. The no-branch failure has no path, so the exception does not reach the branch half of the pitch; the text does not claim it does.
4. **L10.** The added clause "as does any tool that writes a claim ref without `branches` (git plumbing, a future relay; Q-T042-2 (a))" sits inside the existing older-CLI sentence and inherits its fail clause, which the M1 scoping sentence covers under (a). Correct.
5. **Dry-run build naming.** Item 1 now defines "the dry-run build: the moved-pin build of item 5, or under Q-T037-6 (a) a build of main carrying T-039 (either records `t-037`, D-292)"; item 5 refers to "the dry-run build". Under (a) the fallback is taken precisely when the pin move slips, so naming a main build carrying T-039 there is right; both builds record `t-037` (`request.go:118-119` default per fable r4 N5).
6. **T-040-stall fallback.** "alone, under the Q-T037-6 option the card states: (b) if T-042 has merged by then (it does not depend on T-040), otherwise (a) with its section 12 item 10 touch-up". T-042's Depends on at `67d04fd` (lines 16-21) lists T-039, the pin move and the claim renewals, and no T-040, so the parenthetical holds.
7. **No regression.** The diff touches only the Status line, Depends on, the pitch paragraph, L10, section 12 items 1, 5 and 10, and Q-T037-6. No em or en dash in the file at `dad99b3` (grep for U+2013 and U+2014 empty). The Head Keeper is named by role. Budget lines are unchanged and the new text is brief instruction, not doc lines.

## Notes (not must-fix)

- **N1. Q-T037-6 (a) list punctuation.** "the docs describe the T-039 fallback in `docs/gate.md` (L1, the claim-map row, L4, L10) and the README claim section, README check 1 and the pitch carry the one hedge clause (3.1)" can be misparsed as the README claim section carrying the hedge. A semicolon after "the README claim section" would fix it. The scoping sentence that follows removes any practical doubt, so this is cosmetic.
- **N2. Item 5 is now near-tautological.** "the item 1 dry run uses the dry-run build" says nothing item 1 does not; "uses this build" or dropping the clause would read cleaner.
- **N3. The T-040-stall (b) branch is rarely reachable under the ruled order.** Q-T042-4 (a) (D-303 to D-307, per the caller) puts T-040 before the pin move, so with T-040 stalled T-042 would normally not have merged unless the queen's card also moves the pin ahead of T-040. The text is conditional and correct; the queen card should say which it is, which the sentence already requires.
- **N4. The fallback gloss omits the ignored-branch guard.** `:356` also requires `branchTicket(h.name)` not ignored. "its ticket's own same-repo branch names" is a fair plain-terms reading and the builder writes from T-039 2.4 item 3 (b), which carries the detail.
- **N5. The pitch cite is at main.** `gate_commits.go:221` is the `32adb18` line; at `8978c62` the same check is `:223`. Within the brief's re-read-at-dispatch-base allowance.
