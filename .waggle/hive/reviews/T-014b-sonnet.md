<!-- SPDX-License-Identifier: MIT-0 -->
PASS WITH FIXES

# Guard review: T-014 phase B (PR #36, `t-014b`, 43bf7e3 fa5a341 009f749)

- Guard: sonnet (second guard runs on another model)
- Risk-Tier: 3
- Build: `go build ./... && go vet ./... && go test -race ./...` all pass on `origin/t-014b`.
- Evidence: `.waggle/hive/media/T-014b/` (`lib.sh` harness, `p1.sh` to `p6.sh` probes, `.out` logs). The scripts hardcode the scratch dir in `S=`; change it to run them. No secrets.

The gate holds against every lease escape, tier floor bypass and annotation injection I could build. Two defects in code the PR adds or leans on need fixing before merge. Neither lets a PR through the gate, but each leaves a stated guarantee unmet.

## Blocking findings

**B1. The `lease.json` cap does not bound memory (`internal/comb/git.go:361`, `:372`, `:389`).**
`loadClaims` runs `b.git(ctx, ..., "cat-file", "--batch")`, and `b.git` (`git.go:222`) collects all of stdout into a `bytes.Buffer`. The cap only skips decoding after the whole blob is already in Go memory. The phase A review (T-014a.md line 15) raised exactly this ("buffers the whole `cat-file --batch` output"), and D-107 folded it into this phase.
Repro (`media/T-014b/p4.sh`, `p5.sh`): forge a claim with a 300,000,000 byte `lease.json`; `waggle gate leases` returns the right "over the 65536 byte cap" error but peaks at 1,084,375,040 bytes RSS (about 3.6x the blob). `git cat-file --batch` itself peaks at 6 MB, so the cost is all in `waggle`. Boundaries are right: 65,536 bytes passes, 65,537 fails.
Not exploitable from a PR (claim refs are writable only by pushers of `refs/waggle/*`). It is still the one thing D-107 asked for, and it also hits `ListActiveLeases`, where one hostile ref inflates every command.
Fix: run `cat-file --batch-check` over the refs first (or `-s` per ref), drop oversize refs from the `--batch` input, and add a test that asserts a 1 MiB blob is never returned by `b.git`. About 10 lines. Alternatively stream `--batch` stdout through a pipe instead of `bytes.Buffer`.

