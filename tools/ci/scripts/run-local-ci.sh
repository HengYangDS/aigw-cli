#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"

go run ./tools/architecture --root .
go run ./tools/coveragegate --race
go vet ./...
sh scripts/checks/quality/check-static-analysis.sh
