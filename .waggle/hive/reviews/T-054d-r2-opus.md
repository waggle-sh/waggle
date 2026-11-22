<!-- waggle-review v1 verdict=must-fix head=8577421dd68db3ad06017bc2d9072cf4923e7749 -->

Risk-Tier: 2

Reviewer: guard, opus, confirming guard for the fix round. Read only. Nothing pushed, commented, approved or merged; no file in the clone was edited. (One `git fetch -q origin main` ran in the clone to read main; it changed no tracked file, and origin/main stayed at 2cf9c11.)

## Intent Summary

The fix commit 8577421 answers the three round 1 must-fix items. `merged` in `cmd/waggle/quorum.go` no longer greps first-parent subjects for any mention of the task. It reads every non-merge commit reachable from main with its subject and `Waggle-Ticket` trailer, and a new `commitTask` binds each one by the gate's `ticketSubject` and `trailerTickets` (Head Keeper subjects and subject/trailer conflicts bind none). `waggle quorum veto` now refuses a `--decision` that does not match `^D-[0-9]{3,6}$`, with exit 2 before any git or signing work. `docs/quorum.md` fixes the Nectar ratio (4 times, not 16), says which commands take `--json`, and describes the veto ID form. Blast radius: the `quorum status --next` ready list and the veto argument check. Nothing outside `cmd/waggle` and `docs` changed.

## Blocking Issues (Must-Fix Only)

### 1. The veto ID check still accepts IDs that close can never match (D-0007, D-000, D-01000)

`cmd/waggle/quorum.go` `vetoDecision = ^D-[0-9]{3,6}$`. Close builds its set as `fmt.Sprintf("D-%03d", e.ID)` (`internal/quorum/close.go:50`), and `internal/decisions/entry.go:222` `parseDecisionID` accepts only that canonical form. So:

- `--decision D-0007` passes the new check and the library's `^D-[0-9]{1,6}$`, signs and appends a veto, prints "veto recorded" and exits 0. At close, `ids["D-0007"]` is false (the entry is `D-007`), so the veto is ignored and listed, after the round has closed. The veto record is on the append-only chain and cannot be corrected.
- `D-000` passes too; no entry has ID 0.
- `D-01000` passes; the entry would be `D-1000`.

This is the same defect class as round 1's item (a decision ID close can never match), and the round 1 fix asked for exactly that guarantee, with D-7 and D-07 as examples rather than the full set. The fix is one rule: accept the ID only when it parses to n >= 1 and `fmt.Sprintf("D-%03d", n)` equals the input (the `parseDecisionID` rule). Add `D-0007` and `D-000` cases to `TestQuorumExitCodes`, exit 2. Then reword the error and the doc sentence, since "D- and at least three digits" also describes D-0007: for example "as DECISIONS.md writes it: D-007, D-372, D-1000".

## Items confirmed fixed

### Round 1 opus item 1: ready-list merged check binds by the gate rule. Fixed.

- `commitTask` uses the gate's own `ticketSubject` and `trailerTickets` (same package), so the subject regex, trailer bounds (256 bytes, 8 values, 16 chars), dedup and canonicalization (`T-0101` to `T-101`) are the gate's, not a copy. No third copy of `bindTask` was made.
- Head Keeper subject binds none, with or without a trailer: matches `classify` (`gate_commits.go:316`) and `bindTask`.
- Subject and trailer naming different tickets binds none; a multi-ticket trailer binds none; a malformed trailer binds none. Matches the gate (conflict or bad trailer fails) and `bindTask`.
- Mentions (`Records: T-101 brief`, `T-102: build on the T-101 API`, `Merge pull request ... t-101-x`) and revert subjects bind nothing. The branch-name form is not read, as in `bindTask`, because main history holds no PR branch.
- Merge commits: excluded by `--no-merges`, and a PR merged with a merge commit binds through its side commits. TestQuorumMergedBinding covers this (T-101 via `--no-ff`).
- Failed git read: `err != nil` leaves `bound` empty, so nothing is ready. Covered by the `no-such-ref` case. The dependency ID is parsed with `comb.ParseTicketID`, so `T-0101` matches and `issue-7` does not.
- Deviation, non-blocking: the rule is "any non-merge commit reachable from main binds", not `mergeTask`'s "each first-parent commit binds as a unit". The two differ only for a merge whose side commits are mixed or include an unbound commit. The gate fails such a PR, so it can only land by admin bypass. I checked real history: on origin/main (2cf9c11) both rules give the same 71 bound tasks, with no task in one set and not the other (comparison script kept outside the repository). The code comment states the rule it uses, so this is acceptable. A later pass could reuse `mergeTask`'s unit rule if one is exported.
- Reverts: a reverted task's original commit still binds, so it still counts as merged. That is the gate rule as Q-T054-4 (a) states it, the same as the multi-phase caveat the brief already accepts. Not a regression.

### Round 1 opus item 2: weight sentence ratio. Fixed.

"A has 4 times B's Nectar but only twice B's weight" is correct: 400/100 = 4, 20/10 = 2. The RFC-0006 line is outside the lease and still carries 16; it needs a note for its next edit.

### Round 1 aws item: short veto IDs. Partly fixed (see Blocking Issue 1).

D-7 and D-07 are now refused with exit 2 before `openQuorum`, so no git or ssh-keygen work and no ref write. The two new `TestQuorumExitCodes` cases run before "veto ok" with a valid `--key` on an open round, so without the check they would have exited 0. They prove the check, not some other usage path.

### Doc edits

- `--json` sentence: "Every command except `veto` takes `--json`" matches the code (round 1 non-blocking note 4).
- Veto paragraph: accurate for D-7 and D-07; see Blocking Issue 1 for D-0007.

## Lease and regression checks

- Changed files: `cmd/waggle/quorum.go`, `cmd/waggle/quorum_merged_test.go` (new, SPDX header present), `cmd/waggle/quorum_test.go`, `docs/quorum.md`. All inside the lease. No `cmd/waggle/gate*.go`, `gate/`, `internal/dispatch`, `internal/nectar`, `.waggle/`, `.github/`, `go.mod` or `go.sum` change.
- No added line holds an em or en dash (count 0).
- `regexp` is still imported and used (`vetoDecision`); `slices` added. The old per-call `regexp.MustCompile` inside the closure is gone.
- No regression found in other commands: only `merged` and the veto argument check changed.

## Commands run (at 8577421)

- `go build ./...`: ok.
- `go vet ./...`: ok.
- `gofmt -l .`: empty.
- `go test -race -count=1 ./cmd/waggle/`: `ok github.com/waggle-sh/waggle/cmd/waggle 304.537s`.

Verdict: must-fix
