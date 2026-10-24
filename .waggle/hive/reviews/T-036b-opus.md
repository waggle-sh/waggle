<!-- waggle-review v1 verdict=pass head=132112e220de58f7159580ded3d17fa365901203 -->

Risk-Tier: 3

**Intent Summary:** Phase B of T-036 adds `waggle doctor [--repo] [--no-write]`. It runs two checks. The first, `claim-refs` in `internal/forge/probe.go`, finds the remote tip with `ls-remote --symref`. With `--no-write` it stops there and reports skipped. Otherwise it checks the tip object is in the local clone, prints a notice line, pushes two random `refs/waggle/probe/<16 hex>-a|b` refs with `--atomic --porcelain`, and deletes them the same way. It never fetches, forces, uses a `+` refspec or retries, and no git text reaches a finding. The second, `github.Probe` in `probe_github.go`, replaces the old `ErrUnsupported` stub. It reads the default branch, which must pass `checkBranch` and is path-escaped. It then pages the branch rules, at most 10 pages, and falls back to classic required status checks. It reports `required-gate` and either `up-to-date` or, under `native_queue`, a skipped `up-to-date` plus a `merge-queue` finding that is not required and reads `unknown`. Summaries carry only fixed phrases and status codes. `cmd/waggle/doctor.go` connects these through the `newForge` and `probeClaims` test seams, with exit codes 0, 1, 2 and 3. `main.go` gains the `doctor` case and its usage line. Blast radius: one new command. No existing code path changes except the removed stub, and no existing test or assertion changed. All 8 changed files are inside the phase B lease rule: `internal/forge/{probe,probe_github}.go`, the `Probe` stub removed from `github.go`, `cmd/waggle/doctor.go`, `main.go`, and tests.

**Verification (run here, at the head `132112e`):** `go build ./...`, `go vet ./...` and `gofmt -l .` printed nothing. `go test -race ./...` passed in every package, including `internal/dispatch` in 288s, so no push-tripwire failures came up in this run. No em or en dash appears in the new files.

**Checked against 2.9 and section 5:**
- **Notice and push order:** the notice is printed before the first push. It is not printed with `--no-write` or when the tip object is missing locally. Doctor never fetches.
- **Git environment:** `GIT_NO_LAZY_FETCH=1` is appended after `childEnv`, which keeps the existing scrubbing, `GIT_TERMINAL_PROMPT=0` and `LC_ALL=C`.
- **Name validation:** the remote name is checked against `remotePattern` before it goes into git's argv. Probe ref names come from `crypto/rand`.
- **Push outcomes:** the atomic case is detected from the error text, which `LC_ALL=C` keeps stable. A `!\t` porcelain line counts as a failure. A failed delete gives both ref names and the exact `git push <remote> :<a> :<b>` command.
- **GitHub read failures:** a 500, bad JSON, a 403 or an unrecognised 404 gives `unknown`, never `ok`. Only the two exact 404 messages give `missing`. A default-branch read or `checkBranch` failure returns an error, which doctor turns into exit 3 before any push.
- **Render contract:** every `missing` and `unknown` finding has both Not enforced and Fix lines, which `Render` requires.
- **Leaks:** the tests confirm the token, GitHub message text and hostile stderr (CR, `::error::`, a token-like string) never reach the report.

**Blocking Issues (Must-Fix Only):** none.

**Notes (not blocking):**
1. `ProbeClaimRefs` takes an extra `notice io.Writer` argument that the brief's signature doesn't have, and it prints the notice itself rather than leaving that to doctor. This keeps the notice-before-push order in one place, so the behaviour matches 2.9.
2. `probeGit` defaults to a new `probeGitRun`, a copy of `gitRun` that adds `GIT_NO_LAZY_FETCH=1`, instead of defaulting to `gitRun` as the brief says. This was needed to add the env var without touching `gitRun`. It is duplicated code for the polish ticket.
3. AC-B4's path-escape test passes because the fake decodes the path; it doesn't check that `%2F` is actually sent. The code clearly uses `url.PathEscape`.
4. AC-B4's "default branch fails `checkBranch`, so doctor exits 3" is covered in two halves: the `Probe` error tests, and doctor's exit 3 with a fake error. No single end-to-end test covers it.
5. The goldens are inline constants in `doctor_test.go`, not files under `testdata/`. The lease allows either.
6. An empty remote with no HEAD gets the `unknown` "Cannot read <remote>" finding, which slightly misdescribes that case.
7. AC-B2 holds trivially because no existing assertion changed. The PR body wasn't available to me.

Verdict: pass