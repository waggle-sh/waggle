#!/bin/sh
# prune_dups probe for T-023 phase D0b. Usage: opus-prunedups.sh <scratch dir>
# A planted worktrees/<id> whose gitdir names the run's own worktree .git, with
# an id sorting before or after the run's id, then `git worktree prune` with the
# PR's host flags. Which admin dir survives?
set -u
s=$1
base() { env -i PATH="$PATH" HOME="$s/home" GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 "$@"; }
for planted in a zz; do
  d="$s/dups-$planted"; rm -rf "$d"; mkdir -p "$d" "$s/home"
  base git init -q -b main "$d/repo"
  base git -C "$d/repo" -c user.name=a -c user.email=a@e.co commit -q --allow-empty -m init
  base git -C "$d/repo" worktree add -q -b t-023 "$d/t-023" main
  mkdir "$d/repo/.git/worktrees/$planted"
  cp "$d/repo/.git/worktrees/t-023/gitdir" "$d/repo/.git/worktrees/$planted/gitdir"
  echo "ref: refs/heads/main" > "$d/repo/.git/worktrees/$planted/HEAD"
  echo "../.." > "$d/repo/.git/worktrees/$planted/commondir"
  base git -C "$d/repo" -c core.hooksPath=/dev/null -c maintenance.auto=false -c gc.auto=0 worktree prune -v 2>&1 | sed "s#$s#<tmp>#g"
  echo "planted=$planted left: $(ls "$d/repo/.git/worktrees" | tr '\n' ' ')"
  base git -C "$d/t-023" status --short >/dev/null 2>&1 && echo "  run worktree status: ok" || echo "  run worktree status: FAILS"
done
