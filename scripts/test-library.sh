#!/bin/sh
# Import public packages from an unrelated module, without internal access.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
cp -R "$root/testdata/library/codecs" "$root/testdata/library/terminal" "$work/"
cd "$work"
export GOWORK=off
go mod init example.org/ntrecord-consumer
go mod edit -require=nimbleterminal.dev/ntrecord@v0.0.0 -replace="nimbleterminal.dev/ntrecord=$root"
CGO_ENABLED=0 go test -mod=mod ./codecs
CGO_ENABLED=1 go test -mod=mod ./terminal
