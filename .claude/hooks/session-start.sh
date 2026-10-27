#!/bin/sh
# SPDX-License-Identifier: MIT-0
# SessionStart hook (hive): prime a fresh, resumed, cleared or compacted queen
# session with the handoff and the live repo state. Stdout becomes context.
# Handoff order (WAGGLE.md section 4): the newest
# .waggle/control/handoffs/queen-<n>.md (the role template's Handoff section,
# gitignored), else the committed queen.md summary (a fresh clone or a new
# machine). The legacy queen.local.md handoff was retired at cutover (D-220).
# The repo is CLAUDE_PROJECT_DIR, so a cd into a worktree does not change which
# handoff is read.
root=${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null)} || exit 0
[ -n "$root" ] || exit 0
summary=.waggle/hive/handoff/queen.md
f="" max=-1
for h in "$root"/.waggle/control/handoffs/queen-*.md; do
  [ -f "$h" ] || continue
  n=${h##*/queen-}; n=${n%.md}
  case $n in ''|*[!0-9]*) continue;; esac
  [ "${#n}" -le 9 ] || continue
  if [ "$n" -gt "$max" ]; then max=$n f=$h; fi
done
[ -n "$f" ] || f="$root/$summary"
echo "=== Queen handoff (${f#"$root"/}) ==="
[ -f "$f" ] && cat "$f" || echo "(missing)"
[ "$f" = "$root/$summary" ] || echo "(Standing facts and habits: $summary. Read it before dispatching.)"
echo; echo "=== main (last 5) ==="; git -C "$root" log --oneline -5 main 2>/dev/null
echo; echo "=== local branches and worktrees ==="; git -C "$root" branch --list 2>/dev/null; git -C "$root" worktree list 2>/dev/null
echo; echo "=== open PRs ==="; (cd "$root" && env -u GITHUB_TOKEN -u GH_TOKEN gh pr list --limit 20 2>/dev/null) || echo "(gh unavailable)"
echo; echo "=== uncommitted hive files ==="; git -C "$root" status --short .waggle/hive .claude 2>/dev/null | head -40
exit 0
