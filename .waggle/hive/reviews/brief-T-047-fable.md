<!-- SPDX-License-Identifier: MIT-0 -->
# Brief review: T-047 (fable, round 1)

- **Reviewed:** `.waggle/hive/tasks/T-047.md` on `origin/t-047-brief` at `8f0c03f6d1230269ff01a4938db0e91e173fc323`, read with `git show`. Base `339c823`. The branch adds that one file (371 lines) and nothing else.
- **Risk-Tier:** 3 (by content, as the brief states: the doc it specifies is instructions an agent runs with a user's credentials). Tier 0 by path.
- **Threat bar (D-203):** an attack that needs write access to this repo's tags or releases, the maintainer's account, or an agent already disobeying the doc while holding the user's credentials is a note. Must-fix is a wrong fact, an untestable criterion, a contradicted ruling or a step that fails as written.

## Intent summary

The brief specifies a pinned `docs/setup.md` an agent follows to install Waggle Gate (`init --gate-only`), open the install PR, stop for a person to merge and create the rule, run `waggle doctor` and report what doctor does not check; one README subsection with the pasteable prompt; and a `cmd/waggle/setup_doc_test.go` drift test (T1 to T5) that parses every doc command against the real CLI by re-running the test binary with `-h`. Lease `docs`, `README.md`, `cmd/waggle`; no product code. Blast radius at merge is one new test in `go test -race ./...`; blast radius at the first tag is every user who pastes the prompt.

## What was checked and found correct

Every code citation was read at `339c823` with line numbers. All match: `cmd/waggle/main.go:17, 33, 41, 43-50, 66`; `internal/scaffold/init.go:107-128, 133-158, 404-406, 412-416, 430-432` and `renderFile` (`[skip]` on an existing file, `--force` applies to the workflow only, `init.go:436`); `markers.go:15-16`; `uninstall.go:112-113, 247-272` and the `[keep] refs/waggle/*` line (`:126`); `internal/forge/remote.go:14, 44-47`; `report.go:107`; `probe_github.go:150`; `github_transport.go:53-65`; `probe.go` finding ids `claim-refs`, `required-gate`, `up-to-date` and the `refs/waggle/probe/` pushes; `cmd/waggle/doctor.go:25-30, 54, 84-88` (exit codes 0, 1, 2, 3; `-h` returns 0); `gate_commits.go:306-308`; `changecheck.go:76-78, 87` (`path is outside the claim directories`); `dispatch/task.go:136-144`, and `refuseProtected` does run under `waggle claim` (`claim.go:211` calls `dispatch.ClaimTask`, which calls it at `task.go:93`), so the Q-T047-5 (b) cost statement holds; `license/rules.go:63, 112-113`, `rules_test.go:34`; `runtime/launch.go:85-87`; `layout` constants for the three `git add` paths; `forge.DefaultGateCheck`; `TestGateRefTable` and `TestUninstallGateOnly` exist and the latter has no rules block; no `TestMain` in `cmd/waggle`; `claim`, `release`, `uninstall` and `doctor` return 0 on `-h`; `init -h` exits 0 through `flag.ExitOnError` before `os.Getwd`; `gate verify` takes `--base`, `--head`, `--branch`, and a base with no `.waggle/control/config.yaml` yields an empty policy, not an error (`gate_verify.go:263-266`), so the install PR and the section 9 dry run exit 1 as the brief says.

`release.yml`: asset name `waggle_${version}_${goos}_${goarch}` (`:112`), `.tar.gz` of a directory of that name (`:149`), so `<dir>/waggle_0.1.0_<os>_<arch>/waggle` is right; `version` prints `waggle v<version>` (`:135-136`); `SHA256SUMS` is `sha256sum` output (`:164`), which the `grep -F ' <file>' SHA256SUMS | shasum -a 256 -c -` form accepts; draft release at `:207-223`; the workflow refuses a lightweight tag (`:52-56`), so the peeled `refs/tags/v0.1.0^{}` ref of Step 5 will exist. Targets are linux and darwin on amd64 and arm64 plus Windows zips (`:108`), matching Step 2 and section 10.

Live, read-only, 2026-11-01: repo `PRIVATE`; raw URL 404 unauthenticated; `rules/branches/main` prints `required_status_checks`, `non_fast_forward`, `deletion`, `pull_request`; rulesets list holds only `main`; Actions access `none`; no tags; `gh release download` has `--pattern`, `--dir`, `--clobber`; `git ls-remote <repo> 'refs/tags/vX^{}'` prints the commit for an annotated tag (git/git v2.47.0) and nothing for a lightweight one (cli/cli v2.60.0).

Records: D-100, D-136, D-185, D-193, D-203, D-205, D-210, D-229, D-230, D-235, D-236, D-257, D-275, D-277, D-341, D-343 say what the brief says. T-037 section 2 holds the "No doc names a concrete Waggle commit SHA or tag" rule (`:41`) and the `README.md` Tier 0 and file-lease facts (`:7, :9-10, :216`). T-041 2.3 rule 2 (`<<`) and rule 5 (blob URL at `main` for an existing off-manifest path) read as cited. T-036 line 50 (Actions access row), line 269, 2.10 (tag ruleset on its own card) and section 5 read as cited. `media/T-046/findings.md` selects O2 and says G5 waits for T-046b. README lines 24, 31-37, 58-60 and `docs/gate.md` sections 5, 10 (L3, L11, L12), 12 and 13 read as cited. The recipe block is at `claude-code-agent-teams.md:32-34`.

Rulings of 2026-11-01: (1) gate only, Step 4 and section 16; (2) in GA scope, section 11 inserts the build before redaction and the smoke run after the flip without reordering the D-341 or D-277 cards, and D-277's "first PR after the rewrite is the pin move" is kept; (3) URL pinned to `refs/tags/v0.1.0`, `SHA256SUMS` check in Step 3, T2 and T4 keep `main` out; (4) agent opens the PR (Step 6), person merges and creates the rule (Step 7), agent runs doctor and reports the "Not checked" lines (Step 8). The brief says plainly, in section 2 bullet 4, 8.1, 8.3 and 8.5, that the human steps are carried by a MUST NOT line only, that an agent with admin credentials can skip them, that the pins do not make the gate bind, and that `claim-refs ok` means the agent's credentials can write claims. Nothing in the brief implies otherwise.

Budget: bottom-up 187 doubled is 374; estimate 370, target 480, stop 650, planned cut named (WAGGLE.md section 4, D-185, D-210). The brief holds no en or em dash. Acceptance criteria AC1 to AC8 are checkable by reading the diff or running the listed commands; AC9 and AC10 are assigned to the queen and marked as not the build's gate.

## Blocking issues (must-fix)

None.

## Notes (not must-fix)

1. **Section 4.2 Step 5, section 8.3 row 3.** `git ls-remote ... 'refs/tags/v0.1.0^{}'` prints nothing when the tag is lightweight or missing (verified on cli/cli). `release.yml:52-56` refuses a lightweight tag, so the published release's tag is annotated and the step works, but the doc's success rule compares against an empty string in that case. Add one sentence: an empty `ls-remote` result is a stop (the tag is missing or not annotated).

2. **Section 4.3, rows 3 and 5.** An already installed repo has both `.github/workflows/waggle-gate.yml` with the `waggle:managed` line and `.waggle/`. Row 3 says "Already installed. Skip to Step 8 if the user wants a check"; row 5 says stop, fresh repo only. State the order: row 3 is evaluated first and row 5 applies only when the workflow is absent.

3. **Section 9 item 2 and Q-T047-4 (a).** The rehearsal on the demo repo assumes the demo repo has no gate workflow at the "demo repo pins" card, so that the agent's install is the pin step. D-277 says the demo repo is set up after the rewrite; D-341 lists "the demo repo" before redaction. If the earlier card installs the gate, the doc's own row 3 stops the agent at Step 1 ("Moving a pin is `waggle upgrade`, outside this doc") and the demo repo cannot rehearse Steps 4 to 7. The recommendation does not rest on a false fact, but it rests on one reading of two rulings. State the assumption, or make the throwaway repo the install rehearsal and the demo repo the pin move.

4. **Section 4.2 Step 6 and section 6 T3.** Step 6 says "add that file too" when `agent_rules` is `yes`, while T3 asserts the `git add` line names exactly the three gate paths. Tell the builder to put the `CLAUDE.md` or `AGENTS.md` add on its own line so the doc and the test agree.

5. **Section 6 T5.** `checkPlain` fails on any `hive` outside `.waggle/hive/`, which includes the words "archive" and "archives". README line 31 uses "archives" for the release assets, so a builder who copies that word into `docs/setup.md` fails T5. Say "tar.gz file" or similar in the doc, or note the trap for the builder.

6. **Section 4.2 Step 10 and section 6 T1.** `"<waggle>" uninstall` and `--apply` appear in prose, not in a fence, so T1 does not parse them. Put the two uninstall forms in a ` ```text ` fence so the drift test covers them.

7. **Section 15.** D-136 counts added non-test Go lines; counting doc lines is stricter than the rule requires and is fine, but say so in one clause so a later reader does not think the budget was misread.

8. **Section 2 bullet 2 and README Install step 3.** The brief is right that "admin merge" holds only once a rule blocks the merge. README step 3 stays as is under this lease rule; the wording is a note for a later README edit, as the brief already says.

9. **Section 4.2 Step 1.** `merge_queue` as the rule `type` value is marked unverified in section 12; GitHub's rulesets API does use that name. Section 9 confirms it at no extra cost.

Verdict: pass

# Brief review: T-047 (fable, round 3, confirming)

- **Reviewed:** `.waggle/hive/tasks/T-047.md` on `origin/t-047-brief` at `e7c9014e71a23966d2041e29234db22174cc7963`, read with `git show`. Scope: the diff from `8f0c03f` (rounds 2 and 3, commits `cb065e3` and `e7c9014`), 94 lines changed in that one file; nothing else on the branch.
- **Risk-Tier:** 3 by content, as round 1.
- **Threat bar (D-203):** as round 1. Must-fix is a wrong fact, an untestable criterion, a contradicted ruling or a step that fails as written.

## How this was checked

Built `cmd/waggle` from the branch (same product code as `339c823`) with the `release.yml` flags on go1.27.2 into a scratch directory; `version` prints `waggle v0.1.0`. Ran section 9 item 1 as written against local bare remotes: a `main` origin for Steps 1, 4, 6 and `gate verify`, a `master` origin with `init --gate-only` committed and pushed for Step 9, then the non-GitHub origin and the `.waggle/`-holding repo. Ran `-h` for `init`, `doctor`, `claim`, `release` and `uninstall` and checked every `--<name>` the doc's fenced lines carry against the help text the way T1 now specifies. Ran the T4 pattern with `grep -E`. Read `claim.go:185`, `release.go:112-115`, `gate_commits.go:35, 38`, `decisions/gate.go:50-52`, `plain_terms_test.go:20, 49, 53`, D-136, D-205, D-256, and README lines 20 to 60. One read-only `gh pr list` and `gh run list` on the probe repo. Nothing in this repo or its branches was changed.

## M1 to M10

- **M1, fixed and run.** `claim T-900 --dirs src --gate full --owner tester --branch t-900 --base origin/master` prints `claimed T-900 for tester until ...: src (gate full, claim ...)`, exit 0. `claim.go:185` is the `--base` line with "default main on the remote".
- **M2, fixed and run.** `release T-900 --unmerged --owner tester` prints `released T-900 (claim ..., unmerged)`, exit 0, and `git ls-remote origin 'refs/waggle/*'` is then empty. It also works with the `t-900` branch still on the remote, which is the order section 9 item 1 gives. `release.go:112-115` is the owner block.
- **M3, fixed.** Steps 9 and 10 use `"<waggle>"`, the form T1 and T4 parse.
- **M4, fixed and run.** `\|\s*(sh|bash)(\s|$)` does not match Step 3's `| shasum -a 256 -c -` and matches `| sh`, `|sh` and `| bash -s`.
- **M5, fixed and run.** Part one: Step 1's three `git` lines, Step 4's `git fetch`, `git switch -c waggle-gate-install origin/main` and `init --gate-only` (exactly three `[create]` lines), Step 6's first `git add` and the commit, then `gate verify --base main --head HEAD --branch waggle-gate-install` exit 1 with the `commit has no T-<id>: prefix` line, two Tier 3 path lines and `decisions: FAIL: decisions: change is not a pure append: head: entries removed or rewritten (base has 0 entries, head has 0)`, which is what Step 6 and section 9 item 1 say.
- **M6, fixed and consistent.** `gh` is a stop everywhere: Step 1's success line requires it signed in; the "No `gh`" and "`gh` not signed in" rows stop; no fallback wording remains (the only "fallback" left is Step 3's sentence that building from source is not one); section 9 item 1 skips the `gh` lines and the URL check and says why; item 3 needs `gh` as Step 3 does. The "No `gh`" row points people who will not use `gh` at the README Install steps, and those steps (README lines 31 to 35) run `go build`, `init`, a commit and `doctor`, which takes `GITHUB_TOKEN`, so the pointer holds.
- **M7, fixed.** All ten steps now carry a success clause (Steps 1, 2, 3, 4, 5, 7, 8, 9, 10 as `Success:`, Step 6 as "Success is"), so AC2 and 8.5 are satisfiable. Step 7's "the person says it is done" and the row "The person says the plan does not offer the rule" do not collide: one is the success, the other a stop.
- **M8, fixed and run.** With the T1 rule "`-<name>` at the start of a flag line", every flag on the doc's fenced lines is found once: `init -gate-only`; `claim -dirs`, `-gate`, `-owner`, `-branch` (printed as `-branch value`), `-base`; `release -unmerged`, `-owner`; `uninstall -apply`. All five `-h` calls exit 0. `plain_terms_test.go:49, 53` do compare on `-dirs` and `-unmerged` as the brief says.
- **M9, fixed and run.** With `init --gate-only` committed to `master` and pushed in the second repo, Step 9's claim exits 0, the `t-900` push succeeds, `gate verify --base origin/master --head HEAD --branch t-900` exits 1 with `leases: FAIL: other.txt: T-900: path is outside the claim directories`, and the release exits 0. The item now describes exactly that sequence.
- **M10, fixed, and the wording claims one run only.** The row's Why column says "Opus guard, run on a private repo under a free personal account (exit 1)", and section 12 lists "beyond one run by the opus guard: other plans, and an organization on a free plan" as not verified. The sentence the agent speaks ("GitHub does not offer the required rule on this private repo's plan") restates GitHub's own 403 text for that call, so it does not extrapolate past the run. Not naming a plan is right: `docs/gate.md` L2 names Team and Enterprise, GitHub's message names Pro, and this lease does not touch `gate.md`.

The non-GitHub case: `doctor` in a repo with a `gitlab.com` origin exits 3 (before `init` for the missing config, after `init` with `host "gitlab.com" is not github.com ... bad forge remote`). The `.waggle/`-holding case: a bare `.waggle/` directory does not make `init --gate-only` refuse; it writes all three files and exits 0, which is why the doc, not `init`, carries that stop. The 4.3 row cites `init.go:412-416` only for `.waggle/config.yaml` and `.waggle/roles`, so it is not wrong.

## Other round 2 and 3 changes checked

- Section 2's "the probe is still running": the probe repo has eight open PRs and workflow runs within the hour of this review, and no T-046b outcome is recorded in `.waggle/hive/`. True.
- Section 15's correction of my round 1 note 7: D-136's Keeper choice counts "numstat additions over non-test files", so doc lines count. The correction is right and my note was wrong.
- Section 15's `T-047b` sentence: `gate_commits.go:35` is `ticketSubject`, `:38` is `ticketBranch`; neither binds `T-047b` or `t-047b`.
- Step 6's `internal/decisions/gate.go:50-52` is the `n <= o` branch that prints the quoted line.
- Status: round 3 on fable follows D-205's "if a single confirming guard finds a must-fix, the other model confirms the next round"; D-256 keeps brief reviews as subagents. No ruling is contradicted. The Status's reading that the Risk Tier bullet's two-guard rule is for the build review is the author's scope statement for a rule stricter than D-205, not a contradiction of it.
- 4.1 "Pause only at Step 7 and, when asked, Step 9; a stop in 4.3 is not a pause" now matches AC3. "`gh` not signed in" resumes at the failed step; "No `gh`" resumes at Step 1, which is the only step where a missing `gh` can first fail.
- The brief holds no en or em dash, no home path and no email address. Q-T047-1 to Q-T047-7 are unchanged apart from the Q-T047-3 reversibility sentence, which now says what `uninstall` does; nothing in them rests on a false fact.

## Blocking issues (must-fix)

None.

## Notes (not must-fix)

1. **Risk Tier bullet.** The Status says its two-guard fix-round rule is for the doc's build review. The bullet itself does not say so, which is how opus N17 arose. Adding "of the build" to "Fix rounds:" keeps the next queen from re-deciding it.
2. **Section 9 item 1, the `.waggle/`-holding repo.** The item names the case but states no expected result. Run: `init --gate-only` writes three files and exits 0 with a bare `.waggle/` present. Say "expect three `[create]` lines and exit 0, which is why 4.3's row is the doc's check", so the queen's `dry-run.txt` records the point.
3. **Section 9 item 1, the non-GitHub repo.** Without `init` first, `doctor` exits 3 for the missing config, not the remote. Say to run `init --gate-only` first so the recorded message is the `bad forge remote` one the 4.3 row relies on.
4. **4.3 "No `gh`" row, Why column.** "the download while the repo is private" undersells it: `gh release download` needs a signed-in `gh` whatever the repo's visibility, so the row's reasoning holds after the flip too.

Verdict: pass

# Brief review: T-047 (fable, round 5, confirming)

- **Reviewed:** `.waggle/hive/tasks/T-047.md` on `origin/t-047-brief` at `aa4f227538cc0f493f2967774144d24100f7fb6d`, read with `git show`. Scope: the diff from `e7c9014` (commits `0b2740e`, `e34a697`, `43384dd`, `aa4f227`), all in that one file. T-048 read at `de96174` on `origin/t-048-brief`, the commit that merges first. Rulings D-349 to D-358 and `media/T-046/probe-log.md` read on `origin/main` at `1344b41`.
- **Risk-Tier:** 3 by content, as every round.
- **Threat bar (D-203):** as round 1. Must-fix is a wrong fact, an untestable criterion, a contradicted ruling, a step that fails as written, or a sentence that overclaims protection.

## How this was checked

Built `cmd/waggle` from the checkout at `1344b41` with the `release.yml` flags (`-trimpath -ldflags "-s -w -X main.version=0.1.0"`); `version` prints `waggle v0.1.0`, stamped `vcs.modified=false`. The product tree is the same as `339c823`: `git diff --stat 339c823 1344b41 -- cmd internal gate docs .github README.md go.mod` is empty, as the Status says. Ran Steps 1, 4 and 6 against local bare `main` origins in scratch space, with `HOME` pointed at an empty scratch directory and global git config off, once with neither `CLAUDE.md` nor `AGENTS.md` and once with `CLAUDE.md` seeded and the section 5 block appended; then Step 10's uninstall in the second repo; then `-h` for `init`, `doctor`, `claim`, `release` and `uninstall` with every `--<name>` the fenced lines carry. Read `report.go` `Enforcement()`, `doctor.go:25-30, 54, 84-88`, `comb/git.go:570`, `uninstall.go:105-115`, `markers.go:15-16`, `docs/gate.md` section 10 and line 106, `release.yml:100-140`, README lines 29 to 60 and T-048 sections 1 and 4. Two read-only calls on this repo: `git ls-remote origin 'refs/waggle/*'` and `gh api repos/waggle-sh/waggle/rules/branches/main`. Nothing in this repo or its branches was changed, and nothing was fetched into `refs/`.

## M11 to M13

- **M11, fixed and run.** Step 6 now runs the second `git add` only if section 5 wrote a file, and Step 10 shows the block when nothing was written. With neither file present: the first `git add` stages exactly the three init files, the commit and push succeed, and `gate verify --base main --head HEAD --branch waggle-gate-install` exits 1 with the three lines Step 6 names (`commit has no T-<id>: prefix`, two `Tier 3 path` lines for the workflow and `config.yaml`, and `decisions: change is not a pure append: head: entries removed or rewritten (base has 0 entries, head has 0)`). For the record, the skipped line would have failed: `git add CLAUDE.md` exits 128 with `pathspec 'CLAUDE.md' did not match any files`. Step 10's success clause now names both outcomes, so AC2 stays testable.
- **M12, fixed against the code.** `Report.Enforcement()` (`internal/forge/report.go`) loops over `required-gate` and the merge strategy's rule (`up-to-date`, or `merge-queue` under `native_queue`) and never reads `claim-refs`. The new line, "covers only required-gate and up-to-date", is what the code does; the merge queue variant is a 4.3 stop before Step 8, so the doc need not mention it. "If it printed that" (opus N25) covers the `advisory` case.
- **M13, fixed and matched.** Step 7 item 3 now reads "runs its own copy either way. This makes a code owner review that change before it merges. It does not cover a workflow on another branch." That is byte for byte T-048 section 4 item 6 at `de96174`, and it sits right with the probe log's own-PR row (`CLEAN`) and `docs/gate.md` L5's "Mitigation, not a fix".

## The warning, Step 7, Step 8 and Step 10 against the code and the log

- **8.1 warning.** Each added sentence has a row: "push workflow files" is the S1 rows and the bound workflow row (`UNSTABLE`, mergeable); "both delete Actions runs and write commit statuses" is the token route delete row (`UNSTABLE`, mergeable, not bound); "shown with test workflows, not with the real gate" is the log's own qualifier; "closes only the status route" is the bound token row (`BLOCKED`) beside the bound workflow row (mergeable); "a person's required approval only keeps a person in the merge" is the required review row (`REVIEW_REQUIRED BLOCKED`, forged green visible to the approver). "Give agents credentials limited to Contents and Pull requests" is advice, and "None of these is complete alone" follows it, so the S12 copy case is not contradicted; section 12 records that such a token was never run. The text is the card-confirmed text and matches T-048 section 4 item 5 at `de96174`. Nothing in it is false. "(gate spec, L11)" points at the right item after T-048: T-048 rewrites L11's first sentence to "the required check can be forged" and keeps "test workflows, not the real gate".
- **Step 7.** Item 2's "This closes the status route only" and the approval sentences are the T-048 item 6 text, follow D-356's "stating the risk without one", and the "merge with no person" claim is the other-PR row (merged, approvals 0, no admin flag). "Forged after the approval with no new push" is marked not run in section 12 and T-048 section 1 says the same. Every new sentence is inside the printed block that opens "Only a person should do this part"; nothing moves a human step to the agent, and the success rule is unchanged.
- **Step 8.** The rules list on this repo prints `required_status_checks`, `non_fast_forward`, `deletion`, `pull_request`, as quoted, and the `pull_request` parameters include `required_approving_review_count` and `dismiss_stale_reviews_on_push`, as the new paragraph says. "Not checked" naming the approval and credential permissions is right: doctor reads neither, and `internal/forge` never reads an integration id. The report lines are in a ` ```text ` fence and T4's "Not checked:" assertion still holds.
- **Step 10.** "Could push workflow files, which the install needed" is true by Step 6's push of `.github/workflows/waggle-gate.yml`. A classic `repo` token holds more than Contents and Pull requests. "Such a token can still push claim refs" follows from Contents write and L3, and section 12 marks it not run. Uninstall, run in the seeded repo after a fast-forward merge: the plan prints `[edit] CLAUDE.md (waggle block removed)` and `[keep] refs/waggle/*`, exit 0; `--apply` leaves `CLAUDE.md` with only its original line and removes the three files, exit 0. The back-out lines do what Step 10 says.
- **The second path of Step 6, run.** With `CLAUDE.md` seeded and the block appended, `git add CLAUDE.md` stages it, and `gate verify` prints exactly the same three reasons as the bare case: the `CLAUDE.md` change adds no fourth line, because the lease check fails on the commit, not per path. So Step 6's "three expected reasons" holds whether or not section 5 wrote a file.
- **T1 parse.** All five `-h` calls exit 0 and every flag on the fenced lines is found at the start of a flag line: `init -gate-only`; `claim -dirs`, `-gate`, `-owner`, `-branch`, `-base`; `release -unmerged`, `-owner`; `uninstall -apply`. No fenced command changed in this diff.

## Other changes in the diff

- **Depends on.** `comb/git.go:570` is the `!isActive(c.lease, now)` skip, so an expired claim does not block a new one. `git ls-remote origin 'refs/waggle/*'` lists six refs: `ticket-25`, `37`, `41`, `42`, `43`, `45`, as the line says; T-048 at `de96174` records the same six as expired by 2026-10-28 on a 2026-11-02 read.
- **Section 4's link clause** settles N20 inside D-236's exception; the `#10-limits-and-known-bypasses` anchor matches the heading `## 10. Limits and known bypasses`, the URL names `v0.1.0` and no `main`, so T2, T4 and AC7 still pass on it.
- **Section 7's anchor** "the paragraph that starts 'The full workflow is in the gate spec'" is README line 37 (the link brackets are what a grep misses, not a reader). T-048 edits lines 13 to 15, 20 and 33 and does not touch it; the Install section has no `###` heading today.
- **Section 9 item 2** follows D-353 (throwaway private repos, Claude Code and Codex CLI) and seeds both files, so the block path runs. **Section 11** and **section 17** carry the T-048 order and the tag ruleset check of D-354. **Section 13** marks D-351 to D-354 as the records read. **D-355** is applied: no O1 wait, no template change, N3 and N4 renumbered, section 16 reads N1 to N3.
- Hygiene: no en or em dash, no home path, no email address (the one `@` is the `git@github.com:` URL form), no login other than `waggle-sh`, Keeper by role throughout.

## Blocking issues (must-fix)

None.

## Notes (not must-fix)

1. **"The status route."** A new reader of the doc meets it twice, in the warning and in Step 7 item 2, and neither place says it means forging through a commit status with no workflow file. The warning is card-confirmed and stays. Step 7's sentence is T-048 item 6 text, not card text, and the Step 7 block is not among AC1's verbatim items, so the builder may write "This closes the commit-status route only" or add a short gloss there. Worth one clause; the Head Keeper decides on the warning.
2. **Status, last paragraph.** "M12 and M13 correct wording that `T-048.md` section 4 items 6 and 7 still carry" was true at `e387340` and is stale at `de96174`, where both items carry the corrected text and credit T-047 for it. A history clause, harmless to the builder; a records batch can drop "still carry".
3. **Depends on, "Step 8 quotes its doctor line."** Step 8 quotes the `Enforcement: enforced` label, which T-048 keeps; what T-048 changes is doctor's closing line (`report.go:107`, to `No required setting above was found missing.`), which Step 8 no longer quotes. The lease overlap carries the dependency on its own.
4. **Step 10, "wrote nothing, because neither file exists."** Section 5 has a second wrote-nothing case: the markers already present, where the agent leaves the file alone and reports. Step 10 need not show the block then, but one clause ("or the markers were already there, which section 5 reports") keeps the two sections in step.
5. **Section 9 item 2, "each agent's own file gets the block."** Under section 5 ("each only if the file already exists") both seeded files get it, which also exercises Step 6's "naming each file it wrote" with two files and uninstall's two-file loop (run here with one). Saying "both files" sets the expectation for the transcript.

Verdict: pass
