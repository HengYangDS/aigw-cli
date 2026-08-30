## Why

The locked AIGW toolchain is current, but its Go module closure has newer stable
releases. Refreshing that one owned graph now prevents avoidable dependency
drift without changing product behavior or adding another package authority.

## What Changes

- Refresh the Go module graph to current stable releases selected by the Go
  module resolver.
- Keep `go.mod` and `go.sum` as the sole authority for that graph.
- Re-run the complete source and native release verification before landing.
- Do not add compatibility targets, wrappers, or parallel dependency tooling.

## Capabilities

This maintenance change does not alter observable product requirements and
therefore opts out of specification deltas.

## Impact

Only Go dependency declarations and checksums may change. AIGW commands,
configuration, storage, client projections, and Forge semantics remain
unchanged.
