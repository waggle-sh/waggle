# T-030 guard review: must-fix, the merged check passes a branch with no commits of its own

Release passes a branch with no commits of its own as "merged into main", and deletes its claim, once main has absorbed a "merge main into feature" commit. The brief lists that case as must-fix. Everything else checks out.

**Verdict: must-fix**
**Risk-Tier: 3**

## Intent summary
Commit `ec71b19` adds `waggle dispatch release <ticket> [--branch] [--unmerged] [--repo]`. The new `internal/dispatch/release.go` does the following, in order:
- takes `dispatch.lock` through `lockRepo`;
- refuses with exit 4 if any worktree, lone lock, tainted marker or takeover guard of the ticket exists, using `clean`'s own checks;
- reads the claim with `ClaimOID`;
- unless `--unmerged` is given, requires the branch head to be the second or later parent of a merge commit on main;
- releases through `comb.ReleaseLease` with `keeper_id` and `combEnv`, and prints one line with the OID passed through `printable`.

`cmd/waggle/dispatch.go` routes the verb and adds the usage line. Only the four leased files change; nothing in `internal/comb`, `clean`, the gate or `go.mod` is touched.

Blast radius: claim refs, deleted locally and on the remote, but only the named ticket's, only the caller's own, and only through the Comb's compare-and-swap.

## Blocking issue

**The merged check passes a branch with no commits of its own** (`internal/dispatch/release.go:132-140`). The check runs `rev-list --parents --merges <head>..main` over *every* merge reachable from main, not just the merges on main's own history. So a no-commit `t-<nnn>` sitting at an old main tip X (for example a `no_change` run's leftover local branch) counts as merged in this case:
1. Some other feature branch is updated from main while main is at X (GitHub "Update branch", or `git merge main`). That makes X the second parent of a merge on the feature branch.
2. The feature branch then merges into main.

I confirmed this with a probe test in the fixture repo: release printed `released T-023 (claim …, merged into main)` and deleted the claim without `--unmerged`. Ordinary repo state triggers it, so it is must-fix under the brief's threat model (section 5).

**Fix for the builder:** add `--first-parent` to that rev-list, so only merges on main's own history count. With that flag, the probe's only qualifying merge has the feature branch, not X, as its second parent, so the release is refused. A normal `--merge` of a rebased ticket branch still qualifies. Add this case to `TestReleaseClaimRefusals` next to the existing first-parent case.

## Notes (not blocking)
- The `branchMerged` doc comment says every git call except the fetch is pinned, but the rev-parse of `<remote>/main` inside `ResolveBase` runs on the unpinned `g`. That matches how `Run` resolves its base, so it is acceptable; only the comment should say so.
- About 184 non-test lines were added: over the 180 target, under the 240 stop.

## What I ran
- `go build ./...` and `go vet ./...`: both pass.
- `go test -race ./... -run 'TestZZGuardProbe|TestReleaseClaim|TestDispatchReleaseCLI'`:
  - All local-mode release tests and `TestDispatchReleaseCLI` pass.
  - `TestReleaseClaimRemote` fails here: the push to the local origin gets a `/dev/null` prefix. The brief expects this until T-028 merges, so that test still needs a result from product-ci.
  - The probe test failed, which is the finding above.

## Cleanup needed
I couldn't delete my probe file: both `rm` and `git clean` were denied. I overwrote it with an inert `//go:build ignore` stub so it can't affect builds or tests, but it is still an untracked file and someone needs to remove it: `internal/dispatch/zz_guardprobe_test.go` in this worktree. I committed nothing.

Verdict: must-fix