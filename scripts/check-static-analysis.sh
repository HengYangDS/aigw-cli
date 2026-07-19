#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

# The analyzer is declared through Go's tracked tool dependency, so every
# maintainer and CI plane resolves the same version without a host-specific
# installation path. ST1000 is inapplicable to internal-only packages; ST1005
# conflicts with the product's intentionally title-cased user-facing errors.
go tool staticcheck -checks=all,-ST1000,-ST1005 ./...
