<!-- waggle-review v1 verdict=pass head=2e627830ff1514bfad954fafe7acbd7ddd3e55ff -->

Risk-Tier: 2

## Intent Summary
Confirming round 3 over 058182c..2e62783. Only `cmd/waggle/quorum.go`, `cmd/waggle/quorum_merged_test.go`, `cmd/waggle/quorum_test.go` and `docs/quorum.md` changed. No gate file, `.waggle/control` file or `gate/` file was touched. The ready list's `merged` check now binds commits by the gate's rule. The docs Nectar ratio is corrected, and `quorum veto` now rejects any `--decision` that is not in the canonical form. Blast radius is the `waggle quorum` CLI only.

## Blocking Issues (Must-Fix Only)
None.

## Confirmation of fixes
1. **Merged binding.** `merged` now runs `git log --no-merges` with subject and `Waggle-Ticket` trailer. It binds through `ticketSubject` and `trailerTickets` (same package as the gate) and `comb.ParseTicketID`. A `Head Keeper: ` subject binds nothing. A subject and trailer naming different tickets bind nothing. A malformed trailer binds nothing. A git error means nothing is merged, so no item becomes ready on a bad read. Mentions, revert subjects and `Merge pull request ... t-101-x` subjects no longer bind. A PR merge commit binds through its side commits, which `--no-merges` reaches. `T-0101` matches `T-101` via `ParseTicketID`. This matches `internal/dispatch/merged.go` mergeTask for every well-formed case. It differs only in edge cases the gate already rejects before merge: a PR whose side commits mix tasks, or has an unbound side commit, binds per commit here and binds nothing in mergeTask. Non-blocking.
2. **docs/quorum.md.** 400/100 gives 4 times the Nectar, and sqrt gives 2 times the weight, so "4 times" is correct. The `--json` sentence now says `veto` takes none, and the veto section names the canonical form.
3. **Veto IDs.** `canonicalDecision` requires the `D-` prefix, at most 6 digits, `n >= 1` and `Sprintf("D-%03d", n) == id`. This is the same rule as `internal/decisions` `parseDecisionID`/`digits` (1 to 6 digits, `n >= 1`, `formatID` round trip). It is also what `internal/quorum/close.go` looks up (`D-%03d` of `e.ID`). D-0007, D-000, D-01000, D-7, D-07 and D-+01 are rejected with exit 2, before the repo is opened. D-001, D-372 and D-1000 pass.

## Tests
- `TestCommitTask`: covers subject, trailer, a conflict between them, a bad trailer, revert, mention, Head Keeper, merge subject and empty subject.
- `TestQuorumMergedBinding`: a real repo with a no-ff PR merge, a trailer-only commit, mentions, a revert, Head Keeper and a subject/trailer conflict. It also covers a bad ref, which gives false.
- `TestCanonicalDecision`: covers the padded, zero, signed, exponent, lowercase, empty and 7-digit cases.
- The new veto cases in `TestQuorumExitCodes` pass: short, two-digit, padded, zero and signed IDs give exit 2, and `D-001` still gives 0.

## Commands run at 2e62783
- `go build ./...`: ok
- `go vet ./...`: ok
- `gofmt -l .`: empty
- `go test ./cmd/waggle/`: ok (25s). No `-race`, as there is no gcc on this image.

Verdict: pass

