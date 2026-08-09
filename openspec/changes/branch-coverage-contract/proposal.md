# Truthful Go branch coverage

## Problem

The release-readiness contract names branch coverage, while the executable gate
measures only Go statements. A passing statement profile therefore cannot prove
the promised branch claim.

## Change

Keep the Go cover profile as the sole statement and per-package authority. The
evaluation found no stable, maintained tool that measures the complete module
in one test execution across macOS, Linux, and Windows. Therefore remove the
unsupported branch claim instead of adopting a source-rewriting fork or owning
an instrumenter. CI and local proof continue to call one repository command.

## Boundary

The gate does not rename statement coverage as branch coverage, exclude hard
packages, duplicate tests, or invent an unproved measure. Go's native cover
profile remains authoritative for statements. Branch coverage becomes a future
capability only when one admitted tool satisfies the complete contract.
