#!/bin/sh
# Comb fetch under the D0b ExtraEnv (COUNT=3) and the pre-PR one (COUNT=1),
# against a repo built by opus-trace.sh. Usage: opus-comb-fetch.sh <repo dir>
set -u
r=$1
common="PATH=/usr/bin:/bin HOME=/nonexistent GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_TERMINAL_PROMPT=0 LC_ALL=C GIT_NO_REPLACE_OBJECTS=1"
for v in pr old; do
  if [ $v = pr ]; then extra="GIT_CONFIG_COUNT=3 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null GIT_CONFIG_KEY_1=maintenance.auto GIT_CONFIG_VALUE_1=false GIT_CONFIG_KEY_2=gc.auto GIT_CONFIG_VALUE_2=0"
  else extra="GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null"; fi
  t="$r/../comb-$v.trace"; rm -f "$t"
  env -i $common $extra GIT_TRACE2_EVENT="$t" git -C "$r" fetch --prune --no-tags --quiet origin '+refs/heads/*:refs/remotes/origin/*'
  echo "comb fetch $v: $(grep '"child_start"' "$t" | grep -c '"maintenance"') maintenance child_start"
done
