#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Install the waggle CLI from a GitHub release of waggle-sh/waggle.
#
#   curl -fsSL https://raw.githubusercontent.com/waggle-sh/waggle/main/install.sh | sh
#
# It picks the archive for this OS and CPU, checks it against the release's
# SHA256SUMS, and puts the waggle binary in the install directory.
#
# Environment:
#   WAGGLE_VERSION      release to install, e.g. 0.1.0 or v0.1.0 (default: latest)
#   WAGGLE_INSTALL_DIR  where to put the binary (default: ~/.local/bin if it is
#                       writable or can be created, else /usr/local/bin)
#   WAGGLE_BASE_URL     directory URL holding SHA256SUMS and the archives,
#                       overriding the GitHub release URL (any URL curl or wget
#                       accepts, including file://); used for testing and mirrors
#
# Windows is not handled here: download the .zip from the release page.

set -eu

REPO="waggle-sh/waggle"
# curl may follow only these schemes. A WAGGLE_BASE_URL override also allows
# http, for a local test server.
PROTO="=https"

say() {
	printf 'waggle-install: %s\n' "$*" >&2
}

die() {
	say "error: $*"
	exit 1
}

have() {
	command -v "$1" >/dev/null 2>&1
}

# fetch <url> <file>
fetch() {
	if have curl; then
		curl -fsSL --proto "$PROTO" -o "$2" "$1"
	elif have wget; then
		# Same scheme rule as curl's --proto: https only, and http and file
		# only under a WAGGLE_BASE_URL override.
		case "$1" in
		https://*) wget -q -O "$2" "$1" ;;
		file://*)
			[ -n "${WAGGLE_BASE_URL:-}" ] || die "refusing non-https URL: $1"
			cp "${1#file://}" "$2"
			;;
		http://*)
			[ -n "${WAGGLE_BASE_URL:-}" ] || die "refusing non-https URL: $1"
			wget -q -O "$2" "$1"
			;;
		*) die "refusing URL with unsupported scheme: $1" ;;
		esac
	else
		die "need curl or wget"
	fi
}

# sha256 <file> prints the hex digest.
sha256() {
	if have sha256sum; then
		sha256sum "$1" | cut -d ' ' -f 1
	elif have shasum; then
		shasum -a 256 "$1" | cut -d ' ' -f 1
	elif have openssl; then
		openssl dgst -sha256 -r "$1" | cut -d ' ' -f 1
	else
		die "need sha256sum, shasum or openssl to check the download"
	fi
}

detect_os() {
	case "$(uname -s)" in
	Linux) echo linux ;;
	Darwin) echo darwin ;;
	MINGW* | MSYS* | CYGWIN*) die "on Windows, download the .zip from https://github.com/$REPO/releases" ;;
	*) die "unsupported OS: $(uname -s)" ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo amd64 ;;
	arm64 | aarch64) echo arm64 ;;
	*) die "unsupported CPU: $(uname -m) (release archives are amd64 and arm64)" ;;
	esac
}

pick_dir() {
	if [ -n "${WAGGLE_INSTALL_DIR:-}" ]; then
		echo "$WAGGLE_INSTALL_DIR"
		return
	fi
	if [ -n "${HOME:-}" ] && mkdir -p "$HOME/.local/bin" 2>/dev/null && [ -w "$HOME/.local/bin" ]; then
		echo "$HOME/.local/bin"
		return
	fi
	echo /usr/local/bin
}

main() {
	os=$(detect_os)
	arch=$(detect_arch)

	version="${WAGGLE_VERSION:-}"
	version="${version#v}"
	if [ -n "${WAGGLE_BASE_URL:-}" ]; then
		base="${WAGGLE_BASE_URL%/}"
		PROTO="=https,http,file"
	elif [ -n "$version" ]; then
		base="https://github.com/$REPO/releases/download/v$version"
	else
		base="https://github.com/$REPO/releases/latest/download"
	fi

	tmp=$(mktemp -d 2>/dev/null || mktemp -d -t waggle-install)
	trap 'rm -rf "$tmp"' EXIT
	trap 'exit 130' INT TERM

	say "fetching $base/SHA256SUMS"
	if ! fetch "$base/SHA256SUMS" "$tmp/SHA256SUMS"; then
		if [ -z "${WAGGLE_BASE_URL:-}" ] && [ -z "$version" ]; then
			say "a 404 from releases/latest/download usually means no release is published yet"
			say "set WAGGLE_VERSION (e.g. WAGGLE_VERSION=0.1.0) to pick one, or see https://github.com/$REPO/releases"
		fi
		die "could not download $base/SHA256SUMS"
	fi

	# A SHA256SUMS line is "<hex>  <file>". Pick this platform's archive,
	# pinned to WAGGLE_VERSION when it is set.
	if [ -n "$version" ]; then
		want="waggle_${version}_${os}_${arch}.tar.gz"
		line=$(awk -v f="$want" '$2 == f || $2 == "*" f { print; exit }' "$tmp/SHA256SUMS")
	else
		line=$(awk -v s="_${os}_${arch}.tar.gz" '
			{ f = $2; sub(/^\*/, "", f) }
			f ~ /^waggle_/ && substr(f, length(f) - length(s) + 1) == s { print; exit }
		' "$tmp/SHA256SUMS")
	fi
	[ -n "$line" ] || die "no ${os}_${arch} archive listed in $base/SHA256SUMS"
	sum=$(echo "$line" | awk '{ print $1 }')
	archive=$(echo "$line" | awk '{ f = $2; sub(/^\*/, "", f); print f }')
	case "$archive" in
	*/* | .*) die "unexpected archive name in SHA256SUMS: $archive" ;;
	esac
	dir="${archive%.tar.gz}"

	say "fetching $base/$archive"
	fetch "$base/$archive" "$tmp/$archive" || die "could not download $base/$archive"

	got=$(sha256 "$tmp/$archive")
	if [ "$got" != "$sum" ]; then
		die "checksum mismatch for $archive: SHA256SUMS says $sum, download is $got"
	fi
	say "checksum ok ($sum)"

	mkdir "$tmp/x"
	tar -xzf "$tmp/$archive" -C "$tmp/x" "$dir/waggle" || die "$archive has no $dir/waggle"
	bin="$tmp/x/$dir/waggle"
	[ -f "$bin" ] || die "$archive has no $dir/waggle"
	chmod 755 "$bin"

	dest=$(pick_dir)
	if [ ! -e "$dest" ]; then
		mkdir -p "$dest" 2>/dev/null || true
	fi
	if [ -d "$dest" ] && [ -w "$dest" ]; then
		mv -f "$bin" "$dest/waggle"
	elif have sudo; then
		say "$dest is not writable; using sudo"
		sudo mkdir -p "$dest"
		sudo mv -f "$bin" "$dest/waggle"
	else
		die "$dest is not writable; set WAGGLE_INSTALL_DIR to a directory you own"
	fi

	say "installed $("$dest/waggle" version) to $dest/waggle"
	case ":${PATH:-}:" in
	*":$dest:"*) ;;
	*) say "$dest is not on your PATH; add it, e.g. export PATH=\"$dest:\$PATH\"" ;;
	esac
}

main "$@"
