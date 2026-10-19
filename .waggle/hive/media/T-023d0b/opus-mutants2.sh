#!/bin/sh
# Does the comb or host half matter? Mutants against the opus probe
# TestOpusPlantNoGitdir (no gitdir plant after the result line, no later worker
# commit). Run from the repo root at 1599c2b with opus-prunedups-inrun_test.go.txt
# installed as internal/dispatch/opus_probe_test.go.
set -u
T='TestOpusPlantNoGitdir$'
mut() { # name file perl-expr
  cp "$2" "$2.orig"; perl -0pi -e "$3" "$2"
  cmp -s "$2" "$2.orig" && echo "MUTANT $1: edit did not apply"
}
restore() { mv "$1.orig" "$1"; }
report() { echo "$1: $(go test -count=10 -run "$T" ./internal/dispatch/ 2>&1 | grep -c -- '--- FAIL: TestOpusPlantNoGitdir') of 10 failed"; }
report none
mut C internal/dispatch/run.go 's/GIT_CONFIG_COUNT=3/GIT_CONFIG_COUNT=1/'; report C-comb; restore internal/dispatch/run.go
mut A internal/runtime/launch.go 's/,\n\t\t\[2\]string\{"maintenance.auto", "false"\}, \[2\]string\{"gc.auto", "0"\}\)/)/'; report A-worker; restore internal/runtime/launch.go
mut B internal/dispatch/worktree.go 's/, "-c", "maintenance.auto=false", "-c", "gc.auto=0"\}/}/'; report B-host; restore internal/dispatch/worktree.go
mut C internal/dispatch/run.go 's/GIT_CONFIG_COUNT=3/GIT_CONFIG_COUNT=1/'
mut B internal/dispatch/worktree.go 's/, "-c", "maintenance.auto=false", "-c", "gc.auto=0"\}/}/'; report BC-host-and-comb
restore internal/dispatch/worktree.go; restore internal/dispatch/run.go
