#!/usr/bin/env bash
# Guard probe for .waggle/control/scripts/scan-hive.sh (hive-process-1).
# Each scenario is a detached commit (never a branch, never pushed) on top of
# BASE; the scan runs BASE..commit and the probe prints its exit code and hit
# lines. Private values (hostname, git user.name) are read at run time and
# never written to this file or its output. The run used literal planted
# values; this copy builds them at run time so it carries no address.
# Usage: plant-probe.sh <base sha>   (run from the repo root, clean tree)
set -u
BASE=$1
SCAN=.waggle/control/scripts/scan-hive.sh
D=.waggle/hive/media/guard-plant
GC=(git -c user.name=fixture -c user.email=fixture@example.test -c commit.gpgsign=false)
HOSTV=$(hostname -s)
NAMEV=$(git config --global user.name | awk '{print $1}')
EM=$(printf '\342\200\224'); EN=$(printf '\342\200\223')
PD=gmail; PD=$PD.com; U=Users; H=home; VF=var/folders; PT=private/tmp; PM=proton; PM2=mail

scen() { # name, then commands run in the tree before commit
  local name=$1; shift
  git checkout -q --detach "$BASE"
  mkdir -p "$D"
  "$@"
  git add -A -- .waggle .claude .gitignore 2>/dev/null
  "${GC[@]}" commit -q --allow-empty -m "${MSG:-plant $name}"
  local sha; sha=$(git rev-parse HEAD)
  out=$("$SCAN" "$BASE" "$sha" 2>&1); rc=$?
  echo "== $name: exit $rc"
  printf '%s\n' "$out" | sed -n '/^hits/,/^result/p' | sed 's/^/   /'
  MSG=
}
w() { printf '%s\n' "$2" >"$D/$1"; }

scen baseline-clean w a.md "plain text, jane@example.test, git@github.com:waggle-sh/waggle.git, /home/keeper/x"
scen email w a.md "contact jane.doe@${PD} today"
scen email-upper w a.md "JANE.DOE@$(printf '%s' "$PD" | tr a-z A-Z)"
scen email-md-escape w a.md "jane\\.doe@${PD%.com}\\.com"
scen email-md-escape-at w a.md "jane\\@${PD}"
scen home-users w a.md "/$U/alice/code/x"
scen home-home w a.md "/$H/bob/x"
scen home-encoded w a.md "projects/-$U-alice-code-waggle/x.jsonl"
scen temp-macos w a.md "/$VF/ab/cd1234/T/x"
scen temp-harness w a.md "/$PT/claude-redacted/x"
scen hostname w a.md "ran on $HOSTV today"
scen hostname-upper w a.md "ran on $(printf '%s' "$HOSTV" | tr '[:lower:]' '[:upper:]').local"
scen name w a.md "reviewed by $NAMEV"
scen tok-ghp w a.md "ghp_$(printf 'A%.0s' $(seq 36))"
scen tok-pat w a.md "github_pat_$(printf 'B%.0s' $(seq 40))"
scen tok-skant w a.md "sk-ant-api03-$(printf 'c%.0s' $(seq 40))"
scen tok-jwt-not-covered w a.md "eyJhbGciOiJIUzI1NiJ9.$(printf 'd%.0s' $(seq 40)).$(printf 'e%.0s' $(seq 20))"
scen tok-aws-not-covered w a.md "AKIA$(printf 'Q%.0s' $(seq 16))"
scen em-dash w a.md "a ${EM} b"
scen en-dash w a.md "1${EN}2"
scen dash-in-code-block w a.md $'```\nx '"${EM}"$' y\n```'
scen personal-mail w a.md "via ${PM}${PM2} today"
scen binary sh -c "printf 'a\000b' >$D/bin.dat"
scen non-hive sh -c 'echo x >>.gitignore'
scen decisions-edit sh -c "sed 's/^\*\*Rationale:\*\* Saves one wait per fix round/**Rationale:** Edited/' .waggle/hive/DECISIONS.md >d.tmp && mv d.tmp .waggle/hive/DECISIONS.md"
scen crlf sh -c "printf 'x jane.doe@${PD}\r\ny ${EM}\r\n' >$D/a.md"
[ -n "${SKIP_LONG:-}" ] || scen long-line sh -c "{ head -c 2000000 /dev/zero | tr '\\0' a; printf ' jane.doe@${PD}\n'; } >$D/a.md"
scen name-with-space w 'a b.md' "x ${EM} y"
scen email-in-filename w "jane.doe@${PD}.txt" "x"
scen non-ascii-filename w $'caf\303\251.md' "x"
MSG="plant msg only: jane.doe@${PD}" scen msg-only true

# Renamed file carrying an existing hit: plant a hit, then rename it.
git checkout -q --detach "$BASE"; mkdir -p "$D"; w r1.md "x /$U/alice/y"
git add -A -- .waggle; "${GC[@]}" commit -q -m p1; mid=$(git rev-parse HEAD)
git mv "$D/r1.md" "$D/r2.md"; "${GC[@]}" commit -q -m p2; r=$(git rev-parse HEAD)
out=$("$SCAN" "$mid" "$r" 2>&1); echo "== renamed-only (base = commit with the hit): exit $?"; printf '%s\n' "$out" | sed -n '/^hits/,/^result/p' | sed 's/^/   /'

# Hit added in one commit, removed in the next (lands in history on --merge).
git checkout -q --detach "$BASE"; mkdir -p "$D"; w a.md "contact jane.doe@${PD}"
git add -A -- .waggle; "${GC[@]}" commit -q -m q1
w a.md "contact redacted"; git add -A -- .waggle; "${GC[@]}" commit -q -m q2; r=$(git rev-parse HEAD)
out=$("$SCAN" "$BASE" "$r" 2>&1); echo "== added-then-removed (net diff clean, hit in history): exit $?"; printf '%s\n' "$out" | sed -n '/^hits/,/^result/p' | sed 's/^/   /'

# Existing line deleted, then re-added verbatim (net no change).
git checkout -q --detach "$BASE"
sed '/^# Queen handoff/d' .waggle/hive/handoff/queen.md >q.tmp && mv q.tmp .waggle/hive/handoff/queen.md; git add -A -- .waggle; "${GC[@]}" commit -q -m s1
git checkout -q "$BASE" -- .waggle/hive/handoff/queen.md; "${GC[@]}" commit -q -m s2; r=$(git rev-parse HEAD)
out=$("$SCAN" "$BASE" "$r" 2>&1); echo "== deleted-then-readded: exit $?"; printf '%s\n' "$out" | sed -n '/^hits/,/^result/p' | sed 's/^/   /'

# Usage errors.
"$SCAN" a b c >/dev/null 2>&1; echo "== three args: exit $?"
"$SCAN" no-such-ref >/dev/null 2>&1; echo "== unknown base: exit $?"

git checkout -q --detach "$BASE"
