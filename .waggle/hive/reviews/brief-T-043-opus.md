Risk-Tier: 3

**Intent Summary:** Branch `t-043-brief` at 633029d adds one file, `.waggle/hive/tasks/T-043.md` (261 lines, no other diff from a450280). It is the first-round brief for the owner-set agent pace settings ruled in D-321 to D-324. Phase A (Tier 2) adds an optional `pace:` section to `config.yaml` and a profile pick in `local.yaml`, a resolver and load reader in a new `internal/pace/`, a pace line in `waggle status`, a pace finding in `waggle doctor`, and a print-only `waggle pace` render for Claude Code, Copilot and Gas Town. Phase B (Tier 3) adds a load and memory refusal to `waggle dispatch` before the lock. Phase C (parallel runs) is deferred to a design card. Merging the brief has no product effect. The blast radius comes later, when phase B dispatches: it changes the refusal order in `internal/dispatch/run.go`. The research table holds up. All 13 Claude Code rows, the Copilot rows, the Gas Town rows and the OS rows I re-fetched match their cited pages, and no env var name, default or version is wrong. Four spec errors in the design and records still need fixing before dispatch. Each is a small text change.

**Blocking Issues (Must-Fix Only)**

1. **must-fix: a `pace:` block can fail `config.Parse` and with it every gate run (T-043.md:48, :180, :204).**
   - 2.1 says "a typo in `pace:` can never fail a merge". The threat model lists as must-fix "a `pace:` section, valid or not, that ... makes `config.Parse` fail".
   - At a450280, `config.Parse` (`internal/config/config.go:118-119`) runs `parseTree` over the whole file before it picks out its four sections. `parseTree` (`yaml_subset.go:51-128`) rejects these anywhere in the file: a bare `pace:` with no children ("key has no value and no children"), a duplicate key, a `- ` sequence, tab indentation, a flow construct.
   - The gate parses the base `config.yaml` with `config.Parse` (`cmd/waggle/gate_verify.go:287-293`). So a merged `pace:` block with a syntax slip fails every later gate run, and `doctor` exits 3 at `doctor.go:52` before it can print `pace: invalid`.
   - Every builder fails that threat-model line, and the claim is false as written.
   - Fix: limit the guarantee to "any `pace:` content the subset parser accepts (semantic errors: unknown key, bad word, range, order)". State that syntax errors fail `config.Parse` the same way they do for `context:` and `runtimes:` today, which owner approval of the Tier 3 change covers. Change the threat-model line and the 2.6 doctor text to match.

2. **must-fix: the darwin load decoder spec does not match what `syscall.Sysctl` returns (T-043.md:85, :207).**
   - The brief calls for `syscall.Sysctl("vm.loadavg")` decoded as a "fixed 24-byte `struct loadavg`" (AC5).
   - `syscall.Sysctl` (Go `src/syscall/syscall_bsd.go:456-458`) drops the last byte when it is NUL. `struct loadavg` is `{ fixpt_t ldavg[3]; long fscale; }` (SDK `sys/sysctl.h:550-553`), and `fscale` is 2048 little-endian, so the last byte is always 0.
   - On this Mac, `sysctl -b vm.loadavg` gives 24 bytes and a Go probe of `syscall.Sysctl("vm.loadavg")` gives `23 <nil>`.
   - A decoder that needs 24 bytes passes its fixed-bytes test and then reads `unknown` on every Mac. Doctor would print unknown, and phase B would never refuse on the keepers' main machines (Q-T043-6 says they are Macs).
   - The layout itself is not cited anywhere: row O3 says the sysctl(3) mirror "does not define its fields or sizes", and the builder has no network.
   - Fix:
     - State the layout with a source the builder can read offline, or quote it in section 4: three `uint32` values, 4 bytes of padding, an `int64` `fscale`, little-endian on amd64 and arm64, load1 = `ldavg[0] / fscale`.
     - Require the decoder to accept 23 or 24 bytes, padding a missing trailing zero.
     - Have AC5 test both lengths.

