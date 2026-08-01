#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"

fail() {
  echo "product surface contract failed: $1" >&2
  exit 1
}

[ -f LICENSE ] || fail "missing LICENSE"
[ -f README.md ] || fail "missing README.md"
[ -f CHANGELOG.md ] || fail "missing CHANGELOG.md"
[ -f CONTRIBUTING.md ] || fail "missing CONTRIBUTING.md"

expected_license=$(cat <<'LICENSE'
MIT License

Copyright (c) 2026 AIGW CLI Contributors
LICENSE
)
actual_license=$(sed -n '1,3p' LICENSE)
[ "$actual_license" = "$expected_license" ] || fail "LICENSE must declare the canonical MIT grant and copyright holder"

grep -Fq 'THE SOFTWARE IS PROVIDED "AS IS"' LICENSE || fail "LICENSE is not the complete MIT text"
grep -Fq '[MIT](LICENSE)' README.md || fail "README must link the MIT license"
grep -Fq 'MIT License' README.md || fail "README must name the MIT license"
grep -Fq 'license: MIT' packaging/linux/nfpm.yaml || fail "Linux package metadata must declare MIT"
grep -Fq 'maintainer: ${AIGW_PACKAGE_MAINTAINER}' packaging/linux/nfpm.yaml || fail "Linux package metadata must receive its maintainer from release context"
grep -Fq 'vendor: AIGW CLI' packaging/linux/nfpm.yaml || fail "Linux package metadata must declare the formal product vendor"
grep -Fq 'homepage: ${AIGW_PACKAGE_HOMEPAGE}' packaging/linux/nfpm.yaml || fail "Linux package metadata must receive its homepage from release context"
grep -Fq 'Manufacturer="AIGW CLI"' packaging/windows/aigw.wxs || fail "Windows package metadata must declare the formal product manufacturer"
grep -Fq 'dst: /usr/share/doc/aigw/copyright' packaging/linux/nfpm.yaml || fail "Linux package must include the MIT license file"
grep -Fq '[LICENSE](../LICENSE)' docs/README.md || fail "documentation root must link the license"
grep -Fq 'MIT License' CONTRIBUTING.md || fail "contribution policy must name the license"
grep -Fq '## [Unreleased]' CHANGELOG.md || fail "CHANGELOG must retain the Unreleased section"

if grep -RInE 'license:[[:space:]]*Proprietary|Proprietary' \
  README.md CHANGELOG.md CONTRIBUTING.md docs packaging .github .gitlab-ci.yml 2>/dev/null; then
  fail "proprietary licensing residue found"
fi

printf '%s\n' 'product surface contract: OK'
