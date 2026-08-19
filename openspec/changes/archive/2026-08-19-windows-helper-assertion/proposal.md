## Why

The host-native executable fixture is valid on Windows, but its assertion still
searched raw JSON bytes. Windows path separators are escaped in JSON, so the
assertion rejected the correct decoded helper value.

## What Changes

Decode the projected settings and assert the semantic `apiKeyHelper` value
instead of its serialization.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This is a test-only assertion correction.

## Impact

Only one synchronization test and this completed Change record change.
