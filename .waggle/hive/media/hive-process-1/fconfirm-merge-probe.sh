#!/usr/bin/env bash
# Hostile merge probe for scan-hive.sh at 462f589 (hive-process-1): merge
# shapes the remerge pass must cover (octopus, hand-resolved conflicts, -s
# ours, criss-cross, unrelated histories, redundant parent, message-only,
# path-only and binary-only merge content), the clean update-branch merge
# that must still exit 0, and the NUL sentinel collision.
# Each scenario is a detached commit on BASE (never a branch, never pushed).
# The planted address is built at run time; none is written to this file.
# Usage: fconfirm-merge-probe.sh <base sha> <scan script> <main sha>
#   (repo root, clean tree; main sha is the records branch base for the
#   update-branch case, normally origin/main). Keep this file outside .waggle
#   while it runs: the scenarios stage everything under .waggle.
set -u
BASE=$1
SCAN=$2
MAIN=$3
M=.waggle/hive/media/confirm-plant
GC=(git -c user.name=fixture -c user.email=fixture@example.test -c commit.gpgsign=false)
export GIT_AUTHOR_NAME=fixture GIT_AUTHOR_EMAIL=fixture@example.test
export GIT_COMMITTER_NAME=fixture GIT_COMMITTER_EMAIL=fixture@example.test
PD=gmail; PD=$PD.com; HIT="contact jane.doe@$PD"

report() { # name, sha
  local out rc
  out=$("$SCAN" "$BASE" "$2" 2>&1); rc=$?
  echo "== $1: exit $rc"
  printf '%s\n' "$out" | grep -E 'HOLD|commit\(s\) scanned|^fatal|^error' | sed 's/^/   /'
  printf '%s\n' "$out" | sed -n '/^hits/,/^result/p' | sed 's/^/   /'
}
fresh() { git reset -q; git checkout -q --detach "$BASE"; rm -rf "$M"; mkdir -p "$M"; }
ci() { git add -A -- .waggle; "${GC[@]}" commit -q --allow-empty -m "$1"; }
w() { mkdir -p "$(dirname "$1")"; printf '%s\n' "$2" >"$1"; }
mg() { "${GC[@]}" merge -q --no-ff --no-edit "$@" >/dev/null 2>&1; }
mgnc() { "${GC[@]}" merge -q --no-ff --no-commit "$@" >/dev/null 2>&1; }
cim() { git add -A -- .waggle; "${GC[@]}" commit -q --no-edit; } # commit a stopped merge
# A root commit holding one file at $M/root.md with the given content, built
# with plumbing so the working tree is never emptied.
root_commit() {
  local b t
  b=$(printf '%s\n' "$1" | git hash-object -w --stdin)
  t=$(printf '100644 blob %s\troot.md\n' "$b" | git mktree)
  t=$(printf '040000 tree %s\tconfirm-plant\n' "$t" | git mktree)
  t=$(printf '040000 tree %s\tmedia\n' "$t" | git mktree)
  t=$(printf '040000 tree %s\thive\n' "$t" | git mktree)
  t=$(printf '040000 tree %s\t.waggle\n' "$t" | git mktree)
  git commit-tree "$t" -m "unrelated root"
}

echo "### octopus merges"
fresh; w "$M/s1.md" s; ci o1; s1=$(git rev-parse HEAD)
fresh; w "$M/s2.md" t; ci o2; s2=$(git rev-parse HEAD)
fresh; w "$M/main.md" m; ci o3; mg "$s1" "$s2"
w "$M/evil.md" "$HIT"; git add -A -- .waggle; "${GC[@]}" commit -q --amend --no-edit
echo "   parents of the octopus: $(( $(git rev-list --parents -n1 HEAD | wc -w) - 1 ))"
w "$M/evil.md" redacted; ci o4; report octopus-evil-then-removed "$(git rev-parse HEAD)"
fresh; w "$M/s1.md" s; ci p1; s1=$(git rev-parse HEAD)
fresh; w "$M/s2.md" "$HIT"; ci p2; s2=$(git rev-parse HEAD)
fresh; w "$M/main.md" m; ci p3; mg "$s1" "$s2"
w "$M/s2.md" redacted; ci p4; report octopus-side-hit-then-removed "$(git rev-parse HEAD)"

echo "### conflicts resolved by hand"
fresh; w "$M/both.md" base; ci c0; b0=$(git rev-parse HEAD)
w "$M/both.md" side; ci c1; side=$(git rev-parse HEAD)
git checkout -q --detach "$b0"; w "$M/both.md" main; ci c2; mg "$side"
w "$M/both.md" "$HIT"; cim
w "$M/both.md" base; ci c3; report conflict-resolved-with-hit-then-restored "$(git rev-parse HEAD)"
fresh; w "$M/both.md" base; ci c4; b0=$(git rev-parse HEAD)
w "$M/both.md" side; ci c5; side=$(git rev-parse HEAD)
git checkout -q --detach "$b0"; w "$M/both.md" main; ci c6; mg "$side"
printf 'main\nside\n%s\n' "$HIT" >"$M/both.md"; cim
w "$M/both.md" base; ci c7; report conflict-resolved-both-sides-plus-hit "$(git rev-parse HEAD)"
fresh; w "$M/both.md" base; ci c8; b0=$(git rev-parse HEAD)
w "$M/both.md" side; ci c9; side=$(git rev-parse HEAD)
git checkout -q --detach "$b0"; w "$M/both.md" main; ci c10; mg "$side"
w "$M/both.md" resolved; cim; report conflict-resolved-clean "$(git rev-parse HEAD)"
# Directory and file conflict, resolved as a file carrying the hit.
fresh; w "$M/d" sidefile; ci d1; side=$(git rev-parse HEAD)
fresh; w "$M/d/x" mainfile; ci d2; mg "$side"
rm -rf "$M/d" "$M"/d~*; w "$M/d" "$HIT"; cim
w "$M/d" redacted; ci d3; report df-conflict-resolved-with-hit "$(git rev-parse HEAD)"
# Binary add and add conflict, resolved as text carrying the hit.
fresh; printf 'a\000b1\n' >"$M/x.dat"; ci bc1; side=$(git rev-parse HEAD)
fresh; printf 'a\000b2\n' >"$M/x.dat"; ci bc2; mg "$side"
w "$M/x.dat" "$HIT"; cim
w "$M/x.dat" redacted; ci bc3; report binary-conflict-text-resolution "$(git rev-parse HEAD)"

