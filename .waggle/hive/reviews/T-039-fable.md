<!-- waggle-review v1 verdict=pass head=8978c6265a0ac51cc41691b1b4a89f8d89bfd709 -->

# Guard review: T-039 round 1 (fable)

Risk-Tier: 3

Head under review `8978c62` on `t-039`, diffed against the branch point `32adb18`. Base `b80f83c` changed nothing under `cmd/waggle`, `internal/comb` or `internal/dispatch` since the branch point, so the review of the branch diff holds against Base.

**Intent Summary.** The claim schema gains an optional `branches` list at lease version 2, validated on every read and write by a new exported `ValidateBranches` in `internal/comb`. The two dispatch claim writers and `ClaimTask` record the list (given list, else the stored list on a same-keeper renewal, else the request branch), the keepalive repeats what the claim recorded, and `waggle claim` gains a repeatable `--branch` flag and a `branches` JSON field. The gate reads `head.repo` and `base.repo` from the PR event, resolves one head identity (name, same-repo, repository, Actions) in `runGateRange`, and `claimFor` now fails a bound ticket whose readable claim does not name that head. A claim with no list falls back to the ticket's own same-repo branch names with a warning (Q-T039-1 (b)); forks match only `<owner>/<repo>:<branch>` entries (Q-T039-3 (a)). Blast radius: every gate run on every repo built from the new base, and every claim write. Sixteen files changed, all inside the three leased directories; no `.waggle/`, `gate/`, `.github/`, `docs/` or `go.mod` change.

**Verification run here.** `go build ./...`, `go vet ./...`, `gofmt -l .` (empty) and `go test -race ./...` all passed. The dispatch package took 218 seconds and passed, so the remote-mode push fixtures ran fine on this host. Added non-test lines by numstat: 201, under the 240 estimate and the 312 target. Worktree clean after the review; no probe files written.

## Threat model walk (section 5)

Threat bar (D-203): a finding whose attacker needs to write claim refs, the base revision, the base config, the workflow or GitHub's event payload is a note, not a must-fix; must-fix is for any way a PR author, through commits, subjects, trailers, the branch name, the PR body or a fork, gets a bound commit past a claim that does not name the PR's head branch, and for any claim writer that loses the branch list.

Each must-fix item checked against `checkHead` (`cmd/waggle/gate_commits.go:343-371`), the head resolution (`gate_verify.go:148-155`) and the writers:

- **Listed claim, no match passes:** no. Same-repo heads match a plain entry byte for byte only; fork heads match only a fork entry with `EqualFold` on the repo and exact bytes on the branch. A fork head never matches a plain entry (`!fork && h.sameRepo`), and a same-repo head never matches a fork entry.
- **Fork takes the fallback:** no, the fallback requires `h.sameRepo`.
- **Same-repo without a confirming event under Actions:** `sameRepo` is `!gha || *branch != ""` only when no event was read, so `pull_request_target`, an unreadable event file and a plain `GITHUB_HEAD_REF` under Actions are forks. With an event, `sameRepo` needs both repo names non-empty and equal. `--branch` with an event that says fork stays a fork.
- **Fallback for another ticket:** `ticketBranchName` cuts the exact `t-<d>` or `waggle/T-<d>` prefix with canonical digits and requires an empty or non-digit remainder; `t-0040`, `t-40`, `t-0400`, `xt-040` all refused; `branchTicket` ignored names refused. Table test covers these.
- **Case folding on the branch part:** none; `branch == h.name`.
- **Empty head under Actions / ignored name skipping:** empty name under Actions fails per ticket; an ignored name is known, cannot be a claim entry (validation is ASCII under 256 bytes), so it fails.
- **Approver filter or approval clearing it:** the failure has no path and is filed from `claimFor`, outside the path loop; the rescan at `gate_approve.go:235-242` builds a fresh `commitScan` and the filter only skips `CheckChangesPolicy` findings. `TestClaimNamesHead` covers the rescan with an approval at head.
- **Echo:** all four new gate messages are constants plus the canonical id; validation errors name position and rule, and the comb test asserts no entry text in the error. The hostile head test asserts no `\r` or injected line in output.
- **Writers losing the list:** `AcquireClaim` swaps in the stored list before `AcquireLease` and `c.Lease` is reassigned from the read-back or the mutated `want` (`claim.go:78-80`), so the `run.go:409` keepalive repeats the stored list; AC7 test records every lease and checks them. Takeover goes through the expired case first and never inherits. `ClaimTask` validates before `refuseHardcoded` and writes nothing on refusal (ref list compared before and after).
- **Invalid stored list read as valid:** `validateLeaseFields` calls `ValidateBranches`; round-trip test shows a forged `a..b` list reads as `ErrMalformedClaim`.
- **Limits:** 4 entries, duplicates, 255 bytes all enforced and tested.
- **`classify`, `branchTicket`, `LeaseVersion`, `Validate`, `checkBranch`, `run.go` outside `:409`:** unchanged in the diff. Older claims read with `Branches == nil`, tested.
- **D-193:** the one `Run` test uses `f.run` with `KeepaliveEvery`, no `Timeout`, no cancel, no signal, the same shape as `fix_test.go:93`.
- **Dashes:** no em or en dash in any `.go` file in the tree.

