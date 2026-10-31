## Why

ETHOS v0.2.0-alpha.23, released on 2026-10-31, extends its 500 effective-line
ceiling from Python to every language it measures, Go included. Ten aigw-cli
Go test files hold 501 to 509 lines, so the repository cannot rebind to that
runtime. The repository's own source-size gate still allows 512 lines for code
and for main specifications, so a file can pass one gate and fail the other.

## What Changes

- Lower the SCC policy's `max_code_lines` and `max_specification_lines` from
  512 to 500, so that the repository's gate and ETHOS share one ceiling, and
  move the gate's boundary tests to 500 and 501 lines.
- Move one group of tests that share a concern out of each of the ten files,
  into a file named after that concern in the same package or into the file
  that already owns it.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-quality`: when the governing framework tightens a size limit that
  the repository's own gate also enforces, the repository's limit follows it in
  the same change, and each file above it gives up a whole concern.

## Impact

Test files and the SCC policy change; no product code, release artifact,
configuration or credential changes, so no release follows. Once this Change
lands, the repository rebinds to ETHOS alpha.23 through its runtime selector.
