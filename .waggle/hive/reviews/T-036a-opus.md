<!-- waggle-review v1 verdict=pass head=f3bff0be98e855ae9a8eccee6e7ace6536534735 -->

Risk-Tier: 3

**Intent Summary:** Phase A of T-036 (G4) is in commit `f3bff0b`. It adds the composite Action `gate/action.yml`. The Action sets up Go 1.23 at a pinned commit, builds `cmd/waggle` from its own tree with the five locked-down build settings and no token, fetches the claim refs with force and prune, and runs `gate verify` with the token in that step's env only. In the workflow template, the relay job, the `issue_comment` trigger and the shell tier step are gone, `checkout` and `setup-go` are pinned by commit, and a `Waggle gate (Action)` step is added after the base-source gate. That step runs only when `steps.src.outputs.present == 'false'`, and its `uses:` ref comes from a new `{{GATE_REF}}` value: the build's clean revision first, then a release tag with a `[warn]` line on `Out`, and no ref at all for an unpinned build. In that case a full `init` skips the workflow, `--gate-only` refuses before writing anything, and `upgrade` refuses. `init --gate-only` writes exactly the three files from two new templates. A full `init` with `claude-code` refuses before any write when the config has no `runtimes:` block. `upgrade` treats a repo with no roster and no roles as gate-only: it touches only the pin line and the workflow, compares keys against `gate-config.yaml.tmpl`, and skips the removal, block, policy and shim steps. `waggle dispatch release` now exits 2 with the removal hint, and `version` is a `var` printed by `printVersion`. Every file is inside the phase A lease; `uninstall.go`, `internal/dispatch` and `.github/` are untouched. Blast radius: every `init` and `upgrade` user's workflow, and at the next pin move the hive's workflow is rewritten (expected, 2.4 and 2.11 item 3).

**Verification:**
- **Commands:** at `f3bff0b`, `go build ./...`, `go vet ./...` and `gofmt -l .` all printed nothing. `go test -race ./... -count=1` passed every package: cmd/waggle, comb, config, decisions, dispatch, forge, runtime, scaffold and usage.
- **Lease:** `git diff --stat` shows 16 files, all under `gate/`, `internal/scaffold/` or `cmd/waggle/`. Only the doc comment changed in `claim.go`.
- **Existing assertions changed:**
  - `TestGateTemplateBaseBuilt` now checks for no `: write` at all, and its `:9` comment changed, as 2.7 and section 6 require.
  - `TestRenderedTemplatesClean` now checks for no `for file in` and no `::notice file=` (2.7).
  - `TestUpgradeRoundTripRemovesDroppedRole` expects `[rewrite]` for the workflow, a rewrite count of 2, and the status string exactly as section 6 gives it (2.4).
  - `TestDispatchReleaseCLI` expects exit 2 for every case, the removal hint, and no `release` in the usage (2.8).
  - No other existing assertion changed and no `t.Skip` was added.
- **Threat bar (D-203):**
  - No `${{` appears inside any `run:` block of the Action or the template.
  - `inputs.token` appears only in the gate step's env, and the build step's env has no token.
  - The build runs from `$GITHUB_ACTION_PATH/..`, never the workspace or the PR head.
  - `present` comes from the base tree, and every `uses:` has the form `@` + 40 hex + `# v`.
  - No write permission and no `pull_request_target` remain.
  - Every gate-only refusal happens before the first write. The tests check that the file set is exactly the three files, that existing bytes are kept, and that `--forge none` writes nothing.
- **AC-A3 to AC-A10** each have a matching test in `gate_action_test.go`, `gate_only_test.go`, `dispatch_test.go` or `version_test.go`, and all pass. No added line in the leased paths has an em or en dash.

**Notes (not blocking):**
- AC-A2 and AC-A11 need two lists in the PR body: the changed existing assertions with their brief sections, and the 2.11 Head Keeper follow-ups. The commit message has neither. Whoever opens the PR should add them.
- A pre-release tag counts as a release tag here, so a tag-only pre-release build pins the Action by that tag and prints the warning. This matches 2.4 rule 2.
- The 2.4 tag warning is now printed on a full hive's tag-only `upgrade` too, before the last line. `cmd/waggle/upgrade_test.go` still passes.

**Blocking Issues (Must-Fix Only):** None.

Verdict: pass