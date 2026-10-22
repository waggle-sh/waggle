# Guard review: hive records batch 15 (PR #64, `hive-records-15` at `0a06c05`, base main `790ade0`)

Verdict: pass
Risk-Tier: 3

Guard on opus. Records-only batch under the D-183 records card, threat bar D-203: must-fix is for records that misstate a decision or review, privacy leaks, a broken append-only `DECISIONS.md`, or a carry routed so it steers phase E or F wrongly. Nothing in this batch meets that bar. The notes below are small edits the queen can take in this batch or the next one.

## Intent Summary

One commit. It appends D-211 to D-213 (no first-dispatch card for v0.1, a later product rename of the upgrade error prefix, the phase F trial records what dispatch does when the session closes), adds the three T-023 phase F step 2 guard reviews (`reviews/T-023f-opus.md`, `T-023f-fable.md`, `T-023f-fable-confirm.md`), records the #63 merge at `790ade0` in `tasks/T-023.md` (status, 10.4 item 2 with its carry, a D-213 trial bullet that folds opus N6, fable confirm N1 and N3 in 10.3), applies hive-records-14 N1 to N4 (inlined 10.8.1 guard-prompt sentence, the D2 sweep notes pointer in `tasks/T-024.md`, the flake-bullet precision clause, fable D2b N6 answered by D-210), and moves the queen summary to main `790ade0` with 213 entries and next ID D-214. Blast radius: the records phase F runs from on 2026-10-22 and the carry phase E's scout will read; no product file changes.

## Checks

1. **DECISIONS.md (probe 2).** `git diff --numstat` gives 57 added, 0 removed. `waggle gate decisions --base origin/main --head origin/hive-records-15` (built from this worktree into the scratchpad) prints `decisions: ok, appended D-211, D-212, D-213`, exit 0. Each entry has Date, Kind, Tickets, Supersedes, Question, Options considered, Hive recommendation, Keeper choice and Rationale, in the D-210 format. 213 `## D-` headings, last `## D-213`. Keeper choices match the #63 merge commit: D-211 (`--no-cell` kept, no first-dispatch card; opus check 1 cites it), D-212 (rename in a T-012 or T-021 follow-up, drop rule 4's second sentence, not phase F; merge "Carry: D-212"), D-213 (trial verifies session close; template wording stays; merge "Closing the session mid-run is verified in the phase F trial (D-213)").

