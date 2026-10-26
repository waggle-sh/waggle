<!-- waggle-review v1 verdict=pass head=020bcc17034c9ffe5a063d4a135d5035986ae3cf -->

Risk-Tier: 3

**Intent Summary:** T-042 removes the Q-T039-1 (b) transition fallback from the merge gate: in `checkHead`, a bound commit whose claim decodes with an empty or nil branch list now always fails `leases` with the T-039 renew hint, instead of passing with a warning when the PR head happened to be the ticket's own branch name. The helper `ticketBranchName` and the warning string are deleted outright (D-303). Nothing else in the branch check, binding, claim writing or the schema changes. Non-test numstat is 0 added, 17 deleted, all in `cmd/waggle/gate_commits.go`, matching section 8 exactly. Test changes flip the four AC12 (b) rows to (a), switch four T-039 (a)-only tests from the no-list `claim` fixture to `claimOn`, delete the unit test of the deleted helper, and add four tests for AC3 to AC6. Blast radius: from the first hive PR after merge, any bound commit whose claim ref lacks a `branches` key fails the gate on every head, so the section 9 preconditions (pin moved, every claim ref renewed or released) must hold at merge; the brief's Status says they do.

**Verification run in this worktree at `020bcc1`:**

```
go build ./...     (clean)
go vet ./...       (clean)
gofmt -l .         (no output)
go test -race ./... 
ok  github.com/waggle-sh/waggle/cmd/waggle          55.299s
ok  github.com/waggle-sh/waggle/internal/comb       29.448s
ok  github.com/waggle-sh/waggle/internal/config      1.401s
ok  github.com/waggle-sh/waggle/internal/decisions   3.503s
ok  github.com/waggle-sh/waggle/internal/dispatch  218.975s
ok  github.com/waggle-sh/waggle/internal/forge      14.633s
ok  github.com/waggle-sh/waggle/internal/runtime     6.468s
ok  github.com/waggle-sh/waggle/internal/scaffold   12.302s
ok  github.com/waggle-sh/waggle/internal/usage       1.181s
```

**Lease and lease-rule check:** the diff touches only `cmd/waggle/`: `gate_commits.go`, `branches_test.go`, `claim_branches_test.go`, `gate_approve_test.go`, `gate_bind_test.go`. No file in `internal/`, `.waggle/`, `.github/`, `gate/`, `docs/`, `README.md` or `go.mod` changes. In `gate_commits.go` the deletion is exactly the four-line `if` block inside the `len(l.Branches) == 0` case and `ticketBranchName` with its doc comment and trailing blank line. No doc comment changed, as section 8 predicted. `branchTicket` stays and is still called from `gate_verify.go:140` and `classify`. The deleted helper had no other caller, so no unexported function is left orphaned (AC7). A grep of `cmd/waggle/` finds `ticketBranchName` nowhere and the warning text only in the `fallbackWarn` test constant, which two tests still assert absent, which section 6 allows. No em or en dash in any changed file or the commit message (AC8).

**Threat bar (D-203), applied:** after the change `checkHead` has three outcomes only: unknown head (unchanged, runs first), no list (one failure, no path, returned before the loop), listed (unchanged match loop). The no-list failure is filed with an empty path under `leases`, so the approver filter cannot drop it, and `cl.ok` stays true in `claimFor` so path rules still run. `len()` treats nil and empty identically, so a hand-written `"branches":[]` fails closed. I found no head, same-repo or fork, known or ignored, that passes a bound commit on a no-list claim. The failure message is the T-039 hint word for word with only the canonical ID substituted.

**Section 6 map against the diff (AC2):** every changed existing assertion and fixture call matches an entry.

- `TestClaimWithoutBranchList` first loop: exit 0 plus warning flipped to exit 1, `noListMsg` present, `fallbackWarn` absent; comment changed from (b) to (a). Section 6 entry "AC12's (b) rows flip".
- `TestTicketBranchName` deleted with the helper. Section 6 entry "unit test of the deleted helper".
- `TestBindSources` "flag", "flag suffix", "flag waggle": `claimOn` with the row's `--branch` value; "env": `claimOn` with `t-031`. Trailer rows and the second loop keep the no-list `claim`, which is correct since they run with no head. Section 6 entry "(a) only breaks".
- `TestBindConflicts` "all agree": `claimOn` with the row's branch `t-031-x`; other rows keep `claim`, so the per-row rule in section 6 applies and no cross-row report is needed.
- `TestBranchDefaultFromEvent`: shared claim switched to `claimOn` with `t-031`. The `t-099` row still fails because T-099 has no claim; the hostile row still fails on binding. Both hold.
- No `t.Skip` added. The pinned tests section 6 lists as "must pass unchanged" are untouched in the diff and pass.

**New tests against AC3 to AC6:** AC3's four heads and the expired row (`TestExpiredClaimWithoutBranchList` asserts the failure, the `expired at` warning and no fallback warning). AC4 (`TestApproverNoBranchList`) follows the `TestApproverPasses` setup exactly as the brief specifies, overwriting ticket 31's claim with a no-list claim and asserting the T-031 hint under an approval at head. AC5 (`TestNoListUnchangedCases`) covers the local once-only warning, the Actions `pull request branch unknown` row with `names no branch` absent, the listed pass and the `feature/x` failure. AC6 (`TestRenewalClearsNoBranchList`) is the full form: a plumbing-forged `alice`/`forager`/`internal/x` claim with no `branches` key, gate fails with the T-023 hint, `runClaim` with `--owner alice` and no `--branch` exits 0 and reads back `"branches":["t-023"]`, gate then passes. The file carries `//go:build unix` and `dispatchRepo` has no `origin`, so nothing pushes.

**Notes, not must-fix:**

- AC3 says the failure appears "once"; the new first-loop assertion uses `Contains` rather than `Count == 1`. Once-per-ticket is structural through the `s.claims` cache, and `TestClaimGapClosed` already pins `Count == 1` for the sibling message on the same path, so I do not require a change.
- The lease rule requires the PR body to list each changed existing assertion with its section 6 entry. The commit body summarizes but does not itemize. The map above is that list; the queen should carry it into the PR body when the PR opens, and the Keeper should check it there.
- AC5 extends `TestClaimUnknownHead`'s Actions row as a new test rather than inside the existing one. Behavior coverage is the same.
- Budget: 0 added non-test lines, so the 16-line target was never approached and D-307's stop-and-report did not trigger. Added test lines total 86, a little over section 8's 50 to 70 estimate; tests do not count.

**Blocking Issues (Must-Fix Only):** none.

Verdict: pass