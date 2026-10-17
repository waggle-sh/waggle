#!/bin/bash
WG=/tmp/claude-501/wg
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=a GIT_AUTHOR_EMAIL=a@a GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a
W=$(mktemp -d /tmp/claude-501/probeXXXX); cd $W; git init -q -b main .; mkdir -p .waggle/hive
D=.waggle/hive/DECISIONS.md
mk(){ i=$1; cat > in.json <<J
{"kind":"technical","title":"Decision $i","question":"Q $i?","options":["A: a","B: b"],"recommendation":"B","choice":"B: chosen $i","rationale":"Because $i.","tickets":["T-004"],"supersedes":[]}
J
$WG decide --file in.json >/dev/null || echo decide-fail; rm in.json; }
mk 1; mk 2; git add -A; git commit -qm base
run(){ name=$1; shift; out=$("$WG" gate decisions "$@" 2>&1); rc=$?; echo "[$name] rc=$rc :: $out"; }
echo "== base over cap (base 5MiB valid-looking prefix, head differs)"; git checkout -q -b big; python3 -c "
p='$D'; b=open(p,'rb').read(); open(p,'wb').write(b+b'x'*(5<<20))"; git commit -qam big; git checkout -q -b big2; echo y >> $D; git commit -qam big2; run base-oversize --base big --head big2
echo "== parent dir case variant via update-index"; git checkout -q main; git checkout -q -b pc; h=$(git hash-object -w $D); git update-index --add --cacheinfo 100644,$h,.Waggle/hive/DECISIONS.md; git commit -qm pc; run parent-case --base main --head pc
echo "== base lacks file, head adds with mid-gap"; git checkout -q --orphan o2; git rm -rfq .; echo r>r; git add r; git commit -qm r; git checkout -q -b o3; mkdir -p .waggle/hive; git show main:$D > $D; echo >> $D; git add -A; git commit -qm add; run create-trailing-blank --base o2 --head o3
echo "== git status clean / fsck"; git fsck 2>&1 | head -3; git status --porcelain | head
