## 1. Contract

- [x] 1.1 Add RED store tests proving availability does not call value retrieval and preserves backend errors; verify the focused tests fail for the current `Has` contract
- [x] 1.2 Add RED caller tests for status, setup, sync, and authentication boundaries; verify read-only paths retrieve no Token bytes and authentication reads once

## 2. Implementation

- [x] 2.1 Replace Boolean `Has` with one error-bearing `Exists` operation across API Token and diagnostic slots; verify all store tests pass
- [x] 2.2 Implement metadata-only Keyring observation for macOS, Linux, and Windows without adding a cache or fallback authority; verify platform-focused tests and cross-builds pass
- [x] 2.3 Migrate callers to propagate observation failures or read the Token only when consumed; verify focused CLI and synchronization tests pass

## 3. Product Evidence

- [x] 3.1 Verify real macOS Keychain status/setup/sync observation produces no credential prompt and authentication still succeeds
- [x] 3.2 Verify native Linux Secret Service and Windows Credential Manager build, install, observation, and authentication journeys
- [x] 3.3 Run formatting, static analysis, full source gates, and exact-HEAD ETHOS proof with pristine output
