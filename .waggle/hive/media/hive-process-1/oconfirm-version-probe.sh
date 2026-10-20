#!/usr/bin/env bash
# Confirmation probe for scan-hive.sh at 2a99fb5 (hive-process-1): the git
# version gate against real `git version` strings (Apple Git suffix,
# .windows.N, rc builds, vendor suffixes) and the remerge-skipped second guard
# on its own. Part 1 feeds each string to the script's own parse line (read
# from the script, not copied). Part 2 runs the whole script under a PATH shim
# that answers `git version` with a few of those strings.
# Usage: oconfirm-version-probe.sh <base sha> <scan script>   (repo root)
set -u
BASE=$(git rev-parse "$1"); SCAN=$2
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT

echo "### part 1: the parse line on version strings"
line=$(grep -n '^gv=' "$SCAN" | head -1); echo "   script line ${line%%:*}"
gate=$(grep -n '^\[ "\${gv:-0}" -ge 2036 \]' "$SCAN" | head -1 | cut -d: -f2-)
cmd=${line#*:}
cmd=${cmd#gv=\$(git version 2>/dev/null | }; cmd=${cmd% || true)}
while IFS= read -r v; do
  gv=$(printf '%s\n' "$v" | eval "$cmd" || true)
  if [ "${gv:-0}" -ge 2036 ] 2>/dev/null; then r="runs"; else r="exit 3"; fi
  printf '   %-42s parsed %-6s %s\n' "[$v]" "${gv:-empty}" "$r"
done <<'EOF'
git version 2.54.0 (Apple Git-157)
git version 2.39.5 (Apple Git-154)
git version 2.35.1 (Apple Git-136)
git version 2.45.1.windows.1
git version 2.36.0.windows.1
git version 2.35.1.windows.2
git version 2.36.0.rc1
git version 2.36.0.rc0.windows.1
git version 2.35.0.rc2
git version 2.34.1
git version 2.36.0.vfs.0.0
git version 2.100.0
git version 3.0.0.rc0
garbage

EOF
echo "   gate line: $gate"

echo "### part 2: whole script under a version shim (expect exit 0 at 2.36 and above, 3 below)"
real=$(command -v git); mkdir -p "$T/shim"
for v in "2.39.5 (Apple Git-154)" "2.35.1 (Apple Git-136)" "2.36.0.rc1" "2.35.0.rc2" "2.45.1.windows.1" "2.34.1"; do
  printf '#!/bin/sh\ncase "$1" in version|--version) echo "git version %s"; exit 0;; esac\nexec %s "$@"\n' "$v" "$real" >"$T/shim/git"
  chmod +x "$T/shim/git"
  out=$(PATH="$T/shim:$PATH" "$SCAN" "$BASE" "$BASE" 2>&1); rc=$?
  printf '   == shim git %s: exit %s\n' "$v" "$rc"
  printf '%s\n' "$out" | grep -E '^scan-hive: needs|^result' | sed 's/^/      /'
done
