#!/usr/bin/env bash
# Fix round 3 probe for scan-hive.sh (hive-process-1): C5, crafted commit
# objects with a NUL byte in the message or author (must exit 1, nul-commit),
# and C6, the git version check under PATH shims that report an older or
# newer git (below 2.36: exit 3 with the version message before any scan).
# Usage: fix3-probe.sh <base sha> <scan script>   (repo root)
# Writes three dangling commit objects with hash-object --literally; never a
# branch, never pushed. The planted address is built at run time.
set -u
BASE=$(git rev-parse "$1"); SCAN=$2
PD=gmail; PD=$PD.com; HIT="contact jane.doe@$PD"
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
show() { printf '%s\n' "$1" | grep -E 'commit\(s\) scanned|^fatal|^error|^scan-hive: needs' | sed 's/^/   /'
         printf '%s\n' "$1" | sed -n '/^hits/,/^result/p' | sed 's/^/   /'; }

echo "### C5: commit objects with a NUL byte (expect exit 1, nul-commit)"
tree=$(git rev-parse "$BASE^{tree}")
mk() { # name, author name bytes, message bytes
  printf 'tree %s\nparent %s\nauthor ' "$tree" "$BASE" >"$T/obj"
  printf '%b' "$2" >>"$T/obj"
  printf ' <fixture@example.test> 1760000000 +0000\ncommitter fixture <fixture@example.test> 1760000000 +0000\n\n' >>"$T/obj"
  printf '%b' "$3" >>"$T/obj"
  c=$(git hash-object -t commit -w --literally --stdin <"$T/obj")
  out=$("$SCAN" "$BASE" "$c" 2>&1); rc=$?
  echo "== $1: exit $rc"; show "$out"
}
mk nul-in-message-same-line fixture "nul message\\000 $HIT\\n"
mk nul-in-message-next-line fixture "nul message\\000\\n$HIT\\n"
mk nul-in-author-then-hit "fix\\000ture $HIT" "clean message\\n"

echo "### C6: git version check (PATH shim answering git version)"
real=$(command -v git)
mkdir -p "$T/shim"
for v in "2.35.1" "2.30.0" "2.9.5" "1.8.3.1" "2.36.0" "2.36.0.windows.1" "3.0.0"; do
  printf '#!/bin/sh\ncase "$1" in version|--version) echo "git version %s"; exit 0;; esac\nexec %s "$@"\n' "$v" "$real" >"$T/shim/git"
  chmod +x "$T/shim/git"
  out=$(PATH="$T/shim:$PATH" "$SCAN" "$BASE" "$BASE" 2>&1); rc=$?
  echo "== shim git $v: exit $rc"; show "$out"
done
echo "### C6: git version itself fails"
printf '#!/bin/sh\ncase "$1" in version|--version) exit 1;; esac\nexec %s "$@"\n' "$real" >"$T/shim/git"
out=$(PATH="$T/shim:$PATH" "$SCAN" "$BASE" "$BASE" 2>&1); rc=$?
echo "== shim git version fails: exit $rc"; show "$out"
