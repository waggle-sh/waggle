<!-- waggle-review v1 verdict=pass head=020bcc17034c9ffe5a063d4a135d5035986ae3cf -->

Risk-Tier: 3

**Intent Summary:** This commit (`020bcc1`, one commit on `t-042` over `87870f2`) removes the Q-T039-1 (b) ticket-branch-name fallback from the gate, as D-291 and D-303 ordered. In `cmd/waggle/gate_commits.go` it deletes the `if` block in the `len(l.Branches) == 0` case of `checkHead` and the now-uncalled `ticketBranchName`. The numstat is 0 added and 17 deleted, which matches section 8 and AC7 exactly. A known head with a no-list claim now always fails `leases` with no path, with the T-039 renew hint word for word (`gate_commits.go:356`). The unknown-head case (`:345-354`) and the listed-branch case (`:359-368`) are byte-identical. `branchTicket` and `h.sameRepo` are still used by binding and the listed-branch match. The test changes follow section 6:
- `TestClaimWithoutBranchList`'s first loop flips to exit 1 with `noListMsg` and no `fallbackWarn`, and its comment now says (a).
- `TestTicketBranchName` is deleted along with its helper.
- `TestBindSources` "flag", "flag suffix", "flag waggle" and "env" switch to `claimOn` with the branch each row runs on. The trailer rows keep the no-list `claim`.
- In `TestBindConflicts`, only the "all agree" row switches, to `claimOn([t-031-x])`. The other rows keep their claim and expectations unchanged.
- `TestBranchDefaultFromEvent` switches to `claimOn([t-031])`.
- New tests cover AC3's expired row (`TestExpiredClaimWithoutBranchList`), AC4 (`TestApproverNoBranchList`, using the `TestApproverPasses` setup with ticket 31's claim overwritten by a no-list claim), AC5 (`TestNoListUnchangedCases`, local, Actions, listed and other-branch rows) and AC6 (`TestRenewalClearsNoBranchList` on `dispatchRepo`, in the `//go:build unix` file `claim_branches_test.go`).

Blast radius: from the first hive PR after merge, any bound commit whose claim has no `branches` gets a fail-closed `leases` result. That is why section 9's freeze reruns must hold at merge.

Evidence (run at head `020bcc1`):
- `go build ./...`, `go vet ./...`: no output.
- `gofmt -l .`: no output.
- `go test -race ./...`: every package `ok`, including `cmd/waggle` (56.2s) and `internal/dispatch` (219.9s). No test failed because of a push.
- Grep in `cmd/waggle/` finds `ticketBranchName` and `accepted on ticket branch name` only in the test constant `fallbackWarn`. That constant is kept because tests assert the warning is absent, which section 6 allows. No added line has an em or en dash.
- Lease: only the five `cmd/waggle/` files above changed. No `t.Skip` was added.

Threat bar (D-203): a finding whose attacker needs to write claim refs, the base revision, the base config, the workflow or GitHub's event payload is a note, not a must-fix; must-fix is for any way a PR author, through commits, subjects, trailers, the branch name, the PR body or a fork, gets a bound commit past a claim that names no branch or does not name the PR's head branch, and for any change to the branch check beyond removing the ticket-branch-name fallback.

Against that bar, no head passes a no-list claim any more:
- **Same-repo, fork and ignored-name heads** all reach the single failure.
- **Approver filter:** the failure is filed with an empty path, so the filter cannot drop it. `TestApproverNoBranchList` shows an approval at head does not clear it.
- **Path rules:** `cl.ok` is still set before `checkHead`, so a claim's path rules still apply when its list is empty.

**Blocking Issues (Must-Fix Only):** none.

Notes, not must-fix:
1. I don't have the PR body, so I could not check AC2's list of changed assertions. The changes I see in the diff are exactly section 6's entries, plus the deleted `TestTicketBranchName`. The queen should confirm the PR body lists each of them.
2. AC3 says "once", but the first loop and the expired row check the message with `strings.Contains`, not a count. Through the `s.claims` cache the message is still emitted once per ticket, so this is only a looser test.

Verdict: pass