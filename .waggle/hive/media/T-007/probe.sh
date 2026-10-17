#!/bin/bash
# usage: probe.sh ; prints evidence
WG=/tmp/claude-redactedredacted/wg
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=a GIT_AUTHOR_EMAIL=a@a GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a
W=$(mktemp -d /tmp/claude-redactedredacted/probeXXXX); cd $W; git init -q -b main .; mkdir -p .waggle/hive
D=.waggle/hive/DECISIONS.md
mk(){ i=$1; cat > in.json <<J
{"kind":"technical","title":"Decision $i","question":"Q $i?","options":["A: a","B: b"],"recommendation":"B","choice":"B: chosen $i","rationale":"Because $i.","tickets":["T-004"],"supersedes":[]}
J
$WG decide --file in.json >/dev/null || echo decide-fail; rm in.json; }
mk 1; mk 2; mk 3; git add -A; git commit -qm base; BASE=$(git rev-parse HEAD)
cp $D $W/../base.md 2>/dev/null
run(){ name=$1; shift; out=$("$WG" gate decisions "$@" 2>&1); rc=$?; echo "[$name] rc=$rc :: $out"; }
reset(){ git checkout -q -f main; git branch -q -D $1 2>/dev/null; git checkout -q -b $1; }
commit(){ git add -A; git commit -qm "$1"; }
echo "== baseline"; reset b0; run noop --base main --head b0
reset b1; mk 4; commit app; run valid-append --base main --head b1
echo "== 1 same-length rewrite"; reset p1; sed -i.bak 's/Because 2\./Because X\./' $D; rm $D.bak; cmp -s $D <(git show main:$D) ; wc -c $D <(git show main:$D)|head -2; commit rw; run same-len-rewrite --base main --head p1
echo "== 2 delete last entry"; reset p2; python3 - <<'P'
import re
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); i=s.rfind('\n**D-003'); open(p,'w').write(s[:i] if i>0 else s)
P
tail -c 80 $D | od -c | tail -3; commit del; run delete-last --base main --head p2
echo "== 2b delete last entry and append new D-003 differing"; reset p2b; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); open(p,'w').write(s.replace('Because 3.','Because 3 changed.'))
P
commit x; run rewrite-last-longer --base main --head p2b
echo "== 3 insert middle"; reset p3; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); i=s.find('**D-003'); 
ins=s[s.find('**D-002'):i].replace('D-002','D-003').replace('Decision 2','Decision ins')
s2=s[:i]+ins+s[i:].replace('**D-003','**D-004',1)
open(p,'w').write(s2)
P
commit ins; run insert-middle --base main --head p3
echo "== 4 CRLF head"; reset p4; mk 4; sed -i.bak 's/$/\r/' $D; rm $D.bak; commit crlf; run crlf-head --base main --head p4
echo "== 4b CRLF only on appended entry"; reset p4b; mk 4; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; b=open(p,'rb').read(); i=b.rfind(b'\n**D-004'); open(p,'wb').write(b[:i]+b[i:].replace(b'\n',b'\r\n'))
P
commit crlf2; run crlf-new-entry --base main --head p4b
echo "== 5 corrupt base"; git checkout -q -f main; git checkout -q -b cb; truncate -s -1 $D; commit corruptbase; CB=cb
git checkout -q -b cbhead; mk 4 2>/dev/null; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; b=open(p,'rb').read(); open(p,'wb').write(b+b'\n')
P
commit h; run corrupt-base --base cb --head cbhead
echo "== 5b corrupt base, unrelated head change"; git checkout -q -b cbh2 cb; echo hi>f; sed -i.bak 's/Because 1\./Because 1!/' $D; rm $D.bak; commit h2; run corrupt-base-edit --base cb --head cbh2
echo "== 6 over MaxLogBytes"; reset p6; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; open(p,'ab').write(b'x'*(5<<20))
P
commit big; run oversize --base main --head p6
echo "== 6b exactly at cap+1 head,"; 
echo "== 7 lower seq"; reset p7; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); i=s.find('**D-003'); e=s[i:]
open(p,'w').write(s+'\n'+e.replace('D-003','D-002'))
P
commit low; run lower-seq --base main --head p7
echo "== 7b duplicate seq D-003"; reset p7b; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); i=s.find('**D-003'); e=s[i:]
open(p,'w').write(s+'\n'+e)
P
commit dup; run dup-seq --base main --head p7b
echo "== 7c gap D-005"; reset p7c; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); i=s.find('**D-003'); e=s[i:]
open(p,'w').write(s+'\n'+e.replace('D-003','D-005'))
P
commit gap; run gap-seq --base main --head p7c
echo "== 8 --repo non-repo / subdir / missing"; N=$(mktemp -d /tmp/claude-redactedredacted/nonrepoXXXX); (cd /; run nonrepo --repo $N --base main --head b1)
cd $W; run subdir --repo $W/.waggle/hive --base main --head b1
run missing-repo --repo $W/nope --base main --head b1
echo "== 9 base ref missing"; run nosuchref --base nosuchref --head b1; run nosuchhead --base main --head nosuch
run opt-inject --base --upload-pack=x --head b1; run opt-inject2 --base=-x --head b1; run ws --base "main b1" --head b1; run empty --base "" --head b1; run nohead --base main; run extra --base main --head b1 extra
run unknown-check; $WG gate frobnicate >/dev/null 2>&1; echo "[frobnicate] rc=$?"; $WG gate >/dev/null 2>&1; echo "[bare] rc=$?"; $WG gate decisions -h >/dev/null 2>&1; echo "[-h] rc=$?"; $WG gate decisions --bogus >/dev/null 2>&1; echo "[--bogus] rc=$?"
echo "== other"
git checkout -q -f main; git checkout -q -b p10; git rm -q $D; commit rm; run deleted --base main --head p10
git checkout -q -f main; git checkout -q -b p11; ln -sf /dev/null x; cp $D real; git rm -q -f $D; ln -s real $D; commit sym; run symlink --base main --head p11
git checkout -q -f main; git checkout -q -b p12; h=$(git hash-object -w $D); git update-index --add --cacheinfo 100644,$h,.waggle/hive/decisions.md; git commit -qm case; run casevariant --base main --head p12
git checkout -q -f main; git checkout -q -b p13; chmod +x $D; commit exec; run exec-bit --base main --head p13
git checkout -q -f main; git checkout -q -b p14; git rm -q $D; commit rm;  git checkout -q -f main; echo "dirty" >> $D; run dirty-tree --base main --head main; git checkout -q -f main
git checkout -q --orphan orph; git rm -rfq . ; echo x>y; git add y; git commit -qm o; run unrelated --base main --head orph
git checkout -q -f main; git checkout -q -b p15; git rm -q -f $D; commit del2; git checkout -q main; git checkout -q -b newfile main~0; 
# file created on branch where base lacks it
git checkout -q --orphan fresh; git rm -rfq . 2>/dev/null; echo r>r; git add r; git commit -qm r; git checkout -q -b fr2; mkdir -p .waggle/hive; git show main:$D > $D; commit add; run file-created --base fresh --head fr2
echo "== GITHUB_ACTIONS annotation"; out=$(GITHUB_ACTIONS=true $WG gate decisions --base main --head p1 2>&1); echo "rc=$? $out"
echo "== crash on git missing"; out=$(PATH=/nonexistent $WG gate decisions --base main --head b1 2>&1); echo "rc=$? $out"
echo "== unborn/ wrong HEAD ref names"; run branchnamedash --base main --head 'refs/heads/b1'
echo "== write side effects check"; git status --porcelain | head; ls .git | head -30
echo $W