3. **must-fix: the phase B refusal is placed after a write, which contradicts the brief's own threat model and AC11 (T-043.md:113, :180, :216).**
   - 2.8 puts the load step "after `worktreesDir` and before `takeLock`".
   - `worktreesDir` (`internal/dispatch/worktree.go:169-182`) runs `os.MkdirAll(.waggle/worktrees, 0o700)`, which is a write.
   - The threat model makes "the check placed after `takeLock` or any write" a must-fix. AC11 wants nothing created on refusal, but `.waggle/worktrees/` would exist after a refused run in a fresh repo.
   - The check does not need `dir`. Fix: place it before `worktreesDir` (after `resolveIdentity`, or in `seq` ahead of it), and update the lease-rule wording at T-043.md:12.

4. **must-fix: question IDs collide with ones already recorded (T-043.md:4, :253-261, and inline references such as :45, :59, :67, :80, :113).**
   - `DECISIONS.md` D-321 to D-324 already record Q-T043-1 to Q-T043-4, as the agent pace enforcement, authority, knobs and schedule questions.
   - The brief reuses Q-T043-1 to Q-T043-7 for new questions: profile names, phase split, CPU target, clamp, render, macOS memory and absent section. The status line also calls Q-T043-1 to Q-T043-7 "open".
   - Ruling these would put a second, different Q-T043-1 to Q-T043-4 into the append-only record.
   - Fix: renumber them Q-T043-5 to Q-T043-11 everywhere in the brief.

5. **must-fix: dispatch clamps the keeper's pick silently, against the recommended Q-T043-4 (a) (T-043.md:113, :217, :258).**
   - The recommendation promises the clamp and unknown-word warnings "in status, doctor, `pace` and dispatch".
   - 2.8 gives dispatch only `pace_unmeasured` and `pace_serial`, and AC12 tests no clamp warning. A builder following 2.8 ships a dispatch that quietly runs a slower profile than the keeper picked.
   - Fix: add a `pace_clamped` or `pace_unknown_pick` entry to `Result.Warnings` (passed through the control-character filter) and add it to AC12.

**Notes (not blocking)**

N1. Research table: every quote in C1 to C13, G1, G7 to G11, T4, T5, O1, O3, O4 and O5 was found on the cited page. Statuses are right; C10, C11, C12, G8, T5, O5 and O6 are correctly partial.
   - C3 leaves out that agent teams are "experimental and disabled by default". The serial `"0"` is harmless, but it matters only when some lower settings source enables teams, and the render text could say so.
   - C8's page says "Linux and WSL", while the render says "Linux".
   - T5: the scheduler page does mention CPU and memory, as things it aims not to exhaust, but its capacity formula counts only active polecats. "Measures no load" is accurate. "The page measures no CPU, memory" would read better as "uses no CPU or memory reading".

N2. A few code cites are off by a little:
   - `release.go:67` is a doc comment. The `lockRepo` call is at `release.go:95`, and release takes no lock when `.waggle/worktrees/` is absent.
   - `signalGroup` is at `cell_unix.go:18`, not `:24`.
   - The renewal closure is `run.go:402-414`.
   - All other cites check out at a450280: `run.go:66`, `:189`, `:221`, `:224-236`, `:594`; `config.go:118`, `:332`; `context.go:13`, `:30`; `local.go:30-65`; `layout.go:22`, `:32`, `:67`; `hostrecord.go:36`, `:106`; `check.go:89`; `prerun.go:115`, `:136`; `clean.go:64`, `:76`; `worktree.go:153-157`, `:475`, `:524`; `launch.go:73-74`, `:305`, `:377`; `status.go:105-107`, `:158`; `doctor.go:56-58`, `:76-80`; `plain_terms_test.go:16`, `:39`, `:62`, `:81`; `status_test.go:136`, `:255`, `:286`; `doctor_test.go:89`, `:173`, `:200`; `gate_verify_test.go:67`; `dispatch_test.go:41`; `context_test.go:32`; `upgrade_test.go:252`; `reap_darwin.go:31`.
   - The host-file analysis is right: `hostFiles` covers entry names and each peer's `HEAD`, per `check.go:85-88`. So is the claim that today's single `dispatch.lock` already enforces serial. The D-193 and D-205 mapping in 3.2 is sound.

