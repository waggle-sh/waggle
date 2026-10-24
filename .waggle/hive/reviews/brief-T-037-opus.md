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
