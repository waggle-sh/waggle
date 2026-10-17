<!-- SPDX-License-Identifier: MIT-0 -->
# WAGGLE.md: Hive Architecture and Boundary Map

> **Hardcoded Tier 3 File:** changes to `.waggle/control/**` need Head Keeper approval.

## 1. Repository Overview
- **Profile:** `cli`
- **Delivery Mode:** `installed`
- **Comb Mode:** `direct`

## 2. Module Boundaries (Directory Leases)
Define the top-level modules Foragers may lease:
- `cmd/waggle/`: Operator CLI entrypoint
- `internal/comb/`: Unified Git ref `CombBackend` implementation
- `internal/config/`: Parses `.waggle/control/config.yaml` into typed options (T-004)
- `internal/decisions/`: Append-only `DECISIONS.md` writer behind `waggle decide` (T-006)
- `internal/forge/`: `Forge` interface, `none` and GitHub adapters, `waggle doctor` probes (T-018)
- `internal/layout/`: Repo-relative Waggle path constants (T-011)
- `internal/runtime/`: `AgentRuntime` adapters and token cap monitors
- `internal/scaffold/`: `waggle init` embedded templates
- `internal/usage/`: Token and cost ledger reader behind `waggle status` (T-022)

## 3. Build and Verification Commands
- **Build:** `go build ./...`
- **Test:** `go test -race ./...`
- **Vet:** `go vet ./...`
