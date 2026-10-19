#!/bin/sh
# SessionStart hook (hive): prime a fresh, resumed, cleared or compacted queen
# session with the handoff and the live repo state. Stdout becomes context.
# The working handoff is the gitignored queen.local.md (D-182); the committed
# queen.md summary is printed only when the local file is missing (a fresh
# clone or a new machine).
root=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
f="$root/.waggle/hive/handoff/queen.local.md"
[ -f "$f" ] || f="$root/.waggle/hive/handoff/queen.md"
echo "=== Queen handoff (${f#"$root"/}) ==="
[ -f "$f" ] && cat "$f" || echo "(missing)"
echo; echo "=== main (last 5) ==="; git -C "$root" log --oneline -5 main 2>/dev/null
echo; echo "=== local branches and worktrees ==="; git -C "$root" branch --list 2>/dev/null; git -C "$root" worktree list 2>/dev/null
echo; echo "=== open PRs ==="; env -u GITHUB_TOKEN -u GH_TOKEN gh pr list --limit 20 2>/dev/null || echo "(gh unavailable)"
echo; echo "=== uncommitted hive files ==="; git -C "$root" status --short .waggle/hive .claude 2>/dev/null | head -40
exit 0
