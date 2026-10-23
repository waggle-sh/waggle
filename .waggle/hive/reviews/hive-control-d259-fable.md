# Guard review: hive control PR #105 (pin 7bef53b to 1475d74)

Risk-Tier: 3

Model: fable. Read-only review of branch `hive-pin-1475d74` at 655d867 against origin/main d422800, in a scratch worktree; the main checkout was read (run records, `bin/waggle` provenance) and never written.

## Intent Summary
PR #105 (head 655d867, one signed-off non-merge commit on base d422800) moves `waggle.pin` in `.waggle/control/config.yaml` from 7bef53b400910d55bfdcf397134b227168fa0bde to 1475d74c6ee8ed9e50de8dffa6984edfd454ddbb, the merge of T-034 (PR #103), and appends D-259 to `.waggle/hive/DECISIONS.md`. No managed file re-renders: T-034 changed only `internal/dispatch/guard.go`, `internal/dispatch/run.go`, `internal/runtime/policy.go` and tests, so templates, roles, roster and the gate workflow are unchanged between the two pins. Blast radius: model resolution and base defaulting for every guard the queen dispatches from `bin/waggle` once it is rebuilt at the new pin. Builder and forager runs, the CI gate (built from `BASE_SHA`, not the pin) and the scaffold templates are untouched.

