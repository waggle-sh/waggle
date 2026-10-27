<!-- waggle-review v1 verdict=pass head=8deec62ee592ab082f56428ccf376d63b6026fd7 -->

Risk-Tier: 3

**Intent Summary:** This is phase B of T-025, the Head Keeper commit `8deec62` on top of `b16c98a`, which already contains the merged C1 sweep. It changes 8 files (+11 −5) and matches brief section 6 items 1 to 7 exactly:
- **Code owners:** `CODEOWNERS` gains `/LICENSE`, `/NOTICE` and `/LICENSES/` for the Head Keeper, written like the existing lines (Q-T025-10).
- **Switched to Apache-2.0:** line 1 of `CODEOWNERS`, `ci.yml` and `release.yml` (D-129), and line 1 of `gate/action.yml` (D-333). The same commit changes the `TestGateActionText` prefix at `internal/scaffold/gate_action_test.go:49` (line 47 before C1 added its two lines).
- **New MIT-0 headers:** `# SPDX-License-Identifier: MIT-0` goes in as line 2, after the shebang, in `.claude/hooks/session-start.sh` and `.waggle/control/scripts/scan-hive.sh`. Both keep file mode 100755.
- **Module list:** `WAGGLE.md` section 2 now lists `internal/license/`.
- **Left alone, as the brief says:** `CLAUDE.md` (exempt, D-334). The only workflow changes are the header lines, with no logic edits.

Blast radius: header lines and code-owner routing only. No behaviour changes, apart from the test assertion that follows the Action's new first line. The commit carries a `Signed-off-by` that matches its author. Every Tier 3 path now carries the header its section 4 row requires (rows 9 to 13), so C2's repo check has nothing to flag there.

Evidence:
- `go build ./...`: clean.
- `go vet ./...`: clean.
- `gofmt -l .`: prints nothing.
- `go test -race ./...`: every package `ok`, including `internal/scaffold` (12.184s).
- No other Go test checks for an MIT-0 line. The only remaining MIT-0 string in Go is the `internal/decisions/render.go:14` const, which the brief keeps on purpose.
- No U+2013 or U+2014 in the touched trees.
- No CRLF in the diff.

**Blocking Issues (Must-Fix Only):** None.

Two optional notes:
- The new line in `WAGGLE.md` section 2 sits before `internal/layout/`, but in alphabetical order it belongs after it.
- The comment at the top of `CODEOWNERS` (lines 2 to 5) explains the rule only for templates and README, not for the new legal-file entries.

Verdict: pass