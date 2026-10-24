# Brief review: T-036 (G4 distribution), opus, round 1

Risk-Tier: 3

Reviewed: `.waggle/hive/tasks/T-036.md` at `e8a68eb` on `t-036-brief` (base `c9cb754`; main `1037e77` adds only the pin 97127fc and the re-rendered hive workflow, which change no claim made here except one line cite, N11). Checked against `docs/rfc-notes/repositioning.md` sections 4 to 6, DECISIONS D-229 to D-269, T-035 2.1, 2.6, 2.7 and 5, T-033 2.3, T-018 sections 6 and 7, and the code in `internal/scaffold/`, `internal/forge/`, `internal/config/`, `internal/decisions/`, `cmd/waggle/`. Local experiments ran in the scratchpad only (git push behavior, Go VCS stamping, an offline build from a git-less export).

## Intent summary

T-036 delivers G4 in two phases. Phase A adds a composite Action at `gate/action.yml` that builds `cmd/waggle` from its own pinned tree and runs `gate verify`. The scaffold workflow template gains an Action step, pinned to the commit of the build that rendered it, for repos without the gate source on base. The template loses the relay job, the `issue_comment` trigger and the shell tier step. `waggle init --gate-only` writes three files, `waggle upgrade` keeps such repos gate-only, `version` becomes an ldflags `var`, and `waggle dispatch release` is removed (D-241). Phase B adds `waggle doctor` with two checks: claim-ref push and the required gate check with strict up-to-date. The release workflow, tags, signing, repo settings and the hive's own workflow stay Head Keeper commits. Blast radius: every outside gate user's installed workflow and Action. In the hive: a workflow rewrite at every pin move from now on, and `dispatch release` breaking at the first pin move past phase A.

The core design holds. Building from the Action's own tree at a SHA covers everything that runs through git's content addressing. The hash-cycle argument against committed checksums is correct. Rendering the Action ref from `vcs.revision` closes T-035 2.1's skew for the template path. Three spec defects block dispatch. Each needs only a few lines in the brief.

## Blocking issues (must-fix)

**M1. `--gate-only` followed by a full `init` cannot produce `TestInitFileSet`'s set, and `--gate-only` in a fresh full hive is not a no-op (2.5 at T-036.md:103, AC-A6 at :206).**
- Full `init` ends with `config.LoadPolicy(root)` for the default runtime `claude-code` (`internal/scaffold/init.go:308-316`). `LoadPolicy` calls `ParseRuntimes`, which fails when `runtimes:` is absent (`internal/config/roster.go:107`, `internal/config/runtimes.go:34-37`). The gate-only config of 2.5 has no `runtimes:` block, and the seeded config is skipped on the second `init` (`init.go:262-264`). So the full `init` after `--gate-only` writes the template files, directories and blocks, then fails before it writes the shims. That leaves a half-written hive and an error.
- The gate-only `DECISIONS.md` is not in `TestInitFileSet`'s list (`internal/scaffold/init_test.go:174-182`), so even with runtime `none` the set differs.
- A full `init` never creates `.waggle/hive/DECISIONS.md`. So in a hive that has not yet run `waggle decide`, "`--gate-only` finds every file present and writes nothing" is false: it creates `DECISIONS.md`.
- Fix: the brief picks one of these:
  - (a) A full `init` over a gate-only footprint refuses before any write. It names the missing config sections, or tells the user to move the gate-only config aside.
  - (b) The full `init` is defined to skip shims, with a note, when the policy cannot load.
  - (c) Drop the "full `init` afterwards" promise and that AC clause.

  Under any choice, state that `--gate-only` in a full hive creates only a missing `DECISIONS.md`, and rewrite AC-A6 to match. Option (a) is the smallest and fails closed.

