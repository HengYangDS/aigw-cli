# Single-Pass Reader Verification

## Why

AIGW 0.4.0 verifies its versioned credential readers by content. Each setup,
sync or Route change hashes the roughly 13 MB program and its reader three or
four times, which costs 35 to 70 ms of CPU over 0.3.5. Together with hosted
runner spikes, that cost pushed some 0.4.0 p95 measurements past their budgets.
The operator accepted the trade-off for 0.4.0 on 2026-10-14; this Change removes
the repeated reads.

## What Changes

- Verify the running program and each versioned reader once per command, and
  project exactly the bytes that verification read.
- Keep verifying content in every command, so a same-size rewrite that leaves
  coarse file timestamps unchanged is still found; within one command, refuse a
  reader that disappears or is replaced before projection completes.
- Requalify the native performance budgets on macOS, Linux and Windows against
  the published 0.4.1 predecessor; `v0.4.0` has no release assets.

## Capabilities

`secret-storage` gains a requirement that one command reads each verified
executable once and installs only the bytes it verified.

## Impact

The owner is `internal/credential` and its synchronization caller. Content
addressing, receipts, the private namespace and versioned reader retention are
unchanged. No cache outlives one command; across commands, file metadata never
stands in for content verification.
