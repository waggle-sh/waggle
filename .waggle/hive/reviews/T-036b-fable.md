<!-- waggle-review v1 verdict=pass head=132112e220de58f7159580ded3d17fa365901203 -->

**Risk-Tier: 3** (per the brief: first doctor probes, a push to the user's remote, token-bearing GitHub reads).

**Intent Summary:** Phase B of T-036 adds `waggle doctor`. The GitHub forge's `Probe` stub is replaced by a real implementation that reads the default branch, pages the branch rulesets (at most 10 pages of 100), falls back to classic branch protection, and yields `required-gate`, `up-to-date` and, for `native_queue`, a non-required `merge-queue` finding. A new forge-level `ProbeClaimRefs` pushes and then deletes two `crypto/rand`-named refs under `refs/waggle/probe/` with `--atomic --porcelain`, after `ls-remote` and a local `cat-file -e` check under `GIT_NO_LAZY_FETCH=1`, and prints the notice line before the first push. The doctor command wires config, forge construction via a `newForge` seam, exit codes 0/1/2/3 and `forge.Render`. Blast radius: a new CLI verb and one previously-erroring method; no existing tests, templates, hive files or other packages change. Eight files, all inside the phase B lease rule, about 380 non-test lines against an estimate of 526.

**Verification run here (host, darwin):** `go build ./...`, `go vet ./...` and `gofmt -l .` are clean; `go test -race ./...` passes in every package, including the forge, comb and dispatch push fixtures.

**Checked against the threat model and 2.9:**
- Lease: only `cmd/waggle/doctor.go`, `doctor_test.go`, `main.go` (doctor case and usage line), `internal/forge/github.go` (stub `Probe` removed only), and the new `probe.go`, `probe_github.go` and their tests. No existing test file changed, so the AC-B2 list in the PR body should be empty. I could not read the PR body (no `gh`); the queen should confirm it lists no assertion changes and the 2.11 follow-ups.
- Push probe: notice precedes the first push and is suppressed with `--no-write` and when the tip object is absent; no `--force`, `+` refspec, `fetch` or retry anywhere; ref names match `^refs/waggle/probe/[0-9a-f]{16}-[ab]$` and differ across runs; `--no-write` stops after `ls-remote`. Tested in `probe_test.go`.
- Output hygiene: git stderr is only inspected for the `does not support --atomic` substring, never printed; GitHub message text is reduced to `GitHub answered <status>` by `failWhy`; the default-branch name is validated by `checkBranch` before it reaches any Summary or Fix. The hostile-stderr and `SECRET-TEXT` tests cover both paths.
- Status mapping: every read failure (non-2xx, bad JSON, transport error, 11th page) becomes `unknown`, never `ok`; classic 404 with exactly the two documented messages is `missing`; other 403/404 is `unknown`. Exit 3 only for config, forge construction, and the default-branch read or `checkBranch` failure. The `none` forge's `code-host` finding is not `Required`, so a `none` provider with a remote exits on `claim-refs` alone.
- Goldens: mixed `up_to_date` and `native_queue` reports are inline constants in `doctor_test.go`; the queue golden shows `Enforcement: advisory (merge-queue is unknown)` and no "Everything Waggle relies on is enforced" line, as 2.9 requires.
- No em or en dash in any added file; messages are in plain terms.

**Notes (not must-fix):**
- `ProbeClaimRefs` takes a fifth `notice io.Writer` argument, which the brief's signature lacks. It is the cleaner way to satisfy "notice before the first push" and is tested as such.
- `TestGitHubProbeEscapesBranch` does not actually prove escaping: the fake keys on the decoded `r.URL.Path`, which matches whether or not `%2F` was sent. The code does call `url.PathEscape` (`probe_github.go:97, 134`); pinning `r.URL.EscapedPath()` in the fake would close this in a polish round.
- An empty remote (no commits) makes `ls-remote` print nothing, which lands as `unknown` "Cannot read origin". Fail-closed and reasonable, but the Fix text points at URL and credentials rather than an empty remote.
- `GIT_NO_LAZY_FETCH=1` is also set for the `ls-remote` and `push` calls. Harmless.

**Blocking Issues (Must-Fix Only):** none.

Verdict: pass