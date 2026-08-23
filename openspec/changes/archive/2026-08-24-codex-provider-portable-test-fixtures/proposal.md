# Portable Codex Provider Test Fixtures

## Why

The native-provider acceptance tests used the POSIX-only `/opt/aigw` fixture.
The product implementation was portable, but the fixture failed on Windows
before exercising it and therefore invalidated native Windows evidence.

## What changes

- derive synthetic absolute credential-command paths with `filepath.Join`;
- compare rendered TOML against the platform-native path with Go quoting;
- retain the existing requirement and production behavior unchanged.

## Capabilities

No capability changes. This is a hermetic cross-platform test repair.

## Impact

Only native-provider test fixtures and their lifecycle record change.
