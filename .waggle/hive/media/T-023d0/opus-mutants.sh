#!/bin/sh
# Opus guard mutation check for T-023 phase D0: remove each fix, run its test, restore.
# Usage: sh opus-mutants.sh <repo root> <scratch dir>
set -u
cd "$1/internal/runtime" || exit 2
S=$2
cp claude.go launch.go policy.go "$S/"
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
m item16 claude.go 'cfg.DefaultArgs = slices.Clone(cfg.DefaultArgs)' '_ = slices.Clone[[]string]' 'TestConfigDoesNotAliasDefaultArgs$'
m item17 launch.go 'if !helpHas(out.buf.Bytes(), tok)' 'if !bytes.Contains(out.buf.Bytes(), []byte(tok))' 'TestVerifyHelpTokenBoundary$'
m item19-partial launch.go 'if (g.Name == "") != (g.Email == "") ||' 'if false && (g.Name == "") != (g.Email == "") ||' 'TestPrepareGitIdentity$'
m item19-ctrl launch.go 'Name+g.Email, func(r rune) bool { return r < 0x20 || r == 0x7f })' 'Name+g.Email, func(r rune) bool { return false })' 'TestPrepareGitIdentity$'
m item19-env launch.go '"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"' '"GIT_CONFIG_NOSYSTEM=1"' 'TestPrepareGitIdentity$'
m item19-zero launch.go 'if id != (GitIdentity{}) {' 'if true {' 'TestPrepareGitIdentity$'
m item20 launch.go 'if role == roleGuard && (cmd' 'if false && role == roleGuard && (cmd' 'TestGuardSettingsDropGitWrites$'
m item21 policy.go 'if task.SecondGuard {' 'if false && task.SecondGuard {' 'TestSecondGuard$'
for f in claude.go launch.go policy.go; do cmp -s "$f" "$S/$f" && echo "$f restored"; done
