S=/private/tmp/claude-redactedredactedredacted/waggle-scratch/34da4506-3988-4b28-8d4d-f2aaf4df6194/scratchpad
. $S/lib.sh
mk(){ # $1 ticket $2 total-bytes
  base="{\"version\":2,\"ticket_id\":\"T-$(printf %03d $1)\",\"keeper_id\":\"k\",\"role\":\"forager\",\"gate\":\"full\",\"directories\":[\"docs\"],\"claimed_at\":\"2020-01-01T00:00:00Z\",\"expires_at\":\"2999-01-01T00:00:00Z\"}"
  n=${#base}; pad=$(( $2 - n ))
  { printf %s "$base"; head -c $pad /dev/zero | tr '\0' ' '; } > $S/lease.$1
  b=$(git hash-object -w $S/lease.$1); t=$(printf '100644 blob %s\tlease.json\n' $b | git mktree); c=$(git commit-tree $t -m claim); git update-ref refs/waggle/claims/ticket-$1 $c; wc -c < $S/lease.$1; }
for sz in 65536 65537; do
echo "=== lease.json $sz bytes"
( newrepo; mk 1 $sz; echo a>docs/a.md; git add -A; git commit -qm "T-001: a"; gate 2>&1 )
done
echo "=== lease.json 300 MiB (sparse-ish whitespace)"
( newrepo; mk 1 300000000; echo a>docs/a.md; git add -A; git commit -qm "T-001: a"; /usr/bin/time -l env GITHUB_ACTIONS= $W gate leases --base main --head branch --repo "$R" 2>&1 | egrep "leases|maximum resident|real|exit" ; echo "exit=${PIPESTATUS[0]}" )
