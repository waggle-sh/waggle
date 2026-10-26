<!-- waggle-review v1 verdict=must-fix head=52450efda9a55e50e34681d17ea05be008c3c6d2 -->

Risk-Tier: 2

**Intent Summary:** A single commit (`52450ef`) rewrites `README.md` in plain terms around Waggle Gate: the pitch, the five checks, what the gate does not do, install, writing a claim, recipes, status, docs and license. It adds `docs/gate.md`, the gate spec, which has a claim map with code cites, limits L1 to L12, a config reference, install and `waggle doctor`, and the full calling workflow. It adds `docs/recipes/` with an index, a Gas Town requirements and status section, and recipes for Claude Code agent teams and Copilot cloud agent. It moves the Roles and Terms glossary into a new section 4 of `CONTRIBUTING.md`, with "Cut or frozen" entries and the stale facts corrected. It also adds SPDX headers: Apache-2.0 on every touched file, CONTRIBUTING switched from MIT-0, and one added line 1 in `docs/RFC-0003.md`. No Go code changes. The risk is in what readers will believe the gate enforces. Hive records that point to the README glossary now need to read CONTRIBUTING.

**Evidence checked**
- AC1: the diff touches only `CONTRIBUTING.md`, `README.md`, `docs/RFC-0003.md` (+1 line, line 1, nothing else), `docs/gate.md` and the three `docs/recipes/*.md` files. All are inside the lease rule. 457 lines added, under the 1,680-line target.
- AC2: I opened the cited code at the base and the cites hold: `gate_commits.go` 16-23, 33-36, 51-64, 199-207, 222-233, 244-253, 260-310, 315-338, 343-369 and 373-395; `changecheck.go:79-104`; `backend.go` 29-53 and 458-544; `gate_verify.go` 65-94, 125-181, 225-229 and 258-296; `gate_approve.go` 39-92, 107-121 and 175-242; `gate.go` 22-28, 51-56 and 147-158; `doctor.go` 25-81; `probe_github.go:66-73`; `dispatch/claim.go:42-65`; `claim.go` 178, 182, 186 and 247; `gate/action.yml`. The tests I cited exist at the lines given (`TestBaseConfigFaultsExit3` at `gate_bind_test.go:255`, plus `branches_test.go` 202/215/237 and `claim_branches_test.go:28`). The five README checks match brief 3.1. Who writes a claim is stated as a requirement on the user, not as a check. The listed-path finding is quoted with its approval gloss (`gate.md:50`). A local run with `--branch` sets `sameRepo=true` (`gate_verify.go:154`), so the hook recipe's command is sound.
- AC3: the vocabulary Grep hits are all inside code spans: paths (`internal/comb/...`, `.waggle/hive/...`), the `Head Keeper: ` literal, `keeper_id`, and `Waggle Gatekeeper` inside the template YAML.
- AC4: no U+2013 or U+2014 in any changed file.
- AC5: every anchor matches a GitHub heading slug, and the external links are section 4 sources.
- AC6: the YAML in `gate.md` is the whole template with only `{{GATE_REF}}` and `{{PIN}}` replaced by `<commit-sha>`.
- AC7: no Waggle SHA or tag appears in the docs; pins use `^{commit}` / `^{}`; no doc says a release is published.
- AC9: L1 to L12 are all present. The README covers L2, L1, L3, L5 (as mitigated, not fixed), L6 and L11. `native_queue` is not presented as merge-queue support.
- AC10: the README License section differs only by the dropped "and the `waggled` daemon". CONTRIBUTING section 6 no longer claims a CI check runs.
- `go build ./...`, `go vet ./...` and `gofmt -l .` all produce no output. I did not run `go test -race ./...` because the diff changes no Go file.

**Blocking Issues (Must-Fix Only)**

- **M1: `docs/gate.md:100` says init refuses more than the code checks.** The line says `waggle init --gate-only` refuses "a build with no pushed commit or release tag to pin the Action to (`:426-429`)". The code (`gateRef`/`resolvePin`, `internal/scaffold/init.go:105-155, 426-429`) only refuses a build with no clean `vcs.revision` and no release tag. Nothing checks whether the commit was pushed; the message is `this build has no commit or release tag to pin the gate Action to` (`:162`). A reader would expect init to catch an unpushed build, and it does not. L10 in the same file says so correctly ("an unpushed commit renders an Action ref that fails at job setup"). Fix: replace "pushed commit" with "clean commit", or quote the `:162` text.
- **M2: `README.md:41-44` prints the brief's shorthand as a usage block.** Brief 3.1 says this shorthand "is for this brief only: the docs never print it as usage, they copy the literal usage lines from `cmd/waggle/claim.go`". The README prints `waggle claim T-<id> --dirs a,b --gate light|full --owner <id> [--branch <name>]...` and `waggle release T-<id>` as the section's usage. The release line leaves out `--owner`, which the same section says is required in a gate-only repo, so a reader who copies it gets a failure. Fix: replace the block with the literal usage lines at `cmd/waggle/claim.go:178` and `:243`, or turn it into worked examples that include `--owner`.

**Notes (not blocking)**
- L6, L7 and L11 in `docs/gate.md` give no source cite. The brief asks for "each with its condition and source", so add a short source to each.
- `README.md:58-59` has no blank line before `## Documentation`. It renders correctly, but nothing else in the file is written that way.
- The CONTRIBUTING "Lane" entry adds a reason the brief does not give ("The per-commit check already makes lanes safe, so no one needs them yet"). Consider a plain "Frozen: not used".
- README check 1 says a claim with no branch "fails every commit". The exact condition is every bound commit on a known head. That holds under Actions, but `docs/gate.md` states the narrower wording.
- PR-body items (AC3 hit list, AC7 section 2 value list, AC11 trailers and follow-ups) can only be checked on the PR. I reviewed the commit.

Verdict: must-fix