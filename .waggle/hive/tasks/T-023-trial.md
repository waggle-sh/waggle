<!-- SPDX-License-Identifier: MIT-0 -->
# T-023 phase F trial runbook (section 10.4 item 4)

- **Status:** Written by the scout on 2026-10-21 with hive PR `t-023f-switch` (10.4 item 3); fix round 1 on 2026-10-22 for `reviews/T-023f3-{opus,fable}.md` and D-214 to D-216. Not run. The queen runs it with the Keeper present on 2026-10-22; steps marked **[CARD]** wait for a Keeper card, and nothing after a card runs before the Keeper answers it.
- **Layer:** `hive`. Nothing here changes product code; the trial ticket T-026 is built by dispatch.
- **Evidence:** `.waggle/hive/media/T-023/trial/`: every `result.json`, the gate run, `waggle status` token totals, `versions.txt`, the host-record and clean output, the redacted `ps` captures (`ps-*.txt`) and `report.md`. Steps write the `ps` captures to `$S/t023-trial/` first; only the redacted copies go under `media/`. Committed in a records batch after the trial.
- **Decisions in force:** D-147 (switch after phases A to D), D-176, D-197 and D-214 (probe shapes and the survivors_killed cutover gate), D-193 and D-195 (no extra sweeps or signals on the Mac), D-198 (host-record card), D-199 (report sentence and the `&` measurement), D-203 (threat bar), D-211 (`--no-cell`, no first-dispatch card), D-213 (session close mid-run), D-215 (step 3 defaults).

## 0. Rules for the whole trial

- **Placeholders.** `<main>` is the main checkout root; `$S` is the queen's scratchpad; `P=790ade0b64b8cf5a6d6f6c1522c47cf5fdcd3548` (the pin); `D=/tmp/t023-trial-pids` (the probe pid files). Run git and gh as `env -u GITHUB_TOKEN -u GH_TOKEN ...` like every other call in this hive.
- **How the queen starts a dispatch.** Every `waggle dispatch` below runs through the Bash tool's background mode (`run_in_background`), which keeps stdout and reports the exit. That follows the role template (Dispatching workers item 1: a background shell command, never in the foreground, never under `nohup`). Do not use `cmd >/dev/null 2>&1 &` for dispatch itself: the last stdout line names the run record. The `>/dev/null 2>&1 &` form below is a tool command the probe worker runs inside its session (D-199).
- **D-193.** Do not run any `sweep()` probe, `kill`, `pkill` or other signal by hand on the Mac, and do not run dispatch tests that go through `Run`. The only sweeps in this trial are the ones real `waggle dispatch` runs perform on their own worker's tree, which is the product path D-197 and D-214 make this trial prove. `ps` is read only and allowed. Every probe child exits on its own (at most 300 s), so an escaped child is recorded, never signaled. If a check ever needs a signal, it runs in `docker --context colima-waggle` (2 CPU, 4 GiB) under the lock dir `$S/colima.lock` (`until mkdir "$S/colima.lock"; do sleep 10; done`, `rmdir` after), never the default `colima` context.
- **Never** run `git worktree prune`, remove a lock, marker or dispatch worktree by hand, or rerun dispatch with other flags to get past a refusal. Every nonzero exit goes to the Keeper.
- **Redaction.** `result.json` holds absolute `worktree` and `run_dir` paths, and `ps` command lines carry the Claude binary path, shell snapshot paths and the temp root. Every copy saved under `media/` goes through `sed "s#$(pwd -P)#<repo>#g; s#$HOME#<home>#g; s#${TMPDIR%/}#<tmp>#g; s#$S#<scratch>#g"`, and the records batch runs `scan-hive.sh` before it is pushed.

## 1. Preconditions (queen)

1. **[CARD]** Head Keeper approves and merges hive PR `t-023f-switch` (upgrade to `P`, `forge.remote: origin`, `context:` block, hooks, `WAGGLE.md` section 4, this runbook, `tasks/T-026.md`). Then pull main in the main checkout (records-batch habit: restore copied hive files first).
2. Confirm the state, one `check-ignore` per path (with several paths it exits 0 when any one is ignored): `git -C <main> grep -n 'remote: origin' main -- .waggle/control/config.yaml`; `git -C <main> check-ignore -q .waggle/control/usage.jsonl`; `git -C <main> check-ignore -q .waggle/control/handoffs/x`; `git -C <main> check-ignore -q .waggle/worktrees/x`; `git -C <main> show main:.waggle/hive/tasks/T-026.md | head -3` (dispatch reads the brief only from base).
3. Build a stamped binary from a clean checkout of the pin:
   ```
   git -C <main> worktree add --detach "$S/t023-trial/pin" "$P"
   go -C "$S/t023-trial/pin" build -o <main>/bin/waggle ./cmd/waggle
   go version -m <main>/bin/waggle | grep vcs; claude --version
   ```
   Save the `vcs.revision` (must equal `P`, `vcs.modified=false`) and the `claude --version` line to `media/T-023/trial/versions.txt`; `waggle version` prints only a constant. If `claude --version` differs from 2.1.304 (probe 2), say so in the report; `Prepare` fails closed if a security flag is missing (10.1).
