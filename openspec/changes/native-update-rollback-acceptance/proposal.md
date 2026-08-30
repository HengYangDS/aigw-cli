## Why

AIGW's native acceptance proves source builds, portable installation, setup,
credentials, client projection, and uninstall on macOS, Linux, and Windows, but
its released-artifact update and rollback behavior is proved only below the
public CLI boundary. A trusted release therefore lacks direct evidence that the
same shipped executable can update, roll back, recover forward, and uninstall
without losing retained user state or leaving lifecycle residue.

## What Changes

- Extend the existing native product journey to exercise the public update and
  rollback commands using real portable archives and checksum manifests.
- Require every supported native host to prove install-old, update-new,
  rollback-old, recover-new, and uninstall behavior from released artifact
  shapes.
- Verify that configuration and credentials follow the existing retention
  contract while the executable, its single rollback copy, staging files, and
  other lifecycle residue converge to the declared terminal state.
- Reuse the existing `tools/ci native` command and `internal/upgrade`
  implementation; do not add a lifecycle framework, script state machine,
  platform-specific product path, or Proxy dependency.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: Require the public installed executable to prove its
  complete portable update, rollback, forward-recovery, and uninstall journey
  on every supported native platform.

## Impact

- Affected surfaces: `tools/ci/native_journey_test.go`, the existing native CI
  command, and the product-control-plane specification.
- No public command, configuration schema, dependency, client Adapter, provider
  behavior, credential backend, or external service ownership is added.
