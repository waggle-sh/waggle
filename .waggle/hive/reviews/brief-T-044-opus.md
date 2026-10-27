# Brief review: T-044 (opus guard, first review)

Risk-Tier: 3

Brief at `3b53c00` (`.waggle/hive/tasks/T-044.md`), cites checked at `bae5b28`, rulings D-335 and D-336 read on `origin/main`.

## Intent Summary

T-044 makes a `waggle dispatch` renewal of its own live claim append the run's branch to the stored branch list (unless already listed), refuse before any write when the list already holds 4 entries without it, and leave `waggle claim` / `ClaimTask` (D-292), takeovers, fresh claims and the guard unchanged. The change is one case in `AcquireClaim` gated by a new unexported `Request` field that only `Run`'s claim step sets. Blast radius: the branch list written into claim refs by dispatch, which the gate's `checkHead` matches against the PR head. The lease is `internal/dispatch/` only, with a per-file rule. Hive effect is nil until a pin move.

## What I checked

- Root cause is right. At `bae5b28`, `claim.go:65-66` sets `want.Branches = old.Branches` on a same-keeper renewal with no `ClaimBranches`, and `Run` (`run.go:360`) never sets `ClaimBranches` (only `ClaimTask` does, `task.go:52`). The keepalive (`run.go:411`) repeats `claim.Lease.Branches`, so the wrong list holds for the whole run. T-039 `:41`, `:87` and `:5` say this was by design and give the workaround the brief quotes.
- Every file:line cite in the brief matches `bae5b28`: `claim.go` 23-25, 34-39, 41-43, 44-47, 60-62, 63-72, 80-83; `run.go` 358-364, 359, 360, 404-416, 407-409, 411; `request.go` 57-72, 120-122, 146-163; `task.go:52`; `backend.go` 477, 489-516, 494-495; `cmd/waggle/dispatch.go:54`; `config.yaml:9` (pin `0d102ca`); `branches_test.go` 1, 21-48, 39-42, 51-72, 74-86, 89-108; `dispatch_test.go:167-227`, `:229`; `guard_test.go:138-177`; `guardrun_test.go:68-69`; `run_test.go:35`; `task_test.go:55`; `docs/gate.md:74` (cites `claim.go:58-60`) and `docs/recipes/README.md:29` (cites `:58-65`), both stale as the brief says. The only other `AcquireClaim` callers are `run.go:360` and `task.go:110`.
- The fix matches D-335 (append unless listed) and D-336 (full list refuses with ExitClaim before any write; docs cites wait). The custom refusal sits before `AcquireLease`, and `AcquireLease` refuses a fifth entry anyway (`backend.go:494-495`), so the planned cut still fails closed.
- Lease rule: three named files with named regions, plus tests in `internal/dispatch/`. That is enough for the change and no wider. `comb.ValidateBranches` already enforces the cap and rejects duplicates, so `internal/comb/` stays out. `cmd/waggle/` cannot set an unexported field.
- Gate semantics: `checkHead` (`cmd/waggle/gate_commits.go:342-370`) accepts if any entry matches the head. Appending keeps every earlier entry, so no earlier PR loses its binding. `claimFor` is untouched. This is consistent with T-042 (no fallback for an empty list), since dispatch never writes an empty list. It is also consistent with T-039: D-292 behavior for `waggle claim` and the takeover reset are kept, and the existing T-039 AC5/AC7 tests keep their results because they go through `AcquireClaim` without the flag or through a fresh claim.
- Budget follows WAGGLE.md section 4: bottom-up 15, doubled to estimate 30, target 39 (1.3x), stop 54 (1.8x), with a planned cut (drop the custom refusal, follow-up T-044b) and the D-210 rule for fix rounds.
- D-193: no signal, Timeout or cancel anywhere. No AC pushes to a local origin (`newRepo`/`claimEnv` and `newFixture(t, false, false)`).
- AC1 gate change: switching `okReq`'s `internal/x` from full to light is accepted, as `TestAcquireClaim` already does it.

## Findings

1. **must-fix (AC4 is not achievable as written).** `claimEnv` pins the backend clock at `t0` (2026-10-18 12:00 UTC, `dispatch_test.go:19`, `:145-156`) with a 1-hour TTL, so the pre-written claim expires at `t0+1h`. `Run` passes `now()` to `AcquireClaim` (`run.go:209-212`, `:360`), and that falls back to `time.Now` unless `Deps.Now` is set. With the real clock, `AcquireClaim` takes the `!old.ExpiresAt.After(now)` branch (`claim.go:60-62`). The verb is then `takeover` and the list resets to `[t-023b]`, so AC4's `renew` and `[t-023 t-023b]` assertions fail. `TestRunKeepaliveKeepsBranches` never hit this because its claim is fresh. Fix: AC4 must say the test sets `d.Now` to the `claimEnv` clock (for example `d.Now = func() time.Time { return *now }`), or writes the first claim with the same clock `Run` uses. Without this, a builder chasing a red AC4 may "fix" `AcquireClaim`'s expiry order instead, and that would be a takeover-semantics regression.
2. **note.** The brief cites "Nothing is written ... (`claim.go:37-39`)". The sentence is at `:38-39`. This is harmless.
3. **note.** Suggest the builder build the appended list with `slices.Concat(old.Branches, []string{req.Branch})` or a clone, not `append(old.Branches, ...)`. `old` is local, so aliasing does no harm today, but the clone makes the "stored entries unaltered" must-fix easy to check in review.
4. **note.** The full-list message format `<id>: claim already names 4 branches ...` puts the ticket id (`T-023`) in the message. AC3 checks that no list entry appears. If a test entry happened to contain the id's exact case-sensitive text, the check would fire falsely. The builder should pick entries that do not share it (for example `a`, `b`, `copilot/x`, `t-023`). The message is fine as specified, since `strings.Contains` is case-sensitive.
5. **note (D-203).** As section 5 already says, up to 3 more ticket-named branches can stay bound until release. That widens L1 only for repo writers, who can already push to any unprotected listed branch. It is not a must-fix.

