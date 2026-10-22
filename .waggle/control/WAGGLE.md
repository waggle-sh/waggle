<!-- SPDX-License-Identifier: MIT-0 -->
# WAGGLE.md: Hive Architecture and Boundary Map

> **Hardcoded Tier 3 File:** changes to `.waggle/control/**` need Head Keeper approval.

## 1. Repository Overview
- **Profile:** `cli`
- **Delivery Mode:** `installed`
- **Comb Mode:** `direct`

## 2. Module Boundaries (Directory Leases)
Define the top-level modules Builders may lease:
- `cmd/waggle/`: Operator CLI entrypoint
- `internal/comb/`: Unified Git ref `CombBackend` implementation
- `internal/config/`: Parses `.waggle/control/config.yaml` into typed options (T-004)
- `internal/decisions/`: Append-only `DECISIONS.md` writer behind `waggle decide` (T-006)
- `internal/dispatch/`: `waggle dispatch` claim, worktree, headless run and post-run lease check (T-023)
- `internal/forge/`: `Forge` interface, `none` and GitHub adapters, `waggle doctor` probes (T-018)
- `internal/layout/`: Repo-relative Waggle path constants (T-011)
- `internal/runtime/`: `AgentRuntime` adapters and token cap monitors
- `internal/scaffold/`: `waggle init` embedded templates
- `internal/usage/`: Token and cost ledger reader behind `waggle status` (T-022)

## 3. Build and Verification Commands
- **Build:** `go build ./...`
- **Test:** `go test -race ./...`
- **Vet:** `go vet ./...`

## 4. Hive Overrides
- **Queen summary (D-182, D-215, T-023 phase F):** from the phase F cutover card on, the queen's working handoff follows the Handoff section of her role template (`.waggle/control/handoffs/queen-<n>.md`, gitignored). Until that card she hands off only through the legacy gitignored `.waggle/hive/handoff/queen.local.md` and writes no `queen-<n>.md`; while it exists, the Stop hook refuses to end a turn when it is over 40 lines or older than main's last commit. At cutover she writes the next unused `queen-<n>.md` from it and deletes it, which ends that check. The committed `.waggle/hive/handoff/queen.md` stays a short summary (framing, standing facts, habits, phase status), refreshed only at phase merges and in records batches, and refers to the Keeper by role, never by name; no hook checks it. The SessionStart hook prints `queen.local.md` while it exists, else the newest `queen-<n>.md`, else the summary.
- **Hive scan (D-183):** `.waggle/control/scripts/scan-hive.sh [base] [head]` scans a branch's added lines, path names and commit fields for privacy and dash hits and runs `waggle gate decisions`. It scans the net diff, each non-merge commit against its first parent and each merge commit by remerge diff. Hits print as `<file>:<line>: <rule>` without the matched value, but a path-name hit (`<file>:0`) and the changed-paths list print the path as is. Exit 0 means no hit and every change is an added file under `.waggle/hive/reviews/` or `.waggle/hive/media/`, a pure-append `DECISIONS.md` or an edit to `.waggle/hive/handoff/queen.md`; 1 means a hit; 2 means no hit but some other change (anything outside `.waggle/hive/`, any other added, modified, deleted or renamed hive file, a symlink or a `.gitattributes`). A later guard round appended to an existing review file (role template, Dispatching workers item 3) is a modified hive file, so that batch exits 2. It needs git 2.36 or later and exits 3 under an older git. The queen commits a records batch on its own branch from `origin/main` and runs the scan there: on exit 0 she opens the PR, merges with `--admin` and tells the Keeper in one line; on exit 1 or 2 she fixes the hit by rewriting the branch, or keeps the opus guard and the Keeper card.
- **Review depth (D-205):** every Tier 3 first review gets two guards on different models. A fix round gets one confirming guard, alternating models round to round, unless its diff changes code that sends signals, deletes files or git state, or touches leases or claims, or it adds more than 100 non-test lines (D-136); those get two. If a single confirming guard finds a must-fix, the other model confirms the next round.
- **Budgets (D-185, D-210, standing since D-216):** the scout doubles its bottom-up line estimate before setting estimate, target and stop, and gives every brief with a target a planned cut. A build that passes its target before review splits along that cut without a Keeper card; fix and polish rounds after review count toward the stop only. Only crossing the stop goes to the Keeper.