echo "### -s ours"
fresh; w "$M/side.md" "$HIT"; ci u1; side=$(git rev-parse HEAD)
fresh; w "$M/main.md" m; ci u2; "${GC[@]}" merge -q -s ours --no-edit "$side" >/dev/null 2>&1
report ours-merge-side-hit "$(git rev-parse HEAD)"
fresh; w "$M/side.md" s; ci v1; side=$(git rev-parse HEAD)
fresh; w "$M/main.md" m; ci v2; "${GC[@]}" merge -q -s ours --no-commit "$side" >/dev/null 2>&1
w "$M/evil.md" "$HIT"; cim
w "$M/evil.md" redacted; ci v3; report ours-merge-evil-then-removed "$(git rev-parse HEAD)"

echo "### criss-cross"
fresh; w "$M/a.md" a; ci x1; A=$(git rev-parse HEAD)
fresh; w "$M/b.md" b; ci x2; B=$(git rev-parse HEAD)
git checkout -q --detach "$A"; mg "$B"; X=$(git rev-parse HEAD)
git checkout -q --detach "$B"; mg "$A"; Y=$(git rev-parse HEAD)
git checkout -q --detach "$X"; mgnc "$Y"
w "$M/evil.md" "$HIT"; cim
echo "   merge bases of X and Y: $(git merge-base --all "$X" "$Y" | wc -l | tr -d ' ')"
w "$M/evil.md" redacted; ci x3; report crisscross-evil-then-removed "$(git rev-parse HEAD)"

echo "### unrelated histories"
fresh; R=$(root_commit "$HIT")
w "$M/main.md" m; ci r1; mg --allow-unrelated-histories "$R"; echo "   merge exit $?"
w "$M/root.md" redacted; ci r2; report unrelated-root-hit-then-removed "$(git rev-parse HEAD)"
fresh; R=$(root_commit clean)
w "$M/main.md" m; ci r3; mgnc --allow-unrelated-histories "$R"
w "$M/evil.md" "$HIT"; cim
w "$M/evil.md" redacted; ci r4; report unrelated-evil-then-removed "$(git rev-parse HEAD)"

echo "### merge message, merge-only paths, redundant parent"
fresh; w "$M/side.md" s; ci mm1; side=$(git rev-parse HEAD)
fresh; w "$M/main.md" m; ci mm2; "${GC[@]}" merge -q --no-ff "$side" -m "$HIT" >/dev/null 2>&1
report merge-message-hit "$(git rev-parse HEAD)"
fresh; w "$M/side.md" s; ci lf1; side=$(git rev-parse HEAD)
fresh; w "$M/main.md" m; ci lf2; mgnc "$side"
w "$M/notes.local" x; cim
git rm -q "$M/notes.local"; ci lf3; report merge-adds-local-file-then-removed "$(git rev-parse HEAD)"
fresh; w "$M/side.md" s; ci bn1; side=$(git rev-parse HEAD)
fresh; w "$M/main.md" m; ci bn2; mgnc "$side"
printf 'a\000b\n' >"$M/x.dat"; cim
git rm -q "$M/x.dat"; ci bn3; report merge-adds-binary-then-removed "$(git rev-parse HEAD)"
fresh; w "$M/main.md" m; ci rp1; p1=$(git rev-parse HEAD)
w "$M/evil.md" "$HIT"; git add -A -- .waggle; tree=$(git write-tree)
mc=$(git commit-tree "$tree" -p "$p1" -p "$BASE" -m "redundant merge")
git checkout -q --detach "$mc"; w "$M/evil.md" redacted; ci rp2
report redundant-parent-evil-then-removed "$(git rev-parse HEAD)"

echo "### clean update-branch merge of a records batch (expect exit 0)"
git checkout -q --detach "$MAIN"; rm -rf "$M"; mkdir -p "$M"
w .waggle/hive/reviews/fconfirm-probe.md "new review"; w "$M/a.md" "new media"; ci "records batch"
mg "$BASE"; echo "   merge exit $?"
report records-batch-update-merge "$(git rev-parse HEAD)"

echo "### NUL bytes"
# A NUL past the 8000-byte sniff window, then a line reading NUL + binary:
# the sentinel collision must fail closed.
fresh; { head -c 9000 /dev/zero | tr '\0' a; printf '\n\000binary\n'; } >"$M/sentinel.txt"; ci sn1
report nul-binary-sentinel "$(git rev-parse HEAD)"
# A NUL in a commit message or author needs hash-object --literally
# (commit-tree refuses it); that case is in fconfirm-inspect.sh.

git reset -q; git checkout -q --detach "$BASE"; rm -rf "$M"
