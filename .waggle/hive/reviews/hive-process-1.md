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
