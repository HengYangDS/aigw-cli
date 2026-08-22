## Why

A formal version currently resolves a different release epoch locally than in
tag CI. The compiler output remains identical, but archive metadata and the
SBOM differ, so one accepted source can produce multiple release byte sets.

## What Changes

Use `VERSION` and its committed `CHANGELOG.md` heading as the complete local and
CI release identity. The public build command no longer accepts a caller-chosen
version or timestamp.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This change makes the existing reproducible-release contract true across
local and hosted builders.

## Impact

Only release input resolution, its focused tests, and this Change record are
modified. Artifact formats, release topology, provider behavior, and installed
runtime behavior remain unchanged.
