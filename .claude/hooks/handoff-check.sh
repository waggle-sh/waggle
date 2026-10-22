#!/bin/sh
# Stop hook (hive), transitional until the T-023 phase F cutover card. The
# queen's working handoff follows the role template's Handoff section
# (.waggle/control/handoffs/queen-<n>.md, written when the context is large);
# no hook checks those files or the committed queen.md summary (WAGGLE.md
# section 4). While the legacy gitignored queen.local.md still exists (D-182),
# this hook keeps its old check: refuse to end the queen's turn while it is
# longer than 40 lines or older than the last commit on main. At cutover the
# queen writes the next unused queen-<n>.md from it and deletes it, and this hook passes from
# then on; a later control PR removes the hook. Exit 2 feeds the message back
# to the model. The repo is CLAUDE_PROJECT_DIR, so a cd into a worktree does
# not change which file is checked.
input=$(cat)
case "$input" in *'"stop_hook_active":true'*|*'"stop_hook_active": true'*) exit 0;; esac
root=${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null)} || exit 0
[ -n "$root" ] || exit 0
rel=.waggle/hive/handoff/queen.local.md
f="$root/$rel"
[ -f "$f" ] || exit 0
lines=$(wc -l <"$f" | tr -d ' ')
if [ "$lines" -gt 40 ]; then
  echo "Handoff $rel is $lines lines; the cap is 40 (D-182). Cut it to facts only (live state, running agents, next work) before stopping." >&2
  exit 2
fi
last=$(git -C "$root" log -1 --format=%ct main 2>/dev/null) || exit 0
mtime=$(stat -c %Y "$f" 2>/dev/null || stat -f %m "$f" 2>/dev/null) || exit 0  # GNU first: GNU stat -f means file system
if [ "$mtime" -lt "$last" ]; then
  echo "Handoff $rel is older than the last commit on main ($(git -C "$root" log -1 --format='%h %s' main)). Rewrite its facts (live state, running agents, next work) before stopping." >&2
  exit 2
fi
exit 0
