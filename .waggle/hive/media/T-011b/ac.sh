set -u
S=/private/tmp/claude-redacted/waggle-scratch/56d2a18d-df7f-4cdd-bc5f-42eb57375762/scratchpad
W=$S/waggle
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
rm -rf $S/r; mkdir $S/r; cd $S/r
git init -q .; git config user.email a@b; git config user.name a
printf '# proj\n' > README.md; printf 'node_modules\n' > .gitignore; printf 'my notes\n' > CLAUDE.md
mkdir -p .claude/agents; printf 'x\n' > .claude/agents/foo.md
git add -A; git commit -qm base; BASE=$(git rev-parse HEAD)
git update-ref refs/waggle/claims/x HEAD; git for-each-ref > $S/refs-before.txt
$W init --runtime=claude-code; git add -A; git commit -qm init
echo "== status after init commit"; git status --short
echo "== dry run"; $W uninstall; echo "exit=$?"; git status --short | wc -l
echo "== apply"; $W uninstall --apply; echo "exit=$?"
echo "== status"; git status --short
echo "== diff vs base (stat)"; git diff $BASE --stat; git diff --quiet $BASE; echo "diffexit=$?"
echo "== status ignored"; git status --short --ignored
ls -a . .claude 2>&1
echo "== refs"; git for-each-ref | diff - $S/refs-before.txt && echo refs-same
