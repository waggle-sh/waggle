<!-- waggle-review v1 verdict=pass head=e0cea3e797ece39814348df4c84a78c9e91a1dc3 -->

Everything checks out. The `Enforcement:` line is printed by `internal/forge/report.go:91`, the remote URL forms match `remote.go:14`, the claim `--base` default matches `claim.go:185`, the README has no `<<`, and the worktree is clean with no probe files.

Risk-Tier: 3

**Intent Summary:** T-047 adds one user doc, `docs/setup.md` (180 lines, Apache-2.0 header), that a coding agent follows to install Waggle Gate on a user's repo with `init --gate-only`, open the install PR, stop for a person to merge and create the ruleset, then run `waggle doctor` and report what doctor does not check. It adds a README subsection under Install with the pinned `refs/tags/v0.1.0` prompt and the 8.1 warning, one Status sentence saying the prompt has not been run with any agent, and a drift test `cmd/waggle/setup_doc_test.go` that re-runs its own test binary with `<sub> -h` to parse every doc command against the real flag sets, pins one tag, checks the quoted names against `release.yml` and the `layout` and `forge` constants, bans shell pipes, `--force`, `--admin`, `gh pr merge`, claim and release lines and `main` URLs in `sh` fences, and runs `checkPlain`. No product code changes. Blast radius: the doc is code execution with a user's credentials once a tag pins it, which is why it is Tier 3 by content; in the repo today the only effect is the new test.

**Verification run:**

```
go build ./...        clean
go vet ./...          clean
gofmt -l .            empty
go test -race ./... -run 'TestSetupDoc|TestHelpPlainTerms|TestLicense' -v -count=1
  TestSetupDocCommandsParse  PASS (6.20s, reached init, doctor, claim, release, uninstall)
  TestSetupDocOneTag         PASS
  TestSetupDocQuotedNames    PASS
  TestSetupDocForbidden      PASS
  TestSetupDocPlainTerms     PASS
git diff --stat base..head   README.md +15, cmd/waggle/setup_doc_test.go +336, docs/setup.md +180
git status --porcelain       clean
```

**Checked against the brief:**
- Lease rule: exactly the three permitted files, all additions. README diff adds one `### Set up with an agent` subsection and one Status sentence; no existing line changed, no heading renamed, no `<<`.
- Every `sh` and `text` command in Steps 1 to 10 matches section 4 byte for byte, placeholders aside. The 8.1 warning, Step 7 block, Step 8 report block and section 5 rules block are verbatim, with the gate spec link on the following line in both doc and README.
- MUST NOT list complete; pauses only at Step 7 and Step 9; every step has a Success clause; all 16 failure-table rows present, including the non-GitHub origin, `[skip]`, dirty tree, merge queue, 403 and no-`gh` stops.
- No `main` URL, one tag (`v0.1.0`), no decision or ticket ID outside `T-<id>` and `T-900`, no bee words or dashes. The doc says the pins prove consistency and the Action pin, not that the binary is genuine or that the setup binds the agent.
- Cites hold: `remote.go:14` URL forms, `claim.go:185` base default, `report.go:91` Enforcement line, `forge.DefaultGateCheck`, `layout.GateWorkflow`, `layout.Config`, `layout.Decisions`.
- Test safety: the child runs in `t.TempDir()` with only `<sub> -h`; `init` uses `flag.ExitOnError` and the others `ContinueOnError`, so all exit before any write, and the test passes with exit 0 for all five. Nothing is pushed, signalled or deleted. Planted-value checks exist for every test.

**Blocking Issues (Must-Fix Only):** none.

**Notes, not must-fix (threat bar D-203):**
1. The README copies the warning verbatim as the brief requires, so it opens "This file is instructions your agent follows", where "this file" reads as the README rather than `docs/setup.md`. A queen or Head Keeper wording call, not a builder one, since the brief fixed the text.
2. T1's flag match `^\s+-<name>\b` would also accept a longer flag with the same prefix, for example `-base-branch` for `--base`. A loosening only; every flag the doc uses exists today.
3. `TestSetupDocHelper` calls `main()` in-process whenever `WAGGLE_SETUP_DOC_ARGS` is set, so a developer with that variable exported would run it in the parent. Unlikely and harmless with `-h`.
4. T1's planted case checks the regex and the extractor separately rather than feeding a planted doc through the full check. AC4's intent is met for T2 to T5 end to end and for T1 in parts.
5. The brief's failure table had a third "Why, from the code" column; the doc drops it for plain terms. The user-facing instruction in every row is intact.

