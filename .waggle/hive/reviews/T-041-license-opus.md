<!-- waggle-review v1 verdict=pass head=16275eb6c51c5bb5b061c632c541e65d9846c22d -->

Risk-Tier: 2

**Intent Summary:** This is phase A of T-041: one signed-off commit (`16275eb`) that changes only `internal/license/rules.go` (+30/−3) and `internal/license/rules_test.go` (+24). `rules.go` gets two new rows, placed right after the root-files exempt row. The first exempts the four copied or fetched asset patterns (`site/public/**`, `site/src/assets/brand/tokens.css`, `.../diagrams/**`, `.../mark/**`). The second makes the rest of `site/**` checked as Apache-2.0. The JSON exempt row still comes first, so `site/package.json` and the lockfile stay exempt. It also adds a `Block` form (`/* ... */`) after `HTML` and returns it from `Line`, leaving the existing form values alone. `formOf` gains base-name cases (`.gitignore` and `.env.example` give `Hash`, `go.mod` gives `Slash`) and extension cases (`.ts`, `.mjs` and `.astro` give `Slash`, `.css` gives `Block`). `Check` gains the `.astro` fence rule. If line 1 is not exactly `---` you get the 2.8 item 4 error text. Otherwise line 2 goes through the existing checks, and the Slash blank-line rule applies at line 3. The table comment now names T-041.

Blast radius: every PR's product-ci, through `TestRepoHeaders`. No tracked file changes result: the only tracked `.gitignore` and `go.mod` are at the root and still match the exempt row first (`git ls-files` finds no `.ts`, `.mjs`, `.astro`, `.css` or `.env.example` files). The widening of `formOf` beyond `site/` is the deliberate one that 2.8 describes. The diff matches 2.8 items 1 to 5 exactly:
- **Test rows:** `TestWantRows` adds all 15 listed paths with the right result and form, and the existing root `.gitignore` and `go.mod` exempt rows remain.
- **Error cases:** `TestWantErrors` adds `site/src/x.svg` and `site/notes.txt`.
- **Check cases:** all seven are present. They are CSS ok, CSS with the wrong form, `.astro` ok, `.astro` header on line 1, fenced `.astro` with the wrong identifier, `.astro` with no blank line 3, and `.ts` with no blank line 2.
- **Unchanged:** no existing row, test case or error string changed. The diff only adds lines, apart from the `formOf` `if` becoming a `switch`, which behaves the same.

That puts about 27 non-test lines against a target of 65.

**Evidence (at the head, in this worktree):**
- `go build ./...`: no output.
- `go vet ./...`: no output.
- `gofmt -l .`: no output.
- `go test -race ./... -count=1`: every package `ok`, including `internal/license` (1.247s), with no push-tripwire failures.
- `git log`: the single commit is signed off with the author's email (`27632860+paulternate@users.noreply.github.com`).

**Blocking Issues (Must-Fix Only):** None.

**Non-blocking notes:**
- The doc comment on `Check` (`rules.go:155-157`) still says the header is "line 1, or line 2 when line 1 starts with `#!`". It doesn't mention the `.astro` fence.
- A CRLF `.astro` file (`---\r` on line 1) gets the fence error instead of the CRLF error. It still fails, so nothing goes unchecked.

Verdict: pass