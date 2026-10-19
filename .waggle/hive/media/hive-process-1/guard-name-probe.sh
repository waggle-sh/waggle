#!/bin/sh
# Name rule probes for scan-hive.sh: SCAN_HIVE_NAMES, the gitignored
# scan-names.local, the home directory component, and CI skipping.
# Usage: name-probe.sh <base sha>   (repo root, clean tree)
BASE=$1; D=.waggle/hive/media/guard-plant; SCAN=.waggle/control/scripts/scan-hive.sh
hits() { sed -n '/^hits/,/^result/p' | sed 's/^/   /'; }
git checkout -q --detach "$BASE"; mkdir -p "$D"
printf 'reviewed by Zorblax.\nand by zorblaxian (not whole word)\nhome word: %s here\n' "${HOME##*/}" >"$D/n.md"
git add -- "$D"; git -c user.name=fixture -c user.email=fixture@example.test commit -q -m names
echo "== no extra names (home component only):"; $SCAN "$BASE" HEAD 2>&1 | hits
echo "== SCAN_HIVE_NAMES=Zorblax:"; SCAN_HIVE_NAMES=Zorblax $SCAN "$BASE" HEAD 2>&1 | hits
echo "== scan-names.local with Zorblax:"; echo Zorblax >.waggle/control/scan-names.local
git check-ignore -q .waggle/control/scan-names.local && echo "   (scan-names.local is ignored)"
$SCAN "$BASE" HEAD 2>&1 | hits; rm -f .waggle/control/scan-names.local
echo "== CI=1 (names and hostname skipped):"; CI=1 SCAN_HIVE_NAMES=Zorblax $SCAN "$BASE" HEAD 2>&1 | hits
git checkout -q --detach "$BASE"
sed 's/^\*\*Rationale:\*\* Saves one wait per fix round/**Rationale:** Edited SECRETVALUE_ghp_/' .waggle/hive/DECISIONS.md >"$D.tmp" && mv "$D.tmp" .waggle/hive/DECISIONS.md
git add -- .waggle/hive/DECISIONS.md; git -c user.name=fixture -c user.email=fixture@example.test commit -q -m dedit
echo "== decisions edit, full gate output:"; $SCAN "$BASE" HEAD 2>&1 | sed -n '/^decisions gate/,$p' | sed 's/^/   /'
git checkout -q --detach "$BASE"
