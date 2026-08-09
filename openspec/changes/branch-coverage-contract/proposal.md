# Truthful Go branch coverage

## Problem

The release-readiness contract names branch coverage, while the executable gate
measures only Go statements. A passing statement profile therefore cannot prove
the promised branch claim.

## Change

Keep the Go cover profile as the sole statement and per-package owner. Add one
mature source instrumenter for aggregate branch coverage, lock it through the Go
tool graph, and make the existing coverage gate enforce both results strictly
above 95 percent. CI and local proof continue to call one repository command.

## Boundary

The gate does not rename statement coverage as branch coverage, exclude hard
packages, instrument tests, or invent a second threshold. Go's native cover
profile remains authoritative for statements; the branch tool owns only branch
measurement. Select statements remain outside the admitted tool's branch model
and retain ordinary statement/race coverage.
