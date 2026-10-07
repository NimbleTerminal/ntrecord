#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
. "$root/scripts/native-common.sh"
[ "$(uname -s)" = Darwin ] || fail 'Release packaging requires macOS with the Xcode Command Line Tools SDK (Linux targets use Zig).'
need xcrun
xcrun --find clang >/dev/null
# Bootstrap once, then build each native archive into its own prefix. Zig's
# cache handles reuse; the usual task build prefix is never overwritten.
for pair in darwin_amd64:x86_64-macos.13.0 darwin_arm64:aarch64-macos.13.0 linux_amd64:x86_64-linux-musl linux_arm64:aarch64-linux-musl; do
    platform=${pair%%:*}
    target=${pair#*:}
    GHOSTTY_PREFIX="$deps/release/$platform" sh scripts/build-libghostty.sh "-Dtarget=$target" -Dcpu=baseline
done
GOWORK=off go mod download
GOWORK=off go run ./internal/tools/release-licenses "$deps/release/licenses" "$deps/ghostty-$ghostty_revision" "$deps/zig-cache/p" "${ZIG:-$deps/tools/bin/zig}"