N3. Package direction is unstated. `ParsePace`'s ordering check needs the default profile table, and `pace.Resolve` takes the owner `Pace`. If `Pace` lives in `internal/config` and the table in `internal/pace`, the two packages import each other. Follow the `context:` precedent (config imports runtime): say the `Pace` type and the table live in `internal/pace`, and `config.ParsePace` returns it.

N4. `runtime.NumCPU` counts CPUs usable by the process (affinity or cgroup), while load average is system-wide. On a limited process the percent overstates load. That is fine for a coarse check, but worth one line in 2.5.

N5. Q-T043-5 option (c) contradicts D-322, as the brief itself says, so it is not a live choice. Q-T043-3 (c) calls the macOS host call "undocumented" without a source. Both are minor.

N6. `memory_floor_mib: 0` renders `CLAUDE_CODE_WARN_MEMORY_FREE_GIB=0`, which turns the warning off. Say that, or skip the export when the floor is 0.

N7. These all check out:
   - Lease: phase A uses 4 directories and phase B uses 2, within D-319's limit of 5.
   - The Bash forms match WAGGLE.md section 3, and nothing needs the network.
   - Budget follows D-185: phase A is 462 bottom-up, doubled to 920, target 1,200, stop 1,660, with a planned cut. Phase B is 43, doubled to 90, target 120, stop 160, with a cut.
   - The threat model has the D-203 guard sentence.
   - Phase A touches no gate file, and with no `pace:` it adds no status or doctor output. The only visible change is one `pace` line in the usage text.
   - The shape matches T-042 and T-037.

Read-only run: nothing written, committed or pushed, and no branch switched. The one scratch probe directory was removed. Files: `.waggle/hive/tasks/T-043.md` (at 633029d), `internal/config/yaml_subset.go`, `cmd/waggle/gate_verify.go`, `internal/dispatch/worktree.go`, `internal/dispatch/run.go`.

Verdict: must-fix

Risk-Tier: 3

**Intent Summary:** Round 2 (8d53204) edits only `.waggle/hive/tasks/T-043.md`. It answers the six round 1 must-fix items from the opus and fable guards. The changes:
- The `pace:` "never fails a merge" guarantee now covers semantic errors only, and YAML syntax errors are called out as failing `config.Parse` and the gate, as any broken section does today.
- The darwin decoder accepts 23 or 24 bytes and pads its input.
- The phase B load step moves before `worktreesDir`.
- The open questions are renumbered Q-T043-5 to Q-T043-11.
- The `pace_clamped` and `pace_unknown_pick` warnings now reach `Result.Warnings`.
- The phase B lease rule and AC14 now name `Deps.Load` and the two refusal strings.
- The notes are folded in, and the phase B budget is now 49 bottom-up, with estimate 100, target 130 and stop 180.

Blast radius is the brief only: no code, gate or hive behavior changes. All six round 1 items are resolved correctly. The round leaves one internal contradiction in the phase B lease rule.