**B2. `rev-list --format=%s` parsing assumes a non-empty subject (`cmd/waggle/gate_commits.go:56` to `:72`).**
With an empty subject git prints only the `commit <sha>` line, so the "exactly two lines per commit" assumption breaks.
- One empty-subject commit: odd line count, `verify: error: git rev-list: odd line count 1`, exit 3 instead of a check failure at exit 1 (`p2.sh` K, `p6.sh` last two cases).
- Two empty-subject commits, or any even total: the pairs desync. The second commit's `commit <sha> <parent>` line becomes the first commit's subject, and the second commit is never classified or diffed (`p2.sh` K2: its out-of-lease edit is not examined). Result is still exit 1, because the misparsed subject lacks a `T-` prefix, but only by accident. Any later relaxation of the prefix rule (a Head Keeper allowlist, Q-T014-4) turns this into a hidden-commit hole.
Fix: parse unambiguously, for example `git rev-list --reverse --parents --max-count=501 mb..head` for oids and parents, then `git log -1 --format=%s <oid>` per commit (the brief's own 4.2 wording), or `git log -z --format=%H%x00%P%x00%s`. Add tests for one and for two empty-subject commits (expect exit 1, "no T-<id>: prefix", and each commit checked).

## Probe results (no finding)

1. **Lease escape.** Held: outside file, copy into lease, delete outside, rename in, case variants, prefix confusion (`doc` vs `docs`), Cyrillic lookalike dir, newline and space paths (inside the lease they pass, correctly), symlink add, file to symlink type change, gitlink, mode-only change (passes), merge commit (fail), root commit path (`--root` works, only reachable through a merge anyway), `.` and `.waggle` claims (rejected at claim decode). `p1.sh`, `p3.sh`.
2. **Tier floors.** `.GitHub/Workflows`, `.WAGGLE/HIVE`, `.devcontainer`, `.github/workflows/sub`: lease and tier both fail. Light/full Tier 2 behaves per brief (see ruling 8). Gaps below are list gaps, not gate bugs (N3, N4).
3. **Annotations.** Every output line goes through `annotData` or `annotProp`: findings (`print`), path in `file=`, the `error:` line, and the `merge commit` / `no claim` messages (all built into `msg`). CR, LF, `%`, `:`, `,`, `::` in subjects and paths were neutralized; subject echo is `%q` plus a 72 byte cut (multibyte cut mid-rune is harmless, `%q` escapes it). Invalid UTF-8 paths print safely.
4. **Cap.** Boundaries correct; memory is B1.
5. **Commit cap.** 500 passes, 501 fails "split it", 5,001 paths in a commit fails. Subjects with `%n%H`, `commit <sha>`, CR: no effect. Empty subjects: B2.
6. **Workflow.** See N5 to N8; no blocker. Permissions are `contents: read` at both levels, trigger is `pull_request` (no `pull_request_target`), checker is built from `BASE_SHA` in a separate worktree, `|| true` is gone from the claim fetch, `relay-claim` keeps its three write scopes, template unchanged (scaffold drift test passes).
7. and 8. Rulings below.

## Rulings

- **Size (probe 7): accept.** D-136 counts added non-test lines: 510 (215 + 176 + 83 + 25 + 11). That is over the 450 stop, but 83 are `changecheck.go` (D-114 put the shared check in this phase, outside the brief's phase B estimate) and 11 are the D-107 cap. The `cmd/waggle/` part is 416, under 450. A `tiers` split would peel off roughly 40 lines (`caseCollisions`, the tier branch of `scan`) and force a second Tier 3 review of the same shared code for no safety gain. After B1 and B2 land, expect about 520; I accept that too.
- **Test 7 unreachable: accept.** Every Tier 2 path (`.waggle/hive/`, root `CLAUDE.md`, `AGENTS.md`) needs a claim on `.waggle` or `.`, and both are rejected at claim decode. So under `verify` it fails `leases`, and the Tier 2 rule is defence in depth, covered by the `comb.CheckChanges` unit test (and by T-023 dispatch). The substitute test asserts the right split.
- **Test 8 substitution (`GO.MOD` not Tier 3): accept.** T-008 is not merged; `go.mod` can only be leased via `.`, which is rejected, so it fails `leases`. The test says so. Restore the Tier 3 assertion when T-008 lands.
- **Merge commits: accept.** `continue` after failing is fine: the PR fails regardless and Q-T014-3 says merge commits fail. A merge-in-range also skips its tier findings, which only reduces noise.
- **`main.go` usage line left stale: accept.** `cmd/waggle/main.go:90` is outside the phase lease. Fold into the next brief that leases `main.go` (N1).

## Non-blocking notes

- **N1.** `cmd/waggle/main.go:90` still says "v0.1: 'gate decisions' rejects non-append DECISIONS.md changes". Update to list verify, leases, tiers.
- **N2.** `CheckChanges` accepts path components `.` and `..` (`docs/../evil` is "inside" `docs`, `p6.sh`). Git checkout and GitHub receive-pack reject these, so it is defence in depth only; reject empty, `.`, `..` and `.git` components as a lease violation. A one-line loop.
- **N3.** Tier lists are not exhaustive for CI-executed content: a claim on `.github/actions` is accepted and an edit to `.github/actions/evil/action.yml` passes `verify` (`p3.sh`). Same for `.github/CODEOWNERS`, `.github/dependabot.yml` if leased by `.github` subpaths. Add `.github/` as a whole to `HardcodedTier3Prefixes` in the T-008 or next comb list ticket.
- **N4.** Steering file lists are root-anchored: `docs/CLAUDE.md` or `pkg/AGENTS.md` are Tier 0 (`p1.sh` Q), yet Claude Code reads nested `CLAUDE.md`. Consider basename matching for Tier 2.
- **N5.** `actions/setup-go@v5` and `actions/checkout@v4` are tag-pinned, not SHA-pinned. Acceptable for first-party actions, but this job is the merge gate; pin by SHA in the Head Keeper's next workflow edit.
- **N6.** `setup-go` uses `go-version-file: go.mod` from the PR checkout, so a PR chooses the Go version that builds the base checker. The effect is a build failure at worst (fails closed). Point it at `$RUNNER_TEMP/gate-src/go.mod` after the worktree step to remove the PR from the trust path.
- **N7.** The `relay-claim` job interpolates `${{ github.event.comment.user.login }}` into a `run:` script. Pre-existing and logins are charset-limited, but move it to `env:` in a later workflow edit. Also consider `timeout-minutes` on both jobs.
- **N8.** Workflow bootstrap skips the whole gate with exit 0 when base lacks `cmd/waggle/gate.go`. Base has it now, so it is dead code; delete it once this PR merges so a future base change cannot silently disable the gate.
- **N9.** D-131/D-132 sign-off (`-s`) is absent from the three commits. The branch predates the `dco` job; confirm the Head Keeper's rebase adds sign-offs, or note the exemption.
- **N10.** Windows and macOS only: `go.mod.`, `GO~1.MOD`, NFD forms and NTFS streams fold to protected names on those checkouts. CI is Linux, so not a gate issue; relevant if a Windows runner is ever added.
- **N11.** Unrelated: `verify` in the error path (exit 3) prints "decisions: unchanged" but no leases or tiers result. Fine, but say "leases and tiers not run" so the log is not read as clean.

## Intent Packet

- What: adds `waggle gate verify|leases|tiers`, the shared pure `comb.CheckChanges` (D-114), a 64 KiB `lease.json` cap in `loadClaims` (D-107), and a Head Keeper workflow commit that builds the checker from `BASE_SHA` and runs `verify` with `contents: read`.
- Guarantees: every commit in `merge-base..head` is a `T-<id>:` commit (no merges, at most 500 commits, 5,000 paths each) whose changed paths, including deletions and both sides of renames, sit inside its claim's directories with exact case and add no symlink or submodule; no commit touches a Tier 3 path; Tier 2 paths fail under light claims; head tree has no case collisions; all output is encoded for GitHub workflow commands.
- Does not guarantee: Tier 3 PRs pass (they fail and merge by Head Keeper bypass, Q-T014-1), claim ownership or signatures, tier coverage beyond the current lists (`.github/actions`, nested `CLAUDE.md`), or bounded memory for oversized claim blobs until B1 is fixed.
- Blast radius: `cmd/waggle/gate*.go`, new `internal/comb/changecheck.go`, a cap in `internal/comb/git.go`, and `.github/workflows/waggle-gate.yml`; every future PR runs the new gate.

## Fix verification (sonnet, 6cf0567)

Reviewed `origin/t-014b` at `6cf0567` (4 commits over `main` `11f3327`). Read the full diff of the fix commit. Probes were throwaway tests in the guard worktree, deleted afterward (worktree clean). Nothing committed.

### Per-finding verdicts

- **B1 (lease.json read before size check): FIXED.** `loadClaims` now runs `cat-file --batch-check` first, classifies by type and size, and feeds only blob oids under the cap to `--batch`. Probe: three forged claims (valid, 65,717 byte, 314,572,980 byte lease.json). `ReadClaim` returns ok, "over the 65536 byte cap", "over the 65536 byte cap"; `ListActiveLeases` reports both malformed. `runtime.MemStats` TotalAlloc delta across all calls was 0 MiB (was about 1.08 GB RSS for 300 MB before). The in-tree test now wraps git and asserts the oversized blob oid never appears in `--batch` input; it passes. The `--batch` reply is re-validated (type blob, size within cap) so a race cannot slip a large blob through. Non-blob lease.json (tree) is rejected at the check stage.
- **B2 / F1 (empty subject desyncs rev-list): FIXED.** Format is `--format=.%s`, the dot is cut with `CutPrefix`, and a missing dot is a git error. New `TestVerifyEmptySubject` builds empty-message commits with `commit-tree`: one gives exit 1 with "commit has no T-<id>: prefix" and its path diffed; two give both reported (count of 2) and both diffed. Passes under `-race`.
- **F2 (missing BASE_SHA fails open): FIXED.** Workflow now runs `git cat-file -e "$BASE_SHA^{commit}"` and exits 1 with `::error::` before the file probe. Empty and bogus base both return rc 128 from git, so the step exits 1. The binary also rejects empty `--base` or `--head` (exit 2). The remaining `exit 0` is only the intentional "gate.go absent on base" bootstrap (N8, dead code today).
- **F3 (test 8 breaks when T-008 lands): FIXED.** Expectation is now keyed on `comb.RequiredMinTier(p) == comb.Tier3ControlPlane`. `TestVerifyTier3Paths` passes.
- **Sign-offs: FIXED.** All 4 commits in `origin/main..origin/t-014b` carry `Signed-off-by: paulternate` (checked via trailers). Closes N9 and F12.
- **F4 (workflow YAML runs from PR merge ref): ACCEPTED AS IS.** Platform property, not fixable in this PR. Needs T-015 ruleset or CODEOWNERS on `.github/workflows/**`; Head Keeper reads the diff until then. Recommend a decision entry.
- **F5 (CheckChanges precondition docs), F6 (EqualFold vs ToLower, file vs dir collision): NOT FIXED, ACCEPTED.** Academic for this hive, no gate bypass; carry into T-023 and the next comb brief.
- **F7, F9, F10, F11: ACCEPTED, Keeper decisions.** F9 (AC7 evidence) is post-merge. F10 (no full claim on `.waggle/hive/`, so hive-record commits merge by bypass) needs a Keeper ruling. F11: D-136 does not exist on this branch (DECISIONS.md ends at D-133); the Keeper must record the size decision under a real number.
- **F8 / N1 (stale `main.go:90` usage): NOT FIXED, ACCEPTED** (outside lease).
- **N2, N3, N4, N5, N6, N7, N8, N10, N11: NOT FIXED, ACCEPTED** as non-blocking follow-ups. None is a gate bypass. N5 to N8 belong in the Head Keeper's next workflow edit.

### Size (D-089 metric)

`git diff --numstat origin/main..origin/t-014b`, added non-test physical lines: gate.go 25, gate_commits.go 218, gate_verify.go 176, changecheck.go 83, git.go 48 = **550**. This matches the builder claim. It is up 40 from the 510 the guards accepted: git.go +37 (batch-check pass), gate_commits.go +3. The `cmd/waggle/` part is 419, under 450; the comb part is 131 (D-107 and D-114 scope). Workflow adds 34 more lines, not counted by the metric. Accepted: the growth is exactly the B1 fix the reviews demanded, and a split would only peel off about 40 lines of `tiers`.

### Build and tests

`go build ./...`, `go vet ./...`, `go test -race ./...` all pass at `6cf0567` (cmd/waggle, comb, config, decisions, forge, runtime, scaffold ok; layout has no tests).

### Regressions from 6cf0567

None found. Diff touches `git.go` (loadClaims only), `gate_commits.go` (listCommits only) and three test files. Behavior on small and valid claims is unchanged; the new path adds one extra git process per loadClaims call and fails closed on any short or malformed `--batch-check` output.

### Updated Intent Packet

- What: unchanged, plus loadClaims now sizes blobs with `--batch-check` and reads only blobs under 64 KiB; commit subjects are listed as `.%s`.
- Guarantees: previous guarantees hold, and now additionally oversized claim blobs are never loaded into memory, empty-subject commits are classified and diffed (exit 1, not a git error), and a missing base commit fails the workflow step.
- Does not guarantee: workflow YAML edits are caught by the gate (F4, T-015), tier coverage beyond current lists, hive-record commits passing without bypass (F10).
- Blast radius: unchanged.

**Final verdict: PASS**
