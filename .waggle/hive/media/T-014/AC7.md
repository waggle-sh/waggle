# T-014 AC7 evidence

Captured by the queen on 2026-10-18 from the GitHub check-runs API, after T-014 merged (fable F9: AC7 can only be proven once `gate_verify.go` is on base).

| PR | Scratch edit | Head | `verify-leases-and-tiers` | Failure annotations |
|----|--------------|------|---------------------------|---------------------|
| #44 | one blank line appended to `README.md`, commit `T-014: SCRATCH AC7 out-of-lease edit` | `969b25d` | failure | `leases: no claim for T-014; claims must stay until merge` |
| #45 | new file `GO.MOD` beside `go.mod`, commit `T-014: SCRATCH AC7 GO.MOD edit` | `4f5d1af` | failure | `go.mod`: `tiers: path collides with GO.MOD on case-insensitive checkouts`; `leases: no claim for T-014; claims must stay until merge` |

Raw API output: `ac7-pr44-annotations.json`, `ac7-pr44-checkrun.json`, `ac7-pr45-annotations.json`, `ac7-pr45-checkrun.json`. `product-ci` passed on both, so the failure comes from the gate alone.

Limit: the remote has no claim refs (`refs/waggle/claims/*` is empty), so #44 proves the gate refuses an edit no lease covers, not an edit outside an existing claim's paths. That second case needs a claim on the remote, which arrives with `waggle dispatch` (T-023 phase C). A first run of both PRs with untagged commit subjects failed on `leases: commit has no T-<id>: prefix`, as expected.
