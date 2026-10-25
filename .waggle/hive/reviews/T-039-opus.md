<!-- waggle-review v1 verdict=pass head=8978c6265a0ac51cc41691b1b4a89f8d89bfd709 -->

Risk-Tier: 3

**Intent Summary:** T-039 closes the D-278 gap. Before this change, the gate matched a bound commit only against the claim of the ticket the commit named, so a PR on any branch could carry `T-040: ` commits and pass on T-040's live lease. Now:

- **Claim schema.** `comb.Lease` gains an optional `branches` list at lease version 2. `ValidateBranches` checks it: at most 4 entries, no duplicates, 255 bytes per entry, an ASCII branch-name rule plus the `<owner>/<repo>:<branch>` fork form, and errors that name an entry by position only. `validateLeaseFields` calls it, so every stored-claim read checks it again.
- **Claim writers.** `AcquireClaim` uses the given list, else keeps the stored list on a same-keeper renewal, else records `[req.Branch]`; a takeover gets a fresh list. The dispatch keepalive repeats `claim.Lease.Branches`. `ClaimTask` refuses a bad list as `usage` before `refuseHardcoded`. `waggle claim` gains a repeatable `--branch`, and its JSON body gains `branches`.
- **Gate.** `readPREvent` reads the head and base `repo.full_name`. `runGateRange` works out a `prHead` (name, head repo, sameRepo, gha) once and carries it in `scanInput`, so the approver rescan sees the same head. `claimFor` calls a new `checkHead`, which:
  - with an unknown head, fails each ticket under Actions and warns once locally;
  - matches plain entries byte for byte, same-repo only;
  - matches fork entries with the repo case-folded and the branch exact;
  - applies the Q-T039-1 (b) fallback only to same-repo, non-ignored, canonical `t-<d>` or `waggle/T-<d>` names.
  
  Each failure is a pathless `leases` finding that names only the ticket ID.
- **Blast radius.** The claim schema, every claim read and write, and the merge gate's lease check. Under (b), hive PRs keep passing on `t-<nnn>` branches through the fallback until the pin move.

**Checked against the brief:**
- **Fork rule (2.3).** `sameRepo` matches the brief exactly. With an event, the head repo must be set and equal to the base repo. With no event, it is true only off Actions or when the name came from `--branch`. So `pull_request_target`, an unreadable event and a null `head.repo` all count as forks.
- **Ignored names.** A name `branchTicket` ignores can never equal a valid entry, and the fallback checks for it explicitly.
- **Approver filter.** The failure has no path and sits outside the path loop, so the filter never drops it. `TestClaimNamesHead` confirms an approval at head does not clear it.
- **No echo.** Messages are built only from the ticket ID (AC14 by construction; the hostile-head test passes).
- **Claim writers keep the list.** The read-back is used by the keepalive, and the takeover reset is tested (AC5).
- **Unchanged parts.** `LeaseVersion` stays 2. `classify`, `branchTicket`, `Validate` and `checkBranch` are unchanged. `run.go` differs only in the `:409` literal.
- **Signals (D-193).** No new test sets `Timeout`, cancels a session or sends a signal; AC7 uses the `fix_test.go:81` shape.
- **Dashes and file set.** No em or en dash in any `.go` file. All 16 changed files are inside the lease rule.
- **Existing tests.** Every changed existing assertion matches a section 6 entry: `writeEvent`, `apprRepo`/`TestTrailerCheckKeepsExit` moved to `claimOn x`, the `--branch x` baselines, `TestApproverDegrades` minus the unknown-head line for the event-read rows, "branch name ignored stays" going to exit 1, and the `TestBindSources` "env" row moved to a same-repo event. No `t.Skip` was added.

**Commands (Head `8978c62`):** `go build ./...`, `go vet ./...` and `gofmt -l .` all printed nothing. `go test -race ./...` passed in every package; no test failed, push-related or otherwise.

**Blocking Issues (Must-Fix Only):** None.

**Notes (not blocking):**
1. **Lease wording.** The lease rule allows one new matching function after `claimFor`; the build adds two, `checkHead` and the helper `ticketBranchName`. It also adds a new type `prHead` in `gate_verify.go`. That type is implied by "one new field" carrying the head identity, and the split changes no behavior. The Keeper may want to read this as within intent.
2. **Case folding.** `strings.EqualFold` folds Unicode case (the Kelvin sign folds to `k`), but 2.3 and 2.4 say ASCII case folding. Both inputs come from GitHub's payload, which is trusted (D-203), and stored entries are ASCII-validated, so a PR author cannot exploit it. An ASCII-only fold would match the brief to the letter.
3. **AC12 gate rows.** The `t-0400` and `t-041` rows are covered only by `TestTicketBranchName`. As gate rows they bind another ticket and fail in `classify` with a conflict before the fallback is reached, and the test comment says so. The rows are not reachable as specified, so the deviation is justified.
4. **AC14 fork name.** There is no gate-level test with a hostile fork repo name. It is safe by construction, since no message is built from `h.repo`.
5. **Keepalive overwrite.** The keepalive overwrites the claim with dispatch's recorded list, so a human `--branch` renewal during a live dispatch run is reverted at the next keepalive. This is the behavior 2.2 specifies, and the same already happens for directories. Worth a line in the T-037 docs.
6. **AC2 PR body.** AC2 needs a PR body listing each changed assertion with its section. No PR exists yet, so the queen should check that the body lists the entries above.

Verdict: pass