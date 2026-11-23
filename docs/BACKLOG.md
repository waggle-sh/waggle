<!-- SPDX-License-Identifier: Apache-2.0 -->
# Backlog: Tier 1 and Tier 2 work toward a public source release

Written by dev-mgr on 2026-11-23 from `origin/main` at `2a2ed3e`. The hive's own ticket records (`.waggle/hive/ROADMAP.md`, `.waggle/hive/tasks/`) are off limits to dev-mgr, so this file holds the new tickets until the scout ports them. IDs continue the hive's scheme: T-086 is the highest in use (PR #292), so these start at T-087.

Rules for every ticket here:

- Decision-free: no signed head, visibility, license or release change, no secrets, no new service, no deploy.
- Tier 1 or Tier 2 paths only. Nothing under `.waggle/` or `.github/workflows/`. Nothing under `internal/scaffold/templates/` (CODEOWNERS review, and it reaches every hive through `waggle upgrade`).
- Avoid `docs/multi-keeper.md` (PR #289) and `internal/layout/` tests (PR #293).
- New files start with an SPDX header (Apache-2.0 for code and docs). No em dashes or en dashes anywhere.
- Where a test exposes a real bug, fix it only if the fix is under 30 non-test lines and inside the ticket's files. Otherwise keep the test, mark it `t.Skip` with a one-line reason, and say so in the PR body. Never weaken an assertion to pass.
- Standard checks for a Go ticket: `go build ./...`, `go vet ./...`, `gofmt -l .` (empty), `go test -race ./<touched packages>`. Run the full `go test -race ./...` once before the PR; it takes several minutes, so run it in the background.

## Status corrections (the hive records are stale; this PR cannot edit them)

Verified against `origin/main` and `gh pr list` on 2026-11-23. `.waggle/hive/ROADMAP.md` still says `proposed` for all of these.

| Ticket | Real state |
| --- | --- |
| T-050, T-053 | Merged (design briefs, #234, #235) |
| T-051 | Merged (phases A to D2, #241 to #254) |
| T-052 | Merged (phases A and B, #258, #260) |
| T-054 | Code merged through phase D (#279). Pin move is PR #283, open |
| T-055 | Merged (#236) |
| T-060 | Decision D-382 merged (#270) |
| T-065 | Phase A merged (#257). Later phases not started |
| T-071 | Merged (#273) |
| T-072 | Merged (phases A, B and the README line: #267, #271, #272) |
| T-073 | Merged (phases A and B: #266, #274) |
| T-074, T-075 | Scout briefs merged (#277, #278). No build yet |
| T-076 | Brief merged (#276). Build is PR #287, open |
| T-081 to T-084 | Stubs only, not built. Each needs a scout brief before a builder can take it |
| T-085 | Merged (#291) |
| T-086 | PR #292, open (AGENTS.md and INSTALL-AGENT.md) |
| T-061 | Docs audit, site workflow and multi-keeper guide merged (#282, #284, #286, #289). Docs impact check is PR #285, open |

## Tickets

### T-087 · Top-level help and version flags, usage on the right stream · P1 · S

- **Touches:** `cmd/waggle/main.go`, new `cmd/waggle/main_test.go`.
- **Gap:** `main()` has no case for `-h`, `--help`, `help`, `-v` or `--version`, so `waggle --help` prints "Unknown subcommand" and exits 1. No arguments and an unknown subcommand print usage on stdout. `site/content/cli.md` says exit code 2 means usage. `version` ignores extra arguments. The usage heading reads "Commands (v0.1 Bootstrap)".
- **Acceptance:**
  - `main` delegates to `run(args []string, stdout, stderr io.Writer) int` and calls `os.Exit` once.
  - `waggle -h`, `--help` and `help` print the usage to stdout and exit 0.
  - `waggle --version` and `-v` print the same line as `waggle version`.
  - No arguments, and an unknown subcommand, print usage to stderr and exit 2; the unknown case names the bad word first.
  - An unknown subcommand within edit distance 2 of a real one adds `did you mean "<cmd>"?`.
  - `waggle version extra` prints `waggle version takes no arguments` to stderr and exits 2.
  - The "(v0.1 Bootstrap)" heading becomes "Commands".
  - Table test in `main_test.go` covers every row above. Existing plain-terms tests still pass.
- **Depends on:** none.
- **Checks:** `go test -race ./cmd/waggle/...`, vet, gofmt.

### T-088 · `waggle init` testable and strict about its arguments · P1 · S

- **Touches:** `cmd/waggle/main.go` (init case moves to `cmd/waggle/init.go`), new `cmd/waggle/init_test.go`, the `init` section of `site/content/cli.md`.
- **Gap:** init is inline in `main()` with `flag.ExitOnError` and ignores stray positional arguments, so it has no command-level test.
- **Acceptance:**
  - `runInit(args []string, cwd string, stdout, stderr io.Writer) int` with `flag.ContinueOnError`. Depends on T-087 only for the `run` shape; if T-087 is not merged, add the `init` case to call `runInit` directly.
  - A positional argument (`waggle init foo`) exits 2 with `waggle init takes no arguments`.
  - An invalid `--profile`, `--delivery`, `--forge` or `--comb-mode` exits 2 and the message lists the allowed values (the scaffold already words these; the exit code becomes 2).
  - `--gate-only` in a temp git repo writes exactly the three files named in its success line and prints that line. A second run without `--force` does not change them.
  - Table tests for each row. `cli.md` `init` section states the exit codes.
  - Do not change the `--forge` flag help or the set of accepted forges (see Decision D4).
- **Depends on:** none (T-087 helpful).
- **Checks:** `go test -race ./cmd/waggle/... ./internal/scaffold/...`, vet, gofmt.

### T-089 · Docs drift test: usage lines and command list · P1 · S

- **Touches:** new `cmd/waggle/docs_drift_test.go`; fixes to `README.md` or `site/content/cli.md` it finds.
- **Gap:** README quotes the `claim`, `release` and `force-release` usage lines by hand, and `cli.md` lists every command, with nothing tying either to the source.
- **Acceptance:**
  - The test runs `claim -h`, `release -h` and `force-release -h` in-process and asserts each `Usage:` line appears verbatim in the README "Writing a claim" block and in `cli.md`.
  - Every command word printed by the top-level usage appears as a heading or table row in `cli.md`.
  - Every flag in the `nectar`, `vote` and `quorum` rows of `cli.md` exists in that command's `-h` output.
  - The test reads files with paths relative to the repo root found by walking up to `go.mod`, and fails with the file and the missing string.
  - Any drift it finds is fixed in the docs, not in the code.
- **Depends on:** none.
- **Checks:** `go test -race ./cmd/waggle/...`, vet, gofmt.

### T-090 · Claim and release error messages name the flag and the fix · P1 · S

- **Touches:** `cmd/waggle/claim.go`, `cmd/waggle/claim_test.go` or a new `cmd/waggle/claim_errors_test.go`.
- **Gap:** an operator's first command is `waggle claim`; its failures should each say what to change.
- **Acceptance (table test, one row each, asserting exit code and the stderr text):**
  - missing `--dirs`, missing `--gate`, and a `--gate` other than `light` or `full` name the flag and the allowed values;
  - an invalid task id (`t-1`, `T-`, `1a`, `../x`) says the accepted form `T-<digits>`;
  - missing `--owner` in a gate-only repo says to pass `--owner <your GitHub login>` or create `.waggle/control/local.yaml`;
  - a fifth `--branch` and a fifth `--co` say the limit of 4;
  - an overlapping claim names the other task and the overlapping directory;
  - `release` of a task with no claim says no claim exists and names the ref it looked for.
  - Where a message already satisfies a row, add only the test. Where it does not, reword it; keep existing tests green.
- **Depends on:** none.
- **Checks:** `go test -race ./cmd/waggle/... ./internal/comb/...`, vet, gofmt.

### T-091 · Clear message when run outside a git repo or without a remote · P1 · S

- **Touches:** `cmd/waggle/` command entry points that take `--repo` (`claim`, `release`, `status`, `doctor`, `nectar`, `quorum`, `vote`), one shared helper, new `cmd/waggle/norepo_test.go`.
- **Gap:** these commands can fail with raw git errors when the directory is not a repo or has no `origin`.
- **Acceptance:**
  - Run in a temp dir that is not a git repo, each command exits 3 with one line `waggle <cmd>: <dir> is not a git repository (run it inside your repo or pass --repo <dir>)` and no raw `fatal:` text.
  - Run in a repo with no `origin`, `claim`, `release` and `doctor` exit 3 with `waggle <cmd>: this repo has no remote named origin; claim refs are pushed there` and the `git remote add` hint. `doctor --no-write` keeps working with local-only checks as `TestDoctorLocalOnly` expects.
  - The helper is shared, not copied per command.
  - A table test covers every command in both situations.
- **Depends on:** none.
- **Checks:** `go test -race ./cmd/waggle/...`, vet, gofmt.

### T-092 · Gate tests: submodule end to end and rename out of a Tier 3 path · P2 · S

- **Touches:** `cmd/waggle/gate_verify_test.go` (or new `cmd/waggle/gate_verify_modes_test.go`).
- **Gap:** the gitlink rule is tested only at `internal/comb` level; `TestVerifySymlink` covers symlinks through `gate verify`. Renames are tested only into a claim.
- **Acceptance:**
  - A commit adding a gitlink (mode 160000) under the claim directory makes `gate verify` exit 1 with the rule text `symlink or submodule mode`.
  - A commit that renames `.github/workflows/x.yml` to a path inside the claim directory fails `tiers` (the old path counts), exit 1.
  - A commit that renames a Tier 2 file (`.waggle/hive/x.md`) into the claim directory under a `light` claim fails `tiers`.
  - A commit that changes a file's mode to executable inside the claim passes.
  - Uses the existing `leaseRepo` helpers. No change to gate code unless a test fails (see the rules above).
- **Depends on:** none.
- **Checks:** `go test -race ./cmd/waggle/... ./internal/comb/...`, vet, gofmt.

### T-093 · Gate tests: awkward path names and claim directory spelling · P2 · S

- **Touches:** new `cmd/waggle/gate_paths_test.go`.
- **Gap:** `TestVerifyEncodesHostileText` covers hostile commit text. Paths with spaces, quotes or odd bytes under a claim, and claim directory spellings, are not exercised end to end. Unicode normalization is T-074's job; leave it out.
- **Acceptance:**
  - Files named with a space, a double quote, a single quote, a leading dash, and non-ASCII (already NFC) bytes inside the claim directory pass `leases`.
  - The same names one directory outside the claim fail, and the failure line prints the path encoded the way `TestVerifyEncodesHostileText` expects (no raw control bytes in output).
  - A claim written `--dirs src/`, `--dirs ./src` and `--dirs src` either behave identically or the unsupported spelling is refused at `waggle claim` with a message; the test pins whichever the code does today and the PR body states which.
  - A prefix sibling (`srcfoo/x` against claim `src`) fails.
- **Depends on:** none.
- **Checks:** `go test -race ./cmd/waggle/...`, vet, gofmt.

### T-094 · Test the install script offline · P2 · S

- **Touches:** new `cmd/waggle/install_script_test.go` (reads `../../install.sh`), small fixes to `install.sh` the tests expose.
- **Gap:** `curl | sh` is the headline install and nothing tests it. `WAGGLE_BASE_URL` with a `file://` URL exists for exactly this.
- **Acceptance:**
  - The test skips when `sh`, `tar` or `sha256sum`/`shasum` is missing.
  - It builds a fake release in a temp dir: an archive named `waggle_<version>_<os>_<arch>.tar.gz` holding a stub `waggle` script, plus a matching `SHA256SUMS`.
  - Pinned `WAGGLE_VERSION=0.0.1` into `WAGGLE_INSTALL_DIR=<tmp>` installs an executable file and prints no error.
  - A tampered archive (checksum mismatch) exits non-zero, installs nothing, and the message contains `checksum`.
  - A missing archive for the current OS and CPU exits non-zero with a message naming the file it looked for.
  - An unwritable install directory exits non-zero with the directory in the message.
  - The test never touches the network or the real home directory (`HOME` set to a temp dir).
  - If `shellcheck` is on `PATH`, the test also runs it with `-s sh` and fails on findings; it skips otherwise.
- **Depends on:** none.
- **Checks:** `go test -race ./cmd/waggle/...`, vet, gofmt.

### T-095 · Docs cite source by line number: audit and guard · P2 · S

- **Touches:** `README.md`, `docs/gate.md`, `docs/nectar.md`, `docs/quorum.md`, `site/content/*.md` (line citations only), new `cmd/waggle/docs_citations_test.go`. Not `docs/multi-keeper.md`.
- **Gap:** about 80 distinct `cmd/...go:NNN` and `internal/...go:NNN` citations. Any edit to a Go file can make them point at the wrong line.
- **Acceptance:**
  - The test extracts every `path.go:N` and `path.go:N-M` citation from those docs and fails when the file is missing or the line is past the end of the file.
  - For each citation preceded by a backticked identifier on the same line (for example `` `runClaim` ``), the cited line or the 20 lines before it contain that identifier. Citations without an identifier are only range-checked.
  - The audit fixes every citation the test flags by updating the line number to the current one. Prose is not rewritten.
  - The test lists all failures at once.
- **Depends on:** none. Land after PR #285 and #287 merge if they touch the same docs; otherwise rebase.
- **Checks:** `go test -race ./cmd/waggle/...`, vet, gofmt.

### T-096 · Runnable gate walkthrough in `examples/` · P2 · S

- **Touches:** new `examples/gate-walkthrough/README.md` and `run.sh`, new `cmd/waggle/examples_test.go`, one README link under Documentation.
- **Gap:** no example shows the gate working without a GitHub repo. A newcomer cannot see a pass and a fail locally.
- **Acceptance:**
  - `run.sh <path-to-waggle-binary>` works in a `mktemp -d` directory only: makes a bare repo as `origin` and a clone, runs `waggle init --gate-only`, commits it, writes `waggle claim T-001 --dirs src --gate light --branch t-001 --owner demo`, commits `T-001: add src/a.txt` on `t-001`, and runs `waggle gate verify --base main --head t-001` (exit 0 printed).
  - It then adds a second commit `T-001: touch README.md` outside `src/` and shows `gate verify` exit 1 with the failing line.
  - No network, no GitHub, no write outside the temp dir; the script removes it on exit. `set -eu`, POSIX sh.
  - The README for the example shows the real output of both runs, pasted from a run, and says what each line means.
  - `examples_test.go` builds the binary into a temp dir, runs `run.sh`, and asserts the two exit codes; it skips when `git` or `sh` is missing.
  - The root README links the example in its Documentation list.
- **Depends on:** none. If PR #293 or #292 touch the README Documentation list, rebase.
- **Checks:** `go test -race ./cmd/waggle/...`, `sh -n examples/gate-walkthrough/run.sh`, vet, gofmt.

### T-097 · Troubleshooting page built from real gate messages · P2 · S

- **Touches:** new `docs/troubleshooting.md`, one link each in `README.md` and `docs/recipes/README.md`, `site/scripts/sync-docs.mjs` (add the route so the site build still passes), new `cmd/waggle/docs_messages_test.go`.
- **Gap:** the failure lines the gate and doctor print are the first thing a user searches for, and the docs only mention a few.
- **Acceptance:**
  - The page covers at least ten failure lines, each a quoted message exactly as printed by `waggle gate verify`, `waggle doctor` or `waggle claim` (for example `claim names no branch`, `path is outside the claim directories`, `symlink or submodule mode`), the cause in one sentence, and the fix command.
  - Each entry names the exit code.
  - The test extracts each quoted message from the page and fails when no non-test Go file under `cmd/` or `internal/` contains it (matching the static part of a format string).
  - The sync script maps the page to `troubleshooting`; if node 22 is available, `cd site && npm ci && npm run build` passes, otherwise say in the PR body that it was not run.
  - Plain terms per CONTRIBUTING (task, claim, lease; bee words only with a gloss).
- **Depends on:** none.
- **Checks:** `go test -race ./cmd/waggle/...`, vet, gofmt, site build if node is present.

### T-098 · Empty states for `status` and `doctor` · P2 · S

- **Touches:** `cmd/waggle/status.go`, `cmd/waggle/doctor.go` (messages only), their tests.
- **Gap:** first-run output on a fresh repo should say what is empty and what to do, not print blank sections.
- **Acceptance (assert exact stdout in each row):**
  - `waggle status` in a repo with no claims prints a line `no active claims` and, with none, no per-ticket usage block, `no token use recorded`; the daemon line says `daemon: not running` when there is no socket.
  - `waggle status` with only a malformed claim still prints the existing force-release line.
  - `waggle doctor --no-write` in a repo with no config prints the config-missing reason and the command `waggle init --gate-only`, exit code unchanged from today.
  - No row changes an exit code. Existing goldens are updated only where the new line is added.
  - Verify current output first; if a row already holds, add the test only.
- **Depends on:** none.
- **Checks:** `go test -race ./cmd/waggle/...`, vet, gofmt.

### T-099 · Golden tests for `--json` output · P3 · S

- **Touches:** new `cmd/waggle/json_golden_test.go`, `cmd/waggle/testdata/json/`, a "JSON output" paragraph in `site/content/cli.md`.
- **Gap:** scripts and agents read `--json`; nothing pins the shape.
- **Acceptance:**
  - Golden files for `claim --json`, `release --json`, `force-release --json` (inspect mode), `doctor --json` if it has one, `status --json` if it has one, and `nectar --json`, each from a fixed repo with fixed times (`--now`).
  - Volatile values (object ids, timestamps) are replaced by placeholders before comparison.
  - A `-update` flag rewrites the goldens; without it a difference fails with a diff.
  - `cli.md` lists the top-level keys for each command that has `--json` and says new keys may be added but existing ones are not renamed in 0.x.
  - Commands that have no `--json` are skipped, not changed.
- **Depends on:** none.
- **Checks:** `go test -race ./cmd/waggle/...`, vet, gofmt.

### T-100 · One local check script that matches CI and the release matrix · P3 · S

- **Touches:** new `scripts/check.sh`, `CONTRIBUTING.md` (Build and test bullet only).
- **Gap:** CI runs build, vet, gofmt and race tests on Linux only. Releases ship macOS and Windows archives that nothing cross-compiles before the tag. Changing the workflow is Tier 3, so this ticket gives contributors and the Head Keeper a script that does it.
- **Acceptance:**
  - `scripts/check.sh` runs, in order: `go build ./...`, `go vet ./...`, the gofmt check over tracked files exactly as `ci.yml` does it, and `go test -race ./...`; each step prints a header and the script stops at the first failure with that step's name.
  - `scripts/check.sh --cross` additionally runs `GOOS=<os> GOARCH=<arch> go build -o /dev/null ./cmd/waggle` for linux, darwin and windows on amd64 and arm64, and `go vet` for darwin and windows. The matrix is read from `.github/workflows/release.yml` if it lists one, otherwise hard-coded with a comment saying why.
  - `--quick` skips the race tests.
  - POSIX sh, `set -eu`, passes `sh -n`; runs from any directory by finding the repo root.
  - CONTRIBUTING's Build and test bullet names the script and the flags.
  - Any cross-build failure it finds is reported in the PR body with the file and error; fix only if it is a missing build tag or a one-file portability fix.
- **Depends on:** none.
- **Checks:** `sh -n scripts/check.sh`, `scripts/check.sh --quick --cross`, `gofmt -l .`.

## Existing stubs that need a scout brief before a builder can take them

T-081, T-082, T-083, T-084 (T-073 phase B guard notes) and T-075 (doctor collision count) and T-074 (Unicode head tree check) are in the hive's ROADMAP. They are not rewritten here.

## Decisions found (not tickets)

These need a Paul or Head Keeper answer and have no ticket in this file.

- **D1. Where the backlog lives.** `.waggle/hive/ROADMAP.md` and `tasks/` are the real backlog and are off limits to dev-mgr. This file is interim. Options: (a) the scout ports T-087 to T-100 into the hive and deletes this file; (b) keep this file as the public backlog; (c) delete it and keep records only in the hive. Recommend (a): one backlog, and the ROADMAP status lines above need the scout anyway.
- **D2. CI additions are Tier 3.** `go mod verify`, `govulncheck`, a cross-compile job, running T-094's install test and a link check all need edits under `.github/workflows/`. Options: (a) Head Keeper approves one workflow PR; (b) rely on `scripts/check.sh` (T-100) only; (c) wait until after the public release. Recommend (a), because Windows and macOS builds are promised in the README and are not built before a tag.
- **D3. Cut a v0.1.2 release.** README and the site mark about eight features "On main, not in a release". Options: cut v0.1.2 now; wait until phase 3 work lands; keep as is. Recommend cut after T-087 to T-091 merge, so the release has working help and error messages. This is a release step, so no ticket.
- **D4. Is `--forge none` a supported `init` mode?** The scaffold accepts `github` and `none`, the flag help says `v0.1: 'github'`, and `--gate-only` refuses `none`. Options: document `none` as supported; remove it from the scaffold; leave undocumented. Recommend document it for the full hive only, so the help matches the code. T-088 avoids this.
