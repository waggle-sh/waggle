Risk-Tier: 3

**Intent Summary:** Head `5d6a4d2` makes the three changes T-033 briefs. Item 1 rewrites the guard role template and roster entry so the guard owns nothing in the repo and returns its review as its final message, and adds the "write no file in the repo; your final message is the review" clause to the dispatch `Rules:` line. Item 2 adds `reviewFile` and `countReviewMustFix` in `internal/dispatch/guard.go`: dispatch now prefixes `review.md` with a `<!-- waggle-review v1 verdict=... head=... -->` marker when a verdict was parsed and the head is a 40 or 64 lowercase hex OID, indents any column-0 line of the final that opens like a marker, and `ReadInputs` and `roundMustFix` count markers after the first one while text before it keeps the legacy `runtime.CountMustFix` rule. Item 3 brings the scaffold gate workflow template up to the hive's copy with the three ruled differences. Blast radius: the guard prompt and review record for every dispatched guard, the must-fix count that drives the builder step-up, and the Tier 3 workflow and role files `waggle upgrade` will propose at the next pin move. Nothing under `.waggle/` or `.github/` changes in this commit.

**Verification run on the worktree (one command per call):**

| Command | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | no output |
| `go test -race ./...` | all packages ok |

**Checks against the brief and threat model:**

- **Lease rule (AC2):** the diff touches exactly the eight files the lease rule names. `internal/runtime/`, `cmd/waggle/`, the other role templates, `.github/`, `.waggle/` and `go.mod` are unchanged. `guardrun_test.go`, `dispatch_test.go`, `fix2_test.go` and `policy_test.go` are unchanged. One commit, signed off.
- **Marker writer:** the marker is written only when `res.Verdict` is non-empty and `req.Head` matches the hex pattern. `req.Head` is set by `guardTarget` from `git rev-parse --verify <rev>^{commit}` before the run, so it is the resolved head OID, the same value recorded as `head_oid`. Escaping runs on every write, verdict or not, and `final.md` still receives `s.Final` untouched.
- **Escaping cannot be bypassed:** the writer indents any line that has the `<!-- waggle-review` prefix at column 0, and the counter's regexp is anchored at the line start after a right-side trim only. Both split on `\n`, so a line that the counter would accept is always one the writer indented. A `\r` or tab before the comment fails both the prefix test and the regexp.
- **Counting:** for a file with no marker, the legacy slice is the whole file byte for byte, so the count equals `runtime.CountMustFix`. The new test's legacy table is the same nine-line table `TestCountMustFix` uses. After the first marker only marker lines count, so top-and-bottom pairs and quoted lines add nothing. Caps and file selection in `inputs.go` are unchanged.
- **Guard task lines (AC4):** the `Rules:` line keeps `do not commit`, the probe-file clause and the exact last-line verdict clause, one line.
- **Templates (AC3):** the rendered guard role names the final message as the review and contains neither hive path. The roster guard entry has `"owns": []` and only `owns` and `purpose` changed. The template is 12 lines.
- **Gate workflow (AC8, AC9):** I read the rendered template and main's `.github/workflows/waggle-gate.yml` side by side. Lines 1 to 67 are identical, including workflow-level `contents: read`, the job-level permissions, the fetch without `|| true` and the tier `case` step. The rest differs only by the three ruled items: the `src` presence step guarding setup-go and the gate step, no `gate_verify.go` fallback to `gate decisions`, and the reworded comment. The presence check and the build read `$BASE_SHA`, never the PR head. `2>/dev/null` on the two `cat-file` calls hides stderr only, the exit codes are still tested.
- **AC10:** no em or en dash in any changed file. New text uses plain terms.

**Blocking Issues (Must-Fix Only):** none.

**Notes, not must-fix:**

- The gate step comment says the checker is built from the base revision and that a release binary replaces it later, but does not add the brief's "when the repo carries waggle's source" clause. The `src` step name and its notice carry that meaning.
- The guard role template does not repeat the probe-file deletion rule. The brief put that in a parenthetical and the dispatch `Rules:` line already states it.
- The setup-go step reads `go.mod` from the merge commit, as the brief's 2.3 step 6 accepts.

Verdict: pass