**M2. The claim-refs push probe reports `missing` for any clone that lacks the remote tip object (2.9 at T-036.md:121; same assumption in T-018 section 6, table row `custom-refs`).**
- The probe reads the tip OID with `ls-remote` and pushes `<oid>:refs/waggle/probe/...`, on the stated basis that "the OID is already on the remote, so nothing uploads". But `git push` needs the object locally.
- Verified in the scratchpad. A clone that had not fetched the remote's current tip ran `git push --atomic --porcelain origin <oid>:refs/waggle/probe/x-a <oid>:refs/waggle/probe/x-b`. It printed `fatal: bad object <oid>` and `!` porcelain lines (`[remote rejected] (unpacker error)`), exit 1. After a `git fetch` the same push succeeded and the delete succeeded.
- Under 2.9 that maps to `missing` ("this remote does not accept pushes to refs/waggle/*"), a false finding. It is the common case: a developer clone that has not fetched since the last merge, or a fresh `git init` with a remote added.
- Fix: before the push, check `git cat-file -e <oid>^{commit}` through the same git seam. If the object is absent, report `unknown` with Fix "run git fetch <remote>, then waggle doctor" and push nothing. Doctor should not fetch on its own, since 2.9 limits what it writes. Add an AC-B3 row: absent object gives `unknown` and no push call.

**M3. Q-T036-5 (a) contradicts the template's byte-for-byte list and the phase A lease rule (T-036.md:68, :12, :146, :162, AC-A4 at :204).**
- 2.3 says the checkout and the `setup-go` step stay "byte for byte". The lease rule's list of allowed template changes does not include their `uses:` lines.
- But the recommended Q-T036-5 (a) pins `actions/checkout` and `actions/setup-go` "in the template and the Action" by full SHA. The threat model makes "a nested `uses:` in the Action or template not pinned as Q-T036-5 rules" a must-fix.
- Under (a), whichever way the builder goes, a guard has a must-fix. AC-A4 checks no template `uses:` form, and AC-A3 checks only the Action's.
- Fix: in 2.3 and the lease rule, say that under Q-T036-5 (a) the two template `uses:` lines change to `@<sha> # vX.Y.Z` with the ruled SHAs and nothing else in those steps changes, and that under (b) they stay. Add the template `uses:` form to AC-A4.

## Notes (not blocking)

**N1. Release workflow (2.10, T-036.md:127): build outside the checkout and assert a clean stamp.** Verified with Go 1.27: an untracked file in the checkout makes the next `go build` stamp `vcs.modified=true` and a `+dirty` version. If `release.yml` writes the first archive or binary inside the checkout, every later platform build is "unpinned". `waggle init` then writes no workflow (2.4 rule 3) and `waggle upgrade` refuses, silently, for users of those binaries. Add to 2.10:
- write outputs under `$RUNNER_TEMP`;
- fail the job unless `go version -m` shows `vcs.modified=false` and `vcs.revision` equals the tag commit.

Also, with `go 1.23` in `go.mod` and no toolchain line, a 1.23 toolchain stamps no `Main.Version` from tags, so release pins are 40-hex. The "tag plus clean revision" case of 2.4 rule 1 arises only with Go 1.24 or later. State the release Go version.

**N2. Section 6 misses two `InitRepo` callers that move to rule 3.** `cmd/waggle/dispatch_test.go:59` and `internal/dispatch/fixture_test.go:91` call `InitRepo` with nil `BuildInfo`. A `go test` binary carries no VCS stamp, so after 2.4 rule 3 these fixtures have no `.github/workflows/waggle-gate.yml`. I found no assertion that depends on the file: `internal/dispatch/host_test.go:180` refuses `.github/workflows` by path, not by existence. `internal/dispatch` is outside the lease, so list both as "expected to pass unchanged, workflow now absent".

**N3. Section 6 details for the builder.**
- `cmd/waggle/upgrade_test.go:76` and `:80` require `errb == ""` and that stdout ends with the last line. The fixture is tag-only (`v1.0.0`), so 2.4 rule 2's new warning must go to `Out` and print before the last line. Say so in 2.4.
- `internal/scaffold/upgrade_test.go:173`: the scaffold `git` helper trims output (`internal/scaffold/uninstall_test.go:33`). The new workflow line sorts first and becomes `M .github/workflows/waggle-gate.yml`, and the config line becomes ` M .waggle/control/config.yaml`. That is more than "gains a line" (T-036.md:179).

  These are the details that cost T-034 its rounds.
- The rest of section 6 checked out against the files:
  - `guard_template_test.go:43-48` breaks as stated.
  - `init_test.go:400-405` breaks as stated.
  - `upgrade_test.go:137-138` and `:143` break as stated.
  - `dispatch_test.go:181-211` breaks as stated.
  - `gate_approver_template_test.go:16` finds the first `- name: Waggle gate`, and `token:` does not match `GITHUB_TOKEN: ${{ github.token }}`.
  - Exactly two `present == 'true'` still holds.
  - No test names `actions/checkout`, `setup-go`, `gate skipped` or `A release binary`.
  - No test asserts `github.Probe`'s error.

