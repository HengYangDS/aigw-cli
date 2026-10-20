# Failure Stage Diagnostics

## Why

AIGW reports every local file failure as "Local file access failed" and every
configuration load failure as "Cannot read or validate local configuration".
Neither names the failed operation or stage, so an operator cannot tell a
missing file from a permission refusal or a schema mismatch, and the file
access behind the 2026-10-19 report remains unlocated. Credential readers
already name their version and failed stage; the full CLI does not.

The native job commands from `repository-paradigm-adoption` also leave their
failure paths unexercised, so aggregate statement coverage on Windows is
94.94%, below the 95% floor.

## What Changes

- A local file failure names the failed operation and the operating-system
  error, without the file path.
- A configuration load failure names its stage: the read stage with its
  operation and error, the parse stage with its line and column, or the
  validate stage with the running AIGW version and the next step for a
  configuration that a newer AIGW wrote.
- Tests cover the native job commands' failure paths, so every native platform
  stays above the coverage floor.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: gains the requirement "Local failures name the
  failed stage".

## Impact

Only diagnostic text changes. No path, configuration content, credential or
internal error message enters human or JSON output, and no stored file changes.
