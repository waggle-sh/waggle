# T-030 guard review (fable, confirming round on `d0c776e`)

**Verdict: pass**
**Risk-Tier: 3**

## Intent summary
Commit `d0c776e` answers the opus must-fix and the product-ci gofmt failure on `ec71b19`. The merged check in `branchMerged` now runs `rev-list --first-parent --parents --merges <head>..<main>`, so only merges on main's own first-parent history count, and the head must appear as a second or later parent of one of them. A new refusal case covers the opus probe: a no-commit `t-023` at an old main tip that another branch merged main from, before that branch merged into main. The fake backend's one-line method is split for gofmt, and the doc comment now states that the fetch and the rev-parse of main run unpinned as in `Run`. The verb itself is unchanged from the first round: `waggle dispatch release <ticket> [--branch] [--unmerged] [--repo]` takes `dispatch.lock`, refuses while any worktree, lone lock, tainted marker or takeover guard of the ticket exists, reads the claim OID, requires the merged check unless `--unmerged`, and deletes only through `comb.ReleaseLease` with the Keeper's own id and `combEnv`. Blast radius: the named ticket's claim ref, locally and on the remote, deleted only by its owner through the Comb's compare-and-swap. Against the merge base the branch adds only the four leased files; the hive-file deletions in a diff against the stated base are an artifact of the base being a later records batch, not changes on the branch.

## Verification of the fix
The `--first-parent` walk is sound. The merge that carries the head is a descendant of the head, so the `^head` exclusion can never drop it, and any merge the exclusion does drop is an ancestor of the head, which cannot have the head as a parent. A head is therefore reported merged exactly when a merge on main's first-parent chain lists it as a non-first parent. Squash and rebase merges, a head with no commits of its own, and the opus "updated from main" case all stay refused. The two new refusal cases and the merged case pass locally.

## Blocking issues
None.

## Notes (not blocking)
- **Head ref is worker-writable in the local fallback.** In remote mode the head is `origin/<branch>` after a fresh fetch, as designed. When that ref is absent the local branch is used, and a worker on that branch can point it at any already-merged commit. The only effect is that the queen's mistaken early release of that worker's own claim passes the check. The design chooses this fallback on purpose, and the harm stays inside the worker's own ticket.
- **Unpinned main resolution.** The rev-parse of main inside `ResolveBase` runs under the Keeper's config, matching `Run`. Every host git call already sets `GIT_NO_REPLACE_OBJECTS=1`, so replace refs cannot alter the parent lists the pinned rev-list reads.
- **Budget.** Non-test lines added are now 186: `release.go` 151 and `dispatch.go` 35. That is over the 180 target and under the 240 stop. The first build was already over target before review, so the planned cut was not taken; this round adds two lines. The queen decides whether that needs a word.
- **gofmt.** The `gofmt` call is denied in this cell. I checked the struct and method formatting by eye and found nothing off, but product-ci's gofmt step is the authority.

## What I ran
- `go build ./...` and `go vet ./...`: pass.
- `go test -race ./... -run 'TestReleaseClaim|TestDispatchReleaseCLI'`: `cmd/waggle` passes. In `internal/dispatch` every local-mode test passes, including the new first-parent case. `TestReleaseClaimRemote` fails only at the test helper's own `git push -q origin main`, with the known `/dev/null` path prefix (trial follow-up 3, until T-028 merges). The test got past its first push and its unmerged refusal with the claim still on the origin before that fault, so the failure is in the plain test helper, not in product code. That test still needs its result from product-ci.
- No probe files were created; `git status` is clean. Nothing committed.

Verdict: pass