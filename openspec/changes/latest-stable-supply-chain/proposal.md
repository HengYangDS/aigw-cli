## Why

The locked Go graph is one stable indirect release behind for
`github.com/charmbracelet/ultraviolet`. The repository contract requires the
current stable dependency set rather than a stale lock.

## What Changes

- Refresh the Go module graph with the repository's declared Go toolchain.
- Accept only the resolver-selected indirect dependency update.
- Preserve canonical text layout after the preceding OpenSpec archive projection.

## Capabilities

### Modified Capabilities

- `product-control-plane`: retain behavior while using the current stable locked graph.

## Impact

- **Authority:** `go.mod` and `go.sum` remain the only Go dependency SSOT.
- **Breaking changes:** none.
- **Reuse:** the Go module resolver owns transitive selection.
- **Non-goals:** no product behavior, API, CLI, or publication change.