**Checks run against a450280**
- **Item 1 (syntax vs semantic errors):** confirmed. `config.Parse` calls `parseTree` at `config.go:119`. Parse looks up only named top-level keys, so an unknown top-level `pace:` is ignored. `parseTree` rejects tabs, bad indentation and duplicate keys file-wide. The gate parses the base config at `gate_verify.go:291`. Section 2.1, section 5 (must-fix and note lists) and AC2 are now scoped consistently.
- **Item 2 (darwin 23 bytes):** confirmed on this darwin host. A Go program printed `len(syscall.Sysctl("vm.loadavg")) == 23`; padded to 24 it decodes `ldavg[0]=5410`, `fscale=2048`, load 2.64, which matches `sysctl -n vm.loadavg`. `reap_darwin.go:31` is the Sysctl call and `:35-36` is the padding. AC5 tests both lengths.
- **Item 3 (step before the first write):** confirmed. `worktreesDir` runs `os.MkdirAll` at `worktree.go:175` (func 169-182). The first `seq` is at `run.go:224-229`, and `resolveIdentity` comes right before `worktreesDir`. Nothing before it in `Run` or `cmd/waggle/dispatch.go` writes. AC11 checks that `.waggle/worktrees/` does not exist after a refusal.
- **Item 4 (renumbering):** confirmed. DECISIONS.md at a450280 holds exactly Q-T043-1 to Q-T043-4 (lines 6194 to 6254). Every in-text reference was remapped by +4 consistently.
- **Item 5 (warnings):** done in 2.8 and AC12, and the threat model must-fix list includes it.
- **Item 6 (lease rule and AC14):** done. `Deps` is `run.go:28-39` and `load` is `run.go:66`.
- **Folded notes:**
  - The package direction mirrors `context.go:7`.
  - The ordering direction matches the 2.4 table and the clamp.
  - Status keeps exit 0 on a semantic error, and the refusal on a syntax error at `status.go:79-82` is exit 1.
  - The doctor cites are right: `doctor.go:52-53` (exit 3), `:56-58` and `:76-80`.
  - The NumCPU reasoning errs toward refusing, which is correct.
  - A zero floor prints no export and is consistent with C8.
  - All cites are correct: `release.go:95`, `release.go:67-69` (doc comment), `cell_unix.go:18`, `run.go:221`, `run.go:232-236`, `run.go:382-386` (where `res.Warnings` is set at 386), `run.go:402-414` (renew closure), `status.go:105-107`, `status.go:158`, `hostrecord.go:36`, `hostrecord.go:106` and `config.go:332`.
- **Section 4 rows re-fetched today:**
  - C8: the env-vars row says "The reminder is off when the variable is unset, `0`, or not plain digits" and "ignores a copy delivered through a settings `env` block". The settings reference lists `CLAUDE_CODE_WARN_DISK_FREE_GIB` and `CLAUDE_CODE_WARN_MEMORY_FREE_GIB` as "read from the launch environment only, are ignored from every file", v2.1.304.
  - C10: the shell-vs-env sentence matches. The "Variables Claude Code ignores in `env`" section names none of C1, C3, C5 or C6.
  - C6: "from `1` to `256`" and v2.1.269 match.
  - C3: all five quotes match, including "To make named subagents launch as subagents again ... set `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` to `0`".
- **Budget math:** checks out. Phase B: 15+8+6+12+2+6 = 49, doubled to 100, then 130 and 180. Phase A: 462, doubled to 920, then 1,200 and 1,660.
- No em or en dashes in the brief.

**Blocking Issues (Must-Fix Only)**

1. **must-fix**, `.waggle/hive/tasks/T-043.md:12` (phase B lease rule) contradicts `:113` (2.8) and `:219` (AC14). The lease rule says `run.go` may change only four things:
   - the `Deps.Load` field;
   - the load step in the first `seq`;
   - `load`;
   - the two refusal strings.

   But 2.8 also requires run.go changes outside that list:
   - carrying the reading and the Resolve warnings from the step to the `Result` literal;
   - joining `pace_unmeasured`, `pace_serial`, `pace_clamped` and `pace_unknown_pick` into `res.Warnings` at `run.go:382-386`;
   - filling the new `pace`, `load_percent` and `available_mib` fields there.

   This round's AC14 allows "the pace warnings where `Result` is built", so the lease rule and AC14 now disagree. A builder following the lease rule cannot deliver item 5 without leaving it, and the phase B guards would have opposite instructions. The fix is one clause in the lease rule, for example: "the pace warnings and the three `Result` pace fields where `Result` is built, `run.go:382-386`". AC14 could also name the field assignments explicitly.

