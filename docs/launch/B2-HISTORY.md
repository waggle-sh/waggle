<!-- SPDX-License-Identifier: Apache-2.0 -->
# B2: git history before the repo goes public

Decision memo for readiness blocker B2 (T-062). It compares three ways to handle history and recommends one. This memo changes nothing: no history is rewritten, no ref is pushed, and repository visibility is not touched. Secret values are not printed here. Only categories are named.

## 1. Facts found in the repo

Read from the clone and from `gh` on 2026-11-19.

- **Visibility and reach.** The repo is private. Stars 0, forks 0. No outside repo can hold a copy through a fork. (Watchers were not readable with the available token; assume only the owner.)
- **History size.** 679 commits on `main`. 219 pull requests (open and closed). The issue count was not readable with the available token, so treat issues as unchecked.
- **What the history contains that must not go public.** Four categories, as the redaction cookbook (`docs/redaction/README.md`) records them:
  1. Three private author and committer email addresses on a subset of commits (one of them a private mail provider address). Commit counts are in the cookbook's census table.
  2. Machine-specific local paths: a per-user home path, a per-user macOS temp directory and a Claude scratch directory prefix, in hive probe logs, scripts and reviews.
  3. Company-internal file names and a design-repo home path, and a Vercel team slug, in hive briefs and reviews.
  4. A competitor survey and internal sizing and schedule notes in an earlier version of the repositioning note (the public text is already on `main`).
  Token-shaped strings (`ghp_`) in history are short test fixtures, not live credentials. The cookbook says to leave them.
- **Already done on the working tree.** T-058 added the cookbook, `docs/redaction/dry-run.sh` and the public repositioning text. T-062 B1 (PR 219, open) scrubs the current hive files. Neither touches history. Old blobs stay in old commits.
- **Tag and release.** `v0.1.0` is an annotated tag whose target commit is `220f1a7`. The GitHub release is published and marked Latest. It has six archives (darwin, linux and windows, amd64 and arm64) and `SHA256SUMS`. Each archive was built from that tag by `release.yml`, which refuses a binary whose `vcs.revision` stamp is not the tag commit.
- **Where the old SHA is load-bearing.**
  - The released binary is stamped `vcs.revision=220f1a7...`. `waggle init` uses a clean stamp as the gate Action pin (`internal/scaffold/init.go`, `gateRef`), so a user who installs v0.1.0 and runs `init --gate-only` gets a workflow pinned to `waggle-sh/waggle/gate@220f1a7...`.
  - `install.sh` (PR 220, open) fetches `SHA256SUMS` and the archives from `releases/latest` or a pinned release tag, and checks each archive against `SHA256SUMS`. It names no commit SHA. It depends on the release assets, not on history.
  - `.github/workflows/waggle-gate.yml` pins the in-repo gate to `3a9c054` (a pre-rewrite SHA). D-277 already says the first PR after a rewrite moves that pin.
  - Hive records cite pre-rewrite SHAs as text. D-277 says they stay as historical text.
- **Existing decisions.** D-235 decided a history rewrite after redaction. D-277 ordered the cards so no outside repo pins a SHA before the rewrite, and the hive's first PR after the rewrite is a pin move. D-308 added four T-042 brief commits to the list. This memo asks whether D-235 still holds, given what the release and PR refs add.
- **Contributors.** Identities on `main`: the Keeper under two display names (`paulternate`, `Paul Cleary`) and a second noreply account (3 commits), plus `Cursor Agent` (3 commits). About 305 commits carry a `Signed-off-by` trailer; the others are merge commits GitHub created and similar. 6 commit messages carry a `Co-authored-by` trailer. The `dco` CI job checks that each PR commit has a trailer matching its author email.

## 2. What must be true on the day the repo goes public

1. No commit, tag, branch or other ref reachable from a public URL contains the four categories above.
2. `v0.1.0` still installs and verifies, and a binary a user already has still produces a working gate pin.
3. Every link in public docs resolves.
4. Attribution is not falsified and DCO sign-offs stay valid.

## 3. Options

### A. Rewrite history in place with `git filter-repo`

Run the cookbook's mailmap and `--replace-text` / `--replace-message` expressions in a fresh mirror, then force-push every rewritten ref (D-235).

