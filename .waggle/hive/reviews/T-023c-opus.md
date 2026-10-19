<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: T-023 phase C (PR #50, `t-023c` at `05343b4`, base `23bdf75`), first guard (opus)

Verdict: must-fix
Risk-Tier: 3

## Intent Summary

Phase C makes `waggle dispatch <ticket>` launch a real headless Claude Code worker for the first time. `dispatch.Run` validates, refuses Tier 3 and `.waggle/` directories before any fetch, verifies the adapter once, takes a repo-wide `dispatch.lock`, resolves the base (refusing a base that sits on the branch), reads inputs from base, claims through the Comb, provisions or reuses the worktree (including from a remote-tracking branch), snapshots host state (refs, main checkout status and HEAD, hashes of common `config`, `hooks/`, `info/`, alternates, shallow, the worktree admin entries and the global git config), launches the worker in its own process group through the new `CellRuntime` seam, renews the claim while it runs, then runs the 7.2 checks (files first, then git under a pinned env) and pushes only on a clean pass. It writes `result.json`, taints the worktree on a host-state violation, and adds the CLI with the section 4 exit codes. Blast radius: every future agent launch in every hive; the code holds the Keeper's full environment and git push auth on the host side. 1186 added non-test lines (D-169 accepts this; stop 1350 including fix rounds). The design is sound and most carry items are met. Three demonstrated holes remain: the push sends whatever the branch names at push time rather than the commit that was checked; dispatch's own Comb git calls during the session and before the files step run hooks and config the agent planted, with the Keeper's credentials in the environment; and the default global excludes file hides main checkout and worktree writes from both status checks. The fixes total about 30 added lines.

## Blocking Issues

Ranked by severity. Estimated total: about 30 added non-test lines, well inside the 164 left under the 1350 stop. If they had to be cut, I would drop the keepalive half of B2 (B2c) first, then B4; B1, B2a, B2b and B3 are the floor.

### B1. The push sends the branch by name, not the checked commit (AC5) (about 6 lines)

`run.go:302` pushes `refs/heads/<branch>:refs/heads/<branch>`, and the checks read the branch by name: `log` on `<checkBase>..refs/heads/<branch>` (`check.go:218`), then a separate `rev-parse` for `head_oid` (`run.go:291`). A process that left the worker's group (`setsid`, which any `go test` the worker runs can do; P3 shows it survives and acts after `Run` returns) moves the branch between the check and the push. Probe P6 does this: it waits for dispatch's own worktree `status` call to rewrite the admin index, then sleeps 5 to 15 ms and points `refs/heads/t-023` at a commit that adds `docs/evil.md` outside the lease. In 2 of 6 runs, the result was exit 0, `pushed: true`, zero violations, and the bare remote held `docs/evil.md`. In one of those runs `head_oid` in `result.json` also differs from what was pushed.

Fix: resolve `refs/heads/<branch>` to an OID once, at the start of the git step. Use that OID for the `log` range, `head_oid` and the push refspec: `git push <remote> <oid>:refs/heads/<branch>`, still without `+`. Test: a fake agent whose escaped child moves the branch after the check; the remote must hold the checked OID or nothing.

### B2. Dispatch's own Comb git calls run planted hooks and config with the Keeper's credentials (7.2 order, AC12, section 8) (about 17 lines)

The comb backend that `Run` builds runs plain `git` under the repo's hooks and config, with the full Keeper environment (`GITHUB_TOKEN`, `GH_TOKEN`, `SSH_AUTH_SOCK`, `OP_SERVICE_ACCOUNT_TOKEN`: exactly the variables the adapter keeps from the worker). It runs at three points after the agent can write:

- **a. Final claim sync, remote mode** (`run.go:284`). This runs before `ck.run`, so before the files step. Probe P2: the worker runs `git config remote.origin.uploadpack <script>`. The final sync's `git fetch` executes that script on the host, and the script sees `GITHUB_TOKEN`. The run then fails with `host_state config`, but only after the planted code has run. This breaks 7.2 ("files first ... git second, under pinned config") and AC12 ("no host git call after the session runs a filter or hook the snapshot did not verify"). The builder's interpretation (15) says the fetch also prunes planted admin entries before they are inspected, which is the same ordering problem.
- **b. Keepalive renewals, local mode** (`run.go:269`, every `TTL/2`). A same-Keeper renewal runs `git update-ref` on the claim ref, which runs `hooks/reference-transaction`. Probe P1: the worker plants that hook; the next renewal runs it on the host and it sees `GITHUB_TOKEN`. This is local mode, the mode our hive runs today.
- **c. Keepalive renewals, remote mode.** These run `fetch` and `push --atomic` under the repo config (uploadpack, receivepack, `core.sshCommand`, `credential.helper`, an `ext::` url).

Fix:
- (a, b) Give the dispatch-built backend `opts.ExtraEnv = GIT_CONFIG_COUNT=1, GIT_CONFIG_KEY_0=core.hooksPath, GIT_CONFIG_VALUE_0=/dev/null`. That closes b entirely; `update-ref` runs nothing else.
- (a) Run the files comparison before the final sync. Skip the sync when any host file changed (the run already fails). Also compare claim refs strictly, with no remote-mode exemption, when the sync was skipped or failed (`run.go:285` today only warns and keeps the exemption).
- (c) Before each renewal, compare `hostFiles` to the snapshot; on a change, skip the renewal with a warning. Add one `not_enforced` line naming the residual race (a config written and reverted between the hash and the renewal's git call).
- Test: P1 and P2 as regression tests, with the canary never firing.

### B3. The default global excludes file hides writes from both status checks (rules 3 and 4) (about 4 lines)

Pinned git sets `GIT_CONFIG_GLOBAL=/dev/null`, but git still reads `$XDG_CONFIG_HOME/git/ignore` (and `.../git/attributes`) when `core.excludesFile` (`core.attributesFile`) is unset. `hostFiles` hashes only `$XDG_CONFIG_HOME/git/config` and `~/.gitconfig`. Probe P7: the worker writes `*` to that ignore file, drops an untracked `CLAUDE.local.md` into the main checkout and leaves an uncommitted file in the worktree. The result was exit 0, pushed, zero violations. This is test 12's "write into the main checkout" and rule 3, defeated by one write to a user file.

Fix: add `$XDG_CONFIG_HOME/git/ignore` and `$XDG_CONFIG_HOME/git/attributes` (with the `~/.config` fallback) to the global list in `hostFiles`. Also add two `not_enforced` lines:
- "writes to gitignored paths in the main checkout are not detected": `.claude/settings.local.json` and `.waggle/control/local.yaml` are ignored in our hive, and the first one gives the queen's next session hooks.
- "a process that leaves the worker's process group (setsid, a daemon) is not killed and can change host state after the checks". P3 shows this.

Test: P7 as a regression test, giving `host_state`.

### B4. Literal en and em dash characters in a test (0 added lines)

`cmd/waggle/dispatch_test.go` passes the two literal dash characters (U+2013, U+2014) to `strings.ContainsAny`. Write them as Go escapes (backslash u2013 and backslash u2014), so the repository holds no U+2013 or U+2014 anywhere.

## Rulings on the builder's interpretations

- **(3) Budget stop** as `end: "max_budget"`, exit 5, with the raw subtype in `result_subtype`: **accepted.** It needs the Keeper, like `max_turns`. `max_budget` is not in the 9.1 `end` list or the T-022 ledger mapping, so phase D must map it (to `error`, or add it to `Record.End`), and the scout should add it to 9.1.
- **(4) Limit stop with only a dirty tree stays exit 5; any other violation is exit 1:** **accepted.** A cut-short session is expected to leave uncommitted work. Every serious violation still blocks the push and wins.
- **(5) Checks use the in-memory lease; the own claim ref is excluded from rule 4 and not otherwise verified:** **accepted for phase C.** It matches the rule 4 text, and the in-memory lease means a rewritten claim cannot widen this run's check. The residual: in local mode the agent can rewrite its own claim after the last renewal (another keeper, which makes the next round exit 4; or wider directories, which a local `gate verify` would then read). Phase D should record the claim OID after each renewal and require the ref to still equal it (N3).
- **(6) `dispatch.lock` from before the fetch until `Run` returns:** **accepted.** It is wider than "snapshot to end of checks" and also serializes claim writes. The agent can delete it (same user); see N6.
- **(7) `refuseDirs` with `HardcodedTier3Prefixes` and `DirsOverlap`:** **accepted.** It is conservative (case-folded) and the Comb stays the authority. It does not catch the `.gitlab-ci.yml` prefix-match form (`.gitlab-ci.ymlx`), but the Comb refuses that at the claim, which is fine.
- **(8) Shell-script fake `claude`, unix-tagged tests:** **accepted.** This departs from the helper-process wording in section 11, but the scripts make the scripted git actions readable, and test 27 keeps the Windows build honest.
- **(9) No per-turn `usage` event:** **accepted as deferred.** 9.2 lists the event, but `Collect` does not expose turns. Carry it to phase D (ledger) or E (stream input), and note it in the brief.
- **(10) `Git` gains `Remote` and `Pinned`:** **accepted.**
- **(11) `waggleCommands` parser for WAGGLE.md section 3:** **accepted.** It reads base only (Tier 3 file), skips placeholders, and the adapter's `workerSettings` still refuses widening commands.
- **(12) A repo without `.waggle/worktrees/` in `.gitignore` fails rule 4:** **accepted as behavior,** but it costs a whole session before failing. A pre-launch `git check-ignore -q .waggle/worktrees/` giving exit 3 would cost about 3 lines (N5).
- **(14) The kill test assumes an init that reaps orphans:** **accepted.** It holds on ubuntu runners and macOS.
- **(15) "New admin entry" plants a valid gitdir because fetch prunes otherwise:** **accepted for the test,** but the reason is B2a: the final sync runs before the files step. After B2 the test can also plant an invalid entry.

## Carry items (verified)

- `not_enforced` names the tripwire limits: yes (`cell.go:43`, `05343b4`).
- `ErrUnverified` and `ErrRefusedArg` both exit 3: yes. Both are non-`*Error` errors and `ExitCode` maps them to 3. Tests `TestRunVerifyOnce` and `TestRunRefusedArgExitsLikeUnverified`. See N1 on where `ErrRefusedArg` surfaces.
- `error_max_budget_usd` named in `result.json`: yes (`result_subtype`, `TestRunBudgetStop`).
- One `Verify` before `Prepare` on a cloned `DefaultArgs`: yes (`run.go:180`, `run.go:185`; `Verify` clones again).
- Item 10: the five git env variables are in a separate list (`configVars`, plus the global, system and attr variables), `redirectVars` is unchanged, and test 28 parses `scrubbedGitEnv` with `go/parser`: yes (`TestRedirectVarsMatchComb`, `TestPinnedEnv`, `TestRunGitEnvDropped`).
- Test 27 is a Go test: yes (`TestCrossCompileWindows`, the `go` binary from `GOROOT`).
- Stale `worktree.go` comment fixed: yes (`NewRunDir`).
- Refusal-loop no-claim-ref assertion: yes for refused bases (`TestRunBaseAndBranch`), the held `dispatch.lock` (`TestRunDispatchLock`) and an unverified runtime. Not present for refused directories (`TestRunClaims` asserts zero fetches and no `.waggle/worktrees/`, which implies no claim).
- Stale `main.go` usage line: yes, plus the `case "dispatch":` line. Within the lease.
- 10.5.2 items 1 to 9: met. Admin dir snapshot and pin (test 20); explicit `--git-dir`/`--work-tree` and `Remove` re-pinning; base on branch every spelling (test 22); claim OID from the local ref with read-back as a warning; files-first order and tainted marker (except B2a); refusal before fetch, remote-tracking must-fix round, keeper id pattern; build tags and `cell_other.go`; `dispatch.lock` (test 24); sign-off rule and `dco`.

## Checks

- `go build ./...`, `go vet ./...`, gofmt on tracked files and `go test -race ./...`: pass locally (darwin).
- `product-ci` on PR #50: pass (ubuntu, 1m54s). `verify-leases-and-tiers` fails with "no claim for T-023". This is expected in a local-claims hive until phase F (D-119, D-168) and is not a phase C defect.
- Lease: every changed file is under `internal/dispatch/`, `internal/config/local.go`, `internal/config/local_test.go` or `cmd/waggle/`. `go.mod` is unchanged and only the stdlib is used.
- Size: 1186 added non-test lines (`git diff --numstat`), matching D-169.
- Sign-off: all three commits carry `Signed-off-by` with the subject prefix `T-023:`.
- Dashes: commit messages have none. The diff has one occurrence, a test literal (B4).
- Probes (not committed; source and output in `.waggle/hive/media/T-023c/opus-probes_test.go.txt` and `opus-probes-output.txt`):
  - P1, keepalive runs a planted `reference-transaction` hook with `GITHUB_TOKEN`: confirmed.
  - P2, final sync runs a planted `uploadpack` with `GITHUB_TOKEN`: confirmed.
  - P3, a `setsid` child survives the group kill and writes after `Run` returns: confirmed.
  - P4, an escaped child holding the stdout pipe keeps `Run` past its timeout (9 s against 2 s): confirmed.
  - P5, naive branch toggling is caught by the dirty check: 0 of 8.
  - P6, timed branch move after the check is pushed: 2 of 6.
  - P7, XDG ignore hides main checkout and worktree writes: confirmed, exit 0.

## Notes (non-blocking)

- **N1.** `ErrRefusedArg`, and a WAGGLE.md command that `workerSettings` refuses, surface from `Prepare`. That is after the claim, the worktree and the run dir exist, and `Run` returns an error with no `result.json`. `cell_other.go` refuses at the same late point on Windows. Moving `Prepare` (and a unix check) before `AcquireClaim` would keep refusals write-free. The worktree path is deterministic; the prompts can be built in memory.
- **N2.** `result.json` and `final.md` use `os.WriteFile`. That is not atomic: a crash leaves a partial file. It also follows a symlink and blocks on a FIFO the agent planted in the run dir, since the run dir is agent-writable in a host run. Write to a temp file with `O_EXCL|O_NOFOLLOW` and rename (about 5 lines).
- **N3.** Own claim ref in local mode: record the claim OID after each renewal and require the ref to equal it at check time (see ruling 5).
- **N4.** `Collect` reads until EOF, so a process holding the stdout pipe (P4) keeps `Run`, and `dispatch.lock`, alive past the timeout. A second Ctrl-C cannot end it. Closing the pipe's read end once the leader is reaped and `tctx` is done would bound it. With real Claude Code this needs a child to reopen the worker's stdout, so it is a liveness note, not an escape.
- **N5.** Pre-launch check that `.waggle/worktrees/` is gitignored (ruling 12).
- **N6.** The agent can delete `dispatch.lock`, the run lock or (if it escapes) the `.tainted` marker. The tainted marker is written after the checks, so an escaped process can also plant a hook after a clean pass. The next dispatch's pre-launch `fetch` then runs it unpinned. B3's `not_enforced` line covers the disclosure. A Cell is the real fix.
- **N7.** `TimedOut` requires `End == interrupted`. If Claude Code answers SIGTERM with an error result event, a timeout reads as `error`, exit 1, not `timeout`, exit 5. Key `TimedOut` on the deadline alone.
- **N8.** `tap` keeps only the first 64 KiB of a line. A result event with a final report over 64 KiB loses its subtype, so a budget stop with a long report would read as `error`. Parse the subtype from a bounded prefix, or have `Collect` expose it (phase D, with `Truncated`).
- **N9.** Verify against the real pinned Claude Code (AC7-style manual probe) that Bash tool children stay in the worker's process group. If Claude Code spawns them detached, the group kill at timeout misses a running `go test` tree. The local binary is 2.1.303.
- **N10.** `adminDir` in the files step runs `git rev-parse --git-common-dir`. It runs no hook or filter, but `ck.common` already holds the answer and would make the step git-free as 7.2 says.
- **N11.** An `include.path` or `includeIf` in the repo config that names a file outside the hashed set lets the agent change effective config without changing a hashed byte. It is rare, but worth one `not_enforced` clause, or a note in the 7.2 doc comment.
