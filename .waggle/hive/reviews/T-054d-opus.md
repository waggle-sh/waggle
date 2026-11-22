<!-- waggle-review v1 verdict=must-fix head=058182c735111b67e9b12e8558afe5ba28663862 -->

Risk-Tier: 2

Reviewer: guard, opus. Read only. Nothing pushed, commented, approved or merged; no file in the clone was edited.

## Intent Summary

Phase D adds the user-facing surface over the phase A to C2 library. `internal/config` now parses an optional `quorum:` block (`credits`, `round_days`, defaults 100 and 14, unknown key is a load error with its line). `internal/quorum/round.go` drops its private `scanQuorumBlock` and reads the parsed config instead. `store.go` exports `CheckVotes` so `vote --dry-run` can validate with no git or ssh-keygen call. The new `cmd/waggle/quorum.go` adds `waggle vote` and `waggle quorum open|status|veto|close|verify`, a `NectarSource` adapter over `internal/nectar.Score` and a `merged` check for the ready list. The new `docs/quorum.md` is the user doc. Blast radius: two new top-level commands and one new config section. The gate, the claim paths, Nectar and the comb are untouched. Nothing changes in this hive until a pin move and a committed registry.

## Blocking Issues (Must-Fix Only)

### 1. The ready-list `merged` check does not use the binding rule D-372 adopted (Q-T054-4 (a))

`cmd/waggle/quorum.go:262-269`. Q-T054-4 (a), adopted by D-372, says a dependency is met when a first-parent commit on main **binds** to it. In this codebase binding has a fixed meaning: a `T-NNN: ` subject prefix or a `Waggle-Ticket` trailer, with `Head Keeper: ` subjects binding nothing (`internal/dispatch/merged.go:61`, copied as `internal/nectar/bind.go:28`). The CLI uses its own rule instead: any first-parent subject that contains `t-0*<n>` between non-alphanumerics, case-insensitive.

Concrete false positives for dependency T-101:
- `Revert "T-101: add x"` counts as merged.
- A records or docs subject such as `Records: T-101 brief, T-102 status` or `T-102: build on the T-101 API` counts as merged.
- A `Head Keeper: ... T-101` subject counts, though the gate binds none.

So `waggle quorum status --next` can list an item as ready while its dependency has only been mentioned or reverted. That is the "what gets picked up next" half of RFC 4.3. No test reaches this code: the D2 fixture's T-101 to T-103 have no task briefs, so `round.Depends` is empty and `merged` is never called.

Fix inside the lease: bind first-parent subjects and trailers by the gate rule. One way is a `--format=%s%x1f%(trailers:key=Waggle-Ticket,...)` log with the `bindTask` rule. A third copy needs a "keep the same" comment like `nectar/bind.go`'s. If the builder judges a copy wrong, the alternative is to stop and report, since `internal/dispatch` and `internal/nectar` are outside the lease. Add a D2 or unit case where a dependency is only mentioned or reverted (not ready) and where it is bound (ready).

### 2. `docs/quorum.md` states the weight rule wrongly

`docs/quorum.md:22`: "A has 16 times B's Nectar but only twice B's weight." In the example A has 400 and B has 100. That is 4 times the Nectar for twice the weight (20 against 10). The 16-to-4 ratio is A against C (400 against 25, weight 20 against 5). Brief 2.4 item 5 asks for an accurate plain doc of exactly this rule. The sentence is copied from RFC-0006 section 5.6 line 140, which carries the same error. Fix the doc in this PR, for example "A has 4 times B's Nectar but only twice B's weight". The RFC line is outside the lease, so it gets a note for its next edit (like F2).

## Checks that passed

