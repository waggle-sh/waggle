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
- **Queen handoff (D-182):** until T-023 phase F, this hive's queen ignores the `.waggle/control/handoffs/` path in its role template and keeps two files. The working handoff is `.waggle/hive/handoff/queen.local.md`: gitignored, facts only, at most 40 lines, rewritten after every merge. The SessionStart hook prints it, and the Stop hook refuses to end a turn while it is missing, over 40 lines or older than main's last commit; the starter structure is the comment at the top of `.claude/hooks/handoff-check.sh`. The committed `.waggle/hive/handoff/queen.md` is a short summary (framing, standing facts, habits, phase status) refreshed only at phase merges and in records batches; the SessionStart hook prints it only when the local file is missing, and no hook checks it.
- **Hive scan (D-183):** `.waggle/control/scripts/scan-hive.sh [base] [head]` scans a branch's added lines, path names and commit fields for privacy and dash hits and runs `waggle gate decisions`. Exit 0 means clean and every changed path is under `.waggle/hive/`; 1 means a hit; 2 means clean but a path outside `.waggle/hive/` changed.
