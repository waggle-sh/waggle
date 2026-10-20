#!/usr/bin/env bash
# Fix-round probes for scan-hive.sh (hive-process-1): N10 records set, N9
# local files, N3 token shapes (positive and negative), N2 long lines, N5
# warning, N6 space in a path, N12 pinned prefixes, and B1 for a local file
# and a token. Each scenario is a detached commit on BASE, never a branch,
# never pushed. Planted token values are built at run time from repeated
# letters; none is a real credential.
# Usage: fix-new-probe.sh <base sha>   (repo root, clean tree)
set -u
BASE=$1
SCAN=.waggle/control/scripts/scan-hive.sh
M=.waggle/hive/media/fix-plant
GC=(git -c user.name=fixture -c user.email=fixture@example.test -c commit.gpgsign=false)
rep() { printf "$1%.0s" $(seq "$2"); }

report() { # name, sha, [base]
  local out rc
  out=$("$SCAN" "${3:-$BASE}" "$2" 2>&1); rc=$?
  echo "== $1: exit $rc"
  printf '%s\n' "$out" | grep -E '^warning|HOLD' | sed 's/^/   /'
  printf '%s\n' "$out" | sed -n '/^hits/,/^result/p' | sed 's/^/   /'
}
scen() { # name, then commands run in the tree before commit
  local name=$1; shift
  git checkout -q --detach "$BASE"; mkdir -p "$M"
  "$@"
  git add -A -- .waggle 2>/dev/null
  "${GC[@]}" commit -q --allow-empty -m "plant $name"
  report "$name" "$(git rev-parse HEAD)"
}
w() { printf '%s\n' "$2" >"$1"; }

echo "### N10: records set"
scen add-review w .waggle/hive/reviews/fix-probe.md "new review"
scen add-media w "$M/a.md" "new media"
scen edit-handoff-summary sh -c 'echo "probe line" >>.waggle/hive/handoff/queen.md'
scen append-decisions sh -c 'printf "\n## D-187: probe entry\n\n- **Date:** 2026-10-19\n- **Kind:** policy\n- **Tickets:** none\n- **Supersedes:** none\n\n**Question:** Probe?\n\n**Options considered:**\n1. Yes\n2. No\n\n**Hive recommendation:** Yes\n\n**Keeper choice:** Yes.\n\n**Rationale:** Probe.\n" >>.waggle/hive/DECISIONS.md'
scen modify-task-brief sh -c 'echo "probe line" >>.waggle/hive/tasks/T-002.md'
scen add-task-brief w .waggle/hive/tasks/T-999.md "new brief"
scen modify-existing-review sh -c 'echo "probe line" >>.waggle/hive/reviews/hive-records-4.md'
scen delete-existing-review git rm -q .waggle/hive/reviews/hive-records-4.md
scen rename-existing-media git mv .waggle/hive/media/hive-process-1/scan-branch.txt .waggle/hive/media/hive-process-1/scan-branch2.txt
scen add-roadmap w .waggle/hive/ROADMAP.md "steering"
scen add-symlink-media ln -s ../../DECISIONS.md "$M/link.md"
scen add-gitattributes-media w "$M/.gitattributes" "* -diff"
scen delete-handoff-summary git rm -q .waggle/hive/handoff/queen.md

echo "### N9: local files"
scen force-add-handoff-local sh -c 'echo "facts" >.waggle/hive/handoff/queen.local.md && git add -f .waggle/hive/handoff/queen.local.md'
scen add-local-dot-ext w "$M/notes.local.md" "x"
scen add-local-suffix w "$M/names.local" "x"
scen not-local-word w "$M/localhost-notes.md" "x"
# Forced add, then removed in the next commit (only the per-commit pass sees it).
git checkout -q --detach "$BASE"; mkdir -p "$M"; echo facts >.waggle/hive/handoff/queen.local.md
git add -f .waggle/hive/handoff/queen.local.md; "${GC[@]}" commit -q -m l1
git rm -q --cached .waggle/hive/handoff/queen.local.md; rm -f .waggle/hive/handoff/queen.local.md; w "$M/a.md" x; git add -A -- .waggle
"${GC[@]}" commit -q -m l2; report local-added-then-removed "$(git rev-parse HEAD)"

