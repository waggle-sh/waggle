<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: hive-process-1 (PR #56, head 5e1c79d, base origin/main 2ecd449)

Verdict: must-fix
Risk-Tier: 3

## Intent Summary

PR #56 implements Keeper rulings D-182 to D-185 in the hive layer and bundles hive-records batch 11. The working queen handoff moves to the gitignored `.waggle/hive/handoff/queen.local.md`. The Stop hook now refuses a missing file, one over 40 lines, or one older than main's last commit. SessionStart prints the local file, or the committed `queen.md` summary when the local file is missing. A new `.waggle/control/scripts/scan-hive.sh` scans a branch for privacy, token and dash hits, runs the decisions gate, and exits 0 only when the branch is clean and touches nothing outside `.waggle/hive/`, which lets a records batch merge without a guard or card (D-183). The queen role gains handoff, records-batch and review-depth sections, and the scout role gains budget rules. Records: D-178 to D-186 appended, the D0 and D0b reviews, and media for T-023, T-023d0, T-023d0b and hive-process-1. Nothing ships. The blast radius is the queen's session start and stop, every future records batch (a merge with no guard), and repo history privacy. Two issues block. The scan misses a hit that is added in one commit and removed in a later commit, so with `--merge` it lands in main's history. And the committed summary still describes the old hook and records rules.

## Blocking Issues (Must-Fix Only)

**B1. scan-hive.sh only scans the net diff, so a hit added and then removed on the branch passes with exit 0 and stays in main's history.**
The script runs `git diff mb..head` plus each commit's author, committer and message. It never looks at intermediate commit contents. Probe `added-then-removed` (commit 1 adds an email, commit 2 replaces it) returns `exit 0`, "clean and hive-only: merge with --admin", on both macOS and Linux (`media/hive-process-1/guard-plant-output-macos.txt`, `guard-linux-output.txt`). Records merges use `gh pr merge --merge --admin`, which keeps every branch commit. The new queen role text makes this the normal path: "Exit 1 or 2 ... fix the hit", and a fix is usually a follow-up commit. D-183 promises a check on "every added line of a records batch", and the Redaction list in `queen.md` exists because history leaks cost a history rewrite. Fix: scan each non-merge commit against its first parent as well as the net diff. For example, feed `git log --no-merges -p -U0 --no-color --no-ext-diff --no-textconv --no-renames --format='commit %h' mb..head` into the same awk with the commit as part of the label. The other option is to require squash merges for records batches and say so in the role. The first option is about 10 lines and keeps the role as written.

**B2. The committed `.waggle/hive/handoff/queen.md` summary still states the pre-D-182 and pre-D-183 rules.**
SessionStart prints this file whenever the local file is missing (fresh clone, new machine, first session after merge), so these lines direct a fresh queen:
- "Hooks:" bullet: SessionStart "prints this file", and the Stop hook "refuses to end a turn while this file is older than main's last commit, so refresh it after every merge batch; `touch` is not a refresh". Both are false at this head. The Stop hook checks `queen.local.md`, and nothing checks `queen.md` (WAGGLE.md section 4).
- "Hive records:" bullet: "every guard review, brief edit and handoff refresh is an uncommitted tracked change ... until the next hive-records batch PR (Tier 3, Keeper card)". After D-182 the handoff refresh is untracked. After D-183 a clean hive-only batch has no card.
- Last "Habits" bullet: "refresh this file after every merge batch". D-182 says phase merges and records batches only.
Rewrite those three lines to match WAGGLE.md section 4 and the queen role. Otherwise the summary contradicts the rule it introduces two paragraphs earlier, which item 5 of this review asks to rule out.

## Checks

1. **Hooks (macOS `/bin/sh` and bash, Linux dash and bash)**, probe `guard-hooks-probe.sh`, outputs `guard-hooks-output-macos.txt`, `guard-linux-output.txt`:
   - missing local file: exit 2.
   - 40 lines: exit 0. 41 lines: exit 2.
   - older than main: exit 2. Newer: exit 0.
   - outside a repo: exit 0.
   - `stop_hook_active` true, with or without a space: exit 0, so no loop.
   - only the committed `queen.md` present: exit 2 (the hook no longer accepts it).
   - SessionStart: prints the local file, falls back to `queen.md`, prints `(missing)` when both are absent, and is silent outside a repo.
   - `${f#"$root"/}` is POSIX and works under dash.
   - No hook checks the committed `queen.md`.
   - `.claude/settings.json` is unchanged and consistent (both hooks still run via `sh`).
   - Headless dispatch passes `--setting-sources=` (`internal/runtime/launch.go`), so workers never run these hooks.
2. **.gitignore**: `git check-ignore -v` matches `queen.local.md` and `scan-names.local`. It also matches case variants on this case-insensitive checkout. `git add -f` can still bypass it (N9).
3. **scan-hive.sh on this branch**: 0 hits, decisions gate "appended D-178 ... D-186", exit 2 (7 control and hook paths outside the hive), as documented. Per commit, each of the 5 commits is clean (`guard-percommit-output.txt`), so this branch's own history has no B1 leak.
4. **Planted probes, macOS BSD awk and Linux mawk with GNU grep and sed** (`guard-plant-probe.sh`). Results are identical on both, except the decisions gate, which uses a `go` stub on Linux and was tested on macOS.
   - Caught:
     - email, including uppercase, CRLF, and an email in a file name
     - `/Users/<name>`, `/home/<name>`, and the encoded `-Users-<name>`
     - the macOS per-user temp root and the harness temp prefix
     - the hostname, in any case
     - `ghp_`, `github_pat_` and `sk-ant-` tokens
     - en and em dashes, including inside a fenced code block
     - the personal-mail provider
     - a binary file
     - a hit only in a commit message
     - a hit carried by a renamed file
     - a DECISIONS rewrite (gate exit 1; the gate output does not echo content)
   - Exit 2 (fail safe): a non-hive path, and a non-ASCII file name, which `core.quotePath` turns into a quoted path.
   - Exit 3: three arguments, or an unknown base.
   - Not caught: B1; markdown-escaped emails (`jane\.doe@...`, `jane\@...`); JWT and AWS key shapes (N3).
   - A deleted then re-added line nets to no change, which is correct.
   - No false positives on `example.test`, `git@github.com:` forms or `/home/keeper` (the baseline scenario).
   - Hits print `<file>:<line>: <rule>` only. No value is printed.
5. **Names** (`guard-name-output.txt`): `SCAN_HIVE_NAMES`, `scan-names.local` and the home component all hit on whole words only, and `CI=1` skips names and hostname. See N5 for coverage.
6. **D-183 safety**: see N10. Not clearly unsafe, given B1 is fixed and the decisions gate runs (it does, and it fails a non-append edit).
7. **Records privacy**: I read the added lines by hand and searched for:
   - home paths, the macOS temp root and `/private/var`
   - token shapes and non-fixture emails
   - the hostname and the git user.name and home words, counted without printing values
   Only the scan script's own split literals and the D-160 handle matched. The reviews and probe outputs use placeholders such as `<tree-g6>`.
   DECISIONS has 186 entries, and the gate confirms D-178 to D-186 as a pure append. The `queen.md` state paragraph is true at this head: main `2ecd449`, D0 `53941e6` (#53), D0b `2ecd449` (#55), PR #56 is this branch, next ID D-187. It names no person. It does use the public GitHub handle (`keeper_id`), as D-160 does.
8. **Role and WAGGLE.md wording against D-182 to D-185**:
   - queen.md "Handoff (D-182)" matches D-182. "Records Batches (D-183)" matches, and is slightly stricter, because any non-hive path, not only `.waggle/control/`, keeps the guard. "Review Depth (D-184)" matches, using the D-136 non-test metric.
   - scout.md "Budgets (D-185)" matches.
   - WAGGLE.md section 4 replaces the old override cleanly and adds the scan's exit codes.
   - The only contradiction left is in the committed summary (B2).
9. **Dashes**: zero U+2013 or U+2014 in added lines (scan rule and a byte grep).

## Notes (not blocking)

- **N1.** On Linux, `stat -f %m` (GNU `-f` means file system) prints file-system info, and the `||` fallback appends the mtime. Then `[ -lt ]` errors, and the freshness check silently passes: the older-than-main case returns exit 0 in the container. This line predates the PR and the hive runs on macOS. Swapping the order (`stat -c %Y ... || stat -f %m ...`) fixes it on both, since BSD `stat -c` fails cleanly.
- **N2.** The awk match loops are quadratic in line length. A 100 KB line takes 20 s, and a 2 MB line ran 11 minutes before I killed it (`guard-longline-bench.txt`). The longest line in the hive today is 5.7 KB, but a jsonl transcript line can pass 100 KB. Consider skipping or truncating lines over about 64 KB with a `long-line` hit.
- **N3.** Token shapes miss JWTs (`eyJ...`, the Supabase anon and service role keys), AWS `AKIA...`, Slack `xox[abp]-`, Supabase `sbp_`, and Resend `re_`. These are in this stack, so add them.
- **N4.** Obfuscated emails (markdown escapes, `[at]`) evade the scan. That is acceptable for an accidental-leak scan.
- **N5.** Name coverage on this machine: the global `user.name` is the D-160 handle, which is excluded, so only the home component covers the first name. `scan-names.local` does not exist in the main checkout, so the Keeper's surname is not checked, though D-183 lists "the Keeper's name". Create the file before relying on exit 0, and have the script print a warning when the file is missing and `CI` is unset. Under a Linux root container, `HOME=/root` adds "root" as a name and gives 62 false positives (fail safe).
- **N6.** For a path with spaces, git appends a tab to the `+++` header, so the label loses its line number (`a b.md:: dash`). The hit is still reported.
- **N7.** The 40-line cap counts newlines. 40 lines plus an unterminated 41st, an empty file, or very long lines all pass. That is acceptable for an honest queen.
- **N8.** Any interactive Claude session in this repo other than the queen's main checkout now gets one Stop refusal per turn, because the local file is absent there; the committed file used to satisfy the check. This applies to a Keeper scratch worktree or a fresh clone. `stop_hook_active` prevents a loop.
- **N9.** `.gitignore` does not stop `git add -f`. A path rule in the scan for `*.local` and `*.local.*` would catch a forced add of the handoff or names file.
- **N10, D-183 path set.** A hive-only batch with no guard can still rewrite a task brief, which is the spec guards check builds against. It can also rewrite or delete an existing review (audit trail), rewrite `handoff/queen.md`, or add a `ROADMAP.md` or `WORKPLAN.md` change. The scout role treats those as steering docs that need the full gate. It could also add a symlink or `.gitattributes` under the hive. The decisions gate covers DECISIONS.md. Proposed tighter rule for exit 0:
  - added files under `reviews/` and `media/`
  - a pure-append DECISIONS.md
  - edits to `handoff/queen.md`

  Any modified, deleted or renamed file elsewhere under `.waggle/hive/` would exit 2. That covers `tasks/**`, existing reviews and steering docs, plus any symlink or `.gitattributes`. It is a few lines using `git diff --name-status`.
- **N11.** `okmail` accepts any address whose local part is `git` or ends in `.git` at any domain. `e.co` is a real registrable domain. Both are minor.
- **N12.** User diff config (`diff.noprefix`, `diff.mnemonicPrefix`, `diff.interHunkContext`) only skews labels or line numbers. Passing `--src-prefix=a/ --dst-prefix=b/` would pin them.

## Evidence

Under `.waggle/hive/media/hive-process-1/`:
- `guard-hooks-probe.sh`, `guard-hooks-output-macos.txt`
- `guard-plant-probe.sh`, `guard-plant-output-macos.txt`
- `guard-name-probe.sh`, `guard-name-output.txt`
- `guard-longline-bench.sh`, `guard-longline-bench.txt`
- `guard-percommit.sh`, `guard-percommit-output.txt`
- `guard-linux.sh`, `guard-linux-output.txt` (Debian bookworm container: mawk, dash, GNU grep 3.8; colima started and stopped for the run)

All planted commits were detached and never pushed or branched.

## Confirmation at 5c7ada6

Verdict: must-fix
Risk-Tier: 3

Scope: `git diff 5e1c79d 5c7ada6` (fixes `519179d` and `5c7ada6`). B1 and B2 are fixed, and the notes are applied as described. One new blocking issue: the per-commit pass skips merge commits, so a hit that only a merge commit introduces, removed by a later commit, still exits 0. The fix is one line and is tested below.

### Blocking Issues (Must-Fix Only)

**B3. A hit introduced only by a merge commit and removed later passes with exit 0.**
`lopts=(--no-merges ...)` drops merges from the per-commit content pass. Side-branch commits merged in are still scanned (probe `side-commit-merged-then-removed`: exit 1), but content that exists only in the merge result does not reach the scan. That covers a conflict resolution, or a manual edit made during `merge --no-commit`. Probes `evil-merge-then-removed` (new file) and `evil-merge-edit-then-restored` (edit to a file both parents have) both return exit 0, "clean records batch", on macOS and on Linux (`confirm-scan-output-macos.txt`, `confirm-linux-output.txt`). The queen role now says the scan covers "every commit, so a hit removed in a later commit still counts", and the script header says "a hit added then removed on the branch is still reported". Both are false for merges. Records branches do take merges: update-branch merges, and hand-resolved conflicts on `handoff/queen.md` or `DECISIONS.md`, which records batches and phase merges both edit. This is the B1 leak through a different door, on the path that merges without a guard.
Fix (tested): after the per-commit `g log` in the content block, add
`g log --merges --no-show-signature "${dopts[@]}" --diff-merges=remerge -p -U0 --format='commit %h' "$mb..$headsha"`
A remerge diff shows only what the merge result changes against git's own automatic merge, so a clean update-branch merge adds nothing. The existing awk parses it unchanged. With that line, both evil-merge probes exit 1, and every other scenario keeps its exit code (`confirm-remerge-output-macos.txt`). A records branch built on 2ecd449 and merged with 5c7ada6 still exits 0 with no hits. Add the same `--merges --diff-merges=remerge --raw` pass to the path-name block, so a file added only by a merge has its name scanned too. Needs git 2.36 or later (2.54 here). Alternatively, fix the role and header text and say that records branches must not contain merges, then check for that in the script (`git rev-list --merges` non-empty means exit 2). The one-line fix is simpler.

### Confirmed

1. **B1 fixed.** Probe `added-then-removed` exits 1 on macOS and Linux, and so do the scout's `token-added-then-removed` and `local-added-then-removed`. New evasions:
   - added then undone by `git revert`: exit 1.
   - a side-branch commit merged as the second parent, then removed: exit 1.
   - amended history, where the hit commit is no longer reachable from head: exit 0. That is correct, because the commit never reaches main.
   - binary then text, and text then binary: exit 1 (a binary hit on the flipping commit, plus the email on the text commit).
   - a `.gitattributes` with `-diff` turns the file binary: exit 1. This fails safe.
   - merge-only content: B3.
2. **B2 fixed.** The "Hooks:", "Hive records:" and last "Habits" lines of `handoff/queen.md` now match WAGGLE.md section 4, the queen role and the hooks' behavior. No other line in the summary describes the old rules.
3. **Records path still exits 0.** Probe `records-batch` exits 0 with no hits and no HOLD lines on macOS, with the real decisions gate. It combines a pure-append D-187, a new review, a new media file and a new nested media file, and a `handoff/queen.md` edit.
4. **Exit 0 set matches the text.** The script's class awk, its header, WAGGLE.md section 4 and the queen role all name the same set. Exit 0 needs an added regular file under `reviews/` or `media/`, a `DECISIONS.md` modification (the gate fails a non-append with exit 1), or a `handoff/queen.md` modification. Everything else holds with exit 2: other added, modified or deleted hive files, renames (which `--no-renames` splits into a delete and an add), symlinks, submodules and `.gitattributes`. Test results:
   - rewriting an existing DECISIONS entry at 5c7ada6: exit 1 (gate).
   - moving an existing media file: exit 2.
   The scout's N10 set reproduces at base 2ecd449, apart from base artifacts. Its `append-decisions` uses D-187, which is not the next ID at 2ecd449, so it exits 1 there. Its `rename-existing-media` file does not exist at 2ecd449, so it makes an empty commit.
   The guard probe's `decisions-edit` sed no longer matches at 2ecd449, so its exit 0 there is an empty commit. The 5c7ada6 rewrite probe above replaces it.
5. **Rerun of the planted probes against the 5c7ada6 script (macOS).**
   - `guard-plant-probe.sh` (base 2ecd449): every exit code matches `fix-plant-output-macos.txt`. JWT and AWS shapes now exit 1. The 2 MB line exits 1 at once (long-line).
   - `fix-new-probe.sh`: every exit code matches `fix-new-output-macos.txt`. The names warning prints when `scan-names.local` is missing, and is absent under `CI=1` and when the file exists. Hostile diff config does not change labels.
6. **Token rules, false positives.** `scan-hive.sh 2ecd449 5c7ada6` gives 0 hits and exit 2, holding 7 control and hook paths (`confirm-branch-scan.txt`). Over the whole history (root to 5c7ada6, 241 commits, 16 s), the 30 token hits are all `ghp_` test fixtures under the pre-existing rule. The new JWT, AWS, Slack, `sbp_` and `re_` rules have 0 hits. The `local-file` rule matches only the existing `T-023a/probe-repo/CLAUDE.local.md` fixture, which hits only if a scanned range adds it (`confirm-history-scan.txt`).
7. **Linux stat order in `handoff-check.sh`.**
   - macOS `/bin/sh` and bash: all hook cases are unchanged, and the older-than-main case exits 2 (`confirm-hooks-output-macos.txt`).
   - Linux dash and bash with GNU stat, `golang:1.23-bookworm` via `docker run --rm` on the running colima, which I did not start or stop: older-than-main now exits 2. N1 is fixed.
   - The direct check prints only the mtime in the new order. The old order prints the file-system block and then the mtime.
8. **Privacy and dashes on the added lines of 5e1c79d..5c7ada6.**
   - Emails are only `example.test` fixtures and `git@github.com`.
   - Path and provider literals appear only as the probe's split variables and as the review's placeholders.
   - Hostname and first name: 0.
   - The scout's `<planted-email>` redaction holds: 8 occurrences, and no real address remains in any `fix-*` output.
   - U+2013 and U+2014: 0.
   - The commit author, committer and sign-off are the D-160 handle.
   My new files pass the same checks.

### Notes (not blocking)

- **C1. NUL bytes hide text from BSD awk.** Git treats a file as text when its first NUL comes after byte 8000. BSD awk on macOS stops reading a line at a NUL, so a hit later on that line is missed. Probe `nul-after-8000-bytes`: exit 0 on macOS, exit 1 on Linux mawk. The hive runs on macOS. Piping the content stream through `tr '\000' '\001'`, or treating any NUL as a binary hit, closes this. Accidental leaks rarely carry a NUL, so this does not block.
- **C2. The evidence head differs from the committed head.** Every `fix-*` output is labeled `89778dd`. That commit differs from `5c7ada6` by one line: 5c7ada6 drops the email fast path `t = index(s, "@") ? s : ""`. The exit codes are the same at 5c7ada6, but timing is not. A 65536-byte line just under the cap now takes 19 s on macOS (`fix-new-probe.sh` N2 rerun), not the 1 s in `fix-longline-bench.txt`. Either restore the fast path, which is correct because a line with no `@` cannot hold an address, or relabel the bench.
- **C3.** A mode change on `handoff/queen.md` and an added `.gitignore` under `media/` both exit 0. Both are harmless and consistent with "an edit" and "an added file". No action.
- **C4.** `.waggle/control/scan-names.local` still does not exist in the main checkout, so the Keeper's surname is not checked. The new warning says so on every run (N5 carried). Create it before the first exit 0 merge.

### Evidence

Under `.waggle/hive/media/hive-process-1/`:
- `confirm-scan-probe.sh` and `confirm-scan-output-macos.txt` (5c7ada6 as committed)
- `confirm-remerge-output-macos.txt` (the same probe with the B3 line added)
- `confirm-linux.sh` and `confirm-linux-output.txt`
- `confirm-hooks-output-macos.txt`
- `confirm-branch-scan.txt`
- `confirm-history-scan.txt`

All planted commits were detached and never pushed or branched. The `guard-*` and `fix-*` probes were rerun against a copy of the 5c7ada6 script, because the base 2ecd449 checkout predates it.

## Confirmation at 462f589

Verdict: must-fix
Risk-Tier: 3

Scope: `git diff aa0e08d 462f589` (fix commit `462f589`: a remerge pass over merges in the content and path-name blocks, `tr '\000' '\001'` before the content awk, the email fast path restored, merges counted in the summary line). B3 is fixed for two-parent merges, and C1 and C2 are fixed. One new blocking issue: git skips the remerge diff for a merge with three or more parents and says so only in a stdout warning the awk drops, so a hit introduced in an octopus merge and removed later still exits 0. The fix is one line.

### Blocking Issues (Must-Fix Only)

**B4. A hit introduced only by an octopus merge (three or more parents) and removed later passes with exit 0.**
`--diff-merges=remerge` handles two-parent merges only. For a three-parent merge git prints `diff: warning: Skipping remerge-diff for octopus merges.` on stdout, with no diff, no `--raw` entry and exit 0, and the content awk drops that line because it is neither a header nor a `+` line (`fconfirm-inspect-output-macos.txt`). Probe `octopus-evil-then-removed` (two clean side commits, `git merge s1 s2`, the merge amended to add a file in no parent, a later commit replacing it) exits 0, "clean records batch", with 5 commits scanned (`fconfirm-merge-output-macos.txt`). The header's "each merge commit by remerge diff" and the queen role's "every commit" are false for that shape, on the path that merges with no guard. A hit in an octopus side commit is still caught (`octopus-side-hit-then-removed`: exit 1).
Fix (one line, fail closed): an octopus merge has no place in a records branch, so report each one as a hit. Next to the decisions gate line, after the awk that creates `$tmp/hits`:
`for c in $(git rev-list --min-parents=3 "$mb..$headsha"); do echo "commit $(git rev-parse --short "$c"):0: octopus-merge" >>"$tmp/hits"; done`
plus an `octopus-merge` line in the header's rule list. `git rev-list --min-parents=3 462f589..<probe head>` returns the planted merge. As a second guard, the content awk can treat `/^diff: warning: Skipping remerge-diff/` as a hit on the current commit, so any shape git declines to remerge fails closed. Either way, add `octopus-evil-then-removed` to the probe set.

### Confirmed

1. **B3 fixed for two-parent merges.** The builder's `confirm-scan-probe.sh`, rerun on base 462f589 with the committed script: `evil-merge-then-removed` and `evil-merge-edit-then-restored` exit 1 on the merge commit, and every other scenario keeps its exit code from `fix2-probe-output-macos.txt` (`fconfirm-scan-probe-rerun-macos.txt`).
2. **Evasions tried against the remerge pass** (`fconfirm-merge-probe.sh`, `fconfirm-merge-output-macos.txt`, macOS, real decisions gate). Each plants a hit that exists only in the merge result and is removed by a later commit, unless noted:
   - a conflict in a file both parents changed, resolved by hand to the hit: exit 1. Resolved to both sides plus the hit: exit 1 (line 3). Resolved to clean text: exit 0.
   - a directory and file conflict resolved as a file carrying the hit: exit 1.
   - a binary add and add conflict resolved as text with the hit: exit 1 (binary hits on the flipping commits).
   - `-s ours` with the hit in the side commit: exit 1 (per-commit pass). `-s ours --no-commit` plus an added file: exit 1 (remerge).
   - a criss-cross merge (two merge bases) with an added file: exit 1.
   - `--allow-unrelated-histories` with the hit in the root commit: exit 1 (the root commit diffs against the empty tree). The same merge with an added file: exit 1 (remerge with no merge base works).
   - a merge whose message alone carries the hit: exit 1 (`message:1`).
   - a merge that is the only commit adding `notes.local`: exit 1 (`local-file`, from the new `--raw` remerge pass). A merge that is the only commit adding a binary: exit 1 (`binary`).
   - a crafted merge whose second parent is an ancestor of the first (`commit-tree -p p1 -p base`): exit 1.
   - an octopus merge: B4.
3. **A clean update-branch merge still exits 0.** A records branch on 2ecd449 (new review, new media file) merged with 462f589 by `git merge --no-ff`: merge exit 0, then the scan with base 462f589 reports 2 commits scanned, 0 hits, exit 0. The remerge pass adds nothing for a clean merge. `conflict-resolved-clean` above, a hand-resolved conflict with clean text, also exits 0.
4. **NUL handling on macOS BSD awk.** `tr '\000' '\001'` runs before the content awk, and `nul-after-8000-bytes` and `nul-after-hit` exit 1 on macOS (C1 fixed). A text line reading NUL then `binary` becomes the `\001binary` sentinel and reports a binary hit, so the collision fails closed (`nul-binary-sentinel`: exit 1). Commit fields are not covered: see C5.
5. **Email fast path restored** (C2). A 65536-byte line with no `@` scans in 1 s with 0 email hits (`fconfirm-inspect-output-macos.txt`).
6. **git version.** The header says "Needs git 2.36 or later"; nothing else in the repo states it (role, WAGGLE.md and the handoff summary do not). This machine has 2.54. Under a git whose `--diff-merges` rejects `remerge` (a PATH shim that dies like 2.31 to 2.35, and one with the pre-2.30 "unrecognized argument" wording) the first `g log` in the path-name block dies, `set -euo pipefail` ends the script, and the result is exit 128 with the fatal line on stderr and no `changed paths`, `hits` or `result` line. Loud, not a silent skip. The exit code is 128, not the documented 3 (C6).
7. **Branch scan.** `scan-hive.sh origin/main 462f589`: 0 hits, 9 commits scanned (merges now counted), exit 2 holding the same 7 control and hook paths (`fconfirm-branch-scan.txt`).
8. **Privacy and dashes on the added lines of aa0e08d..462f589** (223 lines): no home path, encoded project directory, temp root, hostname or home-component word; no email at all; 0 U+2013 or U+2014. Author, committer and sign-off are the D-160 handle. Line 1 of `fix2-probe-output-macos.txt` carries a raw 0x01 byte where the builder's header means "NUL to \001"; harmless. My new files pass the same greps.

### Notes (not blocking)

- **C5. A NUL in a commit message or author hides what follows it from the scan.** `git commit-tree` refuses such a message, so this needs `git hash-object -t commit --literally`. Git's own `%B` and `%an` stop at the NUL (the author prints empty), so the identities loop never sees the bytes after it, and the scan exits 0 (`fconfirm-inspect-output-macos.txt`, three cases). That is forgery, not an accidental leak, so it does not block. A one-line fail-closed check: `git cat-file commit "$c" | tr -cd '\000' | wc -c` above 0 is a hit.
- **C6.** Under an old git the script ends with exit 128 from the pipeline, not the documented 3, and the message names `--diff-merges`, not the version. A check up front (`git version` below 2.36: "scan-hive: needs git 2.36 or later", exit 3) would be clearer, and WAGGLE.md section 4 or the queen role could state the requirement.
- **C7.** Linux was not rerun this round: colima stopped between my first `docker ps` and the container runs, and the instruction was not to start it. The remerge change is on the git side, the new `tr` call is GNU-compatible, and the 5c7ada6 Linux run covered the rest, so the risk is low. The builder's fix2 evidence is also macOS only.
- **C8.** A clean merge made by GitHub's update-branch can differ from git's local merge-ort result in rare rename cases. The remerge diff would then show lines from main as added and scan them, which can only add hits, never hide one.
- **C9.** Three malformed dangling commit objects remain in the shared object store from a first run of the crafted-object probe with a short parent id (`739f723d`, `b3e25484`, `185320b0`). They are unreachable, `git fsck` reports "bad parents" for them until pruned, and `git prune` removes them. The valid crafted ones (`6dcfe8e`, `c7f56eb`, `e2d5133`) and every planted probe commit are dangling, detached and never pushed.

### Evidence

Under `.waggle/hive/media/hive-process-1/`:
- `fconfirm-merge-probe.sh`, `fconfirm-merge-output-macos.txt`
- `fconfirm-inspect.sh`, `fconfirm-inspect-output-macos.txt` (octopus remerge output, crafted NUL commits, long-line timing, old-git shims)
- `fconfirm-scan-probe-rerun-macos.txt` (the builder's `confirm-scan-probe.sh` on base 462f589)
- `fconfirm-branch-scan.txt`

All planted commits were detached and never pushed or branched. The probe scripts were run from outside `.waggle` because the scenarios stage everything under it.

## Confirmation at 2a99fb5

Verdict: pass
Risk-Tier: 3

Scope: `git diff 462f589 2a99fb5 -- .waggle/control` (20 added lines in `scan-hive.sh`, one sentence each in WAGGLE.md section 4 and the queen role) plus the new fix3 media. The fix adds three fail-closed rules and a version gate: `octopus-merge` (every commit from `git rev-list --min-parents=3`), `remerge-skipped` (git's `diff: warning: Skipping remerge-diff` line, on the commit it follows), `nul-commit` (a NUL byte anywhere in `git cat-file commit`), and an up-front `git version` check that exits 3 below 2.36. B4 is fixed, and C5 and C6 are resolved.

### Ruling: good enough for D-183

Yes. For its purpose, catching accidental leaks on a records batch that merges without a guard, the scan is now good enough. Every ordinary queen or scout shape I could build fails closed or is scanned: normal commits, a hit added then removed, `git revert` (of a commit and of a merge), amends, cherry-picks, squash merges, update-branch merges (clean and with a hand-resolved `DECISIONS.md` conflict), conflict resolutions, renames (including through a name that is itself a hit), binaries and UTF-16, CRLF, lines over and just under 65536 bytes, UTF-8 content and non-ASCII or tab-bearing path names. The gaps left need forgery (crafted objects, deliberate obfuscation such as splitting or encoding an address), which D-183 does not try to stop. No blocking issue.

### Confirmed

1. **B4 fixed.** `octopus-evil-then-removed`, now in the builder's `confirm-scan-probe.sh`, and the same scenario in `fconfirm-merge-probe.sh` exit 1 with two hits on the merge: `remerge-skipped` and `octopus-merge`. `octopus-side-hit-then-removed` still exits 1. Every other scenario in both probes keeps its exit code from the builder's fix3 reruns (the `==` lines diff clean against `fix3-scan-probe-rerun-macos.txt` and `fix3-merge-probe-output-macos.txt`), including `records-batch` and `records-batch-update-merge` at exit 0.
2. **Each guard holds alone.** With the four-line `rev-list --min-parents=3` block removed, the octopus head still exits 1 on `remerge-skipped` alone (`oconfirm-second-guard-only-macos.txt`). A `+diff: warning` content line cannot trigger the sentinel, since added lines keep their `+`.
3. **C5 resolved.** `fix3-probe.sh` rerun: a NUL in a crafted message on the same line as the hit, on the line before it, and in the author name each exit 1 with `nul-commit`. Ordinary commits cannot carry a NUL (`git commit` and `commit-tree` refuse one), so this rule has no false positive on a normal batch; signed commits keep their signature in a NUL-free header.
4. **C6 resolved, version parse checked against real strings** (`oconfirm-version-probe.sh`, which reads the parse line from the script rather than copying it). `2.54.0 (Apple Git-157)`, `2.39.5 (Apple Git-154)`, `2.45.1.windows.1`, `2.36.0.windows.1`, `2.36.0.rc1`, `2.36.0.rc0.windows.1`, `2.36.0.vfs.0.0`, `2.100.0` and `3.0.0.rc0` run; `2.35.1 (Apple Git-136)`, `2.35.1.windows.2`, `2.35.0.rc2` and `2.34.1` exit 3 with "scan-hive: needs git 2.36 or later"; garbage, an empty string and a failing `git version` exit 3. The parse takes field 3 and its first two dot parts, so trailing text and fourth parts are ignored. The full script under a PATH shim gives the same split for the Apple, rc, windows and 2.34.1 strings. Both docs now state the requirement.
5. **Accidental shapes** (`oconfirm-accident-probe.sh`, all exits as expected): a `DECISIONS.md` append conflict from update-branch resolved by keeping both entries, exit 0 against the moved main; the same resolution with a hit slipped in and fixed in a later commit, exit 1 on the merge; `revert -m 1` of a merge whose side carried a hit, cherry-pick then removal, squash merge then removal, exit 1; a merge message amended to carry a hit and then amended clean, exit 0 (the hit is no longer in history); a rename of a file the branch added, exit 0; a rename through a file name holding an address, exit 1; non-ASCII and tab-bearing paths with a hit in content or name, exit 1; a 70000-byte line, exit 1 (`long-line`); a 60000-byte line, CRLF and UTF-8 lines with a hit, exit 1 (`email`); a UTF-16 file, exit 1 (`binary`).
6. **Branch scan.** `scan-hive.sh origin/main 2a99fb5`: 10 commits scanned, decisions gate exit 0, 0 hits, exit 2 holding the same 7 control and hook paths (`oconfirm-branch-scan.txt`).
7. **Privacy and dashes** on the 1049 added lines of 462f589..2a99fb5: no home path, encoded project directory, temp root, hostname, home component word or mail provider; 0 U+2013 or U+2014; the only address is `fixture@example.test`. Author, committer and sign-off are the D-160 handle. My new files pass the same greps, and the planted address is redacted from my outputs.

### Notes (not blocking)

- **O1. A hit in a path name prints the path as is.** The header says hits never print the matched value, but when the path itself carries the address the hit key is that path (`rename-through-hit-name`, `non-ascii-path-hit-in-name`). The scan output then holds the leak; if the queen pastes it into a PR comment or a review, the leak spreads. The hit still fails the batch, so it is not a bypass. A redaction of the key (for example the commit and the path's directory only, or a hash of the name) would close it, and the queen role could say not to paste a FAIL output.
- **O2. Non-ASCII path names hold a clean batch.** `core.quotePath=true` quotes them, so the class awk sees a path starting with `"` and reports "outside .waggle/hive/" (`non-ascii-path-clean`: exit 2). That fails closed and only costs a guard; the HOLD reason is misleading, and content hit labels under such paths keep a `"b/` prefix.
- **O3.** Ubuntu 22.04 ships git 2.34.1, so a scout or CI runner there gets exit 3. That is loud, not a silent pass.
- **O4.** Linux was not rerun: the Docker daemon is down and the instruction was not to start Colima. The new lines use `tr -cd '\000'`, `wc -c`, `rev-list --min-parents` and a plain awk `split`, all GNU-compatible.
- **O5.** `.waggle/control/scan-names.local` is absent in this worktree, so the Keeper surname rule was not exercised by the branch scan; the scan prints its warning as designed.
- **O6.** `queen-md-mode-change` (chmod +x on the committed handoff) still exits 0. A mode bit is not a leak, so this is consistent with D-183, recorded only because the classifier ignores mode on that path.
- **C9 carry-over.** The rerun of `fix3-probe.sh` wrote three more crafted, valid NUL commit objects; they and every planted probe commit are dangling, detached and never pushed. No prune or gc was run.

### Evidence

Under `.waggle/hive/media/hive-process-1/`:
- `oconfirm-scan-probe-rerun-macos.txt`, `oconfirm-merge-probe-rerun-macos.txt`, `oconfirm-fix3-probe-rerun-macos.txt` (the three existing probes on base 2a99fb5)
- `oconfirm-second-guard-only-macos.txt`
- `oconfirm-version-probe.sh`, `oconfirm-version-output-macos.txt`
- `oconfirm-accident-probe.sh`, `oconfirm-accident-output-macos.txt`
- `oconfirm-branch-scan.txt`

The probes ran from the worktree root with no untracked or ignored files under `.waggle`, each probe script and a copy of the committed `scan-hive.sh` kept outside `.waggle` while it ran.
