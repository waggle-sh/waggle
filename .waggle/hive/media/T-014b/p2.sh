S=/private/tmp/claude-redacted/waggle-scratch/34da4506-3988-4b28-8d4d-f2aaf4df6194/scratchpad
. $S/lib.sh
echo "=== K2. two empty-subject commits, second edits outside lease"
( newrepo; claim 1 full docs
  echo a>docs/a.md; git add -A; git commit -q --allow-empty-message -m ""
  echo evil>other/evil.md; git add -A; git commit -q --allow-empty-message -m ""
  git log --format='%h [%s]' main..branch
  git rev-list --reverse --parents --format=%s main..branch | od -c | head
  gate 2>&1 )
echo "=== K3. empty then evil then good ticket commit"
( newrepo; claim 1 full docs
  echo a>docs/a.md; git add -A; git commit -q --allow-empty-message -m ""
  echo evil>other/evil.md; git add -A; git commit -q --allow-empty-message -m ""
  echo b>docs/b.md; git add -A; git commit -qm "T-001: ok"
  gate 2>&1 )
echo "=== K4. single empty-subject commit with body only"
( newrepo; claim 1 full docs
  echo a>docs/a.md; git add -A; git commit -q --allow-empty-message -F - <<<"

body only"
  git log --format='%h [%s]' main..branch
  gate 2>&1 )
