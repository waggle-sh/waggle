#!/bin/sh
# Scans each commit of the branch against its own parent, so content added in
# one commit and removed in a later one is seen. Usage: percommit.sh base head
for c in $(git rev-list --reverse --no-merges "$1..$2"); do
  out=$(.waggle/control/scripts/scan-hive.sh "$c^" "$c" 2>&1); rc=$?
  echo "== $(git rev-parse --short "$c"): exit $rc"
  printf '%s\n' "$out" | sed -n '/^hits/,/^result/p' | sed 's/^/   /'
done
