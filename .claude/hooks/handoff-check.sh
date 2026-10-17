#!/bin/sh
# Stop hook (hive): refuse to end the queen's turn while the handoff is older
# than the last commit on main. Exit 2 feeds the message back to the model.
input=$(cat)
case "$input" in *'"stop_hook_active":true'*|*'"stop_hook_active": true'*) exit 0;; esac
root=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
f="$root/.waggle/hive/handoff/queen.md"
[ -f "$f" ] || { echo "Handoff missing: write $f before stopping." >&2; exit 2; }
last=$(git -C "$root" log -1 --format=%ct main 2>/dev/null) || exit 0
mtime=$(stat -f %m "$f" 2>/dev/null || stat -c %Y "$f" 2>/dev/null) || exit 0
if [ "$mtime" -lt "$last" ]; then
  echo "Handoff .waggle/hive/handoff/queen.md is older than the last commit on main ($(git -C "$root" log -1 --format='%h %s' main)). Refresh it (state, in-flight PRs, next work) before stopping." >&2
  exit 2
fi
exit 0
