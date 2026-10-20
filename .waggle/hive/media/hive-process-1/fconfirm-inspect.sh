#!/usr/bin/env bash
# Inspections for the 462f589 confirmation: what git does with remerge on an
# octopus merge, a crafted commit object with NUL bytes in its message and
# author, and the scan under a git that lacks --diff-merges=remerge.
# Usage: fconfirm-inspect.sh <base sha> <scan script> <octopus head>
#   octopus head: the merge probe's octopus-evil-then-removed head, whose
#   first parent is the three-parent merge (a planted commit, run both in
#   one session). Writes three dangling commit objects with hash-object.
set -u
BASE=$(git rev-parse "$1"); SCAN=$2; AFTER=$3
PD=gmail; PD=$PD.com; HIT="contact jane.doe@$PD"
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
red() { sed "s/jane\.doe@$PD/<planted>/g"; }

echo "### octopus merge under --diff-merges=remerge"
o=$(git rev-parse "$AFTER^")
echo "parents: $(( $(git rev-list --parents -n1 "$o" | wc -w) - 1 ))"
echo "-- remerge -p stdout:"
git log -1 --no-show-signature --diff-merges=remerge -p -U0 --format='commit %h' "$o" 2>"$T/err" | red
echo "-- remerge stderr:"
cat "$T/err"
echo "-- remerge --raw stdout:"
git log -1 --no-show-signature --diff-merges=remerge --raw --format='commit %h' "$o" 2>&1 | red
echo "-- first-parent diff of the same merge (--diff-merges=first-parent):"
git log -1 --no-show-signature --diff-merges=first-parent --raw --format='commit %h' "$o" 2>&1 | red
echo "-- commits with three or more parents in the range:"
git rev-list --min-parents=3 "$BASE..$AFTER" | wc -l | tr -d ' '

echo "### crafted commit objects with NUL bytes (hash-object --literally)"
tree=$(git rev-parse "$BASE^{tree}")
mk() { # name, message bytes (printf format already expanded)
  printf 'tree %s\nparent %s\nauthor ' "$tree" "$BASE" >"$T/obj"
  printf '%b' "$2" >>"$T/obj"
  printf ' <fixture@example.test> 1760000000 +0000\ncommitter fixture <fixture@example.test> 1760000000 +0000\n\n' >>"$T/obj"
  printf '%b' "$3" >>"$T/obj"
  c=$(git hash-object -t commit -w --literally --stdin <"$T/obj")
  echo "-- $1: $c (commit-tree refuses a NUL in a message; only hash-object --literally writes one)"
  echo "   NUL bytes in the stored object: $(git cat-file commit "$c" | tr -cd '\000' | wc -c | tr -d ' ')"
  echo "   log %B as seen (cat -v): $(git log -1 --format=%B "$c" | cat -v | red | tr '\n' '|')"
  echo "   log %an as seen (cat -v): $(git log -1 --format=%an "$c" | cat -v | red)"
  out=$("$SCAN" "$BASE" "$c" 2>&1); rc=$?
  echo "== $1: exit $rc"
  printf '%s\n' "$out" | grep -E 'commit\(s\) scanned|^fatal|^error' | sed 's/^/   /'
  printf '%s\n' "$out" | sed -n '/^hits/,/^result/p' | sed 's/^/   /'
}
mk nul-in-message-same-line fixture "nul message\\000 $HIT\\n"
mk nul-in-message-next-line fixture "nul message\\000\\n$HIT\\n"
mk nul-in-author-then-hit "fix\\000ture $HIT" "clean message\\n"

echo "### email fast path: one 65536-byte line with no @, scan time"
git checkout -q --detach "$BASE"; mkdir -p .waggle/hive/media/confirm-plant
{ head -c 65535 /dev/zero | tr '\0' a; echo; } >.waggle/hive/media/confirm-plant/long.txt
git add -A -- .waggle; git -c user.name=fixture -c user.email=fixture@example.test commit -q -m long
s=$(date +%s); out=$("$SCAN" "$BASE" HEAD 2>&1); rc=$?; e=$(date +%s)
echo "== long-line-65536-no-at: exit $rc, $((e - s)) s, $(printf '%s\n' "$out" | grep -c ': email') email hit(s)"
git checkout -q --detach "$BASE"; rm -rf .waggle/hive/media/confirm-plant

echo "### scan under a git without --diff-merges=remerge (shim dies like 2.31 to 2.35)"
real=$(command -v git)
mkdir -p "$T/shim"
printf '#!/bin/sh\nfor a in "$@"; do case "$a" in --diff-merges=remerge) echo "fatal: invalid value for %s: remerge" >&2; exit 128;; esac; done\nexec %s "$@"\n' "'--diff-merges'" "$real" >"$T/shim/git"
chmod +x "$T/shim/git"
echo "shim check: $(PATH="$T/shim:$PATH" git log -1 --diff-merges=remerge --format=%h "$BASE" 2>&1; echo " rc=$?")"
out=$(PATH="$T/shim:$PATH" "$SCAN" "$BASE" "$AFTER" 2>&1); rc=$?
echo "== old-git-shim: exit $rc"
printf '%s\n' "$out" | red | sed 's/^/   /'
echo "### same shim, but git pre-2.30 wording (unrecognized argument)"
printf '#!/bin/sh\nfor a in "$@"; do case "$a" in --diff-merges=remerge) echo "fatal: unrecognized argument: --diff-merges=remerge" >&2; exit 128;; esac; done\nexec %s "$@"\n' "$real" >"$T/shim/git"
out=$(PATH="$T/shim:$PATH" "$SCAN" "$BASE" "$AFTER" 2>&1); rc=$?
echo "== old-git-shim-2: exit $rc"
printf '%s\n' "$out" | red | sed 's/^/   /'