**N4. Phase B with `native_queue` prints contradictory output.** `Report.Enforcement` looks up `merge-queue` for `native_queue` (`internal/forge/report.go:52-71`). Doctor emits no such finding, so `Render` prints `Enforcement: advisory (merge-queue is not checked)`. Since nothing is missing, it then also prints "Everything Waggle relies on is enforced on this repo." (`report.go:89`, `:103-104`). Doctor exits 0. `report.go` is outside phase B's lease. Either emit a `merge-queue` finding (`unknown`, Fix naming the cut) or say in 2.9 that this output is accepted. Pin it in AC-B5's golden.

**N5. GitHub resolves every `uses:` in the job at "Set up job", skipped steps included.** As I understand runner behavior (not verifiable offline), the hive downloads `waggle-sh/waggle/gate@<pin>` on every run even though the step never runs, and a malformed `gate/action.yml` at the pin fails the job before the base-source gate runs. For any repo, a `GATE_REF` that is not on GitHub fails every run, the base-source path included. That covers a build from an unpushed local commit and a build made in a private fork. All of these fail closed. Add to 2.11 item 2 a check that the first post-pin run shows the Action resolved. Add to the G5 recipes a sentence that the binary running `upgrade` must be built from a pushed commit.

**N6. Private-repo Action access before G5.** Under D-235 the demo must work before the public flip. A private repo's Action is usable from other repos only when the repo's Actions access setting allows repositories in the organization. Add that setting to the 2.1 Head Keeper table for the G5 demo, or say the demo repo is this repo's fork. Q-T036-1 mentions only release downloads being blocked; (a) is blocked the same way.

**N7. The two scout calls made without asking.**
- Removing the shell tier step (2.7) is sound. It duplicates the gate's own `tiers` failures (`cmd/waggle/gate_verify.go:74-80` annotate per file with encoding), misses T-008's paths, and echoes unencoded path names into workflow commands. It also reopens nothing: D-249 kept it only because T-033 deferred the change. But it changes the hive's own workflow at the pin move, so name it in the Status line or as Q-T036-8. Then the ruling card, and later the pin-move card, show it.
- `internal/scaffold` importing `internal/decisions` is fine. `decisions` imports only the standard library, so there is no cycle. `Render(nil)` is exactly the three header lines `waggle decide` writes (`internal/decisions/render.go:12-26`). Reusing it avoids a drifting template copy.

**N8. `gate/` can go into `gate.protected_paths` right after phase A merges (2.11 item 4 at T-036.md:134).** Protected paths are enforced at lease time by the pinned dispatch binary and at merge by the base-built gate. Both have supported this since T-031 and T-032, so waiting for the pin move only leaves `gate/` leasable longer. Also, `cmd/waggle/` and `internal/` are what the Action actually runs, so protecting only `gate/` is a narrow fence. That is fine as a note, since users pin by SHA and the Head Keeper cuts releases.

**N9. The brief has no list of facts not checkable offline.** I propose this one.

Verified here:
- The pushed OID must be local (M2).
- Untracked files make the stamp dirty (N1).
- A git-less export of `e8a68eb` builds offline with `CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off GOFLAGS=-mod=readonly GOWORK=off`. The binary carries no VCS stamp, which does not matter: only `cmd/waggle/upgrade.go:15` and scaffold read build info.
- `go build -o` creates missing parent directories.
- `config.Parse` accepts `forge.remote` and empty `gate.*` scalars, and ignores `waggle:` (`internal/config/config.go:145-221`).
- git's atomic refusal text contains "does not support --atomic".

Not checkable offline:
- The runner downloads the whole repository tarball for `owner/repo/path@ref`.
- `${{ github.action_path }}` works in a composite step's `with:`.
- Skipped-step actions are resolved at setup (N5).
- How private-repo Actions are accessed (N6).
- Fork-network SHAs resolve under the parent repo.
- The shape of `rules/branches/{b}`, and whether it lists only active rulesets.
- The exact classic-protection 404 messages, and 403 or 404 for non-admins.
- Whether GitHub accepts `%2F` from `url.PathEscape` in both branch endpoints.
- Availability of tag rulesets, immutable releases and attestations on a private repo's plan.
- `gh release create --verify-tag --draft` behavior.
- How `setup-go` resolves a bare `go 1.23`.

