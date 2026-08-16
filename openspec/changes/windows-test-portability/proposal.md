## Why

The native Windows gate exposed a false portability assumption in one
architecture test: clearing POSIX mode bits does not make a directory
unreadable on Windows. The test therefore fails before it can prove the
production error contract.

## What Changes

- Construct the directory-read failure deterministically through a private
  filesystem operation seam.
- Exercise the same production error path on macOS, Linux, and Windows.
- Preserve the package-topology policy and its public behavior unchanged.

## Boundary

This Change repairs test construction only. It does not weaken the architecture
gate, skip Windows, alter product behavior, or add a runtime dependency.
