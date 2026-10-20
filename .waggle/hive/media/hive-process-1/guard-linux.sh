#!/bin/sh
# Runs the scan and hook probes in a Debian container (mawk, GNU grep, GNU
# sed, GNU stat, dash as sh). The repo is cloned from a read-only mount; `go`
# is a stub that exits 0 so only the shell and awk logic is under test (the
# decisions gate is covered on macOS).
set -u
mkdir -p /stub; printf '#!/bin/sh\nexit 0\n' >/stub/go; chmod +x /stub/go
export PATH=/stub:$PATH
apt-get >/dev/null 2>&1 || true
echo "awk: $(readlink -f "$(command -v awk)")  sh: $(readlink -f /bin/sh)  grep: $(grep --version | head -1)"
git config --global --add safe.directory '*'
git clone -q /src /w && cd /w && git checkout -q --detach 5e1c79d
echo "=== scan on the branch (base origin/main) ==="
.waggle/control/scripts/scan-hive.sh origin/main HEAD | sed -n '/^decisions gate/,$p'; echo "exit $?"
echo "=== planted probes ==="
SKIP_LONG=1 bash /probe/plant-probe.sh 5e1c79d
echo "=== hooks under dash and bash, GNU stat ==="
mkdir -p /hk && cd /hk && git init -q -b main && git -c user.name=t -c user.email=t@example.test commit -q --allow-empty -m init && mkdir -p .waggle/hive/handoff
sh /probe/hooks-probe.sh /w /hk