**N10. Version skew (Q-T036-1, 2.4).** It is settled for the template path. Action SHA and `waggle.pin` come from one build and one `upgrade` commit. A newer key in base config fails closed with exit 3 (D-239). Dirty and non-git builds write no workflow. A module build pins by tag with a warning (rule 2), and that tag pin follows a moved tag for the Action even though sumdb protects the binary. Two skews remain and should be named in 2.11 item 8:
- hand-written workflows;
- a newer local CLI writing claims that an older Action reads (claims carry a `version` field, `internal/comb/backend.go:113`).

A tag name that is also a branch name is an ambiguous `@ref`. The tag ruleset covers that.

**N11. Line cite.** T-036.md:5 cites `.github/workflows/waggle-gate.yml:98-100`. That is right at `c9cb754`, but on main (`1037e77`) the build lines are 105-107.

**N12. AC-A10.** `waggle version` prints inline in `main()` (`cmd/waggle/main.go:80-81`), and `cmd/waggle` has no `main_test.go`. Say the builder extracts a small function for the test.

**N13. Pin-move cost.** Every later pin move now rewrites a `.github/workflows/` file, which takes a Head Keeper commit with workflow push rights. T-033's "after which upgrade reports [same]" no longer holds. Q-T036-2 should state this cost next to the 8 lines.

**N14. Budgets** follow D-185 and D-136. A: 170 doubled to 340, target 440, stop 610. B: 245 doubled to 490, target 640, stop 880. Both have planned cuts. My count puts the bottom-up nearer 210 for A once M1 is fixed and nearer 300 for B once M2 is fixed. That is still inside the targets after doubling, so no change.

**N15. Decisions.** I found nothing reopened. Checked:
- D-241 drop of `dispatch release` in G4;
- D-249 template follows the hive;
- D-258 G3 first;
- D-261 to D-267 untouched;
- D-235 public at G5, private until then, which Q-T036-1 and Q-T036-6 respect;
- the note's "pinned by tag and SHA": release tags exist, and the workflow pins by SHA.

Q-T036-1 to Q-T036-7 recommendations hold. Q-T036-7 (a) is the right call: `pull_request_target` would reverse T-035's ban. The edited-workflow bypass predates G4 and needs only an equal-capability write.

Verdict: must-fix

## Round 2 (34bf972)

Risk-Tier: 3

Confirming guard on the fix round. Read `git diff e8a68eb 34bf972` (one file, `.waggle/hive/tasks/T-036.md`, +114 / -72) and the full brief at `34bf972`, checked against `internal/scaffold/init.go`, `upgrade.go`, `uninstall_test.go`, `init_test.go`, `init_models_test.go`, `upgrade_test.go`, `upgrade_models_test.go`, `internal/config/runtimes.go`, `internal/forge/report.go`, `internal/decisions/render.go`, `cmd/waggle/main.go`, `upgrade_test.go`, `status_test.go`, `uninstall_test.go`, `dispatch_test.go` and `internal/dispatch/fixture_test.go`. Two scratchpad experiments (below). Product code is unchanged since `c9cb754`, so every cite holds on main `1037e77`.

### Intent summary

