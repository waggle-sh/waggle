#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Shows the Waggle gate passing and failing on a throwaway repo, with no
# network and no GitHub. Everything happens in a mktemp directory that is
# removed on exit.
#
# Usage: run.sh <path-to-waggle-binary>
set -eu

if [ "$#" -ne 1 ]; then
	echo "usage: run.sh <path-to-waggle-binary>" >&2
	exit 2
fi
case $1 in
/*) waggle=$1 ;;
*) waggle=$(pwd)/$1 ;;
esac
if [ ! -x "$waggle" ]; then
	echo "run.sh: $waggle is not an executable file" >&2
	exit 2
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/waggle-gate-walkthrough.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM

# Keep git away from the user's own configuration and identity.
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_TERMINAL_PROMPT=0
export GIT_AUTHOR_NAME=Demo GIT_AUTHOR_EMAIL=demo@example.com
export GIT_COMMITTER_NAME=Demo GIT_COMMITTER_EMAIL=demo@example.com

step() {
	printf '\n== %s\n' "$1"
}

step "bare repo as origin, and a clone"
git init --quiet --bare --initial-branch=main "$work/origin.git"
git clone --quiet "$work/origin.git" "$work/repo" 2>/dev/null
cd "$work/repo"
git checkout --quiet -b main

step "waggle init --gate-only, committed on main"
"$waggle" init --gate-only
git add .github .waggle
git commit --quiet -s -m "Install the Waggle gate"
git push --quiet origin main

step "waggle claim T-001 --dirs src --gate light --branch t-001 --owner demo"
"$waggle" claim T-001 --dirs src --gate light --branch t-001 --owner demo

step "commit T-001: add src/a.txt on branch t-001"
git checkout --quiet -b t-001
mkdir src
echo "first file" >src/a.txt
git add src/a.txt
git commit --quiet -s -m "T-001: add src/a.txt"

step "waggle gate verify --base main --head t-001"
status=0
"$waggle" gate verify --base main --head t-001 || status=$?
echo "exit $status"

step "commit T-001: touch README.md, outside the leased src/"
echo "demo" >README.md
git add README.md
git commit --quiet -s -m "T-001: touch README.md"

step "waggle gate verify --base main --head t-001"
status=0
"$waggle" gate verify --base main --head t-001 || status=$?
echo "exit $status"
