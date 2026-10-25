# Brief review round 2 (confirming), T-040 at 58945fe, opus, against main 300fe7a

Verdict: must-fix

Risk-Tier: 3

**Intent summary.** T-040 rewords 36 user-facing strings in 17 files (15 Go, two templates) so a gate-only user (D-236) reads no bee name, and splits `printDecision` so a gate-only `waggle upgrade` stops printing T-016 and D-041. Text only: no exit code, JSON code, sentinel identity, gate result or rule changes. The blast radius is gate findings, claim and release errors, help text, the `waggle status` header, the upgrade and uninstall last lines, the decision input, and one step name in every installed gate workflow (the hive takes it at the next pin move, D-271). Round 1 items are closed. I re-checked every section 3 old string against main 300fe7a, and all 36 exist verbatim. Two new must-fixes come from reading the code paths the ACs exercise.

## Round 1 closure

- **Opus M1 (`waggle status` reachable in a gate-only repo): closed.** Rows 33 to 36 are added. The lease rule names `status.go` `:77`, `:164`, `:166`, `:168` only, and those match main exactly. Section 2 now excludes only the rest of the status body. Section 6 lists `status_test.go:122-123`, `:203`, `:230`, `:265-266`, which match main exactly and are the only status tests holding the old text. `config.Load` needs only `config.yaml` (`status.go:70-81`). A gate-only config loads, and the rest of `renderStatus` (`status.go:170-190`) holds no bee word. The usage footer (`internal/usage/render.go:13`) prints only with ledger lines, as section 2 says. AC10b has a fixture gap, see M2.
- **Opus M2 (two tests pinning row 6's `force-release`): closed.** `internal/dispatch/release_test.go:174` (in `TestReleaseClaimNoneAndMalformed`, `:168`) and `internal/dispatch/dispatch_test.go:235` (`TestAcquireClaimMalformed`) both assert `Contains(err.Error(), "force-release")`. Both are in section 6 with the parts to keep. `doRelease` still exists after B6 (`release_test.go:50`).
- **Fable round 1:** the Status line records a pass with no must-fix, so nothing to close. Its notes are folded in as listed: the skew labels in Q-T040-2 and Q-T040-4, the full-hive golden in AC6, the T-036 phase B ordering, the T-039 cite, the file count (15 Go plus 2 templates is correct), and the dispatch-base anchors.

## Must-fix

**M1. Rows 10 and 11 are never printed, so AC10's `waggle claim -h` and `waggle release -h` assertions cannot pass inside the lease rule.**
Evidence: `cmd/waggle/claim.go:166-168` (`runClaim`) and `:227-229` (`runTaskRelease`) set `fs.Usage` to a single `Fprintln` of the usage line, with no `fs.PrintDefaults()`. Every other help path calls it: `decide.go:29`, `doctor.go:32`, `status.go:50`, `upgrade.go:27`, `uninstall.go:22`. With `flag.ContinueOnError`, `-h` and every flag parse error print only that one line (`parseTaskArgs`, `claim.go:137-157`). So the `--owner` help strings at `claim.go:170` and `:232` reach no user today. The base `5540658` behaves the same (`claim.go:158-160`, `:219-221` there), so round 1 missed it. The reach column for rows 10 and 11 is wrong. AC10 requires those outputs to "contain the new texts", but the lease rule allows only "the `--owner` help strings" in `claim.go`, so the builder must either fail AC10 or break the lease.
Fix: pick one and state it in the brief.
- (a) **Recommended.** Add `fs.PrintDefaults()` to both `fs.Usage` closures. Name those two lines in the `claim.go` lease rule, count them in section 9 (+2), and add "claim and release `-h` now list their flags" to the Head Keeper ruling card as a help-output change. Every other subcommand already does this. I checked the other flag help in both functions for bee words: `--dirs`, `--gate`, `--ttl`, `--base`, `--repo`, `--json`, `--branch` and `--unmerged` hold none. After T-039 merges, its new `claim --branch` help also becomes visible. T-039's brief gives it no bee word.
- (b) Keep the scope text-only. Set the reach column of rows 10 and 11 to "not printed: `fs.Usage` omits `PrintDefaults` (`claim.go:166-168`, `:227-229`)". Drop `waggle claim -h` and `waggle release -h` from AC10, and record the missing flag help as a follow-up in section 10. Under (b), also tell section 10 item 1 (the T-037 docs touch-up) that the `--owner` help is text no command prints.

**M2. AC10b's gate-only `waggle status` test exits 3, not 0, on the fixture the brief points to.**
Evidence: the gate-only config sets `forge.remote: origin` (`internal/scaffold/templates/gate-config.yaml.tmpl:11`). `runStatus` passes it to the backend (`status.go:83-89`). `ListActiveLeases` then runs `sync`, which is `git fetch --prune ... origin +refs/waggle/claims/*:...`, whenever the remote is non-empty (`internal/comb/git.go:870-874`, `:286-289`). A fetch error returns `exit(3, "failed: ...")` (`status.go:96-98`). The gate-only fixture in `internal/scaffold/gate_only_test.go` (`gateOpts`, `:33-37`; `gateOnlyRepo`, `:229`) adds no remote. The push tripwire says new tests "use the local fixtures named in section 6", and none of those has an `origin`. Followed literally, AC10b's "exits 0 and its first two lines are `Repo: <name>  pin <pin>  claim refs direct (origin)`" fails. Removing the remote to get exit 0 prints `(local)` and contradicts the AC.
Fix: in AC10b and the push tripwire paragraph, say the test adds a local bare repository as `origin` (`git init --bare` plus `git remote add origin <path>`). A fetch is not a push, so the tripwire does not apply to it. Alternatively, let AC10b accept the header with whichever remote the fixture configures. Also name the package the test lives in (`cmd/waggle`, where `runStatus` is) and how it builds the gate-only repo, for example `scaffold.InitRepo` with `GateOnly: true`.

## Notes

N1. **Row 29 is unreachable even with T-036 phase B merged.** `githubRepo` returns the row 29 text only when `remote == ""` (`internal/forge/remote.go:20-22`). `runDoctor` prints `Local-only repo: no forge checks...` and returns 0 at `cmd/waggle/doctor.go:57-60`, before `newForge` runs, so doctor never reaches it. No other caller of `forge.New` exists. Fix the reach column ("no command: doctor returns early on an empty remote, `doctor.go:57-60`"). Keeping the edit is harmless, and section 9's cut already treats 29 as unreached. The Depends line and lease justification can stop presenting doctor as the reason `internal/forge/` is leased.

N2. **T-036 phase B strings are clean.** I scanned every string literal in `cmd/waggle/doctor.go`, `internal/forge/probe.go`, `probe_github.go` and `report.go`. They print no bee word. Only comments say "hive" (`report.go:29-30`). The new `printUsage` line `doctor    Check that claim refs can be pushed ...` (`main.go:129`) is plain. `none.go:151` ("solo local hive") stays out of scope correctly: `initGateOnly` refuses any forge but github (`internal/scaffold/init.go:402-403`).

N3. **T-039 cite and ordering.** The brief cites `t-039b-brief` at `b9b758a` (header and section 0 Read-only context). The branch tip is now `c0329f5` (round 2, a descendant), so update both cites. No conflict, and the order T-039 then T-040 is right:
- T-039's lease rule makes `gate.go`, `internal/dispatch/release.go`, `internal/scaffold/` and `internal/forge/` must-fix for it.
- In `gate_commits.go`, T-039 touches `commitScan` and `claimFor` and bans `classify`. Row 2's message sits in `classify` (`:263-264`).
- In `internal/dispatch/claim.go`, T-039 touches `AcquireClaim` (`:42-62`). Row 6 is in `claimError` (`:84-87`).
- In `claim.go`, both edit `runClaim`. T-039 adds `--branch` and its usage text near the `--owner` line T-040 rewords, so T-040's builder finds row 10 by text after T-039 moves it.
- T-039 pins `TestBindHeadKeeperIgnoresBranch` unchanged (its section 6, `gate_bind_test.go:124-132`). T-040 later changes that test's `Contains` string under row 2, which is legal under T-040's own assertion rule.
- T-039's new failure and warning texts (its 2.4, ACs 9 to 13) hold no bee word.

N4. **Stale anchors at 300fe7a.** All strings exist verbatim, and the Status line says the builder finds them by text. But the lease rule is written in line numbers, and these moved:
- `gate_commits.go` message `:264` (the `HasPrefix` test is `:263`)
- `release.go` row 5 at `:158`
- `claim.go` rows 10 and 11 at `:170` and `:232`
- `main.go` rows 14 and 15 at `:114` and `:127`
- `gate.go` doc comment `:38-40`
- section 6: `gate_bind_test.go` row 1 assertion at `:233` (not `:189`), listed-path text at `:193`, comment at `:229` (not `:181`, `:185-191`)
- section 3 note: `"; or pass --owner <id>"` at `task.go:71` and `release.go:113` (not `:67`, `:131`)
- Q-T040-6: `ErrRelayNotImplemented` defined at `internal/comb/backend.go:170` and returned at `git.go:523` and later (`git.go:130` is the relay switch)

Either refresh the numbers to 300fe7a or add "found by text" to the lease rule itself, so a guard does not read a moved line as out of lease.

N5. **Row 6 advice in remote mode.** The claim lives on the remote. `sync` force-fetches `refs/waggle/claims/*` with `--prune` (`git.go:286-289`), so a ref deleted only locally comes back on the next `waggle claim`, `release` or `status`. Consider `%w; a maintainer must delete the malformed claim ref on the remote by hand`. This is wording, not must-fix. The wrapped error does name the ref (`claim_read.go:37`, `git.go:554`, `backend.go:497`), as the row says.

N6. **Decisions after the base.** D-287 (product copy keeps plain terms, Waggle Gate leads) supports the ticket and the row 14 tagline. Consider adding it to Read-only context next to D-236 and D-281. D-279 and D-280, D-282 and D-283, D-284 to D-286, and D-288 and D-289 do not bear on this brief. D-281's "the Head Keeper subject prefix and keeper_id stay literal until that ticket decides" matches Q-T040-1 (a) and Q-T040-2 (a). The scope widening past D-281's "about 25 lines" is flagged for the ruling card, as it should be.

N7. **Habits.**
- Budget follows D-185: estimate 114 is double the bottom-up 57, target 148 (1.3 times), stop 205 (1.8 times). The planned cut is present, and post-review rounds count toward the stop only (D-210). M1 option (a) adds about 2 lines.
- Review depth meets or exceeds D-205: two first-review guards. Fix rounds touching the named claim, gate and template files get two confirming guards, which is stricter than D-205 requires and fine.
- The Status line's description of the round 1 must-fixes matches what changed.

N8. **AC10 case-sensitivity.** With row 10's new text, `claim -h` contains `keeper_id`. AC10 forbids `Keeper` with a capital K, so it passes, but the builder's section 7 Grep for lowercase `keeper` must treat `keeper_id` as kept, which section 7 already says.
