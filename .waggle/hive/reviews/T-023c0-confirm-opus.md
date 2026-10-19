<!-- SPDX-License-Identifier: MIT-0 -->
# T-023c0 confirmation review (opus): fix commit afcc9c3

Verdict: pass
Risk-Tier: 2

Reviewed: `git diff e71d298 afcc9c3` (PR #49, branch `t-023c0`), plus the full PR against origin/main (768e03d).

## Checks

- Lease: the PR touches only `internal/runtime/launch.go` and `internal/runtime/launch_test.go`. OK.
- Size (D-136, added non-test lines): 64 per `git diff --numstat` on `launch.go` (60 excluding lines that start with `++`), against estimate 60, stop 130. OK; matches the builder's 64.
- `Signed-off-by` on both commits (e71d298, afcc9c3). OK.
- No em or en dashes in the PR diff or in either commit message. OK.
- `go build ./...`, `go vet ./...`: clean. `go test -race -count=1 ./internal/runtime/`: ok.
- `product-ci` (linux) on head afcc9c3: pass (1m28s).
- `verify-leases-and-tiers`: fail with `leases: no claim for T-023; claims must stay until merge` (tiers ok, decisions unchanged). Same failure, same message at e71d298, so not a regression from this fix and not a code defect. The T-023 claim ref has to exist before merge; this is queen or Keeper bookkeeping, not a Builder fix.

## Probes

1. Mutation test. Replacing the `slices.Clone` line in `Verify` with a comment makes `TestDefaultArgsMutatedAfterVerify` fail ("Prepare used the mutated slice", argv ends `--max-budget-usd=3`). With the clone restored it passes. The test is a real guard, and its fake help text (`--max-budget-usd` renamed to `--max-cost`) is the right setup: the mutated flag is allowlisted, so only the help check would have stopped it.
2. Comment truth. Every listed gap is accurate: bare relative paths match no prefix (a worker symlink reached by a relative name included); scp forms without `git@` match nothing; prefix matching in `pushInsteadOf` is case-sensitive; `<transport>::<address>` is not URL-rewritten; `remote.<name>.pushurl` is not subject to `pushInsteadOf`; the longest matching `insteadOf` wins regardless of source, so a longer one from `git -c` or the Keeper's global config (HOME is carried) overrides. "Best effort, defense in depth behind the Bash deny rules and 7.2 rule 4" is accurate. The list is incomplete (N2) but nothing in it is false, and the framing no longer claims full coverage.
3. Clone coverage. Production reads of `c.cfg.DefaultArgs` are `Verify` (help check), `claudeArgs` (allowlist loop and append), and `Prepare` (`cfg := c.cfg`, used for other fields). `Prepare` refuses before `verified` is set, and the clone is written before `verified.Store(true)`, so every post-Verify `Prepare` reads the clone, with the atomic giving the happens-before edge. One alias remains through `Config()` (N1).
4. Regressions from e71d298. None. New env indexes (`GIT_CONFIG_COUNT=10`, value 5 `git+ssh://`, 6 `ssh+git://`, 8 `/`, 9 `.`) match the slice order and the test. `git+ssh://` and `ssh+git://` do not collide with `ssh://` because prefix matching is anchored at the start. `slices.Clone(nil)` returns nil, so the empty default_args path is unchanged.

## Blocking Issues

None.

## Non-blocking

- N1 (`Config()` alias). `Config()` returns `c.cfg` by value, but `DefaultArgs` still shares the cloned backing array, so `a.Config().DefaultArgs[0] = "--max-budget-usd=3"` after `Verify` skips the help check exactly as the caller-slice mutation did. No production caller of `Config()` exists today, and the `claudeArgs` allowlist still runs at `Prepare`, so this is latent. Cheapest fix: clone in `Config()` too, or move the clone to the top of `Verify` (before the help loop), which also closes the small window where a concurrent writer could change the slice between the check and the clone. A related side effect: `Verify` now writes `c.cfg`, so a `Config()` call concurrent with `Verify` is a data race where before it was read-only. Not reachable from current callers.
- N2 (tripwire comment completeness). The list omits the largest bypass: any process that clears or overrides the `GIT_CONFIG_*` variables, for example a worker-written test run through the allowed `go test`, which can exec `git push` with its own environment. The deny rules do not see that path either; only 7.2 rule 4 and the config hash do. One added clause in the comment ("or a process, such as a test run by go test, that clears or overrides the GIT_CONFIG_* variables") would make the list honest about the dominant gap. Suggest carrying it into phase C rather than another C0 fix round.
- Gate: the `verify-leases-and-tiers` failure needs the T-023 claim restored before merge (see Checks).

## Intent Packet

Afcc9c3 is the guard fix round for T-023 phase C0. It adds `git+ssh://` and `ssh+git://` to the push tripwire's `pushInsteadOf` prefixes (GIT_CONFIG_COUNT goes from 8 to 10) and rewrites the `pushBlocked` doc comment from a claim of full coverage to an explicit best-effort list of what it misses. It also makes `ClaudeAdapter.Verify` replace `cfg.DefaultArgs` with a private clone once the help check passes, so a caller that mutates its own slice after `Verify` cannot slip an unverified flag into `Prepare`; a new test proves this and fails when the clone is removed. Blast radius is limited to worker launch argv and env in `internal/runtime`: the env gains two `GIT_CONFIG_*` pairs, and adapter behavior is otherwise unchanged. Size is 64 of 130 added non-test lines. Product CI passes; the lease gate fails only because the T-023 claim ref is missing, as it did at e71d298.