4. Regenerate the main checkout's shims (the PR's upgrade ran in the scout's worktree, so the gitignored `.claude/agents/waggle-*.md` in `<main>` still carry the old roles). With a clean main checkout: `(cd <main> && bin/waggle upgrade)` (dry run: expect the pin unchanged, every file `[same]` except the `[rewrite]` of `.github/workflows/waggle-gate.yml`, and `[shims]`), then `(cd <main> && bin/waggle upgrade --apply)`, then `git -C <main> checkout -- .github/workflows/waggle-gate.yml` (D-143) and `git -C <main> status --short` (expect nothing). Then ask the Keeper to restart the queen session so it loads the new role and shims. `git -C <main> worktree remove "$S/t023-trial/pin"` after the build.
5. Auth preflight, read and write: `env -u GITHUB_TOKEN -u GH_TOKEN GIT_TERMINAL_PROMPT=0 git -C <main> ls-remote origin refs/heads/main` and `env -u GITHUB_TOKEN -u GH_TOKEN GIT_TERMINAL_PROMPT=0 git -C <main> push --dry-run origin HEAD:refs/heads/t-026-preflight` must both exit 0 (a dry run authenticates and creates nothing). Dispatch fetches the base, writes claim refs and pushes the branch with the Keeper's own git credential helper on the host, not the `gh auth git-credential` override subagents used. If either fails, stop: **[CARD]** (credentials, never a default): the Keeper either signs in the existing helper or sets a repo-local helper himself.
6. Baseline: `<main>/bin/waggle status --repo <main> > "$S/t023-trial/status-before.txt"`; `mkdir -p <main>/.waggle/hive/media/T-023/trial`.
7. **[CARD]** Start the trial. The card says: real dispatch runs on the Mac with `--no-cell` (D-211), each run's own end-of-run sweep of its worker's tree included (D-197, D-214; D-193 bans only extra probes and hand signals); save open work first; the probe runs use a scratch hive with no remote; cost is three short probe sessions plus one Tier 2 ticket (builder sonnet, guard opus by policy, since the guard must differ from the builder and `standard` has no alternate).

## 2. Probe runs in a scratch hive (D-176, D-197, D-214, D-199)

The first macOS trial runs. A scratch repo keeps the probe commands out of our `WAGGLE.md` and keeps the deliberate timeout off a real ticket.

1. Scratch hive (local only, so nothing is pushed):
   ```
   T="$S/t023-trial/hive"; mkdir -p "$T"
   git -C "$T" init -q -b main
   git -C "$T" config user.name paulternate
   git -C "$T" config user.email 27632860+paulternate@users.noreply.github.com
   (cd "$T" && <main>/bin/waggle init)
   ```
   Optional cost cut: in `$T/.waggle/control/config.yaml` set `standard: haiku` (the builder level) and leave `frontier`; this is the scratch hive only.
