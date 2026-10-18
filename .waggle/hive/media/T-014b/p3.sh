S=/private/tmp/claude-redacted/waggle-scratch/34da4506-3988-4b28-8d4d-f2aaf4df6194/scratchpad
. $S/lib.sh
t(){ echo "=== $1"; }
t "claim .GitHub (case variant) accepted?"
( newrepo; claim 1 full .GitHub; idx .GitHub/Workflows/x.yml; git commit -qm "T-001: wf"; gate 2>&1 )
t "claim .github/actions"
( newrepo; claim 1 full .github/actions; idx .github/actions/evil/action.yml; git commit -qm "T-001: act"; gate 2>&1 )
t "claim .github/workflows/sub"
( newrepo; claim 1 full .github/workflows; idx .github/workflows/x.yml; git commit -qm "T-001: wf"; gate 2>&1 )
t "claim .devcontainer"
( newrepo; claim 1 full .devcontainer; idx .devcontainer/devcontainer.json; git commit -qm "T-001: dc"; gate 2>&1 )
t "claim .WAGGLE/HIVE"
( newrepo; claim 1 full .WAGGLE/HIVE; idx .WAGGLE/HIVE/x; git commit -qm "T-001: wh"; gate 2>&1 )
t "claim .waggle/hive/tasks"
( newrepo; claim 1 full .waggle/hive/tasks; idx .waggle/hive/tasks/T-099.md; git commit -qm "T-001: wh"; gate 2>&1 )
t "claim .waggle/hive/tasks light, Tier2 prefix"
( newrepo; claim 1 light .waggle/hive/tasks; idx .waggle/hive/tasks/T-099.md; git commit -qm "T-001: wh"; gate 2>&1 )
t "claim docs; edit with dotdot-like components / trailing"
( newrepo; claim 1 full docs; idx "docs/./a.md"; git commit -qm "T-001: dot" 2>&1; gate 2>&1 )
t "path .waggle/hive/../control/x under claim 'docs' (invalid tree entry) - tier fn"
cat > $S/tier_test.go <<'EOF'
EOF
t "GHA output for tier3 + path w/ colon and comma"
( newrepo; claim 1 full docs; idx "docs/a:b,c.md"; idx ".github/workflows/w,x:y.yml"; git commit -qm "T-001: x"; GHA=true gate 2>&1 )
t "light claim, subject 120 chars with CR LF and %"
( newrepo; claim 1 full docs; idx "docs/a.md"; git commit -qm "$(printf 'fix\r::error::pwned %%0A%s' $(head -c 200 /dev/zero | tr '\0' x))"; GHA=true gate 2>&1 | cat -v | head -5 )
t "multibyte subject truncation at 72 bytes"
( newrepo; claim 1 full docs; idx "docs/a.md"; git commit -qm "$(printf 'é%.0s' $(seq 1 60))"; GHA=true gate 2>&1 | cat -v | head -3 )
t "path longer than 4096 in head tree"
( newrepo; claim 1 full docs; p="docs/$(head -c 4200 /dev/zero | tr '\0' a)"; idx "$p"; git commit -qm "T-001: long"; GHA=true gate 2>&1 | cut -c1-200 )
t "invalid utf8 path in annotation"
( newrepo; claim 1 full docs; idx "other/$(printf '\xff\xfe')x"; git commit -qm "T-001: bad"; GHA=true gate 2>&1 | cat -v | head -4 )
