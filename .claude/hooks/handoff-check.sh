#!/bin/sh
# Stop hook (hive): refuse to end the queen's turn while the local handoff is
# missing, longer than 40 lines, or older than the last commit on main (D-182).
# Exit 2 feeds the message back to the model. The committed queen.md summary
# is not checked; the queen refreshes it at phase merges and in records batches.
#
# Starter for .waggle/hive/handoff/queen.local.md (gitignored, facts only, at
# most 40 lines; rewrite the facts rather than append, and `touch` is not a
# refresh):
#
#   # Queen handoff (local)
#   Role and framing: .waggle/control/roles/queen.md, WAGGLE.md section 4.
#   Standing facts and habits: the committed .waggle/hive/handoff/queen.md.
#   ## Live state (main at <sha>, <date>)
#   - merged since the summary: <PR, sha, one line each>
#   - open PRs and branches: <number, branch, state>
#   - uncommitted hive records: <paths, or none>
#   ## Running agents
#   - <role, model, worktree or branch, what it writes and where>
#   ## Next work
#   1. <step, owner, gate>
input=$(cat)
case "$input" in *'"stop_hook_active":true'*|*'"stop_hook_active": true'*) exit 0;; esac
root=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
rel=.waggle/hive/handoff/queen.local.md
f="$root/$rel"
[ -f "$f" ] || { echo "Handoff missing: write $rel (facts only, at most 40 lines; starter in .claude/hooks/handoff-check.sh) before stopping." >&2; exit 2; }
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
