<!-- SPDX-License-Identifier: MIT-0 -->
# Guard re-review: T-023 phase C (PR #50), fable, fix round

Verdict: pass
Risk-Tier: 3

PR #50, branch `t-023c`, head `22ea50a` (fix commit on top of `05343b4`), base `23bdf75`. Second independent guard (fable); no coordination with the other model. Reviewed `git diff 05343b4 22ea50a` in full and spot-checked the whole PR against `origin/main`.

## Intent Summary

The fix round closes the one class of defect both first-round reviews found: host git calls that ran after the agent could write without being pinned or gated, and a push that re-resolved the branch after the checks. `ck.run` now resolves `refs/heads/<branch>` once, checks that OID, records it as `head_oid` and pushes `<oid>:refs/heads/<branch>`, so a ref move after the check cannot change what is pushed. The remote-mode final claim sync and every keepalive renewal are gated on a fresh `hostFiles` comparison and skipped when any hashed file changed; when the sync is skipped or fails, other tickets' claim mirrors lose their remote-mode exemption. The dispatch-built Comb backend gets `core.hooksPath=/dev/null` through `GIT_CONFIG_*` env, so claim writes run no hook even under a Keeper's pre-existing hooks. `RunIn` pins `log.showSignature=false`, the three status calls pass `--ignore-submodules=dirty`, the XDG `git/ignore` and `git/attributes` files join the hashed set, `result.json` and `final.md` go through an O_EXCL temp file and rename, `TimedOut` keys on the deadline alone, the dash literals are Go escapes, and four `not_enforced` lines name the residuals. 59 added non-test lines in the fix round, 1223 total against the D-169 stop of 1350 (127 left). Every file is inside the phase C lease, sign-off present, no en or em dash anywhere in the diff or the commit messages, `go build`, `go vet`, gofmt and `go test -race ./...` clean locally (darwin, git 2.54, go 1.27).

Blast radius is unchanged from the first round: this is the code that launches every agent and holds the Keeper's push credentials on the host. After this round, every host git call that runs after the agent can write is either pinned or gated on the files check (table below); the only unpinned and ungated calls are the next run's pre-launch calls, reachable only by a process that escaped the worker's group after a clean pass, which `not_enforced` now states. That is a Cell-level limitation, consistent with what both first-round reviews asked for (disclosure, not a fix), and I keep it as a note with a cheap phase D mitigation.

## First-round blockers: fixed or not

Fable first-round review (B1 to B4) and opus first-round review (B1 to B4), as merged by the queen into fixes 1 to 11.

