#!/bin/sh
# Times scan-hive.sh on one added line of N bytes (letters only, no hit).
# Usage: longline-bench.sh <base sha> <bytes>...   (repo root, clean tree)
BASE=$1; shift
D=.waggle/hive/media/guard-plant
for n in "$@"; do
  git checkout -q --detach "$BASE"; mkdir -p "$D"
  head -c "$n" /dev/zero | tr '\0' a >"$D/long.txt"; echo >>"$D/long.txt"
  git add -- "$D"; git -c user.name=fixture -c user.email=fixture@example.test commit -q -m long
  s=$(date +%s); .waggle/control/scripts/scan-hive.sh "$BASE" HEAD >/dev/null 2>&1; rc=$?; e=$(date +%s)
  echo "line of $n bytes: exit $rc, $((e - s)) s (includes the go run gate)"
done
git checkout -q --detach "$BASE"