## Lease rule and section 6

Every changed file is in the lease rule list and every change is inside a named item, with one addition the brief did not enumerate (see notes). Existing assertion and fixture changes, each matched to section 6:

- `writeEvent` gains both repo names as `o/r`: section 6 first break item.
- `apprRepo` and `TestTrailerCheckKeepsExit` claims switch to `claimOn` with `x`: section 6 (a) or (b) item.
- `TestApproverFailsClosed` "older head" baseline gains `--branch x`; `TestTrailerCheckKeepsExit` baseline gains `--branch x`: section 6, builder's pick stated inline.
- `TestApproverInertWithoutApprovers` (i) both runs gain `--branch x`: section 6 names (ii), whose baseline `ob` is shared with (i), so (i)'s `a` run must change with it for `oa == ob` to hold. Required by the same behavior change.
- `TestApproverDegrades`: `bout` unchanged, event-read rows compare against `bout` minus the unknown-head line, the six no-event rows unchanged: exactly as section 6 prescribes.
- "branch name ignored stays": exit 0 to 1 with the 2.4 failure: section 6.
- `TestBindSources` "env" row switches to a same-repo `writeEvent` event with `t-031`: section 6. Flag rows unchanged under (b).
- `claimOn` added next to `claim` as the brief suggested. No `t.Skip` added.

All sixteen acceptance criteria have a test or were checked by review above. AC12's `t-041` and `t-0400` rows are covered by the pure `TestTicketBranchName` table rather than a gate run, with a comment explaining the binding conflict fires first; that is a faithful reading of the criterion.

## Notes (not must-fix)

- **`prHead` type** (`gate_verify.go:188-193`) is a new unexported type the lease rule did not enumerate. The brief asks for "one new field" on `scanInput` and "the head identity" as one field on `commitScan`, which cannot carry name, same-repo, repository and Actions flags without a struct, so the type is the only way to meet 2.3 as written. It also carries `repo` and `gha`, which the brief's three-part identity did not list but 2.4 needs. The queen may want to record it as the brief's home for the type rather than a widening.
- **`strings.EqualFold`** is Unicode simple folding, where the brief says ASCII. Claim entries are ASCII by validation, and the only other operands are GitHub's `full_name` fields, which GitHub writes and restricts to ASCII. An attacker would need to write the event payload, so this is a note under the threat bar.
- **AC2 PR body list:** the head under review is a single commit whose message is the subject and sign-off only. I could not see a PR body from this worktree, so the per-change section list is unverified here; the inline `// T-039 2.x` comments on each changed assertion cover the same ground. The queen should check the body carries the list before merge.
- **Hive effect** still applies as the brief states: the next hive PR runs this check from base while the pinned dispatch writes no branch, so hive PRs ride the Q-T039-1 (b) fallback with a warning until the pin move and renewals.

## Blocking Issues (Must-Fix Only)

None.

Verdict: pass