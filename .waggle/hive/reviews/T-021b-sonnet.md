# Review: T-021 phase B, guard (sonnet) (branch t-021b, head f23b13d)

Risk-Tier: 3

Reviewed: branch t-021b, head f23b13d (two commits on main 6a4886e), draft PR #42. Gate: full. Did not read any other guard's review.

Checks: `go build ./...`, `go vet ./...`, `go test -race ./...` all pass; `gofmt -l .` is empty. `waggle gate verify --base origin/main --head HEAD` prints `leases: FAIL: commit 63b1bbfb149a ... Head Keeper commit: merge with a Head Keeper bypass`, `leases: FAIL: no claim for T-021; claims must stay until merge`, `tiers: ok`, `decisions: unchanged`. Both lease lines are the pre-merge state (Head Keeper commit not yet merged with the bypass; no claim ref in this clone), not lease violations. No em or en dashes in any touched file (grep clean). No `go.mod` or `go.sum` change.

## Intent Summary

Phase B adds the `context:` block to the seeded config template (`profile: balanced`, no `roles`), a Handoff section to the queen role prompt, a parser for the block in `internal/config` (`ParseContext`, `LoadContext`) that rejects unknown keys, bad profile words, roles keys outside the roster and `waggled`, two layout constants (`Handoffs`, `Usage`), and both paths in the `.gitignore` block that init and upgrade write. `LoadPolicy` and `ParsePolicy` keep their signatures and now also validate the context block, so a hive cannot carry a bad context block past init or upgrade. Blast radius: every new hive (template), every hive on `waggle upgrade` (queen prompt rewrite, ignore block edit, a `[note]` that the seeded config lacks `context`), and T-022 phase B and T-023 phase B, which consume `layout.Usage`, `layout.Handoffs` and `config.LoadContext`.

## Blocking Issues

None.

## Findings

1. **Commit 63b1bbf matches section 7.3 and touches only the two templates.** `git show --stat` lists `config.yaml.tmpl` (+4) and `roles/queen.md` (+3) only. The template gets the `context:` block with `profile: balanced` and no `roles`, plus the required comment line verbatim. The inline `# lean | balanced | thorough` comment is an addition to the brief's literal block; harmless, and test 10's `withContext` helper depends on that exact text. The queen change is the Handoff section only; nothing else in the prompt moved. No model or level name in the block (AC4); the comment's word "models" is prose.

2. **Queen handoff text (Tier 3 prompt content): correct and consistent, one clarity gap.** It matches `internal/runtime/handoff.go`: title `# Handoff: <task> <role> <seq>`, five headings in the exact order, 150 lines and 16 KiB caps, path `.waggle/control/handoffs/queen-<n>.md` (matches the brief and the gitignored directory). The style matches the neighboring sections (imperative, backticks, no dashes). Deviation 5 (`<task>` is the ticket in focus, or `hive`) is sound: the parser needs exactly three whitespace-free tokens, and `hive` satisfies that. Gap, non-blocking: the text does not say the headings are `##` level, that each is preceded by one blank line, or that bullets start with `- `, all of which `ParseHandoff` requires. Nothing parses the queen's file in v0.1 (the queen is interactive), so no failure today, but a future `waggled` that parses it would reject a freehand file. Suggest a one-clause fix at the next Head Keeper template touch: "`## ` headings, one blank line between sections, `- ` bullets". Not worth holding the merge.

3. **Phase B commit is inside the lease.** Files: `internal/config/context.go` and `context_test.go` (new), `internal/config/roster.go` (3 lines, the one call site), `internal/layout/layout.go` (two constants), `internal/scaffold/init.go` (ignore-block body, 2 lines), `init_test.go` (one constant), `internal/scaffold/context_ignore_test.go` (new). Templates are not in this commit. Two items are slightly beyond the literal lease text and are declared: the `init_test.go` edit (required, because `wantGitignore` encodes the block) and a new scaffold test file (test 12 needs a home; the lease says "ignore-block body only"). Both are benign; Keeper may want to ratify them in the merge note (see Rulings).

4. **Deviation 1 (`LoadPolicy` keeps its signature; `policy()` call site) is sound.** Section 7.1 says `LoadPolicy` "also returns the ContextPolicy", which would change a signature whose callers (`init.go`, `upgrade.go`) are outside the lease. Keeping the signature and calling `ParseContext(cfg, mins)` inside the shared `policy()` body gives the validation (the safety property: no hive carries a bad context past init or upgrade) through both `LoadPolicy` and `ParsePolicy` with three lines. T-023 phase B's "Depends on" names "`ContextPolicy` from config", which `LoadContext`/`ParseContext` satisfies. The cost is that `LoadContext` re-reads config and roster when a caller wants both policies; negligible. `ParseContext` runs after the roster loads, so the roles-key check has the roster it needs.

5. **Deviation 2 (context error reports after roster errors) is fine.** A bad context word with a bad roster reports the roster first; both are hard errors, only the order differs. No test depends on it.

