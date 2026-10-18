S=/private/tmp/claude-501/-Users-paul-code-waggle/34da4506-3988-4b28-8d4d-f2aaf4df6194/scratchpad
cd $S/$(ls -t $S | grep '^r\.' | head -1)
echo "--- cat-file --batch (what loadClaims runs)"
echo "refs/waggle/claims/ticket-1:lease.json" | /usr/bin/time -l git cat-file --batch 2>&1 >/dev/null | egrep "maximum resident|real"
echo "--- cat-file --batch-check"
echo "refs/waggle/claims/ticket-1:lease.json" | /usr/bin/time -l git cat-file --batch-check 2>&1 | egrep "lease|maximum resident|real"
echo "--- waggle alone (leases)"
/usr/bin/time -l $S/waggle gate leases --base main --head branch --repo . 2>&1 | egrep "maximum resident|real"
