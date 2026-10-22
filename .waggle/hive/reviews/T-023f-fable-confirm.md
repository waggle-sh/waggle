# Guard review: T-023 phase F fix round 1 (PR #63, `t-023f-template` at 4eb77da, base 43c75cc)

Verdict: pass
Risk-Tier: 3

Confirming guard (fable, D-205). Fix round 1 for the opus and fable reviews at 0df202d. The fix commit `4eb77da` changes four lines of `internal/scaffold/templates/roles/queen.md` (4 insertions, 4 deletions, one file): escalation rule 4 and Dispatching workers items 1, 3 and 5. Every new claim was checked against `cmd/waggle/dispatch.go`, `internal/dispatch/{inputs,request,result,run,guard,hostrecord,clean,worktree}.go` and `internal/runtime/{policy,stream}.go` at 4eb77da.

## Intent Summary

The fix resolves both blocking items and every note the two first-round reviews raised. Item 1 now takes `--dir` from the brief's Lease line and `--gate` from its Gate line and sends a missing or unclear value to the Keeper (opus B1). Item 3 keys the draft PR on `result.json` `pushed: true` (opus N1), commits a review only after a guard run that exits 0 with a verdict (opus N4), compares the branch head with the PR head as the case when there is one (fable N3), names the review `T-<nnn><suffix>-<model>.md` with the suffix defined as the branch name after `t-<nnn>` and the model from `result.json`, and appends later rounds by the same model after one blank line, never rewriting or removing an earlier round (opus B2, fable B1). Item 5 takes the digest from "the one line of the exact form" (opus N3, fable N1) and matches the lock warning as "a line starting" (opus N2, fable N2). Rule 4 says "contains" (opus N7). No Go code changes; blast radius is unchanged from the first round: every installed hive's queen after `waggle upgrade`.

## Blocking Issues

None.

## Checks on each fix item

1. **Opus B1, gate line.** Item 1 reads "with one `--dir` per leased directory from the brief's Lease line and `--gate` from its Gate line; if either is missing or unclear, ask the Keeper". All nine briefs in this hive with a gate carry it on a `- **Gate:**` line. The fallback is a Keeper card, not a guess, so no gate can be lowered by reading the brief. `--dir` is repeatable in `runDispatch` (`fs.Var(&dirs, "dir", ...)`), so "one `--dir` per leased directory" is the right form and covers opus N5.

2. **Opus B2 and fable B1, review name.** `roundMustFix` (`inputs.go:126`) builds `prefix := req.Ticket + strings.TrimPrefix(req.Branch, "t-"+strings.TrimPrefix(req.Ticket, "T-")) + "-"` and counts every `reviews/<prefix>*.md` at base. `checkBranch` (`request.go`) requires the branch to start with `t-<nnn>` and refuses a suffix that starts with a digit, so "the branch name after `t-<nnn>`" is exactly what the code strips. The template's two examples are right: `t-023f` gives prefix `T-023f-` and file `T-023f-<model>.md`; `t-023` gives `T-023-` and `T-023-<model>.md`. `<model>` is `result.json` `model`, set from `sel.Model` (`run.go:302`), which is the policy's level name, not worker-chosen. Phases cannot bleed into each other: `T-023f-opus.md` does not start with `T-023-`, and `T-023.md` (the file `in.Review` reads, `inputs.go:109`) does not start with `T-023-`, so no round is counted twice.

3. **Append rule counts each round once.** `CountMustFix` (`policy.go:231`) counts every line equal to `Verdict: must-fix` in a file. With one file per branch and model and each round appended, a round's verdict line stays in the file and is counted on every later resolution, which is the D-177 intent ("the step-up counts this phase's rounds"). A pass round appended after a must-fix round adds nothing and removes nothing, as D-177 reads. The old overwrite path, which dropped the first round's line, is closed by "never rewrite or remove an earlier round". `parseVerdict` (`guard.go`) takes the last `^Verdict: (pass|must-fix)$` match of the run's own `final`, so what the queen acts on for the new round is unaffected by the earlier rounds in the committed file.

4. **Caps in practice.** `maxReviews` (50) is a file count per prefix; one file per model makes it unreachable (two or three files per phase, against 50). `maxBrief` (400 KiB) is a per-file cap that `blobAt` enforces with `failf(ExitEnv, ...)`, which `roundMustFix` joins into its error and the next builder dispatch on that branch exits 3 with a message naming the file and the cap. Each committed round is under `MaxFinal` (256 KiB, `stream.go:18`) because a longer final gets no verdict (exit 5) and is never committed under item 3. In this hive the T-023 reviews run 45 to 72 KiB, so one model's file reaches the cap after roughly six must-fix rounds, far past the two that fire the step-up. When it trips it fails loudly to the Keeper, never a silently skipped count. Note N1.

