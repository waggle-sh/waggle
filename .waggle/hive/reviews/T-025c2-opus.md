<!-- waggle-review v1 verdict=pass head=79ae6bae4b2f008b852f2c5360552f3ef5afaed6 -->

Risk-Tier: 2

**Intent Summary:** This change adds one new package, `internal/license/`, at 168 non-test lines (estimate 270, target 300, stop 400). Three files are new: `rules.go`, `repo_test.go` and `rules_test.go`. `rules.go` holds the brief's section 4 table as ordered data, with row 5 folded into row 3 as D-334 requires. It provides `Want` and `Check`, a `Line` helper and a path matcher that understands `**`. `Check` reads only the header line: line 1, or line 2 after a `#!` shebang. For Go files it also requires a blank line after the header. It rejects a CRLF header line and an empty file. `repo_test.go` checks every file listed by `git ls-files -z`, reports all offenders in one failure, and skips if git is missing, if the directory is not a checkout, or if the git top level differs from the module root after `EvalSymlinks`. The blast radius is every `go test ./...` in this repo: from now on, a tracked file with a missing or wrong header, or at a path no rule covers, turns CI red. Nothing changes in `waggle gate` or in any hive.

**Evidence**
- **Base:** `1c5da8a` already contains C1 (`8d4d361` via #150) and phase B (`8deec62` via #153), as checklist 2.1 items 4 and 5 require. Phase B added the `internal/license/` entry to `WAGGLE.md` section 2.
- **Diff scope:** `git diff --name-status` shows only `A internal/license/{repo_test.go,rules.go,rules_test.go}`, which meets AC5. Numstat is 73, 168 and 139 added lines, with no deletions.
- **Commit:** `79ae6ba`'s `Signed-off-by` matches the author email. All three files carry the Apache-2.0 header on line 1 followed by a blank line.
- **Build, vet, format:** `go build ./...`, `go vet ./...` and `gofmt -l .` all produce no output.
- **Full race run:** `go test -race ./... -count=1` exits 0 and every package passes, including `internal/dispatch` (222s) and `internal/license` (1.3s). `internal/layout` has no test files. This meets AC6 at this head.
- **License tests:** run verbosely, `TestRepoHeaders` ran and passed (0.08s; it did not skip), along with the five other package tests. They also pass with `-count=20`.
- **Phase B tests:** `TestGateActionText`, the assertion phase B changed, passes.
- **Dashes:** there are no U+2013 or U+2014 characters in `internal/license/`, which meets AC8.

**Spec conformance (section 5)**
- **Matcher:** it is case-sensitive, `*` stays within one segment, and `**/` matches zero or more segments. Tests cover `**/*.json` against `a.json` and `a/**/b` against `a/b`, and `roster.json.tmpl` lands on row 2.
- **Rule order:** the first match wins. Paths that fall through every row give `unknown path`, and a checked file with an unsupported extension gives `unknown extension`.
- **Tests cover the brief's list:** each row, shebang handling, the blank line after a Go header, a header above `//go:build`, a wrong identifier, an SPDX line later in a file being ignored, CRLF, an empty file, an unknown path, an unknown extension, and zero-segment `**`.
- **Error message:** the "missing SPDX header" message matches the brief's example exactly (`TestCheckNamesOffender`).
- **Repo walk:** it only runs `git rev-parse` and `git ls-files -z` and reads files; it never writes, fetches or pushes.

**Blocking Issues (Must-Fix Only):** None.

**Notes (not blocking):**
1. **`.tmpl` extensions:** `formOf` strips any `.tmpl` suffix, so `x.go.tmpl` and `x.sh.tmpl` would get a form. The brief lists only `.yml.tmpl`, `.yaml.tmpl` and `.md.tmpl`, and says any other extension is an error. No such template exists today, and the form chosen would match the rendered file's syntax, so this is a small deviation from the wording only.
2. **Row identity not tested directly:** `TestWantRows` checks each path's result (identifier, form, checked), not which row it matched. Two reorders it would not catch are harmless, because the result is the same. The 4.1 inventory's `.waggle/control/*.json` (row 2 before row 12) has no representative path in the test. `TestRepoHeaders` would still catch a reorder there on the real tree, since `.json` would fail with "unknown extension".
3. **Skipped entries:** `repo_test.go` skips tracked entries that are not regular files: symlinks, submodules, and files deleted from the working tree. So a symlink at a path no rule covers would not report "unknown path". No symlinks are tracked today, and the threat model (D-203) makes this a note.
4. **AC3 evidence:** the unit tests cover the three failure cases (a removed header, a swapped header, an unknown top-level path). AC3 asks for evidence of the repo check failing in each of the three cases. The queen should confirm the builder's run report includes it.

Verdict: pass