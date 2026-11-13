## 1. Tests

- [x] 1.1 Replace the six test-side five-second bounds in the macOS Keychain tests with `keychainFixtureTimeout`
      (30 seconds); `go test ./internal/secret/native` passes on macOS, and no product deadline changes.
- [x] 1.2 Add the test-bound scenario to the `secret-storage` requirement; strict OpenSpec validation of the Change
      passes.

## 2. Verification

- [ ] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane head.
- [ ] 2.2 The hosted graph passes at the proposal head.
