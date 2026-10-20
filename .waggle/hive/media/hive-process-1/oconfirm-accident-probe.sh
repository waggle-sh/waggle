#!/usr/bin/env bash
# Confirmation probe for scan-hive.sh at 2a99fb5 (hive-process-1): ordinary
# queen and scout workflows that could leak by accident, with no crafted
# objects. Update-branch merges with a DECISIONS.md conflict, reverts of a
# merge, cherry-picks, squash merges, renames, non-ASCII paths, long lines,
# CRLF and UTF-16 files, and amends of a merge message.
# Each scenario is a detached commit on BASE (never a branch, never pushed).
# The planted address is built at run time; none is written to this file.
# Usage: oconfirm-accident-probe.sh <base sha> <scan script>
#   (repo root, clean tree). Keep this file outside .waggle while it runs.
set -u
BASE=$(git rev-parse "$1"); SCAN=$2
M=.waggle/hive/media/confirm-plant
GC=(git -c user.name=fixture -c user.email=fixture@example.test -c commit.gpgsign=false)
export GIT_AUTHOR_NAME=fixture GIT_AUTHOR_EMAIL=fixture@example.test
export GIT_COMMITTER_NAME=fixture GIT_COMMITTER_EMAIL=fixture@example.test
PD=gmail; PD=$PD.com; HIT="contact jane.doe@$PD"
SB="$BASE"

report() { # name, sha
  local out rc
  out=$("$SCAN" "$SB" "$2" 2>&1); rc=$?
  echo "== $1: exit $rc"
  printf '%s\n' "$out" | grep -E 'HOLD|commit\(s\) scanned|^fatal|^error' | sed 's/^/   /'
  printf '%s\n' "$out" | sed -n '/^hits/,/^result/p' | sed 's/^/   /'
}
fresh() { git reset -q; git checkout -q -f --detach "$BASE"; rm -rf "$M"; mkdir -p "$M"; }
ci() { git add -A -- .waggle; "${GC[@]}" commit -q --allow-empty -m "$1"; }
w() { mkdir -p "$(dirname "$1")"; printf '%s\n' "$2" >"$1"; }
dec() { # id, title
  printf '\n## %s: %s\n\n- **Date:** 2026-10-19\n- **Kind:** policy\n- **Tickets:** none\n- **Supersedes:** none\n\n**Question:** Probe?\n\n**Options considered:**\n1. Yes\n2. No\n\n**Hive recommendation:** Yes\n\n**Keeper choice:** Yes.\n\n**Rationale:** Probe.\n' "$1" "$2" >>.waggle/hive/DECISIONS.md
}

echo "### update-branch merge with a DECISIONS.md append conflict, resolved by keeping both (expect 0)"
fresh; dec D-187 "main side entry"; ci main-dec; main2=$(git rev-parse HEAD)
fresh; dec D-188 "batch entry"; w .waggle/hive/reviews/oconfirm-probe.md "new review"; ci batch
"${GC[@]}" merge -q --no-ff --no-commit "$main2" >/dev/null 2>&1
# Keep main's entry then the batch's, as a hand resolution would.
git show "$main2:.waggle/hive/DECISIONS.md" >.waggle/hive/DECISIONS.md; dec D-188 "batch entry"
git add -A -- .waggle; "${GC[@]}" commit -q --no-edit
SB=$main2; report decisions-conflict-resolved-clean "$(git rev-parse HEAD)"
echo "### same, with a hit slipped into the resolution and removed later (expect 1)"
fresh; dec D-188 "batch entry"; ci batch
"${GC[@]}" merge -q --no-ff --no-commit "$main2" >/dev/null 2>&1
git show "$main2:.waggle/hive/DECISIONS.md" >.waggle/hive/DECISIONS.md; dec D-188 "batch entry"; echo "$HIT" >>.waggle/hive/DECISIONS.md
git add -A -- .waggle; "${GC[@]}" commit -q --no-edit
git show "$main2:.waggle/hive/DECISIONS.md" >.waggle/hive/DECISIONS.md; dec D-188 "batch entry"; ci fixup
report decisions-conflict-resolved-with-hit-then-fixed "$(git rev-parse HEAD)"
SB=$BASE

