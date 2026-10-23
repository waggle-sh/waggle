# Guard review: hive control PR #85 (D-230 gofmt)

Verdict: pass
Risk-Tier: 3

Subagent guard hand-back (fable), condensed by the queen.

## Intent Summary
PR #85 (one commit 1802e18 on origin/main 52a0efb) adds one bullet to `.waggle/control/WAGGLE.md` section 3: "- **Format:** `gofmt -l .` (lists unformatted files; empty output means clean)". Every future builder, forager and guard gains `Bash(gofmt -l .)` and `Bash(gofmt -l . *)` and sees the command in its task prompt. List mode is read-only and the wildcard form cannot become a write or exec. The branch is gofmt clean.

## Checks
1. Parsing: `waggleCommands` on the branch file yields "go build ./...", "go test -race ./...", "go vet ./...", "gofmt -l ."; each passes `runtime.BashCommands`; deniedRules (git push, gh, curl, wget, ssh, WebFetch, WebSearch) still win.
2. Widening: after the positional "." further arguments are paths. Verified in scratch: `gofmt -l . -w`, `-r 'f -> g'` and `-d` fail with lstat errors, exit 2, file unchanged. Shell operators split into separately matched commands, so `gofmt -l . && <x>` grants nothing.
3. Upgrade and scaffold: WAGGLE.md.tmpl is seeded and skipped by upgrade (TestUpgradeSeededFilesKeepTheirBytes); its section 3 holds only placeholders.
4. Tests, CI, gate: no test pins the real section 3 command set (run_test.go:340, d1_test.go:90 and :159, bash_test.go:35); ci.yml runs its own gofmt step; waggle-gate.yml, scan-hive.sh, roles and .claude do not read section 3. On the branch `gofmt -l .` is empty, `go vet ./...` passes, `go test -race` passes for internal/dispatch, internal/runtime, internal/scaffold and cmd/waggle.
5. Diff hygiene: one line, no em or en dashes.

## Findings
1. note: `Bash(gofmt -l . *)` can list unformatted Go files outside the worktree (file names only), within the existing unscoped Read allow. No action.
2. note: the worker check covers untracked files and testdata, a superset of CI's tracked-file check. Safe direction. No action.
