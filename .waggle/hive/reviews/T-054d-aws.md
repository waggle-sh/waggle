<!-- waggle-review v1 verdict=must-fix head=058182c735111b67e9b12e8558afe5ba28663862 -->

Risk-Tier: 2

## Intent Summary

Phase D of T-054 adds the operator surface for quorum rounds. It adds `waggle vote` and `waggle quorum open|status|veto|close|verify` in `cmd/waggle/quorum.go`, and wires them into `main.go` and the usage text. It adds a `quorum:` section to `internal/config` (`credits` default 100, `round_days` default 14) and deletes the phase C stopgap `scanQuorumBlock` from `internal/quorum/round.go`. It exports `CheckVotes` from the quorum store for `--dry-run`, and adds `docs/quorum.md`. Blast radius is the CLI plus config parsing. A config with a `quorum:` block now goes through the strict parser instead of a lenient line scan.

Reviewed at head 058182c735111b67e9b12e8558afe5ba28663862, in a clean worktree at that exact commit.

## Verification run (no -race: no gcc in this image)

- `go build ./...` ok.
- `go vet ./...` ok.
- `gofmt -l .` prints nothing.
- `go test ./internal/quorum/... ./internal/config/... ./cmd/waggle/` passes: quorum ok, config ok, cmd/waggle ok.
- The ssh-keygen signature tests **ran, not skipped**. `/usr/bin/ssh-keygen` is present. With `-v`, `TestRealSSHSignatures`, `TestRealSSHCraftedSignaturesAreBad`, `TestSignedApprovalPassesProtectedClasses` and `TestQuorumEndToEnd` all report PASS. I saw no SKIP lines in the filtered output.

## Blocking Issues (Must-Fix)

1. **`--decision D-1` is accepted and writes a veto that can never apply, with no error.**
   - `decisionPattern` in `internal/quorum/round.go:64` is `^D-[0-9]{1,6}$`, so `D-7` passes `Veto`.
   - Decision IDs are canonical `D-%03d`: `internal/quorum/close.go:50` builds the set as `fmt.Sprintf("D-%03d", ...)`, and `internal/decisions/entry.go:222` accepts only that form.
   - The CLI (`cmd/waggle/quorum.go`, `quorumVeto`) neither normalizes nor checks the ID.
   - The Head Keeper runs `waggle quorum veto --round 1 --item T-102 --decision D-7`. It signs, prints "veto recorded" and exits 0. At close the cited `D-7` is not in the `D-007` set, so the veto is ignored and listed. That surfaces only after the round closes.
   - The veto record sits on the append-only round chain, so the mistake cannot be corrected.
   - The CLI help says "such as D-001, three digits or more", and the code does not enforce that.
   - Fix: require `^D-[0-9]{3,}$` in the CLI or in `Veto`. Alternatively normalize with `%03d`, which must be applied identically at tally time. Add a test for `D-7` and `D-07`.
   - Check that this does not break the phase C tests that use short IDs.

## Non-blocking notes

- `--round_days` overflow: `quorumOpen` computes `time.Duration(bc.Quorum.RoundDays) * 24 * time.Hour`. A `round_days` above about 106,751 wraps the duration. The library probably refuses a close time in the past, so it should fail closed. A sane upper bound in `config.validate` (for example 3650) would remove the question.
- Behavior change from deleting `scanQuorumBlock`: the old scan ignored unknown keys under `quorum:`. The new parser rejects them with "unknown key". Defaults (100 and 14), positive-integer refusals, and the quorum-block base read all match. It is stricter and arguably correct. Mention it in the PR body.
- `keyPath` rejects any `--key` beginning with `ssh-`, including a relative file named `ssh-foo`. It is rare, and `./ssh-foo` works. Worth a line in the help.
- `vote --now` lets a keeper pass a past time to get through the CLI's close-time check. The ballot is still ignored or flagged by `verify` once the tally exists, and the docs say the closer's fetch is the deadline. It is a replay and test flag, so I judge it acceptable.

## Checks asked for in the brief (all pass except the item above)

- **Lease:** the changed files are only `cmd/waggle/{main,quorum,quorum_test}.go`, `docs/quorum.md`, `internal/config/*` and `internal/quorum/*`. `internal/nectar`, `internal/comb`, `internal/layout`, `cmd/waggle/gate*.go`, `gate/`, `.waggle/`, `.github/`, `go.mod` and `go.sum` are untouched.
- **Ref writes:** the CLI writes refs only through library code (`Lifecycle.Open`/`Veto`/`Close` and `Store.Cast`). It makes no direct `update-ref` or push calls. Its own git calls are read-only: `config --get` and `log --first-parent`.
- **Nectar scoring:** `scoreSource.Snapshot` scores at the round's pinned `MainSHA`, `ReceiptsSHA`, `RulesVersion` and `Now`, parsed from `nectar_at`. It does not use the wall clock.
- **Clock:** `statusClock` is read only in `cmd/waggle`. The library has no clock, and `close` checks the close time in the CLI.
- **Key and keeper resolution:** `--key`, else `user.signingkey` when `gpg.format=ssh`, else a usage error (exit 2). Literal keys are refused and `~/` is expanded. `keeper_id` comes from `config.LoadKeeperID` (`.waggle/local.yaml`).
- **Exit codes:** `quorumCode` maps usage to 2, the check and validation sentinels plus `ReasonError` to 1, and everything else (git, tool, file) to 3. `TestQuorumExitCodes` passes. `--json` goes through `printJSON`, one object on stdout. Warnings and errors go to stderr.
- **Help text:** `TestQuorumHelpPlainTerms` runs `checkPlain` over `vote -h` and every `quorum <sub> -h`, and passes. The `main.go` usage lines are plain.
- **docs/quorum.md:** accurate against the code on credits, square-root weight, the round-days default, exit codes, key resolution, the registry line format and the veto flow. The worked example numbers add up: 20*6+10*10=220, 20*8+5*3=175, 5*9=45. One inaccuracy: it describes the veto ID loosely, and should say `D-` followed by at least three digits once fix 1 lands.

Verdict: must-fix

