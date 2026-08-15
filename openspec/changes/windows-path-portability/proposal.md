## Why

Native Windows verification exposes two host-dependent assumptions in the CI
projection tool: manifest paths are validated with the host separator, and CUE
receives an absolute model path that may be on another Windows volume.

## What Changes

- Treat projection manifest paths as slash-separated repository paths.
- Reject traversal, backslashes, absolute paths, and Windows volume prefixes.
- Run CUE from the repository root with one repository-relative model path.
- Add focused contracts for both portability boundaries.

## Boundary

This Change repairs the repository CI tool only. It does not change CI topology,
runner ownership, product behavior, Forge publication, or release semantics.
