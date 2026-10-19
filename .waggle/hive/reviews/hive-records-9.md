<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: hive-records-9 (PR #51, head 6e33547)

Verdict: must-fix
Risk-Tier: 2

## Intent Summary

Hive-layer PR, base `main` at `5faf4e2` (T-023 phase C merged), three commits, all authored and committed by the D-160 noreply address. `272bff5` appends D-169 to `.waggle/hive/DECISIONS.md` (the Keeper's phase C size ruling: accepted at 1186 added non-test lines, stop moved from 1000 to 1350 with fix rounds included). `59dc2a1` commits the ten phase C0 and C guard reviews (`reviews/T-023c0-{opus,fable,confirm-opus,confirm-fable}.md`, `reviews/T-023c-{opus,fable,reopus,refable,confirm-opus,confirm-fable}.md`), eleven probe sources and outputs under `media/T-023c0/` and `media/T-023c/`, and the `tasks/T-023.md` status line (C0 and C done). `6e33547` rewrites the queen handoff after the phase C merge with a phase D carry list. Nothing under `cmd/`, `internal/`, `go.mod`, `.github/` or `.waggle/control/`; nothing ships. Blast radius: the handoff is what a fresh queen starts from and the carry list is what the phase D scout check reads. The decision, the reviews, the probes and the status line are sound and the privacy scan is clean; one handoff paragraph repeats the hive-records-8 B4 defect (in-flight state described as uncommitted).

## Blocking Issues (Must-Fix Only)

**B1. The handoff's in-flight state is false at the commit that carries it (same defect as hive-records-8 B4).** `handoff/queen.md` line 41 says "Uncommitted here for hive-records-9: ... Running: nothing. No open PRs. ... `main` is the only local branch." Line 55 says "168 entries on main, plus D-169 uncommitted here (hive-records-9)." At `6e33547` all of it is committed on branch `hive-records-9`, checked out in the queen's scratchpad worktree `hr9`, with PR #51 open; `git worktree list` also shows this guard's worktree (`agent-ae178359e6be58594`, branch `worktree-agent-ae178359e6be58594`), so `main` is not the only local branch. After merge a fresh queen would look for uncommitted files that do not exist. Fix, as hive-records-8 did after its B4:
- Line 41: "In flight: hive-records-9 (PR #51, branch `hive-records-9`, worktree `hr9` in the queen's scratchpad): D-169, the `tasks/T-023.md` status line, this handoff, the ten C0 and C reviews, `media/T-023c0/`, `media/T-023c/`, and `reviews/hive-records-9.md`. Then Keeper card, merge with `--admin`." Add `agent-ae178359e6be58594` to the locked guard worktrees to remove.
- Line 55: "DECISIONS.md: 169 entries once hive-records-9 merges (168 on main). Next ID D-170."

## Checks

1. Paths. 24 files, +2292 / -16, all under `.waggle/hive/` (`DECISIONS.md`, `handoff/queen.md`, `tasks/T-023.md`, ten new reviews, eleven new media files). No Tier 3 path, hence Risk-Tier 2 (Keeper card is still hive practice for record batches). Commits `272bff5`, `59dc2a1`, `6e33547`: author and committer `paulternate <27632860+paulternate@users.noreply.github.com>` (D-160).
2. Decisions gate. `waggle` built from `origin/main` (`5faf4e2`): `waggle gate decisions --base origin/main --head origin/hive-records-9` returns `decisions: ok, appended D-169`, exit 0. Pure append (20 lines, 0 removed), D-169 follows D-168 directly; 169 `## D-` headings on the branch. Field layout (Date, Kind `scope`, Tickets, Supersedes, Question, Options, Hive recommendation, Keeper choice, Rationale) matches D-168 exactly; the file ends with a single newline.
3. D-169 against the merge. `5faf4e2` (PR #50) says "1223 added non-test lines (D-169 stop 1350)". Recomputed by D-136 from `git diff --numstat 23bdf75 5faf4e2` over non-test files: 97+5+8+66+10+24+282+17+1+94+497+122 = 1223. 1223 - 1186 = 37 lines of fix rounds, under 1350. D-164 does say "Guard fix rounds count toward the stop", as D-169's question cites. C0: 64 added non-test lines (`launch.go`) against the D-164 stop of 130, as the handoff and status line say.
4. D-169 `Supersedes: none`. It moves D-164's phase C stop (1000) to 1350, so strictly it partly supersedes D-164. No follow-up entry needed: D-161 set the hive precedent for an accepted overrun with `Supersedes: none`, D-169's question names D-164, and hive-records-8 N1 accepted the same pattern for D-168. A new entry would only restate D-169. See N1.
5. Privacy. Every added line scanned, probe `.txt` files read in full. No `/Users/`, `/home/`, encoded `-Users-` segment, `/private/tmp/claude-<n>/`, `/var/folders/` segment, `*.local` or machine hostname, username other than the public `paulternate` login, or personal email (the only addresses are the D-160 noreply, `a@e.co`, `t@example.com`, `user@127.0.0.1` and `git@localhost` fixtures). Temp paths are redacted to `<tmp>` (for example `global:<tmp>/TestReprobeXDGIgnore1488757905/001/git/ignore`; the number is Go's random TempDir suffix). Env markers: `GITHUB_TOKEN` is only ever set to the literal `probe-secret-token`; the outputs report `secret_seen=true/false`, never a value. `GH_TOKEN`, `SSH_AUTH_SOCK` and `OP_SERVICE_ACCOUNT_TOKEN` appear as names only. No `ghp_`, `gho_`, `github_pat_`, `sk-` or similar token shapes. Clean.
6. Handoff facts against the repo and PRs (`gh pr list`):
   - #48 merged as `768e03d`, #49 as `23bdf75`, #50 as `5faf4e2`, all first-parent on `origin/main`; #51 open on `hive-records-9`.
   - C0: opus and fable reviews are at `e71d298` (base `768e03d`), both pass; confirmations of `afcc9c3` both pass. Commit subjects match.
   - C: `T-023c-opus.md` and `T-023c-fable.md` must-fix at `05343b4`; fix `22ea50a`; `T-023c-refable.md` pass, `T-023c-reopus.md` must-fix (R1 `.gitignore` wording, R2 `info/refs` from auto maintenance); fix `6d7b613`; both confirmations pass at `6d7b613`. The handoff's summary of first-round blockers matches fable B1 to B4 and opus B1 to B3 (it omits opus B4, literal dashes in a test, which is minor).
   - Phase D lease as stated (`internal/dispatch/`, `cmd/waggle/`, `internal/runtime/` `policy.go`) matches `tasks/T-023.md` line 13, so the runtime carry items do need a widened lease or a ticket, as the handoff says.
   - "T-014b fable F8 done in phase C": `5faf4e2` edits the `main.go` usage block (+5 / -1). Consistent.
   - Dropped hive-records-8 notes N6 and R-N1 are closed on main: `worktree.go:489` now says the agent can write run records without a Cell; `dispatch_test.go` asserts a refused claim writes no ref; phase C merged with the test 28 sync intact.
   - Wrong: B1.
7. Phase D carry list against the review notes. Every open note maps: fable re-review N1 (hostFiles hashes plus pre-launch compare), N2 (`global:` home path), N3 (`--ignore-submodules` variants); opus N1 (Prepare before claim), N3 (own claim OID), N4 (pipe holder), N5 (`check-ignore`), N8 (64 KiB `tap`), N9 (process-group check), N10 (git-free `adminDir`); fable N1 (`max_budget` in 9.1 and the ledger), N3 (per-turn usage), N4 (`## 3.` heading); reopus N2 and confirm-opus N2 (`GIT_CONFIG_PARAMETERS`, `GIT_CONFIG_KEY_1+`); C0 confirm-opus N1 and confirm-fable N1 (`Config()` clone), C0 opus N3 and fable N3 (word boundary), C0 confirm-fable N2 (`envFixed`), C0 fable N5 and opus N6 (`--debug` under HOME); phase B `.takeover`. Closed in the fix rounds and correctly absent: opus N2, N7, N11, fable N5, N6, reopus N1, N3, N4, C0 opus N1, N2, N5, N7 and confirm-opus N2 (the tripwire `not_enforced` line at `cell.go:47-49` names the `GIT_CONFIG_*`-clearing bypass). Not carried: N2.
8. Next work. Item 2 (phase D) then item 3 (phase F switch) matches D-147 ("after phases A to D merge") and D-117 (phase E not a precondition). Item 4 keeps T-024 phases A to E after all of T-023 (D-123).
9. T-023 status line. C0 at `23bdf75` (#49), 64 lines, `afcc9c3` confirmed; C at `5faf4e2` (#50), 1223 lines, D-169 stop 1350, two fix rounds, both pass at `6d7b613`; "Next: phase D readiness check". Consistent with the merges and the reviews.
10. Dashes and headers. Zero U+2013 or U+2014 in any added line. All ten new review files start with `<!-- SPDX-License-Identifier: MIT-0 -->`. Media files carry no header, consistent with `media/T-023b/` on main.

## Notes (non-blocking)

- **N1 (D-169 Supersedes).** Strictly a partial supersede of D-164's phase C stop. Accepted under the D-161 and hive-records-8 N1 precedent; no follow-up entry. If the hive wants the link machine-readable, record `Supersedes: D-164` on the next size ruling of this kind rather than re-recording this one.
- **N2 (carry gap, phase F relevant).** Remote-mode fail-closed taint on a flaky network (reopus N6, repeated in confirm-opus N2): when the final claim sync fails, other tickets' claim mirrors compare strictly, read as `host_state` and taint the worktree. Phase F is the first run in remote mode, so add one bullet to "Before the first live dispatch": expect `claim_sync_failed` plus a taint on a network blip, and `dispatch clean` is the recovery. confirm-opus N2's untested failed-sync `c.synced` race path can ride along.
- **N3 (Risk-Tier in a committed review).** `reviews/T-023c0-confirm-opus.md` records `Risk-Tier: 2` for a PR that `tasks/T-023.md` makes Tier 3 by content in every phase; the other nine say 3. Historical record, merged under a Keeper card either way; no edit needed, but the phase D guard brief could restate "Tier 3 by content".
- **N4 (probe reproducibility).** `media/T-023c/fable-probe_guard_test.go.txt` lines with `<tmp>/waggle-probe-canary-*` are the redacted form of real temp paths, so the file no longer reproduces its canaries verbatim. Fine for evidence; say so if anyone reruns it.
- **N5 (email in merge commits, outside this diff).** `5faf4e2`, `23bdf75` and `768e03d` are authored by the Keeper's protonmail address with committer `GitHub`: GitHub's merge button uses the account's primary email, so every merge adds another protonmail commit until the Keeper ticks "Keep my email addresses private". The handoff lists that to-do and "protonmail ... in commits" for redaction; one clause saying each merge grows the list would make the cost of waiting visible.
- **N6 (handoff wording).** The 05343b4 blocker summary omits opus B4 (literal dashes in a test). The C0 carry item "decide `GIT_CONFIG_GLOBAL=/dev/null` for workers" has no single source note; it follows from the C0 fable "Keeper global config" `pushInsteadOf` probe. Naming that source would help the phase D scout.

## Confirmation at 1249c54

Verdict: must-fix
Risk-Tier: 2

Scope: `git diff 6e33547 1249c54` (2 files, +49 / -5): the handoff rewrite and the newly committed `reviews/hive-records-9.md`. Hive layer only, no Tier 3 path.

### B1 at 6e33547: fixed

The in-flight paragraph now names hive-records-9 as PR #51 on branch `hive-records-9` with worktree `hr9` in the queen's scratchpad, lists the batch contents, and drops "uncommitted" and "No open PRs". The DECISIONS.md line now reads "169 entries once hive-records-9 merges (168 on main). Next ID D-170." Checked at the branch head:

- PR #51 is OPEN with head `hive-records-9`; the worktree list shows `hr9` in the queen's scratchpad on `refs/heads/hive-records-9`.
- `.waggle/hive/DECISIONS.md` has 169 `D-` headings on the branch (last `D-169`) and 168 on `origin/main`. D-170 is next.
- The ten C0 and C reviews (`T-023c-{opus,fable,reopus,refable,confirm-opus,confirm-fable}`, `T-023c0-{opus,fable,confirm-opus,confirm-fable}`) are on the branch.
- `agent-a5cf9263a97987f8b`, `agent-ae8076c9563372d8c` and `agent-ae178359e6be58594` all exist under `.claude/worktrees/` of the main checkout.
- The N2 bullet matches reopus N6 (remote-mode failed final sync, strict claim-mirror compare, `host_state`, taint). The N5 clause is correct: `768e03d`, `23bdf75`, `5faf4e2` are protonmail-authored with committer GitHub. The N6 source clause points at the C0 fable "Keeper global config" probe.
- No other handoff line changed. No em or en dashes in the handoff or the diff.

### Blocking

- **B1 (privacy, added line in this diff).** `reviews/hive-records-9.md` line 43 (the first review's N5), committed in `1249c54`, contains the Keeper's full name and personal protonmail address in `Name <address>` form. The provider as a word is fine; the address is not. Fix: replace the author string with "the Keeper's protonmail address" and drop the full name, in a follow-up commit before merge so the merged tree does not carry it (the copy in `1249c54` history falls under the existing history-rewrite card). Every other added address is the D-160 noreply or a test fixture (`a@e.co`, `t@example.com`, `user@127.0.0.1`, `git@localhost`). No home paths, uids or hostnames.

### Non-blocking

- **C-N1 (lock state).** The handoff says all three guard worktrees are locked by the harness. At confirmation time `agent-ae178359e6be58594` has no lock (only `agent-a5cf9263a97987f8b` and `agent-ae8076c9563372d8c` do), so it can be removed now. Only `agent-ae178359e6be58594` has a `worktree-agent-*` branch; the other two are detached, so "then delete their branches" applies to one. Harness state moves, so this is wording only.
- **C-N2 ("so far").** "(`768e03d`, `23bdf75`, `5faf4e2` so far)" reads as the full list, but earlier merge-button commits (`a06d1b7`, `3fff262`, `f816050`, `8650124`, `d60819a` and others) are protonmail-authored too. Say "since hive-records-8" or drop "so far". The redaction line's "and commits" already covers the whole set.
- **C-N3 (N6 source wording).** "where the Keeper's global `pushInsteadOf` beat the tripwire" overstates the probe: C0 fable used "a typical dotfile entry" in a global config, not the Keeper's own config. "a Keeper-style global `pushInsteadOf`" is accurate.

### Re-check after fix

Grep `.waggle/hive/` at the branch head for the address and the full name and confirm zero hits. Nothing else needs re-review.
