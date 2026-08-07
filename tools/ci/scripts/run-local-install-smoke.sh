#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
workspace=$(mktemp -d "${TMPDIR:-/tmp}/aigw-local-install.XXXXXX")
trap 'rm -rf "$workspace"' EXIT HUP INT TERM

cd "$root"
go build -trimpath -o "$workspace/aigw" ./cmd/aigw
sh scripts/tests/install/test-portable-install.sh "$workspace/aigw"
