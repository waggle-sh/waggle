# T-014 phase B review (guard, fable), PR #36, branch `t-014b`

**Verdict: PASS WITH FIXES**
**Risk-Tier: 3**

Reviewed at `009f749` (commits `43bf7e3`, `fa5a341`, `009f749`) against the brief `.waggle/hive/tasks/T-014.md` and D-089, D-107, D-114. Reviewed independently of the sonnet guard.

`go build ./...`, `go vet ./...`, `go test -race ./...` pass (`media/T-014b/fable-build-test.log`). Repro material: `media/T-014b/fable-probe_test.go.txt` (a throwaway test file dropped into `cmd/waggle/`, uses the PR's own harness) and `media/T-014b/fable-probes.log` (its output).

## Blocking findings

None. Nothing fails open, every lease boundary holds, and no attacker-chosen text reaches output unencoded. The fixes below are small and should land either as one more `T-014:` commit before the bypass merge or in the next brief that touches these files (D-107); F1 to F3 are the ones I would do before merge.

## Fixes (non-blocking, numbered)

**F1. Empty-subject commit breaks the rev-list parser (`cmd/waggle/gate_commits.go:53-75`).** `git rev-list --format=%s` omits the format line entirely when the expansion is empty (git prints it only `if (buf.len)`), so an empty-message commit drops a line and the two-line pairing breaks. One such commit gives `verify: error: git rev-list: odd line count 1`, exit 3, instead of the spec's exit 1 "commit has no T-<id>: prefix". Two such commits keep the count even and misalign every pair: the second empty commit is never diffed, and the commit before it is reported with the next header line as its subject. This fails closed in every arrangement I could build (a header line `commit <oid> ...` never matches `^T-...: `, so at least one commit fails leases), but it is the wrong exit class and a confusing message. Repro: `fable-probes.log`, `TestFableEmptySubjectPairs` (plumbing `commit-tree` with empty stdin, exit 3; two `--allow-empty-message` commits, exit 1 with the mis-paired subject). Fix: make the format line non-empty, for example `--format=.%s` and `strings.TrimPrefix(lines[i+1], ".")`, and add a test with a `commit-tree` empty-message commit expecting exit 1 and the no-prefix message.

**F2. Workflow bootstrap exits 0 when `BASE_SHA` itself is missing (`.github/workflows/waggle-gate.yml:80-83`).** `git cat-file -e "$BASE_SHA:cmd/waggle/gate.go" 2>/dev/null || exit 0` treats "base commit not in the clone" the same as "file absent on base". With `fetch-depth: 0` that needs `base.sha` to have become unreachable between the event and the checkout (base branch rewound), so it is narrow, but it is the one path in the step that exits 0 on an error. Fix: `git cat-file -e "$BASE_SHA^{commit}" || { echo "::error::base $BASE_SHA not in clone"; exit 1; }` before the file probe. Everything else in the step fails closed: the shell is `/usr/bin/bash -e {0}` (confirmed in the PR's run log), `git worktree add` and `go build` failures fail the step, `exec` passes the gate's exit code through, and the `|| true` on the claim fetch is gone.

**F3. Test 8 breaks the day T-008 lands (`cmd/waggle/gate_verify_test.go:165-178`).** `strings.Contains(out, "Tier 3 path") == (p == "GO.MOD")` asserts that `GO.MOD` is not Tier 3. T-008 adds `go.mod` to the Tier 3 list and its lease is `internal/comb/`, not `cmd/waggle/`, so T-008's PR fails this test with no way to fix it inside its lease. Fix now: key the expectation on `comb.RequiredMinTier(p) == comb.Tier3ControlPlane` instead of the path name, or give T-008 a one-line lease on this test file.

**F4. The workflow runs from the PR's merge ref, so a PR that edits `waggle-gate.yml` runs its own edit.** The brief's section 6 says a PR touching a Tier 3 path "always fails verify". That holds for every Tier 3 path except the workflow itself: GitHub runs `pull_request` workflows from the merge commit, so a PR can replace the `Waggle gate` step with `exit 0` and the required check goes green. Building the checker from `BASE_SHA` closes the Go side (confirmed: the PR's run built from `12a4372` and printed `waggle gate verify not present on base`), but not the YAML side. `go.mod`, `go.sum`, `go.work`, `vendor/`, `replace` directives and `.gitattributes` are all read from the base worktree, and `setup-go`'s `go-version-file` only chooses a toolchain from an official download (a bad version fails the job), so I found no build-side hole. The YAML hole is a platform property, not something this PR can fix. It needs a line in T-015's ruleset work: CODEOWNERS on `.github/workflows/**` with "require review from code owners", or a push ruleset restricting that path to bypass actors. Until then the real control for workflow edits is the Head Keeper reading the diff before the bypass merge, which is what section 6 already relies on. Worth a decision entry so nobody reads section 6 as stronger than it is.

**F5. `comb.CheckChanges` needs a stated precondition before T-023 calls it from a worktree (`internal/comb/changecheck.go:40-83`).** The API shape is right for both callers: pure, takes `Lease` and `[]Change` with both modes, returns typed violations with JSON tags, and `RequiredMinTier` cleans its input. Two gaps for a caller that does not get its paths from `git diff-tree`: (a) `leaseMatch` does no cleaning, so `docs/../.github/workflows/x` matches lease `docs` while the tier rule says Tier 3; git never emits such a path, but a worktree walker or `git status` parser might. Either document "paths are canonical repo-relative slash paths as git prints them" or add a guard (`path.Clean(p) != p`, leading `/`, empty, or `\` on any OS gives a `RuleOutsideLease` violation with detail "path is not canonical"). (b) An empty `NewMode` silently skips the symlink and submodule rule; a `git status` based caller would have to `lstat` to fill it. Say so in the doc comment. Neither affects the gate, whose paths and modes come from `diff-tree --raw -z`.

**F6. Case collision check misses two shapes (`cmd/waggle/gate_commits.go:193-215`).** The key is `strings.ToLower(p)` but the rule is `strings.EqualFold`; they differ for a few runes, for example U+017F (`ſ.md` vs `s.md`, EqualFold true, ToLower unchanged), so that pair passes `tiers` (probe output in `fable-probes.log`). A file `Docs` next to a directory `docs/` also passes, since only blobs are listed. Both are academic for this hive; the first is cheap to close by keying on a fold like `strings.ToUpper(strings.ToLower(p))` or by comparing against every seen path with the same length after folding.

**F7. Small semantic notes, no action needed.** Merge commits `continue` past the tier rules (`gate_commits.go:120-123`), so a merge touching `.github/workflows/` reports only the merge failure; exit is 1 either way. `gateReport.fail` dedupes on (check, path), so a head tree with `.github/workflows/A.yml` and `.github/workflows/a.yml` reports the Tier 3 failure and drops the collision message for the same path; still exit 1 and the path is named. A `T-0001:` subject is accepted against `ticket-1` through `ParseTicketID`, which is fine. A `\r` inside a ticket subject is kept by `%s` and never echoed. Submodule add and submodule bump both fail with mode `160000`; unicode paths come through `-z` as raw bytes regardless of `core.quotePath`; mode-only changes are counted as paths. All verified in `fable-probes.log`.

**F8. `cmd/waggle/main.go:90` usage line is stale** ("v0.1: 'gate decisions' ..."). Outside this phase's lease; fold into the next brief that leases `main.go`.

**F9. AC7 has no evidence and cannot have any before merge.** The checker is built from base, and base lacks `gate_verify.go` until this PR merges, so the two scratch PRs (out-of-lease edit, `GO.MOD` edit) only prove anything after the bypass merge. The Keeper should open them right after merging and drop the screenshots in `media/T-014/`.

**F10. Q-T014-4's recommended "planning ticket with a full claim on `.waggle/hive/tasks/`" is not possible today.** `validateLeaseFields` rejects any directory under `.waggle/` (`internal/comb/backend.go`, `isWaggleDir`), so no claim can ever cover hive records. With this gate every brief, decision and review commit fails `leases` and merges by bypass. That is also why test 7's "full claim, exit 0" half is unreachable through the gate (see deviations). Needs a Keeper decision: allow full-gate claims on `.waggle/hive/` in comb, or accept bypass merges for all hive-record commits.

**F11. The dispatch brief cites D-136, which does not exist.** `DECISIONS.md` on both `main` and this branch ends at D-133.

**F12. No `Signed-off-by` trailers on the three commits.** D-131 and D-132 want DCO sign-off; the `dco` job is not in `ci.yml` yet, so nothing enforces it today. Not blocking.

## Rulings

**Size, 510 vs 450.** `git diff --numstat` added non-test lines: `gate.go` 25, `gate_commits.go` 215, `gate_verify.go` 176 (cmd/waggle total 416), `changecheck.go` 83, `git.go` 11 (comb total 94), 510 in all. The 350 target and 450 stop were estimated for `cmd/waggle/` alone; the 94 comb lines are scope the Keeper added after the estimate (D-114's shared `CheckChanges`, D-107's `lease.json` cap). The part the stop was written for is at 416, under 450. The stop's purpose is to let the architect split `tiers` out; `tiers` is about 40 lines here (`caseCollisions` plus the tier branch), so a split would save nothing. Recommend accepting at 510 under the D-089 precedent (T-018 phase A at 527 against 350), recording that the overrun is exactly the D-107 and D-114 additions. Keeper decides.

**Test 7 unreachable.** Confirmed: comb rejects `.waggle/` in any claim, so the gate can never see a full claim on `.waggle/hive/tasks/`. The light half is tested through the gate (exit 1, tier message present, `tiers: ok` absent), the full half in `comb.TestCheckChanges` ("tier 2 full"). Accepted. See F10 for the policy consequence.

**Test 8 substitution.** `go.mod` joins the Tier 3 list only with T-008, which is not merged. The test uses `.WAGGLE/control/waggle.md` for the `waggle.md` case and asserts `GO.MOD` fails leases but not tiers. Accepted for now; see F3 for the forward-compatibility fix.

**Merge commits fail `leases` only.** The brief's table does not name a check for merge commits; rule 7 ("non-ticket commits are not exempt from tiers") is not literally applied to merges. Exit code is unaffected. Accepted, noted in F7.

**`main.go` usage stale.** Outside the lease; accepted, F8.

## Acceptance criteria

- AC1 pass (build, vet, race tests).
- AC2 pass: forager commits touch `cmd/waggle/` plus the comb files D-107 and D-114 assign to this phase; the Head Keeper commit touches only the workflow; `go.mod` unchanged.
- AC3 over the stop; see the size ruling.
- AC4 pass with the two documented substitutions (tests 7 and 8); every 4.2 row and 4.3 and 4.4 rule has a test.
- AC5 pass: test 15 plus my own probe; `%`, `\r`, `\n`, `:` and `,` encoded in `file=`, `%`, `\r`, `\n` in data; subjects go through `%q` then the encoder.
- AC6 pass: built from `BASE_SHA` in a separate worktree, job has only `contents: read`, claim fetch failure is fatal. Caveat F4 (workflow YAML) and F2.
- AC7 not met, deferred to post-merge (F9).
- AC8 pass: `relay-claim` keeps `contents: write`, `pull-requests: write`, `issues: write` at job level; workflow-level block is `contents: read`.

## Intent Packet

Phase B adds `waggle gate verify|leases|tiers` on top of T-007's `gate decisions`: every commit in merge-base..head must be a `T-<id>:` commit whose paths sit inside its claim directories with exact case and no new symlink or submodule; Tier 3 paths and Tier 2 paths under a light claim fail; the head tree is checked for case collisions; all three checks run and report every failure with GitHub-encoded annotations. The lease and tier rules live in the pure `comb.CheckChanges` that T-023's dispatch will reuse (D-114), and `loadClaims` now rejects a `lease.json` over 64 KiB (D-107). The Head Keeper commit flips the workflow to `contents: read`, makes the claim fetch fatal, and builds the checker from `BASE_SHA`.

Blast radius: every PR to `main` from now on fails unless every commit carries a ticket prefix and a live claim; hive-record commits and Tier 3 commits always fail and merge by Head Keeper bypass (F10). Nothing fails open; the three code fixes (F1 to F3) are each under ten lines and change no semantics. F4 needs a ruleset decision in T-015.
