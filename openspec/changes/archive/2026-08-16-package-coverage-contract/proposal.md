# Package Coverage Contract

## Why

The repository reports exact package ratios but admits coverage only at the
aggregate boundary. That contradicts the product acceptance target requiring
statement and branch coverage above 95 percent for every canonical package.

## What Changes

- Make the machine policy the sole owner of aggregate and package thresholds.
- Apply the same strict comparison to statement and branch evidence.
- Close the two observed package gaps through meaningful portable behavior
  tests and simpler control flow, without exclusions or ignored branches.
- Remove prose that still describes package ratios as non-blocking.

## Impact

Coverage evidence and repository-tool tests change. Product runtime behavior,
provider routing, credentials, client projections, and release topology do not.
