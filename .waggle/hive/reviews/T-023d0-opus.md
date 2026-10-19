<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: T-023 phase D0 (PR #53, `t-023d0` at `79171ec`, base `2ee74d9`), opus guard

Verdict: pass
Risk-Tier: 3

## Intent Summary

Phase D0 lands the runtime half of the phase D carry (10.6 items 16 to 21) inside `internal/runtime/`. `Config()` now returns a copy of `DefaultArgs` (16). `Verify` matches a help token only when no flag-name character follows it, so `--debug-file` no longer satisfies `--debug` (17). Two comments are corrected (18). Every worker now runs with `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1`. A new `Invocation.GitIdentity` adds the four `GIT_AUTHOR_*`/`GIT_COMMITTER_*` values when set. `Prepare` refuses a half-set identity or one with a C0 or DEL character (19, runtime half). `workerSettings` takes the role and drops `git add` and `git commit` for the guard; Edit and Write stay, per D-174 (20). `Resolve` gains `TaskContext.SecondGuard`: the second guard re-resolves the first guard from policy and avoids both its model and the builder's, and `ErrNoCrossModel` names `alternates.<guard level>` (21). Blast radius: the environment and permissions of every dispatched worker, and model selection for every guard. The global-config isolation closes the C0 fable probe: a Keeper's global `pushInsteadOf` no longer beats the push tripwire. The PR adds 80 non-test lines (stop 130). Nothing outside the lease changes, and phase C's dispatch and CLI tests pass unchanged.

## Blocking Issues (Must-Fix Only)

None.

## Checks

- **Lease:** the PR changes exactly `internal/runtime/{claude,launch,policy,runtime}.go`, `launch_test.go` and `policy_test.go`. `gh pr view 53` lists the same six files. All of them are inside the D0 lease line (D-170).
- **Size (D-136):** `git diff origin/main --numstat -- internal/runtime ':!internal/runtime/*_test.go'` gives 4 + 48 + 20 + 8 = 80 added lines (estimate 60, target 90, stop 130). Confirmed.
- **Commit:** a single commit `79171ec` with a `T-023:` subject and a `Signed-off-by` trailer. No U+2013 or U+2014 in the changed files.
- **Build, vet, tests:** `go build ./...`, `go vet ./...` and `go test -race -count=1 ./...` all pass locally (go 1.27.2, git 2.54 macOS), including `internal/dispatch` and `cmd/waggle`, which are unchanged. On the PR, `product-ci` passes on linux. `verify-leases-and-tiers` fails with "leases: no claim for T-023", as expected. The item 23 flake did not occur in this run.
- **Each item test fails with its fix removed** (`media/T-023d0/opus-mutants.sh`, output in `opus-mutants.txt`). Eight mutants (item 16; item 17; item 19 partial, control character, env pair and zero-identity guard; item 20; item 21) each make the PR's own test FAIL. The sources were restored byte for byte.
- **Item 19, refusals (probe P1):** refused: name only, email only, NUL, LF, CR, tab, ESC, US (0x1f) in the email, DEL. Accepted: the zero value, a plain identity, C1 U+0085 and U+009B, a raw 0x9b byte, U+2028, U+2029, angle brackets, and an all-space identity. See N1.
- **Item 19, env cannot be overridden (P2):** the parent env sets `GIT_CONFIG_GLOBAL`, `GIT_CONFIG_NOSYSTEM=0`, `GIT_CONFIG_SYSTEM`, `GIT_CONFIG_PARAMETERS`, `GIT_CONFIG_COUNT=99`, `GIT_CONFIG_KEY_10`, `GIT_AUTHOR_NAME`, `GIT_DIR` and `XDG_CONFIG_HOME`. The effective worker env, read through `os/exec` and `env(1)`, holds only our `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, `COUNT=10` and keys 0 to 9. None of the parent values survive. `workerEnv` still never copies `os.Environ()`. `envAllowed` has no `GIT_*` or `XDG_*` name. `default_args` are argv only, and `ContextSettings` adds only two integer variables. The identity variables are appended after `envFixed` and use different keys, so they cannot collide with it.
- **Item 19, the C0 fable probe (P3):** the test uses a temp HOME whose `~/.gitconfig` holds `url."git@github.com:".pushInsteadOf https://github.com/` and whose XDG `git/config` holds a longer `ssh://` rewrite. The push URL of `https://github.com/waggle-sh/waggle.git` is `ssh://git@github.com/waggle.git` under the C0 env and `file:///dev/null/github.com/waggle-sh/waggle.git` under the D0 env. Both the global and the XDG files are ignored now. Three residuals have the same reach as before D0. A `pushInsteadOf` in the repo's own `.git/config` still wins. A worker shell that sets `GIT_CONFIG_PARAMETERS` still wins. A worker shell that sets `GIT_CONFIG_GLOBAL` back also wins: it is the same class as clearing `GIT_CONFIG_COUNT`, and it needs code execution through a hive command. `GIT_CONFIG_COUNT/KEY/VALUE` interplay is unchanged. The count is still the last key, set only by `tripwireEnv`. See N3.
- **Item 19, zero identity (P8 and the PR test):** a zero identity sets both `GIT_CONFIG_*` values and no `GIT_AUTHOR_*` or `GIT_COMMITTER_*`. A commit under the D0 env in a repo with only a local `user.email` succeeds. It takes the local email and git's passwd-derived name, never the Keeper's global name. See N4.
- **Item 17 (P4, against the pinned help `media/T-023a/claude-help.txt`):** `Verify` with `--debug` and `--max-budget-usd=2` passes on the unmodified pinned help. It fails, naming the right token, when `--debug` becomes `--debugx`, when `--debug` is removed but `--debug-file` stays, when `--max-budget-usd` becomes `--max-budget-usdx`, when `--restricted` becomes `--restricted-mode`, and when `"dontAsk"` becomes `"dontAskMe"`. It passes when `--debug` appears only as `--debug=<f>`, `--debug.`, `--debug` followed by a non-ASCII letter, or `--no--debug`. Unit checks: `-file`, `_x`, `9` and `x` after the token are refused. `=`, `,`, `[`, `<`, end of text, `)`, `"`, `.` and `/` are accepted. A later true match after a false one (`--debug-file --debug`) is found. See the ruling on deviation 1 and N2.
- **Item 16 (P7):** after `Verify`, editing `Config().DefaultArgs` in place or appending to it leaves both the next `Config()` and the next argv unchanged. Editing the caller's original slice after `Verify` also leaves argv unchanged, because `Verify` already clones it (C0). `AdapterConfig` has no other slice or map field. No alias remains.
- **Item 20 (P5):** the guard settings for `go build`, `go test` hold Read, Glob, Grep, `Edit(./**)`, `Write(./**)`, the hive commands and `git status`/`diff`/`log`/`show`, and no `git add` or `git commit` rule in any form. The builder settings are byte-identical to the pre-PR shape. Role spellings other than `guard` (`Guard`, `forager`, `scout`) keep the builder set. That is harmless, because `Prepare` requires `Selection.Role == Role` and `Resolve` gives the guard path only to `guard`. `git  commit` (double space) as a hive command is refused for every role by the existing plain-words check. Keeper-listed hive commands that begin with `git add`/`git commit` but are not exactly those two words still reach the guard. See N5.
- **Item 21 (P6):** an eleven-config matrix. The second guard never shares the builder's or first guard's model when both guards resolve with the same `TaskContext`. Cases covered: no alternates, an empty alternates map, an alternate at the guard level, an alternate at frontier, an alternate equal to the builder model, an alternate duplicating another level's primary, alternates duplicated across levels, and a frontier builder with and without a frontier alternate. Each one either picks a third model or returns `ErrNoCrossModel` naming `alternates.<guard level>` and the task. The one SHARED case needs both a duplicated primary (`frontier` = `fast`) and a `GuardMustFix` change between the two guard dispatches. See N6.
- **D-174:** the guard keeps Edit and Write inside the worktree and the `WAGGLE.md` commands. Only the two git write rules go.

## Rulings on the builder's interpretations

1. **Help follower rule ("no flag-name character follows" instead of the brief's list):** accepted. The brief's list would fail `Verify` on the pinned help: `dontAsk` appears there only as `"dontAsk"`, followed by a quote. The cases the brief targets still fail: `--debug-file`, `--debugx`, `--debug_x` and `--debug9`. The wider rule also accepts `.`, `/` and non-ASCII after a token, which no real help prints after a flag name.
2. **New `TaskContext.SecondGuard`:** accepted. `--second` needs some way to reach `Resolve`, and the field is ignored for every role but `guard`. Phase D must set it only for `--second`, and must give the second guard the same `TaskContext` facts as the first (N6).
3. **`ErrNoCrossModel` wording:** accepted. The message now prints the avoid list as a Go slice (`["sonnet" "opus"]`) and still names `alternates.<level>`. The sentinel text ("no model differs from the builder's") is unchanged. It is slightly narrow for the second-guard case, but `errors.Is` callers are unaffected.

## Notes (non-blocking)

- **N1. "Control character" is C0 plus DEL only.** `GitIdentity.check` refuses `r < 0x20 || r == 0x7f`, the same definition `workerSettings` uses. It accepts C1 controls (U+0080 to U+009F), invalid UTF-8 bytes such as a raw 0x9b, and U+2028/U+2029. P8 shows git writes a U+009B name into the commit header unchanged, where it reaches the Keeper's terminal through `git log`. The identity comes from the Keeper's repo or global config (dispatch half), and an agent write to the common `config` is already a host-state violation, so the reach is small. If a fix round happens anyway, the one-line change is `unicode.IsControl(r) || r == utf8.RuneError` (or `!utf8.ValidString`) in `check`. Git itself strips LF, `<` and `>`.
- **N2. `helpHas` checks only the character after the token, not the one before.** `--no--debug` satisfies `--debug`. A flag removed from help but still named in another flag's prose also passes: line 50 of the pinned help says "via --settings". The brief's rule has the same limits, and the fixed argv fails loudly on an unknown flag. Recorded only so nobody treats `Verify` as a parser.
- **N3. Two comments are now stale on the global config.** `pushBlocked` (`launch.go`) still lists "a longer pushInsteadOf from git -c or from the Keeper's global git config". The global no longer applies, and the remaining source is the repo's own `.git/config`. The `not_enforced` line in `internal/dispatch/cell.go:48` says the same. The second is a phase D fix in its lease. The first is a one-line comment in this lease; folding it in now saves opening `internal/runtime/` again in phase E.
- **N4. Between D0's merge and phase D, a dispatched worker gets git's passwd name.** Phase C passes a zero identity, and this repo has a local `user.email` but no local `user.name`. Under D0, a worker's commits take the OS account's passwd name with the noreply email, instead of the Keeper's global name. This hive does not dispatch through `waggle dispatch` before phase D (workers are subagents), so this is only interim. Phase D's resolution order (local, then explicit global read, else exit 3) closes it.
- **N5. The guard filter matches exact strings.** It drops `git add` and `git commit` from `gitAllowed`, but a `WAGGLE.md` section 3 command such as `git commit -s`, `git add -A`, `git -C . commit` or `env git commit` still becomes a guard allow rule. Section 3 is read from base and is a Tier 3 file, so only the Keeper can add one. Comparing the first two fields for the guard would be about one line. Separately, any guard holding Write plus `go test` can run a commit from test code. Phase D's post-run check that the detached guard worktree's HEAD still equals `head_oid` is the real control, and phase D should test it.
- **N6. Re-resolving the first guard assumes the second guard's `TaskContext` is the first one's.** If `GuardMustFix` rises between the two guard dispatches (the queen commits the first guard's must-fix review before dispatching `--second`), the builder step-up changes the re-resolved builder and first-guard models. With a policy that repeats a primary across levels (P6: `fast` = `frontier` = `haiku`, alternate `opus`), the second guard then gets `opus`, the model the first guard actually ran. `Validate` permits such a policy. The brief mandates re-resolution from policy, so this is not a D0 defect. Phase D should compute `GuardMustFix` once per review round for both guards (or count only reviews from earlier rounds). `Validate` could also refuse a model that appears in more than one slot.
- **N7. A frontier builder leaves no second guard.** Each level has two slots, and the walk only goes up. When the builder resolves to frontier (by minimum or after the step-up on two must-fix verdicts), the builder and first guard fill both frontier slots. `--second` then always fails, with advice to set `alternates.frontier` even when it is already set (P6, "builder frontier, alt frontier"). That is the D-100/D-116 rule working as written, but it fails exactly on the Tier 3 tasks that have needed a step-up. This is a queen question for phase D or F (more than one alternate per level, or letting the second guard go below the builder's level), not a D0 fix.

## Probe files

- `media/T-023d0/opus-probes_test.go.txt`: probes P1 to P8, run as `internal/runtime/opus_probe_test.go` and then removed.
- `media/T-023d0/opus-probes.txt`: `go test -v -run TestOpus` output.
- `media/T-023d0/opus-mutants.sh`, `opus-mutants.txt`: the fix-removal check.

## Confirmation at ebce792

Verdict: pass
Risk-Tier: 3

Scope: `git diff 79171ec ebce792`. One fix commit (`T-023:` subject, `Signed-off-by` trailer) touches `launch.go`, `policy.go` and their two tests. The PR head on GitHub is `ebce792`.

### Checks

- **Lease:** the PR still changes exactly `internal/runtime/{claude,launch,policy,runtime}.go`, `launch_test.go` and `policy_test.go` (`gh pr view 53` agrees). All six are inside the D0 lease (D-170).
- **Size (D-136):** `git diff --numstat 2ee74d9 ebce792` over non-test files gives 4 + 66 + 35 + 8 = 113 added lines (stop 130). The fix round adds 24 + 16 = 40. Confirmed.
- **Dashes:** no U+2013 or U+2014 in the PR diff.
- **Build, vet, tests:** `go build ./...`, `go vet ./...` and `go test -race -count=1 ./...` pass locally on every package. `gh pr checks 53`: `product-ci` passes. `verify-leases-and-tiers` fails with "leases: no claim for T-023; claims must stay until merge", as expected. The item 23 flake did not occur.
- **Each fix's test fails with the fix removed** (`media/T-023d0/opus-confirm-mutants.sh`, output in `opus-confirm-mutants.txt`). Fourteen of sixteen mutants make the package's tests FAIL. They remove, in turn: the blank trim in `check`, `utf8.ValidString`, `unicode.IsControl` (reverted to C0 plus DEL), the U+2028 check, the U+2029 check, the blank guard in `workerEnv`, `GIT_ATTR_NOSYSTEM`, `core.excludesFile` and `core.attributesFile`. They also force a stale `GIT_CONFIG_COUNT` of 10, remove the D-179 walk, apply the walk to the first guard, leave `Level` unchanged by the walk, and walk lowest level first. Two mutants survive the PR's tests: swapping primary and alternate inside the walk, and dropping the avoid check inside the walk. The P6 reference below catches both (5568 and 45984 violations), and the shipped code has zero. See C3. The sources were restored byte for byte.
- **P1, identity refusals** (`opus-confirm-probes.txt`): all 32 C0 characters, DEL, all 32 C1 characters and U+2028/U+2029 are refused, in the name and in the email. Invalid UTF-8 is refused: a raw 0xff, a raw 0x9b, a truncated sequence, an overlong `/`, an encoded surrogate, and 0xff in the email. These count as half set and are refused: a blank name with an email, an email of one space, an email of one tab, an NBSP name, and a NEL name. Two blank fields (spaces, or NBSP plus U+3000) are accepted as the zero value and produce no identity variables. That makes `workerEnv`'s `TrimSpace(Name)` test safe, because every non-zero identity reaches `workerEnv` through `Prepare`, which calls `check` first (`Verify` passes a zero identity). These are accepted and produce four identity variables: plain, non-ASCII letters, a literal U+FFFD, U+202E and U+200B. See C2.
- **P2, env under a hostile parent.** The parent sets `GIT_CONFIG_GLOBAL`, `GIT_CONFIG_NOSYSTEM=0`, `GIT_ATTR_NOSYSTEM=0`, a `GIT_CONFIG_PARAMETERS` that sets `core.excludesfile`, `GIT_CONFIG_COUNT=99`, `GIT_CONFIG_KEY_10=core.pager` and `GIT_CONFIG_KEY_12`. The home holds a global `.gitconfig` with `pushInsteadOf` and `core.excludesFile`, plus XDG `config`, `ignore` and `attributes` files. The effective env, read through `env(1)`, holds only our values: `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, `GIT_ATTR_NOSYSTEM=1` and `GIT_CONFIG_COUNT=12`. Keys 0 to 9 are the push rewrites, and keys 10 (`core.excludesFile`) and 11 (`core.attributesFile`) are both `/dev/null`. No key appears twice in `spec.Env`, with or without an identity, and the last `GIT_CONFIG_*` entry is `VALUE_11`. `tripwireEnv` emits the count once, from `len(keys)`. Under that env, against the hostile home, git behaves as intended. The push URL is `file:///dev/null/github.com/waggle-sh/waggle.git`, so the tripwire wins. Neither the XDG ignore file nor the global `excludesFile` applies. `check-attr` reports `unspecified`, and `git config` reports `/dev/null` for both keys. The repo's own `.git/info/exclude` and `.gitattributes` still apply, as they should.
- **P6, D-179 matrix.** The matrix is exhaustive over a four-model alphabet for the three primaries and three optional alternates, every builder and guard minimum, and `GuardMustFix` 0 and 2. That gives 73728 valid combinations. Each is checked against an independent reference: upward from the guard level, then downward from one level below it, primary before alternate. Zero violations:
  - The second guard never returns the builder's or the first guard's model.
  - The first guard matches the pre-D-179 upward rule exactly and never walks down.
  - `builder` and `forager` resolve identically with `SecondGuard` set or unset.
  - When the first guard cannot resolve, the second guard returns `ErrNoCrossModel`.
  - The walk returns the highest unused lower level, primary first.
  - It errors (4668 cases) only when no slot at any level holds an unused model.
  - Whenever the walk is used, `Selection.Level` is the level of the slot the model came from.

  Named case: this hive's config plus `alternates.frontier: fable` with a stepped builder gives opus, fable, then sonnet at standard. That is the case D-179 was written for.
- **Comments:** the `pushBlocked` comment now names the repo's own `.git/config` (including an `include.path`) and a worker shell that resets `GIT_CONFIG_*` or `GIT_CONFIG_PARAMETERS`. It also says the global and system config no longer apply. All three statements are accurate. `internal/dispatch/cell.go:48` still says "from git -c or the Keeper's global config". That file is in phase D's lease (N3 carry).

### Guard floor and the D-179 walk

`guardLevel` (rule 5) sets the guard level to `max(roster guard minimum, builder effective level)`. It refuses an override below that on Tier 2, Tier 3 or unknown tier. D-100 asks for "a third model different from both sonnet and the builder's model" and sets no level. D-116 and brief item 21 ask for a model different from the builder's and the first guard's, resolved "from the guard level up". D-179 replaces "up" with "up, then down to any level", and allows `ErrNoCrossModel` "only when no unused model exists at any level". No rule in the brief, D-100 or D-116 bounds the second guard's level, and D-179 explicitly allows going below the floor. The code matches D-179 word for word, so this is not a defect.

The effect is wider than D-179's question suggests, since that question was about a frontier builder. In P6, 18792 of 58800 successful second-guard resolutions land below the roster guard minimum, and 11592 land on fast.

**This hive's own config now sends an unstepped Tier 3 second guard to haiku at fast.** The config is `opus`/`sonnet`/`haiku`, builder and guard minimum standard, no alternates. The builder gets sonnet, the first guard opus, and the second guard haiku. Before the fix round, that case returned `ErrNoCrossModel` with a hint to set an alternate. The old test asserted exactly that, and the fix round replaced it with an all-`m` policy. `Selection.Level` honestly reports fast, but nothing warns. The stepped case still needs `alternates.frontier`, because without it the first guard has no frontier model (rule unchanged).

This is a Keeper question for the queen, not a D0 blocker: should the walk stop at the roster guard minimum? The change is one line: `for l := level - 1; l >= max(LevelFast, p.Minimums[roleGuard]) ...`. D-179's motivating case still resolves (frontier builder, first guard on `alternates.frontier`, second guard on sonnet at standard). The unstepped hive case goes back to the config-hint error. Phase D's trial (Q-T023-10) runs `--second` only on a Tier 3 ticket, so this needs a ruling before that trial at the latest.

### Notes (non-blocking)

- **C1. The previous round's N1 and N3 are closed.** N1: C1 characters, invalid UTF-8 and U+2028/U+2029 are refused, and blank fields count as unset. N3: the runtime comment is fixed. The `cell.go` half stays a phase D carry. Keys 10 and 11 and `GIT_ATTR_NOSYSTEM` close fable N2 (home ignore and attributes). `TestEnvIgnoresHomeIgnoreAndAttributes` has working controls: it first shows that the files apply under a plain env.
- **C2. Format characters are accepted.** U+202E (right-to-left override) and U+200B are category Cf, not Cc, so `unicode.IsControl` accepts them. A U+202E name can reorder text in `git log` output on a terminal. The identity comes from Keeper-owned config (the dispatch half), so the reach stays small. If anything else reopens `check`, adding `unicode.Is(unicode.Cf, r)` covers it.
- **C3. Three D-179 properties have no PR test.** `TestSecondGuard`'s D-179 case has no alternate at standard and leaves sonnet unused, so a walk that tries the alternate first, or one that ignores the avoid list, still passes. The second of those would break the D-100 invariant: the second guard could share the builder's model. The "every model taken" case (`m` at all levels) fails at the first guard's `pick`, not after the walk, so the walk's own exhaustion path is untested too. Three small cases cover them. Each uses a builder minimum of frontier, primary frontier `opus` and `alternates.frontier: fable`, so the builder gets opus and the first guard fable:
  - Standard `sonnet` with `alternates.standard: s2` expects `sonnet`, catching the swap.
  - Standard primary `opus` with alternate `s2` expects `s2`, catching the ignored avoid list.
  - Standard `opus` and fast `fable` with no other alternates expects `ErrNoCrossModel` from the walk.

  Worth adding in phase D or whenever `policy_test.go` is next open. The P6 reference shows the shipped code is correct.
- **C4. The sentinel text names a first guard on first-guard errors.** `ErrNoCrossModel` reads "no model differs from the builder's and the first guard's" even when the first guard itself fails, or when `Materialize` reports it. `errors.Is` callers are unaffected, and the detail text ("with the builder or first guard") was already shared. Cosmetic.
- **C5. Keys 10 and 11 also override a repo-local `core.excludesFile` or `core.attributesFile`.** They are set at command scope, so they beat a local setting of either key. The repo's `.git/info/exclude` and tracked `.gitattributes` still apply (P2), and this repo sets neither key locally, so worker `git status` output does not change here. Recorded so a later repo with a local `core.excludesFile` is not surprised.

### Probe files

- `media/T-023d0/opus-confirm-probes_test.go.txt`: probes P1, P2 and P6, run as `internal/runtime/opus_confirm_probe_test.go` and then removed.
- `media/T-023d0/opus-confirm-probes.txt`: `go test -v -run TestOpusConfirm` output.
- `media/T-023d0/opus-confirm-mutants.sh`, `opus-confirm-mutants.txt`: the fix-removal check.

## Confirmation at d969d57

Verdict: pass
Risk-Tier: 3

Scope: `git diff ebce792 d969d57`, which touches only `internal/runtime/policy.go` (+3 -2) and `internal/runtime/policy_test.go` (+57). PR #53 head is `d969d57`.

### Checks

- **The bound matches D-180.** The walk is now `for l := level - 1; l >= max(LevelFast, p.Minimums[roleGuard]) && model == ""; l-- {`. That is the line the previous section proposed, and it is D-180's "goes no lower than the roster guard minimum". `LevelFast` in the `max` is redundant after `Validate` but harmless. When the walk finds nothing, `err` keeps the `pick` error, so the config hint survives. The first guard, other roles and the walk's order are unchanged.
- **P6b, bounded matrix** (`media/T-023d0/opus-confirm2-probes.txt`). Same exhaustive space as P6 (73728 valid combinations). The reference is now upward from the guard level, then downward no lower than the guard minimum, primary before alternate. Zero violations. The second guard resolves in 40008 cases (17568 by walking down) and never below the guard minimum. It errors in 23460 cases. Each of those is `ErrNoCrossModel`, has no unused model at or above the minimum, and names an `alternates.<level>` hint. Of the 18792 former below-minimum resolutions, all 18792 now error. The rest are unchanged: 58800 minus 18792 is 40008, and 4668 plus 18792 is 23460. The other P6 invariants still hold. The first guard matches the upward rule, builder and forager ignore `SecondGuard`, and `Selection.Level` is the level of the slot the walk took.
- **This hive's unstepped shape refuses again.** With opus/sonnet/haiku, builder and guard minimum standard and no alternates, the builder gets sonnet and the first guard opus. The second guard returns `ErrNoCrossModel` with "for example runtimes.adapters.claude-code.alternates.standard". `TestSecondGuardHiveShapeRefuses` asserts that hint.
- **D-179's motivating case still resolves.** With a frontier builder (stepped, or builder minimum frontier) and `alternates.frontier: fable`, the builder gets opus, the first guard fable, and the second guard sonnet at standard. The P6b named cases and `TestSecondGuardWalk/motivating_case` both show it.
- **C3 is closed. Each test fails with its fix removed** (`media/T-023d0/opus-confirm2-mutants.sh`, output in `opus-confirm2-mutants.txt`). All eight mutants fail, and the sources were restored byte for byte.
  - Unbounded walk (D-179 line restored): `TestSecondGuardHiveShapeRefuses` and `TestSecondGuardWalk/floor` fail.
  - Bound off by one (`>`), or bounded at the builder minimum: the motivating, primary-first and avoid-list subtests fail.
  - Alternate before primary: `primary_before_alternate` fails. This mutant survived at ebce792.
  - Avoid check dropped: `avoid_list_inside_the_walk`, `walk_exhaustion` and `floor` fail. This mutant also survived at ebce792.
  - Walk exhaustion broken: `walk_exhaustion` fails, alone when a used model is accepted at fast, and with `floor` when the error is cleared before the walk. `walk_exhaustion` now reaches the walk. The first guard resolves (opus, then fable), and the walk then finds standard `opus` and fast `fable` both used.
  - Walk removed: `TestSecondGuard` and three walk subtests fail.
- **Lease:** `gh pr view 53` lists the same six files, all under `internal/runtime/` in the D0 lease (D-170).
- **Size (D-136):** `git diff --numstat 2ee74d9 d969d57` over non-test files gives 4 + 66 + 36 + 8 = 114 added lines (stop 130). Confirmed.
- **Dashes:** no U+2013 or U+2014 in the PR diff.
- **Build, vet, tests:** `go build ./...`, `go vet ./...` and `go test -race -count=1 ./...` pass locally on every package. `gh pr checks 53`: `product-ci` passes. `verify-leases-and-tiers` fails with "leases: no claim for T-023; claims must stay until merge", as expected. `relay-claim` was skipped. The item 23 flake did not occur.

### Notes (non-blocking)

- **C6. A recorded low-tier guard override below the minimum gets no walk.** On a Tier 0 or 1 task, a recorded override can set the guard below the roster minimum (rule 5). Its walk starts below the bound and never runs, so `--second` errors where the unbounded walk would have resolved. That follows D-180's wording and only touches a second guard on a low-tier task, which nothing schedules. Recorded so the case is known.
- **C3** from the ebce792 section is closed by `TestSecondGuardWalk`. C2, C4 and C5 stand as written. The guard-floor question raised at ebce792 is settled by D-180.

### Probe files

- `media/T-023d0/opus-confirm2-probes_test.go.txt`: probe P6b, run as `internal/runtime/opus_confirm2_probe_test.go` and then removed.
- `media/T-023d0/opus-confirm2-probes.txt`: `go test -v -run TestOpusConfirm2` output.
- `media/T-023d0/opus-confirm2-mutants.sh`, `opus-confirm2-mutants.txt`: the fix-removal check.
