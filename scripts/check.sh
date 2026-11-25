#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# check.sh: run the CI checks locally, optionally across the release matrix.
#
# Usage: scripts/check.sh [--quick] [--cross]
#   --quick  skip the race tests
#   --cross  also cross-build (and vet darwin and windows) for the release matrix
set -eu

quick=0
cross=0
for arg in "$@"; do
	case "$arg" in
	--quick) quick=1 ;;
	--cross) cross=1 ;;
	-h | --help)
		sed -n '3,7p' "$0" | sed 's/^# \{0,1\}//'
		exit 0
		;;
	*)
		echo "check.sh: unknown argument: $arg" >&2
		exit 2
		;;
	esac
done

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

step=""
trap 'rc=$?; if [ "$rc" -ne 0 ] && [ -n "$step" ]; then echo "check.sh: FAILED at step: $step" >&2; fi' EXIT

run() {
	step="$1"
	shift
	printf '\n==> %s\n' "$step"
	"$@"
}

gofmt_check() {
	unformatted="$(git ls-files -z '*.go' | xargs -0 gofmt -l)"
	if [ -n "$unformatted" ]; then
		echo "gofmt -l reports unformatted files:"
		echo "$unformatted"
		return 1
	fi
}

run "go build ./..." go build ./...
run "go vet ./..." go vet ./...
run "gofmt (tracked files)" gofmt_check
if [ "$quick" -eq 0 ]; then
	run "go test -race ./..." go test -race ./...
else
	printf '\n==> go test -race ./... (skipped by --quick)\n'
fi

if [ "$cross" -eq 1 ]; then
	# Matrix source: the "for target in" line of .github/workflows/release.yml.
	# Fall back to the same list, hard-coded, if that line cannot be read.
	targets="$(sed -n 's/^[[:space:]]*for target in \(.*\); do$/\1/p' .github/workflows/release.yml | head -n 1)"
	if [ -z "$targets" ]; then
		targets="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"
	fi
	for target in $targets; do
		goos="${target%/*}"
		goarch="${target#*/}"
		step="cross build $target"
		printf '\n==> %s\n' "$step"
		GOOS="$goos" GOARCH="$goarch" go build -o /dev/null ./cmd/waggle
		case "$goos" in
		darwin | windows)
			step="cross vet $target"
			printf '\n==> %s\n' "$step"
			GOOS="$goos" GOARCH="$goarch" go vet ./...
			;;
		esac
	done
fi

step=""
printf '\ncheck.sh: all steps passed\n'
