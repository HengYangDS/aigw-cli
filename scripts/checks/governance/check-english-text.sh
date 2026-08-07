#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"

go run ./tools/repositorycheck --root "$root" english-text

echo "English text contract: OK"
