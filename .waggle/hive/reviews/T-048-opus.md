<!-- waggle-review v1 verdict=must-fix head=fc60d1806a718da03b5f17c900004ae5ad3a974d -->

Risk-Tier: 2

**Intent Summary:** T-048 corrects the gate's main public claim under outcome O2. `README.md`, `docs/gate.md` and three recipe pages now say the required check can be forged, name both routes and the three partial protections, and say that an approval needs a re-run to clear a failed check. Doctor's closing line, `Everything Waggle relies on is enforced on this repo.`, becomes two lines that claim no enforcement, and its golden follows. Exit codes, findings, the `Enforcement: enforced` label, templates, the Action and other program messages are unchanged. The blast radius is what adopters read and doctor's output on a clean run, and that output reaches the hive at its next pin move.

**Blocking Issues (Must-Fix Only):**

1. **`cmd/waggle/doctor_test.go:210` was not changed.**
   - It still reads `strings.Contains(out, "Everything Waggle relies on is enforced")`, and the file is absent from the diff.
   - The brief requires this edit in the lease rule, in section 6 ("asserts `found missing` is absent from `queueGolden`") and in AC5 ("the golden and the test follow").
   - The check now looks for text no build can print, so the clause guards nothing. The suite stays green only because of this and the exact `queueGolden` comparison.
   - **Fix:** replace the string with `found missing`.

2. **The cite corrections in brief section 3.2 were not made.** The brief says to match the ranges at `gate.md:20` and old `:106` (now `:114`) to the code at the commit built on. The `:114` line was edited, but its cites were left as they were:

   | Location | Cite in docs | Actual code |
   |---|---|---|
   | `docs/gate.md:20` | `gate_verify.go:258-296` | `baseTierPolicy` is at `:260-298`; `:258` is the closing brace of the previous function |
   | `docs/gate.md:114` | `probe_github.go:153-170` | `gateFinding` is at `:155-172` |
   | `docs/gate.md:114` | `probe_github.go:172-189` | `upToDateFinding` is at `:174-191` |

   This falls under AC1 ("every place in section 3 changed"), and these docs promise cites to the code.

**Checks**

| Check | Result |
|---|---|
| `go build ./...`, `go vet ./...`, `gofmt -l .` | Clean, no output |
| `go test -race ./...` | All packages pass, though this says nothing about issue 1 |
| Lease | All 7 changed files are inside the lease rule; nothing changed under `.waggle`, `gate`, `.github`, `internal/scaffold`, `internal/comb` or `cmd/waggle` |
| Section 3 text | Every other section 3 place matches, and no README heading is renamed |
| AC2, `admin` search | Every hit is corrected, linked or exempt; quoted program messages at `gate.md:27, 49, 50` are byte for byte |
| AC3 and AC4 | Both are met: routes, places and shapes, "none is complete alone", the copy exception, the untested Pull requests token, "test workflows, not the real gate", Write, and "should" for dismissal and request for changes |
| AC5, apart from issue 1 | `report.go:107-108` prints the section 2 text, and the 100-column test passes |
| AC6, plain-language search | No dash or hive record path. `T-123`, `Head Keeper: ` and `.waggle/hive/DECISIONS.md` are listed non-hits. I read the `hive` hits on changed `gate.md` lines as product paths and did not open the other files' hits, which are on unchanged lines (README:53, `gate.md:108`) or are the `DECISIONS.md` path |
| Threat bar | No sentence goes beyond brief section 1, and no protection is presented as complete |
| Probe log spot-check | L11's "a workflow token cannot edit one" is supported by P4b run 3 (`probe-log.md:30`, HTTP 403 on patch) |

I compared the docs with brief section 1 and spot-checked the log. I did not re-verify the other probe rows, which the brief's guards fact-checked.

**Notes (not blocking)**

- **More drifted cites:** the same kind of drift is on lines the brief did not name, and is worth fixing in the same round:
  - `gate.md:114`: `doctor.go:25-28` is now `:27-30`, `:56-59` is now `:64-67`, and `probe.go:36-41` is now `:38-43`.
  - `gate.md:21`: `gate_verify.go:225-229` is now `:227-231`.
  - `gate.md:88-90`, L11's kept doctor sentence: `probe_github.go:99-115, 138-143` is off by a few lines.
- **Loose wording:** `gate.md:114` says "With no finding missing it closes with...", but that branch needs no finding missing or unknown (`report.go:42-50, 105-106`). "Missing or unknown" would be exact.

Verdict: must-fix