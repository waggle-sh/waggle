<!-- SPDX-License-Identifier: MIT-0 -->
# T-023 phase C0 confirmation review (fix commit, guard: fable)

Verdict: pass
Risk-Tier: 3

Reading: PASS. Branch `t-023c0` at `afcc9c3`, one fix commit on top of `e71d298`, two commits over main `768e03d`, PR #49. Reviewed in a detached read-only checkout, independently of the parallel guard on another model (its review was not read). Evidence under `.waggle/hive/media/T-023c0/fable-confirm-probes_test.go.txt` (the probe source) and `fable-confirm-probes.txt` (its output, temp and tool paths redacted).

## Intent Summary

The fix closes the three asks from the `e71d298` reviews inside the phase C0 lease: `pushBlocked` gains `git+ssh://` and `ssh+git://` (10 prefixes, `GIT_CONFIG_COUNT=10`), its doc comment now calls the tripwire best effort behind the Bash deny rules and 7.2 rule 4 and lists what it does not catch, and `Verify` clones `DefaultArgs` after the help check so a caller that keeps a reference to its slice cannot swap in an unchecked flag; `TestDefaultArgsMutatedAfterVerify` covers that and `TestPrepareEnv` is updated for the new count and indices. Blast radius: the worker launch environment and argv for every future `waggle dispatch` run (phase C), nothing that runs today. The diff touches only `internal/runtime/launch.go` and `launch_test.go`; build, vet, gofmt and `go test -race ./...` are clean on darwin and `product-ci` passes on linux. Size is 64 added non-test lines against main (60 non-blank), under the D-164 stop of 130, matching the builder's count. No blocking issue.

## Blocking Issues (Must-Fix Only)

None.

## Checks against origin/main

1. Lease: `git diff --stat origin/main..afcc9c3` lists exactly `internal/runtime/launch.go` and `internal/runtime/launch_test.go`.
2. Size (D-136 metric): 64 added lines in `launch.go` against main, 60 excluding blank lines; stop 130 (D-164).
3. `Signed-off-by` trailer present on both PR commits (`e71d298`, `afcc9c3`), author matching the trailer.
4. No U+2013 or U+2014 in the diff against main or in either commit message.
5. `go build ./...`, `go vet ./...`, `gofmt -l internal cmd`: clean. `go test -race -count=1 ./internal/runtime/`: ok. Full `go test -race -count=1 ./...`: every package ok (no regression outside the lease).
6. `product-ci` on PR #49: pass at `afcc9c3` (1m28s). `verify-leases-and-tiers`: fail, see the note for the Keeper below; it also failed at `e71d298` for the same reason.

## Probes

