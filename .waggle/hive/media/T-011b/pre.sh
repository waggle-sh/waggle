set -u
S=/private/tmp/claude-501/-Users-paul-code-waggle/56d2a18d-df7f-4cdd-bc5f-42eb57375762/scratchpad
W=$S/waggle
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
mk() {
  rm -rf $S/p; mkdir $S/p; cd $S/p
  git init -q .; git config user.email a@b; git config user.name a
  printf '# proj\n' > README.md
  git add -A; git commit -qm base
  $W init --runtime=claude-code >/dev/null; git add -A; git commit -qm init
}
snap() { (find . -path ./.git -prune -o -type f -print0 | sort -z | xargs -0 shasum; git status --porcelain) | shasum; }
try() {
  name=$1; shift
  before=$(snap)
  $W uninstall "$@" > $S/out.txt 2> $S/err.txt; rc=$?
  after=$(snap)
  [ "$before" = "$after" ] && same=nochange || same=CHANGED
  echo "$name: exit=$rc $same | $(head -c 200 $S/err.txt | tr '\n' ' ')"
}
echo "# dirty tree"; mk; echo x >> README.md; try dirty-dry; try dirty-apply --apply
echo "# untracked"; mk; echo x > new.txt; try untracked --apply
echo "# legacy"; rm -rf $S/p; mkdir $S/p; cd $S/p; git init -q .; git config user.email a@b; git config user.name a; mkdir -p .waggle/roles; echo "x: 1" > .waggle/config.yaml; echo r > .waggle/roles/a.md; git add -A; git commit -qm legacy; try legacy --apply
echo "# no waggle at all"; rm -rf $S/p; mkdir $S/p; cd $S/p; git init -q .; git config user.email a@b; git config user.name a; echo a > a; git add -A; git commit -qm a; try none --apply
echo "# run.sock"; mk; touch .waggle/control/run.sock; try sock --apply
echo "# unparseable block"; mk; printf '# --- waggle:start ---\nfoo\n' >> .gitignore; git commit -qam bad; try badblock --apply
echo "# two blocks in CLAUDE.md"; mk; cat CLAUDE.md > x.tmp; cat x.tmp >> CLAUDE.md; rm x.tmp; git commit -qam dup; try dupblock --apply
echo "# subdir"; mk; mkdir sub; cd sub; try subdir --apply; cd ..
echo "# not a repo"; rm -rf $S/q; mkdir $S/q; cd $S/q; try norepo --apply
echo "# usage"; cd $S; $W uninstall --bogus >/dev/null 2>&1; echo "bogus exit=$?"; $W uninstall extra >/dev/null 2>&1; echo "stray exit=$?"; $W uninstall -h >/dev/null 2>&1; echo "-h exit=$?"
echo "# unmarked workflow + project agent survive"; mk; sed -i '' '/waggle:managed/d' .github/workflows/waggle-gate.yml; git commit -qam unmark; printf 'y\n' > .claude/agents/foo.md; git add -A; git commit -qm foo; $W uninstall --apply | grep -E 'skip|warn'; ls .github/workflows .claude/agents; git status --short