2. **tasks/T-023.md (probe 1).** Every new line checked against its source:
   - Status: `790ade0` (#63), reviews `reviews/T-023f-*.md`, D-211 to D-213: matches `git show 790ade0`.
   - 10.4 item 2: "11 added lines in one file" matches the merge stat (11 insertions, one file). "Opus and fable first reviews at `0df202d` were must-fix, fix round 1 at `4eb77da` was confirmed by one fable guard (D-205)" matches both review headers and the confirm header. D-212 carry matches D-212's Keeper choice. Fable confirm N2 (scan exits 2 on an appended review, one line in the hive override at adoption) matches the confirm review's N2 and the merge carry; routing it to step 3 is right, since step 3 is where this hive adopts the template and edits its override.
   - D-213 sub-bullet under item 4: matches D-213, and the folded clause matches opus N6's last sentence ("the trial should also record whether `dispatch.lock` and the worktree remain afterwards"), correctly attributed to `reviews/T-023f-opus.md` N6.
   - 10.3 fable confirm N1: "about six must-fix rounds by one model (two at the 256 KiB `MaxFinal` worst case)", "loud exit 3", "raise the cap for review files or count bytes per round" all match the confirm review's N1 and check 4. N3 (`inputs.go:109`, pre-D-177 holdover, inert) matches. Routed to phase E as the merge's "phase E if it bites" says.
   - 10.3 flake clause: verbatim hive-records-14 N3.
   - 10.8 rules in force: the quoted sentence is byte-identical to the 10.8.1 guard-prompt sentence at `git show 1cc63ab:.waggle/hive/tasks/T-023.md` line 249.
   - Line 266: D2 opus N2, fable N3 and N11 now point to `tasks/T-024.md`. "No T-024 lease covers `internal/dispatch/reap_*.go`" holds: T-024 section 2 leases only `internal/cell/` in A to E and `internal/cell/`, `internal/config/`, `internal/runtime/` (`claude.go`), `cmd/waggle/` (`doctor.go`) and the config template in F. "Fable N6 (headroom) was answered by D-210" holds: D2b fable N6 asks which reading of the D-206 cut applies to a post-review round, and D-210's Keeper choice rules exactly that.

3. **tasks/T-024.md (probe 1).** The carry bullet's summaries match the sources: D2 opus N2 ("record a leader for any live tracked process with pgid equal to pid on every poll"), D2 fable N3 ("refreshing `leaders` on every read for a live tracked process with `pgid == pid`"), D2 fable N11 ("two consecutive successful reads that list the group empty"). Finding IDs and model attribution are right. "Routed by `tasks/T-023.md` section 10.8" is right: line 266 sits in 10.8. No lease is claimed.

4. **Reviews.** The three files are added whole (49, 55, 54 lines). Their headers name the right PR, branch, heads (`0df202d`, `4eb77da`) and base `43c75cc`.

5. **Privacy (probe 3).** Added lines and the commit message grepped for home paths, macOS per-user temp-folder paths, `<tmp>` paths, emails, hostnames and personal names: no added line holds an email, and the only one in the commit is the noreply address in `Signed-off-by`; the only name is `paulternate`. The two grep hits were false positives (`queen.local.md`, `objects/info/alternates`). `scan-hive.sh origin/main origin/hive-records-15` with `scan-names.local` copied in and removed after: 0 hits, decisions gate exit 0, exit 2 for the two task-file HOLDs, as expected for this batch. Worktree clean afterwards apart from this file.

6. **Dashes (probe 4).** No U+2013 or U+2014 in any added line or in the commit message (perl scan). Author and sign-off are `paulternate <27632860+paulternate@users.noreply.github.com>`.

7. **queen.md (probe 6).** 213 entries and next ID D-214 match the file at head. "Main at `790ade0`" matches the base, and the step 2 sentence matches the merge. Refresh date 2026-10-21 matches. Two stale lines elsewhere in the file are N2 and N3.

8. **Opus N1 (probe 5).** Ruling: yes, the code half belongs in 10.3 as a phase E carry. The template half is done (`4eb77da` keys the PR on `result.json` `pushed: true`, confirmed by fable confirm check 5). The code half is still live: `run.go:606` builds the `survivors_killed` warning with `%v` of `[]Survivor`, and `Survivor.Command` is the raw `comm` collected in `reap_unix.go` (`killed = append(killed, Survivor{p.pid, p.comm})`), so a worker-left process whose name holds a newline still writes a line of its choosing to dispatch's stdout. It is the same class as opus D2b N2 (`checkHostRecord` keys through `printable`), which 10.3 already carries, and phase E's lease already covers `internal/dispatch/`. Not a must-fix: the record faithfully copies the #63 carry line, which omitted it, and after `4eb77da` no template rule acts on that stdout line. Text in N1. Note also that fable's first review check 11(e) says the `pushed:` line "cannot be faked"; opus N1 shows it can. The review is committed unchanged as the record, so no edit, but N1's bullet keeps the correct finding findable.

## Blocking Issues (Must-Fix Only)

None.

## Notes (non-blocking)

- **N1 (opus N1 code half, `tasks/T-023.md` 10.3 and lease line).** Add after the fable confirm bullet in 10.3:

  "- Phase F step 2, opus N1: the `survivors_killed` warning (`run.go:606`) prints `%v` of `[]Survivor`, whose `Command` is the raw process `comm` from `reap_unix.go` (up to 15 bytes on linux, 16 on macOS), so a worker-left process named with a newline puts a line of its choosing (for example `pushed: x`) on dispatch's stdout; pass each `Command` through `printable` when building the warning. The template keys on `result.json` `pushed: true` since `4eb77da`, so this is defense in depth."

  In the phase E lease line (line 17), after "`checkHostRecord`'s key print in `prerun.go`," add "the `survivors_killed` warning in `run.go`,". In 10.4 item 2's carry, change "Fable confirm N1 and N3 are phase E items (10.3)." to "Fable confirm N1 and N3 and opus N1's code half (survivor names through `printable`) are phase E items (10.3)."

- **N2 (`queen.md` Next work item 1 still lists the template commit).** It reads "(section 10.4: Keeper queen-template commit, upgrade, config, trial, cutover card)", and step 2 is merged. A session reading the list first could plan the template commit again. Replace with "(section 10.4: upgrade, config, trial, cutover card; the queen-template commit, step 2, merged at `790ade0`)".

- **N3 (`queen.md` open notes from hive-records-7).** "N1 (D-157 routes fable N1, a Go error-prefix change, to the phase F template edit; \"sonnet note 3\" is ambiguous; clarify in the phase F brief)" is now settled: #63 took the handoff-format note and D-212 routes the prefix rename to a T-012 or T-021 follow-up. Replace that clause with "N1 closed by #63 and D-212 (the prefix rename is a T-012 or T-021 follow-up)", or drop it.

- **N4 (two wordings of the same routing).** `tasks/T-023.md` line 266 says the D2 sweep notes go to "whichever comes first, the Cell phase that owns the worker's process tree or the next `reap_*.go` lease"; `tasks/T-024.md` says the Cell phase "decides whether the sweep still needs them; otherwise they go to the next `internal/dispatch/` sweep lease". Both lead to the same action in practice and neither misstates a review; no change needed unless the queen wants one wording.

- **N5 (fable confirm N3 label).** 10.4 calls fable confirm N3 a "phase E item"; the merge commit calls it inert and the review says "a later cleanup". 10.3's bullet says "a cleanup could drop the read", which is accurate, so the label is only loose.

## Evidence

Commands and outputs are in Checks. The `waggle` binary was built into the session scratchpad; `scan-names.local` was copied in for the scan and removed. No media file was written.
