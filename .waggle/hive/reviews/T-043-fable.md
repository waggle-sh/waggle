<!-- waggle-review v1 verdict=pass head=c4d61ddf06659020e52825725e0fcef17389d6e8 -->

The full race suite passed on this host, including the license header check. No probe files were written. Here is the review.

Risk-Tier: 2

**Intent Summary:** T-043 phase A adds the owner-set agent pace: a new `internal/pace/` package (profile table, `Resolve` with typed warnings, control-character filter, shared line formatter, load parsers and per-OS readers, per-tool render), `config.ParsePace` and `config.LoadPacePick` in a new `internal/config/pace.go`, a pace line in `waggle status` through a new `header.Pace` field, a pace finding in `waggle doctor` placed after `config.Load` and before the local-only return, a new `waggle pace` command plus its usage line, and commented `pace:` examples in the two templates. One signed-off commit, 16 files, about 746 non-test lines, under the 1,250 target. Blast radius: none without a `pace:` section. `config.Parse`, the gate files, `ParseContext`, `ParseRuntimes`, `LoadKeeperID`, the subset parser, `internal/layout/`, `go.mod`, dispatch and claim are untouched. This hive's `config.yaml` has no `pace:` block, so status, doctor and the gate behave exactly as before.

**Verification on this host (darwin):**

```
go build ./...     clean
go vet ./...       clean
gofmt -l .         no output
go test -race ./... all packages ok (cmd/waggle 120s, internal/dispatch 595s, internal/license ok, internal/pace ok)
```

**Lease and scope (AC9, AC14 style check):** every hunk fits the phase A allowed list. The `status.go` edit only adds a `header` field and one conditional write, keeping `renderStatus`'s signature. The `doctor.go` hunk is the one insertion point the brief names. `main.go` adds the `pace` case and one usage line. Templates change by commented lines only, and `TestPaceFromTemplate` proves the uncommented example parses to the brief's values. No forbidden item is touched.

**Acceptance criteria checked:**
- AC2: `TestGateIgnoresPaceSection` runs `gate verify` on a repo with each semantic error and compares exit and output byte for byte to the no-section run. `TestParsePaceErrors` also feeds each bad section to `config.Parse` and requires success. `TestParsePaceSyntaxErrorFailsParse` pins the documented tab case.
- AC3: defaults, per-key overrides, both bounds and one past each, ordering on every key in both directions.
- AC4: all `Resolve` rows including control characters and an unknown word, `LoadPacePick` on a missing file, other keys only, and a `local.yaml` with raw keys that are ignored.
- AC5: Linux parsers on malformed input, the darwin decoder on 23 and 24 bytes with equal results, lengths 0, 22 and 25 and `fscale` 0 rejected, percent math on 1, 8 and 64 CPUs. The decoder is pure and in `load.go`; only `load_darwin.go` calls `syscall.Sysctl`.
- AC6: status prints exactly one pace line and the clamp warning on stderr; doctor finding appears on a local-only repo and a forge repo, and the forge missing-check exit stays 1 with pace present.
- AC7: JSON object parses, exactly four keys, serial-only keys hold `"1"` and `"0"`, Gas Town line equals `parallel_runs`, Copilot says advice only, file snapshot (mode, mtime, size) unchanged after the command.
- AC8: every new output goes through `checkPlain` or `checkText`; no bare word, no dash.
- AC10: every new Go file has the Apache-2.0 header on line 1 and a blank line 2 (the `//go:build` line in `load_other.go` follows); `TestRepoHeaders` passes with the files committed.

**Threat bar (D-203):** the load reader opens only the two fixed `/proc` paths. `local.yaml` values pass through exact-word matching and `Printable` before any output, as does the `--tool` argument. `Resolve` clamps and asserts `chosen <= Limit`. Render emits only the four keys with digit or `"0"`/`"1"` values and writes nothing. No model, guard, claim, lease or worker permission is touched.

**Blocking Issues (Must-Fix Only):** none.

**Notes, not must-fix:**
- The config template comment says "Keepers pick a profile", copied from the brief's own 2.1 example. The same template already says "Keeper-assigned models" on an existing line, and the plain-terms rule covers command output, not template comments.
- The render prints "agent teams are experimental and off by default" only for `serial`, the profile where the teams variable is emitted. The brief's 2.7 wording could be read as asking for it on every profile.
- `waggle pace` exits 3 when no repo root is found, matching status and doctor; the brief lists only 0, 1 and 2.
- `runPace` checks `root + "/" + layout.Config` with string joining where sibling commands use `filepath.Join`. Harmless on the supported platforms.

Verdict: pass