Verdict: must-fix

# Brief review: T-044 (opus guard, round 2 confirming)

Risk-Tier: 3

Brief at `9564bba` (`.waggle/hive/tasks/T-044.md`), diffed against round 1 at `3b53c00`. Every changed or new cite read at `bae5b28` with `git show`. Nothing checked out, no repo file written.

## Intent Summary

Round 2 changes text only, in the brief. It amends AC4 so the `f.run` mutation sets `d.Now` to the `claimEnv` clock, and it forbids changing `AcquireClaim`'s expiry order to make the test pass. It folds in the round 1 notes: the `claim.go:38-39` and `backend.go:489-518` cite fixes, building the appended list as a new slice, the AC3 rule that no test entry is a substring of the refusal message, and a section 5 note on concurrent same-ticket runs flapping the list. The substance (D-335, D-336), the lease, the budget and the threat model (D-203) are unchanged. Blast radius is the same as round 1: dispatch-written claim branch lists, `internal/dispatch/` only, no hive effect until a pin move.

## What I checked

- AC4 must-fix. `claimEnv` pins the backend clock at `t0` (`dispatch_test.go:19`) with `DefaultTTL: time.Hour` and returns `&now` (`:145-156`). `Run` uses `d.Now` when set (`run.go:209-211`) and passes `now()` to `AcquireClaim` at `:360`. With `d.Now = func() time.Time { return *now }`, `old.ExpiresAt` (t0 plus 1h) is after `now`, so `claim.go:60` is skipped and `:63` gives `renew`. The keepalive renews through `backend.AcquireLease` (`run.go:411`), which uses the backend's own pinned clock, so it stays consistent. Setting `d.Now` also stamps `StartedAt` and `EndedAt` (`run.go:387`, `:422`) and the ledger `TS` (`ledger.go:25`). Nothing in the test depends on those. The `f.run` mutation signature `func(*Request, *Deps)` (`fixture_test.go:129`, used at `branches_test.go:94`) allows the change. The ban on changing the expiry order (`claim.go:60-62`) is explicit and marked must-fix. Resolved.
- Changed cites: `claim.go:38-39` (the "Nothing is written when the claim is refused" sentence), `claim.go:60-62`, `backend.go:489-518` (`ValidateBranches` from its doc comment to its closing brace), `run.go:209-211`, `:360`, `:411`, `check.go:220-221` (`own claim ref changed after the last renewal`), `dispatch_test.go:19`, `branches_test.go:74-86`. All match at `bae5b28`.
- `slices.Concat` is already used in `run.go` (`:388`), so the new-slice wording needs no new import or Go version.
- AC3: none of the example entries `t-023a`, `t-023b`, `copilot/x`, `owner/repo:t-023z` is a substring of `T-023: claim already names 4 branches and not this run's; set the list with waggle claim T-023 --branch, or release the claim`, since `strings.Contains` is case-sensitive. `t-023c` is not one either. The added sentence is consistent with the existing assertion.
- The concurrent-runs note matches the fable round 1 finding and cites correctly. It stays a note and adds no must-fix or scope.
- The diff touches only the Status line, the lease justification cite, 3.2 item 2, the full-list paragraph cite, the section 5 note, and AC3/AC4. Nothing else in the brief moved. No em or en dash was added.

## Findings

1. **note.** The Status line says this round "needs one confirming guard (D-205)", but the Risk Tier bullet says "Every fix round also gets two confirming guards, since any diff here touches claims". Both are right in their own scope. This round's diff is brief text that changes no code, and the Risk Tier bullet covers build fix rounds. A builder or later guard could read them as conflicting. One clause in the Risk Tier bullet ("build fix round") would remove the ambiguity. This does not block.
2. **note.** AC4's example uses `*now` without saying it is the pointer `claimEnv` returns (the test needs `b, now := claimEnv(t, f.real)` rather than the `b, _ :=` in `TestRunKeepaliveKeepsBranches`). The preceding words "the `claimEnv` clock" make the intent clear enough. No change needed.
3. **note.** The pre-claim in AC4 must go through the unwrapped backend, not the `recordBackend`. Otherwise the first recorded lease is `[t-023]` and fails "every one carries `[t-023 t-023b]`". The brief already says "with the fixture's backend" and only then introduces the `recordBackend`, so this is correct as written. A builder should keep that order.

No must-fix. The round 1 AC4 must-fix is resolved, the notes are folded in correctly, and the round introduced no new error.

Verdict: pass
