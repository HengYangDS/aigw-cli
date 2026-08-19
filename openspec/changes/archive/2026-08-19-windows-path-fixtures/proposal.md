## Why

Native Windows verification exercised Claude projection with POSIX-only executable fixtures. The product correctly rejected those values, so the test data—not the product contract—caused the hosted failure.

## What Changes

- derive absolute executable fixtures with the host path implementation;
- preserve explicit assertions for helper quoting and invalid paths;
- keep production code, client configuration, credentials, and runtime behavior unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This is a test-only portability correction.

## Impact

Only Claude projection tests and this completed change record are affected. No public API, dependency, release topology, or product behavior changes.
