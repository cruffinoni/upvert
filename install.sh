#!/bin/sh
# Install upvert from GitHub Releases.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/dayrion/upvert/master/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- --version vX.Y.Z --bin-dir "$HOME/.local/bin"
set -eu

repo="dayrion/upvert"
releases_url="https://github.com/${repo}/releases"
download_url="${releases_url}/download"

die() {
	printf 'upvert installer: %s\n' "$*" >&2
	exit 1
}

say() {
	printf '%s\n' "$*"
}

usage() {
	cat <<'EOF'
Install upvert from GitHub Releases.

Usage: install.sh [options]

Options:
  --version <tag>   Release tag to install (default: latest stable release)
  --bin-dir <dir>   Directory to install into (default: ~/.local/bin)
  -h, --help        Show this help
EOF
}

version=""
bin_dir=""

while [ "$#" -gt 0 ]; do
	case "$1" in
	--version)
		[ "$#" -ge 2 ] || die "--version requires a value"
		version=$2
		shift 2
		;;
	--version=*)
		version=${1#--version=}
		shift
		;;
	--bin-dir)
		[ "$#" -ge 2 ] || die "--bin-dir requires a value"
		bin_dir=$2
		shift 2
		;;
	--bin-dir=*)
		bin_dir=${1#--bin-dir=}
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		die "unknown argument: $1 (try --help)"
		;;
	esac
done

if [ -z "$bin_dir" ]; then
	if [ -n "${XDG_BIN_HOME:-}" ]; then
		bin_dir="$XDG_BIN_HOME"
	elif [ -n "${HOME:-}" ]; then
		bin_dir="$HOME/.local/bin"
	else
		die "HOME is not set; pass --bin-dir"
	fi
fi
[ -n "$bin_dir" ] || die "install directory must not be empty"

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v tar >/dev/null 2>&1 || die "tar is required"
command -v awk >/dev/null 2>&1 || die "awk is required"
command -v grep >/dev/null 2>&1 || die "grep is required"

os=$(uname -s)
case "$os" in
Linux) os=linux ;;
Darwin) os=darwin ;;
MINGW* | MSYS* | CYGWIN*)
	die "Windows is supported by install.ps1 in PowerShell"
	;;
*)
	die "unsupported operating system: $os (supported: Linux, macOS)"
	;;
esac

machine=$(uname -m)
case "$machine" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*)
	die "unsupported CPU architecture: $machine (supported: amd64, arm64)"
	;;
esac

if [ -z "$version" ]; then
	say "Resolving the latest upvert release..."
	latest_url=$(curl -fsSL --retry 3 -o /dev/null -w '%{url_effective}' "${releases_url}/latest") ||
		die "could not resolve the latest release"
	version=${latest_url##*/}
fi

case "$version" in
v*) ;;
*) version="v${version}" ;;
esac

printf '%s\n' "$version" |
	grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$' ||
	die "invalid release version: $version"

asset_version=${version#v}
archive="upvert_${asset_version}_${os}_${arch}.tar.gz"

tmp=$(mktemp -d "${TMPDIR:-/tmp}/upvert.XXXXXX") || die "could not create a temporary directory"
cleanup() {
	rm -rf "$tmp"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

say "Downloading upvert ${asset_version} for ${os}/${arch}..."
curl -fsSL --retry 3 -o "$tmp/$archive" "${download_url}/${version}/${archive}" ||
	die "could not download $archive (does this release publish ${os}/${arch} binaries?)"
curl -fsSL --retry 3 -o "$tmp/checksums.txt" "${download_url}/${version}/checksums.txt" ||
	die "could not download checksums.txt"

expected=$(awk -v name="$archive" '$2 == name { print $1; exit }' "$tmp/checksums.txt")
[ -n "$expected" ] || die "checksums.txt does not list $archive"

sha256_file() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{ print $1 }'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{ print $1 }'
	elif command -v openssl >/dev/null 2>&1; then
		openssl dgst -sha256 "$1" | sed 's/.*= //'
	else
		return 1
	fi
}

actual=$(sha256_file "$tmp/$archive") || die "no SHA-256 tool found (tried sha256sum, shasum, openssl)"
[ "$actual" = "$expected" ] || die "checksum mismatch for $archive"

mkdir -p "$tmp/extract"
tar -xzf "$tmp/$archive" -C "$tmp/extract" || die "could not extract $archive"

binary="$tmp/extract/upvert"
if [ ! -f "$binary" ]; then
	binary=$(find "$tmp/extract" -type f -name upvert -print | head -n 1)
fi
[ -n "$binary" ] && [ -f "$binary" ] || die "archive does not contain upvert"

mkdir -p "$bin_dir" || die "could not create $bin_dir"
if command -v install >/dev/null 2>&1; then
	install -m 0755 "$binary" "$bin_dir/upvert" || die "could not install to $bin_dir/upvert"
else
	cp "$binary" "$bin_dir/upvert" || die "could not install to $bin_dir/upvert"
	chmod 0755 "$bin_dir/upvert" || die "could not make $bin_dir/upvert executable"
fi

say "Installed upvert $version to $bin_dir/upvert"

case ":${PATH:-}:" in
*":${bin_dir}:"*) ;;
*)
	say ""
	say "Add $bin_dir to your PATH:"
	say "  export PATH=\"$bin_dir:\$PATH\""
	;;
esac
