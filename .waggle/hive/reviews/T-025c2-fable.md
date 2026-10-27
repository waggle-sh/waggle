<!-- waggle-review v1 verdict=pass head=79ae6bae4b2f008b852f2c5360552f3ef5afaed6 -->

Risk-Tier: 2

**Intent Summary:** Phase C2 of T-025 adds the new `internal/license/` package: an ordered rule table mirroring brief section 4, a segment-wise matcher with `**` support, a `Want` lookup and a `Check` that reads only the header line, plus a unit table and a repo-walk test over `git ls-files -z`. Nothing else in the tree changes. Blast radius is every `go test ./...` run and the `product-ci` job: from this merge on, any tracked file under a checked path without the right SPDX line, or any file under a path no row covers, turns the build red until fixed or a row is added. The walk skips cleanly outside a git checkout and in a module copy inside another repo.

**Verification (all at head 79ae6ba, parent is the stated base 1c5da8a):**

| Check | Result |
| :--- | :--- |
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | no output |
| `go test -race ./...` | all packages ok, including `internal/license` |
| `TestRepoHeaders` with `-v` in this worktree | ran, not skipped, passed |
| Diff scope (`git diff --numstat`) | only the three files under `internal/license/`, 380 added lines, 168 non-test (target 300, stop 400) |
| Commit trailer | `Signed-off-by` present under the Head Keeper's identity |
| U+2013 or U+2014 in the new files | none |
| Phase B on the base | yes: WAGGLE.md module entry present, and the walk passes, so `gate/action.yml`, `scan-hive.sh` and `session-start.sh` already carry their headers |

**AC3 probe evidence.** I swapped, then removed, the header on `cmd/waggle/doctor.go` and ran the walk each time, then restored the file (`git status --porcelain` is empty and the walk passes again):

```
--- FAIL: TestRepoHeaders (0.08s)
    repo_test.go:71: 1 license offenders:
        wrong SPDX header: cmd/waggle/doctor.go: got "// SPDX-License-Identifier: MIT-0", want "// SPDX-License-Identifier: Apache-2.0"

--- FAIL: TestRepoHeaders (0.07s)
    repo_test.go:71: 1 license offenders:
        missing SPDX header: cmd/waggle/doctor.go: want line 1 "// SPDX-License-Identifier: Apache-2.0"
```

The missing-header message matches the brief's section 5 example byte for byte, and `TestCheckNamesOffender` pins it. The third AC3 case, a file under a new top-level path, I could not reproduce through the walk: making a file tracked needs `git add`, which is outside my allowed forms. It is covered at the unit level by `TestWantErrors` and the `unknown path` case in `TestCheck`, and the walk calls the same `Check`, so the integration follows.

**Spec conformance, section 4 and 5.** The rule table matches rows 1 to 13 in order, with row 5 folded into row 3 per D-334. Row 9 precedes row 10 so `waggle-gate.yml` stays MIT-0. `**/` matches zero or more segments and is tested on `a.json` and `a/b`. `formOf` trims a trailing `.tmpl` and handles `CODEOWNERS`, and any other extension on a checked path is an error. `Check` reads only line 1, or line 2 after a shebang, rejects a `\r`-terminated header line, rejects an empty file, enforces the Go blank line, and accepts a header above `//go:build`. The walk skips on missing git, outside a checkout, and when the git top level differs from the module root after `EvalSymlinks` on both sides. It only runs `rev-parse` and `ls-files`, never writes or fetches. The three files carry Apache-2.0 headers.

**Notes (not blocking, D-203):**

- **4.1 coverage gap in the unit table.** Section 5 asks for one representative path per 4.1 row. `TestWantRows` has no path for the `.waggle/control/devcontainer.json, roster.json` row. A reorder that put row 12 ahead of row 2 would still fail loudly in `TestRepoHeaders` on this repo with an unknown-extension error, so the stated purpose is met. Adding `.waggle/control/roster.json` to the table would close it in one line.
- **Non-regular tracked files are skipped silently** by the walk (symlinks, submodule entries). There are none today per 4.1. If a tracked symlink ever appears under a checked path, it will neither pass nor fail.
- **A file whose only content is the header with no trailing newline** passes the Go blank-line check because there is no line 2. Harmless, since such a file is not a valid Go package.

**Blocking Issues (Must-Fix Only):** none.

Verdict: pass