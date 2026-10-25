<!-- waggle-review v1 verdict=pass head=90b4f493be47f9440a81533cf77a4df4751f49cb -->

Everything checks out. The worktree is clean and I wrote no files, so there is nothing to delete.

Risk-Tier: 3

**Intent Summary:** T-038 phase B (one commit, `90b4f49` on `t-038b`, base `4829cc9`) closes six review notes in the CLI and removes dead code in dispatch. `parseTaskArgs` now re-parses the remaining arguments after taking the task, so flags may sit before, between or after the task argument for both `waggle claim` and `waggle release` (B1). Usage errors in both verbs pass the canonical ticket ID from `comb.ParseTicketID` to `fail`, so the JSON `task` field is filled when the argument parses; text output is untouched because `fail` never prints the task (B2). The gate usage text moves the `--branch` paragraph after the `decisions` entry with its wording unchanged (B3). `trailerTickets` checks the field length and comma count before splitting, which is an exact equivalent since `strings.Count(raw, ",")+1` equals the split length for non-empty input (B4). `runGateRange` files the bad-branch warning under `tiers` on a `tiers` run and under `leases` otherwise (B5). `dispatch.Release`, the `legacyOwner` field and its exit-code branch, and the unused `io` import are deleted; `ReleaseLine` stays and `doRelease` in the test file reproduces the old wrapper's output lines, so no assertion changed (B6, D-286). Blast radius: the hive's next PR runs this gate from the PR base; `verify` and `leases` output is byte-identical, so only the usage text is visible there. The claim and release CLI changes reach the hive only at the next pin move. Added non-test lines are about 24 against a target of 86.

**Verification run (one command per call):** `go build ./...` clean, `go vet ./...` clean, `gofmt -l .` printed nothing, `go test -race ./...` passed every package (`cmd/waggle` 46s, `internal/dispatch` 217s, all others ok). The remote-mode fixtures ran on this host and passed, so no push-tripwire failures to report.

**Lease and rule checks:**
- Eight files changed, all inside the phase B lease rule: `claim.go` (import, `parseTaskArgs`, the usage `o.fail` calls), `claim_test.go`, `gate.go` (usage text only, first line unchanged), `gate_bind_test.go`, `gate_commits.go` (`trailerTickets` only), `gate_verify.go` (the warning in `runGateRange` only), `release.go` (`Release`, `legacyOwner`, `io`), `release_test.go` (`doRelease` only). Nothing in `.waggle/`, `.github/`, `go.mod`, `main.go`, `dispatch.go` or `internal/comb/`.
- No existing assertion changed anywhere in the diff, so the PR body has nothing to list under the T-034 rule. No `t.Skip` added.
- No caller of `Release` or reference to `legacyOwner` remains in any Go file, and `runRelease` is gone from base (G4 phase A merged), so the B6 precondition holds.
- No em or en dash in any Go file. No signal or running-git cancel in tests (D-193 not engaged).
- Commit is signed off with the `T-038: ` prefix.

**Acceptance criteria traced:** AC11 and AC12 in `TestTaskArgPosition` (three task positions claim `T-032`, release with flags before the task, `-h` after the task exits 0, two tasks exit 2 with empty stdout, unknown flag after the task prints no JSON, `a,,b` gives `usage` with `T-034`, no task gives `""`, `release --json 23 24` gives `T-023`). AC12b verified by grep and the passing `doRelease` tests. AC13 verified by reading the usage text: `decisions` follows `tiers`, then a blank line, then the unchanged paragraph. AC14 in `TestTrailerTicketsBounds` with all ten rows the brief lists and `TestBindConflicts` unchanged. AC15 in `TestBadBranchWarningPerCheck`. AC16 clean.

**Blocking Issues (Must-Fix Only):** none.

**Notes (not blocking):**
- The `parseTaskArgs` doc comment still says the task comes "before or after the flags"; it may now also sit between them. Cosmetic.
- The AC11 release row uses ticket 23 rather than the brief's 32 and asserts exit 0 and command only. It still exercises flags before the task, which is the point of the row.
- In `runClaim` the ticket parse sits as one line before the switch rather than inside each `fail` call. That is the natural minimal form of the lease's "the `o.fail` calls for usage errors" and adds no behavior.

Verdict: pass