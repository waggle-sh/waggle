# Guard review: hive control PR #100 (pin 851f707 to 7bef53b)

Risk-Tier: 3

Subagent guard hand-back, condensed by the queen; local paths replaced by `<scratch>`.

## Intent Summary
PR #100 (head a8eff90, base e08db83) moves `waggle.pin` from 851f707 to 7bef53b (T-033, #98) and applies all three re-rendered managed files instead of reverting the gate workflow per D-143: `roles/guard.md` (guard owns nothing, review is the final message, verdict last), the `roster.json` guard entry (`owns: []`, purpose reworded) and `.github/workflows/waggle-gate.yml` (base-source presence step gating setup-go and the gate step, no `gate decisions` fallback, reworded comment). DECISIONS.md appends D-250. Dispatch reads `roles/guard.md` from base, so the new contract takes effect on merge.

## Checks
- Scope: five files; pin line only in config.yaml; DECISIONS.md a pure append (+20); one signed-off commit.
- Ancestry: e08db83 is the parent of a8eff90; 7bef53b is an ancestor of origin/main.
- Gate workflow diff against main is exactly the three D-249 differences; permissions unchanged (read, writes only on relay-claim), no `|| true`, checker still built from `BASE_SHA`, `gate verify` unchanged, no `pull_request_target`.
- No silent skip: a failed src step fails the job (implicit `success()` on later steps); both branches write the output; the only passing skip is `gate.go` absent on base, which the PR head cannot influence.
- This PR's own CI on the new workflow: src step success, setup-go success, Waggle gate built from e08db83 and failed with the expected no-claim lease error and four Tier 3 path errors, `decisions: ok, appended D-250`. Proves `present=true` and the gate ran.
- `guard.md` byte-identical to the 7bef53b template; upgrade replay from a clean 7bef53b build on an export of a8eff90 reports every managed file `[same]`, no rewrite.
- `waggle status` loads the config under 7bef53b.
- D-250 well formed, Keeper by role only, no dashes.
- Nothing else in the committed hive tells a guard to write `reviews/T-<id>.md`; queen role item 3 fits D-248's per-round markers.

## Blocking Issues
None.

## Notes
- N1: local shims `.claude/agents/waggle-guard.md` still carry the old contract; regenerate after merge with `waggle upgrade --apply` from the pin build.
- N2: `handoff/queen.md` still lists the guard template follow-up as open; clear it at the next records batch.
- N3: a `cat-file` read error on `gate.go` reads as absent and skips with a notice; only a corrupt object store reaches it; same as before.
- N4: a future base with `gate.go` but no `gate_verify.go` fails closed; acceptable.
- N5: setup-go reads go.mod from the merge commit; accepted in T-033 2.3 step 6, a note under D-203.
- N6: wait for product-ci green before merge.

Verdict: pass
