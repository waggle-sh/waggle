# Guard review: T-025 phase B, commit 8deec62 (PR #153), second guard (fable)

Risk-Tier: 3

**Intent Summary:** Commit `8deec62` on `t-025b` is the phase B Head Keeper commit from T-025 section 6. It adds `/LICENSE`, `/NOTICE` and `/LICENSES/` to `<repo>/.github/CODEOWNERS` for `@paulternate`, switches the SPDX line of `CODEOWNERS`, `ci.yml`, `release.yml` and `gate/action.yml` from MIT-0 to Apache-2.0 (D-129, D-333) with the matching `TestGateActionText` prefix change in the same commit, inserts `# SPDX-License-Identifier: MIT-0` as line 2 (after the shebang) in `session-start.sh` and `scan-hive.sh`, and adds the `internal/license/` lease line to `<repo>/.waggle/control/WAGGLE.md` section 2. Eight files, 11 insertions, 5 deletions, no behavior change to any script or workflow. Blast radius: license metadata on product config files, one extra owner-review requirement on three license files, and one new leasable module name. The commit subject is exactly the one section 6 prescribes and carries a `Signed-off-by` trailer.

## Findings

1. **note** (scope): the diff touches exactly the eight files items 1 to 7 name and nothing else. `git diff b16c98a 8deec62 --name-only` lists `.claude/hooks/session-start.sh`, `.github/CODEOWNERS`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.waggle/control/WAGGLE.md`, `.waggle/control/scripts/scan-hive.sh`, `gate/action.yml`, `internal/scaffold/gate_action_test.go`. `CLAUDE.md` is untouched (D-334). Each of `ci.yml`, `release.yml` and `gate/action.yml` changes only its line 1.

2. **note** (header lines exact): verified byte for byte with `cat -vet`. `<repo>/.github/CODEOWNERS:1`, `<repo>/.github/workflows/ci.yml:1`, `<repo>/.github/workflows/release.yml:1`, `<repo>/gate/action.yml:1` are each `# SPDX-License-Identifier: Apache-2.0$` (LF, no CR, no trailing space). `<repo>/.claude/hooks/session-start.sh:1-2` is `#!/bin/sh` then `# SPDX-License-Identifier: MIT-0`; `<repo>/.waggle/control/scripts/scan-hive.sh:1-2` is `#!/usr/bin/env bash` then `# SPDX-License-Identifier: MIT-0`. These match rows 10, 11, 12 and 13 of the section 4 table and the form `Check` will read (line 2 when line 1 starts with `#!`).

3. **note** (CODEOWNERS form and last-match-wins): the three new lines at `<repo>/.github/CODEOWNERS:13-15` use the same leading slash, the same padding to column 33 and the same handle as lines 8 to 12. All three targets are tracked (`git ls-files LICENSE NOTICE LICENSES` returns `LICENSE`, `LICENSES/MIT-0.txt`, `NOTICE`). None of the new patterns overlaps an existing pattern (`/LICENSE` is an exact root file and does not match `LICENSES/`), so last-match-wins removes no owner from any path previously covered. The only ownership effect is additive: PRs to the three license files now require the code owner's review.

4. **note** (scripts): both scripts keep mode `100755` in the index (`git ls-files -s`), `sh -n session-start.sh` and `bash -n scan-hive.sh` both exit 0. Adding a comment at line 2 cannot change either script's behavior.

5. **note** (tests): `go test ./internal/scaffold/ -run TestGateActionText -count=1` passes. The assertion is at `<repo>/internal/scaffold/gate_action_test.go:49` as the queen said (shifted by C1). Full `go vet ./...`, `gofmt -l .` (empty) and `go test -race ./...` all pass on the checkout.

6. **note** (no other assertion on the changed lines): `git grep` over `*.go`, `*_test.go`, `*.golden`, `*.txt`, `*.tmpl` and `internal/scaffold/templates/` finds no other test or fixture asserting the old MIT-0 line on any of the six changed files. The only SPDX asserts are `gate_action_test.go:49` (updated here), `internal/decisions/render.go:14` plus its `backfill.golden` (DECISIONS.md header, unrelated) and the template files' own line 1 (unchanged). The second `gate/action.yml` read at `gate_action_test.go:147` checks inputs and pinned SHAs, not the header. `waggle upgrade` is not affected: `internal/layout/layout.go:65` `Managed = []string{Roster, RolesDir, Devcontainer, GateWorkflow}`; `GateWorkflow` is `.github/workflows/waggle-gate.yml` (untouched), and neither `.claude/hooks/session-start.sh` nor `gate/action.yml` is managed or seeded, so nothing here shows as drift. The `hasManagedLine` check in `upgrade.go:458` is the waggle-gate.yml marker line, not the SPDX line.

7. **note** (future C2 pass on paths outside C1): walking inventory 4.1 after this commit, every row marked "phase B" is now fixed: `session-start.sh` (row 13, MIT-0 line 2), `CODEOWNERS`, `ci.yml`, `release.yml` (row 10, Apache-2.0), `scan-hive.sh` (row 12, MIT-0 line 2), `gate/action.yml` (row 11, Apache-2.0). The rows marked "ok" are confirmed unchanged by grep: `waggle-gate.yml` still MIT-0 (row 9), `.waggle/control/WAGGLE.md`, `config.yaml` and the five role files still MIT-0 (row 12). Exempt rows (`settings.json`, `.gitignore`, `CLAUDE.md`, `LICENSE`, `NOTICE`, `LICENSES/`, `go.mod`, the two control JSON files, testdata) need nothing. The remaining "C1" rows (`cmd/`, `internal/`, `internal/scaffold/templates/`) are inside the C1 lease and already merged. I see no path outside the C1 lease that the C2 repo check would fail on.

8. **note** (WAGGLE.md ordering, agrees with the opus note): `<repo>/.waggle/control/WAGGLE.md:19` places `internal/license/` between `internal/forge/` and `internal/layout/`. The existing list is alphabetical and `layout` sorts before `license`, so the new line is one slot early. Section 6 item 4 gives the exact line text but no position, so this is cosmetic. Worth one line swap in a later Tier 3 pass; not worth a fix round on its own.

9. **note** (CODEOWNERS top comment): `<repo>/.github/CODEOWNERS:2-6` still explains only the templates and README rationale (T-015) and does not say why the license files are owned. Item 1 asks for the entries "in the form of the existing lines" and nothing about the comment, so leaving it alone is the stricter reading of "nothing else". Optional, same as the first guard's note.

## Blocking Issues (Must-Fix Only)

None.

Verdict: pass
