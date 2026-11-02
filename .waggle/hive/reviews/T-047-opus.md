<!-- waggle-review v1 verdict=must-fix head=e0cea3e797ece39814348df4c84a78c9e91a1dc3 -->

Risk-Tier: 3 (by content; Tier 0 by path)

**Intent Summary:** Head `e0cea3e` is the first build of T-047. It adds `docs/setup.md` (180 lines), the pinned instructions an agent follows to install Waggle Gate. It also adds a `### Set up with an agent` README subsection, one README Status sentence, and `cmd/waggle/setup_doc_test.go` (336 lines, the T1 to T5 drift checks). No product code changes. The hive only gains the doc check in `go test -race ./...`. The blast radius is users: an agent runs this doc with the user's credentials once a tag pins it, so a wrong or tampered doc amounts to code execution. Non-test additions are 195 lines, under the 370 estimate.

## Blocking Issues (Must-Fix Only)

**M1. T2 does not check that the README prompt's URL is exactly the pinned URL (spec section 6 T2, AC4).**
- **Spec:** "the README prompt's URL is exactly `https://raw.githubusercontent.com/waggle-sh/waggle/refs/tags/<tag>/docs/setup.md`".
- **Code:** `cmd/waggle/setup_doc_test.go:153` uses `strings.Contains(prompt, want)`, and `readmePrompt` (`:231`) calls `FindString`, so the `(\S+)` group in `promptRe` is never read.
- **Effect (from reading the code; I did not run these):** both of these README prompts pass T2, and both send the user's agent somewhere other than the pinned file:
  ```
  Set up Waggle Gate by following instructions: https://evil.example/https://raw.githubusercontent.com/waggle-sh/waggle/refs/tags/v0.1.0/docs/setup.md
  Set up Waggle Gate by following instructions: https://raw.githubusercontent.com/waggle-sh/waggle/refs/tags/v0.1.0/docs/setup.md.evil
  ```
- **Why it blocks:** the shipped README prompt is correct today, so this is a test gap. But this URL is the one string users paste, and the planted rows at `:271-275` miss both shapes.
- **Fix:** compare the captured group for equality, and add a prefix row and a suffix row. Any other prompt line in the README should also be checked or refused, because `FindString` reads only the first.

## Queen item, not a builder must-fix

**AC9 evidence is missing.** Section 9 item 1 and AC9 require `media/T-047/dry-run.txt` before the first guard review. It is not in this tree, and `git log --all -- .waggle/hive/media/T-047` is empty in this worktree. It may exist uncommitted elsewhere. The queen should run or file it before merge, because this review has no recorded run of the doc's git and `init` steps at this head.

## Verified

| Check | Result |
| :--- | :--- |
| `go build ./...`, `go vet ./...` | Clean |
| `gofmt -l .` | Prints nothing |
| `go test -race ./... -count=1` | All packages pass, including `internal/license` and T1 to T5 |
| Lease rule | Only `README.md`, `cmd/waggle/setup_doc_test.go` and `docs/setup.md` changed. The README diff is one subsection plus one sentence, with no heading renamed |
| 8.1 warning | Word for word in the doc and README |
| Steps and table | Fenced commands, Step 7 and Step 8 blocks and the rules block match by reading, no fence-level diff. All 16 failure rows are present, and every step has a success criterion |
| Flag parsing | `claim` and `release` accept flags after the task id (`claim.go:150-171`) |
| Test safety | The helper runs only `<sub> -h`, from a temp directory |
| Quoted strings against code | Gate log lines, finding ids, `Enforcement:`, `Fix:`, `waggle/gate@`, `# waggle:managed` and the `refs/waggle/*` keep line all match. Asset directory and `SHA256SUMS` format match `release.yml:112, 141-149, 164`. `grep -F ' waggle_…'` matches the two-space sums line |
| AC6, AC7 | No decision id, stray ticket id, bee word or dash in the doc or added README lines, and no `<<`. No URL or fenced command names `main`; the only tag is `v0.1.0` |
| 8.5 must-fix list | No instruction tells an agent to merge, create a rule, write a claim, handle a credential, pipe to a shell or pass `--force` or `--admin` |

**Not run:** no doc command was run, because this guard has no `gh`, network or arbitrary shell. No probe files were written.

## Notes (not blocking)

1. **T1 flag match is prefix-tolerant.** The `\b` at `setup_doc_test.go:251` lets `--gate` on `init` pass against `-gate-only`, and `--no` on `doctor` pass against `-no-write`. Neither form is in the doc today. Anchoring on `(\s|$)` closes it.
2. **T1's planted row is split.** `:257-262` checks flag extraction and regex non-match separately, because the check is inline in the test. It never drives the real failure path, which is thinner than AC4's wording.
3. **Small test leniencies:**
   - `gateAddProblems` accepts a duplicated path, such as `A A B`.
   - `mainURLRe` misses `?ref=main` and `@main`.
   - T4 reads only fences with the info string `sh`, as specified, so a `bash` fence escapes it.
   - An unclosed fence is dropped silently.
4. **"This file" in the README warning.** The brief requires it verbatim, but in the README it reads as the README, and `docs/setup.md` is first named three paragraphs later.
5. **The README link line is not a link.** `docs/gate.md#10-limits-and-known-bypasses` follows the brief's form, but it renders as plain text run into the end of the warning paragraph, unlike the Markdown links around it.
6. **PR body link.** Step 6 says the PR body "repeats the warning at the top of this file". It does not say the full-URL link line goes with it, which the plain-terms rule in section 4 asks for.
7. **`<run-id>` in Step 6.** Its source is not stated, though an agent can read it from the `gh pr checks` output.
8. **Step 9's end state.** The agent is left on `t-900`, then told to delete it, with no instruction to switch back first.
9. **Test size.** 336 test lines against the expected 120 to 180. These do not count toward the budget.

