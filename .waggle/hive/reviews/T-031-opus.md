I found no blocking issues in T-031, so the verdict is pass. One cosmetic fix and three notes are listed below; none of them blocks the merge.

## Review: T-031 (round 1, `00460f0..613e890`)

**Risk-Tier:** 3

**Intent Summary:** T-031 changes how `waggle gate verify`, `leases` and `tiers` work in four ways:
- **Binding:** a commit now binds to a ticket by its `T-<id>: ` subject prefix, a `Waggle-Ticket:` trailer (bounded) or the branch name (`--branch`, or `GITHUB_HEAD_REF` under Actions). The first source found wins, and any later source naming another ticket fails the gate.
- **Protected paths:** a new `gate.protected_paths` key in the base revision's `.waggle/control/config.yaml` adds Tier 3 paths. It can only raise a path's tier, never lower it. The file is read with `ls-tree` and `cat-file` with a size cap, so a symlink, directory, submodule, oversized file or invalid file makes the gate exit 3.
- **Base merges:** an "Update branch" merge is now accepted when `merge-base --all M base` prints exactly its second parent. It is diffed from that second parent and bound normally, falling back to its first parent's ticket.
- **T-008 entries:** `go.mod`, `go.sum`, `go.work`, `go.work.sum` and `vendor/` join the hardcoded Tier 3 list.

`CheckChanges` keeps its signature and now wraps `CheckChangesPolicy` with an empty policy, so dispatch's results change only for those five T-008 entries. Effect on other code: every PR gate in this repo (it is built from the PR base), plus dispatch's post-run check and `refuseDirs` for those five entries.

**Checks against the brief:**
- **Lease boundary (AC2):** only the allowed files changed. `go.mod`, `internal/dispatch/`, `internal/scaffold/` and `.github/` are untouched, and no existing test file was modified (AC11).
- **Policy source:** the policy is read only from `baseOID`. The head tree, worktree and merge-base are never read for it, and the head-side config test confirms this.
- **Hostile text:** messages carry only canonical ticket IDs or fixed reasons. The trailer value and branch name are never echoed, and config and git errors go through `annotData`.
- **Raise-only:** a hardcoded Tier 3 hit is checked first and keeps its text. A listed path goes to Tier 3 before the Tier 2 case, so a listed hive file is raised.
- **Base merges:** the merge-base rule rejects both crafted shapes. I also traced a side-branch commit forked at or after the merge-base. There `merge-base --all` gives just that commit, and the diff from it is exactly what a three-way merge would add to base, so nothing escapes the lease or tier rules.
- **Size:** roughly 330 added non-test lines, under the 670 target.

**Commands run:**

| Command | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | printed nothing |
| `go test -race ./...` | all packages ok |

**Blocking Issues:** none.

**Non-blocking notes for the Builder or Keeper:**
1. **Usage text (`cmd/waggle/gate.go:69-70`):** the new `--branch` paragraph sits between the `tiers` and `decisions` entries, which breaks up the Checks list. It should move after `decisions`.
2. **Trailer bound order (`trailerTickets`, `cmd/waggle/gate_commits.go`):** `strings.Split` runs before the 256-byte check on the field. The field is already in memory from `rev-list`, so this is not exploitable, but putting the length check first would match the brief's wording more closely.
3. **Empty trailer:** a trailer with an empty value (`Waggle-Ticket:` alone) produces an empty field and is treated as absent. It cannot get a commit past the lease or tier rules (the commit then falls back to the branch or stays unbound), so it is a note. It also needs a git new enough for `%(trailers:...,separator=)`.

I wrote no review file and no probe files, made no commits, and left the worktree clean. The text above is the content for `.waggle/hive/reviews/T-031.md`.

Verdict: pass