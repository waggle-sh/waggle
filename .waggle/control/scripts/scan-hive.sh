#!/usr/bin/env bash
# scan-hive.sh: privacy and policy scan for a hive branch (D-183).
#
# Usage: .waggle/control/scripts/scan-hive.sh [base] [head]
#   base defaults to origin/main, head to HEAD. Run from anywhere inside the
#   repo; only committed content in merge-base(base, head)..head is scanned.
#
# Scanned: every added line of `git diff base...head`, every changed path
# name, and each commit's author, committer and message. Rules:
#   email       any address except the D-160 noreply, noreply@github.com,
#               git@ remote forms and the fixture domains example.test,
#               example.com, e.co, evil.test, included.test (and subdomains)
#   home-path   /Users/<name>, /home/<name> (except the fixture /home/keeper),
#               -Users-<name> (encoded project dirs)
#   temp-path   the macOS temp root (/var/ + folders) and the Claude scratch
#               prefix (/private/tmp/ + claude-)
#   hostname    this machine's `hostname -s`, whole word, any case (skipped
#               when CI is set)
#   name        each word (3+ letters) of the local and global git user.name,
#               the home directory's last component, $SCAN_HIVE_NAMES and the
#               gitignored .waggle/control/scan-names.local (the Keeper's
#               names, never committed); whole word, any case, minus the
#               D-160 noreply handle (skipped when CI is set)
#   token       ghp_, gho_, github_pat_, sk-ant-, sk- followed by a long run
#   dash        en or em dash
#   personal-mail  the Keeper's personal mail provider name, any case
#   binary      a changed binary file (cannot be scanned)
#   decisions   `go run ./cmd/waggle gate decisions` fails
#
# Hits print as <file>:<line>: <rule>, never the matched value. Commit fields
# print as `commit <sha> author|committer|message:<line>`; a path name hit
# prints as <file>:0.
#
# Exit: 0 no hit and every changed path under .waggle/hive/; 1 any hit;
# 2 no hit but a path outside .waggle/hive/ changed; 3 usage or git error.
set -euo pipefail
export LC_ALL=C

