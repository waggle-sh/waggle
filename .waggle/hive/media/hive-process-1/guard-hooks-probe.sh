#!/bin/sh
# Guard probe for .claude/hooks/handoff-check.sh and session-start.sh (hive-process-1).
# Usage: hooks-probe.sh <worktree> <scratch repo with a main branch>
W=$1; R=$2
H=$W/.claude/hooks/handoff-check.sh; SS=$W/.claude/hooks/session-start.sh
f=.waggle/hive/handoff/queen.local.md
cd "$R" || exit 9
run() {
  for shl in sh bash; do
    out=$(printf '%s' "$2" | $shl "$H" 2>&1); rc=$?
    echo "[$1][$shl] rc=$rc ${out:-<no output>}"
  done
}
rm -f "$f" .waggle/hive/handoff/queen.md
run missing '{}'
run active-compact '{"stop_hook_active":true}'
run active-spaced '{"stop_hook_active": true}'
seq 40 >"$f"; run 40-lines '{}'
seq 41 >"$f"; run 41-lines '{}'
seq 40 >"$f"; printf 'x' >>"$f"; run 41-lines-no-final-newline '{}'
: >"$f"; run empty-file '{}'
seq 10 >"$f"; touch -t 202001010000 "$f"; run older-than-main '{}'
touch "$f"; run newer-than-main '{}'
echo "[committed queen.md only, local missing]"; rm -f "$f"; echo '# summary' >.waggle/hive/handoff/queen.md; run committed-only '{}'
cd /
out=$(echo '{}' | sh "$H" 2>&1); echo "[outside-repo][sh] rc=$? ${out:-<no output>}"
cd "$R"
echo "=== session-start, local present ==="; seq 3 >"$f"; sh "$SS" | head -5
echo "=== session-start, local missing, summary present ==="; rm -f "$f"; sh "$SS" | head -3
echo "=== session-start, both missing ==="; rm -f .waggle/hive/handoff/queen.md; sh "$SS" | head -3
echo "=== session-start, bash ==="; bash "$SS" | head -2
cd /; out=$(sh "$SS" 2>&1); echo "=== session-start outside repo rc=$? ${out:-<no output>}"
