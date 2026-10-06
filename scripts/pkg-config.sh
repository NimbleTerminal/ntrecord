#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ -x "$root/.deps/tools/bin/pkgconf" ]; then
    exec "$root/.deps/tools/bin/pkgconf" "$@"
fi
if command -v pkg-config >/dev/null 2>&1; then exec pkg-config "$@"; fi
if command -v pkgconf >/dev/null 2>&1; then exec pkgconf "$@"; fi
printf 'pkg-config is missing; run task deps to install project-local tools.\n' >&2
exit 1
