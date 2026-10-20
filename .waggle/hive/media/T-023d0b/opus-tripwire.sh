#!/bin/sh
# Push tripwire still wins under the 14-key env. Usage: opus-tripwire.sh <worker.env> <repo>
set -u
ef=$1 r=$2
for u in https://github.com/waggle-sh/waggle.git git@github.com:waggle-sh/waggle.git ssh://git@github.com/x.git /srv/x.git ./x.git; do
  out=$(cd "$r" && env -i PATH=/usr/bin:/bin HOME=/nonexistent sh -c 'set -a; . "$1"; set +a; git remote set-url origin "$2" && git remote get-url --push origin && git config --get maintenance.auto && git config --get gc.auto' sh "$ef" "$u" | tr '\n' ' ')
  echo "$u -> $out"
done
