#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$root/scripts/native-common.sh"
# Build the pinned Ghostty release with the project-local toolchain.
sh "$root/scripts/bootstrap-tools.sh"
prefix="${GHOSTTY_PREFIX:-$deps/libghostty-vt}"
case "$prefix" in /*) ;; *) fail 'GHOSTTY_PREFIX must be an absolute path.' ;; esac
zig="${ZIG:-$deps/tools/bin/zig}"
mkdir -p "$deps"
if [ ! -d "$deps/ghostty-$ghostty_revision" ]; then
    download "https://codeload.github.com/ghostty-org/ghostty/tar.gz/$ghostty_revision" "ghostty-$ghostty_revision.tar.gz" "$ghostty_sha256"
    unpack=$(mktemp -d "$deps/.ghostty-unpack.XXXXXX")
    tar -xzf "$archive" -C "$unpack"
    mv "$unpack/ghostty-$ghostty_revision" "$deps/ghostty-$ghostty_revision"
    rmdir "$unpack"
fi
cd "$deps/ghostty-$ghostty_revision"
# This source archive has no Git metadata. Do not let Ghostty discover the
# enclosing ntrecord checkout and mistake our release tag for its own version.
export GIT_CEILING_DIRECTORIES="$deps"
ZIG_GLOBAL_CACHE_DIR="$deps/zig-cache" "$zig" build -Demit-lib-vt -Demit-xcframework=false -Doptimize=fast --prefix "$prefix" "$@"
printf '\nBuilt libghostty-vt. Set PKG_CONFIG_PATH to %s/share/pkgconfig\n' "$prefix"
