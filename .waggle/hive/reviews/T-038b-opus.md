<!-- waggle-review v1 verdict=pass head=90b4f493be47f9440a81533cf77a4df4751f49cb -->

Risk-Tier: 3

**Intent Summary:** I found no blocking issues in T-038 phase B (head `90b4f49`, one commit on base `4829cc9`); verdict pass. The commit makes six changes:
- **B1:** `parseTaskArgs` in `cmd/waggle/claim.go` now accepts the task argument before, between or after the flags. When the first parse leaves arguments and no task has been taken, it takes the first one as the task and parses the rest again. Help and parse errors on that second parse are handled the same way as on the first.
- **B2:** the usage errors in `runClaim` and `runTaskRelease` now pass the canonical ID from `comb.ParseTicketID`. That function returns `""` on error, so a raw task string never reaches the JSON.
- **B3:** the `decisions` entry in the gate usage text now comes straight after `tiers`, followed by a blank line and the unchanged `--branch` paragraph.
- **B4:** `trailerTickets` checks the field length and `strings.Count(raw, ",")+1` before splitting. `strings.Split` always returns that many parts, so every input gets the same result as before.
- **B5:** `runGateRange` files `branch name ignored` under `tiers` on a `tiers` run and under `leases` otherwise, so `verify` and `leases` output does not change.
- **B6:** `dispatch.Release`, the `legacyOwner` field and its branch, and the unused `io` import are deleted. The `doRelease` test helper now calls `ReleaseTask` and prints `warning: <w>` lines and then `ReleaseLine` itself, exactly as `Release` did. No caller of `Release` remains in any Go file at the head (grep). `runRelease` is gone, and `cmd/waggle/dispatch.go:45` prints the removal hint.

Blast radius: the `claim` and `release` CLI argument handling, and the gate's usage text and warning placement. The gate's pass and fail results do not change.

**Verification:**
- **Commands:** `go build ./...`, `go vet ./...` and `gofmt -l .` print nothing. `go test -race ./...` passes in every package (cmd/waggle 45.6s, internal/dispatch 215.2s, and the rest).
- **Lease:** the 8 changed files all fall inside the phase B lease rule. The only claim.go lines outside the named functions are the two `id, _ := comb.ParseTicketID(task)` lines that feed the `o.fail` calls, plus the allowed import line.
- **Existing tests:** no existing assertion changed. Only the body of the `doRelease` helper changed, as B6 allows, and no `t.Skip` was added.
- **Budget:** about 24 added non-test lines, well under the 86-line target.
- **Dashes:** no em or en dash in `cmd/waggle/` or `internal/dispatch/`.
- **Acceptance criteria:**
  - `TestTaskArgPosition` covers AC11 and AC12. `runJSON` adds `--json` at the end, so in the row `--json --repo R 23 24` the first parse sets JSON and `extra` is 2, giving exit 2, `usage`, `T-023` as AC12 asks.
  - `TestBadBranchWarningPerCheck` covers AC15.
  - `TestTrailerTicketsBounds` covers AC14. The 256- and 257-byte fields are constructed at those exact lengths.

**Blocking Issues (Must-Fix Only):** none.

**Notes (not blocking):**
- N1. After a `--` terminator, flags that come after the task are now parsed: `waggle claim -- T-32 --dirs x --gate full` is accepted. Before this change it was a usage error, because everything after `--` counted as an extra argument. It follows directly from B1's "parse the rest again". No test pins the old behavior, and no exit code or JSON code changes for any input that worked before.
- N2. The `parseTaskArgs` doc comment (`claim.go:134`) still says the task "comes before or after the flags". It may now also sit between them. This is wording only.
- N3. The AC11 release row runs `--unmerged 23 --repo R --json` rather than the brief's `--json 32 --unmerged --repo R`. Both put the task between flags, so the case under test is the same.
- N4. I can't see the PR body from here, so the queen still needs to check that it lists the `doRelease` helper rewrite under B6 (AC2).

Verdict: pass