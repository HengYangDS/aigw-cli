#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"
go run ./tools/cicontract github-release "$root"
echo "GitHub Actions release contract: OK"
