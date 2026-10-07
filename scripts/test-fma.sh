#!/bin/sh
# Re-run the renderer's golden frames with FMA fusion disabled. arm64 fuses
# a*b+c into one rounding and amd64 does not, so this imitates amd64 arithmetic on
# an arm64 machine: the frames must not change. The compiler logs every fusion
# site it disables, so the output is filtered.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
log=$(mktemp)
trap 'rm -f "$log"' EXIT HUP INT TERM
status=0
go test -mod=readonly -count=1 -gcflags=all=-d=fmahash=n -run 'Golden|FaceMatches' ./internal/render >"$log" 2>&1 || status=$?
grep -v -E 'bisect-match|fmahash triggered|^# ' "$log" || true
exit "$status"
