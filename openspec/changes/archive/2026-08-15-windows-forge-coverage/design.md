## Context

The accepted AIGW source passes native Linux, macOS, and source/governance
checks. Native Windows alone fails because `tools/forge` covers 168 of 177
branches, or 94.92 percent, below the repository's strict greater-than-95
percent package floor.

## Decision

Exercise the existing projected-target revision failure boundary. The test uses
the shared Go Git helper and matches only `rev-parse refs/heads/*`, so it is
portable across Windows, Linux, and macOS and cannot intercept the earlier
source revision lookup.

Do not change production code, weaken coverage, add an exclusion, or introduce
a platform-specific test stack.

## Verification

1. Observe the regression fail before the helper recognizes the new failure
   mode.
2. Run the focused regression and the complete `tools/forge` package tests.
3. Run the repository statement and branch coverage gate with the unchanged
   policy.
4. Execute exact-HEAD proof, archive, land, and confirm hosted Windows
   acceptance.
