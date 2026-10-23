Verdict: pass

Risk-Tier: 3

**Intent Summary:** T-030 adds `waggle dispatch release <ticket> [--branch] [--unmerged] [--repo]`. It lets the Keeper give back their own claim on a ticket once the PR has merged, so overlapping tickets stop waiting out the TTL. `Release` in `internal/dispatch/release.go` takes `dispatch.lock` through `lockRepo`. It refuses with exit 4 while any builder, forager or guard worktree, lock, tainted marker or takeover guard of the ticket exists. Unless `--unmerged` is given, it fetches the remote, takes the head from `<remote>/<branch>` or else the local branch, and requires that head to be a second or later parent of a merge on main's first-parent history. That is stricter than the brief, and it rejects a no-commit head. The delete goes only through `ReleaseLease`, which does the owner check and the remote compare-and-swap. Errors map as the brief says: not-owner and concurrent-update to 4, relay and transport to 3, no claim to 0, malformed to 4 with the force-release hint. Printed values go through `printable`. The blast radius is one new verb. The diff touches only `cmd/waggle/dispatch.go`, `internal/dispatch/release.go` and the two test files. It doesn't touch `clean`, the gate, `AcquireClaim` or `internal/comb/`, and it adds no `Run`, `sweep()` or signal use and no `t.Skip`.

I ran:
- `go build ./...`: pass.
- `go vet ./...`: pass.
- `go test -race ./... -run 'TestDispatchReleaseCLI|TestReleaseClaim'`: all pass except `TestReleaseClaimRemote`.

`TestReleaseClaimRemote` fails inside this sandbox. Its `git push -q origin main` can't reach the local bare origin, because the temp path resolves under `/dev/null/...`. That is the environment limitation the brief names (trial follow-up 3, before T-028). I read the remote-mode code path and found no defect. Product-ci should run that test.

**Blocking Issues:** none.

**Notes, not must-fix:**
- If another process deletes the claim between `ClaimOID` and `ReleaseLease`, the `ErrNotFound` from `ReleaseLease` goes through `claimError` and exits 3, not 0. This is a narrow race under `dispatch.lock`.
- Error text from the Comb is wrapped without `printable`. The text comes from the Comb's own errors and ref names, which are already validated.

Verdict: pass