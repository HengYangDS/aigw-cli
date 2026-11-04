## Why

Archiving `verification-time-bounds` merged its new requirement, "Hang bounds
tolerate a shared CI host", into the product-quality specification, which grew
from 483 to 512 non-blank lines. The repository's specification budget is 500
non-blank lines per main specification, so the quality gate refuses the merged
result.

## What Changes

- Restate "Hang bounds tolerate a shared CI host" with the same obligation and
  the same four scenarios in 14 non-blank lines instead of 29, which brings
  the product-quality specification to 497 lines.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-quality`: the hang-bound requirement keeps its obligation and
  scenarios in a shorter statement.

## Impact

Only the specification text changes; tests, tools and the released programs
do not. The product-quality specification stays close to its budget, so the
next requirement it gains needs a split by cohesive concern first.
