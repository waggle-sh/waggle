Verdict: pass
Risk-Tier: 2

## Intent Summary

PR #66 (branch `t-004-remote-test`, head `f6c74ba`, base main `d87640f`, ticket T-004, D-219) changes one condition in `TestRepoConfig` in `internal/config/config_test.go` so the hive config may carry `forge.remote` of `""` or `"origin"`, where before only `""` passed. It also adds a one-line comment. This unblocks hive PR #65 (`t-023f-switch`, which adds `forge.remote: origin` and a `context:` section) on product-ci. No product code changes. Blast radius is this one test assertion. `Load` still has to succeed with no error, so a parse regression in the hive config still fails the test; only the accepted remote set widened, by one exact value.

## Blocking Issues

None.

## Evidence

- Diff: 1 file, `internal/config/config_test.go`, +2/-1, inside the `internal/config/` lease.
- Gate on head: `go build ./...` ok, `go vet ./...` ok, `go test -race -count=1 ./internal/config/` ok.
- Negative checks (temporary edits to `.waggle/control/config.yaml`, reverted, worktree clean): `remote: evil` fails TestRepoConfig with `unexpected config {... Remote:evil ...}`; `remote: upstream` fails too. Base config (no remote) passes.
- #65 config (`origin/t-023f-switch` at `ec50cac`, copied temporarily, reverted): TestRepoConfig passes, and the full `go test -race -count=1 ./...` passes with it in place.
- Other tests reading the hive config: only `FuzzParse` in `internal/config/yaml_subset_test.go` seeds from `../../.waggle/control/config.yaml`; it passed with #65's config (including `context:`) in the full run. The `.waggle/control/config.yaml` reads in `internal/dispatch/` and `cmd/waggle/` tests target `waggle init` temp fixtures, not the hive file.
- Commit message and diff: no em or en dashes, no attribution trailers. DCO sign-off present.

## Notes (non-blocking)

- N1: TestRepoConfig only goes through `Load`, which does not parse `context:`. A regression in `ParseContext` that rejected the hive's new `context:` section would not be caught here. Phase F could add a `LoadContext("../../")` assertion to TestRepoConfig when that path is next leased.
- N2: The test accepts either value in both states, so it will not catch an accidental drop of `remote: origin` after #65 merges. Once #65 is on main, a follow-up could tighten this to `"origin"` only.
