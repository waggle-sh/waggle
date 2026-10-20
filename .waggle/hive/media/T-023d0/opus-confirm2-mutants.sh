#!/bin/sh
# Opus guard second confirmation at d969d57: remove each D-180 change or C3 property, run the PR's tests, restore.
# Usage: sh opus-confirm2-mutants.sh <repo root> <scratch dir>
set -u
cd "$1/internal/runtime" || exit 2
S=$2
cp policy.go "$S/"
m() {
	name=$1 from=$2 to=$3 run=$4
	cp "$S/policy.go" policy.go
	FROM="$from" TO="$to" perl -0pi -e 's/\Q$ENV{FROM}\E/$ENV{TO}/' policy.go
	if cmp -s policy.go "$S/policy.go"; then
		echo "$name: MUTATION NOT APPLIED"
	else
		echo "$name: $(go test -v -count=1 -run "$run" . 2>&1 | grep -E '(--- FAIL|^ok|^FAIL\s)' | sed 's/ *(0.00s)//' | sed 's/^ *//' | tr '\n' ' ')"
	fi
	cp "$S/policy.go" policy.go
}
B='l >= max(LevelFast, p.Minimums[roleGuard]) && model == ""'
m d180-unbounded "$B" 'l >= LevelFast && model == ""' 'TestSecondGuard'
m d180-bound-exclusive "$B" 'l > max(LevelFast, p.Minimums[roleGuard]) && model == ""' 'TestSecondGuard'
m d180-bound-builder-min "$B" 'l >= max(LevelFast, p.Minimums[roleBuilder]) && model == ""' 'TestSecondGuard'
m c3-alt-before-primary '[]string{p.Models.Primary[l], p.Models.Alternate[l]} {
				if m != "" && !slices.Contains(avoid, m) {
					model, level, err' '[]string{p.Models.Alternate[l], p.Models.Primary[l]} {
				if m != "" && !slices.Contains(avoid, m) {
					model, level, err' 'TestSecondGuardWalk$'
m c3-ignore-avoid 'if m != "" && !slices.Contains(avoid, m) {
					model, level, err' 'if m != "" {
					model, level, err' 'TestSecondGuardWalk$'
m c3-exhaustion-swallowed '		for l := level - 1; l >= max' '		err = nil
		for l := level - 1; l >= max' 'TestSecondGuardWalk$'
m c3-exhaustion-any-model 'if m != "" && !slices.Contains(avoid, m) {
					model, level, err' 'if m != "" && (!slices.Contains(avoid, m) || l == LevelFast) {
					model, level, err' 'TestSecondGuardWalk$'
m d179-walk-removed 'if err != nil && task.SecondGuard && role == roleGuard && errors.Is(err, ErrNoCrossModel) {' 'if false {' 'TestSecondGuard'
cmp -s policy.go "$S/policy.go" && echo "policy.go restored"