- **Lease.** Changed files: `cmd/waggle/{main.go,quorum.go,quorum_test.go}`, `docs/quorum.md`, `internal/config/{config.go,config_test.go,quorum_test.go}`, `internal/quorum/{round.go,store.go,lifecycle_test.go}`. Nothing in `internal/nectar`, `internal/comb`, `internal/layout`, `cmd/waggle/gate*.go`, `gate/`, `.waggle/`, `.github/`, `go.mod` or `go.sum`. SPDX headers are present on all three new files. No added line holds an em or en dash (perl scan of `+` lines: 0).
- **scanQuorumBlock removal.** `readBase` already ran `config.Parse` on the base config, so it now takes `cfg.Quorum` from the same parse. Defaults are the same (100, 14). Zero, negative and non-integer values are still refused, now as `ErrInvalidConfig` with the line. Inline `# comment` is handled by the subset parser's `stripComment`. Two cases are stricter, as D1 asks: an unknown key under `quorum:` (silently ignored before) and a non-mapping `quorum: yes` (defaults before). No real round exists, so no stored base config can be newly refused. The `quorum.DefaultCredits` and `DefaultRoundDays` constants (`round.go:36-37`) are now unused duplicates of the config defaults (nit).
- **Ref writes.** The CLI writes only through `Lifecycle.Open`, `Lifecycle.Veto`, `Lifecycle.Close` and `Store.Cast`. Its only direct backend calls are reads: `FetchObjectNamespace`, `ListObjectRefs`, `ReadBlobAt`, `ReadReceipts`. It has no `update-ref`, no claim write and no audit-ref write.
- **Nectar pin.** `scoreSource.Snapshot` passes `nectar_at.main_sha`, `receipts_sha`, `rules_version` and `now` straight into `nectar.Input` and nothing live (`quorum.go:46-57`). `Open` pins `MainSHA = Base = ResolveBase(main)`; `receipts_sha` and `rules_version` (1) are set by the library. D3 proves the adapter bytes equal `Score`'s for the same pins and that `ParseNectar` reads the real 9.2 output.
- **--key.** `--key`, else `git config user.signingkey` when `gpg.format` is `ssh`, else exit 2. A literal `key::` or `ssh-` value is refused, and `~/` is expanded. This matches Q-T054-5 (a). D2 exercises both the flag and the git-config path.
- **keeper_id.** `config.LoadKeeperID` reads layout.Local, as claim's default does (D-016). The placeholder is refused with a remote.
- **Exit codes and --json.** Each command has 0, 1, 2 and 3 cases in D4. Library sentinels and `ReasonError` map to 1, usage to 2, everything else to 3. With `--json`, a success prints exactly one JSON line. A failure prints nothing on stdout, and warnings and errors go to stderr.
- **Clock.** `internal/quorum` has no `time.Now`, `time.Since` or `time.Until` outside tests. The CLI reads `statusClock` only, and `close` checks `closes_at` against it.
- **Help text.** `vote -h`, `quorum -h` and each subcommand's `-h` pass `checkPlain` (D5). The two usage lines match the brief's suggestions, and `TestPrintUsagePlainTerms` passes.
- **Veto decision ID.** The library checks the `^D-[0-9]{1,6}$` shape at write. Existence is checked at close against `DECISIONS.md` at the close commit. A veto without its entry is ignored and listed. This matches 2.3 item 2 and the section 7 must-fix.
- **Budget.** About 700 net added non-test lines, under the phase D target of 845. No cut is needed.

## Commands run (at 058182c)

- `go build ./...`: ok.
- `go vet ./...`: ok.
- `gofmt -l .`: empty.
- `go test -race -count=1 -v ./internal/quorum/... ./internal/config/... ./cmd/waggle/`: exit 0.
  - `ok internal/quorum 484.7s`
  - `ok internal/config 1.3s`
  - `ok cmd/waggle 223.9s`
- **ssh-keygen tests ran; none skipped.** `/usr/bin/ssh-keygen` is OpenSSH_10.3p1. The log has 0 `--- SKIP` lines. `TestRealSSHSignatures`, `TestRealSSHCraftedSignaturesAreBad`, `TestSignThenWrite`, `TestC4VerifyFails`, `TestVerifyReceipt`, `TestQuorumEndToEnd`, `TestQuorumExitCodes` (all 28 subtests) and `TestQuorumNectarParity` all PASS. `qKey` calls `t.Fatalf` when ssh-keygen is missing; it does not skip.

## Non-blocking notes

1. `--now` is accepted on a real `vote`, not only `vote --dry-run` (brief 2.4 item 3). A voter can backdate past the CLI's close-time check. This is not a security gap: anyone who can push the refs can write a ballot, and the closer's fetch is the deadline. Suggest refusing `--now` without `--dry-run`.
2. `--decision D-1` is accepted and reported as "veto recorded", but `DECISIONS.md` IDs are `D-001` form, so close will ignore it. The flag help says "three digits or more" without enforcing it. Suggest the CLI refuse non-canonical IDs, or warn when the ID is not in `DECISIONS.md` at main.
3. A missing or bad `keeper_id` exits 3. Claim exits 2 for a missing owner. Either is defensible; say which in the doc.
4. `docs/quorum.md:31` says every command takes `--json` where it prints a result, but `veto` prints a result and has no `--json`. The brief's veto line has none either, so fix the sentence.
5. The help text and the doc say `.waggle/local.yaml`, but the file is `.waggle/control/local.yaml` (`layout.Local`). This repeats the existing claim help text; fix both in one later pass.
6. D3 checks that the adapter matches `Score` and that one Keeper parses. It does not reproduce the section 6 fixture values (400, 100, 25, 14.25), which the brief's wording implies. The format-drift risk is covered and value drift is not.
7. `printJSON` drops a `json.Marshal` error. It cannot fail with these types today.

Verdict: must-fix