| Topic | Effect |
| :--- | :--- |
| **v0.1.0 tag and release** | `filter-repo` rewrites the annotated tag to point at the rewritten commit, so `v0.1.0` survives with a new tag object and a new target SHA. The GitHub release is keyed by tag name and stays published. Its six archives and `SHA256SUMS` are stored assets, not tied to a commit, so the checksums stay valid and `install.sh` keeps working unchanged. The tag's signature, if any, is lost (none was verified at inspection). |
| **The 220f1a7 pin** | `220f1a7` stops existing in the public history. The v0.1.0 binary still carries it as `vcs.revision`, so `waggle init` run from that binary writes a workflow pinned to a SHA that no longer resolves on a public repo. That breaks every new gate setup from v0.1.0. The binary cannot be rebuilt to the new SHA: the tree changed (scrubbed blobs), so the archives and their checksums would change, and `release.yml` would rightly refuse a rebuild whose stamp differs from the original. Fix: cut `v0.1.1` from the rewritten history right after the rewrite, make it Latest, and note on the v0.1.0 release that its pin is stale. `waggle-gate.yml` moves off `3a9c054` in the first post-rewrite PR (D-277). |
| **PR and issue links** | Links to PRs and issues by number keep working, because the PR objects stay on the same repo. PR diffs show the old commits through the hidden `refs/pull/<n>/head` refs, which a push cannot update (see residual exposure). Commit links in docs, hive records and PR bodies by old SHA break, or resolve only while GitHub still serves the unreachable object. |
| **Attribution and DCO** | Commit count and authorship stay. The mailmap changes the three private emails to the Keeper's public noreply identity, so each commit's `Signed-off-by` trailer, where it names a private address, is rewritten by `--replace-message` to match the new author. The `dco` check compares trailer email to author email, so the pairs stay consistent. Cost: the rewrite edits signed-off text after the fact. The Keeper made both the commits and the edit, so the certification is the Keeper's own. `Cursor Agent` commits and `Co-authored-by` trailers are unchanged. |
| **Stars, watchers, forks** | Kept (all zero today). Forks do not exist, so no fork carries old history. |
| **Effort** | Moderate. About 10 steps (section 5 runbook): freeze merges, merge pending PRs, copy the recipe out, mirror clone, run `filter-repo`, verify, force-push all refs, pin-move PR, release `v0.1.1`, support request. Most of the tooling exists already (cookbook and `dry-run.sh`). Reviewing PR bodies and comments for private text is extra work (below). |
| **Risk** | Medium. Force-pushing all refs, including `refs/waggle/claims/*`, can strand open branches and claims. Open PRs need rebasing onto the new `main`. A missed needle leaves private text in a public commit. A wrong mailmap re-attributes commits. |
| **Reversibility** | Good before the force push: keep the untouched mirror and a bundle of the old history. After the push, old SHAs are gone from branches and tags, but the backup restores them if needed. Anything already public after the visibility flip cannot be recalled. |
| **Residual exposure** | The weakest point. (1) GitHub keeps `refs/pull/<n>/head` and `refs/pull/<n>/merge` for all 219 PRs. These point at pre-rewrite commits, a force push does not touch them, and once the repo is public anyone can fetch them and browse them by SHA. (2) GitHub serves unreachable commits by full or short SHA until garbage collection, and cached views can outlive that. (3) PR titles, bodies, review comments and issue text are not part of git, so `filter-repo` never touches them. The B1 PR body style shows they carry hive detail, including the team slug and design-repo name categories. (4) Local clones and CI caches on the Keeper's machines. Forks: none now, so no fork exposure if the rewrite lands before the visibility flip. Closing (1) and (2) needs a GitHub Support request to remove the hidden PR refs and purge cached views for the old SHAs. That is slow and outside our control. |

### B. Fresh squashed public repo

Create a new repository, import one squashed commit of the scrubbed tree, and publish that. The old private repo stays as the archive.

