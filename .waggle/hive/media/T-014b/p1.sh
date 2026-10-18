S=/private/tmp/claude-501/-Users-paul-code-waggle/34da4506-3988-4b28-8d4d-f2aaf4df6194/scratchpad
. $S/lib.sh
t(){ echo "=== $1"; }

t "A. copy from outside into lease (new file inside lease only)"
( newrepo; claim 1 full docs; cp other/x.md docs/copy.md; git add -A; git commit -qm "T-001: copy"; gate 2>&1 )

t "B. file with spaces / unicode lookalike dir 'dоcs' (cyrillic o)"
( newrepo; claim 1 full docs; mkdir -p "d$(printf '\xd0\xbe')cs"; echo q > "d$(printf '\xd0\xbe')cs/a.md"; git add -A; git commit -qm "T-001: lookalike"; gate 2>&1 )

t "C. path with newline in dir inside lease"
( newrepo; claim 1 full docs; idx "docs/a
b.md"; git commit -qm "T-001: nl"; gate 2>&1 )

t "D. dotdot entry inside lease (via mktree --missing)"
( newrepo; claim 1 full docs
  b=$(echo z | git hash-object -w --stdin)
  sub=$(printf '100644 blob %s\tgo.mod\n' $b | git mktree)
  dd=$(printf '040000 tree %s\t..\n' $sub | git mktree 2>&1); echo "mktree: $dd"
)

t "E. root commit in range (orphan branch unrelated history)"
( newrepo; claim 1 full docs; git checkout -q --orphan orph; git rm -rqf . ; mkdir docs; echo r>docs/r.md; git add -A; git commit -qm "T-001: root"; git branch -f branch orph; git checkout -q branch; gate 2>&1 )

t "F. submodule gitlink inside lease"
( newrepo; claim 1 full docs; git update-index --add --cacheinfo 160000,$(git rev-parse HEAD),docs/sub; git commit -qm "T-001: sub"; gate 2>&1 )

t "G. exec-bit/mode-only change inside lease (should pass)"
( newrepo; claim 1 full docs; chmod +x docs/keep.md; git add -A; git commit -qm "T-001: chmod"; gate 2>&1 )

t "H. type change file->symlink (old 100644 new 120000)"
( newrepo; claim 1 full docs; rm docs/keep.md; ln -s ../other/x.md docs/keep.md; git add -A; git commit -qm "T-001: typechg"; gate 2>&1 )

t "I. lease dir prefix confusion: claim 'doc', path 'docs/a'"
( newrepo; claim 1 full doc; echo n>docs/n.md; git add -A; git commit -qm "T-001: prefix"; gate 2>&1 )

t "J. merge commit then revert? (merge fails)"
( newrepo; claim 1 full docs; git checkout -q -b side; echo s>docs/s.md; git add -A; git commit -qm "T-001: s"; git checkout -q branch; echo b>docs/b.md; git add -A; git commit -qm "T-001: b"; git merge -q --no-ff -m "T-001: m" side; gate 2>&1 )

t "K. empty-subject commit"
( newrepo; claim 1 full docs; echo n>docs/n.md; git add -A; git commit -q --allow-empty-message -m ""; gate 2>&1 )

t "L. subject 'commit <sha>' lookalike and multi-line body"
( newrepo; claim 1 full docs; echo n>docs/n.md; git add -A; git commit -qm "commit $(git rev-parse HEAD)" -m "body
commit $(git rev-parse HEAD)"; gate 2>&1 )

t "M. subject containing format-ish: 'T-001: x%n%H'"
( newrepo; claim 1 full docs; echo n>docs/n.md; git add -A; git commit -qm 'T-001: x%n%H commit'; gate 2>&1 )

t "N. subject with CR only"
( newrepo; claim 1 full docs; echo n>docs/n.md; git add -A; git commit -qm "$(printf 'T-001: a\rcommit deadbeef')"; gate 2>&1 )

t "O. ticket T-0001 / T-001 variants: T-1000000 (7 digits) , 't-001: ' lowercase"
( newrepo; claim 1 full docs; echo n>docs/n.md; git add -A; git commit -qm "t-001: x"; gate 2>&1 )

t "P. claim for ticket 1 used by commit 'T-0001:' (4 digits, canonical?)"
( newrepo; claim 1 full docs; echo n>docs/n.md; git add -A; git commit -qm "T-0001: x"; gate 2>&1 )

t "Q. Tier2 CLAUDE.md in nested dir under light"
( newrepo; claim 1 light docs; echo n>docs/CLAUDE.md; git add -A; git commit -qm "T-001: x"; gate 2>&1 )

t "R. subject mimicking Head Keeper with ticket prefix: 'T-001: Head Keeper: x'"
( newrepo; claim 1 full docs; echo n>docs/n.md; git add -A; git commit -qm "T-001: Head Keeper: x"; gate 2>&1 )

t "S. lease dirs trailing slash / dot-segments are normalized?"
( newrepo; claim 1 full "docs/../other"; echo n>other/n.md; git add -A; git commit -qm "T-001: x"; gate 2>&1 )

t "T. claim dir '.'"
( newrepo; claim 1 full "."; echo n>go.mod; git add -A; git commit -qm "T-001: x"; gate 2>&1 )
