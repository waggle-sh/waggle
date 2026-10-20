#!/bin/sh
# Opus guard confirmation at ebce792: remove each fix-round change, run the PR's tests, restore.
# Usage: sh opus-confirm-mutants.sh <repo root> <scratch dir>
set -u
cd "$1/internal/runtime" || exit 2
S=$2
cp launch.go policy.go "$S/"
m() {
	name=$1 file=$2 from=$3 to=$4 run=$5
	cp "$S/$file" "$file"
	FROM="$from" TO="$to" perl -0pi -e 's/\Q$ENV{FROM}\E/$ENV{TO}/' "$file"
	if cmp -s "$file" "$S/$file"; then
		echo "$name: MUTATION NOT APPLIED"
	else
		echo "$name: $(go test -count=1 -run "$run" . 2>&1 | tail -1)"
	fi
	cp "$S/$file" "$file"
}
ID='TestPrepareGitIdentity$'
m id-blank-trim launch.go '(strings.TrimSpace(g.Name) == "") != (strings.TrimSpace(g.Email) == "")' '(g.Name == "") != (g.Email == "")' "$ID"
m id-utf8 launch.go '|| !utf8.ValidString(g.Name+g.Email) ||' '|| false ||' "$ID"
m id-iscontrol launch.go 'return unicode.IsControl(r) ||' 'return r < 0x20 || r == 0x7f ||' "$ID"
m id-u2028 launch.go '|| r == 0x2028 ||' '||' "$ID"
m id-u2029 launch.go '|| r == 0x2029 })' '})' "$ID"
m env-blank-guard launch.go 'if strings.TrimSpace(id.Name) != "" {' 'if id != (GitIdentity{}) {' "$ID"
m env-attr-nosystem launch.go ', "GIT_ATTR_NOSYSTEM=1"}' '}' 'TestPrepareEnv$'
m env-excludesfile launch.go '[2]string{"core.excludesFile", "/dev/null"}, ' '' 'TestEnvIgnoresHomeIgnoreAndAttributes$'
m env-attributesfile launch.go ', [2]string{"core.attributesFile", "/dev/null"})' ')' 'TestEnvIgnoresHomeIgnoreAndAttributes$'
m env-count-stale launch.go '"GIT_CONFIG_COUNT=" + strconv.Itoa(len(keys))' '"GIT_CONFIG_COUNT=" + strconv.Itoa(len(pushBlocked))' 'TestPrepareEnv$|TestEnvIgnoresHomeIgnoreAndAttributes$'
P=.
m d179-walk-removed policy.go 'if err != nil && task.SecondGuard && role == roleGuard && errors.Is(err, ErrNoCrossModel) {' 'if false {' "$P"
m d179-any-guard policy.go 'if err != nil && task.SecondGuard && role == roleGuard &&' 'if err != nil && role == roleGuard &&' "$P"
m d179-level-not-reported policy.go 'model, level, err = m, l, nil' 'model, err = m, nil' "$P"
m d179-lowest-first policy.go 'for l := level - 1; l >= LevelFast && model == ""; l-- {' 'for l := LevelFast; l < level && model == ""; l++ {' "$P"
m d179-alt-before-primary policy.go '[]string{p.Models.Primary[l], p.Models.Alternate[l]} {
				if m != "" && !slices.Contains(avoid, m) {
					model, level, err' '[]string{p.Models.Alternate[l], p.Models.Primary[l]} {
				if m != "" && !slices.Contains(avoid, m) {
					model, level, err' "$P"
m d179-ignore-avoid policy.go 'if m != "" && !slices.Contains(avoid, m) {
					model, level, err' 'if m != "" {
					model, level, err' "$P"
for f in launch.go policy.go; do cmp -s "$f" "$S/$f" && echo "$f restored"; done
