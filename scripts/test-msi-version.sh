#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$root/scripts/msi-version.sh"

assert_version() {
  input=$1
  want=$2
  version=$input
  got=$(msi_version)
  [ "$got" = "$want" ] || {
    echo "msi_version $input = $got, want $want" >&2
    exit 1
  }
}

# Windows Installer compares exactly three numeric fields. The third field is
# patch * 256 + release-stage, preserving patch and SemVer release ordering.
assert_version 0.1.0-abcdef 0.1.0
assert_version 0.1.0-alpha.1 0.1.1
assert_version 0.1.0-alpha.63 0.1.63
assert_version 0.1.0-beta.1 0.1.64
assert_version 0.1.0-beta.64 0.1.127
assert_version 0.1.0-rc.1 0.1.128
assert_version 0.1.0-rc.127 0.1.254
assert_version 0.1.0 0.1.255
assert_version v1.2.3-beta.7 1.2.838
assert_version 1.2.3-alpha.4 1.2.772
assert_version 1.2.3-rc.2 1.2.897
assert_version 1.2.3 1.2.1023

# A new patch must sort after the prior patch's GA artifact.
version=1.2.3
old_ga=$(msi_version)
version=1.2.4-alpha.1
new_alpha=$(msi_version)
old_build=${old_ga##*.}
new_build=${new_alpha##*.}
[ "$new_build" -gt "$old_build" ] || {
  echo "next patch alpha must sort after prior patch GA: $new_alpha <= $old_ga" >&2
  exit 1
}

assert_invalid() {
  input=$1
  if version=$input msi_version >/dev/null 2>&1; then
    echo "invalid MSI version was accepted: $input" >&2
    exit 1
  fi
}

assert_invalid 1.2
assert_invalid 1.2.3.4
assert_invalid 256.1.0
assert_invalid 1.256.0
assert_invalid 1.2.256
assert_invalid 1.2.3-rc.128

echo "MSI version mapping: OK"