echo "### N3: token shapes, positive"
scen jwt w "$M/a.md" "key eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ$(rep d 40).$(rep e 30)"
scen jwt-in-quotes w "$M/a.md" "anon=\"eyJ$(rep A 20).eyJ$(rep B 30).$(rep c 20)\""
scen aws w "$M/a.md" "AKIA$(rep Q 8)$(rep 7 8)"
scen slack-bot w "$M/a.md" "xoxb-$(rep 1 12)-$(rep 2 12)-$(rep Z 24)"
scen slack-user w "$M/a.md" "token xoxp-$(rep 3 12)"
scen supabase-pat w "$M/a.md" "sbp_$(rep a1 20)"
scen resend w "$M/a.md" "RESEND_API_KEY=re_Ab$(rep 3 6)_$(rep Xy9 8)"
echo "### N3: token shapes, negative (expect exit 0)"
scen neg-words w "$M/a.md" $'eyJ is the base64 of {"\nAKIA alone, AKIAPHONE, AKIA'"$(rep Q 17)"$'\nxoxb without dash, xox-thing\nsbp_short\nre_run_the_whole_test_suite_again_now, are_you_sure_about_this_long_name\nre_compile_pattern_for_matching_all_lines\nheader.payload.signature, a.b.c'
scen neg-jwt-short w "$M/a.md" "eyJx.y.z"
scen neg-aws-in-word w "$M/a.md" "XAKIA$(rep Q 16)"

echo "### N2: long lines"
for n in 65536 65537 2000000; do
  git checkout -q --detach "$BASE"; mkdir -p "$M"
  head -c "$n" /dev/zero | tr '\0' a >"$M/long.txt"; echo >>"$M/long.txt"
  git add -- "$M"; "${GC[@]}" commit -q -m long
  s=$(date +%s); report "line of $n bytes" "$(git rev-parse HEAD)"; e=$(date +%s); echo "   ($((e - s)) s including the go run gate)"
done

echo "### N5: names warning"
scen warn-default w "$M/a.md" "x"
git checkout -q --detach "$BASE"; mkdir -p "$M"; w "$M/a.md" x; git add -A -- .waggle; "${GC[@]}" commit -q -m ci
out=$(CI=1 "$SCAN" "$BASE" HEAD 2>&1); echo "== CI=1: exit $?, warning lines: $(printf '%s\n' "$out" | grep -c '^warning')"
echo Zorblax >.waggle/control/scan-names.local
out=$("$SCAN" "$BASE" HEAD 2>&1); echo "== with scan-names.local: exit $?, warning lines: $(printf '%s\n' "$out" | grep -c '^warning')"
rm -f .waggle/control/scan-names.local

echo "### N6 and N12: labels with a space in the path and hostile diff config"
scen space-in-path w "$M/a b.md" "x $(printf '\342\200\224') y"
sha=$(git rev-parse HEAD)
out=$(GIT_CONFIG_COUNT=4 GIT_CONFIG_KEY_0=diff.noprefix GIT_CONFIG_VALUE_0=true \
  GIT_CONFIG_KEY_1=diff.mnemonicPrefix GIT_CONFIG_VALUE_1=true \
  GIT_CONFIG_KEY_2=log.showSignature GIT_CONFIG_VALUE_2=true \
  GIT_CONFIG_KEY_3=color.ui GIT_CONFIG_VALUE_3=always "$SCAN" "$BASE" "$sha" 2>&1); rc=$?
echo "== same commit, diff.noprefix, mnemonicPrefix, showSignature, color.ui=always: exit $rc"
printf '%s\n' "$out" | sed -n '/^hits/,/^result/p' | sed 's/^/   /'

echo "### B1: token added then removed"
git checkout -q --detach "$BASE"; mkdir -p "$M"; w "$M/a.md" "ghp_$(rep A 36)"; git add -A -- .waggle; "${GC[@]}" commit -q -m t1
w "$M/a.md" "redacted"; git add -A -- .waggle; "${GC[@]}" commit -q -m t2; report token-added-then-removed "$(git rev-parse HEAD)"

git checkout -q --detach "$BASE"