**Notes (not blocking)**

2. **note**, `T-043.md:5`: "The gate never reads the new `pace:` section" is looser than 2.1's corrected "does not read its meaning". The gate does tokenize it through `parseTree`. Suggest the same wording as 2.1.
3. **note**, `T-043.md:153` (C10) and `:105` (2.7 render text): the settings reference adds caveats that could go into the render text if wanted:
   - "How `env` values interact with your shell" says that when the Claude Desktop app or a self-hosted runner starts the session, "Claude Code ignores an `env` value from any settings file for a variable the launch environment already sets".
   - "When Claude Code applies `env` values" says project and local values apply "after you trust the workspace".

   Neither changes the C10 verdict that C1, C3, C5 and C6 are not on the ignored list.
4. **note**, `T-043.md:113`: "Refuse, not wait (Q-T043-6)" points to the phase split question, which does not discuss refuse vs wait. This is carried over from round 1's Q-T043-2 under the consistent renumbering, so it is not a regression.
5. **note**, `T-043.md:113`: `load` is cited as `run.go:66-84`, but the function ends at line 81 (82 to 84 are the `refuseDirs` comment). This is trivial and the cites are rechecked at dispatch anyway.
6. **note**, environment: the main checkout now reports branch `main`, not `t-043-brief` as at session start. I ran only `git show` and `git diff` there, so this was not my doing. My scratch files under <scratchpad> are removed, and no worktree was created.

Verdict: must-fix

Risk-Tier: 3

**Intent Summary:** Round 4 (2de1a77, one commit, brief file only) replaces the line-range lease rule with the behavior-based scope the Head Keeper ruled on the D-204 design card. Each phase now has an allowed list and a forbidden list, and line cites are marked as orientation only. `pace.Resolve` returns `[]Warning` with a `Kind` (`clamped`, `unknown_pick`) and a `Message`. Dispatch maps `Kind` to `pace_clamped` and `pace_unknown_pick` and never matches text (2.3, 2.8, AC4, AC12, AC14). Blast radius: only `.waggle/hive/tasks/T-043.md` changes. No code changes. I checked every cite I used against a450280: run.go:28-39 (Deps), 58-64 (loaded), 66-81 (load), 221, 224-229 (first seq), 382-386 (Result); status.go:79, 105-107, 158; doctor.go:52-58. All match. There are no em or en dashes in the brief.

**Findings**

1. Note (question 1, walk-through). Every design step fits an allowed item, and I found no step that needs a forbidden one.
   - **Phase A:** `ParsePace` and `LoadPacePick` are new code in `internal/config/`. They use `parseTree`, `invalid`, `sortedKeys`, `intValue` and `readCapped` without changing them. `readCapped` wraps the open error with `%w`, so the missing-file case can be detected with `errors.Is` and `readCapped` needs no change.
   - **Phase A, status:** status needs a new `header` field (status.go:36), a print inside `renderStatus`, and reading code in `runStatus`.
   - **Phase A, doctor:** doctor prints between doctor.go:52 and :56.
   - **Phase A, `waggle pace`:** this is a `case` in main.go plus usage text.
   - **Phase B, config:** `load` gains seq steps after `ParseRuntimes` (it already has `data`) and fields on `loaded`. `load` has one caller (run.go:202), so the new refusal reaches no other command.
   - **Phase B, load step:** it inserts into the first seq between run.go:225 and :226 without editing the `worktreesDir` line.
   - **Phase B, Result:** the warnings join at or after `slices.Concat` (run.go:385).
   - **Phase B, ledger:** `appendLedger` (ledger.go:21) copies named fields, so the new Result fields do not reach the usage ledger.

