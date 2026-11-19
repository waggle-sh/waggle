#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Install the waggle CLI from the latest GitHub release of waggle-sh/waggle.
#
#   curl -fsSL https://raw.githubusercontent.com/waggle-sh/waggle/main/install.sh | sh
#
# Environment:
#   WAGGLE_VERSION      release tag to install (for example v0.1.0); default is the latest
#   WAGGLE_INSTALL_DIR  directory to install into; default is ~/.local/bin, or
#                       /usr/local/bin when ~/.local/bin cannot be created
set -eu

repo="waggle-sh/waggle"

say() { printf '%s\n' "$*"; }
die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

have curl || die "curl is required"
have tar || die "tar is required"
have gzip || die "gzip is required"

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) die "unsupported OS $(uname -s); on Windows download the .zip from https://github.com/$repo/releases" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) die "unsupported architecture $(uname -m)" ;;
esac

tag="${WAGGLE_VERSION:-}"
if [ -z "$tag" ]; then
  # /releases/latest redirects to /releases/tag/<tag>.
  url="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")" ||
    die "could not look up the latest release"
  tag="${url##*/}"
fi
case "$tag" in
  v[0-9]*) ;;
  *) die "unexpected release tag '$tag'" ;;
esac
version="${tag#v}"

name="waggle_${version}_${os}_${arch}"
archive="$name.tar.gz"
base="https://github.com/$repo/releases/download/$tag"

if have sha256sum; then
  sha() { sha256sum "$1" | cut -d ' ' -f 1; }
elif have shasum; then
  sha() { shasum -a 256 "$1" | cut -d ' ' -f 1; }
else
  die "sha256sum or shasum is required to verify the download"
fi

tmp="$(mktemp -d)" || die "could not create a temporary directory"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

say "Downloading waggle $tag for $os/$arch"
curl -fsSL -o "$tmp/$archive" "$base/$archive" || die "download failed: $base/$archive"
curl -fsSL -o "$tmp/SHA256SUMS" "$base/SHA256SUMS" || die "download failed: $base/SHA256SUMS"

want="$(awk -v f="$archive" '$2 == f || $2 == "*" f { print $1; exit }' "$tmp/SHA256SUMS")"
[ -n "$want" ] || die "$archive is not listed in SHA256SUMS"
got="$(sha "$tmp/$archive")"
[ "$got" = "$want" ] || die "checksum mismatch for $archive (want $want, got $got)"
say "Checksum verified"

mkdir "$tmp/x"
tar -xzf "$tmp/$archive" -C "$tmp/x" || die "could not extract $archive"
bin="$tmp/x/$name/waggle"
[ -f "$bin" ] || die "$archive does not contain $name/waggle"

dir="${WAGGLE_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
  if [ -d "$HOME/.local/bin" ] || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
    dir="$HOME/.local/bin"
  else
    dir=/usr/local/bin
  fi
fi

if [ -d "$dir" ] && [ -w "$dir" ]; then
  install -m 755 "$bin" "$dir/waggle"
elif have sudo; then
  say "Installing to $dir with sudo"
  sudo mkdir -p "$dir" && sudo install -m 755 "$bin" "$dir/waggle"
else
  die "cannot write to $dir; set WAGGLE_INSTALL_DIR to a writable directory"
fi

say "Installed $dir/waggle"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) say "Note: $dir is not on your PATH; add it, for example: export PATH=\"$dir:\$PATH\"" ;;
esac
