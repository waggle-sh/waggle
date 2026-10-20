#!/bin/sh
# Real-git proof for T-023 phase D0b. Usage: opus-trace.sh <worker.env> <scratch dir>
# worker.env is the GIT_* part of runtime.workerEnv at 1599c2b (opus_probe_test.go).
# For each variant: build a repo with a bare origin and a linked worktree,
# plant worktrees/planted with no gitdir, then run the proving scenario's git
# commands (add, commit three times) in the worktree under the worker env, plus
# the host-side calls dispatch makes (fetch, worktree prune/add, status, push)
# with RunIn's -c flags, all under GIT_TRACE2_EVENT. Report every maintenance
# or gc child and whether the plant survived.
set -u
envfile=$1 scratch=$2
base() { env -i PATH="$PATH" HOME="$scratch/home" "$@"; }
mkdir -p "$scratch/home"
variant() { # name env-lines-file host-c-flags [hostile]
  name=$1 ef=$2 hostc=$3 hostile=${4:-}
  d="$scratch/$name"; rm -rf "$d"; mkdir -p "$d"
  base git init -q --bare "$d/origin.git"
  base git init -q -b main "$d/repo"
  base git -C "$d/repo" -c user.name=a -c user.email=a@e.co commit -q --allow-empty -m init
  base git -C "$d/repo" remote add origin "$d/origin.git"
  if [ -n "$hostile" ]; then
    for kv in "maintenance.auto true" "maintenance.strategy geometric" "gc.auto 1" "gc.autoDetach true" \
      "fetch.writeCommitGraph true" "maintenance.worktree-prune.auto 1" "gc.worktreePruneExpire now" "fetch.prune true"; do
      base git -C "$d/repo" config $kv
    done
  fi
  base git -C "$d/repo" push -q origin main
  host() { base GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_TRACE2_EVENT="$d/host.trace" git -C "$d/repo" $hostc "$@"; }
  host fetch -q origin
  host worktree prune
  host worktree add -q -b t-1 "$d/wt" main
  mkdir "$d/repo/.git/worktrees/planted"
  ( cd "$d/wt" && set -a && . "$ef" && set +a && export GIT_TRACE2_EVENT="$d/worker.trace" HOME="$scratch/home" &&
    for i in 1 2 3; do echo $i > f$i; git add f$i; git -c user.name=a -c user.email=a@e.co commit -q -s -m "T-1: $i"; done )
  sleep 3
  host status --porcelain >/dev/null
  host push -q origin t-1
  sleep 2
  m=$(cat "$d/worker.trace" "$d/host.trace" 2>/dev/null | grep '"event":"child_start"' | grep -c -E '"maintenance"|"gc"')
  if [ -d "$d/repo/.git/worktrees/planted" ]; then p=survived; else p=PRUNED; fi
  echo "$name: maintenance/gc child_start events=$m planted=$p"
  cat "$d/worker.trace" "$d/host.trace" 2>/dev/null | grep '"event":"child_start"' | grep -o '"argv":\[[^]]*\]' | sort | uniq -c | sed 's/^/    /'
}
PR_FLAGS="-c core.hooksPath=/dev/null -c core.fsmonitor=false -c log.showSignature=false -c maintenance.auto=false -c gc.auto=0"
OLD_FLAGS="-c core.hooksPath=/dev/null -c core.fsmonitor=false -c log.showSignature=false"
# 1: the PR's worker env and host flags.
variant pr "$envfile" "$PR_FLAGS"
# 2: pre-PR: keys 12 and 13 removed (COUNT=12) and the old host flags.
grep -v -E '_1[23]=|COUNT=' "$envfile" > "$scratch/base.env"; echo GIT_CONFIG_COUNT=12 >> "$scratch/base.env"
variant base "$scratch/base.env" "$OLD_FLAGS"
# 3: the PR's env and flags, but the repo's own config asks for every auto path.
variant pr-local-hostile "$envfile" "$PR_FLAGS" hostile
# 4: pre-PR with the hostile repo config, to show the hostile config is live.
variant base-local-hostile "$scratch/base.env" "$OLD_FLAGS" hostile
