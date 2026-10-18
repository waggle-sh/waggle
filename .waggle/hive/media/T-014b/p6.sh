S=/private/tmp/claude-redacted/waggle-scratch/34da4506-3988-4b28-8d4d-f2aaf4df6194/scratchpad
. $S/lib.sh
echo "=== dotdot tree entry inside lease: docs/../other/evil (crafted with mktree; GitHub receive fsck would reject)"
( newrepo; claim 1 full docs
  b=$(echo z | git hash-object -w --stdin)
  other=$(printf '100644 blob %s\tevil\n' $b | git mktree)
  dd=$(printf '040000 tree %s\t..\n' $other | git mktree)
  docs=$(printf '040000 tree %s\t..\n100644 blob %s\tkeep.md\n' $other $b | git mktree)
  root=$(printf '040000 tree %s\tdocs\n100644 blob %s\tREADME.md\n' $docs $b | git mktree)
  c=$(git commit-tree $root -p HEAD -m "T-001: dd"); git update-ref refs/heads/branch $c
  git diff-tree -r --raw -z --no-renames main branch | tr '\0' '\n'
  gate 2>&1 )
echo "=== .git component inside lease: docs/.git/config"
( newrepo; claim 1 full docs; idx docs/.git/config; git commit -qm "T-001: g" 2>&1; gate 2>&1 )
echo "=== 501 commits"
( newrepo; claim 1 full docs; git commit -q --allow-empty -m "T-001: e"; for i in $(seq 1 500); do git commit -q --allow-empty -m "T-001: e$i"; done; gate 2>&1 )
echo "=== 500 commits"
( newrepo; claim 1 full docs; for i in $(seq 1 500); do git commit -q --allow-empty -m "T-001: e$i"; done; gate 2>&1 )
echo "=== 5001 paths in one commit"
( newrepo; claim 1 full docs; for i in $(seq 1 5001); do echo "100644 $(echo z | git hash-object -w --stdin)	docs/f$i"; done | git update-index --index-info; git commit -qm "T-001: many"; gate 2>&1 )
echo "=== 'verify' with leases error and tiers error both printed? exit 3 for git error"
( newrepo; claim 1 full docs; echo a>docs/a; git add -A; git commit -q --allow-empty-message -m ""; gate 2>&1 )
echo "=== GHA mode, empty-subject error line"
( newrepo; claim 1 full docs; echo a>docs/a; git add -A; git commit -q --allow-empty-message -m ""; GHA=true gate 2>&1 )
