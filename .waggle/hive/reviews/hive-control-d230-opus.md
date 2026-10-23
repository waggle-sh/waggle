# Guard review: hive control PR #85 (D-230 gofmt)

Verdict: pass
Risk-Tier: 3

Subagent guard hand-back (opus), condensed by the queen.

## Intent Summary
Commit 1802e18 on origin/main 52a0efb adds one bullet to `.waggle/control/WAGGLE.md` section 3: "- **Format:** `gofmt -l .` (lists unformatted files; empty output means clean)". Dispatch now gives every builder, forager and guard `gofmt -l .` as a fourth verification command. Nothing else changes.

## Checks
1. Parser and allow rules: `waggleCommands` (internal/dispatch/run.go) run on the branch file returns "go build ./...", "go test -race ./...", "go vet ./...", "gofmt -l ."; the parenthetical is dropped. `runtime.BashCommands` accepts it and `workerSettings` emits `Bash(gofmt -l .)` and `Bash(gofmt -l . *)`. Go's flag package stops at the positional ".", so `gofmt -l . -w` and `gofmt -l . -r 'a->b'` treat the extras as paths (exit 2, file unchanged, verified in scratch); `-w`, `-r` and `-cpuprofile` are unreachable. The `*` form can only name extra paths to list, read-only.
2. Upgrade and scaffold: WAGGLE.md.tmpl is seeded; upgrade skips seeded files (TestUpgradeSeededFilesKeepTheirBytes). No revert or conflict.
3. Other parsers, tests and CI: only `waggleCommands` (run.go line 302) reads section 3; its tests use inline fixtures. waggle-gate.yml only warns on the Tier 3 path; ci.yml runs its own `git ls-files` gofmt step. Nothing breaks.
4. Diff hygiene: 1 file, 1 insertion, no em or en dashes.

## Findings
1. note: `gofmt -l .` walks untracked, vendored and testdata .go files while CI checks tracked files only, so a worker may see a hit CI would not. `gofmt -l cmd internal` would match CI more closely if it gets noisy.
2. note: under D-203 the trailing `*` rule can list paths outside the worktree, a read the Read tool already allows. Not a must-fix.
