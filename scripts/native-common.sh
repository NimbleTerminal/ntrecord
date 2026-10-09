# Shared by the native setup scripts. The caller supplies the repository root.
zig_version=0.16.0
pkgconf_version=2.5.1
ghostty_revision=34f39002c6e3974b54e6a6d400bd83777c5ea558
ghostty_sha256=932fede7be27fbd0bfdffbe7b42c25fea8c249ed1b52d92b50c1e599ffd41aec
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
