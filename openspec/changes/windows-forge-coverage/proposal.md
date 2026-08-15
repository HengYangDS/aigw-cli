# Native Windows Forge Coverage

## Why

Native Windows acceptance is the only failing GitHub job. The product and
governance checks pass, but `tools/forge` records 168 of 177 branches, or
94.92 percent, below the repository-wide strict greater-than-95-percent floor.

## What Changes

- Exercise the projected target revision failure boundary in
  `prepareProjection`.
- Keep the existing coverage policy unchanged.
- Use the existing cross-platform Git test helper rather than add production
  code or platform-specific behavior.

## Acceptance

- The regression is observed red before helper support exists.
- Focused `tools/forge` tests pass after the helper recognizes the exact
  failure mode.
- Full statement and branch coverage passes on the locked toolchain and the
  hosted Windows job becomes green.