The fix round closes the three round 1 must-fixes (and fable's two, which are the same defects) inside the brief, with no change to scope or leases. A full `init` over a gate-only config now refuses before any write unless `--runtime none` is passed. `upgrade` on a gate-only footprint skips steps 2 to 6 for every file but the workflow. `--gate-only` in a full hive creates only a missing `DECISIONS.md`. The doctor push probe checks the tip object locally first. The template's two GitHub action lines change only in their `@` suffix, with SHA placeholders the queen fills before dispatch. The round also folds in the notes: gate-only `DECISIONS.md` as a template instead of a product import, a rule 3 refusal for `--gate-only`, a `merge-queue` finding, the release build outside the checkout, a facts list, and a new Q-T036-8 on the history rewrite. Budgets rise to A 392/510/705 and B 526/685/945. Blast radius is unchanged from round 1.

### Must-fix closure

**M1 (fable M2): closed.**
- 2.5 now refuses a full `init` with runtime `claude-code` when the seeded config exists and `topKeys` finds no `runtimes`. The refusal comes before any write. That matches the code path it guards: `LoadPolicy` at `init.go:310-313`, seeded skip at `init.go:266-268`, and `ParseRuntimes` failing at `runtimes.go:34-37`.
- `topKeys` (`upgrade.go:598`) is in the same package, with regex `^([A-Za-z_][A-Za-z0-9_-]*):` at `upgrade.go:44`. It sees `runtimes:` in `config.yaml.tmpl:24`, ignores a commented line, and needs no `internal/config/` change. The lease justification says so.
- Only `claude-code` and `none` pass `InitRepo`'s runtime check (`init.go:208`), so keying the refusal on `claude-code` covers every policy-loading runtime.
- `TestInitFailsWithoutLevels` (`init_models_test.go:41-52`) seeds a config that has `runtimes:`, so the new refusal does not fire there and its error text still holds, as section 6 says.
- With `--runtime none`, no policy load runs and every other file writes as today. AC-A6's expected tree is right.
- The follow-on claim is right too: after that, `upgrade` with `claude-code` refuses at step 6 with `ErrRefused` from `planUpgrade` before `execute` (`upgrade.go:528-531`, `:360-371`).

**M2 (doctor push probe): closed.** 2.9 step 3 adds `git cat-file -e <oid>^{commit}` after the `--no-write` exit and before the notice line. Absent gives `unknown` with the fetch Fix and no push. AC-B3 and AC-B5 pin both. Threat-model Doctor bullet adds "without the local object check" and "a fetch by doctor".
- Scratchpad check: present gives exit 0; an absent OID gives exit 128 (`fatal: Not a valid object name`); a tree OID gives 128. Any nonzero maps to the same `unknown`, so the builder needs no exit-code table.

**M3 (fable M1): closed.**
- 2.3 has a new "Pinned under Q-T036-5 (a)" bullet: only `@v4` and `@v5` become `@<sha> # vX.x`. The lease rule now lists "the `@` suffix of the `actions/checkout` and `actions/setup-go` `uses:` lines", and "kept byte for byte" now names only the `with:` and `if:` of those steps.
- AC-A4 adds the template form.
- The SHAs reach the builder through the queen, by Depends on, 2.1 and 2.11 item 1.
- The template today has exactly these lines at `waggle-gate.yml.tmpl:43` and `:91` (`:25` goes with the relay job). No test pins either.

**Gate-only upgrade, steps 2 to 6 skipped: closed.**
- 2.6 now says that, for every file but the workflow, no managed render, stale removal, block, policy load or shim step runs, whatever `--runtime` says. Every precondition stays.
- The workflow template has no `{{PROFILE}}`, `{{FORGE}}` or `{{COMB_MODE}}` placeholders (grep: none). So the missing `hive:` keys in a gate-only config cannot make step 2 render it differently from `init`.
- AC-A7 tests both runtimes.
- No existing test deletes `roster.json` or `roles/` (grep for `os.Remove` and `RemoveAll` in the scaffold and cmd upgrade tests: none). So the footprint rule changes no current test.

**`--gate-only` in a full hive: closed.** 2.5 and AC-A6 now say it creates only a missing `DECISIONS.md`, and writes nothing when the file is present. `templateFiles` (`init.go:69-87`) is an explicit list, not a walk of `templates/`, so the two new templates do not leak into a full `init`. `templatesA` copies the whole embedded tree, so test FS overrides carry them.

### Section 6 re-check for what the diff touches

- `upgrade_test.go:173`: the corrected string is right. The `git` helper trims (`uninstall_test.go:33`), `.github/...` sorts before `.waggle/...`, and the config entry regains its leading space. The other pinA-to-pinB tests are `upgrade_models_test.go:24-60`, `upgrade_test.go:236-259`, `:376-420` and `:498-528`. They assert by `wantLines` or by specific files, not by rewrite counts or exact status. So the extra workflow `[rewrite]` breaks only `:137-138` and `:143`, as listed.
- `InitRepo` callers: the brief corrects fable's round 1 note. `cmd/waggle/status_test.go:79` and `cmd/waggle/uninstall_test.go:80` do pass a `v1.0.0` `BuildInfo`; only `cmd/waggle/dispatch_test.go:59` and `internal/dispatch/fixture_test.go:91` pass nil. Scratchpad check: a `go test` binary in a clean committed module reports `Main.Version (devel)` and no `vcs.*` settings. Those two fixtures therefore move to rule 3 on every host and on product-ci, as the brief says.
- `TestResolvePin` (`init_test.go:337-379`): its unpinned init asserts only the pin text and the existing warning. The new skip warning is additive.
- `topKeys`: no test calls it directly. `TestUpgradeNotesMissingConfigKeys` (`upgrade_test.go:261-273`) is a full hive and keeps its one note.
- Version function: `main.go:80-81` prints inline, and no test pins `0.1.0-bootstrap` or `waggle v`. AC-A10 now names the extracted function.
- `merge-queue` finding: `Enforcement` (`report.go:53-71`) gives `advisory (merge-queue is unknown)` when the finding is `unknown`. `NotEnforced` lists any `unknown`, so the "Everything Waggle relies on is enforced" line is not printed (`report.go:103-104`). That matches 2.9 and AC-B5 exactly, with `report.go` unchanged.
- `gate-decisions.md.tmpl`: `Render(nil)` is exactly `header` (`render.go:12-26`). The equality test imports `internal/decisions` from a scaffold test only, and that package imports only the standard library, so there is no cycle.

No missing breaking or pinning test found.

### Q-T036-8, budgets, facts list, contradictions

- **Q-T036-8 is framed fairly.** D-235 makes the history rewrite and the demo both prerequisites of the flip without ordering them, so ordering the demo after the rewrite is a new call and correctly asked. (b)'s cost (a destructive step pulled forward while redaction can grow) is stated plainly, and nothing relies on the unverified old-SHA behavior.
- **Budgets** follow D-185 and D-136.
  - A: the parts sum to 196; doubled 392; times 1.3 is 509.6, so 510; times 1.8 is 705.6, so 705.
  - B: the parts sum to 263; doubled 526; times 1.3 is 683.8, so 685; times 1.8 is 946.8, so 945.
  - Both are rounded to the nearest 5, as in round 1. The planned cuts are unchanged and still fit. "One ticket about 920, stop near 1,650" is 918 and 1,652.
- **Section 10.** Every "verified" item I can check offline holds:
  - git push needs the object locally (my round 1);
  - untracked files make the stamp dirty;
  - the offline git-less build;
  - `config.Parse` behavior;
  - `ParseRuntimes` at `runtimes.go:34-37`;
  - `comb/git.go:757`;
  - `Render(nil)`.

  Every cite in the new text resolves: `init.go:310-313` and `:266-268`, `upgrade.go:356` and `:528-531`, `gate_verify.go:205`, `backend.go:113`, `uninstall_test.go:127, 218`, `host_test.go:180`. The fable doc items are labeled with their source. Unverifiable items sit under "Queen checks".
- **No new contradiction found.** The Status line, lease rules, 2.1 table, 2.11 items, threat model and ACs agree on: the template `@` change, the template-not-import `DECISIONS.md`, rule 3 refusing `--gate-only`, the `merge-queue` finding and `gate/` protected right after phase A. The added lines contain no em or en dash and no home path or email.

### Notes (not blocking)

- **R2-N1. Partial clones.** In a partial clone, `git cat-file -e` on a missing OID can lazily fetch it from the promisor remote. That would quietly break "doctor never fetches". It is harmless: one object from the user's own remote, and no ref changes. The builder can set `GIT_NO_LAZY_FETCH=1` (git 2.44 or later) in the probe's env, or the brief can scope "never fetches" to refs.
- **R2-N2. Q-T036-8 (a), the admin-bypass hedge.** A `pull_request` run takes its workflow from the PR's merge commit. So the post-rewrite pin-move PR's own check names the new ref and should resolve. The admin bypass is more likely needed for PRs opened between the force push and that merge than for the pin-move PR itself. One clause would say so.
- **R2-N3. Symlink precondition on a gate-only upgrade.** "Every precondition stays" includes `checkSymlinks` over `writePaths(..., "claude-code")`, which walks `.claude/agents`. A gate-only repo whose own `.claude/agents` is a symlink then refuses an upgrade that would never write there. This fails closed, is rare, and the user's workaround is `--runtime none`.

Verdict: pass
