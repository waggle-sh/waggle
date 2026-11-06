#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Dry-run report for the public-history rewrite.
# Reads git objects and prints counts. Does not run git-filter-repo, does not
# update a ref, and does not write inside the repository.

set -euo pipefail

if [[ $# -ne 0 ]]; then
  echo "dry-run only: this script takes no arguments and does not rewrite history" >&2
  exit 2
fi
if [[ -n "${GIT_FILTER_REPO_EXECUTE:-}" ]]; then
  echo "refusing: this script cannot execute a rewrite" >&2
  exit 2
fi

root=$(git rev-parse --show-toplevel)
cd "$root"

if command -v git-filter-repo >/dev/null 2>&1; then
  echo "note: git-filter-repo is on PATH and is not invoked"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# Mailmap. Any name on these emails becomes Paul Cleary at the public noreply
# address. Commits that already use that noreply address are not listed.
cat >"$tmp/mailmap" <<'EOF'
Paul Cleary <27632860+paulternate@users.noreply.github.com> <27632860+paulternate@users.noreply.github.com>
Paul Cleary <27632860+paulternate@users.noreply.github.com> <27632860+paulternate@users.noreply.github.com>
Paul Cleary <27632860+paulternate@users.noreply.github.com> <27632860+paulternate@users.noreply.github.com>
EOF

# Expressions for git-filter-repo --replace-text and --replace-message.
# Order is mandatory: each literal is applied to the bytes produced by the
# previous one. Longer strings that contain a later needle come first.
cat >"$tmp/expressions.txt" <<'EOF'
literal:Signed-off-by: Paul Cleary <27632860+paulternate@users.noreply.github.com>==>Signed-off-by: Paul Cleary <27632860+paulternate@users.noreply.github.com>
literal:27632860+paulternate@users.noreply.github.com==>27632860+paulternate@users.noreply.github.com
literal:27632860+paulternate@users.noreply.github.com==>27632860+paulternate@users.noreply.github.com
literal:27632860+paulternate@users.noreply.github.com==>27632860+paulternate@users.noreply.github.com
literal:/private/tmp/claude-redactedredactedredacted/waggle-scratch==>/private/tmp/claude-redactedredactedredacted/waggle-scratch
literal:/tmp/claude-redactedredacted==>/tmp/claude-redactedredacted
literal:/var/folders/xx/redacted==>/var/folders/xx/redacted
literal:/Users/example==>/Users/example
literal:~/redacted/znap==>~/redacted/znap
literal:redacted-company.md==>redacted-company.md
literal:redacted.local==>redacted.local
EOF

readme="$root/docs/redaction/README.md"
if [[ ! -f "$readme" ]]; then
  echo "missing $readme" >&2
  exit 2
fi
python3 - "$readme" "$tmp/expressions.txt" "$tmp/mailmap" <<'PY'
import pathlib, sys
readme, expr, mail = map(pathlib.Path, sys.argv[1:])
text = readme.read_text()
missing = []
for line in expr.read_text().splitlines():
    if not line.startswith("literal:"):
        continue
    old, new = line[len("literal:"):].split("==>", 1)
    if old not in text or new not in text:
        missing.append(old)
for line in mail.read_text().splitlines():
    if line and line not in text:
        missing.append(line)
if missing:
    sys.stderr.write("README.md is missing a mailmap or expression line:\n")
    for item in missing:
        sys.stderr.write(f"  {item}\n")
    sys.exit(2)
PY

echo "== mailmap preview (display only; refs are not updated) =="
echo "-- author emails before --"
git log --all --format='%ae' | sort | uniq -c | sort -nr
echo "-- author emails after mailmap (%aE; %ae stays raw) --"
git -c mailmap.file="$tmp/mailmap" log --use-mailmap --all --format='%aE' | sort | uniq -c | sort -nr
echo "-- committer emails after mailmap (%cE) --"
git -c mailmap.file="$tmp/mailmap" log --use-mailmap --all --format='%cE' | sort | uniq -c | sort -nr
echo "-- author name+email pairs that the mailmap changes --"
git log --all --format='%H%x09%an%x09%ae%x09%cn%x09%ce' >"$tmp/before"
git -c mailmap.file="$tmp/mailmap" log --use-mailmap --all --format='%H%x09%aN%x09%aE%x09%cN%x09%cE' >"$tmp/after"
python3 - "$tmp/before" "$tmp/after" <<'PY'
import collections, sys
before = {}
for line in open(sys.argv[1]):
    h, an, ae, cn, ce = line.rstrip("\n").split("\t")
    before[h] = (an, ae, cn, ce)
authors = collections.Counter()
committers = collections.Counter()
for line in open(sys.argv[2]):
    h, an, ae, cn, ce = line.rstrip("\n").split("\t")
    old = before.get(h)
    if not old:
        continue
    if old[0:2] != (an, ae):
        authors[(old[0], old[1], an, ae)] += 1
    if old[2:4] != (cn, ce):
        committers[(old[2], old[3], cn, ce)] += 1
print("authors:")
for (on, oe, nn, ne), n in authors.most_common():
    print(f"{n}\t{on} <{oe}>  =>  {nn} <{ne}>")
if not authors:
    print("(none)")
print("committers:")
for (on, oe, nn, ne), n in committers.most_common():
    print(f"{n}\t{on} <{oe}>  =>  {nn} <{ne}>")
if not committers:
    print("(none)")
PY

echo "== commit-message needles =="
git log --all --format='%B' >"$tmp/msgs"
python3 - "$tmp/msgs" <<'PY'
import sys
text = open(sys.argv[1], encoding="utf-8", errors="replace").read()
needles = [
    "27632860+paulternate@users.noreply.github.com",
    "27632860+paulternate@users.noreply.github.com",
    "27632860+paulternate@users.noreply.github.com",
    "/Users/example",
    "redacted.local",
    "claude-redacted",
    "redacted-company.md",
    "/var/folders/xx/redacted",
]
for n in needles:
    print(f"{text.count(n)}\t{n}")
PY

echo "== blob needles at HEAD, outside docs/redaction =="
python3 - "$root" <<'PY'
import subprocess, sys
root = sys.argv[1]
needles = [
    "27632860+paulternate@users.noreply.github.com",
    "27632860+paulternate@users.noreply.github.com",
    "27632860+paulternate@users.noreply.github.com",
    "/private/tmp/claude-redactedredactedredacted/waggle-scratch",
    "/tmp/claude-redactedredacted",
    "/var/folders/xx/redacted",
    "/Users/example",
    "~/redacted/znap",
    "redacted-company.md",
    "redacted.local",
]
files = {}
for needle in needles:
    r = subprocess.run(
        ["git", "grep", "-I", "-l", "-F", "-e", needle],
        cwd=root, text=True, capture_output=True,
    )
    if r.returncode not in (0, 1):
        sys.stderr.write(r.stderr)
        sys.exit(r.returncode or 1)
    for path in r.stdout.splitlines():
        if path.startswith("docs/redaction/"):
            continue
        files.setdefault(path, []).append(needle)
for path in sorted(files):
    print(path)
    for needle in files[path]:
        print(f"    {needle}")
print(f"-- {len(files)} files outside docs/redaction --")
PY

echo "== history blob presence (every commit tree; this is a read) =="
python3 - "$root" <<'PY'
import subprocess, sys
root = sys.argv[1]
needles = [
    "27632860+paulternate@users.noreply.github.com",
    "27632860+paulternate@users.noreply.github.com",
    "27632860+paulternate@users.noreply.github.com",
    "claude-redacted",
    "/Users/example",
    "/var/folders/xx/redacted",
    "redacted-company.md",
    "~/redacted/znap",
    "redacted.local",
]
commits = subprocess.check_output(["git", "rev-list", "--all"], cwd=root, text=True).split()
hits = {n: set() for n in needles}
for i in range(0, len(commits), 40):
    batch = commits[i:i + 40]
    for needle in needles:
        r = subprocess.run(
            ["git", "grep", "-I", "-l", "-F", "-e", needle, *batch],
            cwd=root, text=True, capture_output=True,
        )
        if r.returncode not in (0, 1):
            sys.stderr.write(r.stderr)
            sys.exit(r.returncode or 1)
        for line in r.stdout.splitlines():
            hits[needle].add(line.split(":", 1)[0])
print(f"commits walked: {len(commits)}")
for needle in needles:
    print(f"{len(hits[needle])}\tcommits contain {needle}")
PY

cat <<'EOF'

== intended rewrite (not run) ==
Copy docs/redaction/ out of the repository first. Run the following only in a
fresh mirror clone, after PR 201 and the working-tree scrub commits are merged,
and do not push from this script:

  git clone --mirror git@github.com:waggle-sh/waggle.git waggle-rewrite.git
  cd waggle-rewrite.git
  git filter-repo \
    --mailmap /ABSOLUTE/PATH/OUTSIDE/mailmap \
    --replace-text /ABSOLUTE/PATH/OUTSIDE/expressions.txt \
    --replace-message /ABSOLUTE/PATH/OUTSIDE/expressions.txt

filter-repo rewrites every ref in that mirror, drops the origin remote, and
stops. Publishing the result is a later force-push by the Head Keeper. This
script does not print or run that push.
EOF