- **Fable B1 / opus B1, push re-resolves the branch (fix 1): fixed.** `check.go` lines 221 to 226 resolve the OID once with `rev-parse --verify` and build the `log` range from it; `run.go` line 300 records it and line 309 pushes `ck.head+":"+ref`. Re-probe R-A (20 runs, setsid survivor flipping the branch between a checked tip and an unchecked child): 0 of 20 pushed anything (every run caught either the outside-lease commit or the dirty tree), `head_oid` never differed from the remote tip, no evil commit on the remote. First round was 1 of 12. Regression test `TestFixPushesCheckedOID` kills both mutations (push the ref; re-resolve `head_oid`).
- **Fable B2 / opus B2a, final claim sync under planted config (fix 2): fixed.** `run.go` lines 289 to 297. Re-probe R-B: planted `remote.origin.uploadpack`, `core.sshCommand` with an scp-style url, `ext::` transport with `protocol.ext.allow`, `core.fsmonitor`, and `credential.helper` with an http url: every canary silent, every run exit 1 unpushed with `claim_sync_skipped: host files changed`, worktree tainted. `TestFixClaimSyncSkipped` kills the ungated-sync mutation.
- **Opus B2b, keepalive runs the planted `reference-transaction` hook (fix 3): fixed.** `run.go` line 223. Re-probe R-C local-mode case: hook planted at session start, renewals every 200 ms, canary silent (the canary checks a host-only env marker so the worker's own git cannot trip it). `TestFixCombRunsNoHooks` kills the mutation (and shows the initial claim write runs no hook either).
- **Opus B2c / fable N6, keepalive renewals under planted repo config (fix 4): fixed.** `run.go` lines 271 to 276. Re-probe R-C remote-mode cases (uploadpack, sshCommand, fsmonitor planted at session start): every renewal skipped with `claim_renew_failed: host files changed; renewal skipped`, host canary silent. `TestFixKeepaliveGate` kills the mutation. The residual (config written and reverted between the hash and the renewal) is in `not_enforced`.
- **Opus B3 / fable N5, XDG global ignore hides writes (fix 5): fixed.** `check.go` lines 114 to 127. Re-probe R-F: `*` in the XDG ignore plus untracked files in both the worktree and the main checkout: exit 1, `host_state global:.../git/ignore`, not pushed, worktree kept. Re-probe R-G: XDG attributes naming a filter plus a config filter command: `host_state` on both files, canary silent. `TestFixGlobalIgnore` kills the mutation.
- **Fable B3, `log.showSignature` runs `gpg.program` (fix 6): fixed.** `worktree.go` line 52. Re-probe R-D: canary silent, wrapper log empty. `TestFixShowSignature` kills the mutation.
- **Fable B4, status descends into a staged nested repo (fix 7): fixed.** Three calls (`check.go` lines 66 and 213, `worktree.go` line 402). Re-probe R-E (worktree) and R-E2 (main checkout): canary silent, the staged gitlink still fails the dirty rule (worktree) and the main-state rule (main checkout). `TestFixNestedRepoFilter` kills the worktree-status mutation only; the main-checkout and reuse-path mutations survive (see N3).
- **Opus N2, `result.json` and `final.md` follow a planted symlink (fix 8): fixed.** `result.go` lines 81 to 94; rename replaces a symlink or FIFO instead of following it. `TestFixResultSymlink` kills both mutations.
- **Opus N7, `TimedOut` requires `interrupted` (fix 9): fixed.** `run.go` line 492. `TestFixTimeoutWithErrorResult` kills the mutation.
- **Opus B4, literal dashes in a test (fix 10): fixed.** `cmd/waggle/dispatch_test.go` line 107 spells them as the Go escapes backslash-u2013 and backslash-u2014; no U+2013 or U+2014 remains in the diff or commit messages.
- **Opus B3 disclosure, opus N6, opus N11, fable N6 (fix 11): fixed.** `cell.go` lines 43 to 46 add the four lines; the happy-path test asserts 11 entries.

## Git-call table: every git invocation after the agent can write

"Pinned" is `RunIn` with `-c core.hooksPath=/dev/null -c core.fsmonitor=false -c log.showSignature=false` plus `pinnedEnv` (config and exec-path variables dropped, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, `GIT_ATTR_NOSYSTEM=1`, `GIT_NO_REPLACE_OBJECTS=1`): the only config that can name a program is the repo config, `info/attributes`, the admin dir files and the XDG ignore and attributes files, all hashed. "Gated" means the call runs only after a fresh `hostFiles` comparison found no change, or only on exit 0 (which implies no `host_state` violation).

During the session:

| Call | Where | Pinned | Gated | Programs reachable | Status |
|---|---|---|---|---|---|
| Comb `AcquireLease` (keepalive): `for-each-ref`, `cat-file --batch-check`, `cat-file --batch`, `hash-object -w --stdin`, `mktree`, `commit-tree --no-gpg-sign`, `update-ref` | `run.go` 271 to 276 | hooks off only (`GIT_CONFIG_*` env); Keeper env and global config kept on purpose | yes, files compared before every renewal | none of these run a filter, gpg or fsmonitor | ok |
| Comb keepalive, remote mode: `fetch --prune --no-tags`, `push --atomic --force-with-lease`, `update-ref` | same | hooks off only | yes | uploadpack, receivepack, sshCommand, credential.helper, fsmonitor, ext::, insteadOf: all from hashed config | ok; residual hash-to-call race in `not_enforced` |

After the session:

| Call | Where | Pinned | Gated | Programs reachable | Status |
|---|---|---|---|---|---|
| Comb `ClaimOID` final sync, remote mode: `fetch --prune --no-tags`, `for-each-ref` | `run.go` 289 to 297 | hooks off only | yes, files compared first; skipped on change; strict claim compare when skipped | as above, from hashed config | ok (R-B, R-K) |
| `rev-parse --git-common-dir` (admin pin) | `check.go` 188 | yes | no | none | ok |
| `for-each-ref` (ref diff) | `check.go` 199 | yes | no | none | ok |
| `status --porcelain=v1 --ignore-submodules=dirty -b -z --untracked-files=all` and `rev-parse HEAD` (main checkout) | `check.go` 66 to 67, 210 | yes | yes (`inspect` false when config, admin or `info/` changed) | clean filters from hashed config only; no descent | ok (R-E2) |
| `status --porcelain=v1 --ignore-submodules=dirty -z --untracked-files=all` with `--git-dir`/`--work-tree` (worktree) | `check.go` 213 | yes | yes (`inspect`) | same | ok (R-E) |
| `rev-parse --verify refs/heads/<branch>` | `check.go` 221 | yes | no | none | ok |
| `log --reverse -z -n 501 --format=... <base>..<oid>` | `check.go` 226 | yes, signature off | no | none with showSignature off (`%(trailers)` parses only) | ok (R-D) |
| `diff-tree -r -z --no-renames --raw --root --no-commit-id <oid>` per commit | `check.go` 264 | yes | no | none with `--raw` (no textconv, no external diff) | ok |
| `push <remote> <oid>:refs/heads/<branch>` | `run.go` 309 | the three `-c` flags; Keeper env and global config kept on purpose (credentials) | yes, exit 0 only | receivepack, sshCommand, credential.helper, push.gpgSign plus gpg.program, insteadOf: hashed repo config, clean by the gate; the rest is the Keeper's own | ok (R-H) |
| `rev-parse --git-common-dir` and `worktree remove <path>` | `worktree.go` 415 to 428 | yes (`remove` spawns an inner `status --ignore-submodules=none` child that inherits the `-c` flags) | yes, exit 0 only; a staged gitlink cannot reach it (dirty rule), a committed one makes `remove` refuse | clean filters from hashed config only | ok |

Before the next run (not this run's agent, only a process that escaped the group after a clean pass): `fetch <remote>` in `ResolveBase`, `rev-parse`, `ls-tree`, `cat-file`, `merge-base`, `worktree prune`, `worktree list`, `worktree add` (all unpinned `g`, hooks and fsmonitor off by `-c`, repo config honored, Keeper credentials present), the pinned reuse `status`, and the Comb claim calls (hooks off). Re-probe R-J: a survivor plants `hooks/reference-transaction` after a clean run (canary silent in the next run: hooks are off everywhere), and a planted `remote.origin.uploadpack` does run in the next run's pre-launch fetch. This is the `not_enforced` line about processes leaving the group, accepted by both first-round reviews as a Cell-level limitation; see N1 for a cheap phase D mitigation.

No remaining call after the agent can write is both unpinned and ungated. No blocker.

## Probes (uncommitted `internal/dispatch/probe_refable_test.go` in my worktree; not for merge)

- R-A push race, 20 runs: 0 pushed, 0 evil on remote, 0 `head_oid` mismatches.
- R-B final sync under planted uploadpack, sshCommand, ext::, fsmonitor, credential.helper: all canaries silent, all exit 1 unpushed, tainted.
- R-C keepalive under the same plants plus a local-mode `reference-transaction` hook, renewals every 200 ms, host-only env marker in the canary: all silent. (A first attempt without the marker showed the fsmonitor canary firing from the worker's own `git add`; that is the agent running its own config inside its own process and not a host call.)
- R-D `log.showSignature` plus `gpg.program`: silent.
- R-E and R-E2 staged nested repo with a clean filter, in the worktree and in the main checkout: silent, both caught.
- R-F XDG ignore, R-G XDG attributes plus filter, R-H `push.gpgSign` plus `gpg.program`, R-I branch turned into a symref to `origin/main` (caught dirty, not pushed): all clean.
- R-K another ticket's claim mirror rewritten plus a config change (sync skipped): reported as `host_state refs/waggle/claims/ticket-24`; with clean config the sync overwrites the mirror and the run passes, which is the intended remote-authoritative behavior.

Mutation results for `fix_test.go`: 13 of 16 mutations killed. Survivors: M2b (`synced` flag dropped: no test rewrites another ticket's mirror with the sync skipped; R-K shows the code is right), M7b and M7c (`--ignore-submodules=dirty` removed from the main-checkout status and from the reuse `checkClean`: `TestFixNestedRepoFilter` only covers the worktree status; R-E2 shows the main-checkout behavior is right). See N3.

## Checks

- Lease: every changed file is under `internal/dispatch/`, `internal/config/local.go` and its test, or `cmd/waggle/`; the fix round touched only `internal/dispatch/` and one test literal in `cmd/waggle/`. No `.waggle/control/**`, `go.mod` or `go.sum` change; stdlib only.
- Size: 1223 added non-test lines against `origin/main` (matches the PR), 59 of them in the fix round; 127 left under the D-169 stop.
- Sign-off: all four commits carry `Signed-off-by` under the Keeper's git identity with the `T-023:` subject prefix.
- Dashes: none in the diff, the commit messages or this review.
- AC10: no role name beyond the claim-role check and `guard`, no model name in `internal/dispatch/` or `cmd/waggle/dispatch.go` (the one "fable" hit is a comment citing a T-014b finding).
- `go build ./...`, `go vet ./...`, gofmt on `cmd` and `internal`, `go test -race ./...`: pass locally. The race detector saw no report from the renewal goroutine reading `ck.before.files`, which is written only before launch.
- `product-ci` on PR #50 at `22ea50a`: pass (ubuntu, 1m37s). `verify-leases-and-tiers` was still pending when I finished and stays the expected D-168 local-claims gap, not a phase C defect.

## Notes (non-blocking)

- **N1 (scout, phase D):** the escaped-process residual has a cheap mitigation that needs no Cell: record the `hostFiles` map (or its hash) in `result.json` at the end of a clean run, and have the next dispatch compare before its first unpinned call (the `ResolveBase` fetch), refusing with exit 3 on a change. About 10 lines; it turns the R-J case from "runs with the Keeper's credentials" into "refused before any git call".
- **N2 (builder, low):** `hostFiles` keys the global files as `global:<absolute path>`, so a `host_state` violation on the XDG ignore file prints the Keeper's home path into `result.json` and the CLI output, against the comment on line 85 ("a violation never names a host path"). Pre-existing for the global config since phase B; a relative key such as `global:xdg/git/ignore` would keep the record path-free.
- **N3 (builder, low):** extend `TestFixNestedRepoFilter` with the main-checkout and reuse-path variants, and add a strict-compare case for a skipped sync, so the three surviving mutations are killed.
- **N4 (carry from round one):** fable N1 to N4 and opus N1, N3 to N5, N8 to N10 stand unchanged: `max_budget` missing from section 9.1, the per-turn `usage` event deferred, the `## 3.` heading coupling, the late `ErrRefusedArg` surface, the own-claim OID re-check in local mode, the 64 KiB `tap` line cap, and the pinned Claude Code process-group check (AC7-style manual probe) before the first real run.
- **N5 (observation):** `TimedOut` is now true whenever the deadline fired, including the narrow case where the worker's success result lands in the same instant; that reads as `timeout`, exit 5, which still needs the Keeper, so it is the safe side.

## Intent Packet (for the Keeper)

- **What:** phase C fix round at `22ea50a` on PR #50; 11 fixes, 59 added non-test lines, 1223 total.
- **Why:** both guards found the push could carry an unchecked commit and that host git ran under agent-written config with the Keeper's credentials at three points; all closed.
- **Blast radius:** `internal/dispatch/` only (plus one test literal); no config, lease, scaffold or Comb change; the Comb backend used by dispatch now runs hooks-off, which does not change claim semantics.
- **Residual, stated in `not_enforced`:** a process escaping the worker's group can change host state after the checks and poison the next run's pre-launch fetch (N1 mitigation for phase D); the hash-to-renewal race during a session; gitignored main-checkout writes; `include.path`.
- **Merge:** pass; `product-ci` is green at `22ea50a`.