2. Note (question 2, precedence), lease rule, T-043.md line 12. The rule does not say which list wins when a hunk fits both. Three likely hunks touch this:
   - The new seq step adds a refusal ahead of the existing `worktreesDir` refusal, and the forbidden list names "the existing refusal order".
   - The natural pace-warning edit changes the `slices.Concat(lockWarns, w.Warnings, claim.Warnings, guardWarns)` line, which names lock and claim values.
   - The `internal/pace` import line and the `Deps.Load` nil-default code in `Run` (near `now := time.Now`) are not named items.

   I read all three as allowed, because "other" in "any other existing behavior" excludes the allowed items. A strict guard could split on them, though. A one-line fix would settle it:
   - "the relative order of existing refusals";
   - "a hunk that fits an allowed item is not caught by the catch-all; a hunk that changes what a named forbidden item does is a must-fix even if it fits an allowed item";
   - "imports and locals needed by an allowed item are allowed."

3. Note (question 2), phase A allowed (a), line 12. The parenthetical "(types, profile table, `Resolve`, load parsers and readers, per-tool render)" can read as a closed list. It leaves out the control-character filter and any shared formatter for the status line, the doctor finding and the `waggle pace` line, if the builder puts that formatter in `internal/pace`. Suggest "for example" or naming both.

4. Note (question 4), phase A forbidden "any rendered template value", line 12. Allowed (d), AC9 and section 6 all expect the rendered `config.yaml` and `local.yaml` to gain commented lines, and section 6 expects any golden of them to change. "Value" most likely means parsed values, so this is not a conflict. Rewording it as "any parsed value of a rendered template" would close the gap. Nothing else on the phase A forbidden list blocks a needed change:
   - status and doctor add lines only;
   - `config.Load` and `config.Parse` stay untouched, since pace is read from separate bytes as `LoadContext` does;
   - the `header` field keeps `renderStatus`'s signature.

5. Note (question 1, D-205 areas), phase B forbidden list, line 12. D-205 names deleting files or git state as a two-guard trigger. Several things in that area are covered only by "any other existing behavior":
   - `ResolveBase`'s fetch;
   - the push (run.go:~462);
   - `newScratch` and `dropScratch`;
   - the `.tainted` marker write;
   - `refuseDirs`, `checkIgnored` and `ownGroup`.

   They are covered, but naming "any file or git state deletion, fetch or push, and the scratch directory" would match D-205 directly. The Risk Tier line (line 4) also says only "a fix round that touches the lock, claims or the host-file check gets two". D-205 also lists signals, deletes and leases.

6. Note (question 5), AC14 versus the lease rule, line 220 versus line 12. AC14's copy of the forbidden list leaves out "the tracker and `runSession`" and "the usage ledger", which the lease rule names. The catch-all covers them, but the two lists should match. Simplest fix: AC14 says "the lease rule's phase B lists" and does not restate them.

7. Note (question 5), push tripwire, line 10. It says phase B tests "use the `dispatchRepo` fixture (`cmd/waggle/dispatch_test.go:41`) or the `internal/dispatch` fixtures". `cmd/waggle/` is outside the phase B lease, and line 12 says "A file outside the phase's lease is a must-fix". The "or" leaves an in-lease route, but the cmd/waggle option should be dropped for phase B. The Layer line (line 3) also gives phase B as `internal/dispatch/` only, while the lease includes `internal/pace/` for fixes.

8. Note (question 3, typed warnings). 2.3, 2.6 (prints the Message), 2.8, AC4, AC12 and AC14 are consistent and buildable. Nothing in the brief still refers to a `[]string` return; the only `[]string` left is the existing `Result.Warnings`, which is correct. One gap: 2.8 and AC12 do not say what the Kind-to-code mapping does with a `Kind` it does not know. A future kind would then be dropped silently, against "dispatch never clamps silently". Suggest a fallback code such as `pace_warning` and a test row for it.

