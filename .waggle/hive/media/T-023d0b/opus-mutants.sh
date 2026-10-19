#!/bin/sh
# Fix-removal check for T-023 phase D0b. Run from the repo root at 1599c2b.
# Each mutant edits one source with perl, runs the tests, then restores it.
set -u
run() { # name file perl-expr count tests pkg
  name=$1 file=$2 expr=$3 count=$4 tests=$5 pkg=$6
  cp "$file" "$file.orig"
  perl -0pi -e "$expr" "$file"
  if cmp -s "$file" "$file.orig"; then echo "MUTANT $name: edit did not apply"; mv "$file.orig" "$file"; return; fi
  out=$(go test -count="$count" -run "$tests" "$pkg" 2>&1)
  echo "MUTANT $name: count=$count"
  printf '%s\n' "$out" | grep -E -- '--- FAIL|--- PASS|^ok|^FAIL' | sort | uniq -c
  mv "$file.orig" "$file"
}
run A-worker-keys internal/runtime/launch.go 's/,\n\t\t\[2\]string\{"maintenance.auto", "false"\}, \[2\]string\{"gc.auto", "0"\}\)/)/' 5 'TestRunPlantDuringAutoMaintenance$' ./internal/dispatch/
run A2-worker-no-maintenance-auto internal/runtime/launch.go 's/\[2\]string\{"maintenance.auto", "false"\}, //' 5 'TestRunPlantDuringAutoMaintenance$' ./internal/dispatch/
run A3-worker-no-gc-auto internal/runtime/launch.go 's/, \[2\]string\{"gc.auto", "0"\}//' 5 'TestRunPlantDuringAutoMaintenance$' ./internal/dispatch/
run A4-worker-keys-unit internal/runtime/launch.go 's/,\n\t\t\[2\]string\{"maintenance.auto", "false"\}, \[2\]string\{"gc.auto", "0"\}\)/)/' 1 'TestPrepareEnv$' ./internal/runtime/
run B-host-runin internal/dispatch/worktree.go 's/, "-c", "maintenance.auto=false", "-c", "gc.auto=0"\}/}/' 5 'TestRunPlantDuringAutoMaintenance$' ./internal/dispatch/
run B2-host-runin-unit internal/dispatch/worktree.go 's/, "-c", "maintenance.auto=false", "-c", "gc.auto=0"\}/}/' 1 'TestRunInDisablesAutoMaintenance$' ./internal/dispatch/
run C-comb-extraenv internal/dispatch/run.go 's/GIT_CONFIG_COUNT=3/GIT_CONFIG_COUNT=1/' 5 'TestRunPlantDuringAutoMaintenance$|TestRunInDisablesAutoMaintenance$' ./internal/dispatch/
run AB-worker-and-host internal/runtime/launch.go 's/,\n\t\t\[2\]string\{"maintenance.auto", "false"\}, \[2\]string\{"gc.auto", "0"\}\)/)/' 1 'TestRunInDisablesAutoMaintenance$' ./internal/dispatch/
