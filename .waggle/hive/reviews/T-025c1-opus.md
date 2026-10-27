<!-- waggle-review v1 verdict=pass head=8d4d361d71d2c2b4606a57a32f182171287f15b8 -->

# Guard review: T-025 phase C1 (`t-025c1`, head `8d4d361`)

Risk-Tier: 2

**Intent Summary:** This is the C1 header sweep. One commit, `8d4d361`, adds `// SPDX-License-Identifier: Apache-2.0` and a blank line as the first two lines of all 194 Go files under `cmd/` and `internal/`. It also adds `# SPDX-License-Identifier: MIT-0` as line 1 of `internal/scaffold/templates/local.yaml.tmpl`. Nothing else changes: no file is deleted or renamed and no code changes, so the risk is mechanical. Two things affect people beyond this PR. The template is code-owner gated, so the PR needs the Head Keeper's code-owner review (T-015). New hives will get the MIT-0 line in their seeded `local.yaml`; existing hives won't, because `upgrade` never rewrites seeded files. The planned cut didn't fire, so there's no C1b. Every brief cite into a Go file now shifts down by 2 lines (checklist item 6).

**Checks:**
- **AC4 numstat:** `git diff --numstat` against base `d0e81c8` gives 195 files: 194 Go files at `2 0` and `local.yaml.tmpl` at `1 0`. Nothing outside `cmd/` and `internal/` is touched.
- **AC4 content:** across `git diff -U0` I checked:
  - All 195 hunks read `@@ -0,0 +1,2 @@`, or `+1` for the template.
  - All 584 `+` lines are one of the 195 `+++ b/` file lines, 194 Apache-2.0 headers, 194 blank lines, or the single MIT-0 header.
  - All 195 `-` lines are `--- a/` file lines.
  - So there is no change other than the headers.
- **AC4 coverage:** there are 194 Go files under `cmd/` and `internal/`, and all 194 start with the header followed by a blank line. Every Go file is covered.
- **Build-constrained files:** in files like `reap_other.go`, the header sits above `//go:build` with a blank line between. `go vet` (buildtag analyzer) is clean and the tests pass on darwin.
- **Commit:** the subject starts `T-025:`, and the `Signed-off-by` line matches the author identity.
- **AC6:** `gofmt -l .` printed nothing; `go build ./...` and `go vet ./...` are clean; `go test -race ./...` passes in every package (`layout` has no tests).
- **AC8:** the diff contains no U+2013 or U+2014.

**Blocking Issues (Must-Fix Only):** None.

Verdict: pass