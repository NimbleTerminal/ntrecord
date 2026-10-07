#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
# Do not accidentally link the host library while cgo cross-compiles.
export PKG_CONFIG_PATH="$root/.deps/release/${GOOS:?}_${GOARCH:?}/share/pkgconfig"
export PKG_CONFIG_LIBDIR="$PKG_CONFIG_PATH"
exec sh "$root/scripts/pkg-config.sh" "$@"
