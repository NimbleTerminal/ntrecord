# Shared by the native setup scripts. The caller supplies the repository root.
zig_version=0.16.0
pkgconf_version=2.5.1
ghostty_revision=0e0ff282f72c92d930d41f4703547d3bc9618ba2
ghostty_sha256=23a9ba8b578ee3c27bb5fef0130ffb1df9304eee7f264ed71b1e47e4a88ad559
pkgconf_sha256=cd05c9589b9f86ecf044c10a2269822bc9eb001eced2582cfffd658b0a50c243
deps="$root/.deps"

fail() { printf '%s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || fail "Required tool '$1' is missing. See docs/development.md (Build)."; }

verify_sha256() {
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$1")
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$1")
    else
        fail 'A SHA-256 tool is required: sha256sum (Linux) or shasum (macOS).'
    fi
    actual=${actual%% *}
    [ "$actual" = "$2" ] || fail "SHA-256 mismatch for $1; refusing to extract it."
}

# Cache only verified archives. A failed/interrupted download is never used.
download() {
    archive="$deps/downloads/$2"
    mkdir -p "$deps/downloads"
    if [ ! -f "$archive" ]; then
        need curl
        printf 'Downloading %s\n' "$2"
        curl --fail --silent --show-error --location --retry 3 --connect-timeout 30 "$1" -o "$archive.part"
        verify_sha256 "$archive.part" "$3"
        mv "$archive.part" "$archive"
    else
        verify_sha256 "$archive" "$3"
    fi
}
