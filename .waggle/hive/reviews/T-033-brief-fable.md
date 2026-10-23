# Guard review: T-033 brief

Verdict: pass
Risk-Tier: 3

Subagent guard hand-back, condensed by the queen; local paths replaced by `<scratch>`.

## Intent Summary
Second Tier 3 brief review of `.waggle/hive/tasks/T-033.md` at a1b9704 on 6deaf6a. The brief bundles three product fixes under one lease (`internal/scaffold/`, `internal/dispatch/`): the guard template and roster stop claiming review and media paths and the `Rules:` line gains a no-file clause; dispatch writes a versioned verdict marker at the top of `review.md`, escapes quoted copies, and counts markers after the first while legacy text keeps `runtime.CountMustFix`; `waggle-gate.yml.tmpl` becomes the hive workflow with a base-presence step, no `gate decisions` fallback and a reworded comment. Nothing in `.waggle/` or `.github/` changes in the ticket; the hive takes the renders at the next pin move under Head Keeper approval.

## Checks
- Item 1: guard.md lines 5 and 8 and roster `owns` confirmed; no Go code reads `owns`; T-032 guard-2 run record shows fable, `second: true`, `lease_violation`, rule `dirty`, verdict pass, exit 1.
- Item 2: `CountMustFix` and its two callers and the `>= 2` step-up confirmed; `hive-control-d224-opus.md` counts 2 for one round; `hive-records-9.md` and `-10.md` are two-round files counted correctly.
- Item 2 design: marker from `res.Verdict` and `req.Head`; escaping makes no guard-authored line countable; existing tests hold; scan rules do not match the marker. Sound.
- Item 3: template versus hive diff confirmed; claim fetch with no refs exits 0 (tested in `<scratch>`); gating setup-go on base presence keeps repos without go.mod green; permissions read except `relay-claim`; `TestRenderedTemplatesClean` and upgrade tests satisfiable.
- Lease and Gate `full`, ACs, budget 220/290/400 with item 3 as cut, threat model and D-193 statement, Q-T033-1 to 3, section 2.4, shape and dashes: all correct.

## Blocking Issues
None.

## Notes
- N1: 2.1 tells the builder to write "never under `.waggle/hive/media/`" while AC3 forbids that string in the rendered `guard.md`; fix before dispatch.
- N2: 2.2 and AC5 read as no escaping on `no_verdict`; say escaping applies to every `review.md` write.
- N3: 2.3 step 5 `src` step needs its own `env: BASE_SHA` block; say so.
- N4: AC9 "removed echo loop" is not a difference against main's copy.
- N5: AC7 holds only when the four `Verdict: must-fix` lines sit after the first marker; say so.
- N6: pointer fixes: forbidden strings live in `TestRenderedTemplatesClean`; T-007 template-scope text at line 145, Q-T007-1 at line 220.
