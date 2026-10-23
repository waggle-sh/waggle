# Guard review: T-033 brief

Verdict: pass
Risk-Tier: 3

Subagent guard hand-back, condensed by the queen; local paths replaced by `<scratch>`.

## Intent Summary
The brief (a1b9704 on 6deaf6a, one added file) specifies three product fixes in `internal/scaffold/` and `internal/dispatch/`: the guard role template and roster entry stop claiming `reviews/` and `media/`, and `guardTaskLines` gains a write-no-file clause; dispatch writes a versioned marker (`<!-- waggle-review v1 verdict=... head=... -->`) at the top of `review.md`, escapes column-0 copies, and counts only markers, keeping `runtime.CountMustFix` for text before the first marker; the scaffold gate workflow template converges on the hive copy with three stated differences. Diagnosis correct on every point checked, design sound, lease sufficient.

## Checks
- Item 1: guard.md lines 5 and 8 and roster `owns` confirmed; no Go code reads `owns`; dirty guard worktree gives ruleDirty, exit 1, matching T-032.
- Item 2: `CountMustFix` (policy.go:231) counts every `Verdict: must-fix` line; callers inputs.go:111 and :139; step-up at `GuardMustFix >= 2`. `hive-control-d224-opus.md` double counts (lines 3 and 25, one round); `hive-records-9.md` and `-10.md` are true two-round files.
- Item 2 design: escape prefix covers every countable marker; marker head equals `res.HeadOID`; existing tests pass unchanged.
- Item 3: template versus hive diff as stated; base presence step fixes repos without go.mod; `gate_verify.go` exists, so dropping the decisions fallback is safe; permissions read except `relay-claim`; init and upgrade tests hold.
- Lease, Gate `full`, Tier 3, ACs testable, budget 220/290/400 with a clean cut, threat model per D-203, Q-T033-1 to 3, section 2.4 pin move, references and hygiene: all correct.

## Blocking Issues
None.

## Notes
- N1: setup-go reads go.mod from the PR side, so the PR picks the toolchain that builds the base checker; a note under D-203. Accept it in one sentence in 2.3 so a build guard does not read it as conflicting with the section 5 must-fix.
- N2: on pull_request the workflow file runs from the merge commit; base building protects checker source, not the workflow. Pre-existing; G4 fixes. Tighten the threat model sentence.
- N3: whether escaping applies on `no_verdict` is ambiguous between 2.2 and AC5; state it once.
- N4: subagent-committed reviews (brief reviews) stay marker-less and keep legacy counting, including double counts; a marker-less round after a marker round counts 0; brief-review must-fix counts toward the build step-up (pre-existing). Name these as known limits.
- N5: marker writer could refuse a head that is not 40 or 64 lowercase hex (defensive).
- N6: AC9 "removed echo loop" is a difference from the old template, not the hive copy.
- N7: AC10 "plain terms" applies to the workflow template comment, per D-236.
- N8: 2.1's cross-reference to Q-T033-2 needs one clause on why no other line may start with `Verdict:`.