9. Note (status line, line 4). The D-204 ruling is "to be recorded as a decision". The queen should record it and cite its D-number in the lease rule before phase A dispatch, so guards can cite it.

No must-fix found. The brief gives every edit 2.1 to 2.8 need a place on an allowed list, and the forbidden lists keep the lock, claims, keepalive, host-file check, signals, sweep and worktree lifecycle out of scope. That matches the section 5 threat model and the guard prompt sentence under D-203.

Files: .waggle/hive/tasks/T-043.md (at 2de1a77), internal/dispatch/run.go, internal/dispatch/ledger.go, cmd/waggle/status.go, cmd/waggle/doctor.go, internal/config/local.go, internal/config/roster.go (all read at a450280). Scratch copy <scratchpad> was written there; the caller can remove it.

Verdict: pass

Confirming brief guard (opus), T-043 brief finalize round, commit `021b1b5`, diff from the merged round 5 at `1425404`, code checked at `1425404`. Read-only: nothing in the repo written, committed, pushed or switched.

Risk-Tier: 3

**Intent Summary.** The finalize round touches one file, `.waggle/hive/tasks/T-043.md` (44 insertions, 43 deletions). It moves every code cite from `a450280` to `1425404` (absorbing the T-025 phase C1 two-line SPDX header and the T-044 and T-045 growth of `internal/dispatch/run.go`), folds the five fable round 5 notes, applies T-025 (AC10 names the exact header form, `TestRepoHeaders` is cited, the `internal/license` table needs no new row for `internal/pace/`, a new AC15 for phase B, `git commit -s` for the required dco check per D-339, the Head Keeper WAGGLE.md section 2 commit is signed off), names the T-044 and T-045 code each phase must leave alone in the forbidden lists, adds 16 header lines to the phase A budget, and updates Depends on. Blast radius: a hive brief; no code, no control file, no gate behavior.

**What I verified.**

