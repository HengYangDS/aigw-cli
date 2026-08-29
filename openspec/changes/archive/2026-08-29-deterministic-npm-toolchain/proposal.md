## Why

The repository currently asks mise to install npm tools whose transitive
dependencies are not represented in `mise.lock`. A clean runner can therefore
resolve different package bytes for the same product commit, making the
declared locked source gate non-deterministic.

## What Changes

- Give the standard npm lockfile sole authority over direct and transitive npm
  repository tools.
- Keep mise authoritative for the Node runtime and non-npm tools only.
- Materialize the npm closure explicitly before source verification on both
  Forge projections.
- Correct documentation that currently attributes the complete repository tool
  closure to `mise.lock`.

## Capabilities

### Modified Capabilities

- `product-quality`: distinguish the Go, runtime-tool, and npm dependency
  authorities and require clean-runner determinism.
- `ci-diagnostics`: expose the exact npm materialization step in the shared CI
  topology without duplicating Forge logic.

## Impact

The change updates repository dependency metadata, the CUE-owned CI topology,
its generated Forge projections, focused projection tests, and the canonical
contributor and release documentation. Product runtime behavior is unchanged.
