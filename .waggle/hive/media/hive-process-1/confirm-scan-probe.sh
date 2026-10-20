#!/usr/bin/env bash
# Confirmation probe for scan-hive.sh at 5c7ada6 (hive-process-1): the
# records path, history evasions against the per-commit pass, and type flips.
# Each scenario is a detached commit on BASE (never a branch, never pushed).
# The planted address is built at run time; none is written to this file.
# Usage: confirm-scan-probe.sh <base sha> <scan script>   (repo root, clean tree)
set -u
BASE=$1
SCAN=$2
M=.waggle/hive/media/confirm-plant
GC=(git -c user.name=fixture -c user.email=fixture@example.test -c commit.gpgsign=false)
PD=gmail; PD=$PD.com; HIT="contact jane.doe@$PD"

report() { # name, sha
  local out rc
  out=$("$SCAN" "$BASE" "$2" 2>&1); rc=$?
  echo "== $1: exit $rc"
  printf '%s\n' "$out" | grep -E 'HOLD|commit\(s\) scanned' | sed 's/^/   /'
  printf '%s\n' "$out" | sed -n '/^hits/,/^result/p' | sed 's/^/   /'
}
fresh() { git checkout -q --detach "$BASE"; rm -rf "$M"; mkdir -p "$M"; }
ci() { git add -A -- .waggle; "${GC[@]}" commit -q --allow-empty -m "$1"; }
w() { printf '%s\n' "$2" >"$1"; }

echo "### records path (expect exit 0)"
fresh
printf '\n## D-187: probe entry\n\n- **Date:** 2026-10-19\n- **Kind:** policy\n- **Tickets:** none\n- **Supersedes:** none\n\n**Question:** Probe?\n\n**Options considered:**\n1. Yes\n2. No\n\n**Hive recommendation:** Yes\n\n**Keeper choice:** Yes.\n\n**Rationale:** Probe.\n' >>.waggle/hive/DECISIONS.md
w .waggle/hive/reviews/confirm-probe.md "new review"
w "$M/a.md" "new media"
mkdir -p "$M/sub"; w "$M/sub/b.txt" "nested media"
echo "probe line" >>.waggle/hive/handoff/queen.md
ci "records batch probe"; report records-batch "$(git rev-parse HEAD)"

echo "### records set edges"
fresh; sed 's/^\*\*Rationale:\*\* Saves one wait per fix round/**Rationale:** Edited/' .waggle/hive/DECISIONS.md >d.tmp && mv d.tmp .waggle/hive/DECISIONS.md
ci dec-rewrite; report decisions-rewrite "$(git rev-parse HEAD)"
fresh; git mv .waggle/hive/media/hive-process-1/scan-branch.txt "$M/scan-branch.txt"; ci mv; report rename-existing-media "$(git rev-parse HEAD)"
fresh; chmod +x .waggle/hive/handoff/queen.md; ci mode; report queen-md-mode-change "$(git rev-parse HEAD)"
fresh; w "$M/.gitignore" "*.tmp"; ci gi; report add-gitignore-in-media "$(git rev-parse HEAD)"

echo "### history evasions"
# Added in one commit, replaced in the next.
fresh; w "$M/a.md" "$HIT"; ci h1; w "$M/a.md" "redacted"; ci h2; report added-then-removed "$(git rev-parse HEAD)"
# Added, then undone by git revert.
fresh; w "$M/a.md" "$HIT"; ci r1; "${GC[@]}" revert --no-edit HEAD >/dev/null; report added-then-reverted "$(git rev-parse HEAD)"
# Amended away: the hit commit is no longer reachable from head.
fresh; w "$M/a.md" "$HIT"; ci a1; w "$M/a.md" "clean"; git add -A -- .waggle; "${GC[@]}" commit -q --amend -m a1; report amended-away "$(git rev-parse HEAD)"
# Hit in a side-branch commit merged as the second parent, removed afterwards.
fresh; w "$M/side.md" "$HIT"; ci s1; side=$(git rev-parse HEAD)
fresh; w "$M/main.md" "m"; ci m1; "${GC[@]}" merge -q --no-ff --no-edit "$side" >/dev/null
w "$M/side.md" "redacted"; ci s2; report side-commit-merged-then-removed "$(git rev-parse HEAD)"
# Evil merge: the merge result adds a line in neither parent; a later commit removes it.
fresh; w "$M/side.md" "s"; ci e1; side=$(git rev-parse HEAD)
fresh; w "$M/main.md" "m"; ci e2; "${GC[@]}" merge -q --no-ff --no-commit "$side" >/dev/null 2>&1
w "$M/evil.md" "$HIT"; git add -A -- .waggle; "${GC[@]}" commit -q --no-edit
w "$M/evil.md" "redacted"; ci e3; report evil-merge-then-removed "$(git rev-parse HEAD)"
# Same, but the hit edits a file both parents already have.
fresh; w "$M/both.md" "base"; ci f0; b0=$(git rev-parse HEAD)
w "$M/side.md" "s"; ci f1; side=$(git rev-parse HEAD)
git checkout -q --detach "$b0"; w "$M/main.md" "m"; ci f2; "${GC[@]}" merge -q --no-ff --no-commit "$side" >/dev/null 2>&1
w "$M/both.md" "$HIT"; git add -A -- .waggle; "${GC[@]}" commit -q --no-edit
w "$M/both.md" "base"; ci f3; report evil-merge-edit-then-restored "$(git rev-parse HEAD)"
# Octopus merge (B4): git skips the remerge diff for three or more parents,
# so the scan must fail closed on the merge itself (expect exit 1).
fresh; w "$M/o1.md" "s"; ci o1; o1=$(git rev-parse HEAD)
fresh; w "$M/o2.md" "t"; ci o2; o2=$(git rev-parse HEAD)
fresh; w "$M/main.md" "m"; ci o3; "${GC[@]}" merge -q --no-ff --no-edit "$o1" "$o2" >/dev/null 2>&1
w "$M/evil.md" "$HIT"; git add -A -- .waggle; "${GC[@]}" commit -q --amend --no-edit
w "$M/evil.md" "redacted"; ci o4; report octopus-evil-then-removed "$(git rev-parse HEAD)"

echo "### binary and text flips"
fresh; printf 'a\000b %s\n' "$HIT" >"$M/x.dat"; ci b1; w "$M/x.dat" "clean text"; ci b2; report binary-then-text "$(git rev-parse HEAD)"
fresh; w "$M/x.dat" "$HIT"; ci t1; printf 'a\000b\n' >"$M/x.dat"; ci t2; report text-then-binary "$(git rev-parse HEAD)"
# NUL past git's 8000-byte sniff window: git diffs it as text.
fresh; { head -c 9000 /dev/zero | tr '\0' a; printf '\nx\000 %s\n' "$HIT"; } >"$M/late-nul.txt"; ci n1; report nul-after-8000-bytes "$(git rev-parse HEAD)"
fresh; { head -c 9000 /dev/zero | tr '\0' a; printf '\n%s\000x\n' "$HIT"; } >"$M/late-nul2.txt"; ci n2; report nul-after-hit "$(git rev-parse HEAD)"

git checkout -q --detach "$BASE"
