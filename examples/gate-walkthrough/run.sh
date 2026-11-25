#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Gate walkthrough: shows `waggle gate verify` passing and then failing, in a
# throwaway repo. No network, no GitHub. Everything lives in one mktemp
# directory that is removed on exit.
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
	echo "run.sh: not an executable file: $1" >&2
	exit 2
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM

# Keep git out of the user's own config and identity.
export HOME="$work/home"
export GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=Demo GIT_AUTHOR_EMAIL=demo@example.com
export GIT_COMMITTER_NAME=Demo GIT_COMMITTER_EMAIL=demo@example.com
mkdir -p "$HOME"

step() {
	printf '\n== %s\n' "$1"
}

step "make a bare origin and a clone"
git init -q --bare -b main "$work/origin.git"
git clone -q "$work/origin.git" "$work/repo" 2>/dev/null
cd "$work/repo"
git checkout -q -b main

step "waggle init --gate-only"
"$waggle" init --gate-only
git add -A
git commit -q -s -m "Add the Waggle gate"
git push -q origin main

step "claim T-001 for src/ on branch t-001"
"$waggle" claim T-001 --dirs src --gate light --branch t-001 --owner demo
git fetch -q origin 'refs/waggle/*:refs/waggle/*' 2>/dev/null || true

step "first commit stays inside src/"
git checkout -q -b t-001
mkdir src
echo one >src/a.txt
git add src/a.txt
git commit -q -s -m "T-001: add src/a.txt" -m "Waggle-Ticket: T-001"

step "gate verify: expect exit 0"
set +e
"$waggle" gate verify --base main --head t-001
code=$?
set -e
echo "exit $code"

step "second commit touches README.md, outside src/"
echo hello >README.md
git add README.md
git commit -q -s -m "T-001: touch README.md" -m "Waggle-Ticket: T-001"

step "gate verify: expect exit 1"
set +e
"$waggle" gate verify --base main --head t-001
code=$?
set -e
echo "exit $code"