Tripwire under the exact worker env (`Prepare(...).Env` plus `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, `GIT_SSH_COMMAND` and `GIT_SSH` pointed at a stub that writes a marker file and exits 255, `GIT_TRACE=1`):

- Control with `GIT_CONFIG_COUNT=0`: a push to `git+ssh://127.0.0.1/x.git` invoked the stub (marker written), so the stub is live and the "no ssh call" results below mean something.
- With the tripwire on: `git+ssh://127.0.0.1/x.git`, `ssh+git://127.0.0.1/x.git`, `git+ssh://user@127.0.0.1:22/x.git`, `ssh://127.0.0.1/x.git` and `git@127.0.0.1:x.git` each fail with exit 128 at `'/dev/null/...' does not appear to be a git repository`; the trace shows `run_command: ... git-receive-pack '/dev/null/...'` as the only child, no marker file, no `ssh-stub` in the trace. Rewrite happens before transport selection, so the ssh transport is never chosen.
- Index check: the generated env carries `GIT_CONFIG_VALUE_5=git+ssh://`, `GIT_CONFIG_VALUE_6=ssh+git://`, `GIT_CONFIG_VALUE_8=/`, `GIT_CONFIG_VALUE_9=.`, as `TestPrepareEnv` now asserts.

Mutation test of `TestDefaultArgsMutatedAfterVerify`: with the `c.cfg.DefaultArgs = slices.Clone(c.cfg.DefaultArgs)` line removed, the test fails with `Prepare used the mutated slice` and an argv ending in `--max-budget-usd=3` although the fake help lacks that flag; restored, it passes. The test is sound: the fake help lists `--max-cost` instead of `--max-budget-usd`, `Verify` runs with `--debug` only, then the caller rewrites index 0 to a well-formed `--max-budget-usd=3`, which `checkDefaultArg` alone would accept. Only the clone keeps it out.

Comment accuracy (each stated gap probed under the same env, output in the evidence file): `HTTPS://` is not rewritten and on darwin reaches curl (the remote helper lookup is case-insensitive there); `SSH://` is not rewritten and fails only because `git-remote-SSH` does not exist; `https::http://...` is not rewritten and reaches curl; `127.0.0.1:x.git` and `user@127.0.0.1:x.git` are not rewritten and invoke the stub ssh; a global config `url.<x>.pushInsteadOf = https://127.0.0.1:1/` and the same via `git -c` both win over the tripwire's shorter `https://`; `remote.<name>.pushurl` wins over a `file:///dev/null/` url; a push to a worker-made symlink `link.git` pointing at a bare repository succeeds and moves a ref there. Every gap the comment lists is real, and nothing it claims is false. The `envFixed` comment three lines below still says the tripwire "rewrites every push URL"; see N2.

## Non-blocking notes

N1 (carry to phase C). `Config()` returns `c.cfg` by value, but `DefaultArgs` still aliases the adapter's internal slice after `Verify`, so `a.Config().DefaultArgs[0] = "--max-budget-usd=3"` after `Verify` is the same bypass the clone closes for the caller's original slice. No non-test caller exists today. One `slices.Clone` in `Config()` (or cloning in `NewClaudeAdapter` as well) closes it; not worth a C0 round.

N2 (cosmetic). The `envFixed` comment says the tripwire "rewrites every push URL to file:///dev/null/". With the honest `pushBlocked` comment directly above it, no reader will be misled, but "every push URL it recognizes" would be exact.

N3 (design, phase C). `Verify` now writes `c.cfg.DefaultArgs`; a second `Verify` call racing a `Prepare` on the same adapter would be a data race on the slice header. The contract is one `Verify` before any `Prepare`, so this is a note for the phase C caller, not a defect here.

Note for the Keeper (outside this lease). The `verify-leases-and-tiers` gate job fails on PR #49 at both `e71d298` and `afcc9c3` with `leases: no claim for T-023; claims must stay until merge`, and `git ls-remote origin 'refs/waggle/claims/*'` returns nothing. The code in this PR cannot fix that; the T-023 claim ref needs to be on origin before the gate can pass, so the merge path is blocked on a hive action, not on the builder.

## Intent Packet (merge commit)

T-023 phase C0 at `afcc9c3` (PR #49, lease `internal/runtime/launch.go` and `launch_test.go`): replaces the `default_args` denylist with the D-137 allowlist (`--max-budget-usd=<positive decimal>`, `--debug`, each also required in the pinned `--help` by `Verify`, which then clones the slice so later caller mutation cannot skip that check), adds `--no-session-persistence` and `--disable-slash-commands` to the fixed argv (D-163), generates the push tripwire from `pushBlocked` (ten prefixes: https, ssh, git@, http, git, git+ssh, ssh+git, file, `/`, `.`) rewriting to `file:///dev/null/`, a dead local target that makes no network or ssh attempt, and documents the tripwire as best effort with its uncaught forms listed. Verified live: pushes to every listed scheme including the two new ones fail at `/dev/null/` without an ssh invocation under the exact worker env. Size 64 added non-test lines, under the 130 stop (D-164). Carry to phase C: clone `DefaultArgs` in `Config()` (N1), tighten the `envFixed` comment (N2), one `Verify` per adapter before `Prepare` (N3). The `verify-leases-and-tiers` gate is red for a missing T-023 claim ref on origin, which is a hive action before merge. Tier 3: Head Keeper approval required (D-100).