5. **Opus N1, `pushed: true`.** `Result.Pushed` (`result.go`, `json:"pushed"`) is set only at `run.go:372`, after a successful `git push` of the checked OID with a remote, not `--no-push`, not a guard. The forgeable stdout line is no longer the trigger.

6. **Opus N4, exit 0 with a verdict.** For a guard, `commits = 1` and an empty `Verdict` turns `done` into `no_verdict`, exit 5 (`run.go:362`), so exit 0 implies a verdict; the template's "exits 0 with a `verdict`" is exact and redundant in a safe direction. A timed-out, interrupted or file-changing guard run (nonzero) is no longer committed before rule 2's "bring every nonzero exit to the Keeper".

7. **Opus N2 and fable N2, warning.** `clean.go:82` and `:106` print `"warning: " + w` where `w` is `took over stale run lock of dead pid <n>` (`worktree.go:514`). "A line starting `warning: took over stale run lock`" is exact.

8. **Opus N3 and fable N1, digest.** `HostRecord` prints `digest <hex>` then the accept command line (`hostrecord.go:81`); every key line is `printable(k) + ": recorded ..."` with control and format runes dropped and 100 bytes max, and the unreadable line is fixed text, so exactly one line matches `^digest [0-9a-f]{16}$`. "The one line of the exact form" is exact.

9. **Opus N7.** Rule 4 says "contains"; the upgrade line is `waggle upgrade refused: model policy for claude-code shims: ...`, so "contains" is the literal reading. D-212 still removes the sentence later.

10. **Fable N3.** "The branch head (the PR head when there is one)" gives a hive without a forge a referent; the guard's `head_oid` is the branch OID resolved once in `guardTarget`, which `git rev-parse <branch>` reproduces locally.

## Checks that the fix introduced nothing new

11. **Flags, fields and lines.** New references: `result.json` `pushed` (exists), `model` (exists), `verdict` (exists), the brief's Gate line (nine briefs carry it; the scaffold's `scout.md` sets no line format, and the "ask the Keeper" fallback covers a brief without one). Nothing else new is named.

12. **Threat bar (D-203).** The new text adds one Keeper card (missing or unclear lease or gate), forbids rewriting or removing an earlier review round, and narrows two triggers (PR after `pushed: true`, commit after exit 0 with a verdict). No auto-accept of a host record, no default for the six protected classes, no kill, signal, prune or manual removal, no waiting, no lowered or skipped gate. Item 4 still names every run-record file and every `note:` and `violation:` line as data.

13. **Injection through the appended file.** Dispatch never puts a committed review into a worker prompt: `in.Review` is count-only (`inputs.go:109` to `:110`; `BuildPrompts` uses RolePrompt, WaggleMD, Brief, TaskLines and ReadOnly), and the template's builder and guard command forms pass no `--read`. So round 1 content cannot steer a round 2 guard through the template. On the count, appending makes a worker-repeated `Verdict: must-fix` line durable across rounds where the old overwrite would have dropped it, but the overwrite was the B2 bug and the inflation itself is opus N9 and fable N5: at most a builder step-up (a spend increase, never a lowered guard). No new vector.

14. **Build and hygiene.** On `git archive 4eb77da` in the scratchpad: `go build ./...` ok, `go vet ./...` ok, `go test -race ./internal/scaffold/... ./cmd/...` ok (16.0 s and 26.4 s). No U+2013 or U+2014 in the file. The commit message is one plain line, author `paulternate`, no attribution trailer. No sweep, signal, container or dispatch run was needed.

## Notes

N1. **Append and the 400 KiB cap.** A single model's file can reach `maxBrief` after several large must-fix rounds (about six at this hive's review sizes, two at the 256 KiB `MaxFinal` worst case). The failure is exit 3 naming the file and the cap, so it is loud, and six must-fix rounds by one model is already a Keeper matter. If it ever bites, a phase E carry could raise the cap for review files or count bytes per round; no template change is needed now.

N2. **This hive's D-183 scan.** `scan-hive.sh` exits 0 only for added review files; an appended round is a modified hive file and gives exit 2 (no hit, "some other change"), which the Keeper then looks at. That is this hive's own override, not the shipped template, and this hive's queen runs from `.waggle/control/roles/queen.md`, not the scaffold copy. Worth one line in the hive override when the template is adopted here.

N3. **`T-<nnn>.md` is still read.** `inputs.go:109` reads `reviews/T-<nnn>.md` for the count as a holdover from the pre-D-177 design. The template never names that file, so it is inert; a later cleanup could drop the read.
