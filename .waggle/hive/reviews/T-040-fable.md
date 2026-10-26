<!-- waggle-review v1 verdict=pass head=5c27ffbcbbc451bab11f4d2693c427594dcea99c -->

Risk-Tier: 3

**Intent Summary:** One commit on `t-040` rewords 35 user-facing strings so a gate-only user sees no bee name: gate findings (rows 1, 2, 4), gate usage, claim and release errors and sentinels, the `--owner`, `--comb-mode`, `--runtime`, `--allow-downgrade` and `decide` help, the `printUsage` tagline, the `waggle status` header, the upgrade and uninstall last lines, the gate workflow step name and two gate-config comments. It also adds `fs.PrintDefaults()` to the `claim` and `release` usage closures (Q-T040-7 a) and makes `printDecision` take a `gateOnly` flag so a gate-only repo gets a decision input with no T-016 or D-041 and an empty ticket list, while a full hive keeps today's block byte for byte. No exit code, JSON code, sentinel identity, gate rule, path, trigger, permission or `run:` line changes. Blast radius: every installed gate workflow takes the step rename at the next pin move, and the hive gate prints the new row 1 to 4 texts from the next PR; everything else is text.

**Verification run on the head:**

| command | result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | no output |
| `go test -race ./...` | all packages ok, including the remote-mode fixtures in `internal/dispatch`, `internal/comb` and `internal/forge` |

**Checked against the brief:**

- **Lease:** all 27 changed files are under the five leased directories; the only non-test files touched are the ones the lease rule names, and in each only the named text plus the two `fs.PrintDefaults()` lines, the `gateOnly` field, its setter in `planUpgrade` and the `printDecision` signature. Nothing in `internal/config/`, `internal/forge/`, `.waggle/`, `.github/`, `go.mod` or any other template.
- **Rows 1 to 28 and 30 to 36:** each "exact" text matches the table byte for byte (placeholders and order unchanged). Row 1 stays distinct from the listed-path text at `changecheck.go:96`. Row 2 keeps the `HasPrefix` test and the literal prefix (Q-T040-1 a). Row 3's doc comment and row 4's comment at `gate.go:19` match. Rows 7 and 8 change only the `errors.New` text, so `errors.Is` identity holds and the sentinel tests in section 6 pass unchanged. Row 29 is absent, as round 4 requires.
- **Templates:** the workflow diff is the single step-name line; the `run:` line and every other byte are unchanged. The gate-config diff is the two comment lines; no YAML value changes. `TestGateTemplatesPlainTerms` pins both on a rendered `--gate-only` init.
- **printDecision (section 4, AC6):** gate-only branch sets rows 21 to 25 and `tickets` to an empty slice; `TestUpgradeDecisionInputGateOnly` decodes the block with `DecodeInput`, asserts no `T-0`, `D-0`, `Keeper`, `hive` or `queen` substring, and runs `decisions.Next` against the repo's log. `TestUpgradeDecisionBlockFullHiveGolden` pins the full-hive block and `Run:` line to today's text. The existing `TestUpgradeDecisionInput` is untouched.
- **Changed existing assertions:** all eleven are on section 6's expected-to-break list and change only the reworded substring: `decide_test.go:95` and `:106` (rows 26, 28), `gate_bind_test.go:135` and `:239` (rows 2, 1), `gate_verify_test.go:198` (row 2), `upgrade_test.go:74` in `cmd/waggle` (row 18), `git_test.go:967` (row 9), `policy_test.go:42` (row 1), `dispatch_test.go:235` and `release_test.go:174` (row 6, keeping `ExitClaim`, `errors.Is` and the claim-kept checks), and the four `status_test.go` lines (rows 33 to 36). No `t.Skip` added. No existing test needed a change for the new `claim`/`release` flag listing.
- **AC10 and AC10b:** `TestHelpPlainTerms` captures `gate`, `claim`, `release`, `decide` and `upgrade` `-h` and rejects `Keeper`, `queen`, `Cells`, `waggled`, `hive` outside the log path, and en or em dashes. `TestStatusGateOnly` builds a gate-only repo with a local bare `origin` and asserts exit 0 with the row 34 and 36 lines, plus exit 1 with the row 33 text outside a repo.
- **AC11:** no en or em dash anywhere in the five leased directories. Remaining bee-word string literals in the changed files are all section 2 keeps: the `waggled` socket messages (`uninstall.go:88`, `upgrade.go:322`), `ErrNotHeadKeeper` (`backend.go:176`), the full-hive `printDecision` texts (`upgrade.go:700-709`), the `.waggle/hive/` paths in `printUsage` (`main.go:120-121`) and the literal `Head Keeper: ` prefix quoted inside row 2. The rest are comments and identifiers.
- **Budget:** about 53 added non-test lines by numstat, under the 118 estimate and the 153 target.

