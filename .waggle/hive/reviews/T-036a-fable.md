<!-- waggle-review v1 verdict=pass head=f3bff0be98e855ae9a8eccee6e7ace6536534735 -->

Risk-Tier: 3

**Intent Summary:** Phase A of T-036 (G4) at `f3bff0b` over base `e82da6f`, one commit, 16 files, all inside the phase A lease (`gate/`, `internal/scaffold/`, `cmd/waggle/`). It adds the composite gate Action at `gate/action.yml`, which builds `cmd/waggle` from the Action's own tree with `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOFLAGS=-mod=readonly`, `GOWORK=off` and `CGO_ENABLED=0`, force-fetches claim refs with prune, and runs `gate verify` with the token only in the gate step's env. The template drops the `issue_comment` trigger, the `relay-claim` job and the shell tier step (no `: write` remains), pins `actions/checkout` and `actions/setup-go` to the D-274 SHAs, and appends a `Waggle gate (Action)` step keyed on the base-tree `present == 'false'` output with `uses: waggle-sh/waggle/gate@{{GATE_REF}} # waggle.pin {{PIN}}`. `gateRef` implements the 2.4 table (clean revision, else release tag with the `[warn]` on `Out`, else no ref: full `init` skips the workflow with the warning, `--gate-only` and `upgrade` refuse). `init --gate-only` writes exactly the three files through a separate `gateOnlyFiles` list after the forge, legacy-layout, symlink and ref checks, with `--force` limited to the workflow. A full `init` with runtime `claude-code` over a config lacking `runtimes:` refuses before any write. `upgrade` detects the gate-only footprint by `Lstat` on the roster and roles dir (D-273), plans only the pin line and the workflow, compares keys against `gate-config.yaml.tmpl`, and returns before stale removal, blocks, policy load and shims. `waggle dispatch release` now exits 2 with the removal hint; `version` is a `var` printed by `printVersion`. Blast radius at merge: none to the hive (its gate builds from base and no gate code changed; its workflow file is untouched); at the next pin move the hive workflow gets a `[rewrite]` and `dispatch release` stops working, as the brief states.

**Verification run on the host (darwin):** `go build ./...`, `go vet ./...`, `gofmt -l .` (empty), `go test -race ./...` all pass, including the push-based `internal/dispatch`, `internal/comb` and `internal/forge` suites:

```
ok  github.com/waggle-sh/waggle/cmd/waggle       44.679s
ok  github.com/waggle-sh/waggle/internal/dispatch 237.877s
ok  github.com/waggle-sh/waggle/internal/scaffold 11.742s
(all other packages ok)
```

**Checks against the brief and threat model (D-203):**
- Lease: diffstat shows only the 16 leased files; `.waggle/`, `.github/`, `go.mod`, `internal/dispatch`, `internal/config`, `internal/decisions` untouched. `uninstall.go` not changed and AC-A8's test passes.
- AC-A3: SPDX first line, `using: composite`, three inputs with 2.2 defaults, no `${{` in any `run:` block, `go-version: "1.23"` equals the `go.mod` line (tested against `../../go.mod`), build step `cd "$GITHUB_ACTION_PATH/.."` with the five env values and no token, fetch line exact, `inputs.token` once in the gate step env, both `uses:` lines in the 40-hex-plus-version-comment form, no `pull_request_target` or `GITHUB_WORKSPACE`. No dashes in any leased file (grep over `gate/**`, `internal/scaffold/**`, `cmd/waggle/**` returns nothing).
- AC-A4: template keeps `on:`, `permissions`, job `if:`, `concurrency`, checkout `with:`, the claim fetch, the `src` step, the `setup-go` `if:`/`with:` and the base-source gate step byte for byte apart from the two SHA pins, the notice text and the comment. The `present` value still comes from `$BASE_SHA:cmd/waggle/gate.go`. `GITHUB_TOKEN: ${{ github.token }}` appears once (gate_approver_template_test unchanged and passing).
- Section 6 changed assertions, each verified in the diff: `guard_template_test.go:43-48` (no `: write`; comment at `:9`), `init_test.go:400-405` (no `for file in`/`::notice file=`), `upgrade_test.go:137-138, 143, 173` (`[rewrite]` for the workflow, count 2, the exact four-line status string), `dispatch_test.go:182-211` (every case exit 2 with the hint; `:208` flipped). No other existing assertion changed; no `t.Skip` added. Pinned-behavior tests in section 6 pass unchanged. I could not read the PR body (no `gh`), so the "listed in the PR body" half of AC-A2 is for the queen.
- Footprint: `initGateOnly` refuses `--forge != github` and an unpinned build before any write; the 2.5 refusal in `InitRepo` sits before `planBlock`, `MkdirAll` and every write (`init.go:263-271`); tests assert a byte-identical tree after each refusal. `--gate-only` in a full hive creates only `DECISIONS.md`.
- Upgrade gate-only path: `plan.renders` holds only the workflow, `plan.removes`, `blocks`, `shimRemoves` and `materialize` stay empty, so `execute()` writes the pin line and the workflow only; `resolvePolicy` is never reached whatever `--runtime` says (tested for both). All preconditions (`checkSymlinks`, block checks, clean tree) still run first, as 2.6 requires.
- 2.4 rule 2 ordering: the tag warning goes to `Out` before `printDecision` and the last line; `cmd/waggle/upgrade_test.go:75-82` passes.

**Notes, not must-fix:**
- The new `!refOK` refusal in `Upgrade` (`upgrade.go:360-363`) is unreachable: `gateRef` fails only when `resolvePin` fails, which `upgrade.go:353` already refuses. Harmless defensive code.
- `gate-config.yaml.tmpl` says "only the Head Keeper or a listed approver may change"; a gate-only user has no hive to define that role. The gate's own messages use the same term (`gate_commits.go:265`), so it is consistent; the G5 recipes should define it.
- `TestDispatchReleaseCLI` dropped three argument-shape cases that all now hit the same first-argument branch; coverage of the new behavior is complete since every form exits identically before flag parsing.
- The hive's own `.github/workflows/waggle-gate.yml` is unchanged, as the brief requires; the Head Keeper's 2.11 follow-ups (`gate/` in `protected_paths`, the pin-move re-render, release workflow) remain open and are not part of this diff.

**Blocking Issues (Must-Fix Only):** none.

Verdict: pass