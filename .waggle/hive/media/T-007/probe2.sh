#!/bin/bash
WG=/tmp/claude-redactedredacted/wg
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=a GIT_AUTHOR_EMAIL=a@a GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a
W=$(mktemp -d /tmp/claude-redactedredacted/probeXXXX); cd $W; git init -q -b main .; mkdir -p .waggle/hive
D=.waggle/hive/DECISIONS.md
mk(){ i=$1; cat > in.json <<J
{"kind":"technical","title":"Decision $i","question":"Q $i?","options":["A: a","B: b"],"recommendation":"B","choice":"B: chosen $i","rationale":"Because $i.","tickets":["T-004"],"supersedes":[]}
J
$WG decide --file in.json >/dev/null || echo decide-fail; rm in.json; }
mk 1; mk 2; mk 3; git add -A; git commit -qm base
run(){ name=$1; shift; out=$("$WG" gate decisions "$@" 2>&1); rc=$?; echo "[$name] rc=$rc :: $out"; }
reset(){ git checkout -q -f main; git branch -q -D $1 2>/dev/null; git checkout -q -b $1; }
commit(){ git add -A; git commit -qm "$1"; }
grep -n '^## D-' $D
echo "== 2 delete last entry (D-003)"; reset p2; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); i=s.find('\n## D-003'); open(p,'w').write(s[:i]); 
P
commit del; git diff --stat main p2; run delete-last --base main --head p2
echo "== 2c delete last and append a replacement D-003"; reset p2c; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); i=s.find('\n## D-003'); open(p,'w').write(s[:i]+s[i:].replace('Decision 3','Decision 3 replaced'))
P
commit x; run replace-last --base main --head p2c
echo "== 2d delete last entry and append two new (net n>o)"; reset p2d; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); i=s.find('\n## D-003'); e=s[i:]
open(p,'w').write(s[:i]+e.replace('Because 3.','Because 3 edited.')+e.replace('D-003','D-004'))
P
commit y; run rewrite-last-plus-append --base main --head p2d
echo "== 3 insert middle (new D-003, old D-003 renumbered D-004)"; reset p3; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); i=s.find('## D-003'); j=s.find('## D-002')
ins=s[j:i].replace('D-002','D-003').replace('Decision 2','Decision ins')
open(p,'w').write(s[:i]+ins+s[i:].replace('D-003','D-004',1))
P
commit ins; git diff --stat main p3; run insert-middle --base main --head p3
echo "== 3b insert middle and renumber all subsequent = remove nothing"; 
echo "== same-length rewrite (D-002 body)"; reset p1; sed -i.bak 's/Because 2\./Because X\./' $D; rm $D.bak; git diff --stat main|tail -1; commit rw; run same-len-rewrite --base main --head p1
echo "== same-length rewrite plus append"; reset p1b; sed -i.bak 's/Because 2\./Because X\./' $D; rm $D.bak; mk 4; commit rw; run same-len-rewrite-plus-append --base main --head p1b
echo "== symlink"; reset p11; cp $D /tmp/claude-redactedredacted/real.md; git rm -q -f $D; mkdir -p .waggle/hive; ln -s /tmp/claude-redactedredacted/real.md $D; commit sym; git ls-tree HEAD -- $D; run symlink --base main --head p11
echo "== exec bit"; reset p13; chmod +x $D; commit exec; git ls-tree HEAD -- $D; run exec-bit --base main --head p13
echo "== dir at path"; reset p16; git rm -q -f $D; mkdir -p $D; echo x > $D/f; commit dir; run tree-at-path --base main --head p16
echo "== parent dir case variant"; reset p17; mkdir -p .Waggle/hive; cp $D .Waggle/hive/DECISIONS.md; commit pc; run parent-case --base main --head p17
echo "== exactly 4 MiB and 4MiB+1"; reset p18; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; b=open(p,'rb').read(); open(p,'wb').write(b+b'x'*((4<<20)-len(b)))
P
commit cap; wc -c $D; run exactly-cap --base main --head p18
echo "== base over cap"; reset p19; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; b=open(p,'rb').read(); open(p,'wb').write(b+b'x'*((5<<20)))
P
commit big; git checkout -q -b p19b; mk 9 2>/dev/null; echo y>>f; commit z; run base-oversize --base p19 --head p19b
echo "== duplicate-seq with identical content"; reset p7b; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); i=s.find('## D-003'); open(p,'w').write(s+'\n'+s[i:])
P
commit dup; run dup-seq --base main --head p7b
echo "== lower seq"; reset p7; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); i=s.find('## D-003'); open(p,'w').write(s+'\n'+s[i:].replace('D-003','D-002'))
P
commit low; run lower-seq --base main --head p7
echo "== moved base"; reset mb1; echo a>a; commit a; git checkout -q main; mk 4; commit m4; run moved-base --base main --head mb1; git reset -q --hard HEAD~1
echo "== extra blank line in new entry"; reset p20; mk 4; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); open(p,'w').write(s.replace('## D-004','\n## D-004'))
P
commit blank; run extra-blank --base main --head p20
echo "== trailing garbage line"; reset p21; mk 4; echo "stray" >> $D; commit st; run trailing-garbage --base main --head p21
echo "== truncated new entry"; reset p22; mk 4; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; s=open(p).read(); open(p,'w').write(s[:s.rfind('**Rationale')])
P
commit tr; run truncated --base main --head p22
echo "== head deleted file but also base-without"; 
echo "== shellout check: strace-less: grep"; grep -c 'exec.Command' /Users/example/code/waggle/.claude/worktrees/agent-a78428dff2da7fe63/cmd/waggle/gate.go
echo "== env GIT_DIR hostile"; out=$(GIT_DIR=/nonexistent "$WG" gate decisions --base main --head p20 2>&1); echo "rc=$? $out"
echo "== no-write check"; git checkout -q -f main; before=$(find .git -type f -newer $D | wc -l); "$WG" gate decisions --base main --head p20 >/dev/null 2>&1; git fsck --no-dangling 2>&1 | head -2; git status --porcelain
