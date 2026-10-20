#!/bin/sh
# Confirmation Linux run for hive-process-1 at 5c7ada6 (golang:1.23-bookworm:
# mawk, dash as sh, GNU grep, sed and stat). The repo is cloned from a
# read-only mount; `go` is a stub that exits 0, so the decisions gate is
# covered on macOS only. HOME is a fixture directory.
# Usage (in the container): confirm-linux.sh <head sha>
set -u
H=$1
mkdir -p /stub /tmp/fixturehome; printf '#!/bin/sh\nexit 0\n' >/stub/go; chmod +x /stub/go
export PATH=/stub:$PATH HOME=/tmp/fixturehome
echo "awk: $(readlink -f "$(command -v awk)")  sh: $(readlink -f /bin/sh)  stat: $(stat --version | head -1)"
git config --global --add safe.directory '*'
git clone -q /src /w && cd /w && git checkout -q --detach "$H"
echo "=== confirm-scan-probe.sh ==="
bash /probe/confirm-scan-probe.sh "$H" /w/.waggle/control/scripts/scan-hive.sh
git checkout -q --detach "$H"; rm -rf .waggle/hive/media/confirm-plant
echo "=== hooks under dash and bash, GNU stat ==="
mkdir -p /hk && cd /hk && git init -q -b main && git -c user.name=t -c user.email=t@example.test commit -q --allow-empty -m init && mkdir -p .waggle/hive/handoff
sh /w/.waggle/hive/media/hive-process-1/guard-hooks-probe.sh /w /hk
echo "=== stat order directly ==="
f=/hk/.waggle/hive/handoff/queen.local.md; echo x >"$f"; touch -t 202001010000 "$f"
echo "new order: $(stat -c %Y "$f" 2>/dev/null || stat -f %m "$f" 2>/dev/null)"
echo "old order: $(stat -f %m "$f" 2>/dev/null || stat -c %Y "$f" 2>/dev/null | tr '\n' ' ')" | tr '\n' ' '; echo
