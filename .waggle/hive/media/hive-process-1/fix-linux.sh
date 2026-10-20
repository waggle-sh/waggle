#!/bin/sh
# Fix-round Linux run (golang:1.23-bookworm: mawk, dash as sh, GNU grep, sed
# and stat). The repo is cloned from a read-only mount of the main checkout;
# `go` is a stub that exits 0, so the decisions gate is covered on macOS only.
# HOME is a fixture directory so the home-component name rule does not turn
# "root" into a name. Usage (in the container): fix-linux.sh <head sha>
set -u
H=$1
mkdir -p /stub /tmp/fixturehome; printf '#!/bin/sh\nexit 0\n' >/stub/go; chmod +x /stub/go
export PATH=/stub:$PATH HOME=/tmp/fixturehome
echo "awk: $(readlink -f "$(command -v awk)")  sh: $(readlink -f /bin/sh)  grep: $(grep --version | head -1)  stat: $(stat --version | head -1)"
git config --global --add safe.directory '*'
git clone -q /src /w && cd /w && git checkout -q --detach "$H"
echo "=== scan on the branch (base 2ecd449) ==="
.waggle/control/scripts/scan-hive.sh 2ecd449 HEAD | sed -n '/^decisions gate/,$p'; echo "exit $?"
echo "=== guard planted probes ==="
bash .waggle/hive/media/hive-process-1/guard-plant-probe.sh "$H"
echo "=== fix-round probes ==="
bash /probe/fix-new-probe.sh "$H"
echo "=== hooks under dash and bash, GNU stat ==="
mkdir -p /hk && cd /hk && git init -q -b main && git -c user.name=t -c user.email=t@example.test commit -q --allow-empty -m init && mkdir -p .waggle/hive/handoff
sh /w/.waggle/hive/media/hive-process-1/guard-hooks-probe.sh /w /hk
