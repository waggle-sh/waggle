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