case "${1:-}" in -h|--help) awk 'NR > 1 && !/^#/ { exit } NR > 1 { sub(/^# ?/, ""); print }' "$0"; exit 0;; esac
[ $# -le 2 ] || { echo "usage: scan-hive.sh [base] [head]" >&2; exit 3; }
base=${1:-origin/main}
head=${2:-HEAD}

root=$(git rev-parse --show-toplevel) || exit 3
cd "$root"
git rev-parse -q --verify "$base^{commit}" >/dev/null || { echo "scan-hive: unknown base $base" >&2; exit 3; }
git rev-parse -q --verify "$head^{commit}" >/dev/null || { echo "scan-hive: unknown head $head" >&2; exit 3; }
mb=$(git merge-base "$base" "$head") || { echo "scan-hive: no merge base" >&2; exit 3; }
headsha=$(git rev-parse "$head^{commit}")

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
stream=$tmp/stream # <label> TAB <line> TAB <text>
g() { git -c core.quotePath=true "$@"; }

g diff --name-only --no-renames "$mb" "$headsha" >"$tmp/paths"

# Changed path names are scanned as line 0 of themselves.
awk '{ printf "%s\t0\t%s\n", $0, $0 }' "$tmp/paths" >"$stream"

# Added lines. -U0 means every hunk line is + or -, plus header lines.
g diff --no-color --no-ext-diff --no-textconv --no-renames -U0 "$mb" "$headsha" | awk '
  /^diff --git / { hdr = 1; path = ""; next }
  hdr && /^\+\+\+ / { path = substr($0, 5); sub(/^b\//, "", path); next }
  hdr && /^Binary files .* and \/dev\/null differ$/ { next }
  hdr && /^Binary files / { p = $0; sub(/^Binary files .* and b\//, "", p); sub(/ differ$/, "", p)
                            printf "%s\t0\t\001binary\n", p; next }
  /^@@ / { hdr = 0; split($3, a, ","); ln = substr(a[1], 2) + 0; next }
  hdr { next }
  /^\+/ { printf "%s\t%d\t%s\n", path, ln, substr($0, 2); ln++ }
' >>"$stream"

# Commit identities and messages.
for c in $(git rev-list "$mb..$headsha"); do
  s=$(git rev-parse --short "$c")
  git log -1 --format='%an <%ae>' "$c" | awk -v l="commit $s author" '{ printf "%s\t%d\t%s\n", l, NR, $0 }'
  git log -1 --format='%cn <%ce>' "$c" | awk -v l="commit $s committer" '{ printf "%s\t%d\t%s\n", l, NR, $0 }'
  git log -1 --format='%B' "$c" | awk -v l="commit $s message" '{ printf "%s\t%d\t%s\n", l, NR, $0 }'
done >>"$stream"

host="" name=""
if [ -z "${CI:-}" ]; then
  host=$(hostname -s 2>/dev/null || true)
  name="$(git config --global user.name 2>/dev/null || true) $(git config user.name 2>/dev/null || true)"
  name="$name ${HOME##*/} ${SCAN_HIVE_NAMES:-}"
  [ -f .waggle/control/scan-names.local ] && name="$name $(tr '\n' ' ' <.waggle/control/scan-names.local)"
fi
en=$(printf '\342\200\223')
em=$(printf '\342\200\224')

awk -F '\t' -v host="$host" -v names="$name" -v en="$en" -v em="$em" '
function hit(rule) { if (!((key, rule) in seen)) { seen[key, rule] = 1; printf "%s: %s\n", key, rule } }
# Whole-word, case-insensitive search for w (already lower case) in s.
function word(s, w,   i, b, a) {
  while ((i = index(s, w)) > 0) {
    b = (i == 1) ? "" : substr(s, i - 1, 1); a = substr(s, i + length(w), 1)
    if (b !~ /[a-z0-9]/ && a !~ /[a-z0-9]/) return 1
    s = substr(s, i + length(w))
  }
  return 0
}
function okmail(m,   at, local, dom, n, k) {
  m = tolower(m); at = index(m, "@"); local = substr(m, 1, at - 1); dom = substr(m, at + 1)
  if (m == "27632860+paulternate@users.noreply.github.com" || m == "noreply@github.com") return 1
  if (local == "git" || local ~ /\.git$/) return 1 # git@host remotes, url.git@host config keys
  for (k in fixture) { n = length(k)
    if (dom == k || (length(dom) > n && substr(dom, length(dom) - n) == "." k)) return 1 }
  return 0
}
# Token shapes: prefix followed by at least min characters of the body set,
# not preceded by a word character.
function token(s, pre, min,   re, b) {
  re = pre "[A-Za-z0-9_-]+"
  while (match(s, re)) {
    b = (RSTART == 1) ? "" : substr(s, RSTART - 1, 1)
    if (b !~ /[A-Za-z0-9_]/ && RLENGTH - length(pre) >= min) return 1
    s = substr(s, RSTART + RLENGTH)
  }
  return 0
}
BEGIN {
  split("example.test example.com e.co evil.test included.test", f, " "); for (i in f) fixture[f[i]] = 1
  host = tolower(host)
  n = split(tolower(names), w, /[^a-z]+/)
  for (i = 1; i <= n; i++) if (length(w[i]) >= 3 && w[i] != "paulternate") nameword[w[i]] = 1
  vf = "/var/" "folders"; pt = "/private/tmp/" "claude-"; pm = "proton" "mail"
}
{
  key = $1 ":" $2; s = $3; for (i = 4; i <= NF; i++) s = s "\t" $i; low = tolower(s)
  if (s == "\001binary") { hit("binary"); next }
  t = s
  while (match(t, /[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z][A-Za-z]+/)) {
    if (!okmail(substr(t, RSTART, RLENGTH))) hit("email")
    t = substr(t, RSTART + RLENGTH)
  }
  if (s ~ /\/Users\/[A-Za-z0-9_.]/ || s ~ /-Users-[A-Za-z0-9_.]/) hit("home-path")
  t = s
  while (match(t, /\/home\/[A-Za-z0-9_.-]+/)) {
    if (substr(t, RSTART + 6, RLENGTH - 6) != "keeper") hit("home-path")
    t = substr(t, RSTART + RLENGTH)
  }
  if (index(s, vf) || index(s, pt)) hit("temp-path")
  if (host != "" && word(low, host)) hit("hostname")
  for (k in nameword) if (word(low, k)) hit("name")
  if (token(s, "ghp_", 20) || token(s, "gho_", 20) || token(s, "github_pat_", 20) || token(s, "sk-ant-", 10) || token(s, "sk-", 20)) hit("token")
  if (index(s, en) || index(s, em)) hit("dash")
  if (index(low, pm)) hit("personal-mail")
}' "$stream" >"$tmp/hits"

gate_rc=0
go run ./cmd/waggle gate decisions --base "$base" --head "$headsha" >"$tmp/gate" 2>&1 || gate_rc=$?
[ "$gate_rc" -eq 0 ] || echo "decisions gate: decisions (exit $gate_rc)" >>"$tmp/hits"

outside=$(grep -cv '^\.waggle/hive/' "$tmp/paths" || true)
echo "scan-hive: base $base (merge-base $(git rev-parse --short "$mb")), head $(git rev-parse --short "$headsha")"
echo "changed paths ($(wc -l <"$tmp/paths" | tr -d ' ')):"
awk '{ print ($0 ~ /^\.waggle\/hive\//) ? "  " $0 : "  OUTSIDE HIVE " $0 }' "$tmp/paths"
echo "decisions gate (exit $gate_rc):"
sed 's/^/  /' "$tmp/gate"
nhits=$(wc -l <"$tmp/hits" | tr -d ' ')
echo "hits ($nhits):"
sed 's/^/  /' "$tmp/hits"
if [ "$nhits" -gt 0 ]; then
  echo "result: FAIL, $nhits hit(s): guard and card"; exit 1
elif [ "$outside" -gt 0 ]; then
  echo "result: clean, but $outside path(s) outside .waggle/hive/: guard and card"; exit 2
fi
echo "result: clean and hive-only: merge with --admin, one-line note to the Keeper"
exit 0
