<!-- waggle-review v1 verdict=pass head=c4d61ddf06659020e52825725e0fcef17389d6e8 -->

Risk-Tier: 2

**Intent Summary:** Phase A of T-043 is a single signed-off commit (`c4d61dd`, `Signed-off-by` present) that adds 1,480 lines across 16 files, all inside the four leased directories. It adds:
- a new `internal/pace/` package: profile table, `Resolve` with typed `Warning{Kind, Message}`, the `Printable` control-character filter, the shared `Line`/`Finding` formatters, Linux `/proc` parsers, the darwin `vm.loadavg` decoder that accepts 23 or 24 bytes, a stub for other platforms, and a pure per-tool `Render`;
- `config.ParsePace`, `LoadPace` and `LoadPacePick` in a new `internal/config/pace.go`;
- a pace line in status through a new `header.Pace` field, with `renderStatus`'s signature unchanged;
- a pace finding in doctor, placed after `config.Load` and before the local-only return;
- the `waggle pace` command and its usage line;
- commented `pace:` examples in the two templates.

No existing function, parser, gate file, dispatch or claim code, `go.mod` or layout file changes. Blast radius: a repo with no `pace:` section behaves exactly as before, and the new and existing status and doctor tests show this.

**Checks run**
- `go build ./...`, `go vet ./...` and `gofmt -l .` all passed with no output.
- `go test -race ./...`: every package passed (`cmd/waggle`, `internal/config`, `internal/pace`, `internal/scaffold`, `internal/license` and the rest) except `internal/dispatch`. That package hit the 10-minute test timeout in `TestRunViolations/hooksPath_set`, stuck in a `git` subprocess (`offBranch`, then `branchRev`). The diff does not touch `internal/dispatch/`, and its code paths reach nothing new. I take this to be slow git on this host, not this change. I could not check product-ci from here; the queen should confirm product-ci is green before merging.

**Lease rule (phase A)**
- Every hunk fits an allowed item: (a) the new package, (b) the new `internal/config/pace.go`, (c) the status field, the doctor placement, the command and the usage line, (d) commented template lines only, (e) tests.
- Nothing on the forbidden list is touched: `config.Parse`, `ParseContext`, `ParseRuntimes`, `LoadKeeperID`, the subset parser, `gate*.go`, `dispatch.go`, `claim.go`, `internal/layout/` and `go.mod` are all unchanged.

**Acceptance criteria, checked by reading the code**
- **AC2:** `TestParsePaceErrors` runs `config.Parse` on every semantic error and expects success. `TestGateIgnoresPaceSection` compares gate output for three bad sections against the output with no section. The syntax-error case is pinned too.
- **AC3:** bounds and one value past each bound are tested per key, and the low and high ends are set in the right profile for the ordering direction.
- **AC4:** the `Resolve` table covers the clamped, unknown and control-character cases. `Resolve` also guards against `chosen > Limit`.
- **AC5:** the 23- and 24-byte darwin inputs give identical results. 22 and 25 bytes and `fscale` 0 are unknown. Percent is tested on 1, 8 and 64 CPUs.
- **AC7:** the env object holds only the four allowed keys, with plain digits or `"0"`/`"1"` as values. The Gas Town line equals `parallel_runs`. A before-and-after tree snapshot shows the command writes no file.
- **AC10:** every new Go file has the SPDX header on line 1 and an empty line 2. `load_other.go` puts `//go:build` after the header. Both templates keep their header on line 1.
- **Budget:** about 746 added non-test lines, under the 1,250 target.
- **Threat model:**
  - The load reader opens only `/proc/loadavg` and `/proc/meminfo`.
  - A word from `local.yaml` reaches output only through `Printable`, both in `Resolve` messages and in the `paceWord` errors.
  - The render writes no file and runs no program.
  - Nothing touches models, guards, claims, leases or worker permissions.

**Notes (not blocking)**
- In `cmd/waggle/pace_test.go:161`, the `snapshot` helper encodes file size as `string(rune(size))`, so some sizes collide. The mtime and mode still catch any write, so the no-write check holds.
- Section 2.7 says the Claude text states that agent teams are experimental and off by default. `render.go:66-68` prints that only for `serial`, the one profile that sets `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS`. This is a reasonable reading of the brief.
- AC2 names the `leaseRepo` fixture, but the gate test uses `newGateRepo` with a claim and a docs change. It is equivalent for the purpose.

**Blocking Issues (Must-Fix Only):** None.

Verdict: pass