# Guard review: hive control PR #100 (pin 851f707 to 7bef53b)

Risk-Tier: 3

Subagent guard hand-back, condensed by the queen; local paths replaced by `<scratch>`.

## Intent Summary
PR #100 (head a8eff90, one commit on base e08db83) moves `waggle.pin` from 851f707 to 7bef53b (T-033, #98) and, for the first time since D-143, applies every re-rendered managed file: the gate workflow (D-249 shape), `roles/guard.md` (guard writes no repo file, review is the final message, verdict last line) and `roster.json` (guard `owns: []`, new purpose). DECISIONS.md appends D-250. Every future PR runs the new gate workflow, and every guard dispatched after merge gets the new role template.

## Checks
- Scope: exactly five modified files, pin line only in config.yaml; no adds, deletes, renames, symlinks or `.gitattributes`.
- Ancestry: merge-base e08db83; one signed-off non-merge commit, same convention as prior pin commits.
- Gate workflow diff is exactly the three D-249 differences; permissions, claim fetch, tier case block, checkout depth, base build and `gate verify` untouched.
- Silent skip: the src step always runs and fails the job on error; the only skip path is `cmd/waggle/gate.go` absent on base, as before. `go.mod` and `gate_verify.go` exist on main.
- This PR's own CI (run 40018188819) ran the new workflow end to end: src step, setup-go, gate built from e08db83 running `gate verify`; failure is the expected no-claim lease error plus four Tier 3 path errors, `decisions: ok, appended D-250`. product-ci passed.
- Templates at 7bef53b byte-identical to the branch workflow, guard.md and roster.json (with `{{PROFILE}}` as `cli`).
- Upgrade replay from a clean 7bef53b build: every managed file `[same]`, seeded files skipped, no rewrite. `waggle status` loads the config.
- DECISIONS.md: main's bytes are a prefix; D-250 well formed, Keeper by role only, no dashes, no name in content.
- Old guard contract elsewhere: queen role item 3 already matches the new contract; scan-hive and WAGGLE.md section 4 stay correct; no Go code reads roster `owns`; the base dispatch task line already says write no file.

## Blocking Issues
None.

## Notes
- N1: `handoff/queen.md` still lists the guard template and must-fix counting follow-ups as open; both closed by T-033 and this pin; clear at the next records batch.
- N2: local `.claude/agents/waggle-*.md` shims keep the old guard text until `waggle upgrade --apply` runs in the main checkout after merge.
- N3: setup-go reads go.mod from the merge commit; accepted in T-033 2.3 item 6, a note under D-203.
- N4: a base that loses `cmd/waggle/gate.go` skips the gate with a notice and passes, as before.

Verdict: pass