**Not verified here:** the PR body (AC12: the three `Waggle-` trailers, the changed-assertion list and any found-but-unchanged string) is not in the repo and `gh` is outside the allowed Bash forms. The Keeper should confirm it before merge.

**Notes (not must-fix):**
- Row 3's rewrapped paragraph leaves its last line noticeably shorter than the others; the brief marks line wrap as a note.
- The `ErrWaggleDir` doc comment at `backend.go:153` still says "hive included"; the lease rule allowed changing it but did not require it, and it is a comment.

**Blocking Issues (Must-Fix Only):** none.

Verdict: pass
<!-- waggle-review v1 verdict=pass head=b4886b067854b6bea9b830827f69b8403b375eed -->

Everything passes. The worktree is clean and I wrote no probe files.

Risk-Tier: 3

**Intent Summary:** Fix round 1 on `t-040` adds one commit (`b4886b0`) on top of the full build at `5c27ffb`. It touches only four `*_test.go` files inside the lease (`internal/comb`, `internal/decisions`, `internal/dispatch`, `internal/scaffold`), 42 lines added and 1 changed, and no non-test file. It closes the opus guard's must-fix by adding the missing AC4 and AC5 tests and takes the optional note on AC8. Blast radius is test coverage only. The product strings, exit codes, sentinels, gate rules and templates reviewed in the first round are untouched by this commit.

**What the commit does, checked against the fix-round brief:**

- **Must-fix 1, row 4:** `TestVerifyTransitionCorruptBaseText` in `internal/decisions/gate_test.go` feeds the same one-byte-truncated base that the existing `corrupt base` case uses, asserts `errors.Is(err, ErrCorrupt)`, the substring `repair needs an admin merge`, and no `Head Keeper`. The old text (`repair needs a Head Keeper admin merge`) does not contain the new substring, so a revert fails the test.
- **Must-fix 1, rows 7 and 8:** `TestSentinelPlainTerms` in `internal/comb/plain_terms_test.go` compares `ErrWaggleDir.Error()` and `ErrNotOwner.Error()` by whole-string equality against the row 7 and row 8 texts, as the brief required (not `errors.Is`). Both match `backend.go:154` and `:167`.
- **Must-fix 2, row 5:** `TestReleaseTaskUnmergedText` in `internal/dispatch/task_test.go` reuses the `unmerged` setup from `TestReleaseTask` (`newFixture`, `releaseClaim`, `branchWith` on `t-023`) and asserts the exact row 5 tail from `release.go:158`. Local repo only, no push. The old tail (`or release it with --unmerged on a Keeper card`) fails it.
- **Optional note, row 30:** `TestGateTemplatesPlainTerms` now requires the line after `- name: Fetch claim refs` to be exactly `run: git fetch origin 'refs/waggle/claims/*:refs/waggle/claims/*'`, matching `waggle-gate.yml.tmpl:29-30`.
- **No existing assertion changed** other than the tightened AC8 line, which the brief invited. No `t.Skip`, no em or en dash, no bee name in added test names or comments.

**Verification run on `b4886b0`:**

```
go build ./...        (clean)
go vet ./...          (clean)
gofmt -l .            (empty)
go test -race ./...   all packages ok (dispatch 220s, cmd/waggle 53s, comb 29s)
go test -race ./... -run 'TestSentinelPlainTerms|TestVerifyTransitionCorruptBaseText|TestReleaseTaskUnmergedText|TestGateTemplatesPlainTerms' -v
  all four PASS
```

No remote-mode fixture failed in this run, so the push tripwire did not trigger here.

**Done criteria:** one new commit on `t-040` on top of `5c27ffb`, subject `T-040: tests for rows 4, 5, 7, 8 and the exact claim-fetch step line`, signed off; all four checks pass; each new test fails on a revert of its row text by inspection of the old strings in section 3.

**Blocking Issues (Must-Fix Only):** none.

**Notes (not blocking):** this round changes no code that sends signals, deletes state, or touches leases or claims, and adds no non-test lines, so under D-205 one confirming guard suffices.

Verdict: pass