# Confirming guard review: hive records batch 15 (PR #64, `hive-records-15` at `d69fe23`, polish of `0a06c05`, base main `790ade0`)

Verdict: pass
Risk-Tier: 3

Confirming guard on fable (D-205, one round, alternating model). The first guard on opus passed `0a06c05` with notes N1 to N3; this round covers the polish commit `d69fe23`, which applies those notes and adds the opus review as `reviews/hive-records-15.md`. Threat bar D-203: records text, so must-fix is for a privacy leak into a repo going public or a record that misstates a decision or the code. Nothing in the polish meets that bar.

## Intent Summary

One commit on top of `0a06c05`, three files, 64 added and 4 removed lines. `tasks/T-023.md` gains a 10.3 bullet carrying the code half of phase F step 2 opus N1 to phase E (the `survivors_killed` warning prints raw process names), adds `run.go` to the phase E lease line, and widens the 10.4 item 2 carry sentence to name that item. `handoff/queen.md` drops the already-merged queen-template commit from Next work item 1 and closes the hive-records-7 N1 open note by #63 and D-212. `reviews/hive-records-15.md` is the opus review of `0a06c05`, with its privacy check line reworded by the scout so no literal temp-path pattern lands in the repo. Blast radius: the phase E lease and carry list that phase E's scout reads, and the queen summary's next-work line for the 2026-10-22 phase F session; no product file changes.

## Checks

1. **N1 to N3 applied as written.** The new 10.3 bullet at `tasks/T-023.md` line 225 is byte-identical to the quoted text in the opus review's N1 (extracted both, `diff` empty). The phase E lease line (line 17) now reads "`checkHostRecord`'s key print in `prerun.go`, the `survivors_killed` warning in `run.go`, `printable`'s doc comment in `hostrecord.go`", the insertion point and wording N1 asked for. 10.4 item 2's last carry sub-bullet is the exact replacement sentence from N1. `queen.md` Next work item 1 carries the exact N2 replacement text and the old "Keeper queen-template commit" wording is gone from the file. The hive-records-7 open-notes line carries the exact N3 replacement clause; "sonnet note 3" and the D-157 routing clause no longer appear in `queen.md`. Each of the four target strings occurs exactly once; "opus N1" appears in `tasks/T-023.md` only at the new bullet and the 10.4 sentence (the two other matches are unrelated text at lines 262 and 340), so nothing was duplicated.

2. **The 10.3 bullet's technical claim, against `origin/main`.** True in every part. `internal/dispatch/run.go` line 606 is `s.Warnings = append(s.Warnings, fmt.Sprintf("survivors_killed: %d process(es) outlived the worker and were killed: %v", len(s.Survivors), s.Survivors))`. `Survivor` (`result.go:17`) is `{PID int; Command string}`, and `reap_unix.go:153` fills it with `Survivor{p.pid, p.comm}`, where `comm` is the raw table value: on linux the parenthesized field of `/proc/<pid>/stat` (`reap_linux.go:37`, kernel `TASK_COMM_LEN` 16 with NUL, so 15 bytes), on macOS a 17-byte NUL-terminated field of `kern.proc.all` (`reap_darwin.go:38`, so 16 bytes). Neither reader escapes newlines. `printable` exists at `hostrecord.go:106` and is applied in `hostrecord.go:79`, `hostrecord.go:102` and `clean.go:147`, not in `run.go`. The warning reaches dispatch's stdout: `cmd/waggle/dispatch.go:105` prints every warning as `note: <w>` to `stdout` (and in `--json` mode the raw string lands in `result.json`). So a survivor named `x\npushed: x` does put a line of its choosing on stdout. "Defense in depth" is right: `4eb77da` keys the template's PR step on `result.json` `pushed: true`, and `result.json` is encoded JSON, where the newline is escaped. The 15 and 16 byte figures leave room for `pushed: x` (9 bytes) plus a newline.

3. **Review line 29 (redaction).** The line is the opus privacy check. It names the pattern classes in words ("home paths, macOS per-user temp-folder paths, `<tmp>` paths, emails, hostnames and personal names") and keeps every finding: no email in added lines, the noreply address only in `Signed-off-by`, `paulternate` the only name, the two named false positives, `scan-hive.sh` 0 hits, decisions gate exit 0, scan exit 2 for the two task-file HOLDs, worktree clean. Nothing in the surrounding checks or notes depends on the literal patterns, and the verdict and N1 to N5 are untouched. I cannot see the pre-redaction text, so this confirms coherence and completeness of the record, not a byte diff.

4. **Privacy, whole branch against `origin/main`.** 293 added lines over 8 files (`DECISIONS.md` +57, `queen.md`, `T-023.md`, `T-024.md` +1, four review files). Grepped the added lines for home-directory prefixes, the macOS per-user temp-folder prefix, the system temp prefix, `TMPDIR`, email-shaped tokens, `.lan`, `.ltd`, "hostname" and the Keeper's first name: three matches, all benign (`paulternate` in the T-023f-fable check 14 line; the redacted line 29 itself matching the word "hostnames"; the GitHub noreply address `27632860+paulternate@users.noreply.github.com` in the opus review's dashes check, which already appears in five committed reviews on `origin/main`, so it is not a new exposure, and it is public commit metadata of the public handle). A two-capitalized-word heuristic over the added lines found only headings and product terms. `scan-hive.sh origin/main origin/hive-records-15` with the Keeper's names file copied in and removed afterwards: 0 hits, `decisions: ok, appended D-211, D-212, D-213`, exit 2 for the two expected task-file HOLDs. Both commit messages (`0a06c05`, `d69fe23`) hold no path, no address beyond the noreply sign-off, and name only `paulternate`.

5. **Dashes.** A perl scan for U+2013 and U+2014 over all 293 added lines, over both commit messages (author, subject, body, trailers) and over the full text of the three polished files: none.

6. **Nothing else broken.** `git diff 0a06c05 origin/hive-records-15` touches exactly the four hunks the notes call for plus the new review file; no other line of `tasks/T-023.md` or `queen.md` changed. The lease line stays one sentence with its parentheses balanced. The new 10.3 bullet sits directly after the fable confirm N1 and N3 bullet, where N1 placed it, and before the 10.4 heading. The 10.4 sentence still ends the carry list under item 2. `queen.md` item 1 still names section 10.4 and keeps the D-197, D-199, D-198 and D-117 clauses unchanged.

## Blocking Issues (Must-Fix Only)

None.

## Notes (non-blocking)

- **N1 (phase E brief wording).** When phase E's brief is cut from 10.3, the fix for the opus N1 bullet should also cover the `--json` path: `printable` on each `Command` before the `Sprintf` fixes the text warning, but `result.json` carries `Survivors` as structured fields too, and a reader of that file sees the raw `command` string. That is already safe (JSON escapes control bytes) and needs no record change now; it is a hint for the builder so they do not also strip the structured field.

- **N2 (opus review header says "One commit").** `reviews/hive-records-15.md` describes the branch at `0a06c05` as one commit; the branch now has two. The header pins the reviewed head, so the record is accurate for what it reviewed. No edit.

- **N3 (the noreply address in reviews).** The address is public GitHub metadata and has precedent on main, so it is not a leak. If the Keeper ever wants reviews free of it, a one-line records habit ("author `paulternate`, noreply sign-off") would do; no action for this batch.

## Evidence

Commands and outputs are in Checks. The scan ran from this worktree with the names file copied into `.waggle/control/` and removed after; the worktree is clean apart from this file. No media file was written and nothing was committed or pushed.
