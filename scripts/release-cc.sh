#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
case "${GOARCH:?}" in
    amd64) arch=x86_64 ;;
    arm64) arch=aarch64 ;;
    *) printf 'Unsupported release architecture: %s\n' "$GOARCH" >&2; exit 1 ;;
esac
case "${GOOS:?}" in
    linux)
        # musl makes Linux archives independent of the builder's glibc.
        exec "${ZIG:-$root/.deps/tools/bin/zig}" cc -target "$arch-linux-musl" -static "$@"
        ;;
    darwin)
        [ "$arch" != aarch64 ] || arch=arm64
        exec xcrun clang -arch "$arch" -mmacosx-version-min=13.0 "$@"
        ;;
    *) printf 'Unsupported release OS: %s\n' "$GOOS" >&2; exit 1 ;;
esac