6. **Deviations 3, 4 and 5 are sound.** 3 and 4 are covered in finding 3; 5 in finding 2.

7. **Tests 10 to 12 are meaningful.**
   - Test 10 parses the rendered template and checks `DeepEqual` against balanced with an empty `Roles`, then exercises `LoadContext` from disk (template config plus rendered roster) and its failure on an empty directory. `withContext` fails loudly if the template block text drifts, so the tests cannot silently stop covering the template.
   - Test 11 covers all five brief cases (`Lean`, `max`, `context.budget`, roles key not in roster, `waggled`) plus empty string, bad role value, and three shape errors (`roles` scalar, `context` scalar, `profile` mapping). Each asserts `ErrInvalidConfig`, the key path text, and a `line ` number, which is the brief's requirement.
   - `TestPolicyChecksContext` proves the `policy()` wiring: a stale roles key fails `ParsePolicy` naming the key, and `guard: lean` is accepted (the floor is applied at `Settings`, not rejected at parse). Reverting the three-line `roster.go` edit would fail this test.
   - Test 12 starts from a pre-T-021 ignore block, runs `init --force`, and compares the whole file to `wantGitignore` plus checks both new lines. Passes.

8. **The guard floor cannot be bypassed by any profile or config value (AC3).** Config can only produce a `ContextPolicy{Default, Roles}`. The floor lives in the merged phase A `Settings`: for role `guard` and any tier that is not Tier 0 or 1 (including nil, i.e. unknown), `floorSettings` raises every field to at least the balanced value. `ParseContext` never short-circuits that: `roles.guard: lean` is accepted by design and lifted at resolve time; no config key mentions the floor, skipping, or the guard's tier. `roles` keys are limited to roster ids and `waggled` is refused, so there is no alias for the guard. A zero `Default` resolves to balanced in both `ParseContext` (explicit) and `Settings` (fallback). Residual note, not for this phase: the floor keys on the literal id `guard` (`roleGuard`), which the brief says T-020 leaves unchanged.

9. **`layout.Handoffs` trailing slash is safe for every current caller, with one latent hazard.** The only use today is `gitignoreBody` in `init.go`, where the slash is wanted (directory-only pattern, same shape as `layout.Worktrees + "/"`). The pattern contains an interior slash, so it is anchored at the repo root; correct. Latent hazard for T-023 phase E: a caller writing `layout.Handoffs + "/T-..."` would produce `//`, and the T-023 brief writes `layout.Handoffs/T-<nnn>-<role>-<n>.md` loosely. `filepath.Join` cleans it, string concatenation does not, and `Handoffs` is the only constant with a trailing slash (all others, including `Worktrees`, have none). Brief section 7.2 mandates the slash, so this is compliant. Suggest the doc comment on the constant say "trailing slash: append the file name directly or use `filepath.Join`", or that the scout note it in T-023 phase E. Non-blocking.

10. **`waggle upgrade` from this branch against a hive whose config lacks `context:` works; absent means balanced.** Rehearsal: built `origin/main` and the branch binary, ran `waggle-old init --forge none` in a scratch repo (config has no `context`), committed, then `waggle-new upgrade` and `upgrade --apply`. The plan lists `[rewrite] roles/queen.md`, `[skip] config.yaml (seeded)`, a `[note]` that the config lacks top-level key `context` and to add it by hand, `[edit] .gitignore (waggle block)`, and shim regeneration. `--apply` succeeds; the ignore block gains `handoffs/` and `usage.jsonl`; the only config.yaml change is the pin line; the regenerated queen role has the Handoff section. `waggle status` on the upgraded hive works. Unit evidence: `ParseContext` of an absent block returns `Default: balanced`, empty `Roles` (`TestContextAbsentAndOverrides`). No hive edit is forced.

11. **Size: 103 added non-test lines against the 300 cap (AC8).** `context.go` 94, `roster.go` 3, `layout.go` 4, `init.go` 2.

12. **Downstream contracts hold.** T-022 phase B needs `layout.Usage`; present, doc comment tied to section 8. T-023 phase B needs `ContextPolicy` from config (`LoadContext`/`ParseContext`, exported) and `layout.Handoffs` (present). No `internal/usage` or `internal/dispatch` was touched.

13. **Minor, non-blocking.** `LoadContext` has no caller outside its test yet (T-023 phase B is the first); acceptable for a phase that ships an API. `ParseContext` returns an initialized empty `Roles` map for the absent case; a nil map is also valid for `Settings`, so both shapes behave the same.

## Rulings needed from the Keeper

- Ratify the two small lease overreaches (new `internal/scaffold/context_ignore_test.go`, one-constant edit in `init_test.go`) in the merge note. Both are required for test 12 and the existing assertion; no behavior risk.
- The gate's two `leases: FAIL` lines clear only through the Head Keeper bypass merge for `63b1bbf` and a claim for T-021; confirm that is the intended merge route.
- Optional, Head Keeper's discretion: tighten the queen Handoff clause with the `##`, blank line and `- ` bullet requirements (finding 2).

Verdict: PASS
