# Probe harness for T-014b. Set S to a dir containing the waggle binary, then source.
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@e.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@e.com
W=$S/waggle
newrepo(){ R=$(mktemp -d $S/r.XXXX); cd $R; git init -q -b main .; mkdir -p docs other; echo a>README.md; echo x>other/x.md; echo y>docs/keep.md; git add -A; git commit -qm init; git checkout -q -b branch; }
claim(){ n=$1; gate=$2; shift 2; dirs=$(printf '"%s",' "$@"); dirs=${dirs%,}
 lease="{\"version\":2,\"ticket_id\":\"T-$(printf %03d $n)\",\"keeper_id\":\"k\",\"role\":\"forager\",\"gate\":\"$gate\",\"directories\":[$dirs],\"claimed_at\":\"2020-01-01T00:00:00Z\",\"expires_at\":\"2999-01-01T00:00:00Z\"}"
 b=$(printf %s "$lease" | git hash-object -w --stdin); t=$(printf '100644 blob %s\tlease.json\n' $b | git mktree); c=$(git commit-tree $t -m claim); git update-ref refs/waggle/claims/ticket-$n $c; }
idx(){ b=$(echo z | git hash-object -w --stdin); git update-index --add --cacheinfo 100644,$b,"$1"; }
gate(){ GITHUB_ACTIONS=${GHA:-} $W gate ${CHK:-verify} --base main --head branch --repo "$R"; echo "exit=$?"; }