Verdict: pass

<!-- waggle-review v1 verdict=pass head=d30d31adb0c2e98b9c7898e1c895802ef1aa867a -->

Risk-Tier: 3

**Intent Summary:** Fix round 1 of the T-047 build, one commit `d30d31a` on `t-047`, touches only `README.md` and `cmd/waggle/setup_doc_test.go`. It makes the three section 18 changes: the T2 test now reads the README whole, counts the prompt text, extracts the URL from the first prompt line and compares it for equality with the pinned `refs/tags/v0.1.0` URL, with eight new planted rows; the README warning's link line becomes `See [L11](docs/gate.md#10-limits-and-known-bypasses).`; and the README warning's first sentence opens with `docs/setup.md is instructions`. `docs/setup.md` is byte-identical to `e0cea3e`, so no fenced command, stop rule, MUST NOT line or human step changed. Blast radius: one test file and two README lines. No product code, no `internal/`, `gate/`, `.github/` or `.waggle/` change. Hive effect at merge is unchanged from the first review.

**Verification run (all at `d30d31a`):**

| Check | Result |
| :--- | :--- |
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | no output |
| `go test -race ./...` | all packages ok, including `internal/license` |
| `go test -race ./... -run 'TestSetupDoc\|TestHelpPlainTerms' -v ./cmd/waggle` | T1 to T5 pass |
| `git diff e0cea3e d30d31a --stat -- docs/setup.md` | empty |
| `git status --porcelain` | clean |
| Commit | signed off, subject prefix `T-047: ` |

**Item 1 (opus M1), checked against the code at `setup_doc_test.go:129-170` and `:269-293`:**
- `promptRe.FindStringSubmatch` reads the `(\S+)` group and `url != want` is an equality test (`:146-151`, `:165-168`). The old `strings.Contains` and `readmePrompt` are gone.
- `strings.Count(readme, promptText) > 1` counts the literal text, not regex matches, so an indented or trailing-word second line is caught (`:143-145`); rows "second, indented" and "second, trailing" cover both.
- Prefix and suffix rows are full prompt lines and fail on "prompt URL". The `asset version` and `branch URL` rows are now full prompt lines and each fails for its own reason. Each row now asserts the problem text names the intended reason, which is stricter than the old any-problem check.
- The real README holds the prompt text exactly once, at `README.md:48` (grep across the repo outside `.waggle/` finds only that line and the test constant).
- The checks are on a function taking text, so the rows drive the real check. The test still writes, pushes and signals nothing; the `-h` helper path is untouched.

**Item 2:** `README.md:43` is exactly `See [L11](docs/gate.md#10-limits-and-known-bypasses).`, the same form the README's doctor line uses at `README.md:36`. Not a URL that follows `main`; it is a relative path, and only the pasted prompt at `:48` is a pinned URL.

**Item 3:** `README.md:42` opens `docs/setup.md is instructions your agent follows with your credentials.` and the rest of the paragraph matches section 8.1 word for word. No test compares the README warning to the doc, and T4 and T5 still read only `docs/setup.md`, so nothing weakened.

**Lease and scope:** both touched files are inside the lease (`README.md`, `cmd/waggle`). No other file changed in this round. The branch's full diff from `416e457` is still only `README.md`, `docs/setup.md` and the test file. Branch point is `416e457`, not Base `f3ea86c`; the brief says nothing under the lease paths differs between them, and the targeted and full test runs pass on this tree.

**Threat bar (D-203):** no finding. The round adds no network endpoint, download, credential handling, permission or change to the human step. The test change narrows what a README edit can slip past, which is the point of M1.

**Blocking Issues (Must-Fix Only):** none.

**Notes, not must-fix:**
1. The "trailing word" row relies on `(\S+)$` failing when text follows the URL. It does, since the URL has no whitespace to split on, and the test proves it, so this is only a comment on why the row passes.
2. `README.md:42` now begins with a bare path in prose. The T-041 site renders the README as markdown, so it stays plain text, which the ruling intends.

Verdict: pass