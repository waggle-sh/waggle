#!/usr/bin/env bash
# scan-hive.sh: privacy and policy scan for a hive branch (D-183).
#
# Usage: .waggle/control/scripts/scan-hive.sh [base] [head]
#   base defaults to origin/main, head to HEAD. Run from anywhere inside the
#   repo; only committed content in merge-base(base, head)..head is scanned.
#
# Scanned: every added line of `git diff base...head`, of each non-merge
# commit against its first parent, and of each merge commit by remerge diff
# (what the merge result changes against git's own automatic merge, so a
# conflict resolution or an edit made during the merge counts and a clean
# merge adds nothing), so a hit added then removed on the branch is still
# reported; every changed path name, including paths a merge adds; and each
# commit's author, committer and message. A NUL byte in a text line is read
# as \001, so awk does not stop at it. Needs git 2.36 or later (checked
# first, exit 3 below it). Rules:
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
#               D-160 noreply handle (skipped when CI is set; a warning line,
#               not a hit, says so when scan-names.local is missing)
#   token       ghp_, gho_, github_pat_, sk-ant-, sk-, JWTs (eyJ + three
#               base64url parts), AWS AKIA + 16, Slack xox[abp]-, Supabase
#               sbp_, Resend re_ + a long mixed-case run with a digit
#   dash        en or em dash
#   personal-mail  the Keeper's personal mail provider name, any case
#   local-file  an added path named *.local or *.local.* (a forced add of the
#               gitignored handoff or names file)
#   long-line   a line over 65536 bytes (reported, not regex-scanned)
#   binary      a changed binary file (cannot be scanned)
#   octopus-merge  a commit with three or more parents (git does not remerge
#               it, so its result cannot be scanned)
#   remerge-skipped  any other merge git declines to remerge (a `diff: warning:
#               Skipping remerge-diff` line), on the commit it follows
#   nul-commit  a commit object containing a NUL byte (git stops reading the
#               author or message there, so the rest cannot be scanned)
#   decisions   `go run ./cmd/waggle gate decisions` fails
#
# Hits print as <file>:<line>: <rule>, never the matched value. A per-commit
# hit prints as `commit <sha> <file>:<line>`; commit fields print as
# `commit <sha> author|committer|message:<line>`; a path name hit prints as
# <file>:0.
#
# Exit: 0 no hit and every change is a records change: an added regular file
# under .waggle/hive/reviews/ or .waggle/hive/media/, a pure-append
# .waggle/hive/DECISIONS.md, or an edit to .waggle/hive/handoff/queen.md;
# 1 any hit; 2 no hit but some other change (anything outside .waggle/hive/,
# any other added, modified, deleted or renamed hive file such as a task brief,
# an existing review or a steering doc, any symlink or .gitattributes);
# 3 usage or git error.
set -euo pipefail
export LC_ALL=C