| Topic | Effect |
| :--- | :--- |
| **v0.1.0 tag and release** | The old tag is not carried. A new `v0.1.0` on the new repo would point at the squash commit, a different tree from the one the release binary was built from, and `release.yml` would refuse to rebuild it with a matching stamp. Honest options: publish `v0.1.1` (or `v0.2.0`) as the first public release, built fresh by `release.yml`, and leave v0.1.0 private; or re-upload the six archives to a new v0.1.0 release whose binary stamp names a commit that does not exist in the new repo. The first is clean. The second is not. |
| **The 220f1a7 pin** | The commit is absent from the new repo. Same break as A, with no workaround through Support: the old binary's pin can never resolve. Anyone holding a v0.1.0 binary (so far, one download per asset) has a dead pin. Because the Action path is `waggle-sh/waggle/gate`, a new repo under a different name or org also changes the Action path. |
| **PR and issue links** | All 219 PRs and every issue are left behind. Public docs, hive records and `docs/` cite PR numbers and commit SHAs (hive decisions cite both). All of those become dead or point at a private repo. Hive records are text history, so they can stay stale, but a reader cannot follow them. PR numbers restart at 1 in the new repo. |
| **Attribution and DCO** | Loses per-commit authorship. One commit carries one author line; 679 commits, three identities and the `Cursor Agent` contributions collapse. A single `Signed-off-by` for the importer is valid DCO, but it certifies the whole tree in one statement. The record of who wrote what and when is gone from the public repo (it stays in the private archive). |
| **Stars, watchers, forks** | Reset. All zero today, so the loss is nil. Do it now or never: this cost grows with every star. |
| **Effort** | Moderate to high. Create the repo, replicate settings (branch protection, the required `verify-leases-and-tiers` check, CODEOWNERS, secrets, Vercel link, Actions permissions), import and scrub the tree, cut a new first release, update every pin and URL in README, `site/` and `install.sh`, and keep two repos in step during the move. |
| **Risk** | Low for private-text leakage: nothing old is in the new repo, and PR bodies and refs cannot leak because none move. Higher for operations: forgotten settings, broken URLs, and a gate Action path that changes. |
| **Reversibility** | High before launch: the old repo is untouched. After launch the new repo's PR numbers and public SHAs are fixed. Moving back to full history later is a second rewrite plus a force push on a public repo. |
| **Residual exposure** | Smallest of the three. The new repo has no old refs, PR refs or issue text. The old repo stays private, so its exposure is its current exposure (it has none beyond access control). The one carry-over risk is anything pasted into the tree itself, which the scrub covers. |

### C. Keep history as is

Make the repo public with all 679 commits, 219 PRs and every ref.

| Topic | Effect |
| :--- | :--- |
| **v0.1.0 tag and release** | Nothing changes. Tag, release, checksums, `install.sh` and the `220f1a7` pin all keep working. |
| **The 220f1a7 pin** | Resolves. |
| **PR and issue links** | All work. |
| **Attribution and DCO** | Unchanged and exact. |
| **Stars, watchers, forks** | Unchanged. |
| **Effort** | None beyond the visibility step. |
| **Risk** | High. The four categories of private text, and the private mail addresses, are published permanently the moment the repo flips. The cookbook's census shows 126+ commits with a private mail address as author and committer, and dozens of hive files with local paths. This breaks requirement 1 in section 2 directly. |
| **Reversibility** | None. Visibility can go back to private, but the content may be cloned or indexed within minutes, so it cannot be recalled. Mail addresses in git history are scraped by bots. |
| **Residual exposure** | Total: all of it. Rejected. Option C is listed to show what A and B buy. |

## 4. Comparison

| | A. Rewrite in place | B. Fresh squash | C. As is |
| :--- | :--- | :--- | :--- |
| Private text gone from public refs | Yes | Yes | No |
| Private text gone from PR refs, PR bodies, caches | No until Support acts | Yes | No |
| v0.1.0 assets and checksums valid | Yes | Only as a private archive | Yes |
| `220f1a7` pin resolves | No (cut v0.1.1) | No (cut a new release) | Yes |
| PR numbers and links | Kept; commit-SHA links break | Lost | Kept |
| Per-commit authorship, DCO | Kept (mailmap applied) | Lost | Kept, exact |
| Effort | Moderate | Moderate to high | None |
| Reversible before launch | Yes (backup) | Yes (old repo untouched) | No |

## 5. Recommendation

**Recommend A, with a hard gate and a named fallback to B.**

Why A: D-235 already chose it, the cookbook, mailmap, expression list and `dry-run.sh` are written and reviewed, and it keeps the 679-commit record, per-commit authorship and the DCO trail that the project's own policy is about. With zero stars, zero forks and no outside pins (D-277), a rewrite now costs the least it ever will. The v0.1.0 release assets and `install.sh` survive untouched, and the one real break, the stale `220f1a7` pin inside the v0.1.0 binary, is fixed by cutting `v0.1.1`, which the T-062 install flow wants anyway.

Why a gate: A has one weakness that a force push cannot fix. The hidden PR refs and PR text keep old content reachable once the repo is public. So the visibility flip is blocked on this check, run by the Head Keeper:

- PR bodies, PR comments and issue text were audited for the four categories and edited where they hit; and
- GitHub Support confirmed in writing that the old `refs/pull/*` refs and cached views for the pre-rewrite SHAs are removed or, at minimum, the old SHAs no longer resolve.