Verdict: must-fix

<!-- waggle-review v1 verdict=must-fix head=e0cea3e797ece39814348df4c84a78c9e91a1dc3 -->

Risk-Tier: 3 (by content; Tier 0 by path)

**Intent Summary:** Head `e0cea3e` is the first build of T-047. It adds `docs/setup.md` (180 lines), the pinned instructions an agent follows to run `waggle init --gate-only`, open the install PR, stop for a person to merge and create the rule, then run `waggle doctor` and report what it does not check. It also adds a `### Set up with an agent` README subsection, one README Status sentence, and the T1 to T5 drift test `cmd/waggle/setup_doc_test.go` (336 lines). There are no product code changes, and the only effect on the repo today is the new test. The blast radius is users: once a tag pins this doc, a wrong or tampered copy is code execution with their credentials. Non-test additions are 195 lines, under the 370 estimate.

## Blocking Issues (Must-Fix Only)

**M1. T2 does not check that the README prompt's URL is exactly the pinned URL (section 6 T2, AC4).**
- **Spec:** "the README prompt's URL is exactly `https://raw.githubusercontent.com/waggle-sh/waggle/refs/tags/<tag>/docs/setup.md`".
- **Code:** `cmd/waggle/setup_doc_test.go:153` uses `strings.Contains(prompt, want)`. `readmePrompt` (`:231`) calls `FindString`, so the `(\S+)` group in `promptRe` (`:131`) is never read.
- **Effect:** both of these prompts pass T2, though each sends the agent somewhere other than the pinned file. This comes from reading the code; I did not run them, because this role writes no files in the repo.
  ```
  Set up Waggle Gate by following instructions: https://evil.example/https://raw.githubusercontent.com/waggle-sh/waggle/refs/tags/v0.1.0/docs/setup.md
  Set up Waggle Gate by following instructions: https://raw.githubusercontent.com/waggle-sh/waggle/refs/tags/v0.1.0/docs/setup.md.evil
  ```
- **Why it blocks:** the shipped prompt is correct, so this is a test gap rather than a live bad URL. But this is the one string users paste, and the planted rows at `:271-275` miss both shapes.
- **Fix:** compare the captured group for equality and add a prefix row and a suffix row. The fix touches only the test, so it needs one confirming guard under the brief's Risk Tier bullet.

I found this before reading the earlier reviews at Base. It is the same defect as M1 in `reviews/T-047-opus.md`, and head has not changed since that review, so it is still open. The fable review at the same head passed without flagging it.

## Verified

| Check | Result |
| :--- | :--- |
| `go build ./...`, `go vet ./...` | Clean |
| `gofmt -l .` | Prints nothing |
| `go test -race ./...` | All packages pass |
| Lease rule | Only `README.md`, `docs/setup.md` and `cmd/waggle/setup_doc_test.go` changed, all additions. No heading renamed, no `<<` in the README |
| 8.1 warning | One exact whole-line match in each of the doc and README, with the link on the next line as the brief specifies |
| Steps and table | Fenced commands, the Step 7, 8 and 9 text and the rules block match section 4 by reading. All 16 failure rows are present and every step has a success clause |
| 8.5 must-fix list | No instruction tells an agent to merge, create a rule, write a claim, handle a credential, pipe to a shell or pass `--force` or `--admin`. Nothing claims the setup strengthens the gate against the installing agent, and no run is claimed |
| Quoted names against code | These all match: `commit has no T-<id>: prefix`, the decisions line (`internal/decisions/gate.go:52`), `path is outside the claim directories`, `gate@` in the workflow template, `# waggle:managed`, `Enforcement:`, `Fix:`, and the asset layout and version line (`release.yml:112, 136, 141-149`) |
| Flag order | `claim` and `release` accept flags after the task id (`cmd/waggle/claim.go:150-171`) |
| Test safety | The helper runs only `<sub> -h`, from a temp directory. It pushes, signals and deletes nothing |
| AC6, AC7 | No decision ID, stray ticket ID, bee word or dash. `T-900` appears only in Step 9 of the doc, and the only tag is `v0.1.0` |
| Dry run at Base (`media/T-047/dry-run.txt`, against this head) | It shows the three expected install failures, Step 9's refusal, claim and release, doctor exit 3 on a non-GitHub origin, and three `[create]` lines over a bare `.waggle/`. Nothing in it contradicts the doc |

**Not run:** this guard has no `gh`, network or general shell, so I ran no doc command myself. The `gh` lines are unproven until the section 9 rehearsal.

## Notes (not blocking)

1. **AC9 timing:** the dry-run file's header says it ran after the first guard reviews, while AC9 asks for before. The evidence now exists, so this is a process point for the queen.
2. **T1 flag match:** the `\b` at `setup_doc_test.go:251` accepts a flag that is a prefix of a longer one, such as `--gate` on `init` against `-gate-only`. No such form is in the doc today. Anchoring on `(\s|$)` closes it.
3. **T1 planted row:** `:257-262` checks the extractor and the regex separately, because the check is inline in the test. It never drives the failure path end to end, which is thinner than AC4's wording.
4. **"This file" in the README warning:** the brief fixes the text, but in the README it reads as the README itself.
5. **Step 6 PR body:** it says to repeat "the warning at the top of this file", which does not plainly include the link line that section 4 asks for.
6. **Step 6 and Step 9 details:** the source of `<run-id>` is not stated, though `gh pr checks` output shows it. Step 9 also tells the agent to delete `t-900` without first switching off it.

Verdict: must-fix