2. Probe files, all committed on `main` of the scratch hive. Each perl file starts with `mkdir '/tmp/t023-trial-pids'; open my $f, '>', '/tmp/t023-trial-pids/<name>.pid' or die; print $f $$; close $f;` and then sleeps:
   - `probe/held.pl` (`<name>` = `held`, `sleep 120;`) and `probe/hold.sh`: `perl probe/held.pl & sleep 1`. The tool shell lives about 1 s, so the 25 ms tracker lists it and its group, and `held.pl` outlives the worker's normal exit (fable T-023f3 M1).
   - `probe/bg1.pl` and `probe/bg2.pl` (`bg1`, `bg2`, `sleep 90;`) and `probe/bg.sh`: `perl probe/bg2.pl >/dev/null 2>&1 &`.
   - `probe/probe.pl` (`probe`, `sleep 300;`).
   - In `$T/.waggle/control/WAGGLE.md` section 3, add four bullets: ``- **Hold:** `sh probe/hold.sh` ``, ``- **Background:** `perl probe/bg1.pl` ``, ``- **Background script:** `sh probe/bg.sh` ``, ``- **Probe:** `perl probe/probe.pl` ``.
   - `.waggle/hive/tasks/T-901.md`: "Run exactly these Bash tool calls, one per call, in this order, and nothing else: 1. `sh probe/hold.sh` 2. `perl probe/bg1.pl >/dev/null 2>&1 &` 3. `sh probe/bg.sh`. Do not edit or create files. If a call is refused, go on to the next one. Then end at once with a one-line report."
   - `.waggle/hive/tasks/T-903.md`: "Run exactly one Bash tool call, `perl probe/probe.pl`, and nothing else; wait for it. Do not edit or create files. End with a one-line report."
   - `.waggle/hive/tasks/T-902.md` (run 0c): the same text as T-903.
   - `git -C "$T" add -A && git -C "$T" commit -q -s -m "trial probe"`
3. The capture command used below, matching only the probe files so no other session's shells are collected (opus T-023f3 M3): `ps -axo pid,ppid,pgid,sess,stat,command | grep -E 'probe/(probe|bg[12]|held)\.pl|probe/(bg|hold)\.sh' | grep -v grep`. Call it `CAP`. The tool shell that runs a probe matches too, since its command line carries the eval'd probe command (`media/T-023/d176-probe-2.txt`).

### Run 0a: outliving child on the normal exit path (D-214 gate)