**If either condition cannot be met before launch, switch to B** (fresh repo, first public release `v0.1.1`, old repo kept private as the archive). B is the safe exit, and its main cost, losing PR history and authorship detail, is a cost the project can still afford because stars and forks are zero. Do not choose C.

Decision requested from the Head Keeper: confirm A with this gate, or choose B now. Either answer supersedes nothing in D-235 for A; for B it needs a new decision that supersedes D-235 and D-277.

## 6. Runbook for the recommended option (A)

Every step that changes the remote is a Head Keeper step. None of it is done by this memo. At no step is repository visibility changed; the flip is a separate decision after step 14.

**Before the rewrite**

1. Land the leaf work that must be in the rewritten tree: B1 scrub (PR 219), `install.sh` (PR 220), and any other open T-062 branch. Then freeze merges to `main` and tell contributors.
2. Release or let expire the active claims that lease paths the scrub touches (the cookbook names claim `ticket-55`). Note that claim refs are rewritten and force-pushed with everything else.
3. Copy `docs/redaction/` out of the clone to a directory outside it. The expression file holds the private strings, and `--replace-text` would rewrite it inside history.
4. Run `docs/redaction/dry-run.sh` and save the counts as the "before" figures.
5. Take backups: `git clone --mirror` of the current remote to a private location, plus `git bundle create backup.bundle --all`. Keep both until after launch, then archive them off the machine.
6. Audit PR bodies, PR review comments and issue text for the four categories (for example with `gh api` and the scanner's rules). Edit hits in place. Record in the PR for this memo's follow-up which categories hit, never the values.

**The rewrite**

7. In a fresh mirror, not a working checkout (`filter-repo` removes the origin remote):
   ```sh
   git clone --mirror git@github.com:waggle-sh/waggle.git waggle-rewrite.git
   cd waggle-rewrite.git
   git filter-repo \
     --mailmap /ABSOLUTE/PATH/OUTSIDE/mailmap \
     --replace-text /ABSOLUTE/PATH/OUTSIDE/expressions.txt \
     --replace-message /ABSOLUTE/PATH/OUTSIDE/expressions.txt
   ```
8. Run the cookbook's verification block in the mirror: author and committer fields, commit messages, and `git grep` of every commit tree. Every search must print nothing. Confirm the kept strings are still present (copyright line in `NOTICE`, owner in `CODEOWNERS`, the T-046 probe file). Confirm `v0.1.0` exists and its tag object points at a commit with the same tree as the old `220f1a7` apart from the scrubbed files:
   ```sh
   git rev-parse 'v0.1.0^{commit}'
   git diff --stat <old-backup>:220f1a7 v0.1.0   # run against the backup; only scrubbed paths differ
   ```
9. Re-read the tag object. Confirm it is annotated (`release.yml` rejects a lightweight tag).

**Publish**

10. Force-update every rewritten ref in one push from the mirror, including `main`, tags and `refs/waggle/claims/*`. Do not use `--mirror` against hidden refs; GitHub rejects updates to `refs/pull/*`, and that is expected. Delete branches that exist only on old history.
11. Immediately, before any other PR merges (D-277): open and merge the hive's pin-move PR, re-rendering `.github/workflows/waggle-gate.yml` to a post-rewrite SHA. Rebase or close the open PRs onto the new `main`.
12. Tag and release `v0.1.1` from the rewritten `main` through `release.yml`, publish the draft, and make it Latest. Edit the v0.1.0 release notes to say that its embedded gate pin is stale and to use v0.1.1. Do not delete v0.1.0 or its assets: the checksums stay valid. Check `install.sh` against v0.1.1 and a pinned `WAGGLE_VERSION=0.1.0`.

**Close the residual exposure**

13. File a GitHub Support sensitive-data removal request for the repo: remove the cached views and hidden `refs/pull/*` for the pre-rewrite SHAs, and run garbage collection. Keep the reply.
14. Re-check from a clean clone with `git ls-remote origin` and by fetching `refs/pull/*`: no pre-rewrite SHA should resolve (`gh api repos/waggle-sh/waggle/commits/<old-sha>` should return 404). Re-run the cookbook verification searches over everything fetched.

**Gate to launch**

15. Visibility stays private until steps 6, 13 and 14 pass. If they do not, stop and switch to option B. The flip itself is a separate, explicit Head Keeper action and is not part of this runbook.

**After**

16. Destroy the expression file and mailmap copies once launch is done. Keep the backup bundle off-machine in case an attribution dispute needs the old identities. Pre-rewrite SHAs in hive records stay as historical text (D-277); add one line to the hive records, in a normal records batch, that says the rewrite happened and where the pin moved.
