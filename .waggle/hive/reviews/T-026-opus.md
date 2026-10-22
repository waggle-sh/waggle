# Guard review: T-026

The change does what the brief asks and stays inside its lease. I could not run the tests that push to an origin, so product-ci on the PR is the only run of those.

Verdict: pass
Risk-Tier: 2

## Intent Summary

Fixture repos in `internal/dispatch` tests now switch off git auto maintenance in their own repo config, which should stop the product-ci flakes recorded in T-023c-reopus R2 and run 39604347542. A new helper, `quietGit` (`dispatch_test.go:42`), writes `maintenance.auto=false`, `gc.auto=0` and `receive.autogc=false` with `git config --local`. It is called right after init or clone at 8 sites, before any commit or push:
- `newRepo`
- `newFixture`'s work repo and its `origin.git`
- `TestResolveBase`'s bare origin and its `other/c` clone
- the `TestForeignSameBranchRefused` clone
- the `TestAcquireClaimRemoteNotes` origin and `mk` repos

`shIn` keeps `-c maintenance.auto=false` and adds `-c gc.auto=0`. The new test `TestFixtureReposQuietGit` checks all three keys in `newFixture`'s work repo, its origin and in `newRepo`. It only runs git: no `Run`, no `sweep()`, no signals, no `t.Skip`.

The commit changes only 4 `*_test.go` files: 31 added lines, 1 changed, no non-test lines, `go.mod` untouched. That is under the target of 80. Blast radius is limited to test setup. Worktrees share the common config, so they are covered too. The host record will hash the new keys, but they are written before any `Run`.

## Blocking Issues

None.

## Notes (not must-fix)

- **N1, tests I couldn't run:** `go build ./...` and `go vet ./...` pass. Of the `-run` set:
  - `TestForeignSameBranchRefused` and `TestPlantedRepoNotReused` passed.
  - `TestFixtureReposQuietGit`, `TestResolveBase` and `TestAcquireClaimRemoteNotes` failed with `'/dev/null/<tmp path>' does not appear to be a git repository`.
  
  The cause is this guard session's own push block (`url.file:///dev/null/.pushInsteadOf`, `internal/runtime/launch.go:118`), which rewrites every push, local ones included. The diff did not cause it. AC1 and the new test depend on product-ci (ubuntu, full race run).
- **N2, skipped sites are justified:** 4 init sites have no `quietGit` call: `guard_test.go:25`, `guard2_test.go:49` and `:63`, and `guard3_test.go:27`. Each is a planted, hostile repo in a refusal test. Nothing pushes to or fetches from them, and the test's own git calls go through `sh`, which now passes both `-c` flags. Adding config there could also change what those tests plant. I can't see the PR description, so I couldn't check its list of sites for AC3.
- **N3, branch point:** the branch starts at `5df0422`, which is behind base `8deba1c`. The only difference is the brief fix to `.waggle/hive/tasks/T-026.md`, so the net diff against base shows that file without the branch touching it. There is no conflict.
- **N4, formatting:** `gofmt` is denied in this session, so I checked the added lines by eye: tab indentation, no trailing spaces, no aligned blocks touched.

Verdict: pass