1. `rm -rf /tmp/t023-trial-pids`. Dispatch in the background: `<main>/bin/waggle dispatch T-901 --role builder --dir probe/ --gate full --no-cell --timeout 10m --repo "$T"`.
2. As soon as `/tmp/t023-trial-pids/held.pid` exists (the Keeper says when, or the queen checks once), run `CAP > "$S/t023-trial/ps-0a-during.txt"` once. No loop.
3. On exit (expect 1, no commits; `end` is the session's normal end): copy the redacted `result.json` to `media/T-023/trial/run0a-hold-result.json`. Then for each pid file in `/tmp/t023-trial-pids/`: `ps -o pid,stat,command -p "$(cat /tmp/t023-trial-pids/<name>.pid)" >> "$S/t023-trial/ps-0a-after.txt"`.

### Run 0p: probe 2 shape on the timeout path (record only)

1. `rm -rf /tmp/t023-trial-pids`. Dispatch in the background: `<main>/bin/waggle dispatch T-903 --role builder --dir probe/ --gate full --no-cell --timeout 90s --repo "$T"`. 90 s leaves room for session start and stays under the Bash tool's 120 s limit, so the timeout, not the tool, stops `probe.pl`.
2. Once `probe.pid` exists: `CAP > "$S/t023-trial/ps-0p-during.txt"` once. On exit (expect 5, `end: timeout`): copy the redacted `result.json` to `media/T-023/trial/run0p-probe-result.json` and capture `ps -o pid,stat,command -p "$(cat /tmp/t023-trial-pids/probe.pid)" > "$S/t023-trial/ps-0p-after.txt"`.
3. Dispatch sends SIGTERM first and SIGKILL only after its 10 s grace (`killGrace`, `cell.go`). Record which happened: `survivors_killed` lists the tool shell (`zsh`) and the `perl` from `probe.pid` (the SIGKILL path was reached, the probe 2 shape), or `survivors_killed` is empty and both are dead (Claude Code cleaned up on SIGTERM).

### Gate (D-197 as set by D-214)

Over runs 0a and 0p, a listable leftover is a pid whose tool shell the tracker could list: `held` in run 0a, and `probe` with its shell in run 0p when the SIGKILL path was taken.

- **Pass:** every listable leftover pid is in `survivors_killed` and dead in its after capture.
- **Blocks cutover:** any listable leftover pid alive in an after capture, or listed alive with no `survivors_killed` entry. Stop the trial and bring `result.json`, the `ps` captures and the stderr log to the Keeper **[CARD]**.
- **Not evidence, not a block:** Claude Code's own SIGTERM cleanup (`survivors_killed` empty, shell and child dead). Record it.
- **Recorded, not required:** whether the SIGKILL path of run 0p was reached.
- If Claude Code also cleans up `held.pl` itself on its normal exit (it is dead and not in `survivors_killed`), the sweep was not exercised: say so on the cutover card rather than counting it as a pass.

### D-199 measurement (not the gate)

For `bg1` (the literal `cmd >/dev/null 2>&1 &` tool command) and `bg2` (the same shape from a script) in run 0a, record one of: refused by the permission rules (the transcript shows a denial), caught (its pid is in `survivors_killed`, so the group sweep saw its tool shell alive), or escaped (alive in `ps-0a-after.txt`, left to exit on its own). If `bg1` was refused, the report says the D-199 measurement rests on `bg2` alone (opus T-023f3 N5). Fable D2 N10: `probe.pl` exercises the pid path; `held`, `bg1` and `bg2` the group path.

## 3. Run 0c: session closed mid-run (D-213)

1. `rm -rf /tmp/t023-trial-pids`. The Keeper opens a second terminal, starts a throwaway Claude Code session in `$T` (not the queen's session) and asks it to run in the background: `<main>/bin/waggle dispatch T-902 --role builder --dir probe/ --gate full --no-cell --timeout 10m --repo "$T"`.
2. Once `probe.pid` exists, record dispatch's pid and the worker's: `ps -axo pid,ppid,pgid,sess,stat,command | grep -E 'waggle dispatch T-902|probe/probe\.pl' | grep -v grep > "$S/t023-trial/ps-0c-before.txt"`. Then the Keeper closes that session. Record how: `/exit` or closing the terminal tab.
3. From the queen's session, without waiting in a loop: check once after the 10 m timeout plus one minute (11 minutes after the start).
   - If the scratch run record's `result.json` exists: copy it (redacted) to `media/T-023/trial/run0c-session-close-result.json`. Record `end` (`interrupted` means a hangup reached dispatch; `timeout` or `done` means it kept running after the session closed) and `survivors_killed`.
   - If no `result.json` exists: dispatch was killed without a record (the closing session SIGKILLed its tree). This is an expected case, not a failure of the runbook: record it with a `ps -o pid,stat,command -p <pid>` of the dispatch pid and the worker pid from `ps-0c-before.txt`.
   - Either way record whether `probe.pl` outlived the run, and whether the scratch `dispatch.lock` and the `T-902` worktree remain under `$T/.waggle/worktrees/` (`reviews/T-023f-opus.md` N6). Never signal an orphan; it exits within 300 s. A stale lock in the scratch hive goes with the scratch hive's deletion card (7.2).

## 4. Host record (D-198, fable D2b N7)

1. Before the first dispatch in our hive: `<main>/bin/waggle dispatch clean --host-record --repo <main>`. Expected: `no host record; the next run writes one`. A first line `warning: took over stale run lock ...` may precede any listing line, `host record matches` included; it is not part of the listing. Save the output to `media/T-023/trial/host-record-first.txt`.
2. If any dispatch below exits 3 because host files changed: run the listing again, put the full listing on a **[CARD]**, and only after the Keeper approves run `<main>/bin/waggle dispatch clean --host-record --accept <digest> --repo <main>` with `<digest>` from the one line of the form `digest <16 lowercase hex>`. If `--accept` exits 3, list again and ask again.

## 5. Run 1: T-026 builder (first remote-mode run)

1. Lease and gate from `tasks/T-026.md`: Lease line `internal/dispatch/`, Gate line `full`. Dispatch in the background from `<main>`: `env -u GITHUB_TOKEN -u GH_TOKEN <main>/bin/waggle dispatch T-026 --role builder --dir internal/dispatch/ --gate full --no-cell`. End the turn on the Keeper.
2. On exit: read `result.json`. Exit 0 with `pushed: true` is the clean case. Check the claim reached the remote: `env -u GITHUB_TOKEN -u GH_TOKEN git -C <main> ls-remote origin 'refs/waggle/*'` lists a T-026 claim ref. Save `result.json` (redacted) as `media/T-023/trial/run1-builder-result.json`. A failed remote-mode final claim sync taints the worktree (reopus N6): bring it to the Keeper; `clean` on a card recovers it.
3. Open the draft PR: `env -u GITHUB_TOKEN -u GH_TOKEN gh pr create --draft --base main --head t-026 --title "T-026: dispatch test fixtures turn off git auto maintenance" --body-file <file>` (body: link to the brief, the site list the builder reported, the added-line count).
4. Gate run: when CI finishes, save `gh pr checks <n>` and the `verify-leases-and-tiers` job log to `media/T-023/trial/gate-run.txt`. This is the first PR whose claim the gate can see (D-051, D-119), so `verify-leases-and-tiers` should pass unless an update-branch merge commit is present (D-159 and D-168: the T-014 AC7 proof moves here). Any other failure goes to the Keeper.

## 6. Run 2: T-026 guard

1. In the background: `env -u GITHUB_TOKEN -u GH_TOKEN <main>/bin/waggle dispatch T-026 --role guard --branch t-026 --no-cell`. The policy picks the guard model (opus here, since it must differ from the builder's sonnet). Tier 2 needs no `--second`.
2. On exit 0 with a `verdict`: check `head_oid` equals `gh pr view <n> --json headRefOid`; if it moved, the review is stale. Then save `review.md` unchanged as `.waggle/hive/reviews/T-026-<model>.md` (`<model>` from `result.json`) and the redacted `result.json` as `media/T-023/trial/run2-guard-result.json`.
3. `must-fix`: rerun the builder with the same run 1 command (the branch exists, so it is a must-fix round) and the guard again; a later round by the same model is appended to the review file after one blank line (D-205 rules for confirming guards). Two consecutive must-fix rounds on one function mean a design card (D-204).

## 7. Totals, clean and report

1. `<main>/bin/waggle status --repo <main> > <main>/.waggle/hive/media/T-023/trial/status.txt` (claims and per-ticket token totals; redact paths).
2. **[CARD]** `waggle dispatch clean T-026` (removes the guard worktree and any kept worktree; never the branch or the claim). Run it only after the card; save its output to `media/T-023/trial/clean.txt`. The scratch hive (`$T`) and `/tmp/t023-trial-pids` are deleted after the report, also on that card (a delete).
3. Write `media/T-023/trial/report.md`: one line per run (ticket, role, model, `end`, exit, tokens, cost, wall time); the D-214 gate result with the pids, and whether run 0p reached the SIGKILL path; a sentence that an empty `survivors_killed` after a SIGTERM self-cleanup is Claude Code cleaning up, not a sweep result; the D-199 sentence below verbatim; the D-199 measurement for `bg1` and `bg2`; D2 opus N8 (a darwin child still in `SIDL` when its parent is reaped can be missed by one read: say whether any child in this trial was missed and that the sample cannot rule N8 out); fable D2 N8 (on darwin the group sweep is exercised only by this trial: say which survivors came from the pid path and which from the group path); fable D2 N10 (trial shape: probe 2 for the pid path, `held`, `bg1` and `bg2` for the group path); D-213's outcome, how the session was closed, and whether the lock and worktree remained; the host-record listing and whether the stale lock warning appeared; the gate run; the token totals; anything the template did not cover.
4. D-199 sentence for the report (copy exactly): "on macOS (sysctl read every 25 ms) a background child escapes until T-024 when its tool shell started and exited between two reads, which is the usual cmd & from a short-lived shell (0 of 20 caught with a shell gone within 5 ms, 20 of 20 with one alive 20 ms), or when it calls setsid or daemonizes before the next read; a child that stays in the group of a tool shell the tracker listed alive is killed by the group sweep"
5. Records batch: reviews, media and the brief edits on their own branch; run `scan-hive.sh`; expect exit 2 if a review round was appended (WAGGLE.md section 4).

## 8. Cutover (10.4 item 5, after the trial)

1. **[CARD]** Cutover: shows the report, the D-214 gate result (a live listable leftover blocks cutover), T-026's merge state and the rollback path (subagent shims stay for the queen and scout sessions). On approval the queen records one `waggle decide` entry for the cutover in that session (not before), writes the next unused `.waggle/control/handoffs/queen-<n>.md` from `queen.local.md` in the template format, and deletes `queen.local.md` (a delete on the same card), which turns the Stop hook check off.
2. Follow-up control PR (Tier 3): drop the transitional Stop hook and its `settings.json` entry, the `queen.local.md` ignore line and the legacy sentences in `WAGGLE.md` section 4.

## 9. Threat model (D-203)

Must-fix for this runbook and its PR: a step that breaks the queen's session (a hook that blocks every turn, a missing handoff with no fallback), a step that puts private data into a committed file (unredacted run records, probe captures with home or temp paths or other sessions' commands, the Keeper's name or personal address), a dispatch run that can reach cutover without the D-214 gate, or a step that signals a process or deletes outside the worker's own tree from ordinary inputs. Note, not must-fix: an attack that needs write access to the git dir, to `.waggle/worktrees/` outside the worker's own tree or to another ticket's worktree, or a process that already escaped the tracker, since each can already do the same damage directly.