echo "### reverts, cherry-picks, squash (expect 1)"
fresh; w "$M/side.md" "$HIT"; ci s1; side=$(git rev-parse HEAD)
fresh; w "$M/main.md" "m"; ci m1; "${GC[@]}" merge -q --no-ff --no-edit "$side" >/dev/null 2>&1
"${GC[@]}" revert -m 1 --no-edit HEAD >/dev/null 2>&1; report merge-reverted "$(git rev-parse HEAD)"
fresh; w "$M/m.md" "m"; ci c0; "${GC[@]}" cherry-pick "$side" >/dev/null 2>&1; w "$M/side.md" "redacted"; ci c1
report cherry-pick-then-removed "$(git rev-parse HEAD)"
fresh; "${GC[@]}" merge -q --squash "$side" >/dev/null 2>&1; ci squash; w "$M/side.md" "redacted"; ci sq2
report squash-then-removed "$(git rev-parse HEAD)"
echo "### merge message amended to carry a hit, then amended clean (expect 0: the hit is gone from history)"
fresh; w "$M/o.md" "o"; ci o1; o=$(git rev-parse HEAD)
fresh; w "$M/p.md" "p"; ci p1; "${GC[@]}" merge -q --no-ff -m "merge $HIT" "$o" >/dev/null 2>&1
"${GC[@]}" commit -q --amend -m "merge clean"; report merge-message-amended-clean "$(git rev-parse HEAD)"

echo "### renames (expect 0 for a rename of a file the branch added, 1 when an intermediate name is a hit)"
fresh; w "$M/a.md" "clean"; ci r0; git mv "$M/a.md" "$M/b.md"; ci r1; report rename-own-file "$(git rev-parse HEAD)"
fresh; w "$M/a.md" "clean"; ci r0; git mv "$M/a.md" "$M/jane.doe@$PD-notes.md"; ci r1; git mv "$M/jane.doe@$PD-notes.md" "$M/c.md"; ci r2
report rename-through-hit-name "$(git rev-parse HEAD)"

echo "### non-ASCII paths"
fresh; w "$M/caf$(printf '\303\251').md" "clean text"; ci u1; report non-ascii-path-clean "$(git rev-parse HEAD)"
fresh; w "$M/caf$(printf '\303\251').md" "$HIT"; ci u1; w "$M/caf$(printf '\303\251').md" "redacted"; ci u2
report non-ascii-path-hit-then-removed "$(git rev-parse HEAD)"
fresh; w "$M/jane.doe@$PD/caf$(printf '\303\251').md" "clean"; ci u1; report non-ascii-path-hit-in-name "$(git rev-parse HEAD)"
fresh; w "$M/tab$(printf '\t')name.md" "$HIT"; ci u1; report tab-in-path-hit "$(git rev-parse HEAD)"

echo "### long lines, CRLF, UTF-16"
fresh; { head -c 70000 /dev/zero | tr '\0' a; printf ' %s\n' "$HIT"; } >"$M/long.txt"; ci l1; report long-line-with-hit "$(git rev-parse HEAD)"
fresh; { head -c 60000 /dev/zero | tr '\0' a; printf ' %s\n' "$HIT"; } >"$M/long.txt"; ci l1; report 60k-line-with-hit "$(git rev-parse HEAD)"
fresh; printf 'line one\r\n%s\r\n' "$HIT" >"$M/crlf.txt"; ci cr; report crlf-hit "$(git rev-parse HEAD)"
fresh; printf '%s\n' "$HIT" | iconv -f UTF-8 -t UTF-16 >"$M/u16.txt"; ci u16; report utf16-file "$(git rev-parse HEAD)"
fresh; printf 'caf\303\251 %s\n' "$HIT" >"$M/u8.txt"; ci u8; report utf8-line-hit "$(git rev-parse HEAD)"
fresh
