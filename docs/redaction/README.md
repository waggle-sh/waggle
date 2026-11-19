<!-- SPDX-License-Identifier: Apache-2.0 -->
# History rewrite cookbook

Dry-run only. Nothing in this directory rewrites git history, force-pushes, changes repository visibility, or merges a pull request. `docs/redaction/dry-run.sh` takes no arguments. It reads commits and prints counts. It does not invoke `git-filter-repo`.

Run the scanner from the repository root:

```sh
docs/redaction/dry-run.sh
```

Copy this directory out of the clone before any real rewrite. The expression file contains the private strings, and `--replace-text` rewrites those strings inside this directory too. The copy outside the clone is the recipe that still has the old strings. The copy inside the rewritten history will not.

This pull request does not rewrite history. It adds this recipe, the public text of `docs/rfc-notes/repositioning.md`, and one leased fixture edit. Hive files and `docs/rfc-notes/` are not edited here. The reasons are in "Working tree" below.

## What stays

- `NOTICE` copyright line `Level 19 Enterprises, LLC`.
- `.github/CODEOWNERS` owner `@paulternate`.
- The whole `.waggle/hive/` tree, including `.waggle/hive/media/T-046/probe/atk-delete-post.yml` and the rest of `media/T-046/probe/`.
- Author and committer identity `27632860+paulternate@users.noreply.github.com`, whatever name it already has (`paulternate` or `Paul Cleary`).
- Committer `noreply@github.com` / `GitHub` on commits GitHub itself created.
- `ghp_` strings. They are short test fixtures, not live tokens. Do not treat them as secrets to strip.
- Generic prose that names the macOS temp root without a machine-specific directory under it. That is how the repo talks about macOS temp dirs, including `.waggle/control/scripts/scan-hive.sh`.

## Mailmap