case "${1:-}" in -h|--help) awk 'NR > 1 && !/^#/ { exit } NR > 1 { sub(/^# ?/, ""); print }' "$0"; exit 0;; esac
[ $# -le 2 ] || { echo "usage: scan-hive.sh [base] [head]" >&2; exit 3; }
# --diff-merges=remerge needs git 2.36 or later.
gv=$(git version 2>/dev/null | awk 'NR == 1 { split($3, v, "."); print v[1] * 1000 + v[2] }' || true)
[ "${gv:-0}" -ge 2036 ] 2>/dev/null || { echo "scan-hive: needs git 2.36 or later" >&2; exit 3; }
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
# Pinned diff options: user config (noprefix, mnemonicPrefix, external diff,
# textconv, color, renames) must not change labels or hide content.
dopts=(--no-color --no-ext-diff --no-textconv --no-renames --src-prefix=a/ --dst-prefix=b/)
lopts=(--no-merges --no-show-signature)
mopts=(--merges --no-show-signature --diff-merges=remerge)

# Net changes with modes and status: :<old mode> <new mode> <old> <new> <status> TAB <path>
g diff "${dopts[@]}" --raw "$mb" "$headsha" >"$tmp/raw"
cut -f 2- "$tmp/raw" >"$tmp/paths"

# Path names: every net changed path, and every path a commit added or
# changed, as line 0 of itself. An added *.local or *.local.* path is a hit.
{
  cat "$tmp/raw"
  g log "${lopts[@]}" "${dopts[@]}" --raw --format='commit %h' "$mb..$headsha"
  g log "${mopts[@]}" "${dopts[@]}" --raw --format='commit %h' "$mb..$headsha"
} | awk -F '\t' '
  /^commit [0-9a-f]+$/ { pre = $0 " "; next }
  /^:/ { split($1, m, " "); st = m[5]; p = $2
         if (pre == "" || st != "D") printf "%s%s\t0\t%s\n", pre, p, p
         b = p; sub(/.*\//, "", b)
         if (st == "A" && (b ~ /\.local$/ || b ~ /\.local\./)) printf "%s%s\t0\t\001local-file\n", pre, p }
' >"$stream"

# Added lines of the net diff, of each non-merge commit against its first
# parent and of each merge by remerge diff. -U0 means every hunk line is + or
# -, plus header lines; a commit line sets the label prefix for the
# per-commit part. NUL becomes \001 because BSD awk ends a line at a NUL.
{
  g diff "${dopts[@]}" -U0 "$mb" "$headsha"
  g log "${lopts[@]}" "${dopts[@]}" -p -U0 --format='commit %h' "$mb..$headsha"
  g log "${mopts[@]}" "${dopts[@]}" -p -U0 --format='commit %h' "$mb..$headsha"
} | tr '\000' '\001' | awk '
  /^commit [0-9a-f]+$/ { pre = $0 " "; hdr = 1; path = ""; next }
  /^diff: warning: Skipping remerge-diff/ { printf "%s\t0\t\001remerge-skipped\n", substr(pre, 1, length(pre) - 1); next }
  /^diff --git / { hdr = 1; path = ""; next }
  hdr && /^\+\+\+ / { path = substr($0, 5); sub(/\t$/, "", path); sub(/^b\//, "", path); next }
  hdr && /^Binary files .* and \/dev\/null differ$/ { next }
  hdr && /^Binary files / { p = $0; sub(/^Binary files .* and b\//, "", p); sub(/ differ$/, "", p)
                            printf "%s%s\t0\t\001binary\n", pre, p; next }
  /^@@ / { hdr = 0; split($3, a, ","); ln = substr(a[1], 2) + 0; next }
  hdr { next }
  /^\+/ { printf "%s%s\t%d\t%s\n", pre, path, ln, substr($0, 2); ln++ }
' >>"$stream"

# Commit identities and messages.
for c in $(git rev-list "$mb..$headsha"); do
  s=$(git rev-parse --short "$c")
  git log -1 --no-show-signature --format='%an <%ae>' "$c" | awk -v l="commit $s author" '{ printf "%s\t%d\t%s\n", l, NR, $0 }'
  git log -1 --no-show-signature --format='%cn <%ce>' "$c" | awk -v l="commit $s committer" '{ printf "%s\t%d\t%s\n", l, NR, $0 }'
  git log -1 --no-show-signature --format='%B' "$c" | awk -v l="commit $s message" '{ printf "%s\t%d\t%s\n", l, NR, $0 }'
  # git stops %an and %B at a NUL, so a NUL in the raw object fails closed.
  [ "$(git cat-file commit "$c" | tr -cd '\000' | wc -c | tr -d ' ')" -eq 0 ] || printf 'commit %s\t0\t\001nul-commit\n' "$s"
done >>"$stream"
# git does not remerge an octopus merge, so each one fails closed.
for c in $(git rev-list --min-parents=3 "$mb..$headsha"); do
  printf 'commit %s\t0\t\001octopus-merge\n' "$(git rev-parse --short "$c")"
done >>"$stream"

host="" name="" warn=""
if [ -z "${CI:-}" ]; then
  host=$(hostname -s 2>/dev/null || true)
  name="$(git config --global user.name 2>/dev/null || true) $(git config user.name 2>/dev/null || true)"
  name="$name ${HOME##*/} ${SCAN_HIVE_NAMES:-}"
  if [ -f .waggle/control/scan-names.local ]; then
    name="$name $(tr '\n' ' ' <.waggle/control/scan-names.local)"
  else
    warn="warning: .waggle/control/scan-names.local is missing, so the Keeper's surname is not being checked"
  fi
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
# Token shapes: prefix regex pre (plen characters long) followed by at least
# min characters of the body set, not preceded by a word character.
function token(s, pre, plen, min,   b) {
  while (match(s, pre "[A-Za-z0-9_-]+")) {
    b = (RSTART == 1) ? "" : substr(s, RSTART - 1, 1)
    if (b !~ /[A-Za-z0-9_]/ && RLENGTH - plen >= min) return 1
    s = substr(s, RSTART + RLENGTH)
  }
  return 0
}
# JWT: eyJ header, payload and signature in base64url, 36+ characters.
function jwt(s,   b) {
  while (match(s, /eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/)) {
    b = (RSTART == 1) ? "" : substr(s, RSTART - 1, 1)
    if (b !~ /[A-Za-z0-9_]/ && RLENGTH >= 36) return 1
    s = substr(s, RSTART + RLENGTH)
  }
  return 0
}
# AWS access key id: AKIA plus exactly 16 upper-case letters or digits.
function aws(s,   b, a) {
  while (match(s, /AKIA[A-Z0-9]+/)) {
    b = (RSTART == 1) ? "" : substr(s, RSTART - 1, 1); a = substr(s, RSTART + RLENGTH, 1)
    if (b !~ /[A-Za-z0-9_]/ && a !~ /[A-Za-z0-9_]/ && RLENGTH == 20) return 1
    s = substr(s, RSTART + RLENGTH)
  }
  return 0
}
# Resend: re_ plus 20+ characters with an upper-case letter, a lower-case
# letter and a digit, so snake_case identifiers do not match.
function resend(s,   b, body) {
  while (match(s, /re_[A-Za-z0-9_]+/)) {
    b = (RSTART == 1) ? "" : substr(s, RSTART - 1, 1); body = substr(s, RSTART + 3, RLENGTH - 3)
    if (b !~ /[A-Za-z0-9_]/ && length(body) >= 20 && body ~ /[A-Z]/ && body ~ /[a-z]/ && body ~ /[0-9]/) return 1
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
  key = $1 ":" $2; s = $3; for (i = 4; i <= NF; i++) s = s "\t" $i
  if (s == "\001binary") { hit("binary"); next }
  if (s == "\001local-file") { hit("local-file"); next }
  if (s == "\001octopus-merge" || s == "\001remerge-skipped" || s == "\001nul-commit") { hit(substr(s, 2)); next }
  if (length(s) > 65536) { hit("long-line"); next }
  low = tolower(s)
  t = index(s, "@") ? s : "" # no @, no address: skip the email regex
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
  if (token(s, "ghp_", 4, 20) || token(s, "gho_", 4, 20) || token(s, "github_pat_", 11, 20) || token(s, "sk-ant-", 7, 10) || token(s, "sk-", 3, 20) \
      || token(s, "xox[abp]-", 5, 10) || token(s, "sbp_", 4, 20) || jwt(s) || aws(s) || resend(s)) hit("token")
  if (index(s, en) || index(s, em)) hit("dash")
  if (index(low, pm)) hit("personal-mail")
}' "$stream" >"$tmp/hits"

gate_rc=0
go run ./cmd/waggle gate decisions --base "$base" --head "$headsha" >"$tmp/gate" 2>&1 || gate_rc=$?
[ "$gate_rc" -eq 0 ] || echo "decisions gate: decisions (exit $gate_rc)" >>"$tmp/hits"

# Records set (D-183): anything else holds the batch for the guard and card.
awk -F '\t' '{
  split($1, m, " "); om = substr(m[1], 2); nm = m[2]; st = m[5]; p = $2; r = ""
  if (p !~ /^\.waggle\/hive\//) r = "outside .waggle/hive/"
  else if (om == "120000" || nm == "120000") r = "symlink"
  else if (om == "160000" || nm == "160000") r = "submodule"
  else if (p ~ /(^|\/)\.gitattributes$/) r = ".gitattributes"
  else if (st == "A" && p ~ /^\.waggle\/hive\/(reviews|media)\//) r = ""
  else if (st == "M" && (p == ".waggle/hive/DECISIONS.md" || p == ".waggle/hive/handoff/queen.md")) r = ""
  else r = (st == "A" ? "added" : st == "D" ? "deleted" : st == "M" ? "modified" : "status " st) " outside the records set"
  printf "%s\t%s\n", r, p
}' "$tmp/raw" >"$tmp/class"
held=$(awk -F '\t' '$1 != ""' "$tmp/class" | wc -l | tr -d ' ')

echo "scan-hive: base $base (merge-base $(git rev-parse --short "$mb")), head $(git rev-parse --short "$headsha"), $(git rev-list --count "$mb..$headsha") commit(s) scanned"
[ -z "$warn" ] || echo "$warn"
echo "changed paths ($(wc -l <"$tmp/paths" | tr -d ' ')):"
awk -F '\t' '{ print ($1 == "") ? "  " $2 : "  HOLD (" $1 ") " $2 }' "$tmp/class"
echo "decisions gate (exit $gate_rc):"
sed 's/^/  /' "$tmp/gate"
nhits=$(wc -l <"$tmp/hits" | tr -d ' ')
echo "hits ($nhits):"
sed 's/^/  /' "$tmp/hits"
if [ "$nhits" -gt 0 ]; then
  echo "result: FAIL, $nhits hit(s): guard and card"; exit 1
elif [ "$held" -gt 0 ]; then
  echo "result: clean, but $held change(s) outside the records set: guard and card"; exit 2
fi
echo "result: clean records batch: merge with --admin, one-line note to the Keeper"
exit 0