- Cites. I extracted all 88 backticked Go cites from round 5 and all 89 from the finalize round, aligned them (the one extra is the new `internal/license/repo_test.go:33`), resolved short names (`claim.go` and `ledger.go` checked by hand as the `internal/dispatch/` files) and compared the cited text at `a450280` with the new cite at `1425404`. Every pair points at the same code. Every shift is +2 except `run.go:382-386` to `392-396` and `run.go:594-` to `604-` (+10, the T-044 and T-045 growth; the only text change inside the `Result` block is the added `KeeperCommit: req.KeeperCommit` field) and three ranges whose end moved from a trailing blank line to the closing brace (`run.go:191-500` still ends at the same `}` before `d.emit("recorded", ...)`, `prerun.go:107-144`, `launch.go:256-401`). `repo_test.go:33` is `func TestRepoHeaders`. No non-Go file:line cites; `a450280` survives only in the round 1 history sentence.
- Fable round 5 notes, all five folded: (1) phase B allowed (d) now says "five pace warning codes of 2.8, the `pace_warning` fallback included" (2.8 lists `pace_unmeasured`, `pace_serial`, `pace_clamped`, `pace_unknown_pick`, `pace_warning`, five); (2) the refusal-order parenthetical now reads "inserted after `resolveIdentity`, ahead of `worktreesDir` and every later refusal"; (3) AC12 grammar fixed; (4) Read-only context lists D-321 to D-332 plus D-337 and D-339; (5) the round 4 sentence cites D-325. `DECISIONS.md` at `1425404` has D-325 to D-332 under the numbers the brief uses, so the forward-reference risk fable raised did not materialize.
- T-025. `internal/license/rules.go` at `1425404`: `internal/scaffold/templates/**` is MIT-0 and comes before `cmd/**`, `internal/**` (Apache-2.0), so a new `internal/pace/*.go` matches the Apache-2.0 row with no table change. `Check` wants `// SPDX-License-Identifier: Apache-2.0` on line 1 and an empty line 2 for Go files; existing files with a build constraint (`cmd/waggle/claim_test.go` and others) put `//go:build` on line 3, matching AC10. Both templates start with `# SPDX-License-Identifier: MIT-0`. `TestRepoHeaders` walks `git ls-files`, so it sees tracked files only, as stated. The `dco` job in `.github/workflows/ci.yml` checks each PR commit for a `Signed-off-by` naming its author email, matching D-339; the worker allowlist (`launch.go:82` `gitAllowed`) permits `git commit`, dispatch's own prompt already says `git commit -s` (`inputs.go:166`), and the worker env sets author and committer to the same identity (`launch.go:391-392`), so `-s` satisfies the job. The WAGGLE.md section 2 precondition (section 12 item 1) is unchanged except for "signed off, D-339"; WAGGLE.md section 2 still has no `internal/pace/` line, so the precondition remains real.
- Lease rules. Phase A forbidden gains `waggle dispatch` and `waggle claim` and their flags (`cmd/waggle/dispatch.go`, `cmd/waggle/claim.go`, both present); no phase A allowed item (status, doctor, `waggle pace`, `main.go` usage) needs either file. Phase B forbidden gains the T-044 claim branch append, `guardModel`, `checkSecond`, `guardTarget` (all in `guard.go`, all reached after `worktreesDir` in `Run`, so the pace step stays ahead of them without reordering anything) and `Request` and `request.go`; 2.8 adds no `Request` field or flag. Each named item was existing behavior already caught by "any other existing behavior", and none is something an allowed item or a 2.x step needs, so nothing is narrowed or widened. The first `seq` at `run.go:226-231` is unchanged apart from the shift, so allowed (b) still fits.
- Budget. Phase A: 215 + 125 + 108 + 14 + 16 = 478; doubled 956, rounded 960; target 1.3 x 960 = 1,248, rounded 1,250; stop 1.8 x 960 = 1,728, rounded 1,730. Same nearest-ten rounding as rounds 1 to 5 (924 to 920). Planned cut: 55 + 60 + 4 header lines = 119, about 120, 240 doubled. Phase B: 15 + 8 + 6 + 12 + 2 + 6 = 49; 100, 130, 180, unchanged. Doubling, target, stop, planned cut and "only crossing a stop goes to the Head Keeper" follow WAGGLE.md section 4 (D-185, D-210).
- Rulings. Section 13 and the D-321 to D-332 text are untouched by the diff; the only D-32x edits are the D-325 citation and the Read-only context list. Depends on matches D-324 (build after T-025 phase C and the T-041 finalize round); T-041's brief at `1425404` stops at its round 3 rulings (D-310 to D-318), so the T-041 hold stands as written.
- Threat model (D-203): the only diff hunks in sections 5 and 6 are cite shifts; wording unchanged.
- Scope claim: holds. The round changes cites, notes, header and sign-off requirements that CI already enforces, named examples inside the existing catch-all, and budget arithmetic for the header lines. No allowed item is added or removed and no 2.x design step changes, so one confirming guard is right under D-205.
- No em or en dash in the file; no home path, personal name or email; the Keeper is named by role.

**Findings.**

1. Note, Bash forms line: "`TestRepoHeaders` ... passes locally on an untracked file and fails on product-ci once the file is committed" is slightly incomplete: after the builder commits in its worktree, a local `go test -race ./...` also sees the file and fails. The instruction (header before the first commit) is correct either way; the wording could say "fails on the next test run once the file is committed".

2. Note, section 10: the phase A planned cut figure (120 bottom-up, 240 doubled) now includes header lines, which matches how the 478 total counts them. No change needed; flagged only so the builder's numstat comparison counts headers the same way.

No must-fix. Every cite is right at `1425404` and points at the same code as before, the fable round 5 notes are folded, the T-025 additions match `internal/license` and the dco job, the lease lists neither widen nor narrow scope, the budget arithmetic is right, and the rulings stand.

Verdict: pass
