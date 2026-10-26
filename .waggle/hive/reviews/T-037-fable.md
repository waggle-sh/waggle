<!-- waggle-review v1 verdict=must-fix head=52450efda9a55e50e34681d17ea05be008c3c6d2 -->

# Review: T-037 docs build, head `52450ef` against base `7b946bc`

Risk-Tier: 2

**Intent Summary:** The build rewrites `README.md` around Waggle Gate, adds the gate spec `docs/gate.md`, three recipe pages under `docs/recipes/`, moves the Roles and Terms glossary into `CONTRIBUTING.md` with a cut-or-frozen list, and adds Apache-2.0 SPDX lines (one added line on `docs/RFC-0003.md`). Seven files change, all inside the lease rule; 457 lines added, under the 1,290 estimate. No Go, template, workflow or control file changes; build, vet, gofmt and the full race test suite pass on the head. Blast radius is the public statement of what the gate enforces, which is why the one must-fix below matters even though it is a single clause.

## Evidence

- `git diff --name-status base...head`: M `CONTRIBUTING.md`, M `README.md`, M `docs/RFC-0003.md`, A `docs/gate.md`, A `docs/recipes/README.md`, A `docs/recipes/claude-code-agent-teams.md`, A `docs/recipes/copilot-cloud-agent.md`. `docs/RFC-0003.md` shows `1 0` in numstat, the added line is line 1 (AC1).
- `go build ./...`, `go vet ./...`, `gofmt -l .`: no output. `go test -race ./...`: every package `ok` (dispatch 228s). `git status --porcelain`: clean.
- AC2: I opened every `file:line` cite in `docs/gate.md` and both recipes against the head tree. All name the right function and lines: `gate_commits.go` binding (33-36, 51-64, 69-90, 260-310), maintainer commit (265-268), base merge (199-207, 244-253), claim read and expiry warning (315-338), `checkHead` (343-369, 355-358, 368), limits (16-23, 173-174, 194-195), case collisions (373-395); `gate_verify.go` head resolution (130-155), merge-base (225-229), backend (230), base config (258-296), printing (65-94), verify order (172-181); `gate_approve.go` (39-42, 107-121, 107-158, 175-187, 189-242, 203-205); `gate.go` (22-28, 51-56, 81-129, 147-158); `changecheck.go` (79-100); `backend.go` (29-39, 43, 49-53, 88, 121-123, 458-474, 487-544); `git.go:601-605`; `config.go` (31, 76-83, 127, 207-212, 311); `init.go` (159, 391-397, 402-404, 426-440); `main.go:58-65`; `doctor.go` (25-34, 56-59, 76-81); `probe.go:36-41`; `probe_github.go` (66-73, 153-189); `claim.go` (182, 186, 247); `dispatch/claim.go` (42-45, 58-65); `gate_trailers.go:27-104`; `gate-config.yaml.tmpl:9-17`; `layout.go:32`; tests `gate_bind_test.go:255`, `claim_branches_test.go:28`, `branches_test.go:202, 215, 237-242`. No cite names a brief.
- Quoted program strings match the head byte for byte: the Tier 3 finding, the listed-path finding (always with the approval gloss), `path is outside the claim directories`, the no-branch and does-not-name findings, `pull request branch unknown`, the local warning, the maintainer-commit format, the no-ticket message, the `--owner` help, the init next-step line, `name: Waggle Gatekeeper`.
- AC6: the YAML block in `docs/gate.md` lines 113-184 is the 72-line template with only `{{GATE_REF}}` and `{{PIN}}` replaced by `<commit-sha>`; every other line, including the two action SHAs, matches `waggle-gate.yml.tmpl`.
- AC3 grep (case-insensitive, the brief's pattern) over README, `docs/gate.md`, `docs/recipes/`: 19 hits, every one inside a code span holding a path (`internal/comb/...`, `.waggle/hive/...`), the `keeper_id` key, the `Head Keeper: ` literal, or the YAML `name: Waggle Gatekeeper`. No bee word in prose, no decision or `T-0xx` id.
- AC4: no U+2013 or U+2014 in any changed file. AC7: no `<<` marker; the only 40-hex strings are the two pinned actions inside the copied template; the only `vX.Y.Z` is Gas Town's `v1.0.0` release link; pin instructions use `^{commit}` and `^{}`; no doc says a Waggle release is published.
- AC5: every external link is a section 4 source (C1-C7, G1-G6, T1-T3) or `developercertificate.org`; relative links and anchors (`#3-the-claims-branch-check`, `#gas-town-requirements-and-status`, `#install` and the rest) resolve to headings at the head.
- AC8: both recipes and the Gas Town section state only verified rows or the G6 and T3 fallback wording; T2 is cited as a reference harness in `docs/contrib-harnesses/`, the polecat sentence is quoted as a fragment; Copilot is named Copilot cloud agent with one "formerly" line; no logos.
- AC9: L1 to L12 are all in `docs/gate.md` section 10 with sources; the README's "What it does not do" names L2, L1, L3, L5 (mitigated, not fixed), L6 and L11; L12 says `waggle doctor` does not detect a merge queue; the config reference presents `native_queue` only as a doctor behavior.
- AC10: SPDX lines as 3.5 requires; CONTRIBUTING section 6 says the automatic check "does not run yet"; the README License paragraph differs from the base only by dropping "and the `waggled` daemon".

## Blocking Issues (Must-Fix Only)

**M1. `docs/gate.md` section 12, line 100, states a refusal the cited code does not make.** It says `waggle init --gate-only` refuses "a build with no pushed commit or release tag to pin the Action to (`internal/scaffold/init.go:426-429`)". The cited code calls `gateRef`, which reads only the build's `vcs.revision` and `vcs.modified` stamps (`init.go:131-156`); it has no way to know whether a commit is pushed and does not refuse an unpushed one. The same page's L10 (line 83) says the opposite, correctly: "an unpushed commit renders an Action ref that fails at job setup". The brief's section 2 wording is "a build with no commit or release tag". A reader could run `init` from an unpushed build believing it would stop them. Fix: drop "pushed" from that sentence (or say "no commit", matching `noRefReason` at `init.go:162`).

## Notes (not blocking)

- README "Writing a claim" (lines 41-44) prints the brief's shorthand synopsis rather than the literal usage lines from `cmd/waggle/claim.go:178` and `:243`, which the brief asked for. Every flag shown is real and the form works; the fix round can swap in the literal lines if the queen wants the rule kept.
- README check 2 states who writes the claim as a user requirement, as AC2 requires, but does not point at the L3 bullet; the "What it does not do" list carries it.
- README line 58 to 59: no blank line before `## Documentation`. Renders fine in GFM.
- CONTRIBUTING line 56 still lists Cell among "concepts Waggle invents" in the D-073 naming rule, while section 4 now marks Cell cut.
- `docs/gate.md` carries no release-archive line (the brief's budget counted about 6 lines for it); the README has the one line section 2 allows. Section 3.2's section list does not require it.
- AC11 and the PR-body parts of AC3 and AC7 (trailers, the section 2 value list, the listed AC3 hits, follow-ups) are not checkable from this worktree; the queen confirms them on the PR.

Verdict: must-fix