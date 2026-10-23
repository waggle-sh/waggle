I found no blocking issues, and the build, vet and the allowed tests all pass.

Verdict: pass
Risk-Tier: 3

**Intent Summary:** Dispatch now tells each worker exactly which Bash commands it may run, and turns off Claude Code's commit attribution. A new exported `runtime.BashCommands(role, commands)` (`internal/runtime/launch.go`) holds the validation and the guard filter that `workerSettings` used to do inline. `workerSettings` builds its `Bash(cmd)` / `Bash(cmd *)` allow rules from that list. `Run` calls it on the base WAGGLE.md commands before building the task lines (`internal/dispatch/run.go:264-275`, before `AcquireClaim` at line 280). The same list feeds a new `Bash:` line in `TaskLines` and `guardTaskLines`, built by `bashLine` in `inputs.go`. That line sits in the variable half of the prompt, so the stable half doesn't change between tasks.

The settings JSON gains `"attribution":{"commit":"","pr":"","sessionUrl":false}`; the `permissions` block is the same struct, built in the same order. The post-run check (`check.go`) now counts commits in `CheckBase..head` that have a `co-authored-by:` line (any case). It reads them through the pinned `c.g`, capped at 501 like the existing commit listing, and adds one `attribution_trailer: <n> commit(s)…` warning. The message text is never copied, and exit code and push are unchanged. Blast radius: every dispatched worker's settings and prompt. No allow rule is added and no deny rule is dropped. A guard still gets no `git add` or `git commit`.

**Checks:**
- **Lease:** the diff touches only the allowed files: `launch.go`, `inputs.go`, `guard.go`, `run.go`, `check.go` and `*_test.go`. `runtime.go` and `go.mod` are unchanged.
- **Budget:** about 90 added non-test lines, under the target of 120.
- **Commands I ran:**
  - `go build ./...`: pass.
  - `go vet ./...`: pass.
  - `go test -race ./... -run '…'`: ok for `internal/dispatch` and `internal/runtime`. The pattern named the new tests (`TestBashCommandsMatchSettings`, `TestBashCommandsRefusals`, `TestBashLineNamesAllowedCommands`, `TestBashLineForms`, `TestBashLineStablePromptUnchanged`, `TestTrailerCountWarnsWithoutText`) plus the brief's list: `TestPrepare[A-Z]`, `TestTripwireBlocksPush`, `TestGuardSettingsDropGitWrites`, `TestInputsComeFromBase`, `TestWaggleCommandsByTitle`, `TestWaggleCommandsAndOutcome`.
- **Formatting:** I couldn't run gofmt here. Reading the new struct fields and literals by hand, they look gofmt-clean. product-ci will confirm.
- **Acceptance criteria:**
  - AC3 is covered by two tests together: the `Bash:` line matches `BashCommands`, and `BashCommands` matches the settings allow rules.
  - AC4: each refused command shape is tested against both functions.
  - AC6 is covered in `TestPrepareArgv`.
  - AC7: no test proves that end, exit code and push are unchanged, because that path goes through `Run`, which can't be tested on this host. The code only appends to `res.Warnings`.
  - AC8 and AC9 hold.

**Blocking Issues:** None.

**Notes (not must-fix):**
- The head commit `a353947` itself carries `Co-Authored-By: Claude Sonnet 5.5`. The builder ran under the old dispatch, before this change. Drop the trailer when rewriting for merge.
- If `rev-parse` of the branch fails, `rng` becomes `<base>..` and the trailer count scans up to the main HEAD. The commit listing already behaves this way and a ruleCommit violation is raised anyway, so the only effect is a possibly wrong warning count.

I created no probe files.

Verdict: pass