## Checks
- Pin: 1475d74c6ee8ed9e50de8dffa6984edfd454ddbb is 40 hex, `git merge-base --is-ancestor` confirms it is on origin/main, and `git log -1` shows it as `Merge pull request #103 from waggle-sh/t-034` with parents 7ed603d (PR #102, the T-034 brief) and 273d5e0 (the T-034 build commit). The old pin 7bef53b is also on origin/main; the first-parent path between them is PRs #99 to #103.
- Scope: `git diff --stat d422800 655d867` is two files, 20 insertions, 1 deletion. config.yaml changes the pin line only (comment preserved). DECISIONS.md: main's 253966 bytes are a byte-for-byte prefix of the branch file (`cmp` on `head -c`), so D-259 is a pure append with no edit to earlier entries. No adds, deletes, renames, symlinks or `.gitattributes`.
- Gates: `scan-hive.sh origin/main 655d867` exits 2 as expected (one change outside `.waggle/hive/`: the Tier 3 config.yaml, which is why this review exists), zero privacy or dash hits, and its decisions gate reports `decisions: ok, appended D-259`. The `scan-names.local` warning is a host setup note, not a finding.
- Upgrade dry run: `waggle-1475d74 upgrade` from the worktree prints `[pin] 1475d74... (unchanged)`, `[same]` for roster.json, devcontainer.json, waggle-gate.yml, all five role templates, the .gitignore block and the CLAUDE.md block, `[skip] (seeded)` for config.yaml, WAGGLE.md and local.yaml, `[shims] regenerate .claude/agents/waggle-*.md` (gitignored) and `[keep] refs/waggle/* and .waggle/hive/**`. `git status` stays clean afterwards. So no rendered file should change, and none does.
- Binary: `go version -m` on the supplied build shows `path github.com/waggle-sh/waggle/cmd/waggle`, `mod ... v0.0.0-20261023224536-1475d74c6ee8`, `vcs.revision=1475d74c6ee8ed9e50de8dffa6984edfd454ddbb`, `vcs.modified=false`.
- Source at the pin: `git archive 1475d74` exported to the scratchpad; `go vet` clean and `go test -race` passes for `internal/dispatch` (190s), `internal/runtime` and `internal/scaffold`.
- D-259 text: date, kind, tickets (T-016 the upgrade ticket, T-034), options (stay on the old pin, move), recommendation, choice and rationale are all accurate against the diff and the dry run. The queen corrected the generated template's "Stay on" option, which on an already-moved worktree prints the new OID. "waggle upgrade reports every managed file [same]" matches the dry run. "alternate confirming-guard models" and "--second takes the first guard record's base" match D-251 to D-255 and the code (guardModel, secondBase). D-041 is the pinned-build policy it cites. Keeper by role only; no em or en dashes, names or emails in the added lines. Format matches D-250.
- Commit and PR: one commit, `Signed-off-by` present, no attribution trailer. `gh pr view 105` shows base `main`, head `hive-pin-1475d74` at 655d867, state OPEN, not draft, title matching the commit.
- Comparison with D-250 (a8eff90): that move carried three re-rendered files and needed a workflow review; this one carries none, so it is strictly smaller.

## Behavior hazard check for the next runs (T-034 under this hive's config)
Config: frontier opus, standard sonnet, fast haiku, alternates.frontier fable; roster guard `min_level: standard`, `require_cross_model: true`. Existing run records (`<repo>/.waggle/worktrees/runs`, T-026 to T-034) carry every field the new code reads: `ticket`, `role`, `model`, `branch`, `base_oid`, `head_oid`, `verdict`, `second`. Every record has `tier` null, so `topTier` is true and alternation applies to every plain guard that has a previous guard record with a verdict on the branch and a different head (D-255 covers unknown tier).

Walked the model space for the realistic sequences:
- Builder sonnet, guard 1 opus, must-fix, fix on sonnet, guard 2 avoids sonnet and opus: fable. `--second` on round 2 avoids sonnet and fable: opus. Resolves.
- Builder stepped up to opus (as T-030 run 2 and T-034), guard 1 avoids opus: fable. `--second` avoids opus and fable: D-179 walk to standard, sonnet (this is exactly the T-034 record set: opus, fable, sonnet). Round 2 plain guard avoids opus and fable: sonnet; its `--second` avoids opus and sonnet: fable. Resolves.
- With three usable models above the guard minimum and at most two to avoid, `confirm_model_repeated` cannot fire under this config; `confirm_model_unknown` fires only when a must-fix count exists with no guard record on the branch (a fresh host or a records wipe), and it is a warning, not a refusal (D-252).
- `--second` without `--base` now reviews on the first guard record's `base_oid` and requires the builder base to be an ancestor of it; all records to date share one base per round, and `req.Head` is set by `guardTarget` (run.go:333) before the guard's model and `checkSecond` steps run, so the head match is sound. A first guard that died before printing a verdict leaves no accepted record, and `--second` refuses with the `secondMissing` message until the first guard is rerun; by design (D-253).
- The queen template line 15 already dispatches a guard without `--base` and `--second` without a model, which is the D-254 default path; nothing in the committed hive asks the queen to pass the previous model any more (the summary line 45 says the manual step ends once the pin includes T-034).
No immediate hazard found.

## Blocking Issues
None.

## Notes
- N1: `<repo>/bin/waggle` in the main checkout still reports `vcs.revision=7bef53b`, `vcs.modified=false`. The queen summary says dispatch runs `bin/waggle` rebuilt from the pin after every pin move; until that rebuild (and `waggle upgrade --apply` for the gitignored shims), guards keep the pre-T-034 behavior and `--second` still needs `--base` and a card model. Same note as D-250 N1 and N2.
- N2: `.waggle/hive/handoff/queen.md` line 15 still names the 7bef53b pin and line 45 still says "until the pin moves"; refresh at the next records batch.
- N3: D-255's accepted edge stays accepted: because the builder rule avoids only the latest builder record's model, a plain guard on a stepped-up fix round can walk down to sonnet and review a diff containing sonnet's own earlier commits. Under this config it needs the builder to change model between runs and the previous guard to be fable; rare, Keeper-accepted, so a note.
- N4: `confirm_model_repeated` and `confirm_model_unknown` reach the queen only through `result.json` warnings; the queen should read the warnings line of every guard record on a confirming round.
- N5: the upgrade dry run on an already-moved worktree prints "Stay on 1475d74" as option 1; cosmetic in the generator, and D-259 has the right text. A product polish item, not a hive issue.
- N6: a D-203 observation only: run records are host-local, unsigned JSON that the new code trusts for model avoidance and base defaulting (after a hex check). Anyone who can write them can already run dispatch, so no new capability.

Verdict: pass
