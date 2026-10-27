<!-- waggle-review v1 verdict=pass head=16275eb6c51c5bb5b061c632c541e65d9846c22d -->

**Risk-Tier: 2**

**Intent Summary:** Phase A of T-041 adds the license rule rows and comment forms that let `site/` files exist without failing `TestRepoHeaders` in product-ci. One signed-off commit on `t-041-license` changes only `internal/license/rules.go` and `rules_test.go`. It adds an exempt row for the four copied or fetched asset paths, a checked `site/**` Apache-2.0 row after it, a `Block` CSS comment form, base-name and extension cases in `formOf` for `.gitignore`, `.env.example`, `go.mod`, `.ts`, `.mjs`, `.astro` and `.css`, and the `.astro` frontmatter fence rule in `Check`. Blast radius: the `formOf` widening applies to every checked row, but the repo tracks no file of those kinds outside the root (root `.gitignore` and `go.mod` stay exempt by the earlier row), so no tracked file changes result today. The rows are inert until the asset PR lands. The check is a compliance check, not a security boundary.

**Checks run (all from the worktree at the head):**

| Check | Result |
| :--- | :--- |
| `go build ./...` | pass, no output |
| `go vet ./...` | pass, no output |
| `gofmt -l .` | empty |
| `go test -race ./...` | all packages ok, `internal/license` included |
| `TestRepoHeaders`, `TestWantRows`, `TestWantErrors`, `TestCheck`, `TestCheckNamesOffender`, `TestMatchSegs` | all PASS (verbose run) |
| `git status --porcelain` | clean, no probe files |
| Tracked `.css`, `.ts`, `.mjs`, `.astro` files | none |
| Tracked `.gitignore`, `go.mod`, `.env.example` | root `.gitignore` and `go.mod` only |

**Diff against brief 2.8, item by item:**

- **Item 1, rows.** Two rows sit right after the `LICENSE`/`NOTICE`/root-files exempt row (`rules.go:52-59`), so the JSON row and the exempt row precede the checked one. The exempt row names exactly the four patterns of 2.8 and nothing broader, which satisfies threat item 5.6. The table comment names T-041. No existing row changed.
- **Item 2, `Block`.** Added after `HTML` with existing iota values unchanged, documented as a CSS comment; `Line` returns `/* ... */` for it.
- **Item 3, `formOf`.** `CODEOWNERS`, `.gitignore`, `.env.example` give `Hash`; `go.mod` gives `Slash`; `.ts`, `.mjs`, `.astro` join `.go` under `Slash`; `.css` gives `Block`. The `.tmpl` trim is untouched.
- **Item 4, `Check`.** For a path ending in `.astro`, line 1 must be exactly `---`, else the error is the exact string the brief specifies. With the fence present, line 2 goes through the unchanged header checks, so a wrong identifier still gives `wrong SPDX header`, and the `Slash` blank-line rule lands on line 3. Shebang and CRLF code paths are unchanged. No existing error string changed; `TestCheckNamesOffender` still pins the original wording.
- **Item 5, tests.** All fifteen new `TestWantRows` rows match the 2.8 table (seven exempt, `SOURCE.md` HTML, four Slash, one Block, two Hash); root `.gitignore` and `go.mod` rows remain. `TestWantErrors` gains `site/src/x.svg` and `site/notes.txt`. The seven `Check` cases are all present and pass.

**Lease, commit and budget:** only the two leased files change (`git diff --stat`). The single commit has subject `T-041: license rows and comment forms for site/` and a `Signed-off-by` matching the author email. Net non-test lines added to `rules.go`: 27, under the estimate of 50 and the target of 65, so no planned cut applies.

**Notes, not blocking:**

- The `Check` doc comment (`rules.go:155-157`) still says the header is line 1 or line 2 after a shebang and does not mention the `.astro` fence. Harmless; a one-line comment update in a later round if anyone touches the file.
- A `.astro` file whose line 1 is `---\r` reports the fence error rather than the CRLF error. Both fail the check, so this is a message nuance only.

**Blocking Issues (Must-Fix Only):** none.

Verdict: pass