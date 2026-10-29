#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$root/scripts/native-common.sh"
tools="$deps/tools"
mkdir -p "$tools/bin"
need tar
need make
# cgo and pkgconf still need the platform's C compiler and SDK/libc headers.
need cc

if [ -n "${ZIG:-}" ]; then
    need "$ZIG"
    [ "$("$ZIG" version)" = "$zig_version" ] || fail "ZIG must point to Zig $zig_version. Unset ZIG to use the project-local toolchain."
else
    case "$(uname -s)-$(uname -m)" in
        Darwin-arm64|Darwin-aarch64)
            target=aarch64-macos
            zig_sha256=b607e9b9234790a008116ae5bdb71c6243b84b9fb42a53a9e70fde41c06c536a ;;
        Darwin-x86_64)
            target=x86_64-macos
            zig_sha256=4f9a1c5269aa17ebda5e6d3c2b89d6cbf36f7d2b22a0306e9ab98f25f95529c6 ;;
        Linux-aarch64|Linux-arm64)
            target=aarch64-linux
            zig_sha256=9e8d11661d4ae3bd57702a3832781e23ad151dde5798e16a5ccd503f65234ff8 ;;
        Linux-x86_64)
            target=x86_64-linux
            zig_sha256=1cbe9df9f27e6b78d14ccbca43b6703a404ef79ef1c463de901d7f088d4e2026 ;;
        *) fail 'Automatic Zig setup supports macOS/Linux amd64/arm64. Set ZIG to an existing Zig 0.17.0 executable on other hosts.' ;;
    esac
    zig_name="zig-$target-$zig_version"
    if [ ! -x "$tools/$zig_name/zig" ]; then
        download "https://ziglang.org/download/$zig_version/$zig_name.tar.xz" "$zig_name.tar.xz" "$zig_sha256"
        unpack=$(mktemp -d "$tools/.zig-unpack.XXXXXX")
        tar -xf "$archive" -C "$unpack"
        mv "$unpack/$zig_name" "$tools/$zig_name"
        rmdir "$unpack"
    fi
    ln -sfn "../$zig_name/zig" "$tools/bin/zig"
fi

if [ ! -x "$tools/bin/pkgconf" ] || [ "$("$tools/bin/pkgconf" --version)" != "$pkgconf_version" ]; then
    download "https://distfiles.dereferenced.org/pkgconf/pkgconf-$pkgconf_version.tar.xz" "pkgconf-$pkgconf_version.tar.xz" "$pkgconf_sha256"
    work=$(mktemp -d "$tools/.pkgconf-build.XXXXXX")
    tar -xf "$archive" -C "$work"
    printf 'Building project-local pkgconf %s (log: %s/pkgconf-build.log)\n' "$pkgconf_version" "$tools"
    if ! (
        cd "$work/pkgconf-$pkgconf_version"
        ./configure --disable-shared --prefix="$tools" &&
        make -j4 &&
        make install
    ) >"$tools/pkgconf-build.log" 2>&1; then
        cat "$tools/pkgconf-build.log" >&2
        fail 'pkgconf build failed; check the C compiler and platform SDK installation.'
    fi
fi
