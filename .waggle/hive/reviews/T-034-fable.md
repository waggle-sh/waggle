<!-- waggle-review v1 verdict=pass head=273d5e0601a24ba7e9e94ead25cb7709bbd346e7 -->

Risk-Tier: 3

**Intent Summary:** One commit (`273d5e0`) on `t-034` over base `7ed603d`, touching only `internal/runtime/policy.go`, `internal/dispatch/guard.go`, `internal/dispatch/run.go` and test files in those two directories. It makes dispatch pick a guard's model from its own run records: every guard avoids the builder record's model, a Tier 3 or unknown-tier plain guard also avoids the previous round's guard model (retrying with `confirm_model_repeated`, or warning `confirm_model_unknown` when records are missing), and `--second` avoids this round's first guard record. `checkSecond` now requires the first guard's base exactly, hex-checks both recorded base OIDs before any git call, and accepts a builder base that is an ancestor of the first guard's via `merge-base --is-ancestor`. `--second` with no `--base` defaults to the first guard record's base. The second guard's `ErrNoCrossModel` names the level whose alternate keeps the first guard where it is. Blast radius: model resolution for every dispatched guard on this hive once the pin moves; builder and forager resolution and step order are unchanged. Nothing in `cmd/waggle/`, `.waggle/` or `.github/` changes.

**Verification run on this host (darwin):**

| Command | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | no output |
| `go test -race ./...` | all packages ok, including `internal/dispatch` (204s) with the remote-mode fixtures |
| targeted `-run` of the new and changed tests, `-v` | all 13 tests and every subtest pass; `TestGuardSecondResetsWorktree` passes unchanged |

Added non-test lines (numstat): guard.go 140, run.go 43, policy.go 79, total 262, under the 300 target. No em or en dash anywhere under `internal/`. Working tree clean; I wrote no files.

**Blocking Issues (Must-Fix Only):** none.

I checked each item of the section 5 must-fix list against the code:
- **Guard never on the builder's model:** `Resolve` avoids the policy's builder model plus every `AvoidModels` entry (policy.go:145-155); `guardModel` always seeds `AvoidModels` with the builder record's model (guard.go:163-165) and `checkBuilderModel` is the post-resolution fallback. The AC4b stepped-up case (record sonnet, policy opus, previous fable) resolves fable with the warning, and on the no-alternates policy returns `ErrNoCrossModel` with `pick`'s text, never sonnet.
- **`--second` never on the first guard's model:** `AvoidModels` carries the head-H first guard record (guard.go:166-173); rule 5 kept (guard.go:257).
- **Alternation with warning:** Tier gate at guard.go:177, retry only on `ErrNoCrossModel`, warning names the previous head. Round-to-round alternation over the fabricated history matches AC5 exactly.
- **Record filters:** `latestRun` requires the `ticket` field to match, a canonical integer `<seq>` (`+12` and symlinked directories are rejected), `second: false`; `guardRun` adds branch and non-empty verdict; no `end` filter, so the `lease_violation` record counts (AC4).
- **Hex before git:** both recorded base OIDs are checked at guard.go:242-246 before `merge-base`; `secondBase` checks before `ResolveBase`. Only exit 0 from `merge-base --is-ancestor` accepts; exit 1 and every other failure refuse with the two distinct messages. `Git.Run` wraps with `%w` so the `*exec.ExitError` match works and hooks and fsmonitor are off.
- **Builder and forager unchanged:** the `before`/`after` split (run.go:291-294) keeps the non-guard order identical to the base commit; `Resolve` for non-guard roles takes no `avoid` and no walk.
- **Caps and panics:** reads stay under `maxBrief`; `firstGuardModel` is reached only with a non-empty list.
- **Lease:** only the listed files changed; `run.go` edits sit in the seq steps and the warnings line.

**AC2 (changed existing assertions), each checked against its section:**
- `TestSecondGuardHiveShapeRefuses`: `alternates.standard` to `alternates.frontier` (2.3).
- `TestSecondGuardWalk/floor`: `alternates.frontier` to `alternates.standard` (2.3).
- `TestSecondGuardRefusals/base moved`: explicit `--base main` refuses with `pass --base <first>`; no `--base` runs on the first guard's base (Q-T034-4 (a), 2.2 rule 3).
- `TestSecondGuardRefusals/model repeats`: expects `builder "<model>"` (2.1, 2.3 with `RecordedBuilder`).
- `TestSecondGuardRefusals/review committed between the guards`: first half becomes a default-base run with `must_fix` equal to the first guard's (2.2, D-254).
- `guardrun_test.go`: only the permitted comment above `TestMustFixCountPerPhase`.
No `t.Skip` added. I could not see a PR body from this worktree, so the queen should confirm the PR body lists these five entries when she opens it.

**Notes (not blocking):**
- N1: `Resolve` enables the plain-guard downward walk only when the avoid list holds a model other than the policy's builder model (policy.go:158). This is narrower than a literal reading of 2.1 ("a plain guard with `AvoidModels`") but it is the reading that keeps a round 1 first guard on a stepped-up, no-alternates build at `ErrNoCrossModel` as today, which section 5 requires. AC3 passes under it. Worth one sentence in the merge decision.
- N2: A direct caller of `Resolve` with `SecondGuard`, `AvoidModels` equal to `[RecordedBuilder]` and nothing else would skip the first guard re-resolution and could land on the first guard's policy model. Dispatch cannot produce that input (`guardModel` refuses before `Resolve` when the first guard record is missing), so this is a hardening nicety for a later ticket.
- N3: `guardModel` restates `topTier` inline (guard.go:177); exporting the runtime helper would remove the duplicate.
- N4: `firstGuardModel` with an empty-model record yields `first guard ""` in the hint; harmless, since `pick` ignores empty entries.

Verdict: pass