`git-filter-repo --mailmap` reads this file. The short form maps every author and committer name on that email to the new name and email ([gitmailmap](https://git-scm.com/docs/gitmailmap)):

```
Paul Cleary <27632860+paulternate@users.noreply.github.com> <27632860+paulternate@users.noreply.github.com>
Paul Cleary <27632860+paulternate@users.noreply.github.com> <27632860+paulternate@users.noreply.github.com>
Paul Cleary <27632860+paulternate@users.noreply.github.com> <27632860+paulternate@users.noreply.github.com>
```

Counted on `origin/main` at `8511c8d` before this branch, plus `origin/t-056-self-approve` at `9fa9ac1`:

| Author email | Author name | Committer | Commits |
| :--- | :--- | :--- | ---: |
| `27632860+paulternate@users.noreply.github.com` | `paulternate` | same noreply address | 361 |
| `27632860+paulternate@users.noreply.github.com` | `Paul Cleary` | `noreply@github.com` / `GitHub` | 141 |
| `27632860+paulternate@users.noreply.github.com` | `paulternate` | same email address | 126 |
| `27632860+paulternate@users.noreply.github.com` | `Paul Cleary` | `noreply@github.com` / `GitHub` | 68 |
| `27632860+paulternate@users.noreply.github.com` | `Paul Cleary` | `noreply@github.com` / `GitHub` | 4 |
| `27632860+paulternate@users.noreply.github.com` | `paulternate` | `27632860+paulternate@users.noreply.github.com` | 3 |
| `27632860+paulternate@users.noreply.github.com` | `paulternate` | `27632860+paulternate@users.noreply.github.com` | 2 |

The mailmap rewrites the three private emails only. On those commits the name becomes `Paul Cleary`, including the 126 email commits and the 2 `27632860+paulternate@users.noreply.github.com` commits whose name is currently `paulternate`. The same map changes 131 committer identities `paulternate <27632860+paulternate@users.noreply.github.com>` (the 126, the 2, and 3 commits whose author email is already the numeric noreply address). Commits that already use `27632860+paulternate@users.noreply.github.com` stay as they are, including the ones whose name is `paulternate` and the ones whose committer is `noreply@github.com` / `GitHub`. `dry-run.sh` prints this with `%aN`/`%aE` and `%cN`/`%cE`. `%an`/`%ae` stay raw and are the wrong preview.

`9fa9ac1` (PR 201 tip) is `Cursor Agent <cursoragent@cursor.com>` with `Co-authored-by: Paul Cleary <paulternate@users.noreply.github.com>`. That id-less noreply form is not one of the three private emails and is not the numeric noreply address. The approved mailmap does not change it.

`docs/redaction/dry-run.sh` reprints this census with `git log --use-mailmap`. That flag changes the displayed identity only.

## Blob and message replacements

`git-filter-repo` applies each `literal:` line to the current bytes, then the next line. Order is the behavior. A longer needle that contains a later needle is listed first. Pass the same file to `--replace-text` (blobs) and `--replace-message` (commit messages). The mailmap does not edit messages. Twenty-six commits contain `Signed-off-by: Paul Cleary <27632860+paulternate@users.noreply.github.com>`. No commit message contained the other needles at the census above.

`27632860+paulternate@users.noreply.github.com` is in author fields only. It is still listed so a later blob cannot keep it.

```
literal:Signed-off-by: Paul Cleary <27632860+paulternate@users.noreply.github.com>==>Signed-off-by: Paul Cleary <27632860+paulternate@users.noreply.github.com>
literal:27632860+paulternate@users.noreply.github.com==>27632860+paulternate@users.noreply.github.com
literal:27632860+paulternate@users.noreply.github.com==>27632860+paulternate@users.noreply.github.com
literal:27632860+paulternate@users.noreply.github.com==>27632860+paulternate@users.noreply.github.com
literal:/private/tmp/claude-redactedredactedredacted/waggle-scratch==>/private/tmp/claude-redactedredactedredacted/waggle-scratch
literal:/tmp/claude-redactedredacted==>/tmp/claude-redactedredacted
literal:/var/folders/xx/redacted==>/var/folders/xx/redacted
literal:/Users/example==>/Users/example
literal:~/redacted/znap==>~/redacted/znap
literal:redacted-company.md==>redacted-company.md
literal:redacted.local==>redacted.local
```

Two extensions of the approved strings, found while checking HEAD:

- `/tmp/claude-redactedredacted` is the same uid-qualified temp directory as `/private/tmp/claude-redactedredactedredacted/...`, without the `/private` prefix. It is in the T-006 and T-007 probe logs. It is also a substring of the longer scratchpad path, so it must be the later rule.
- `~/redacted/znap` is the tilde form of `/Users/example/Repos/znap` in `.waggle/hive/reviews/T-020c-sonnet.md`. The `/Users/example` rule does not match a tilde.

`redacted.local` is absent from HEAD. One commit tree still has it: `7cfed306e7`, in the probe logs `.waggle/hive/media/T-023b/fable-recheck-probes.txt`, `fable-recheck2-probes.txt`, `opus-fixpass-output.txt`, and `opus-fixpass2-output.txt`. The next records commit removed it from those files. `--replace-text` still has to rewrite that blob. `.waggle/hive/handoff/queen.md` likewise held the email address and the scratchpad path in history and is clean at HEAD.

## Intended git-filter-repo invocation

Do this in a fresh mirror, after the merges named in "Order". Do not run it in a working checkout of this repository. `git-filter-repo` deletes the origin remote of the clone it rewrites, which is a useful stop, and it rewrites every ref in the mirror, including other branches and `refs/waggle/claims/`.

```sh
git clone --mirror git@github.com:waggle-sh/waggle.git waggle-rewrite.git
cd waggle-rewrite.git
git filter-repo \
  --mailmap /ABSOLUTE/PATH/OUTSIDE/mailmap \
  --replace-text /ABSOLUTE/PATH/OUTSIDE/expressions.txt \
  --replace-message /ABSOLUTE/PATH/OUTSIDE/expressions.txt
```

The outside `mailmap` and `expressions.txt` are the fenced files above, copied out before this command. `dry-run.sh` writes the same bytes to a temporary directory when it runs, then deletes that directory.

Stop there. Inspect with the verification below. A publish push is a separate Head Keeper step. It has to be a force update of every rewritten ref, including `main` and the claim refs, or old commits stay reachable from any ref that was left behind. Repository rules that block force-push apply. This cookbook does not run that push, and this pull request does not ask anyone to run it now.

After the publish, the first pull request on the rewritten history is the pin move from D-277, before any other pull request merges. Pre-rewrite commit ids in hive records stay as historical text. They will not resolve.

## Verification after a real rewrite

Run these in the rewritten mirror. They are reads. Success is empty output from each search. `git grep` exits 1 when it finds nothing; with `set -e` that exit is a pass if the script allows it.

Author and committer fields:

```sh
git log --all --format='%ae%n%ce' | grep -E 'email\.com$|@pm\.me$|@l19\.ltd$' && echo FAIL || echo 'author fields clean'
```

Commit messages:

```sh
git log --all --format='%B' | grep -F -e '27632860+paulternate@users.noreply.github.com' -e '27632860+paulternate@users.noreply.github.com' -e '27632860+paulternate@users.noreply.github.com' -e '/Users/example' -e 'redacted.local' -e 'claude-redacted' -e 'redacted-company.md' -e '/var/folders/xx/redacted' -e '~/redacted/znap' && echo FAIL || echo 'messages clean'
```

Every commit tree. `claude-redacted` is the needle that covers both temp prefixes. A clean history prints no lines:

```sh
git rev-list --all | xargs -n 30 git grep -I -F \
  -e '27632860+paulternate@users.noreply.github.com' \
  -e '27632860+paulternate@users.noreply.github.com' \
  -e '27632860+paulternate@users.noreply.github.com' \
  -e '/Users/example' \
  -e 'redacted.local' \
  -e 'claude-redacted' \
  -e '/var/folders/xx/redacted' \
  -e 'redacted-company.md' \
  -e '~/redacted/znap'
```

Also confirm the kept strings are still present: `Level 19 Enterprises, LLC` in `NOTICE`, `@paulternate` in `.github/CODEOWNERS`, and `.waggle/hive/media/T-046/probe/atk-delete-post.yml` in the HEAD tree.

`dry-run.sh` is the same search before the rewrite. Its "HEAD outside docs/redaction" section is the working-tree leftovers. Its history section counts commits whose trees still contain each needle. After the rewrite both should be empty, including inside `docs/redaction/`, because the expression file in git was rewritten too.

## Working tree

Two lease blocks. This branch does not edit those paths.

**Hive.** A worker lease cannot name `.waggle/` or anything under it. `isWaggleDir` refuses it, and the hive carve-out does not make hive leasable (`internal/comb/backend.go` `isWaggleDir`). Hive changes need a Head Keeper admin merge, or the Head Keeper approval path once that gate behavior is what the repo runs. Do not delete the hive.

**`docs/rfc-notes/`.** Claim `refs/waggle/claims/ticket-55` is still active: keeper `paulternate`, directories `CONTRIBUTING.md` and `docs/rfc-notes`, branches `t-055-vision-restore` and `t-055-docs`, `expires_at` `2026-11-08T02:53:02Z`. Pull request 197 is already merged. The claim ref was not released. Overlap at claim time refuses another ticket on `docs/rfc-notes` until that expiry or a release by `paulternate`. This branch does not edit `docs/rfc-notes/repositioning.md`.

What this branch does lease (`T-058`, directories `docs/redaction` and `internal/runtime`):

- This cookbook.
- `docs/redaction/repositioning.public.md`, the approved public text. It keeps the ruled product decisions, including the D-235 eight-week / three-outside-repos kill criterion, and it drops the competitor survey and the internal sizing and schedule notes.
- `internal/runtime/stream_test.go`, the only HEAD file outside `.waggle/` that contained the scratchpad prefix. The test checks the session id and the malformed-line count, not the path bytes.

### Admin apply, after the leases are clear

Copy `docs/redaction/` outside the repository first. Then, in a Head Keeper commit, not in this pull request:

1. Copy the outside `repositioning.public.md` onto `docs/rfc-notes/repositioning.md`. Do this only after ticket 55 is released or after `2026-11-08T02:53:02Z`.
2. Apply the expression list, in order, to the hive files below. Do not run it on `docs/redaction/` unless the outside copy already exists. Do not delete files. Do not touch `NOTICE`, `CODEOWNERS`, or `media/T-046/probe/`.

Snapshot of HEAD files outside `docs/redaction` that match the needles, after the `stream_test.go` edit in this branch. `dry-run.sh` reprints the live list.

- `.waggle/hive/media/T-006/cli-run.txt`
- `.waggle/hive/media/T-007/probe.sh`
- `.waggle/hive/media/T-007/probe2.sh`
- `.waggle/hive/media/T-007/probe3.sh`
- `.waggle/hive/media/T-007/probe4.sh`
- `.waggle/hive/media/T-007/probes.txt`
- `.waggle/hive/media/T-011b/ac.sh`
- `.waggle/hive/media/T-011b/pre.sh`
- `.waggle/hive/media/T-011b/preconditions.txt`
- `.waggle/hive/media/T-014b/p1.sh`
- `.waggle/hive/media/T-014b/p2.sh`
- `.waggle/hive/media/T-014b/p3.sh`
- `.waggle/hive/media/T-014b/p4.sh`
- `.waggle/hive/media/T-014b/p5.sh`
- `.waggle/hive/media/T-014b/p6.sh`
- `.waggle/hive/media/T-016/hostile.txt`
- `.waggle/hive/media/T-016/hostile4-symlinks.txt`
- `.waggle/hive/media/T-016/rehearsal-diff.txt`
- `.waggle/hive/media/T-020b/log.txt`
- `.waggle/hive/media/T-020c/pr-body.md`
- `.waggle/hive/media/T-023a/fable-run1-permissions.jsonl`
- `.waggle/hive/media/T-023a/fable-run2-prompt.txt`
- `.waggle/hive/media/T-023a/fable-run2-tools.jsonl`
- `.waggle/hive/media/T-023a/fable-run3-restricted-prompt.txt`
- `.waggle/hive/media/T-023a/fable-run3-restricted.jsonl`
- `.waggle/hive/media/T-023a/fable-spec.json`
- `.waggle/hive/media/T-023a/fable-worker-bash-env.txt`
- `.waggle/hive/media/T-023a/probe-control.jsonl`
- `.waggle/hive/media/T-023a/probe-repo/.claude/hooks/start.sh`
- `.waggle/hive/media/T-023a/probe-repo/.claude/hooks/stop.sh`
- `.waggle/hive/media/T-023a/probe-worker.jsonl`
- `.waggle/hive/media/T-023a/sonnet-checks.txt`
- `.waggle/hive/media/T-023a/sonnet-live-compound.jsonl`
- `.waggle/hive/media/T-023a/sonnet-live-prompt2.txt`
- `.waggle/hive/media/T-023a/sonnet-maxturns1.jsonl`
- `.waggle/hive/media/T-023a/sonnet-rereview-gobuild.jsonl`
- `.waggle/hive/media/T-023a/sonnet-rereview-spec.json`
- `.waggle/hive/media/T-023a/sonnet-writescope-bare.jsonl`
- `.waggle/hive/media/T-023a/sonnet-writescope-scoped.jsonl`
- `.waggle/hive/reviews/T-002.md`
- `.waggle/hive/reviews/T-020b.md`
- `.waggle/hive/reviews/T-020c-fable.md`
- `.waggle/hive/reviews/T-020c-sonnet.md`
- `.waggle/hive/reviews/T-025a.md`
- `.waggle/hive/reviews/hive-records-4.md`
- `.waggle/hive/reviews/hive-records-6.md`
- `.waggle/hive/tasks/T-025.md`

Known HEAD hits the approved note named, all on that list: `27632860+paulternate@users.noreply.github.com` in `.waggle/hive/media/T-020b/log.txt` lines 1 and 4; `27632860+paulternate@users.noreply.github.com` in `.waggle/hive/reviews/T-020c-sonnet.md`, `hive-records-4.md`, `hive-records-6.md`, and `.waggle/hive/media/T-023a/fable-run2-tools.jsonl`; `/Users/example` across the hive files above; `redacted-company.md` in `.waggle/hive/tasks/T-025.md` and also `.waggle/hive/reviews/T-025a.md`.

Apply with the outside expression file, in order, only to those paths:

```sh
python3 - "$OUTSIDE/expressions.txt" <<'PY'
import pathlib, sys
pairs = []
for line in pathlib.Path(sys.argv[1]).read_text().splitlines():
    if line.startswith("literal:"):
        pairs.append(line[len("literal:"):].split("==>", 1))
root = pathlib.Path(".").resolve()
for path in pathlib.Path(".waggle/hive").rglob("*"):
    if not path.is_file():
        continue
    data = path.read_bytes()
    new = data
    for old, repl in pairs:
        new = new.replace(old.encode(), repl.encode())
    if new != data:
        path.write_bytes(new)
        print(path.relative_to(root))
PY
```

That snippet is for the admin commit. `dry-run.sh` does not contain it and does not write the tree.

## Order, including pull request 201

Pull request 201 (`T-057`, branch `t-056-self-approve`, head `9fa9ac1392a228919854f2741b60003c4888de85`) is open. Product CI and the gate check passed. GitHub still shows review required: the author is `paulternate`, and GitHub does not count his review of his own pull request. Do not merge it from this work.

Run the rewrite after that merge, and after this cookbook's working-tree scrub is on `main` (this pull request, plus the admin hive and repositioning commits). T-057 should be inside the rewritten history. Its tip does not add a new private email. It still carries the same hive path strings as `main`, plus the `cursoragent@cursor.com` author. If the rewrite runs first, merging pull request 201 afterward brings pre-rewrite commits back, because that branch is based on today's `main`.

If a rewrite is ever started before pull request 201 merges, run this whole cookbook again after the merge. One rewrite, after both lands, is the clean sequence.

Other open branches and the old `origin/hive-records-*` branches need to be in the mirror. A ref left on a pre-rewrite commit keeps that commit reachable on GitHub.

## Residuals the approved rules do not rewrite

These are still in HEAD after the expression list. They are not private emails, the home prefix, the temp prefixes, or the hostname. Listed so a later pass can decide.

- The word `email` in review prose, after the full address is replaced: `.waggle/hive/media/T-020c/pr-body.md`, `.waggle/hive/reviews/T-020c-fable.md`, `.waggle/hive/reviews/T-020c-sonnet.md`, `.waggle/hive/reviews/hive-records-7.md`, `hive-records-8.md`, `hive-records-9.md`, `hive-records-10.md`.
- `USER=example` and `LOGNAME=example` in `.waggle/hive/media/T-023a/fable-spec.json`, `fable-worker-bash-env.txt`, and `sonnet-rereview-spec.json`.
- macOS `ls` owner `paul` in `.waggle/hive/media/T-011a/init-runs.txt`, `.waggle/hive/media/T-016/hostile4-symlinks.txt`, and `.waggle/hive/media/T-016/rehearsal-rerun.txt`.
- The level19 Vercel team name and plan in `.waggle/hive/DECISIONS.md` (D-313, D-318). `DECISIONS.md` is append-only, so a working-tree edit is not allowed; the history rewrite is the only fix. The design repo path in the T-041 briefs, the team name and plan in `tasks/T-041.md` and the briefs, and the machine temp directory in `media/T-023b/` were scrubbed in T-062.
- `Cursor Agent <cursoragent@cursor.com>` and `Co-authored-by: Paul Cleary <paulternate@users.noreply.github.com>` on `9fa9ac1`, once pull request 201 merges.
- The public handle `paulternate` inside the captured `.gitconfig` text in `fable-run2-tools.jsonl`. `CODEOWNERS` keeps `@paulternate`, so the login is not a strip target.

## What a rewrite does not delete (D-277)

GitHub keeps replaced commits reachable by their old ids in cached views, and through pull requests that point at them, until GitHub Support garbage-collects them. D-277 already says the redaction asks Support to do that, and that nothing may rely on those old ids.

A history rewrite does not delete:

- Pull request titles, bodies, and review comments.
- The "Files changed" text GitHub stored for each pull request. This pull request's diff contains the expression file, so it is on that list.
- Actions logs for this repository, including gate logs and any log line that echoed a path or an email.
- Forks, and clones anyone already has.
- A ref that the mirror rewrite did not update.

Support purge is a separate ticket after the rewrite, not part of this cookbook.
