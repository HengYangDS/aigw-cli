#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"
go run ./tools/cicontract pipeline "$root"
echo "pipeline gates contract: OK"
