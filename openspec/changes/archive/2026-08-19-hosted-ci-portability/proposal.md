## Why

Hosted native verification exposed two test-fixture defects that local macOS
execution could not reveal: one synchronization test still supplied a POSIX
absolute executable on Windows, and one Forge failure test inferred
unwritability from POSIX mode bits while the Linux container runs as root.

## What Changes

- derive the AIGW executable fixture from the executing host and assert the
  existing helper encoder's result;
- exercise rejected branch publication with a bare-repository hook rather than
  filesystem permission inference;
- keep production behavior, CI topology, coverage policy, and platform
  admission unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This is a test-only portability correction.

## Impact

Only cross-platform test fixtures and this completed Change record change.
