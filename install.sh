#!/bin/sh
# Install upvert from GitHub Releases.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/cruffinoni/upvert/master/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- --version vX.Y.Z --bin-dir "$HOME/.local/bin"
#   ./install.sh --local
set -eu

local_mode=0
version_flag=0

repo="cruffinoni/upvert"
releases_url="https://github.com/${repo}/releases"
download_url="${releases_url}/download"

die() {
	printf 'upvert installer: %s\n' "$*" >&2
	exit 1
}

say() {
	printf '%s\n' "$*"
}

# default_bin_dir prints the release installer's default install directory.
default_bin_dir() {
	if [ -n "${XDG_BIN_HOME:-}" ]; then
		printf '%s\n' "$XDG_BIN_HOME"
	elif [ -n "${HOME:-}" ]; then
		printf '%s\n' "$HOME/.local/bin"
	else
		die "HOME is not set; pass --bin-dir"
	fi
}

# resolve_source_root finds the checkout containing go.mod and cmd/upvert.
resolve_source_root() {
	script_dir=""
	if [ -n "${0:-}" ]; then
		script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" 2>/dev/null && pwd) || script_dir=""
	fi
	for candidate in "$script_dir" "$(pwd)"; do
		[ -n "$candidate" ] || continue
		if [ -f "$candidate/go.mod" ] && [ -d "$candidate/cmd/upvert" ]; then
			printf '%s\n' "$candidate"
			return 0
		fi
	done
	return 1
}

# resolve_target follows a symlink chain without requiring realpath.
resolve_target() {
	target_path=$1
	hops=0
	while [ -L "$target_path" ]; do
		hops=$((hops + 1))
		[ "$hops" -le 40 ] || die "too many symbolic links: $1"
		target_link=$(readlink "$target_path") || die "could not read symlink $target_path"
		case "$target_link" in
		/*) target_path=$target_link ;;
		*) target_path=$(dirname -- "$target_path")/$target_link ;;
		esac
	done
	printf '%s\n' "$target_path"
}

# find_existing_binary returns the installed upvert path, if any.
find_existing_binary() {
	found=$(command -v upvert 2>/dev/null || true)
	if [ -n "$found" ] && [ -f "$found" ] && [ -x "$found" ]; then
		resolve_target "$found"
	fi
}

# install_local builds upvert from the checkout and replaces the target.
install_local() {
	[ "$version_flag" -eq 0 ] || die "--local cannot be combined with --version"
	[ -z "$version" ] || say "Ignoring release version in --local mode."
	command -v go >/dev/null 2>&1 || die "go is required for --local"
	source_root=$(resolve_source_root) || die "could not find an upvert checkout; run this script from the repository root"
	if [ -n "$bin_dir" ]; then
		target_dir=$bin_dir
		target=$target_dir/upvert
	else
		existing=$(find_existing_binary)
		if [ -n "$existing" ]; then
			target=$existing
			target_dir=$(dirname -- "$target")
		else
			target_dir=$(default_bin_dir)
			target=$target_dir/upvert
		fi
	fi
	[ -d "$target" ] && die "install target is a directory: $target"
	mkdir -p "$target_dir" || die "could not create $target_dir"
	[ -w "$target_dir" ] || die "install directory is not writable: $target_dir (use --bin-dir)"
	build_tmp=$(mktemp "$target_dir/.upvert-local.XXXXXX") || die "could not create a temporary binary in $target_dir"
	trap 'if [ -n "${build_tmp:-}" ]; then rm -f "$build_tmp"; fi' EXIT HUP INT TERM
	say "Building upvert from source..."
	if ! (cd "$source_root" && CGO_ENABLED=0 go build -trimpath -o "$build_tmp" ./cmd/upvert); then
		die "go build failed"
	fi
	chmod 0755 "$build_tmp" || die "could not make $build_tmp executable"
	mv -f "$build_tmp" "$target" || die "could not install to $target"
	build_tmp=""
	say "Installed upvert from source to $target"
	case ":${PATH:-}:" in
	*":$target_dir:"*) ;;
	*)
		say ""
		say "Add $target_dir to your PATH:"
		say "  export PATH=\"$target_dir:\$PATH\""
		;;
	esac
}

usage() {
	cat <<'EOF'
Install upvert from GitHub Releases, or build from a checkout with --local.

Usage: install.sh [options]

Options:
  --version <tag>   Release tag to install (default: latest stable release)
  --bin-dir <dir>   Directory to install into (default: ~/.local/bin)
  --local           Build from the current checkout and replace the installed binary
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
		version_flag=1
		shift 2
		;;
	--version=*)
		version=${1#--version=}
		version_flag=1
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
	--local)
		local_mode=1
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

if [ "$local_mode" -eq 1 ]; then
	install_local
	exit 0
fi

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
