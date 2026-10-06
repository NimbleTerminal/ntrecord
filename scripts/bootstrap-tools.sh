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
            zig_sha256=b23d70deaa879b5c2d486ed3316f7eaa53e84acf6fc9cc747de152450d401489 ;;
        Darwin-x86_64)
            target=x86_64-macos
            zig_sha256=0387557ed1877bc6a2e1802c8391953baddba76081876301c522f52977b52ba7 ;;
        Linux-aarch64|Linux-arm64)
            target=aarch64-linux
            zig_sha256=ea4b09bfb22ec6f6c6ceac57ab63efb6b46e17ab08d21f69f3a48b38e1534f17 ;;
        Linux-x86_64)
            target=x86_64-linux
            zig_sha256=70e49664a74374b48b51e6f3fdfbf437f6395d42509050588bd49abe52ba3d00 ;;
        *) fail 'Automatic Zig setup supports macOS/Linux amd64/arm64. Set ZIG to an existing Zig 0.16.0 executable on other hosts.' ;;
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
