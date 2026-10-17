#!/bin/bash
WG=/tmp/claude-redactedredacted/wg
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=a GIT_AUTHOR_EMAIL=a@a GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a
W=$(mktemp -d /tmp/claude-redactedredacted/probeXXXX); cd $W; git init -q -b main .; mkdir -p .waggle/hive
D=.waggle/hive/DECISIONS.md
mk(){ i=$1; cat > in.json <<J
{"kind":"technical","title":"Decision $i","question":"Q $i?","options":["A: a","B: b"],"recommendation":"B","choice":"B: chosen $i","rationale":"Because $i.","tickets":["T-004"],"supersedes":[]}
J
$WG decide --file in.json >/dev/null || echo decide-fail; rm in.json; }
mk 1; mk 2; git add -A; git commit -qm base
run(){ name=$1; shift; out=$("$WG" gate decisions "$@" 2>&1); rc=$?; echo "[$name] rc=$rc :: $out"; }
git checkout -q -b c1; mk 3; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; b=open(p,'rb').read(); i=b.rfind(b'## D-003'); open(p,'wb').write(b[:i]+b[i:].replace(b'\n',b'\r\n'))
P
git commit -qam crlf; git diff --stat main | tail -1; run crlf-new-entry --base main --head c1
git checkout -q main; git checkout -q -b c2; mk 3; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; b=open(p,'rb').read(); open(p,'wb').write(b.replace(b'\n',b'\r\n'))
P
git commit -qam crlfall; run crlf-whole-file --base main --head c2
git checkout -q main; git checkout -q -b c3; mk 3; python3 - <<'P'
p='.waggle/hive/DECISIONS.md'; b=open(p,'rb').read(); open(p,'wb').write(b.replace(b'Decision 3',b'Decision 3\r',1))
P
git commit -qam cr; run cr-in-title --base main --head c3
