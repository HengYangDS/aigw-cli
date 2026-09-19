## Why

The direct dependency inventory is current, but Go's native resolver reports
newer compatible releases in the locked module graph. The lock must reach one
reproducible compatible fixed point before the latest-stable claim is complete.

## What Changes

- Refresh the resolved Go module graph with the native Go resolver.
- Preserve existing dependency ownership, checks, and product behavior.
- Add no updater, wrapper, compatibility path, or runtime capability.

## Capabilities

This maintenance Change has no specification delta. Existing repository policy
already requires a current, reproducible, compatible dependency graph.

## Impact

Only `go.mod` and `go.sum` may change. Product APIs, configuration, credentials,
client projections, and release identity remain unchanged.
