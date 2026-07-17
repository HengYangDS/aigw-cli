#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
expected=$(awk '$1 == "go" { print $2; exit }' "$root/go.mod")
[ -n "$expected" ] || { echo "release toolchain: go.mod has no Go version" >&2; exit 1; }
actual=$(go env GOVERSION)
[ "$actual" = "go$expected" ] || {
  echo "release toolchain: expected go$expected, found $actual" >&2
  exit 1
}
echo "release toolchain: $actual OK"
